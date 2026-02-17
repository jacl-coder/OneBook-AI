# OneBook AI — 技术框架概览

## 栈概要（当前实现）
- 后端：Go（标准库 `net/http`）。
- 数据库：Postgres + pgvector（元数据与向量）。
- 存储：MinIO（S3 兼容对象存储）。
- 队列：Redis Streams（Ingest/Indexer），Redis（refresh token、撤销状态、分布式限流）。
- LLM：Gemini（回答生成）。
- Embedding：Gemini 或 Ollama（本地模型）。
- 解析：PDF 优先 `pdftotext`（可选），失败则使用 Go PDF 库；EPUB/HTML 解析；TXT 直接分块。
- 前端：React 19 + TypeScript + Vite 7 + React Router 7 + Axios + TanStack Query + Zustand + Tailwind v4。

## 核心流程
1) **上传**：Gateway → Book 服务；校验扩展名/大小后写入 MinIO，并写入书籍元数据。
2) **解析**：Ingest 通过内部接口拉取文件 → 解析与清洗 → 语义分块 → 写入 chunk（替换旧内容）。
3) **索引**：Indexer 拉取 chunks → 生成向量（支持批量/并发）→ 写入 pgvector → 更新书籍状态为 ready。
4) **对话**：Chat 生成问题向量 → TopK 检索 chunks → 拼装上下文与历史 → 调用 Gemini 生成回答 → 保存消息与引用。

## 核心数据模型
- **User**：账号、角色（user/admin）、状态（active/disabled）。
- **Book**：书籍元数据、状态（queued/processing/ready/failed）、文件位置、大小。
- **Chunk**：内容文本、向量、来源元数据（`source_type/source_ref` + `page/section/chunk`）。
- **Message**：按书存储对话历史。

## 可替换与扩展点
- Embedding 可切换为本地 Ollama 或云端（Gemini），维度可配置。
- LLM 目前仅接入 Gemini，接口预留可扩展其他模型。
- 存储层可替换为其他 S3 兼容对象存储；向量可迁移到专用向量数据库。

## 非功能现状与约束
- 可用性：具备任务重试与失败状态，但无统一监控与告警。
- 性能：已支持 embedding 批量与并发，仍需按机器性能调参。
- 可观测性：仅基础日志与 `/healthz`，缺少 metrics/tracing。
- 安全基线：用户 token 使用 RS256 + JWKS，本地验签；Gateway/Auth 限流依赖 Redis（安全优先，Redis 异常时拒绝请求）。
- 一致性基线：refresh token 轮换采用 Redis 原子 CAS；队列重试采用事务化 `XADD + XACK + XDEL`，降低并发与重试时的数据不一致风险。

## 前端联调入口
- 联调统一走 Gateway：`http://localhost:8080`
- 鉴权机制：浏览器会话 Cookie（`withCredentials`），401 走 refresh 单飞重试。
- 联调约定与错误语义：`docs/backend/backend_handoff.md`
