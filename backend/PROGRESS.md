# 开发进度（后端）

## 已完成
- 微服务拆分：Gateway/Auth/Book/Ingest/Indexer/Chat 可独立运行。
- Gateway：统一入口、鉴权校验、admin 查询、/healthz。
- Auth：注册/登录/登出、用户自助、管理员用户管理；JWT 或 Redis 会话；撤销列表生效。
- Book：上传/列表/查询/删除；MinIO 存储；下载预签名 URL（文件名为原始文件名）。
- Ingest：PDF/EPUB/TXT 解析；PDF 优先 `pdftotext`，失败回退 Go PDF；语义分块。
- Ingest：支持可选 PaddleOCR 作为扫描版 PDF 提取；按页质量评估融合 native/OCR 结果。
- Chunk metadata 统一：`source_type/source_ref`，并保留 `page/section/chunk`；新增 `document_id/chunk_index/chunk_count/content_sha256/content_runes` 用于检索前治理与追溯。
- Indexer：Embedding 可选 Gemini/Ollama；支持批量/并发写入 pgvector；状态更新。
- Chat：向量检索 + Gemini 生成回答，附出处；消息入库并拼接最近 N 轮历史。
- 任务队列：Redis Streams 持久队列，支持重试与失败回写。
- 上传限制：网关/Book 扩展名白名单与大小限制（默认 50MB）。
- 工具与脚本：`run.sh` 一键启动；`cmd/bench_embed` 基准测试。
- 文档与规范：OpenAPI（Gateway/Internal）、Swagger UI、通用 Dockerfile、CI（go test）。

## 待办（按优先级）
1) **可观测性**：metrics/tracing、队列与索引进度监控、日志统一。
2) **检索质量**：重排、去重、上下文裁剪、提示模板优化。
3) **安全与配额**：刷新令牌、速率限制、配额管理、多租户策略。
4) **内容处理增强**：OCR/图片 PDF、表格与公式更高保真解析。
5) **管理与前端**：任务看板、失败重试 UI、书库/对话前端。
6) **接口与测试**：契约测试、回归测试、gRPC/proto 规划。

## 备注
- 默认使用本地 Ollama 作为 embedding（可切换 Gemini）。
