# Resource Source Adapter Contract

Media Hub treats resource providers as private adapters with one normalized JSON contract. Provider cookies, share codes, direct links, and provider-specific payloads stay behind the adapter and must never appear in Media Hub logs or public API responses.

## Authentication

When `MEDIA_HUB_SOURCE_<NAME>_TOKEN` is configured, Media Hub sends:

```http
Authorization: Bearer <token>
```

Adapters must serve HTTPS outside a trusted local network. Media Hub does not follow redirects, so credentials cannot be forwarded to another origin.

Canonical project-owned adapter IDs are `dian`, `framehdr`, `gimy`, `guanying`, `hdhive`, `juying`, `mikan`, and `sidhub`. Their configuration keys are `MEDIA_HUB_SOURCE_<UPPERCASE_ID>_URL` and `MEDIA_HUB_SOURCE_<UPPERCASE_ID>_TOKEN`. Legacy `FRAME` and `GATHER` keys map to `framehdr` and `juying`; migration `0009_source_ids.sql` normalizes saved subscription rules.

## Search

```http
GET <base-url>/search?query=<text>&limit=50
Accept: application/json
```

```json
{
  "results": [
    {
      "id": "provider-stable-release-id",
      "title": "Van Helsing",
      "year": 2004,
      "mediaType": "movie",
      "tmdbId": "7131",
      "posterUrl": "https://image.example/poster.jpg",
      "release": {
        "resolution": "2160p",
        "videoCodec": "HEVC",
        "dynamicRange": "HDR10",
        "audio": "DTS:X 7.1",
        "sizeBytes": 64424509440
      },
      "reference": "private-provider-reference"
    }
  ]
}
```

`reference` is encrypted into an opaque short-lived selection token and is never returned directly to Web or Android. `tmdbId` is optional at the adapter boundary, but Media Hub must independently verify identity through TMDB before enabling transfer. Client-visible poster art is taken from the verified TMDB identity rather than an arbitrary adapter URL.

For series resources, adapters may additionally return `season`, `episodeStart`, and `episodeEnd`. Episode bounds must either all be omitted/zero or satisfy `1 <= episodeStart <= episodeEnd <= 10000`; movie results must not carry season or episode values. Media Hub persists this range, suppresses already completed ranges, and verifies every target episode through Emby before completion.

## Start Transfer

```http
POST <base-url>/transfer
Content-Type: application/json
Idempotency-Key: <stable-job-operation-key>
```

```json
{
  "reference": "private-provider-reference",
  "destinationId": "115-directory-id"
}
```

An adapter must bind each idempotency key to the canonical request. Reusing a key for a different request returns `409`. Repeating the same request returns the original operation/result.

A completed response:

```json
{
  "operationId": "operation-id",
  "status": "completed",
  "fileId": "115-file-or-directory-id",
  "path": "/resolved/115/path",
  "isFile": false
}
```

An asynchronous response:

```json
{
  "operationId": "operation-id",
  "status": "pending",
  "fileId": "",
  "path": "",
  "isFile": false
}
```

## Transfer Status

```http
GET <base-url>/transfer/<operation-id>
Accept: application/json
```

The response uses the same shape and returns either `pending` or `completed`. Media Hub rejects redirects, oversized responses, unknown states, malformed operation IDs, and completed responses without both `fileId` and `path`.

## Errors

Adapters return a stable JSON body when possible:

```json
{
  "code": "provider_rate_limited",
  "message": "Provider is temporarily rate limited",
  "retryable": true
}
```

Media Hub maps authentication, rate limiting, conflicts, unavailable providers, and invalid responses to sanitized application errors. Raw upstream bodies and URLs are not exposed.
