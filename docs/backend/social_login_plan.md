# OneBook AI 第三方登录接入准备文档

本文档用于第三方登录接入。v1 已按本文方案落地 Google 登录；Microsoft、Apple 只保留设计预留。

## 1. 结论

- v1 只实现 Google，使用服务端 OAuth 2.0 Authorization Code Flow，并加 PKCE、`state`、`nonce`。
- 前端只负责跳转到 Gateway 的开始登录接口；OAuth code/token 交换、ID Token 校验、账号绑定、Session 签发全部在后端完成。
- 后续接入顺序建议：Microsoft -> Apple。
- 当前 `User + UserIdentity` 模型可以承接第三方身份，但需要扩展 identity type 或新增 provider 字段。
- 第三方登录成功后仍签发我们自己的 HttpOnly Cookie session，不把 Google/Apple/Microsoft token 暴露给前端。

## 2. 参考资料

- OAuth 2.0 Security BCP RFC 9700: https://datatracker.ietf.org/doc/html/rfc9700
- OpenID Connect Core 1.0: https://openid.net/specs/openid-connect-core-1_0-18.html
- Google OAuth Web Server Flow: https://developers.google.com/identity/protocols/oauth2/web-server
- Google OpenID Connect: https://developers.google.com/identity/openid-connect/openid-connect
- Microsoft Authorization Code Flow: https://learn.microsoft.com/en-us/azure/active-directory/develop/v2-oauth2-auth-code-flow
- Microsoft OIDC: https://learn.microsoft.com/en-us/azure/active-directory/develop/v2-protocols-oidc
- Apple Sign in with Apple REST authorization: https://developer.apple.com/documentation/signinwithapplerestapi/request-an-authorization-to-the-sign-in-with-apple-server.
- Apple token validation: https://developer.apple.com/documentation/signinwithapplerestapi/generate-and-validate-tokens
- Apple client secret JWT: https://developer.apple.com/documentation/accountorganizationaldatasharing/creating-a-client-secret

## 3. 推荐架构

### 3.1 新增 Gateway 对外接口

- `GET /api/auth/oauth/{provider}/start`
  - provider: `google|apple|microsoft`
  - 生成 `state`、`nonce`、`code_verifier`、`code_challenge`
  - 将临时上下文写入 Redis，TTL 10 分钟
  - 302 跳转到 provider 授权页

- `GET /api/auth/oauth/{provider}/callback`
  - Google/Microsoft 使用 GET callback。
  - 校验 `state`
  - 使用 authorization code + `code_verifier` 向 provider token endpoint 换 token
  - 校验 ID Token
  - 绑定或创建本地用户
  - 签发 `onebook_access` / `onebook_refresh` HttpOnly Cookie
  - 302 跳回 `/chat`

- `POST /api/auth/oauth/apple/callback`
  - Apple Web 登录通常需要支持 `response_mode=form_post`，因此 callback 要支持 POST。
  - 其它处理与上面一致。

### 3.2 Auth Service 内部接口

- `POST /auth/oauth/complete`
  - Gateway 完成 provider token 校验后，向 auth service 提交标准化身份。
  - 建议 body:

```json
{
  "provider": "google",
  "subject": "provider-stable-sub",
  "email": "user@example.com",
  "emailVerified": true,
  "displayName": "User Name",
  "avatarUrl": "https://...",
  "rawClaims": {}
}
```

返回值沿用现有 `authResponse`，包含内部 access/refresh token 和 user。

## 4. 数据模型建议

当前 `UserIdentity(type, identifier)` 的唯一键是 `(type, identifier)`。第三方登录需要稳定保存 provider subject，不能只靠邮箱。

推荐改造：

```text
UserIdentity
- type: email|phone|oauth
- provider: google|apple|microsoft|null
- identifier: 对 email/phone 保存标准化地址；对 oauth 保存 provider subject
- verified_at
- is_primary
```

唯一约束建议：

```text
email/phone: unique(type, identifier)
oauth: unique(provider, identifier)
```

如果不想改现有唯一约束，也可以把 oauth identifier 存成 `google:<sub>`、`apple:<sub>`、`microsoft:<sub>`，但这会把 provider 信息编码进字符串，后续查询和审计不够干净。

## 5. 账号绑定规则

### 5.1 首次第三方登录

1. 先查 `UserIdentity(type=oauth, provider, subject)`。
2. 如果存在，直接登录对应用户。
3. 如果不存在且 provider 返回 verified email：
   - 查 `UserIdentity(type=email, email)`。
   - 如果存在，把 oauth identity 绑定到该 user。
   - 如果不存在，创建新 user，并创建 email identity + oauth identity。
4. 如果 provider 未返回 verified email：
   - 只创建 oauth identity。
   - `users.email` 留空或填空字符串。
   - 后续引导用户绑定邮箱/手机号。

### 5.2 Apple 特殊规则

- Apple 的 `sub` 是最重要的稳定标识，必须保存。
- Apple 可能只在首次授权时返回用户姓名。
- 用户可能选择隐藏邮箱，邮箱会是 `privaterelay.appleid.com`。
- 不能依赖 Apple email 作为唯一账号匹配依据，优先使用 Apple `sub`。

### 5.3 邮箱冲突策略

如果第三方返回的 email 已被另一个本地用户占用：

- 如果 email identity 已 verified，可以自动绑定 oauth identity 到该用户。
- 如果未来存在未验证 email identity，则不能自动绑定，应要求用户先用原账号登录后手动绑定。
- 所有绑定操作需要记录 audit log。

## 6. Provider 配置

### 6.1 Google

OAuth app 类型：Web application。

Scopes:

```text
openid email profile
```

配置项：

```env
OAUTH_GOOGLE_CLIENT_ID=
OAUTH_GOOGLE_CLIENT_SECRET=
OAUTH_GOOGLE_REDIRECT_URL=https://api.example.com/api/auth/oauth/google/callback
OAUTH_APP_BASE_URL=https://app.example.com
```

要点：

- 使用 Authorization Code Flow。
- 使用 PKCE，即使服务端是 confidential client，RFC 9700 也推荐使用。
- ID Token 校验 `iss`、`aud`、`exp`、`iat`、`nonce`、签名。
- Google OIDC discovery: `https://accounts.google.com/.well-known/openid-configuration`

### 6.2 Microsoft

OAuth app 类型：Microsoft identity platform web app。

Tenant 建议：

- 面向个人 Microsoft 账号：`consumers`
- 面向企业/学校：`organizations`
- 两者都支持：`common`

v1 建议先用 `consumers` 或 `common`，根据产品定位决定。

Scopes:

```text
openid email profile
```

配置项：

```env
OAUTH_MICROSOFT_CLIENT_ID=
OAUTH_MICROSOFT_CLIENT_SECRET=
OAUTH_MICROSOFT_TENANT=consumers
OAUTH_MICROSOFT_REDIRECT_URL=https://api.example.com/api/auth/oauth/microsoft/callback
```

要点：

- 使用 `https://login.microsoftonline.com/{tenant}/oauth2/v2.0/authorize`
- token endpoint 同 tenant 路径。
- ID Token 校验 `iss`、`aud`、`exp`、`iat`、`nonce`、签名。
- 如果使用 `common`，需要谨慎校验 `tid` 和 issuer，避免 tenant mix-up。

### 6.3 Apple

准备项：

- Apple Developer Program 账号。
- Services ID。
- Return URL。
- Sign in with Apple key，下载 `.p8`。
- Team ID、Key ID、Client ID。

Scopes:

```text
name email
```

配置项：

```env
OAUTH_APPLE_CLIENT_ID=
OAUTH_APPLE_TEAM_ID=
OAUTH_APPLE_KEY_ID=
OAUTH_APPLE_PRIVATE_KEY_PATH=
OAUTH_APPLE_REDIRECT_URL=https://api.example.com/api/auth/oauth/apple/callback
```

要点：

- 授权端点：`https://appleid.apple.com/auth/authorize`
- callback 需要支持 `form_post`。
- token 请求需要 `client_secret`，该 secret 是服务端用 `.p8` 私钥签出的 JWT。
- `client_secret` 不要每次请求都重新生成，可以缓存到接近过期前刷新。
- Apple Return URL 生产环境要求 HTTPS，不能用 localhost；本地联调需要 ngrok、Cloudflare Tunnel 或可访问的 HTTPS 域名。

## 7. 安全要求

- 必须校验 `state`，防 CSRF。
- 必须校验 `nonce`，防 ID Token replay。
- 必须使用 PKCE，保存 `code_verifier` 到 Redis，不进 URL。
- callback 只接受预注册 provider，不允许动态 provider URL。
- redirect URI 必须固定配置，不允许从 query 任意传入。
- ID Token 必须服务端校验签名和 claims。
- 只使用 provider 的 `sub` 作为第三方身份主键。
- provider access token / refresh token 默认不持久化；除非未来要调用 Google/Microsoft/Apple API。
- 登录成功后只签发本系统 HttpOnly Cookie，不把 provider token 返回前端。
- 所有失败场景返回通用错误，日志记录 provider、request_id、原因，但不记录 token。

## 8. 前端改动计划

当前 `/log-in-or-create-account` 和 `/chat` 登录弹窗中 Google 按钮已接入真实 OAuth；Apple、Microsoft 暂显示即将支持。后续接入时：

- Google 按钮跳转：`/api/auth/oauth/google/start`
- Apple 按钮跳转：`/api/auth/oauth/apple/start`
- Microsoft 按钮跳转：`/api/auth/oauth/microsoft/start`
- 手机/邮箱按钮继续保留为当前 identifier 切换，不走 OAuth。
- OAuth callback 成功后后端 302 到 `/chat`。
- OAuth callback 失败后后端 302 到 `/log-in/error`，带短错误码或通过 sessionStorage 展示错误。

## 9. 后端实现步骤

1. 新增 OAuth provider 配置结构和环境变量。
2. 新增 Redis OAuth state store：
   - `state`
   - `nonce`
   - `code_verifier`
   - `provider`
   - `return_to`
3. 新增 provider 抽象：
   - `BuildAuthURL`
   - `ExchangeCode`
   - `VerifyIDToken`
   - `NormalizeProfile`
4. 新增 Gateway start/callback handler。
5. 新增 Auth service `CompleteOAuthLogin`。
6. 扩展 `UserIdentity` 模型。
7. 更新 OpenAPI 和认证流程文档。
8. 前端按钮切换到真实 OAuth start URL。

## 10. 测试计划

Backend:

- state 不存在、过期、重复使用。
- nonce 不匹配。
- PKCE verifier 错误。
- code 重放。
- ID Token 签名无效。
- `aud` 不匹配。
- `iss` 不匹配。
- email verified 自动绑定。
- email 未验证不自动绑定。
- 已有 oauth identity 直接登录。
- disabled user 拒绝登录。

Frontend:

- `/log-in-or-create-account` 三个 OAuth 按钮跳转正确。
- `/chat` 登录弹窗三个 OAuth 按钮跳转正确。
- OAuth 成功后进入 `/chat` 并刷新 session。
- OAuth 失败后展示错误页。

Manual:

- Google 测试账号真实登录。
- Microsoft personal account 真实登录。
- Apple 使用 HTTPS tunnel 完成真实回调。
- 检查浏览器中没有 provider token 暴露给 JS。

## 11. 待决策问题

- Microsoft v1 是只支持个人账号，还是同时支持企业/学校账号。
- Apple 是否必须 v1 接入；如果产品暂不上架 iOS，可以放到 Google/Microsoft 之后。
- 是否允许自动把第三方 verified email 绑定到已有邮箱账号。
- 是否需要提供“账号设置 -> 绑定/解绑第三方登录”。
- 是否存 provider refresh token；v1 建议不存。
