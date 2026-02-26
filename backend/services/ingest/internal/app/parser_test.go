package app

import "testing"

func TestNormalizeTextPreserveNewlines(t *testing.T) {
	raw := "\uFEFF  Title\u00A0\x00\t\nLine\u200B one\u0007\r\n\r\nSecond\u2060 line\u00ad"
	got := normalizeTextPreserveNewlines(raw)
	want := "Title\nLine one\n\nSecond line"
	if got != want {
		t.Fatalf("normalizeTextPreserveNewlines() = %q, want %q", got, want)
	}
}
