import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { getApiErrorMessage } from '@/features/auth/api/auth'
import {
  adminQueryKeys,
  disableAdminUser,
  enableAdminUser,
  listAdminUsers,
  updateAdminUser,
  type ListAdminUsersParams,
} from '@/features/admin/api/admin'

const usersTw = {
  panel: 'rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white p-4',
  filters: 'grid gap-2 md:grid-cols-[minmax(0,1fr)_140px_140px_120px]',
  input:
    'h-9 rounded-[10px] border border-[rgba(0,0,0,0.12)] bg-white px-3 text-[13px] outline-none focus:border-[rgba(0,0,0,0.28)]',
  select:
    'h-9 rounded-[10px] border border-[rgba(0,0,0,0.12)] bg-white px-3 text-[13px] outline-none focus:border-[rgba(0,0,0,0.28)]',
  button:
    'inline-flex h-9 items-center justify-center rounded-[10px] border border-[rgba(0,0,0,0.12)] px-3 text-[13px] hover:bg-[#f4f4f4]',
  tableWrap: 'mt-4 overflow-auto rounded-[12px] border border-[rgba(0,0,0,0.08)]',
  table: 'min-w-[920px] w-full border-collapse text-left',
  th: 'border-b border-[rgba(0,0,0,0.08)] bg-[#fafafa] px-3 py-2 text-[12px] font-medium text-[#666]',
  td: 'border-b border-[rgba(0,0,0,0.06)] px-3 py-2 text-[13px]',
  actions: 'inline-flex flex-wrap items-center gap-2',
  actionBtn:
    'inline-flex h-8 items-center rounded-[9999px] border border-[rgba(0,0,0,0.12)] px-[10px] text-[12px] hover:bg-[#f5f5f5] disabled:opacity-50',
  notice: 'mb-3 rounded-[12px] border border-[#f4b0b4] bg-[#fff5f6] px-3 py-[10px] text-[13px] text-[#9f1820]',
  pager: 'mt-3 flex items-center justify-between text-[12px] text-[#666]',
}

function initialParams(): ListAdminUsersParams {
  return {
    query: '',
    role: '',
    status: '',
    page: 1,
    pageSize: 20,
    sortBy: 'createdAt',
    sortOrder: 'desc',
  }
}

export function AdminUsersPage() {
  const queryClient = useQueryClient()
  const [filters, setFilters] = useState<ListAdminUsersParams>(initialParams)
  const [errorText, setErrorText] = useState('')

  const queryKey = useMemo(() => adminQueryKeys.users(filters), [filters])

  const usersQuery = useQuery({
    queryKey,
    queryFn: () => listAdminUsers(filters),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, role, status }: { id: string; role?: 'user' | 'admin'; status?: 'active' | 'disabled' }) =>
      updateAdminUser(id, { role, status }),
    onSuccess: async () => {
      setErrorText('')
      await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
    },
    onError: (error) => {
      setErrorText(getApiErrorMessage(error, '用户更新失败，请稍后重试。'))
    },
  })

  const toggleStatusMutation = useMutation({
    mutationFn: ({ id, disabled }: { id: string; disabled: boolean }) =>
      disabled ? disableAdminUser(id) : enableAdminUser(id),
    onSuccess: async () => {
      setErrorText('')
      await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
    },
    onError: (error) => {
      setErrorText(getApiErrorMessage(error, '用户状态更新失败，请稍后重试。'))
    },
  })

  const items = usersQuery.data?.items ?? []
  const total = usersQuery.data?.total ?? 0
  const page = usersQuery.data?.page ?? filters.page ?? 1
  const pageSize = usersQuery.data?.pageSize ?? filters.pageSize ?? 20
  const totalPages = usersQuery.data?.totalPages ?? 1

  return (
    <div className={usersTw.panel}>
      {errorText ? <div className={usersTw.notice}>{errorText}</div> : null}
      {usersQuery.isError ? (
        <div className={usersTw.notice}>{getApiErrorMessage(usersQuery.error, '用户列表加载失败。')}</div>
      ) : null}

      <div className={usersTw.filters}>
        <input
          className={usersTw.input}
          placeholder="搜索邮箱或用户ID"
          value={filters.query ?? ''}
          onChange={(event) => setFilters((prev) => ({ ...prev, query: event.target.value, page: 1 }))}
        />
        <select
          className={usersTw.select}
          value={filters.role ?? ''}
          onChange={(event) =>
            setFilters((prev) => ({
              ...prev,
              role: event.target.value as ListAdminUsersParams['role'],
              page: 1,
            }))
          }
        >
          <option value="">全部角色</option>
          <option value="admin">管理员</option>
          <option value="user">普通用户</option>
        </select>
        <select
          className={usersTw.select}
          value={filters.status ?? ''}
          onChange={(event) =>
            setFilters((prev) => ({
              ...prev,
              status: event.target.value as ListAdminUsersParams['status'],
              page: 1,
            }))
          }
        >
          <option value="">全部状态</option>
          <option value="active">active</option>
          <option value="disabled">disabled</option>
        </select>
        <button type="button" className={usersTw.button} onClick={() => setFilters(initialParams())}>
          重置筛选
        </button>
      </div>

      <div className={usersTw.tableWrap}>
        <table className={usersTw.table}>
          <thead>
            <tr>
              <th className={usersTw.th}>用户ID</th>
              <th className={usersTw.th}>邮箱</th>
              <th className={usersTw.th}>角色</th>
              <th className={usersTw.th}>状态</th>
              <th className={usersTw.th}>创建时间</th>
              <th className={usersTw.th}>操作</th>
            </tr>
          </thead>
          <tbody>
            {usersQuery.isLoading ? (
              <tr>
                <td className={usersTw.td} colSpan={6}>
                  正在加载...
                </td>
              </tr>
            ) : items.length === 0 ? (
              <tr>
                <td className={usersTw.td} colSpan={6}>
                  暂无数据
                </td>
              </tr>
            ) : (
              items.map((item) => (
                <tr key={item.id}>
                  <td className={usersTw.td}>{item.id}</td>
                  <td className={usersTw.td}>{item.email}</td>
                  <td className={usersTw.td}>{item.role}</td>
                  <td className={usersTw.td}>{item.status}</td>
                  <td className={usersTw.td}>{new Date(item.createdAt).toLocaleString()}</td>
                  <td className={usersTw.td}>
                    <div className={usersTw.actions}>
                      <button
                        type="button"
                        className={usersTw.actionBtn}
                        disabled={updateMutation.isPending || item.role === 'admin'}
                        onClick={() =>
                          void updateMutation.mutateAsync({
                            id: item.id,
                            role: item.role === 'admin' ? 'admin' : 'admin',
                          })
                        }
                      >
                        设为管理员
                      </button>
                      <button
                        type="button"
                        className={usersTw.actionBtn}
                        disabled={updateMutation.isPending || item.role === 'user'}
                        onClick={() => void updateMutation.mutateAsync({ id: item.id, role: 'user' })}
                      >
                        设为用户
                      </button>
                      <button
                        type="button"
                        className={usersTw.actionBtn}
                        disabled={toggleStatusMutation.isPending || item.status === 'disabled'}
                        onClick={() => {
                          const confirmed = window.confirm(`确认禁用用户 ${item.email} 吗？`)
                          if (!confirmed) return
                          void toggleStatusMutation.mutateAsync({ id: item.id, disabled: true })
                        }}
                      >
                        禁用
                      </button>
                      <button
                        type="button"
                        className={usersTw.actionBtn}
                        disabled={toggleStatusMutation.isPending || item.status === 'active'}
                        onClick={() => void toggleStatusMutation.mutateAsync({ id: item.id, disabled: false })}
                      >
                        启用
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className={usersTw.pager}>
        <span>
          第 {page} / {totalPages} 页，共 {total} 条
        </span>
        <div className="inline-flex items-center gap-2">
          <button
            type="button"
            className={usersTw.button}
            disabled={page <= 1}
            onClick={() => setFilters((prev) => ({ ...prev, page: Math.max(1, (prev.page ?? 1) - 1) }))}
          >
            上一页
          </button>
          <button
            type="button"
            className={usersTw.button}
            disabled={page >= totalPages}
            onClick={() =>
              setFilters((prev) => ({ ...prev, page: Math.min(totalPages, (prev.page ?? 1) + 1) }))
            }
          >
            下一页
          </button>
          <select
            className={usersTw.select}
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
