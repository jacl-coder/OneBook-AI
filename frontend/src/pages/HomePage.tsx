import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { useTypewriter } from '@/shared/lib/ui/useTypewriter'
import onebookFeaturePreview from '@/assets/home/feature-preview.svg'
import onebookLogoMark from '@/assets/brand/onebook-logo-mark.svg'
import modeReadPreview from '@/assets/home/mode-read.svg'
import modeAskPreview from '@/assets/home/mode-ask.svg'
import modeCitePreview from '@/assets/home/mode-cite.svg'
import twoQuotesMark from '@/assets/home/two-quotes.png'
import userBg from '@/assets/home/user-bg.jpg'
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
      { label: 'Chat', href: '/chat' },
      { label: 'Login', href: '/log-in' },
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

const cx = (...values: Array<string | false | null | undefined>) => values.filter(Boolean).join(' ')

const homeTw = {
  page: "min-h-screen bg-[#fbfbfb] text-[#232323] [font-family:'Raleway','Century_Gothic','Noto_Sans_SC','IBM_Plex_Sans',sans-serif]",
  nav: "fixed inset-x-0 top-0 z-10 grid h-[70px] grid-cols-[auto_1fr] items-start border-b border-[#eee] bg-white px-4 pt-[15px] max-[760px]:h-auto max-[760px]:grid-cols-1 max-[760px]:justify-items-start max-[760px]:gap-[0.55rem] max-[760px]:p-[0.64rem]",
  logo: "float-left mt-[-8px] block pl-4 text-[#555] [font-family:'Century_Gothic','TeXGyreAdventor','STHeiti',sans-serif] text-[1.67em] font-normal tracking-normal max-[760px]:float-none max-[760px]:mt-0 max-[760px]:pl-0 max-[760px]:text-[1.25em]",
  logoMark: "inline-block h-12 w-12 shrink-0 align-middle object-contain max-[760px]:h-[38px] max-[760px]:w-[38px]",
  logoTitle: "ml-0 mt-[-4px] inline-block align-middle text-inherit [font-family:inherit] [font-style:inherit] [font-weight:inherit] [letter-spacing:inherit] max-[760px]:mt-[-1px]",
  navList: "flex justify-self-end gap-0 max-[760px]:justify-self-start max-[760px]:flex-wrap",
  navLink: "inline-block px-[1em] py-[0.5em] text-[16px] font-normal leading-normal text-[#777] transition-[color,background-color] duration-200 ease-in [font-family:'Century_Gothic','TeXGyreAdventor',sans-serif] hover:bg-[#eee] hover:text-[#777]",
  navLinkActive: "bg-[#eee] !text-[#777]",
  intro: "relative h-screen border-b border-[#ddd] bg-[#fbfbfb] max-[760px]:h-auto max-[760px]:min-h-[86vh]",
  introCenter: "absolute top-[58%] w-full min-h-[200px] -translate-y-1/2 px-4 text-center",
  introTitle: "mb-[34px] px-[30px] text-[48px] font-normal leading-[48px] [font-family:'Century_Gothic','TeXGyreAdventor','STHeiti',sans-serif] max-[760px]:text-[clamp(2.2rem,10vw,3rem)]",
  slogan: "mt-[-10px] text-center text-[12px] font-medium uppercase leading-[22px] tracking-[6px] text-[#999] [font-family:'Raleway','Century_Gothic','TeXGyreAdventor',sans-serif] max-[980px]:mt-[14px] max-[980px]:tracking-[4px]",
  introSlogan: "my-[18px] mb-[20px] mt-[18px] max-[760px]:text-[11px] max-[760px]:tracking-[3px]",
  typedCursor: "ml-px inline-block animate-[tpx-cursor-blink_0.7s_infinite] motion-reduce:animate-none",
  scrollHint: "group absolute inset-x-0 bottom-0 h-8 cursor-pointer text-[#4d4d4d] transition-all duration-200 ease-in focus-visible:rounded-[6px] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#93c5fd]",
  scrollIcon: "absolute left-1/2 top-0 h-7 w-[18px] -translate-x-1/2 text-center text-[28px] font-normal leading-7 text-black opacity-40 transition-all duration-200 ease-in animate-[tpx-scroll-pulse_1.5s_ease_infinite] group-hover:opacity-75 group-focus-visible:opacity-75",
  section: "mx-auto w-[min(1060px,calc(100%-2rem))] py-[5.8rem] max-[760px]:w-[calc(100%-1.2rem)] max-[760px]:py-[4.2rem]",
  readable: "grid w-full max-w-none min-h-[500px] grid-cols-[49%_49%] content-start justify-start gap-0 box-content pb-[100px] pt-[200px] max-[980px]:grid-cols-1 max-[980px]:gap-[1.4rem] max-[980px]:pb-[4rem] max-[980px]:pt-[5.2rem]",
  readableCopy: "relative min-h-[500px] min-w-[100px] w-full max-[980px]:min-h-0",
  readableCopyInner: "absolute right-[50px] w-full max-w-[500px] box-content pl-5 text-[#888] [font-family:'Century_Gothic','TeXGyreAdventor',sans-serif] max-[980px]:static max-[980px]:right-auto max-[980px]:max-w-none max-[980px]:pl-0",
  readableTitle: "my-[36px] text-[36px] font-[450] tracking-normal text-black [text-rendering:auto]",
  readableBody: "mb-5 mt-0 max-w-none text-base leading-[1.75] text-[#868686]",
  readablePoints: "mt-0",
  readablePoint: "mb-[10px] mt-[26.6px] whitespace-pre-wrap text-[20px] font-[450] [line-height:normal] text-[#344048]",
  readablePointIcon: "inline-block w-[26px] text-center text-inherit",
  readableMedia: "w-full text-left",
  featureWrap: "inline-block p-[10px] pl-[50px] text-center max-[980px]:block max-[980px]:p-0",
  featureVideo: "inline-block h-auto w-full max-w-[560px] rounded-[8px] border-0 shadow-[0_0_8px_3px_#ccc]",
  divider: "my-2 w-full border-0 border-t border-dashed border-t-[#aaa] opacity-100",
  simple: "m-0 w-full max-w-none py-[100px] text-center text-[14px] [font-family:'Century_Gothic','TeXGyreAdventor','STHeiti',sans-serif] max-[980px]:py-[80px] max-[760px]:w-full max-[760px]:py-[72px]",
  simpleTitle: "my-[36px] text-[36px] font-[100] [line-height:normal] text-black max-[760px]:my-[28px] max-[760px]:text-[32px]",
  functionMenu: "mx-auto text-center",
  functionDots: "m-0 inline-block max-w-full list-none overflow-x-hidden p-0 select-none [font-family:'Century_Gothic','TeXGyreAdventor','STHeiti',sans-serif] max-[760px]:max-w-[calc(100%-2rem)] max-[760px]:whitespace-normal",
  functionDot: "relative m-0 inline-block h-full p-0 align-middle",
  functionDotLink: "block cursor-pointer whitespace-nowrap px-2 py-2 text-[16px] leading-normal text-[#777] no-underline transition-colors duration-200 ease-in hover:bg-[#eee] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#93c5fd] max-[760px]:px-2 max-[760px]:py-[6px] max-[760px]:text-[14px] max-[760px]:leading-[22px]",
  functionDotActive: "bg-[#eee]",
  functionSlider: "relative !w-[100vw] ml-[calc(50%-50vw)] mt-2 min-h-[560px] overflow-hidden max-[980px]:min-h-0",
  functionSlides: "relative left-0 m-0 flex min-h-[560px] list-none p-0 transition-transform ease-[ease] will-change-transform max-[980px]:min-h-0",
  functionSlide: "relative flex-[0_0_auto]",
  sliderImgWrap: "flex justify-center max-[980px]:block",
  functionMain: "w-[600px] max-[980px]:mx-auto max-[980px]:w-[min(600px,calc(100vw-2rem))]",
  imgRounded: "block h-auto w-full rounded-[6px]",
  sloganLink: "text-[rgb(153,0,0)] no-underline",
  accessibility: "m-0 w-full max-w-none py-[100px] text-center text-[14px] text-black [font-family:'Century_Gothic','TeXGyreAdventor','STHeiti',sans-serif]",
  accessibilityTitle: "my-[36px] text-[36px] font-[100] [line-height:normal] text-black",
  accessLead: "!mt-0 mb-5 max-[980px]:!mt-0",
  accessWrap: "relative mt-12",
  accessWrapInner: "mx-auto max-w-[1100px]",
  featureBox: "mb-4 mt-16 inline-block h-[200px] w-[300px] align-top text-left mx-4 max-[980px]:mb-0 max-[980px]:mt-6 max-[980px]:mx-0 max-[980px]:h-auto max-[980px]:w-[min(320px,100%)] max-[760px]:w-full",
  featureBoxImg: "h-[100px] w-[300px]",
  featureBoxImgEl: "!mt-0 h-full w-full",
  featureBoxTitle: "min-w-[120px] text-left text-[16px] leading-[36px] text-[#546673]",
  featureBoxDesc: "text-[12px] leading-[1.45em] text-[#888]",
  accessMore: "!mb-0 !mt-10 max-[980px]:!mt-[14px]",
  themes: "box-content h-[530px] w-full max-w-none overflow-hidden py-[80px] text-center max-[760px]:h-auto max-[760px]:w-full max-[760px]:overflow-visible max-[760px]:py-[4.2rem]",
  themesTitle: "my-[36px] text-[36px] font-[100] [line-height:normal] tracking-normal [font-family:'Century_Gothic','TeXGyreAdventor','STHeiti',sans-serif]",
  themesLead: "relative !mb-0 !mt-0 max-[980px]:!mt-[14px]",
  themeStack: "relative left-1/2 mt-[60px] flex w-screen -translate-x-1/2 items-start justify-center overflow-hidden max-[980px]:mt-11 max-[980px]:h-[430px] max-[760px]:left-1/2 max-[760px]:mt-8 max-[760px]:w-screen max-[760px]:-translate-x-1/2 max-[760px]:origin-top max-[760px]:scale-[0.63] max-[760px]:overflow-visible",
  modeCard: "mr-[-500px] shrink-0 max-[980px]:mr-[-380px]",
  modeCardFirst: "ml-0",
  modeCardLast: "!mr-0",
  modePreview: "block h-auto w-[600px] max-w-[600px] rounded-[6px] max-[980px]:w-[460px] max-[980px]:max-w-[460px]",
  voices: "relative m-0 h-[400px] w-full max-w-none overflow-hidden bg-[#42444b] bg-cover bg-center p-0 max-[760px]:h-auto",
  likeCardSlide: "m-auto h-full bg-[rgba(66,68,75,0.88)]",
  likeTrack: "m-0 flex list-none p-0 transition-transform duration-[520ms] ease-out will-change-transform max-[760px]:block max-[760px]:w-full max-[760px]:!transform-none max-[760px]:transition-none",
  likeDot: "h-[400px] flex-[0_0_auto] opacity-0 transition-opacity duration-500 ease-out max-[760px]:h-auto max-[760px]:w-full max-[760px]:opacity-100 max-[760px]:transition-none",
  likeDotActive: "opacity-100 duration-200 ease-in max-[760px]:!block",
  likeDotMobileHidden: "max-[760px]:!hidden",
  likeCard: "mx-auto max-w-[996px] px-6 text-white max-[760px]:px-3",
  likeCardInner: "inline-block h-auto w-[calc(100%-120px)] align-middle text-left max-[760px]:mt-10 max-[760px]:w-full",
  likeSentence: "ml-12 p-0 text-[28px] leading-[1.3em] text-inherit [font-family:'PT_Serif','Crimson_Text','Lora',serif] [font-smoothing:antialiased] max-[760px]:ml-0 max-[760px]:text-[18px] max-[760px]:leading-[1.35]",
  likeSentenceTitle: "m-0 text-[48px] font-normal [font-family:inherit] max-[760px]:text-[32px]",
  likeQuoteImg: "block w-12 rotate-180",
  likeUser: "ml-12 mt-6 text-right text-[18px] [font-family:'Century_Gothic','TeXGyreAdventor','STHeiti',sans-serif] max-[760px]:ml-0 max-[760px]:mt-3 max-[760px]:text-[14px]",
  likeUserLine: "mr-[10px] inline-block h-px w-[60px] align-middle bg-white max-[760px]:w-10",
  likeNext: "inline-block h-[360px] w-[100px] cursor-pointer border-0 bg-transparent text-inherit hover:text-[rgba(255,255,255,0.82)] focus-visible:rounded-[4px] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[rgba(255,255,255,0.9)] max-[760px]:hidden",
  likeNextLeft: "[&>img]:rotate-180 [&>svg]:rotate-180",
  likeNextIcon: "h-[18px] w-[42px] fill-current",
  purchase: "relative mt-[-70px] h-[calc(100vh-70px)] min-h-[800px] w-full max-w-none overflow-y-visible bg-[#f6f6f6] pt-[70px] text-center text-[#4d4d4d] [font-family:museo-slab,Georgia,'Times_New_Roman',Times,serif] max-[760px]:mt-0 max-[760px]:h-auto max-[760px]:min-h-0 max-[760px]:w-full",
  contentResizer: "absolute top-[200px] min-h-[200px] w-full p-0 max-[760px]:static max-[760px]:min-h-0 max-[760px]:px-4 max-[760px]:pb-11 max-[760px]:pt-2",
  purchaseCenter: "mx-auto w-[min(600px,100%)] pb-14 max-[760px]:w-full max-[760px]:p-0",
  purchaseTitleWrap: "mx-auto max-w-[600px]",
  purchaseTitle: "mb-[36px] mt-0 text-[36px] font-normal [line-height:normal] tracking-normal [font-family:'Century_Gothic',sans-serif]",
  subGroup: "mt-20 max-[760px]:mt-11",
  purchaseSurface: "mx-auto w-[420px] max-w-[calc(100vw-2rem)] border-0 bg-transparent p-0 shadow-none",
  purchaseProductRow: "m-0 grid w-full grid-cols-[128px_minmax(0,1fr)] items-center justify-start gap-x-7 max-[760px]:w-full",
  purchaseAppIconWrap: "block h-auto w-[128px] border-0 bg-transparent shadow-none",
  purchaseAppIcon: "h-[128px] w-[128px] object-contain",
  purchaseProductCopy: "pl-0 text-left max-[760px]:pt-3 max-[760px]:text-center",
  purchaseMeta: "inline-block text-[12px] uppercase tracking-[0.04em] text-[#747474] [font-family:'Arial',sans-serif]",
  purchaseDesc: "mt-2 text-[14px] leading-[1.45] text-[#4f4f4f] [font-family:'Arial',sans-serif]",
  purchasePriceRow: "mt-3 flex flex-wrap items-center gap-2.5 max-[760px]:justify-center",
  purchasePriceChip: "inline-flex cursor-default items-center justify-center rounded-[4px] bg-[#e4e4e4] px-3 py-1 text-[16px] font-normal text-[#192023] [font-family:'Arial',sans-serif]",
  purchasePriceNote: "ml-0 text-[13px] text-[#5b5b5b] opacity-[0.78] [font-family:'Arial',sans-serif]",
  purchaseBtnGroup: "mt-8 grid w-full grid-cols-[128px_minmax(0,1fr)] items-center gap-0 [font-family:'Arial',sans-serif] max-[760px]:mt-[14px] max-[760px]:inline-flex max-[760px]:w-auto max-[760px]:max-w-full max-[760px]:gap-3",
  purchaseBtn: "inline-flex h-10 min-w-[176px] appearance-none items-center justify-center gap-2 rounded-[4px] border px-[15px] text-[16px] font-normal tracking-normal shadow-[0_0_5px_#d8ddec] transition-all duration-[240ms] ease-[ease] hover:brightness-95 max-[760px]:w-[146px] max-[760px]:min-w-[146px]",
  purchaseBtnDark: "justify-self-start !border-[#4d4d4d] bg-[#4d4d4d] text-white",
  purchaseBtnGhost: "justify-self-end !border-[#cfcfcf] bg-white text-[#111]",
  purchaseBtnIcon: "w-[14px] text-center text-[13px] leading-none opacity-[0.92]",
  footer: "mt-0 flex h-[calc(100vh-100px)] min-h-[760px] flex-col max-[760px]:block max-[760px]:h-auto max-[760px]:min-h-0",
  footerContact: "flex min-h-0 flex-[0.95] items-center justify-center bg-[#333] py-10 text-center text-[#eee] max-[760px]:block max-[760px]:h-auto max-[760px]:min-h-0 max-[760px]:py-[64px] max-[760px]:pb-[52px]",
  footerContactInner: "mx-auto w-[min(840px,calc(100%-2rem))] translate-y-[-34px] max-[760px]:translate-y-0",
  footerPlane: "m-0 text-[34px] font-normal leading-none",
  footerContactLinks: "mt-[38px] inline-flex items-center justify-center gap-3 text-[15px] tracking-[0.2px] text-[#eee] max-[760px]:mt-[30px] max-[760px]:flex-wrap max-[760px]:gap-2 max-[760px]:text-[13px]",
  footerContactLink: "text-[#eee] underline [text-underline-offset:2px]",
  footerContactSmall: "mt-[22px] block text-[13px] tracking-[0.06em] text-[#a8abad] max-[760px]:text-[12px] max-[760px]:tracking-[0.04em]",
  footerAbout: "flex min-h-0 flex-[1.05] items-center justify-center bg-[#29292a] py-12 text-[#c5c5c5] max-[760px]:block max-[760px]:h-auto max-[760px]:min-h-0 max-[760px]:py-[52px] max-[760px]:pb-10",
  footerAboutInner: "mx-auto flex w-[min(980px,calc(100%-2rem))] translate-y-[-26px] items-start justify-between gap-8 max-[760px]:flex-col max-[760px]:items-center max-[760px]:gap-6 max-[760px]:translate-y-0",
  footerColumns: "flex flex-1 flex-wrap gap-2 max-[760px]:grid max-[760px]:w-full max-[760px]:max-w-[420px] max-[760px]:grid-cols-[repeat(2,minmax(130px,1fr))] max-[760px]:gap-x-[14px] max-[760px]:gap-y-[18px] max-[760px]:justify-center",
  footerColumn: "min-w-[150px] max-w-[170px] pl-6 text-left max-[760px]:min-w-0 max-[760px]:max-w-none max-[760px]:pl-0",
  footerColumnTitle: "mb-3 mt-0 text-[16px] font-bold text-[#e1e1e1] [font-family:Arial,Helvetica,sans-serif] max-[760px]:mb-2 max-[760px]:text-[14px]",
  footerColumnLink: "block text-[13.66px] leading-[1.66667] no-underline [font-family:Arial,Helvetica,sans-serif] hover:underline max-[760px]:text-[13px] max-[760px]:leading-[1.55]",
  footerBrand: "w-[120px] text-center max-[760px]:w-auto",
  footerBrandImg: "mx-auto mb-[10px] block h-16 w-16 rounded-[14px]",
  footerBrandText: "block text-[12px] tracking-[0.08em] text-[#b8bbbe]",
} as const

export function HomePage() {
  const contentResizerRef = useRef<HTMLDivElement | null>(null)
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
      const resizer = contentResizerRef.current
      if (!resizer) return
      const blockHeight = resizer.offsetHeight
      const top = viewportHeight / 2 - blockHeight / 2
      resizer.style.top = `${top}px`
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

  const navActiveClass = (hash: string) =>
    cx(homeTw.navLink, activeHash === hash && homeTw.navLinkActive)
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
    <div className={homeTw.page}>
      <header className={homeTw.nav}>
        <Link to="/" className={homeTw.logo}>
          <img className={homeTw.logoMark} src={onebookLogoMark} alt="" aria-hidden="true" />
          {' '}
          <span className={homeTw.logoTitle}>OneBook AI</span>
        </Link>
        <nav aria-label="main" className={homeTw.navList}>
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
        <section className={homeTw.intro} id="home">
          <div className={homeTw.introCenter}>
            <h1 className={homeTw.introTitle}>onebook ai</h1>
            <p className={cx(homeTw.slogan, homeTw.introSlogan)}>
              {/* Match Typora-style animated slogan while keeping React-native state control. */}
              {'/* '}
              <span>{typedSlogan.text}</span>
              {typedSlogan.showCursor ? (
                <span className={homeTw.typedCursor} aria-hidden="true">
                  |
                </span>
              ) : null}
              {' */'}
            </p>
          </div>
          <a className={homeTw.scrollHint} href="#readable" aria-label="scroll to product section">
            <i className={cx('fa fa-angle-down', homeTw.scrollIcon)} aria-hidden="true" />
          </a>
        </section>

        <section id="readable" className={cx(homeTw.section, homeTw.readable)}>
          <div className={homeTw.readableCopy}>
            <div className={homeTw.readableCopyInner}>
              <h2 className={homeTw.readableTitle}>Queryable & Verifiable</h2>
              <p className={homeTw.readableBody}>
                OneBook AI turns your personal library into a source-grounded Q&A workspace.
                Upload PDF, EPUB, or TXT, ask questions against your own books, and get answers
                with traceable citations. Keep follow-up questions in one continuous session
                without losing context.
              </p>
              <div className={homeTw.readablePoints}>
                {readableBullets.map((item) => (
                  <h4 className={homeTw.readablePoint} key={item.label}>
                    <i className={cx(`fa ${item.icon}`, homeTw.readablePointIcon)} aria-hidden="true" />
                    <span>{item.label}</span>
                  </h4>
                ))}
              </div>
            </div>
          </div>
          <div className={homeTw.readableMedia}>
            <div className={homeTw.featureWrap}>
              <img
                className={homeTw.featureVideo}
                src={onebookFeaturePreview}
                alt="OneBook AI feature preview with source-cited answers and continuous context"
                loading="lazy"
                decoding="async"
              />
            </div>
          </div>
        </section>

        <hr className={homeTw.divider} />

        <section className={cx(homeTw.section, homeTw.simple)} id="function">
          <h3 className={homeTw.simpleTitle}>Simple, yet Verifiable</h3>
          <div className={homeTw.functionMenu}>
            <ul className={homeTw.functionDots}>
              {functionSlides.map((slide, index) => (
                <li className={homeTw.functionDot} key={slide.word}>
                  <a
                    href="#function"
                    className={cx(homeTw.functionDotLink, index === activeFunctionDot && homeTw.functionDotActive)}
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
          <div className={homeTw.functionSlider} aria-label="OneBook feature flow">
            <ul
              className={homeTw.functionSlides}
              style={{
                width: `${functionSlides.length * 100}%`,
                transform: `translateX(-${activeFunctionDot * (100 / functionSlides.length)}%)`,
                transitionDuration: `${functionSlideSpeedMs}ms`,
              }}
            >
              {functionSlides.map((slide, index) => (
                <li
                  className={homeTw.functionSlide}
                  key={slide.word}
                  style={{ width: `${100 / functionSlides.length}%` }}
                  aria-hidden={index !== activeFunctionDot}
                >
                  <div className={homeTw.sliderImgWrap}>
                    <div className={homeTw.functionMain}>
                      <img
                        src={slide.image}
                        className={homeTw.imgRounded}
                        alt={slide.alt}
                        loading="lazy"
                        decoding="async"
                      />
                    </div>
                  </div>
                  <div className={homeTw.slogan}>
                    {/* Keep Typora-like color split: gray comment wrapper + dark-red inner text. */}
                    {'/* '}
                    <a className={homeTw.sloganLink}>{slide.slogan}</a>
                    {' */'}
                  </div>
                </li>
              ))}
            </ul>
          </div>
        </section>

        <hr className={homeTw.divider} />

        <section id="accessibility" className={homeTw.accessibility}>
          <h3 className={homeTw.accessibilityTitle}>Workflow</h3>
          <p className={cx(homeTw.slogan, homeTw.accessLead)}>
            {'/* '}
            <a style={{ cursor: 'text' }}>You focus on the content, OneBook AI helps with the rest</a>
            {' */'}
          </p>

          <div className={homeTw.accessWrap}>
            <div className={homeTw.accessWrapInner}>
              {accessibilityFeatures.map((feature) => (
                <article className={homeTw.featureBox} key={feature.title}>
                  <div className={homeTw.featureBoxImg}>
                    <img className={homeTw.featureBoxImgEl} src={feature.image} alt={feature.alt} loading="lazy" decoding="async" />
                  </div>
                  <div className={homeTw.featureBoxTitle}>{feature.title}</div>
                  <div className={homeTw.featureBoxDesc}>{feature.detail}</div>
                </article>
              ))}
            </div>
          </div>

          <div className={cx(homeTw.slogan, homeTw.accessMore)}>/* <a className={homeTw.sloganLink}>More in OneBook Docs</a> */</div>
        </section>

        <hr className={homeTw.divider} />

        <section className={homeTw.themes} id="modes">
          <h2 className={homeTw.themesTitle}>Modes</h2>
          <p className={cx(homeTw.slogan, homeTw.themesLead)}>
            {'/* '}
            <a className={homeTw.sloganLink} style={{ cursor: 'text' }}>One library, three views: Read, Ask, Cite.</a>
            {' */'}
          </p>
          <div className={homeTw.themeStack} aria-label="theme stack preview">
            {modePreviews.map((mode, index) => (
              <article
                className={cx(
                  homeTw.modeCard,
                  index === 0 && homeTw.modeCardFirst,
                  index === modePreviews.length - 1 && homeTw.modeCardLast,
                )}
                key={mode.key}
              >
                <img
                  className={homeTw.modePreview}
                  src={mode.image}
                  alt={mode.alt}
                  loading="lazy"
                  decoding="async"
                />
              </article>
            ))}
          </div>
        </section>

        <section className={homeTw.voices} id="quotes" style={{ backgroundImage: `url(${userBg})` }}>
          <div className={homeTw.likeCardSlide}>
            <ul
              className={homeTw.likeTrack}
              style={{
                width: `${likeSlides.length * 100}%`,
                transform: `translateX(-${activeVoiceIndex * (100 / likeSlides.length)}%)`,
              }}
              aria-live="polite"
            >
              {likeSlides.map((slide, index) => (
                <li
                  className={cx(
                    homeTw.likeDot,
                    index === activeVoiceIndex ? homeTw.likeDotActive : homeTw.likeDotMobileHidden,
                  )}
                  key={slide.key}
                  style={{ width: `${100 / likeSlides.length}%` }}
                  aria-hidden={index !== activeVoiceIndex}
                >
                  <div className={homeTw.likeCard}>
                    <div className={homeTw.likeCardInner}>
                      {slide.type === 'title' ? (
                        <div className={homeTw.likeSentence}>
                          <h5 className={homeTw.likeSentenceTitle}>and, loved by our users</h5>
                        </div>
                      ) : null}
                      {slide.type === 'quote' ? (
                        <>
                          <div aria-hidden="true">
                            <img className={homeTw.likeQuoteImg} src={twoQuotesMark} alt="" />
                          </div>
                          <div className={homeTw.likeSentence}>{slide.quote}</div>
                          <div className={homeTw.likeUser}>
                            <span className={homeTw.likeUserLine} aria-hidden="true" />
                            {slide.author}
                          </div>
                        </>
                      ) : null}
                      {slide.type === 'more' ? (
                        <div className={homeTw.likeSentence}>
                          <a href="#quotes">
                            <span>Here</span> are more words from our users.
                          </a>
                        </div>
                      ) : null}
                    </div>
                    <button
                      type="button"
                      className={cx(homeTw.likeNext, slide.type === 'more' && homeTw.likeNextLeft)}
                      onClick={handleNextVoice}
                      aria-label="next quote"
                    >
                      <img className={homeTw.likeNextIcon} src={nextArrowIcon} alt="" aria-hidden="true" />
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section className={homeTw.purchase} id="purchase">
          <div className={homeTw.contentResizer} ref={contentResizerRef}>
            <div className={homeTw.purchaseCenter}>
              <div className={homeTw.purchaseTitleWrap}>
                <h2 className={homeTw.purchaseTitle}>want OneBook AI ?</h2>
              </div>

              <div className={homeTw.subGroup}>
                <div className={homeTw.purchaseSurface}>
                  <div className={homeTw.purchaseProductRow}>
                    <div className={homeTw.purchaseAppIconWrap}>
                      <img
                        className={homeTw.purchaseAppIcon}
                        src={onebookLogoMark}
                        alt="OneBook AI app icon"
                        loading="eager"
                        decoding="async"
                      />
                    </div>
                    <div className={homeTw.purchaseProductCopy}>
                      <div>
                        <span className={homeTw.purchaseMeta}>
                          login · cited answers · session memory
                        </span>
                      </div>
                      <div>
                        <p className={homeTw.purchaseDesc}>
                          Ask across your own books with source-grounded responses.
                        </p>
                      </div>
                      <div className={homeTw.purchasePriceRow}>
                        <span className={homeTw.purchasePriceChip}>Free</span>
                        <span className={homeTw.purchasePriceNote}>no subscription · no payment</span>
                      </div>
                    </div>
                  </div>

                  <div className={homeTw.purchaseBtnGroup}>
                    <Link to="/log-in" className={cx(homeTw.purchaseBtn, homeTw.purchaseBtnDark)}>
                      <i className={cx('fa fa-key', homeTw.purchaseBtnIcon)} aria-hidden="true" />
                      <span>Login</span>
                    </Link>
                    <Link to="/chat" className={cx(homeTw.purchaseBtn, homeTw.purchaseBtnGhost)}>
                      <i className={cx('fa fa-arrow-right', homeTw.purchaseBtnIcon)} aria-hidden="true" />
                      <span>Try</span>
                    </Link>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer className={homeTw.footer} id="contact">
        <section className={homeTw.footerContact}>
          <div className={homeTw.footerContactInner}>
            <h3 className={homeTw.footerPlane}>
              <i className="fa fa-paper-plane-o" aria-hidden="true" />
            </h3>
            <p className={homeTw.footerContactLinks}>
              {contactLinks.map((item, index) => (
                <span key={item.label}>
                  {index > 0 ? <span aria-hidden="true"> · </span> : null}
                  <a
                    href={item.href}
                    target={item.href.startsWith('http') ? '_blank' : undefined}
                    rel={item.href.startsWith('http') ? 'noreferrer' : undefined}
                    className={homeTw.footerContactLink}
                  >
                    {item.label}
                  </a>
                </span>
              ))}
            </p>
            <small className={homeTw.footerContactSmall}>real reading · real asking · real answers</small>
          </div>
        </section>

        <section className={homeTw.footerAbout}>
          <div className={homeTw.footerAboutInner}>
            <div className={homeTw.footerColumns}>
              {footerColumns.map((column) => (
                <div className={homeTw.footerColumn} key={column.title}>
                  <h5 className={homeTw.footerColumnTitle}>{column.title}</h5>
                  {column.links.map((item) =>
                    item.href.startsWith('/') ? (
                      <Link key={item.label} to={item.href} className={homeTw.footerColumnLink}>
                        {item.label}
                      </Link>
                    ) : (
                      <a key={item.label} href={item.href} className={homeTw.footerColumnLink}>
                        {item.label}
                      </a>
                    ),
                  )}
                </div>
              ))}
            </div>
            <div className={homeTw.footerBrand} aria-label="OneBook AI brand">
              <img className={homeTw.footerBrandImg} src={onebookLogoMark} alt="" aria-hidden="true" />
              <span className={homeTw.footerBrandText}>OneBook AI</span>
            </div>
          </div>
        </section>
      </footer>
    </div>
  )
}
