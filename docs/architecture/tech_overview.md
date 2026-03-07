# OneBook AI — 架构决策说明

> 本文补充 README 未详细说明的架构选型理由与约束。核心流程、技术栈、端口请见根目录 README。

## 可替换与扩展点

- **Embedding**：Ollama 本地模型，维度可配（`ONEBOOK_EMBEDDING_DIM`）。可替换为任何兼容 HTTP API 的 Embedding 服务。
- **LLM**：`TextGenerator` 接口抽象，已实现 Gemini、Ollama、OpenAI 兼容三种 provider，通过 `GENERATION_PROVIDER` 切换。
- **向量存储**：当前使用 Qdrant；接口隔离于 `pkg/retrieval`，可按需迁移。
- **对象存储**：MinIO（S3 兼容），可替换为任何 S3 兼容服务。

## 非功能约束与现状

### 可观测性
- 结构化 JSON 日志（`log/slog`），字段含 `request_id`、`service`、`level`。
- 按状态码分级：5xx → Error，4xx → Warn，2xx → Info。
- 慢请求告警（≥ 5s）、healthz 排除日志。
- 日志同时写 stdout 和文件（`backend/logs/<service>.log` + `all.log`）。
- **待引入**：Metrics / Tracing。

### 安全基线
- 用户 Access Token：RS256 JWT，通过 JWKS 由各服务本地验签（不依赖中心化验签）。
- 内部服务接口：独立密钥对签发的短时效服务 JWT（`ONEBOOK_INTERNAL_JWT_*`），校验 `iss/aud/exp`。
- 限流：Gateway/Auth 使用 Redis 分布式固定窗口；Redis 异常时默认拒绝请求（fail-closed）。

### 一致性基线
- **Refresh Token 并发安全**：Redis 原子 CAS，防止并发请求下同一 token 双成功。
- **队列重试一致性**：失败重试在单事务内执行 `XADD + XACK + XDEL`，避免已确认未重投的丢任务窗口。

## 关联文档

- RAG 演进蓝图：`docs/architecture/advanced_rag_plan.md`
- RAG 评测系统：`docs/architecture/rag_eval_system_plan.md`
- 前后端联调：`docs/backend/backend_handoff.md`
