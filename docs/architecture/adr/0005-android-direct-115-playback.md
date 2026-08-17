# ADR 0005: Android Direct Playback for Trusted 115 Files

- Status: Accepted
- Date: 2026-08-16

## Context

Media Hub already authenticates 115, exposes bounded file browsing, and verifies Emby playback readiness. Users also need to play a selected 115 video directly on Android without waiting for QMediaSync or Emby indexing. The existing boundary rejected playback proxies and playback redirects, and prohibited ordinary APIs from exposing provider direct URLs.

115 download descriptions are upstream-issued and may not include a verifiable expiry. Their User-Agent must match the one used when the URL is resolved. The 115 cookie must remain server-side.

## Decision

Add an independent backend `playback` domain and an Android-only Media3 player.

- `POST /api/v1/playback/descriptors/drive115` accepts a trusted numeric `parentId` and `fileId` under normal Media Hub authentication.
- The 115 adapter loads the encrypted session, reads file metadata by `file_id`, verifies the returned file and parent IDs, resolves the current upstream URL with a fixed player User-Agent, and returns no cookie or pickcode.
- The response contains an upstream temporary HTTPS URL, User-Agent, title, and `expiresAt` only when the upstream supplies a verifiable expiry. Unknown expiry is not synthesized.
- Android keeps the URL only in memory. It is not persisted, logged, placed in navigation URLs, or included in screenshots.
- Media3 connects directly to 115 CDN. Media Hub never proxies media bytes and does not add a playback redirect route.
- Recognized expiry responses (`401`, `403`, `404`, `410`) trigger at most one re-resolution of the same trusted target, preserving playback position. New targets cancel older in-flight resolutions.
- A stable SHA-256 identity of the normalized Media Hub server URL binds playback requests to one server configuration. Process-local generations only scope ViewModels.
- `MediaSessionService` owns ExoPlayer, the queue, playback resolution, refresh, background controls, and session state. `PlayerActivity` owns full-screen, orientation, and picture-in-picture lifecycle. Compose UI remains in `feature/player`.
- Emby and QMediaSync remain the primary managed-library workflow and are not removed or made optional by this decision.

## Rejected Alternatives

- Add playback methods to `drive115`: mixes player lifecycle with authorization, transfer, upload, and file commands.
- Proxy media through Media Hub: violates the direct CDN path and creates bandwidth, Range, and failure responsibilities.
- Return 115 cookies to Android: expands the credential boundary and is unnecessary.
- Use a server-controlled `307` redirect: risks forwarding Media Hub authorization across hosts and recreates the rejected playback-route behavior.
- Replace Emby immediately: direct decoding does not provide media indexing, episode organization, progress, subtitles, or transcoding.

## Consequences

- Direct playback can work independently of Emby for individually selected 115 files.
- Real-device acceptance must prove initial playback, Range seek, matching User-Agent, and failure-triggered re-resolution before the feature is declared production-ready.
- Unsupported codecs remain a client capability error because Media Hub does not transcode.
- Upstream API or response-schema changes can break direct playback without affecting transfer and Emby workflows.
