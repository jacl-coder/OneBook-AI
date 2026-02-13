import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { PropsWithChildren } from 'react'
import { useEffect, useState } from 'react'

import { useSessionStore } from '@/features/auth/store/session'
import { http } from '@/shared/lib/http/client'
import { setupAuthInterceptors } from '@/shared/lib/http/setupAuthInterceptors'

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
      getRefreshToken: () => useSessionStore.getState().refreshToken,
      updateTokens: ({ token, refreshToken }) =>
        useSessionStore.getState().updateTokens({ accessToken: token, refreshToken }),
      onAuthFailed: () => useSessionStore.getState().clearSession(),
    })
  }, [])

  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
}
