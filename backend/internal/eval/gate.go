package eval

import "strings"

// GateSummary reports warning-only gate status.
type GateSummary struct {
	Mode     string   `json:"mode"`
	Passed   bool     `json:"passed"`
	Warnings []string `json:"warnings,omitempty"`
}

func EvaluateGate(mode string, warnings []string) GateSummary {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "warn"
	}
	warnings = uniqueStrings(warnings)
	summary := GateSummary{Mode: mode, Passed: true, Warnings: warnings}
	switch mode {
	case "off":
		summary.Passed = true
	case "warn":
		summary.Passed = true
	case "strict":
		summary.Passed = len(warnings) == 0
	default:
		summary.Mode = "warn"
		summary.Passed = true
	}
	return summary
}
