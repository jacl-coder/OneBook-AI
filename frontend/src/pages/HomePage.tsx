import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useTypewriter } from '@/shared/lib/ui/useTypewriter'

const readableBullets = [
  { icon: 'fa-search', label: 'Ask Across Your Books' },
  { icon: 'fa-link', label: 'Source-Cited Answers' },
  { icon: 'fa-comments-o', label: 'Continuous Follow-up Context' },
]

const simplePowerful = [
  'PDF Upload',
  'EPUB Support',
  'TXT Support',
  'Cited Answers',
  'Session History',
  'Library Management',
  'Status Tracking',
]

const accessibilityCards = [
  {
    title: 'Organize Library',
    detail: '按书籍管理文件，快速定位需要提问的资料。',
  },
  {
    title: 'Outline Navigation',
    detail: '基于章节结构快速跳转，减少翻阅成本。',
  },
  {
    title: 'Import & Export',
    detail: '支持导入常见电子书格式并保留原始文件管理。',
  },
  {
    title: 'Word & Context Focus',
    detail: '针对局部段落提问，保持上下文聚焦。',
  },
  {
    title: 'Follow-up Questions',
    detail: '基于已有会话继续追问，形成连续学习链路。',
  },
  {
    title: 'Source Trace',
    detail: '回答附章节/页码/段落来源，便于回到原文核验。',
  },
]

const userVoices = [
  '“终于可以直接问我的资料，而不是在几十页里来回找。”',
  '“最有价值的是来源可追溯，答案不是黑盒。”',
  '“会话历史很实用，复习时能直接续聊。”',
]

export function HomePage() {
  const [activeHash, setActiveHash] = useState<string>('')

  useEffect(() => {
    const sections = ['readable', 'accessibility', 'purchase']
    let rafId = 0

    const syncByScroll = () => {
      const scrollTop = window.scrollY || document.documentElement.scrollTop
      const threshold = Math.round(window.innerHeight * 0.5)
      let current = ''

      for (const id of sections) {
        const section = document.getElementById(id)
        if (!section) continue
        const top = Math.round(section.getBoundingClientRect().top + window.scrollY)
        if (top - threshold < scrollTop) {
          current = `#${id}`
        }
      }

      setActiveHash((prev) => (prev === current ? prev : current))
    }

    const onScrollOrResize = () => {
      if (rafId) return
      rafId = window.requestAnimationFrame(() => {
        rafId = 0
        syncByScroll()
      })
    }

    syncByScroll()
    window.addEventListener('scroll', onScrollOrResize, { passive: true })
    window.addEventListener('resize', onScrollOrResize)

    return () => {
      if (rafId) window.cancelAnimationFrame(rafId)
      window.removeEventListener('scroll', onScrollOrResize)
      window.removeEventListener('resize', onScrollOrResize)
    }
  }, [])

  const navActiveClass = (hash: string) => (activeHash === hash ? 'tpx-nav-active' : '')

  const typedSlogan = useTypewriter({
    strings: [
      'a readable, queryable library.',
      'read and ask your own books.',
      'answers with sources, not guesses.',
    ],
    typeSpeed: 100,
    deleteSpeed: 50,
    holdDelay: 1500,
    loop: true,
  })

  return (
    <div className="tpx-page">
      <header className="tpx-nav">
        <Link to="/" className="tpx-logo">
          <span className="tpx-logo-mark" aria-hidden="true">
            OB
          </span>
          <span className="tpx-logo-title">OneBook AI</span>
        </Link>
        <nav aria-label="main">
          <a href="#readable" className={navActiveClass('#readable')}>
            Product
          </a>
          <a href="#accessibility" className={navActiveClass('#accessibility')}>
            Features
          </a>
          <a href="#purchase" className={navActiveClass('#purchase')}>
            Get Started
          </a>
        </nav>
      </header>

      <main>
        <section className="tpx-intro" id="home">
          <div className="tpx-intro-center">
            <h1>onebook ai</h1>
            <p className="tpx-intro-slogan">
              {/* Match Typora-style animated slogan while keeping React-native state control. */}
              {'/* '}
              <span className="tpx-typed-quotes">{typedSlogan.text}</span>
              {typedSlogan.showCursor ? (
                <span className="tpx-typed-cursor" aria-hidden="true">
                  |
                </span>
              ) : null}
              {' */'}
            </p>
          </div>
          <a className="tpx-scroll-hint" href="#readable" aria-label="scroll to product section">
            <i className="fa fa-angle-down tpx-scroll-icon" aria-hidden="true" />
          </a>
        </section>

        <section id="readable" className="tpx-section tpx-readable">
          <div className="tpx-readable-copy">
            <div className="tpx-readable-copy-inner">
              <h2>Queryable & Verifiable</h2>
              <p>
                OneBook AI turns your personal library into a source-grounded Q&A workspace.
                Upload PDF, EPUB, or TXT, ask questions against your own books, and get answers
                with traceable citations. Keep follow-up questions in one continuous session
                without losing context.
              </p>
              <div className="tpx-readable-points">
                {readableBullets.map((item) => (
                  <h4 className="tpx-readable-point" key={item.label}>
                    <i className={`fa ${item.icon}`} aria-hidden="true" />
                    <span>{item.label}</span>
                  </h4>
                ))}
              </div>
            </div>
          </div>
          <div className="tpx-readable-media">
            <div className="tpx-feature-wrap">
              <video
                className="tpx-feature-video"
                autoPlay
                muted
                loop
                playsInline
                preload="metadata"
                poster="https://typora.io/img/beta-thumbnail.png"
                src="https://typora.io/img/beta.mp4"
              />
            </div>
          </div>
        </section>

        <hr className="tpx-divider" />

        <section className="tpx-section tpx-simple">
          <h2>Simple, yet Powerful</h2>
          <p>Built for real reading, real asking, and real knowledge reuse.</p>
          <div className="tpx-keywords">
            {simplePowerful.map((word) => (
              <span key={word}>{word}</span>
            ))}
          </div>
          <div className="tpx-floating-card">
            <div className="tpx-window-bar">
              <span />
              <span />
              <span />
              <p className="mono">library-note.md</p>
            </div>
            <div className="tpx-floating-body">
              <h3>Big Idea</h3>
              <p>把个人电子书库变成可检索、可提问、可追溯的知识系统。</p>
              <h4>Chapter 1</h4>
              <p>Upload → Parse → Ready → Ask → Trace → Continue</p>
            </div>
          </div>
        </section>

        <section id="accessibility" className="tpx-section tpx-accessibility">
          <h2>Accessibility</h2>
          <p>You focus on your content, OneBook AI handles retrieval and structure.</p>
          <div className="tpx-access-grid">
            {accessibilityCards.map((card) => (
              <article key={card.title}>
                <div className="tpx-icon-box" aria-hidden="true">
                  ◻
                </div>
                <h3>{card.title}</h3>
                <p>{card.detail}</p>
              </article>
            ))}
          </div>
          <p className="tpx-more">More in product experience</p>
        </section>

        <section className="tpx-section tpx-themes">
          <h2>Custom Themes</h2>
          <p>Clean structure that fits different reading and asking workflows.</p>
          <div className="tpx-theme-stack" aria-label="theme stack preview">
            <div />
            <div />
            <div />
            <div />
            <div />
          </div>
        </section>

        <section className="tpx-section tpx-voices" id="quotes">
          <div className="tpx-voices-overlay">
            <h2>and, loved by our users</h2>
            <div className="tpx-voice-list">
              {userVoices.map((voice) => (
                <blockquote key={voice}>{voice}</blockquote>
              ))}
            </div>
          </div>
        </section>

        <section className="tpx-section tpx-purchase" id="purchase">
          <h2>want OneBook AI ?</h2>
          <div className="tpx-product-card">
            <div className="tpx-product-icon">T</div>
            <div className="tpx-product-copy">
              <strong>OneBook AI</strong>
              <p>Personal AI layer for your own books</p>
              <p className="mono">upload / parse / ask / cite / continue</p>
            </div>
          </div>
          <div className="tpx-purchase-actions">
            <Link to="/app/library" className="tpx-btn tpx-btn-dark">
              Upload Now
            </Link>
            <Link to="/app/chat" className="tpx-btn">
              Ask Now
            </Link>
          </div>
          <p className="tpx-purchase-note">PDF / EPUB / TXT · source trace available</p>
        </section>
      </main>

      <footer className="tpx-footer">
        <div className="tpx-footer-inner">
          <p>✈</p>
          <nav aria-label="footer">
            <Link to="/app/library">Library</Link>
            <Link to="/app/chat">Chat</Link>
            <Link to="/app/history">History</Link>
          </nav>
          <small>real reading real asking real answers</small>
        </div>
      </footer>
    </div>
  )
}
