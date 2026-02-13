import axios from 'axios'

import { http } from '@/shared/lib/http/client'
import type { AuthUser } from '@/features/auth/store/session'

type AuthRequest = {
  email: string
  password: string
}

type LoginMethodsRequest = {
  email: string
}

export type OtpPurpose = 'signup_password' | 'signup_otp' | 'login_otp'

type OtpSendRequest = {
  email: string
  purpose: OtpPurpose
}

type OtpVerifyRequest = {
  challengeId: string
  email: string
  purpose: OtpPurpose
  code: string
  password?: string
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

export type OtpSendResponse = {
  challengeId: string
  expiresInSeconds: number
  resendAfterSeconds: number
  maskedEmail?: string
}

export type LoginMethodsResponse = {
  passwordLogin: boolean
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

export function getApiErrorCode(error: unknown): string {
  if (!axios.isAxiosError(error)) return ''
  const data = error.response?.data as Partial<ErrorResponse> | undefined
  if (!data || typeof data.code !== 'string') return ''
  return data.code.trim()
}

export async function login(payload: AuthRequest): Promise<AuthResponse> {
  const { data } = await http.post<AuthResponse>('/api/auth/login', payload)
  return data
}

export async function loginMethods(payload: LoginMethodsRequest): Promise<LoginMethodsResponse> {
  const { data } = await http.post<LoginMethodsResponse>('/api/auth/login/methods', payload)
  return data
}

export async function signup(payload: AuthRequest): Promise<AuthResponse> {
  const { data } = await http.post<AuthResponse>('/api/auth/signup', payload)
  return data
}

export async function sendOtp(payload: OtpSendRequest): Promise<OtpSendResponse> {
  const { data } = await http.post<OtpSendResponse>('/api/auth/otp/send', payload)
  return data
}

export async function verifyOtp(payload: OtpVerifyRequest): Promise<AuthResponse> {
  const { data } = await http.post<AuthResponse>('/api/auth/otp/verify', payload)
  return data
}
