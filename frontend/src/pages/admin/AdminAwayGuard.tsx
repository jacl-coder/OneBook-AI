import { Navigate, Outlet } from 'react-router-dom'

import { useSessionStore } from '@/features/auth/store/session'

export function AdminAwayGuard() {
  const sessionUser = useSessionStore((state) => state.user)

  if (sessionUser?.role === 'admin') {
    return <Navigate to="/admin" replace />
  }

  return <Outlet />
}
