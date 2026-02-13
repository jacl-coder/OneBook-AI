# OneBook AI RAG API 返回格式标准（推荐落地版）

## 1. 目标与结论

针对当前 OneBook AI（Gateway + Auth/Book/Chat/Ingest/Indexer）的最佳实践，不建议一次性把所有接口改成统一 `success/data` 信封；建议采用以下策略：

1. 统一错误返回格式（优先级最高）
2. 成功返回保持资源语义（兼容现有前端与网关）
3. 对 AI 特有能力单独定义协议（SSE 流式、异步任务）
4. 在现有 `/api/*` 路径上分阶段原地演进，不拆新版本前缀

这套方案在你的项目里最平衡：改动小、联调稳、可持续扩展。

---

## 2. 适用范围

- 对外接口：`/api/*`（经 Gateway 暴露）
- 内部接口：`/internal/*`（服务间调用）
- 返回类型：JSON、SSE、文件/预签名 URL

要求：对外与内部 JSON 错误统一使用 `error + code` 结构，避免网关/下游二次猜测。

---

## 3. 统一规则（必须执行）

### 3.1 HTTP 状态码表达协议结果

- `2xx`：请求成功
- `4xx`：客户端请求问题（参数、鉴权、权限、资源状态冲突）
- `5xx`：服务端错误（数据库、下游不可用、内部异常）

禁止把内部错误伪装成 `401/400`。

### 3.2 错误响应统一结构

当前项目已使用 `{"error":"..."}`，建议兼容扩展为：

```json
{
  "error": "Incorrect email address or password",
  "code": "AUTH_INVALID_CREDENTIALS",
  "requestId": "req_01HR...",
  "details": [
    { "field": "password", "reason": "too_short" }
  ]
}
```

字段约束：

- `error`：人类可读文案（向后兼容必保留）
- `code`：稳定机器码（前端分支、埋点、告警）
- `requestId`：全链路追踪 ID
- `details`：可选，字段级错误

### 3.3 成功响应保持语义化

保持当前风格，不强制统一信封：

- 单资源：直接返回对象（如 `User`、`Book`）
- 列表：`{ items, count }`
- 删除/无体：`204 No Content` 或 `{ status: "deleted" }`

这样能避免一次性重写全部前端与文档。

### 3.4 安全文案规则

- 登录失败统一外显：`Incorrect email address or password`
- 不暴露账号存在性、禁用状态、内部堆栈
- 内部真实错误写日志，不回传客户端

---

## 4. 错误码规范（落地版本）

推荐格式：`<DOMAIN>_<NAME>`，`DOMAIN` 固定为 `AUTH | BOOK | CHAT | ADMIN | SYSTEM`。

### 4.1 Auth

- `AUTH_INVALID_CREDENTIALS`：邮箱或密码错误
- `AUTH_INVALID_TOKEN`：访问令牌缺失/无效
- `AUTH_INVALID_REFRESH_TOKEN`：刷新令牌无效/过期/撤销
- `AUTH_PASSWORD_NOT_SET`：账号未设置密码，应走 OTP 登录
- `AUTH_EMAIL_ALREADY_EXISTS`：注册邮箱已存在
- `AUTH_INVALID_EMAIL`：邮箱格式不合法
- `AUTH_PASSWORD_POLICY_VIOLATION`：密码不满足强度策略
- `AUTH_EMAIL_REQUIRED`：缺少 email
- `AUTH_REFRESH_TOKEN_REQUIRED`：缺少 refreshToken
- `AUTH_PASSWORD_FIELDS_REQUIRED`：缺少 currentPassword/newPassword
- `AUTH_OTP_CHALLENGE_INVALID`：OTP challenge 无效或不存在
- `AUTH_OTP_CODE_INVALID`：OTP 验证码错误
- `AUTH_OTP_CODE_EXPIRED`：OTP 验证码过期
- `AUTH_OTP_SEND_RATE_LIMITED`：OTP 发送限流
- `AUTH_OTP_VERIFY_RATE_LIMITED`：OTP 校验限流
- `AUTH_INVALID_REQUEST`：Auth 域通用参数错误
- `AUTH_JWKS_NOT_CONFIGURED`：JWKS 未配置
- `AUTH_SIGNUP_RATE_LIMITED`：注册限流
- `AUTH_LOGIN_RATE_LIMITED`：登录限流
- `AUTH_REFRESH_RATE_LIMITED`：刷新限流
- `AUTH_PASSWORD_CHANGE_RATE_LIMITED`：改密限流
- `AUTH_SERVICE_UNAVAILABLE`：Auth 下游不可用

### 4.2 Book

- `BOOK_NOT_FOUND`：书籍不存在
- `BOOK_FORBIDDEN`：无权限访问书籍
- `BOOK_FILE_REQUIRED`：上传缺少 file
- `BOOK_UNSUPPORTED_FILE_TYPE`：不支持的文件类型
- `BOOK_INVALID_UPLOAD_FORM`：上传表单不合法
- `BOOK_FILE_TOO_LARGE`：上传文件超过限制
- `BOOK_INVALID_STATUS`：内部状态更新参数非法
- `BOOK_DOWNLOAD_URL_FAILED`：生成下载链接失败
- `BOOK_INVALID_REQUEST`：Book 域通用参数错误
- `BOOK_SERVICE_UNAVAILABLE`：Book 下游不可用

### 4.3 Chat

- `CHAT_BOOK_ID_REQUIRED`：缺少 bookId
- `CHAT_QUESTION_REQUIRED`：缺少 question
- `CHAT_BOOK_NOT_READY`：书籍未完成处理，暂不可问答
- `CHAT_FORBIDDEN`：无权限发起问答
- `CHAT_INVALID_REQUEST`：Chat 域通用参数错误
- `CHAT_SERVICE_UNAVAILABLE`：Chat 下游不可用

### 4.4 Admin

- `ADMIN_INVALID_ROLE`：角色参数不合法
- `ADMIN_INVALID_STATUS`：状态参数不合法
- `ADMIN_UPDATE_FIELDS_REQUIRED`：至少需要 role/status 之一
- `ADMIN_FORBIDDEN`：非管理员访问
- `ADMIN_NOT_FOUND`：管理员域资源不存在
- `ADMIN_INVALID_REQUEST`：Admin 域通用参数错误

### 4.5 System

- `SYSTEM_METHOD_NOT_ALLOWED`
- `SYSTEM_NOT_FOUND`
- `SYSTEM_RATE_LIMITED`
- `SYSTEM_UPSTREAM_UNAVAILABLE`
- `SYSTEM_INTERNAL_ERROR`
- `REQUEST_ERROR`（兜底，不建议业务依赖）

---

## 5. 状态码映射建议

- `400`：JSON 解析失败、字段缺失、格式错误
- `401`：未登录/凭证无效
- `403`：权限不足
- `404`：资源不存在
- `409`：资源或流程状态冲突（例如书未 `ready`，或账号未设密码需走 OTP 登录）
- `422`：业务校验失败（可选；若不用 422 可继续用 400）
- `429`：限流
- `500`：内部错误
- `502/503`：下游服务不可用/临时不可用

---

## 6. AI 场景专用协议

### 6.1 流式回答（SSE）

建议新增流式端点（例如 `/api/chats/stream`），事件类型固定：

- `message.delta`
- `usage`
- `message.completed`
- `error`
- `done`

示例：

```text
event: message.delta
data: {"requestId":"req_01","delta":"Hello"}

event: usage
data: {"requestId":"req_01","promptTokens":120,"completionTokens":80}

event: message.completed
data: {"requestId":"req_01","answer":"Hello world","finishReason":"stop"}

event: done
data: {"requestId":"req_01"}
```

### 6.2 异步任务（Ingest/Indexer）

统一任务状态：

- `queued`
- `running`
- `succeeded`
- `failed`
- `canceled`

统一任务结构建议：

```json
{
  "id": "job_123",
  "status": "running",
  "progress": 42,
  "error": "",
  "createdAt": "2026-02-13T14:00:00Z",
  "updatedAt": "2026-02-13T14:00:10Z"
}
```

---

## 7. 可观测性标准

### 7.1 `requestId` 贯穿

- 客户端可传 `X-Request-Id`
- 网关若缺失则生成
- 所有响应头回传 `X-Request-Id`
- 所有日志包含同一 `requestId`

### 7.2 审计与安全日志

- 鉴权相关统一记录：`event/outcome/path/method/ip/requestId`
- 对客户端返回泛化错误，对日志保留具体原因

---

## 8. 原地迁移（建议分阶段）

## Phase 1（立即执行，兼容）

1. 全部 `404` 统一为 JSON 错误体（禁止 `http.NotFound` 文本响应）
2. 错误响应新增可选字段 `code`、`requestId`、`details`
3. 加 `X-Request-Id` 中间件并透传
4. OpenAPI 同步补齐 `500`/`502` 等真实返回

## Phase 2（中期）

1. 建立项目级错误码表（文档 + 常量）- 已落地
2. Gateway 统一下游错误映射（内部错误不外泄）- 已落地
3. 补齐 SSE 协议文档与 SDK 解析约定

## Phase 3（可选，后续增强）

1. 若需要，再引入统一成功信封（仅在收益明确时执行）
2. 在当前 `/api/*` 上灰度切换并逐个前端页面联调验收

---

## 9. 本项目的推荐最终形态（简版）

- 成功：保持现有资源化返回
- 失败：统一 `error + code + requestId (+details)`
- AI 流式：独立 SSE 事件协议
- 任务型：统一状态机与任务结构
- 网关：统一错误归一化与可观测性

这是对当前 OneBook AI 成本最低、收益最高的“行业最佳实践”落地方案。
