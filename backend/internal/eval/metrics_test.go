package eval

import (
	"path/filepath"
	"testing"
)

func TestEvaluateRetrievalMetrics(t *testing.T) {
	base := filepath.Join("testdata")
	res, _, err := EvaluateRetrieval(RetrievalOptions{
		QueriesPath: filepath.Join(base, "queries.jsonl"),
		QrelsPath:   filepath.Join(base, "qrels.tsv"),
		RunPath:     filepath.Join(base, "run.jsonl"),
		TopK:        20,
	})
	if err != nil {
		t.Fatalf("EvaluateRetrieval failed: %v", err)
	}
	if metricFloat(res.Metrics, "Recall@20") <= 0 {
		t.Fatalf("expected Recall@20 > 0, got %v", res.Metrics["Recall@20"])
	}
	if metricFloat(res.Metrics, "MRR@10") <= 0 {
		t.Fatalf("expected MRR@10 > 0, got %v", res.Metrics["MRR@10"])
	}
}

func TestEvaluateAnswerMetrics(t *testing.T) {
	base := filepath.Join("testdata")
	res, err := EvaluateAnswer(AnswerOptions{
		QueriesPath:     filepath.Join(base, "queries.jsonl"),
		QrelsPath:       filepath.Join(base, "qrels.tsv"),
		PredictionsPath: filepath.Join(base, "predictions.jsonl"),
	})
	if err != nil {
		t.Fatalf("EvaluateAnswer failed: %v", err)
	}
	if metricFloat(res.Metrics, "citation_hit_rate") < 0.5 {
		t.Fatalf("expected citation_hit_rate >= 0.5, got %v", res.Metrics["citation_hit_rate"])
	}
	if metricFloat(res.Metrics, "abstain_accuracy") < 0.66 {
		t.Fatalf("expected abstain_accuracy >= 0.66, got %v", res.Metrics["abstain_accuracy"])
	}
}

func TestEvaluateIngestionRequiresInput(t *testing.T) {
	if _, err := EvaluateIngestion(IngestionOptions{}); err == nil {
		t.Fatalf("expected error for missing chunks path")
	}
}
