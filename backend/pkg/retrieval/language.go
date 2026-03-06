package retrieval

import (
	"hash/fnv"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const sparseHashMod = 65521

var nonAlphaNumPattern = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func NormalizeText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.ToLower(strings.ToValidUTF8(text, ""))
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\t", " ")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	text = strings.Join(lines, "\n")
	text = strings.ReplaceAll(text, "，", ",")
	text = strings.ReplaceAll(text, "。", ".")
	text = strings.ReplaceAll(text, "：", ":")
	text = strings.ReplaceAll(text, "；", ";")
	text = strings.ReplaceAll(text, "？", "?")
	text = strings.ReplaceAll(text, "！", "!")
	text = strings.ReplaceAll(text, "（", "(")
	text = strings.ReplaceAll(text, "）", ")")
	return strings.TrimSpace(text)
}

func DetectLanguage(text string) string {
	text = NormalizeText(text)
	if text == "" {
		return "other"
	}
	cjkCount := 0
	latinCount := 0
	otherLetters := 0
	for _, r := range text {
		switch {
		case isCJK(r):
			cjkCount++
		case isLatinLetter(r):
			latinCount++
		case unicode.IsLetter(r):
			otherLetters++
		}
	}
	switch {
	case cjkCount > latinCount && cjkCount >= 2:
		return "zh"
	case latinCount >= cjkCount && latinCount >= 3:
		return "en"
	case otherLetters > 0:
		return "other"
	default:
		return "other"
	}
}

func Tokenize(text, language string) []string {
	text = NormalizeText(text)
	if text == "" {
		return nil
	}
	switch strings.TrimSpace(language) {
	case "zh":
		return tokenizeChinese(text)
	case "en":
		return tokenizeLatin(text)
	default:
		tokens := tokenizeLatin(text)
		if len(tokens) > 0 {
			return tokens
		}
		return tokenizeChinese(text)
	}
}

func BuildSparseVector(text, language string) SparseVector {
	tokens := Tokenize(text, language)
	if len(tokens) == 0 {
		return SparseVector{}
	}
	counts := map[uint32]float32{}
	for _, token := range tokens {
		if strings.TrimSpace(token) == "" {
			continue
		}
		index := hashToken(token)
		counts[index]++
	}
	if len(counts) == 0 {
		return SparseVector{}
	}
	indices := make([]uint32, 0, len(counts))
	for idx := range counts {
		indices = append(indices, idx)
	}
	slices.Sort(indices)
	values := make([]float32, 0, len(indices))
	denominator := float32(len(tokens))
	if denominator <= 0 {
		denominator = 1
	}
	for _, idx := range indices {
		values = append(values, counts[idx]/denominator)
	}
	return SparseVector{
		Indices: indices,
		Values:  values,
	}
}

func BuildQueryVariants(query string) []string {
	normalized := NormalizeText(query)
	if normalized == "" {
		return nil
	}
	variants := []string{normalized}
	withoutQuestion := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(normalized, "?"), "？"))
	if withoutQuestion != "" && withoutQuestion != normalized {
		variants = append(variants, withoutQuestion)
	}
	for _, prefix := range []string{"请问一下", "请问", "帮我", "我想知道", "我想了解", "what is", "how to", "tell me"} {
		trimmed := strings.TrimSpace(strings.TrimPrefix(withoutQuestion, prefix))
		if trimmed != "" && trimmed != normalized && trimmed != withoutQuestion {
			variants = append(variants, trimmed)
		}
	}
	return uniqueStrings(variants)
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func tokenizeLatin(text string) []string {
	normalized := nonAlphaNumPattern.ReplaceAllString(text, " ")
	fields := strings.Fields(normalized)
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if utf8.RuneCountInString(field) < 2 {
			continue
		}
		out = append(out, field)
	}
	return out
}

func tokenizeChinese(text string) []string {
	runes := make([]rune, 0, utf8.RuneCountInString(text))
	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || isCJK(r) {
			runes = append(runes, r)
		}
	}
	if len(runes) == 0 {
		return nil
	}
	out := make([]string, 0, len(runes)*2)
	for i, r := range runes {
		out = append(out, string(r))
		if i+1 < len(runes) {
			out = append(out, string([]rune{runes[i], runes[i+1]}))
		}
	}
	return out
}

func hashToken(token string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(token))
	return h.Sum32() % sparseHashMod
}

func isLatinLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han)
}
