# Media Hub Agent Rules

## Product Boundary

- Media Hub is a single-user, single-NAS media automation control plane.
- The repository contains one backend, one Web client, one Android client, and may add a Windows desktop client when native decode is required.
- Media payloads must never be proxied through Media Hub. Playback remains 115 CDN to player.
- Media Hub owns STRM generation and `/115/url/` 302 playback. Emby owns library management and playback APIs.
- PT search and download are delegated to MoviePilot. Hub does not store PT site cookies, talk to trackers, or become a second downloader. Local NAS upload remains outside the MVP.

## Architecture

- Keep the backend a Go modular monolith until measured pressure justifies a split.
- Keep one OpenAPI contract under `api/`; Web and Android clients consume the same semantics.
- Persist workflow state before executing external side effects.
- Every external operation must be timeout-bounded, retryable when safe, and idempotent.
- Never delete cloud or Emby media as a side effect of a failed workflow.

## Android

- Phone client is Android. Do not add iOS, Flutter, or Kotlin Multiplatform source sets.
- Use Kotlin, Jetpack Compose, and pinned stable dependencies.
- Material 3 is the presentation system. Business screens use project-owned `MediaHub*` wrappers instead of importing Material 3 components directly.
- Use unidirectional data flow, ViewModel, StateFlow, repositories, and explicit UI states.

## Desktop

- Windows desktop is allowed when the browser cannot decode the library stream (DTS / TrueHD / similar).
- Prefer Tauri wrapping the existing Web UI. Use native playback (mpv / FFmpeg or OS decoders) only for the player surface.
- Do not add Electron, or a second desktop UI stack, unless Tauri is proven insufficient.
- Desktop still consumes the same OpenAPI contract. Do not proxy media bytes through Media Hub.

## Web

- The first screen is the usable search experience, not a landing page.
- Use React, TypeScript, Vite, TanStack Query, and Lucide icons.
- Use native DOM controls and preserve keyboard and screen-reader support.
- Avoid generic admin dashboards, nested cards, decorative gradients, and feature-tour copy.

## Security

- Never commit secrets, cookies, tokens, passwords, private media names, or NAS exports.
- User-entered provider credentials stay server-side and encrypted at rest before real integrations ship.
- Logs must redact query credentials, share codes, direct URLs, and media-provider cookies.

## Verification

- Coding tasks authorize necessary test maintenance, focused low-risk local tests and analysis, and needed local builds. Inspect script/configuration side effects first; documentation, prompt, and comment-only changes normally need only readback and diff review.
- Database/container integration tests or setup, NAS/115/Emby access, real accounts/data, device installation, deployment, and full or substantially costly checks require explicit authorization. Ordinary checks use synthetic/mock data and must not contact live services.
- Choose the smallest relevant checks for the affected area:
  - Backend: run focused Go tests from `backend/`.
  - Web: run `pnpm build` and relevant tests from `web/`.
  - Android: run `./gradlew :app:assembleDebug` and focused unit/UI tests from `android/`.
  - Desktop Rust: run `cargo test --manifest-path desktop/src-tauri/Cargo.toml` from the repository root.
- API contract changes must keep client models consistent and buildable; run the smallest relevant local checks within the limits above.
- Report checks actually run and checks not run. Do not broaden or repeat successful checks without new changes, failures, or unresolved concerns.

## 远程 SSH 目标

- 仅在部署或远程运维任务中读取 [远程运维目标](docs/deployment/remote-targets.md)。操作前同时确认 SSH alias、remote dir、服务归属、egress overlay 状态及授权范围；文档不授予执行权限，不读取或记录秘密值。

## 注释语言

- 新增或修改注释（含文档注释）默认简体中文，保留必要技术术语、标识符、命令、协议字段与工具固定注释；仅按需调整相关代码的注释，不批量翻译。用户或外部规范另有要求时从其要求。

## Git 提交说明

- 提交使用 Conventional Commits：`<type>(<scope>): <中文描述>`；scope 用实际模块英文名，无明确模块时省略。type 限 `feat`、`fix`、`refactor`、`perf`、`docs`、`test`、`build`、`ci`、`chore`、`revert`。标题准确描述差异，保留必要技术术语，不用泛称或 emoji。
- 简单改动只写标题；复杂改动说明原因、主要变化及兼容性。破坏性变更用 `!` 和正文 `BREAKING CHANGE:` 说明影响与迁移方式。
- 仅约束后续提交，不改写历史；Git 自动生成的合并/撤销说明可保留。提交、推送仍需相应授权。
