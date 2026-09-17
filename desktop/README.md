# Media Hub Desktop

Tauri 2 shell around the existing Web UI. Playback hands the stream to **mpv**, which opens its own window and keeps its own fullscreen, seek bar and track menus. Hub only shows a small panel and records watch progress.

The window, installer, and taskbar use `src-tauri/icons/` (dark tile + green film/play mark). Regenerate with `pnpm --dir desktop exec tauri icon src-tauri/icons/app-icon-source.png`.

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

On a version tag, the same workflow copies the NSIS installer to the NAS `releases/` directory as `media-hub-<versionCode>.exe` plus `desktop-latest.json`. The packaged app checks that feed after login and can download, verify SHA-256, and run a silent current-user install.

## macOS disk image

`.github/workflows/desktop-macos.yml` builds a `.dmg` on `macos-latest`. A version tag also writes `media-hub-<versionCode>.dmg` and `desktop-darwin-latest.json` to the NAS `releases/` directory. From 0.21.21 the Mac app copies `Media Hub.app` into `/Applications` and strips `com.apple.quarantine` on the installed bundle. Older shells still only download the DMG.

The DMG is unsigned unless Apple notarization secrets are added later. macOS reports an unsigned internet download as “Media Hub is damaged” if the copied app still has quarantine. If Finder blocks a dragged install, run `xattr -cr "/Applications/Media Hub.app"` and open the app, or right-click → Open.

## Scope

- Packaged installers are Windows NSIS and macOS DMG. `tauri dev` works on the current host OS.
- Do not add Electron.
