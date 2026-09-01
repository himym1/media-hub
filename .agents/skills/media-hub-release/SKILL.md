---
name: media-hub-release
description: >-
  Commit, tag, package, and deploy Media Hub. Use when the user says
  提交打包部署, 发版, release Media Hub, deploy media hub, 发布 0.x, 更新 NAS,
  or asks why NAS deploys hang. Do not create a second release skill.
---

# Media Hub release

Follow `.grok/skills/media-hub-release/SKILL.md` as the source of truth.

The long wait is NAS `docker build` → `go mod download`, not missing process docs. Use Aliyun GOPROXY and Clash `HTTP_PROXY` when `:7890` is up. Never rewrite the module `RUN`. APK is built before the image so a hung build does not block the app update.
