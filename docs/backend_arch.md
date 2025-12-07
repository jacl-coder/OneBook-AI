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
- 解析/嵌入使用队列异步处理，状态可查询。
- 日志/指标/Tracing 统一采集，支持健康检查 `/healthz`。

## 现状
- 仓库内已按 `backend/services/<service>/` 划分目录；当前网关/单体逻辑在 `backend/services/gateway/`，其他服务为占位入口，后续逐步拆分实现。 
- 网关实现：路由涵盖 signup/login/logout/me、书籍 CRUD、聊天、admin 查询、healthz；元数据存储在 Postgres，文件存储本地，会话 token 使用 JWT（HMAC，含 TTL，登出立即失效），向量检索尚未接入。
