import { create } from 'zustand'

export type AuthUser = {
  id: string
  email: string
  role: 'user' | 'admin'
  status: 'active' | 'disabled'
}

const SESSION_STORAGE_KEY = 'onebook:session'

type StoredSession = {
  user: AuthUser
}

function readStoredSession(): StoredSession | null {
  if (typeof window === 'undefined') return null
  const raw = window.sessionStorage.getItem(SESSION_STORAGE_KEY)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as Partial<StoredSession>
    if (!parsed || typeof parsed !== 'object') return null
    const user = parsed.user as Partial<AuthUser> | undefined
    if (!user || typeof user !== 'object') return null
    if (typeof user.id !== 'string' || typeof user.email !== 'string') return null
    if (user.role !== 'user' && user.role !== 'admin') return null
    if (user.status !== 'active' && user.status !== 'disabled') return null
    return {
      user: {
        id: user.id,
        email: user.email,
        role: user.role,
        status: user.status,
      },
    }
  } catch {
    return null
  }
}

function writeStoredSession(session: StoredSession | null) {
  if (typeof window === 'undefined') return
  if (!session) {
    window.sessionStorage.removeItem(SESSION_STORAGE_KEY)
    return
  }
  window.sessionStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(session))
}

const initialSession = readStoredSession()

type SessionState = {
  user: AuthUser | null
  setSession: (payload: { user: AuthUser }) => void
  clearSession: () => void
}

export const useSessionStore = create<SessionState>((set) => ({
  user: initialSession?.user ?? null,
  setSession: ({ user }) =>
    set(() => {
      const nextSession: StoredSession = { user }
      writeStoredSession(nextSession)
      return { user }
    }),
  clearSession: () =>
    set(() => {
      writeStoredSession(null)
      return { user: null }
    }),
}))
