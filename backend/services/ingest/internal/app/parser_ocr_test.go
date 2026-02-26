package app

import (
	"strings"
	"testing"
)

func TestParsePaddleOCRJSONWithOCRResults(t *testing.T) {
	raw := []byte(`{
  "ocrResults": [
    {
      "page_index": 0,
      "prunedResult": {
        "rec_texts": ["第一行", "第二行"],
        "rec_scores": [0.90, 0.70]
      }
    },
    {
      "page_index": 1,
      "prunedResult": {
        "rec_texts": ["第三行"],
        "rec_scores": [0.80]
      }
    }
  ]
}`)
	pages, err := parsePaddleOCRJSON(raw)
	if err != nil {
		t.Fatalf("parsePaddleOCRJSON() error = %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("len(pages) = %d, want 2", len(pages))
	}
	if pages[0].Page != 1 || strings.TrimSpace(pages[0].Text) != "第一行\n第二行" {
		t.Fatalf("page[0] = %+v, want page=1 text=第一行\\n第二行", pages[0])
	}
	if pages[0].AvgScore <= 0 || pages[0].AvgScore < 0.79 || pages[0].AvgScore > 0.81 {
		t.Fatalf("page[0].AvgScore = %f, want about 0.8", pages[0].AvgScore)
	}
	if pages[1].Page != 2 || strings.TrimSpace(pages[1].Text) != "第三行" {
		t.Fatalf("page[1] = %+v, want page=2 text=第三行", pages[1])
	}
	if pages[1].AvgScore < 0.79 || pages[1].AvgScore > 0.81 {
		t.Fatalf("page[1].AvgScore = %f, want about 0.8", pages[1].AvgScore)
	}
}

func TestParsePaddleOCRJSONFallbackToSinglePage(t *testing.T) {
	raw := []byte(`{"result":{"rec_texts":["Only one page text"]}}`)
	pages, err := parsePaddleOCRJSON(raw)
	if err != nil {
		t.Fatalf("parsePaddleOCRJSON() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("len(pages) = %d, want 1", len(pages))
	}
	if pages[0].Page != 1 || strings.TrimSpace(pages[0].Text) != "Only one page text" {
		t.Fatalf("page[0] = %+v, want page=1 text=Only one page text", pages[0])
	}
}
