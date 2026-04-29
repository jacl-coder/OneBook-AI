import type React from 'react'

export type DocumentReaderFormat = 'pdf' | 'epub' | 'txt'

export type DocumentReaderRoute = 'overview' | 'single_fact' | 'summary' | 'analysis'

export type DocumentCitationSource = 'chunk' | 'fact' | 'profile' | 'summary'

export type EpubChapterContentType = 'text' | 'html'

export interface DocumentReaderSource {
  id: string
  title: string
  format: DocumentReaderFormat
  url?: string
  text?: string
  chapters?: EpubChapter[]
}

export interface EpubChapter {
  id: string
  title: string
  href?: string
  content: string
  contentType?: EpubChapterContentType
}

export interface DocumentProfileItem {
  label: string
  value: string
}

export interface DocumentReaderProfile {
  typeLabel?: string
  summary?: string
  entities?: DocumentProfileItem[]
  dates?: DocumentProfileItem[]
  facts?: DocumentProfileItem[]
}

export interface DocumentCitationTarget {
  id: string
  label?: string
  page?: number
  chapterId?: string
  lineStart?: number
  lineEnd?: number
  snippet: string
  sourceReason?: string
  sourceType?: DocumentCitationSource
  highlightText?: string
}

export interface DocumentReaderLocation {
  format: DocumentReaderFormat
  page?: number
  chapterId?: string
  line?: number
}

export interface DocumentReaderProps {
  source: DocumentReaderSource
  profile?: DocumentReaderProfile
  citations?: DocumentCitationTarget[]
  activeCitationId?: string
  chatSlot?: React.ReactNode
  className?: string
  onCitationSelect?: (citation: DocumentCitationTarget) => void
  onLocationChange?: (location: DocumentReaderLocation) => void
}
