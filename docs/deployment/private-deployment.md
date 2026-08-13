# Media Hub 私有部署

Media Hub 以一个镜像部署：Go API 同源提供 Web 静态资源，SQLite 和 Android release 文件保留在宿主机持久目录。

## 目录

```text
/volume1/docker/media-hub/
  compose.yaml
  .env                 # mode 0600, never commit
  data/media-hub.db
  releases/latest.json
  releases/media-hub-<versionCode>.apk
  backups/
```

容器只发布到 `127.0.0.1:18080`。DSM 反向代理将一个 HTTPS 域名转发到该端口；Web、API 和 APK 下载保持同源。

首次启动前，从源码 checkout 执行 `deploy/install-layout.sh /volume1/docker/media-hub`，或手工复制部署文件并创建目录。脚本会设置严格目录权限，并把 SQLite 目录分配给容器 UID/GID `65532`；只有 NAS 明确需要其他映射时才覆盖 `MEDIA_HUB_UID` 和 `MEDIA_HUB_GID`。

## 首次配置

1. 在 NAS 本地使用仅具备 `read:packages` 的 GitHub token 登录 GHCR：

   ```sh
   printf '%s' "$GHCR_READ_TOKEN" | docker login ghcr.io -u himym1 --password-stdin
   ```

2. 从 `backend/.env.example` 创建服务器本地 `.env`。生产至少设置：

   ```text
   MEDIA_HUB_SECURE_COOKIES=true
   MEDIA_HUB_ADMIN_PASSWORD=<one-time bootstrap password, 12+ bytes>
   MEDIA_HUB_DATA_ENCRYPTION_KEY=<base64 AES-256 key>
   MEDIA_HUB_ENABLE_FIXTURES=false
   ```

   密码只用于首次初始化；数据库已有管理员后可以从 `.env` 删除。加密密钥必须长期保留，否则无法恢复加密的 provider 凭据和 workflow payload。

   内置公开源需要独立代理时，在生产 `.env` 中启用可选覆盖文件；该设置只影响蜜柑、Sidhub 等内置源的 HTTP 客户端：

   ```text
   COMPOSE_FILE=compose.yaml:compose.mikan-egress.yaml
   MEDIA_HUB_SOURCE_PROXY_URL=http://source-egress:17898
   MEDIA_HUB_MIKAN_EGRESS_NETWORK=existing-egress-network
   ```

   `install-layout.sh` 会安装覆盖文件。`COMPOSE_FILE` 让发布、备份、恢复和重启都保留外部网络连接。代理必须是无凭据的 HTTP(S) URL；旧的 `MEDIA_HUB_MIKAN_PROXY_URL` 仍可作为兼容回退。不要把全局 `HTTP_PROXY` / `HTTPS_PROXY` 注入 Media Hub。

3. 先备份，再拉取固定版本：

   ```sh
   ./backup.sh
   MEDIA_HUB_IMAGE_TAG=0.9.0 docker compose pull
   MEDIA_HUB_IMAGE_TAG=0.9.0 docker compose up -d
   docker compose ps
   curl --fail http://127.0.0.1:18080/api/v1/health
   ```

不要使用 `latest`。升级时先记录旧 tag 并备份 SQLite。回滚应用镜像可以恢复旧 tag；如果新版本执行了不向后兼容的迁移，使用 `restore.sh` 恢复对应数据库快照。

## Android release

GitHub tag workflow 生成签名 APK 和 `latest.json`。将两者放入 `releases/`，文件名保持 `media-hub-<versionCode>.apk`。App 使用 Bearer 会话下载，验证声明的大小和 SHA-256 后交给 Android Package Installer；系统仍要求用户确认，并需要为 Media Hub 授予“安装未知应用”权限。

Android 签名密钥只保存在 GitHub Actions secrets 和离线备份中，不能提交到仓库或放入 Docker 镜像。所有后续 APK 必须使用同一签名密钥。

## 验收

- 先验证登录、Web/API 同源、115 授权状态和只读 provider 状态。
- 再验证真实搜索（蜜柑/Sidhub 内置或已部署的合同适配器）、单次转存、QMediaSync、Emby 可播放性、订阅调度和未知态恢复。
