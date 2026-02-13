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

import { getApiErrorMessage, login, signup } from '@/features/auth/api/auth'
import { useSessionStore } from '@/features/auth/store/session'

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/
const AUTH_EMAIL_STORAGE_KEY = 'auth:email'
const AUTH_ERROR_MESSAGE_STORAGE_KEY = 'auth:error-message'
const DEFAULT_AUTH_ERROR_MESSAGE = 'Invalid client. Please start over.'

type AuthNavigationState = {
  email?: string
  errorMessage?: string
}

type Step = 'entry' | 'password' | 'verify' | 'reset' | 'resetNew' | 'resetSuccess' | 'error'

function getStep(pathname: string): Step {
  if (pathname === '/log-in/password' || pathname === '/create-account/password') return 'password'
  if (pathname === '/log-in/verify' || pathname === '/email-verification') return 'verify'
  if (pathname === '/reset-password') return 'reset'
  if (pathname === '/reset-password/new-password') return 'resetNew'
  if (pathname === '/reset-password/success') return 'resetSuccess'
  if (pathname === '/log-in/error') return 'error'
  return 'entry'
}

function normalizeText(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function readSessionValue(key: string) {
  if (typeof window === 'undefined') return ''
  return normalizeText(window.sessionStorage.getItem(key))
}

function writeSessionValue(key: string, value: string) {
  if (typeof window === 'undefined') return
  if (value) {
    window.sessionStorage.setItem(key, value)
    return
  }
  window.sessionStorage.removeItem(key)
}

function schedule(fn: () => void) {
  if (typeof queueMicrotask === 'function') {
    queueMicrotask(fn)
    return
  }
  setTimeout(fn, 0)
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
  const [stepEmail, setStepEmail] = useState(() => locationStateEmail || readSessionValue(AUTH_EMAIL_STORAGE_KEY))
  const [authErrorMessage, setAuthErrorMessage] = useState(() => {
    const initialError = locationStateErrorMessage || readSessionValue(AUTH_ERROR_MESSAGE_STORAGE_KEY)
    return initialError || DEFAULT_AUTH_ERROR_MESSAGE
  })

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

  const getAuthState = (emailValue: string): AuthNavigationState => {
    const normalizedEmail = normalizeText(emailValue)
    const state: AuthNavigationState = {}
    if (normalizedEmail) state.email = normalizedEmail
    return state
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
    await new Promise((resolve) => setTimeout(resolve, 250))
    const nextEmail = syncEmail(normalizedEmail)
    navigate(isCreateAccountEntry ? '/create-account/password' : '/log-in/password', {
      state: getAuthState(nextEmail),
    })
  }

  const handlePasswordSubmit = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (isPasswordSubmitting) return

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
      const auth = isCreateAccountPassword
        ? await signup({ email: stepEmail.trim(), password: password.trim() })
        : await login({ email: stepEmail.trim(), password: password.trim() })
      setSession({
        accessToken: auth.token,
        refreshToken: auth.refreshToken,
        user: {
          id: auth.user.id,
          email: auth.user.email,
          role: auth.user.role,
          status: auth.user.status,
        },
      })
      navigate('/library')
    } catch (error) {
      setPasswordErrorText(
        getApiErrorMessage(error, isCreateAccountPassword ? 'Sign up failed. Please try again.' : 'Login failed. Please try again.'),
      )
      setIsPasswordSubmitting(false)
      passwordInputRef.current?.focus()
    }
  }

  const resendEmail = () => {
    lastSubmittedVerifyCodeRef.current = ''
    setOtp(['', '', '', '', '', ''])
    setVerifyErrorText('')
    otpRefs.current[0]?.focus()
  }

  const updateOtpAt = (index: number, rawValue: string) => {
    lastSubmittedVerifyCodeRef.current = ''
    const value = rawValue.replace(/\D/g, '').slice(-1)
    if (verifyErrorText) setVerifyErrorText('')
    setOtp((prev) => {
      const next = [...prev]
      next[index] = value
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

  const handleOtpPaste = (event: ClipboardEvent<HTMLInputElement>) => {
    event.preventDefault()
    const pasted = event.clipboardData.getData('text').replace(/\D/g, '').slice(0, 6)
    if (!pasted) return

    lastSubmittedVerifyCodeRef.current = ''
    if (verifyErrorText) setVerifyErrorText('')
    const filled = ['','','','','','']
    for (let i = 0; i < pasted.length; i += 1) filled[i] = pasted[i]
    setOtp(filled)

    const nextIndex = Math.min(pasted.length, 5)
    otpRefs.current[nextIndex]?.focus()
  }

  const handleResetPasswordSubmit = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (isResetSubmitting) return

    setIsResetSubmitting(true)
    navigate('/log-in/error', {
      state: {
        ...getAuthState(stepEmail),
        errorMessage: 'Password reset is not implemented yet. Please log in with your password or sign up again.',
      } satisfies AuthNavigationState,
    })
  }

  const handleVerifySubmit = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    navigate('/log-in/error', {
      state: {
        ...getAuthState(stepEmail),
        errorMessage: 'Email code login/sign-up is not implemented yet. Please continue with password.',
      } satisfies AuthNavigationState,
    })
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

    setIsResetNewSubmitting(true)
    navigate('/log-in/error', {
      state: {
        ...getAuthState(stepEmail),
        errorMessage: 'Password reset is not implemented yet. Please log in with your password or sign up again.',
      } satisfies AuthNavigationState,
    })
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
                      <button type="submit" className="auth-continue-btn" disabled={isPasswordSubmitting}>
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
                          onClick={() => {
                            navigate('/log-in/error', {
                              state: {
                                ...getAuthState(stepEmail),
                                errorMessage:
                                  '当前版本未接入邮箱验证码登录/注册，请使用密码方式继续。',
                              } satisfies AuthNavigationState,
                            })
                          }}
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
                            disabled={isVerifySubmitting}
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
                      <button type="button" className="auth-outline-btn" onClick={resendEmail}>
                        重新发送电子邮件
                      </button>
                    </div>
                    <div className="auth-button-wrapper">
                      <Link className="auth-link-btn" to="/log-in/password" state={getAuthState(stepEmail)}>
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
