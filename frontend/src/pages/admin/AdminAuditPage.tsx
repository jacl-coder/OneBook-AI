import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'

import { getApiErrorMessage } from '@/features/auth/api/auth'
import { adminQueryKeys, listAdminAuditLogs, type ListAdminAuditLogsParams } from '@/features/admin/api/admin'

const auditTw = {
  panel: 'rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white p-4',
  filters: 'grid gap-2 md:grid-cols-[160px_160px_minmax(0,1fr)_120px_120px]',
  input:
    'h-9 rounded-[10px] border border-[rgba(0,0,0,0.12)] bg-white px-3 text-[13px] outline-none focus:border-[rgba(0,0,0,0.28)]',
  button:
    'inline-flex h-9 items-center justify-center rounded-[10px] border border-[rgba(0,0,0,0.12)] px-3 text-[13px] hover:bg-[#f4f4f4]',
  tableWrap: 'mt-4 overflow-auto rounded-[12px] border border-[rgba(0,0,0,0.08)]',
  table: 'min-w-[980px] w-full border-collapse text-left',
  th: 'border-b border-[rgba(0,0,0,0.08)] bg-[#fafafa] px-3 py-2 text-[12px] font-medium text-[#666]',
  td: 'border-b border-[rgba(0,0,0,0.06)] px-3 py-2 text-[13px] align-top',
  notice: 'mb-3 rounded-[12px] border border-[#f4b0b4] bg-[#fff5f6] px-3 py-[10px] text-[13px] text-[#9f1820]',
  pager: 'mt-3 flex items-center justify-between text-[12px] text-[#666]',
}

function initialParams(): ListAdminAuditLogsParams {
  return {
    actorId: '',
    action: '',
    targetType: '',
    targetId: '',
    from: '',
    to: '',
    page: 1,
    pageSize: 20,
  }
}

function toCsv(value: unknown): string {
  const text = typeof value === 'string' ? value : JSON.stringify(value ?? '')
  return `"${text.replaceAll('"', '""')}"`
}

export function AdminAuditPage() {
  const [filters, setFilters] = useState<ListAdminAuditLogsParams>(initialParams)

  const queryKey = useMemo(() => adminQueryKeys.auditLogs(filters), [filters])

  const logsQuery = useQuery({
    queryKey,
    queryFn: () => listAdminAuditLogs(filters),
    refetchInterval: 15_000,
  })

  const items = logsQuery.data?.items ?? []
  const total = logsQuery.data?.total ?? 0
  const page = logsQuery.data?.page ?? filters.page ?? 1
  const pageSize = logsQuery.data?.pageSize ?? filters.pageSize ?? 20
  const totalPages = logsQuery.data?.totalPages ?? 1

  const exportCsv = () => {
    if (items.length === 0) return
    const header = ['id', 'createdAt', 'actorId', 'action', 'targetType', 'targetId', 'requestId', 'ip', 'userAgent']
    const rows = items.map((item) => [
      item.id,
      item.createdAt,
      item.actorId,
      item.action,
      item.targetType,
      item.targetId,
      item.requestId ?? '',
      item.ip ?? '',
      item.userAgent ?? '',
    ])
    const lines = [header, ...rows].map((row) => row.map((cell) => toCsv(cell)).join(','))
    const blob = new Blob([`${lines.join('\n')}\n`], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `admin-audit-${new Date().toISOString()}.csv`
    link.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className={auditTw.panel}>
      {logsQuery.isError ? (
        <div className={auditTw.notice}>{getApiErrorMessage(logsQuery.error, '审计日志加载失败。')}</div>
      ) : null}

      <div className={auditTw.filters}>
        <input
          className={auditTw.input}
          placeholder="actorId"
          value={filters.actorId ?? ''}
          onChange={(event) => setFilters((prev) => ({ ...prev, actorId: event.target.value, page: 1 }))}
        />
        <input
          className={auditTw.input}
          placeholder="action"
          value={filters.action ?? ''}
          onChange={(event) => setFilters((prev) => ({ ...prev, action: event.target.value, page: 1 }))}
        />
        <input
          className={auditTw.input}
          placeholder="targetType / targetId"
          value={`${filters.targetType ?? ''} ${filters.targetId ?? ''}`.trim()}
          onChange={(event) => {
            const [targetType, targetId] = event.target.value.trim().split(/\s+/, 2)
            setFilters((prev) => ({ ...prev, targetType: targetType ?? '', targetId: targetId ?? '', page: 1 }))
          }}
        />
        <button type="button" className={auditTw.button} onClick={() => setFilters(initialParams())}>
          重置
        </button>
        <button type="button" className={auditTw.button} disabled={items.length === 0} onClick={exportCsv}>
          导出 CSV
        </button>
      </div>

      <div className={auditTw.tableWrap}>
        <table className={auditTw.table}>
          <thead>
            <tr>
              <th className={auditTw.th}>时间</th>
              <th className={auditTw.th}>操作人</th>
              <th className={auditTw.th}>动作</th>
              <th className={auditTw.th}>目标</th>
              <th className={auditTw.th}>请求信息</th>
              <th className={auditTw.th}>变更</th>
            </tr>
          </thead>
          <tbody>
            {logsQuery.isLoading ? (
              <tr>
                <td className={auditTw.td} colSpan={6}>
                  正在加载...
                </td>
              </tr>
            ) : items.length === 0 ? (
              <tr>
                <td className={auditTw.td} colSpan={6}>
                  暂无日志
                </td>
              </tr>
            ) : (
              items.map((item) => (
                <tr key={item.id}>
                  <td className={auditTw.td}>{new Date(item.createdAt).toLocaleString()}</td>
                  <td className={auditTw.td}>{item.actorId}</td>
                  <td className={auditTw.td}>{item.action}</td>
                  <td className={auditTw.td}>
                    <div>{item.targetType}</div>
                    <div className="text-[12px] text-[#6f6f6f]">{item.targetId}</div>
                  </td>
                  <td className={auditTw.td}>
                    <div>{item.requestId ?? '-'}</div>
                    <div className="text-[12px] text-[#6f6f6f]">{item.ip ?? '-'}</div>
                  </td>
                  <td className={auditTw.td}>
                    <details>
                      <summary className="cursor-pointer text-[12px] text-[#444]">查看 before/after</summary>
                      <pre className="mt-1 max-w-[320px] overflow-auto rounded-[8px] bg-[#fafafa] p-2 text-[11px] leading-[1.5]">
                        {JSON.stringify({ before: item.before ?? {}, after: item.after ?? {} }, null, 2)}
                      </pre>
                    </details>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className={auditTw.pager}>
        <span>
          第 {page} / {totalPages} 页，共 {total} 条
        </span>
        <div className="inline-flex items-center gap-2">
          <button
            type="button"
            className={auditTw.button}
            disabled={page <= 1}
            onClick={() => setFilters((prev) => ({ ...prev, page: Math.max(1, (prev.page ?? 1) - 1) }))}
          >
            上一页
          </button>
          <button
            type="button"
            className={auditTw.button}
            disabled={page >= totalPages}
            onClick={() =>
              setFilters((prev) => ({ ...prev, page: Math.min(totalPages, (prev.page ?? 1) + 1) }))
            }
          >
            下一页
          </button>
          <select
            className={auditTw.input}
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
