# Development Roadmap

Status: API `0.8.0-dev` removed the SubX compatibility layer. Provider settings cover TMDB, 115, QMediaSync, Emby, WeCom, workflow targets, and eight native source IDs. Mikan has a built-in adapter; the other seven still require a contract-compliant adapter URL.

## Phase 0: Foundation

- Write product, architecture, and design decisions.
- Initialize Go, React, and Android builds.
- Establish `/api/v1/health` and `/api/v1/system/overview`.
- Build the search-first shell on Web and Android with deterministic fixture data.
- Add CI-ready local verification commands.

Exit gate: all three projects build and render the same system-status semantics.

## Phase 1: Read-only Integration

- QMediaSync health and task status.
- Emby health, library mapping, duplicate lookup, and index status.
- 115 authorization health without exposing credentials.
- Source adapter health and normalized search from two sources.

Exit gate: search and system status are useful without any write side effect.

## Phase 2: Transfer Workflow

- TMDB identity matching.
- Movie/series destination policy.
- Durable workflow and job tables.
- 115 transfer with idempotency and outcome verification.
- QMediaSync trigger and STRM readiness checks.
- Emby refresh and index verification.
- Enterprise WeChat completion/failure notifications.

Exit gate: one selected release reaches playback-ready state without manual intervention.

Implementation status: API `0.3.0` includes the durable state machine and matching Web/Android task, library, and service views. The exit gate still requires a real configured source-adapter acceptance run.

## Phase 3: Native Subscriptions

- Tracked movies, series, seasons, and episode progress.
- Saved searches, update intervals, and source selection.
- Resolution, codec, dynamic-range, audio, size, and release preference rules.
- Scheduled source polling with durable leases and manual runs.
- Duplicate suppression across TMDB identity, active/completed jobs, subscription history, and Emby.
- Per-subscription pause, resume, run history, and append-only audit events.
- Backup export/import and batch subscription changes.

Exit gate: an update can select one preferred release, transfer once, recover after restart, and never duplicate on retries.

Implementation status: SQLite persistence, scheduler, source/release rules, episode progress, API, Web and Android management, run history, backup import/export, and batch enable/pause are implemented. The exit gate still requires a real configured scheduled run.

## Phase 4: Full Operational Parity

- [x] Native TMDB trending and recommendations plus tracked-content catalog and Emby presence summaries.
- [ ] Deploy project-owned adapters for `dian`, `framehdr`, `gimy`, `guanying`, `hdhive`, `juying`, `mikan`, and `sidhub`; Media Hub currently has only the shared adapter contract/client, with SubX available as an explicitly enabled migration fallback.
- [x] Source health/search/transfer capabilities use one project-owned contract; provider-specific login, feeds, and check-in remain adapter responsibilities.
- [x] 115 QR login, connection testing, destination routing, bounded browsing, durable file commands, archive plans, and symlink-rejecting local upload.
- [x] QMediaSync-backed synchronization records, persisted transfer events, Emby refresh, and playback verification replace SubX STRM operations.
- [ ] Promote Android Emby-managed Media3 playback from experimental after real movie/episode redirect, Range seek, resume, progress, and fallback acceptance.
- [x] Native user-scoped operational summary and Media Hub-owned structured logs; provider-private logs are not proxied.
- [x] MoviePilot and Telegram are intentionally rejected by the product boundary; Enterprise WeChat is the supported notification channel.
- [x] Native administrator password change with Argon2id rehashing and transactional revocation of other sessions.
- [x] Authenticated Enterprise WeChat delivery-state list and exact-ID-confirmed manual resend for uncertain outcomes.

Media Hub will not implement SubX playback proxy routes: Android resolves typed 115 or Emby targets to external HTTPS media, Media3 connects directly, and Emby App/Web remains the fallback. Media Hub forwards opaque playback progress events but never proxies media bytes.

Exit gate: every item in the [SubX parity matrix](../integrations/subx-parity.md) has a tested native, delegated, optional, or intentionally rejected mapping.

Implementation status: native control paths exist for the main workflow, but the eight deployable source adapters remain unfinished. The compatibility catalog is not advertised to clients; its internal allowlist remains only for read-only backup migration and the explicitly enabled SubX source fallback.

## Phase 5: Remove SubX

- Migrate stored source and subscription configuration.
- Run side-by-side sanitized contract fixtures and real acceptance workflows.
- Compare feature outcomes and failure-mode behavior.
- Stop SubX only after parity gates pass and Media Hub can recover independently.

Implementation status: backup imports map recognized subscriptions to native source IDs instead of `subx`. `/migration/subx/readiness` also blocks on an enabled fallback, explicitly fallback-bound subscriptions, queued/submitting/uncertain commands, retryable failed commands, missing or unhealthy core providers, absent healthy native sources, and incomplete parallel acceptance.

## Deferred

- Web built-in player.
- iOS or desktop app.
- Multi-user roles.
- Public registration and billing.
- PT/MoviePilot workflows.
- PostgreSQL, Redis, or distributed workers.

## Upgrade Signals

- Move SQLite to PostgreSQL only when concurrent writer contention is measured.
- Split workers only when long jobs affect API availability or deployments.
- Add server-sent events only when polling produces measurable UX or load problems.
- Add Gradle modules only when build times or ownership boundaries justify them.
