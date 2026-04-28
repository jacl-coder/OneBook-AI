import axios from 'axios'

import { http } from '@/shared/lib/http/client'
import type { AuthUser } from '@/features/auth/store/session'

type AuthRequest = {
  identifier: string
  password: string
}

type LoginMethodsRequest = {
  identifier: string
}

export type OtpPurpose = 'signup' | 'login' | 'password_reset'

export type VerificationChannel = 'email' | 'phone'

type OtpSendRequest = {
  channel: VerificationChannel
  identifier: string
  purpose: OtpPurpose
}

type OtpVerifyRequest = {
  challengeId: string
  channel: VerificationChannel
  identifier: string
  purpose: OtpPurpose
  code: string
}

type PasswordResetCompleteRequest = {
  channel: VerificationChannel
  identifier: string
  verificationToken: string
  newPassword: string
}

type SignupCompleteRequest = {
  channel: VerificationChannel
  identifier: string
  verificationToken: string
  password: string
}

type BackendUser = AuthUser & {
  createdAt: string
  updatedAt: string
}

export type AuthResponse = {
  user: BackendUser
}

export type UpdateMePayload = {
  email?: string
  displayName?: string
}

export type OtpSendResponse = {
  challengeId: string
  expiresInSeconds: number
  resendAfterSeconds: number
  maskedIdentifier?: string
  maskedEmail?: string
}

export type LoginMethodsResponse = {
  exists: boolean
  passwordLogin: boolean
}

export type PasswordResetVerifyResponse = {
  verificationToken: string
  resetToken: string
  expiresInSeconds: number
}

export type VerificationVerifyResponse = {
  user?: BackendUser
  verificationToken?: string
  resetToken?: string
  expiresInSeconds?: number
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
  const { data } = await http.post<AuthResponse>('/api/auth/login/password', payload)
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

export async function completeSignup(payload: SignupCompleteRequest): Promise<AuthResponse> {
  const { data } = await http.post<AuthResponse>('/api/auth/signup/complete', payload)
  return data
}

export async function sendOtp(payload: OtpSendRequest): Promise<OtpSendResponse> {
  const { data } = await http.post<OtpSendResponse>('/api/auth/verification/send', payload)
  return data
}

export async function verifyOtp(payload: OtpVerifyRequest): Promise<VerificationVerifyResponse> {
  const { data } = await http.post<VerificationVerifyResponse>('/api/auth/verification/verify', payload)
  return data
}

export async function verifyPasswordReset(
  payload: OtpVerifyRequest,
): Promise<PasswordResetVerifyResponse> {
  const { data } = await http.post<PasswordResetVerifyResponse>('/api/auth/verification/verify', payload)
  return data
}

export async function completePasswordReset(payload: PasswordResetCompleteRequest): Promise<void> {
  await http.post('/api/auth/password/reset/complete', payload)
}

export async function updateMe(payload: UpdateMePayload): Promise<BackendUser> {
  const { data } = await http.patch<BackendUser>('/api/users/me', payload)
  return data
}

export async function uploadMyAvatar(file: File): Promise<BackendUser> {
  const form = new FormData()
  form.set('file', file)
  const { data } = await http.post<BackendUser>('/api/users/me/avatar', form)
  return data
}

export async function logout(): Promise<void> {
  await http.post('/api/auth/logout', {})
}
