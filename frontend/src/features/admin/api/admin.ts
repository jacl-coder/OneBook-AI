import { http } from '@/shared/lib/http/client'
import type { AuthUser } from '@/features/auth/store/session'

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
  const { data } = await http.post<AdminBook>(`/api/admin/books/${id}/reprocess`, {})
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

export const adminQueryKeys = {
  users: (params: ListAdminUsersParams) => ['admin', 'users', params] as const,
  books: (params: ListAdminBooksParams) => ['admin', 'books', params] as const,
  overview: ['admin', 'overview'] as const,
  auditLogs: (params: ListAdminAuditLogsParams) => ['admin', 'audit', params] as const,
}
