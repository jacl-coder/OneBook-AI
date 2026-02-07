package security

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestAuditAlerterObserveTriggers(t *testing.T) {
	redis := miniredis.RunT(t)
	alerter := NewAuditAlerter(redis.Addr(), "", "test:alerts")
	if alerter == nil {
		t.Fatalf("expected alerter")
	}
	var lastTriggered bool
	for i := 0; i < 10; i++ {
		result, err := alerter.Observe("auth.login", "fail", "127.0.0.1")
		if err != nil {
			t.Fatalf("observe: %v", err)
		}
		lastTriggered = result.Triggered
	}
	if !lastTriggered {
		t.Fatalf("expected alert threshold to trigger")
	}
}

func TestAuditAlerterObserveIgnoresUnknownRule(t *testing.T) {
	redis := miniredis.RunT(t)
	alerter := NewAuditAlerter(redis.Addr(), "", "test:alerts")
	result, err := alerter.Observe("auth.custom", "success", "127.0.0.1")
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result.Triggered {
		t.Fatalf("unexpected trigger for unknown rule")
	}
}
