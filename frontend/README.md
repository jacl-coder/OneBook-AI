# OneBook AI Frontend

## 1. 技术栈（当前）
- React 19 + TypeScript
- Vite 7
- React Router 7
- Axios + `withCredentials`
- TanStack Query
- Zustand
- Tailwind CSS v4

## 2. 启动方式
```bash
cd frontend
npm install
npm run dev
```

默认地址：`http://localhost:5173`

## 3. 环境变量
复制并修改 `frontend/.env.example`：

```env
VITE_API_BASE_URL=http://localhost:8080
VITE_API_TIMEOUT_MS=15000
```

## 4. 常用命令
```bash
# 代码检查
npm run lint

# 生产构建
npm run build

# 单元测试
npm run test:unit

# 本地预览打包产物
npm run preview
```

## 5. 目录说明
- `src/app/`：路由、Provider
- `src/pages/`：页面实现
- `src/shared/`：共享配置与 HTTP 客户端
- `src/styles/`：全局基础样式与动画

## 6. 认证与请求约定
- 受保护接口走 Cookie 会话，不在前端拼接 Bearer token
- axios 拦截器支持 `401 -> refresh -> retry`
- refresh 使用 single-flight，避免并发风暴

## 7. 参考文档
- `/Users/jacl/Documents/Code/Golang/OneBook-AI/docs/frontend/frontend_tech_stack.md`
- `/Users/jacl/Documents/Code/Golang/OneBook-AI/docs/frontend/frontend_development_workflow.md`
- `/Users/jacl/Documents/Code/Golang/OneBook-AI/docs/backend/backend_handoff.md`
