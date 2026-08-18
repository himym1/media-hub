# ADR 0006: Emby-Managed Library Playback in Media3

- Status: Accepted, production playback probe pending
- Date: 2026-08-18

## Context

Android direct playback was initially exposed from the 115 operations screen. That made a diagnostic file browser the primary user journey and left Emby library organization disconnected from the Media3 player. The Android shell also allowed global, system, and detail chrome to stack on one screen.

Emby remains the source of library identity, movie/series/episode organization, resume position, watched state, and playback history. Media Hub must not expose the Emby API key or proxy video bytes.

## Decision

- The normal Android flow is Emby library -> movie or episode target -> Media3.
- Playback uses typed `Drive115Target` and `EmbyItemTarget` resolver interfaces. Provider adapters do not depend on each other.
- An Emby target is direct-playable only when `PlaybackInfo` exposes an external HTTPS URL or the deployment-configured QMediaSync `emby302` facade returns one. The ordinary Emby base URL remains authoritative for library reads and progress; `MEDIA_HUB_EMBY_PLAYBACK_URL` is an immutable deployment route used only for redirect headers.
- Emby credentials remain server-side. Media Hub returns no Emby API key and proxies no video payload.
- An Emby descriptor may contain a user-bound opaque playback session and `startPositionMs`. The Android player reports ordered started, progress, paused, and stopped events; Media Hub forwards them to Emby `Sessions/Playing*` endpoints.
- Playback sessions are in-memory, expire after 24 hours, are removed on stop, and are opportunistically pruned during session creation.
- Series details expose playable episodes with episode-specific Emby fallback URLs. A series itself is never sent to Media3 as a playable item.
- Primary poster images use a separate authenticated, 2 MiB-bounded image endpoint. Android loads them through a four-request bounded loader with a 20-image decoded LRU. Image bytes do not enter screen StateFlow.
- The workspace owns the selected library item ID. Detail mode is derived directly from that ID and hides global top and bottom navigation, leaving one detail toolbar.
- System settings and operations share a separate system layer with explicit back navigation and a Services/Operations segmented control.

## Consequences

- The 115 operations player remains available for diagnostics but is not the normal media-consumption entry.
- A failed direct resolution presents a validated Emby fallback in the player error state.
- Media Hub still does not transcode. QMediaSync `emby302` reads STRM and emits the upstream redirect; Media Hub consumes only the redirect header and never proxies media bytes. Unsupported codecs and sources without a safe external redirect require Emby playback.
- Production readiness requires a real movie and episode probe through QMediaSync/Emby, including Range seek, resume position, and Emby progress updates.
