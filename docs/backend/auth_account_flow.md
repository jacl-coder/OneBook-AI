# OneBook AI 账号流程文档（登录/注册/验证码/密码）

本文档定义本项目账号相关流程，作为前后端联调与实现依据。

## 1. 范围
- 注册（密码注册、一次性验证码注册）
- 登录（密码登录、一次性验证码登录）
- 登录态修改密码
- 刷新会话与退出登录

## 2. 核心规则
- 注册支持两种模式：
  - 密码注册：先设置密码，再验证码校验。
  - 一次性验证码注册：无密码创建账号。
- 注册成功后直接签发 `token + refreshToken`，用户处于已登录状态。
- 已存在账号登录时，始终支持“密码登录”与“一次性验证码登录”二选一。
- 若账号无密码，访问 `/log-in/password` 时应跳转到 `/email-verification`（或返回错误码后前端跳转）。
- 登录态允许进入 `/reset-password/new-password` 设置/修改密码。

## 3. 页面映射
- 入口页：`/log-in`、`/create-account`
- 密码页：`/log-in/password`、`/create-account/password`
- 验证码页：`/email-verification`（兼容 `/log-in/verify`）
- 重置密码页：`/reset-password/new-password`
- 错误页：`/log-in/error`

## 4. 总流程图
```mermaid
flowchart TD
  A["打开 /log-in 或 /create-account"] --> B{"账号是否存在"}

  B -->|不存在| R1["注册流程"]
  B -->|存在| L1["登录流程"]

  R1 --> R2["输入邮箱"]
  R2 --> R3{"注册方式"}

  R3 -->|密码注册| R4["设置密码"]
  R4 --> R5["发送 OTP"]
  R5 --> R6["校验 OTP"]
  R6 --> R7["创建账号（有密码）"]
  R7 --> R8["签发 token + refreshToken（自动登录）"]
  R8 --> HOME["进入系统（已登录）"]

  R3 -->|一次性验证码注册| R9["发送 OTP"]
  R9 --> R10["校验 OTP"]
  R10 --> R11["创建账号（无密码）"]
  R11 --> R8

  L1 --> L2["输入邮箱后进入登录方式选择"]
  L2 --> L3["一次性验证码登录"]
  L2 --> L4["密码登录（/log-in/password）"]

  L3 --> L3a["发送 OTP"]
  L3a --> L3b["校验 OTP"]
  L3b --> L3c["签发 token + refreshToken"]
  L3c --> HOME

  L4 --> L4a{"账号是否有密码"}
  L4a -->|有| L4b["输入密码校验"]
  L4b --> L4c["签发 token + refreshToken"]
  L4c --> HOME
  L4a -->|无| L4d["跳转 /email-verification"]
  L4d --> L3

  HOME --> P1["登录态进入 /reset-password/new-password"]
  P1 --> P2{"当前是否有密码"}
  P2 -->|无密码账号| P3["仅输入新密码"]
  P2 -->|有密码账号| P4["输入当前密码 + 新密码"]
  P3 --> P5["更新成功"]
  P4 --> P5["更新成功"]

  HOME --> T1["业务请求携带 access token"]
  T1 --> T2{"是否 401"}
  T2 -->|否| T1
  T2 -->|是| T3["调用 /api/auth/refresh"]
  T3 --> T4{"刷新是否成功"}
  T4 -->|成功| T5["更新 token 并重放请求"]
  T5 --> T1
  T4 -->|失败| T6["清理会话并回到 /log-in"]
```

## 5. 接口契约建议（对外走 Gateway）
- `POST /api/auth/signup`：密码注册
- `POST /api/auth/login`：密码登录
- `POST /api/auth/refresh`：刷新 access token
- `POST /api/auth/logout`：退出登录
- `POST /api/auth/otp/send`：发送一次性验证码
- `POST /api/auth/otp/verify`：校验一次性验证码并换取登录态或注册凭据
- `POST /api/users/me/password`：登录态修改密码

### 5.1 OTP 关键字段
- `otp/send` 请求体：
  - `email`
  - `purpose`：`signup_password | signup_otp | login_otp`
- `otp/verify` 请求体：
  - `challengeId`
  - `email`
  - `purpose`
  - `code`
  - `password`（仅 `signup_password` 必填）

### 5.2 关键错误码约定
- `AUTH_PASSWORD_NOT_SET`：密码登录入口命中无密码账号，前端跳转到 `/email-verification`
- `AUTH_OTP_CHALLENGE_INVALID`：challenge 无效
- `AUTH_OTP_CODE_INVALID`：验证码错误
- `AUTH_OTP_CODE_EXPIRED`：验证码过期
- `AUTH_OTP_SEND_RATE_LIMITED`：OTP 发送限流
- `AUTH_OTP_VERIFY_RATE_LIMITED`：OTP 校验限流

## 6. 安全与风控要求
- OTP 单次使用，建议 5 分钟有效期，成功后立即失效。
- OTP 发送与校验均需限流（email + IP 维度）。
- 统一错误结构：`error + code + requestId + details`。
- 全链路透传并回传 `X-Request-Id`，日志保留 `request_id` 字段。
