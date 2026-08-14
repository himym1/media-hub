# Web 客户端

Media Hub 的 Web 客户端。首屏就是可用的搜索工作台，而不是落地页。

- React 19、TypeScript、Vite
- TanStack Query
- Lucide 图标
- 原生 DOM 控件，保留键盘与读屏支持

客户端与 Android 共用同一套 OpenAPI 语义，覆盖搜索、订阅、转存状态、事件历史、提供方设置、115 操作和 Emby 媒体库。会话使用 HttpOnly Cookie，并配合 CSRF 保护。

生产构建会嵌入 Go 服务镜像，与 `/api/v1` 同源提供。本地开发：

```bash
pnpm --dir web install --frozen-lockfile
pnpm --dir web test
pnpm --dir web lint
pnpm --dir web build
```
