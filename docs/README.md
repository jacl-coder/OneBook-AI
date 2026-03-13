# Docs 分类索引

## architecture

- [`advanced_rag_plan.md`](architecture/advanced_rag_plan.md)：Advanced RAG 实施蓝图（目标、里程碑、KPI、执行清单）
- [`rag_eval_system_plan.md`](architecture/rag_eval_system_plan.md)：RAG 评测系统（6 类指标、`offline_approx` / `online_real`、输入/输出契约）
- [`tech_overview.md`](architecture/tech_overview.md)：架构决策说明（PostgreSQL/OpenSearch/Qdrant 职责边界、非功能约束、安全/一致性基线）

## backend

- [`backend_handoff.md`](backend/backend_handoff.md)：前后端联调交接文档（联调边界、认证约定、主要接口、检索调试与评测参数）
- [`auth_account_flow.md`](backend/auth_account_flow.md)：账号流程（登录/注册/OTP/密码重置），含 Mermaid 流程图
- [`api_response_standard.md`](backend/api_response_standard.md)：API 响应规范（错误结构、完整错误码表、SSE 协议草案）
- [`backend_arch.md`](backend/backend_arch.md)：后端实现细节（鉴权机制、索引一致性、混合检索链路、队列一致性）
- [`backend/api/rest/openapi.yaml`](../backend/api/rest/openapi.yaml)：对外接口 OpenAPI 规范（含管理员接口）
- [`backend/api/rest/openapi-internal.yaml`](../backend/api/rest/openapi-internal.yaml)：内部服务接口规范

## frontend

- [`frontend_development_workflow.md`](frontend/frontend_development_workflow.md)：前端开发流程、技术栈与当前实践约定
- [`svg_usage_guideline.md`](frontend/svg_usage_guideline.md)：SVG 使用规范（sprite 选型、样式与可访问性）
- [`frontend/README.md`](../frontend/README.md)：前端本地开发与测试命令

## product

- [`requirements.md`](product/requirements.md)：高层需求说明
- [`functional_spec.md`](product/functional_spec.md)：功能规格与验收标准
