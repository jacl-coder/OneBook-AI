import { useCallback, useEffect, useRef, useState, type ChangeEvent } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Link, NavLink, Outlet, useNavigate } from 'react-router-dom'

import { getApiErrorMessage, logout, updateMe, uploadMyAvatar } from '@/features/auth/api/auth'
import { useSessionStore } from '@/features/auth/store/session'
import { resolveApiAssetURL } from '@/shared/lib/http/assets'

const uiSansStyle = {
  fontFamily:
    "ui-sans-serif, -apple-system, system-ui, 'Segoe UI', Helvetica, 'Apple Color Emoji', Arial, sans-serif, 'Segoe UI Emoji', 'Segoe UI Symbol'",
} as const

const cx = (...values: Array<string | false | null | undefined>) =>
  values.filter(Boolean).join(' ')

const layoutTw = {
  shell: 'grid h-screen overflow-hidden bg-white text-[#0d0d0d] md:grid-cols-[240px_minmax(0,1fr)]',
  side: 'grid h-full min-h-0 grid-rows-[auto_minmax(0,1fr)_auto] border-r border-[rgba(0,0,0,0.08)] bg-[#f7f7f7] p-3',
  brand: 'inline-flex h-9 items-center rounded-[10px] px-2 text-[15px] font-semibold text-[#0d0d0d]',
  navScroll: 'min-h-0 overflow-auto',
  nav: 'mt-3 grid gap-1',
  navLink:
    'inline-flex h-9 items-center rounded-[10px] px-3 text-[14px] text-[#232323] hover:bg-[#ececec] aria-[current=page]:bg-[#e7e7e7]',
  accountPanel: 'relative bg-[#f7f7f7] pt-2',
  accountCard:
    'group/profile grid min-h-11 w-full cursor-pointer grid-cols-[auto_minmax(0,1fr)] items-center gap-2 rounded-[10px] border-0 bg-transparent p-2 text-left hover:bg-[#ececec] focus-visible:bg-[#ececec] focus-visible:outline-none',
  avatar:
    'inline-flex h-6 w-6 shrink-0 items-center justify-center overflow-hidden rounded-full border-0 bg-[#222] p-0 text-[11px] font-semibold text-white',
  avatarImg: 'h-full w-full object-cover',
  accountMeta: 'grid min-w-0',
  accountName: 'truncate text-[13px] leading-5 text-[#171717]',
  roleMuted: 'text-[11px] leading-[14px] text-[#6f6f6f]',
  notice: 'text-[11px] leading-4 text-[#a4161a]',
  profileMenu:
    'absolute bottom-full left-0 right-0 z-40 mb-2 grid max-h-[min(520px,calc(100vh-96px))] gap-0 overflow-auto rounded-[16px] border border-[rgba(0,0,0,0.10)] bg-white py-1.5 shadow-[0_16px_48px_rgba(0,0,0,0.16)]',
  profileMenuHeader:
    'mx-1.5 grid min-h-11 grid-cols-[auto_minmax(0,1fr)] items-center gap-2 rounded-[10px] px-2 py-1.5 text-left hover:bg-[#f1f1f1]',
  profileMenuAvatar:
    'inline-flex h-6 w-6 shrink-0 items-center justify-center overflow-hidden rounded-full bg-[#222] text-[11px] font-semibold text-white',
  profileMenuName: 'truncate text-[13px] font-medium leading-5 text-[#171717]',
  profileMenuSub: 'truncate text-[12px] leading-4 text-[#777]',
  menuButton:
    'mx-1.5 grid min-h-9 w-[calc(100%-12px)] cursor-pointer grid-cols-[20px_minmax(0,1fr)_auto] items-center gap-2 rounded-[10px] border-0 bg-transparent px-2 text-left text-[13px] text-[#171717] hover:bg-[#f1f1f1] disabled:cursor-not-allowed disabled:opacity-55',
  menuIcon: 'inline-flex h-5 w-5 items-center justify-center text-[#5f5f5f]',
  menuTrailing: 'inline-flex h-4 w-4 items-center justify-center text-[#858585]',
  menuDanger: 'text-[#a4161a] hover:bg-[#fff1f1]',
  menuDivider: 'mx-4 my-1 h-px bg-[rgba(0,0,0,0.08)]',
  main: 'grid h-full min-h-0 grid-rows-[52px_minmax(0,1fr)]',
  topBar: 'flex h-[52px] items-center justify-between border-b border-[rgba(0,0,0,0.08)] bg-white px-4',
  topTitle: 'text-[16px] font-medium',
  body: 'min-h-0 overflow-auto bg-white px-4 py-4 max-[767px]:px-3',
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
    'inline-flex h-32 w-32 items-center justify-center overflow-hidden rounded-full bg-[#222] text-[42px] font-semibold text-white shadow-[0_0_0_1px_rgba(0,0,0,0.08)]',
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
  dialogPrimary: 'border-[#171717] bg-[#171717] text-white hover:bg-[#2f2f2f]',
}

export function AdminLayout() {
  const navigate = useNavigate()
  const sessionUser = useSessionStore((state) => state.user)
  const setSession = useSessionStore((state) => state.setSession)
  const clearSession = useSessionStore((state) => state.clearSession)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const profileMenuRef = useRef<HTMLDivElement>(null)
  const [errorText, setErrorText] = useState('')
  const [displayNameDraft, setDisplayNameDraft] = useState(sessionUser?.displayName ?? '')
  const [isProfileMenuOpen, setIsProfileMenuOpen] = useState(false)
  const [isProfileDialogOpen, setIsProfileDialogOpen] = useState(false)
  const displayName = sessionUser?.displayName?.trim() || sessionUser?.email || '管理员'
  const avatarLetter = displayName.slice(0, 1).toUpperCase()
  const avatarUrl = resolveApiAssetURL(sessionUser?.avatarUrl)

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
      setErrorText('')
      setSession({ user })
    },
    onError: (error) => {
      setErrorText(getApiErrorMessage(error, '头像上传失败。'))
    },
  })

  const profileMutation = useMutation({
    mutationFn: updateMe,
    onSuccess: (user) => {
      setErrorText('')
      setSession({ user })
      setIsProfileDialogOpen(false)
    },
    onError: (error) => {
      setErrorText(getApiErrorMessage(error, '资料更新失败。'))
    },
  })

  function handleAvatarChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    void avatarMutation.mutateAsync(file)
  }

  function openProfileDialog() {
    setDisplayNameDraft(sessionUser?.displayName ?? '')
    setErrorText('')
    setIsProfileMenuOpen(false)
    setIsProfileDialogOpen(true)
  }

  const handleLogout = useCallback(async () => {
    try {
      await logout()
    } catch {
      // Keep UX responsive even if network logout fails.
    } finally {
      clearSession()
      navigate('/chat')
    }
  }, [clearSession, navigate])

  return (
    <div className={layoutTw.shell} style={uiSansStyle}>
      <aside className={layoutTw.side}>
        <Link to="/admin/overview" className={layoutTw.brand}>
          OneBook Admin
        </Link>

        <div className={layoutTw.navScroll}>
          <nav className={layoutTw.nav}>
            <NavLink className={layoutTw.navLink} to="/admin/overview">
              概览
            </NavLink>
            <NavLink className={layoutTw.navLink} to="/admin/users">
              用户管理
            </NavLink>
            <NavLink className={layoutTw.navLink} to="/admin/books">
              书籍管理
            </NavLink>
            <NavLink className={layoutTw.navLink} to="/admin/evals">
              评测中心
            </NavLink>
            <NavLink className={layoutTw.navLink} to="/admin/audit">
              审计日志
            </NavLink>
          </nav>
        </div>

        <div className={layoutTw.accountPanel}>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif"
            className="hidden"
            onChange={handleAvatarChange}
          />
          <div ref={profileMenuRef} className="relative">
            {isProfileMenuOpen ? (
              <div className={layoutTw.profileMenu} role="menu">
                <div className={layoutTw.profileMenuHeader}>
                  <span className={layoutTw.profileMenuAvatar} aria-hidden="true">
                    {avatarUrl ? <img src={avatarUrl} alt="" className={layoutTw.avatarImg} /> : avatarLetter}
                  </span>
                  <span className={layoutTw.accountMeta}>
                    <span className={layoutTw.profileMenuName}>{displayName}</span>
                    <span className={layoutTw.profileMenuSub}>{sessionUser?.email || '管理员'}</span>
                  </span>
                </div>
                <div className={layoutTw.menuDivider} />
                <button
                  type="button"
                  className={layoutTw.menuButton}
                  disabled={avatarMutation.isPending}
                  onClick={() => fileInputRef.current?.click()}
                >
                  <span className={layoutTw.menuIcon} aria-hidden="true">
                    <svg viewBox="0 0 20 20" className="h-5 w-5">
                      <path d="M4.5 6.5h2l1-1.5h5l1 1.5h2a1.5 1.5 0 0 1 1.5 1.5v5.5A1.5 1.5 0 0 1 15.5 15h-11A1.5 1.5 0 0 1 3 13.5V8a1.5 1.5 0 0 1 1.5-1.5Z" fill="none" stroke="currentColor" strokeWidth="1.5" />
                      <circle cx="10" cy="10.8" r="2.3" fill="none" stroke="currentColor" strokeWidth="1.5" />
                    </svg>
                  </span>
                  <span>更换头像</span>
                </button>
                {errorText ? <div className={layoutTw.notice}>{errorText}</div> : null}
                <button type="button" className={layoutTw.menuButton} onClick={openProfileDialog}>
                  <span className={layoutTw.menuIcon} aria-hidden="true">
                    <svg viewBox="0 0 20 20" className="h-5 w-5">
                      <path d="M10 10.5a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z" fill="none" stroke="currentColor" strokeWidth="1.5" />
                      <path d="M4.5 16c.8-2.4 2.7-3.7 5.5-3.7s4.7 1.3 5.5 3.7" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                    </svg>
                  </span>
                  <span>个人资料</span>
                </button>
                <div className={layoutTw.menuDivider} />
                <button
                  type="button"
                  className={cx(layoutTw.menuButton, layoutTw.menuDanger)}
                  onClick={() => void handleLogout()}
                >
                  <span className={layoutTw.menuIcon} aria-hidden="true">
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
              className={layoutTw.accountCard}
              aria-label={`${displayName}，打开个人资料菜单`}
              aria-haspopup="menu"
              aria-expanded={isProfileMenuOpen}
              data-state={isProfileMenuOpen ? 'open' : 'closed'}
              onClick={() => setIsProfileMenuOpen((prev) => !prev)}
            >
              <span className={layoutTw.avatar} aria-hidden="true">
                {avatarUrl ? <img src={avatarUrl} alt="" className={layoutTw.avatarImg} /> : avatarLetter}
              </span>
              <span className={layoutTw.accountMeta}>
                <span className={layoutTw.accountName} title={displayName}>{displayName}</span>
                <span className={layoutTw.roleMuted}>管理员</span>
              </span>
            </button>
          </div>
        </div>
      </aside>

      <main className={layoutTw.main}>
        <header className={layoutTw.topBar}>
          <h1 className={layoutTw.topTitle}>后台管理</h1>
        </header>

        <section className={layoutTw.body}>
          <Outlet />
        </section>
      </main>
      {isProfileDialogOpen ? (
        <div
          className={layoutTw.dialogOverlay}
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
            aria-labelledby="admin-profile-dialog-title"
            className={layoutTw.dialogPanel}
            tabIndex={-1}
          >
            <header className={layoutTw.dialogHeader}>
              <h2 id="admin-profile-dialog-title" className={layoutTw.dialogTitle}>编辑个人资料</h2>
              <button
                type="button"
                className={layoutTw.dialogClose}
                aria-label="关闭"
                onClick={() => setIsProfileDialogOpen(false)}
              >
                <svg viewBox="0 0 20 20" aria-hidden="true" className="h-5 w-5">
                  <path d="M5.5 5.5 14.5 14.5M14.5 5.5 5.5 14.5" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
                </svg>
              </button>
            </header>
            <div className={layoutTw.dialogBody}>
              <form
                className={layoutTw.dialogForm}
                onSubmit={(event) => {
                  event.preventDefault()
                  void profileMutation.mutateAsync({ displayName: displayNameDraft })
                }}
              >
                <div className={layoutTw.dialogAvatarSection}>
                  <button
                    type="button"
                    className={layoutTw.dialogAvatarButton}
                    aria-label="更新个人资料照片"
                    aria-busy={avatarMutation.isPending}
                    disabled={avatarMutation.isPending}
                    onClick={() => fileInputRef.current?.click()}
                  >
                    <span className={layoutTw.dialogAvatar}>
                      {avatarUrl ? <img src={avatarUrl} alt={displayName} className={layoutTw.avatarImg} /> : avatarLetter}
                    </span>
                    <span className={layoutTw.dialogAvatarBadge} aria-hidden="true">
                      <svg viewBox="0 0 20 20" className="h-[18px] w-[18px]">
                        <path d="M4.5 6.5h2l1-1.5h5l1 1.5h2a1.5 1.5 0 0 1 1.5 1.5v5.5A1.5 1.5 0 0 1 15.5 15h-11A1.5 1.5 0 0 1 3 13.5V8a1.5 1.5 0 0 1 1.5-1.5Z" fill="none" stroke="currentColor" strokeWidth="1.5" />
                        <circle cx="10" cy="10.8" r="2.3" fill="none" stroke="currentColor" strokeWidth="1.5" />
                      </svg>
                    </span>
                  </button>
                </div>
                <label className={layoutTw.dialogField}>
                  <span className={layoutTw.dialogLabel}>显示名称</span>
                  <input
                    className={layoutTw.dialogInput}
                    value={displayNameDraft}
                    maxLength={80}
                    placeholder={displayName}
                    autoComplete="off"
                    onChange={(event) => setDisplayNameDraft(event.target.value)}
                  />
                </label>
                <label className={layoutTw.dialogField}>
                  <span className={layoutTw.dialogLabel}>账号</span>
                  <input className={layoutTw.dialogInput} value={sessionUser?.email || '未绑定邮箱'} disabled readOnly />
                </label>
                <p className={layoutTw.dialogHint}>个人资料有助于你在 OneBook AI 中识别当前管理员账号。</p>
                {errorText ? <div className={layoutTw.dialogError}>{errorText}</div> : null}
                <div className={layoutTw.dialogActions}>
                  <button type="button" className={layoutTw.dialogButton} onClick={() => setIsProfileDialogOpen(false)}>
                    取消
                  </button>
                  <button type="submit" className={cx(layoutTw.dialogButton, layoutTw.dialogPrimary)} disabled={profileMutation.isPending}>
                    保存
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
