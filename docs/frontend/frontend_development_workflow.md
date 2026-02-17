# OneBook AI 前端开发流程（当前实践）

本文档聚焦“当前仓库如何开发与验收”，不再描述通用方法论。

## 1. 联调边界
- 前端只调用 Gateway：`http://localhost:8080`
- API 语义以 `docs/backend/backend_handoff.md` 与 OpenAPI 为准
- 不直接请求内部服务端口

## 2. 本地启动
```bash
cd frontend
npm install
npm run dev
```

默认开发地址：`http://localhost:5173`

## 3. 环境变量
文件：`frontend/.env.example`

```env
VITE_API_BASE_URL=http://localhost:8080
VITE_API_TIMEOUT_MS=15000
```

## 4. 当前目录约定
- `frontend/src/app/`：应用级路由与 provider
- `frontend/src/pages/`：页面级实现（`HomePage`、`ChatPage`、`LoginPage`）
- `frontend/src/shared/lib/http/`：HTTP 客户端与拦截器
- `frontend/src/styles/`：保留的少量全局样式（`base.css`、`animations.css`）

## 5. 请求与会话约定
- 统一客户端：`frontend/src/shared/lib/http/client.ts`
- 必须启用 `withCredentials: true`
- 401 刷新策略：
  - 非 refresh 请求遇到 `401` 时进入刷新流程
  - 并发请求共享单个 refresh Promise（single-flight）
  - refresh 成功后自动重放原请求
  - refresh 失败时透传原错误，由上层做登录态回收

## 6. 页面实现约定
- 默认采用 Tailwind v4 utilities 组织样式
- 页面中重复 class 允许收敛为常量对象（例如 `homeTw`、`loginTw`、`chatTw`）
- 新增样式前优先判断是否可复用现有 utilities
- 只有全局基础样式与动画进入 CSS 文件

## 7. 测试与质量门禁
按 AGENTS.md 约定，前端改动需执行：

```bash
cd frontend
npm run lint
npm run build
```

E2E 主链路：

```bash
cd frontend
npm run test:e2e
```

当前主链路用例：`frontend/tests/e2e/chat-main-flow.spec.ts`

## 8. 提交流程建议
1. 完成单一目标改动（避免混杂重构）
2. 本地运行 `lint`、`build`（必要时补跑 E2E）
3. 更新对应文档（若行为/结构变化）
4. 使用 Conventional Commits 提交，scope 必填（如 `frontend`、`docs`、`app`）

## 9. 常见问题排查
- 页面样式偏差：
  - 先比对 TSX utilities 常量是否回归
  - 再检查 `index.css` 的导入顺序是否变化
- 登录态异常：
  - 检查浏览器是否携带 `onebook_access` / `onebook_refresh` Cookie
  - 检查 refresh 请求是否被错误拦截或重复触发
- E2E 不稳定：
  - 检查 `frontend/playwright.config.ts` 的 `baseURL/webServer` 是否匹配本机环境
