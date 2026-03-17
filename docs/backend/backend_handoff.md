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
- `POST /api/books`：上传书籍（`multipart/form-data`，字段名 `file`，支持 `primaryCategory` 与 `tags[]`）。
- `GET /api/books`：获取书库列表（返回 `items` + `count`，支持 `query/status/primaryCategory/tag/format/language` 筛选）。
- `GET /api/books/{id}`：查询单本书状态与元数据。
- `PATCH /api/books/{id}`：更新书名、主分类、标签。
- `GET /api/books/{id}/download`：获取预签名下载链接。
- `DELETE /api/books/{id}`：删除书籍。
- `POST /api/chats`：问答（body: `bookId`, `question`, 可选 `conversationId`, `debug`）。
- `GET /api/conversations`：会话列表（“你的聊天”）。
- `GET /api/conversations/{id}/messages`：单会话消息列表。
- `GET /api/users/me`：当前用户信息。
- `PATCH /api/users/me`：更新邮箱。
- `POST /api/users/me/password`：修改密码。
- `GET /api/admin/overview`：管理员概览指标。
- `GET /api/admin/users`：管理员用户分页列表（支持 `query/role/status/page/pageSize/sortBy/sortOrder`）。
- `GET /api/admin/users/{id}`：管理员查看单用户。
- `PATCH /api/admin/users/{id}`：管理员更新用户角色/状态。
- `POST /api/admin/users/{id}/disable`：管理员禁用用户。
- `POST /api/admin/users/{id}/enable`：管理员启用用户。
- `GET /api/admin/books`：管理员书籍分页列表（支持 `query/status/ownerId/primaryCategory/tag/format/language/page/pageSize/sortBy/sortOrder`）。
- `DELETE /api/admin/books/{id}`：管理员删除书籍。
- `POST /api/admin/books/{id}/reprocess`：管理员重处理书籍。
- `GET /api/admin/books/{id}/index-status`：查看书籍索引同步状态。
- `POST /api/admin/books/{id}/repair-index`：触发索引修复；当前实现为整书重处理。
- `GET /api/admin/audit-logs`：管理员操作审计日志分页列表。

## 4. 状态机与前端交互建议

### 4.0 幂等要求
- `POST /api/books`
- `POST /api/admin/books/{id}/reprocess`
- `POST /api/admin/books/{id}/repair-index`
- `POST /api/admin/evals/runs`
- 以上 4 个接口现在强制要求请求头 `Idempotency-Key`。
- 命中回放时，响应头会返回 `Idempotency-Replayed: true`。
- 同一个 `Idempotency-Key` 如果对应了不同请求内容，会返回 `409`。

### 4.1 书籍状态
- 状态流转：`queued -> processing -> ready | failed`
- 书籍元数据：
  - `primaryCategory`：固定单选分类。
  - `tags`：补充标签，最多 5 个。
  - `format`：系统字段，当前为 `pdf/epub/txt`。
  - `language`：系统字段，当前为 `zh/en/other/unknown`。
- 推荐前端策略：
  - 上传后立即进入详情页或列表轮询。
  - 每 2~3 秒轮询 `GET /api/books/{id}`。
  - 进入 `ready` 开放提问；进入 `failed` 展示 `errorMessage`。

### 4.2 对话可用性
- `POST /api/chats` 仅在书籍 `ready` 时可用。
- 当 `conversationId` 为空时，后端自动创建新会话并返回 `conversation` 对象。
- 当 `conversationId` 有值时，后端在该会话中续聊并回写历史消息。
- 响应主字段为：`conversation`、`answer`、`citations`、`abstained`；管理员可通过 `debug=true` 获取 `retrievalDebug`。
- 当前 chat 策略有 3 个独立开关：
  - `CHAT_QUERY_REWRITE_ENABLED`：是否启用模型驱动 query rewrite
  - `CHAT_MULTI_QUERY_ENABLED`：是否启用多查询召回
  - `CHAT_ABSTAIN_ENABLED`：是否启用策略拒答（书外实时问题、证据不足、grounding 失败）
- 当 `CHAT_ABSTAIN_ENABLED=false` 时，后端不会因为上述策略条件强制 `abstained=true`，而会返回更偏 best-effort 的谨慎回答。
- `retrievalDebug` 当前阶段固定包含 4 个阶段：
  - `dense`
  - `lexical`
  - `fused`
  - `reranked`
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
- 如需联调不同问答策略，可关注：
  - `CHAT_QUERY_REWRITE_ENABLED`
  - `CHAT_MULTI_QUERY_ENABLED`
  - `CHAT_ABSTAIN_ENABLED`
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
- 队列失败重试：Kafka `at-least-once` 投递，任务状态与去重落 Postgres；失败会按重试次数重投，超限进入 DLQ。
- 书籍删除采用“软删标记 + 后台异步清理 + 最终硬删”，正常列表/详情不会再返回软删记录。
- ingest/indexer 队列按 `job_type + resource_id` 去重，重复重处理不会堆积同书多条进行中任务。

## 8. 前端最小联调清单
1. 注册/登录成功后确认浏览器写入 `onebook_access` + `onebook_refresh`。
2. 上传一本 `pdf/epub/txt`。
3. 轮询到 `ready`。
4. 发起一次问答并展示 `answer + citations + abstained`。
5. 验证会话过期后通过 `/api/auth/refresh` 自动恢复。

## 9. 评测中心（Admin Eval Center）
- 后台新增 `/admin/evals`，管理员可创建评测数据集、发起评测任务、查看结果和下载 artifacts。
- 评测数据集支持两类来源：
  - `upload`：上传 `chunks/queries/qrels/predictions` 等文件。
  - `book`：绑定现有书籍并复用系统内 chunk，仍需补充 `queries/qrels`。
- 持久化拆分：
  - Postgres：`eval_dataset_models`、`eval_run_models`
  - 文件目录：默认 `data/eval-center`
- 运行模型：
  - 不使用 websocket
  - auth service 内部 worker 轮询数据库中的 `queued` 任务并执行
  - 前端通过轮询刷新运行状态
- 检索模式：
  - `hybrid_best`
  - `hybrid_no_rerank`
  - `dense_only`
  - `lexical_only`
- 评测参数支持：
  - `params.lexicalMode = offline_approx | online_real`
  - `params.rerankMode = fallback | service`
  - `params.denseWeight` / `params.lexicalWeight`
- artifacts 默认包含：
  - `run.json`
  - `metrics.json`
  - `per_query.jsonl`
  - `dense_run.jsonl` / `lexical_run.jsonl` / `fusion_run.jsonl` / `rerank_run.jsonl`（按实际运行生成）
