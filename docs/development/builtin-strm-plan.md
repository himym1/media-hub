# Built-in STRM Sync Plan

Status: Phases A–F implemented in repo; NAS overlay/E2E and release still pending  
Goal: replace QMediaSync for the Media Hub transfer workflow while keeping one 115 authorization inside Media Hub.

## Confirmed production paths (NAS `himym`, 2026-08-24)

Verified from live containers; do not guess these again.

| Role | Path |
|------|------|
| Host STRM root | `/volume2/media/115-strm` |
| QMS container | `/volume2/media/115-strm` → `/media` |
| Media Hub builtin mount | same as QMS: `/volume2/media/115-strm` → `/media` |
| Emby (`himym-embypt`) | `/volume2/media` → `/media3`, so STRM is `/media3/115-strm` |
| Library subdirs | `电影/`, `电视剧/` |
| Emby libraries | `115电影` (`15075`), `115电视剧` (`15077`) |
| QMS `sync_paths` | `local_path=/media`, `remote_path=电影` / `电视剧` |
| STRM URL written today | `https://qms.himym.us.ci/115/url/video{ext}?pickcode={pc}&userid={uid}` |
| Emby playback redirect (Phase A keep) | `MEDIA_HUB_EMBY_PLAYBACK_URL=http://192.168.8.146:58095` |

Media Hub settings `qMediaSyncTargetPath` should stay `/media/电影` and `/media/电视剧` so builtin writes land next to existing `.strm` files.

Compose overlay: `deploy/compose.strm.yaml`. Enable with `COMPOSE_FILE=compose.yaml:compose.strm.yaml` (and mikan overlay if needed). Overlay default base URL is `https://media.himym.us.ci`. Empty `MEDIA_HUB_STRM_SYNC_MODE` plus mount and base URL infers builtin; set `qmediasync` to roll back.

## Problem

Media Hub currently depends on a separate QMediaSync container for post-transfer STRM generation. QMediaSync uses 115 Open Platform OAuth with a fragile refresh path: transient network errors clear stored tokens and force manual re-authorization. Media Hub already maintains its own longer-lived 115 cookie session for transfers.

## Principles

- Media Hub remains the single control plane; media bytes are never proxied.
- `syncMode` stays `qmediasync` unless set to `builtin`, or left empty with both `strmBaseUrl` and `strmRootMount`.
- Builtin sync reuses `drive115` session APIs; do not add a second 115 login.
- STRM URL format stays compatible with existing Emby libraries during migration.
- Persist workflow state before filesystem or external side effects.

## Architecture

```text
transfer completed (115 file/folder ID known)
  -> strm.Syncer (builtin | qmediasync adapter)
  -> [builtin] list 115 tree, write .strm under Emby library mount
  -> emby.RefreshLibrary
  -> emby index verification
  -> wecom notification
```

New package:

```text
backend/internal/strm/
  syncer.go           # interface
  builtin/            # native implementation
  qms/                # wraps existing qms.Client during migration
```

Settings:

| Field | Purpose |
|-------|---------|
| `workflow.syncMode` | `qmediasync` (default) or `builtin` |
| `workflow.strmBaseUrl` | URL written into `.strm` files |
| `workflow.strmRootMount` | container path matching the QMS/Emby STRM root; production is `/media` |
| `workflow.movie.strmTargetPath` | existing `qMediaSyncTargetPath` |
| `workflow.series.strmTargetPath` | existing `qMediaSyncTargetPath` |

STRM content (compatible with QMS / wabisabi926 fork):

```text
{strmBaseUrl}/115/url/video{ext}?pickcode={pc}&userid={uid}
```

## Phase A — MVP: transfer-triggered builtin sync

**Goal:** one movie or one series folder synced after transfer without calling QMediaSync.

### Scope

- [x] `strm.Syncer` interface; QMS path stays on `qms.Client` so `syncMode=qmediasync` is unchanged
- [x] `builtin` walker: recursive 115 listing for the transferred folder only
- [x] Video extension filter (`.mkv`, `.mp4`, `.avi`, `.mov`, `.wmv`, `.flv`, `.webm`, `.m4v`, `.3gp`, `.ts`)
- [x] Path mapper: 115 logical path → `{strmRootMount}/{targetSubpath}/.../*.strm`
- [x] Writer: create parent dirs, atomic write, skip if content unchanged
- [x] Resolve 115 `userid` and per-file `pickcode` from `drive115`
- [x] Workflow branch in `worker.go`: builtin runs synchronously inside `submitting_sync` then jumps to Emby refresh
- [x] Error codes: `strm_auth_expired`, `strm_list_failed`, `strm_path_unwritable` (retryable where safe; never clear 115 session on network blips)
- [x] OpenAPI + Web settings: sync mode, STRM base URL; hide QMS fields when builtin
- [x] Android settings models follow OpenAPI
- [x] Deploy overlay: `deploy/compose.strm.yaml` mounts `/volume2/media/115-strm` → `/media`
- [x] Go tests for mapper, content, writer, workflow branch
- [ ] NAS: enable compose overlay and run one movie + one series E2E

### Playback during MVP

New `.strm` files can point at Media Hub `/115/url/`. Existing files keep their written host until a full library sync rewrites them. `MEDIA_HUB_EMBY_PLAYBACK_URL` is a separate Android/Emby stream-probe route and is not required for STRM file playback.

### Exit gate

- New transfer with `syncMode=builtin` completes without QMS container running.
- `syncMode=qmediasync` unchanged.
- One movie and one series E2E on NAS.

**Estimate:** 1–2 weeks.

---

## Phase B — Reliability and observability

**Goal:** production-grade error handling and operator visibility.

- [x] Bounded retries with backoff for 115 list/write (distinct from QMS "clear token on any error")
- [x] Per-job sync summary persisted on transfer record (files scanned, strm created, updated, skipped, duration)
- [x] `GET /api/v1/integrations/strm/status` health: mount writable, 115 session OK, last sync error
- [x] Web/Android services view: show builtin STRM health instead of/alongside QMS when builtin mode
- [x] WeCom alert when 115 session degrades (reuse existing integration health patterns)
- [x] Startup check: fail fast or warn if `strmRootMount` missing or not writable in builtin mode
- [x] Structured logs with redacted paths only (no cookies, pickcodes in info logs)

### Exit gate

- Operator can tell from UI/API why sync failed without reading container logs.

**Estimate:** 3–5 days.

---

## Phase C — Incremental and scheduled library sync

**Goal:** keep libraries current without manual transfers or QMS cron.

- [x] SQLite tables `strm_folder_states` and `strm_sync_runs`
- [x] Incremental scan: only folders touched since last run (mtime / 115 `t` field)
- [x] Detect pickcode change → rewrite `.strm` (reference wabisabi926 `CompareStrm`)
- [x] Optional scheduled sync per workflow target (movie root, series root) via coordinator ticker
- [x] Manual "sync library now" API for movie/series roots
- [x] Rate limiting for 115 list API (400 ms gap)
- [x] Skip files below minimum size threshold (default 100 MB like QMS)

### Exit gate

- Adding a file manually to 115 under a watched folder appears in Emby within one scheduled cycle.

**Estimate:** 1–2 weeks.

---

## Phase D — Builtin playback redirect (retire QMS emby302)

**Goal:** remove dependency on QMS for Emby STRM playback URLs.

- [x] Media Hub endpoint compatible with existing STRM URLs, e.g. `GET /115/url/video.mkv?pickcode=&userid=`
- [x] Resolve pickcode → 115 CDN URL using existing `drive115` + `playback` packages
- [x] Cache redirect URLs with TTL (~50 min, under 115 1h expiry)
- [x] Config: `workflow.strmBaseUrl` points to Media Hub public or internal URL
- [x] Emby `MEDIA_HUB_EMBY_PLAYBACK_URL` can equal Media Hub base (optional; STRM files do not use it)
- [ ] Range request passthrough or redirect semantics validated per Emby version on NAS
- [x] Android direct playback path does not assume QMS host for new STRM URLs

### Exit gate

- QMS container stopped; new and existing STRM files play through Media Hub redirect only.

**Estimate:** 1–2 weeks.

---

## Phase E — Library hygiene

**Goal:** STRM tree stays aligned with 115 without orphan files.

- [x] Remove local `.strm` when 115 file deleted (transfer-triggered prune on; library prune on full sync)
- [x] Remove empty local directories after delete (`delEmptyLocalDir` behavior)
- [ ] Optional: upload missing metadata to 115 (out of scope if not needed)
- [ ] Optional: download metadata from 115 to local library (nfo, poster) — only if Emby setup requires it
- [x] Reconcile job: full diff 115 tree vs local STRM tree with dry-run preview API

### Exit gate

- Renaming or deleting on 115 is reflected locally within one reconcile run.

**Estimate:** 1–2 weeks.

---

## Phase F — QMS deprecation and removal

**Goal:** QMediaSync is no longer required in deployment.

- [x] Default `syncMode` → `builtin` when mode is empty and both STRM fields are set
- [x] Mark QMS settings as deprecated in UI; keep the toggle for rollback
- [ ] Remove `qms` package and QMS health endpoints after one release with deprecation warning
- [x] Update `private-deployment.md`, `system-design.md`, and this plan
- [x] NAS compose docs: QMS is optional rollback, not required for Media Hub
- [ ] Remove `MEDIA_HUB_QMS_*` env vars after the rollback window

### Exit gate

- Fresh install does not mention QMediaSync. Existing users migrated via settings toggle.

**Estimate:** 3–5 days plus one release cycle.

---

## Phase G — Advanced (optional, demand-driven)

Only if product needs exceed personal NAS automation:

| Item | Notes |
|------|--------|
| OpenList STRM driver | Separate driver behind `strm.Syncer`; not needed for 115-only setup |
| Baidu Pan driver | Same |
| Multi-115-account | Map workflow target → account; today single account is enough |
| Webhook notifications for sync failures | Beyond WeCom |
| Full metadata scrape / TMDB organize | Explicitly out of Media Hub MVP; MoviePilot boundary |
| Distributed workers | Deferred per main roadmap until measured pressure |

---

## Migration path (NAS)

1. Deploy Media Hub with library volume mount; keep `syncMode=qmediasync`.
2. Release with builtin code behind flag; test one manual transfer with `syncMode=builtin` on a copy path.
3. Switch production workflow to `builtin`; keep QMS up for emby302 only (Phase A).
4. Phase D: point STRM URLs to Media Hub; stop QMS.
5. Phase F: remove QMS container.

## Risk register

| Risk | Mitigation |
|------|------------|
| Container cannot write Emby paths | Phase A deploy mount + startup writable check |
| 115 cookie still expires | Single auth in Media Hub; retry without session wipe; WeCom alert |
| STRM path mismatch with Emby | Document single source of truth for mount paths; validation tool |
| Large series folder timeout | Phase C rate limits; Phase A subtree-only scope; job timeout with resume |
| Regression for QMS users | `syncMode` toggle; qms adapter unchanged until Phase F |

## References

- Upstream QMS: [qicfan/qmediasync](https://github.com/qicfan/qmediasync)
- Active fork (STRM format reference): [wabisabi926/qmediasync](https://github.com/wabisabi926/qmediasync) `internal/syncstrm/driver_115.go`
- Token clearing issue: [qicfan/qmediasync#278](https://github.com/qicfan/qmediasync/issues/278)
- Media Hub workflow: `backend/internal/workflow/worker.go`
- 115 session: `backend/internal/drive115/auth_service.go`

## Timeline summary

| Phase | Focus | Duration |
|-------|--------|----------|
| A | MVP transfer-triggered sync | 1–2 weeks |
| B | Reliability + health UI | 3–5 days |
| C | Incremental + scheduled sync | 1–2 weeks |
| D | Builtin 302 / playback | 1–2 weeks |
| E | Delete/reconcile hygiene | 1–2 weeks |
| F | QMS removal | 3–5 days + release |
| G | Optional drivers | as needed |

**Total to QMS-free production:** roughly 5–8 weeks after Phase A starts, assuming NAS validation between phases.
