import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { getApiErrorMessage } from '@/features/auth/api/auth'
import {
  adminQueryKeys,
  deleteAdminBook,
  listAdminBooks,
  reprocessAdminBook,
  type ListAdminBooksParams,
} from '@/features/admin/api/admin'

const booksTw = {
  panel: 'rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white p-4',
  filters: 'grid gap-2 md:grid-cols-[minmax(0,1fr)_160px_180px_120px]',
  input:
    'h-9 rounded-[10px] border border-[rgba(0,0,0,0.12)] bg-white px-3 text-[13px] outline-none focus:border-[rgba(0,0,0,0.28)]',
  select:
    'h-9 rounded-[10px] border border-[rgba(0,0,0,0.12)] bg-white px-3 text-[13px] outline-none focus:border-[rgba(0,0,0,0.28)]',
  button:
    'inline-flex h-9 items-center justify-center rounded-[10px] border border-[rgba(0,0,0,0.12)] px-3 text-[13px] hover:bg-[#f4f4f4]',
  tableWrap: 'mt-4 overflow-auto rounded-[12px] border border-[rgba(0,0,0,0.08)]',
  table: 'min-w-[980px] w-full border-collapse text-left',
  th: 'border-b border-[rgba(0,0,0,0.08)] bg-[#fafafa] px-3 py-2 text-[12px] font-medium text-[#666]',
  td: 'border-b border-[rgba(0,0,0,0.06)] px-3 py-2 text-[13px]',
  actions: 'inline-flex flex-wrap items-center gap-2',
  actionBtn:
    'inline-flex h-8 items-center rounded-[9999px] border border-[rgba(0,0,0,0.12)] px-[10px] text-[12px] hover:bg-[#f5f5f5] disabled:opacity-50',
  notice: 'mb-3 rounded-[12px] border border-[#f4b0b4] bg-[#fff5f6] px-3 py-[10px] text-[13px] text-[#9f1820]',
  pager: 'mt-3 flex items-center justify-between text-[12px] text-[#666]',
}

function initialParams(): ListAdminBooksParams {
  return {
    query: '',
    ownerId: '',
    status: '',
    page: 1,
    pageSize: 20,
    sortBy: 'updatedAt',
    sortOrder: 'desc',
  }
}

export function AdminBooksPage() {
  const queryClient = useQueryClient()
  const [filters, setFilters] = useState<ListAdminBooksParams>(initialParams)
  const [errorText, setErrorText] = useState('')

  const queryKey = useMemo(() => adminQueryKeys.books(filters), [filters])

  const booksQuery = useQuery({
    queryKey,
    queryFn: () => listAdminBooks(filters),
    refetchInterval: 10_000,
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

  const items = booksQuery.data?.items ?? []
  const total = booksQuery.data?.total ?? 0
  const page = booksQuery.data?.page ?? filters.page ?? 1
  const pageSize = booksQuery.data?.pageSize ?? filters.pageSize ?? 20
  const totalPages = booksQuery.data?.totalPages ?? 1

  return (
    <div className={booksTw.panel}>
      {errorText ? <div className={booksTw.notice}>{errorText}</div> : null}
      {booksQuery.isError ? (
        <div className={booksTw.notice}>{getApiErrorMessage(booksQuery.error, '书籍列表加载失败。')}</div>
      ) : null}

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
          <option value="queued">queued</option>
          <option value="processing">processing</option>
          <option value="ready">ready</option>
          <option value="failed">failed</option>
        </select>
        <input
          className={booksTw.input}
          placeholder="按 ownerId 筛选"
          value={filters.ownerId ?? ''}
          onChange={(event) => setFilters((prev) => ({ ...prev, ownerId: event.target.value, page: 1 }))}
        />
        <button type="button" className={booksTw.button} onClick={() => setFilters(initialParams())}>
          重置筛选
        </button>
      </div>

      <div className={booksTw.tableWrap}>
        <table className={booksTw.table}>
          <thead>
            <tr>
              <th className={booksTw.th}>书籍ID</th>
              <th className={booksTw.th}>标题</th>
              <th className={booksTw.th}>Owner</th>
              <th className={booksTw.th}>状态</th>
              <th className={booksTw.th}>更新时间</th>
              <th className={booksTw.th}>操作</th>
            </tr>
          </thead>
          <tbody>
            {booksQuery.isLoading ? (
              <tr>
                <td className={booksTw.td} colSpan={6}>
                  正在加载...
                </td>
              </tr>
            ) : items.length === 0 ? (
              <tr>
                <td className={booksTw.td} colSpan={6}>
                  暂无数据
                </td>
              </tr>
            ) : (
              items.map((item) => (
                <tr key={item.id}>
                  <td className={booksTw.td}>{item.id}</td>
                  <td className={booksTw.td}>
                    <div className="max-w-[260px] truncate">{item.title}</div>
                    <div className="text-[12px] text-[#6f6f6f]">{item.originalFilename}</div>
                    {item.errorMessage ? <div className="text-[12px] text-[#b42318]">{item.errorMessage}</div> : null}
                  </td>
                  <td className={booksTw.td}>{item.ownerId}</td>
                  <td className={booksTw.td}>{item.status}</td>
                  <td className={booksTw.td}>{new Date(item.updatedAt).toLocaleString()}</td>
                  <td className={booksTw.td}>
                    <div className={booksTw.actions}>
                      <button
                        type="button"
                        className={booksTw.actionBtn}
                        disabled={reprocessMutation.isPending}
                        onClick={() => {
                          const confirmed = window.confirm(`确认重处理《${item.title}》吗？`)
                          if (!confirmed) return
                          void reprocessMutation.mutateAsync(item.id)
                        }}
                      >
                        重处理
                      </button>
                      <button
                        type="button"
                        className={booksTw.actionBtn}
                        disabled={deleteMutation.isPending}
                        onClick={() => {
                          const confirmed = window.confirm(`确认删除《${item.title}》吗？此操作不可撤销。`)
                          if (!confirmed) return
                          void deleteMutation.mutateAsync(item.id)
                        }}
                      >
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

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
            onClick={() =>
              setFilters((prev) => ({ ...prev, page: Math.min(totalPages, (prev.page ?? 1) + 1) }))
            }
          >
            下一页
          </button>
          <select
            className={booksTw.select}
            value={pageSize}
            onChange={(event) =>
              setFilters((prev) => ({ ...prev, pageSize: Number(event.target.value), page: 1 }))
            }
          >
            <option value={20}>20 / 页</option>
            <option value={50}>50 / 页</option>
            <option value={100}>100 / 页</option>
          </select>
        </div>
      </div>
    </div>
  )
}
