# gRPC API (draft)

当前内部服务通信使用 HTTP/JSON（详见 `backend/api/rest/openapi-internal.yaml`）。

如需引入 gRPC：
- 在此目录新增 `.proto` 文件（建议 packages：auth、book、ingest、indexer、chat、admin）。
- 补充 codegen 脚本（例如 `scripts/gen_proto.sh` 或 Makefile 任务）。
