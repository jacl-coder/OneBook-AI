# OneBook AI 前端技术栈选型说明

## 1. 选型目标
- 快速完成毕业设计可演示版本（开发效率优先）。
- 保证核心链路稳定：登录、上传、状态轮询、对话、历史。
- 便于后续扩展与维护（类型安全、可测试、可迭代）。

## 2. 推荐技术栈

## 2.1 核心框架
- `React + TypeScript`
- 原因：
  - React 组件化适合页面拆分与复用（登录、书库、对话等模块）。
  - TypeScript 可减少前后端联调时字段与状态错误。

## 2.2 构建工具
- `Vite`
- 原因：
  - 冷启动快、热更新快，适合高频迭代。
  - 配置相对简单，适合课程项目节奏。

## 2.3 路由
- `React Router`
- 原因：
  - 可清晰管理页面路径与鉴权页面跳转。
  - 易实现未登录重定向、登录后回跳。

## 2.4 服务端状态管理（接口数据）
- `TanStack Query`
- 原因：
  - 适合本项目的接口特性：缓存、重试、失效刷新、轮询。
  - 书籍状态轮询（`queued/processing/ready/failed`）可直接用 query 机制实现。

## 2.5 客户端状态管理（本地 UI/会话状态）
- `Zustand`
- 原因：
  - 轻量、上手快，适合管理用户会话和少量全局 UI 状态。
  - 比重型状态库更贴合毕业设计规模。

## 2.6 UI 与样式
- `Tailwind CSS`（可选搭配 `shadcn/ui`）
- 原因：
  - 开发效率高，样式一致性好。
  - 响应式实现成本低，适合桌面+移动端适配。

## 2.7 表单与校验
- `React Hook Form + Zod`
- 原因：
  - 登录/注册/上传等表单开发快且稳定。
  - 校验规则集中管理，错误提示一致。

## 2.8 网络请求层
- `Axios`（或原生 `fetch` 封装）
- 原因：
  - 易实现统一会话请求：`withCredentials`、`401 -> refresh -> retry`。
  - 统一错误处理，减少页面重复代码。

## 2.9 测试体系
- 单元/组件：`Vitest + Testing Library`
- 端到端：`Playwright`
- 原因：
  - 可覆盖答辩核心链路：登录、上传、轮询、问答、刷新 token。

## 3. 与后端契约的关键对齐点
- 网关基址：`http://localhost:8080`
- 会话机制：登录返回 `user`；鉴权由 HttpOnly Cookie 承载，401 时走刷新再重试。
- 上传：`multipart/form-data`，字段名固定 `file`。
- 书籍状态：前端轮询 `GET /api/books/{id}`，到 `ready` 才允许问答。
- 统一错误结构：`{ "error": "..." }`

## 4. 为什么这套最适合本项目
- 学习曲线可控：主流、资料多、社区成熟。
- 交付效率高：能先稳定完成主流程，再逐步增强体验。
- 质量可保障：类型系统 + 测试体系可支撑答辩演示稳定性。

## 5. 可选替代方案（不作为首选）
- `Vue 3 + Pinia`：也可行，但当前项目文档与后续协作更偏 React 生态。
- `Next.js`：适合 SSR/SEO 场景；本项目是应用型内页，SSR收益有限。
- `Redux Toolkit`：功能强，但相对重，不是当前规模的最优解。

## 6. 最终建议
- 按以下组合落地：
  - `React + TypeScript + Vite`
  - `React Router + TanStack Query + Zustand`
  - `Tailwind CSS + React Hook Form + Zod`
  - `Axios`
  - `Vitest + Testing Library + Playwright`

这套方案在“开发速度、稳定性、可维护性、答辩展示效果”之间平衡最佳。

## 7. 答辩可用严谨表述
本项目采用 `React + TypeScript + Vite` 为核心的现代前端工程栈，结合 `TanStack Query`（数据获取与缓存）、`Zustand`（轻量全局状态）、`Tailwind + shadcn/ui`（统一 UI 组件与样式）、`React Hook Form + Zod`（表单与校验）以及 `Vitest/Playwright`（测试）构成一套在业界广泛验证的主流方案，兼顾快速迭代与后期可维护性。
