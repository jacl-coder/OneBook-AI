package eval

import (
	"fmt"
	"strings"
)

func EvaluateAnswer(opts AnswerOptions) (EvalResult, error) {
	if strings.TrimSpace(opts.QueriesPath) == "" {
		return EvalResult{}, fmt.Errorf("queries path required")
	}
	if strings.TrimSpace(opts.QrelsPath) == "" {
		return EvalResult{}, fmt.Errorf("qrels path required")
	}
	if strings.TrimSpace(opts.PredictionsPath) == "" {
		return EvalResult{}, fmt.Errorf("predictions path required")
	}
	queries, err := ReadQueriesJSONL(opts.QueriesPath)
	if err != nil {
		return EvalResult{}, err
	}
	qrels, err := ReadQrels(opts.QrelsPath)
	if err != nil {
		return EvalResult{}, err
	}
	preds, err := ReadPredictionsJSONL(opts.PredictionsPath)
	if err != nil {
		return EvalResult{}, err
	}
	predByQ := map[string]PredictionRecord{}
	for _, p := range preds {
		predByQ[p.QID] = p
	}
	relMap := buildRelevantMap(qrels)

	citationHitNumerator := 0
	citationTotal := 0
	unsupported := 0
	abstainCorrect := 0
	nonEmpty := 0
	groundedCount := 0
	groundedEligible := 0
	f1Vals := make([]float64, 0, len(queries))
	f1Count := 0
	per := make([]map[string]any, 0, len(queries))

	for _, q := range queries {
		pred := predByQ[q.QID]
		answer := strings.TrimSpace(pred.Answer)
		if answer != "" {
			nonEmpty++
		}

		if pred.Abstained == q.ExpectAbstain {
			abstainCorrect++
		}

		relevant := relMap[q.QID]
		hitCount := 0
		for _, c := range pred.Citations {
			citationTotal++
			if relevant[c] > 0 {
				hitCount++
				citationHitNumerator++
			} else {
				unsupported++
			}
		}
		if !pred.Abstained && answer != "" {
			groundedEligible++
			if len(pred.Citations) > 0 && hitCount == len(pred.Citations) {
				groundedCount++
			}
		}

		f1 := 0.0
		if strings.TrimSpace(q.ExpectedAnswer) != "" {
			f1 = lexicalF1(q.ExpectedAnswer, pred.Answer)
			f1Vals = append(f1Vals, f1)
			f1Count++
		}
		per = append(per, map[string]any{
			"qid":                  q.QID,
			"answer_nonempty":      answer != "",
			"abstain_correct":      pred.Abstained == q.ExpectAbstain,
			"citation_total":       len(pred.Citations),
			"citation_hit":         hitCount,
			"unsupported_citation": len(pred.Citations) - hitCount,
			"grounded_answer":      !pred.Abstained && answer != "" && len(pred.Citations) > 0 && hitCount == len(pred.Citations),
			"lexical_f1":           f1,
		})
	}

	metrics := map[string]any{
		"queries":                   len(queries),
		"predictions":               len(preds),
		"citation_hit_rate":         safeDiv(float64(citationHitNumerator), float64(citationTotal)),
		"unsupported_citation_rate": safeDiv(float64(unsupported), float64(citationTotal)),
		"abstain_accuracy":          safeDiv(float64(abstainCorrect), float64(len(queries))),
		"answer_nonempty_rate":      safeDiv(float64(nonEmpty), float64(len(queries))),
		"grounded_answer_rate":      safeDiv(float64(groundedCount), float64(groundedEligible)),
	}
	if f1Count > 0 {
		metrics["lexical_f1"] = mean(f1Vals)
	} else {
		metrics["lexical_f1"] = 0.0
	}

	warnings := evaluateAnswerWarnings(metrics)
	return EvalResult{Metrics: metrics, PerQuery: per, Warnings: warnings}, nil
}

func lexicalF1(gold, pred string) float64 {
	gold = strings.TrimSpace(gold)
	pred = strings.TrimSpace(pred)
	if gold == "" || pred == "" {
		return 0
	}
	goldCounts := runeCounts(gold)
	predCounts := runeCounts(pred)

	common := 0
	for r, gc := range goldCounts {
		pc := predCounts[r]
		if pc < gc {
			common += pc
		} else {
			common += gc
		}
	}
	if common == 0 {
		return 0
	}
	precision := float64(common) / float64(len([]rune(pred)))
	recall := float64(common) / float64(len([]rune(gold)))
	if precision+recall == 0 {
		return 0
	}
	return 2 * precision * recall / (precision + recall)
}

func runeCounts(s string) map[rune]int {
	out := map[rune]int{}
	for _, r := range []rune(s) {
		if strings.TrimSpace(string(r)) == "" {
			continue
		}
		out[r]++
	}
	return out
}

func evaluateAnswerWarnings(metrics map[string]any) []string {
	warnings := make([]string, 0)
	cite := metricFloat(metrics, "citation_hit_rate")
	unsupported := metricFloat(metrics, "unsupported_citation_rate")
	abstain := metricFloat(metrics, "abstain_accuracy")
	grounded := metricFloat(metrics, "grounded_answer_rate")
	if cite < 0.6 {
		warnings = append(warnings, fmt.Sprintf("citation_hit_rate %.4f below threshold 0.60", cite))
	}
	if unsupported > 0.2 {
		warnings = append(warnings, fmt.Sprintf("unsupported_citation_rate %.4f exceeds threshold 0.20", unsupported))
	}
	if abstain < 0.8 {
		warnings = append(warnings, fmt.Sprintf("abstain_accuracy %.4f below threshold 0.80", abstain))
	}
	if grounded < 0.7 {
		warnings = append(warnings, fmt.Sprintf("grounded_answer_rate %.4f below threshold 0.70", grounded))
	}
	return warnings
}
