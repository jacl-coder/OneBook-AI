# 后端联调交接文档（Frontend Handoff）

本文档用于前端开发阶段，约定和后端联调的最小必要信息。默认本地网关地址为 `http://localhost:8080`。

## 1. 联调边界
- 前端只调用 Gateway 暴露的 `/api/*` 接口。
- 不直接调用内部服务（`book/chat/ingest/indexer`）端口。
- OpenAPI:
  - Gateway: `backend/api/rest/openapi.yaml`
  - Internal: `backend/api/rest/openapi-internal.yaml`

## 2. 认证与会话约定
- 账号完整流程（登录/注册/验证码/修改密码）请参考：`docs/backend/auth_account_flow.md`

### 2.1 登录态
- 注册：`POST /api/auth/signup`
- 登录：`POST /api/auth/login`
- 发送 OTP：`POST /api/auth/otp/send`
- 校验 OTP：`POST /api/auth/otp/verify`
- 忘记密码验证码校验：`POST /api/auth/password/reset/verify`
- 忘记密码完成重置：`POST /api/auth/password/reset/complete`
- 登录/注册/OTP 校验成功响应包含 `user`。
- Gateway 同时下发 HttpOnly 会话 Cookie：
  - `onebook_access`（短期）
  - `onebook_refresh`（长期）
- 若密码登录命中无密码账号，返回 `AUTH_PASSWORD_NOT_SET`，前端应跳转 `/email-verification`。

### 2.2 刷新逻辑
- 接口：`POST /api/auth/refresh`
- 不传请求体，依赖浏览器自动携带 `HttpOnly refresh cookie`。
- 成功会轮换更新 `onebook_access` / `onebook_refresh` cookie。
- 前端建议采用 single-flight：并发 `401` 仅发起一次 refresh，请求成功后重放原请求。
- 安全语义：
  - refresh token 采用轮换（rotation）。
  - 旧 token 被重放时，会撤销该 family，后续 refresh 会失败，需要重新登录。

### 2.3 鉴权方式
- 所有受保护接口由浏览器自动携带会话 Cookie。
- 前端不再注入 `Authorization` 头。

## 3. 主要业务接口（给前端）
- `GET /healthz`：服务健康检查。
- `POST /api/books`：上传书籍（`multipart/form-data`，字段名 `file`）。
- `GET /api/books`：获取书库列表（返回 `items` + `count`）。
- `GET /api/books/{id}`：查询单本书状态与元数据。
- `GET /api/books/{id}/download`：获取预签名下载链接。
- `DELETE /api/books/{id}`：删除书籍。
- `POST /api/chats`：问答（body: `bookId`, `question`, 可选 `conversationId`）。
- `GET /api/conversations`：会话列表（“你的聊天”）。
- `GET /api/conversations/{id}/messages`：单会话消息列表。
- `GET /api/users/me`：当前用户信息。
- `PATCH /api/users/me`：更新邮箱。
- `POST /api/users/me/password`：修改密码。

## 4. 状态机与前端交互建议

### 4.1 书籍状态
- 状态流转：`queued -> processing -> ready | failed`
- 推荐前端策略：
  - 上传后立即进入详情页或列表轮询。
  - 每 2~3 秒轮询 `GET /api/books/{id}`。
  - 进入 `ready` 开放提问；进入 `failed` 展示 `errorMessage`。

### 4.2 对话可用性
- `POST /api/chats` 仅在书籍 `ready` 时可用。
- 当 `conversationId` 为空时，后端自动创建新会话并返回 `conversationId`。
- 当 `conversationId` 有值时，后端在该会话中续聊并回写历史消息。
- 书籍未就绪时会返回冲突类错误（通常为 `409`）。

## 5. 错误响应约定
- 统一错误结构：
```json
{
  "error": "...",
  "code": "AUTH_INVALID_TOKEN",
  "requestId": "req_xxx",
  "details": [
    { "field": "email", "reason": "invalid_format" }
  ]
}
```
- 字段说明：
  - `error`：面向用户的错误说明。
  - `code`：稳定机器码，供前端分支处理与埋点。
  - `requestId`：请求追踪 ID（与响应头 `X-Request-Id` 对应）。
  - `details`：可选字段级错误详情。
- 端到端示例（推荐前端/运维直接照此落地）：
```bash
curl -i -X POST http://localhost:8080/api/chats \
  -H "Content-Type: application/json" \
  --cookie "onebook_access=<cookie_value>" \
  -H "X-Request-Id: req-demo-20260213-001" \
  -d '{"bookId":"book_xxx","question":"请总结第一章"}'
```
```http
HTTP/1.1 409 Conflict
X-Request-Id: req-demo-20260213-001
Content-Type: application/json

{
  "error": "book not ready",
  "code": "CHAT_BOOK_NOT_READY",
  "requestId": "req-demo-20260213-001",
  "details": [
    { "reason": "book_status_conflict" }
  ]
}
```
- 若请求未携带 `X-Request-Id`，网关会自动生成并在响应头/响应体返回。
- 常见状态码：
  - `400`：参数/请求体错误
  - `401`：未登录或 token 无效
  - `403`：权限不足
  - `404`：资源不存在
  - `409`：资源状态冲突（例如书籍未 ready）
  - `429`：限流
  - `500`：服务内部错误
  - `502`：网关下游服务不可用

## 6. 本地联调准备
- 复制并填写环境变量：`.env.example` -> `.env`
- 推荐一键启动：
```bash
./run.sh
```
- 若前端本地跨域访问网关，确保设置：
```bash
CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:8086
CORS_ALLOW_CREDENTIALS=true
```
（可与 Swagger 地址并存，逗号分隔）

## 7. 一致性与可靠性说明（当前实现）
- refresh token 并发轮换：Redis 原子 CAS，避免并发双成功。
- 队列失败重试：在同一事务中执行 `XADD + XACK + XDEL`，避免“已确认但未重投”的丢任务窗口。

## 8. 前端最小联调清单
1. 注册/登录成功后确认浏览器写入 `onebook_access` + `onebook_refresh`。
2. 上传一本 `pdf/epub/txt`。
3. 轮询到 `ready`。
4. 发起一次问答并展示 `answer + sources`。
5. 验证会话过期后通过 `/api/auth/refresh` 自动恢复。
