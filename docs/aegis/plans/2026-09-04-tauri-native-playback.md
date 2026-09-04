# Tauri Native Playback Implementation Plan

Date: `2026-09-04`
Status: executable
Spec: conversation + `AGENTS.md` Desktop section + `.cursor/rules/desktop-tauri.mdc`

## Goal

Let the user play **own-library** streams from a Media Hub desktop shell with **native decode** (mpv), so DTS / TrueHD can have sound. Browser `<video>` stays the fallback when Tauri is not present.

## Architecture

- **UI owner:** existing Web (`web/`). No second React app.
- **Desktop shell:** new `desktop/` Tauri 2 project. Webview loads the production origin (`https://media.himym.us.ci`) so login cookies stay same-origin.
- **Play owner:** Rust command `play_native` spawns `mpv` with the already-minted descriptor `streamUrl`. Media Hub still does not proxy bytes.
- **Web hook:** `LibraryPlayer` detects Tauri and `invoke('play_native', …)` instead of assigning `video.src`. Browser builds ignore the hook.

```text
LibraryPlayer -> POST /playback/descriptors/emby -> streamUrl
  browser: <video src>
  Tauri:   invoke play_native -> mpv -- {url}
```

## Tech Stack

Tauri 2 (Rust), existing React/Vite Web, mpv on PATH, Vitest for the Web hook. No Electron. No OpenAPI change in this slice.

## Baseline/Authority Refs

- `AGENTS.md` Desktop + media-byte rule
- `.cursor/rules/desktop-tauri.mdc`
- `docs/product/vision.md` (Windows via Tauri when native decode is required)
- `docs/architecture/adr/0006-emby-managed-media3-playback.md` (no byte proxy)
- `docs/aegis/baseline/2026-09-01-initial-baseline.md` (updated: Tauri allowed)
- `docs/aegis/specs/2026-09-01-web-library-player-design.md` (Web player exists; this plan adds a desktop decode path)

## Compatibility Boundary

- Browser Web behavior unchanged when `__TAURI_INTERNALS__` is absent
- No `streamUrl` in location bar, logs, screenshots, or Rust `log!` of the full URL
- Shared Emby items may use the same hook later; this slice does not special-case them
- Android unchanged
- Release of the `.msi` / `.exe` is a follow-up; first verify on the host OS (`darwin`) with `tauri dev`

## TDD Route

```text
TDD Route:
- Mode: off
- Decision: skipped
- Strict authority: not applicable
- Strict signals: new distribution surface, behavior (recorded, not used to force TDD)
- Light eligibility: no
- TDD-fit exception: tdd_mode=off
- Test posture: post-change regression
- Reason: user-local Aegis tdd_mode is off
- Verification: pnpm web test/lint/quality/build + e2e; cargo check in desktop/
```

## Verification

```bash
pnpm --dir web test
pnpm --dir web lint
pnpm --dir web quality
pnpm --dir web build
pnpm --dir web test:e2e
cd desktop && cargo check
```

Host smoke (manual): `pnpm --dir desktop tauri dev`, log in, play a silent-in-Chrome title, confirm mpv has audio.

```text
Aegis Visibility: This is a new desktop distribution surface and a second playback owner (mpv). Planning keeps Web/Android/OpenAPI stable and avoids Electron or a second UI stack.

Plan Basis: user “开始做” after changing product rules to allow Tauri native playback.

BaselineUsageDraft:
- Required baseline refs: AGENTS.md Desktop, desktop-tauri rule, ADR 0006, vision, baseline 2026-09-01
- Delivered context refs: conversation (browser cannot decode DTS/TrueHD; Tauri+mpv chosen)
- Acknowledged before plan refs: AGENTS.md, ADR 0006, vision, baseline, web LibraryPlayer
- Cited in plan refs: those plus web-library-player spec (as existing player, not superseded)
- Missing refs: none
- Decision: continue

Requirement Ready Check:
- Requirement source refs: AGENTS.md Desktop; user “用 Media Hub 在电脑端出声”
- Goals and scope refs: native decode for own-library DirectPlay
- User / scenario refs: Chrome silent audio on DTS/TrueHD; want Media Hub not Emby-only
- Acceptance / verification criteria refs: Tauri window + mpv audio; browser fallback unchanged
- Open blocker questions: none for v1 (mpv must be installed)
- Decision: ready

Change Necessity:
- User-visible need: computer playback with sound inside Media Hub
- No-change / non-code option: “在 Emby 打开” — user rejected as the only path
- Why code change is necessary: browser cannot decode the codec; rules now allow a desktop native player
- Minimum change boundary: Web detect+invoke hook + desktop/ Tauri mpv spawn
- Decision: code-change

Existence Check:
- Proposed new surface: desktop/ Tauri crate + play_native command
- Existing owner / reuse candidate: Web LibraryPlayer, Android Media3
- Why existing surface is insufficient: Web cannot decode DTS/TrueHD; Android is phone-only
- Creation proof: product rule change 2026-09-04
- Entropy / retirement impact: no Electron; retire only if Tauri is abandoned
- Decision: add-with-proof

Architecture Integrity Lens:
- Invariant: no media byte proxy; one OpenAPI; Web remains the UI
- Canonical owner: Web for chrome; desktop Rust for native play only
- Overlap: none if browser path stays in LibraryPlayer
- Verdict: proceed

Plan Pressure Test:
- Owner / contract / retirement: new desktop owner, no API contract change
- Verification scope: Web regression + cargo check + manual mpv
- Task executability: concrete files
- Pressure result: proceed

Plan-Time Complexity Check:
- Target files: LibraryPlayer.tsx (add-in-place risk), new desktop/
- Recommendation: extract web/src/shared/desktop/nativePlayback.ts; keep LibraryPlayer branch tiny
- Budget result: within-budget

Execution Readiness View:
- Intent Lock: Tauri wraps Web; mpv plays descriptor URL
- Scope Fence: no Electron, no iOS, no transcode proxy, no Windows installer CI this slice
- Baseline Lock: AGENTS.md Desktop, ADR 0006
- Approved Behavior: own-library play in desktop has sound via mpv
- Owner / Contract Constraints: OpenAPI unchanged
- Compatibility Boundary: browser unchanged
- Retirement Boundary: none
- Task Batches: Web hook → Tauri scaffold → native command → verify
- Test Obligations: Vitest hook; Playwright still green
- Review Gates: cargo check; web quality/e2e
- Drift / Rewind Rules: do not add a second UI; do not log stream URLs
- Evidence Required Before Completion: tests + cargo check + note if mpv missing on host
- Advisory Boundary: method-pack execution guidance only
```

## Tasks

### Task 1 — Web native-play hook (browser no-op)

Files:
- create `web/src/shared/desktop/nativePlayback.ts`
- create `web/src/shared/desktop/nativePlayback.test.ts`
- modify `web/src/features/library/LibraryPlayer.tsx`

Why: Desktop and browser share one LibraryPlayer. Detection must be explicit and testable.

Change Necessity: code-change; without the hook Tauri cannot intercept play.

Impact/Compatibility: Playwright fixtures have no Tauri; e2e stays on `<video>`.

Steps:

1. Add `nativePlayback.ts`:

```ts
export type NativePlayRequest = {
  title: string
  startPositionMs?: number
}

type TauriInvoke = (cmd: string, args: Record<string, unknown>) => Promise<unknown>

function tauriInvoke(): TauriInvoke | null {
  const internals = (globalThis as { __TAURI_INTERNALS__?: { invoke?: TauriInvoke } }).__TAURI_INTERNALS__
  return typeof internals?.invoke === 'function' ? internals.invoke.bind(internals) : null
}

export function canPlayNatively() {
  return tauriInvoke() !== null
}

export async function playNatively(streamUrl: string, request: NativePlayRequest) {
  const invoke = tauriInvoke()
  if (!invoke) throw new Error('native playback is unavailable')
  await invoke('play_native', {
    url: streamUrl,
    title: request.title,
    startPositionMs: request.startPositionMs ?? 0,
  })
}
```

2. Tests: `canPlayNatively()` false by default; true when `__TAURI_INTERNALS__.invoke` is a function; `playNatively` calls invoke with `play_native`.

3. In `LibraryPlayer` after descriptor resolves, if `canPlayNatively()`: `await playNatively(descriptor.streamUrl, { title, startPositionMs: descriptor.startPositionMs })`, skip `video.src` / hls. Keep subtitle fetch for later; v1 may still show the overlay chrome with a short status “已交给系统播放器”. Do not put `streamUrl` in UI.

4. Verify:

```bash
pnpm --dir web test
pnpm --dir web lint
```

### Task 2 — Tauri 2 scaffold

Files:
- create `desktop/package.json`, `desktop/src-tauri/Cargo.toml`, `desktop/src-tauri/tauri.conf.json`, `desktop/src-tauri/capabilities/default.json`, `desktop/src-tauri/src/main.rs`, `desktop/src-tauri/src/lib.rs`

Why: New distribution surface lives next to `web/` and `android/`, not inside them.

Change Necessity: no existing desktop crate.

Compatibility: do not change CI release.yml in this slice.

Steps:

1. `desktop/package.json` scripts: `"tauri": "tauri"`, depend on `@tauri-apps/cli` (pinned 2.x).
2. `tauri.conf.json`:
   - `productName`: Media Hub
   - `identifier`: `us.ci.himym.mediahub`
   - `app.windows[0].url`: `https://media.himym.us.ci`
   - `app.security.capabilities`: default
   - allow remote IPC for `https://media.himym.us.ci` (Tauri 2 `capabilities` `remote.urls`)
3. `Cargo.toml`: `tauri` 2.x, `tauri-build`, no extra plugins if `std::process::Command` is enough.
4. `cargo check` in `desktop/src-tauri`.

### Task 3 — `play_native` command

Files:
- modify `desktop/src-tauri/src/lib.rs`

Why: Native decode is the only reason the desktop shell exists.

Steps:

1. Command signature:

```rust
#[tauri::command]
fn play_native(url: String, title: String, start_position_ms: u64) -> Result<(), String> {
    if !(url.starts_with("https://") || url.starts_with("http://")) {
        return Err("unsupported playback url".into());
    }
    let mut cmd = std::process::Command::new("mpv");
    cmd.arg("--force-window=yes")
        .arg("--keep-open=no")
        .arg(format!("--title={}", title.replace(['\n', '\r'], " ")))
        .arg("--no-terminal");
    if start_position_ms > 0 {
        cmd.arg(format!("--start={:.3}", start_position_ms as f64 / 1000.0));
    }
    cmd.arg(&url);
    cmd.spawn().map(|_| ()).map_err(|_| "未找到 mpv。请先安装 mpv 并确保在 PATH 中。".to_string())
}
```

2. Do not `println`/`log` the URL. Errors are generic.
3. Register in `tauri::Builder::default().invoke_handler(tauri::generate_handler![play_native])`.
4. Verify: `cargo check`.

### Task 4 — Docs + index

Files:
- create `desktop/README.md` (install mpv, `pnpm --dir desktop tauri dev`, origin URL)
- modify `docs/aegis/INDEX.md`
- optional one-line pointer in `docs/development/roadmap.md` if the Tauri row is still “not started”

Verify: `pnpm --dir web test:e2e` after Task 1 lands.

## Risks

- Host is macOS; Windows `.msi` needs a Windows builder later.
- mpv missing → clear error, no silent fallback to `<video>` in Tauri (that would reintroduce mute).
- Remote webview requires the **deployed** Web hook; until v0.20.25+ is live, `tauri dev` against production will still use `<video>`. Mitigation: after Task 1, ship Web before relying on production origin, or temporarily point `url` at a local Vite with production API (out of this slice unless blocked).

## Retirement

None. Electron remains rejected.

## ADR signal

Preserve for later ADR: “Desktop playback uses Tauri + mpv spawn; rejected Electron and in-browser WASM/ffmpeg.”
