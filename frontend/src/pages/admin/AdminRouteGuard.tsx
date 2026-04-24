import { Link, Outlet } from 'react-router-dom'

import { useSessionStore } from '@/features/auth/store/session'

const uiSansStyle = {
  fontFamily:
    "ui-sans-serif, -apple-system, system-ui, 'Segoe UI', Helvetica, 'Apple Color Emoji', Arial, sans-serif, 'Segoe UI Emoji', 'Segoe UI Symbol'",
} as const

export function AdminRouteGuard() {
  const sessionUser = useSessionStore((state) => state.user)

  if (!sessionUser) {
    return (
      <div className="grid min-h-screen place-items-center bg-white p-6 text-[#0d0d0d]" style={uiSansStyle}>
        <div className="w-full max-w-[420px] rounded-[14px] border border-[rgba(0,0,0,0.08)] p-6 text-center">
          <h1 className="text-[20px] font-medium">请先登录后进入后台管理</h1>
          <p className="mt-2 text-[14px] text-[#666]">管理员登录后可查看用户、书籍、审计日志与系统概览。</p>
          <div className="mt-5 flex items-center justify-center gap-2">
            <Link className="inline-flex h-8 items-center rounded-[9999px] border border-[rgba(0,0,0,0.12)] px-[12px] text-[12px]" to="/log-in-or-create-account">
              去登录
            </Link>
            <Link className="inline-flex h-8 items-center rounded-[9999px] border border-[rgba(0,0,0,0.12)] px-[12px] text-[12px]" to="/chat">
              返回聊天
            </Link>
          </div>
        </div>
      </div>
    )
  }

  if (sessionUser.role !== 'admin') {
    return (
      <div className="grid min-h-screen place-items-center bg-white p-6 text-[#0d0d0d]" style={uiSansStyle}>
        <div className="w-full max-w-[420px] rounded-[14px] border border-[#f4b0b4] bg-[#fff5f6] p-6 text-center">
          <h1 className="text-[20px] font-medium">403 无权限访问</h1>
          <p className="mt-2 text-[14px] text-[#9f1820]">该页面仅管理员可访问。</p>
          <div className="mt-5 flex items-center justify-center gap-2">
            <Link className="inline-flex h-8 items-center rounded-[9999px] bg-[#0d0d0d] px-[12px] text-[12px] text-white" to="/chat">
              返回聊天
            </Link>
          </div>
        </div>
      </div>
    )
  }

  return <Outlet />
}
