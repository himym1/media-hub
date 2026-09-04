# Media Hub Desktop

Tauri 2 shell around the existing Web UI. Playback uses **mpv** so DTS / TrueHD can have sound.

## Prerequisites

- Rust (`rustc` 1.77+)
- [mpv](https://mpv.io/) on `PATH` (`brew install mpv` on macOS)
- Node / pnpm for the Tauri CLI

## Run

```bash
pnpm --dir desktop install
pnpm --dir desktop tauri dev
```

The window loads `https://media.himym.us.ci`. Native play requires the Web hook shipped in **v0.20.25+**.

## Scope

- Windows is the intended packaged target; `tauri dev` works on the current host OS.
- Media Hub never proxies video bytes. mpv opens the descriptor HTTPS URL directly.
- Do not add Electron.
