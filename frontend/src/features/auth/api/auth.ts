import axios from 'axios'

import { http } from '@/shared/lib/http/client'
import type { AuthUser } from '@/features/auth/store/session'

type AuthRequest = {
  email: string
  password: string
}

type BackendUser = AuthUser & {
  createdAt: string
  updatedAt: string
}

export type AuthResponse = {
  token: string
  refreshToken: string
  user: BackendUser
}

type ErrorResponse = {
  error?: string
  message?: string
  detail?: string
  title?: string
  code?: string
  requestId?: string
}

export function getApiErrorMessage(error: unknown, fallback: string): string {
  if (!axios.isAxiosError(error)) return fallback
  const data = error.response?.data as Partial<ErrorResponse> | undefined
  if (data && typeof data.error === 'string' && data.error.trim()) return data.error.trim()
  if (data && typeof data.message === 'string' && data.message.trim()) return data.message.trim()
  if (data && typeof data.detail === 'string' && data.detail.trim()) return data.detail.trim()
  if (data && typeof data.title === 'string' && data.title.trim()) return data.title.trim()
  return fallback
}

export async function login(payload: AuthRequest): Promise<AuthResponse> {
  const { data } = await http.post<AuthResponse>('/api/auth/login', payload)
  return data
}

export async function signup(payload: AuthRequest): Promise<AuthResponse> {
  const { data } = await http.post<AuthResponse>('/api/auth/signup', payload)
  return data
}
