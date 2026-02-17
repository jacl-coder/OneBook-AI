import { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it } from 'vitest'

import { http } from '../../src/shared/lib/http/client'

const REFRESH_PATH = '/api/auth/refresh'

type AdapterResponse = {
  status: number
  statusText: string
  headers: Record<string, string>
  config: InternalAxiosRequestConfig
  data: unknown
}

function resolveResponse(config: InternalAxiosRequestConfig, status: number, data: unknown): Promise<AdapterResponse> {
  return Promise.resolve({
    status,
    statusText: status === 200 ? 'OK' : 'Unauthorized',
    headers: {},
    config,
    data,
  })
}

function rejectWith401(config: InternalAxiosRequestConfig): Promise<never> {
  return Promise.reject(
    new AxiosError(
      'Unauthorized',
      'ERR_BAD_REQUEST',
      config,
      undefined,
      {
        status: 401,
        statusText: 'Unauthorized',
        headers: {},
        config,
        data: { error: 'unauthorized' },
      },
    ),
  )
}

describe('http refresh single-flight', () => {
  const originalAdapter = http.defaults.adapter

  afterEach(() => {
    http.defaults.adapter = originalAdapter
  })

  it('reuses one refresh request for concurrent 401 responses', async () => {
    let refreshCalls = 0
    let usersMeCalls = 0
    let booksCalls = 0

    http.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      if (config.url?.includes(REFRESH_PATH)) {
        refreshCalls += 1
        await new Promise((resolve) => setTimeout(resolve, 20))
        return resolveResponse(config, 200, { ok: true })
      }

      if (config.url?.includes('/api/users/me')) {
        usersMeCalls += 1
        if (usersMeCalls === 1) {
          return rejectWith401(config)
        }
        return resolveResponse(config, 200, { id: 'u1' })
      }

      if (config.url?.includes('/api/books')) {
        booksCalls += 1
        if (booksCalls === 1) {
          return rejectWith401(config)
        }
        return resolveResponse(config, 200, { items: [], count: 0 })
      }

      return resolveResponse(config, 200, {})
    }

    const [userResponse, booksResponse] = await Promise.all([
      http.get('/api/users/me'),
      http.get('/api/books'),
    ])

    expect(userResponse.status).toBe(200)
    expect(booksResponse.status).toBe(200)
    expect(refreshCalls).toBe(1)
    expect(usersMeCalls).toBe(2)
    expect(booksCalls).toBe(2)
  })

  it('does not loop retries when refresh fails', async () => {
    let refreshCalls = 0
    let usersMeCalls = 0
    let booksCalls = 0

    http.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      if (config.url?.includes(REFRESH_PATH)) {
        refreshCalls += 1
        return rejectWith401(config)
      }

      if (config.url?.includes('/api/users/me')) {
        usersMeCalls += 1
        return rejectWith401(config)
      }

      if (config.url?.includes('/api/books')) {
        booksCalls += 1
        return rejectWith401(config)
      }

      return resolveResponse(config, 200, {})
    }

    const results = await Promise.allSettled([
      http.get('/api/users/me'),
      http.get('/api/books'),
    ])

    expect(results[0].status).toBe('rejected')
    expect(results[1].status).toBe('rejected')
    expect(refreshCalls).toBe(1)
    expect(usersMeCalls).toBe(1)
    expect(booksCalls).toBe(1)
  })
})
