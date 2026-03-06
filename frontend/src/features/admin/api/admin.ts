import { http } from '@/shared/lib/http/client'
import { createIdempotencyKey } from '@/shared/lib/http/idempotency'
import type { AuthUser } from '@/features/auth/store/session'
import type { BookFormat, BookLanguage, BookPrimaryCategory } from '@/features/library/books'

export type AdminUser = AuthUser & {
  createdAt: string
  updatedAt: string
}

export type AdminBookStatus = 'queued' | 'processing' | 'ready' | 'failed'

export type AdminBook = {
  id: string
  ownerId: string
  title: string
  originalFilename: string
  primaryCategory: BookPrimaryCategory
  tags: string[]
  format: BookFormat | ''
  language: BookLanguage
  status: AdminBookStatus
  errorMessage?: string
  sizeBytes: number
  createdAt: string
  updatedAt: string
}

export type AdminAuditLog = {
  id: string
  actorId: string
  action: string
  targetType: string
  targetId: string
  before?: Record<string, unknown>
  after?: Record<string, unknown>
  requestId?: string
  ip?: string
  userAgent?: string
  createdAt: string
}

export type BookStatusCount = {
  status: string
  count: number
}

export type AdminOverview = {
  totalUsers: number
  activeUsers: number
  disabledUsers: number
  totalBooks: number
  booksByStatus: BookStatusCount[]
  booksCreated24h: number
  booksFailed24h: number
  refreshedAt: string
  windowStart: string
  windowHours: number
}

export type EvalDatasetSourceType = 'upload' | 'book'
export type EvalDatasetStatus = 'active' | 'archived'
export type EvalRunStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'canceled'
export type EvalRunMode = 'retrieval' | 'post_retrieval' | 'answer' | 'all'
export type EvalRetrievalMode = 'hybrid' | 'dense_only' | 'sparse_only'
export type EvalGateStatus = 'passed' | 'warn' | 'failed'

export type AdminEvalDataset = {
  id: string
  name: string
  sourceType: EvalDatasetSourceType
  bookId?: string
  version: number
  status: EvalDatasetStatus
  description?: string
  files: Record<string, string>
  createdBy: string
  createdAt: string
  updatedAt: string
}

export type AdminEvalRunArtifact = {
  name: string
  path?: string
  contentType?: string
  sizeBytes: number
  createdAt: string
}

export type AdminEvalRunStageSummary = {
  stage: string
  metrics: Record<string, unknown>
}

export type AdminEvalRun = {
  id: string
  datasetId: string
  status: EvalRunStatus
  mode: EvalRunMode
  retrievalMode: EvalRetrievalMode
  params?: Record<string, unknown>
  gateMode: string
  gateStatus: EvalGateStatus
  summaryMetrics?: Record<string, unknown>
  warnings?: string[]
  artifacts?: AdminEvalRunArtifact[]
  stageSummaries?: AdminEvalRunStageSummary[]
  progress: number
  errorMessage?: string
  startedAt?: string
  finishedAt?: string
  createdBy: string
  createdAt: string
  updatedAt: string
}

export type AdminEvalOverview = {
  totalDatasets: number
  activeDatasets: number
  totalRuns: number
  queuedRuns: number
  runningRuns: number
  successfulRuns: number
  failedRuns: number
  canceledRuns: number
  recentRuns: number
  recentGateFailed: number
  successRate: number
  refreshedAt: string
}

export type PagedResponse<T> = {
  items: T[]
  count: number
  page: number
  pageSize: number
  total: number
  totalPages: number
}

export type ListAdminUsersParams = {
  query?: string
  role?: 'user' | 'admin' | ''
  status?: 'active' | 'disabled' | ''
  page?: number
  pageSize?: number
  sortBy?: 'createdAt' | 'updatedAt' | 'email' | ''
  sortOrder?: 'asc' | 'desc' | ''
}

export type ListAdminBooksParams = {
  query?: string
  ownerId?: string
  status?: AdminBookStatus | ''
  primaryCategory?: BookPrimaryCategory | ''
  tag?: string
  format?: BookFormat | ''
  language?: BookLanguage | ''
  page?: number
  pageSize?: number
  sortBy?: 'updatedAt' | 'createdAt' | 'title' | ''
  sortOrder?: 'asc' | 'desc' | ''
}

export type ListAdminAuditLogsParams = {
  actorId?: string
  action?: string
  targetType?: string
  targetId?: string
  from?: string
  to?: string
  page?: number
  pageSize?: number
}

export type AdminUserUpdatePayload = {
  role?: 'user' | 'admin'
  status?: 'active' | 'disabled'
}

export type ListAdminEvalDatasetsParams = {
  query?: string
  sourceType?: EvalDatasetSourceType | ''
  status?: EvalDatasetStatus | ''
  bookId?: string
  page?: number
  pageSize?: number
}

export type ListAdminEvalRunsParams = {
  datasetId?: string
  status?: EvalRunStatus | ''
  mode?: EvalRunMode | ''
  retrievalMode?: EvalRetrievalMode | ''
  page?: number
  pageSize?: number
}

export type CreateAdminEvalDatasetPayload = {
  name: string
  sourceType: EvalDatasetSourceType
  bookId?: string
  version?: number
  description?: string
  chunks?: File | null
  queries?: File | null
  qrels?: File | null
  predictions?: File | null
  metadata?: File | null
}

export type UpdateAdminEvalDatasetPayload = {
  name?: string
  description?: string
  status?: EvalDatasetStatus
}

export type CreateAdminEvalRunPayload = {
  datasetId: string
  mode: EvalRunMode
  retrievalMode: EvalRetrievalMode
  gateMode: 'warn' | 'strict' | 'off'
  params?: Record<string, unknown>
}

function toQuery(params: Record<string, unknown>): string {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null) continue
    if (typeof value === 'string' && value.trim() === '') continue
    query.set(key, String(value))
  }
  const encoded = query.toString()
  return encoded ? `?${encoded}` : ''
}

export async function listAdminUsers(params: ListAdminUsersParams): Promise<PagedResponse<AdminUser>> {
  const { data } = await http.get<PagedResponse<AdminUser>>(`/api/admin/users${toQuery(params)}`)
  return data
}

export async function getAdminUser(id: string): Promise<AdminUser> {
  const { data } = await http.get<AdminUser>(`/api/admin/users/${id}`)
  return data
}

export async function updateAdminUser(id: string, payload: AdminUserUpdatePayload): Promise<AdminUser> {
  const { data } = await http.patch<AdminUser>(`/api/admin/users/${id}`, payload)
  return data
}

export async function disableAdminUser(id: string): Promise<AdminUser> {
  const { data } = await http.post<AdminUser>(`/api/admin/users/${id}/disable`, {})
  return data
}

export async function enableAdminUser(id: string): Promise<AdminUser> {
  const { data } = await http.post<AdminUser>(`/api/admin/users/${id}/enable`, {})
  return data
}

export async function listAdminBooks(params: ListAdminBooksParams): Promise<PagedResponse<AdminBook>> {
  const { data } = await http.get<PagedResponse<AdminBook>>(`/api/admin/books${toQuery(params)}`)
  return data
}

export async function deleteAdminBook(id: string): Promise<{ status: string }> {
  const { data } = await http.delete<{ status: string }>(`/api/admin/books/${id}`)
  return data
}

export async function reprocessAdminBook(id: string): Promise<AdminBook> {
  const { data } = await http.post<AdminBook>(`/api/admin/books/${id}/reprocess`, {}, {
    headers: {
      'Idempotency-Key': createIdempotencyKey(),
    },
  })
  return data
}

export async function getAdminOverview(): Promise<AdminOverview> {
  const { data } = await http.get<AdminOverview>('/api/admin/overview')
  return data
}

export async function listAdminAuditLogs(
  params: ListAdminAuditLogsParams,
): Promise<PagedResponse<AdminAuditLog>> {
  const { data } = await http.get<PagedResponse<AdminAuditLog>>(`/api/admin/audit-logs${toQuery(params)}`)
  return data
}

export async function getAdminEvalOverview(): Promise<AdminEvalOverview> {
  const { data } = await http.get<AdminEvalOverview>('/api/admin/evals/overview')
  return data
}

export async function listAdminEvalDatasets(
  params: ListAdminEvalDatasetsParams,
): Promise<PagedResponse<AdminEvalDataset>> {
  const { data } = await http.get<PagedResponse<AdminEvalDataset>>(`/api/admin/evals/datasets${toQuery(params)}`)
  return data
}

export async function createAdminEvalDataset(payload: CreateAdminEvalDatasetPayload): Promise<AdminEvalDataset> {
  const form = new FormData()
  form.set('name', payload.name)
  form.set('sourceType', payload.sourceType)
  if (payload.bookId?.trim()) form.set('bookId', payload.bookId.trim())
  if (payload.description?.trim()) form.set('description', payload.description.trim())
  if (payload.version) form.set('version', String(payload.version))
  if (payload.chunks) form.set('chunks', payload.chunks)
  if (payload.queries) form.set('queries', payload.queries)
  if (payload.qrels) form.set('qrels', payload.qrels)
  if (payload.predictions) form.set('predictions', payload.predictions)
  if (payload.metadata) form.set('metadata', payload.metadata)
  const { data } = await http.post<AdminEvalDataset>('/api/admin/evals/datasets', form)
  return data
}

export async function getAdminEvalDataset(id: string): Promise<AdminEvalDataset> {
  const { data } = await http.get<AdminEvalDataset>(`/api/admin/evals/datasets/${id}`)
  return data
}

export async function updateAdminEvalDataset(
  id: string,
  payload: UpdateAdminEvalDatasetPayload,
): Promise<AdminEvalDataset> {
  const { data } = await http.patch<AdminEvalDataset>(`/api/admin/evals/datasets/${id}`, payload)
  return data
}

export async function deleteAdminEvalDataset(id: string): Promise<{ status: string }> {
  const { data } = await http.delete<{ status: string }>(`/api/admin/evals/datasets/${id}`)
  return data
}

export async function listAdminEvalRuns(params: ListAdminEvalRunsParams): Promise<PagedResponse<AdminEvalRun>> {
  const { data } = await http.get<PagedResponse<AdminEvalRun>>(`/api/admin/evals/runs${toQuery(params)}`)
  return data
}

export async function createAdminEvalRun(payload: CreateAdminEvalRunPayload): Promise<AdminEvalRun> {
  const { data } = await http.post<AdminEvalRun>('/api/admin/evals/runs', payload, {
    headers: {
      'Idempotency-Key': createIdempotencyKey(),
    },
  })
  return data
}

export async function getAdminEvalRun(id: string): Promise<AdminEvalRun> {
  const { data } = await http.get<AdminEvalRun>(`/api/admin/evals/runs/${id}`)
  return data
}

export async function cancelAdminEvalRun(id: string): Promise<AdminEvalRun> {
  const { data } = await http.post<AdminEvalRun>(`/api/admin/evals/runs/${id}/cancel`, {})
  return data
}

export async function getAdminEvalPerQuery(id: string): Promise<{ items: Record<string, unknown>[]; count: number }> {
  const { data } = await http.get<{ items: Record<string, unknown>[]; count: number }>(
    `/api/admin/evals/runs/${id}/per-query`,
  )
  return data
}

export const adminQueryKeys = {
  users: (params: ListAdminUsersParams) => ['admin', 'users', params] as const,
  books: (params: ListAdminBooksParams) => ['admin', 'books', params] as const,
  overview: ['admin', 'overview'] as const,
  auditLogs: (params: ListAdminAuditLogsParams) => ['admin', 'audit', params] as const,
  evalOverview: ['admin', 'evals', 'overview'] as const,
  evalDatasets: (params: ListAdminEvalDatasetsParams) => ['admin', 'evals', 'datasets', params] as const,
  evalRuns: (params: ListAdminEvalRunsParams) => ['admin', 'evals', 'runs', params] as const,
  evalRun: (id: string) => ['admin', 'evals', 'run', id] as const,
  evalPerQuery: (id: string) => ['admin', 'evals', 'per-query', id] as const,
}
