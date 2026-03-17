# 开发进度（后端）

## 已完成

- 微服务拆分：Gateway/Auth/Book/Ingest/Indexer/Chat 可独立运行。
- Gateway：统一入口、鉴权校验、admin 查询、/healthz。
- Auth：注册/登录/登出、用户自助、管理员用户管理；JWT 或 Redis 会话；撤销列表生效。
- Book：上传/列表/查询/删除；MinIO 存储；下载预签名 URL（文件名为原始文件名）。
- Ingest：PDF/EPUB/TXT 解析；PDF 优先 `pdftotext`，失败回退 Go PDF；语义分块。
- Ingest：支持可选 Docker OCR 服务（`services/ocr/`，PaddleOCR HTTP）作为扫描版 PDF 提取；按页质量评估融合 native/OCR 结果；OCR 服务通过 `INGEST_OCR_SERVICE_URL` 配置，降级支持本地 CLI。
- Ingest：双 tier retrieval chunk（`lexical` / `semantic`）已落地，支持 `chunk_family` 去重键与统一 chunk 元数据。
- Chunk metadata 统一：`source_type/source_ref`，并保留 `page/section/chunk`；新增 `document_id/chunk_index/chunk_count/content_sha256/content_runes` 用于检索前治理与追溯。
- Indexer：Embedding 使用 Ollama；支持批量/并发写入 Qdrant；状态更新。
- Chat：`dense + lexical + fusion + rerank + context pack` 混合检索主链已落地；支持 `retrievalDebug` 输出。
- Chat：已接入 query normalize、query variants、模型驱动 query rewrite、多查询召回、基础 query route（history-only / out-of-scope reject）。
- Chat：LLM（TextGenerator，支持 Gemini/Ollama/OpenAI 兼容）生成回答，附出处；消息入库并拼接最近 N 轮历史。
- Chat：证据约束 prompt、证据不足拒答、基础 groundedness 校验已落地。
- 任务队列：Redis Streams 持久队列，支持重试与失败回写。
- 上传限制：网关/Book 扩展名白名单与大小限制（默认 50MB）。
- 工具与脚本：`run.sh` 一键启动；`cmd/bench_embed` 基准测试。
- 文档与规范：OpenAPI（Gateway/Internal）、Swagger UI、通用 Dockerfile、CI（go test）。
- 评测：`cmd/rag_eval` 离线评测 CLI、固定测试数据、一键脚本 `scripts/run-rag-eval.sh` 已可运行。
- 结构化日志：基于 `log/slog` JSON 输出，支持上下文传播（`request_id`/`service`）、按状态码分级、慢请求告警（≥5s）、健康检查排除；日志同时写入 stdout 和文件（按服务 + 汇总 `all`，目录 `backend/logs/`）。
- TextGenerator 接口：抽象 LLM 调用，已支持 Gemini、Ollama、OpenAI 兼容三种 provider，通过配置切换。
- 超时链路：前端 120s → Gateway WriteTimeout 150s → Chat 内部客户端 120s → LLM provider 120s。
- Gemini Embedding 移除：Indexer 统一使用 Ollama Embedding。

## 待办（按优先级）

1. **可观测性增强**：metrics/tracing、队列与索引进度监控。
2. **Advanced RAG 收尾**：术语归一、metadata filter、query route 深化与检索参数治理。
3. **索引治理**：增量重建、索引修复、幂等重跑，避免长期依赖整书替换。
4. **评测门禁**：把 `rag_eval` 指标阈值接入 CI，补版本对比与失败回归报告。
5. **安全与配额**：刷新令牌、速率限制、配额管理、多租户策略。
6. **内容处理增强**：OCR/图片 PDF、表格与公式更高保真解析。
7. **管理与前端**：任务看板、失败重试 UI、书库/对话前端。
8. **接口与测试**：契约测试、回归测试、gRPC/proto 规划。

## 备注

- 默认使用本地 Ollama 作为 embedding。
- LLM 回答生成通过 `TextGenerator` 接口切换 provider（环境变量前缀 `GENERATION_*`）。
