# 开发进度（后端）

## 已完成
- 目录重构为微服务式布局：`backend/services/<service>/`，Auth/Book/Ingest/Indexer/Chat/Gateway 服务可独立运行。
- Gateway 负责对外 API 聚合；Auth/Book/Chat/Ingest/Indexer 通过 HTTP/JSON 协作。
- Auth：用户注册/登录、管理员用户管理；JWT 登出通过撤销列表生效。
- Book：书籍上传/列表/查询/删除，对接 MinIO。
- Ingest/Indexer：解析 PDF/EPUB/文本 → 语义分块 → 生成向量 → 写库并更新书籍状态。
- Chat：检索向量召回 + 生成回答（Gemini），消息落库并返回引用。
- Embedding 支持切换 Gemini/Ollama（本地模型），维度可配置；已修复维度不一致问题。
- Chunk metadata 统一结构：`source_type/source_ref`，同时保留 `page/section/chunk`；旧数据自动回填。
- 本地基准工具：`backend/cmd/bench_embed` 支持批量/并发/分块测速（支持 EPUB 解析）。
- 运行说明与结构已更新至仓库 `README.md` 和 `docs/backend_arch.md`。
- 初步契约与共享包：`api/rest/openapi.yaml`（草稿），`api/grpc/` 说明；共享 `pkg/domain` 与 `pkg/auth`（密码散列）。

## 待办（按优先级）
1) **接口契约**：在 `api/` 目录定义 REST/OpenAPI 与 gRPC proto（对外与内网接口）。
2) **分层落地**：按 handler/service/repo 拆分 gateway 逻辑；抽离 shared 包（domain/config/auth/logger/observability）。
3) **存储与鉴权强化**：JWT/刷新 token、配额/速率限制。
4) **异步与索引**：为 ingest/indexer 接入队列（Kafka/NATS/Redis Streams），实现重试、幂等与索引重建流程。
5) **性能优化**：embedding 批量/并发、chunk 参数调优、重复文本去重与缓存。
6) **聊天编排**：检索重排、引用过滤与更精细的提示模板。
7) **可观测性与 CI**: 统一日志/metrics/tracing，健康/就绪探针，Makefile + golangci-lint + CI 流水线。
8) **部署脚本**：Dockerfile、docker-compose/k8s manifests，配置样例 `.env.example`。

## 备注
- 当前通过 `run.sh` 可一键启动依赖与全部后端服务；默认使用本地 Ollama 作为 embedding（可切换 Gemini）。
- Ingest/Indexer 任务队列为内存实现，重启会丢失任务。
