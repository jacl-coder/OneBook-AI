import { Link } from 'react-router-dom'
import { useTypewriter } from '@/shared/lib/ui/useTypewriter'

const readableBullets = [
  'Distraction-free reading and asking',
  'Seamless live query preview',
  'What you ask is what you can verify',
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
          <span className="brand-mark" aria-hidden="true">
            OB
          </span>
          <span>OneBook AI</span>
        </Link>
        <nav aria-label="main">
          <a href="#readable">Product</a>
          <a href="#accessibility">Features</a>
          <a href="#purchase">Get Started</a>
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
          <a className="tpx-scroll-hint" href="#readable" aria-label="scroll">
            ˅
          </a>
        </section>

        <section id="readable" className="tpx-section tpx-readable">
          <div className="tpx-readable-copy">
            <h2>Readable & Queryable</h2>
            <p>
              OneBook AI 让你在同一界面完成阅读与提问。上传 PDF / EPUB / TXT 后，系统解析内容并支持基于书籍上下文问答，
              回答附来源线索，便于核验。
            </p>
            <ul className="list-reset">
              {readableBullets.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </div>
          <div className="tpx-mock-window">
            <div className="tpx-window-bar">
              <span />
              <span />
              <span />
              <p className="mono">Typora-like Product Preview</p>
            </div>
            <article>
              <h3>OneBook AI</h3>
              <p className="mono">Q: 请总结第三章并给出页码依据。</p>
              <p className="mono">A: 第三章重点是范式与依赖关系，来源见第 82-91 页。</p>
            </article>
          </div>
        </section>

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
