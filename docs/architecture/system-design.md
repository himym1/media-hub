# System Design

## Context

```text
                         +------------------+
                         | Enterprise WeChat|
                         +---------^--------+
                                   | notifications
+-------------+     HTTPS      +---+-------------------+
| Web browser |--------------->|                       |
+-------------+                |      Media Hub        |
                               |   Go control plane    |
+-------------+     HTTPS      |                       |
| Android app |--------------->+---+-----+-----+-------+
+-------------+                    |     |     |
                                   |     |     |
                         search    |     |     | workflow control
                                   v     v     v
                              +--------+ +---+ +----------+
                              |Sources | |115| |QMS / Emby|
                              +--------+ +---+ +----------+

Video playback paths:
- Managed Android library: Emby metadata/episodes -> Media Hub playback descriptor -> Media3 -> 115 CDN
- Emby fallback: Media Hub -> Emby App/Web -> STRM -> 115 CDN
- Diagnostic direct file: trusted 115 file -> Media Hub playback descriptor -> Media3 -> 115 CDN
```

## Runtime Containers

```text
NAS
+-----------------------------------------------------------+
| media-hub container                                       |
| +-------------------------------------------------------+ |
| | Go API + workflow runner + embedded Web static files | |
| +------------------------------+------------------------+ |
|                                |                          |
|                         SQLite WAL volume                  |
+-----------------------------------------------------------+

Android device
+-----------------------------------------------------------+
| Native Kotlin / Compose application                       |
| HTTPS API client; no server credentials embedded          |
+-----------------------------------------------------------+
```

The Web build is embedded into the Go server image for deployment. Development keeps independent dev servers.

## Backend Modules

- `auth`: single-user session and device tokens.
- `catalog`: normalized media identity and TMDB matching.
- `search`: fan-out, normalization, ranking, and source health.
- `sources`: one adapter per external resource provider.
- `transfer`: 115 destination resolution and transfer execution.
- `workflow`: durable stage machine, retries, and compensation rules.
- `qms`: QMediaSync synchronization adapter kept for rollback when `workflow.syncMode=qmediasync`.
- `strm`: built-in STRM writer, scheduled library sync, prune, health, and public `/115/url/` 302 used when `workflow.syncMode=builtin` (see [builtin-strm-plan.md](../development/builtin-strm-plan.md)). Empty mode plus both STRM base URL and root mount infers builtin.
- `emby`: duplicate checks, refresh, index checks, and playback readiness.
- `playback`: resolves typed trusted 115 and Emby item targets, owns opaque progress sessions, and validates upstream HTTPS descriptions without proxying media.
- `notify`: enterprise WeChat delivery with idempotent event keys.
- `store`: SQLite repositories and migrations.
- `httpapi`: versioned HTTP API and Web static serving.

Modules depend inward on domain contracts. Provider-specific response types do not escape adapters.

## Main Workflow

```text
1. Client submits a TMDB-verified, encrypted release selection with an idempotency key.
2. API validates the identity, source capability, and movie/series destination policy.
3. A transfer job and first append-only event are committed in one database transaction.
4. Worker asks the configured source adapter to transfer the release to 115 with a deterministic idempotency key.
5. Worker verifies asynchronous transfer completion through the adapter status endpoint.
6. Worker requests STRM synchronization only after the transfer result is persisted (`qmediasync` or `builtin`).
7. In QMediaSync mode, the worker correlates records by the returned 115 `base_cid`. In builtin mode it writes `.strm` files under the mounted library path, records a sync summary, and continues. Playback URLs resolve through Media Hub `GET /115/url/{name}` to 115 HTTPS CDN; bytes are never proxied.
8. Worker refreshes the mapped Emby library and matches the item by TMDB ID.
9. Worker requires Emby `PlaybackInfo` to contain a media source before completion.
10. Worker persists notification submission before sending one Enterprise WeChat application-chat message.
```

Clients never infer workflow completion from elapsed time. They render server-owned states.

## State Machine

```text
QUEUED
  -> TRANSFERRING
  -> TRANSFERRED
  -> SUBMITTING_SYNC
  -> SYNCING
  -> REFRESHING_EMBY
  -> INDEXING_EMBY
  -> VERIFYING_PLAYBACK
  -> COMPLETED

Recoverable stages may move to RETRY_WAIT or FAILED. A QMediaSync or Enterprise WeChat write with an uncertain result moves to NEEDS_ATTENTION and is never replayed automatically.
```

Each transition stores `attempt`, `started_at`, `finished_at`, an error classification, and redacted evidence.

## Data Model

- `integrations`: non-secret provider configuration metadata.
- `library_mappings`: destination and library metadata reserved for managed configuration.
- `users` and `sessions`: one administrator, Argon2id credentials, and revocable sessions.
- `transfer_jobs`: encrypted selection and provider-result payloads, TMDB identity, current stage, retry data, and Emby correlation ID.
- `transfer_job_events`: append-only user-visible transition and notification evidence.
- `transfer_notifications`: persisted Enterprise WeChat submission state; interrupted submissions require attention.

SQLite WAL is sufficient for the expected single-node workload. A PostgreSQL migration is triggered only by measured concurrent-writer pressure or multi-instance deployment.

## Reliability

- Every external call has connect, response, and total deadlines.
- Retries use exponential backoff with jitter and an explicit maximum.
- Transfer requests carry deterministic idempotency keys enforced by source adapters.
- Operations without upstream idempotency, including QMediaSync and Enterprise WeChat submission, are not automatically replayed after an unknown result.
- Worker leases expire and can be reclaimed after a crash.
- Configuration health is separate from workflow health.
- Source failures degrade search results instead of failing the whole search when at least one source succeeds.

## Security

- One administrator in the MVP; no public registration.
- Passwords use a memory-hard hash.
- Sessions use secure, HTTP-only cookies on Web and revocable bearer tokens on Android.
- Provider secrets are encrypted at rest with a key supplied outside the database.
- API responses and logs never contain 115 cookies, share passwords, API keys, or enterprise WeChat secrets. Authenticated playback descriptors may return an upstream temporary HTTPS URL to Android memory; URLs are never persisted or logged, and opaque sessions synchronize Emby progress without exposing provider credentials.
- CORS is disabled in production because Web is served from the same origin.
- Mutating operations require CSRF protection for cookie-authenticated Web requests.

## API Strategy

- REST under `/api/v1`.
- OpenAPI is the contract authority.
- Errors use stable machine codes plus user-facing summaries.
- Long workflows are polled initially; server-sent events are added only when polling causes a measured UX problem.
- Breaking API changes require `/api/v2`; clients advertise their build version.

## Deployment

The MVP deploys one container and one persistent directory:

```text
/volume1/docker/media-hub/
  config/
  data/media-hub.db
```

QMediaSync, Emby, and SubX remain independent during migration. Media Hub first calls their supported interfaces, then replaces SubX adapters incrementally.

## Failure Hotspots

1. Resource-source anti-bot or schema changes.
2. 115 authentication expiry and uncertain transfer outcomes.
3. Incorrect media identity causing wrong movie/series routing.
4. QMediaSync synchronization accepted but not completed.
5. Emby index delay or metadata-provider timeout.
6. Playback description generated for a mismatched player User-Agent or changed 115 response schema.

Each hotspot must have a contract test or deterministic health probe before its integration is declared production-ready.
