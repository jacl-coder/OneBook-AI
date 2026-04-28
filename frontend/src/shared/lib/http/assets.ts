import { env } from '@/shared/config/env'

const absoluteURLPattern = /^[a-z][a-z\d+\-.]*:/i

export function resolveApiAssetURL(value: string | null | undefined): string {
  const url = value?.trim()
  if (!url) return ''
  if (absoluteURLPattern.test(url)) return url
  return new URL(url, env.apiBaseUrl).toString()
}
