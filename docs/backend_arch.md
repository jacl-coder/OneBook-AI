# 后端架构说明（当前实现）

## 服务划分（已落地）
- **Gateway**：统一入口与鉴权校验，路由到 Auth/Book/Chat；提供 admin 查询与 healthz。
- **Auth**：注册/登录/登出、用户自助、管理员用户管理；支持 JWT 或 Redis 会话，JWT 登出通过撤销列表生效。
- **Book**：书籍元数据管理、上传校验、预签名下载 URL、删除书籍；文件存储在 MinIO。
- **Ingest**：拉取文件、解析 PDF/EPUB/TXT、清洗与语义分块、写入 chunks，失败回写状态。
- **Indexer**：Embedding 生成（Gemini/Ollama）、向量写入 pgvector、更新书籍状态。
- **Chat**：向量检索 + Gemini 生成回答，返回出处，并保存历史消息。

## 数据与依赖
- **Postgres + pgvector**：用户/书籍/消息/chunk/向量。
- **MinIO**：书籍文件存储（S3 兼容）。
- **Redis**：
  - Session/Token revocation（Auth）。
  - Redis Streams 持久队列（Ingest/Indexer）。

## 核心调用链路
1) Gateway → Book：上传文件，写 MinIO 与书籍元数据，入队 Ingest。
2) Ingest：解析文件 → 分块 → 写入 chunks → 触发 Indexer。
3) Indexer：批量/并发生成向量 → 写入 pgvector → 状态更新为 ready。
4) Chat：问题向量检索 TopK → 拼装上下文与历史 → Gemini 回答 → 保存消息。

## 安全与权限
- 对外接口通过 Gateway 统一鉴权（Bearer Token）。
- 内部服务接口通过 `X-Internal-Token` 保护。
- 管理员角色可查看全量用户/书籍数据。

## 运行与运维
- 服务均提供 `/healthz` 健康检查。
- Swagger UI 可查看 REST/OpenAPI 规范。
- 目前仅基础日志，尚未引入 metrics/tracing。

## 仍待完善的方向
- 统一可观测性（日志、metrics、tracing）。
- 检索质量优化（重排、去重、提示模板）。
- 任务与索引进度可视化、失败重试 UI。
- 安全与配额（刷新令牌、速率限制、配额管理）。
