import axios from 'axios'

import { env } from '@/shared/config/env'

export const http = axios.create({
  baseURL: env.apiBaseUrl,
  timeout: env.requestTimeoutMs,
  headers: {
    'Content-Type': 'application/json',
  },
})

