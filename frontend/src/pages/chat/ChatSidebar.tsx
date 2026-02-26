import type { RefObject } from 'react'
import { Link } from 'react-router-dom'

import onebookLogoMark from '@/assets/brand/onebook-logo-mark.svg'
import { CHAT_ICON_SPRITE_URL } from '@/pages/chat/shared'

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
  const avatarLetter = accountEmail.slice(0, 1).toUpperCase()

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
          'min-h-screen grid grid-rows-[auto_minmax(0,1fr)_auto] bg-[#f7f7f7] p-0 max-[767px]:fixed max-[767px]:bottom-0 max-[767px]:left-0 max-[767px]:top-0 max-[767px]:z-[39] max-[767px]:w-[min(82vw,300px)] max-[767px]:-translate-x-[104%] max-[767px]:shadow-[6px_0_30px_rgba(0,0,0,0.15)] max-[767px]:transition-transform max-[767px]:duration-180',
          isSidebarOpen && 'max-[767px]:translate-x-0',
          isDesktopSidebarCollapsed && 'md:hidden',
        )}
        aria-label="会话侧边栏"
      >
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
          <div className={sidebarTw.sidebarAccountCard}>
            <span className={sidebarTw.sidebarAvatar} aria-hidden="true">
              {avatarLetter}
            </span>
            <div className={sidebarTw.sidebarAccountMeta}>
              <span className={sidebarTw.sidebarAccountEmail}>{accountEmail}</span>
              <span className={sidebarTw.roleMuted}>{accountRoleLabel}</span>
            </div>
            <button type="button" className={sidebarTw.sidebarLogoutButton} onClick={onLogout}>
              退出
            </button>
          </div>
        </div>
      </aside>
    </>
  )
}
