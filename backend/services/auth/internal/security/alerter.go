package security

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var alertCounterScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return count
`)

// AlertResult contains alert evaluation output.
type AlertResult struct {
	Triggered bool
	Count     int64
	Threshold int64
	Window    time.Duration
}

// AuditAlerter aggregates security events and triggers threshold alerts.
type AuditAlerter struct {
	redisClient *redis.Client
	prefix      string
}

// NewAuditAlerter creates an alerter backed by Redis counters.
func NewAuditAlerter(addr, password, prefix string) *AuditAlerter {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "onebook:auth:alerts"
	}
	return &AuditAlerter{
		redisClient: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
		}),
		prefix: prefix,
	}
}

// Observe records a security event and returns whether alert threshold is reached.
func (a *AuditAlerter) Observe(event, outcome, ip string) (AlertResult, error) {
	result := AlertResult{}
	if a == nil || a.redisClient == nil {
		return result, nil
	}
	threshold, window, ok := alertRule(event, outcome)
	if !ok {
		return result, nil
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		ip = "unknown"
	}
	windowMs := window.Milliseconds()
	if windowMs <= 0 {
		return result, nil
	}
	slot := time.Now().UTC().UnixMilli() / windowMs
	key := fmt.Sprintf("%s:%s:%s:%s:%d", a.prefix, sanitizeSegment(event), sanitizeSegment(outcome), sanitizeSegment(ip), slot)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	count, err := alertCounterScript.Run(ctx, a.redisClient, []string{key}, windowMs).Int64()
	if err != nil {
		return result, err
	}
	result.Count = count
	result.Threshold = threshold
	result.Window = window
	result.Triggered = count >= threshold
	return result, nil
}

func alertRule(event, outcome string) (threshold int64, window time.Duration, ok bool) {
	event = strings.TrimSpace(event)
	outcome = strings.TrimSpace(outcome)
	if outcome == "rate_limited" {
		return 20, time.Minute, true
	}
	if outcome != "fail" {
		return 0, 0, false
	}
	switch event {
	case "auth.login", "auth.signup":
		return 10, 5 * time.Minute, true
	case "auth.refresh", "auth.logout", "auth.password.change":
		return 15, 5 * time.Minute, true
	case "auth.authorize", "auth.admin.authorize":
		return 25, 5 * time.Minute, true
	default:
		return 0, 0, false
	}
}

func sanitizeSegment(in string) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer(":", "_", "|", "_", " ", "_")
	return replacer.Replace(in)
}
