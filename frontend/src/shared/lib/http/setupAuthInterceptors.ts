import type {
  AxiosError,
  AxiosInstance,
  AxiosRequestConfig,
  AxiosRequestHeaders,
} from 'axios'
import axios from 'axios'

import { env } from '@/shared/config/env'

type RefreshPayload = {
  token: string
  refreshToken: string
}

type SetupAuthInterceptorsOptions = {
  getAccessToken: () => string | null
  getRefreshToken: () => string | null
  updateTokens: (payload: RefreshPayload) => void
  onAuthFailed: () => void
}

type RetriableConfig = AxiosRequestConfig & { _retry?: boolean }

let inflightRefresh: Promise<RefreshPayload> | null = null

function isRefreshRequest(url?: string): boolean {
  if (!url) {
    return false
  }
  return url.includes('/api/auth/refresh')
}

function createRefreshClient() {
  return axios.create({
    baseURL: env.apiBaseUrl,
    timeout: env.requestTimeoutMs,
    headers: { 'Content-Type': 'application/json' },
  })
}

async function refreshTokens(refreshToken: string): Promise<RefreshPayload> {
  const refreshClient = createRefreshClient()
  const { data } = await refreshClient.post<RefreshPayload>('/api/auth/refresh', {
    refreshToken,
  })
  return data
}

export function setupAuthInterceptors(
  http: AxiosInstance,
  options: SetupAuthInterceptorsOptions,
) {
  const requestInterceptor = http.interceptors.request.use((config) => {
    const accessToken = options.getAccessToken()
    if (!accessToken) {
      return config
    }
    const headers = (config.headers ?? {}) as AxiosRequestHeaders
    headers.Authorization = `Bearer ${accessToken}`
    config.headers = headers
    return config
  })

  const responseInterceptor = http.interceptors.response.use(
    (response) => response,
    async (error: AxiosError) => {
      const responseStatus = error.response?.status
      const originalConfig = error.config as RetriableConfig | undefined

      if (responseStatus !== 401 || !originalConfig || originalConfig._retry) {
        return Promise.reject(error)
      }
      if (isRefreshRequest(originalConfig.url)) {
        options.onAuthFailed()
        return Promise.reject(error)
      }

      const currentRefreshToken = options.getRefreshToken()
      if (!currentRefreshToken) {
        options.onAuthFailed()
        return Promise.reject(error)
      }

      originalConfig._retry = true

      try {
        if (!inflightRefresh) {
          inflightRefresh = refreshTokens(currentRefreshToken)
        }
        const refreshed = await inflightRefresh
        options.updateTokens(refreshed)

        const headers = (originalConfig.headers ?? {}) as AxiosRequestHeaders
        headers.Authorization = `Bearer ${refreshed.token}`
        originalConfig.headers = headers
        return await http.request(originalConfig)
      } catch (refreshError) {
        options.onAuthFailed()
        return Promise.reject(refreshError)
      } finally {
        inflightRefresh = null
      }
    },
  )

  return () => {
    http.interceptors.request.eject(requestInterceptor)
    http.interceptors.response.eject(responseInterceptor)
  }
}

