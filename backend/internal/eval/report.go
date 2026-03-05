package eval

import "path/filepath"

func WriteReport(outDir string, run ReportRun, metrics map[string]any, perQuery []map[string]any, gate GateSummary) error {
	if err := ensureDir(outDir); err != nil {
		return err
	}
	metricsCopy := map[string]any{}
	for k, v := range metrics {
		metricsCopy[k] = v
	}
	metricsCopy["gate_result"] = gate

	if err := writeJSON(filepath.Join(outDir, "run.json"), run); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "metrics.json"), metricsCopy); err != nil {
		return err
	}
	if err := writeJSONL(filepath.Join(outDir, "per_query.jsonl"), perQuery); err != nil {
		return err
	}
	return nil
}
