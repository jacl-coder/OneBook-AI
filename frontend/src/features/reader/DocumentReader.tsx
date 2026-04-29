import { useEffect, useMemo, useRef, useState } from 'react'

import type {
  DocumentCitationTarget,
  DocumentReaderFormat,
  DocumentReaderLocation,
  DocumentReaderProfile,
  DocumentReaderProps,
  DocumentReaderSource,
  EpubChapter,
} from './types'

const cx = (...values: Array<string | false | null | undefined>) =>
  values.filter(Boolean).join(' ')

const readerTw = {
  shell:
    'grid h-full min-h-[680px] grid-cols-[minmax(0,1fr)_360px] overflow-hidden bg-[#f7f7f5] text-[#111] max-[980px]:grid-cols-1',
  leftPane: 'grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden',
  toolbar:
    'flex min-h-[52px] items-center justify-between gap-3 border-b border-[rgba(0,0,0,0.08)] bg-[#ffffff] px-3',
  toolbarLeft: 'flex min-w-0 items-center gap-2',
  toolbarTitleWrap: 'grid min-w-0 gap-[1px]',
  toolbarTitle:
    'overflow-hidden text-ellipsis whitespace-nowrap text-[14px] leading-5 font-medium text-[#111]',
  toolbarSub: 'text-[12px] leading-4 text-[#6b6b6b]',
  toolbarRight: 'flex shrink-0 items-center gap-1',
  iconBtn:
    'inline-flex h-8 min-w-8 cursor-pointer items-center justify-center rounded-[8px] border-0 bg-transparent px-2 text-[13px] text-[#333] hover:bg-[#ececec] focus-visible:bg-[#ececec] focus-visible:outline-none',
  formatTabs:
    'hidden items-center rounded-[9px] bg-[#eeeeec] p-[2px] text-[12px] text-[#5f5f5f] sm:inline-flex',
  formatTab:
    'inline-flex h-7 min-w-[42px] items-center justify-center rounded-[7px] px-2 font-medium',
  formatTabActive: 'bg-white text-[#111] shadow-[0_1px_2px_rgba(0,0,0,0.08)]',
  readerCanvas:
    'relative min-h-0 overflow-hidden bg-[#ededeb] max-[980px]:min-h-[560px]',
  readerScroll:
    'h-full overflow-auto px-4 py-5 [scrollbar-gutter:stable] max-[700px]:px-2',
  page:
    'mx-auto min-h-[720px] w-full max-w-[820px] rounded-[4px] bg-white px-12 py-10 shadow-[0_8px_28px_rgba(0,0,0,0.16)] max-[700px]:px-5',
  pageText:
    'whitespace-pre-wrap break-words text-[16px] leading-8 text-[#1b1b1b] max-[700px]:text-[15px]',
  pageMuted:
    'grid min-h-[520px] place-items-center text-center text-[13px] leading-6 text-[#777]',
  pdfObject: 'h-full min-h-[620px] w-full border-0 bg-white',
  citationMark:
    'rounded-[5px] bg-[#fff0a8] px-[2px] py-[1px] shadow-[0_0_0_1px_rgba(176,130,0,0.18)]',
  rightPane:
    'grid min-h-0 grid-rows-[auto_minmax(0,1fr)] border-l border-[rgba(0,0,0,0.08)] bg-white max-[980px]:border-l-0 max-[980px]:border-t',
  panelHeader: 'border-b border-[rgba(0,0,0,0.08)] px-4 py-3',
  panelTitle: 'text-[14px] font-medium text-[#111]',
  panelSub: 'mt-1 text-[12px] text-[#6b6b6b]',
  panelScroll: 'min-h-0 overflow-auto p-4',
  profileCard:
    'rounded-[12px] border border-[rgba(0,0,0,0.08)] bg-[#fbfbfa] p-3',
  profileType:
    'inline-flex w-fit rounded-full bg-[#e9f1ff] px-2 py-[3px] text-[11px] font-medium text-[#2458b8]',
  profileSummary: 'mt-2 text-[13px] leading-6 text-[#3f3f3f]',
  profileGrid: 'mt-3 grid gap-2',
  profileRow: 'grid grid-cols-[84px_minmax(0,1fr)] gap-2 text-[12px] leading-5',
  profileLabel: 'text-[#777]',
  profileValue: 'min-w-0 break-words text-[#222]',
  sectionTitle: 'mb-2 mt-4 text-[12px] font-medium uppercase tracking-[0.04em] text-[#777]',
  citationList: 'grid gap-2',
  citationButton:
    'grid cursor-pointer gap-1 rounded-[12px] border border-[rgba(0,0,0,0.08)] bg-white p-3 text-left hover:border-[rgba(0,0,0,0.18)] hover:bg-[#fbfbfb] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8db7ff]',
  citationButtonActive:
    'border-[#8db7ff] bg-[#f4f8ff] shadow-[0_0_0_1px_rgba(60,130,246,0.18)]',
  citationTop: 'flex items-center justify-between gap-2',
  citationPage: 'text-[12px] font-medium text-[#111]',
  citationType:
    'rounded-full bg-[#efefef] px-2 py-[2px] text-[11px] text-[#666]',
  citationSnippet:
    'line-clamp-3 text-[12px] leading-5 text-[#404040]',
  citationReason: 'text-[11px] leading-4 text-[#777]',
  emptyCard:
    'rounded-[12px] border border-dashed border-[rgba(0,0,0,0.16)] bg-[#fbfbfb] p-4 text-[13px] leading-6 text-[#666]',
  epubLayout: 'grid h-full grid-cols-[220px_minmax(0,1fr)] max-[760px]:grid-cols-1',
  epubToc:
    'min-h-0 overflow-auto border-r border-[rgba(0,0,0,0.08)] bg-[#fafafa] p-2 max-[760px]:max-h-[180px] max-[760px]:border-r-0 max-[760px]:border-b',
  epubTocButton:
    'block w-full cursor-pointer rounded-[8px] border-0 bg-transparent px-3 py-2 text-left text-[13px] leading-5 text-[#444] hover:bg-[#eeeeec]',
  epubTocButtonActive: 'bg-[#e9f1ff] text-[#174ea6]',
  lineRow:
    'grid grid-cols-[56px_minmax(0,1fr)] gap-3 border-b border-transparent px-2 py-[2px] text-[14px] leading-7',
  lineNo: 'select-none text-right text-[11px] text-[#9b9b9b]',
  lineText: 'min-w-0 whitespace-pre-wrap break-words text-[#222]',
  lineActive: 'rounded-[6px] bg-[#fff6cf]',
} as const

function formatLabel(format: DocumentReaderFormat): string {
  switch (format) {
    case 'pdf':
      return 'PDF'
    case 'epub':
      return 'EPUB'
    case 'txt':
      return 'TXT'
    default:
      return format
  }
}

function citationLabel(citation: DocumentCitationTarget): string {
  if (citation.label) return citation.label
  if (citation.page) return `page ${citation.page}`
  if (citation.lineStart) return `line ${citation.lineStart}`
  return 'source'
}

function getActiveCitation(
  citations: DocumentCitationTarget[],
  activeCitationId?: string,
): DocumentCitationTarget | undefined {
  if (!citations.length) return undefined
  return citations.find((citation) => citation.id === activeCitationId) ?? citations[0]
}

function sourceTypeLabel(sourceType: DocumentCitationTarget['sourceType']): string {
  switch (sourceType) {
    case 'fact':
      return 'fact'
    case 'profile':
      return 'profile'
    case 'summary':
      return 'summary'
    case 'chunk':
    default:
      return 'chunk'
  }
}

function HighlightedText({
  text,
  highlightText,
}: {
  text: string
  highlightText?: string
}) {
  if (!highlightText) {
    return <>{text}</>
  }
  const index = text.toLowerCase().indexOf(highlightText.toLowerCase())
  if (index < 0) {
    return <>{text}</>
  }
  const before = text.slice(0, index)
  const match = text.slice(index, index + highlightText.length)
  const after = text.slice(index + highlightText.length)
  return (
    <>
      {before}
      <mark className={readerTw.citationMark}>{match}</mark>
      {after}
    </>
  )
}

function ProfilePanel({ profile }: { profile?: DocumentReaderProfile }) {
  if (!profile) {
    return (
      <div className={readerTw.emptyCard}>
        文档资料会显示在这里：类型、摘要、关键人物、关键日期和结构化事实。
      </div>
    )
  }

  const groups = [
    { title: '关键实体', items: profile.entities ?? [] },
    { title: '关键日期', items: profile.dates ?? [] },
    { title: '结构化事实', items: profile.facts ?? [] },
  ].filter((group) => group.items.length > 0)

  return (
    <div className={readerTw.profileCard}>
      {profile.typeLabel ? <div className={readerTw.profileType}>{profile.typeLabel}</div> : null}
      {profile.summary ? <p className={readerTw.profileSummary}>{profile.summary}</p> : null}
      <div className={readerTw.profileGrid}>
        {groups.map((group) => (
          <div key={group.title}>
            <div className={readerTw.sectionTitle}>{group.title}</div>
            {group.items.slice(0, 6).map((item) => (
              <div className={readerTw.profileRow} key={`${group.title}-${item.label}-${item.value}`}>
                <div className={readerTw.profileLabel}>{item.label}</div>
                <div className={readerTw.profileValue}>{item.value}</div>
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}

function CitationPanel({
  citations,
  activeCitation,
  onCitationSelect,
}: {
  citations: DocumentCitationTarget[]
  activeCitation?: DocumentCitationTarget
  onCitationSelect?: (citation: DocumentCitationTarget) => void
}) {
  if (citations.length === 0) {
    return <div className={readerTw.emptyCard}>回答引用会显示页码、片段和命中原因。</div>
  }

  return (
    <div className={readerTw.citationList}>
      {citations.map((citation) => (
        <button
          className={cx(
            readerTw.citationButton,
            activeCitation?.id === citation.id && readerTw.citationButtonActive,
          )}
          key={citation.id}
          onClick={() => onCitationSelect?.(citation)}
          type="button"
        >
          <div className={readerTw.citationTop}>
            <span className={readerTw.citationPage}>{citationLabel(citation)}</span>
            <span className={readerTw.citationType}>
              {sourceTypeLabel(citation.sourceType)}
            </span>
          </div>
          <div className={readerTw.citationSnippet}>{citation.snippet}</div>
          {citation.sourceReason ? (
            <div className={readerTw.citationReason}>{citation.sourceReason}</div>
          ) : null}
        </button>
      ))}
    </div>
  )
}

function PdfPane({
  source,
  activeCitation,
  zoom,
}: {
  source: DocumentReaderSource
  activeCitation?: DocumentCitationTarget
  zoom: number
}) {
  const page = activeCitation?.page ?? 1
  const [pdfState, setPdfState] = useState({ sourceURL: '', objectURL: '', error: '' })
  const objectURL = pdfState.sourceURL === source.url ? pdfState.objectURL : ''
  const error = pdfState.sourceURL === source.url ? pdfState.error : ''
  const urlWithHint = objectURL ? `${objectURL}#page=${page}&zoom=${zoom}` : ''

  useEffect(() => {
    if (!source.url) return undefined
    const controller = new AbortController()
    fetch(source.url, { credentials: 'include', signal: controller.signal })
      .then((response) => {
        if (!response.ok) throw new Error(`HTTP ${response.status}`)
        return response.blob()
      })
      .then((blob) => {
        const pdfBlob = blob.type === 'application/pdf' ? blob : blob.slice(0, blob.size, 'application/pdf')
        const nextObjectURL = URL.createObjectURL(pdfBlob)
        setPdfState((previous) => {
          if (previous.objectURL) URL.revokeObjectURL(previous.objectURL)
          return { sourceURL: source.url ?? '', objectURL: nextObjectURL, error: '' }
        })
      })
      .catch((fetchError: unknown) => {
        if (fetchError instanceof DOMException && fetchError.name === 'AbortError') return
        setPdfState((previous) => {
          if (previous.objectURL) URL.revokeObjectURL(previous.objectURL)
          return {
            sourceURL: source.url ?? '',
            objectURL: '',
            error: 'PDF 原文加载失败，请稍后重试。',
          }
        })
      })
    return () => controller.abort()
  }, [source.url])

  useEffect(
    () => () => {
      if (pdfState.objectURL) URL.revokeObjectURL(pdfState.objectURL)
    },
    [pdfState.objectURL],
  )

  if (!source.url) {
    return (
      <div className={readerTw.readerScroll}>
        <div className={readerTw.page}>
          <div className={readerTw.pageMuted}>PDF 文件地址未提供。接入后端文件流后将在这里渲染页面。</div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className={readerTw.readerScroll}>
        <div className={readerTw.page}>
          <div className={readerTw.pageMuted}>
            {error}
            <br />
            <a className="font-medium text-[#2458b8]" href={source.url} rel="noreferrer" target="_blank">
              打开 PDF
            </a>
          </div>
        </div>
      </div>
    )
  }

  if (!objectURL) {
    return (
      <div className={readerTw.readerScroll}>
        <div className={readerTw.page}>
          <div className={readerTw.pageMuted}>正在加载 PDF...</div>
        </div>
      </div>
    )
  }

  return (
    <object
      aria-label={source.title}
      className={readerTw.pdfObject}
      data={urlWithHint}
      type="application/pdf"
    >
      <div className={readerTw.readerScroll}>
        <div className={readerTw.page}>
          <div className={readerTw.pageMuted}>
            当前浏览器无法内嵌预览 PDF。
            <br />
            <a className="font-medium text-[#2458b8]" href={source.url} rel="noreferrer" target="_blank">
              打开 PDF
            </a>
          </div>
        </div>
      </div>
    </object>
  )
}

function TextPane({
  text,
  activeCitation,
}: {
  text: string
  activeCitation?: DocumentCitationTarget
}) {
  const lines = useMemo(() => text.split(/\r?\n/), [text])
  const activeLineRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    activeLineRef.current?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  }, [activeCitation?.id])

  return (
    <div className={readerTw.readerScroll}>
      <div className={readerTw.page}>
        {lines.map((line, index) => {
          const lineNumber = index + 1
          const isActive =
            Boolean(activeCitation?.lineStart) &&
            lineNumber >= Number(activeCitation?.lineStart) &&
            lineNumber <= Number(activeCitation?.lineEnd ?? activeCitation?.lineStart)
          return (
            <div
              className={cx(readerTw.lineRow, isActive && readerTw.lineActive)}
              key={`${lineNumber}-${line.slice(0, 12)}`}
              ref={isActive ? activeLineRef : undefined}
            >
              <span className={readerTw.lineNo}>{lineNumber}</span>
              <span className={readerTw.lineText}>
                <HighlightedText text={line || ' '} highlightText={activeCitation?.highlightText} />
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function EpubPane({
  chapters,
  activeCitation,
  onLocationChange,
}: {
  chapters: EpubChapter[]
  activeCitation?: DocumentCitationTarget
  onLocationChange?: (location: DocumentReaderLocation) => void
}) {
  const initialChapterID = activeCitation?.chapterId ?? chapters[0]?.id ?? ''
  const [chapterID, setChapterID] = useState(initialChapterID)
  const selectedChapterID = activeCitation?.chapterId ?? chapterID
  const activeChapter = chapters.find((chapter) => chapter.id === selectedChapterID) ?? chapters[0]

  if (!activeChapter) {
    return (
      <div className={readerTw.readerScroll}>
        <div className={readerTw.page}>
          <div className={readerTw.pageMuted}>EPUB 章节内容未提供。接入 EPUB 解析器后将在这里渲染章节。</div>
        </div>
      </div>
    )
  }

  const chapterLines = activeChapter.content.split(/\r?\n/)

  return (
    <div className={readerTw.epubLayout}>
      <nav aria-label="EPUB 目录" className={readerTw.epubToc}>
        {chapters.map((chapter) => (
          <button
            className={cx(
              readerTw.epubTocButton,
              activeChapter.id === chapter.id && readerTw.epubTocButtonActive,
            )}
            key={chapter.id}
            onClick={() => {
              setChapterID(chapter.id)
              onLocationChange?.({ format: 'epub', chapterId: chapter.id })
            }}
            type="button"
          >
            {chapter.title}
          </button>
        ))}
      </nav>
      <div className={readerTw.readerScroll}>
        <article className={readerTw.page}>
          <h2 className="mb-5 text-[20px] leading-7 font-semibold text-[#111]">{activeChapter.title}</h2>
          <div className={readerTw.pageText}>
            {chapterLines.map((line, index) => (
              <p className="mb-3" key={`${activeChapter.id}-${index}`}>
                <HighlightedText text={line || ' '} highlightText={activeCitation?.highlightText} />
              </p>
            ))}
          </div>
        </article>
      </div>
    </div>
  )
}

function ReaderPane({
  source,
  activeCitation,
  zoom,
  onLocationChange,
}: {
  source: DocumentReaderSource
  activeCitation?: DocumentCitationTarget
  zoom: number
  onLocationChange?: (location: DocumentReaderLocation) => void
}) {
  if (source.format === 'pdf') {
    return <PdfPane activeCitation={activeCitation} source={source} zoom={zoom} />
  }
  if (source.format === 'epub') {
    return (
      <EpubPane
        activeCitation={activeCitation}
        chapters={source.chapters ?? []}
        onLocationChange={onLocationChange}
      />
    )
  }
  return <TextPane activeCitation={activeCitation} text={source.text ?? ''} />
}

export function DocumentReader({
  source,
  profile,
  citations = [],
  activeCitationId,
  chatSlot,
  className,
  onCitationSelect,
  onLocationChange,
}: DocumentReaderProps) {
  const [zoom, setZoom] = useState(100)
  const activeCitation = getActiveCitation(citations, activeCitationId)

  return (
    <section className={cx(readerTw.shell, className)} data-reader-format={source.format}>
      <div className={readerTw.leftPane}>
        <header className={readerTw.toolbar}>
          <div className={readerTw.toolbarLeft}>
            <div className={readerTw.formatTabs} aria-label="文档格式">
              {(['pdf', 'epub', 'txt'] as const).map((format) => (
                <span
                  className={cx(
                    readerTw.formatTab,
                    source.format === format && readerTw.formatTabActive,
                  )}
                  key={format}
                >
                  {formatLabel(format)}
                </span>
              ))}
            </div>
            <div className={readerTw.toolbarTitleWrap}>
              <div className={readerTw.toolbarTitle}>{source.title}</div>
              <div className={readerTw.toolbarSub}>
                {activeCitation ? citationLabel(activeCitation) : 'ready'}
              </div>
            </div>
          </div>
          <div className={readerTw.toolbarRight}>
            <button
              aria-label="缩小"
              className={readerTw.iconBtn}
              onClick={() => setZoom((value) => Math.max(50, value - 10))}
              type="button"
            >
              <i aria-hidden="true" className="fa fa-search-minus" />
            </button>
            <span className="min-w-[44px] text-center text-[12px] text-[#555]">{zoom}%</span>
            <button
              aria-label="放大"
              className={readerTw.iconBtn}
              onClick={() => setZoom((value) => Math.min(200, value + 10))}
              type="button"
            >
              <i aria-hidden="true" className="fa fa-search-plus" />
            </button>
          </div>
        </header>
        <div className={readerTw.readerCanvas}>
          <ReaderPane
            activeCitation={activeCitation}
            onLocationChange={onLocationChange}
            source={source}
            zoom={zoom}
          />
        </div>
      </div>
      <aside className={readerTw.rightPane}>
        <header className={readerTw.panelHeader}>
          <div className={readerTw.panelTitle}>Document Expert</div>
          <div className={readerTw.panelSub}>文档资料、引用证据和 AI 对话共用同一阅读上下文。</div>
        </header>
        <div className={readerTw.panelScroll}>
          {chatSlot ?? (
            <>
              <ProfilePanel profile={profile} />
              <div className={readerTw.sectionTitle}>引用证据</div>
              <CitationPanel
                activeCitation={activeCitation}
                citations={citations}
                onCitationSelect={onCitationSelect}
              />
            </>
          )}
        </div>
      </aside>
    </section>
  )
}
