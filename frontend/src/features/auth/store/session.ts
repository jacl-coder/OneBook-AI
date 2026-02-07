import { create } from 'zustand'

export type AuthUser = {
  id: string
  email: string
  role: 'user' | 'admin'
  status: 'active' | 'disabled'
}

type SessionState = {
  accessToken: string | null
  refreshToken: string | null
  user: AuthUser | null
  setSession: (payload: {
    accessToken: string
    refreshToken: string
    user: AuthUser
  }) => void
  clearSession: () => void
}

export const useSessionStore = create<SessionState>((set) => ({
  accessToken: null,
  refreshToken: null,
  user: null,
  setSession: ({ accessToken, refreshToken, user }) =>
    set({
      accessToken,
      refreshToken,
      user,
    }),
  clearSession: () =>
    set({
      accessToken: null,
      refreshToken: null,
      user: null,
    }),
}))

