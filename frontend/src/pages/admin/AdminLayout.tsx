import { useCallback } from 'react'
import { Link, NavLink, Outlet, useNavigate } from 'react-router-dom'

import { logout } from '@/features/auth/api/auth'
import { useSessionStore } from '@/features/auth/store/session'

const uiSansStyle = {
  fontFamily:
    "ui-sans-serif, -apple-system, system-ui, 'Segoe UI', Helvetica, 'Apple Color Emoji', Arial, sans-serif, 'Segoe UI Emoji', 'Segoe UI Symbol'",
} as const

const layoutTw = {
  shell: 'grid min-h-screen bg-white text-[#0d0d0d] md:grid-cols-[240px_minmax(0,1fr)]',
  side: 'border-r border-[rgba(0,0,0,0.08)] bg-[#f7f7f7] p-3',
  brand: 'inline-flex h-9 items-center rounded-[10px] px-2 text-[15px] font-semibold text-[#0d0d0d]',
  nav: 'mt-3 grid gap-1',
  navLink:
    'inline-flex h-9 items-center rounded-[10px] px-3 text-[14px] text-[#232323] hover:bg-[#ececec] aria-[current=page]:bg-[#e7e7e7]',
  accountCard:
    'mt-6 grid gap-1 rounded-[12px] border border-[rgba(0,0,0,0.08)] bg-white p-3 text-[12px] text-[#4f4f4f]',
  logout:
    'mt-2 inline-flex h-8 items-center justify-center rounded-[8px] border border-[rgba(0,0,0,0.12)] text-[12px] hover:bg-[#f4f4f4]',
  main: 'grid min-h-screen grid-rows-[52px_minmax(0,1fr)]',
  topBar: 'flex h-[52px] items-center justify-between border-b border-[rgba(0,0,0,0.08)] bg-white px-4',
  topTitle: 'text-[16px] font-medium',
  body: 'overflow-auto bg-white px-4 py-4 max-[767px]:px-3',
}

export function AdminLayout() {
  const navigate = useNavigate()
  const sessionUser = useSessionStore((state) => state.user)
  const clearSession = useSessionStore((state) => state.clearSession)

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

        <div className={layoutTw.accountCard}>
          <span className="text-[11px] text-[#757575]">当前账号</span>
          <span className="truncate text-[12px] text-[#0d0d0d]">{sessionUser?.email ?? '-'}</span>
          <span className="text-[11px] text-[#757575]">管理员</span>
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
