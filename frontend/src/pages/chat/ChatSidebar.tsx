import type { ChangeEvent, RefObject } from 'react'
import { useEffect, useRef, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Link } from 'react-router-dom'

import onebookLogoMark from '@/assets/brand/onebook-logo-mark.svg'
import { getApiErrorMessage, updateMe, uploadMyAvatar } from '@/features/auth/api/auth'
import { useSessionStore } from '@/features/auth/store/session'
import { CHAT_ICON_SPRITE_URL } from '@/pages/chat/shared'
import { resolveApiAssetURL } from '@/shared/lib/http/assets'

const cx = (...values: Array<string | false | null | undefined>) =>
  values.filter(Boolean).join(' ')

const sidebarTw = {
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
  sidebarShell:
    'relative flex h-full min-h-0 flex-col bg-[#f9f9f9]',
  sidebarRail:
    'hidden h-full w-[52px] shrink-0 flex-col items-center bg-transparent pb-2 md:flex',
  sidebarRailHeader: 'flex h-[52px] items-center justify-center',
  sidebarRailSection: 'mt-[14px] grid gap-1',
  sidebarRailGrow: 'flex-1',
  sidebarRailButton:
    'inline-flex h-9 w-9 cursor-pointer items-center justify-center rounded-[10px] border-0 bg-transparent p-0 text-[#525252] hover:bg-[#ececec] focus-visible:bg-[#ececec] focus-visible:outline-none disabled:opacity-50',
  sidebarRailAvatar:
    'inline-flex h-8 w-8 cursor-pointer items-center justify-center overflow-hidden rounded-full border-0 bg-[#222] p-0 text-[12px] font-semibold text-white hover:opacity-90 disabled:opacity-50',
  sidebarContent:
    'flex h-full min-h-0 w-full flex-col overflow-x-clip overflow-y-auto bg-[#f9f9f9] text-clip whitespace-nowrap opacity-100 transition-opacity duration-150 ease-linear',
  sidebarHeader: 'sticky top-0 z-30 block bg-[#f9f9f9] px-2 pt-2',
  sidebarHeaderRow: 'flex items-center justify-between',
  sidebarHomeLink:
    'm-0 inline-flex h-9 w-9 flex-[0_0_36px] items-center justify-center rounded-[10px] p-0 leading-none hover:bg-[#ececec] focus-visible:bg-[#ececec]',
  sidebarHomeLogo: 'block h-9 w-9',
  sidebarCloseButton:
    'inline-flex h-9 w-9 shrink-0 cursor-[w-resize] items-center justify-center rounded-[10px] border-0 bg-transparent text-[#737373] hover:bg-[#ececec] focus-visible:bg-[#ececec] max-[767px]:h-10 max-[767px]:w-10',
  sidebarCloseDesktopIcon: 'block h-5 w-5 max-[767px]:hidden',
  sidebarCloseMobileIcon: 'hidden h-5 w-5 max-[767px]:block',
  sidebarScrollArea: 'grid min-h-0 flex-1 content-start gap-0 overflow-visible p-0',
  sidebarListAside: 'pt-0',
  sidebarMenuList: 'mt-[14px] grid gap-0 p-0',
  sidebarNewChatButton:
    'group/menu flex min-h-9 w-full cursor-pointer items-center justify-between gap-2 rounded-[10px] border-0 bg-transparent px-4 py-[6px] text-left tracking-[0] text-[#0d0d0d] hover:bg-[#ececec] focus:outline-none max-[767px]:min-h-10',
  sidebarNavButtonActive: 'bg-[#e7e7e7]',
  sidebarNewChatText:
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
  sidebarThreadButton:
    'group/thread relative block min-h-[34px] cursor-pointer rounded-[10px] border-0 bg-transparent px-[9px] pr-[30px] text-left text-[#0d0d0d] transition-colors duration-120 hover:bg-[#ececec] focus:outline-none',
  sidebarThreadButtonActive: '!bg-[#ececec] hover:!bg-[#ececec]',
  sidebarThreadTitle:
    'block overflow-hidden text-ellipsis whitespace-nowrap text-[14px] leading-[34px] font-normal',
  sidebarAccountPanel:
    'sticky bottom-0 z-30 bg-[#f9f9f9] p-2 pt-2 shadow-[0_-18px_26px_rgba(249,249,249,0.92)]',
  sidebarAccountCard:
    'group/profile grid min-h-11 w-full cursor-pointer grid-cols-[auto_minmax(0,1fr)] items-center gap-2 rounded-[10px] border-0 bg-transparent p-2 text-left hover:bg-[#ececec] focus-visible:bg-[#ececec] focus-visible:outline-none',
  sidebarAvatar:
    'inline-flex h-6 w-6 shrink-0 items-center justify-center overflow-hidden rounded-[9999px] border-0 bg-[#0d0d0d] p-0 text-[11px] font-semibold text-white',
  sidebarAvatarImg: 'h-full w-full object-cover',
  sidebarAccountMeta: 'grid min-w-0',
  sidebarAccountEmail:
    'overflow-hidden text-ellipsis whitespace-nowrap text-[13px] leading-5 text-[#171717]',
  sidebarProfileMenu:
    'absolute bottom-full left-0 right-0 mb-2 grid max-h-[min(520px,calc(100vh-96px))] min-w-[calc(100%-4px)] gap-0 overflow-auto rounded-[16px] border border-[rgba(0,0,0,0.10)] bg-white py-1.5 shadow-[0_16px_48px_rgba(0,0,0,0.16)]',
  sidebarProfileMenuHeader:
    'mx-1.5 grid min-h-11 grid-cols-[auto_minmax(0,1fr)] items-center gap-2 rounded-[10px] px-2 py-1.5 text-left hover:bg-[#f1f1f1]',
  sidebarProfileMenuAvatar:
    'inline-flex h-6 w-6 shrink-0 items-center justify-center overflow-hidden rounded-full bg-[#0d0d0d] text-[11px] font-semibold text-white',
  sidebarProfileMenuName: 'truncate text-[13px] font-medium leading-5 text-[#171717]',
  sidebarProfileMenuSub: 'truncate text-[12px] leading-4 text-[#777]',
  sidebarLogoutButton:
    'h-8 cursor-pointer rounded-[8px] border-0 bg-transparent px-2 text-[12px] text-[#474747] hover:bg-[#efefef] hover:text-[#0d0d0d]',
  sidebarMenuButton:
    'mx-1.5 grid min-h-9 w-[calc(100%-12px)] cursor-pointer grid-cols-[20px_minmax(0,1fr)_auto] items-center gap-2 rounded-[10px] border-0 bg-transparent px-2 text-left text-[13px] text-[#171717] hover:bg-[#f1f1f1] disabled:cursor-not-allowed disabled:opacity-55',
  sidebarMenuIcon:
    'inline-flex h-5 w-5 items-center justify-center text-[#5f5f5f]',
  sidebarMenuTrailing: 'inline-flex h-4 w-4 items-center justify-center text-[#858585]',
  sidebarMenuDanger:
    'text-[#a4161a] hover:bg-[#fff1f1]',
  sidebarMenuDivider: 'mx-4 my-1 h-px bg-[rgba(0,0,0,0.08)]',
  sidebarUploadError: 'px-1 text-[11px] leading-4 text-[#a4161a]',
  dialogOverlay:
    'fixed inset-0 z-[70] grid place-items-center bg-black/35 px-4 py-6',
  dialogPanel:
    'flex max-h-[min(720px,calc(100vh-32px))] w-full max-w-md flex-col overflow-hidden rounded-[16px] bg-white text-left shadow-[0_24px_80px_rgba(0,0,0,0.24)] focus:outline-none',
  dialogHeader: 'flex min-h-[52px] items-center justify-between px-4 py-2.5',
  dialogTitle: 'text-[18px] font-normal leading-7 text-[#171717]',
  dialogClose:
    'inline-flex h-9 w-9 cursor-pointer items-center justify-center rounded-[10px] border-0 bg-transparent text-[#6f6f6f] hover:bg-[#f1f1f1] focus-visible:bg-[#f1f1f1] focus-visible:outline-none',
  dialogBody: 'grow overflow-y-auto px-4 pb-4 pt-1',
  dialogForm: 'flex flex-col',
  dialogAvatarSection: 'flex flex-col items-center py-6',
  dialogAvatarButton:
    'relative -m-1 inline-flex rounded-full border-0 bg-transparent p-1 hover:bg-[#f1f1f1] focus-visible:bg-[#f1f1f1] focus-visible:outline-none disabled:opacity-60',
  dialogAvatar:
    'inline-flex h-32 w-32 items-center justify-center overflow-hidden rounded-full bg-[#0d0d0d] text-[42px] font-semibold text-white shadow-[0_0_0_1px_rgba(0,0,0,0.08)]',
  dialogAvatarBadge:
    'absolute bottom-2 right-2 flex h-7 w-7 items-center justify-center rounded-full bg-[#ececec] text-[#444] shadow-[0_0_0_1px_rgba(0,0,0,0.12)]',
  dialogField:
    'my-1 rounded-[6px] border border-[rgba(0,0,0,0.16)] px-3 py-2 shadow-none focus-within:border-[#171717] focus-within:ring-1 focus-within:ring-[#171717]',
  dialogLabel: 'block text-[12px] font-normal leading-4 text-[#171717]',
  dialogInput:
    'mt-1 block w-full border-0 bg-transparent p-0 text-[14px] leading-5 text-[#171717] outline-none placeholder:text-[#8a8a8a] disabled:text-[#737373]',
  dialogHint: 'mt-2 text-center text-[12px] leading-4 text-[#777]',
  dialogError: 'mt-3 rounded-[10px] border border-[#f4b0b4] bg-[#fff5f6] px-3 py-2 text-[12px] text-[#9f1820]',
  dialogActions: 'mt-5 flex justify-end gap-2',
  dialogButton:
    'inline-flex h-9 cursor-pointer items-center justify-center rounded-[10px] border border-[rgba(0,0,0,0.12)] px-4 text-[14px] hover:bg-[#f4f4f4] disabled:cursor-not-allowed disabled:opacity-55',
  dialogPrimary:
    'border-[#171717] bg-[#171717] text-white hover:bg-[#2f2f2f]',
} as const

export type SidebarThreadItem = {
  id: string
  title: string
  preview?: string
  active?: boolean
}

type ChatSidebarProps = {
  isSidebarOpen: boolean
  isDesktopSidebarCollapsed: boolean
  isSidebarExpanded: boolean
  onCloseSidebar: () => void
  onOpenSidebar: () => void
  onMaskClick: () => void
  searchInputId: string
  searchInputRef: RefObject<HTMLInputElement | null>
  searchValue: string
  onSearchChange: (value: string) => void
  isHistoryExpanded: boolean
  onToggleHistoryExpanded: () => void
  threads: SidebarThreadItem[]
  onThreadClick: (threadID: string) => void
  onNewChatClick: () => void
  onLibraryClick: () => void
  isLibraryActive?: boolean
  newChatShortcutKeys: string[]
  searchShortcutKeys: string[]
  libraryShortcutKeys: string[]
  getShortcutAriaLabel: (key: string) => string | undefined
  accountEmail: string
  accountRoleLabel: string
  onLogout: () => void
  closeButtonTestId?: string
  scrollAreaRole?: string
  scrollAreaLabel?: string
  libraryButtonRef?: RefObject<HTMLButtonElement | null>
}

export function ChatSidebar({
  isSidebarOpen,
  isDesktopSidebarCollapsed,
  isSidebarExpanded,
  onCloseSidebar,
  onOpenSidebar,
  onMaskClick,
  searchInputId,
  searchInputRef,
  searchValue,
  onSearchChange,
  isHistoryExpanded,
  onToggleHistoryExpanded,
  threads,
  onThreadClick,
  onNewChatClick,
  onLibraryClick,
  isLibraryActive = false,
  newChatShortcutKeys,
  searchShortcutKeys,
  libraryShortcutKeys,
  getShortcutAriaLabel,
  accountEmail,
  accountRoleLabel,
  onLogout,
  closeButtonTestId,
  scrollAreaRole,
  scrollAreaLabel,
  libraryButtonRef,
}: ChatSidebarProps) {
  const sessionUser = useSessionStore((state) => state.user)
  const setSession = useSessionStore((state) => state.setSession)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const profileMenuRef = useRef<HTMLDivElement>(null)
  const [previewUrl, setPreviewUrl] = useState('')
  const [uploadError, setUploadError] = useState('')
  const [displayNameDraft, setDisplayNameDraft] = useState(sessionUser?.displayName ?? '')
  const [isProfileMenuOpen, setIsProfileMenuOpen] = useState(false)
  const [isProfileDialogOpen, setIsProfileDialogOpen] = useState(false)
  const displayName = sessionUser?.displayName?.trim() || accountEmail
  const avatarLetter = (displayName || accountEmail || 'U').slice(0, 1).toUpperCase()
  const avatarUrl = previewUrl || resolveApiAssetURL(sessionUser?.avatarUrl)
  const showCollapsedRail = isDesktopSidebarCollapsed && !isSidebarOpen

  useEffect(() => {
    return () => {
      if (previewUrl) URL.revokeObjectURL(previewUrl)
    }
  }, [previewUrl])

  useEffect(() => {
    if (!isProfileMenuOpen) return undefined

    function handlePointerDown(event: MouseEvent) {
      if (!profileMenuRef.current?.contains(event.target as Node)) {
        setIsProfileMenuOpen(false)
      }
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setIsProfileMenuOpen(false)
      }
    }

    document.addEventListener('mousedown', handlePointerDown)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handlePointerDown)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isProfileMenuOpen])

  useEffect(() => {
    if (!isProfileDialogOpen) return undefined

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setIsProfileDialogOpen(false)
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isProfileDialogOpen])

  const avatarMutation = useMutation({
    mutationFn: uploadMyAvatar,
    onSuccess: (user) => {
      setUploadError('')
      setSession({ user })
    },
    onError: (error) => {
      setUploadError(getApiErrorMessage(error, '头像上传失败，请稍后重试。'))
      setPreviewUrl('')
    },
  })

  const profileMutation = useMutation({
    mutationFn: updateMe,
    onSuccess: (user) => {
      setUploadError('')
      setSession({ user })
      setIsProfileDialogOpen(false)
    },
    onError: (error) => {
      setUploadError(getApiErrorMessage(error, '资料更新失败，请稍后重试。'))
    },
  })

  function handleAvatarChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    if (previewUrl) URL.revokeObjectURL(previewUrl)
    setPreviewUrl(URL.createObjectURL(file))
    void avatarMutation.mutateAsync(file)
  }

  function openProfileDialog() {
    setDisplayNameDraft(sessionUser?.displayName ?? '')
    setUploadError('')
    setIsProfileMenuOpen(false)
    setIsProfileDialogOpen(true)
  }

  return (
    <>
      <button
        type="button"
        className={cx(
          'fixed inset-0 z-[38] border-0 bg-[rgba(0,0,0,0.42)] transition-opacity duration-180',
          isSidebarOpen ? 'pointer-events-auto opacity-100' : 'pointer-events-none opacity-0',
        )}
        aria-hidden={!isSidebarOpen}
        tabIndex={-1}
        onClick={onMaskClick}
      />

      <aside
        id="stage-slideover-sidebar"
        className={cx(
          'relative z-[21] h-full shrink-0 overflow-hidden border-r border-[rgba(0,0,0,0.10)] bg-[#f9f9f9] p-0 print:hidden max-[767px]:fixed max-[767px]:bottom-0 max-[767px]:left-0 max-[767px]:top-0 max-[767px]:z-[39] max-[767px]:w-[min(82vw,300px)] max-[767px]:-translate-x-[104%] max-[767px]:shadow-[6px_0_30px_rgba(0,0,0,0.15)] max-[767px]:transition-transform max-[767px]:duration-180',
          isSidebarOpen && 'max-[767px]:translate-x-0',
        )}
        aria-label="会话侧边栏"
      >
        <div className={sidebarTw.sidebarShell}>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif"
            className="hidden"
            onChange={handleAvatarChange}
          />
          {showCollapsedRail ? (
            <div className={sidebarTw.sidebarRail} aria-label="折叠会话侧边栏">
              <div className={sidebarTw.sidebarRailHeader}>
                <button
                  type="button"
                  className={sidebarTw.sidebarRailButton}
                  aria-label="打开边栏"
                  aria-expanded={false}
                  aria-controls="stage-slideover-sidebar"
                  onClick={onOpenSidebar}
                >
                  <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={sidebarTw.iconBlockH5W5}>
                    <use href={`${CHAT_ICON_SPRITE_URL}#chat-sidebar-close-desktop`} fill="currentColor" />
                  </svg>
                </button>
              </div>
              <div className={sidebarTw.sidebarRailSection}>
                <button type="button" className={sidebarTw.sidebarRailButton} aria-label="新聊天" onClick={onNewChatClick}>
                  <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={sidebarTw.iconBlockH5W5}>
                    <use href={`${CHAT_ICON_SPRITE_URL}#chat-new-chat`} fill="currentColor" />
                  </svg>
                </button>
                <button type="button" className={sidebarTw.sidebarRailButton} aria-label="搜索聊天" onClick={onOpenSidebar}>
                  <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={sidebarTw.iconBlockH5W5}>
                    <use href={`${CHAT_ICON_SPRITE_URL}#chat-search`} fill="currentColor" />
                  </svg>
                </button>
                <button type="button" className={sidebarTw.sidebarRailButton} aria-label="书库管理" onClick={onLibraryClick}>
                  <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={sidebarTw.iconBlockH5W5}>
                    <use href={`${CHAT_ICON_SPRITE_URL}#chat-library`} fill="currentColor" />
                  </svg>
                </button>
              </div>
              <div className={sidebarTw.sidebarRailGrow} />
              <button
                type="button"
                className={sidebarTw.sidebarRailAvatar}
                aria-label="上传头像"
                disabled={avatarMutation.isPending}
                onClick={() => fileInputRef.current?.click()}
              >
                {avatarUrl ? <img src={avatarUrl} alt="" className={sidebarTw.sidebarAvatarImg} /> : avatarLetter}
              </button>
            </div>
          ) : (
            <div className={sidebarTw.sidebarContent}>
              <div className={sidebarTw.sidebarHeader}>
          <div className={sidebarTw.sidebarHeaderRow}>
            <Link to="/chat" className={sidebarTw.sidebarHomeLink} aria-label="OneBook AI">
              <img src={onebookLogoMark} alt="" aria-hidden="true" className={sidebarTw.sidebarHomeLogo} />
            </Link>
            <button
              type="button"
              className={sidebarTw.sidebarCloseButton}
              aria-expanded={isSidebarExpanded}
              aria-controls="stage-slideover-sidebar"
              aria-label="关闭边栏"
              data-testid={closeButtonTestId}
              data-state={isSidebarExpanded ? 'open' : 'closed'}
              onClick={onCloseSidebar}
            >
              <svg
                viewBox="0 0 20 20"
                xmlns="http://www.w3.org/2000/svg"
                aria-hidden="true"
                data-rtl-flip=""
                className={sidebarTw.sidebarCloseDesktopIcon}
              >
                <use href={`${CHAT_ICON_SPRITE_URL}#chat-sidebar-close-desktop`} fill="currentColor" />
              </svg>
              <svg
                viewBox="0 0 20 20"
                xmlns="http://www.w3.org/2000/svg"
                aria-hidden="true"
                className={sidebarTw.sidebarCloseMobileIcon}
              >
                <use href={`${CHAT_ICON_SPRITE_URL}#chat-sidebar-close-mobile`} fill="currentColor" />
              </svg>
            </button>
          </div>
        </div>

              <div className={sidebarTw.sidebarScrollArea} role={scrollAreaRole} aria-label={scrollAreaLabel}>
          <aside className={sidebarTw.sidebarListAside}>
            <div className={sidebarTw.sidebarMenuList}>
              <button type="button" className={sidebarTw.sidebarNewChatButton} onClick={onNewChatClick}>
                <span className={sidebarTw.menuMainIconWrap}>
                  <span className={sidebarTw.menuMainIcon} aria-hidden="true">
                    <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" className={sidebarTw.iconBlockH5W5}>
                      <use href={`${CHAT_ICON_SPRITE_URL}#chat-new-chat`} fill="currentColor" />
                    </svg>
                  </span>
                  <span className={sidebarTw.menuMainTextWrap}>
                    <span className={sidebarTw.sidebarNewChatText}>新聊天</span>
                  </span>
                </span>
                <span className={sidebarTw.shortcutLabel} aria-hidden="true">
                  <span className={sidebarTw.shortcutRow}>
                    {newChatShortcutKeys.map((key) => (
                      <kbd key={`new-chat-shortcut-${key}`} aria-label={getShortcutAriaLabel(key)} className={sidebarTw.shortcutKeyWrap}>
                        <span className={sidebarTw.shortcutKey}>{key}</span>
                      </kbd>
                    ))}
                  </span>
                </span>
              </button>

              <label className={sidebarTw.sidebarSearchLabel} htmlFor={searchInputId}>
                <span className={sidebarTw.menuMainIconWrap}>
                  <span className={sidebarTw.menuMainIcon} aria-hidden="true">
                    <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" className={sidebarTw.iconBlockH5W5}>
                      <use href={`${CHAT_ICON_SPRITE_URL}#chat-search`} fill="currentColor" />
                    </svg>
                  </span>
                  <span className={sidebarTw.menuMainTextWrap}>
                    <input
                      id={searchInputId}
                      ref={searchInputRef}
                      className={sidebarTw.sidebarSearchInput}
                      type="search"
                      placeholder="搜索聊天"
                      value={searchValue}
                      onChange={(event) => onSearchChange(event.target.value)}
                    />
                  </span>
                </span>
                <span className={sidebarTw.shortcutLabel} aria-hidden="true">
                  <span className={sidebarTw.shortcutRow}>
                    {searchShortcutKeys.map((key) => (
                      <kbd key={`search-shortcut-${key}`} aria-label={getShortcutAriaLabel(key)} className={sidebarTw.shortcutKeyWrap}>
                        <span className={sidebarTw.shortcutKey}>{key}</span>
                      </kbd>
                    ))}
                  </span>
                </span>
              </label>

              <button
                ref={libraryButtonRef}
                type="button"
                className={cx(
                  sidebarTw.sidebarNewChatButton,
                  isLibraryActive && sidebarTw.sidebarNavButtonActive,
                )}
                aria-label="书库管理"
                aria-current={isLibraryActive ? 'page' : undefined}
                onClick={onLibraryClick}
              >
                <span className={sidebarTw.menuMainIconWrap}>
                  <span className={sidebarTw.menuMainIcon} aria-hidden="true">
                    <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" className={sidebarTw.iconBlockH5W5}>
                      <use href={`${CHAT_ICON_SPRITE_URL}#chat-library`} fill="currentColor" />
                    </svg>
                  </span>
                  <span className={sidebarTw.menuMainTextWrap}>
                    <span className={sidebarTw.sidebarNewChatText}>书库管理</span>
                  </span>
                </span>
                <span className={sidebarTw.shortcutLabel} aria-hidden="true">
                  <span className={sidebarTw.shortcutRow}>
                    {libraryShortcutKeys.map((key) => (
                      <kbd key={`library-shortcut-${key}`} aria-label={getShortcutAriaLabel(key)} className={sidebarTw.shortcutKeyWrap}>
                        <span className={sidebarTw.shortcutKey}>{key}</span>
                      </kbd>
                    ))}
                  </span>
                </span>
              </button>
            </div>
          </aside>
          <div className={sidebarTw.sidebarMenuSpacer} aria-hidden="true" />

          <div className={cx('group/expando mb-[6px]', isHistoryExpanded && 'mb-2')}>
            <button
              type="button"
              aria-expanded={isHistoryExpanded}
              className={sidebarTw.sidebarHistoryToggle}
              onClick={onToggleHistoryExpanded}
            >
              <h2 className={sidebarTw.sidebarHistoryTitle}>你的聊天</h2>
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
            <div className={sidebarTw.sidebarThreadList}>
              {threads.map((thread) => (
                <button
                  key={thread.id}
                  type="button"
                  className={cx(
                    sidebarTw.sidebarThreadButton,
                    thread.active && sidebarTw.sidebarThreadButtonActive,
                  )}
                  aria-current={thread.active ? 'true' : undefined}
                  onClick={() => onThreadClick(thread.id)}
                  title={thread.preview || undefined}
                >
                  <span className={sidebarTw.sidebarThreadTitle}>{thread.title}</span>
                  <span
                    className={cx(
                      'pointer-events-none absolute right-[6px] top-1/2 inline-flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded-[6px] text-[#8c8c8c] opacity-0 transition-opacity duration-140 group-hover/thread:opacity-100',
                      thread.active && 'opacity-100',
                    )}
                    aria-hidden="true"
                  >
                    <svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg" className={sidebarTw.iconBlockH14W14}>
                      <path d="M5 10a1.2 1.2 0 1 0 0-2.4A1.2 1.2 0 0 0 5 10Zm5 0a1.2 1.2 0 1 0 0-2.4A1.2 1.2 0 0 0 10 10Zm5 0a1.2 1.2 0 1 0 0-2.4A1.2 1.2 0 0 0 15 10Z" fill="currentColor" />
                    </svg>
                  </span>
                </button>
              ))}
            </div>
          ) : null}
        </div>

              <div className={sidebarTw.sidebarAccountPanel}>
                <div ref={profileMenuRef} className="relative">
                  {isProfileMenuOpen ? (
                    <div className={sidebarTw.sidebarProfileMenu} role="menu">
                      <div className={sidebarTw.sidebarProfileMenuHeader}>
                        <span className={sidebarTw.sidebarProfileMenuAvatar} aria-hidden="true">
                          {avatarUrl ? <img src={avatarUrl} alt="" className={sidebarTw.sidebarAvatarImg} /> : avatarLetter}
                        </span>
                        <span className={sidebarTw.sidebarAccountMeta}>
                          <span className={sidebarTw.sidebarProfileMenuName}>{displayName}</span>
                          <span className={sidebarTw.sidebarProfileMenuSub}>{accountEmail || accountRoleLabel}</span>
                        </span>
                      </div>
                      <div className={sidebarTw.sidebarMenuDivider} />
                      <button
                        type="button"
                        className={sidebarTw.sidebarMenuButton}
                        disabled={avatarMutation.isPending}
                        onClick={() => fileInputRef.current?.click()}
                      >
                        <span className={sidebarTw.sidebarMenuIcon} aria-hidden="true">
                          <svg viewBox="0 0 20 20" className="h-5 w-5">
                            <path d="M4.5 6.5h2l1-1.5h5l1 1.5h2a1.5 1.5 0 0 1 1.5 1.5v5.5A1.5 1.5 0 0 1 15.5 15h-11A1.5 1.5 0 0 1 3 13.5V8a1.5 1.5 0 0 1 1.5-1.5Z" fill="none" stroke="currentColor" strokeWidth="1.5" />
                            <circle cx="10" cy="10.8" r="2.3" fill="none" stroke="currentColor" strokeWidth="1.5" />
                          </svg>
                        </span>
                        <span>更换头像</span>
                      </button>
                      {uploadError ? <div className={sidebarTw.sidebarUploadError}>{uploadError}</div> : null}
                      <button
                        type="button"
                        className={sidebarTw.sidebarMenuButton}
                        onClick={openProfileDialog}
                      >
                        <span className={sidebarTw.sidebarMenuIcon} aria-hidden="true">
                          <svg viewBox="0 0 20 20" className="h-5 w-5">
                            <path d="M10 10.5a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z" fill="none" stroke="currentColor" strokeWidth="1.5" />
                            <path d="M4.5 16c.8-2.4 2.7-3.7 5.5-3.7s4.7 1.3 5.5 3.7" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                          </svg>
                        </span>
                        <span>个人资料</span>
                      </button>
                      <div className={sidebarTw.sidebarMenuDivider} />
                      <button
                        type="button"
                        className={cx(sidebarTw.sidebarMenuButton, sidebarTw.sidebarMenuDanger)}
                        onClick={onLogout}
                      >
                        <span className={sidebarTw.sidebarMenuIcon} aria-hidden="true">
                          <svg viewBox="0 0 20 20" className="h-5 w-5">
                            <path d="M8.5 4.5h-2A1.5 1.5 0 0 0 5 6v8a1.5 1.5 0 0 0 1.5 1.5h2" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                            <path d="M10.5 10h6M14 7l3 3-3 3" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                          </svg>
                        </span>
                        <span>退出登录</span>
                      </button>
                    </div>
                  ) : null}
                  <button
                    type="button"
                    className={sidebarTw.sidebarAccountCard}
                    aria-label={`${displayName}，打开个人资料菜单`}
                    aria-haspopup="menu"
                    aria-expanded={isProfileMenuOpen}
                    data-state={isProfileMenuOpen ? 'open' : 'closed'}
                    onClick={() => setIsProfileMenuOpen((prev) => !prev)}
                  >
                    <span className={sidebarTw.sidebarAvatar} aria-hidden="true">
                      {avatarUrl ? <img src={avatarUrl} alt="" className={sidebarTw.sidebarAvatarImg} /> : avatarLetter}
                    </span>
                    <span className={sidebarTw.sidebarAccountMeta}>
                      <span className={sidebarTw.sidebarAccountEmail} title={displayName}>{displayName}</span>
                      <span className={sidebarTw.roleMuted}>{accountRoleLabel}</span>
                    </span>
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      </aside>

      {isProfileDialogOpen ? (
        <div
          className={sidebarTw.dialogOverlay}
          role="presentation"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) {
              setIsProfileDialogOpen(false)
            }
          }}
        >
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="profile-dialog-title"
            className={sidebarTw.dialogPanel}
            tabIndex={-1}
          >
            <header className={sidebarTw.dialogHeader}>
              <h2 id="profile-dialog-title" className={sidebarTw.dialogTitle}>编辑个人资料</h2>
              <button
                type="button"
                className={sidebarTw.dialogClose}
                aria-label="关闭"
                onClick={() => setIsProfileDialogOpen(false)}
              >
                <svg viewBox="0 0 20 20" aria-hidden="true" className="h-5 w-5">
                  <path d="M5.5 5.5 14.5 14.5M14.5 5.5 5.5 14.5" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
                </svg>
              </button>
            </header>
            <div className={sidebarTw.dialogBody}>
              <form
                className={sidebarTw.dialogForm}
                onSubmit={(event) => {
                  event.preventDefault()
                  void profileMutation.mutateAsync({ displayName: displayNameDraft })
                }}
              >
                <div className={sidebarTw.dialogAvatarSection}>
                  <button
                    type="button"
                    className={sidebarTw.dialogAvatarButton}
                    aria-label="更新个人资料照片"
                    aria-busy={avatarMutation.isPending}
                    disabled={avatarMutation.isPending}
                    onClick={() => fileInputRef.current?.click()}
                  >
                    <span className={sidebarTw.dialogAvatar}>
                      {avatarUrl ? <img src={avatarUrl} alt={displayName} className={sidebarTw.sidebarAvatarImg} /> : avatarLetter}
                    </span>
                    <span className={sidebarTw.dialogAvatarBadge} aria-hidden="true">
                      <svg viewBox="0 0 20 20" className="h-[18px] w-[18px]">
                        <path d="M4.5 6.5h2l1-1.5h5l1 1.5h2a1.5 1.5 0 0 1 1.5 1.5v5.5A1.5 1.5 0 0 1 15.5 15h-11A1.5 1.5 0 0 1 3 13.5V8a1.5 1.5 0 0 1 1.5-1.5Z" fill="none" stroke="currentColor" strokeWidth="1.5" />
                        <circle cx="10" cy="10.8" r="2.3" fill="none" stroke="currentColor" strokeWidth="1.5" />
                      </svg>
                    </span>
                  </button>
                </div>
                <label className={sidebarTw.dialogField}>
                  <span className={sidebarTw.dialogLabel}>显示名称</span>
                  <input
                    className={sidebarTw.dialogInput}
                    value={displayNameDraft}
                    maxLength={80}
                    placeholder={displayName}
                    autoComplete="off"
                    onChange={(event) => setDisplayNameDraft(event.target.value)}
                  />
                </label>
                <label className={sidebarTw.dialogField}>
                  <span className={sidebarTw.dialogLabel}>账号</span>
                  <input
                    className={sidebarTw.dialogInput}
                    value={accountEmail || '未绑定邮箱'}
                    disabled
                    readOnly
                  />
                </label>
                <p className={sidebarTw.dialogHint}>个人资料有助于你在 OneBook AI 中识别当前账号。</p>
                {uploadError ? <div className={sidebarTw.dialogError}>{uploadError}</div> : null}
                <div className={sidebarTw.dialogActions}>
                  <button
                    type="button"
                    className={sidebarTw.dialogButton}
                    onClick={() => setIsProfileDialogOpen(false)}
                  >
                    取消
                  </button>
                  <button
                    type="submit"
                    className={cx(sidebarTw.dialogButton, sidebarTw.dialogPrimary)}
                    disabled={profileMutation.isPending}
                  >
                    保存
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      ) : null}
    </>
  )
}
