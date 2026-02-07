import { Link } from 'react-router-dom'

export function NotFoundPage() {
  return (
    <div className="center-wrap">
      <section className="panel card-narrow">
        <h2>页面不存在</h2>
        <p className="muted">请检查地址是否正确。</p>
        <Link to="/" className="btn btn-primary">
          返回官网
        </Link>
      </section>
    </div>
  )
}
