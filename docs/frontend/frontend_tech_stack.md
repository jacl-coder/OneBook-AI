# OneBook AI 前端技术栈（当前实现）

本文档只描述仓库中已落地的实现，不包含候选方案。

## 1. 核心栈
- 框架：`React 19 + TypeScript`
- 构建：`Vite 7`
- 路由：`react-router-dom 7`
- 请求：`axios`
- 服务端状态：`@tanstack/react-query`
- 客户端会话状态：`zustand`
- 样式：`Tailwind CSS v4`（utilities 为主）+ 少量全局 CSS
- E2E：`Playwright`

## 2. 当前路由与页面
- `/`：`HomePage`
- `/chat`：`ChatPage`
- 认证相关路由（均由同一页面承载）：`LoginPage`
  - `/log-in`
  - `/create-account`
  - `/log-in/password`
  - `/create-account/password`
  - `/log-in/verify`
  - `/log-in/error`
  - `/email-verification`
  - `/reset-password`
  - `/reset-password/new-password`
  - `/reset-password/success`

## 3. 样式架构（已收敛）
- 入口：`frontend/src/index.css`
- 当前保留的样式文件：
  - `frontend/src/styles/base.css`
  - `frontend/src/styles/animations.css`
- 页面级遗留 CSS 已移除，页面样式主要在 TSX 的 Tailwind utilities 与常量中维护。

## 4. 认证与请求机制（与代码一致）
- 前端请求统一通过 `frontend/src/shared/lib/http/client.ts`。
- 请求配置为 `withCredentials: true`，由浏览器自动携带会话 Cookie。
- 响应拦截器实现 `401 -> refresh -> retry`。
- refresh 采用单飞（single-flight）：并发 401 只触发一次 `/api/auth/refresh`。
- 前端不维护 `Authorization: Bearer` 令牌注入逻辑。

## 5. 测试与验证
- 代码检查：`npm run lint`
- 生产构建：`npm run build`
- E2E：`npm run test:e2e`
- 主链路用例：`frontend/tests/e2e/chat-main-flow.spec.ts`

## 6. 当前未采用（避免误解）
- 未引入 `shadcn/ui`。
- 未采用 `React Hook Form` 作为统一表单层（当前登录页为自管理表单状态）。
- `zod` 目前在依赖中存在，但不是当前页面表单主路径的必需项。

## 7. 实践约定
- 以“薄 CSS + 厚 utilities”为默认策略。
- 仅在以下场景保留 CSS 文件：
  - 全局基础样式与字体
  - 全局动画
  - 第三方样式覆盖（如后续需要）
- 文档变更以代码现状为准；若实现发生变化，请同步更新本文件。
