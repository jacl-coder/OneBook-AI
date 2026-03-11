# OneBook AI

> **面向个人/小团队的"书本对话"应用。** 用户上传电子书（PDF / EPUB / TXT），系统自动解析、分块、向量索引，之后可对书本内容进行中文问答，回答附带原文出处和引用来源。

---

## 目录

1. [项目定位与功能](#1-项目定位与功能)
2. [整体架构](#2-整体架构)
3. [技术栈](#3-技术栈)
4. [目录结构](#4-目录结构)
5. [后端微服务详解](#5-后端微服务详解)
6. [核心数据模型](#6-核心数据模型)
7. [完整 API 参考](#7-完整-api-参考)
8. [环境变量说明](#8-环境变量说明)
9. [本地开发快速上手](#9-本地开发快速上手)
10. [前端说明](#10-前端说明)
11. [安全机制](#11-安全机制)
12. [可观测性与日志](#12-可观测性与日志)
13. [RAG 演进路线（Advanced RAG 蓝图）](#13-rag-演进路线advanced-rag-蓝图)
14. [开发规范（AI Agent 必读）](#14-开发规范ai-agent-必读)
15. [待办事项](#15-待办事项)

---

## 1. 项目定位与功能

### 已实现功能

| 功能域 | 说明 |
|---|---|
| 书籍管理 | 上传 PDF/EPUB/TXT（最大 50MB），书库列表/查询/删除，预签名下载 URL（含原始文件名） |
| 解析与分块 | PDF 优先 `pdftotext`，失败回退 Go PDF 库；扫描版 PDF 可用 PaddleOCR Docker 服务按页质量融合；EPUB/TXT 语义分块 |
| 向量索引 | Ollama 本地 Embedding（可配置模型/维度），批量/并发写入 Qdrant；书籍状态自动流转 |
| RAG 对话 | Dense + Sparse 混合检索，证据约束生成，答案含引用及拒答支持；消息入库可续聊 |
| 认证与鉴权 | RS256 JWT（Access 15 分钟）+ Refresh Token 轮换（Redis 原子 CAS + 重放检测）；统一 Cookie 会话 |
| 管理后台 | 用户/书籍管理、重处理、操作审计日志、系统概览、RAG 评测中心（Admin Eval Center） |
| 速率限制 | Gateway/Auth 使用 Redis 分布式固定窗口限流；Redis 异常时拒绝请求（fail-closed） |
| 可靠性 | Redis Streams 持久队列（Ingest/Indexer），失败重试采用事务化 `XADD + XACK + XDEL` |

---

## 2. 整体架构

```
Browser / Frontend (React + Vite, :5173)
            │  HTTP + Cookie (withCredentials)
            ▼
  ┌─────────────────────┐
  │    Gateway  :8080   │  统一入口 · 鉴权 · 限流 · 路由
  └─────┬───────────────┘
        │ Internal JWT (RS256)
   ┌────┴────────────────────────────────────────────┐
   │                                                 │
   ▼                       ▼                         ▼
Auth :8081          Book :8082                Chat :8083
注册/登录/JWT        书籍元数据/MinIO          向量检索 + LLM 生成
RefreshToken                │                        │
撤销/限流           ┌───────┘              Qdrant (dense+sparse)
管理员/审计         │                               │
Eval Worker         │                    TextGenerator (Gemini/Ollama/OpenAI)
                    ▼
             Ingest :8084
             文件解析/分块
                    │ Redis Stream
                    ▼
             Indexer :8085
             Ollama Embedding → Qdrant

基础设施：
  Postgres             ← 用户/书籍/消息/chunk 元数据
  Redis                ← Token/限流/Streams 队列
  MinIO (S3)           ← 书籍文件存储
  Qdrant               ← chunk 向量索引与检索
  Ollama               ← 本地 Embedding
  OCR Service :8087    ← 可选 PaddleOCR Docker 服务（扫描版 PDF）
```

### 核心数据流

1. **上传** → Gateway → Book 服务 → 写 MinIO + Postgres → 入队 Ingest Stream
2. **解析** → Ingest 拉文件 → PDF/EPUB/TXT 解析 → 语义分块 → 写 chunks → 入队 Indexer Stream
3. **索引** → Indexer → Ollama Embedding → Qdrant 写向量 → 状态更新为 `ready`
4. **对话** → Chat → 问题向量化 → TopK 检索 → 上下文拼装 + 历史 N 轮 → LLM → 保存消息 + 引用

---

## 3. 技术栈

### 后端

| 组件 | 技术选型 |
|---|---|
| 语言/框架 | Go（标准库 `net/http`），无第三方 Web 框架 |
| ORM | GORM |
| JWT | `github.com/golang-jwt/jwt/v5`（RS256 + JWKS） |
| 对象存储 | MinIO SDK（`minio-go/v7`） |
| 队列 | Redis Streams（`go-redis/v9`） |
| 向量检索 | Qdrant（HTTP） |
| PDF 解析 | 优先 `pdftotext` CLI，回退 `ledongthuc/pdf` |
| EPUB 解析 | `golang.org/x/net` 解析 HTML |
| Embedding | Ollama HTTP API |
| LLM | `TextGenerator` 接口，支持 Gemini API / Ollama / OpenAI 兼容 endpoint |
| 加密 | `golang.org/x/crypto`（bcrypt，minimum cost 12） |

### 前端

| 组件 | 技术选型 |
|---|---|
| 框架 | React 19 + TypeScript |
| 构建工具 | Vite 7 |
| 路由 | React Router v7 |
| 数据获取 | TanStack Query v5 + Axios |
| 状态管理 | Zustand v5 |
| 样式 | Tailwind CSS v4 |
| Markdown 渲染 | react-markdown + rehype-highlight + remark-gfm |
| 单元测试 | Vitest |

---

## 4. 目录结构

```
OneBook-AI/
├── backend/                    # 后端 Go monorepo
│   ├── services/               # 各微服务（独立可运行）
│   │   ├── gateway/            # 统一入口、鉴权、限流、路由
│   │   ├── auth/               # 认证、用户管理、Eval Worker、审计
│   │   ├── book/               # 书籍元数据、MinIO 存储
│   │   ├── chat/               # 检索 + LLM 生成
│   │   ├── ingest/             # 文件解析与分块
│   │   └── indexer/            # Embedding + 向量写入
│   ├── pkg/                    # 可复用公共包
│   │   ├── ai/                 # TextGenerator 接口（Gemini/Ollama/OpenAI 兼容）
│   │   ├── auth/               # JWT 工具
│   │   ├── domain/             # 共享领域类型（Book / User / Chunk 等）
│   │   ├── queue/              # Redis Streams 封装
│   │   ├── retrieval/          # 向量检索逻辑（dense + sparse）
│   │   ├── storage/            # MinIO 封装
│   │   └── store/              # GORM 数据访问层
│   ├── internal/               # 内部工具（不对外复用）
│   │   ├── eval/               # RAG 评测（指标计算、测试数据）
│   │   ├── ratelimit/          # 分布式限流
│   │   ├── servicetoken/       # 内部服务 JWT
│   │   ├── usertoken/          # 用户 Token 工具
│   │   └── util/               # 通用工具
│   ├── cmd/                    # 独立可执行命令
│   │   ├── check_openapi/      # OpenAPI 规范校验工具
│   │   └── rag_eval/           # RAG 离线评测 CLI
│   ├── api/rest/               # OpenAPI 规范
│   │   ├── openapi.yaml        # 对外（Gateway）接口
│   │   └── openapi-internal.yaml  # 内部服务接口
│   ├── Dockerfile              # 通用多服务 Dockerfile
│   ├── go.mod                  # 模块：onebookai
│   └── logs/                   # 运行时日志（gitignored）
│
├── frontend/                   # 前端 React 应用
│   └── src/
│       ├── app/                # 路由（router.tsx）& 全局 Providers
│       ├── pages/              # 页面组件（ChatPage / LibraryPage / LoginPage 等）
│       ├── features/           # 功能域（auth / library / admin）
│       └── shared/             # 共享组件、hooks、API 客户端
│
├── services/ocr/               # 可选 OCR 服务（PaddleOCR，Docker 部署）
├── docs/                       # 详细文档
│   ├── architecture/           # 技术框架、Advanced RAG 蓝图、RAG 评测计划
│   ├── backend/                # 后端架构、前端联调交接、API 响应规范、认证流程
│   ├── frontend/               # 前端开发流程
│   └── product/                # 需求文档、功能规格
├── docker-compose.yml          # 依赖服务（Postgres/Redis/Qdrant/MinIO/Swagger UI）
├── .env.example                # 所有配置项示例（复制为 .env 后填写）
├── run.sh                      # 一键启动脚本（含密钥自动生成）
├── scripts/                    # 辅助脚本
│   ├── restart-service.sh      # 单服务热重启
│   ├── run-rag-eval.sh         # RAG 评测一键脚本
│   ├── start-backend.sh        # 后端各服务启动脚本
│   ├── start-frontend.sh       # 前端启动脚本
│   └── ollama-embedding.sh     # Ollama Embedding 模型拉取辅助脚本
├── secrets/                    # 本地 JWT 密钥（gitignored，run.sh 自动生成）
└── AGENTS.md                   # AI Agent 工作规范（规范优先级同本文件）
```

---

## 5. 后端微服务详解

### Gateway（:8080）

- 统一对外入口：所有 `/api/*` 路由在此鉴权后转发到下游服务。
- 通过 Gateway 下游调用时使用**内部短时效服务 JWT**（RS256，校验 `iss/aud/exp`）。
- 提供管理员后台聚合接口（`/api/admin/*`）。
- 对外 CORS：`CORS_ALLOWED_ORIGINS` + `CORS_ALLOW_CREDENTIALS`。
- `/healthz` 健康检查。

### Auth（:8081）

- 注册（`POST /api/auth/signup`）、登录（`POST /api/auth/login`）、登出（`POST /api/auth/logout`）。
- OTP 验证（`POST /api/auth/otp/send`、`POST /api/auth/otp/verify`）。
- 忘记密码重置（`POST /api/auth/password/reset/verify`、`POST /api/auth/password/reset/complete`）。
- RS256 JWT 签发（私钥），JWKS 端点（`GET /api/auth/jwks`）供其他服务本地验签。
- Refresh Token：轮换 + Redis 原子 CAS + 重放整个 token family 撤销。
- 管理员用户管理（启停、角色变更）、操作审计日志、系统概览。
- **Admin Eval Center Worker**：轮询 Postgres 中 `queued` 评测任务并执行，结果写回数据库+文件。

### Book（:8082）

- 上传校验（扩展名白名单：pdf/epub/txt；大小限制：默认 50MB）。
- 书籍元数据（`primaryCategory`、`tags[]`、`format`、`language`）存 Postgres，文件存 MinIO。
- 上传后写入 Redis Stream → 触发 Ingest。
- 状态机：`queued → processing → ready | failed`。
- 软删除 + 后台异步清理（最终硬删除）。
- 支持 `PATCH /api/books/{id}` 更新书名/分类/标签。

### Ingest（:8084）

- 从 Redis Stream 消费任务，拉取 MinIO 文件。
- PDF：优先 `pdftotext`，失败回退 Go PDF 库；按页质量评估触发 OCR（阈值可配置）。
- OCR 融合策略：native 低质量页优先采用 OCR 结果，阈值可通过 `INGEST_PDF_*` 环境变量配置。
- EPUB：解析 HTML 内容。TXT：直接分块。
- 语义分块（`INGEST_CHUNK_SIZE`/`INGEST_CHUNK_OVERLAP`），保留来源元数据。
- Chunk 元数据：`source_type`、`source_ref`、`extract_method`、`page`、`section`、`chunk`、`document_id`、`chunk_index`、`chunk_count`、`content_sha256`、`content_runes`、`page_quality_score`。
- 写入 chunks 后触发 Indexer Stream。

### Indexer（:8085）

- 从 Redis Stream 消费任务，批量调用 Ollama 生成向量（维度 `ONEBOOK_EMBEDDING_DIM`，默认 3072）。
- 写入 Qdrant（collection：`QDRANT_COLLECTION`，默认 `onebook_chunks`）。
- 写入完成后更新书籍状态为 `ready`。

### Chat（:8083）

- 问题向量化 → Dense + Sparse 混合检索 Qdrant（TopN recall → rerank → TopK context）。
- 拼装上下文（最近 N 轮历史 + 检索 chunks）。
- 调用 `TextGenerator` → LLM 生成回答，附引用；证据不足时拒答（返回 `abstained: true`）。
- 保存消息至 Postgres，支持同一会话续聊（`conversationId`）。
- 管理员可带 `debug=true` 获取 `retrievalDebug`。
- 关键参数：`CHAT_RERANK_TOPN`、`CHAT_CONTEXT_BUDGET`、`CHAT_MIN_EVIDENCE_COUNT`。

---

## 6. 核心数据模型

```
User
  id, email, role(user|admin), status(active|disabled), created_at

Book
  id, owner_id, title, status(queued|processing|ready|failed)
  primary_category, tags[], format(pdf|epub|txt), language(zh|en|other|unknown)
  minio_bucket, minio_key, original_filename, size_bytes
  error_message, created_at, updated_at, deleted_at(软删)

Chunk
  id, book_id, document_id, chunk_index, chunk_count
  content, content_sha256, content_runes
  source_type, source_ref, extract_method
  page, section, chunk, page_quality_score
  dense/sparse vectors stored in Qdrant

Message
  id, conversation_id, book_id, user_id
  role(user|assistant), content, citations[], abstained
  created_at

Conversation
  id, book_id, user_id, created_at

AuditLog
  id, actor_id, action, target_type, target_id, detail, created_at

EvalDataset / EvalRun  (在 Auth 服务管理)
  Postgres 持久化 + 文件存储（data/eval-center/）
  artifacts: run.json, metrics.json, per_query.jsonl, *_run.jsonl
```

---

## 7. 完整 API 参考

> 所有接口均经由 Gateway（`:8080`）对外暴露，完整规范见 `backend/api/rest/openapi.yaml`。

### 认证（无需登录）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/auth/signup` | 注册（需含大写/小写/数字/特殊字符，最少 12 位） |
| POST | `/api/auth/login` | 登录（返回 HttpOnly Cookie） |
| POST | `/api/auth/otp/send` | 发送 OTP |
| POST | `/api/auth/otp/verify` | 验证 OTP |
| POST | `/api/auth/password/reset/verify` | 忘记密码 — 验证码校验 |
| POST | `/api/auth/password/reset/complete` | 忘记密码 — 完成重置 |
| POST | `/api/auth/refresh` | 刷新 Access Token（依赖 Cookie，无需 Body） |
| POST | `/api/auth/logout` | 登出 |
| GET | `/api/auth/jwks` | 获取公钥 JWKS |
| GET | `/healthz` | 健康检查 |

### 用户（需登录）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/users/me` | 获取当前用户信息 |
| PATCH | `/api/users/me` | 更新邮箱 |
| POST | `/api/users/me/password` | 修改密码 |

### 书籍（需登录）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/books` | 上传书籍（`multipart/form-data`，字段 `file`；需 `Idempotency-Key` 请求头） |
| GET | `/api/books` | 书库列表（支持 `query/status/primaryCategory/tag/format/language` 筛选） |
| GET | `/api/books/{id}` | 查询单本书状态 |
| PATCH | `/api/books/{id}` | 更新书名/主分类/标签 |
| GET | `/api/books/{id}/download` | 获取预签名下载链接 |
| DELETE | `/api/books/{id}` | 删除书籍 |

### 对话（需登录，书籍须 `ready`）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/chats` | 发起问答（body: `bookId`, `question`, 可选 `conversationId`, `debug`） |
| GET | `/api/conversations` | 会话列表 |
| GET | `/api/conversations/{id}/messages` | 单会话消息列表 |

### 管理员（需 admin 角色）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/overview` | 系统概览指标 |
| GET | `/api/admin/users` | 用户分页列表（支持多维度筛选/排序） |
| GET | `/api/admin/users/{id}` | 查看单用户 |
| PATCH | `/api/admin/users/{id}` | 更新用户角色/状态 |
| POST | `/api/admin/users/{id}/disable` | 禁用用户 |
| POST | `/api/admin/users/{id}/enable` | 启用用户 |
| GET | `/api/admin/books` | 书籍分页列表（支持多维度筛选/排序） |
| DELETE | `/api/admin/books/{id}` | 删除书籍 |
| POST | `/api/admin/books/{id}/reprocess` | 重处理书籍（需 `Idempotency-Key`） |
| GET | `/api/admin/audit-logs` | 操作审计日志分页列表 |
| POST | `/api/admin/evals/runs` | 创建评测任务（需 `Idempotency-Key`） |

### 错误响应格式（统一）

```json
{
  "error": "面向用户的错误说明",
  "code": "CHAT_BOOK_NOT_READY",
  "requestId": "req_xxx",
  "details": [{ "field": "bookId", "reason": "book_status_conflict" }]
}
```

常见状态码：`400` 参数错误 · `401` 未登录 · `403` 权限不足 · `404` 不存在 · `409` 状态冲突 · `429` 限流 · `500` 内部错误 · `502` 下游不可用

> 若请求未携带 `X-Request-Id`，网关自动生成并在响应头和响应体返回。

---

## 8. 环境变量说明

复制 `.env.example` 为 `.env` 并填写。`run.sh` 会自动加载 `.env`。

| 变量 | 默认值（示例） | 说明 |
|---|---|---|
| `DATABASE_URL` | `postgres://onebook:onebook@localhost:5432/onebook?sslmode=disable` | Postgres 连接 |
| `REDIS_ADDR` | `localhost:6379` | Redis 地址 |
| `REDIS_PASSWORD` | `` | Redis 密码（为空则不鉴权） |
| `JWT_PRIVATE_KEY_PATH` | `secrets/jwt/private.pem` | RS256 私钥（`run.sh` 自动生成） |
| `JWT_PUBLIC_KEY_PATH` | `secrets/jwt/public.pem` | RS256 公钥 |
| `JWT_KEY_ID` | `jwt-active` | JWK kid |
| `JWT_ISSUER` | `onebook-auth` | JWT iss |
| `JWT_AUDIENCE` | `onebook-api` | JWT aud |
| `JWT_LEEWAY` | `30s` | 时钟偏差容忍 |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173,http://localhost:8086` | 允许的 CORS 来源（逗号分隔） |
| `CORS_ALLOW_CREDENTIALS` | `true` | 允许 Cookie 跨域 |
| `MINIO_ENDPOINT` | `localhost:9000` | MinIO 地址 |
| `MINIO_ACCESS_KEY` | `onebook` | MinIO 访问密钥 |
| `MINIO_SECRET_KEY` | `onebook123` | MinIO 密钥 |
| `MINIO_BUCKET` | `onebook` | MinIO Bucket |
| `MINIO_USE_SSL` | `false` | 是否使用 SSL |
| `ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH` | `secrets/internal-jwt/private.pem` | 内部服务 JWT 私钥 |
| `ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH` | `secrets/internal-jwt/public.pem` | 内部服务 JWT 公钥 |
| `ONEBOOK_INTERNAL_JWT_KEY_ID` | `internal-active` | 内部 JWK kid |
| `GENERATION_PROVIDER` | `gemini` | LLM Provider：`gemini` / `ollama` / `openai-compat` |
| `GENERATION_API_KEY` | — | Gemini API Key（provider=gemini 时必填） |
| `GENERATION_MODEL` | `gemini-2.5-flash` | 生成模型名 |
| `GENERATION_BASE_URL` | — | OpenAI 兼容 endpoint（provider=openai-compat 时填写） |
| `QDRANT_URL` | `http://localhost:6333` | Qdrant 地址 |
| `QDRANT_COLLECTION` | `onebook_chunks` | Qdrant Collection 名 |
| `ONEBOOK_EMBEDDING_DIM` | `3072` | Embedding 维度（与 Ollama 模型一致） |
| `OLLAMA_HOST` | `http://127.0.0.1:11434` | Ollama 地址 |
| `OLLAMA_EMBEDDING_MODEL` | `qwen3-embedding:latest` | Embedding 模型名 |
| `INGEST_CHUNK_SIZE` | `800` | 分块目标大小（runes） |
| `INGEST_CHUNK_OVERLAP` | `120` | 分块重叠大小（runes） |
| `INGEST_OCR_ENABLED` | `true` | 是否启用 OCR |
| `INGEST_OCR_SERVICE_URL` | `http://localhost:8087` | OCR Docker 服务地址 |
| `INGEST_OCR_DEVICE` | `cpu` | OCR 设备（cpu / gpu） |
| `INGEST_OCR_TIMEOUT_SECONDS` | `300` | OCR 超时（秒） |
| `INGEST_PDF_MIN_PAGE_RUNES` | `80` | PDF 低质量页判断阈值（字符数） |
| `INGEST_PDF_MIN_PAGE_SCORE` | `0.45` | PDF 低质量页判断阈值（质量分 0~1） |
| `INGEST_PDF_OCR_MIN_SCORE_DELTA` | `0.08` | OCR 相对 native 最小增益阈值 |
| `CHAT_RERANK_TOPN` | `8` | Rerank 后保留 TopN |
| `CHAT_CONTEXT_BUDGET` | `2200` | 上下文字数预算（runes） |
| `CHAT_MIN_EVIDENCE_COUNT` | `2` | 最少证据数（低于此数时拒答） |
| `LOGS_DIR` | `backend/logs` | 日志文件目录 |
| `AUTH_EVAL_STORAGE_DIR` | `data/eval-center` | Eval Center 文件存储目录 |
| `AUTH_EVAL_WORKER_POLL_INTERVAL` | `3s` | Eval Worker 轮询间隔 |

---

## 9. 本地开发快速上手

### 9.1 前置依赖

- Docker & Docker Compose
- Go 1.25+
- Node.js 20+ / npm
- Ollama（本地运行，已拉取 Embedding 模型，默认 `qwen3-embedding:latest`）

### 9.2 一键启动（推荐）

```bash
# 1. 复制环境变量并填写（至少填写 GENERATION_API_KEY）
cp .env.example .env

# 2. 启动基础设施 + 所有后端服务
# 如果本机可用 npm 且存在 frontend/package.json，run.sh 默认也会自动启动前端
./run.sh
# run.sh 顺序：按需启动前端 → 启动 Docker 依赖 → 生成 JWT 密钥 → 按 Auth→Book→Chat→Ingest→Indexer→Gateway 顺序启动
# 前端默认地址：http://localhost:5173

# 3. 如果只想启动后端，可显式关闭前端自动启动
START_FRONTEND=off ./run.sh

# 4. 如果前端未被自动拉起，再单独启动
cd frontend && npm install && npm run dev
# 访问 http://localhost:5173
```

### 9.3 手动逐服务启动

```bash
# 启动依赖
docker compose up -d postgres redis qdrant minio minio-init swagger-ui

# 各服务启动命令（在 backend/ 目录下）
cd backend/services/auth    && GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/auth
cd backend/services/book    && GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/book
cd backend/services/chat    && GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/chat
cd backend/services/ingest  && GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/ingest
cd backend/services/indexer && GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/indexer
cd backend/services/gateway && GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/server
```

### 9.4 单服务热重启

```bash
# 修改某服务代码后无需重启全部服务
./scripts/restart-service.sh <service>
# service 可选：auth | book | chat | ingest | indexer | gateway
```

### 9.5 服务端口

| 服务 | 端口 |
|---|---|
| Gateway | 8080 |
| Auth | 8081 |
| Book | 8082 |
| Chat | 8083 |
| Ingest | 8084 |
| Indexer | 8085 |
| Swagger UI | 8086 |
| OCR Service | 8087 |
| MinIO Console | 9001 |
| Qdrant Dashboard | 6333 |

### 9.6 测试与验证

```bash
# 后端单元测试
cd backend && go test ./...

# 前端 Lint + 构建
cd frontend && npm run lint && npm run build

# OpenAPI 规范校验
cd backend && go run ./cmd/check_openapi

# RAG 离线评测（一键脚本）
./scripts/run-rag-eval.sh
```

### 9.7 Docker 构建（服务镜像）

```bash
# 示例：构建 gateway 镜像
docker build -f backend/Dockerfile -t onebook-gateway \
  --build-arg SERVICE=gateway --build-arg CMD=server backend
```

### 9.8 接口文档

- Gateway REST/OpenAPI：`backend/api/rest/openapi.yaml`
- Internal REST/OpenAPI：`backend/api/rest/openapi-internal.yaml`
- Swagger UI（启动后）：`http://localhost:8086`

---

## 10. 前端说明

### 路由结构

| 路径 | 组件 | 说明 |
|---|---|---|
| `/` | `HomePage` | 首页 |
| `/log-in` 等 | `LoginPage` | 登录/注册/OTP/密码重置（多步骤，统一组件） |
| `/library` | `LibraryPage` | 书库管理 |
| `/chat`、`/chat/:conversationId` | `ChatPage` | 对话页面 |
| `/admin/overview` | `AdminOverviewPage` | 管理后台概览 |
| `/admin/users` | `AdminUsersPage` | 管理后台用户管理 |
| `/admin/books` | `AdminBooksPage` | 管理后台书籍管理 |
| `/admin/evals` | `AdminEvalsPage` | 管理后台 RAG 评测中心 |
| `/admin/audit` | `AdminAuditPage` | 管理后台审计日志 |

### 联调约定

- 所有请求统一发往 Gateway：`http://localhost:8080`。
- 使用 `withCredentials: true`（HttpOnly Cookie 认证，不手动注入 `Authorization` 头）。
- 收到 `401` 时触发 `POST /api/auth/refresh`（**Single-flight**：并发 401 只发一次 refresh），成功后自动重放原请求。
- 上传书籍后轮询 `GET /api/books/{id}`（建议每 2~3 秒）直到 `status` 为 `ready` 或 `failed`。
- 仅 `ready` 书籍可发起 `POST /api/chats`。
- `POST /api/books` 和 `POST /api/admin/books/{id}/reprocess` 强制要求请求头 `Idempotency-Key`。

---

## 11. 安全机制

### 认证

- **Access Token**：RS256 JWT，默认 15 分钟有效期，通过 Cookie（`onebook_access`）传递。
- **Refresh Token**：长期 Cookie（`onebook_refresh`），轮换策略（每次刷新发新 Token + 作废旧 Token）。
- **重放检测**：检测到旧 Refresh Token 重放后，撤销整个 token family，强制重新登录。
- **Redis 原子 CAS**：防止并发请求下同一 Refresh Token 双成功。

### 密码

- bcrypt 哈希，cost ≥ 12。
- 注册时密码至少 12 位，且需包含大写字母、小写字母、数字、特殊字符。

### 限流

- Gateway/Auth 使用 Redis 分布式固定窗口限流。
- Redis 不可用时默认拒绝请求（fail-closed 安全优先）。

### 内部服务通信

- 所有内部服务接口受短时效服务 JWT 保护（独立 `ONEBOOK_INTERNAL_JWT_*` 密钥对）。
- Gateway → 下游服务时注入服务 JWT（Bearer）。

### 管理员审计

- 管理员所有强操作（启停用户、删除书籍、重处理等）均记录审计日志。

---

## 12. 可观测性与日志

- 基于 `log/slog` JSON 结构化日志，字段含 `request_id`、`service`、`level`。
- 按状态码分级：5xx → Error，4xx → Warn，2xx/3xx → Info。
- 慢请求告警：≥ 5 秒请求记录 Warn 日志。
- 健康检查（`/healthz`）请求不写日志。
- 日志同时输出到 stdout 和文件（`backend/logs/<service>.log` + `backend/logs/all.log`）。
- Metrics / Tracing 尚未引入（待办）。

---

## 13. RAG 演进路线（Advanced RAG 蓝图）

完整蓝图见 `docs/architecture/advanced_rag_plan.md`。

### 当前阶段：M0（已完成）
- 文档清洗与 chunk 元数据规范落地。
- PDF 页级 OCR 触发与 native/OCR 融合机制。
- 基础测试覆盖。

### 下一阶段：M1（进行中）— 检索前增强
- Query normalize + 术语归一。
- 查询改写（可开关）+ 多查询召回。
- 结构优先分块升级 + 增量幂等重建。

### 后续里程碑

| 里程碑 | 目标 |
|---|---|
| M2 | 混合检索（Dense + BM25）+ 两阶段检索（rerank）+ 去重打包 |
| M3 | 证据约束生成模板 + 拒答策略 + 引用一致性校验 |
| M4 | 固定离线评测集 + CI 门禁 + 指标看板 |
| M5 | 特性开关灰度 + 成本预算 + 回滚预案 |

### KPI 目标（M3）

| 维度 | 指标 | 目标 |
|---|---|---|
| 检索 | Recall@20 | +10%（相对 M0） |
| 检索 | nDCG@10 | +10%（相对 M0） |
| 生成 | 引用命中率 | ≥ 90% |
| 生成 | 幻觉率 | ≤ 8% |
| 生成 | 拒答正确率 | ≥ 85% |
| 工程 | 问答 P95 延迟增幅 | ≤ 20% |

---

## 14. 开发规范（AI Agent 必读）

> 完整规范见 `AGENTS.md`，本节为摘要。

### 工作原则

1. **理解先于修改**：修改前先读懂现有行为，实现最小可行改动，不引入 Breaking Change。
2. **小步精准**：避免大范围重构，目标改动周边的逻辑要求基本不变。
3. **文档同步**：API 行为变更必须同步 OpenAPI 和 `docs/`，前端解析逻辑一并更新。

### 验证标准

| 改动范围 | 必须执行 |
|---|---|
| 后端代码 | `cd backend && go test ./...` |
| 前端代码 | `cd frontend && npm run lint && npm run build` |
| 任何失败 | 必须明确报告，不得静默忽略 |

### OpenAPI 更新规则

- 对外 API（Gateway 对前端）变更 → 更新 `backend/api/rest/openapi.yaml`
- 内部服务 API 变更 → 更新 `backend/api/rest/openapi-internal.yaml`
- 若两者均受影响，同步更新两个文件

### Commit 规范（Conventional Commits）

格式：`<type>(<scope>): <subject>`

推荐 scope：`frontend` · `auth` · `gateway` · `backend` · `api` · `docs` · `ci` · `infra` · `app`

| Type | 适用场景 |
|---|---|
| `feat` | 用户可感知的新能力或 API 行为变更/新增 |
| `fix` | Bug 修复（行为纠正） |
| `refactor` | 内部重构（行为不变） |
| `docs` | 仅文档变更 |
| `test` | 仅测试变更 |
| `chore` / `build` / `ci` / `perf` | 其他维护类 |

---

## 15. 待办事项

按优先级排序：

1. **可观测性**：Metrics / Tracing 引入，队列与索引进度监控。
2. **检索质量（M1~M3）**：查询改写、多查询召回、混合检索、重排、去重、拒答优化。
3. **安全与配额**：细粒度权限、密钥轮换自动化、用量配额管理。
4. **内容处理**：表格与公式高保真解析，多格式扩展。
5. **前端体验**：上传进度展示、失败重试 UI、任务进度看板。
6. **测试覆盖**：契约测试、回归测试集建设。
