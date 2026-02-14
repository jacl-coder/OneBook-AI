import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { PropsWithChildren } from 'react'
import { useEffect, useState } from 'react'

import type { AuthUser } from '@/features/auth/store/session'
import { useSessionStore } from '@/features/auth/store/session'
import { http } from '@/shared/lib/http/client'

type SessionBootstrapResponse = AuthUser

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
    let cancelled = false
    const bootstrapSession = async () => {
      try {
        const { data } = await http.get<SessionBootstrapResponse>('/api/users/me')
        if (cancelled) return
        useSessionStore.getState().setSession({
          user: data,
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
