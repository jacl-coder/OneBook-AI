package store

import (
	"testing"
	"time"
)

func TestMemoryTokenRevokerUserCutoffMonotonic(t *testing.T) {
	r := NewMemoryTokenRevoker()
	first := time.Now().UTC().Add(-time.Minute)
	second := time.Now().UTC()

	if err := r.RevokeUser("user-1", first); err != nil {
		t.Fatalf("revoke user first: %v", err)
	}
	if err := r.RevokeUser("user-1", first.Add(-time.Minute)); err != nil {
		t.Fatalf("revoke user older cutoff: %v", err)
	}
	got, err := r.RevokedAfter("user-1")
	if err != nil {
		t.Fatalf("revoked after first: %v", err)
	}
	if !got.Equal(first) {
		t.Fatalf("expected first cutoff to be kept, got %v", got)
	}

	if err := r.RevokeUser("user-1", second); err != nil {
		t.Fatalf("revoke user second: %v", err)
	}
	got, err = r.RevokedAfter("user-1")
	if err != nil {
		t.Fatalf("revoked after second: %v", err)
	}
	if !got.Equal(second) {
		t.Fatalf("expected newest cutoff, got %v", got)
	}
}
