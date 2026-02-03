# OneBook AI

面向个人/小团队的“书本对话”应用：用户上传电子书，系统解析并基于书本内容进行对话式问答（附出处），支持书库管理和会话历史。

## 当前状态
- 需求与功能规格：见 `docs/requirements.md` 与 `docs/functional_spec.md`
- 技术框架概览：见 `docs/tech_overview.md`
- 后端服务已拆分并实现：上传 → 解析/分块 → 向量索引 → 检索问答链路。
- Embedding 支持本地 Ollama 或 Gemini；回答生成使用 Gemini。
- Ingest/Indexer 使用 Redis 持久队列，支持重试与失败恢复。

## 目标功能（MVP）
- 上传 PDF/EPUB/TXT，处理状态可视（排队/处理中/可对话/失败）。
- 书库列表与删除；按书发起对话。
- 基于书本内容回答，并附章节/页码/段落出处；会话保留最近若干轮上下文。

## 技术栈（规划）
- 后端：Go（Gin/Fiber/Echo 皆可）
- 数据库：Postgres + pgvector
- 前端：React（Vite/Next.js 皆可）
- 存储：MinIO（S3 兼容对象存储）；JWT/Session 鉴权

## 后端（当前实现）
- 位置：`backend/services/`（Gateway + 多服务）
- 路由（均需 Bearer token，除认证与健康检查）：
  - 认证：`POST /api/auth/signup`，`POST /api/auth/login` 返回 JWT；`GET /api/users/me`
  - 登出：`POST /api/auth/logout`（需要 Bearer token，立即失效会话）
  - 用户自助：`PATCH /api/users/me`（更新邮箱），`POST /api/users/me/password`（修改密码）
  - 书籍：`/api/books`（POST 上传字段 `file`，GET 列表），`/api/books/{id}`（GET/DELETE）
  - 对话：`POST /api/chats`（body: bookId + question）
  - 管理员：`GET /api/admin/users`，`PATCH /api/admin/users/{id}`（更新角色/状态），`GET /api/admin/books`
  - 健康：`/healthz`
- 状态流转：上传后进入排队→处理中→可对话；失败会写入错误信息。
- 存储：元数据保存在 Postgres，书籍文件存储在 MinIO。
- 服务目录：`backend/services/gateway/`（BFF/入口）、`backend/services/auth/`、`backend/services/book/`、`backend/services/ingest/`、`backend/services/indexer/`、`backend/services/chat/`。

### 本地运行
```bash
# 启动本地依赖（Postgres + Redis + MinIO）
docker compose up -d postgres redis minio minio-init

# 可选：统一内部服务令牌
export ONEBOOK_INTERNAL_TOKEN=onebook-internal

# 运行 Auth 服务（独立端口，默认 8081）
cd backend/services/auth
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/auth
# 配置：backend/services/auth/config.yaml；Gateway 配置 authServiceURL 指向该地址
# 如需使用 Redis 会话：保持 redisAddr 为 localhost:6379，并将 jwtSecret 置空

# 运行 Book 服务（独立端口，默认 8082）
cd ../book
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/book
# 配置：backend/services/book/config.yaml；Gateway 配置 bookServiceURL 指向该地址

# 运行 Chat 服务（独立端口，默认 8083）
cd ../chat
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/chat
# 配置：backend/services/chat/config.yaml；Gateway 配置 chatServiceURL 指向该地址
# 环境变量：GEMINI_API_KEY（用于生成回答）
# Embedding 可通过 OLLAMA_* 或 GEMINI_* 配置切换

# 运行 Ingest 服务（独立端口，默认 8084）
cd ../ingest
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/ingest
# 配置：backend/services/ingest/config.yaml

# 运行 Indexer 服务（独立端口，默认 8085）
cd ../indexer
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/indexer
# 配置：backend/services/indexer/config.yaml
# 环境变量：GEMINI_API_KEY（仅在使用 Gemini embedding 时必需）

# 另开终端运行 Gateway
cd ../gateway
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/server
# 环境变量：PORT（默认 8080）
# Gateway 配置：backend/services/gateway/config.yaml

# 示例：注册并调用（需替换 token）
# curl -X POST http://localhost:8080/api/auth/signup -d '{"email":"user@example.com","password":"secret"}'
# curl -H "Authorization: Bearer <token>" http://localhost:8080/api/users/me
# curl -H "Authorization: Bearer <token>" -F "file=@/path/book.pdf" http://localhost:8080/api/books
```

一键启动（含依赖 + 本地 Ollama embeddings）：
```bash
./run.sh
```
会同时启动 Auth 服务、Book 服务、Chat 服务、Ingest 服务、Indexer 服务与 Gateway。

## Docker 构建（服务镜像）
项目提供一个通用 `backend/Dockerfile`，通过构建参数指定服务与入口：
```bash
# 示例：构建 gateway
docker build -f backend/Dockerfile -t onebook-gateway \\
  --build-arg SERVICE=gateway --build-arg CMD=server \\
  backend
```

## 接口文档
- REST/OpenAPI（Gateway）：`backend/api/rest/openapi.yaml`
- REST/OpenAPI（Internal）：`backend/api/rest/openapi-internal.yaml`
- 本地 Swagger UI：`docker compose up -d swagger-ui` 后访问 `http://localhost:8086`（可在下拉中切换 Gateway/Internal）

## 后续步骤（建议）
1) 接入异步队列（Kafka/NATS/Redis Streams）与任务重试。
2) 打磨提示模板与检索重排策略，提升回答质量。
3) 完善鉴权、配额、错误路径与前端对接。
