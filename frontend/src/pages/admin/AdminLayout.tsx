import { useCallback, useRef, useState, type ChangeEvent } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Link, NavLink, Outlet, useNavigate } from 'react-router-dom'

import { getApiErrorMessage, logout, uploadMyAvatar } from '@/features/auth/api/auth'
import { useSessionStore } from '@/features/auth/store/session'
import { resolveApiAssetURL } from '@/shared/lib/http/assets'

const uiSansStyle = {
  fontFamily:
    "ui-sans-serif, -apple-system, system-ui, 'Segoe UI', Helvetica, 'Apple Color Emoji', Arial, sans-serif, 'Segoe UI Emoji', 'Segoe UI Symbol'",
} as const

const layoutTw = {
  shell: 'grid h-screen overflow-hidden bg-white text-[#0d0d0d] md:grid-cols-[240px_minmax(0,1fr)]',
  side: 'grid h-full min-h-0 grid-rows-[auto_minmax(0,1fr)_auto] border-r border-[rgba(0,0,0,0.08)] bg-[#f7f7f7] p-3',
  brand: 'inline-flex h-9 items-center rounded-[10px] px-2 text-[15px] font-semibold text-[#0d0d0d]',
  navScroll: 'min-h-0 overflow-auto',
  nav: 'mt-3 grid gap-1',
  navLink:
    'inline-flex h-9 items-center rounded-[10px] px-3 text-[14px] text-[#232323] hover:bg-[#ececec] aria-[current=page]:bg-[#e7e7e7]',
  accountCard:
    'grid gap-1 rounded-[12px] border border-[rgba(0,0,0,0.08)] bg-white p-3 text-[12px] text-[#4f4f4f]',
  avatar:
    'mb-1 inline-flex h-9 w-9 items-center justify-center overflow-hidden rounded-full border-0 bg-[#222] p-0 text-[13px] font-semibold text-white',
  avatarImg: 'h-full w-full object-cover',
  notice: 'text-[11px] leading-4 text-[#a4161a]',
  logout:
    'mt-2 inline-flex h-8 items-center justify-center rounded-[8px] border border-[rgba(0,0,0,0.12)] text-[12px] hover:bg-[#f4f4f4]',
  main: 'grid h-full min-h-0 grid-rows-[52px_minmax(0,1fr)]',
  topBar: 'flex h-[52px] items-center justify-between border-b border-[rgba(0,0,0,0.08)] bg-white px-4',
  topTitle: 'text-[16px] font-medium',
  body: 'min-h-0 overflow-auto bg-white px-4 py-4 max-[767px]:px-3',
}

export function AdminLayout() {
  const navigate = useNavigate()
  const sessionUser = useSessionStore((state) => state.user)
  const setSession = useSessionStore((state) => state.setSession)
  const clearSession = useSessionStore((state) => state.clearSession)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [errorText, setErrorText] = useState('')
  const avatarLetter = (sessionUser?.displayName || sessionUser?.email || 'A').slice(0, 1).toUpperCase()
  const avatarUrl = resolveApiAssetURL(sessionUser?.avatarUrl)

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

  function handleAvatarChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    void avatarMutation.mutateAsync(file)
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

        <div className={layoutTw.accountCard}>
          <span className="text-[11px] text-[#757575]">当前账号</span>
          <button
            type="button"
            className={layoutTw.avatar}
            aria-label="上传头像"
            disabled={avatarMutation.isPending}
            onClick={() => fileInputRef.current?.click()}
          >
            {avatarUrl ? <img src={avatarUrl} alt="" className={layoutTw.avatarImg} /> : avatarLetter}
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif"
            className="hidden"
            onChange={handleAvatarChange}
          />
          <span className="truncate text-[12px] text-[#0d0d0d]">{sessionUser?.displayName || sessionUser?.email || '-'}</span>
          <span className="text-[11px] text-[#757575]">管理员</span>
          {errorText ? <span className={layoutTw.notice}>{errorText}</span> : null}
          <button type="button" className={layoutTw.logout} onClick={() => void handleLogout()}>
            退出登录
          </button>
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
    </div>
  )
}
