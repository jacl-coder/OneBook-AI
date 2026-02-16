import type { OtpPurpose } from '@/features/auth/api/auth'

export const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/
export const AUTH_EMAIL_STORAGE_KEY = 'auth:email'
export const AUTH_ERROR_MESSAGE_STORAGE_KEY = 'auth:error-message'
export const AUTH_OTP_CHALLENGE_STORAGE_KEY = 'auth:otp:challenge-id'
export const AUTH_OTP_PURPOSE_STORAGE_KEY = 'auth:otp:purpose'
export const AUTH_OTP_PENDING_PASSWORD_STORAGE_KEY = 'auth:otp:pending-password'
export const AUTH_OTP_EMAIL_STORAGE_KEY = 'auth:otp:email'
export const AUTH_RESET_TOKEN_STORAGE_KEY = 'auth:reset:token'
export const DEFAULT_AUTH_ERROR_MESSAGE = 'Invalid client. Please start over.'

export type AuthNavigationState = {
  email?: string
  errorMessage?: string
  challengeId?: string
  purpose?: OtpPurpose
  pendingPassword?: string
  otpEmail?: string
  resetToken?: string
}

export type Step = 'entry' | 'password' | 'verify' | 'reset' | 'resetNew' | 'resetSuccess' | 'error'

export function getStep(pathname: string): Step {
  if (pathname === '/log-in/password' || pathname === '/create-account/password') return 'password'
  if (pathname === '/log-in/verify' || pathname === '/email-verification') return 'verify'
  if (pathname === '/reset-password') return 'reset'
  if (pathname === '/reset-password/new-password') return 'resetNew'
  if (pathname === '/reset-password/success') return 'resetSuccess'
  if (pathname === '/log-in/error') return 'error'
  return 'entry'
}

export function normalizeText(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

export function normalizeOtpPurpose(value: unknown): OtpPurpose | '' {
  const normalized = normalizeText(value).toLowerCase()
  if (normalized === 'signup_password') return 'signup_password'
  if (normalized === 'signup_otp') return 'signup_otp'
  if (normalized === 'login_otp') return 'login_otp'
  if (normalized === 'reset_password') return 'reset_password'
  return ''
}

export function readSessionValue(key: string) {
  if (typeof window === 'undefined') return ''
  return normalizeText(window.sessionStorage.getItem(key))
}

export function writeSessionValue(key: string, value: string) {
  if (typeof window === 'undefined') return
  if (value) {
    window.sessionStorage.setItem(key, value)
    return
  }
  window.sessionStorage.removeItem(key)
}

export function schedule(fn: () => void) {
  if (typeof queueMicrotask === 'function') {
    queueMicrotask(fn)
    return
  }
  setTimeout(fn, 0)
}
