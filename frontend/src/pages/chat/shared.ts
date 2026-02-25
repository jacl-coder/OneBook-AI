export const quickActions = [
  { key: 'attach', symbolId: 'chat-attach', label: '附件' },
] as const

export const headingPool = [
  '先读你的书，再来提问。',
  '你的资料里，今天想问什么？',
  '先看原文，再看答案。',
  '从书中检索，让回答可追溯。',
  '围绕你的书库，开始一次对话。',
  '先定位证据，再生成结论。',
]

export const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/
export const CHAT_ICON_SPRITE_URL = '/icons/chat/sprite.svg'

export type AuthModalMode = 'login' | 'register'

export type ThreadSource = {
  label: string
  location: string
  snippet?: string
}

export type ThreadMessage = {
  id: string
  role: 'user' | 'assistant'
  text: string
  createdAt: number
  sources?: ThreadSource[]
}

export type ThreadStatus = 'idle' | 'sending' | 'error'

export type ChatThread = {
  id: string
  title: string
  updatedAt: number
  messages: ThreadMessage[]
  status: ThreadStatus
  lastUserPrompt: string
  errorText: string
}

export type ChatThreadSummary = {
  id: string
  title: string
  updatedAt: number
  preview: string
}

export type BookSummary = {
  id: string
  title: string
  status: 'queued' | 'processing' | 'ready' | 'failed'
}

export type ListBooksResponse = {
  items: BookSummary[]
  count: number
}

export type ChatAnswer = {
  bookId: string
  question: string
  answer: string
  sources: Array<{
    label: string
    location: string
    snippet: string
  }>
  createdAt: string
}

export function nowTimestamp(): number {
  return Date.now()
}

export function createThreadId() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `thread-${nowTimestamp()}-${Math.random().toString(16).slice(2, 8)}`
}

export function createMessageId() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `msg-${nowTimestamp()}-${Math.random().toString(16).slice(2, 8)}`
}

export function truncateThreadTitle(input: string): string {
  const text = input.trim()
  if (!text) return '新对话'
  return text.length <= 24 ? text : `${text.slice(0, 24)}…`
}

export function getThreadPreview(thread: ChatThread): string {
  const last = thread.messages[thread.messages.length - 1]
  if (!last) return '开始一段新对话'
  return last.text.length <= 34 ? last.text : `${last.text.slice(0, 34)}…`
}

export function getRelativeTimeLabel(timestamp: number): string {
  const diff = nowTimestamp() - timestamp
  if (diff < 60_000) return '刚刚'
  if (diff < 3_600_000) return `${Math.max(1, Math.floor(diff / 60_000))} 分钟前`
  if (diff < 86_400_000) return `${Math.max(1, Math.floor(diff / 3_600_000))} 小时前`
  if (diff < 172_800_000) return '昨天'
  const date = new Date(timestamp)
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${month}-${day}`
}

export function createEmptyThread(): ChatThread {
  return {
    id: createThreadId(),
    title: '新对话',
    updatedAt: nowTimestamp(),
    status: 'idle',
    lastUserPrompt: '',
    errorText: '',
    messages: [],
  }
}

export function updateThreadAndMoveTop(
  threads: ChatThread[],
  threadId: string,
  updater: (thread: ChatThread) => ChatThread,
): ChatThread[] {
  const index = threads.findIndex((thread) => thread.id === threadId)
  if (index < 0) return threads
  const updated = updater(threads[index])
  const rest = [...threads.slice(0, index), ...threads.slice(index + 1)]
  return [updated, ...rest]
}

const CHAT_THREAD_SUMMARIES_STORAGE_KEY = 'onebook:chat:thread-summaries'

export function summarizeThreads(threads: ChatThread[]): ChatThreadSummary[] {
  return threads
    .filter((thread) => thread.messages.length > 0)
    .map((thread) => ({
      id: thread.id,
      title: thread.title,
      updatedAt: thread.updatedAt,
      preview: getThreadPreview(thread),
    }))
}

export function readStoredChatThreadSummaries(): ChatThreadSummary[] {
  if (typeof window === 'undefined') return []
  const raw = window.sessionStorage.getItem(CHAT_THREAD_SUMMARIES_STORAGE_KEY)
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    const result: ChatThreadSummary[] = []
    for (const item of parsed) {
      if (!item || typeof item !== 'object') continue
      const record = item as Partial<ChatThreadSummary>
      if (typeof record.id !== 'string' || !record.id.trim()) continue
      if (typeof record.title !== 'string') continue
      if (typeof record.updatedAt !== 'number' || Number.isNaN(record.updatedAt)) continue
      if (typeof record.preview !== 'string') continue
      result.push({
        id: record.id,
        title: record.title,
        updatedAt: record.updatedAt,
        preview: record.preview,
      })
    }
    return result
  } catch {
    return []
  }
}

export function writeStoredChatThreadSummaries(summaries: ChatThreadSummary[]) {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(
    CHAT_THREAD_SUMMARIES_STORAGE_KEY,
    JSON.stringify(summaries),
  )
}
