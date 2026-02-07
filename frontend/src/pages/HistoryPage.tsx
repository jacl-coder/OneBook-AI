const sessions = [
  {
    id: 'S-001',
    book: '深入理解计算机系统',
    time: '2026-02-07 21:10',
    turns: 14,
    summary: '围绕进程与内存管理进行问答。',
  },
  {
    id: 'S-002',
    book: '机器学习实战',
    time: '2026-02-07 19:42',
    turns: 9,
    summary: '讨论监督学习与特征工程关键点。',
  },
  {
    id: 'S-003',
    book: '数据库系统概论',
    time: '2026-02-06 22:16',
    turns: 11,
    summary: '聚焦事务隔离级别与索引策略。',
  },
]

export function HistoryPage() {
  return (
    <section className="panel">
      <header className="panel-header">
        <div>
          <h2>会话历史</h2>
          <p className="muted">按书籍、时间和关键字回看问答记录。</p>
        </div>
      </header>

      <div className="history-filters">
        <button className="btn btn-secondary" type="button">
          最近 7 天
        </button>
        <button className="btn btn-secondary" type="button">
          全部书籍
        </button>
        <button className="btn btn-secondary" type="button">
          包含引用
        </button>
      </div>

      <div className="history-list" aria-live="polite">
        {sessions.map((session) => (
          <article className="history-item" key={session.id}>
            <div>
              <h3>{session.book}</h3>
              <p className="muted">
                会话 {session.id} · {session.time}
              </p>
            </div>
            <p>{session.summary}</p>
            <div className="history-item-footer">
              <span className="mono">{session.turns} turns</span>
              <button className="btn btn-ghost" type="button">
                继续会话（待接入）
              </button>
            </div>
          </article>
        ))}
      </div>
    </section>
  )
}
