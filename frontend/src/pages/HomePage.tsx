import { useEffect, useLayoutEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useTypewriter } from '@/shared/lib/ui/useTypewriter'
import onebookFeaturePreview from '@/assets/home/feature-preview.svg'
import onebookLogoMark from '@/assets/brand/onebook-logo-mark.svg'
import modeReadPreview from '@/assets/home/mode-read.svg'
import modeAskPreview from '@/assets/home/mode-ask.svg'
import modeCitePreview from '@/assets/home/mode-cite.svg'
import twoQuotesMark from '@/assets/home/two-quotes.png'
import nextArrowIcon from '@/assets/icons/next-arrow.svg'
import uploadPlaceholder from '@/assets/features/upload.svg'
import parsePlaceholder from '@/assets/features/parse.svg'
import embedPlaceholder from '@/assets/features/embed.svg'
import retrievePlaceholder from '@/assets/features/retrieve.svg'
import citedPlaceholder from '@/assets/features/cited.svg'
import sessionPlaceholder from '@/assets/features/session.svg'
import libraryPlaceholder from '@/assets/features/library.svg'
import securityPlaceholder from '@/assets/features/security.svg'
import organizeLibrarySvg from '@/assets/workflow/organize-library.svg'
import outlineNavigationSvg from '@/assets/workflow/outline-navigation.svg'
import importExportSvg from '@/assets/workflow/import-export.svg'
import readingMetricsSvg from '@/assets/workflow/reading-metrics.svg'
import focusModeSvg from '@/assets/workflow/focus-mode.svg'
import smartPairingSvg from '@/assets/workflow/smart-pairing.svg'

const readableBullets = [
  { icon: 'fa-search', label: 'Ask Across Your Books' },
  { icon: 'fa-link', label: 'Source-Cited Answers' },
  { icon: 'fa-comments-o', label: 'Continuous Follow-up Context' },
]

const functionSlides = [
  {
    word: 'Upload',
    image: uploadPlaceholder,
    slogan: 'drag and drop your pdf, epub or txt to start',
    alt: 'Upload electronic books to OneBook AI',
  },
  {
    word: 'Parse',
    image: parsePlaceholder,
    slogan: 'extract text and structure from your books automatically',
    alt: 'Automatic text extraction and parsing',
  },
  {
    word: 'Embed',
    image: embedPlaceholder,
    slogan: 'turn content into searchable vector representations',
    alt: 'Vector embedding for semantic search',
  },
  {
    word: 'Retrieve',
    image: retrievePlaceholder,
    slogan: 'find the most relevant passages for your question',
    alt: 'Semantic retrieval of relevant book passages',
  },
  {
    word: 'Cited Answers',
    image: citedPlaceholder,
    slogan: 'answers grounded in your books, with page and chapter refs',
    alt: 'AI answers with source citations',
  },
  {
    word: 'Session History',
    image: sessionPlaceholder,
    slogan: 'pick up where you left off, every conversation preserved',
    alt: 'Conversation history and session management',
  },
  {
    word: 'Library',
    image: libraryPlaceholder,
    slogan: 'manage your books with status tracking and object storage',
    alt: 'Library management with MinIO object storage',
  },
  {
    word: 'Security',
    image: securityPlaceholder,
    slogan: 'rate limiting, brute-force detection and token revocation',
    alt: 'Multi-layer security protection',
  },
]

const accessibilityFeatures = [
  {
    title: 'Organize Library',
    detail:
      'Manage every book in one library with parse status, source metadata, and ask-ready entry points, so you can move from collection to questioning without manually tracking scattered files across folders, exports, and temporary drafts.',
    image: organizeLibrarySvg,
    alt: 'Library organization and status tracking',
  },
  {
    title: 'Outline Navigation',
    detail:
      'Use chapter and section outlines to jump directly to exact passages, then launch context-aware questions from that location instead of scrolling through long documents.',
    image: outlineNavigationSvg,
    alt: 'Outline-based chapter navigation',
  },
  {
    title: 'Import & Export',
    detail:
      'Import PDF, EPUB, and TXT while preserving originals, then export structured notes and extracted passages so your reading insights can be reused across later workflows.',
    image: importExportSvg,
    alt: 'Import and export workflow',
  },
  {
    title: 'Reading Metrics',
    detail:
      'Track pages, words, and reading time to review progress at a glance.',
    image: readingMetricsSvg,
    alt: 'Reading metrics and progress summary',
  },
  {
    title: 'Focus Mode',
    detail:
      'Highlight one active paragraph or answer thread while surrounding content stays subdued, keeping attention on the current reasoning path during deep reading and follow-up.',
    image: focusModeSvg,
    alt: 'Focus mode for concentrated reading and asking',
  },
  {
    title: 'Smart Pairing',
    detail:
      'Auto-complete markdown brackets, quotes, and emphasis markers in notes and prompts, reducing edit friction and formatting mistakes while you ask.',
    image: smartPairingSvg,
    alt: 'Automatic pairing of markdown symbols',
  },
]

const likeSlides = [
  {
    key: 'title',
    type: 'title' as const,
    title: 'and, loved by our users',
  },
  {
    key: 'quote-1',
    type: 'quote' as const,
    quote: 'I can finally ask my own materials directly instead of searching back and forth across dozens of pages.',
    author: 'Academic Research User',
  },
  {
    key: 'quote-2',
    type: 'quote' as const,
    quote: 'The most valuable part is traceable sources, not black-box answers.',
    author: 'Knowledge Base Operations Team',
  },
  {
    key: 'quote-3',
    type: 'quote' as const,
    quote: 'Session history is really useful. I can continue where I left off during review.',
    author: 'Long-term Reader',
  },
  {
    key: 'more',
    type: 'more' as const,
    lead: 'Here are more words from our users.',
  },
]

const modePreviews = [
  {
    key: 'read',
    image: modeReadPreview,
    alt: 'Read mode preview',
  },
  {
    key: 'ask',
    image: modeAskPreview,
    alt: 'Ask mode preview',
  },
  {
    key: 'cite',
    image: modeCitePreview,
    alt: 'Cite mode preview',
  },
] as const

const footerColumns = [
  {
    title: 'Product',
    links: [
      { label: 'Readable & Writable', href: '#readable' },
      { label: 'Functions', href: '#function' },
      { label: 'Workflow', href: '#accessibility' },
      { label: 'Modes', href: '#modes' },
    ],
  },
  {
    title: 'Workspace',
    links: [
      { label: 'Library', href: '/library' },
      { label: 'Chat', href: '/chat' },
      { label: 'History', href: '/history' },
      { label: 'Login', href: '/login' },
    ],
  },
  {
    title: 'Explore',
    links: [
      { label: 'Loved by Users', href: '#quotes' },
      { label: 'More in Docs', href: '#accessibility' },
      { label: 'Get Started', href: '#purchase' },
    ],
  },
  {
    title: 'OneBook AI',
    links: [
      { label: 'Source-Cited Answers', href: '#function' },
      { label: 'Session Memory', href: '#function' },
      { label: 'Private Book Q&A', href: '#readable' },
    ],
  },
] as const

const contactLinks = [
  { label: 'Email', href: 'mailto:laix1024@gmail.com' },
  { label: 'GitHub', href: 'https://github.com/jacl-coder/OneBook-AI' },
  { label: 'Support', href: 'https://github.com/jacl-coder/OneBook-AI/issues' },
] as const

export function HomePage() {
  const [activeHash, setActiveHash] = useState<string>('')
  const [activeFunctionDot, setActiveFunctionDot] = useState(0)
  const [isFunctionAutoPlay, setIsFunctionAutoPlay] = useState(true)
  const [activeVoiceIndex, setActiveVoiceIndex] = useState(0)
  const [functionSlideSpeedMs, setFunctionSlideSpeedMs] = useState(500)
  const [functionSlideDelayMs, setFunctionSlideDelayMs] = useState(6000)

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

  useEffect(() => {
    const syncSliderMotion = () => {
      const viewportWidth = window.innerWidth || 1200
      setFunctionSlideSpeedMs(Math.max(320, Math.round(viewportWidth / 2)))
      setFunctionSlideDelayMs(Math.max(1800, Math.round(1000 + 4 * viewportWidth)))
    }

    syncSliderMotion()
    window.addEventListener('resize', syncSliderMotion)
    return () => window.removeEventListener('resize', syncSliderMotion)
  }, [])

  useLayoutEffect(() => {
    const syncContentResizerTop = () => {
      const viewportHeight = window.innerHeight || document.documentElement.clientHeight || 0
      const resizers = document.querySelectorAll<HTMLElement>('.tpx-content-resizer')
      resizers.forEach((resizer) => {
        const blockHeight = resizer.offsetHeight
        const top = viewportHeight / 2 - blockHeight / 2
        resizer.style.top = `${top}px`
      })
    }

    syncContentResizerTop()
    window.addEventListener('resize', syncContentResizerTop)

    return () => {
      window.removeEventListener('resize', syncContentResizerTop)
    }
  }, [])

  useEffect(() => {
    if (!isFunctionAutoPlay) {
      return
    }

    const intervalId = window.setInterval(() => {
      setActiveFunctionDot((prev) => (prev + 1) % functionSlides.length)
    }, functionSlideDelayMs)

    return () => window.clearInterval(intervalId)
  }, [functionSlideDelayMs, isFunctionAutoPlay])

  const navActiveClass = (hash: string) => (activeHash === hash ? 'tpx-nav-active' : '')
  const handleFunctionDotClick = (index: number) => {
    setActiveFunctionDot(index)
    setIsFunctionAutoPlay(false)
  }
  const handleNextVoice = () => {
    setActiveVoiceIndex((prev) => (prev + 1) % likeSlides.length)
  }

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
          <img className="tpx-logo-mark" src={onebookLogoMark} alt="" aria-hidden="true" />
          {' '}
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
            <p className="tpx-slogon tpx-intro-slogon">
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
              <img
                className="tpx-feature-video"
                src={onebookFeaturePreview}
                alt="OneBook AI feature preview with source-cited answers and continuous context"
                loading="lazy"
                decoding="async"
              />
            </div>
          </div>
        </section>

        <hr className="tpx-divider" />

        <section className="tpx-section tpx-simple" id="function">
          <h3>Simple, yet Verifiable</h3>
          <div className="tpx-function-menu">
            <ul className="tpx-function-dots">
              {functionSlides.map((slide, index) => (
                <li className={`tpx-dot ${index === activeFunctionDot ? 'tpx-dot-active' : ''}`} key={slide.word}>
                  <a
                    href="#function"
                    className="tpx-dot-link"
                    onClick={(event) => {
                      event.preventDefault()
                      handleFunctionDotClick(index)
                    }}
                  >
                    {index === functionSlides.length - 1 ? slide.word : `${slide.word},`}
                  </a>
                </li>
              ))}
            </ul>
          </div>
          <div className="tpx-function-slider" aria-label="OneBook feature flow">
            <ul
              className="tpx-function-slides"
              style={{
                width: `${functionSlides.length * 100}%`,
                transform: `translateX(-${activeFunctionDot * (100 / functionSlides.length)}%)`,
                transitionDuration: `${functionSlideSpeedMs}ms`,
              }}
            >
              {functionSlides.map((slide, index) => (
                <li
                  className={`tpx-slide ${index === activeFunctionDot ? 'tpx-slide-active' : ''}`}
                  key={slide.word}
                  style={{ width: `${100 / functionSlides.length}%` }}
                  aria-hidden={index !== activeFunctionDot}
                >
                  <div className="tpx-slider-img-wrapper">
                    <div className="tpx-function-main">
                      <img
                        src={slide.image}
                        className="tpx-img-rounded"
                        alt={slide.alt}
                        loading="lazy"
                        decoding="async"
                      />
                    </div>
                  </div>
                  <div className="tpx-slogon">
                    {/* Keep Typora-like color split: gray comment wrapper + dark-red inner text. */}
                    {'/* '}
                    <a>{slide.slogan}</a>
                    {' */'}
                  </div>
                </li>
              ))}
            </ul>
          </div>
        </section>

        <hr className="tpx-divider" />

        <section id="accessibility" className="tpx-accessibility">
          <h3 className="tpx-accessibility-title">Workflow</h3>
          <p className="tpx-slogon tpx-access-lead">
            {'/* '}
            <a style={{ cursor: 'text' }}>You focus on the content, OneBook AI helps with the rest</a>
            {' */'}
          </p>

          <div className="tpx-access-wrap">
            <div className="tpx-access-wrap-inner">
              {accessibilityFeatures.map((feature) => (
                <article className="tpx-feature-box" key={feature.title}>
                  <div className="tpx-feature-box-img">
                    <img src={feature.image} alt={feature.alt} loading="lazy" decoding="async" />
                  </div>
                  <div className="tpx-feature-box-title">{feature.title}</div>
                  <div className="tpx-feature-box-desc">{feature.detail}</div>
                </article>
              ))}
            </div>
          </div>

          <div className="tpx-slogon tpx-access-more">/* <a>More in OneBook Docs</a> */</div>
        </section>

        <hr className="tpx-divider" />

        <section className="tpx-section tpx-themes" id="modes">
          <h2>Modes</h2>
          <p className="tpx-slogon tpx-themes-lead">
            {'/* '}
            <a style={{ cursor: 'text' }}>One library, three views: Read, Ask, Cite.</a>
            {' */'}
          </p>
          <div className="tpx-theme-stack" aria-label="theme stack preview">
            {modePreviews.map((mode, index) => (
              <article
                className={`tpx-mode-card ${index === 0 ? 'tpx-mode-card-first' : ''}`}
                key={mode.key}
              >
                <img
                  className="tpx-mode-preview-img"
                  src={mode.image}
                  alt={mode.alt}
                  loading="lazy"
                  decoding="async"
                />
              </article>
            ))}
          </div>
        </section>

        <section className="tpx-section tpx-voices tpx-voices-done" id="quotes">
          <div className="tpx-like-card-slide">
            <ul
              className="tpx-like-track"
              style={{
                width: `${likeSlides.length * 100}%`,
                transform: `translateX(-${activeVoiceIndex * (100 / likeSlides.length)}%)`,
              }}
              aria-live="polite"
            >
              {likeSlides.map((slide, index) => (
                <li
                  className={`tpx-like-dot ${index === activeVoiceIndex ? 'tpx-like-dot-active' : ''}`}
                  key={slide.key}
                  style={{ width: `${100 / likeSlides.length}%` }}
                  aria-hidden={index !== activeVoiceIndex}
                >
                  <div className="tpx-like-card">
                    <div className="tpx-like-card-inner">
                      {slide.type === 'title' ? (
                        <div className="tpx-like-sentence">
                          <h5>and, loved by our users</h5>
                        </div>
                      ) : null}
                      {slide.type === 'quote' ? (
                        <>
                          <div aria-hidden="true">
                            <img className="tpx-like-quote-img" src={twoQuotesMark} alt="" />
                          </div>
                          <div className="tpx-like-sentence">{slide.quote}</div>
                          <div className="tpx-like-user">
                            <span className="tpx-like-user-line" aria-hidden="true" />
                            {slide.author}
                          </div>
                        </>
                      ) : null}
                      {slide.type === 'more' ? (
                        <div className="tpx-like-sentence">
                          <a href="#quotes">
                            <span>Here</span> are more words from our users.
                          </a>
                        </div>
                      ) : null}
                    </div>
                    <button
                      type="button"
                      className={`tpx-like-next ${slide.type === 'more' ? 'tpx-like-next-left' : ''}`}
                      onClick={handleNextVoice}
                      aria-label="next quote"
                    >
                      <img src={nextArrowIcon} alt="" aria-hidden="true" />
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section className="tpx-section tpx-purchase" id="purchase">
          <div className="tpx-content-resizer">
            <div className="tpx-purchase-center">
              <div className="tpx-purchase-title-wrap">
                <h2>want OneBook AI ?</h2>
              </div>

              <div className="tpx-sub-group">
                <div className="tpx-purchase-surface">
                  <div className="tpx-purchase-product-row">
                    <div className="tpx-purchase-app-icon-wrap">
                      <img
                        className="tpx-purchase-app-icon"
                        src={onebookLogoMark}
                        alt="OneBook AI app icon"
                        loading="eager"
                        decoding="async"
                      />
                    </div>
                    <div className="tpx-purchase-product-copy">
                      <div>
                        <span className="tpx-purchase-meta">
                          login · cited answers · session memory
                        </span>
                      </div>
                      <div>
                        <p className="tpx-purchase-desc">
                          Ask across your own books with source-grounded responses.
                        </p>
                      </div>
                      <div className="tpx-purchase-price-row">
                        <span className="tpx-purchase-price-chip">Free</span>
                        <span className="tpx-purchase-price-note">no subscription · no payment</span>
                      </div>
                    </div>
                  </div>

                  <div className="tpx-purchase-btn-group">
                    <Link to="/login" className="tpx-purchase-btn tpx-purchase-btn-dark">
                      <i className="fa fa-key tpx-purchase-btn-icon" aria-hidden="true" />
                      <span className="tpx-purchase-btn-label">Login</span>
                    </Link>
                    <Link to="/chat" className="tpx-purchase-btn tpx-purchase-btn-ghost">
                      <i className="fa fa-arrow-right tpx-purchase-btn-icon" aria-hidden="true" />
                      <span className="tpx-purchase-btn-label">Try</span>
                    </Link>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer className="tpx-footer" id="contact">
        <section className="tpx-footer-contact">
          <div className="tpx-footer-contact-inner">
            <h3 className="tpx-footer-plane">
              <i className="fa fa-paper-plane-o" aria-hidden="true" />
            </h3>
            <p className="tpx-footer-contact-links">
              {contactLinks.map((item, index) => (
                <span key={item.label}>
                  {index > 0 ? <span aria-hidden="true"> · </span> : null}
                  <a
                    href={item.href}
                    target={item.href.startsWith('http') ? '_blank' : undefined}
                    rel={item.href.startsWith('http') ? 'noreferrer' : undefined}
                  >
                    {item.label}
                  </a>
                </span>
              ))}
            </p>
            <small>real reading · real asking · real answers</small>
          </div>
        </section>

        <section className="tpx-footer-about">
          <div className="tpx-footer-about-inner">
            <div className="tpx-footer-columns">
              {footerColumns.map((column) => (
                <div className="tpx-footer-column" key={column.title}>
                  <h5>{column.title}</h5>
                  {column.links.map((item) =>
                    item.href.startsWith('/') ? (
                      <Link key={item.label} to={item.href}>
                        {item.label}
                      </Link>
                    ) : (
                      <a key={item.label} href={item.href}>
                        {item.label}
                      </a>
                    ),
                  )}
                </div>
              ))}
            </div>
            <div className="tpx-footer-brand" aria-label="OneBook AI brand">
              <img src={onebookLogoMark} alt="" aria-hidden="true" />
              <span>OneBook AI</span>
            </div>
          </div>
        </section>
      </footer>
    </div>
  )
}
