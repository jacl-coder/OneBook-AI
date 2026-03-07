# API 响应规范

## 1. 适用范围

- 对外接口：`/api/*`（经 Gateway 暴露）
- 内部接口：`/internal/*`（服务间调用）

---

## 2. 统一规则

### 2.1 HTTP 状态码

| 状态码 | 含义 |
|---|---|
| `2xx` | 请求成功 |
| `400` | 参数/请求体错误 |
| `401` | 未登录或凭证无效 |
| `403` | 权限不足 |
| `404` | 资源不存在 |
| `409` | 资源/流程状态冲突 |
| `429` | 限流 |
| `500` | 服务内部错误 |
| `502/503` | 下游不可用 |

禁止把服务端错误伪装为 `401/400`。

### 2.2 错误响应结构（统一）

```json
{
  "error": "面向用户的错误说明",
  "code": "AUTH_INVALID_CREDENTIALS",
  "requestId": "req_01HR...",
  "details": [
    { "field": "password", "reason": "too_short" }
  ]
}
```

- `error`：人类可读文案（向后兼容必保留）
- `code`：稳定机器码（前端分支处理与埋点）
- `requestId`：全链路追踪 ID（与响应头 `X-Request-Id` 对应）
- `details`：可选，字段级错误

### 2.3 成功响应

保持资源语义，不强制统一信封：

- 单资源：直接返回对象（如 `User`、`Book`）
- 列表：`{ "items": [...], "count": N }`
- 删除/无体：`204 No Content`

### 2.4 安全文案

- 登录失败统一外显：`Incorrect email address or password`（不暴露账号存在性）
- 内部真实错误写日志，不回传客户端

---

## 3. 错误码规范

格式：`<DOMAIN>_<NAME>`，DOMAIN 固定为 `AUTH | BOOK | CHAT | ADMIN | SYSTEM`。

### AUTH

| 错误码 | 说明 |
|---|---|
| `AUTH_INVALID_CREDENTIALS` | 邮箱或密码错误 |
| `AUTH_INVALID_TOKEN` | 访问令牌缺失/无效 |
| `AUTH_INVALID_REFRESH_TOKEN` | 刷新令牌无效/过期/撤销 |
| `AUTH_PASSWORD_NOT_SET` | 账号未设置密码，应走 OTP 登录 |
| `AUTH_EMAIL_ALREADY_EXISTS` | 注册邮箱已存在 |
| `AUTH_INVALID_EMAIL` | 邮箱格式不合法 |
| `AUTH_PASSWORD_POLICY_VIOLATION` | 密码不满足强度策略 |
| `AUTH_EMAIL_REQUIRED` | 缺少 email |
| `AUTH_REFRESH_TOKEN_REQUIRED` | 缺少 refresh 会话 Cookie |
| `AUTH_PASSWORD_FIELDS_REQUIRED` | 缺少 currentPassword/newPassword |
| `AUTH_OTP_CHALLENGE_INVALID` | OTP challenge 无效或不存在 |
| `AUTH_OTP_CODE_INVALID` | OTP 验证码错误 |
| `AUTH_OTP_CODE_EXPIRED` | OTP 验证码过期 |
| `AUTH_OTP_SEND_RATE_LIMITED` | OTP 发送限流 |
| `AUTH_OTP_VERIFY_RATE_LIMITED` | OTP 校验限流 |
| `AUTH_PASSWORD_RESET_TOKEN_REQUIRED` | 缺少 resetToken |
| `AUTH_PASSWORD_RESET_TOKEN_INVALID` | resetToken 无效/过期 |
| `AUTH_PASSWORD_RESET_VERIFY_RATE_LIMITED` | 忘记密码验证码校验限流 |
| `AUTH_PASSWORD_RESET_RATE_LIMITED` | 忘记密码提交限流 |
| `AUTH_INVALID_REQUEST` | Auth 域通用参数错误 |
| `AUTH_JWKS_NOT_CONFIGURED` | JWKS 未配置 |
| `AUTH_SIGNUP_RATE_LIMITED` | 注册限流 |
| `AUTH_LOGIN_RATE_LIMITED` | 登录限流 |
| `AUTH_REFRESH_RATE_LIMITED` | 刷新限流 |
| `AUTH_PASSWORD_CHANGE_RATE_LIMITED` | 改密限流 |
| `AUTH_SERVICE_UNAVAILABLE` | Auth 下游不可用 |

### BOOK

| 错误码 | 说明 |
|---|---|
| `BOOK_NOT_FOUND` | 书籍不存在 |
| `BOOK_FORBIDDEN` | 无权限访问书籍 |
| `BOOK_FILE_REQUIRED` | 上传缺少 file |
| `BOOK_UNSUPPORTED_FILE_TYPE` | 不支持的文件类型 |
| `BOOK_INVALID_UPLOAD_FORM` | 上传表单不合法 |
| `BOOK_FILE_TOO_LARGE` | 上传文件超过限制 |
| `BOOK_INVALID_STATUS` | 内部状态更新参数非法 |
| `BOOK_DOWNLOAD_URL_FAILED` | 生成下载链接失败 |
| `BOOK_INVALID_REQUEST` | Book 域通用参数错误 |
| `BOOK_SERVICE_UNAVAILABLE` | Book 下游不可用 |

### CHAT

| 错误码 | 说明 |
|---|---|
| `CHAT_BOOK_ID_REQUIRED` | 缺少 bookId |
| `CHAT_QUESTION_REQUIRED` | 缺少 question |
| `CHAT_BOOK_NOT_READY` | 书籍未完成处理，暂不可问答 |
| `CHAT_FORBIDDEN` | 无权限发起问答 |
| `CHAT_INVALID_REQUEST` | Chat 域通用参数错误 |
| `CHAT_SERVICE_UNAVAILABLE` | Chat 下游不可用 |

### ADMIN

| 错误码 | 说明 |
|---|---|
| `ADMIN_INVALID_ROLE` | 角色参数不合法 |
| `ADMIN_INVALID_STATUS` | 状态参数不合法 |
| `ADMIN_UPDATE_FIELDS_REQUIRED` | 至少需要 role/status 之一 |
| `ADMIN_INVALID_PAGINATION` | 分页参数不合法 |
| `ADMIN_INVALID_SORT` | 排序参数不合法 |
| `ADMIN_ACTION_CONFLICT` | 管理动作与当前资源状态冲突 |
| `ADMIN_AUDIT_WRITE_FAILED` | 审计日志写入失败 |
| `ADMIN_FORBIDDEN` | 非管理员访问 |
| `ADMIN_NOT_FOUND` | 管理员域资源不存在 |
| `ADMIN_INVALID_REQUEST` | Admin 域通用参数错误 |

### SYSTEM

| 错误码 | 说明 |
|---|---|
| `SYSTEM_METHOD_NOT_ALLOWED` | 方法不允许 |
| `SYSTEM_NOT_FOUND` | 路由不存在 |
| `SYSTEM_RATE_LIMITED` | 系统级限流 |
| `SYSTEM_UPSTREAM_UNAVAILABLE` | 上游不可用 |
| `SYSTEM_INTERNAL_ERROR` | 内部错误 |

---

## 4. 可观测性

### requestId 贯穿

- 客户端可传 `X-Request-Id` 请求头；网关若缺失则自动生成。
- 所有响应头和响应体回传 `X-Request-Id`。
- 所有结构化日志包含同一 `requestId` 字段。

---

## 5. SSE 流式协议（草案）

> 适用于未来 `/api/chats/stream` 端点，当前未实现。

事件类型固定：

```
event: message.delta
data: {"requestId":"req_01","delta":"Hello"}

event: usage
data: {"requestId":"req_01","promptTokens":120,"completionTokens":80}

event: message.completed
data: {"requestId":"req_01","answer":"Hello world","finishReason":"stop"}

event: done
data: {"requestId":"req_01"}
```
