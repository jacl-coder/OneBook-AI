import { http } from '@/shared/lib/http/client'

export type BookStatus = 'queued' | 'processing' | 'ready' | 'failed'

export type LibraryBook = {
  id: string
  ownerId: string
  title: string
  originalFilename: string
  status: BookStatus
  errorMessage?: string
  sizeBytes: number
  createdAt: string
  updatedAt: string
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
  books: ['library', 'books'] as const,
}

export async function listBooks(): Promise<ListBooksResponse> {
  const { data } = await http.get<ListBooksResponse>('/api/books')
  return data
}

export async function uploadBook(file: File): Promise<LibraryBook> {
  const formData = new FormData()
  formData.append('file', file)
  const { data } = await http.post<LibraryBook>('/api/books', formData)
  return data
}

export async function deleteBook(id: string): Promise<DeleteBookResponse> {
  const { data } = await http.delete<DeleteBookResponse>(`/api/books/${id}`)
  return data
}

export async function getBookDownloadURL(id: string): Promise<DownloadBookResponse> {
  const { data } = await http.get<DownloadBookResponse>(`/api/books/${id}/download`)
  return data
}

export function isBookPending(status: BookStatus): boolean {
  return status === 'queued' || status === 'processing'
}
