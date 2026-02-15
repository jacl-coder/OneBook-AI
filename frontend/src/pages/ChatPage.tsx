import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import type { SubmitEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import onebookLogoMark from '@/assets/brand/onebook-logo-mark.svg'
import googleLogo from '@/assets/brand/provider/google-logo.svg'
import appleLogo from '@/assets/brand/provider/apple-logo.svg'
import microsoftLogo from '@/assets/brand/provider/microsoft-logo.svg'
import phoneIconSvg from '@/assets/icons/phone.svg'
import { getApiErrorMessage, logout } from '@/features/auth/api/auth'
import { useSessionStore } from '@/features/auth/store/session'
import { http } from '@/shared/lib/http/client'

const quickActions = [
  { key: 'attach', symbolId: 'chat-attach', label: '附件' },
] as const

const headingPool = [
  '先读你的书，再来提问。',
  '你的资料里，今天想问什么？',
  '先看原文，再看答案。',
  '从书中检索，让回答可追溯。',
  '围绕你的书库，开始一次对话。',
  '先定位证据，再生成结论。',
]

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/
type AuthModalMode = 'login' | 'register'
const CHAT_ICON_SPRITE_URL = '/icons/chat/sprite.svg'

type ThreadSource = {
  label: string
  location: string
  snippet?: string
}

type ThreadMessage = {
  id: string
  role: 'user' | 'assistant'
  text: string
  createdAt: number
  sources?: ThreadSource[]
}

type ThreadStatus = 'idle' | 'sending' | 'error'

type ChatThread = {
  id: string
  title: string
  updatedAt: number
  messages: ThreadMessage[]
  status: ThreadStatus
  lastUserPrompt: string
  errorText: string
}

type BookSummary = {
  id: string
  title: string
  status: 'queued' | 'processing' | 'ready' | 'failed'
}

type ListBooksResponse = {
  items: BookSummary[]
  count: number
}

type ChatAnswer = {
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

function nowTimestamp(): number {
  return Date.now()
}

function createThreadId() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `thread-${nowTimestamp()}-${Math.random().toString(16).slice(2, 8)}`
}

function createMessageId() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `msg-${nowTimestamp()}-${Math.random().toString(16).slice(2, 8)}`
}

function truncateThreadTitle(input: string): string {
  const text = input.trim()
  if (!text) return '新对话'
  return text.length <= 24 ? text : `${text.slice(0, 24)}…`
}

function getThreadPreview(thread: ChatThread): string {
  const last = thread.messages[thread.messages.length - 1]
  if (!last) return '开始一段新对话'
  return last.text.length <= 34 ? last.text : `${last.text.slice(0, 34)}…`
}

function getRelativeTimeLabel(timestamp: number): string {
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

function createEmptyThread(): ChatThread {
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

function updateThreadAndMoveTop(
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

export function ChatPage() {
  const navigate = useNavigate()
  const sessionUser = useSessionStore((state) => state.user)
  const clearSession = useSessionStore((state) => state.clearSession)

  const guestEditorRef = useRef<HTMLDivElement>(null)
  const authEditorRef = useRef<HTMLDivElement>(null)
  const authInputRef = useRef<HTMLInputElement>(null)
  const threadSearchInputRef = useRef<HTMLInputElement>(null)
  const uploadGuestInputRef = useRef<HTMLInputElement>(null)
  const uploadAuthInputRef = useRef<HTMLInputElement>(null)
  const pendingAskIdRef = useRef(0)

  const [guestPrompt, setGuestPrompt] = useState('')
  const [authPrompt, setAuthPrompt] = useState('')
  const [heading] = useState(
    () => headingPool[Math.floor(Math.random() * headingPool.length)],
  )
  const [isAuthOpen, setIsAuthOpen] = useState(false)
  const [authMode, setAuthMode] = useState<AuthModalMode>('login')
  const [authEmail, setAuthEmail] = useState('')
  const [isAuthFocused, setIsAuthFocused] = useState(false)
  const [isAuthSubmitting, setIsAuthSubmitting] = useState(false)
  const [authErrorText, setAuthErrorText] = useState('')

  const [threads, setThreads] = useState<ChatThread[]>(() => [createEmptyThread()])
  const [activeThreadId, setActiveThreadId] = useState<string>('')
  const [threadSearch, setThreadSearch] = useState('')
  const [isSidebarOpen, setIsSidebarOpen] = useState(false)
  const [isDesktopSidebarCollapsed, setIsDesktopSidebarCollapsed] = useState(false)
  const [isHistoryExpanded, setIsHistoryExpanded] = useState(true)
  const [books, setBooks] = useState<BookSummary[]>([])
  const [selectedBookId, setSelectedBookId] = useState('')
  const [bookListErrorText, setBookListErrorText] = useState('')

  const authEmailId = useId()
  const authErrorId = useId()

  const hasGuestPrompt = guestPrompt.trim().length > 0
  const hasAuthPrompt = authPrompt.trim().length > 0
  const isAuthInvalid = authErrorText.length > 0

  const activeThread = useMemo(
    () => threads.find((thread) => thread.id === activeThreadId) ?? null,
    [threads, activeThreadId],
  )

  const filteredThreads = useMemo(() => {
    const keyword = threadSearch.trim().toLowerCase()
    if (!keyword) return threads
    return threads.filter((thread) => {
      const title = thread.title.toLowerCase()
      const preview = getThreadPreview(thread).toLowerCase()
      return title.includes(keyword) || preview.includes(keyword)
    })
  }, [threads, threadSearch])

  const activeThreadIsSending = activeThread?.status === 'sending'
  const activeThreadHasError = activeThread?.status === 'error'
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

  const getShortcutAriaLabel = useCallback((key: string) => {
    if (key === '⌘') return '命令'
    if (key === 'Ctrl') return 'Control'
    if (key === 'Shift' || key === '⇧') return 'Shift'
    return undefined
  }, [])

  const selectedBook = useMemo(
    () => books.find((book) => book.id === selectedBookId) ?? null,
    [books, selectedBookId],
  )
  const hasReadyBooks = books.length > 0
  const canAsk = hasReadyBooks && selectedBookId !== ''

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

  const loadReadyBooks = useCallback(async () => {
    try {
      const { data } = await http.get<ListBooksResponse>('/api/books')
      const readyBooks = data.items.filter((book) => book.status === 'ready')
      setBooks(readyBooks)
      if (!readyBooks.length) {
        setSelectedBookId('')
        setBookListErrorText('你的书库暂无可提问书籍，请先上传并等待处理完成。')
        return
      }
      setBookListErrorText('')
      setSelectedBookId((current) =>
        current && readyBooks.some((book) => book.id === current) ? current : readyBooks[0].id,
      )
    } catch (error) {
      setBooks([])
      setSelectedBookId('')
      setBookListErrorText(getApiErrorMessage(error, '加载书籍失败，请稍后重试。'))
    }
  }, [])

  useEffect(() => {
    if (activeThreadId) return
    if (!threads.length) return
    setActiveThreadId(threads[0].id)
  }, [activeThreadId, threads])

  useEffect(() => {
    if (!sessionUser) return
    void loadReadyBooks()
  }, [sessionUser, loadReadyBooks])

  function closeAuthModal() {
    setIsAuthOpen(false)
    setAuthMode('login')
    setAuthEmail('')
    setIsAuthFocused(false)
    setIsAuthSubmitting(false)
    setAuthErrorText('')
  }

  useEffect(() => {
    if (!isAuthOpen) return
    const originalOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = originalOverflow
    }
  }, [isAuthOpen])

  useEffect(() => {
    if (!isAuthOpen) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closeAuthModal()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [isAuthOpen])

  useEffect(() => {
    if (!isSidebarOpen) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setIsSidebarOpen(false)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [isSidebarOpen])

  const syncGuestPrompt = () => {
    const value = guestEditorRef.current?.innerText ?? ''
    setGuestPrompt(value.replace(/\u00a0/g, ' '))
  }

  const syncAuthPrompt = () => {
    const value = authEditorRef.current?.innerText ?? ''
    setAuthPrompt(value.replace(/\u00a0/g, ' '))
  }

  const resetGuestComposer = () => {
    if (guestEditorRef.current) guestEditorRef.current.textContent = ''
    setGuestPrompt('')
  }

  const resetAuthComposer = () => {
    if (authEditorRef.current) authEditorRef.current.textContent = ''
    setAuthPrompt('')
  }

  const handleGuestComposerSubmit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!hasGuestPrompt) return
    resetGuestComposer()
  }

  const askAssistant = async (threadId: string, prompt: string, appendUserMessage: boolean) => {
    if (!threadId || activeThreadIsSending) return
    const trimmedPrompt = prompt.trim()
    if (!trimmedPrompt) return
    if (!selectedBookId) {
      setThreads((previous) =>
        updateThreadAndMoveTop(previous, threadId, (thread) => ({
          ...thread,
          status: 'error',
          errorText: bookListErrorText || '请先在书库选择一本已处理完成的书。',
        })),
      )
      return
    }
    const now = nowTimestamp()
    const requestId = pendingAskIdRef.current + 1
    pendingAskIdRef.current = requestId

    setThreads((previous) =>
      updateThreadAndMoveTop(previous, threadId, (thread) => ({
        ...thread,
        title: thread.title === '新对话' ? truncateThreadTitle(trimmedPrompt) : thread.title,
        updatedAt: now,
        status: 'sending',
        errorText: '',
        lastUserPrompt: trimmedPrompt,
        messages: appendUserMessage
          ? [
              ...thread.messages,
              {
                id: createMessageId(),
                role: 'user',
                text: trimmedPrompt,
                createdAt: now,
              },
            ]
          : thread.messages,
      })),
    )
    if (appendUserMessage) {
      resetAuthComposer()
    }
    try {
      const { data } = await http.post<ChatAnswer>('/api/chats', {
        bookId: selectedBookId,
        question: trimmedPrompt,
      })
      if (requestId !== pendingAskIdRef.current) return
      setThreads((previous) =>
        updateThreadAndMoveTop(previous, threadId, (thread) => ({
          ...thread,
          updatedAt: nowTimestamp(),
          status: 'idle',
          errorText: '',
          messages: [
            ...thread.messages,
            {
              id: createMessageId(),
              role: 'assistant',
              text: data.answer,
              createdAt: Date.parse(data.createdAt) || nowTimestamp(),
              sources: data.sources.map((source) => ({
                label: source.label,
                location: source.location,
                snippet: source.snippet,
              })),
            },
          ],
        })),
      )
    } catch (error) {
      if (requestId !== pendingAskIdRef.current) return
      setThreads((previous) =>
        updateThreadAndMoveTop(previous, threadId, (thread) => ({
          ...thread,
          updatedAt: nowTimestamp(),
          status: 'error',
          errorText: getApiErrorMessage(error, '请求失败，请稍后重试。'),
        })),
      )
    }
  }

  const submitAuthPrompt = () => {
    if (!hasAuthPrompt || !activeThreadId || activeThreadIsSending) return
    void askAssistant(activeThreadId, authPrompt, true)
  }

  const handleAuthComposerSubmit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!canAsk) return
    submitAuthPrompt()
  }

  const handleRetryAssistant = () => {
    if (!activeThread || !activeThread.lastUserPrompt || activeThread.status !== 'error') return

    void askAssistant(activeThread.id, activeThread.lastUserPrompt, false)
  }

  const openAuthModal = (mode: AuthModalMode = 'login') => {
    setAuthMode(mode)
    setIsAuthOpen(true)
  }

  const validateAuthEmail = (value: string) => {
    const text = value.trim()
    if (!text) return '电子邮件地址为必填项。'
    if (!EMAIL_PATTERN.test(text)) return '电子邮件地址无效。'
    return ''
  }

  const handleAuthSubmit = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (isAuthSubmitting) return

    const error = validateAuthEmail(authEmail)
    if (error) {
      setAuthErrorText(error)
      authInputRef.current?.focus()
      return
    }

    setAuthErrorText('')
    setIsAuthSubmitting(true)
    await new Promise((resolve) => setTimeout(resolve, 280))
    closeAuthModal()
    const targetPath = authMode === 'register' ? '/create-account/password' : '/log-in/password'
    navigate(`${targetPath}?email=${encodeURIComponent(authEmail.trim())}`)
  }

  const handleCreateConversation = useCallback(() => {
    const newThread = createEmptyThread()
    setThreads((previous) => [newThread, ...previous])
    setActiveThreadId(newThread.id)
    setIsSidebarOpen(false)
    requestAnimationFrame(() => authEditorRef.current?.focus())
  }, [])

  const handleThreadSelect = (threadId: string) => {
    setActiveThreadId(threadId)
    setIsSidebarOpen(false)
    setTimeout(() => authEditorRef.current?.focus(), 0)
  }

  const handleLogout = async () => {
    try {
      await logout()
    } catch {
      // Client state must still be cleared even when network request fails.
    } finally {
      clearSession()
    }
  }

  const handleComposerActionClick = (isGuest: boolean) => {
    if (isGuest) {
      uploadGuestInputRef.current?.click()
    } else {
      uploadAuthInputRef.current?.click()
    }
  }

  useEffect(() => {
    if (!sessionUser) return
    const onKeyDown = (event: KeyboardEvent) => {
      const key = event.key.toLowerCase()
      const modifierPressed = isApplePlatform ? event.metaKey : event.ctrlKey
      if (!modifierPressed || event.altKey) return

      const target = event.target as HTMLElement | null
      const tagName = target?.tagName ?? ''
      const isTypingTarget =
        !!target &&
        (target.isContentEditable ||
          target.closest('[contenteditable="true"]') !== null ||
          tagName === 'INPUT' ||
          tagName === 'TEXTAREA' ||
          tagName === 'SELECT')

      if (key === 'k' && !event.shiftKey) {
        event.preventDefault()
        threadSearchInputRef.current?.focus()
        threadSearchInputRef.current?.select()
        return
      }

      if (key === 'o' && event.shiftKey) {
        if (isTypingTarget) return
        event.preventDefault()
        handleCreateConversation()
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [sessionUser, isApplePlatform, handleCreateConversation])

  if (sessionUser) {
    const avatarLetter = sessionUser.email.slice(0, 1).toUpperCase()
    const isSidebarExpanded = isMobileSidebarViewport() ? isSidebarOpen : !isDesktopSidebarCollapsed

    return (
      <div className={`chatgpt-app-shell ${isDesktopSidebarCollapsed ? 'chatgpt-app-shell-sidebar-collapsed' : ''}`}>
        <button
          type="button"
          className={`chatgpt-app-sidebar-backdrop ${isSidebarOpen ? 'is-open' : ''}`}
          aria-hidden={!isSidebarOpen}
          tabIndex={-1}
          onClick={() => setIsSidebarOpen(false)}
        />

        <aside
          id="stage-slideover-sidebar"
          className={`chatgpt-app-sidebar ${isSidebarOpen ? 'is-open' : ''}`}
          aria-label="会话侧边栏"
        >
          <div className="chatgpt-app-sidebar-top">
            <div className="chatgpt-app-sidebar-brand">
              <Link to="/chat" className="chatgpt-app-logo-link" aria-label="OneBook AI">
                <img src={onebookLogoMark} alt="" aria-hidden="true" />
              </Link>
              <button
                type="button"
                className="chatgpt-app-collapse-btn"
                aria-expanded={isSidebarExpanded}
                aria-controls="stage-slideover-sidebar"
                aria-label="关闭边栏"
                data-testid="close-sidebar-button"
                data-state={isSidebarExpanded ? 'open' : 'closed'}
                onClick={handleCloseSidebar}
              >
                <svg
                  viewBox="0 0 20 20"
                  xmlns="http://www.w3.org/2000/svg"
                  aria-hidden="true"
                  data-rtl-flip=""
                  className="chatgpt-app-collapse-icon-desktop"
                >
                  <use href={`${CHAT_ICON_SPRITE_URL}#chat-sidebar-close-desktop`} fill="currentColor" />
                </svg>
                <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="chatgpt-app-collapse-icon-mobile">
                  <use href={`${CHAT_ICON_SPRITE_URL}#chat-sidebar-close-mobile`} fill="currentColor" />
                </svg>
              </button>
            </div>
          </div>

          <div className="chatgpt-app-sidebar-content" role="listbox" aria-label="会话列表">
            <aside className="chatgpt-app-sidebar-first-section">
              <div className="chatgpt-app-sidebar-quick-actions">
                <button type="button" className="chatgpt-app-menu-item" onClick={handleCreateConversation}>
                  <span className="chatgpt-app-menu-item-leading">
                    <span className="chatgpt-app-menu-item-icon" aria-hidden="true">
                      <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
                        <use href={`${CHAT_ICON_SPRITE_URL}#chat-new-chat`} fill="currentColor" />
                      </svg>
                    </span>
                    <span className="chatgpt-app-menu-item-content">
                      <span className="chatgpt-app-menu-item-title">新聊天</span>
                    </span>
                  </span>
                  <span className="chatgpt-app-menu-item-trailing" aria-hidden="true">
                    <span className="chatgpt-app-menu-item-kbd-row">
                      {newChatShortcutKeys.map((key) => (
                        <kbd key={`new-chat-shortcut-${key}`} aria-label={getShortcutAriaLabel(key)}>
                          <span className="chatgpt-app-menu-item-kbd">{key}</span>
                        </kbd>
                      ))}
                    </span>
                  </span>
                </button>

                <label className="chatgpt-app-menu-item chatgpt-app-menu-item-search" htmlFor="chat-thread-search">
                  <span className="chatgpt-app-menu-item-leading">
                    <span className="chatgpt-app-menu-item-icon" aria-hidden="true">
                      <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
                        <use href={`${CHAT_ICON_SPRITE_URL}#chat-search`} fill="currentColor" />
                      </svg>
                    </span>
                    <span className="chatgpt-app-menu-item-content">
                      <input
                        id="chat-thread-search"
                        ref={threadSearchInputRef}
                        className="chatgpt-app-search-input"
                        type="search"
                        placeholder="搜索聊天"
                        value={threadSearch}
                        onChange={(event) => setThreadSearch(event.target.value)}
                      />
                    </span>
                  </span>
                  <span className="chatgpt-app-menu-item-trailing" aria-hidden="true">
                    <span className="chatgpt-app-menu-item-kbd-row">
                      {searchShortcutKeys.map((key) => (
                        <kbd key={`search-shortcut-${key}`} aria-label={getShortcutAriaLabel(key)}>
                          <span className="chatgpt-app-menu-item-kbd">{key}</span>
                        </kbd>
                      ))}
                    </span>
                  </span>
                </label>
              </div>
            </aside>
            <div className="chatgpt-app-sidebar-section-spacer" aria-hidden="true" />

            <div className={`chatgpt-app-sidebar-expando-section ${isHistoryExpanded ? 'is-expanded' : ''}`}>
              <button
                type="button"
                aria-expanded={isHistoryExpanded}
                className="chatgpt-app-sidebar-expando-btn"
                onClick={() => setIsHistoryExpanded((prev) => !prev)}
              >
                <h2 className="chatgpt-app-menu-label" data-no-spacing="true">你的聊天</h2>
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="16"
                  height="16"
                  aria-hidden="true"
                  className={`chatgpt-app-sidebar-expando-icon ${isHistoryExpanded ? '' : 'is-collapsed'}`}
                >
                  <use href={`${CHAT_ICON_SPRITE_URL}#chat-chevron-down`} fill="currentColor" />
                </svg>
              </button>
            </div>

            {isHistoryExpanded ? (
              <>
                <div className="chatgpt-app-thread-list">
                  {filteredThreads.map((thread) => {
                    const isActive = thread.id === activeThreadId
                    return (
                      <button
                        key={thread.id}
                        type="button"
                        className={`chatgpt-app-thread-item ${isActive ? 'is-active' : ''}`}
                        onClick={() => handleThreadSelect(thread.id)}
                      >
                        <span className="chatgpt-app-thread-title">{thread.title}</span>
                        <span className="chatgpt-app-thread-more" aria-hidden="true">
                          <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
                            <path d="M5 10a1.2 1.2 0 1 0 0-2.4A1.2 1.2 0 0 0 5 10Zm5 0a1.2 1.2 0 1 0 0-2.4A1.2 1.2 0 0 0 10 10Zm5 0a1.2 1.2 0 1 0 0-2.4A1.2 1.2 0 0 0 15 10Z" fill="currentColor" />
                          </svg>
                        </span>
                      </button>
                    )
                  })}
                </div>

                {!filteredThreads.length ? (
                  <div className="chatgpt-app-thread-empty">没有匹配的会话</div>
                ) : null}
              </>
            ) : null}
          </div>

          <div className="chatgpt-app-sidebar-foot">
            <div className="chatgpt-app-account-card">
              <span className="chatgpt-app-avatar" aria-hidden="true">
                {avatarLetter}
              </span>
              <div className="chatgpt-app-account-meta">
                <span className="chatgpt-app-account-email">{sessionUser.email}</span>
                <span className="chatgpt-app-account-role">{sessionUser.role === 'admin' ? '管理员' : '普通用户'}</span>
              </div>
              <button type="button" className="chatgpt-app-logout-btn" onClick={() => void handleLogout()}>
                退出
              </button>
            </div>
          </div>
        </aside>

        <main className="chatgpt-app-main">
          <header className="chatgpt-app-main-header">
            <div className="chatgpt-app-main-left">
              <button
                type="button"
                className="chatgpt-app-menu-btn"
                aria-label="打开会话侧栏"
                onClick={handleOpenSidebar}
              >
                <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                  <path d="M4 5.5H16" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
                  <path d="M4 10H16" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
                  <path d="M4 14.5H12" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
                </svg>
              </button>
              <button type="button" className="chatgpt-app-model-btn" aria-label="模型选择器，当前模型为 OneBook AI">
                <span>OneBook AI</span>
                <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="chatgpt-model-icon">
                  <use href={`${CHAT_ICON_SPRITE_URL}#chat-chevron-down`} fill="currentColor" />
                </svg>
              </button>
            </div>

            <div className="chatgpt-app-main-actions">
              <button type="button" className="chatgpt-app-head-action">临时</button>
              <button type="button" className="chatgpt-app-head-action">分享</button>
            </div>
          </header>

          <section className="chatgpt-app-thread-stage" aria-label="会话内容">
            <div className="chatgpt-app-thread-scroll">
              <div className="chatgpt-app-thread-max">
                {activeThread ? (
                  <>
                    {bookListErrorText ? (
                      <div className="chatgpt-app-thread-error" role="status" aria-live="polite">
                        <span>{bookListErrorText}</span>
                        <button type="button" onClick={() => void loadReadyBooks()}>
                          重新加载
                        </button>
                      </div>
                    ) : null}

                    <div className="chatgpt-app-date-divider">
                      <span>{getRelativeTimeLabel(activeThread.updatedAt)}</span>
                    </div>

                    {activeThread.messages.map((message) => (
                      <article
                        key={message.id}
                        className={`chatgpt-app-message ${message.role === 'user' ? 'is-user' : 'is-assistant'}`}
                      >
                        {message.role === 'assistant' ? <div className="chatgpt-app-assistant-avatar">AI</div> : null}
                        <div className="chatgpt-app-message-body">
                          <div className="chatgpt-app-message-bubble">{message.text}</div>
                          {message.role === 'assistant' && message.sources?.length ? (
                            <div className="chatgpt-app-source-row">
                              {message.sources.map((source) => (
                                <button key={`${source.label}-${source.location}`} type="button" className="chatgpt-app-source-pill">
                                  <span>{source.label}</span>
                                  <span>{source.location}</span>
                                  {source.snippet ? <span>{source.snippet}</span> : null}
                                </button>
                              ))}
                            </div>
                          ) : null}
                        </div>
                      </article>
                    ))}

                    {activeThreadIsSending ? (
                      <article className="chatgpt-app-message is-assistant">
                        <div className="chatgpt-app-assistant-avatar">AI</div>
                        <div className="chatgpt-app-message-body">
                          <div className="chatgpt-app-typing">
                            <span />
                            <span />
                            <span />
                          </div>
                        </div>
                      </article>
                    ) : null}

                    {activeThreadHasError ? (
                      <div className="chatgpt-app-thread-error" role="status" aria-live="polite">
                        <span>{activeThread.errorText}</span>
                        <button type="button" onClick={handleRetryAssistant}>
                          重试
                        </button>
                      </div>
                    ) : null}
                  </>
                ) : (
                  <div className="chatgpt-app-empty-thread">请选择会话或新建对话。</div>
                )}
              </div>
            </div>
          </section>

          <section className="chatgpt-app-composer-wrap" aria-label="输入区">
            <div className="chatgpt-thread-content">
              <div className="chatgpt-thread-max chatgpt-thread-max-auth">
                <div className="chatgpt-composer-container">
                  <form className="chatgpt-composer-form" data-expanded="" data-type="unified-composer" onSubmit={handleAuthComposerSubmit}>
                    <div className="chatgpt-hidden-upload">
                      <input
                        ref={uploadAuthInputRef}
                        accept="image/jpeg,.jpg,.jpeg,image/webp,.webp,image/gif,.gif,image/png,.png"
                        multiple
                        type="file"
                        tabIndex={-1}
                      />
                    </div>

                    <div className="chatgpt-composer-surface" data-composer-surface="true">
                      <div className="chatgpt-composer-primary">
                        <div className="chatgpt-prosemirror-parent">
                          <div
                            ref={authEditorRef}
                            contentEditable
                            suppressContentEditableWarning
                            translate="no"
                            role="textbox"
                            id="prompt-textarea-auth"
                            className="chatgpt-prosemirror"
                            data-empty={hasAuthPrompt ? 'false' : 'true'}
                            aria-label="输入你的问题"
                            onInput={syncAuthPrompt}
                            onKeyDown={(event) => {
                              if (event.key === 'Enter' && !event.shiftKey) {
                                event.preventDefault()
                                if (hasAuthPrompt && canAsk) submitAuthPrompt()
                              }
                            }}
                          />
                          {!hasAuthPrompt ? (
                            <div className="chatgpt-prosemirror-placeholder" aria-hidden="true">
                              {canAsk
                                ? `基于《${selectedBook?.title ?? '所选书籍'}》提问，回答将附带可追溯引用`
                                : '请先在书库上传并等待书籍处理完成'}
                            </div>
                          ) : null}
                        </div>
                      </div>

                      <div className="chatgpt-composer-footer-actions" data-testid="composer-footer-actions">
                        <div className="chatgpt-composer-footer-row">
                          {quickActions.map((item) => (
                            <button
                              key={item.label}
                              type="button"
                              className="chatgpt-action-btn chatgpt-action-btn-attach"
                              onClick={() => handleComposerActionClick(false)}
                            >
                              <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="chatgpt-action-icon">
                                <use href={`${CHAT_ICON_SPRITE_URL}#${item.symbolId}`} fill="currentColor" />
                              </svg>
                              <span>{item.label}</span>
                            </button>
                          ))}
                        </div>
                      </div>

                      <div className="chatgpt-composer-trailing">
                        {!hasAuthPrompt ? (
                          <button
                            type="button"
                            className="chatgpt-voice-btn"
                            aria-label="启动语音功能"
                            data-testid="composer-speech-button"
                          >
                            <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="chatgpt-voice-icon">
                              <use href={`${CHAT_ICON_SPRITE_URL}#chat-voice`} fill="currentColor" />
                            </svg>
                            <span>语音</span>
                          </button>
                        ) : (
                          <button
                            type="submit"
                            className="chatgpt-send-btn"
                            aria-label="发送提示"
                            data-testid="send-button"
                            disabled={activeThreadIsSending || !canAsk}
                          >
                            <svg
                              viewBox="0 0 20 20"
                              xmlns="http://www.w3.org/2000/svg"
                              aria-hidden="true"
                              className="chatgpt-send-icon"
                            >
                              <use href={`${CHAT_ICON_SPRITE_URL}#chat-send`} fill="currentColor" />
                            </svg>
                          </button>
                        )}
                      </div>
                    </div>
                  </form>
                </div>
              </div>
            </div>
          </section>
        </main>
      </div>
    )
  }

  return (
    <div className="chatgpt-entry-page">
      <a className="chatgpt-skip-link" href="#onebook-main">
        跳至内容
      </a>

      <header className="chatgpt-entry-header" role="banner">
        <div className="chatgpt-entry-left">
          <Link to="/chat" className="chatgpt-entry-logo-link" aria-label="OneBook AI">
            <img src={onebookLogoMark} alt="" aria-hidden="true" />
          </Link>
          <button type="button" className="chatgpt-model-btn" aria-label="模型选择器，当前模型为 OneBook AI">
            <span>OneBook AI</span>
            <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="chatgpt-model-icon">
              <use href={`${CHAT_ICON_SPRITE_URL}#chat-chevron-down`} fill="currentColor" />
            </svg>
          </button>
        </div>

        <div className="chatgpt-entry-right">
          <button type="button" className="chatgpt-top-btn chatgpt-top-btn-dark" onClick={() => openAuthModal('login')}>
            <div className="chatgpt-top-btn-label">登录</div>
          </button>
          <button type="button" className="chatgpt-top-btn chatgpt-top-btn-light" onClick={() => openAuthModal('register')}>
            <div className="chatgpt-top-btn-label">免费注册</div>
          </button>
          <button type="button" className="chatgpt-profile-btn" aria-label="打开“个人资料”菜单">
            <div className="chatgpt-profile-btn-inner">
              <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="chatgpt-profile-icon">
                <use href={`${CHAT_ICON_SPRITE_URL}#chat-profile`} fill="currentColor" />
              </svg>
            </div>
          </button>
        </div>
      </header>

      <main id="onebook-main" className="chatgpt-entry-main">
        <div className="chatgpt-entry-center">
          <div className="chatgpt-entry-hero">
            <div className="chatgpt-entry-heading-row">
              <div className="chatgpt-entry-heading-inline">
                <h1>
                  <div className="chatgpt-entry-title">{heading}</div>
                </h1>
              </div>
            </div>
          </div>

          <div className="chatgpt-thread-bottom" id="thread-bottom">
            <div className="chatgpt-thread-content">
              <div className="chatgpt-thread-max">
                <div className="chatgpt-composer-container">
                  <form
                    className="chatgpt-composer-form"
                    data-expanded=""
                    data-type="unified-composer"
                    onSubmit={handleGuestComposerSubmit}
                  >
                    <div className="chatgpt-hidden-upload">
                      <input
                        ref={uploadGuestInputRef}
                        accept="image/jpeg,.jpg,.jpeg,image/webp,.webp,image/gif,.gif,image/png,.png"
                        multiple
                        type="file"
                        tabIndex={-1}
                      />
                    </div>

                    <div className="chatgpt-composer-surface" data-composer-surface="true">
                      <div className="chatgpt-composer-primary">
                        <div className="chatgpt-prosemirror-parent">
                          <div
                            ref={guestEditorRef}
                            contentEditable
                            suppressContentEditableWarning
                            translate="no"
                            role="textbox"
                            id="prompt-textarea"
                            className="chatgpt-prosemirror"
                            data-empty={hasGuestPrompt ? 'false' : 'true'}
                            aria-label="输入你的问题"
                            onInput={syncGuestPrompt}
                            onKeyDown={(event) => {
                              if (event.key === 'Enter' && !event.shiftKey) {
                                event.preventDefault()
                                if (hasGuestPrompt) resetGuestComposer()
                              }
                            }}
                          />
                          {!hasGuestPrompt ? (
                            <div className="chatgpt-prosemirror-placeholder" aria-hidden="true">
                              有问题，尽管问
                            </div>
                          ) : null}
                        </div>
                      </div>

                      <div className="chatgpt-composer-footer-actions" data-testid="composer-footer-actions">
                        <div className="chatgpt-composer-footer-row">
                          {quickActions.map((item) => (
                            <button
                              key={item.label}
                              type="button"
                              className="chatgpt-action-btn chatgpt-action-btn-attach"
                              onClick={() => handleComposerActionClick(true)}
                            >
                              <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="chatgpt-action-icon">
                                <use href={`${CHAT_ICON_SPRITE_URL}#${item.symbolId}`} fill="currentColor" />
                              </svg>
                              <span>{item.label}</span>
                            </button>
                          ))}
                        </div>
                      </div>

                      <div className="chatgpt-composer-trailing">
                        {!hasGuestPrompt ? (
                          <button
                            type="button"
                            className="chatgpt-voice-btn"
                            aria-label="启动语音功能"
                            data-testid="composer-speech-button"
                          >
                            <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="chatgpt-voice-icon">
                              <use href={`${CHAT_ICON_SPRITE_URL}#chat-voice`} fill="currentColor" />
                            </svg>
                            <span>语音</span>
                          </button>
                        ) : (
                          <button
                            type="submit"
                            className="chatgpt-send-btn"
                            aria-label="发送提示"
                            data-testid="send-button"
                          >
                            <svg
                              viewBox="0 0 20 20"
                              xmlns="http://www.w3.org/2000/svg"
                              aria-hidden="true"
                              className="chatgpt-send-icon"
                            >
                              <use href={`${CHAT_ICON_SPRITE_URL}#chat-send`} fill="currentColor" />
                            </svg>
                          </button>
                        )}
                      </div>
                    </div>
                  </form>
                </div>

                <input className="chatgpt-sr-only" type="file" tabIndex={-1} aria-hidden="true" id="upload-photos" accept="image/*" multiple />
                <input
                  className="chatgpt-sr-only"
                  type="file"
                  tabIndex={-1}
                  aria-hidden="true"
                  id="upload-camera"
                  accept="image/*"
                  capture="environment"
                  multiple
                />
              </div>
            </div>
          </div>

          <div className="chatgpt-entry-legal-wrap">
            <p className="chatgpt-entry-legal">
              向 OneBook AI 发送消息即表示，你同意我们的
              <a href="#" onClick={(e) => e.preventDefault()}>
                条款
              </a>
              并已阅读我们的
              <a href="#" onClick={(e) => e.preventDefault()}>
                隐私政策
              </a>
              。查看
              <a href="#" onClick={(e) => e.preventDefault()}>
                Cookie 首选项
              </a>
              。
            </p>
          </div>
        </div>
      </main>

      {isAuthOpen ? (
        <div id="modal-no-auth-login" className="chatgpt-auth-modal-root">
          <div className="chatgpt-auth-modal-backdrop" onClick={closeAuthModal} aria-hidden="true" />
          <div className="chatgpt-auth-modal-grid">
            <div
              role="dialog"
              aria-modal="true"
              aria-labelledby="chatgpt-auth-dialog-title"
              className="chatgpt-auth-dialog"
              onClick={(event) => event.stopPropagation()}
            >
              <header className="chatgpt-auth-dialog-header">
                <div className="chatgpt-auth-dialog-header-title" />
                <button type="button" className="chatgpt-auth-close-btn" aria-label="关闭" onClick={closeAuthModal}>
                  <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="chatgpt-auth-close-icon">
                    <use href={`${CHAT_ICON_SPRITE_URL}#chat-close`} fill="currentColor" />
                  </svg>
                </button>
              </header>

              <div className="chatgpt-auth-dialog-body">
                <div className="chatgpt-auth-dialog-content" data-testid="login-form">
                  <h2 id="chatgpt-auth-dialog-title">登录或注册</h2>
                  <p className="chatgpt-auth-dialog-subtitle">
                    你将可以基于个人书库提问，并获得可追溯来源的回答。
                  </p>

                  <form className="chatgpt-auth-form" onSubmit={handleAuthSubmit} noValidate>
                    <div className="chatgpt-auth-social-group" role="group" aria-label="选择登录选项">
                      <button type="button" className="chatgpt-auth-social-btn" onClick={() => navigate('/log-in')}>
                        <span className="chatgpt-auth-social-btn-inner">
                          <span className="chatgpt-auth-social-icon">
                            <img src={googleLogo} alt="" aria-hidden="true" />
                          </span>
                          <span>继续使用 Google 登录</span>
                        </span>
                      </button>
                      <button type="button" className="chatgpt-auth-social-btn" onClick={() => navigate('/log-in')}>
                        <span className="chatgpt-auth-social-btn-inner">
                          <span className="chatgpt-auth-social-icon">
                            <img src={appleLogo} alt="" aria-hidden="true" />
                          </span>
                          <span>继续使用 Apple 登录</span>
                        </span>
                      </button>
                      <button type="button" className="chatgpt-auth-social-btn" onClick={() => navigate('/log-in')}>
                        <span className="chatgpt-auth-social-btn-inner">
                          <span className="chatgpt-auth-social-icon">
                            <img src={microsoftLogo} alt="" aria-hidden="true" />
                          </span>
                          <span>继续使用 Microsoft 登录</span>
                        </span>
                      </button>
                      <button type="button" className="chatgpt-auth-social-btn" onClick={() => navigate('/log-in')}>
                        <span className="chatgpt-auth-social-btn-inner">
                          <span className="chatgpt-auth-social-icon">
                            <img src={phoneIconSvg} alt="" aria-hidden="true" />
                          </span>
                          <span>继续使用手机登录</span>
                        </span>
                      </button>
                    </div>

                    <div className="chatgpt-auth-divider">
                      <div className="chatgpt-auth-divider-line" />
                      <div className="chatgpt-auth-divider-name">或</div>
                      <div className="chatgpt-auth-divider-line" />
                    </div>

                    <div className="chatgpt-auth-input-block">
                      <div
                        className={`chatgpt-auth-input-frame ${isAuthFocused ? 'is-focused' : ''} ${
                          isAuthInvalid ? 'is-invalid' : ''
                        }`}
                      >
                        <input
                          ref={authInputRef}
                          className="chatgpt-auth-input"
                          id={authEmailId}
                          name="email"
                          type="email"
                          autoComplete="email"
                          placeholder="电子邮件地址"
                          value={authEmail}
                          aria-label="电子邮件地址"
                          aria-describedby={isAuthInvalid ? authErrorId : undefined}
                          aria-invalid={isAuthInvalid || undefined}
                          disabled={isAuthSubmitting}
                          onFocus={() => setIsAuthFocused(true)}
                          onBlur={() => setIsAuthFocused(false)}
                          onChange={(e) => {
                            setAuthEmail(e.target.value)
                            if (isAuthInvalid) setAuthErrorText('')
                          }}
                        />
                      </div>
                      {isAuthInvalid ? (
                        <div className="chatgpt-auth-error" id={authErrorId}>
                          <span className="chatgpt-auth-error-icon">
                            <svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                              <use href={`${CHAT_ICON_SPRITE_URL}#chat-error-circle`} />
                            </svg>
                          </span>
                          <span>{authErrorText}</span>
                        </div>
                      ) : null}
                    </div>

                    <button type="submit" className="chatgpt-auth-continue-btn" disabled={isAuthSubmitting}>
                      继续
                    </button>

                  </form>
                </div>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
