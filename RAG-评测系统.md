# RAG 评测系统

## 1. 概述

本项目已落地一套本地可运行的 Advanced RAG 评测系统，采用 **文件优先、在线可选** 的执行策略。
Retrieval 现支持：

- `offline_approx`：本地近似 lexical + fallback rerank
- `online_real`：真实 OpenSearch BM25 + 本地 reranker 服务

当前首版覆盖 6 类评测：

1. Ingestion（数据清洗/抽取质量）
2. Chunking（切分质量）
3. Embedding（向量质量）
4. Retrieval（检索质量）
5. Post-Retrieval（去重与上下文覆盖）
6. Answer（回答与引用质量）

目标是形成可复现、可对比、可回归的本地评测闭环，为后续接入 CI 门禁打基础。

## 2. 代码位置

- CLI 入口：`backend/cmd/rag_eval/main.go`
- 评测核心：`backend/internal/eval/`
- 示例数据：`backend/internal/eval/testdata/`
- 一键脚本：`scripts/run-rag-eval.sh`
- 详细架构文档：`docs/architecture/rag_eval_system_plan.md`

## 3. 命令使用

在 `backend/` 下执行。

### 3.1 单模块评测

```bash
# 1) Ingestion
go run ./cmd/rag_eval ingest \
  --chunks internal/eval/testdata/chunks.jsonl

# 2) Chunking
go run ./cmd/rag_eval chunking \
  --chunks internal/eval/testdata/chunks.jsonl

# 3) Embedding（离线）
go run ./cmd/rag_eval embedding \
  --embeddings internal/eval/testdata/embeddings.jsonl

# 4) Retrieval（离线）
go run ./cmd/rag_eval retrieval \
  --queries internal/eval/testdata/queries.jsonl \
  --qrels internal/eval/testdata/qrels.tsv \
  --run internal/eval/testdata/run.jsonl

# 4.1) Retrieval（真实 OpenSearch + reranker）
go run ./cmd/rag_eval retrieval \
  --queries internal/eval/testdata/queries.jsonl \
  --qrels internal/eval/testdata/qrels.tsv \
  --chunks internal/eval/testdata/chunks.jsonl \
  --online \
  --lexical-mode online_real \
  --rerank-mode service

# 5) Post-Retrieval（离线）
go run ./cmd/rag_eval post-retrieval \
  --queries internal/eval/testdata/queries.jsonl \
  --qrels internal/eval/testdata/qrels.tsv \
  --run internal/eval/testdata/run.jsonl \
  --chunks internal/eval/testdata/chunks.jsonl

# 6) Answer
go run ./cmd/rag_eval answer \
  --queries internal/eval/testdata/queries.jsonl \
  --qrels internal/eval/testdata/qrels.tsv \
  --predictions internal/eval/testdata/predictions.jsonl
```

### 3.2 一次运行全量 6 类

```bash
go run ./cmd/rag_eval all \
  --chunks internal/eval/testdata/chunks.jsonl \
  --queries internal/eval/testdata/queries.jsonl \
  --qrels internal/eval/testdata/qrels.tsv \
  --predictions internal/eval/testdata/predictions.jsonl \
  --run internal/eval/testdata/run.jsonl \
  --embeddings internal/eval/testdata/embeddings.jsonl
```

或在仓库根目录：

```bash
./scripts/run-rag-eval.sh
```

## 4. 输入数据契约

### 4.1 chunks.jsonl

支持字段别名：

- `chunk_id | id`
- `doc_id | book_id`
- `text | content`
- `metadata | meta`

### 4.2 queries.jsonl

- `qid | id`
- `query | question`
- `book_id`（可选）
- `expected_answer`（可选）
- `expect_abstain`（可选）

### 4.3 qrels.tsv / qrels.jsonl

- TSV 支持：`qid iter doc_id relevance` 或 `qid doc_id relevance`
- 判定规则：`relevance > 0` 视为 relevant

### 4.4 run.jsonl

```json
{"qid":"q1","results":[{"doc_id":"c1","score":0.95}]}
```

### 4.5 predictions.jsonl

```json
{"qid":"q1","answer":"...","citations":["c1"],"abstained":false}
```

### 4.6 embeddings.jsonl

```json
{"id":"c1","vector":[0.1,0.2,0.3]}
```

## 5. 输出产物

每次运行输出目录包含：

1. `run.json`：运行快照（输入、参数、时间、commit）
2. `metrics.json`：汇总指标 + `gate_result`
3. `per_query.jsonl`：逐样本明细（可用于定位 bad case）

默认 gate 模式是 `warn`，只告警不阻断。
若在线构建 retrieval run，还会生成：

- `dense_run.jsonl`
- `lexical_run.jsonl`
- `fusion_run.jsonl`
- `rerank_run.jsonl`

## 6. 核心指标

### Ingestion

- `empty_rate`
- `duplicate_rate_exact`
- `metadata_missing_rate`
- `noise_marker_rate`

### Chunking

- `length_p50 / length_p95`
- `too_short_rate`
- `too_long_rate`
- `boundary_punct_ok_rate`

### Embedding

- `embed_success_rate`
- `dim_mismatch_rate`
- `empty_vector_rate`
- `norm_mean / norm_p50 / norm_p95`
- `latency_ms_mean / latency_ms_p95`（在线模式）

### Retrieval

- `Recall@5/10/20`
- `Hit@5/10/20`
- `MRR@10`
- `nDCG@10`

### Post-Retrieval

- `retrieved_dup_rate`
- `doc_diversity`
- `context_coverage`
- `context_budget_utilization`

### Answer

- `citation_hit_rate`
- `unsupported_citation_rate`
- `abstain_accuracy`
- `answer_nonempty_rate`
- `lexical_f1`

## 7. 在线可选模式

可通过 `--online` 开启在线计算（默认离线）。

- Embedding 在线：调用项目 Embedder（默认 `ollama`）生成向量
- Retrieval/Post-Retrieval 在线：未提供 `--run` 时先在线生成 run 再评测
- Retrieval 额外支持：
  - `--lexical-mode offline_approx | online_real`
  - `--rerank-mode fallback | service`

常用参数：

- `--embedding-provider`
- `--embedding-base-url`
- `--embedding-model`
- `--embedding-dim`
- `--opensearch-url`
- `--opensearch-index`
- `--reranker-url`

## 8. 当前状态与后续

当前已完成：

1. 6 类评测可独立运行
2. `all` 命令可一次产出完整报告
3. 提供 testdata、单测、集成测试与一键脚本

后续建议：

1. 接入固定基线版本对比（历史 metrics 对照）
2. 接入 CI 质量门禁（从 warn 过渡到 strict）
3. 增加 System 层指标（P95、失败率、成本）
4. 接入线上抽样回流评测
