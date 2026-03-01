# Gateway to Microservices — 现状与演进

本项目已完成核心微服务拆分，Gateway 作为统一入口。此文档用于记录当前落地情况与后续演进方向。

## 已完成（现状）
- **Gateway**：统一入口、鉴权校验、路由到 Auth/Book/Chat，提供 admin 查询与 healthz。
- **Auth**：注册/登录/登出、用户自助、管理员用户管理；RS256 JWT + JWKS，本地验签，Redis 管理 refresh token 与撤销状态。
- **Book**：书籍上传/列表/查询/删除，文件存储 MinIO，下载返回预签名 URL。
- **Ingest**：解析 PDF/EPUB/TXT → 语义分块 → 写入 chunks（带来源元数据）。
- **Indexer**：Embedding（Ollama）、批量/并发写入 pgvector，更新书籍状态。
- **Chat**：向量检索 + Gemini 生成回答，保存消息历史。
- **基础设施**：Postgres + pgvector、Redis、MinIO；CI（go test）与通用 Dockerfile。

## 下一步演进方向
1) **可观测性**：统一日志、metrics、tracing；队列与索引进度监控。
2) **检索与回答质量**：重排、去重、上下文裁剪与提示模板优化。
3) **安全与治理**：刷新令牌、速率限制、配额管理、多租户支持。
4) **管理与运维**：任务看板、失败重试 UI、数据修复工具。
5) **前端体验**：上传进度、状态刷新、对话 UI 与引用展示。

## 配置演进
- 当前配置为各服务独立 `config.yaml` + 环境变量。
- 未来可引入统一配置管理与密钥托管。

## 数据与迁移
- 当前数据按服务职责维护；后续关注跨服务一致性与审计。
