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
  error: string
}

export function getApiErrorMessage(error: unknown, fallback: string): string {
  if (!axios.isAxiosError(error)) return fallback
  const data = error.response?.data as Partial<ErrorResponse> | undefined
  if (data && typeof data.error === 'string' && data.error.trim()) return data.error.trim()
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

