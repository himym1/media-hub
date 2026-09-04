# Media Hub Desktop

Tauri 2 shell around the existing Web UI. Playback uses **mpv** so DTS / TrueHD can have sound.

## Prerequisites

- Rust (`rustc` 1.77+)
- [mpv](https://mpv.io/) on `PATH` or a common install location
  - macOS: `brew install mpv`
  - Windows: install [mpv](https://mpv.io/installation/) so `mpv.exe` is on PATH (`winget search mpv`)
- Node / pnpm for the Tauri CLI

The desktop app looks for `mpv` on `PATH`, then Homebrew (`/opt/homebrew/bin`), WinGet links, Scoop, Chocolatey, and `C:\Program Files\mpv`.

## Run

```bash
pnpm --dir desktop install
pnpm --dir desktop tauri dev
```

`devUrl`, `frontendDist`, and the window `url` all point at `https://media.himym.us.ci` so `tauri dev` and the packaged app load the live Web UI, not a local `index.html`. Native play requires the Web hook shipped in **v0.20.25+**.

## Windows installer

GitHub Actions workflow `.github/workflows/desktop-windows.yml` builds NSIS (`.exe`) and WiX (`.msi`) on `windows-latest`.

- Push to `main` that touches `desktop/` uploads artifacts
- Tag `v*.*.*` also attaches the installer to that GitHub Release when the workflow creates or updates it
- Manual: **Actions → desktop-windows → Run workflow**

After install, keep `mpv.exe` available. Media Hub does not proxy video bytes; mpv opens the descriptor HTTPS URL directly.

## Scope

- Windows is the intended packaged target; `tauri dev` works on the current host OS.
- Do not add Electron.
