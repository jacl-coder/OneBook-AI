package eval

import (
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

func estimateLengthUnits(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	return utf8.RuneCountInString(trimmed)
}

func normalizeTextForDup(text string) string {
	text = strings.ToLower(text)
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func vectorNorm(v []float32) float64 {
	if len(v) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range v {
		xf := float64(x)
		sum += xf * xf
	}
	return math.Sqrt(sum)
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return minFloat(values)
	}
	if p >= 1 {
		return maxFloat(values)
	}
	cloned := append([]float64(nil), values...)
	sort.Float64s(cloned)
	idx := p * float64(len(cloned)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return cloned[lo]
	}
	weight := idx - float64(lo)
	return cloned[lo]*(1-weight) + cloned[hi]*weight
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func minFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
