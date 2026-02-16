import { useEffect, type ClipboardEvent, type SubmitEvent, useId, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import onebookWordmark from '@/assets/brand/onebook-wordmark.svg'
import onebookLogoMarkSvg from '@/assets/brand/onebook-logo-mark.svg'
import googleIconSvg from '@/assets/brand/provider/google-logo.svg'
import appleIconSvg from '@/assets/brand/provider/apple-logo.svg'
import microsoftIconSvg from '@/assets/brand/provider/microsoft-logo.svg'
import phoneIconSvg from '@/assets/icons/phone.svg'
import errorIconSvg from '@/assets/icons/error-circle.svg'
import eyeIconSvg from '@/assets/icons/eye.svg'
import eyeOffIconSvg from '@/assets/icons/eye-off.svg'
import successCheckmarkCircleSvg from '@/assets/icons/success-checkmark-circle.svg'

import {
  completePasswordReset,
  getApiErrorCode,
  getApiErrorMessage,
  loginMethods,
  login,
  sendOtp,
  type OtpPurpose,
  verifyPasswordReset,
  verifyOtp,
} from '@/features/auth/api/auth'
import { useSessionStore } from '@/features/auth/store/session'
import {
  AUTH_EMAIL_STORAGE_KEY,
  AUTH_ERROR_MESSAGE_STORAGE_KEY,
  AUTH_OTP_CHALLENGE_STORAGE_KEY,
  AUTH_OTP_EMAIL_STORAGE_KEY,
  AUTH_OTP_PENDING_PASSWORD_STORAGE_KEY,
  AUTH_OTP_PURPOSE_STORAGE_KEY,
  AUTH_RESET_TOKEN_STORAGE_KEY,
  DEFAULT_AUTH_ERROR_MESSAGE,
  EMAIL_PATTERN,
  type AuthNavigationState,
  getStep,
  normalizeOtpPurpose,
  normalizeText,
  readSessionValue,
  schedule,
  writeSessionValue,
} from '@/pages/login/shared'

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()

  const step = getStep(location.pathname)
  const logoLinkTarget = '/chat'
  const isCreateAccountEntry = step === 'entry' && location.pathname === '/create-account'
  const isCreateAccountPassword = step === 'password' && location.pathname === '/create-account/password'
  const locationState = (location.state as AuthNavigationState | null) ?? null
  const locationSearchEmail = normalizeText(new URLSearchParams(location.search).get('email'))
  const locationStateEmail = normalizeText(locationState?.email) || locationSearchEmail
  const locationStateErrorMessage = normalizeText(locationState?.errorMessage)
  const locationStateChallengeId = normalizeText(locationState?.challengeId)
  const locationStatePurpose = normalizeOtpPurpose(locationState?.purpose)
  const locationStatePendingPassword = normalizeText(locationState?.pendingPassword)
  const locationStateOtpEmail = normalizeText(locationState?.otpEmail)
  const locationStateResetToken = normalizeText(locationState?.resetToken)
  const [stepEmail, setStepEmail] = useState(() => locationStateEmail || readSessionValue(AUTH_EMAIL_STORAGE_KEY))
  const [authErrorMessage, setAuthErrorMessage] = useState(() => {
    const initialError = locationStateErrorMessage || readSessionValue(AUTH_ERROR_MESSAGE_STORAGE_KEY)
    return initialError || DEFAULT_AUTH_ERROR_MESSAGE
  })
  const [otpChallengeId, setOtpChallengeId] = useState(
    () => locationStateChallengeId || readSessionValue(AUTH_OTP_CHALLENGE_STORAGE_KEY),
  )
  const [otpPurpose, setOtpPurpose] = useState<OtpPurpose | ''>(
    () => locationStatePurpose || normalizeOtpPurpose(readSessionValue(AUTH_OTP_PURPOSE_STORAGE_KEY)),
  )
  const [otpPendingPassword, setOtpPendingPassword] = useState(
    () => locationStatePendingPassword || readSessionValue(AUTH_OTP_PENDING_PASSWORD_STORAGE_KEY),
  )
  const [otpEmail, setOtpEmail] = useState(
    () => locationStateOtpEmail || readSessionValue(AUTH_OTP_EMAIL_STORAGE_KEY),
  )
  const [resetToken, setResetToken] = useState(
    () => locationStateResetToken || readSessionValue(AUTH_RESET_TOKEN_STORAGE_KEY),
  )

  const emailId = useId()
  const emailLabelId = useId()
  const emailErrorId = useId()

  const passwordId = useId()
  const passwordLabelId = useId()
  const passwordErrorId = useId()
  const readonlyEmailLabelId = useId()
  const newPasswordId = useId()
  const newPasswordLabelId = useId()
  const newPasswordErrorId = useId()
  const confirmPasswordId = useId()
  const confirmPasswordLabelId = useId()
  const confirmPasswordErrorId = useId()
  const verifySubtitleId = useId()
  const verifyErrorId = useId()

  const [email, setEmail] = useState(stepEmail)
  const [isEmailFocused, setIsEmailFocused] = useState(false)
  const [isEntrySubmitting, setIsEntrySubmitting] = useState(false)
  const [emailErrorText, setEmailErrorText] = useState('')

  const [password, setPassword] = useState('')
  const [isPasswordFocused, setIsPasswordFocused] = useState(false)
  const [passwordVisible, setPasswordVisible] = useState(false)
  const [isPasswordSubmitting, setIsPasswordSubmitting] = useState(false)
  const [passwordErrorText, setPasswordErrorText] = useState('')
  const [isResetSubmitting, setIsResetSubmitting] = useState(false)
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [isNewPasswordFocused, setIsNewPasswordFocused] = useState(false)
  const [isConfirmPasswordFocused, setIsConfirmPasswordFocused] = useState(false)
  const [isNewPasswordVisible, setIsNewPasswordVisible] = useState(false)
  const [isConfirmPasswordVisible, setIsConfirmPasswordVisible] = useState(false)
  const [newPasswordErrorText, setNewPasswordErrorText] = useState('')
  const [confirmPasswordErrorText, setConfirmPasswordErrorText] = useState('')
  const [isResetNewSubmitting, setIsResetNewSubmitting] = useState(false)

  const [otp, setOtp] = useState(['', '', '', '', '', ''])
  const otpRefs = useRef<Array<HTMLInputElement | null>>([])
  const lastSubmittedVerifyCodeRef = useRef('')
  const [verifyErrorText, setVerifyErrorText] = useState('')
  const [isVerifySubmitting, setIsVerifySubmitting] = useState(false)
  const [isOtpSending, setIsOtpSending] = useState(false)

  const entryInputRef = useRef<HTMLInputElement>(null)
  const passwordInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (step === 'entry') {
      schedule(() => {
        setEmail(stepEmail)
        setEmailErrorText('')
        setIsEmailFocused(false)
        setIsEntrySubmitting(false)
      })
    }
  }, [step, stepEmail])

  useEffect(() => {
    if (step !== 'verify') return
    schedule(() => {
      setOtp(['', '', '', '', '', ''])
      lastSubmittedVerifyCodeRef.current = ''
      setVerifyErrorText('')
      setIsVerifySubmitting(false)
    })
  }, [step, stepEmail])

  useEffect(() => {
    if (step !== 'verify') return
    const frameId = requestAnimationFrame(() => {
      otpRefs.current[0]?.focus()
    })
    return () => cancelAnimationFrame(frameId)
  }, [step, stepEmail])

  useEffect(() => {
    if (step !== 'reset') return
    schedule(() => setIsResetSubmitting(false))
  }, [step, stepEmail])

  useEffect(() => {
    if (step !== 'resetNew') return
    schedule(() => {
      setIsResetNewSubmitting(false)
      setNewPassword('')
      setConfirmPassword('')
      setNewPasswordErrorText('')
      setConfirmPasswordErrorText('')
    })
  }, [step, stepEmail])

  const hasEmailValue = email.trim().length > 0
  const isEmailInvalid = emailErrorText.length > 0
  const isEmailActive = isEmailFocused || hasEmailValue

  const hasPasswordValue = password.trim().length > 0
  const isPasswordInvalid = passwordErrorText.length > 0
  const isPasswordActive = isPasswordFocused || hasPasswordValue

  const emailWrapClassName = [
    'auth-input-wrap',
    isEmailActive ? 'is-active' : '',
    isEmailFocused ? 'is-focused' : '',
    hasEmailValue ? 'has-value' : '',
    isEmailInvalid ? 'is-invalid' : '',
    isEntrySubmitting ? 'is-submitting' : '',
  ]
    .filter(Boolean)
    .join(' ')

  const passwordWrapClassName = [
    'auth-input-wrap',
    isPasswordActive ? 'is-active' : '',
    isPasswordFocused ? 'is-focused' : '',
    hasPasswordValue ? 'has-value' : '',
    isPasswordInvalid ? 'is-invalid' : '',
    isPasswordSubmitting ? 'is-submitting' : '',
    'auth-input-wrap-with-end',
  ]
    .filter(Boolean)
    .join(' ')

  const hasNewPasswordValue = newPassword.trim().length > 0
  const isNewPasswordInvalid = newPasswordErrorText.length > 0
  const isNewPasswordActive = isNewPasswordFocused || hasNewPasswordValue

  const hasConfirmPasswordValue = confirmPassword.trim().length > 0
  const isConfirmPasswordInvalid = confirmPasswordErrorText.length > 0
  const isConfirmPasswordActive = isConfirmPasswordFocused || hasConfirmPasswordValue
  const verifyDescribedBy = verifyErrorText ? `${verifySubtitleId} ${verifyErrorId}` : verifySubtitleId
  const passwordContinuePath =
    otpPurpose === 'signup_password' || otpPurpose === 'signup_otp' ? '/create-account/password' : '/log-in/password'

  const newPasswordWrapClassName = [
    'auth-input-wrap',
    isNewPasswordActive ? 'is-active' : '',
    isNewPasswordFocused ? 'is-focused' : '',
    hasNewPasswordValue ? 'has-value' : '',
    isNewPasswordInvalid ? 'is-invalid' : '',
    isResetNewSubmitting ? 'is-submitting' : '',
    'auth-input-wrap-with-end',
  ]
    .filter(Boolean)
    .join(' ')

  const confirmPasswordWrapClassName = [
    'auth-input-wrap',
    isConfirmPasswordActive ? 'is-active' : '',
    isConfirmPasswordFocused ? 'is-focused' : '',
    hasConfirmPasswordValue ? 'has-value' : '',
    isConfirmPasswordInvalid ? 'is-invalid' : '',
    isResetNewSubmitting ? 'is-submitting' : '',
    'auth-input-wrap-with-end',
  ]
    .filter(Boolean)
    .join(' ')

  const getAuthState = (
    emailValue: string,
    extras?: Partial<Pick<AuthNavigationState, 'challengeId' | 'purpose' | 'pendingPassword' | 'otpEmail' | 'resetToken'>>,
  ): AuthNavigationState => {
    const normalizedEmail = normalizeText(emailValue)
    const state: AuthNavigationState = {}
    if (normalizedEmail) state.email = normalizedEmail
    if (extras?.challengeId) state.challengeId = normalizeText(extras.challengeId)
    if (extras?.purpose) state.purpose = normalizeOtpPurpose(extras.purpose) || undefined
    if (extras?.pendingPassword) state.pendingPassword = normalizeText(extras.pendingPassword)
    if (extras?.otpEmail) state.otpEmail = normalizeText(extras.otpEmail)
    if (extras?.resetToken) state.resetToken = normalizeText(extras.resetToken)
    return state
  }

  const syncOtpContext = (payload: { challengeId: string; purpose: OtpPurpose; pendingPassword?: string; otpEmail?: string }) => {
    const challengeId = normalizeText(payload.challengeId)
    const purpose = payload.purpose
    const pendingPassword = normalizeText(payload.pendingPassword)
    const normalizedOtpEmail = normalizeText(payload.otpEmail || stepEmail)

    setOtpChallengeId(challengeId)
    writeSessionValue(AUTH_OTP_CHALLENGE_STORAGE_KEY, challengeId)

    setOtpPurpose(purpose)
    writeSessionValue(AUTH_OTP_PURPOSE_STORAGE_KEY, purpose)

    setOtpEmail(normalizedOtpEmail)
    writeSessionValue(AUTH_OTP_EMAIL_STORAGE_KEY, normalizedOtpEmail)

    if (purpose === 'signup_password' && pendingPassword) {
      setOtpPendingPassword(pendingPassword)
      writeSessionValue(AUTH_OTP_PENDING_PASSWORD_STORAGE_KEY, pendingPassword)
      return { challengeId, purpose, pendingPassword, otpEmail: normalizedOtpEmail }
    }
    setOtpPendingPassword('')
    writeSessionValue(AUTH_OTP_PENDING_PASSWORD_STORAGE_KEY, '')
    return { challengeId, purpose, pendingPassword: '', otpEmail: normalizedOtpEmail }
  }

  const clearOtpContext = () => {
    setOtpChallengeId('')
    setOtpPurpose('')
    setOtpPendingPassword('')
    setOtpEmail('')
    writeSessionValue(AUTH_OTP_CHALLENGE_STORAGE_KEY, '')
    writeSessionValue(AUTH_OTP_PURPOSE_STORAGE_KEY, '')
    writeSessionValue(AUTH_OTP_PENDING_PASSWORD_STORAGE_KEY, '')
    writeSessionValue(AUTH_OTP_EMAIL_STORAGE_KEY, '')
  }

  const syncResetToken = (value: string) => {
    const normalized = normalizeText(value)
    setResetToken(normalized)
    writeSessionValue(AUTH_RESET_TOKEN_STORAGE_KEY, normalized)
    return normalized
  }

  const clearResetToken = () => {
    syncResetToken('')
  }

  const syncEmail = (emailValue: string) => {
    const normalizedEmail = normalizeText(emailValue)
    setStepEmail(normalizedEmail)
    writeSessionValue(AUTH_EMAIL_STORAGE_KEY, normalizedEmail)
    return normalizedEmail
  }

  useEffect(() => {
    const nextEmail = locationStateEmail || stepEmail || readSessionValue(AUTH_EMAIL_STORAGE_KEY)
    if (nextEmail !== stepEmail) schedule(() => setStepEmail(nextEmail))
    writeSessionValue(AUTH_EMAIL_STORAGE_KEY, nextEmail)
  }, [location.key, locationStateEmail, stepEmail])

  useEffect(() => {
    const nextChallengeId = locationStateChallengeId || otpChallengeId || readSessionValue(AUTH_OTP_CHALLENGE_STORAGE_KEY)
    if (nextChallengeId !== otpChallengeId) schedule(() => setOtpChallengeId(nextChallengeId))
    writeSessionValue(AUTH_OTP_CHALLENGE_STORAGE_KEY, nextChallengeId)

    const nextPurpose =
      locationStatePurpose || otpPurpose || normalizeOtpPurpose(readSessionValue(AUTH_OTP_PURPOSE_STORAGE_KEY))
    if (nextPurpose !== otpPurpose) schedule(() => setOtpPurpose(nextPurpose))
    writeSessionValue(AUTH_OTP_PURPOSE_STORAGE_KEY, nextPurpose)

    const nextPendingPassword =
      locationStatePendingPassword || otpPendingPassword || readSessionValue(AUTH_OTP_PENDING_PASSWORD_STORAGE_KEY)
    if (nextPendingPassword !== otpPendingPassword) schedule(() => setOtpPendingPassword(nextPendingPassword))
    writeSessionValue(AUTH_OTP_PENDING_PASSWORD_STORAGE_KEY, nextPendingPassword)

    const nextOtpEmail = locationStateOtpEmail || otpEmail || readSessionValue(AUTH_OTP_EMAIL_STORAGE_KEY)
    if (nextOtpEmail !== otpEmail) schedule(() => setOtpEmail(nextOtpEmail))
    writeSessionValue(AUTH_OTP_EMAIL_STORAGE_KEY, nextOtpEmail)

    const nextResetToken = locationStateResetToken || resetToken || readSessionValue(AUTH_RESET_TOKEN_STORAGE_KEY)
    if (nextResetToken !== resetToken) schedule(() => setResetToken(nextResetToken))
    writeSessionValue(AUTH_RESET_TOKEN_STORAGE_KEY, nextResetToken)
  }, [
    location.key,
    locationStateChallengeId,
    locationStatePurpose,
    locationStatePendingPassword,
    locationStateOtpEmail,
    locationStateResetToken,
    otpChallengeId,
    otpPurpose,
    otpPendingPassword,
    otpEmail,
    resetToken,
  ])

  useEffect(() => {
    if (step !== 'error') return
    const nextMessage =
      locationStateErrorMessage || readSessionValue(AUTH_ERROR_MESSAGE_STORAGE_KEY) || DEFAULT_AUTH_ERROR_MESSAGE
    if (nextMessage !== authErrorMessage) schedule(() => setAuthErrorMessage(nextMessage))
    writeSessionValue(AUTH_ERROR_MESSAGE_STORAGE_KEY, nextMessage)
  }, [step, location.key, locationStateErrorMessage, authErrorMessage])

  useEffect(() => {
    if (step === 'error') return
    writeSessionValue(AUTH_ERROR_MESSAGE_STORAGE_KEY, '')
  }, [step])

  const validateEmail = (value: string) => {
    const text = value.trim()
    if (!text) return 'Email is required.'
    if (!EMAIL_PATTERN.test(text)) return 'Invalid email address.'
    return ''
  }

  const validatePasswordPolicy = (value: string) => {
    const text = value.trim()
    if (!text) return 'Password is required.'
    if (text.length < 12) return 'Password must be at least 12 characters.'
    if (text.length > 128) return 'Password must be at most 128 characters.'
    const hasUpper = /[A-Z]/.test(text)
    const hasLower = /[a-z]/.test(text)
    const hasDigit = /\d/.test(text)
    const hasSpecial = /[^A-Za-z0-9]/.test(text)
    if (!hasUpper || !hasLower || !hasDigit || !hasSpecial) {
      return 'Password must include upper, lower, digit, and special character.'
    }
    return ''
  }

  const requestOtpChallenge = async (purpose: OtpPurpose, pendingPassword = '', sourceEmail = stepEmail) => {
    const normalizedEmail = sourceEmail.trim()
    const otpChallenge = await sendOtp({ email: normalizedEmail, purpose })
    const otpContext = syncOtpContext({
      challengeId: otpChallenge.challengeId,
      purpose,
      pendingPassword,
      otpEmail: normalizedEmail,
    })
    return { normalizedEmail, otpContext }
  }

  const getReusableOtpContext = (purpose: OtpPurpose, sourceEmail = stepEmail, pendingPassword = '') => {
    const normalizedEmail = normalizeText(sourceEmail).toLowerCase()
    if (!normalizedEmail) return null

    const challengeId = normalizeText(otpChallengeId || readSessionValue(AUTH_OTP_CHALLENGE_STORAGE_KEY))
    const existingPurpose = normalizeOtpPurpose(otpPurpose || readSessionValue(AUTH_OTP_PURPOSE_STORAGE_KEY))
    const existingEmail = normalizeText(otpEmail || readSessionValue(AUTH_OTP_EMAIL_STORAGE_KEY)).toLowerCase()

    if (!challengeId || existingPurpose !== purpose || existingEmail !== normalizedEmail) {
      return null
    }

    const existingPendingPassword =
      normalizeText(otpPendingPassword || readSessionValue(AUTH_OTP_PENDING_PASSWORD_STORAGE_KEY)) || pendingPassword.trim()
    const otpContext = syncOtpContext({
      challengeId,
      purpose,
      pendingPassword: purpose === 'signup_password' ? existingPendingPassword : '',
      otpEmail: normalizedEmail,
    })
    return { normalizedEmail, otpContext }
  }

  const ensureOtpChallenge = async (purpose: OtpPurpose, pendingPassword = '', sourceEmail = stepEmail) => {
    const reused = getReusableOtpContext(purpose, sourceEmail, pendingPassword)
    if (reused) return reused
    return requestOtpChallenge(purpose, pendingPassword, sourceEmail)
  }

  const handleEntrySubmit = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (isEntrySubmitting) return

    const error = validateEmail(email)
    if (error) {
      setEmailErrorText(error)
      entryInputRef.current?.focus()
      return
    }

    const normalizedEmail = email.trim()
    setEmailErrorText('')
    setIsEntrySubmitting(true)
    try {
      await new Promise((resolve) => setTimeout(resolve, 250))
      const nextEmail = syncEmail(normalizedEmail)

      if (isCreateAccountEntry) {
        clearOtpContext()
        clearResetToken()
        navigate('/create-account/password', {
          state: getAuthState(nextEmail),
        })
        return
      }

      let passwordLogin = true
      try {
        const methods = await loginMethods({ email: nextEmail })
        passwordLogin = methods.passwordLogin
      } catch {
        navigate('/log-in/password', {
          state: getAuthState(nextEmail),
        })
        return
      }

      if (passwordLogin) {
        clearOtpContext()
        clearResetToken()
        navigate('/log-in/password', {
          state: getAuthState(nextEmail),
        })
        return
      }

      try {
        const { otpContext } = await ensureOtpChallenge('login_otp', '', nextEmail)
        navigate('/email-verification', {
          state: getAuthState(nextEmail, otpContext),
        })
      } catch (error) {
        setEmailErrorText(getApiErrorMessage(error, 'Failed to send verification code. Please try again.'))
        entryInputRef.current?.focus()
      }
    } finally {
      setIsEntrySubmitting(false)
    }
  }

  const handlePasswordSubmit = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (isPasswordSubmitting || isOtpSending) return

    if (!stepEmail.trim()) {
      setPasswordErrorText('Please enter your email first.')
      navigate('/log-in', { state: getAuthState('') })
      return
    }
    if (isCreateAccountPassword) {
      const error = validatePasswordPolicy(password)
      if (error) {
        setPasswordErrorText(error)
        passwordInputRef.current?.focus()
        return
      }
    } else if (!password.trim()) {
      setPasswordErrorText('Password is required.')
      passwordInputRef.current?.focus()
      return
    }

    const setSession = useSessionStore.getState().setSession
    setPasswordErrorText('')
    setIsPasswordSubmitting(true)
    try {
      if (isCreateAccountPassword) {
        const { normalizedEmail, otpContext } = await ensureOtpChallenge('signup_password', password.trim())
        setPassword('')
        navigate('/email-verification', {
          state: getAuthState(normalizedEmail, otpContext),
        })
        return
      }

      const auth = await login({ email: stepEmail.trim(), password: password.trim() })
      clearOtpContext()
      clearResetToken()
      setSession({
        user: {
          id: auth.user.id,
          email: auth.user.email,
          role: auth.user.role,
          status: auth.user.status,
        },
      })
      navigate('/chat')
    } catch (error) {
      const code = getApiErrorCode(error)
      if (!isCreateAccountPassword && code === 'AUTH_PASSWORD_NOT_SET') {
        try {
          const { normalizedEmail, otpContext } = await ensureOtpChallenge('login_otp')
          navigate('/email-verification', {
            state: getAuthState(normalizedEmail, otpContext),
          })
          return
        } catch (otpError) {
          setPasswordErrorText(getApiErrorMessage(otpError, 'Failed to send verification code. Please try again.'))
          passwordInputRef.current?.focus()
          return
        }
      }
      setPasswordErrorText(
        getApiErrorMessage(error, isCreateAccountPassword ? 'Failed to send verification code. Please try again.' : 'Login failed. Please try again.'),
      )
      passwordInputRef.current?.focus()
    } finally {
      setIsPasswordSubmitting(false)
    }
  }

  const resendEmail = async () => {
    if (isVerifySubmitting || isOtpSending) return
    if (!stepEmail.trim()) {
      setVerifyErrorText('Please enter your email first.')
      return
    }
    if (!otpPurpose) {
      setVerifyErrorText('Verification session expired. Please restart from login.')
      return
    }
    setVerifyErrorText('')
    setIsOtpSending(true)
    try {
      const { otpContext } = await requestOtpChallenge(otpPurpose, otpPendingPassword)
      navigate('/email-verification', {
        replace: true,
        state: getAuthState(stepEmail, otpContext),
      })
      lastSubmittedVerifyCodeRef.current = ''
      setOtp(['', '', '', '', '', ''])
      otpRefs.current[0]?.focus()
    } catch (error) {
      setVerifyErrorText(getApiErrorMessage(error, 'Failed to resend verification code. Please try again.'))
    } finally {
      setIsOtpSending(false)
    }
  }

  const handlePasswordlessOtpAction = async () => {
    if (isPasswordSubmitting || isOtpSending) return

    const normalizedEmail = stepEmail.trim()
    if (!normalizedEmail) {
      setPasswordErrorText('Please enter your email first.')
      navigate('/log-in', { state: getAuthState('') })
      return
    }

    const purpose: OtpPurpose = isCreateAccountPassword ? 'signup_otp' : 'login_otp'
    setPasswordErrorText('')
    setIsOtpSending(true)
    try {
      const { otpContext } = await ensureOtpChallenge(purpose, '', normalizedEmail)
      navigate('/email-verification', {
        state: getAuthState(normalizedEmail, otpContext),
      })
    } catch (error) {
      setPasswordErrorText(getApiErrorMessage(error, 'Failed to send verification code. Please try again.'))
      passwordInputRef.current?.focus()
    } finally {
      setIsOtpSending(false)
    }
  }

  const updateOtpAt = (index: number, rawValue: string) => {
    lastSubmittedVerifyCodeRef.current = ''
    const value = rawValue.replace(/\D/g, '').slice(-1)
    if (verifyErrorText) setVerifyErrorText('')
    setOtp((prev) => {
      const next = [...prev]
      next[index] = value
      autoSubmitOtpIfComplete(next)
      return next
    })
    if (value && index < 5) {
      otpRefs.current[index + 1]?.focus()
    }
  }

  const handleOtpKeyDown = (index: number, key: string) => {
    if (key === 'Backspace' && !otp[index] && index > 0) {
      otpRefs.current[index - 1]?.focus()
    }
    if (key === 'ArrowLeft' && index > 0) {
      otpRefs.current[index - 1]?.focus()
    }
    if (key === 'ArrowRight' && index < 5) {
      otpRefs.current[index + 1]?.focus()
    }
  }

  const submitVerifyCode = async (code: string) => {
    if (isVerifySubmitting || isOtpSending) return

    const normalizedEmail = stepEmail.trim()
    if (!normalizedEmail) {
      setVerifyErrorText('Please enter your email first.')
      return
    }
    if (!otpChallengeId || !otpPurpose) {
      setVerifyErrorText('Verification session expired. Please restart login or sign up.')
      return
    }
    if (code.length !== 6) {
      setVerifyErrorText('Please enter the 6-digit verification code.')
      return
    }

    const passwordForSignupPassword = otpPurpose === 'signup_password' ? otpPendingPassword.trim() : ''
    if (otpPurpose === 'signup_password' && !passwordForSignupPassword) {
      setVerifyErrorText('Sign-up session expired. Please set password again.')
      return
    }

    if (otpPurpose === 'reset_password') {
      setVerifyErrorText('')
      setIsVerifySubmitting(true)
      try {
        const result = await verifyPasswordReset({
          challengeId: otpChallengeId,
          email: normalizedEmail,
          code,
        })
        clearOtpContext()
        const token = syncResetToken(result.resetToken)
        navigate('/reset-password/new-password', {
          state: getAuthState(normalizedEmail, { resetToken: token }),
        })
      } catch (error) {
        setVerifyErrorText(getApiErrorMessage(error, 'Verification failed. Please try again.'))
      } finally {
        setIsVerifySubmitting(false)
      }
      return
    }

    const setSession = useSessionStore.getState().setSession
    setVerifyErrorText('')
    setIsVerifySubmitting(true)
    try {
      const auth = await verifyOtp({
        challengeId: otpChallengeId,
        email: normalizedEmail,
        purpose: otpPurpose,
        code,
        password: passwordForSignupPassword || undefined,
      })
      clearOtpContext()
      clearResetToken()
      setSession({
        user: {
          id: auth.user.id,
          email: auth.user.email,
          role: auth.user.role,
          status: auth.user.status,
        },
      })
      navigate('/chat')
    } catch (error) {
      setVerifyErrorText(getApiErrorMessage(error, 'Verification failed. Please try again.'))
    } finally {
      setIsVerifySubmitting(false)
    }
  }

  const autoSubmitOtpIfComplete = (digits: string[]) => {
    if (step !== 'verify') return
    if (isVerifySubmitting || isOtpSending) return
    const code = digits.join('')
    if (code.length !== 6) return
    if (lastSubmittedVerifyCodeRef.current === code) return
    lastSubmittedVerifyCodeRef.current = code
    void submitVerifyCode(code)
  }

  const handleOtpPaste = (event: ClipboardEvent<HTMLInputElement>) => {
    event.preventDefault()
    const pasted = event.clipboardData.getData('text').replace(/\D/g, '').slice(0, 6)
    if (!pasted) return

    lastSubmittedVerifyCodeRef.current = ''
    if (verifyErrorText) setVerifyErrorText('')
    const filled = ['','','','','','']
    for (let i = 0; i < pasted.length; i += 1) filled[i] = pasted[i]
    setOtp(filled)
    autoSubmitOtpIfComplete(filled)

    const nextIndex = Math.min(pasted.length, 5)
    otpRefs.current[nextIndex]?.focus()
  }

  const handleResetPasswordSubmit = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (isResetSubmitting) return

    const normalizedEmail = stepEmail.trim()
    if (!normalizedEmail) {
      navigate('/log-in', {
        state: {
          ...getAuthState(''),
          errorMessage: 'Please enter your email first.',
        } satisfies AuthNavigationState,
      })
      return
    }

    setIsResetSubmitting(true)
    try {
      clearResetToken()
      const { otpContext } = await requestOtpChallenge('reset_password', '', normalizedEmail)
      navigate('/email-verification', {
        state: getAuthState(normalizedEmail, otpContext),
      })
    } catch (error) {
      navigate('/log-in/error', {
        state: {
          ...getAuthState(normalizedEmail),
          errorMessage: getApiErrorMessage(error, 'Failed to send verification code. Please try again.'),
        } satisfies AuthNavigationState,
      })
    } finally {
      setIsResetSubmitting(false)
    }
  }

  const handleVerifySubmit = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    const code = otp.join('')
    lastSubmittedVerifyCodeRef.current = code
    await submitVerifyCode(code)
  }

  const handleResetNewPasswordSubmit = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (isResetNewSubmitting) return

    let hasError = false
    let nextPasswordError = ''
    const nextPassword = newPassword.trim()
    const previousPassword = password.trim()

    if (!nextPassword) {
      nextPasswordError = 'New password is required.'
    } else if (previousPassword && nextPassword === previousPassword) {
      nextPasswordError = 'Password must be different from your previous password.'
    }
    if (!nextPasswordError) {
      const policyError = validatePasswordPolicy(nextPassword)
      // keep error copy consistent with "new password" context
      if (policyError) {
        nextPasswordError = policyError
      }
    }
    if (nextPasswordError) hasError = true
    setNewPasswordErrorText(nextPasswordError)

    const nextConfirmError = confirmPassword.trim() ? '' : 'Please re-enter your new password.'
    if (nextConfirmError) hasError = true
    setConfirmPasswordErrorText(nextConfirmError)

    if (hasError) return

    if (newPassword !== confirmPassword) {
      setConfirmPasswordErrorText('Passwords do not match.')
      return
    }

    const normalizedEmail = stepEmail.trim()
    const activeResetToken = normalizeText(resetToken || readSessionValue(AUTH_RESET_TOKEN_STORAGE_KEY))
    if (!normalizedEmail || !activeResetToken) {
      navigate('/log-in/error', {
        state: {
          ...getAuthState(normalizedEmail),
          errorMessage: 'Password reset session expired. Please restart verification.',
        } satisfies AuthNavigationState,
      })
      return
    }

    setIsResetNewSubmitting(true)
    try {
      await completePasswordReset({
        email: normalizedEmail,
        resetToken: activeResetToken,
        newPassword: nextPassword,
      })
      clearResetToken()
      setNewPassword('')
      setConfirmPassword('')
      navigate('/reset-password/success', {
        state: getAuthState(normalizedEmail),
      })
    } catch (error) {
      setConfirmPasswordErrorText(getApiErrorMessage(error, 'Failed to reset password. Please try again.'))
    } finally {
      setIsResetNewSubmitting(false)
    }
  }

  const renderSocialButtons = () => (
    <div className="auth-social-group" role="group" aria-label="选择登录选项">
      <button type="button" className="auth-social-btn">
        <span className="auth-social-icon">
          <img src={googleIconSvg} alt="" aria-hidden="true" />
        </span>
        <span>继续使用 Google 登录</span>
      </button>
      <button type="button" className="auth-social-btn">
        <span className="auth-social-icon">
          <img src={appleIconSvg} alt="" aria-hidden="true" />
        </span>
        <span>继续使用 Apple 登录</span>
      </button>
      <button type="button" className="auth-social-btn">
        <span className="auth-social-icon">
          <img src={microsoftIconSvg} alt="" aria-hidden="true" />
        </span>
        <span>继续使用 Microsoft 登录</span>
      </button>
      <button type="button" className="auth-social-btn">
        <span className="auth-social-icon">
          <img src={phoneIconSvg} alt="" aria-hidden="true" />
        </span>
        <span>继续使用手机登录</span>
      </button>
    </div>
  )

  return (
    <div className="auth-page">
      <main className="auth-main">
        <section className={`auth-card${step === 'error' ? ' auth-card-wide' : ''}`} aria-label="登录卡片">
          {step === 'entry' ? (
            <>
              <div className="auth-title-block">
                <Link to={logoLinkTarget} className="auth-wordmark" aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className="auth-wordmark-img" />
                </Link>
                <h1 className="auth-heading">
                  <span className="auth-heading-text">{isCreateAccountEntry ? '创建帐户' : '欢迎回来'}</span>
                </h1>
              </div>

              <fieldset className="auth-fieldset">
                <form className="auth-form" onSubmit={handleEntrySubmit} noValidate>
                  <div className="auth-section auth-section-ctas">
                    {renderSocialButtons()}
                    <div className="auth-divider">
                      <div className="auth-divider-line" />
                      <div className="auth-divider-name">或</div>
                      <div className="auth-divider-line" />
                    </div>
                  </div>

                  <div className="auth-section auth-section-fields">
                    <div className="auth-textfield" data-rac="" data-invalid={isEmailInvalid || undefined}>
                      <div className="auth-textfield-root">
                        <div className={emailWrapClassName}>
                          <label className="auth-input-label" htmlFor={emailId} id={emailLabelId}>
                            <div className="auth-input-label-pos">
                              <div className="auth-input-label-text">电子邮件地址</div>
                            </div>
                          </label>
                          <input
                            ref={entryInputRef}
                            className="auth-input-target"
                            id={emailId}
                            name="email"
                            type="email"
                            autoComplete="email"
                            placeholder="电子邮件地址"
                            value={email}
                            aria-labelledby={emailLabelId}
                            aria-describedby={isEmailInvalid ? emailErrorId : undefined}
                            aria-invalid={isEmailInvalid || undefined}
                            disabled={isEntrySubmitting}
                            onFocus={() => setIsEmailFocused(true)}
                            onBlur={() => setIsEmailFocused(false)}
                            onChange={(e) => {
                              setEmail(e.target.value)
                              if (isEmailInvalid) setEmailErrorText('')
                            }}
                          />
                        </div>
                        <span className="auth-input-live" aria-live="polite" aria-atomic="true">
                          {isEmailInvalid ? (
                            <span className="auth-field-error-slot" id={emailErrorId}>
                              <ul className="auth-field-errors">
                                <li className="auth-field-error">
                                  <span className="auth-field-error-icon">
                                    <img src={errorIconSvg} alt="" aria-hidden="true" />
                                  </span>
                                  <span>{emailErrorText}</span>
                                </li>
                              </ul>
                            </span>
                          ) : null}
                        </span>
                      </div>
                    </div>
                  </div>

                  <div className="auth-section auth-section-ctas">
                    <button type="submit" className="auth-continue-btn" disabled={isEntrySubmitting}>
                      继续
                    </button>
                    {isCreateAccountEntry ? (
                      <span className="auth-signup-hint">
                        已经有帐户？请
                        <Link to="/log-in" state={getAuthState(email)}>
                          登录
                        </Link>
                      </span>
                    ) : (
                      <span className="auth-signup-hint">
                        还没有帐户？请
                        <Link to="/create-account">注册</Link>
                      </span>
                    )}
                  </div>
                </form>
              </fieldset>
            </>
          ) : null}

          {step === 'password' ? (
            <>
              <div className="auth-title-block">
                <Link to={logoLinkTarget} className="auth-wordmark" aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className="auth-wordmark-img" />
                </Link>
                <h1 className="auth-heading">
                  <span className="auth-heading-text">{isCreateAccountPassword ? '创建密码' : '输入密码'}</span>
                </h1>
              </div>

              <fieldset className="auth-fieldset">
                <form
                  className="auth-form auth-form-password"
                  method="post"
                  action={isCreateAccountPassword ? '/create-account/password' : '/log-in/password'}
                  autoComplete="on"
                  onSubmit={handlePasswordSubmit}
                  noValidate
                >
                  <div className="auth-section auth-section-fields">
                    <input type="hidden" name="username" value={stepEmail} />
                    <div className="auth-field-stack">
                      <div className="auth-textfield-root">
                        <div className="auth-input-wrap has-value is-active auth-input-wrap-with-end auth-input-wrap-readonly">
                          <label className="auth-input-label" htmlFor="readonly-email" id={readonlyEmailLabelId}>
                            <div className="auth-input-label-pos">
                              <div className="auth-input-label-text">电子邮件地址</div>
                            </div>
                          </label>
                          <input
                            id="readonly-email"
                            className="auth-input-target"
                            type="text"
                            value={stepEmail}
                            readOnly
                            placeholder="电子邮件地址"
                            aria-labelledby={readonlyEmailLabelId}
                          />
                          <div className="auth-input-end-decoration">
                            <div className="auth-link-nowrap">
                              <Link
                                to="/log-in"
                                state={getAuthState(stepEmail)}
                                className="auth-link-inline"
                                aria-label="编辑电子邮件"
                              >
                                编辑
                              </Link>
                            </div>
                          </div>
                        </div>
                      </div>

                      <div className="auth-textfield-root">
                        <div className={passwordWrapClassName}>
                          <label className="auth-input-label" htmlFor={passwordId} id={passwordLabelId}>
                            <div className="auth-input-label-pos">
                              <div className="auth-input-label-text">密码</div>
                            </div>
                          </label>
                          <input
                            ref={passwordInputRef}
                            id={passwordId}
                            className="auth-input-target"
                            type={passwordVisible ? 'text' : 'password'}
                            value={password}
                            name="current-password"
                            autoComplete="current-password webauthn"
                            placeholder="密码"
                            aria-labelledby={passwordLabelId}
                            aria-describedby={isPasswordInvalid ? passwordErrorId : undefined}
                            aria-invalid={isPasswordInvalid || undefined}
                            disabled={isPasswordSubmitting}
                            onFocus={() => setIsPasswordFocused(true)}
                            onBlur={() => setIsPasswordFocused(false)}
                            onChange={(e) => {
                              setPassword(e.target.value)
                              if (isPasswordInvalid) setPasswordErrorText('')
                            }}
                          />
                          <button
                            type="button"
                            className="auth-password-toggle"
                            aria-label={passwordVisible ? '隐藏密码' : '显示密码'}
                            aria-controls={passwordId}
                            aria-pressed={passwordVisible}
                            onClick={() => setPasswordVisible((prev) => !prev)}
                          >
                            {passwordVisible ? (
                              <img src={eyeOffIconSvg} alt="" aria-hidden="true" />
                            ) : (
                              <img src={eyeIconSvg} alt="" aria-hidden="true" />
                            )}
                          </button>
                        </div>
                        {isPasswordInvalid ? (
                          <ul className="auth-field-errors" id={passwordErrorId}>
                            <li className="auth-field-error">
                              <span className="auth-field-error-icon">
                                <img src={errorIconSvg} alt="" aria-hidden="true" />
                              </span>
                              <span>{passwordErrorText}</span>
                            </li>
                          </ul>
                        ) : null}
                      </div>

                      {!isCreateAccountPassword ? (
                        <span className="auth-forgot-password">
                          <Link to="/reset-password" state={getAuthState(stepEmail)}>
                            忘记了密码？
                          </Link>
                        </span>
                      ) : null}
                    </div>
                  </div>

                  <div className="auth-section auth-section-ctas auth-section-password-ctas">
                    <div className="auth-button-wrapper">
                      <button type="submit" className="auth-continue-btn" disabled={isPasswordSubmitting || isOtpSending}>
                        继续
                      </button>
                    </div>

                    {!isCreateAccountPassword ? (
                      <span className="auth-signup-hint">
                        还没有帐户？请
                        <Link to="/create-account">注册</Link>
                      </span>
                    ) : null}

                    <div className="auth-divider auth-divider-password">
                      <div className="auth-divider-line" />
                      <div className="auth-divider-name">或</div>
                      <div className="auth-divider-line" />
                    </div>

                    <div className="auth-passwordless-group">
                      <div className="auth-button-wrapper">
                        <button
                          type="button"
                          className="auth-outline-btn auth-inline-passwordless-login"
                          disabled={isPasswordSubmitting || isOtpSending}
                          onClick={handlePasswordlessOtpAction}
                        >
                          {isCreateAccountPassword ? '使用一次性验证码注册' : '使用一次性验证码登录'}
                        </button>
                      </div>
                    </div>

                    {isCreateAccountPassword ? (
                      <span className="auth-signup-hint">
                        已经有帐户了？请
                        <Link to="/log-in" state={getAuthState(stepEmail)}>
                          登录
                        </Link>
                      </span>
                    ) : null}
                  </div>
                </form>
              </fieldset>
            </>
          ) : null}

          {step === 'verify' ? (
            <>
              <div className="auth-title-block">
                <Link to={logoLinkTarget} className="auth-wordmark" aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className="auth-wordmark-img" />
                </Link>
                <h1 className="auth-heading">
                  <span className="auth-heading-text">检查您的收件箱</span>
                </h1>
                <div className="auth-subtitle auth-subtitle-verify">
                  <span className="auth-subtitle-text" id={verifySubtitleId}>
                    输入我们刚刚向 {stepEmail || '你的邮箱'} 发送的验证码
                  </span>
                </div>
              </div>

              <fieldset className="auth-fieldset">
                <form className="auth-form auth-form-verify" noValidate onSubmit={handleVerifySubmit}>
                  <div className="auth-section auth-section-fields">
                    <div className="auth-otp-wrap">
                      <div
                        className="auth-otp-group"
                        role="group"
                        aria-label="验证码"
                        aria-describedby={verifyDescribedBy}
                        data-variant="outlined"
                        data-error={verifyErrorText ? 'true' : undefined}
                      >
                        {otp.map((digit, index) => (
                          <input
                            key={index}
                            ref={(node) => {
                              otpRefs.current[index] = node
                            }}
                            className="auth-otp-input"
                            type="text"
                            inputMode="numeric"
                            autoComplete={index === 0 ? 'one-time-code' : 'off'}
                            maxLength={1}
                            aria-label={`数字位 ${index + 1}`}
                            aria-describedby={verifyDescribedBy}
                            value={digit}
                            disabled={isVerifySubmitting || isOtpSending}
                            onChange={(e) => updateOtpAt(index, e.target.value)}
                            onKeyDown={(e) => handleOtpKeyDown(index, e.key)}
                            onPaste={handleOtpPaste}
                          />
                        ))}
                      </div>
                      <input type="hidden" readOnly value={otp.join('')} name="code" />
                      {verifyErrorText ? (
                        <ul className="auth-field-errors" id={verifyErrorId}>
                          <li className="auth-field-error">
                            <span className="auth-field-error-icon">
                              <img src={errorIconSvg} alt="" aria-hidden="true" />
                            </span>
                            <span>{verifyErrorText}</span>
                          </li>
                        </ul>
                      ) : null}
                    </div>
                  </div>

                  <div className="auth-section auth-section-ctas auth-section-verify-ctas">
                    <div className="auth-button-wrapper">
                      <button type="button" className="auth-outline-btn" disabled={isVerifySubmitting || isOtpSending} onClick={resendEmail}>
                        重新发送电子邮件
                      </button>
                    </div>
                    <div className="auth-button-wrapper">
                      <Link className="auth-link-btn" to={passwordContinuePath} state={getAuthState(stepEmail)}>
                        使用密码继续
                      </Link>
                    </div>
                  </div>
                </form>
              </fieldset>
            </>
          ) : null}

          {step === 'reset' ? (
            <>
              <div className="auth-title-block">
                <Link to={logoLinkTarget} className="auth-wordmark" aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className="auth-wordmark-img" />
                </Link>
                <h1 className="auth-heading">
                  <span className="auth-heading-text">重置密码</span>
                </h1>
                <div className="auth-subtitle">
                  <span className="auth-subtitle-text">
                    点击“继续”以重置 {stepEmail || '你的邮箱'} 的密码
                  </span>
                </div>
              </div>

              <fieldset className="auth-fieldset">
                <form
                  className="auth-form auth-form-reset"
                  method="post"
                  action="/reset-password"
                  onSubmit={handleResetPasswordSubmit}
                >
                  <div className="auth-section auth-section-fields">
                    <div className="auth-reset-actions">
                      <div className="auth-button-wrapper">
                        <button type="submit" className="auth-continue-btn" disabled={isResetSubmitting}>
                          继续
                        </button>
                      </div>
                      <div className="auth-button-wrapper">
                        <button
                          type="button"
                          className="auth-transparent-btn"
                          onClick={() => navigate('/log-in', { state: getAuthState(stepEmail) })}
                        >
                          返回登录
                        </button>
                      </div>
                    </div>
                  </div>
                </form>
              </fieldset>
            </>
          ) : null}

          {step === 'resetNew' ? (
            <>
              <div className="auth-title-block">
                <Link to={logoLinkTarget} className="auth-wordmark" aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className="auth-wordmark-img" />
                </Link>
                <h1 className="auth-heading">
                  <span className="auth-heading-text">重置密码</span>
                </h1>
                <div className="auth-subtitle">
                  <span className="auth-subtitle-text">请在下面输入新密码以更改密码</span>
                </div>
              </div>

              <fieldset className="auth-fieldset">
                <form
                  className="auth-form"
                  method="post"
                  action="/reset-password/new-password"
                  autoComplete="on"
                  noValidate
                  onSubmit={handleResetNewPasswordSubmit}
                >
                  <div className="auth-section auth-section-fields">
                    <input type="hidden" name="username" value={stepEmail} autoComplete="username" />
                    <div className="auth-field-stack">
                      <div className="auth-textfield-root">
                        <div className={newPasswordWrapClassName}>
                          <label className="auth-input-label" htmlFor={newPasswordId} id={newPasswordLabelId}>
                            <div className="auth-input-label-pos">
                              <div className="auth-input-label-text">新密码</div>
                            </div>
                          </label>
                          <input
                            id={newPasswordId}
                            className="auth-input-target"
                            type={isNewPasswordVisible ? 'text' : 'password'}
                            value={newPassword}
                            name="new-password"
                            autoComplete="new-password"
                            spellCheck={false}
                            placeholder="新密码"
                            aria-labelledby={newPasswordLabelId}
                            aria-describedby={isNewPasswordInvalid ? newPasswordErrorId : undefined}
                            aria-invalid={isNewPasswordInvalid || undefined}
                            disabled={isResetNewSubmitting}
                            onFocus={() => setIsNewPasswordFocused(true)}
                            onBlur={() => setIsNewPasswordFocused(false)}
                            onChange={(e) => {
                              setNewPassword(e.target.value)
                              if (isNewPasswordInvalid) setNewPasswordErrorText('')
                              if (isConfirmPasswordInvalid) setConfirmPasswordErrorText('')
                            }}
                          />
                          <button
                            type="button"
                            className="auth-password-toggle"
                            aria-label={isNewPasswordVisible ? '隐藏密码' : '显示密码'}
                            aria-controls={newPasswordId}
                            aria-pressed={isNewPasswordVisible}
                            onClick={() => setIsNewPasswordVisible((prev) => !prev)}
                          >
                            {isNewPasswordVisible ? (
                              <img src={eyeOffIconSvg} alt="" aria-hidden="true" />
                            ) : (
                              <img src={eyeIconSvg} alt="" aria-hidden="true" />
                            )}
                          </button>
                        </div>
                        {isNewPasswordInvalid ? (
                          <ul className="auth-field-errors" id={newPasswordErrorId}>
                            <li className="auth-field-error">
                              <span className="auth-field-error-icon">
                                <img src={errorIconSvg} alt="" aria-hidden="true" />
                              </span>
                              <span>{newPasswordErrorText}</span>
                            </li>
                          </ul>
                        ) : null}
                      </div>

                      <div className="auth-textfield-root">
                        <div className={confirmPasswordWrapClassName}>
                          <label className="auth-input-label" htmlFor={confirmPasswordId} id={confirmPasswordLabelId}>
                            <div className="auth-input-label-pos">
                              <div className="auth-input-label-text">重新输入新密码</div>
                            </div>
                          </label>
                          <input
                            id={confirmPasswordId}
                            className="auth-input-target"
                            type={isConfirmPasswordVisible ? 'text' : 'password'}
                            value={confirmPassword}
                            name="confirm-password"
                            autoComplete="new-password"
                            spellCheck={false}
                            placeholder="重新输入新密码"
                            aria-labelledby={confirmPasswordLabelId}
                            aria-describedby={isConfirmPasswordInvalid ? confirmPasswordErrorId : undefined}
                            aria-invalid={isConfirmPasswordInvalid || undefined}
                            disabled={isResetNewSubmitting}
                            onFocus={() => setIsConfirmPasswordFocused(true)}
                            onBlur={() => setIsConfirmPasswordFocused(false)}
                            onChange={(e) => {
                              setConfirmPassword(e.target.value)
                              if (isConfirmPasswordInvalid) setConfirmPasswordErrorText('')
                            }}
                          />
                          <button
                            type="button"
                            className="auth-password-toggle"
                            aria-label={isConfirmPasswordVisible ? '隐藏密码' : '显示密码'}
                            aria-controls={confirmPasswordId}
                            aria-pressed={isConfirmPasswordVisible}
                            onClick={() => setIsConfirmPasswordVisible((prev) => !prev)}
                          >
                            {isConfirmPasswordVisible ? (
                              <img src={eyeOffIconSvg} alt="" aria-hidden="true" />
                            ) : (
                              <img src={eyeIconSvg} alt="" aria-hidden="true" />
                            )}
                          </button>
                        </div>
                        {isConfirmPasswordInvalid ? (
                          <ul className="auth-field-errors" id={confirmPasswordErrorId}>
                            <li className="auth-field-error">
                              <span className="auth-field-error-icon">
                                <img src={errorIconSvg} alt="" aria-hidden="true" />
                              </span>
                              <span>{confirmPasswordErrorText}</span>
                            </li>
                          </ul>
                        ) : null}
                      </div>
                    </div>
                  </div>

                  <div className="auth-section auth-section-ctas">
                    <div className="auth-button-wrapper">
                      <button type="submit" className="auth-continue-btn" disabled={isResetNewSubmitting}>
                        继续
                      </button>
                    </div>
                  </div>
                </form>
              </fieldset>
            </>
          ) : null}

          {step === 'resetSuccess' ? (
            <>
              <div className="auth-title-block">
                <img
                  src={successCheckmarkCircleSvg}
                  alt=""
                  aria-hidden="true"
                  width={60}
                  height={60}
                  className="auth-success-mark"
                />
                <h1 className="auth-heading">
                  <span className="auth-heading-text">密码已更改</span>
                </h1>
                <div className="auth-subtitle">
                  <span className="auth-subtitle-text">你的密码已成功更改</span>
                </div>
              </div>

              <fieldset className="auth-fieldset">
                <form method="get" action="/reset-password/success" className="auth-form">
                  <div className="auth-section auth-section-fields" />
                  <div className="auth-section auth-section-ctas">
                    <div className="auth-button-wrapper">
                      <Link to="/log-in/password" className="auth-continue-btn">
                        登录
                      </Link>
                    </div>
                  </div>
                </form>
              </fieldset>
            </>
          ) : null}

          {step === 'error' ? (
            <>
              <div className="auth-title-block">
                <a href="https://chatgpt.com" aria-label="OpenAI 主页面" className="auth-logo-link">
                  <img
                    src={onebookLogoMarkSvg}
                    alt="OneBook 徽标"
                    width={48}
                    height={48}
                    className="auth-logo-mark"
                  />
                </a>
                <h1 className="auth-heading">
                  <span className="auth-heading-text">糟糕，出错了！</span>
                </h1>
                <div className="auth-subtitle">
                  <div className="auth-subtitle-error-card">{authErrorMessage}</div>
                </div>
              </div>
              <div className="auth-button-wrapper">
                <button
                  type="button"
                  className="auth-outline-btn"
                  data-dd-action-name="Try again"
                  onClick={() => {
                    writeSessionValue(AUTH_ERROR_MESSAGE_STORAGE_KEY, '')
                    navigate('/log-in', { replace: true })
                  }}
                >
                  重试
                </button>
              </div>
            </>
          ) : null}

          <div className="auth-footer">
            <span className="auth-footer-meta">
              <a href="https://openai.com/policies/terms-of-use">
                使用条款
              </a>
              <span className="auth-footer-separator" aria-hidden="true" />
              <a href="https://openai.com/policies/privacy-policy">
                隐私政策
              </a>
            </span>
          </div>
        </section>
      </main>
    </div>
  )
}
