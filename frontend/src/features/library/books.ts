export const bookPrimaryCategoryOptions = [
  { value: 'course_material', label: '课程资料' },
  { value: 'research_paper', label: '研究论文' },
  { value: 'project_doc', label: '项目文档' },
  { value: 'policy_regulation', label: '制度文件' },
  { value: 'reference_book', label: '参考书/教材' },
  { value: 'personal_note', label: '个人笔记' },
  { value: 'how_to_guide', label: '办事指南' },
  { value: 'other', label: '其他' },
] as const

export const bookFormatOptions = [
  { value: 'pdf', label: 'PDF' },
  { value: 'epub', label: 'EPUB' },
  { value: 'txt', label: 'TXT' },
] as const

export const bookLanguageOptions = [
  { value: 'zh', label: '中文' },
  { value: 'en', label: '英文' },
  { value: 'other', label: '其他' },
  { value: 'unknown', label: '未知' },
] as const

export type BookPrimaryCategory = (typeof bookPrimaryCategoryOptions)[number]['value']
export type BookFormat = (typeof bookFormatOptions)[number]['value']
export type BookLanguage = (typeof bookLanguageOptions)[number]['value']
export type BookStatus = 'queued' | 'processing' | 'ready' | 'failed'

export function getBookPrimaryCategoryLabel(value: string): string {
  return bookPrimaryCategoryOptions.find((item) => item.value === value)?.label ?? '其他'
}

export function getBookFormatLabel(value: string): string {
  return bookFormatOptions.find((item) => item.value === value)?.label ?? '未知格式'
}

export function getBookLanguageLabel(value: string): string {
  return bookLanguageOptions.find((item) => item.value === value)?.label ?? '未知'
}

export function normalizeTagInput(value: string): string[] {
  return Array.from(
    new Set(
      value
        .split(/[,，]/)
        .map((item) => item.trim().replace(/\s+/g, ' '))
        .filter(Boolean),
    ),
  ).slice(0, 5)
}
