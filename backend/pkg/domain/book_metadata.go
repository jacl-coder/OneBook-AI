package domain

import "strings"

type BookPrimaryCategory string

const (
	BookCategoryCourseMaterial   BookPrimaryCategory = "course_material"
	BookCategoryResearchPaper    BookPrimaryCategory = "research_paper"
	BookCategoryProjectDoc       BookPrimaryCategory = "project_doc"
	BookCategoryPolicyRegulation BookPrimaryCategory = "policy_regulation"
	BookCategoryReferenceBook    BookPrimaryCategory = "reference_book"
	BookCategoryPersonalNote     BookPrimaryCategory = "personal_note"
	BookCategoryHowToGuide       BookPrimaryCategory = "how_to_guide"
	BookCategoryOther            BookPrimaryCategory = "other"
)

type BookFormat string

const (
	BookFormatPDF  BookFormat = "pdf"
	BookFormatEPUB BookFormat = "epub"
	BookFormatTXT  BookFormat = "txt"
)

type BookLanguage string

const (
	BookLanguageZH      BookLanguage = "zh"
	BookLanguageEN      BookLanguage = "en"
	BookLanguageOther   BookLanguage = "other"
	BookLanguageUnknown BookLanguage = "unknown"
)

func NormalizeBookPrimaryCategory(value string) BookPrimaryCategory {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case string(BookCategoryCourseMaterial):
		return BookCategoryCourseMaterial
	case string(BookCategoryResearchPaper):
		return BookCategoryResearchPaper
	case string(BookCategoryProjectDoc):
		return BookCategoryProjectDoc
	case string(BookCategoryPolicyRegulation):
		return BookCategoryPolicyRegulation
	case string(BookCategoryReferenceBook):
		return BookCategoryReferenceBook
	case string(BookCategoryPersonalNote):
		return BookCategoryPersonalNote
	case string(BookCategoryHowToGuide):
		return BookCategoryHowToGuide
	default:
		return BookCategoryOther
	}
}

func NormalizeBookFormat(value string) BookFormat {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case string(BookFormatPDF):
		return BookFormatPDF
	case string(BookFormatEPUB):
		return BookFormatEPUB
	case string(BookFormatTXT):
		return BookFormatTXT
	default:
		return ""
	}
}

func NormalizeBookLanguage(value string) BookLanguage {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case string(BookLanguageZH):
		return BookLanguageZH
	case string(BookLanguageEN):
		return BookLanguageEN
	case string(BookLanguageOther):
		return BookLanguageOther
	default:
		return BookLanguageUnknown
	}
}
