import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { getApiErrorMessage } from '@/features/auth/api/auth'
import {
  adminQueryKeys,
  deleteAdminBook,
  getAdminOverview,
  listAdminBooks,
  reprocessAdminBook,
  type AdminBookStatus,
  type ListAdminBooksParams,
} from '@/features/admin/api/admin'
import {
  bookFormatOptions,
  bookLanguageOptions,
  bookPrimaryCategoryOptions,
  getBookFormatLabel,
  getBookLanguageLabel,
  getBookPrimaryCategoryLabel,
} from '@/features/library/books'

const cx = (...values: Array<string | false | null | undefined>) => values.filter(Boolean).join(' ')

const booksTw = {
  shell: 'grid gap-4',
  stats: 'grid gap-3 lg:grid-cols-5',
  statCard: 'rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white p-4',
  statLabel: 'text-[12px] font-medium uppercase tracking-[0.08em] text-[#6f6f6f]',
  statValue: 'mt-2 text-[28px] leading-none font-semibold text-[#0f172a]',
  statMeta: 'mt-2 text-[12px] text-[#8a8a8a]',
  toolbar: 'rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white p-4',
  toolbarTop: 'flex flex-wrap items-center justify-between gap-3',
  sectionTitle: 'text-[16px] font-semibold text-[#0f172a]',
  sectionMeta: 'text-[12px] text-[#6f6f6f]',
  filters: 'mt-4 grid gap-2 xl:grid-cols-[minmax(0,1.4fr)_repeat(5,minmax(0,0.8fr))_minmax(0,0.9fr)_130px]',
  input:
    'h-10 rounded-[10px] border border-[rgba(0,0,0,0.12)] bg-white px-3 text-[13px] text-[#0f172a] outline-none transition-colors placeholder:text-[#9a9a9a] focus:border-[rgba(0,0,0,0.28)]',
  select:
    'h-10 rounded-[10px] border border-[rgba(0,0,0,0.12)] bg-white px-3 text-[13px] text-[#0f172a] outline-none transition-colors focus:border-[rgba(0,0,0,0.28)]',
  button:
    'inline-flex h-10 items-center justify-center rounded-[10px] border border-[rgba(0,0,0,0.12)] bg-white px-3 text-[13px] font-medium text-[#2f2f2f] transition-colors hover:bg-[#f4f4f4]',
  quickFilters: 'mt-4 flex flex-wrap gap-2',
  quickChip:
    'inline-flex h-9 items-center rounded-[9999px] border border-[rgba(0,0,0,0.1)] bg-[#fafafa] px-3 text-[12px] font-medium text-[#3f3f3f] transition-colors hover:bg-[#f1f1f1]',
  quickChipActive: 'border-[rgba(0,0,0,0.16)] bg-[#ececec] text-[#0d0d0d]',
  notice: 'rounded-[12px] border border-[#f4b0b4] bg-[#fff5f6] px-3 py-[10px] text-[13px] text-[#9f1820]',
  content: 'grid gap-4 xl:grid-cols-[minmax(0,1.7fr)_360px]',
  listPanel: 'rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white p-4 xl:sticky xl:top-4',
  detailPanel: 'rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white p-4 xl:sticky xl:top-4',
  metaPill:
    'inline-flex items-center rounded-[9999px] bg-[#f1f1f1] px-2.5 py-1 text-[11px] font-medium text-[#4f4f4f]',
  tagPill:
    'inline-flex items-center rounded-[9999px] border border-[rgba(0,0,0,0.08)] bg-white px-2.5 py-1 text-[11px] text-[#5f5f5f]',
  statusChip: 'inline-flex items-center rounded-[9999px] px-2.5 py-1 text-[11px] font-semibold',
  statusQueued: 'bg-[#fff7ed] text-[#c2410c]',
  statusProcessing: 'bg-[#f1f1f1] text-[#525252]',
  statusReady: 'bg-[#ecfdf5] text-[#047857]',
  statusFailed: 'bg-[#fef2f2] text-[#b91c1c]',
  bookActions: 'flex flex-wrap items-center gap-2',
  actionBtn:
    'inline-flex h-8 items-center rounded-[9999px] border border-[rgba(0,0,0,0.12)] bg-white px-[10px] text-[12px] font-medium text-[#3f3f3f] transition-colors hover:bg-[#f4f4f4] disabled:opacity-50',
  actionDanger: 'border-[rgba(185,28,28,0.18)] text-[#b91c1c] hover:bg-[#fff5f5]',
  tableWrap: 'overflow-auto rounded-[12px] border border-[rgba(0,0,0,0.08)]',
  table: 'min-w-[1120px] w-full border-collapse text-left',
  th: 'border-b border-[rgba(0,0,0,0.08)] bg-[#fafafa] px-4 py-3 text-[12px] font-semibold text-[#666]',
  td: 'border-b border-[rgba(0,0,0,0.06)] px-4 py-3 align-top text-[13px] text-[#3f3f3f]',
  rowSelected: 'bg-[#f5f5f5]',
  empty: 'grid min-h-[260px] place-items-center rounded-[12px] border border-dashed border-[rgba(0,0,0,0.12)] bg-[#fcfcfc] p-6 text-center',
  emptyTitle: 'text-[18px] font-semibold text-[#0f172a]',
  emptyDesc: 'mt-2 text-[13px] text-[#6f6f6f]',
  pager: 'mt-4 flex flex-wrap items-center justify-between gap-3 text-[12px] text-[#6f6f6f]',
  detailHeader: 'border-b border-[rgba(0,0,0,0.08)] pb-4',
  detailTitle: 'text-[18px] font-semibold text-[#0f172a]',
  detailSub: 'mt-1 text-[12px] text-[#6f6f6f]',
  detailBody: 'grid gap-4 pt-4',
  detailSection: 'rounded-[12px] border border-[rgba(0,0,0,0.08)] bg-[#fafafa] p-3',
  detailLabel: 'text-[11px] font-medium uppercase tracking-[0.08em] text-[#6f6f6f]',
  detailValue: 'mt-1 text-[14px] text-[#0f172a]',
  detailGrid: 'grid gap-3 md:grid-cols-2 xl:grid-cols-1',
  errorBox: 'mt-3 rounded-[12px] border border-[#fecaca] bg-[#fff5f5] p-3 text-[12px] text-[#b91c1c]',
}

function initialParams(): ListAdminBooksParams {
  return {
    query: '',
    ownerId: '',
    status: '',
    primaryCategory: '',
    tag: '',
    format: '',
    language: '',
    page: 1,
    pageSize: 20,
    sortBy: 'updatedAt',
    sortOrder: 'desc',
  }
}

function statusLabel(status: AdminBookStatus): string {
  switch (status) {
    case 'queued':
      return '排队中'
    case 'processing':
      return '处理中'
    case 'ready':
      return '可对话'
    case 'failed':
      return '处理失败'
    default:
      return status
  }
}

function statusClass(status: AdminBookStatus): string {
  switch (status) {
    case 'queued':
      return booksTw.statusQueued
    case 'processing':
      return booksTw.statusProcessing
    case 'ready':
      return booksTw.statusReady
    case 'failed':
      return booksTw.statusFailed
    default:
      return booksTw.statusQueued
  }
}

function relativeTime(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  const diff = Date.now() - date.getTime()
  if (Number.isNaN(diff)) return value
  const minute = 60_000
  const hour = 60 * minute
  const day = 24 * hour
  if (diff < hour) return `${Math.max(1, Math.round(diff / minute))} 分钟前`
  if (diff < day) return `${Math.round(diff / hour)} 小时前`
  return `${Math.round(diff / day)} 天前`
}

function quickFilterActive(filters: ListAdminBooksParams, key: 'failed' | 'processing' | 'ready'): boolean {
  return filters.status === key
}

export function AdminBooksPage() {
  const queryClient = useQueryClient()
  const [filters, setFilters] = useState<ListAdminBooksParams>(initialParams)
  const [selectedBookID, setSelectedBookID] = useState('')
  const [errorText, setErrorText] = useState('')

  const booksQuery = useQuery({
    queryKey: adminQueryKeys.books(filters),
    queryFn: () => listAdminBooks(filters),
    refetchInterval: 10_000,
  })

  const overviewQuery = useQuery({
    queryKey: adminQueryKeys.overview,
    queryFn: getAdminOverview,
    refetchInterval: 30_000,
  })

  const deleteMutation = useMutation({
    mutationFn: deleteAdminBook,
    onSuccess: async () => {
      setErrorText('')
      await queryClient.invalidateQueries({ queryKey: ['admin', 'books'] })
      await queryClient.invalidateQueries({ queryKey: adminQueryKeys.overview })
    },
    onError: (error) => {
      setErrorText(getApiErrorMessage(error, '删除书籍失败，请稍后重试。'))
    },
  })

  const reprocessMutation = useMutation({
    mutationFn: reprocessAdminBook,
    onSuccess: async () => {
      setErrorText('')
      await queryClient.invalidateQueries({ queryKey: ['admin', 'books'] })
      await queryClient.invalidateQueries({ queryKey: adminQueryKeys.overview })
    },
    onError: (error) => {
      setErrorText(getApiErrorMessage(error, '重处理触发失败，请稍后重试。'))
    },
  })

  const items = useMemo(() => booksQuery.data?.items ?? [], [booksQuery.data?.items])
  const total = booksQuery.data?.total ?? 0
  const page = booksQuery.data?.page ?? filters.page ?? 1
  const pageSize = booksQuery.data?.pageSize ?? filters.pageSize ?? 20
  const totalPages = booksQuery.data?.totalPages ?? 1

  const activeSelectedBookID = items.some((item) => item.id === selectedBookID)
    ? selectedBookID
    : (items[0]?.id ?? '')
  const selectedBook = useMemo(
    () => items.find((item) => item.id === activeSelectedBookID) ?? items[0] ?? null,
    [activeSelectedBookID, items],
  )

  const filteredStats = useMemo(() => {
    const failed = items.filter((item) => item.status === 'failed').length
    const processing = items.filter((item) => item.status === 'processing' || item.status === 'queued').length
    const ready = items.filter((item) => item.status === 'ready').length
    const categories = new Set(items.map((item) => item.primaryCategory)).size
    return { failed, processing, ready, categories }
  }, [items])

  const overview = overviewQuery.data

  const handleQuickStatus = (status: AdminBookStatus) => {
    setFilters((prev) => ({
      ...prev,
      status: prev.status === status ? '' : status,
      page: 1,
    }))
  }

  return (
    <div className={booksTw.shell}>
      {errorText ? <div className={booksTw.notice}>{errorText}</div> : null}
      {booksQuery.isError ? (
        <div className={booksTw.notice}>{getApiErrorMessage(booksQuery.error, '书籍列表加载失败。')}</div>
      ) : null}

      <section className={booksTw.stats}>
        <div className={booksTw.statCard}>
          <div className={booksTw.statLabel}>总书籍</div>
          <div className={booksTw.statValue}>{overview?.totalBooks ?? total}</div>
          <div className={booksTw.statMeta}>全部</div>
        </div>
        <div className={booksTw.statCard}>
          <div className={booksTw.statLabel}>可对话</div>
          <div className={booksTw.statValue}>{filteredStats.ready}</div>
          <div className={booksTw.statMeta}>ready</div>
        </div>
        <div className={booksTw.statCard}>
          <div className={booksTw.statLabel}>处理中</div>
          <div className={booksTw.statValue}>{filteredStats.processing}</div>
          <div className={booksTw.statMeta}>queued + processing</div>
        </div>
        <div className={booksTw.statCard}>
          <div className={booksTw.statLabel}>失败数</div>
          <div className={booksTw.statValue}>{filteredStats.failed}</div>
          <div className={booksTw.statMeta}>failed</div>
        </div>
        <div className={booksTw.statCard}>
          <div className={booksTw.statLabel}>分类覆盖</div>
          <div className={booksTw.statValue}>{filteredStats.categories}</div>
          <div className={booksTw.statMeta}>当前结果</div>
        </div>
      </section>

      <section className={booksTw.toolbar}>
        <div className={booksTw.toolbarTop}>
          <h3 className={booksTw.sectionTitle}>筛选</h3>
        </div>

        <div className={booksTw.quickFilters}>
          <button
            type="button"
            className={cx(booksTw.quickChip, quickFilterActive(filters, 'failed') && booksTw.quickChipActive)}
            onClick={() => handleQuickStatus('failed')}
          >
            只看失败
          </button>
          <button
            type="button"
            className={cx(booksTw.quickChip, quickFilterActive(filters, 'processing') && booksTw.quickChipActive)}
            onClick={() => handleQuickStatus('processing')}
          >
            只看处理中
          </button>
          <button
            type="button"
            className={cx(booksTw.quickChip, quickFilterActive(filters, 'ready') && booksTw.quickChipActive)}
            onClick={() => handleQuickStatus('ready')}
          >
            只看可对话
          </button>
        </div>

        <div className={booksTw.filters}>
          <input
            className={booksTw.input}
            placeholder="搜索标题 / 文件名 / 书籍ID"
            value={filters.query ?? ''}
            onChange={(event) => setFilters((prev) => ({ ...prev, query: event.target.value, page: 1 }))}
          />
          <select
            className={booksTw.select}
            value={filters.status ?? ''}
            onChange={(event) =>
              setFilters((prev) => ({ ...prev, status: event.target.value as ListAdminBooksParams['status'], page: 1 }))
            }
          >
            <option value="">全部状态</option>
            <option value="queued">排队中</option>
            <option value="processing">处理中</option>
            <option value="ready">可对话</option>
            <option value="failed">处理失败</option>
          </select>
          <select
            className={booksTw.select}
            value={filters.primaryCategory ?? ''}
            onChange={(event) =>
              setFilters((prev) => ({
                ...prev,
                primaryCategory: event.target.value as ListAdminBooksParams['primaryCategory'],
                page: 1,
              }))
            }
          >
            <option value="">全部分类</option>
            {bookPrimaryCategoryOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <input
            className={booksTw.input}
            placeholder="按标签筛选"
            value={filters.tag ?? ''}
            onChange={(event) => setFilters((prev) => ({ ...prev, tag: event.target.value, page: 1 }))}
          />
          <select
            className={booksTw.select}
            value={filters.format ?? ''}
            onChange={(event) =>
              setFilters((prev) => ({ ...prev, format: event.target.value as ListAdminBooksParams['format'], page: 1 }))
            }
          >
            <option value="">全部格式</option>
            {bookFormatOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <select
            className={booksTw.select}
            value={filters.language ?? ''}
            onChange={(event) =>
              setFilters((prev) => ({
                ...prev,
                language: event.target.value as ListAdminBooksParams['language'],
                page: 1,
              }))
            }
          >
            <option value="">全部语言</option>
            {bookLanguageOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <input
            className={booksTw.input}
            placeholder="ownerId"
            value={filters.ownerId ?? ''}
            onChange={(event) => setFilters((prev) => ({ ...prev, ownerId: event.target.value, page: 1 }))}
          />
          <button type="button" className={booksTw.button} onClick={() => setFilters(initialParams())}>
            重置筛选
          </button>
        </div>
      </section>

      <section className={booksTw.content}>
        <div className={booksTw.listPanel}>
          <div className="mb-4 flex items-center justify-between gap-3">
            <h3 className={booksTw.sectionTitle}>结果</h3>
            <div className={booksTw.sectionMeta}>{items.length} / {total}</div>
          </div>

          {booksQuery.isLoading ? (
            <div className={booksTw.empty}>
              <div>
                <p className={booksTw.emptyTitle}>正在加载书籍数据...</p>
              </div>
            </div>
          ) : items.length === 0 ? (
            <div className={booksTw.empty}>
              <div>
                <p className={booksTw.emptyTitle}>没有匹配的书籍</p>
              </div>
            </div>
          ) : (
            <div className={booksTw.tableWrap}>
              <table className={booksTw.table}>
                <thead>
                  <tr>
                    <th className={booksTw.th}>书籍</th>
                    <th className={booksTw.th}>分类</th>
                    <th className={booksTw.th}>格式/语言</th>
                    <th className={booksTw.th}>Owner</th>
                    <th className={booksTw.th}>状态</th>
                    <th className={booksTw.th}>更新时间</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((item) => (
                    <tr
                      key={item.id}
                      className={cx('cursor-pointer transition-colors hover:bg-[#f8fbff]', selectedBookID === item.id && booksTw.rowSelected)}
                      onClick={() => setSelectedBookID(item.id)}
                    >
                      <td className={booksTw.td}>
                        <div className="max-w-[320px] truncate font-semibold text-[#0f172a]">{item.title}</div>
                        <div className="mt-1 text-[12px] text-[#64748b]">{item.originalFilename}</div>
                      </td>
                      <td className={booksTw.td}>
                        <div>{getBookPrimaryCategoryLabel(item.primaryCategory)}</div>
                        <div className="mt-1 text-[12px] text-[#64748b]">{item.tags.length > 0 ? item.tags.join(' / ') : '无标签'}</div>
                      </td>
                      <td className={booksTw.td}>
                        <div>{getBookFormatLabel(item.format)}</div>
                        <div className="mt-1 text-[12px] text-[#64748b]">{getBookLanguageLabel(item.language)}</div>
                      </td>
                      <td className={booksTw.td}>{item.ownerId}</td>
                      <td className={booksTw.td}>
                        <span className={cx(booksTw.statusChip, statusClass(item.status))}>{statusLabel(item.status)}</span>
                      </td>
                      <td className={booksTw.td}>{new Date(item.updatedAt).toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <div className={booksTw.pager}>
            <span>
              第 {page} / {totalPages} 页，共 {total} 条
            </span>
            <div className="inline-flex items-center gap-2">
              <button
                type="button"
                className={booksTw.button}
                disabled={page <= 1}
                onClick={() => setFilters((prev) => ({ ...prev, page: Math.max(1, (prev.page ?? 1) - 1) }))}
              >
                上一页
              </button>
              <button
                type="button"
                className={booksTw.button}
                disabled={page >= totalPages}
                onClick={() => setFilters((prev) => ({ ...prev, page: Math.min(totalPages, (prev.page ?? 1) + 1) }))}
              >
                下一页
              </button>
              <select
                className={booksTw.select}
                value={pageSize}
                onChange={(event) => setFilters((prev) => ({ ...prev, pageSize: Number(event.target.value), page: 1 }))}
              >
                <option value={20}>20 / 页</option>
                <option value={50}>50 / 页</option>
                <option value={100}>100 / 页</option>
              </select>
            </div>
          </div>
        </div>

        <aside className={booksTw.detailPanel}>
          {!selectedBook ? (
            <div className={booksTw.empty}>
              <div>
                <p className={booksTw.emptyTitle}>选择一本书</p>
              </div>
            </div>
          ) : (
            <>
              <header className={booksTw.detailHeader}>
                <div className={booksTw.detailTitle}>{selectedBook.title}</div>
                <div className={booksTw.detailSub}>{selectedBook.originalFilename}</div>
                <div className="mt-3 flex flex-wrap gap-2">
                  <span className={cx(booksTw.statusChip, statusClass(selectedBook.status))}>{statusLabel(selectedBook.status)}</span>
                  <span className={booksTw.metaPill}>{getBookPrimaryCategoryLabel(selectedBook.primaryCategory)}</span>
                </div>
              </header>

              <div className={booksTw.detailBody}>
                <section className={booksTw.detailSection}>
                  <div className={booksTw.detailLabel}>快捷操作</div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <button
                      type="button"
                      className={booksTw.actionBtn}
                      disabled={reprocessMutation.isPending}
                      onClick={() => {
                        const confirmed = window.confirm(`确认重处理《${selectedBook.title}》吗？`)
                        if (!confirmed) return
                        void reprocessMutation.mutateAsync(selectedBook.id)
                      }}
                    >
                      重处理
                    </button>
                    <button
                      type="button"
                      className={cx(booksTw.actionBtn, booksTw.actionDanger)}
                      disabled={deleteMutation.isPending}
                      onClick={() => {
                        const confirmed = window.confirm(`确认删除《${selectedBook.title}》吗？此操作不可撤销。`)
                        if (!confirmed) return
                        void deleteMutation.mutateAsync(selectedBook.id)
                      }}
                    >
                      删除
                    </button>
                  </div>
                </section>

                <div className={booksTw.detailGrid}>
                  <section className={booksTw.detailSection}>
                    <div className={booksTw.detailLabel}>书籍ID</div>
                    <div className={booksTw.detailValue}>{selectedBook.id}</div>
                  </section>
                  <section className={booksTw.detailSection}>
                    <div className={booksTw.detailLabel}>Owner</div>
                    <div className={booksTw.detailValue}>{selectedBook.ownerId}</div>
                  </section>
                  <section className={booksTw.detailSection}>
                    <div className={booksTw.detailLabel}>格式</div>
                    <div className={booksTw.detailValue}>{getBookFormatLabel(selectedBook.format)}</div>
                  </section>
                  <section className={booksTw.detailSection}>
                    <div className={booksTw.detailLabel}>语言</div>
                    <div className={booksTw.detailValue}>{getBookLanguageLabel(selectedBook.language)}</div>
                  </section>
                  <section className={booksTw.detailSection}>
                    <div className={booksTw.detailLabel}>更新时间</div>
                    <div className={booksTw.detailValue}>{new Date(selectedBook.updatedAt).toLocaleString()}</div>
                    <div className={booksTw.sectionMeta}>{relativeTime(selectedBook.updatedAt)}</div>
                  </section>
                  <section className={booksTw.detailSection}>
                    <div className={booksTw.detailLabel}>文件大小</div>
                    <div className={booksTw.detailValue}>{(selectedBook.sizeBytes / 1024 / 1024).toFixed(2)} MB</div>
                  </section>
                </div>

                <section className={booksTw.detailSection}>
                  <div className={booksTw.detailLabel}>标签</div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {selectedBook.tags.length > 0 ? (
                      selectedBook.tags.map((tag) => (
                        <span key={tag} className={booksTw.tagPill}>
                          #{tag}
                        </span>
                      ))
                    ) : (
                      <span className={booksTw.sectionMeta}>这本书还没有标签。</span>
                    )}
                  </div>
                </section>

                {selectedBook.errorMessage ? (
                  <section className={booksTw.detailSection}>
                    <div className={booksTw.detailLabel}>错误信息</div>
                    <div className={booksTw.errorBox}>{selectedBook.errorMessage}</div>
                  </section>
                ) : null}
              </div>
            </>
          )}
        </aside>
      </section>
    </div>
  )
}
