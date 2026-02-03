# 后端技术文档（简版）

目标：面向可商业化的服务拆分，以 Go 为主，支持后续扩展。

## 服务划分
- API Gateway/BFF：统一入口，鉴权校验，限流/熔断，路由到后端服务。
- Auth & User：注册/登录，JWT/Session，角色/权限，配额信息。
- Book Ingest：上传校验，文件存储（对象存储/本地），解析/OCR，分块，元数据写入；异步任务调度。
- Embedding/Index：生成向量，写向量索引（pgvector/向量库），重建/删除索引。
- Chat Orchestration：检索+重排序，提示拼装，调用 LLM，返回回答与出处。
- Admin/Ops：配置、监控、审计，任务看板；管理员查看用户与书籍状态。
- （可选）Billing/Quota：调用与存储计量，阈值告警/计费。

## 核心数据与接口
- 用户：账号、密码散列、角色（user/admin）、配额。
- 书籍：ID、所属用户、状态（排队/处理中/可对话/失败）、元数据、文件位置。
- 分块与向量：chunk 内容、向量、章节/页码元数据。
- 对话：按书维度的会话/消息，返回答案+出处。

## 运行与部署要点
- 所有请求经 API Gateway，携带 Bearer Token 进入各服务。
- 解析/嵌入当前为同步触发 + 后台 goroutine 处理（非持久队列）；状态可查询。
- 日志/指标/Tracing 尚未统一采集，已提供健康检查 `/healthz`。
- 内部服务通过 `X-Internal-Token` 保护（Book/Ingest/Indexer 内部接口）。

## 现状
- 仓库内已按 `backend/services/<service>/` 划分目录；Gateway 作为统一入口，服务通过 HTTP/JSON 通信。 
- Auth 服务：注册/登录/登出/用户自助/管理员用户管理，JWT 登出通过撤销列表生效。 
- Book 服务：上传/列表/查询/删除，文件写入 MinIO，对外由 Gateway 统一暴露。 
- Ingest 服务：拉取书籍文件，解析 PDF/EPUB/TXT，语义分块写入 Postgres（jsonb + pgvector）。 
- Indexer 服务：Embedding 可选 Ollama 或 Gemini，维度可配置；写入 pgvector 并回写状态。 
- Chat 服务：检索 topK chunks，拼装提示并调用 Gemini 生成回答，附出处。 
- Chunk metadata 已统一：`source_type/source_ref`，并保留 `page/section/chunk`。 
- 网关实现：路由涵盖 signup/login/logout/me、书籍 CRUD、聊天、admin 查询、healthz；元数据存储在 Postgres；对象存储使用 MinIO；向量检索使用 pgvector。
