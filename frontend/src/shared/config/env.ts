const defaultApiBaseUrl = 'http://localhost:8080'
const defaultRequestTimeoutMs = 15_000

function parseNumber(value: string | undefined, fallback: number): number {
  if (!value) {
    return fallback
  }
  const parsed = Number.parseInt(value, 10)
  if (Number.isNaN(parsed)) {
    return fallback
  }
  return parsed
}

export const env = {
  apiBaseUrl: import.meta.env.VITE_API_BASE_URL ?? defaultApiBaseUrl,
  requestTimeoutMs: parseNumber(
    import.meta.env.VITE_API_TIMEOUT_MS,
    defaultRequestTimeoutMs,
  ),
}

