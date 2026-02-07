import { useEffect, useState } from 'react'

type UseTypewriterOptions = {
  strings: readonly string[]
  typeSpeed?: number
  deleteSpeed?: number
  holdDelay?: number
  loop?: boolean
}

type UseTypewriterResult = {
  text: string
  showCursor: boolean
}

const DEFAULT_TYPE_SPEED = 50
const DEFAULT_DELETE_SPEED = 30
const DEFAULT_HOLD_DELAY = 1000

export function useTypewriter(options: UseTypewriterOptions): UseTypewriterResult {
  const { strings, typeSpeed = DEFAULT_TYPE_SPEED, deleteSpeed = DEFAULT_DELETE_SPEED, holdDelay = DEFAULT_HOLD_DELAY, loop = true } = options

  const [text, setText] = useState('')
  const [stringIndex, setStringIndex] = useState(0)
  const [isDeleting, setIsDeleting] = useState(false)
  const [reduceMotion, setReduceMotion] = useState(false)

  useEffect(() => {
    if (typeof window === 'undefined' || !window.matchMedia) {
      return
    }

    const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
    const sync = () => setReduceMotion(mediaQuery.matches)

    sync()
    mediaQuery.addEventListener('change', sync)
    return () => mediaQuery.removeEventListener('change', sync)
  }, [])

  useEffect(() => {
    if (strings.length === 0) {
      setText('')
      return
    }

    if (reduceMotion) {
      setText(strings[0])
      return
    }

    const current = strings[stringIndex] ?? ''
    let timeoutId: ReturnType<typeof setTimeout> | undefined

    if (!isDeleting && text === current) {
      timeoutId = setTimeout(() => {
        if (!loop && stringIndex === strings.length - 1) {
          return
        }
        setIsDeleting(true)
      }, holdDelay)
      return () => {
        if (timeoutId) {
          clearTimeout(timeoutId)
        }
      }
    }

    if (isDeleting && text.length === 0) {
      setIsDeleting(false)
      setStringIndex((prev) => {
        if (loop) {
          return (prev + 1) % strings.length
        }
        return Math.min(prev + 1, strings.length - 1)
      })
      return
    }

    timeoutId = setTimeout(
      () => {
        setText((prev) =>
          isDeleting ? current.slice(0, Math.max(0, prev.length - 1)) : current.slice(0, prev.length + 1),
        )
      },
      isDeleting ? deleteSpeed : typeSpeed,
    )

    return () => clearTimeout(timeoutId)
  }, [deleteSpeed, holdDelay, isDeleting, loop, reduceMotion, stringIndex, strings, text, typeSpeed])

  return {
    text,
    showCursor: !reduceMotion,
  }
}
