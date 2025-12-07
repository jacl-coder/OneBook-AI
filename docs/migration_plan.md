# Gateway to Microservices Migration Plan

当前 `backend/services/gateway` 同时承载鉴权、书籍、对话等核心逻辑，方便 MVP 快速跑通。未来拆分时建议按以下边界演进。

## 现状（单体网关）
- 鉴权：注册/登录/退出，JWT（HMAC）发放与校验，Redis 可选会话存储。
- 用户管理：`/api/users/me` 基于 JWT 解析用户。
- 书籍：上传/列表/查询/删除，文件落盘，状态模拟。
- 对话：占位问答。
- 管理：用户/书籍列表。
- 存储：Postgres（元数据）、文件系统（书文件）、Redis（会话 token）。

## 拆分目标
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

## 网关定位（拆分后）
- 只做入口：认证校验、限流、熔断、聚合 BFF。
- 不再持久化业务数据；日志/追踪统一。
- 可用 API Gateway 或 Service Mesh 替代。

## 渐进式步骤
1) 抽象接口：在 Gateway 内为 Auth/Book/Chat 定义 client 接口，先用当前内存/本地实现。
2) 服务化 Auth：将用户/会话逻辑搬到 `services/auth`，Gateway 通过 RPC/REST 调 Auth 获取用户、校验 token。
3) 服务化 Book：拆出文件存储与元数据 CRUD，Gateway 调用 Book 服务。
4) 服务化 Ingest/Index/Chat：将处理/检索/对话逻辑独立，Gateway 仅聚合。
5) 基础设施：统一配置管理、服务发现、链路追踪、指标与告警。

## 配置演进
- 现状：`gateway/config.yaml` 持有 DB/Redis/JWT 等配置。
- 目标：各服务拥有独立配置；Gateway 只需上游 Auth 公钥/地址、下游服务地址/超时/重试策略。

## 数据与迁移
- 用户与会话：从 Gateway 的 Postgres/Redis 迁移到 Auth 专用库。
- 书籍与文件：迁移到 Book 服务的存储，确保文件路径/权限兼容。
- 消息/向量：随 Chat/Index 服务迁移，保持 ID/引用一致性。

## 测试与发布
- 添加契约测试（Gateway↔Auth/Book/Chat）。
- 逐路由切换到新服务（功能旗标/灰度）。
- 回滚策略：保留旧逻辑一段时间，双写/双读验证。
