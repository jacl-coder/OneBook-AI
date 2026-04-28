import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { getApiErrorMessage } from '@/features/auth/api/auth'
import {
  adminQueryKeys,
  deleteAdminUser,
  listAdminUsers,
  updateAdminUser,
  type AdminUser,
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
  actions: 'inline-flex items-center gap-2',
  actionBtn:
    'inline-flex h-8 items-center rounded-[9999px] border border-[rgba(0,0,0,0.12)] px-[10px] text-[12px] hover:bg-[#f5f5f5] disabled:opacity-50',
  dangerBtn:
    'inline-flex h-8 items-center rounded-[9999px] border border-[#f2a6a6] px-[10px] text-[12px] text-[#a4161a] hover:bg-[#fff1f1] disabled:opacity-50',
  notice: 'mb-3 rounded-[12px] border border-[#f4b0b4] bg-[#fff5f6] px-3 py-[10px] text-[13px] text-[#9f1820]',
  pager: 'mt-3 flex items-center justify-between text-[12px] text-[#666]',
  modalBackdrop: 'fixed inset-0 z-40 flex items-center justify-center bg-black/30 px-4',
  modalPanel: 'w-full max-w-[520px] rounded-[14px] bg-white p-5 shadow-xl',
  modalTitle: 'text-[16px] font-semibold text-[#1f1f1f]',
  modalGrid: 'mt-4 grid gap-3',
  modalSection: 'mt-4 grid gap-3 border-t border-[rgba(0,0,0,0.08)] pt-4',
  label: 'grid gap-1 text-[12px] font-medium text-[#555]',
  segmented: 'grid grid-cols-2 gap-2',
  segmentBtn:
    'h-9 rounded-[10px] border border-[rgba(0,0,0,0.12)] px-3 text-[13px] hover:bg-[#f5f5f5] disabled:opacity-50',
  segmentBtnActive: 'border-[#222] bg-[#222] text-white hover:bg-[#222]',
  modalActions: 'mt-5 flex justify-end gap-2',
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
  const [editingUser, setEditingUser] = useState<AdminUser | null>(null)
  const [editForm, setEditForm] = useState({
    email: '',
    phone: '',
    role: 'user' as AdminUser['role'],
    status: 'active' as AdminUser['status'],
  })

  const queryKey = useMemo(() => adminQueryKeys.users(filters), [filters])

  const usersQuery = useQuery({
    queryKey,
    queryFn: () => listAdminUsers(filters),
  })

  const updateMutation = useMutation({
    mutationFn: ({
      id,
      email,
      phone,
      role,
      status,
    }: {
      id: string
      email?: string
      phone?: string
      role?: 'user' | 'admin'
      status?: 'active' | 'disabled'
    }) => updateAdminUser(id, { email, phone, role, status }),
    onSuccess: async () => {
      setErrorText('')
      setEditingUser(null)
      await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
    },
    onError: (error) => {
      setErrorText(getApiErrorMessage(error, '用户更新失败，请稍后重试。'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteAdminUser(id),
    onSuccess: async () => {
      setErrorText('')
      await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] })
    },
    onError: (error) => {
      setErrorText(getApiErrorMessage(error, '用户删除失败，请稍后重试。'))
    },
  })

  const items = usersQuery.data?.items ?? []
  const total = usersQuery.data?.total ?? 0
  const page = usersQuery.data?.page ?? filters.page ?? 1
  const pageSize = usersQuery.data?.pageSize ?? filters.pageSize ?? 20
  const totalPages = usersQuery.data?.totalPages ?? 1

  function openEditor(user: AdminUser) {
    setEditingUser(user)
    setEditForm({
      email: user.email ?? '',
      phone: user.phone ?? '',
      role: user.role,
      status: user.status,
    })
  }

  return (
    <div className={usersTw.panel}>
      {errorText ? <div className={usersTw.notice}>{errorText}</div> : null}
      {usersQuery.isError ? (
        <div className={usersTw.notice}>{getApiErrorMessage(usersQuery.error, '用户列表加载失败。')}</div>
      ) : null}

      <div className={usersTw.filters}>
        <input
          className={usersTw.input}
          placeholder="搜索邮箱、手机号或用户ID"
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
              <th className={usersTw.th}>手机号</th>
              <th className={usersTw.th}>角色</th>
              <th className={usersTw.th}>状态</th>
              <th className={usersTw.th}>创建时间</th>
              <th className={usersTw.th}>操作</th>
            </tr>
          </thead>
          <tbody>
            {usersQuery.isLoading ? (
              <tr>
                <td className={usersTw.td} colSpan={7}>
                  正在加载...
                </td>
              </tr>
            ) : items.length === 0 ? (
              <tr>
                <td className={usersTw.td} colSpan={7}>
                  暂无数据
                </td>
              </tr>
            ) : (
              items.map((item) => {
                const userLabel = item.email || item.phone || item.id

                return (
                  <tr key={item.id}>
                    <td className={usersTw.td}>{item.id}</td>
                    <td className={usersTw.td}>{item.email || '-'}</td>
                    <td className={usersTw.td}>{item.phone || '-'}</td>
                    <td className={usersTw.td}>{item.role}</td>
                    <td className={usersTw.td}>{item.status}</td>
                    <td className={usersTw.td}>{new Date(item.createdAt).toLocaleString()}</td>
                    <td className={usersTw.td}>
                      <div className={usersTw.actions}>
                        <button type="button" className={usersTw.actionBtn} onClick={() => openEditor(item)}>
                          编辑
                        </button>
                        <button
                          type="button"
                          className={usersTw.dangerBtn}
                          disabled={deleteMutation.isPending}
                          onClick={() => {
                            const confirmed = window.confirm(
                              `确认删除用户 ${userLabel} 吗？该用户的书籍会进入删除清理流程。`,
                            )
                            if (!confirmed) return
                            void deleteMutation.mutateAsync(item.id)
                          }}
                        >
                          删除
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              })
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

      {editingUser ? (
        <div className={usersTw.modalBackdrop} role="dialog" aria-modal="true" aria-labelledby="admin-user-edit-title">
          <div className={usersTw.modalPanel}>
            <div id="admin-user-edit-title" className={usersTw.modalTitle}>
              编辑用户
            </div>
            <div className="mt-1 break-all text-[12px] text-[#777]">{editingUser.id}</div>
            <div className={usersTw.modalGrid}>
              <label className={usersTw.label}>
                邮箱
                <input
                  className={usersTw.input}
                  value={editForm.email}
                  onChange={(event) => setEditForm((prev) => ({ ...prev, email: event.target.value }))}
                  placeholder="留空则移除邮箱登录"
                />
              </label>
              <label className={usersTw.label}>
                手机号
                <input
                  className={usersTw.input}
                  value={editForm.phone}
                  onChange={(event) => setEditForm((prev) => ({ ...prev, phone: event.target.value }))}
                  placeholder="留空则移除手机号登录"
                />
              </label>
            </div>
            <div className={usersTw.modalSection}>
              <div className={usersTw.label}>
                角色
                <div className={usersTw.segmented}>
                  <button
                    type="button"
                    className={`${usersTw.segmentBtn} ${editForm.role === 'user' ? usersTw.segmentBtnActive : ''}`}
                    onClick={() => setEditForm((prev) => ({ ...prev, role: 'user' }))}
                  >
                    普通用户
                  </button>
                  <button
                    type="button"
                    className={`${usersTw.segmentBtn} ${editForm.role === 'admin' ? usersTw.segmentBtnActive : ''}`}
                    onClick={() => setEditForm((prev) => ({ ...prev, role: 'admin' }))}
                  >
                    管理员
                  </button>
                </div>
              </div>
              <div className={usersTw.label}>
                状态
                <div className={usersTw.segmented}>
                  <button
                    type="button"
                    className={`${usersTw.segmentBtn} ${editForm.status === 'active' ? usersTw.segmentBtnActive : ''}`}
                    onClick={() => setEditForm((prev) => ({ ...prev, status: 'active' }))}
                  >
                    启用
                  </button>
                  <button
                    type="button"
                    className={`${usersTw.segmentBtn} ${
                      editForm.status === 'disabled' ? usersTw.segmentBtnActive : ''
                    }`}
                    onClick={() => setEditForm((prev) => ({ ...prev, status: 'disabled' }))}
                  >
                    禁用
                  </button>
                </div>
              </div>
            </div>
            <div className={usersTw.modalActions}>
              <button
                type="button"
                className={usersTw.button}
                disabled={updateMutation.isPending}
                onClick={() => setEditingUser(null)}
              >
                取消
              </button>
              <button
                type="button"
                className={usersTw.button}
                disabled={updateMutation.isPending}
                onClick={() =>
                  void updateMutation.mutateAsync({
                    id: editingUser.id,
                    email: editForm.email,
                    phone: editForm.phone,
                    role: editForm.role,
                    status: editForm.status,
                  })
                }
              >
                保存
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
