import { useEffect, useId, useRef, useState } from 'react'
import type { SubmitEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import attachIcon from '@/assets/icons/chat/attach.svg'
import arrowUpIcon from '@/assets/icons/chat/arrow-up.svg'
import chevronDownIcon from '@/assets/icons/chat/chevron-down.svg'
import micIcon from '@/assets/icons/chat/mic.svg'
import profileIcon from '@/assets/icons/chat/profile.svg'
import quoteIcon from '@/assets/icons/chat/quote.svg'
import searchIcon from '@/assets/icons/chat/search.svg'
import studyIcon from '@/assets/icons/chat/study.svg'
import onebookLogoMark from '@/assets/brand/onebook-logo-mark.svg'
import googleLogo from '@/assets/brand/provider/google-logo.svg'
import appleLogo from '@/assets/brand/provider/apple-logo.svg'
import microsoftLogo from '@/assets/brand/provider/microsoft-logo.svg'
import phoneIconSvg from '@/assets/icons/phone.svg'
import errorCircleIcon from '@/assets/icons/error-circle.svg'
import closeIcon from '@/assets/icons/close.svg'

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
type AuthModalMode = 'login' | 'register'

export function ChatPage() {
  const navigate = useNavigate()
  const editorRef = useRef<HTMLDivElement>(null)
  const authInputRef = useRef<HTMLInputElement>(null)

  const [prompt, setPrompt] = useState('')
  const [heading] = useState(
    () => headingPool[Math.floor(Math.random() * headingPool.length)],
  )
  const [isAuthOpen, setIsAuthOpen] = useState(false)
  const [authMode, setAuthMode] = useState<AuthModalMode>('login')
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

  const handleComposerSubmit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!hasPrompt) return
    if (editorRef.current) editorRef.current.textContent = ''
    setPrompt('')
  }

  const openAuthModal = (mode: AuthModalMode = 'login') => {
    setAuthMode(mode)
    setIsAuthOpen(true)
  }

  const closeAuthModal = () => {
    setIsAuthOpen(false)
    setAuthMode('login')
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

  const handleAuthSubmit = async (event: SubmitEvent<HTMLFormElement>) => {
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
    const targetPath = authMode === 'register' ? '/create-account/password' : '/log-in/password'
    navigate(`${targetPath}?email=${encodeURIComponent(authEmail.trim())}`)
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
          <button type="button" className="chatgpt-top-btn chatgpt-top-btn-dark" onClick={() => openAuthModal('login')}>
            登录
          </button>
          <button type="button" className="chatgpt-top-btn chatgpt-top-btn-light" onClick={() => openAuthModal('register')}>
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
                  <img src={closeIcon} alt="" aria-hidden="true" />
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
                      <button type="button" className="chatgpt-auth-social-btn" onClick={() => navigate('/log-in')}>
                        <span className="chatgpt-auth-social-btn-inner">
                          <span className="chatgpt-auth-social-icon">
                            <img src={googleLogo} alt="" aria-hidden="true" />
                          </span>
                          <span>继续使用 Google 登录</span>
                        </span>
                      </button>
                      <button type="button" className="chatgpt-auth-social-btn" onClick={() => navigate('/log-in')}>
                        <span className="chatgpt-auth-social-btn-inner">
                          <span className="chatgpt-auth-social-icon">
                            <img src={appleLogo} alt="" aria-hidden="true" />
                          </span>
                          <span>继续使用 Apple 登录</span>
                        </span>
                      </button>
                      <button type="button" className="chatgpt-auth-social-btn" onClick={() => navigate('/log-in')}>
                        <span className="chatgpt-auth-social-btn-inner">
                          <span className="chatgpt-auth-social-icon">
                            <img src={microsoftLogo} alt="" aria-hidden="true" />
                          </span>
                          <span>继续使用 Microsoft 登录</span>
                        </span>
                      </button>
                      <button type="button" className="chatgpt-auth-social-btn" onClick={() => navigate('/log-in')}>
                        <span className="chatgpt-auth-social-btn-inner">
                          <span className="chatgpt-auth-social-icon">
                            <img src={phoneIconSvg} alt="" aria-hidden="true" />
                          </span>
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
                          <span className="chatgpt-auth-error-icon">
                            <img src={errorCircleIcon} alt="" aria-hidden="true" />
                          </span>
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
