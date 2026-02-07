import { Link, NavLink, Outlet } from 'react-router-dom'

const navItems = [
  { to: '/app/library', label: '书库' },
  { to: '/app/chat', label: '对话' },
  { to: '/app/history', label: '会话历史' },
]

export function AppLayout() {
  const flowSteps = [
    { index: '01', title: '上传电子书', desc: '支持 PDF / EPUB / TXT' },
    { index: '02', title: '内容解析', desc: '状态轮询直到 ready' },
    { index: '03', title: '基于内容问答', desc: '答案附带引用线索' },
  ]

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand">
          <Link to="/" className="brand-link">
            <span className="brand-mark" aria-hidden="true">
              OB
            </span>
            <div>
              <h1>OneBook AI</h1>
              <p>Personal Library Question-Answering System</p>
            </div>
          </Link>
        </div>
        <div className="topbar-meta">毕业设计 · 前端骨架阶段</div>
        <nav className="nav-links" aria-label="Primary">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                isActive ? 'nav-link nav-link-active' : 'nav-link'
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
      </header>

      <main className="page-wrap">
        <section className="workflow-strip" aria-label="核心流程">
          {flowSteps.map((step) => (
            <article className="flow-step" key={step.index}>
              <span className="flow-index">{step.index}</span>
              <h3 className="flow-title">{step.title}</h3>
              <p className="flow-desc">{step.desc}</p>
            </article>
          ))}
        </section>
        <Outlet />
      </main>
    </div>
  )
}
