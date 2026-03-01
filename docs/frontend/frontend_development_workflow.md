# OneBook AI 前端开发流程（当前实践）

本文档聚焦"当前仓库如何开发与验收"，不再描述通用方法论。

## 1. 技术栈

- 框架：`React 19 + TypeScript`
- 构建：`Vite 7`
- 路由：`react-router-dom 7`
- 请求：`axios`
- 服务端状态：`@tanstack/react-query`
- 客户端会话状态：`zustand`
- 样式：`Tailwind CSS v4`（utilities 为主）+ 少量全局 CSS

当前未采用：`shadcn/ui`、`React Hook Form`、`zod`。

## 2. 联调边界

- 前端只调用 Gateway：`http://localhost:8080`
- API 语义以 `docs/backend/backend_handoff.md` 与 OpenAPI 为准
- 不直接请求内部服务端口

## 3. 本地启动

```bash
cd frontend
npm install
npm run dev
```

默认开发地址：`http://localhost:5173`

## 4. 环境变量

文件：`frontend/.env.example`

```env
VITE_API_BASE_URL=http://localhost:8080
VITE_API_TIMEOUT_MS=120000
```

## 5. 目录与路由

### 目录约定

- `frontend/src/app/`：应用级路由与 provider
- `frontend/src/pages/`：页面级实现（`HomePage`、`ChatPage`、`LoginPage`）
- `frontend/src/shared/lib/http/`：HTTP 客户端与拦截器
- `frontend/src/styles/`：保留的少量全局样式（`base.css`、`animations.css`）

### 当前路由

- `/`：`HomePage`
- `/chat`：`ChatPage`
- 认证相关路由（均由 `LoginPage` 承载）：
  - `/log-in`、`/create-account`
  - `/log-in/password`、`/create-account/password`
  - `/log-in/verify`、`/log-in/error`
  - `/email-verification`
  - `/reset-password`、`/reset-password/new-password`、`/reset-password/success`

## 6. 请求与会话约定

- 统一客户端：`frontend/src/shared/lib/http/client.ts`
- 必须启用 `withCredentials: true`
- 前端不注入 `Authorization: Bearer` 头，由浏览器自动携带会话 Cookie
- 401 刷新策略：
  - 非 refresh 请求遇到 `401` 时进入刷新流程
  - 并发请求共享单个 refresh Promise（single-flight）
  - refresh 成功后自动重放原请求
  - refresh 失败时透传原错误，由上层做登录态回收

## 7. 样式架构

- 入口：`frontend/src/index.css`
- 保留的 CSS 文件：`frontend/src/styles/base.css`、`frontend/src/styles/animations.css`
- 页面级遗留 CSS 已移除，样式主要在 TSX 的 Tailwind utilities 与常量中维护
- 以"薄 CSS + 厚 utilities"为默认策略
- 仅在以下场景保留 CSS 文件：全局基础样式与字体、全局动画、第三方样式覆盖
- 页面中重复 class 允许收敛为常量对象（例如 `homeTw`、`loginTw`、`chatTw`）
- 新增样式前优先判断是否可复用现有 utilities

## 8. 测试与质量门禁

按 AGENTS.md 约定，前端改动需执行：

```bash
cd frontend
npm run lint
npm run build
npm run test:unit
```

## 9. 提交流程建议

1. 完成单一目标改动（避免混杂重构）
2. 本地运行 `lint`、`build`、`test:unit`
3. 更新对应文档（若行为/结构变化）
4. 使用 Conventional Commits 提交，scope 必填（如 `frontend`、`docs`、`app`）

## 10. 常见问题排查

- 页面样式偏差：
  - 先比对 TSX utilities 常量是否回归
  - 再检查 `index.css` 的导入顺序是否变化
- 登录态异常：
  - 检查浏览器是否携带 `onebook_access` / `onebook_refresh` Cookie
  - 检查 refresh 请求是否被错误拦截或重复触发
  - 检查后端 `.env`：`CORS_ALLOWED_ORIGINS` 包含 `http://localhost:5173,http://localhost:8086`，且 `CORS_ALLOW_CREDENTIALS=true`
