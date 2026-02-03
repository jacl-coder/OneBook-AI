# Gateway to Microservices Migration Plan

当前项目已完成基础服务拆分，Gateway 作为入口/BFF。此文档转为记录已完成拆分与后续演进方向。

## 现状（已服务化）
- Gateway：统一入口，路由到 Auth/Book/Chat，提供 admin 查询与 healthz。
- Auth：注册/登录/登出/用户自助/管理员用户管理，JWT 撤销列表生效。
- Book：书籍上传/列表/查询/删除，文件存储在 MinIO。
- Ingest：解析 PDF/EPUB/TXT，语义分块写入 Postgres。
- Indexer：Embedding 可选 Ollama 或 Gemini，写入 pgvector 并更新书籍状态。
- Chat：向量检索 + Gemini 生成回答，消息入库。
- 存储：Postgres（元数据/向量/消息）、MinIO（书文件）、Redis（会话或撤销列表）。

## 后续演进目标
1) **Auth Service**
   - 责任：注册/登录、令牌签发与刷新、登出、第三方登录、密码管理、风控/审计。
   - 对外接口：`/auth/signup|login|logout|refresh|introspect`（REST/gRPC）。
   - 数据：用户表、会话/刷新令牌表，审计日志。
   - Gateway 行为：仅校验/透传 token，调用 Auth 获取用户信息与权限。

2) **Book Service**
   - 责任：书籍元数据、上传校验、文件存储、处理状态。
   - 数据：书籍表、文件存储（对象存储/本地），处理任务表。
   - Gateway 行为：BFF，调用 Book 服务获取/管理书籍。

3) **Ingest/Index Service**
   - 责任：解析/OCR、分块、嵌入生成、向量索引管理。
   - 数据：chunk/embedding/索引元数据，向量存储（pgvector/向量库）。
   - Gateway 行为：触发/查询处理状态。

4) **Chat Service**
   - 责任：检索+LLM 调用，返回答案与出处，消息留存。
   - 数据：会话/消息表，检索依赖向量存储。
   - Gateway 行为：路由调用、聚合返回。

5) **Admin/Ops Service（可选）**
   - 责任：用户/书籍/任务监控，配置管理，审计导出。

## 网关定位（当前）
- 只做入口：认证校验、限流、熔断、聚合 BFF。
- 不再持久化业务数据；日志/追踪统一。
- 可用 API Gateway 或 Service Mesh 替代。

## 下一步推进
1) 增加可观测性（metrics/tracing/统一日志）。
2) 增强性能（embedding 批量/并发、chunk 参数调优）。
3) 引入配额/速率限制与多租户治理能力。

## 配置演进
- 现状：各服务拥有独立配置，Gateway 只需下游服务地址与鉴权配置。
- 目标：统一配置管理、密钥托管与配置热更新。

## 数据与迁移
- 已完成拆分，数据按服务职责维护；后续关注跨服务一致性与审计。

## 测试与发布
- 添加契约测试（Gateway↔Auth/Book/Chat）。
- 逐路由切换到新服务（功能旗标/灰度）。
- 回滚策略：保留旧逻辑一段时间，双写/双读验证。
