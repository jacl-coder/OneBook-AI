# OneBook AI 账号流程文档（邮箱/手机号认证）

本文档定义真实邮箱与中国大陆手机号登录、注册、验证码和密码重置流程。前端只调用 Gateway 的 `/api/*` 接口。

## 1. 核心规则
- 认证唯一键为 `UserIdentity(type=email|phone, identifier)`；`users.email` 仅保留为兼容展示字段。
- 邮箱统一 trim + lowercase；手机号统一为 E.164 中国大陆格式，例如 `+8613800000000`。
- 前端统一入口为 `/log-in-or-create-account`：输入邮箱/手机号后先查询账号状态，再进入密码登录、登录验证码或注册验证码流程；验证码必须由用户在验证码页手动点击发送。
- 注册流程固定为：输入邮箱/手机号 -> 手动发送验证码 -> 校验验证码得到 `verificationToken` -> 设置密码完成注册。
- 登录支持密码登录与验证码登录；密码登录命中无密码账号时返回 `AUTH_PASSWORD_NOT_SET`。
- Google 第三方登录使用 Gateway 服务端 OAuth callback 完成，成功后同样下发 HttpOnly Cookie。
- 忘记密码流程为：发送验证码 -> 校验验证码得到 `verificationToken` -> 设置新密码。
- 登录态只通过 Gateway 下发的 HttpOnly Cookie 维护：`onebook_access` 与 `onebook_refresh`。

## 2. 对外接口
- `POST /api/auth/verification/send`
  - body: `{ "channel": "email|phone", "identifier": "...", "purpose": "signup|login|password_reset" }`
  - response: `{ "challengeId": "...", "expiresInSeconds": 600, "resendAfterSeconds": 60, "maskedIdentifier": "..." }`
  - `purpose=login` 会先确认 identifier 已存在；不存在时不发送验证码，返回 `AUTH_INVALID_CREDENTIALS`，前端应提示用户先注册。
- `POST /api/auth/verification/verify`
  - body: `{ "channel": "email|phone", "identifier": "...", "purpose": "signup|login|password_reset", "challengeId": "...", "code": "123456" }`
  - `purpose=login` 成功后返回 `user` 并设置会话 Cookie。
  - `purpose=signup|password_reset` 成功后返回短期 `verificationToken`。
- `POST /api/auth/signup/complete`
  - body: `{ "channel": "email|phone", "identifier": "...", "verificationToken": "...", "password": "..." }`
  - 成功后创建用户并设置会话 Cookie。
- `POST /api/auth/login/password`
  - body: `{ "identifier": "...", "password": "..." }`
  - 成功后设置会话 Cookie。
- `POST /api/auth/login/methods`
  - body: `{ "identifier": "..." }`
  - response: `{ "exists": true, "passwordLogin": true }`
  - `exists=false` 时前端进入注册验证码流程；`exists=true,passwordLogin=false` 时前端进入登录验证码流程；`passwordLogin=true` 时前端进入密码登录流程。
- `POST /api/auth/password/reset/complete`
  - body: `{ "channel": "email|phone", "identifier": "...", "verificationToken": "...", "newPassword": "..." }`
- `GET /api/auth/oauth/google/start`
  - 开始 Google 登录，Gateway 生成 `state`、`nonce`、PKCE challenge 后 302 到 Google。
- `GET /api/auth/oauth/google/callback`
  - Google 回调地址；Gateway 校验 ID Token 后绑定或创建用户，设置 Cookie，并 302 回 `/chat`。
  - 前端回跳 origin 由 `OAUTH_APP_BASE_URL` 控制，本地默认 `http://localhost:5173`。
- 兼容别名：`/api/auth/login`、`/api/auth/signup`、`/api/auth/otp/send`、`/api/auth/otp/verify` 暂仍可用，但新前端应使用上面的新接口。

## 3. 验证码与安全
- 验证码为 6 位数字，Redis 只保存 bcrypt hash，不记录明文。
- 默认有效期 10 分钟，最多尝试 5 次；发送间隔默认 60 秒。
- 限流维度包含 IP、channel、identifier、purpose。
- 响应中的目标地址必须脱敏，例如 `j***e@example.com`、`+86****0000`。
- 注册和重置密码必须使用验证码校验后签发的 `verificationToken`，不能直接用验证码完成最终写操作。
- Google OAuth 使用 Authorization Code Flow + PKCE + `state` + `nonce`；Google token 不返回给前端。
- Google verified email 命中已有 verified email identity 时自动绑定到同一用户；未 verified email 不自动绑定。

## 4. 发送服务配置
- 本地默认使用 `console` provider，只记录脱敏目标和用途，不输出明文验证码。
- 邮箱生产 provider：Resend。
  - `AUTH_EMAIL_PROVIDER=resend`
  - `RESEND_API_KEY`
  - `RESEND_FROM`
- 短信生产 provider：阿里云号码认证服务 PNVS 的短信认证，适合个人开发者免企业资质接入。
  - `AUTH_SMS_PROVIDER=aliyun`
  - `ALIYUN_ACCESS_KEY_ID`
  - `ALIYUN_ACCESS_KEY_SECRET`
  - `ALIYUN_SMS_SIGN_NAME`：号码认证控制台赠送签名名称
  - `ALIYUN_SMS_SIGNUP_LOGIN_TEMPLATE_CODE=100001`
  - `ALIYUN_SMS_CHANGE_PHONE_TEMPLATE_CODE=100002`
  - `ALIYUN_SMS_PASSWORD_RESET_TEMPLATE_CODE=100003`
  - `ALIYUN_SMS_BIND_PHONE_TEMPLATE_CODE=100004`
  - `ALIYUN_SMS_VERIFY_BINDING_TEMPLATE_CODE=100005`
- 当前登录和注册共用模板 `100001`，重置密码使用模板 `100003`；手机号绑定相关模板已预留给后续账号绑定流程。
- 生产邮件域名需要配置 SPF/DKIM/DMARC；PNVS 短信模板变量使用 `{ "code": "123456", "min": "10" }`。
