import { useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { Link } from 'react-router-dom'
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

export function ChatPage() {
  const editorRef = useRef<HTMLDivElement>(null)
  const [prompt, setPrompt] = useState('')
  const [heading] = useState(
    () => headingPool[Math.floor(Math.random() * headingPool.length)],
  )

  const hasPrompt = prompt.trim().length > 0

  const syncPrompt = () => {
    const value = editorRef.current?.innerText ?? ''
    setPrompt(value.replace(/\u00a0/g, ' '))
  }

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!hasPrompt) return
    if (editorRef.current) editorRef.current.textContent = ''
    setPrompt('')
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
          <Link to="/login" className="chatgpt-top-btn chatgpt-top-btn-dark">
            登录
          </Link>
          <Link to="/login" className="chatgpt-top-btn chatgpt-top-btn-light">
            免费注册
          </Link>
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
                    onSubmit={handleSubmit}
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
    </div>
  )
}
