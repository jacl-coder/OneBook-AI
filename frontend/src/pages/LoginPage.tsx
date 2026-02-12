import { type FormEvent, useId, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import onebookLogoMark from '@/assets/home/onebook-logo-mark.svg'

const GoogleIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
    <path
      d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
      fill="#4285F4"
    />
    <path
      d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
      fill="#34A853"
    />
    <path
      d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
      fill="#FBBC05"
    />
    <path
      d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
      fill="#EA4335"
    />
  </svg>
)

const AppleIcon = () => (
  <svg width="17" height="17" viewBox="0 0 64 64" fill="none" aria-hidden="true">
    <path
      d="M48.644 32.4099C48.5739 24.6929 54.9525 20.9851 55.2399 20.8028C51.6511 15.553 46.0715 14.8381 44.0808 14.754C39.3354 14.2704 34.8143 17.5506 32.4031 17.5506C29.9918 17.5506 26.2768 14.8241 22.3375 14.9012C17.1575 14.9783 12.384 17.9081 9.7134 22.5481C4.33714 31.8702 8.33954 45.6991 13.5826 53.2759C16.1481 56.9767 19.1972 61.1471 23.2136 60.9929C27.0829 60.8387 28.5408 58.4906 33.2092 58.4906C37.8775 58.4906 39.1952 60.9929 43.2748 60.9158C47.4314 60.8317 50.0669 57.1309 52.6114 53.4161C55.5483 49.1195 56.761 44.9561 56.8311 44.7388C56.7399 44.7038 48.7281 41.6338 48.644 32.4099"
      fill="currentColor"
    />
    <path
      d="M40.9686 9.74262C43.0995 7.16328 44.5364 3.57463 44.1439 0C41.0737 0.126163 37.3587 2.04665 35.1577 4.62599C33.1881 6.90394 31.4567 10.5557 31.9264 14.0602C35.34 14.3266 38.8377 12.3149 40.9686 9.74262Z"
      fill="currentColor"
    />
  </svg>
)

const MicrosoftIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path d="M3.25 3.25H11.25V11.25H3.25V3.25Z" fill="#F35325" />
    <path d="M12.75 3.25H20.75V11.25H12.75V3.25Z" fill="#81BC06" />
    <path d="M3.25 12.75H11.25V20.75H3.25V12.75Z" fill="#05A6F0" />
    <path d="M12.75 12.75H20.75V20.75H12.75V12.75Z" fill="#FFBA08" />
  </svg>
)

const PhoneIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path
      fillRule="evenodd"
      clipRule="evenodd"
      d="M2 5.57143C2 3.59898 3.59898 2 5.57143 2H8.625C9.0287 2 9.39281 2.24274 9.54808 2.61538L11.4231 7.11538C11.5744 7.47863 11.4987 7.89686 11.2295 8.18394L9.82741 9.67954C10.9044 11.7563 12.2732 13.2047 14.3016 14.2842L15.7929 12.7929C16.0794 12.5064 16.5106 12.4211 16.8846 12.5769L21.3846 14.4519C21.7573 14.6072 22 14.9713 22 15.375V18.4286C22 20.401 20.401 22 18.4286 22C9.35532 22 2 14.6447 2 5.57143ZM5.57143 4C4.70355 4 4 4.70355 4 5.57143C4 13.5401 10.4599 20 18.4286 20C19.2964 20 20 19.2964 20 18.4286V16.0417L16.7336 14.6807L15.2071 16.2071C14.9098 16.5044 14.4582 16.584 14.0771 16.4062C11.0315 14.9849 9.12076 12.9271 7.71882 9.92289C7.54598 9.55251 7.61592 9.11423 7.89546 8.81606L9.32824 7.28777L7.95833 4H5.57143Z"
      fill="currentColor"
    />
  </svg>
)

const ErrorIcon = () => (
  <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
    <path
      fillRule="evenodd"
      clipRule="evenodd"
      d="M8 14.667A6.667 6.667 0 1 0 8 1.333a6.667 6.667 0 0 0 0 13.334z"
      fill="#D00E17"
      stroke="#D00E17"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
    <path
      fillRule="evenodd"
      clipRule="evenodd"
      d="M8 4.583a.75.75 0 0 1 .75.75V8a.75.75 0 0 1-1.5 0V5.333a.75.75 0 0 1 .75-.75z"
      fill="#fff"
    />
    <path d="M8.667 10.667a.667.667 0 1 1-1.334 0 .667.667 0 0 1 1.334 0z" fill="#fff" />
  </svg>
)

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/

export function LoginPage() {
  const navigate = useNavigate()
  const inputRef = useRef<HTMLInputElement>(null)
  const emailId = useId()
  const labelId = useId()
  const errorId = useId()

  const [email, setEmail] = useState('')
  const [isFocused, setIsFocused] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [errorText, setErrorText] = useState('')

  const hasValue = email.trim().length > 0
  const isInvalid = errorText.length > 0
  const isActive = isFocused || hasValue

  const emailWrapClassName = [
    'auth-input-wrap',
    isActive ? 'is-active' : '',
    isFocused ? 'is-focused' : '',
    hasValue ? 'has-value' : '',
    isInvalid ? 'is-invalid' : '',
    isSubmitting ? 'is-submitting' : '',
  ]
    .filter(Boolean)
    .join(' ')

  const validateEmail = (value: string) => {
    const text = value.trim()
    if (!text) return '电子邮件地址为必填项。'
    if (!EMAIL_PATTERN.test(text)) return '电子邮件地址无效。'
    return ''
  }

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (isSubmitting) return

    const error = validateEmail(email)
    if (error) {
      setErrorText(error)
      setIsSubmitting(false)
      inputRef.current?.focus()
      return
    }

    setErrorText('')
    setIsSubmitting(true)

    await new Promise((resolve) => setTimeout(resolve, 550))
    navigate('/library')
  }

  return (
    <div className="auth-page">
      <Link to="/" className="auth-wordmark" aria-label="OneBook AI home">
        <img src={onebookLogoMark} alt="" aria-hidden="true" />
        <span>OneBook AI</span>
      </Link>

      <main className="auth-main">
        <section className="auth-card" aria-label="登录卡片">
          <h1>登录或注册</h1>
          <p className="auth-subtitle">你将可以基于个人书库提问，并获得可追溯来源的回答。</p>

          <form
            className="auth-form"
            onSubmit={handleSubmit}
            noValidate
          >
            <div className="auth-section auth-section-ctas">
              <div className="auth-social-group" role="group" aria-label="选择登录选项">
                <button type="button" className="auth-social-btn">
                  <span className="auth-social-icon">
                    <GoogleIcon />
                  </span>
                  <span>继续使用 Google 登录</span>
                </button>
                <button type="button" className="auth-social-btn">
                  <span className="auth-social-icon">
                    <AppleIcon />
                  </span>
                  <span>继续使用 Apple 登录</span>
                </button>
                <button type="button" className="auth-social-btn">
                  <span className="auth-social-icon">
                    <MicrosoftIcon />
                  </span>
                  <span>继续使用 Microsoft 登录</span>
                </button>
                <button type="button" className="auth-social-btn">
                  <span className="auth-social-icon">
                    <PhoneIcon />
                  </span>
                  <span>继续使用手机登录</span>
                </button>
              </div>

              <div className="auth-divider">
                <div className="auth-divider-line" />
                <div className="auth-divider-name">或</div>
                <div className="auth-divider-line" />
              </div>
            </div>

            <div className="auth-section auth-section-fields">
              <div className="auth-textfield" data-rac="" data-invalid={isInvalid || undefined}>
                <div className="auth-textfield-root">
                  <div className={emailWrapClassName}>
                    <label className="auth-input-label" htmlFor={emailId} id={labelId}>
                      <div className="auth-input-label-pos">
                        <div className="auth-input-label-text">电子邮件地址</div>
                      </div>
                    </label>
                    <input
                      ref={inputRef}
                      className="auth-input-target"
                      id={emailId}
                      name="email"
                      type="email"
                      autoComplete="email"
                      placeholder="电子邮件地址"
                      value={email}
                      aria-labelledby={labelId}
                      aria-describedby={isInvalid ? errorId : undefined}
                      aria-invalid={isInvalid || undefined}
                      data-focused={isFocused || undefined}
                      data-invalid={isInvalid || undefined}
                      disabled={isSubmitting}
                      onFocus={() => setIsFocused(true)}
                      onBlur={() => setIsFocused(false)}
                      onChange={(e) => {
                        setEmail(e.target.value)
                        if (isInvalid) setErrorText('')
                      }}
                    />
                  </div>
                  <span className="auth-input-live" aria-live="polite" aria-atomic="true">
                    {isInvalid ? (
                      <span className="auth-field-error-slot" id={errorId}>
                        <ul className="auth-field-errors">
                          <li className="auth-field-error">
                            <span className="auth-field-error-icon">
                              <ErrorIcon />
                            </span>
                            <span>{errorText}</span>
                          </li>
                        </ul>
                      </span>
                    ) : null}
                  </span>
                </div>
              </div>
            </div>

            <div className="auth-section auth-section-ctas">
              <button type="submit" className="auth-continue-btn" disabled={isSubmitting}>
                继续
              </button>
            </div>
          </form>
        </section>
      </main>

      <footer className="auth-footer">
        <a href="#" onClick={(e) => e.preventDefault()}>
          使用条款
        </a>
        <span aria-hidden="true">|</span>
        <a href="#" onClick={(e) => e.preventDefault()}>
          隐私政策
        </a>
      </footer>
    </div>
  )
}
