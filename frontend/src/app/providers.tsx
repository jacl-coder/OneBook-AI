import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { PropsWithChildren } from 'react'
import { useEffect, useState } from 'react'

import type { AuthUser } from '@/features/auth/store/session'
import { useSessionStore } from '@/features/auth/store/session'
import { http } from '@/shared/lib/http/client'
import { setupAuthInterceptors } from '@/shared/lib/http/setupAuthInterceptors'

type RefreshBootstrapResponse = {
  token: string
  user: AuthUser
}

export function AppProviders({ children }: PropsWithChildren) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            retry: 1,
            refetchOnWindowFocus: false,
            staleTime: 10_000,
          },
          mutations: {
            retry: 0,
          },
        },
      }),
  )

  useEffect(() => {
    return setupAuthInterceptors(http, {
      getAccessToken: () => useSessionStore.getState().accessToken,
      updateTokens: ({ token }) =>
        useSessionStore.getState().updateTokens({ accessToken: token }),
      onAuthFailed: () => useSessionStore.getState().clearSession(),
    })
  }, [])

  useEffect(() => {
    let cancelled = false
    const bootstrapSession = async () => {
      const current = useSessionStore.getState()
      if (current.accessToken && current.user) return
      try {
        const { data } = await http.post<RefreshBootstrapResponse>('/api/auth/refresh', {})
        if (cancelled) return
        useSessionStore.getState().setSession({
          accessToken: data.token,
          user: data.user,
        })
      } catch {
        if (!cancelled) {
          useSessionStore.getState().clearSession()
        }
      }
    }
    void bootstrapSession()
    return () => {
      cancelled = true
    }
  }, [])

  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
}
