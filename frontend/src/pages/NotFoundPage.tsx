import { Link } from 'react-router-dom'

export function NotFoundPage() {
  return (
    <div className="grid min-h-screen place-items-center p-6">
      <section className="grid w-full max-w-[420px] gap-4 rounded-[16px] border border-[var(--line)] bg-[var(--bg-elevated)] p-4">
        <h2 className="text-[1.08rem] tracking-[0.01em]">页面不存在</h2>
        <p className="text-[var(--text-muted)]">请检查地址是否正确。</p>
        <Link to="/" className="inline-flex w-fit items-center rounded-full bg-[var(--primary)] px-4 py-[0.43rem] text-white transition-colors duration-180 hover:bg-[var(--primary-strong)]">
          返回官网
        </Link>
      </section>
    </div>
  )
}
