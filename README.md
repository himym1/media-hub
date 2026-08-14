# Media Hub

Media Hub 是一套自托管控制面：搜索影视资源、转存到 115、经 QMediaSync 生成 STRM，并在 Emby 中建库索引。

本仓库包含：

- `backend/`：Go API、SQLite 持久化、资源源适配器，以及工作流执行器。
- `web/`：React、TypeScript、Vite、TanStack Query 客户端。
- `android/`：原生 Kotlin、Jetpack Compose，以及项目自有的 `MediaHub*` UI 封装。
- `api/`：Web 与 Android 共用的 OpenAPI 契约。
- `docs/deployment/private-deployment.md`：GHCR、NAS Compose、备份/恢复，以及 Android 发版流程。

Media Hub 从不代理媒体本体。播放始终走 115 CDN 到播放器的直连路径。

## 已实现流程

```text
已认证搜索
  -> TMDB 身份核验
  -> 加密的资源版本选择
  -> 经资源源适配器幂等转存到 115
  -> QMediaSync 手动同步并轮询记录
  -> Emby 刷新媒体库，并按 TMDB 匹配条目
  -> Emby 播放信息核验
  -> 企业微信应用会话通知
```

转存任务与只追加事件会在对外产生副作用之前先落库。源引用、适配器操作 ID、115 文件 ID 和提供方路径只保存在 AES-256-GCM 不透明载荷中。安全操作会按有界退避重试。QMediaSync 提交结果不确定时会进入 `needs_attention`，且不会自动重放。

Web 与 Android 暴露同一套搜索、转存状态、事件历史、显式重试、提供方状态和 Emby 媒体库语义。Web 使用 HttpOnly 会话 Cookie 加 CSRF 保护；Android 使用可撤销的 Bearer 会话，并通过 Android Keystore 存储。

原生订阅会持久化电影、剧集、季和集的进度，并支持源选择、版本偏好、持久调度、去重抑制、手动运行、批量暂停/恢复，以及原子备份导入/导出。原生 115 操作提供加密 PKCE 授权、有界浏览、持久文件命令、拒绝符号链接的本地上传，以及 Web/Android 上先审后执行的归档计划。SubX 兼容层已移除。原生适配器内置蜜柑与 Sidhub，其他源走 HTTP 适配器契约。

TMDB 趋势与推荐为原生实现。115 支持加密 PKCE 设备授权与自动刷新，以及按用户隔离的目录浏览和持久文件命令。创建文件夹、移动、重命名和删除会先以加密参数落库，再提交给提供方；删除与结果不确定的重试必须用命令 ID 显式确认。

## 配置

后端把环境变量当作启动基线，并执行内嵌的 SQLite 迁移。已认证的 Web 与 Android 设置会把加密后的运行时覆盖写入 SQLite，无需重启即可生效；设置 API 从不回传密钥。密钥只留在进程内存中，不得写入日志或提交到仓库。

```text
MEDIA_HUB_ADDR=:8080
MEDIA_HUB_DATABASE_PATH=data/media-hub.db
MEDIA_HUB_PROBE_TIMEOUT=3s
MEDIA_HUB_SOURCE_PROXY_URL=<optional-http-proxy-for-built-in-sources>
MEDIA_HUB_SECURE_COOKIES=true
MEDIA_HUB_ENABLE_FIXTURES=false

MEDIA_HUB_ADMIN_PASSWORD=<initial-password-at-least-12-bytes>
MEDIA_HUB_DATA_ENCRYPTION_KEY=<base64-encoded-32-byte-key>

MEDIA_HUB_TMDB_ACCESS_TOKEN=<tmdb-read-access-token>
MEDIA_HUB_TMDB_URL=https://api.themoviedb.org/3

MEDIA_HUB_115_ACCESS_TOKEN=<115-open-api-token>
MEDIA_HUB_115_CLIENT_ID=<115-open-client-id>
MEDIA_HUB_115_MOVIE_DESTINATION_ID=<directory-id>
MEDIA_HUB_115_SERIES_DESTINATION_ID=<directory-id>
MEDIA_HUB_LOCAL_UPLOAD_ROOTS=/mnt/media/incoming,/mnt/media/staging

MEDIA_HUB_QMS_URL=<qmediasync-base-url>
MEDIA_HUB_QMS_API_KEY=<qmediasync-api-key>
MEDIA_HUB_QMS_ACCOUNT_ID=<qmediasync-account-id>
MEDIA_HUB_QMS_MOVIE_TARGET_PATH=<strm-target-path>
MEDIA_HUB_QMS_SERIES_TARGET_PATH=<strm-target-path>

MEDIA_HUB_EMBY_URL=<emby-base-url>
MEDIA_HUB_EMBY_API_KEY=<emby-api-key>
MEDIA_HUB_EMBY_USER_ID=<emby-user-id>
MEDIA_HUB_EMBY_MOVIE_LIBRARY_ID=<library-id>
MEDIA_HUB_EMBY_SERIES_LIBRARY_ID=<library-id>

MEDIA_HUB_SOURCE_DIAN_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_DIAN_TOKEN=<adapter-token>
MEDIA_HUB_SOURCE_FRAMEHDR_URL=<optional-official-base-url>
MEDIA_HUB_SOURCE_FRAMEHDR_ACCOUNT=<username>
MEDIA_HUB_SOURCE_FRAMEHDR_TOKEN=[REDACTED:Generic Password Field]
MEDIA_HUB_SOURCE_GIMY_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_GIMY_TOKEN=<adapter-token>
MEDIA_HUB_SOURCE_GUANYING_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_GUANYING_TOKEN=<adapter-token>
MEDIA_HUB_SOURCE_HDHIVE_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_HDHIVE_TOKEN=<adapter-token>
MEDIA_HUB_SOURCE_JUYING_URL=<optional-official-base-url>
MEDIA_HUB_SOURCE_JUYING_AUTH_MODE=<web-or-developer>
MEDIA_HUB_SOURCE_JUYING_ACCOUNT=<username-or-app-id>
MEDIA_HUB_SOURCE_JUYING_TOKEN=[REDACTED:Generic Password Field]
MEDIA_HUB_SOURCE_MIKAN_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_MIKAN_TOKEN=<adapter-token>
MEDIA_HUB_SOURCE_SIDHUB_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_SIDHUB_TOKEN=<adapter-token>

MEDIA_HUB_WECOM_CORP_ID=<enterprise-id>
MEDIA_HUB_WECOM_SECRET=<application-secret>
MEDIA_HUB_WECOM_SEND_MODE=app
MEDIA_HUB_WECOM_AGENT_ID=<application-agent-id>
MEDIA_HUB_WECOM_TO_USER=@all
MEDIA_HUB_WECOM_CHAT_ID=
MEDIA_HUB_WECOM_URL=https://qyapi.weixin.qq.com
```

集成 URL 必须是不带内嵌凭据、查询参数或片段的绝对 HTTP(S) 地址。只配置了 TMDB token、未设置 `MEDIA_HUB_TMDB_URL` 时，使用官方 API 地址。

资源适配器实现统一的[搜索与转存契约](docs/integrations/source-adapter.md)。蜜柑与 Sidhub 是内置匿名适配器。FrameHDR 与聚影是内置账号适配器，官方 URL 可留空。FrameHDR 的 `ACCOUNT`/`TOKEN` 是用户名/密码。聚影 `AUTH_MODE=web` 使用用户名/密码，`AUTH_MODE=developer` 使用 App ID/API Key；未设置 `AUTH_MODE` 的历史聚影凭据仍按开发者模式处理。`MEDIA_HUB_SOURCE_PROXY_URL` 可选地只把内置源的 HTTP 客户端走无认证 HTTP(S) 代理；旧的 `MEDIA_HUB_MIKAN_PROXY_URL` 仍可作为回退。这两项都不影响 115、TMDB、QMediaSync、Emby、企业微信或契约适配器。其他源使用 HTTP 适配器契约。夹具搜索数据仅在显式设置 `MEDIA_HUB_ENABLE_FIXTURES=true` 时可用，且从不执行转存。

企业微信支持通过普通自建应用的 `message/send` API，用 Agent ID 和 ToUser 做 `app` 投递；也兼容用 Chat ID 的历史 `appchat` 投递。优先使用加密的提供方设置，而不是环境变量，并用固定的测试通知动作验收已保存配置。提交结果未知时不会自动重试。

## 本地验证

```bash
# 后端
go -C backend test ./...
go -C backend test -race ./...
go -C backend vet ./...

# Web
pnpm --dir web install --frozen-lockfile
pnpm --dir web test
pnpm --dir web lint
pnpm --dir web build

# Android
cd android
./gradlew :app:testDebugUnitTest :app:assembleDebug :app:lintDebug
```

## 私有部署

Release tag 会把多架构私有镜像发布到 `ghcr.io/himym1/media-hub`。镜像从同一源提供 Web 客户端和 `/api/v1`。NAS 目录布局、GHCR 登录、SQLite 备份/恢复、TLS 和 Android 发版见[私有部署](docs/deployment/private-deployment.md)。

不要提交凭据、Cookie、API Key、`.env` 文件、NAS 导出、SQLite 数据库或生成的 APK。

另见[路线图](docs/development/roadmap.md)和[系统设计](docs/architecture/system-design.md)。
