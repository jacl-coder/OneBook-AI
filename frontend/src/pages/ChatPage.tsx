import { useEffect, useId, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import attachIcon from '@/assets/chat/attach.svg'
import arrowUpIcon from '@/assets/chat/arrow-up.svg'
import chevronDownIcon from '@/assets/chat/chevron-down.svg'
import micIcon from '@/assets/chat/mic.svg'
import profileIcon from '@/assets/chat/profile.svg'
import quoteIcon from '@/assets/chat/quote.svg'
import searchIcon from '@/assets/chat/search.svg'
import studyIcon from '@/assets/chat/study.svg'
import onebookLogoMark from '@/assets/home/onebook-logo-mark.svg'

const quickActions = [
  { icon: attachIcon, label: '附件' },
  { icon: searchIcon, label: '检索书库' },
  { icon: studyIcon, label: '学习模式' },
  { icon: quoteIcon, label: '引用回答' },
]

const headingPool = [
  '先读你的书，再来提问。',
  '你的资料里，今天想问什么？',
  '先看原文，再看答案。',
  '从书中检索，让回答可追溯。',
  '围绕你的书库，开始一次对话。',
  '先定位证据，再生成结论。',
]

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/

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

const CloseIcon = () => (
  <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
    <path d="M5 5L15 15" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
    <path d="M15 5L5 15" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
  </svg>
)

export function ChatPage() {
  const navigate = useNavigate()
  const editorRef = useRef<HTMLDivElement>(null)
  const authInputRef = useRef<HTMLInputElement>(null)

  const [prompt, setPrompt] = useState('')
  const [heading] = useState(
    () => headingPool[Math.floor(Math.random() * headingPool.length)],
  )
  const [isAuthOpen, setIsAuthOpen] = useState(false)
  const [authEmail, setAuthEmail] = useState('')
  const [isAuthFocused, setIsAuthFocused] = useState(false)
  const [isAuthSubmitting, setIsAuthSubmitting] = useState(false)
  const [authErrorText, setAuthErrorText] = useState('')

  const authEmailId = useId()
  const authErrorId = useId()

  const hasPrompt = prompt.trim().length > 0
  const isAuthInvalid = authErrorText.length > 0

  useEffect(() => {
    if (!isAuthOpen) return
    const originalOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = originalOverflow
    }
  }, [isAuthOpen])

  useEffect(() => {
    if (!isAuthOpen) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        closeAuthModal()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [isAuthOpen])

  const syncPrompt = () => {
    const value = editorRef.current?.innerText ?? ''
    setPrompt(value.replace(/\u00a0/g, ' '))
  }

  const handleComposerSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!hasPrompt) return
    if (editorRef.current) editorRef.current.textContent = ''
    setPrompt('')
  }

  const openAuthModal = () => {
    setIsAuthOpen(true)
  }

  const closeAuthModal = () => {
    setIsAuthOpen(false)
    setAuthEmail('')
    setIsAuthFocused(false)
    setIsAuthSubmitting(false)
    setAuthErrorText('')
  }

  const validateAuthEmail = (value: string) => {
    const text = value.trim()
    if (!text) return '电子邮件地址为必填项。'
    if (!EMAIL_PATTERN.test(text)) return '电子邮件地址无效。'
    return ''
  }

  const handleAuthSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (isAuthSubmitting) return

    const error = validateAuthEmail(authEmail)
    if (error) {
      setAuthErrorText(error)
      authInputRef.current?.focus()
      return
    }

    setAuthErrorText('')
    setIsAuthSubmitting(true)
    await new Promise((resolve) => setTimeout(resolve, 280))
    closeAuthModal()
    navigate('/login')
  }

  return (
    <div className="chatgpt-entry-page">
      <a className="chatgpt-skip-link" href="#onebook-main">
        跳至内容
      </a>

      <header className="chatgpt-entry-header" role="banner">
        <div className="chatgpt-entry-left">
          <Link to="/chat" className="chatgpt-entry-logo-link" aria-label="OneBook AI">
            <img src={onebookLogoMark} alt="" aria-hidden="true" />
          </Link>
          <button type="button" className="chatgpt-model-btn" aria-label="模型选择器，当前模型为 OneBook AI">
            <span>OneBook AI</span>
            <img src={chevronDownIcon} alt="" aria-hidden="true" className="chatgpt-model-icon" />
          </button>
        </div>

        <div className="chatgpt-entry-right">
          <button type="button" className="chatgpt-top-btn chatgpt-top-btn-dark" onClick={openAuthModal}>
            登录
          </button>
          <button type="button" className="chatgpt-top-btn chatgpt-top-btn-light" onClick={openAuthModal}>
            免费注册
          </button>
          <button type="button" className="chatgpt-profile-btn" aria-label="打开“个人资料”菜单">
            <img src={profileIcon} alt="" aria-hidden="true" className="chatgpt-profile-icon" />
          </button>
        </div>
      </header>

      <main id="onebook-main" className="chatgpt-entry-main">
        <div className="chatgpt-entry-center">
          <div className="chatgpt-entry-hero">
            <div className="chatgpt-entry-heading-row">
              <div className="chatgpt-entry-heading-inline">
                <h1>
                  <div className="chatgpt-entry-title">{heading}</div>
                </h1>
              </div>
            </div>
          </div>

          <div className="chatgpt-thread-bottom" id="thread-bottom">
            <div className="chatgpt-thread-content">
              <div className="chatgpt-thread-max">
                <div className="chatgpt-composer-container">
                  <form
                    className="chatgpt-composer-form"
                    data-expanded=""
                    data-type="unified-composer"
                    onSubmit={handleComposerSubmit}
                  >
                    <div className="chatgpt-hidden-upload">
                      <input
                        accept="image/jpeg,.jpg,.jpeg,image/webp,.webp,image/gif,.gif,image/png,.png"
                        multiple
                        type="file"
                        tabIndex={-1}
                      />
                    </div>

                    <div className="chatgpt-composer-surface" data-composer-surface="true">
                      <div className="chatgpt-composer-primary">
                        <div className="chatgpt-prosemirror-parent">
                          <div
                            ref={editorRef}
                            contentEditable
                            suppressContentEditableWarning
                            translate="no"
                            role="textbox"
                            id="prompt-textarea"
                            className="chatgpt-prosemirror"
                            data-empty={hasPrompt ? 'false' : 'true'}
                            aria-label="输入你的问题"
                            onInput={syncPrompt}
                            onKeyDown={(event) => {
                              if (event.key === 'Enter' && !event.shiftKey) {
                                event.preventDefault()
                                if (hasPrompt) {
                                  if (editorRef.current) editorRef.current.textContent = ''
                                  setPrompt('')
                                }
                              }
                            }}
                          />
                          {!hasPrompt && (
                            <div className="chatgpt-prosemirror-placeholder" aria-hidden="true">
                              有问题，尽管问
                            </div>
                          )}
                        </div>
                      </div>

                      <div className="chatgpt-composer-footer-actions" data-testid="composer-footer-actions">
                        <div className="chatgpt-composer-footer-row">
                          {quickActions.map((item, index) => (
                            <button
                              key={item.label}
                              type="button"
                              className={
                                index === 0
                                  ? 'chatgpt-action-btn chatgpt-action-btn-attach'
                                  : 'chatgpt-action-btn'
                              }
                            >
                              <img src={item.icon} alt="" aria-hidden="true" className="chatgpt-action-icon" />
                              <span>{item.label}</span>
                            </button>
                          ))}
                        </div>
                      </div>

                      <div className="chatgpt-composer-trailing">
                        <button type="button" className="chatgpt-voice-btn" aria-label="启动语音功能">
                          <img src={micIcon} alt="" aria-hidden="true" className="chatgpt-voice-icon" />
                          <span>语音</span>
                        </button>
                        <button
                          type="submit"
                          className="chatgpt-send-btn"
                          aria-label="发送"
                          disabled={!hasPrompt}
                        >
                          <img src={arrowUpIcon} alt="" aria-hidden="true" className="chatgpt-send-icon" />
                        </button>
                      </div>
                    </div>
                  </form>
                </div>

                <input className="chatgpt-sr-only" type="file" tabIndex={-1} aria-hidden="true" id="upload-photos" accept="image/*" multiple />
                <input
                  className="chatgpt-sr-only"
                  type="file"
                  tabIndex={-1}
                  aria-hidden="true"
                  id="upload-camera"
                  accept="image/*"
                  capture="environment"
                  multiple
                />
              </div>
            </div>
          </div>

          <div className="chatgpt-entry-legal-wrap">
            <p className="chatgpt-entry-legal">
              向 OneBook AI 发送消息即表示，你同意我们的
              <a href="#" onClick={(e) => e.preventDefault()}>
                条款
              </a>
              并已阅读我们的
              <a href="#" onClick={(e) => e.preventDefault()}>
                隐私政策
              </a>
              。查看
              <a href="#" onClick={(e) => e.preventDefault()}>
                Cookie 首选项
              </a>
              。
            </p>
          </div>
        </div>
      </main>

      {isAuthOpen ? (
        <div id="modal-no-auth-login" className="chatgpt-auth-modal-root">
          <div className="chatgpt-auth-modal-backdrop" onClick={closeAuthModal} aria-hidden="true" />
          <div className="chatgpt-auth-modal-grid">
            <div
              role="dialog"
              aria-modal="true"
              aria-labelledby="chatgpt-auth-dialog-title"
              className="chatgpt-auth-dialog"
              onClick={(event) => event.stopPropagation()}
            >
              <header className="chatgpt-auth-dialog-header">
                <div className="chatgpt-auth-dialog-header-title" />
                <button type="button" className="chatgpt-auth-close-btn" aria-label="关闭" onClick={closeAuthModal}>
                  <CloseIcon />
                </button>
              </header>

              <div className="chatgpt-auth-dialog-body">
                <div className="chatgpt-auth-dialog-content" data-testid="login-form">
                  <h2 id="chatgpt-auth-dialog-title">登录或注册</h2>
                  <p className="chatgpt-auth-dialog-subtitle">
                    你将可以基于个人书库提问，并获得可追溯来源的回答。
                  </p>

                  <form className="chatgpt-auth-form" onSubmit={handleAuthSubmit} noValidate>
                    <div className="chatgpt-auth-social-group" role="group" aria-label="选择登录选项">
                      <button type="button" className="chatgpt-auth-social-btn" onClick={() => navigate('/login')}>
                        <span className="chatgpt-auth-social-btn-inner">
                          <span className="chatgpt-auth-social-icon"><GoogleIcon /></span>
                          <span>继续使用 Google 登录</span>
                        </span>
                      </button>
                      <button type="button" className="chatgpt-auth-social-btn" onClick={() => navigate('/login')}>
                        <span className="chatgpt-auth-social-btn-inner">
                          <span className="chatgpt-auth-social-icon"><AppleIcon /></span>
                          <span>继续使用 Apple 登录</span>
                        </span>
                      </button>
                      <button type="button" className="chatgpt-auth-social-btn" onClick={() => navigate('/login')}>
                        <span className="chatgpt-auth-social-btn-inner">
                          <span className="chatgpt-auth-social-icon"><MicrosoftIcon /></span>
                          <span>继续使用 Microsoft 登录</span>
                        </span>
                      </button>
                      <button type="button" className="chatgpt-auth-social-btn" onClick={() => navigate('/login')}>
                        <span className="chatgpt-auth-social-btn-inner">
                          <span className="chatgpt-auth-social-icon"><PhoneIcon /></span>
                          <span>继续使用手机登录</span>
                        </span>
                      </button>
                    </div>

                    <div className="chatgpt-auth-divider">
                      <div className="chatgpt-auth-divider-line" />
                      <div className="chatgpt-auth-divider-name">或</div>
                      <div className="chatgpt-auth-divider-line" />
                    </div>

                    <div className="chatgpt-auth-input-block">
                      <div
                        className={`chatgpt-auth-input-frame ${isAuthFocused ? 'is-focused' : ''} ${
                          isAuthInvalid ? 'is-invalid' : ''
                        }`}
                      >
                        <input
                          ref={authInputRef}
                          className="chatgpt-auth-input"
                          id={authEmailId}
                          name="email"
                          type="email"
                          autoComplete="email"
                          placeholder="电子邮件地址"
                          value={authEmail}
                          aria-label="电子邮件地址"
                          aria-describedby={isAuthInvalid ? authErrorId : undefined}
                          aria-invalid={isAuthInvalid || undefined}
                          disabled={isAuthSubmitting}
                          onFocus={() => setIsAuthFocused(true)}
                          onBlur={() => setIsAuthFocused(false)}
                          onChange={(e) => {
                            setAuthEmail(e.target.value)
                            if (isAuthInvalid) setAuthErrorText('')
                          }}
                        />
                      </div>
                      {isAuthInvalid ? (
                        <div className="chatgpt-auth-error" id={authErrorId}>
                          <span className="chatgpt-auth-error-icon"><ErrorIcon /></span>
                          <span>{authErrorText}</span>
                        </div>
                      ) : null}
                    </div>

                    <button type="submit" className="chatgpt-auth-continue-btn" disabled={isAuthSubmitting}>
                      继续
                    </button>

                  </form>
                </div>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
