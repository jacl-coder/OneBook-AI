package util

import (
	"net"
	"net/http"
	"strings"
)

// TrustedProxies holds proxy CIDR allowlist used for forwarded-header trust.
type TrustedProxies struct {
	nets []*net.IPNet
}

// NewTrustedProxies parses CIDR/IP entries into a trusted proxy allowlist.
// Empty input means "trust none".
func NewTrustedProxies(entries []string) (*TrustedProxies, error) {
	nets := make([]*net.IPNet, 0, len(entries))
	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, cidr, err := net.ParseCIDR(entry)
			if err != nil {
				return nil, err
			}
			nets = append(nets, cidr)
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			return nil, &net.ParseError{Type: "IP address", Text: entry}
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		nets = append(nets, &net.IPNet{
			IP:   ip,
			Mask: net.CIDRMask(bits, bits),
		})
	}
	if len(nets) == 0 {
		return nil, nil
	}
	return &TrustedProxies{nets: nets}, nil
}

// Contains reports whether the given IP is inside trusted proxy ranges.
func (t *TrustedProxies) Contains(ip net.IP) bool {
	if t == nil || ip == nil {
		return false
	}
	for _, n := range t.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIP resolves the caller IP from request metadata.
// Forwarded headers are trusted only when the direct peer is in trusted proxies.
func ClientIP(r *http.Request, trusted *TrustedProxies) string {
	remoteIP := parseRemoteIP(r.RemoteAddr)
	if remoteIP == nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	if trusted == nil || !trusted.Contains(remoteIP) {
		return remoteIP.String()
	}

	forwarded := parseForwardedFor(r.Header.Get("X-Forwarded-For"))
	if len(forwarded) > 0 {
		chain := append(forwarded, remoteIP)
		for i := len(chain) - 1; i >= 0; i-- {
			if !trusted.Contains(chain[i]) {
				return chain[i].String()
			}
		}
		return chain[0].String()
	}

	if realIP := parseIP(r.Header.Get("X-Real-IP")); realIP != nil {
		return realIP.String()
	}
	return remoteIP.String()
}

func parseForwardedFor(raw string) []net.IP {
	parts := strings.Split(raw, ",")
	out := make([]net.IP, 0, len(parts))
	for _, part := range parts {
		ip := parseIP(part)
		if ip == nil {
			continue
		}
		out = append(out, ip)
	}
	return out
}

func parseRemoteIP(addr string) net.IP {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil
	}
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return parseIP(host)
	}
	return parseIP(addr)
}

func parseIP(raw string) net.IP {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return net.ParseIP(raw)
}
