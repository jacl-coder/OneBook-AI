const stages = [
  { key: 'queued', label: 'queued', hint: '等待解析队列' },
  { key: 'processing', label: 'processing', hint: '正在抽取文本' },
  { key: 'ready', label: 'ready', hint: '可进入问答' },
  { key: 'failed', label: 'failed', hint: '解析失败待重试' },
] as const

const acceptedFormats = ['PDF', 'EPUB', 'TXT']

const parseFlow = [
  '上传文件并创建书籍记录',
  '异步解析文本并建立索引',
  '状态变为 ready 后开放对话',
]

export function LibraryPage() {
  return (
    <section className="panel">
      <header className="panel-header">
        <div>
          <h2>书库</h2>
          <p className="muted">
            统一管理电子书与解析状态，只有 <code>ready</code> 的书籍可进入问答。
          </p>
        </div>
        <button className="btn btn-primary" type="button">
          上传书籍（待接入）
        </button>
      </header>

      <div className="badge-list" aria-label="支持格式">
        {acceptedFormats.map((format) => (
          <span className="badge" key={format}>
            {format}
          </span>
        ))}
      </div>

      <div className="status-grid">
        {stages.map((stage) => (
          <article className="status-card" key={stage.key}>
            <h3>{stage.label}</h3>
            <p className="status-value">0</p>
            <p className="muted">{stage.hint}</p>
          </article>
        ))}
      </div>

      <section className="panel-sub">
        <h3>上传与解析流程</h3>
        <ul className="list-reset flow-list">
          {parseFlow.map((item, index) => (
            <li key={item}>
              <span className="flow-index">{`0${index + 1}`}</span>
              <span>{item}</span>
            </li>
          ))}
        </ul>
      </section>

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>标题</th>
              <th>格式</th>
              <th>状态</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td colSpan={5} className="muted">
                暂无数据，下一步接入 `/api/books`
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  )
}
