# SubX Capability Parity

## Evidence Boundary

This inventory is based only on the reference deployment's publicly exposed FastAPI OpenAPI contract (`115 Subscription Manager API`, version `0.1.0`) and unauthenticated application metadata. No container environment variables, persisted settings, database rows, credentials, cookies, subscription records, search results, or media names were read.

SubX remains a temporary compatibility dependency while Media Hub implements the same user-visible outcomes through project-owned contracts. Closed implementation details are not copied or reverse engineered.

## Capability Inventory

### Account And Settings

- Administrator login and password change.
- Provider connection settings and connection tests.
- Runtime health, logs, and log cleanup.

### Discovery And Catalog

- TMDB search, trending lists, and movie/series details.
- Recommendation sections and poster retrieval.
- Tracked-content create, update, delete, manual search, batch subscription changes, and backup import/export.
- Emby single/batch presence checks, library summaries, and poster retrieval.

### Resource Sources

The reference deployment exposes eight source families:

- `dian`
- `framehdr`
- `gimy`
- `guanying`
- `hdhive`
- `juying`
- `mikan`
- `sidhub`

Common capabilities include profile/status, search, save to 115, and connection testing. Depending on the source, the reference also exposes sign-in, link checking, OAuth lifecycle, resource details, unlock history, latest-resource feeds, and cross-source search.

### Subscriptions

- Create, list, update, delete, enable, and disable subscriptions.
- Per-subscription execution history.
- Scheduled synchronization and manual content search.
- Batch subscription changes for tracked content.
- Duplicate suppression across history and Emby state.

### 115 Operations

- QR login lifecycle and supported QR applications.
- Connection testing and account sign-in.
- Explicit save-to-115 operations.
- Movie/series destination routing.
- Archive organization preview and execution.
- Local-upload preview, execution, and records.

### Archive Organization

- Plans, previews, run history, and record pagination.
- Recognition corrections with create/update/delete behavior.
- Record reset and delete operations.
- Preview-before-write for destructive or bulk changes.

### STRM And Playback Operations

The reference deployment includes STRM generation, queue control, failed-item reruns, records, file indexing, consistency checking/repair, deletion previews, operation audits, and playback redirect routes.

Media Hub preserves its existing product boundary:

- QMediaSync owns STRM generation and STRM consistency operations.
- Emby owns library and playback APIs.
- 115 CDN sends media directly to the player.
- Media Hub never adds an equivalent media proxy or playback redirect endpoint.

Parity therefore means exposing equivalent control, queue, audit, and readiness outcomes through QMediaSync and Emby adapters, not copying SubX media-serving behavior.

### Integrations And Notifications

- MoviePilot connection, subscribe, unsubscribe, subscription checks, and subscription lists.
- Telegram session setup, command synchronization, and notification testing.
- Enterprise WeChat callback, menu synchronization, and notification testing.
- Statistics for recent automatic updates and manual saves.

## Native Media Hub Mapping

| SubX area | Status | Media Hub mapping |
| --- | --- | --- |
| Administrator session | Native | Revocable Cookie/Bearer sessions, Argon2id password rotation, revocation of other sessions, and Web CSRF boundary |
| Search and source save | Native + migration fallback | Eight project-owned source adapter IDs feed normalized search and durable transfer; the explicit SubX source flag remains a migration fallback |
| Subscription scheduler | Native | SQLite subscriptions, deterministic polling leases, episode progress, and append-only run events |
| Duplicate suppression | Native | TMDB identity + season/episode ranges + active/completed jobs + playable Emby episodes |
| Content backup migration | Native | Read-only SubX backup extraction followed by atomic native subscription import |
| 115 authorization and file operations | Native | Encrypted PKCE device authorization, automatic refresh, bounded directory reads, and durable create-folder/move/rename/delete commands; uncertain writes require ID-confirmed replay |
| STRM generation | Native | QMediaSync manual synchronization, record polling, Emby refresh, and playback verification |
| SubX STRM records/audits | Native | QMediaSync status/records plus Media Hub transfer events and Emby playback verification replace SubX-specific STRM projections |
| Playback routes | Rejected | Direct 115 CDN-to-player path through Emby; Media Hub never proxies media |
| Archive organization | Native | Review-only suggestions, explicit editable rename/move steps, encrypted durable plans, plan-ID confirmation, ordered progress, and uncertain-step recovery |
| Local upload | Native | Configured opaque root IDs, symlink-rejecting relative paths, encrypted durable jobs, SHA-1 rapid upload, OSS multipart progress, and ID-confirmed uncertain replay |
| TMDB trends, recommendations, and tracked catalog | Native | Direct bounded TMDB identity reads plus native subscription catalog; recommendation choices return to the existing resource-search flow |
| MoviePilot | Rejected | Outside the Media Hub product boundary; subscription and transfer orchestration remain native |
| Telegram | Rejected | Enterprise WeChat is the supported persisted notification channel |
| Enterprise WeChat | Native | Persisted terminal notifications, uncertain-send recovery, and authenticated manual resend confirmation |
| Statistics and logs | Native | User-scoped operational counters and Media Hub-owned structured runtime logs; provider-private logs are not proxied |

`canStopSubX` now fails closed. It is true only when the fallback source is disabled, no persisted SubX source command remains in `queued`, `submitting`, or `needs_attention`, the TMDB/115/QMediaSync/Emby and movie/series workflow configuration is complete, at least one native source adapter is configured and has passed a real search, every core provider is currently healthy, and `MEDIA_HUB_SUBX_PARALLEL_VALIDATION_COMPLETED=true` explicitly records a successful side-by-side acceptance run. Blocking command IDs and states remain available only through the authenticated migration recovery API; uncertain commands require exact-ID retry after the fallback source is explicitly re-enabled.

## Completion Gates

SubX replacement is complete only when:

1. Every capability above has an explicit native, delegated, optional, or intentionally rejected mapping.
2. Web and Android consume one OpenAPI contract for subscriptions, content tracking, source status, history, and manual actions.
3. Automatic runs are durable, idempotent, bounded, pauseable, and recoverable after restart.
4. Side-by-side sanitized contract fixtures cover every enabled source and failure class.
5. A real configured acceptance run proves search, subscription match, deduplication, 115 transfer, QMediaSync, Emby readiness, and notification delivery.
6. SubX can be stopped without removing any supported Media Hub workflow.
