# OneBook AI Frontend

## 1. 技术栈（当前）
- React 19 + TypeScript
- Vite 7
- React Router 7
- Axios + `withCredentials`
- TanStack Query
- Zustand
- Tailwind CSS v4

## 2. 启动方式
```bash
cd frontend
npm install
npm run dev
```

默认地址：`http://localhost:5173`

## 3. 环境变量
复制并修改 `frontend/.env.example`：

```env
VITE_API_BASE_URL=http://localhost:8081
VITE_API_TIMEOUT_MS=15000
```

代码内置默认值为 `VITE_API_BASE_URL=http://localhost:8081`、请求超时 `120000ms`；`.env.example` 中的 `15000ms` 是本地示例值。

## 4. 常用命令
```bash
# 代码检查
npm run lint

# 生产构建
npm run build

# 单元测试
npm run test:unit

# 本地预览打包产物
npm run preview
```

## 5. 目录说明
- `src/app/`：路由、Provider
- `src/pages/`：页面实现
- `src/shared/`：共享配置与 HTTP 客户端
- `src/styles/`：全局基础样式与动画

主要路由：

- `/`：首页
- `/log-in-or-create-account`、`/log-in/password`、`/email-verification`、`/reset-password*`：登录、注册、验证码、密码重置、OAuth 错误页
- `/chat`、`/chat/:conversationId`：书籍对话页，支持 SSE 流式回答
- `/library`：个人书库管理
- `/books/:bookId`：原文件阅读页，通过 `/api/books/{id}/content` 获取同源代理内容
- `/admin/overview`、`/admin/users`、`/admin/books`、`/admin/evals`、`/admin/audit`：管理员后台

## 6. 认证与请求约定
- 受保护接口走 Cookie 会话，不在前端拼接 Bearer token
- axios 拦截器支持 `401 -> refresh -> retry`
- refresh 使用 single-flight，避免并发风暴
- 用户头像通过 `POST /api/users/me/avatar` 上传，通过 `/api/users/{id}/avatar` 读取
- 书籍上传使用 `Idempotency-Key`，文件字段名为 `file`，分类字段为 `primaryCategory`，标签字段为 `tags[]`
- 阅读器使用 `/api/books/{id}/content`，该接口由 Gateway 代理原始文件并支持浏览器 Range 请求
- 聊天接口普通请求返回 JSON；发送 `Accept: text/event-stream` 时按 `chunk` / `final` / `error` 事件读取流式响应
- 管理后台 `/admin/evals` 支持发起评测任务；检索模式使用：
  - `hybrid_best`
  - `hybrid_no_rerank`
  - `dense_only`
  - `lexical_only`
- 评测任务参数支持：
  - `params.lexicalMode = online_real | offline_approx`
  - `params.rerankMode = service | fallback`

## 7. 参考文档
- `../README.md`
- `../backend/api/rest/openapi.yaml`
- `../frontend/.env.example`
