# Product Vision

## Positioning

Media Hub is a private media operations console for one household. It turns a fragmented workflow across search sites, 115, QMediaSync, Emby, and enterprise WeChat into one observable workflow.

It is not a media server and does not carry video traffic.

## Primary User

The MVP has one administrator who:

- searches for a movie or series;
- compares available releases;
- transfers one release to the correct 115 directory;
- expects it to appear in Emby without manual intervention;
- wants a precise failure reason when any stage fails;
- primarily uses Android for discovery and status checks;
- uses Web for configuration and detailed operations.

## Core Promise

After one transfer action, the user can see exactly where the resource is in the pipeline:

```text
Discovered
-> queued
-> transferred to 115
-> STRM generated
-> indexed by Emby
-> playback ready
```

## MVP Capabilities

- Search aggregation with normalized release metadata.
- TMDB identity matching for movie versus series routing.
- Explicit release selection and 115 destination preview.
- Durable workflow jobs with retry and audit history.
- QMediaSync synchronization trigger and completion checks.
- Emby library refresh, duplicate checks, and indexing verification.
- Playback readiness probe without proxying the media body.
- Enterprise WeChat application notifications.
- Responsive Web interface, native Android app, and a Windows desktop client when the browser cannot decode the stream.

## Non-goals

- No PT downloads or MoviePilot integration.
- No local NAS instant-upload scanner.
- No built-in video player in the MVP.
- No iOS, Flutter, or public app-store release. Windows via Tauri is in scope when native decode is required.
- No multi-tenant accounts, payments, or public registration.
- No replacement for QMediaSync or Emby.
- No credential extraction from browser sessions.
- No attempt to support every resource source in the first release.

## Success Criteria

A first release is successful when:

1. A search can return normalized candidates from at least two sources.
2. A movie and a series route to different 115 directories automatically.
3. Repeating the same transfer does not create duplicate side effects.
4. Every failed stage has a visible, actionable reason and a safe retry.
5. A transferred item reaches Emby and passes a playback redirect probe.
6. Web and Android show the same workflow state from the same API.

## Quality Priorities

In order:

1. Correctness and recoverability.
2. Observable workflow state.
3. Security of provider credentials.
4. Mobile usability.
5. Maintainability of source adapters.
6. Raw throughput.

The expected scale is one user, tens of searches per day, and a small number of concurrent transfer workflows. This does not justify microservices, Redis, Kafka, or distributed orchestration.
