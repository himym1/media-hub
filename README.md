# Media Hub

Media Hub is a self-hosted control plane for searching media resources, transferring them to 115, generating STRM entries through QMediaSync, and indexing them in Emby.

The monorepo contains:

- `backend/`: Go API, SQLite persistence, provider adapters, and the workflow runner.
- `web/`: React, TypeScript, Vite, and TanStack Query client.
- `android/`: Native Kotlin, Jetpack Compose, and project-owned `MediaHub*` UI wrappers.
- `api/`: The OpenAPI contract shared by both clients.
- `docs/deployment/private-deployment.md`: GHCR, NAS Compose, backup/restore, and Android release procedure.

Media Hub never proxies media payloads. Playback remains a direct path from 115 CDN to the player.

## Implemented Flow

```text
Authenticated search
  -> TMDB identity verification
  -> encrypted release selection
  -> idempotent source-adapter transfer to 115
  -> QMediaSync manual synchronization and record polling
  -> Emby library refresh and TMDB-aware item matching
  -> Emby playback-information verification
  -> Enterprise WeChat application-chat notification
```

Transfer jobs and append-only events are persisted before external side effects. Source references, adapter operation IDs, 115 file IDs, and provider paths are stored only inside AES-256-GCM opaque payloads. Safe operations retry with bounded backoff. A QMediaSync submission with an uncertain result moves to `needs_attention` and is never replayed automatically.

Web and Android expose the same search, transfer status, event history, explicit retry, provider status, and Emby library semantics. Web uses an HttpOnly session cookie plus CSRF protection; Android uses a revocable bearer session stored through Android Keystore.

Native subscriptions persist movie, series, season, and episode progress with source selection, release preferences, durable scheduling, duplicate suppression, manual runs, batch pause/resume, and atomic backup import/export. Native 115 operations add encrypted PKCE authorization, bounded browsing, durable file commands, symlink-rejecting local uploads, and review-first archive plans across Web and Android. SubX credentials are retained only for read-only backup migration and the explicitly enabled fallback source; the compatibility catalog is no longer advertised to clients.

TMDB trends and recommendations are native. 115 supports encrypted PKCE device authorization with automatic refresh plus user-scoped directory browsing and durable file commands. Create-folder, move, rename, and delete requests are persisted with encrypted parameters before provider submission; delete and uncertain retries require explicit command-ID confirmation.

## Configuration

The backend reads environment variables as a startup baseline and applies embedded SQLite migrations. Authenticated Web and Android settings store encrypted runtime overrides in SQLite and apply them without a restart; secret values are never returned by the settings API. Secrets remain in process memory and must not be placed in logs or committed files.

```text
MEDIA_HUB_ADDR=:8080
MEDIA_HUB_DATABASE_PATH=data/media-hub.db
MEDIA_HUB_PROBE_TIMEOUT=3s
MEDIA_HUB_SECURE_COOKIES=true
MEDIA_HUB_ENABLE_FIXTURES=false

MEDIA_HUB_ADMIN_PASSWORD=<initial-password-at-least-12-bytes>
MEDIA_HUB_DATA_ENCRYPTION_KEY=<base64-encoded-32-byte-key>

MEDIA_HUB_TMDB_ACCESS_TOKEN=<tmdb-read-access-token>
MEDIA_HUB_TMDB_URL=https://api.themoviedb.org/3

MEDIA_HUB_115_ACCESS_TOKEN=<115-open-api-token>
MEDIA_HUB_115_CLIENT_ID=<115-open-client-id>
MEDIA_HUB_115_MOVIE_DESTINATION_ID=<directory-id>
MEDIA_HUB_115_SERIES_DESTINATION_ID=<directory-id>
MEDIA_HUB_LOCAL_UPLOAD_ROOTS=/mnt/media/incoming,/mnt/media/staging

MEDIA_HUB_QMS_URL=<qmediasync-base-url>
MEDIA_HUB_QMS_API_KEY=<qmediasync-api-key>
MEDIA_HUB_QMS_ACCOUNT_ID=<qmediasync-account-id>
MEDIA_HUB_QMS_MOVIE_TARGET_PATH=<strm-target-path>
MEDIA_HUB_QMS_SERIES_TARGET_PATH=<strm-target-path>

MEDIA_HUB_EMBY_URL=<emby-base-url>
MEDIA_HUB_EMBY_API_KEY=<emby-api-key>
MEDIA_HUB_EMBY_USER_ID=<emby-user-id>
MEDIA_HUB_EMBY_MOVIE_LIBRARY_ID=<library-id>
MEDIA_HUB_EMBY_SERIES_LIBRARY_ID=<library-id>

MEDIA_HUB_SOURCE_DIAN_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_DIAN_TOKEN=<adapter-token>
MEDIA_HUB_SOURCE_FRAMEHDR_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_FRAMEHDR_TOKEN=<adapter-token>
MEDIA_HUB_SOURCE_GIMY_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_GIMY_TOKEN=<adapter-token>
MEDIA_HUB_SOURCE_GUANYING_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_GUANYING_TOKEN=<adapter-token>
MEDIA_HUB_SOURCE_HDHIVE_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_HDHIVE_TOKEN=<adapter-token>
MEDIA_HUB_SOURCE_JUYING_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_JUYING_TOKEN=<adapter-token>
MEDIA_HUB_SOURCE_MIKAN_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_MIKAN_TOKEN=<adapter-token>
MEDIA_HUB_SOURCE_SIDHUB_URL=<adapter-base-url>
MEDIA_HUB_SOURCE_SIDHUB_TOKEN=<adapter-token>

MEDIA_HUB_SUBX_URL=<subx-base-url>
MEDIA_HUB_SUBX_TOKEN=<static-token>
MEDIA_HUB_SUBX_USERNAME=
MEDIA_HUB_SUBX_PASSWORD=
MEDIA_HUB_SUBX_SOURCE_ENABLED=false
MEDIA_HUB_SUBX_PARALLEL_VALIDATION_COMPLETED=false

MEDIA_HUB_WECOM_CORP_ID=<enterprise-id>
MEDIA_HUB_WECOM_SECRET=<application-secret>
MEDIA_HUB_WECOM_CHAT_ID=<application-created-chat-id>
MEDIA_HUB_WECOM_URL=https://qyapi.weixin.qq.com
```

Integration URLs must be absolute HTTP(S) URLs without embedded credentials, query parameters, or fragments. When a TMDB token is set without `MEDIA_HUB_TMDB_URL`, the official API URL is used.

Resource adapters implement the normalized [search and transfer contract](docs/integrations/source-adapter.md). The repository contains the adapter client and contract, not deployable implementations of the eight source services. SubX credentials are used only for authenticated subscription-backup migration and the optional fallback source. Fallback URL, credentials, and activation can be managed through encrypted runtime settings; environment variables remain a startup baseline. Enabling fallback permits search and transfer through SubX's configured 115 destination and means SubX must remain running. Explicitly fallback-bound subscriptions and unresolved or retryable SubX commands remain shutdown blockers. `MEDIA_HUB_SUBX_PARALLEL_VALIDATION_COMPLETED` defaults to `false` and must be enabled only after the real side-by-side acceptance run has verified search, transfer, QMediaSync, Emby playback, subscriptions, and recovery. Fixture search data is available only with explicit `MEDIA_HUB_ENABLE_FIXTURES=true` and never performs transfers.

## Local Verification

```bash
# Backend
go -C backend test ./...
go -C backend test -race ./...
go -C backend vet ./...

# Web
pnpm --dir web install --frozen-lockfile
pnpm --dir web test
pnpm --dir web lint
pnpm --dir web build

# Android
cd android
./gradlew :app:testDebugUnitTest :app:assembleDebug :app:lintDebug
```

## Private deployment

Release tags publish a multi-architecture private image to `ghcr.io/himym1/media-hub`. The image serves the Web client and `/api/v1` from one origin. See [private deployment](docs/deployment/private-deployment.md) for NAS layout, GHCR login, SQLite backup/restore, TLS, and Android release handling.

Do not commit credentials, cookies, API keys, `.env` files, NAS exports, SQLite databases, or generated APKs.

See the [roadmap](docs/development/roadmap.md) and [system design](docs/architecture/system-design.md).
