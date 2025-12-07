# OneBook AI

面向个人/小团队的“书本对话”应用：用户上传电子书，系统解析并基于书本内容进行对话式问答（附出处），支持书库管理和会话历史。

## 当前状态
- 需求与功能规格：见 `docs/requirements.md` 与 `docs/functional_spec.md`
- 技术框架概览：见 `docs/tech_overview.md`
- 后端基础骨架已添加（上传/列表/删除书籍，占位对话接口，内存存储）。

## 目标功能（MVP）
- 上传 PDF/EPUB/TXT，处理状态可视（排队/处理中/可对话/失败）。
- 书库列表与删除；按书发起对话。
- 基于书本内容回答，并附章节/页码/段落出处；会话保留最近若干轮上下文。

## 技术栈（规划）
- 后端：Go（Gin/Fiber/Echo 皆可）
- 数据库：Postgres + pgvector
- 前端：React（Vite/Next.js 皆可）
- 存储：本地文件系统（后续可换对象存储）；简易 JWT/Session 鉴权

## 后端（MVP 骨架）
- 位置：`backend/services/gateway/`（单体入口；后续可拆到多服务）
- 路由（均需 Bearer token，除认证与健康检查）：
  - 认证：`POST /api/auth/signup`，`POST /api/auth/login` 返回 `token`；`GET /api/users/me`
  - 书籍：`/api/books`（POST 上传字段 `file`，GET 列表），`/api/books/{id}`（GET/DELETE）
  - 对话：`POST /api/chats`（body: bookId + question）
  - 管理员：`GET /api/admin/users`，`GET /api/admin/books`
  - 健康：`/healthz`
- 状态流转：上传后会模拟排队→处理中→可对话。
- 存储：元数据保存在内存，文件落盘 `DATA_DIR`（默认 `./data`）。
- 微服务目录（占位入口，便于后续拆分，按服务分文件夹）：`backend/services/gateway/`（BFF/当前单体实现）、`backend/services/auth/`、`backend/services/ingest/`、`backend/services/indexer/`、`backend/services/chat/`、`backend/services/admin/`（后续填充）。

### 本地运行
```bash
cd backend/services/gateway
GOCACHE=$(pwd)/../../.cache/go-build go run ./cmd/server
# 环境变量：PORT（默认 8080），DATA_DIR（默认 ./data）

# 示例：注册并调用（需替换 token）
# curl -X POST http://localhost:8080/api/auth/signup -d '{"email":"user@example.com","password":"secret"}'
# curl -H "Authorization: Bearer <token>" http://localhost:8080/api/users/me
# curl -H "Authorization: Bearer <token>" -F "file=@/path/book.pdf" http://localhost:8080/api/books
```

## 后续步骤（建议）
1) 接入真实解析/分块/向量索引与检索模型。
2) 持久化元数据到 Postgres/pgvector，替换内存存储。
3) 完善鉴权、配额、错误路径与前端对接。
