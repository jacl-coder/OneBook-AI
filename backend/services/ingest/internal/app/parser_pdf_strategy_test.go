package app

import "testing"

func TestMergePDFPagesPrefersOCRForLowQualityNative(t *testing.T) {
	a := &App{
		pdfMinRunes:  80,
		pdfMinScore:  0.45,
		pdfScoreDiff: 0.08,
	}
	native := []pageExtraction{
		{Page: 1, Text: "A1", Method: "pdftotext"},
	}
	ocr := []pageExtraction{
		{Page: 1, Text: "This is a readable OCR sentence on page one.", Method: "paddleocr", OCRAvgScore: 0.92},
	}
	merged := a.mergePDFPages(native, ocr)
	if len(merged) != 1 {
		t.Fatalf("len(merged) = %d, want 1", len(merged))
	}
	if merged[0].Method != "paddleocr" {
		t.Fatalf("merged[0].Method = %q, want paddleocr", merged[0].Method)
	}
}

func TestMergePDFPagesKeepsHighQualityNative(t *testing.T) {
	a := &App{
		pdfMinRunes:  20,
		pdfMinScore:  0.40,
		pdfScoreDiff: 0.08,
	}
	native := []pageExtraction{
		{Page: 2, Text: "This page is already clear, complete, and easy to retrieve.", Method: "pdftotext"},
	}
	ocr := []pageExtraction{
		{Page: 2, Text: "This page is clear complete and easy to retrieve", Method: "paddleocr", OCRAvgScore: 0.85},
	}
	merged := a.mergePDFPages(native, ocr)
	if len(merged) != 1 {
		t.Fatalf("len(merged) = %d, want 1", len(merged))
	}
	if merged[0].Method != "pdftotext" {
		t.Fatalf("merged[0].Method = %q, want pdftotext", merged[0].Method)
	}
}
