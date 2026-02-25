import axios from 'axios'
import type { AxiosError, AxiosRequestConfig } from 'axios'

import { env } from '@/shared/config/env'

export const http = axios.create({
  baseURL: env.apiBaseUrl,
  timeout: env.requestTimeoutMs,
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json',
  },
})

const REFRESH_PATH = '/api/auth/refresh'

type RetriableRequestConfig = AxiosRequestConfig & {
  _retryAfterRefresh?: boolean
  _skipAuthRefresh?: boolean
}

let refreshRequest: Promise<void> | null = null

http.interceptors.request.use((config) => {
  if (typeof FormData !== 'undefined' && config.data instanceof FormData) {
    const headers = axios.AxiosHeaders.from(config.headers)
    headers.delete('Content-Type')
    config.headers = headers
  }
  return config
})

function shouldSkipRefresh(config: RetriableRequestConfig | undefined): boolean {
  if (!config) return true
  if (config._skipAuthRefresh) return true
  if (config._retryAfterRefresh) return true
  const url = typeof config.url === 'string' ? config.url : ''
  return url.includes(REFRESH_PATH)
}

async function refreshSessionOnce(): Promise<void> {
  if (!refreshRequest) {
    refreshRequest = http
      .post(
        REFRESH_PATH,
        {},
        {
          _skipAuthRefresh: true,
        } as RetriableRequestConfig,
      )
      .then(() => undefined)
      .finally(() => {
        refreshRequest = null
      })
  }
  return refreshRequest
}

http.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const status = error.response?.status
    const config = error.config as RetriableRequestConfig | undefined

    if (status !== 401 || shouldSkipRefresh(config)) {
      return Promise.reject(error)
    }

    if (!config) {
      return Promise.reject(error)
    }
    config._retryAfterRefresh = true

    try {
      await refreshSessionOnce()
      return http.request(config)
    } catch {
      return Promise.reject(error)
    }
  },
)
