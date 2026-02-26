import ReactMarkdown, { type Components } from 'react-markdown'
import rehypeHighlight from 'rehype-highlight'
import rehypeSanitize from 'rehype-sanitize'
import remarkGfm from 'remark-gfm'

type MessageMarkdownProps = {
  content: string
  className?: string
}

const cx = (...values: Array<string | false | null | undefined>) =>
  values.filter(Boolean).join(' ')

function sanitizeHref(rawHref?: string): string | null {
  if (!rawHref) return null
  const href = rawHref.trim()
  if (!href) return null

  if (href.startsWith('#') || href.startsWith('/')) {
    return href
  }

  try {
    const url = new URL(href)
    const protocol = url.protocol.toLowerCase()
    if (protocol === 'http:' || protocol === 'https:' || protocol === 'mailto:') {
      return href
    }
  } catch {
    return null
  }

  return null
}

const markdownComponents: Components = {
  a({ href, children }) {
    const safeHref = sanitizeHref(href)
    if (!safeHref) {
      return <span className="chat-md__link-disabled">{children}</span>
    }

    const isExternal = /^https?:\/\//i.test(safeHref)

    return (
      <a
        href={safeHref}
        className="chat-md__link"
        target={isExternal ? '_blank' : undefined}
        rel={isExternal ? 'noopener noreferrer' : undefined}
      >
        {children}
      </a>
    )
  },
  code({ className, children }) {
    return <code className={cx('chat-md__code', className)}>{children}</code>
  },
  pre({ children }) {
    return <pre className="chat-md__pre">{children}</pre>
  },
  table({ children }) {
    return (
      <div className="chat-md__table-wrap">
        <table className="chat-md__table">{children}</table>
      </div>
    )
  },
}

export function MessageMarkdown({ content, className }: MessageMarkdownProps) {
  return (
    <div className={cx('chat-md', className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeSanitize, [rehypeHighlight, { ignoreMissing: true }]]}
        skipHtml
        components={markdownComponents}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}
