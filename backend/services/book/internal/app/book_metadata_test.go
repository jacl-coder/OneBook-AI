package app

import "testing"

func TestNormalizePrimaryCategoryRejectsUnknownValue(t *testing.T) {
	t.Parallel()
	if _, err := normalizePrimaryCategory("made_up"); err == nil {
		t.Fatal("expected invalid primary category error")
	}
}

func TestNormalizeBookTagsDedupesAndTrims(t *testing.T) {
	t.Parallel()
	tags, err := normalizeBookTags([]string{" 财务 ", "制度", "财务", "研究生  管理"})
	if err != nil {
		t.Fatalf("normalizeBookTags returned error: %v", err)
	}
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}
	if tags[0] != "财务" || tags[1] != "制度" || tags[2] != "研究生 管理" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
}

func TestNormalizeBookTagsRejectsTooManyTags(t *testing.T) {
	t.Parallel()
	if _, err := normalizeBookTags([]string{"1", "2", "3", "4", "5", "6"}); err == nil {
		t.Fatal("expected too many tags error")
	}
}
