import { useEffect, type ClipboardEvent, type SubmitEvent, useId, useMemo, useRef, useState } from 'react'
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import onebookWordmark from '@/assets/brand/onebook-wordmark.svg'
import googleIconSvg from '@/assets/brand/provider/google-logo.svg'
import appleIconSvg from '@/assets/brand/provider/apple-logo.svg'
import microsoftIconSvg from '@/assets/brand/provider/microsoft-logo.svg'
import phoneIconSvg from '@/assets/icons/phone.svg'
import errorIconSvg from '@/assets/icons/error-circle.svg'
import eyeIconSvg from '@/assets/icons/eye.svg'
import eyeOffIconSvg from '@/assets/icons/eye-off.svg'

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/

type Step = 'entry' | 'password' | 'verify'

function getStep(pathname: string): Step {
  if (pathname === '/login/password') return 'password'
  if (pathname === '/login/verify' || pathname === '/email-verification') return 'verify'
  return 'entry'
}

function encodeEmail(email: string) {
  const text = email.trim()
  return text ? `?email=${encodeURIComponent(text)}` : ''
}

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const [searchParams] = useSearchParams()

  const step = getStep(location.pathname)
  const stepEmail = useMemo(() => searchParams.get('email')?.trim() ?? '', [searchParams])

  const emailId = useId()
  const emailLabelId = useId()
  const emailErrorId = useId()

  const passwordId = useId()
  const passwordLabelId = useId()
  const passwordErrorId = useId()
  const readonlyEmailLabelId = useId()

  const [email, setEmail] = useState(stepEmail)
  const [isEmailFocused, setIsEmailFocused] = useState(false)
  const [isEntrySubmitting, setIsEntrySubmitting] = useState(false)
  const [emailErrorText, setEmailErrorText] = useState('')

  const [password, setPassword] = useState('')
  const [isPasswordFocused, setIsPasswordFocused] = useState(false)
  const [passwordVisible, setPasswordVisible] = useState(false)
  const [isPasswordSubmitting, setIsPasswordSubmitting] = useState(false)
  const [passwordErrorText, setPasswordErrorText] = useState('')

  const [otp, setOtp] = useState(['', '', '', '', '', ''])
  const otpRefs = useRef<Array<HTMLInputElement | null>>([])

  const entryInputRef = useRef<HTMLInputElement>(null)
  const passwordInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (step === 'entry') {
      setEmail(stepEmail)
      setEmailErrorText('')
      setIsEntrySubmitting(false)
    }
  }, [step, stepEmail])

  useEffect(() => {
    if (step !== 'verify') return
    setOtp(['', '', '', '', '', ''])
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

  const validateEmail = (value: string) => {
    const text = value.trim()
    if (!text) return '电子邮件地址为必填项。'
    if (!EMAIL_PATTERN.test(text)) return '电子邮件地址无效。'
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
    navigate(`/login/password${encodeEmail(normalizedEmail)}`)
  }

  const handlePasswordSubmit = async (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (isPasswordSubmitting) return

    if (!password.trim()) {
      setPasswordErrorText('密码为必填项。')
      passwordInputRef.current?.focus()
      return
    }

    setPasswordErrorText('')
    setIsPasswordSubmitting(true)
    await new Promise((resolve) => setTimeout(resolve, 250))
    navigate('/library')
  }

  const resendEmail = () => {
    setOtp(['', '', '', '', '', ''])
    otpRefs.current[0]?.focus()
  }

  const updateOtpAt = (index: number, rawValue: string) => {
    const value = rawValue.replace(/\D/g, '').slice(-1)
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

    const filled = ['','','','','','']
    for (let i = 0; i < pasted.length; i += 1) filled[i] = pasted[i]
    setOtp(filled)

    const nextIndex = Math.min(pasted.length, 5)
    otpRefs.current[nextIndex]?.focus()
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
        <section className="auth-card" aria-label="登录卡片">
          {step === 'entry' ? (
            <>
              <div className="auth-title-block">
                <Link to="/" className="auth-wordmark" aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className="auth-wordmark-img" />
                </Link>
                <h1 className="auth-heading">
                  <span className="auth-heading-text">登录或注册</span>
                </h1>
                <div className="auth-subtitle">
                  <span className="auth-subtitle-text">你将可以基于个人书库提问，并获得可追溯来源的回答。</span>
                </div>
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
                  </div>
                </form>
              </fieldset>
            </>
          ) : null}

          {step === 'password' ? (
            <>
              <div className="auth-title-block">
                <Link to="/" className="auth-wordmark" aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className="auth-wordmark-img" />
                </Link>
                <h1 className="auth-heading">
                  <span className="auth-heading-text">输入密码</span>
                </h1>
              </div>

              <fieldset className="auth-fieldset">
                <form
                  className="auth-form auth-form-password"
                  method="post"
                  action="/log-in/password"
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
                                to={`/login${encodeEmail(stepEmail)}`}
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

                      <span className="auth-forgot-password">
                        <a href="/reset-password">忘记了密码？</a>
                      </span>
                    </div>
                  </div>

                  <div className="auth-section auth-section-ctas auth-section-password-ctas">
                    <div className="auth-button-wrapper">
                      <button type="submit" className="auth-continue-btn" disabled={isPasswordSubmitting}>
                        继续
                      </button>
                    </div>

                    <span className="auth-signup-hint">
                      还没有帐户？请
                      <Link to="/create-account">注册</Link>
                    </span>

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
                          onClick={() => navigate(`/login/verify${encodeEmail(stepEmail)}`)}
                        >
                          使用一次性验证码登录
                        </button>
                      </div>
                    </div>
                  </div>
                </form>
              </fieldset>
            </>
          ) : null}

          {step === 'verify' ? (
            <>
              <div className="auth-title-block">
                <Link to="/" className="auth-wordmark" aria-label="OneBook AI home">
                  <img src={onebookWordmark} alt="OneBook AI" className="auth-wordmark-img" />
                </Link>
                <h1 className="auth-heading">
                  <span className="auth-heading-text">检查您的收件箱</span>
                </h1>
                <div className="auth-subtitle auth-subtitle-verify">
                  <span className="auth-subtitle-text">
                    输入我们刚刚向 {stepEmail || '你的邮箱'} 发送的验证码
                  </span>
                </div>
              </div>

              <fieldset className="auth-fieldset">
                <form className="auth-form auth-form-verify" noValidate>
                  <div className="auth-section auth-section-fields">
                    <div className="auth-otp-wrap">
                      <label className="auth-otp-label">验证码</label>
                      <div className="auth-otp-group" role="group" aria-label="验证码">
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
                            value={digit}
                            onChange={(e) => updateOtpAt(index, e.target.value)}
                            onKeyDown={(e) => handleOtpKeyDown(index, e.key)}
                            onPaste={handleOtpPaste}
                          />
                        ))}
                      </div>
                      <input type="hidden" readOnly value={otp.join('')} name="code" />
                    </div>
                  </div>

                  <div className="auth-section auth-section-ctas auth-section-verify-ctas">
                    <button type="button" className="auth-outline-btn" onClick={resendEmail}>
                      重新发送电子邮件
                    </button>
                    <Link className="auth-link-btn" to={`/login/password${encodeEmail(stepEmail)}`}>
                      使用密码继续
                    </Link>
                  </div>
                </form>
              </fieldset>
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
