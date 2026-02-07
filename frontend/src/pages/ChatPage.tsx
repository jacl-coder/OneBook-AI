const readyBooks = [
  { id: 'B-001', title: '深入理解计算机系统', status: 'ready' },
  { id: 'B-002', title: '机器学习实战', status: 'ready' },
]

const recentSessions = [
  { id: 'S-101', label: '第 12 章内存管理', time: '今天 21:10' },
  { id: 'S-096', label: 'SVM 与核函数', time: '今天 19:42' },
  { id: 'S-083', label: '事务隔离级别', time: '昨天 22:16' },
]

export function ChatPage() {
  return (
    <section className="panel">
      <header className="panel-header">
        <div>
          <h2>对话</h2>
          <p className="muted">仅对 `ready` 书籍开放提问，答案将附带引用线索。</p>
        </div>
      </header>

      <div className="chat-layout">
        <aside className="panel-sub chat-sidebar">
          <section>
            <h3>可问答书籍</h3>
            <ul className="list-reset">
              {readyBooks.map((book) => (
                <li className="book-item" key={book.id}>
                  <div>
                    <p>{book.title}</p>
                    <p className="muted mono">{book.id}</p>
                  </div>
                  <span className="badge">{book.status}</span>
                </li>
              ))}
            </ul>
          </section>

          <section>
            <h3>最近会话</h3>
            <ul className="list-reset">
              {recentSessions.map((session) => (
                <li className="session-item" key={session.id}>
                  <p>{session.label}</p>
                  <p className="muted">
                    {session.id} · {session.time}
                  </p>
                </li>
              ))}
            </ul>
          </section>
        </aside>

        <div className="panel-sub conversation-window">
          <h3>提问区</h3>
          <div className="message-stream" aria-live="polite">
            <article className="msg msg-user">
              “帮我总结这本书第 3 章关于索引的核心观点。”
            </article>
            <article className="msg msg-ai">
              该章节重点强调了 B+ 树在范围查询中的优势，并比较了聚簇索引与二级索引的访问路径。
              <ul className="citation-list">
                <li>引用：第 3 章 3.2 节，第 67-69 页</li>
              </ul>
            </article>
          </div>

          <form className="form-grid" onSubmit={(e) => e.preventDefault()}>
            <label>
              问题
              <textarea rows={5} placeholder="请输入你的问题..." />
            </label>
            <button className="btn btn-primary" type="submit">
              发送（待接入）
            </button>
          </form>
        </div>
      </div>
    </section>
  )
}
