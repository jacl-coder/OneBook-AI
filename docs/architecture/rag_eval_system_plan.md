# OneBook AI — Advanced RAG 评测系统（6 类首版）

> 状态：已落地（本地运行版本，支持 `offline_approx` / `online_real`，未接 CI 门禁）

## 1. 范围

首版覆盖 6 类评测：

1. Ingestion
2. Chunking
3. Embedding
4. Retrieval
5. Post-Retrieval（去重 + 上下文覆盖）
6. Answer

默认采用“文件优先、在线可选”。

## 2. 命令入口

- 可执行入口：`backend/cmd/rag_eval/main.go`
- 常用命令：

```bash
cd backend

# 单模块
 go run ./cmd/rag_eval ingest --chunks internal/eval/testdata/chunks.jsonl
 go run ./cmd/rag_eval chunking --chunks internal/eval/testdata/chunks.jsonl
 go run ./cmd/rag_eval embedding --embeddings internal/eval/testdata/embeddings.jsonl
 go run ./cmd/rag_eval retrieval --queries internal/eval/testdata/queries.jsonl --qrels internal/eval/testdata/qrels.tsv --run internal/eval/testdata/run.jsonl
 go run ./cmd/rag_eval post-retrieval --queries internal/eval/testdata/queries.jsonl --qrels internal/eval/testdata/qrels.tsv --run internal/eval/testdata/run.jsonl --chunks internal/eval/testdata/chunks.jsonl
 go run ./cmd/rag_eval answer --queries internal/eval/testdata/queries.jsonl --qrels internal/eval/testdata/qrels.tsv --predictions internal/eval/testdata/predictions.jsonl

# 一次跑全量 6 类
go run ./cmd/rag_eval all \
  --chunks internal/eval/testdata/chunks.jsonl \
  --queries internal/eval/testdata/queries.jsonl \
  --qrels internal/eval/testdata/qrels.tsv \
  --predictions internal/eval/testdata/predictions.jsonl \
  --run internal/eval/testdata/run.jsonl \
  --embeddings internal/eval/testdata/embeddings.jsonl
```

一键脚本：`scripts/run-rag-eval.sh`。

## 3. 输入契约

### 3.1 `chunks.jsonl`

支持字段别名：
- ID：`chunk_id | id`
- 文档：`doc_id | book_id`
- 文本：`text | content`
- 元数据：`metadata | meta`

### 3.2 `queries.jsonl`

支持字段：
- `qid | id`
- `query | question`
- `book_id`（可选）
- `expected_answer`（可选）
- `expect_abstain`（可选）

### 3.3 `qrels.tsv` 或 `qrels.jsonl`

- TSV 支持：`qid iter doc_id relevance` 或 `qid doc_id relevance`
- 相关性判定：`relevance > 0` 视为 relevant

### 3.4 `run.jsonl`

每条 query 一条记录：

```json
{"qid":"q1","results":[{"doc_id":"c1","score":0.9}]}
```

### 3.5 `predictions.jsonl`

```json
{"qid":"q1","answer":"...","citations":["c1"],"abstained":false}
```

### 3.6 `embeddings.jsonl`

```json
{"id":"c1","vector":[0.1,0.2,0.3]}
```

## 4. 输出契约

每次运行输出目录包含：

1. `run.json`
2. `metrics.json`
3. `per_query.jsonl`

其中 `metrics.json` 含 `gate_result`（本地 warn 模式）。

当 Retrieval 在线构建 run 时，还会按阶段输出：
- `dense_run.jsonl`
- `lexical_run.jsonl`
- `fusion_run.jsonl`
- `rerank_run.jsonl`

## 5. 指标定义（首版）

### Ingestion
- `empty_rate`
- `duplicate_rate_exact`
- `metadata_missing_rate`
- `noise_marker_rate`

### Chunking
- `length_p50`
- `length_p95`
- `too_short_rate`
- `too_long_rate`
- `boundary_punct_ok_rate`

### Embedding
- `embed_success_rate`
- `dim_mismatch_rate`
- `empty_vector_rate`
- `norm_mean/p50/p95`
- `latency_ms_mean/p95`（在线）

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

## 6. 在线可选模式

- Embedding：`--online` 时调用项目现有 Embedder（默认 Ollama）生成向量。
- Retrieval/Post-Retrieval：`--online` 且未传 `--run` 时，先在线构造 run 再评测。
- Retrieval 额外支持两组模式：
  - `--lexical-mode offline_approx | online_real`
  - `--rerank-mode fallback | service`
- `online_real`：
  - lexical 直连 OpenSearch 构建真实 BM25 run
  - rerank 直连本地 reranker 服务
- `offline_approx`：
  - lexical 使用本地近似词法打分
  - rerank 使用本地 fallback reranker

常用 embedding 参数：
- `--embedding-provider`（默认 `ollama`）
- `--embedding-base-url`
- `--embedding-model`
- `--embedding-dim`

常用 retrieval 参数：
- `--opensearch-url`
- `--opensearch-index`
- `--reranker-url`

## 7. 当前限制

1. 首版未接入 CI 阻断。
2. 首版未接入 LLM Judge（RAGAs/TruLens）。
3. System 层评测（P95/失败率/成本）未纳入本版范围。
4. `online_real` 默认依赖本地 OpenSearch 和 reranker 服务，CI 里通常仍应使用 `offline_approx`。

## 8. 与 Advanced RAG 关系

该评测系统用于支撑 `docs/architecture/advanced_rag_plan.md` 的 M4（评测与发布门禁）落地基础，后续可在此基础上接入：

1. 固定离线评测集扩容
2. 指标基线版本对比
3. CI 阈值阻断
4. 线上抽样回流
