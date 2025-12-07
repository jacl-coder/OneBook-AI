# 开发进度（后端）

## 已完成
- 目录重构为微服务式布局：`backend/services/<service>/`，当前逻辑在 `gateway`，其他服务入口占位。
- Gateway（单体示例）：用户注册/登录（首个用户为 admin）、书籍上传/列表/删除、占位问答、管理员查看用户/书籍，状态模拟排队→处理中→可对话。
- 运行说明与结构已更新至仓库 `README.md` 和 `docs/backend_arch.md`。
- 初步契约与共享包：`api/rest/openapi.yaml`（草稿），`api/grpc/` 说明；共享 `pkg/domain` 与 `pkg/auth`（密码散列）。

## 待办（按优先级）
1) **接口契约**：在 `api/` 目录定义 REST/OpenAPI 与 gRPC proto（对外与内网接口）。
2) **分层落地**：按 handler/service/repo 拆分 gateway 逻辑；抽离 shared 包（domain/config/auth/logger/observability）。
3) **存储与鉴权强化**：接入 Postgres/pgvector（替换内存存储）、JWT/刷新 token、配额/速率限制。
4) **异步与索引**：为 ingest/indexer 服务接入队列（Kafka/NATS/Redis Streams），实现解析/嵌入/索引重建流程。
5) **聊天编排**：chat 服务实现检索+重排+提示拼装+LLM 调用，返回引用。
6) **可观测性与 CI**: 统一日志/metrics/tracing，健康/就绪探针，Makefile + golangci-lint + CI 流水线。
7) **部署脚本**：Dockerfile、docker-compose/k8s manifests，配置样例 `.env.example`。

## 备注
- 现有代码可在 `backend/services/gateway/` 运行，满足基本上传/问答占位链路；其他服务待实现。 
