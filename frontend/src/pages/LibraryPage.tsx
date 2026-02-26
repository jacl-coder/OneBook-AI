import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { getApiErrorMessage, logout } from '@/features/auth/api/auth'
import { useSessionStore } from '@/features/auth/store/session'
import {
  CHAT_ICON_SPRITE_URL,
  conversationQueryKeys,
  fetchConversationSummaries,
  useChatSidebarState,
} from '@/pages/chat/shared'
import {
  deleteBook,
  getBookDownloadURL,
  isBookPending,
  libraryQueryKeys,
  listBooks,
  type LibraryBook,
  uploadBook,
} from '@/features/library/api/library'
import { ChatSidebar, type SidebarThreadItem } from '@/pages/chat/ChatSidebar'

const cx = (...values: Array<string | false | null | undefined>) =>
  values.filter(Boolean).join(' ')

const uiSansStyle = {
  fontFamily:
    "ui-sans-serif, -apple-system, system-ui, 'Segoe UI', Helvetica, 'Apple Color Emoji', Arial, sans-serif, 'Segoe UI Emoji', 'Segoe UI Symbol'",
} as const

const libraryTw = {
  shortcutLabel:
    'pointer-events-none inline-flex shrink-0 items-center gap-0 text-[14px] leading-5 text-[#737373] opacity-0 transition-opacity duration-120 group-hover/menu:opacity-100 group-focus-visible/menu:opacity-100 group-focus-within/menu:opacity-100 max-[767px]:hidden',
  shortcutRow: 'inline-flex whitespace-pre',
  shortcutKeyWrap:
    'm-0 inline-flex border-0 bg-transparent p-0 font-inherit text-inherit',
  shortcutKey:
    'inline-flex min-w-[1em] items-center justify-center font-sans text-[14px] leading-5',
  menuMainIconWrap: 'inline-flex min-w-0 flex-1 items-center gap-[6px]',
  menuMainIcon:
    'inline-flex h-5 w-5 shrink-0 items-center justify-center text-[#525252]',
  menuMainTextWrap: 'inline-flex min-w-0 flex-1 items-center gap-[10px]',
  iconBlockH5W5: 'block h-5 w-5',
  iconBlockH14W14: 'block h-[14px] w-[14px]',
  roleMuted: 'text-[11px] leading-[14px] text-[#6f6f6f]',
  sidebarHeader: 'block px-4 pt-2',
  sidebarHeaderRow: 'flex items-center justify-between',
  sidebarHomeLink:
    'm-0 inline-flex h-9 w-9 flex-[0_0_36px] items-center justify-center rounded-[10px] p-0 leading-none',
  sidebarHomeLogo: 'block h-9 w-9',
  sidebarCloseButton:
    'mr-[-8px] inline-flex h-9 w-9 shrink-0 cursor-[w-resize] items-center justify-center rounded-[8px] border-0 bg-transparent text-[#737373] hover:bg-[#ececec] focus-visible:bg-[#ececec] max-[767px]:h-10 max-[767px]:w-10',
  sidebarCloseDesktopIcon: 'block h-5 w-5 max-[767px]:hidden',
  sidebarCloseMobileIcon: 'hidden h-5 w-5 max-[767px]:block',
  sidebarScrollArea: 'grid min-h-0 content-start gap-0 overflow-auto p-0',
  sidebarListAside: 'pt-0',
  sidebarMenuList: 'mt-[14px] grid gap-0 p-0',
  sidebarNavButton:
    'group/menu flex min-h-9 w-full cursor-pointer items-center justify-between gap-2 rounded-[10px] border-0 bg-transparent px-4 py-[6px] text-left tracking-[0] text-[#0d0d0d] hover:bg-[#ececec] focus:outline-none max-[767px]:min-h-10',
  sidebarNavButtonActive: 'bg-[#e7e7e7]',
  sidebarNavText:
    'block min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[14px] leading-5 font-normal',
  sidebarSearchLabel:
    'group/menu flex min-h-9 w-full cursor-text items-center justify-between gap-2 rounded-[10px] border-0 bg-transparent px-4 py-[6px] text-left tracking-[0] text-[#0d0d0d] hover:bg-[#ececec] focus-within:outline-none max-[767px]:min-h-10',
  sidebarSearchInput:
    'w-full min-w-0 border-0 bg-transparent p-0 text-[14px] leading-5 text-[#0d0d0d] outline-0 placeholder:text-[#0d0d0d] placeholder:opacity-100',
  sidebarMenuSpacer: 'pb-[14px]',
  sidebarHistoryToggle:
    'group/expando-btn inline-flex w-full cursor-pointer items-center justify-start gap-[2px] rounded-none border-0 bg-transparent px-4 py-[6px] text-left text-[#737373] hover:text-[#5f5f5f]',
  sidebarHistoryTitle: 'm-0 text-[14px] leading-4 font-medium tracking-[0]',
  sidebarThreadList: 'mb-2 grid gap-0 px-2',
  sidebarThreadTitle:
    'block overflow-hidden text-ellipsis whitespace-nowrap text-[14px] leading-[34px] font-normal',
  sidebarAccountPanel: 'grid gap-[6px] p-2',
  sidebarAccountCard:
    'mt-[6px] grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2 rounded-[12px] border border-[rgba(0,0,0,0.08)] bg-white p-2',
  sidebarAvatar:
    'inline-flex h-7 w-7 items-center justify-center rounded-[9999px] bg-[#0d0d0d] text-[12px] font-semibold text-white',
  sidebarAccountMeta: 'grid min-w-0',
  sidebarAccountEmail:
    'overflow-hidden text-ellipsis whitespace-nowrap text-[12px] leading-4',
  sidebarLogoutButton:
    'cursor-pointer rounded-[8px] border-0 bg-transparent px-[6px] py-1 text-[12px] text-[#474747] hover:bg-[#efefef] hover:text-[#0d0d0d]',
  topBar:
    'flex h-[52px] items-center justify-between bg-white px-[14px] py-2 max-[767px]:px-[10px]',
  topBarLeft: 'flex items-center gap-2',
  topBarRight: 'inline-flex items-center gap-2',
  topBarTitle:
    'inline-flex h-[34px] items-center rounded-[8px] px-2 text-[16px] leading-6 font-medium text-[#0d0d0d]',
  sidebarOpenButton:
    'hidden h-[34px] w-[34px] cursor-pointer items-center justify-center rounded-[8px] border-0 bg-transparent text-[#0d0d0d] hover:bg-[#f1f1f1] max-[767px]:inline-flex',
  sidebarOpenIcon: 'block h-[18px] w-[18px]',
  uploadButton:
    'inline-flex h-8 cursor-pointer items-center gap-1 rounded-[9999px] border border-[rgba(0,0,0,0.12)] bg-white px-[12px] text-[13px] font-medium text-[#2f2f2f] hover:bg-[#f6f6f6] disabled:cursor-not-allowed disabled:opacity-50',
  pageBody: 'overflow-auto bg-[#ffffff] px-4 pb-6 pt-4 max-[767px]:px-3',
  sectionWrap: 'mx-auto w-full max-w-[920px]',
  notice:
    'mb-4 rounded-[12px] border border-[#f4b0b4] bg-[#fff5f6] px-3 py-[10px] text-[13px] text-[#9f1820]',
  empty:
    'grid min-h-[300px] place-items-center rounded-[14px] border border-dashed border-[rgba(0,0,0,0.16)] bg-[#fcfcfc] p-6 text-center',
  emptyTitle: 'text-[18px] font-medium text-[#0d0d0d]',
  emptyDesc: 'mt-2 text-[13px] text-[#686868]',
  table:
    'overflow-hidden rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white',
  tableHeader:
    'grid grid-cols-[minmax(0,2fr)_110px_90px_220px] items-center gap-3 border-b border-[rgba(0,0,0,0.06)] bg-[#fafafa] px-4 py-3 text-[12px] font-medium text-[#6d6d6d] max-[980px]:hidden',
  row:
    'grid grid-cols-[minmax(0,2fr)_110px_90px_220px] items-center gap-3 border-b border-[rgba(0,0,0,0.06)] px-4 py-3 last:border-b-0 max-[980px]:grid-cols-1 max-[980px]:gap-2 max-[980px]:px-3',
  titleCell: 'min-w-0',
  titleText:
    'block overflow-hidden text-ellipsis whitespace-nowrap text-[14px] font-medium text-[#0d0d0d]',
  subText:
    'mt-1 overflow-hidden text-ellipsis whitespace-nowrap text-[12px] text-[#6f6f6f]',
  statusChip:
    'inline-flex w-fit items-center rounded-[9999px] px-2 py-[2px] text-[12px] font-medium',
  statusQueued: 'bg-[#fff7e8] text-[#9c6500]',
  statusProcessing: 'bg-[#edf5ff] text-[#1d5fbf]',
  statusReady: 'bg-[#e9f8ef] text-[#1e7a3e]',
  statusFailed: 'bg-[#fdecec] text-[#b42318]',
  cellLabel:
    'hidden text-[11px] font-medium uppercase tracking-[0.04em] text-[#7a7a7a] max-[980px]:block',
  actions: 'flex flex-wrap items-center gap-2',
  actionBtn:
    'inline-flex h-8 cursor-pointer items-center justify-center rounded-[9999px] border border-[rgba(0,0,0,0.12)] bg-white px-[10px] text-[12px] font-medium text-[#2f2f2f] hover:bg-[#f7f7f7] disabled:cursor-not-allowed disabled:opacity-50',
  actionDanger:
    'border-[rgba(180,35,24,0.2)] text-[#9f1820] hover:bg-[#fff3f3]',
} as const

function formatSize(sizeBytes: number): string {
  if (sizeBytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = sizeBytes
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  return `${value.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`
}

function statusLabel(status: LibraryBook['status']): string {
  switch (status) {
    case 'queued':
      return '排队中'
    case 'processing':
      return '处理中'
    case 'ready':
      return '可对话'
    case 'failed':
      return '处理失败'
    default:
      return status
  }
}

function statusClass(status: LibraryBook['status']): string {
  switch (status) {
    case 'queued':
      return libraryTw.statusQueued
    case 'processing':
      return libraryTw.statusProcessing
    case 'ready':
      return libraryTw.statusReady
    case 'failed':
      return libraryTw.statusFailed
    default:
      return libraryTw.statusQueued
  }
}

export function LibraryPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const sessionUser = useSessionStore((state) => state.user)
  const clearSession = useSessionStore((state) => state.clearSession)

  const chatSearchInputRef = useRef<HTMLInputElement>(null)
  const libraryNavButtonRef = useRef<HTMLButtonElement>(null)
  const uploadInputRef = useRef<HTMLInputElement>(null)

  const [actionErrorText, setActionErrorText] = useState('')
  const [deletingBookID, setDeletingBookID] = useState('')

  const {
    searchValue: chatSearch,
    setSearchValue: setChatSearch,
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
  } = useChatSidebarState()

  const booksQuery = useQuery({
    queryKey: libraryQueryKeys.books,
    queryFn: listBooks,
  })
  const conversationsQuery = useQuery({
    queryKey: conversationQueryKeys.list(sessionUser?.id ?? '', 100),
    queryFn: () => fetchConversationSummaries(100),
    enabled: Boolean(sessionUser),
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  })
  const { refetch: refetchBooks } = booksQuery

  const uploadMutation = useMutation({
    mutationFn: uploadBook,
    onSuccess: async () => {
      setActionErrorText('')
      await queryClient.invalidateQueries({ queryKey: libraryQueryKeys.books })
    },
    onError: (error) => {
      setActionErrorText(getApiErrorMessage(error, '上传失败，请稍后重试。'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteBook,
    onSuccess: async () => {
      setActionErrorText('')
      await queryClient.invalidateQueries({ queryKey: libraryQueryKeys.books })
    },
    onError: (error) => {
      setActionErrorText(getApiErrorMessage(error, '删除失败，请稍后重试。'))
    },
  })

  const books = useMemo(() => {
    const items = booksQuery.data?.items ?? []
    return [...items].sort((a, b) => {
      const left = Date.parse(a.updatedAt)
      const right = Date.parse(b.updatedAt)
      return Number.isNaN(right) || Number.isNaN(left) ? 0 : right - left
    })
  }, [booksQuery.data?.items])

  const hasPendingBooks = useMemo(
    () => books.some((book) => isBookPending(book.status)),
    [books],
  )
  const chatThreadSummaries = useMemo(
    () =>
      (conversationsQuery.data ?? []).map((item) => ({
        id: item.id,
        title: item.title || '新对话',
        updatedAt: Date.parse(item.lastMessageAt || item.updatedAt) || Date.now(),
        preview: '',
      })),
    [conversationsQuery.data],
  )
  const filteredChatThreads = useMemo(() => {
    const keyword = chatSearch.trim().toLowerCase()
    if (!keyword) return chatThreadSummaries
    return chatThreadSummaries.filter((thread) => {
      const title = thread.title.toLowerCase()
      const preview = thread.preview.toLowerCase()
      return title.includes(keyword) || preview.includes(keyword)
    })
  }, [chatThreadSummaries, chatSearch])
  const sidebarThreads = useMemo<SidebarThreadItem[]>(
    () =>
      filteredChatThreads.map((thread) => ({
        id: thread.id,
        title: thread.title,
        preview: thread.preview,
      })),
    [filteredChatThreads],
  )

  const handleLogout = useCallback(async () => {
    try {
      await logout()
    } catch {
      // Client state must still be cleared even when network request fails.
    } finally {
      clearSession()
      navigate('/chat')
    }
  }, [clearSession, navigate])

  const handleOpenLibrary = useCallback(() => {
    libraryNavButtonRef.current?.focus()
  }, [])

  const handleGoChat = useCallback(() => {
    navigate('/chat')
  }, [navigate])
  const handleSidebarThreadClick = useCallback(
    (threadID: string) => {
      navigate(`/chat/${threadID}`)
    },
    [navigate],
  )

  useEffect(() => {
    if (!hasPendingBooks) return undefined
    const timer = window.setInterval(() => {
      void refetchBooks()
    }, 2500)
    return () => window.clearInterval(timer)
  }, [hasPendingBooks, refetchBooks])

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
        chatSearchInputRef.current?.focus()
        chatSearchInputRef.current?.select()
        return
      }

      if (key === 'o' && event.shiftKey) {
        if (isTypingTarget) return
        event.preventDefault()
        handleGoChat()
        return
      }

      if (key === 'b' && !event.shiftKey) {
        if (isTypingTarget) return
        event.preventDefault()
        handleOpenLibrary()
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [sessionUser, isApplePlatform, handleGoChat, handleOpenLibrary])

  const handleUploadClick = useCallback(() => {
    uploadInputRef.current?.click()
  }, [])

  const handleUploadChange = useCallback(
    async (event: ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0]
      if (!file) return
      setActionErrorText('')
      try {
        await uploadMutation.mutateAsync(file)
      } finally {
        event.target.value = ''
      }
    },
    [uploadMutation],
  )

  const handleDownload = useCallback(async (book: LibraryBook) => {
    try {
      setActionErrorText('')
      const response = await getBookDownloadURL(book.id)
      const popup = window.open(response.url, '_blank', 'noopener,noreferrer')
      if (!popup) {
        window.location.assign(response.url)
      }
    } catch (error) {
      setActionErrorText(getApiErrorMessage(error, '下载链接获取失败，请稍后重试。'))
    }
  }, [])

  const handleDelete = useCallback(
    async (book: LibraryBook) => {
      const confirmed = window.confirm(`确认删除《${book.title}》吗？此操作不可撤销。`)
      if (!confirmed) return
      try {
        setDeletingBookID(book.id)
        await deleteMutation.mutateAsync(book.id)
      } finally {
        setDeletingBookID('')
      }
    },
    [deleteMutation],
  )

  if (!sessionUser) {
    return (
      <div className="grid min-h-screen place-items-center bg-white p-6 text-[#0d0d0d]" style={uiSansStyle}>
        <div className="w-full max-w-[420px] rounded-[14px] border border-[rgba(0,0,0,0.08)] p-6 text-center">
          <h1 className="text-[20px] font-medium">请先登录后进入书库管理</h1>
          <p className="mt-2 text-[14px] text-[#666]">登录后可上传、查看、下载与删除书籍。</p>
          <div className="mt-5 flex items-center justify-center gap-2">
            <Link to="/log-in" className={libraryTw.actionBtn}>
              去登录
            </Link>
            <Link to="/chat" className={libraryTw.actionBtn}>
              返回聊天
            </Link>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div
      className={cx(
        'grid min-h-screen bg-white text-[#0d0d0d] max-[767px]:grid-cols-[minmax(0,1fr)]',
        isDesktopSidebarCollapsed ? 'grid-cols-[minmax(0,1fr)]' : 'grid-cols-[260px_minmax(0,1fr)]',
      )}
      style={uiSansStyle}
    >
      <ChatSidebar
        isSidebarOpen={isSidebarOpen}
        isDesktopSidebarCollapsed={isDesktopSidebarCollapsed}
        isSidebarExpanded={isSidebarExpanded}
        onCloseSidebar={handleCloseSidebar}
        onMaskClick={() => setIsSidebarOpen(false)}
        searchInputId="library-chat-search"
        searchInputRef={chatSearchInputRef}
        searchValue={chatSearch}
        onSearchChange={setChatSearch}
        isHistoryExpanded={isHistoryExpanded}
        onToggleHistoryExpanded={toggleHistoryExpanded}
        threads={sidebarThreads}
        onThreadClick={handleSidebarThreadClick}
        onNewChatClick={handleGoChat}
        onLibraryClick={handleOpenLibrary}
        isLibraryActive
        newChatShortcutKeys={newChatShortcutKeys}
        searchShortcutKeys={searchShortcutKeys}
        libraryShortcutKeys={libraryShortcutKeys}
        getShortcutAriaLabel={getShortcutAriaLabel}
        accountEmail={sessionUser.email}
        accountRoleLabel={sessionUser.role === 'admin' ? '管理员' : '普通用户'}
        onLogout={() => void handleLogout()}
        libraryButtonRef={libraryNavButtonRef}
      />

      <main className="grid min-h-screen grid-rows-[52px_minmax(0,1fr)]">
        <header className={libraryTw.topBar}>
          <div className={libraryTw.topBarLeft}>
            <button
              type="button"
              className={cx(
                libraryTw.sidebarOpenButton,
                isDesktopSidebarCollapsed && 'md:inline-flex',
              )}
              aria-label="打开会话侧栏"
              onClick={handleOpenSidebar}
            >
              <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={libraryTw.sidebarOpenIcon}>
                <path d="M4 5.5H16" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
                <path d="M4 10H16" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
                <path d="M4 14.5H12" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
              </svg>
            </button>
            <h1 className={libraryTw.topBarTitle}>书库管理</h1>
          </div>

          <div className={libraryTw.topBarRight}>
            <input
              ref={uploadInputRef}
              className="hidden"
              type="file"
              tabIndex={-1}
              accept=".pdf,.epub,.txt"
              onChange={handleUploadChange}
            />
            <button
              type="button"
              className={libraryTw.uploadButton}
              onClick={handleUploadClick}
              disabled={uploadMutation.isPending}
            >
              <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" className={libraryTw.iconBlockH14W14} aria-hidden="true">
                <use href={`${CHAT_ICON_SPRITE_URL}#chat-attach`} fill="currentColor" />
              </svg>
              <span>{uploadMutation.isPending ? '上传中...' : '上传书籍'}</span>
            </button>
          </div>
        </header>

        <section className={libraryTw.pageBody}>
          <div className={libraryTw.sectionWrap}>
            {actionErrorText ? <div className={libraryTw.notice}>{actionErrorText}</div> : null}
            {booksQuery.isError ? (
              <div className={libraryTw.notice}>
                {getApiErrorMessage(booksQuery.error, '书籍列表加载失败，请稍后刷新重试。')}
              </div>
            ) : null}

            {booksQuery.isLoading ? (
              <div className={libraryTw.empty}>
                <div>
                  <p className={libraryTw.emptyTitle}>正在加载书库...</p>
                </div>
              </div>
            ) : books.length === 0 ? (
              <div className={libraryTw.empty}>
                <div>
                  <p className={libraryTw.emptyTitle}>你的书库还是空的</p>
                  <p className={libraryTw.emptyDesc}>支持上传 PDF / EPUB / TXT，上传后会自动解析并建立检索索引。</p>
                </div>
              </div>
            ) : (
              <div className={libraryTw.table}>
                <div className={libraryTw.tableHeader}>
                  <span>书籍</span>
                  <span>状态</span>
                  <span>大小</span>
                  <span>操作</span>
                </div>

                {books.map((book) => (
                  <div key={book.id} className={libraryTw.row}>
                    <div className={libraryTw.titleCell}>
                      <span className={libraryTw.cellLabel}>书籍</span>
                      <span className={libraryTw.titleText}>{book.title}</span>
                      <p className={libraryTw.subText}>{book.originalFilename}</p>
                      {book.status === 'failed' && book.errorMessage ? (
                        <p className="mt-1 text-[12px] text-[#b42318]">{book.errorMessage}</p>
                      ) : null}
                    </div>

                    <div>
                      <span className={libraryTw.cellLabel}>状态</span>
                      <span className={cx(libraryTw.statusChip, statusClass(book.status))}>
                        {statusLabel(book.status)}
                      </span>
                    </div>

                    <div>
                      <span className={libraryTw.cellLabel}>大小</span>
                      <span className="text-[13px] text-[#4f4f4f]">{formatSize(book.sizeBytes)}</span>
                    </div>

                    <div className={libraryTw.actions}>
                      <button
                        type="button"
                        className={libraryTw.actionBtn}
                        onClick={handleGoChat}
                        disabled={book.status !== 'ready'}
                      >
                        去对话
                      </button>
                      <button
                        type="button"
                        className={libraryTw.actionBtn}
                        onClick={() => void handleDownload(book)}
                      >
                        下载
                      </button>
                      <button
                        type="button"
                        className={cx(libraryTw.actionBtn, libraryTw.actionDanger)}
                        onClick={() => void handleDelete(book)}
                        disabled={deleteMutation.isPending && deletingBookID === book.id}
                      >
                        {deleteMutation.isPending && deletingBookID === book.id ? '删除中...' : '删除'}
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}

            {hasPendingBooks ? (
              <p className="mt-3 text-[12px] text-[#6f6f6f]">
                正在自动刷新处理状态...
                {booksQuery.isFetching ? '（刷新中）' : ''}
              </p>
            ) : null}
          </div>
        </section>
      </main>
    </div>
  )
}
