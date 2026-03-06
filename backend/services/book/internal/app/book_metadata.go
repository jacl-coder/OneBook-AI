package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"onebookai/pkg/domain"
)

const (
	maxBookTags       = 5
	maxBookTagRunes   = 24
	maxBookTitleRunes = 120
)

func normalizePrimaryCategory(value string) (string, error) {
	normalized := domain.NormalizeBookPrimaryCategory(value)
	if strings.TrimSpace(value) != "" && normalized == domain.BookCategoryOther && strings.TrimSpace(strings.ToLower(value)) != string(domain.BookCategoryOther) {
		return "", fmt.Errorf("invalid primary category")
	}
	return string(normalized), nil
}

func normalizeBookTags(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return []string{}, nil
	}
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, raw := range tags {
		tag := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > maxBookTagRunes {
			return nil, fmt.Errorf("tag too long")
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, tag)
		if len(normalized) > maxBookTags {
			return nil, fmt.Errorf("too many tags")
		}
	}
	return normalized, nil
}

func normalizeBookTitle(value string) (string, error) {
	title := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if title == "" {
		return "", fmt.Errorf("title required")
	}
	if utf8.RuneCountInString(title) > maxBookTitleRunes {
		return "", fmt.Errorf("title too long")
	}
	return title, nil
}

func detectBookFormat(filename string) string {
	switch strings.ToLower(strings.TrimSpace(filepath.Ext(filename))) {
	case ".pdf":
		return string(domain.BookFormatPDF)
	case ".epub":
		return string(domain.BookFormatEPUB)
	case ".txt":
		return string(domain.BookFormatTXT)
	default:
		return ""
	}
}
