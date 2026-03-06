package retrieval

import "testing"

func TestDetectLanguage(t *testing.T) {
	if got := DetectLanguage("请总结第一章的核心观点"); got != "zh" {
		t.Fatalf("DetectLanguage(zh) = %q, want zh", got)
	}
	if got := DetectLanguage("Summarize the first chapter"); got != "en" {
		t.Fatalf("DetectLanguage(en) = %q, want en", got)
	}
}

func TestBuildSparseVector(t *testing.T) {
	vector := BuildSparseVector("OneBook AI retrieval retrieval", "en")
	if len(vector.Indices) == 0 {
		t.Fatalf("BuildSparseVector() indices empty")
	}
	if len(vector.Indices) != len(vector.Values) {
		t.Fatalf("indices/values length mismatch: %d != %d", len(vector.Indices), len(vector.Values))
	}
}

func TestBuildQueryVariants(t *testing.T) {
	variants := BuildQueryVariants("请问一下，如何重置密码？")
	if len(variants) == 0 {
		t.Fatalf("BuildQueryVariants() returned empty result")
	}
}
