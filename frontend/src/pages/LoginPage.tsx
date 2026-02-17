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

const cx = (...values: Array<string | false | null | undefined>) => values.filter(Boolean).join(' ')

type FloatingInputOptions = {
  isActive: boolean
  hasValue: boolean
  isFocused: boolean
  isInvalid: boolean
  isSubmitting: boolean
  withEnd?: boolean
  isReadonly?: boolean
}

const focusRingClass =
  'focus-visible:outline-0 focus-visible:shadow-[0_0_0_2px_#fff,0_0_0_4px_rgb(155,155,155)]'

const loginTw = {
  page:
    'mx-auto flex h-dvh w-full items-center justify-center bg-white pt-10 text-[#0d0d0d] tracking-[-0.01em] [color-scheme:light] min-[450px]:h-auto min-[450px]:justify-start min-[450px]:pt-[15vh]',
  main: 'contents',
  cardBase:
    'flex h-full w-full flex-col items-stretch px-4 text-center min-[450px]:mx-auto min-[450px]:max-w-[calc(21.25rem+2rem)]',
  cardWide: 'min-[450px]:max-w-[calc(28.125rem+2rem)]',
  titleBlock: 'mb-8',
  wordmark: 'text-inherit visited:text-inherit',
  wordmarkImg:
    'mx-auto mb-2 block h-8 w-auto min-[450px]:h-12 min-[800px]:fixed min-[800px]:left-2 min-[800px]:top-2',
  heading: 'm-0',
  headingText: 'm-0 inline-block text-[2rem] leading-10 font-medium tracking-[-0.02em] text-[#0d0d0d]',
  fieldset: 'm-0 contents min-w-0 border-0 p-0',
  form: 'm-0 flex flex-col items-stretch',
  sectionCtas: 'flex flex-col gap-6 py-6',
  sectionFields: 'flex grow flex-col gap-6',
  stack: 'flex flex-col items-stretch gap-3',
  buttonRow: 'flex flex-col gap-1',
  textPrimary: 'text-[1rem] leading-6 font-normal text-[#0d0d0d]',
  textSecondary: 'text-[1rem] leading-6 font-normal text-[#5d5d5d]',
  hintLink: 'ml-[2px] text-[#3e68ff] no-underline hover:underline',
  socialGroup: 'mt-[-1.5rem] flex flex-col items-stretch gap-3',
  socialButton:
    'inline-flex h-[3.25rem] w-full cursor-pointer items-center justify-start rounded-[99999px] border border-[rgb(0_0_0/0.15)] bg-transparent px-6 text-left text-[1rem] leading-6 font-normal text-[#0d0d0d] transition-colors duration-100 hover:bg-[#ececec] active:opacity-80 disabled:cursor-not-allowed disabled:opacity-50',
  socialIcon: 'mr-4 inline-flex h-[18px] w-[18px] shrink-0 items-center justify-center',
  socialIconImg: 'block h-auto max-h-full w-auto max-w-full',
  divider: 'grid grid-cols-[1fr_max-content_1fr] items-center',
  dividerLine: 'h-px bg-[#ececec]',
  dividerName: 'mx-4 text-[13px] font-[510] uppercase text-[#5d5d5d]',
  inlineErrorContainer: 'inline',
  errorList: 'm-0 p-0',
  errorItem: 'mb-3 mt-1 flex items-center gap-2 text-left text-[0.75rem] leading-[1.4] text-[#d00e17] [&>*]:shrink-0',
  otpErrorItem: 'mb-0 mt-2 flex items-center gap-2 text-left text-[0.75rem] leading-[1.4] text-[#d00e17] [&>*]:shrink-0',
  errorIcon: 'inline-flex items-center justify-center',
  errorIconImg: 'block h-4 w-4',
  primaryBtn:
    'inline-flex h-[3.25rem] w-full cursor-pointer items-center justify-center rounded-[99999px] border-0 bg-[#131313] px-6 text-[1rem] leading-6 font-normal text-white transition-colors duration-100 hover:bg-[#333333] active:opacity-80 disabled:cursor-not-allowed disabled:opacity-50',
  outlineBtn:
    'inline-flex h-[3.25rem] w-full cursor-pointer items-center justify-center rounded-[99999px] border border-[rgb(0_0_0/0.15)] bg-transparent px-6 text-[1rem] leading-6 font-normal text-[#0d0d0d] no-underline transition-colors duration-100 hover:bg-[#ececec] active:opacity-80 disabled:cursor-not-allowed disabled:opacity-50',
  linkBtn:
    'inline-flex h-auto w-fit items-center justify-center self-center rounded-none border-0 bg-transparent p-0 text-[#0d0d0d] no-underline transition-colors duration-100 hover:text-[#555555] active:opacity-80',
  transparentBtn:
    'mx-auto my-2 inline-flex h-auto w-fit cursor-pointer items-center justify-center rounded-none border-0 bg-transparent p-0 text-[1rem] leading-6 font-normal text-[#0d0d0d] transition-colors duration-100 hover:text-[#555555] active:opacity-80 disabled:cursor-not-allowed disabled:opacity-50',
  forgotWrap: 'pl-3 text-left text-[0.875rem] leading-5 text-[#0d0d0d]',
  forgotLink: 'text-[#3e68ff] no-underline hover:underline',
  toggleBtn:
    "relative isolate inline-flex cursor-pointer items-center justify-center border-0 bg-transparent p-0 text-inherit before:absolute before:inset-[-0.5rem] before:-z-[1] before:rounded-[99999px] before:content-[''] hover:before:bg-[#ececec] focus-visible:before:bg-[#ececec] active:opacity-80",
  eyeIcon: 'block h-5 w-5',
  subtitle: 'mt-3',
  fieldContainer: 'w-full',
  relative: 'relative',
  noWrap: 'whitespace-nowrap',
  editLink: 'text-[1rem] leading-6 text-[#3e68ff] no-underline hover:underline',
  stackTight: 'flex flex-col gap-3',
  otpWrap: 'flex w-full flex-col items-start gap-2',
  otpGrid: 'grid w-full max-w-[336px] grid-cols-6 gap-2',
  otpInput:
    'h-[3.25rem] rounded-xl border bg-white text-center text-[1.25rem] font-medium text-[#0d0d0d] outline-none transition-[border-color,background-color,box-shadow] duration-150 max-[450px]:h-[clamp(2.5rem,20vw,3.5rem)] max-[450px]:w-[clamp(1.75rem,18vw,2.75rem)] max-[450px]:text-[clamp(1.2rem,7vw,1.7rem)]',
  otpInputInvalid: 'border-[#d00e17] focus:border-[#d00e17] focus:shadow-[0_0_0_1px_#d00e17]',
  otpInputNormal: 'border-[#cdcdcd] focus:border-[#3e68ff] focus:shadow-[0_0_0_1px_#3e68ff]',
  inputEndDecoration: "flex shrink-0 before:block before:w-2 before:content-['']",
  successIcon: 'mb-6',
  underlineLink: 'underline',
  footer: 'mb-4 mt-auto text-center min-[450px]:mt-6',
  footerMeta: 'text-[0.875rem] leading-5 font-normal text-[#5d5d5d]',
  footerSep: 'px-2 text-[1rem] leading-6',
  logoLinkError: 'text-inherit no-underline visited:text-inherit',
  logoMarkError: 'mx-auto mb-6 block h-12 w-12',
  subtitleErrorCard: 'mt-4 whitespace-pre-wrap rounded-[8px] bg-[#f5f5f5] p-4 text-left',
} as const

function getFloatingInputClasses(options: FloatingInputOptions) {
  const shouldFloat = options.isActive || options.hasValue || options.isInvalid || Boolean(options.isReadonly)
  const borderColor = options.isInvalid
    ? 'border-[#d00e17]'
    : options.isFocused
      ? 'border-[#3e68ff]'
      : options.isSubmitting
        ? 'border-[#b4b4b4]'
        : 'border-[rgb(0_0_0/0.15)]'
  const labelColor = options.isInvalid
    ? 'text-[#d00e17]'
    : options.isFocused
      ? 'text-[#3e68ff]'
      : 'text-[#b4b4b4]'

  return {
    wrap: cx(
      'relative flex h-[3.25rem] w-full items-center justify-stretch rounded-[99999px] border px-5 text-[1rem] leading-6 text-[#0d0d0d]',
      borderColor,
      options.withEnd && 'pr-5',
    ),
    label: cx(
      'absolute inset-0 cursor-text',
      shouldFloat && 'pointer-events-none',
      "before:absolute before:inset-0 before:block before:rounded-[99999px] before:bg-white before:content-['']",
      shouldFloat && 'before:hidden',
    ),
    labelPos: cx('absolute inset-0 flex items-center px-5 transition-transform duration-100 ease-in-out', shouldFloat && '-translate-y-1/2'),
    labelText: cx(
      'bg-white px-[6px] py-px text-[1rem] leading-none transition-transform duration-100 ease-in-out',
      labelColor,
      'translate-x-[-6px]',
      shouldFloat && 'translate-x-[-12px] scale-[0.88]',
    ),
    input: cx(
      'w-full min-w-0 flex-1 border-0 bg-transparent p-0 text-[inherit] text-[#0d0d0d] outline-none placeholder:opacity-0',
      options.isReadonly && 'text-[#0d0d0d] [-webkit-text-fill-color:#0d0d0d]',
    ),
  }
}

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

  const emailInputClasses = getFloatingInputClasses({
    isActive: isEmailActive,
    hasValue: hasEmailValue,
    isFocused: isEmailFocused,
    isInvalid: isEmailInvalid,
    isSubmitting: isEntrySubmitting,
  })

  const passwordInputClasses = getFloatingInputClasses({
    isActive: isPasswordActive,
    hasValue: hasPasswordValue,
    isFocused: isPasswordFocused,
    isInvalid: isPasswordInvalid,
    isSubmitting: isPasswordSubmitting,
    withEnd: true,
  })

  const hasNewPasswordValue = newPassword.trim().length > 0
  const isNewPasswordInvalid = newPasswordErrorText.length > 0
  const isNewPasswordActive = isNewPasswordFocused || hasNewPasswordValue

  const hasConfirmPasswordValue = confirmPassword.trim().length > 0
  const isConfirmPasswordInvalid = confirmPasswordErrorText.length > 0
  const isConfirmPasswordActive = isConfirmPasswordFocused || hasConfirmPasswordValue
  const verifyDescribedBy = verifyErrorText ? `${verifySubtitleId} ${verifyErrorId}` : verifySubtitleId
  const passwordContinuePath =
    otpPurpose === 'signup_password' || otpPurpose === 'signup_otp' ? '/create-account/password' : '/log-in/password'

  const newPasswordInputClasses = getFloatingInputClasses({
    isActive: isNewPasswordActive,
    hasValue: hasNewPasswordValue,
    isFocused: isNewPasswordFocused,
    isInvalid: isNewPasswordInvalid,
    isSubmitting: isResetNewSubmitting,
    withEnd: true,
  })

  const confirmPasswordInputClasses = getFloatingInputClasses({
    isActive: isConfirmPasswordActive,
    hasValue: hasConfirmPasswordValue,
    isFocused: isConfirmPasswordFocused,
    isInvalid: isConfirmPasswordInvalid,
    isSubmitting: isResetNewSubmitting,
    withEnd: true,
  })
  const readonlyEmailInputClasses = getFloatingInputClasses({
    isActive: true,
    hasValue: true,
    isFocused: false,
    isInvalid: false,
    isSubmitting: false,
    withEnd: true,
    isReadonly: true,
  })
  const authCardClassName = cx(loginTw.cardBase, step === 'error' && loginTw.cardWide)

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
    <div className={loginTw.socialGroup} role="group" aria-label="选择登录选项">
      <button type="button" className={cx(loginTw.socialButton, focusRingClass)}>
        <span className={loginTw.socialIcon}>
          <img src={googleIconSvg} alt="" aria-hidden="true" className={loginTw.socialIconImg} />
        </span>
        <span>继续使用 Google 登录</span>
      </button>
      <button type="button" className={cx(loginTw.socialButton, focusRingClass)}>
        <span className={loginTw.socialIcon}>
          <img src={appleIconSvg} alt="" aria-hidden="true" className={loginTw.socialIconImg} />
        </span>
        <span>继续使用 Apple 登录</span>
      </button>
      <button type="button" className={cx(loginTw.socialButton, focusRingClass)}>
        <span className={loginTw.socialIcon}>
          <img src={microsoftIconSvg} alt="" aria-hidden="true" className={loginTw.socialIconImg} />
        </span>
        <span>继续使用 Microsoft 登录</span>
      </button>
      <button type="button" className={cx(loginTw.socialButton, focusRingClass)}>
        <span className={loginTw.socialIcon}>
          <img src={phoneIconSvg} alt="" aria-hidden="true" className={loginTw.socialIconImg} />
        </span>
        <span>继续使用手机登录</span>
      </button>
    </div>
  )

  return (
    <div
      className={loginTw.page}
      style={{ fontFamily: "'OpenAI Sans', 'SF Pro', -apple-system, system-ui, Helvetica, Arial, sans-serif" }}
    >
      <main className={loginTw.main}>
        <section className={authCardClassName} aria-label="登录卡片">
          {step === 'entry' ? (
            <>
              <div className={loginTw.titleBlock}>
                <Link to={logoLinkTarget} className={loginTw.wordmark} aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className={loginTw.wordmarkImg} />
                </Link>
                <h1 className={loginTw.heading}>
                  <span className={loginTw.headingText}>{isCreateAccountEntry ? '创建帐户' : '欢迎回来'}</span>
                </h1>
              </div>

              <fieldset className={loginTw.fieldset}>
                <form className={loginTw.form} onSubmit={handleEntrySubmit} noValidate>
                  <div className={loginTw.sectionCtas}>
                    {renderSocialButtons()}
                    <div className={loginTw.divider}>
                      <div className={loginTw.dividerLine} />
                      <div className={loginTw.dividerName}>或</div>
                      <div className={loginTw.dividerLine} />
                    </div>
                  </div>

                  <div className={loginTw.sectionFields}>
                    <div className={loginTw.fieldContainer} data-rac="" data-invalid={isEmailInvalid || undefined}>
                      <div className={loginTw.relative}>
                        <div className={emailInputClasses.wrap}>
                          <label className={emailInputClasses.label} htmlFor={emailId} id={emailLabelId}>
                            <div className={emailInputClasses.labelPos}>
                              <div className={emailInputClasses.labelText}>电子邮件地址</div>
                            </div>
                          </label>
                          <input
                            ref={entryInputRef}
                            className={emailInputClasses.input}
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
                        <span className={loginTw.inlineErrorContainer} aria-live="polite" aria-atomic="true">
                          {isEmailInvalid ? (
                            <span className={loginTw.inlineErrorContainer} id={emailErrorId}>
                              <ul className={loginTw.errorList}>
                                <li className={loginTw.errorItem}>
                                  <span className={loginTw.errorIcon}>
                                    <img src={errorIconSvg} alt="" aria-hidden="true" className={loginTw.errorIconImg} />
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

                  <div className={loginTw.sectionCtas}>
                    <button type="submit" className={cx(loginTw.primaryBtn, focusRingClass)} disabled={isEntrySubmitting}>
                      继续
                    </button>
                    {isCreateAccountEntry ? (
                      <span className={loginTw.textPrimary}>
                        已经有帐户？请
                        <Link to="/log-in" state={getAuthState(email)} className={loginTw.hintLink}>
                          登录
                        </Link>
                      </span>
                    ) : (
                      <span className={loginTw.textPrimary}>
                        还没有帐户？请
                        <Link to="/create-account" className={loginTw.hintLink}>注册</Link>
                      </span>
                    )}
                  </div>
                </form>
              </fieldset>
            </>
          ) : null}

          {step === 'password' ? (
            <>
              <div className={loginTw.titleBlock}>
                <Link to={logoLinkTarget} className={loginTw.wordmark} aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className={loginTw.wordmarkImg} />
                </Link>
                <h1 className={loginTw.heading}>
                  <span className={loginTw.headingText}>{isCreateAccountPassword ? '创建密码' : '输入密码'}</span>
                </h1>
              </div>

              <fieldset className={loginTw.fieldset}>
                <form
                  className={loginTw.form}
                  method="post"
                  action={isCreateAccountPassword ? '/create-account/password' : '/log-in/password'}
                  autoComplete="on"
                  onSubmit={handlePasswordSubmit}
                  noValidate
                >
                  <div className={loginTw.sectionFields}>
                    <input type="hidden" name="username" value={stepEmail} />
                    <div className={loginTw.stack}>
                      <div className={loginTw.relative}>
                        <div className={readonlyEmailInputClasses.wrap}>
                          <label className={readonlyEmailInputClasses.label} htmlFor="readonly-email" id={readonlyEmailLabelId}>
                            <div className={readonlyEmailInputClasses.labelPos}>
                              <div className={readonlyEmailInputClasses.labelText}>电子邮件地址</div>
                            </div>
                          </label>
                          <input
                            id="readonly-email"
                            className={readonlyEmailInputClasses.input}
                            type="text"
                            value={stepEmail}
                            readOnly
                            placeholder="电子邮件地址"
                            aria-labelledby={readonlyEmailLabelId}
                          />
                          <div className={loginTw.inputEndDecoration}>
                            <div className={loginTw.noWrap}>
                              <Link
                                to="/log-in"
                                state={getAuthState(stepEmail)}
                                className={loginTw.editLink}
                                aria-label="编辑电子邮件"
                              >
                                编辑
                              </Link>
                            </div>
                          </div>
                        </div>
                      </div>

                      <div className={loginTw.relative}>
                        <div className={passwordInputClasses.wrap}>
                          <label className={passwordInputClasses.label} htmlFor={passwordId} id={passwordLabelId}>
                            <div className={passwordInputClasses.labelPos}>
                              <div className={passwordInputClasses.labelText}>密码</div>
                            </div>
                          </label>
                          <input
                            ref={passwordInputRef}
                            id={passwordId}
                            className={passwordInputClasses.input}
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
                            className={loginTw.toggleBtn}
                            aria-label={passwordVisible ? '隐藏密码' : '显示密码'}
                            aria-controls={passwordId}
                            aria-pressed={passwordVisible}
                            onClick={() => setPasswordVisible((prev) => !prev)}
                          >
                            {passwordVisible ? (
                              <img src={eyeOffIconSvg} alt="" aria-hidden="true" className={loginTw.eyeIcon} />
                            ) : (
                              <img src={eyeIconSvg} alt="" aria-hidden="true" className={loginTw.eyeIcon} />
                            )}
                          </button>
                        </div>
                        {isPasswordInvalid ? (
                          <ul className={loginTw.errorList} id={passwordErrorId}>
                            <li className={loginTw.errorItem}>
                              <span className={loginTw.errorIcon}>
                                <img src={errorIconSvg} alt="" aria-hidden="true" className={loginTw.errorIconImg} />
                              </span>
                              <span>{passwordErrorText}</span>
                            </li>
                          </ul>
                        ) : null}
                      </div>

                      {!isCreateAccountPassword ? (
                        <span className={loginTw.forgotWrap}>
                          <Link to="/reset-password" state={getAuthState(stepEmail)} className={loginTw.forgotLink}>
                            忘记了密码？
                          </Link>
                        </span>
                      ) : null}
                    </div>
                  </div>

                  <div className={loginTw.sectionCtas}>
                    <div className={loginTw.buttonRow}>
                      <button type="submit" className={cx(loginTw.primaryBtn, focusRingClass)} disabled={isPasswordSubmitting || isOtpSending}>
                        继续
                      </button>
                    </div>

                    {!isCreateAccountPassword ? (
                      <span className={loginTw.textPrimary}>
                        还没有帐户？请
                        <Link to="/create-account" className={loginTw.hintLink}>注册</Link>
                      </span>
                    ) : null}

                    <div className={loginTw.divider}>
                      <div className={loginTw.dividerLine} />
                      <div className={loginTw.dividerName}>或</div>
                      <div className={loginTw.dividerLine} />
                    </div>

                    <div className={loginTw.stack}>
                      <div className={loginTw.buttonRow}>
                        <button
                          type="button"
                          className={cx(loginTw.outlineBtn, focusRingClass)}
                          disabled={isPasswordSubmitting || isOtpSending}
                          onClick={handlePasswordlessOtpAction}
                        >
                          {isCreateAccountPassword ? '使用一次性验证码注册' : '使用一次性验证码登录'}
                        </button>
                      </div>
                    </div>

                    {isCreateAccountPassword ? (
                      <span className={loginTw.textPrimary}>
                        已经有帐户了？请
                        <Link to="/log-in" state={getAuthState(stepEmail)} className={loginTw.hintLink}>
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
              <div className={loginTw.titleBlock}>
                <Link to={logoLinkTarget} className={loginTw.wordmark} aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className={loginTw.wordmarkImg} />
                </Link>
                <h1 className={loginTw.heading}>
                  <span className={loginTw.headingText}>检查您的收件箱</span>
                </h1>
                <div className={loginTw.subtitle}>
                  <span className={loginTw.textSecondary} id={verifySubtitleId}>
                    输入我们刚刚向 {stepEmail || '你的邮箱'} 发送的验证码
                  </span>
                </div>
              </div>

              <fieldset className={loginTw.fieldset}>
                <form className={loginTw.form} noValidate onSubmit={handleVerifySubmit}>
                  <div className={loginTw.sectionFields}>
                    <div className={loginTw.otpWrap}>
                      <div
                        className={loginTw.otpGrid}
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
                            className={cx(
                              loginTw.otpInput,
                              verifyErrorText ? loginTw.otpInputInvalid : loginTw.otpInputNormal,
                            )}
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
                        <ul className={loginTw.errorList} id={verifyErrorId}>
                          <li className={loginTw.otpErrorItem}>
                            <span className={loginTw.errorIcon}>
                              <img src={errorIconSvg} alt="" aria-hidden="true" className={loginTw.errorIconImg} />
                            </span>
                            <span>{verifyErrorText}</span>
                          </li>
                        </ul>
                      ) : null}
                    </div>
                  </div>

                  <div className={loginTw.sectionCtas}>
                    <div className={loginTw.buttonRow}>
                      <button type="button" className={cx(loginTw.outlineBtn, focusRingClass)} disabled={isVerifySubmitting || isOtpSending} onClick={resendEmail}>
                        重新发送电子邮件
                      </button>
                    </div>
                    <div className={loginTw.buttonRow}>
                      <Link className={cx(loginTw.linkBtn, focusRingClass)} to={passwordContinuePath} state={getAuthState(stepEmail)}>
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
              <div className={loginTw.titleBlock}>
                <Link to={logoLinkTarget} className={loginTw.wordmark} aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className={loginTw.wordmarkImg} />
                </Link>
                <h1 className={loginTw.heading}>
                  <span className={loginTw.headingText}>重置密码</span>
                </h1>
                <div className={loginTw.subtitle}>
                  <span className={loginTw.textSecondary}>
                    点击“继续”以重置 {stepEmail || '你的邮箱'} 的密码
                  </span>
                </div>
              </div>

              <fieldset className={loginTw.fieldset}>
                <form
                  className={loginTw.form}
                  method="post"
                  action="/reset-password"
                  onSubmit={handleResetPasswordSubmit}
                >
                  <div className={loginTw.sectionFields}>
                    <div className={loginTw.stackTight}>
                      <div className={loginTw.buttonRow}>
                        <button type="submit" className={cx(loginTw.primaryBtn, focusRingClass)} disabled={isResetSubmitting}>
                          继续
                        </button>
                      </div>
                      <div className={loginTw.buttonRow}>
                        <button
                          type="button"
                          className={cx(loginTw.transparentBtn, focusRingClass)}
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
              <div className={loginTw.titleBlock}>
                <Link to={logoLinkTarget} className={loginTw.wordmark} aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className={loginTw.wordmarkImg} />
                </Link>
                <h1 className={loginTw.heading}>
                  <span className={loginTw.headingText}>重置密码</span>
                </h1>
                <div className={loginTw.subtitle}>
                  <span className={loginTw.textSecondary}>请在下面输入新密码以更改密码</span>
                </div>
              </div>

              <fieldset className={loginTw.fieldset}>
                <form
                  className={loginTw.form}
                  method="post"
                  action="/reset-password/new-password"
                  autoComplete="on"
                  noValidate
                  onSubmit={handleResetNewPasswordSubmit}
                >
                  <div className={loginTw.sectionFields}>
                    <input type="hidden" name="username" value={stepEmail} autoComplete="username" />
                    <div className={loginTw.stack}>
                      <div className={loginTw.relative}>
                        <div className={newPasswordInputClasses.wrap}>
                          <label className={newPasswordInputClasses.label} htmlFor={newPasswordId} id={newPasswordLabelId}>
                            <div className={newPasswordInputClasses.labelPos}>
                              <div className={newPasswordInputClasses.labelText}>新密码</div>
                            </div>
                          </label>
                          <input
                            id={newPasswordId}
                            className={newPasswordInputClasses.input}
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
                            className={loginTw.toggleBtn}
                            aria-label={isNewPasswordVisible ? '隐藏密码' : '显示密码'}
                            aria-controls={newPasswordId}
                            aria-pressed={isNewPasswordVisible}
                            onClick={() => setIsNewPasswordVisible((prev) => !prev)}
                          >
                            {isNewPasswordVisible ? (
                              <img src={eyeOffIconSvg} alt="" aria-hidden="true" className={loginTw.eyeIcon} />
                            ) : (
                              <img src={eyeIconSvg} alt="" aria-hidden="true" className={loginTw.eyeIcon} />
                            )}
                          </button>
                        </div>
                        {isNewPasswordInvalid ? (
                          <ul className={loginTw.errorList} id={newPasswordErrorId}>
                            <li className={loginTw.errorItem}>
                              <span className={loginTw.errorIcon}>
                                <img src={errorIconSvg} alt="" aria-hidden="true" className={loginTw.errorIconImg} />
                              </span>
                              <span>{newPasswordErrorText}</span>
                            </li>
                          </ul>
                        ) : null}
                      </div>

                      <div className={loginTw.relative}>
                        <div className={confirmPasswordInputClasses.wrap}>
                          <label className={confirmPasswordInputClasses.label} htmlFor={confirmPasswordId} id={confirmPasswordLabelId}>
                            <div className={confirmPasswordInputClasses.labelPos}>
                              <div className={confirmPasswordInputClasses.labelText}>重新输入新密码</div>
                            </div>
                          </label>
                          <input
                            id={confirmPasswordId}
                            className={confirmPasswordInputClasses.input}
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
                            className={loginTw.toggleBtn}
                            aria-label={isConfirmPasswordVisible ? '隐藏密码' : '显示密码'}
                            aria-controls={confirmPasswordId}
                            aria-pressed={isConfirmPasswordVisible}
                            onClick={() => setIsConfirmPasswordVisible((prev) => !prev)}
                          >
                            {isConfirmPasswordVisible ? (
                              <img src={eyeOffIconSvg} alt="" aria-hidden="true" className={loginTw.eyeIcon} />
                            ) : (
                              <img src={eyeIconSvg} alt="" aria-hidden="true" className={loginTw.eyeIcon} />
                            )}
                          </button>
                        </div>
                        {isConfirmPasswordInvalid ? (
                          <ul className={loginTw.errorList} id={confirmPasswordErrorId}>
                            <li className={loginTw.errorItem}>
                              <span className={loginTw.errorIcon}>
                                <img src={errorIconSvg} alt="" aria-hidden="true" className={loginTw.errorIconImg} />
                              </span>
                              <span>{confirmPasswordErrorText}</span>
                            </li>
                          </ul>
                        ) : null}
                      </div>
                    </div>
                  </div>

                  <div className={loginTw.sectionCtas}>
                    <div className={loginTw.buttonRow}>
                      <button type="submit" className={cx(loginTw.primaryBtn, focusRingClass)} disabled={isResetNewSubmitting}>
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
              <div className={loginTw.titleBlock}>
                <img
                  src={successCheckmarkCircleSvg}
                  alt=""
                  aria-hidden="true"
                  width={60}
                  height={60}
                  className={loginTw.successIcon}
                />
                <h1 className={loginTw.heading}>
                  <span className={loginTw.headingText}>密码已更改</span>
                </h1>
                <div className={loginTw.subtitle}>
                  <span className={loginTw.textSecondary}>你的密码已成功更改</span>
                </div>
              </div>

              <fieldset className={loginTw.fieldset}>
                <form method="get" action="/reset-password/success" className={loginTw.form}>
                  <div className={loginTw.sectionFields} />
                  <div className={loginTw.sectionCtas}>
                    <div className={loginTw.buttonRow}>
                      <Link to="/log-in/password" className={cx(loginTw.primaryBtn, focusRingClass)}>
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
              <div className={loginTw.titleBlock}>
                <a href="https://chatgpt.com" aria-label="OpenAI 主页面" className={loginTw.logoLinkError}>
                  <img
                    src={onebookLogoMarkSvg}
                    alt="OneBook 徽标"
                    width={48}
                    height={48}
                    className={loginTw.logoMarkError}
                  />
                </a>
                <h1 className={loginTw.heading}>
                  <span className={loginTw.headingText}>糟糕，出错了！</span>
                </h1>
                <div className={loginTw.subtitle}>
                  <div className={loginTw.subtitleErrorCard}>{authErrorMessage}</div>
                </div>
              </div>
              <div className={loginTw.buttonRow}>
                <button
                  type="button"
                  className={cx(loginTw.outlineBtn, focusRingClass)}
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

          <div className={loginTw.footer}>
            <span className={loginTw.footerMeta}>
              <a href="https://openai.com/policies/terms-of-use" className={loginTw.underlineLink}>
                使用条款
              </a>
              <span className={loginTw.footerSep} aria-hidden="true">|</span>
              <a href="https://openai.com/policies/privacy-policy" className={loginTw.underlineLink}>
                隐私政策
              </a>
            </span>
          </div>
        </section>
      </main>
    </div>
  )
}
