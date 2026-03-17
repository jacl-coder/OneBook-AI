# 后端架构说明（实现细节）

> 本文补充 README 未详细说明的安全、一致性与可靠性实现细节。服务划分、端口、启动方式请见根目录 README。

## 安全与权限（细节）

### 对外鉴权
- 所有 `/api/*` 接口经 Gateway 统一鉴权，浏览器通过 HttpOnly Cookie 传递 Access Token。
- 前端**不注入** `Authorization` 头，鉴权完全由 Cookie 承载。
- 业务服务（Auth/Book/Chat）通过 JWKS 端点（`GET /api/auth/jwks`）本地验签，不依赖中心化调用。

### 内部服务通信
- Gateway → 下游服务注入独立密钥对签发的服务 JWT（Bearer），校验 `iss/aud/exp`。
- 密钥对独立于用户 JWT（`ONEBOOK_INTERNAL_JWT_*`），可独立轮换。

### Refresh Token 安全机制
- 每次刷新后旧 Token 立即失效（轮换策略）。
- Redis 原子 CAS：防止并发 refresh 请求中同一 Token 双成功。
- 旧 Token 重放检测：检测到旧 Token 被重用后，撤销整个 token family，强制重新登录。

### 管理员权限
- 管理员可查看全量用户/书籍数据，执行用户启停、书籍删除/重处理等强操作。
- 所有强操作自动记录审计日志（actor / action / target / detail），支持按维度追溯。

## 一致性与可靠性（细节）

### 队列重试一致性
- Ingest/Indexer 使用 Kafka `at-least-once` 交付，任务状态/去重/尝试次数统一落 Postgres。
- 失败时先更新任务状态，再按重试策略重投主 topic；超过最大重试次数后写入 DLQ。

### 检索与索引一致性
- 检索架构固定为：
  - PostgreSQL：正文与引用事实来源
  - OpenSearch：BM25 lexical retrieval
  - Qdrant：semantic retrieval
  - Application Layer：fusion + rerank + 回表
- Chat 检索增强当前支持独立开关：
  - `CHAT_QUERY_REWRITE_ENABLED`
  - `CHAT_MULTI_QUERY_ENABLED`
  - `CHAT_ABSTAIN_ENABLED`
- Ingest 写入/更新 chunk 后，会把 `chunk_index_status` 两路状态重置为 `pending`。
- Indexer 成功写入 OpenSearch/Qdrant 后，分别更新 `opensearch_status` / `qdrant_status` 为 `synced`；失败时记录 `last_error`。
- Chat 与 Eval 命中索引后，不直接信任索引层正文；最终上下文统一按 `chunk_id` 回 PostgreSQL 获取。

### Python 模型服务边界
- OCR 服务独立部署为 `ocr-service`（Docker），供 ingest 在扫描版 PDF 场景调用。
- Reranker 服务独立部署为 `reranker-service`（Docker），供 chat 与 eval 的 `service` 模式调用。
- 两个服务都通过 HTTP 调用接入 Go 主链路，模型缓存通过 Docker volume 持久化。

### 书籍删除一致性
- 软删标记 → 后台异步清理 → 最终硬删。
- 已软删记录不出现在用户接口（列表/详情）中。
- Ingest/Indexer 队列按 `job_type + resource_id` 去重，重复 reprocess 不堆积多条进行中任务。

## 仍待完善的方向

- 可观测性：Metrics / Tracing（结构化日志已落地）。
- 检索质量：更细粒度 query rewrite、术语归一、query route 深化、索引修复后台入口、更严格的 reranker 容量治理（见 `docs/architecture/advanced_rag_plan.md`）。
- 任务进度可视化与失败重试 UI。
- 安全与配额：细粒度权限、密钥轮换自动化、配额管理。
