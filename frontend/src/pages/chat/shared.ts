import { useCallback, useMemo, useState } from 'react'

import { http } from '@/shared/lib/http/client'

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
  isRemote?: boolean
  bookId?: string
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
  conversationId?: string
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

export function createEmptyThread(): ChatThread {
  return {
    id: createThreadId(),
    title: '新对话',
    updatedAt: nowTimestamp(),
    status: 'idle',
    lastUserPrompt: '',
    errorText: '',
    messages: [],
    isRemote: false,
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

export type ConversationSummary = {
  id: string
  userId: string
  bookId?: string
  title: string
  lastMessageAt?: string
  createdAt: string
  updatedAt: string
}

type ListConversationsResponse = {
  items: ConversationSummary[]
  count: number
}

export const conversationQueryKeys = {
  list: (userID: string, limit: number) => ['chat', 'conversations', userID, limit] as const,
}

export async function fetchConversationSummaries(limit: number): Promise<ConversationSummary[]> {
  const { data } = await http.get<ListConversationsResponse>(`/api/conversations?limit=${limit}`)
  return Array.isArray(data.items) ? data.items : []
}

type UseChatSidebarStateResult = {
  searchValue: string
  setSearchValue: (value: string) => void
  isSidebarOpen: boolean
  setIsSidebarOpen: (next: boolean) => void
  isDesktopSidebarCollapsed: boolean
  isHistoryExpanded: boolean
  toggleHistoryExpanded: () => void
  isSidebarExpanded: boolean
  isApplePlatform: boolean
  newChatShortcutKeys: string[]
  searchShortcutKeys: string[]
  libraryShortcutKeys: string[]
  getShortcutAriaLabel: (key: string) => string | undefined
  handleCloseSidebar: () => void
  handleOpenSidebar: () => void
}

export function useChatSidebarState(): UseChatSidebarStateResult {
  const [searchValue, setSearchValue] = useState('')
  const [isSidebarOpen, setIsSidebarOpen] = useState(false)
  const [isDesktopSidebarCollapsed, setIsDesktopSidebarCollapsed] = useState(false)
  const [isHistoryExpanded, setIsHistoryExpanded] = useState(true)

  const isApplePlatform = useMemo(() => {
    if (typeof navigator === 'undefined') return false
    const uaData = navigator as Navigator & { userAgentData?: { platform?: string } }
    const platform = uaData.userAgentData?.platform ?? navigator.platform ?? navigator.userAgent ?? ''
    return /mac|iphone|ipad|ipod/i.test(platform)
  }, [])

  const newChatShortcutKeys = useMemo(
    () => (isApplePlatform ? ['⇧', '⌘', 'O'] : ['Ctrl', 'Shift', 'O']),
    [isApplePlatform],
  )
  const searchShortcutKeys = useMemo(
    () => (isApplePlatform ? ['⌘', 'K'] : ['Ctrl', 'K']),
    [isApplePlatform],
  )
  const libraryShortcutKeys = useMemo(
    () => (isApplePlatform ? ['⌘', 'B'] : ['Ctrl', 'B']),
    [isApplePlatform],
  )

  const getShortcutAriaLabel = useCallback((key: string) => {
    if (key === '⌘') return '命令'
    if (key === 'Ctrl') return 'Control'
    if (key === 'Shift' || key === '⇧') return 'Shift'
    return undefined
  }, [])

  const isMobileSidebarViewport = useCallback(() => {
    if (typeof window === 'undefined') return false
    return window.matchMedia('(max-width: 767px)').matches
  }, [])

  const handleCloseSidebar = useCallback(() => {
    if (isMobileSidebarViewport()) {
      setIsSidebarOpen(false)
      return
    }
    setIsDesktopSidebarCollapsed(true)
  }, [isMobileSidebarViewport])

  const handleOpenSidebar = useCallback(() => {
    if (isMobileSidebarViewport()) {
      setIsSidebarOpen(true)
      return
    }
    setIsDesktopSidebarCollapsed(false)
  }, [isMobileSidebarViewport])

  const isSidebarExpanded = isMobileSidebarViewport() ? isSidebarOpen : !isDesktopSidebarCollapsed

  const toggleHistoryExpanded = useCallback(() => {
    setIsHistoryExpanded((prev) => !prev)
  }, [])

  return {
    searchValue,
    setSearchValue,
    isSidebarOpen,
    setIsSidebarOpen,
    isDesktopSidebarCollapsed,
    isHistoryExpanded,
    toggleHistoryExpanded,
    isSidebarExpanded,
    isApplePlatform,
    newChatShortcutKeys,
    searchShortcutKeys,
    libraryShortcutKeys,
    getShortcutAriaLabel,
    handleCloseSidebar,
    handleOpenSidebar,
  }
}
