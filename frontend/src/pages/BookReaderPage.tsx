import { useEffect, useMemo, useState } from 'react'
import { Link, Navigate, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'

import {
  getBook,
  getBookContentURL,
  libraryQueryKeys,
  type DocumentEntity,
  type DocumentFact,
  type LibraryBook,
} from '@/features/library/api/library'
import { getApiErrorMessage } from '@/features/auth/api/auth'
import { useSessionStore } from '@/features/auth/store/session'
import { DocumentReader, type DocumentCitationTarget, type DocumentReaderProfile, type DocumentReaderSource } from '@/features/reader'

const cx = (...values: Array<string | false | null | undefined>) =>
  values.filter(Boolean).join(' ')

const readerPageStyle = {
  fontFamily:
    "ui-sans-serif, -apple-system, system-ui, 'Segoe UI', Helvetica, 'Apple Color Emoji', Arial, sans-serif, 'Segoe UI Emoji', 'Segoe UI Symbol'",
} as const

const pageTw = {
  shell: 'grid min-h-screen grid-rows-[52px_minmax(0,1fr)] bg-white text-[#0d0d0d]',
  topBar:
    'flex h-[52px] items-center justify-between border-b border-[rgba(0,0,0,0.08)] bg-white px-3',
  topLeft: 'flex min-w-0 items-center gap-2',
  iconButton:
    'inline-flex h-9 min-w-9 cursor-pointer items-center justify-center rounded-[9px] border-0 bg-transparent px-2 text-[14px] text-[#333] hover:bg-[#f1f1f1]',
  titleWrap: 'grid min-w-0 gap-[1px]',
  title: 'overflow-hidden text-ellipsis whitespace-nowrap text-[15px] font-medium text-[#111]',
  subTitle: 'text-[12px] text-[#6b6b6b]',
  actions: 'flex shrink-0 items-center gap-2',
  pillButton:
    'inline-flex h-8 cursor-pointer items-center justify-center rounded-full border border-[rgba(0,0,0,0.12)] bg-white px-3 text-[12px] font-medium text-[#2f2f2f] hover:bg-[#f6f6f6]',
  body: 'min-h-0',
  notice:
    'm-4 rounded-[12px] border border-[#f4b0b4] bg-[#fff5f6] px-3 py-[10px] text-[13px] text-[#9f1820]',
  loading: 'grid min-h-[calc(100vh-52px)] place-items-center bg-[#f7f7f5] p-6 text-center',
  loadingCard:
    'grid gap-2 rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white px-6 py-5 text-[14px] text-[#555]',
} as const

function formatStatus(status: LibraryBook['status']): string {
  switch (status) {
    case 'queued':
      return '排队中'
    case 'processing':
      return '处理中'
    case 'ready':
      return '可阅读'
    case 'failed':
      return '处理失败'
    default:
      return status
  }
}

function entityLabel(entity: DocumentEntity): string {
  return entity.label || entity.type || '实体'
}

function factLabel(fact: DocumentFact): string {
  return fact.label || fact.key || '事实'
}

function isDateLikeEntity(entity: DocumentEntity): boolean {
  const type = entity.type.toLowerCase()
  return type.includes('date') || type.includes('time') || /时间|日期/.test(entityLabel(entity))
}

function profileFromBook(book: LibraryBook): DocumentReaderProfile {
  const entities = (book.documentEntities ?? []).filter((entity) => !isDateLikeEntity(entity))
  const dateEntities = (book.documentEntities ?? []).filter(isDateLikeEntity)
  const dateFacts = (book.documentFacts ?? []).filter((fact) => /date|time|时间|日期/.test(`${fact.key} ${fact.label ?? ''}`.toLowerCase()))

  return {
    typeLabel: book.documentType || undefined,
    summary: book.documentSummary || book.firstPageText || undefined,
    entities: entities.slice(0, 8).map((entity) => ({
      label: entityLabel(entity),
      value: entity.value,
    })),
    dates: [
      ...dateEntities.map((entity) => ({ label: entityLabel(entity), value: entity.value })),
      ...dateFacts.map((fact) => ({ label: factLabel(fact), value: fact.value })),
    ].slice(0, 8),
    facts: (book.documentFacts ?? []).slice(0, 10).map((fact) => ({
      label: factLabel(fact),
      value: fact.value,
    })),
  }
}

function parseNumberParam(value: string | null): number | undefined {
  if (!value) return undefined
  const parsed = Number.parseInt(value, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
}

function pageFromText(value: string | null): number | undefined {
  if (!value) return undefined
  const match = value.match(/page\s*(\d+)|第\s*(\d+)\s*页/i)
  if (!match) return undefined
  const parsed = Number.parseInt(match[1] || match[2], 10)
  return Number.isFinite(parsed) ? parsed : undefined
}

function citationFromParams(searchParams: URLSearchParams): DocumentCitationTarget[] {
  const snippet = searchParams.get('snippet')?.trim()
  const label = searchParams.get('label')?.trim() || undefined
  const location = searchParams.get('location')?.trim()
  if (!snippet && !label && !location) return []
  const page = parseNumberParam(searchParams.get('page')) ?? pageFromText(label ?? '') ?? pageFromText(location ?? '')
  const lineStart = parseNumberParam(searchParams.get('lineStart'))
  return [
    {
      id: searchParams.get('citation') || 'selected-citation',
      label: label || location || (page ? `page ${page}` : '引用来源'),
      page,
      lineStart,
      lineEnd: parseNumberParam(searchParams.get('lineEnd')) ?? lineStart,
      snippet: snippet || location || label || '已选择的引用来源',
      sourceReason: searchParams.get('reason')?.trim() || undefined,
      sourceType: 'chunk',
      highlightText: searchParams.get('highlight')?.trim() || snippet?.slice(0, 32),
    },
  ]
}

function buildReaderSource(book: LibraryBook, contentURL: string, text: string): DocumentReaderSource {
  if (book.format === 'txt') {
    return {
      id: book.id,
      title: book.title,
      format: 'txt',
      url: contentURL,
      text: text || book.firstPageText || book.documentSummary || '',
    }
  }
  if (book.format === 'epub') {
    return {
      id: book.id,
      title: book.title,
      format: 'epub',
      url: contentURL,
      chapters: [
        {
          id: 'overview',
          title: '文档概览',
          content: book.firstPageText || book.documentSummary || 'EPUB 原文解析器接入后将在这里显示章节内容。',
        },
      ],
    }
  }
  return {
    id: book.id,
    title: book.title,
    format: 'pdf',
    url: contentURL,
    text: book.firstPageText,
  }
}

export function BookReaderPage() {
  const { bookId = '' } = useParams()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const sessionUser = useSessionStore((state) => state.user)
  const [textLoadState, setTextLoadState] = useState({ bookId: '', text: '', error: '' })

  const bookQuery = useQuery({
    queryKey: libraryQueryKeys.book(bookId),
    queryFn: () => getBook(bookId),
    enabled: Boolean(sessionUser && bookId),
  })

  const book = bookQuery.data
  const contentURL = book ? getBookContentURL(book.id) : ''

  useEffect(() => {
    if (!book || book.format !== 'txt') {
      return undefined
    }
    const controller = new AbortController()
    fetch(contentURL, { credentials: 'include', signal: controller.signal })
      .then((response) => {
        if (!response.ok) throw new Error(`HTTP ${response.status}`)
        return response.text()
      })
      .then((text) => setTextLoadState({ bookId: book.id, text, error: '' }))
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === 'AbortError') return
        setTextLoadState({
          bookId: book.id,
          text: '',
          error: 'TXT 原文加载失败，已使用文档摘要作为临时预览。',
        })
      })
    return () => controller.abort()
  }, [book, contentURL])

  const textContent = book && textLoadState.bookId === book.id ? textLoadState.text : ''
  const textError = book && textLoadState.bookId === book.id ? textLoadState.error : ''
  const citations = useMemo(() => citationFromParams(searchParams), [searchParams])
  const source = useMemo(
    () => (book ? buildReaderSource(book, contentURL, textContent) : undefined),
    [book, contentURL, textContent],
  )
  const profile = useMemo(() => (book ? profileFromBook(book) : undefined), [book])

  if (!sessionUser) {
    return <Navigate to="/log-in-or-create-account" replace />
  }

  if (!bookId) {
    return <Navigate to="/library" replace />
  }

  if (bookQuery.isLoading) {
    return (
      <div className={pageTw.loading} style={readerPageStyle}>
        <div className={pageTw.loadingCard}>正在打开文档...</div>
      </div>
    )
  }

  if (bookQuery.isError || !book || !source) {
    return (
      <div className={pageTw.loading} style={readerPageStyle}>
        <div className={pageTw.loadingCard}>
          <div>{getApiErrorMessage(bookQuery.error, '文档加载失败。')}</div>
          <Link className={pageTw.pillButton} to="/library">
            返回书库
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className={pageTw.shell} style={readerPageStyle}>
      <header className={pageTw.topBar}>
        <div className={pageTw.topLeft}>
          <button className={pageTw.iconButton} onClick={() => navigate(-1)} type="button" aria-label="返回">
            <i aria-hidden="true" className="fa fa-angle-left" />
          </button>
          <div className={pageTw.titleWrap}>
            <div className={pageTw.title}>{book.title}</div>
            <div className={pageTw.subTitle}>
              {book.originalFilename} · {formatStatus(book.status)}
            </div>
          </div>
        </div>
        <div className={pageTw.actions}>
          <Link className={cx(pageTw.pillButton, 'max-[720px]:hidden')} to="/library">
            书库
          </Link>
          <Link className={pageTw.pillButton} to={`/chat?bookId=${encodeURIComponent(book.id)}`}>
            提问
          </Link>
        </div>
      </header>
      <main className={pageTw.body}>
        {textError ? <div className={pageTw.notice}>{textError}</div> : null}
        <DocumentReader
          activeCitationId={citations[0]?.id}
          citations={citations}
          profile={profile}
          source={source}
        />
      </main>
    </div>
  )
}
