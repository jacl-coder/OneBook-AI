# OneBook AI

面向个人/小团队的“书本对话”应用：用户上传电子书，系统解析并基于书本内容进行对话式问答（附出处），支持书库管理和会话历史。

## 当前状态
- 需求与功能规格：见 `docs/product/requirements.md` 与 `docs/product/functional_spec.md`
- 技术框架概览：见 `docs/architecture/tech_overview.md` 与 `docs/backend/backend_arch.md`
- RAG 演进目标：见 `docs/architecture/advanced_rag_plan.md`（后续检索优化默认按该基线推进）
- 前端联调说明：见 `docs/backend/backend_handoff.md`
- 前端开发流程：见 `docs/frontend/frontend_development_workflow.md`
- 后端链路已打通：上传 → 解析/分块 → 向量索引 → 检索问答
- Embedding 支持本地 Ollama 或 Gemini；回答生成使用 Gemini
- Ingest/Indexer 通过 Redis Streams 持久队列驱动，支持重试

## 功能概览（已实现）
- 上传 PDF/EPUB/TXT，支持扩展名白名单和大小限制（默认 50MB）。
- 书库列表/查询/删除；下载返回预签名 URL，浏览器下载名为原始文件名。
- 解析与分块：PDF/EPUB/TXT，语义分块并保留来源元数据（`source_type/source_ref`）。
- 索引：pgvector 向量写入，Embedding 支持批量/并发。
- 对话：基于向量检索生成回答并附出处；保存消息并拼接最近 N 轮历史。
- 管理员：用户/书籍列表与用户角色/状态管理。
- 认证：bcrypt 密码哈希（最小 12 位，且需包含大写/小写/数字/特殊字符）、短时效 access token（默认 15 分钟，RS256 + JWKS）、refresh token 轮换与重放检测（Redis）。
- 授权与风控：网关统一鉴权，管理员接口基于角色控制；网关与认证服务使用 Redis 分布式限流（安全优先，Redis 异常时拒绝请求）。
- 一致性改进：refresh token 轮换采用 Redis 原子 CAS；检测到旧 token 重放会撤销整个 token family。
- 队列可靠性改进：重试路径采用同一事务内 `XADD + XACK + XDEL`，避免“先 ack 再重投”丢任务窗口。

## 技术栈（当前）
- 后端：Go（标准库 `net/http`）
- 数据库：Postgres + pgvector
- 存储：MinIO（S3 兼容对象存储）
- 队列：Redis Streams（Ingest/Indexer），Redis（认证撤销状态与 refresh token）
- LLM：Gemini（回答生成）
- Embedding：Gemini 或 Ollama（本地模型）
- 解析：PDF 优先调用 `pdftotext`（可选），失败则使用 Go PDF 库

## 后端服务与 API
- 服务目录：`backend/services/`（Gateway + 多服务）
- 默认端口：Gateway 8080、Auth 8081、Book 8082、Chat 8083、Ingest 8084、Indexer 8085
- 公共路由（除认证与健康检查外均需登录态）：
  - 认证：`POST /api/auth/signup`，`POST /api/auth/login`，`POST /api/auth/refresh`，`POST /api/auth/logout`，`GET /api/auth/jwks`，`GET/PATCH /api/users/me`，`POST /api/users/me/password`
  - 书籍：`/api/books`（POST 上传字段 `file`，GET 列表），`/api/books/{id}`（GET/DELETE），`/api/books/{id}/download`
  - 对话：`POST /api/chats`（body: `bookId` + `question` + 可选 `conversationId`），`GET /api/conversations`，`GET /api/conversations/{id}/messages`
  - 管理员：`GET /api/admin/users`，`PATCH /api/admin/users/{id}`，`GET /api/admin/books`
  - 健康：`/healthz`

## 本地运行
```bash
# 启动依赖（Postgres + Redis + MinIO + Swagger UI）
docker compose up -d postgres redis minio minio-init swagger-ui

# 必需环境变量（手动逐服务运行时）
export REDIS_ADDR=localhost:6379
export GATEWAY_AUTH_JWKS_URL=http://localhost:8081/auth/jwks
export BOOK_AUTH_JWKS_URL=http://localhost:8081/auth/jwks
export CHAT_AUTH_JWKS_URL=http://localhost:8081/auth/jwks
# 本地 CORS（Vite + Swagger UI）
export CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:8086
export CORS_ALLOW_CREDENTIALS=true
# 如部署在反向代理后（仅当你能信任代理注入的真实源 IP）：
# export AUTH_TRUSTED_PROXY_CIDRS=10.0.0.0/8,192.168.0.0/16
# export GATEWAY_TRUSTED_PROXY_CIDRS=10.0.0.0/8,192.168.0.0/16

# 推荐：RS256（Auth 会签发 RS256 token，其他服务走 JWKS 验签）
# run.sh 会自动生成本地密钥；手动运行请先准备：
# export JWT_PRIVATE_KEY_PATH=/abs/path/to/OneBook-AI/secrets/jwt/private.pem
# export JWT_PUBLIC_KEY_PATH=/abs/path/to/OneBook-AI/secrets/jwt/public.pem
# export JWT_KEY_ID=jwt-active

# 推荐：内部服务也使用 RS256 短期 JWT（run.sh 会自动生成）
# export ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH=/abs/path/to/OneBook-AI/secrets/internal-jwt/private.pem
# export ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH=/abs/path/to/OneBook-AI/secrets/internal-jwt/public.pem
# export ONEBOOK_INTERNAL_JWT_KEY_ID=internal-active

# 运行 Auth 服务（默认 8081）
cd backend/services/auth
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/auth

# 运行 Book 服务（默认 8082）
cd ../book
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/book

# 运行 Chat 服务（默认 8083）
cd ../chat
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/chat

# 运行 Ingest 服务（默认 8084）
cd ../ingest
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/ingest

# 运行 Indexer 服务（默认 8085）
cd ../indexer
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/indexer

# 运行 Gateway（默认 8080）
cd ../gateway
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/server
```

一键启动（含依赖 + 本地 Ollama embeddings）：
```bash
# 默认会在 secrets/jwt/ 与 secrets/internal-jwt/ 下自动生成 RS256 密钥（若不存在）
# 注意：密钥仅用于本地开发，不应提交到 Git 仓库
# 会按 Auth -> Book -> Chat -> Ingest -> Indexer -> Gateway 顺序启动，并等待 /healthz 就绪
# 未设置 CORS_ALLOWED_ORIGINS 时，默认允许 http://localhost:8086（Swagger UI）和 http://localhost:5173（Vite dev）
./run.sh
```

## Docker 构建（服务镜像）
项目提供通用 `backend/Dockerfile`，通过构建参数指定服务与入口：
```bash
# 示例：构建 gateway
docker build -f backend/Dockerfile -t onebook-gateway \
  --build-arg SERVICE=gateway --build-arg CMD=server \
  backend
```

## 接口文档
- REST/OpenAPI（Gateway）：`backend/api/rest/openapi.yaml`
- REST/OpenAPI（Internal）：`backend/api/rest/openapi-internal.yaml`
- Swagger UI：`docker compose up -d swagger-ui` 后访问 `http://localhost:8086`

## 前端联调（建议先看）
- 统一只请求 Gateway：`http://localhost:8080`
- 会话使用：
  - 登录/注册成功后由网关设置 `HttpOnly` Cookie（`onebook_access` + `onebook_refresh`）
  - 业务请求通过 `withCredentials` 自动携带 Cookie
  - `401` 时前端触发 `POST /api/auth/refresh`，成功后自动重放原请求（单飞刷新）
- 书籍状态建议轮询：上传后轮询 `GET /api/books/{id}`，直到 `status` 为 `ready` 或 `failed`
- 对话前置条件：仅对 `ready` 书籍调用 `POST /api/chats`（新建会话时不传 `conversationId`，续聊时传已有 `conversationId`）
- 详细请求/错误语义与联调清单：`docs/backend/backend_handoff.md`

## 开发与测试
- 后端测试：`cd backend && go test ./...`
- Embedding 基准：`cd backend && go run ./cmd/bench_embed -text "你好" -dim 3072`
- 可选 OCR（扫描版 PDF）：
  - 安装官方 PaddleOCR CLI（参考官方仓库文档）：`python -m pip install paddleocr`
  - 打开 `INGEST_OCR_ENABLED=true`，并按需设置 `INGEST_OCR_COMMAND`/`INGEST_OCR_DEVICE`
  - 当前策略：按页质量触发与融合（native 提取低质量页优先采用 OCR 结果）
  - 阈值参数：
    - `INGEST_PDF_MIN_PAGE_RUNES`：页最小字符数门槛，低于门槛判为低质量页
    - `INGEST_PDF_MIN_PAGE_SCORE`：页质量分阈值（0~1）
    - `INGEST_PDF_OCR_MIN_SCORE_DELTA`：OCR 结果相对 native 的最小增益阈值

## 后续步骤（建议）
1) 可观测性：指标/追踪、队列与索引处理监控。
2) 检索质量：重排、去重与提示模板优化。
3) 安全与配额：细粒度权限、密钥轮换自动化、配额管理。
4) 前端：书库与对话 UI、上传进度与失败重试体验。
