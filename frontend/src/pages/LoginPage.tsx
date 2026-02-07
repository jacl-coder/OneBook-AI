import { Link } from 'react-router-dom'

export function LoginPage() {
  return (
    <div className="center-wrap">
      <section className="panel card-wide login-split">
        <aside className="login-hero">
          <h2>OneBook AI</h2>
          <p className="muted">
            面向个人书库的 AI 对话问答系统，聚焦上传、解析、问答与历史回溯。
          </p>
          <ul className="list-reset login-features">
            <li>电子书上传：PDF / EPUB / TXT</li>
            <li>解析状态跟踪：queued / processing / ready / failed</li>
            <li>基于内容问答：支持引用线索展示</li>
            <li>会话历史：按书籍与时间回看</li>
          </ul>
        </aside>

        <div>
          <h3>登录</h3>
          <p className="muted">先完成基础登录页，后续接入真实认证接口。</p>

          <form className="form-grid" onSubmit={(e) => e.preventDefault()}>
            <label htmlFor="email">邮箱</label>
            <input id="email" type="email" placeholder="you@example.com" autoComplete="email" />

            <label htmlFor="password">密码</label>
            <input
              id="password"
              type="password"
              placeholder="请输入密码"
              autoComplete="current-password"
            />

            <button type="submit" className="btn btn-primary">
              登录
            </button>
          </form>

          <p className="muted">
            开发阶段可直接进入 <Link to="/app/library">书库页</Link>。
          </p>
        </div>
      </section>
    </div>
  )
}
