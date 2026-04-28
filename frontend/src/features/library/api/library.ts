import { http } from '@/shared/lib/http/client'
import { createIdempotencyKey } from '@/shared/lib/http/idempotency'
import type { BookFormat, BookLanguage, BookPrimaryCategory, BookStatus } from '@/features/library/books'

export type LibraryBook = {
  id: string
  ownerId: string
  title: string
  originalFilename: string
  primaryCategory: BookPrimaryCategory
  tags: string[]
  format: BookFormat | ''
  language: BookLanguage
  documentType?: string
  documentSummary?: string
  firstPageText?: string
  keywords?: string[]
  documentEntities?: DocumentEntity[]
  documentFacts?: DocumentFact[]
  status: BookStatus
  errorMessage?: string
  sizeBytes: number
  createdAt: string
  updatedAt: string
}

export type DocumentEntity = {
  type: string
  value: string
  label?: string
  page?: string
}

export type DocumentFact = {
  key: string
  value: string
  label?: string
  page?: string
  sourceRef?: string
}

export type ListBooksResponse = {
  items: LibraryBook[]
  count: number
}

export type DownloadBookResponse = {
  url: string
  filename: string
}

export type DeleteBookResponse = {
  status: string
}

export const libraryQueryKeys = {
  books: (params: ListBooksParams) => ['library', 'books', params] as const,
}

export type ListBooksParams = {
  query?: string
  status?: BookStatus | ''
  primaryCategory?: BookPrimaryCategory | ''
  tag?: string
  format?: BookFormat | ''
  language?: BookLanguage | ''
}

export type UploadBookPayload = {
  file: File
  primaryCategory: BookPrimaryCategory
  tags: string[]
}

export type UpdateBookPayload = {
  title: string
  primaryCategory: BookPrimaryCategory
  tags: string[]
}

function toQuery(params: Record<string, string | undefined>): string {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (!value?.trim()) continue
    query.set(key, value)
  }
  const encoded = query.toString()
  return encoded ? `?${encoded}` : ''
}

export async function listBooks(params: ListBooksParams = {}): Promise<ListBooksResponse> {
  const { data } = await http.get<ListBooksResponse>(
    `/api/books${toQuery({
      query: params.query,
      status: params.status,
      primaryCategory: params.primaryCategory,
      tag: params.tag,
      format: params.format,
      language: params.language,
    })}`,
  )
  return data
}

export async function uploadBook(payload: UploadBookPayload): Promise<LibraryBook> {
  const formData = new FormData()
  formData.append('file', payload.file)
  formData.append('primaryCategory', payload.primaryCategory)
  for (const tag of payload.tags) {
    formData.append('tags[]', tag)
  }
  const { data } = await http.post<LibraryBook>('/api/books', formData, {
    headers: {
      'Idempotency-Key': createIdempotencyKey(),
    },
  })
  return data
}

export async function deleteBook(id: string): Promise<DeleteBookResponse> {
  const { data } = await http.delete<DeleteBookResponse>(`/api/books/${id}`)
  return data
}

export async function updateBook(id: string, payload: UpdateBookPayload): Promise<LibraryBook> {
  const { data } = await http.patch<LibraryBook>(`/api/books/${id}`, payload)
  return data
}

export async function getBookDownloadURL(id: string): Promise<DownloadBookResponse> {
  const { data } = await http.get<DownloadBookResponse>(`/api/books/${id}/download`)
  return data
}

export function isBookPending(status: BookStatus): boolean {
  return status === 'queued' || status === 'processing'
}
