import { create } from 'zustand'

export type AuthUser = {
  id: string
  email: string
  role: 'user' | 'admin'
  status: 'active' | 'disabled'
}

const SESSION_STORAGE_KEY = 'onebook:session'

type StoredSession = {
  accessToken: string
  refreshToken: string
  user: AuthUser
}

function readStoredSession(): StoredSession | null {
  if (typeof window === 'undefined') return null
  const raw = window.sessionStorage.getItem(SESSION_STORAGE_KEY)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as Partial<StoredSession>
    if (!parsed || typeof parsed !== 'object') return null
    if (typeof parsed.accessToken !== 'string' || typeof parsed.refreshToken !== 'string') return null
    const user = parsed.user as Partial<AuthUser> | undefined
    if (!user || typeof user !== 'object') return null
    if (typeof user.id !== 'string' || typeof user.email !== 'string') return null
    if (user.role !== 'user' && user.role !== 'admin') return null
    if (user.status !== 'active' && user.status !== 'disabled') return null
    return {
      accessToken: parsed.accessToken,
      refreshToken: parsed.refreshToken,
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
  accessToken: string | null
  refreshToken: string | null
  user: AuthUser | null
  setSession: (payload: {
    accessToken: string
    refreshToken: string
    user: AuthUser
  }) => void
  updateTokens: (payload: { accessToken: string; refreshToken: string }) => void
  clearSession: () => void
}

export const useSessionStore = create<SessionState>((set) => ({
  accessToken: initialSession?.accessToken ?? null,
  refreshToken: initialSession?.refreshToken ?? null,
  user: initialSession?.user ?? null,
  setSession: ({ accessToken, refreshToken, user }) =>
    set(() => {
      const nextSession: StoredSession = { accessToken, refreshToken, user }
      writeStoredSession(nextSession)
      return nextSession
    }),
  updateTokens: ({ accessToken, refreshToken }) =>
    set((state) => {
      if (!state.user) {
        writeStoredSession(null)
        return { accessToken: null, refreshToken: null, user: null }
      }
      const nextSession: StoredSession = { accessToken, refreshToken, user: state.user }
      writeStoredSession(nextSession)
      return nextSession
    }),
  clearSession: () =>
    set(() => {
      writeStoredSession(null)
      return { accessToken: null, refreshToken: null, user: null }
    }),
}))
