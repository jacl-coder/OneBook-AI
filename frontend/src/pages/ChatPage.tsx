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
import {
  CHAT_ICON_SPRITE_URL,
  EMAIL_PATTERN,
  type AuthModalMode,
  type BookSummary,
  type ChatAnswer,
  type ChatThread,
  createEmptyThread,
  createMessageId,
  getRelativeTimeLabel,
  getThreadPreview,
  headingPool,
  nowTimestamp,
  quickActions,
  truncateThreadTitle,
  type ListBooksResponse,
  updateThreadAndMoveTop,
} from '@/pages/chat/shared'
import { http } from '@/shared/lib/http/client'

const cx = (...values: Array<string | false | null | undefined>) => values.filter(Boolean).join(' ')

const chatUiSansStyle = {
  fontFamily:
    "ui-sans-serif, -apple-system, system-ui, 'Segoe UI', Helvetica, 'Apple Color Emoji', Arial, sans-serif, 'Segoe UI Emoji', 'Segoe UI Symbol'",
} as const

const chatTw = {
  threadContent: 'mx-auto px-4 max-[767px]:px-[10px] max-[760px]:px-[6px]',
  threadMax: 'mx-auto mb-4 w-full max-w-[48rem] flex-1 max-[760px]:mb-3 max-[760px]:max-w-full',
  composerContainer: 'pointer-events-auto relative z-[1] flex h-full max-w-full flex-1 flex-col',
  composerForm: 'w-full',
  hidden: 'hidden',
  srOnly: 'sr-only',
  composerSurface:
    'grid grid-cols-[auto_1fr_auto] overflow-clip rounded-[28px] bg-white p-[10px] shadow-[0_4px_4px_0_rgba(0,0,0,0.04),0_0_1px_0_rgba(0,0,0,0.62)] transition-[box-shadow,background-color] duration-180 max-[760px]:rounded-[24px]',
  composerPrimary: 'col-span-3 -mt-[10px] flex min-h-[56px] items-center overflow-x-hidden px-[10px]',
  composerEditorWrap: 'relative flex-1 max-h-[max(30svh,5rem)] overflow-auto [scrollbar-width:thin]',
  composerEditor:
    'relative z-[1] min-h-[40px] max-h-[208px] whitespace-pre-wrap break-words px-0 py-4 text-[16px] leading-6 text-[#0d0d0d] outline-none',
  composerPlaceholder: 'pointer-events-none absolute left-0 top-4 z-0 select-none text-[#5d5d5d]',
  composerFooterActions:
    'col-start-2 row-start-2 -m-1 max-w-full overflow-x-auto p-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden max-[760px]:pr-0',
  composerFooterRow:
    'flex min-w-fit items-center gap-[6px] pr-[6px] max-[760px]:max-w-[calc(100%-116px)] max-[760px]:overflow-x-auto',
  composerAttachButton:
    'inline-flex h-9 min-w-[72px] cursor-pointer items-center justify-center gap-0 rounded-[99999px] border border-[rgba(13,13,13,0.1)] bg-transparent p-2 text-[13px] leading-[19.5px] font-medium text-[#5d5d5d] transition-colors duration-140 hover:bg-[#f5f5f5] max-[767px]:min-w-9',
  composerActionIcon: 'block h-[18px] w-[18px] opacity-[0.66]',
  composerActionText: 'whitespace-nowrap px-1 font-semibold max-[767px]:hidden',
  composerTrailing: 'col-start-3 row-start-2 flex items-center justify-end gap-2',
  composerVoiceButton:
    'inline-flex h-9 min-w-[70px] cursor-pointer items-center justify-center gap-0 rounded-[99999px] border-0 bg-[#ececec] p-2 text-[16px] leading-5 text-[#0d0d0d] transition-opacity duration-140 hover:opacity-[0.82] max-[767px]:min-w-9',
  composerVoiceIcon: 'block h-5 w-5',
  composerVoiceText: 'whitespace-nowrap px-1 text-[13px] leading-[19.5px] font-semibold max-[767px]:hidden',
  composerSendButton:
    'inline-flex h-9 w-9 items-center justify-center rounded-[99999px] border-0 bg-[#0d0d0d] text-[13px] text-white transition-opacity duration-140 disabled:cursor-default disabled:opacity-30',
  composerSendIcon: 'block h-5 w-5',
  shortcutLabel:
    'pointer-events-none inline-flex shrink-0 items-center gap-0 text-[14px] leading-5 text-[#737373] opacity-0 transition-opacity duration-120 group-hover/menu:opacity-100 group-focus-visible/menu:opacity-100 group-focus-within/menu:opacity-100 max-[767px]:hidden',
  shortcutRow: 'inline-flex whitespace-pre',
  shortcutKeyWrap: 'm-0 inline-flex border-0 bg-transparent p-0 font-inherit text-inherit',
  shortcutKey: 'inline-flex min-w-[1em] items-center justify-center font-sans text-[14px] leading-5',
  menuMainIconWrap: 'inline-flex min-w-0 flex-1 items-center gap-[6px]',
  menuMainIcon: 'inline-flex h-5 w-5 shrink-0 items-center justify-center text-[#525252]',
  menuMainTextWrap: 'inline-flex min-w-0 flex-1 items-center gap-[10px]',
  iconBlockH5W5: 'block h-5 w-5',
  iconBlockH14W14: 'block h-[14px] w-[14px]',
  roleMuted: 'text-[11px] leading-[14px] text-[#6f6f6f]',
  topActionPill:
    'h-8 cursor-pointer rounded-[9999px] border border-[rgba(0,0,0,0.12)] bg-white px-[10px] text-[12px] font-medium text-[#2f2f2f] hover:bg-[#f6f6f6] max-[767px]:hidden',
  alertBox:
    'flex items-center justify-between gap-[10px] rounded-[12px] border border-[#f4b0b4] bg-[#fff5f6] px-3 py-[10px] text-[13px] text-[#9f1820]',
  alertAction: 'cursor-pointer rounded-[9999px] border-0 bg-[#9f1820] px-[10px] py-1 text-[12px] text-white',
  assistantBadge:
    'mt-[2px] inline-flex h-[26px] w-[26px] items-center justify-center rounded-[9999px] bg-[#0d0d0d] text-[11px] font-semibold text-white max-[767px]:hidden',
  messageGrid: 'grid w-full gap-2',
  heroShell:
    'relative flex min-h-[calc(42svh-52px)] basis-auto shrink-0 flex-col justify-end max-[760px]:min-h-auto max-[760px]:justify-start',
  heroCenter: 'flex justify-center',
  heroInner: 'mb-10 text-center max-[760px]:mb-7 max-[760px]:mt-7',
  heroTitle:
    'm-0 inline-flex min-h-[42px] text-center text-[28px] leading-[34px] font-normal tracking-[0.38px] text-[#0d0d0d] max-[760px]:text-[24px] max-[760px]:leading-[30px]',
  heroHeading: 'block h-[34px] px-1 leading-[34px]',
  headerActionRow: 'flex items-center gap-0',
  inlineCenter: 'inline-flex items-center justify-center',
  footerLegalLink: 'mx-[3px] underline [text-underline-offset:auto]',
  authProviderBtn:
    'flex h-[52px] cursor-pointer items-center justify-center rounded-[99999px] border border-[rgba(0,0,0,0.15)] bg-transparent px-6 text-[#0d0d0d] transition-[background-color,border-color] duration-150 hover:border-[#cfd2db] hover:bg-[#f5f5f5]',
  authProviderBtnInner: 'inline-flex items-center justify-center gap-2 text-[16px] leading-6 font-normal',
  authProviderBtnIcon: 'inline-flex h-[18px] w-[18px] items-center justify-center text-[#0d0d0d]',
  dividerLine: 'h-px bg-[#ececec]',

  sidebarHeader: 'block px-4 pt-2',
  sidebarHeaderRow: 'flex items-center justify-between',
  sidebarHomeLink: 'm-0 inline-flex h-9 w-9 flex-[0_0_36px] items-center justify-center rounded-[10px] p-0 leading-none',
  sidebarHomeLogo: 'block h-9 w-9',
  sidebarCloseButton: 'mr-[-8px] inline-flex h-9 w-9 shrink-0 cursor-[w-resize] items-center justify-center rounded-[8px] border-0 bg-transparent text-[#737373] hover:bg-[#ececec] focus-visible:bg-[#ececec] max-[767px]:h-10 max-[767px]:w-10',
  sidebarCloseDesktopIcon: 'block h-5 w-5 max-[767px]:hidden',
  sidebarCloseMobileIcon: 'hidden h-5 w-5 max-[767px]:block',
  sidebarScrollArea: 'grid min-h-0 content-start gap-0 overflow-auto p-0',
  sidebarListAside: 'pt-0',
  sidebarMenuList: 'mt-[14px] grid gap-0 p-0',
  sidebarNewChatButton: 'group/menu flex min-h-9 w-full cursor-pointer items-center justify-between gap-2 rounded-[10px] border-0 bg-transparent px-4 py-[6px] text-left tracking-[0] text-[#0d0d0d] hover:bg-[#ececec] focus:outline-none max-[767px]:min-h-10',
  sidebarNewChatText: 'block min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[14px] leading-5 font-normal',
  sidebarSearchLabel: 'group/menu flex min-h-9 w-full cursor-text items-center justify-between gap-2 rounded-[10px] border-0 bg-transparent px-4 py-[6px] text-left tracking-[0] text-[#0d0d0d] hover:bg-[#ececec] focus-within:outline-none max-[767px]:min-h-10',
  sidebarSearchInput: 'w-full min-w-0 border-0 bg-transparent p-0 text-[14px] leading-5 text-[#0d0d0d] outline-0 placeholder:text-[#0d0d0d] placeholder:opacity-100',
  sidebarMenuSpacer: 'pb-[14px]',
  sidebarHistoryToggle: 'group/expando-btn inline-flex w-full cursor-pointer items-center justify-start gap-[2px] rounded-none border-0 bg-transparent px-4 py-[6px] text-left text-[#737373] hover:text-[#5f5f5f]',
  sidebarHistoryTitle: 'm-0 text-[14px] leading-4 font-medium tracking-[0]',
  sidebarThreadList: 'mb-2 grid gap-0 px-2',
  sidebarThreadTitle: 'block overflow-hidden text-ellipsis whitespace-nowrap text-[14px] leading-[34px] font-normal',
  sidebarAccountPanel: 'grid gap-[6px] p-2',
  sidebarAccountCard: 'mt-[6px] grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2 rounded-[12px] border border-[rgba(0,0,0,0.08)] bg-white p-2',
  sidebarAvatar: 'inline-flex h-7 w-7 items-center justify-center rounded-[9999px] bg-[#0d0d0d] text-[12px] font-semibold text-white',
  sidebarAccountMeta: 'grid min-w-0',
  sidebarAccountEmail: 'overflow-hidden text-ellipsis whitespace-nowrap text-[12px] leading-4',
  sidebarLogoutButton: 'cursor-pointer rounded-[8px] border-0 bg-transparent px-[6px] py-1 text-[12px] text-[#474747] hover:bg-[#efefef] hover:text-[#0d0d0d]',
  chatMainLayout: 'grid min-h-screen grid-rows-[52px_minmax(0,1fr)_auto]',
  chatTopBar: 'flex h-[52px] items-center justify-between bg-white px-[14px] py-2 max-[767px]:px-[10px]',
  chatTopBarLeft: 'flex items-center gap-2',
  chatSidebarOpenIcon: 'block h-[18px] w-[18px]',
  chatModelButton: 'inline-flex h-[34px] cursor-pointer items-center gap-1 rounded-[8px] border-0 bg-transparent px-2 text-[16px] leading-6 font-medium text-[#0d0d0d] hover:bg-[#f4f4f4] max-[767px]:text-[15px]',
  chatTopBarRight: 'inline-flex items-center gap-2',
  chatConversationSection: 'flex min-h-0 justify-center',
  chatConversationScroller: 'w-full overflow-auto px-[18px] py-6 max-[767px]:px-3 max-[767px]:py-[18px]',
  chatConversationStack: 'mx-auto grid max-w-[760px] gap-5',
  chatTimestampRow: 'flex items-center justify-center',
  chatTimestampPill: 'rounded-[9999px] border border-[rgba(0,0,0,0.12)] bg-white px-[10px] py-1 text-[12px] text-[#6d6d6d]',
  sourceList: 'flex flex-wrap gap-2',
  sourceCard: 'grid min-w-[180px] cursor-pointer gap-[2px] rounded-[10px] border border-[rgba(0,0,0,0.14)] bg-white px-[10px] py-2 text-left hover:bg-[#f8f8f8]',
  sourceCardTitle: 'text-[12px] leading-4 text-[#1b1b1b]',
  sourceCardLocation: 'text-[11px] leading-[14px] text-[#6f6f6f]',
  sourceCardSnippet: 'text-[11px] leading-[15px] text-[#7b7b7b]',
  assistantTypingRow: 'grid items-start justify-items-start gap-[10px] [grid-template-columns:auto_minmax(0,1fr)] max-[767px]:grid-cols-[minmax(0,1fr)] max-[767px]:gap-2',
  assistantTypingBubble: 'inline-flex h-9 w-[72px] items-center justify-center gap-[6px] rounded-[18px] border border-[rgba(0,0,0,0.1)] bg-white',
  typingDotOne: 'h-1.5 w-1.5 rounded-[9999px] bg-[#7d7d7d] [animation:chatgpt-app-bounce_1.2s_infinite_ease-in-out]',
  typingDotTwo: 'h-1.5 w-1.5 rounded-[9999px] bg-[#7d7d7d] [animation:chatgpt-app-bounce_1.2s_infinite_ease-in-out] [animation-delay:0.12s]',
  typingDotThree: 'h-1.5 w-1.5 rounded-[9999px] bg-[#7d7d7d] [animation:chatgpt-app-bounce_1.2s_infinite_ease-in-out] [animation-delay:0.24s]',
  chatComposerDock: 'sticky bottom-0 bg-[linear-gradient(to_top,#fff_72%,rgba(255,255,255,0))] pb-2',
  chatEmptyStateSection: 'flex min-h-0 flex-1 w-full flex-col',
  guestPage: 'min-h-screen bg-white px-[15px] text-[#0d0d0d] max-[760px]:px-2',
  skipLink: 'absolute left-[-9999px] top-auto h-px w-px overflow-hidden focus:left-[10px] focus:top-[10px] focus:z-50 focus:h-auto focus:w-auto focus:rounded-[8px] focus:bg-[#0d0d0d] focus:px-[10px] focus:py-[6px] focus:text-white',
  guestTopBar: 'sticky top-0 z-20 flex h-[52px] items-center justify-between bg-white p-2',
  guestLogoButton: 'inline-flex h-9 w-9 items-center justify-center rounded-[10px] border-0 bg-transparent transition-colors duration-150 hover:bg-[#f4f4f4] [@media(hover:none)_and_(pointer:coarse)]:h-10 [@media(hover:none)_and_(pointer:coarse)]:w-10',
  guestLogoImage: 'h-9 w-9 rounded-none object-contain',
  guestModelButton: 'inline-flex h-9 cursor-pointer items-center gap-1 rounded-[8px] border-0 bg-transparent px-[10px] text-[18px] leading-7 font-normal text-[#0d0d0d] transition-colors duration-150 hover:bg-[#f4f4f4] max-[760px]:text-[15px]',
  guestLoginButton: 'inline-flex h-9 cursor-pointer items-center justify-center rounded-[99999px] border border-transparent bg-[#0d0d0d] bg-clip-padding px-3 text-[14px] leading-5 font-medium text-white transition-colors duration-150 focus-visible:outline focus-visible:outline-[1.5px] focus-visible:outline-offset-[2.5px] focus-visible:outline-[#0d0d0d] [@media(pointer:coarse)]:min-h-10',
  guestRegisterButton: 'ml-2 inline-flex h-9 cursor-pointer items-center justify-center rounded-[99999px] border border-[rgba(0,0,0,0.15)] bg-white px-3 text-[14px] leading-5 font-medium text-[#0d0d0d] transition-colors duration-150 hover:bg-[#f8f8f8] focus-visible:outline focus-visible:outline-[1.5px] focus-visible:outline-offset-[2.5px] focus-visible:outline-[#0d0d0d] [@media(pointer:coarse)]:min-h-10',
  guestProfileButton: 'group inline-flex select-none items-center justify-center border-0 bg-transparent pl-2 text-[#0d0d0d] outline-0 max-[760px]:hidden',
  guestProfileIconWrap: 'inline-flex h-9 w-9 items-center justify-center rounded-[99999px] transition-colors duration-150 group-hover:bg-[#f4f4f4]',
  guestProfileIcon: 'block h-6 w-6',
  guestMain: 'flex min-h-[calc(100vh-52px)] justify-center',
  guestMainColumn: 'flex min-h-[calc(100vh-52px)] w-full flex-col items-stretch',
  legalFooter: 'relative mt-auto w-full overflow-hidden px-0 text-center text-[12px] leading-4 text-[#5d5d5d] select-none md:px-[60px] max-[760px]:px-0',
  legalText: 'm-0 flex min-h-8 w-full items-center justify-center px-2 text-center max-[760px]:px-1',
  authModalRoot: 'fixed inset-0 z-50',
  authModalBackdrop: 'absolute inset-0 bg-[rgba(229,231,235,0.5)] opacity-100 backdrop-blur-[1px] transition-[opacity,backdrop-filter] duration-250',
  authModalViewport: 'relative z-[1] grid h-full w-full grid-cols-[10px_1fr_10px] grid-rows-[minmax(10px,1fr)_auto_minmax(10px,1fr)] overflow-y-auto md:grid-rows-[minmax(20px,0.8fr)_auto_minmax(20px,1fr)]',
  authModalDialog: 'col-start-2 row-start-2 flex w-full max-w-[373px] flex-col justify-self-center overflow-hidden rounded-[16px] bg-white shadow-[0_8px_12px_rgba(0,0,0,0.08),0_0_1px_rgba(0,0,0,0.62)] [animation:chatgpt-auth-dialog-in_250ms_ease] md:max-w-[388px]',
  authModalHeader: 'flex min-h-[52px] items-start justify-between px-[10px] pb-0 pl-4 pt-[10px]',
  authModalHeaderSpacer: 'flex-1',
  authModalCloseButton: 'inline-flex h-9 w-9 cursor-pointer items-center justify-center rounded-[99999px] border-0 bg-transparent text-[#0d0d0d] hover:bg-[#f4f4f4]',
  authModalBodyScroll: 'flex-1 overflow-x-hidden overflow-y-auto',
  authModalBody: 'flex flex-col items-stretch gap-5 px-6 pb-10 pt-0',
  authModalTitle: 'm-0 text-center text-[32px] leading-[1.2] font-normal tracking-[-0.01em] text-[#0d0d0d]',
  authModalDescription: 'm-0 mx-4 mb-4 text-center text-[16px] leading-[1.4] font-normal text-[#0d0d0d]',
  authModalForm: 'm-0 flex w-full flex-col gap-4',
  authProviderGroup: 'grid gap-3',
  authDivider: 'my-2 grid grid-cols-[1fr_max-content_1fr] items-center',
  authDividerText: 'mx-6 text-[13px] leading-none font-[510] uppercase text-[#5d5d5d]',
  authEmailBlock: 'flex flex-col gap-1',
  authEmailInput: 'm-0 h-6 w-full rounded-none border-0 bg-transparent p-0 text-[16px] leading-6 tracking-[-0.16px] text-black outline-none [appearance:none] [-webkit-appearance:none] placeholder:text-[#b4b4b4] placeholder:opacity-100 disabled:text-[#545454]',
  authErrorRow: 'mt-1 flex items-center gap-2 text-[12px] leading-[16.8px] font-normal text-[#d00e17]',
  authErrorIconWrap: 'inline-flex h-4 w-4 items-center justify-center',
  authSubmitButton: 'mt-[6px] h-[52px] w-full cursor-pointer rounded-[99999px] border-0 bg-[#131313] text-[16px] leading-6 font-normal tracking-[-0.16px] text-white transition-[background-color,opacity] duration-150 hover:bg-[#090909] disabled:cursor-default disabled:bg-[#131313] disabled:opacity-50',
} as const

function pickNextHeading(current?: string) {
  if (headingPool.length <= 1) return headingPool[0] ?? ''
  const candidates = current ? headingPool.filter((item) => item !== current) : headingPool
  const source = candidates.length ? candidates : headingPool
  return source[Math.floor(Math.random() * source.length)]
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
  const [heading, setHeading] = useState(() => pickNextHeading())
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
    const historyThreads = threads.filter((thread) => thread.messages.length > 0)
    const keyword = threadSearch.trim().toLowerCase()
    if (!keyword) return historyThreads
    return historyThreads.filter((thread) => {
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

  const resetGuestComposer = useCallback(() => {
    if (guestEditorRef.current) guestEditorRef.current.textContent = ''
    setGuestPrompt('')
  }, [])

  const resetAuthComposer = useCallback(() => {
    if (authEditorRef.current) authEditorRef.current.textContent = ''
    setAuthPrompt('')
  }, [])

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
    const isOnDraftThread = Boolean(activeThread && activeThread.messages.length === 0)
    if (isOnDraftThread) {
      setHeading((current) => pickNextHeading(current))
      resetAuthComposer()
      setIsSidebarOpen(false)
      requestAnimationFrame(() => authEditorRef.current?.focus())
      return
    }

    const newThread = createEmptyThread()
    setThreads((previous) => [newThread, ...previous])
    setActiveThreadId(newThread.id)
    setHeading((current) => pickNextHeading(current))
    resetAuthComposer()
    setIsSidebarOpen(false)
    requestAnimationFrame(() => authEditorRef.current?.focus())
  }, [activeThread, resetAuthComposer])

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

  const hasActiveConversation = Boolean(activeThread && activeThread.messages.length > 0)
  const renderAuthComposer = (threadMaxWrapperClass: string) => (
    <div className={chatTw.threadContent}>
      <div className={threadMaxWrapperClass}>
        <div className={chatTw.composerContainer}>
          <form className={chatTw.composerForm} data-expanded="" data-type="unified-composer" onSubmit={handleAuthComposerSubmit}>
            <div className={chatTw.hidden}>
              <input
                ref={uploadAuthInputRef}
                accept="image/jpeg,.jpg,.jpeg,image/webp,.webp,image/gif,.gif,image/png,.png"
                multiple
                type="file"
                tabIndex={-1}
              />
            </div>

            <div className={chatTw.composerSurface} data-composer-surface="true">
              <div className={chatTw.composerPrimary}>
                <div className={chatTw.composerEditorWrap}>
                  <div
                    ref={authEditorRef}
                    contentEditable
                    suppressContentEditableWarning
                    translate="no"
                    role="textbox"
                    id="prompt-textarea-auth"
                    className={chatTw.composerEditor}
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
                    <div className={chatTw.composerPlaceholder} aria-hidden="true">
                      {canAsk
                        ? `基于《${selectedBook?.title ?? '所选书籍'}》提问，回答将附带可追溯引用`
                        : '请先在书库上传并等待书籍处理完成'}
                    </div>
                  ) : null}
                </div>
              </div>

              <div className={chatTw.composerFooterActions} data-testid="composer-footer-actions">
                <div className={chatTw.composerFooterRow}>
                  {quickActions.map((item) => (
                    <button
                      key={item.label}
                      type="button"
                      className={chatTw.composerAttachButton}
                      onClick={() => handleComposerActionClick(false)}
                    >
                      <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={chatTw.composerActionIcon}>
                        <use href={`${CHAT_ICON_SPRITE_URL}#${item.symbolId}`} fill="currentColor" />
                      </svg>
                      <span className={chatTw.composerActionText}>{item.label}</span>
                    </button>
                  ))}
                </div>
              </div>

              <div className={chatTw.composerTrailing}>
                {!hasAuthPrompt ? (
                  <button
                    type="button"
                    className={chatTw.composerVoiceButton}
                    aria-label="启动语音功能"
                    data-testid="composer-speech-button"
                  >
                    <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={chatTw.composerVoiceIcon}>
                      <use href={`${CHAT_ICON_SPRITE_URL}#chat-voice`} fill="currentColor" />
                    </svg>
                    <span className={chatTw.composerVoiceText}>语音</span>
                  </button>
                ) : (
                  <button
                    type="submit"
                    className={chatTw.composerSendButton}
                    aria-label="发送提示"
                    data-testid="send-button"
                    disabled={activeThreadIsSending || !canAsk}
                  >
                    <svg
                      viewBox="0 0 20 20"
                      xmlns="http://www.w3.org/2000/svg"
                      aria-hidden="true"
                      className={chatTw.composerSendIcon}
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
  )

  if (sessionUser) {
    const avatarLetter = sessionUser.email.slice(0, 1).toUpperCase()
    const isSidebarExpanded = isMobileSidebarViewport() ? isSidebarOpen : !isDesktopSidebarCollapsed

    return (
      <div
        className={cx(
          'grid min-h-screen bg-white text-[#0d0d0d] max-[767px]:grid-cols-[minmax(0,1fr)]',
          isDesktopSidebarCollapsed ? 'grid-cols-[minmax(0,1fr)]' : 'grid-cols-[260px_minmax(0,1fr)]',
        )}
        style={chatUiSansStyle}
      >
        <button
          type="button"
          className={cx(
            'fixed inset-0 z-[38] border-0 bg-[rgba(0,0,0,0.42)] transition-opacity duration-180',
            isSidebarOpen ? 'pointer-events-auto opacity-100' : 'pointer-events-none opacity-0',
          )}
          aria-hidden={!isSidebarOpen}
          tabIndex={-1}
          onClick={() => setIsSidebarOpen(false)}
        />

        <aside
          id="stage-slideover-sidebar"
          className={cx(
            'min-h-screen grid grid-rows-[auto_minmax(0,1fr)_auto] bg-[#f7f7f7] p-0 max-[767px]:fixed max-[767px]:bottom-0 max-[767px]:left-0 max-[767px]:top-0 max-[767px]:z-[39] max-[767px]:w-[min(82vw,300px)] max-[767px]:-translate-x-[104%] max-[767px]:shadow-[6px_0_30px_rgba(0,0,0,0.15)] max-[767px]:transition-transform max-[767px]:duration-180',
            isSidebarOpen && 'max-[767px]:translate-x-0',
            isDesktopSidebarCollapsed && 'md:hidden',
          )}
          aria-label="会话侧边栏"
        >
          <div className={chatTw.sidebarHeader}>
            <div className={chatTw.sidebarHeaderRow}>
              <Link to="/chat" className={chatTw.sidebarHomeLink} aria-label="OneBook AI">
                <img src={onebookLogoMark} alt="" aria-hidden="true" className={chatTw.sidebarHomeLogo} />
              </Link>
              <button
                type="button"
                className={chatTw.sidebarCloseButton}
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
                  className={chatTw.sidebarCloseDesktopIcon}
                >
                  <use href={`${CHAT_ICON_SPRITE_URL}#chat-sidebar-close-desktop`} fill="currentColor" />
                </svg>
                <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={chatTw.sidebarCloseMobileIcon}>
                  <use href={`${CHAT_ICON_SPRITE_URL}#chat-sidebar-close-mobile`} fill="currentColor" />
                </svg>
              </button>
            </div>
          </div>

          <div className={chatTw.sidebarScrollArea} role="listbox" aria-label="会话列表">
            <aside className={chatTw.sidebarListAside}>
              <div className={chatTw.sidebarMenuList}>
                <button
                  type="button"
                  className={chatTw.sidebarNewChatButton}
                  onClick={handleCreateConversation}
                >
                  <span className={chatTw.menuMainIconWrap}>
                    <span className={chatTw.menuMainIcon} aria-hidden="true">
                      <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" className={chatTw.iconBlockH5W5}>
                        <use href={`${CHAT_ICON_SPRITE_URL}#chat-new-chat`} fill="currentColor" />
                      </svg>
                    </span>
                    <span className={chatTw.menuMainTextWrap}>
                      <span className={chatTw.sidebarNewChatText}>新聊天</span>
                    </span>
                  </span>
                  <span className={chatTw.shortcutLabel} aria-hidden="true">
                    <span className={chatTw.shortcutRow}>
                      {newChatShortcutKeys.map((key) => (
                        <kbd key={`new-chat-shortcut-${key}`} aria-label={getShortcutAriaLabel(key)} className={chatTw.shortcutKeyWrap}>
                          <span className={chatTw.shortcutKey}>{key}</span>
                        </kbd>
                      ))}
                    </span>
                  </span>
                </button>

                <label className={chatTw.sidebarSearchLabel} htmlFor="chat-thread-search">
                  <span className={chatTw.menuMainIconWrap}>
                    <span className={chatTw.menuMainIcon} aria-hidden="true">
                      <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" className={chatTw.iconBlockH5W5}>
                        <use href={`${CHAT_ICON_SPRITE_URL}#chat-search`} fill="currentColor" />
                      </svg>
                    </span>
                    <span className={chatTw.menuMainTextWrap}>
                      <input
                        id="chat-thread-search"
                        ref={threadSearchInputRef}
                        className={chatTw.sidebarSearchInput}
                        type="search"
                        placeholder="搜索聊天"
                        value={threadSearch}
                        onChange={(event) => setThreadSearch(event.target.value)}
                      />
                    </span>
                  </span>
                  <span className={chatTw.shortcutLabel} aria-hidden="true">
                    <span className={chatTw.shortcutRow}>
                      {searchShortcutKeys.map((key) => (
                        <kbd key={`search-shortcut-${key}`} aria-label={getShortcutAriaLabel(key)} className={chatTw.shortcutKeyWrap}>
                          <span className={chatTw.shortcutKey}>{key}</span>
                        </kbd>
                      ))}
                    </span>
                  </span>
                </label>
              </div>
            </aside>
            <div className={chatTw.sidebarMenuSpacer} aria-hidden="true" />

            <div className={cx('group/expando mb-[6px]', isHistoryExpanded && 'mb-2')}>
              <button
                type="button"
                aria-expanded={isHistoryExpanded}
                className={chatTw.sidebarHistoryToggle}
                onClick={() => setIsHistoryExpanded((prev) => !prev)}
              >
                <h2 className={chatTw.sidebarHistoryTitle}>你的聊天</h2>
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="16"
                  height="16"
                  aria-hidden="true"
                  className={cx(
                    'h-3 w-3 shrink-0 text-[#7a7a7a] opacity-0 transition-[transform,opacity] duration-120 group-hover/expando:opacity-100 group-focus-within/expando:opacity-100',
                    !isHistoryExpanded && 'opacity-100',
                    isHistoryExpanded ? 'rotate-0' : '-rotate-90',
                  )}
                >
                  <use href={`${CHAT_ICON_SPRITE_URL}#chat-chevron-down`} fill="currentColor" />
                </svg>
              </button>
            </div>

            {isHistoryExpanded ? (
              <>
                <div className={chatTw.sidebarThreadList}>
                  {filteredThreads.map((thread) => {
                    const isActive = thread.id === activeThreadId
                    return (
                      <button
                        key={thread.id}
                        type="button"
                        className={cx(
                          'group/thread relative block min-h-[34px] cursor-pointer rounded-[8px] border-0 bg-transparent px-[9px] pr-[30px] text-left text-[#0d0d0d] hover:bg-[#ececec] focus:outline-none',
                          isActive && 'bg-[#e7e7e7]',
                        )}
                        onClick={() => handleThreadSelect(thread.id)}
                      >
                        <span className={chatTw.sidebarThreadTitle}>{thread.title}</span>
                        <span
                          className={cx(
                            'pointer-events-none absolute right-[6px] top-1/2 inline-flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded-[6px] text-[#8c8c8c] opacity-0 transition-opacity duration-140 group-hover/thread:opacity-100',
                            isActive && 'opacity-100',
                          )}
                          aria-hidden="true"
                        >
                          <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" className={chatTw.iconBlockH14W14}>
                            <path d="M5 10a1.2 1.2 0 1 0 0-2.4A1.2 1.2 0 0 0 5 10Zm5 0a1.2 1.2 0 1 0 0-2.4A1.2 1.2 0 0 0 10 10Zm5 0a1.2 1.2 0 1 0 0-2.4A1.2 1.2 0 0 0 15 10Z" fill="currentColor" />
                          </svg>
                        </span>
                      </button>
                    )
                  })}
                </div>
              </>
            ) : null}
          </div>

          <div className={chatTw.sidebarAccountPanel}>
            <div className={chatTw.sidebarAccountCard}>
              <span className={chatTw.sidebarAvatar} aria-hidden="true">
                {avatarLetter}
              </span>
              <div className={chatTw.sidebarAccountMeta}>
                <span className={chatTw.sidebarAccountEmail}>{sessionUser.email}</span>
                <span className={chatTw.roleMuted}>{sessionUser.role === 'admin' ? '管理员' : '普通用户'}</span>
              </div>
              <button type="button" className={chatTw.sidebarLogoutButton} onClick={() => void handleLogout()}>
                退出
              </button>
            </div>
          </div>
        </aside>

        <main className={chatTw.chatMainLayout}>
          <header className={chatTw.chatTopBar}>
            <div className={chatTw.chatTopBarLeft}>
              <button
                type="button"
                className={cx(
                  'hidden h-[34px] w-[34px] cursor-pointer items-center justify-center rounded-[8px] border-0 bg-transparent text-[#0d0d0d] hover:bg-[#f1f1f1] max-[767px]:inline-flex',
                  isDesktopSidebarCollapsed && 'md:inline-flex',
                )}
                aria-label="打开会话侧栏"
                onClick={handleOpenSidebar}
              >
                <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={chatTw.chatSidebarOpenIcon}>
                  <path d="M4 5.5H16" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
                  <path d="M4 10H16" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
                  <path d="M4 14.5H12" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
                </svg>
              </button>
              <button type="button" className={chatTw.chatModelButton} aria-label="模型选择器，当前模型为 OneBook AI">
                <span>OneBook AI</span>
                  <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={chatTw.iconBlockH14W14}>
                  <use href={`${CHAT_ICON_SPRITE_URL}#chat-chevron-down`} fill="currentColor" />
                </svg>
              </button>
            </div>

            <div className={chatTw.chatTopBarRight}>
              <button type="button" className={chatTw.topActionPill}>临时</button>
              <button type="button" className={chatTw.topActionPill}>分享</button>
            </div>
          </header>

          {hasActiveConversation ? (
            <>
              <section className={chatTw.chatConversationSection} aria-label="会话内容">
                <div className={chatTw.chatConversationScroller}>
                  <div className={chatTw.chatConversationStack}>
                    {activeThread ? (
                      <>
                        {bookListErrorText ? (
                          <div className={chatTw.alertBox} role="status" aria-live="polite">
                            <span>{bookListErrorText}</span>
                            <button type="button" className={chatTw.alertAction} onClick={() => void loadReadyBooks()}>
                              重新加载
                            </button>
                          </div>
                        ) : null}

                        <div className={chatTw.chatTimestampRow}>
                          <span className={chatTw.chatTimestampPill}>{getRelativeTimeLabel(activeThread.updatedAt)}</span>
                        </div>

                        {activeThread.messages.map((message) => (
                          <article
                            key={message.id}
                            className={
                              message.role === 'user'
                                ? 'grid justify-items-end gap-2'
                                : 'grid items-start justify-items-start gap-[10px] [grid-template-columns:auto_minmax(0,1fr)] max-[767px]:grid-cols-[minmax(0,1fr)] max-[767px]:gap-2'
                            }
                          >
                            {message.role === 'assistant' ? <div className={chatTw.assistantBadge}>AI</div> : null}
                            <div className={chatTw.messageGrid}>
                              <div className={cx(
                                'max-w-[min(92%,720px)] whitespace-pre-wrap rounded-[18px] px-[14px] py-3 text-[15px] leading-[23px] max-[767px]:max-w-full max-[767px]:text-[14px] max-[767px]:leading-[22px]',
                                message.role === 'user'
                                  ? 'bg-[#f1f1f1] text-[#0d0d0d]'
                                  : 'border border-[rgba(0,0,0,0.1)] bg-white text-[#0d0d0d]',
                              )}>{message.text}</div>
                              {message.role === 'assistant' && message.sources?.length ? (
                                <div className={chatTw.sourceList}>
                                  {message.sources.map((source) => (
                                    <button key={`${source.label}-${source.location}`} type="button" className={chatTw.sourceCard}>
                                      <span className={chatTw.sourceCardTitle}>{source.label}</span>
                                      <span className={chatTw.sourceCardLocation}>{source.location}</span>
                                      {source.snippet ? <span className={chatTw.sourceCardSnippet}>{source.snippet}</span> : null}
                                    </button>
                                  ))}
                                </div>
                              ) : null}
                            </div>
                          </article>
                        ))}

                        {activeThreadIsSending ? (
                          <article className={chatTw.assistantTypingRow}>
                            <div className={chatTw.assistantBadge}>AI</div>
                            <div className={chatTw.messageGrid}>
                              <div className={chatTw.assistantTypingBubble}>
                                <span className={chatTw.typingDotOne} />
                                <span className={chatTw.typingDotTwo} />
                                <span className={chatTw.typingDotThree} />
                              </div>
                            </div>
                          </article>
                        ) : null}

                        {activeThreadHasError ? (
                          <div className={chatTw.alertBox} role="status" aria-live="polite">
                            <span>{activeThread.errorText}</span>
                            <button type="button" className={chatTw.alertAction} onClick={handleRetryAssistant}>
                              重试
                            </button>
                          </div>
                        ) : null}
                      </>
                    ) : null}
                  </div>
                </div>
              </section>

              <section className={chatTw.chatComposerDock} aria-label="输入区">
                {renderAuthComposer('mx-auto mb-[10px] w-full max-w-[48rem] max-[760px]:max-w-full')}
              </section>
            </>
          ) : (
            <section className={chatTw.chatEmptyStateSection} aria-label="输入区">
              <div className={chatTw.heroShell}>
                <div className={chatTw.heroCenter}>
                  <div className={chatTw.heroInner}>
                    <h1 className={chatTw.heroTitle}>
                      <div className={chatTw.heroHeading}>{heading}</div>
                    </h1>
                  </div>
                </div>
              </div>
              <div className={chatTw.composerForm} id="thread-bottom">
                {renderAuthComposer(chatTw.threadMax)}
              </div>
            </section>
          )}
        </main>
      </div>
    )
  }

  return (
    <div className={chatTw.guestPage} style={chatUiSansStyle}>
      <a className={chatTw.skipLink} href="#onebook-main">
        跳至内容
      </a>

      <header className={chatTw.guestTopBar} role="banner">
        <div className={chatTw.headerActionRow}>
          <Link to="/chat" className={chatTw.guestLogoButton} aria-label="OneBook AI">
            <img src={onebookLogoMark} alt="" aria-hidden="true" className={chatTw.guestLogoImage} />
          </Link>
          <button type="button" className={chatTw.guestModelButton} aria-label="模型选择器，当前模型为 OneBook AI">
            <span>OneBook AI</span>
            <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={chatTw.iconBlockH14W14}>
              <use href={`${CHAT_ICON_SPRITE_URL}#chat-chevron-down`} fill="currentColor" />
            </svg>
          </button>
        </div>

        <div className={chatTw.headerActionRow}>
          <button type="button" className={chatTw.guestLoginButton} onClick={() => openAuthModal('login')}>
            <div className={chatTw.inlineCenter}>登录</div>
          </button>
          <button type="button" className={chatTw.guestRegisterButton} onClick={() => openAuthModal('register')}>
            <div className={chatTw.inlineCenter}>免费注册</div>
          </button>
          <button type="button" className={chatTw.guestProfileButton} aria-label="打开“个人资料”菜单">
            <div className={chatTw.guestProfileIconWrap}>
              <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={chatTw.guestProfileIcon}>
                <use href={`${CHAT_ICON_SPRITE_URL}#chat-profile`} fill="currentColor" />
              </svg>
            </div>
          </button>
        </div>
      </header>

      <main id="onebook-main" className={chatTw.guestMain}>
        <div className={chatTw.guestMainColumn}>
          <div className={chatTw.heroShell}>
            <div className={chatTw.heroCenter}>
              <div className={chatTw.heroInner}>
                <h1 className={chatTw.heroTitle}>
                  <div className={chatTw.heroHeading}>{heading}</div>
                </h1>
              </div>
            </div>
          </div>

          <div className={chatTw.composerForm} id="thread-bottom">
            <div className={chatTw.threadContent}>
              <div className={chatTw.threadMax}>
                <div className={chatTw.composerContainer}>
                  <form
                    className={chatTw.composerForm}
                    data-expanded=""
                    data-type="unified-composer"
                    onSubmit={handleGuestComposerSubmit}
                  >
                    <div className={chatTw.hidden}>
                      <input
                        ref={uploadGuestInputRef}
                        accept="image/jpeg,.jpg,.jpeg,image/webp,.webp,image/gif,.gif,image/png,.png"
                        multiple
                        type="file"
                        tabIndex={-1}
                      />
                    </div>

                    <div className={chatTw.composerSurface} data-composer-surface="true">
                      <div className={chatTw.composerPrimary}>
                        <div className={chatTw.composerEditorWrap}>
                          <div
                            ref={guestEditorRef}
                            contentEditable
                            suppressContentEditableWarning
                            translate="no"
                            role="textbox"
                            id="prompt-textarea"
                            className={chatTw.composerEditor}
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
                            <div className={chatTw.composerPlaceholder} aria-hidden="true">
                              有问题，尽管问
                            </div>
                          ) : null}
                        </div>
                      </div>

                      <div className={chatTw.composerFooterActions} data-testid="composer-footer-actions">
                        <div className={chatTw.composerFooterRow}>
                          {quickActions.map((item) => (
                            <button
                              key={item.label}
                              type="button"
                              className={chatTw.composerAttachButton}
                              onClick={() => handleComposerActionClick(true)}
                            >
                              <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={chatTw.composerActionIcon}>
                                <use href={`${CHAT_ICON_SPRITE_URL}#${item.symbolId}`} fill="currentColor" />
                              </svg>
                              <span className={chatTw.composerActionText}>{item.label}</span>
                            </button>
                          ))}
                        </div>
                      </div>

                      <div className={chatTw.composerTrailing}>
                        {!hasGuestPrompt ? (
                          <button
                            type="button"
                            className={chatTw.composerVoiceButton}
                            aria-label="启动语音功能"
                            data-testid="composer-speech-button"
                          >
                            <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={chatTw.composerVoiceIcon}>
                              <use href={`${CHAT_ICON_SPRITE_URL}#chat-voice`} fill="currentColor" />
                            </svg>
                            <span className={chatTw.composerVoiceText}>语音</span>
                          </button>
                        ) : (
                          <button
                            type="submit"
                            className={chatTw.composerSendButton}
                            aria-label="发送提示"
                            data-testid="send-button"
                          >
                            <svg
                              viewBox="0 0 20 20"
                              xmlns="http://www.w3.org/2000/svg"
                              aria-hidden="true"
                              className={chatTw.composerSendIcon}
                            >
                              <use href={`${CHAT_ICON_SPRITE_URL}#chat-send`} fill="currentColor" />
                            </svg>
                          </button>
                        )}
                      </div>
                    </div>
                  </form>
                </div>

                <input className={chatTw.srOnly} type="file" tabIndex={-1} aria-hidden="true" id="upload-photos" accept="image/*" multiple />
                <input
                  className={chatTw.srOnly}
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

          <div className={chatTw.legalFooter}>
            <p className={chatTw.legalText}>
              向 OneBook AI 发送消息即表示，你同意我们的
              <a href="#" onClick={(e) => e.preventDefault()} className={chatTw.footerLegalLink}>
                条款
              </a>
              并已阅读我们的
              <a href="#" onClick={(e) => e.preventDefault()} className={chatTw.footerLegalLink}>
                隐私政策
              </a>
              。查看
              <a href="#" onClick={(e) => e.preventDefault()} className={chatTw.footerLegalLink}>
                Cookie 首选项
              </a>
              。
            </p>
          </div>
        </div>
      </main>

      {isAuthOpen ? (
        <div id="modal-no-auth-login" className={chatTw.authModalRoot}>
          <div className={chatTw.authModalBackdrop} onClick={closeAuthModal} aria-hidden="true" />
          <div className={chatTw.authModalViewport}>
            <div
              role="dialog"
              aria-modal="true"
              aria-labelledby="chatgpt-auth-dialog-title"
              className={chatTw.authModalDialog}
              onClick={(event) => event.stopPropagation()}
            >
              <header className={chatTw.authModalHeader}>
                <div className={chatTw.authModalHeaderSpacer} />
                <button type="button" className={chatTw.authModalCloseButton} aria-label="关闭" onClick={closeAuthModal}>
                  <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={chatTw.iconBlockH5W5}>
                    <use href={`${CHAT_ICON_SPRITE_URL}#chat-close`} fill="currentColor" />
                  </svg>
                </button>
              </header>

              <div className={chatTw.authModalBodyScroll}>
                <div className={chatTw.authModalBody} data-testid="login-form">
                  <h2 id="chatgpt-auth-dialog-title" className={chatTw.authModalTitle}>登录或注册</h2>
                  <p className={chatTw.authModalDescription}>
                    你将可以基于个人书库提问，并获得可追溯来源的回答。
                  </p>

                  <form className={chatTw.authModalForm} onSubmit={handleAuthSubmit} noValidate>
                    <div className={chatTw.authProviderGroup} role="group" aria-label="选择登录选项">
                      <button type="button" className={chatTw.authProviderBtn} onClick={() => navigate('/log-in')}>
                        <span className={chatTw.authProviderBtnInner}>
                          <span className={chatTw.authProviderBtnIcon}>
                            <img src={googleLogo} alt="" aria-hidden="true" />
                          </span>
                          <span>继续使用 Google 登录</span>
                        </span>
                      </button>
                      <button type="button" className={chatTw.authProviderBtn} onClick={() => navigate('/log-in')}>
                        <span className={chatTw.authProviderBtnInner}>
                          <span className={chatTw.authProviderBtnIcon}>
                            <img src={appleLogo} alt="" aria-hidden="true" />
                          </span>
                          <span>继续使用 Apple 登录</span>
                        </span>
                      </button>
                      <button type="button" className={chatTw.authProviderBtn} onClick={() => navigate('/log-in')}>
                        <span className={chatTw.authProviderBtnInner}>
                          <span className={chatTw.authProviderBtnIcon}>
                            <img src={microsoftLogo} alt="" aria-hidden="true" />
                          </span>
                          <span>继续使用 Microsoft 登录</span>
                        </span>
                      </button>
                      <button type="button" className={chatTw.authProviderBtn} onClick={() => navigate('/log-in')}>
                        <span className={chatTw.authProviderBtnInner}>
                          <span className={chatTw.authProviderBtnIcon}>
                            <img src={phoneIconSvg} alt="" aria-hidden="true" />
                          </span>
                          <span>继续使用手机登录</span>
                        </span>
                      </button>
                    </div>

                    <div className={chatTw.authDivider}>
                      <div className={chatTw.dividerLine} />
                      <div className={chatTw.authDividerText}>或</div>
                      <div className={chatTw.dividerLine} />
                    </div>

                    <div className={chatTw.authEmailBlock}>
                      <div
                        className={cx(
                          'flex h-[52px] items-center rounded-[99999px] border border-[rgba(0,0,0,0.15)] bg-white px-5 transition-[border-color,box-shadow] duration-[80ms] ease-in-out',
                          isAuthFocused && 'border-[rgba(0,0,0,0.15)] shadow-[0_0_0_1px_#5d5d5d]',
                          isAuthInvalid && 'border-[#d00e17] shadow-[inset_0_0_0_1px_#d00e17]',
                        )}
                      >
                        <input
                          ref={authInputRef}
                          className={chatTw.authEmailInput}
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
                        <div className={chatTw.authErrorRow} id={authErrorId}>
                          <span className={chatTw.authErrorIconWrap}>
                            <svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                              <use href={`${CHAT_ICON_SPRITE_URL}#chat-error-circle`} />
                            </svg>
                          </span>
                          <span>{authErrorText}</span>
                        </div>
                      ) : null}
                    </div>

                    <button type="submit" className={chatTw.authSubmitButton} disabled={isAuthSubmitting}>
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
