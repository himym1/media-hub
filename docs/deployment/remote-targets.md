# 远程运维目标

仅在部署或远程运维任务中读取；以下为既有用途记录，操作前以当前部署配置与限定目标的只读预检核实，不读取配置秘密值。

- `himym`：Media Hub 生产 NAS 部署主机，承载 Docker、媒体数据和 Media Hub 服务。
- `dmit`：Media Hub 的 Mikan egress SSH 目标；`deploy/compose.mikan-egress.yaml` 通过 `MEDIA_HUB_MIKAN_EGRESS_SSH_HOST` 连接远端 `127.0.0.1:17898`，仅在启用 egress overlay 时使用。

同时确认 SSH alias、remote dir、目标服务和是否启用了 egress overlay；只有主机名不足以确定操作范围。部署、重启、真实 NAS/115/Emby 或数据库操作须有覆盖目标及影响的授权。不记录凭据、私钥、token、私密媒体信息或原始日志，不把生产入口当作本地验证目标。
