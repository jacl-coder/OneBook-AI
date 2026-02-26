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

export function generateSmartThreadTitle(input: string): string {
  const raw = input.replace(/\s+/g, ' ').trim()
  if (!raw) return '新对话'

  const firstSentence = raw.split(/[\n。！？!?]/)[0]?.trim() || raw
  const cleaned = firstSentence
    .replace(/^(请问一下|请问|麻烦你|麻烦|帮我|可以帮我|能帮我|我想问一下|我想问|我想了解一下|我想了解|请你|请)\s*/u, '')
    .replace(/^(关于|有关)\s*/u, '')
    .replace(/\s*(吗|呢)[？?]?$/u, '')
    .trim()

  if (!cleaned) return truncateThreadTitle(firstSentence)

  const headSegment = cleaned
    .split(/[，,:：]/)
    .map((part) => part.trim())
    .find((part) => part.length >= 4)
  if (headSegment && headSegment.length <= 24) return headSegment

  return truncateThreadTitle(cleaned)
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
const CHAT_THREADS_STORAGE_KEY = 'onebook:chat:threads'
const CHAT_ACTIVE_THREAD_STORAGE_KEY = 'onebook:chat:active-thread'

function userScopedStorageKey(baseKey: string, userID: string): string {
  return `${baseKey}:${userID}`
}

function readStorageItem(baseKey: string, userID: string): string | null {
  if (typeof window === 'undefined') return null
  const scopedKey = userScopedStorageKey(baseKey, userID)
  return (
    window.localStorage.getItem(scopedKey) ??
    window.sessionStorage.getItem(scopedKey) ??
    window.localStorage.getItem(baseKey) ??
    window.sessionStorage.getItem(baseKey)
  )
}

function writeStorageItem(baseKey: string, userID: string, value: string) {
  if (typeof window === 'undefined') return
  const scopedKey = userScopedStorageKey(baseKey, userID)
  window.localStorage.setItem(scopedKey, value)
  window.sessionStorage.setItem(scopedKey, value)
}

function parseThreadSource(input: unknown): ThreadSource | null {
  if (!input || typeof input !== 'object') return null
  const record = input as Partial<ThreadSource>
  if (typeof record.label !== 'string') return null
  if (typeof record.location !== 'string') return null
  if (record.snippet != null && typeof record.snippet !== 'string') return null
  return {
    label: record.label,
    location: record.location,
    snippet: record.snippet,
  }
}

function parseThreadMessage(input: unknown): ThreadMessage | null {
  if (!input || typeof input !== 'object') return null
  const record = input as Partial<ThreadMessage>
  if (typeof record.id !== 'string' || !record.id.trim()) return null
  if (record.role !== 'user' && record.role !== 'assistant') return null
  if (typeof record.text !== 'string') return null
  if (typeof record.createdAt !== 'number' || Number.isNaN(record.createdAt)) return null

  const sources = Array.isArray(record.sources)
    ? record.sources
        .map((item) => parseThreadSource(item))
        .filter((item): item is ThreadSource => item !== null)
    : undefined

  return {
    id: record.id,
    role: record.role,
    text: record.text,
    createdAt: record.createdAt,
    sources,
  }
}

function parseThread(input: unknown): ChatThread | null {
  if (!input || typeof input !== 'object') return null
  const record = input as Partial<ChatThread>
  if (typeof record.id !== 'string' || !record.id.trim()) return null
  if (typeof record.title !== 'string') return null
  if (typeof record.updatedAt !== 'number' || Number.isNaN(record.updatedAt)) return null
  if (!Array.isArray(record.messages)) return null
  if (typeof record.lastUserPrompt !== 'string') return null
  if (typeof record.errorText !== 'string') return null

  const messages = record.messages
    .map((item) => parseThreadMessage(item))
    .filter((item): item is ThreadMessage => item !== null)

  const status: ThreadStatus =
    record.status === 'error' ? 'error' : 'idle'

  return {
    id: record.id,
    title: record.title,
    updatedAt: record.updatedAt,
    messages,
    status,
    lastUserPrompt: record.lastUserPrompt,
    errorText: record.errorText,
  }
}

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

export function readStoredChatThreads(userID: string): ChatThread[] {
  if (!userID.trim()) return []
  const raw = readStorageItem(CHAT_THREADS_STORAGE_KEY, userID)
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed
      .map((item) => parseThread(item))
      .filter((item): item is ChatThread => item !== null)
      .sort((a, b) => b.updatedAt - a.updatedAt)
  } catch {
    return []
  }
}

export function writeStoredChatThreads(userID: string, threads: ChatThread[]) {
  if (!userID.trim()) return
  writeStorageItem(CHAT_THREADS_STORAGE_KEY, userID, JSON.stringify(threads))
}

export function readStoredActiveThreadID(userID: string): string {
  if (!userID.trim()) return ''
  const raw = readStorageItem(CHAT_ACTIVE_THREAD_STORAGE_KEY, userID)
  if (!raw) return ''
  return raw.trim()
}

export function writeStoredActiveThreadID(userID: string, threadID: string) {
  if (!userID.trim()) return
  writeStorageItem(CHAT_ACTIVE_THREAD_STORAGE_KEY, userID, threadID)
}

export function readStoredChatThreadSummaries(userID?: string): ChatThreadSummary[] {
  if (typeof window === 'undefined') return []
  const scopedRaw = userID?.trim()
    ? readStorageItem(CHAT_THREAD_SUMMARIES_STORAGE_KEY, userID.trim())
    : window.localStorage.getItem(CHAT_THREAD_SUMMARIES_STORAGE_KEY) ??
      window.sessionStorage.getItem(CHAT_THREAD_SUMMARIES_STORAGE_KEY)
  const raw = scopedRaw
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

export function writeStoredChatThreadSummaries(summaries: ChatThreadSummary[], userID?: string) {
  if (typeof window === 'undefined') return
  const payload = JSON.stringify(summaries)
  if (userID?.trim()) {
    writeStorageItem(CHAT_THREAD_SUMMARIES_STORAGE_KEY, userID.trim(), payload)
    return
  }
  window.localStorage.setItem(CHAT_THREAD_SUMMARIES_STORAGE_KEY, payload)
  window.sessionStorage.setItem(CHAT_THREAD_SUMMARIES_STORAGE_KEY, payload)
}
