import { useQuery } from '@tanstack/react-query'

import { getApiErrorMessage } from '@/features/auth/api/auth'
import { adminQueryKeys, getAdminOverview } from '@/features/admin/api/admin'

const overviewTw = {
  grid: 'grid gap-3 md:grid-cols-4',
  card: 'rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white p-4',
  label: 'text-[12px] text-[#6f6f6f]',
  value: 'mt-1 text-[24px] leading-[1.2] font-semibold text-[#0d0d0d]',
  panel: 'mt-4 rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white p-4',
  title: 'text-[15px] font-medium',
  row: 'mt-3 grid gap-2 md:grid-cols-2',
  statusItem:
    'flex items-center justify-between rounded-[10px] border border-[rgba(0,0,0,0.08)] bg-[#fafafa] px-3 py-2 text-[13px]',
  notice: 'rounded-[12px] border border-[#f4b0b4] bg-[#fff5f6] px-3 py-[10px] text-[13px] text-[#9f1820]',
}

export function AdminOverviewPage() {
  const overviewQuery = useQuery({
    queryKey: adminQueryKeys.overview,
    queryFn: getAdminOverview,
    refetchInterval: 30_000,
  })

  if (overviewQuery.isLoading) {
    return <div className={overviewTw.notice}>正在加载概览数据...</div>
  }

  if (overviewQuery.isError) {
    return <div className={overviewTw.notice}>{getApiErrorMessage(overviewQuery.error, '概览数据加载失败。')}</div>
  }

  const data = overviewQuery.data
  if (!data) return null

  return (
    <div>
      <div className={overviewTw.grid}>
        <div className={overviewTw.card}>
          <p className={overviewTw.label}>总用户数</p>
          <p className={overviewTw.value}>{data.totalUsers}</p>
        </div>
        <div className={overviewTw.card}>
          <p className={overviewTw.label}>活跃用户</p>
          <p className={overviewTw.value}>{data.activeUsers}</p>
        </div>
        <div className={overviewTw.card}>
          <p className={overviewTw.label}>禁用用户</p>
          <p className={overviewTw.value}>{data.disabledUsers}</p>
        </div>
        <div className={overviewTw.card}>
          <p className={overviewTw.label}>书籍总数</p>
          <p className={overviewTw.value}>{data.totalBooks}</p>
        </div>
      </div>

      <div className={overviewTw.row}>
        <div className={overviewTw.panel}>
          <h2 className={overviewTw.title}>书籍状态分布</h2>
          <div className="mt-3 grid gap-2">
            {data.booksByStatus.map((item) => (
              <div key={item.status} className={overviewTw.statusItem}>
                <span>{item.status}</span>
                <strong>{item.count}</strong>
              </div>
            ))}
          </div>
        </div>

        <div className={overviewTw.panel}>
          <h2 className={overviewTw.title}>24 小时统计</h2>
          <div className="mt-3 grid gap-2 text-[13px]">
            <div className={overviewTw.statusItem}>
              <span>新增书籍</span>
              <strong>{data.booksCreated24h}</strong>
            </div>
            <div className={overviewTw.statusItem}>
              <span>失败书籍</span>
              <strong>{data.booksFailed24h}</strong>
            </div>
            <div className={overviewTw.statusItem}>
              <span>统计窗口</span>
              <strong>{data.windowHours}h</strong>
            </div>
            <div className={overviewTw.statusItem}>
              <span>刷新时间</span>
              <strong>{new Date(data.refreshedAt).toLocaleString()}</strong>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
