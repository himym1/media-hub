# QMediaSync Emby Compatibility Proxy

This sidecar is a narrow compatibility layer for QMediaSync `v0.14.23` and Emby servers that return a JSON error string when `Items/{id}/PlaybackInfo` is requested without a body.

It changes only empty-body requests whose path ends in `/PlaybackInfo`:

- method becomes `POST`
- body becomes `{}`
- `Content-Type` becomes `application/json`

Every other method, path, query, authorization header, request body, response, and media stream is passed through unchanged. Media Hub still consumes only external redirect headers and never proxies media bytes.

## Path Contract

`QMS_EMBY_UPSTREAM` is the exact current Emby base URL. It may contain one base path, such as `/emby`; `httputil.ReverseProxy` joins that base path with QMediaSync's request path.

QMediaSync `emby_config.emby_url` must point to the sidecar origin without copying the upstream base path:

```text
QMS_EMBY_UPSTREAM=http://emby-host:8096/emby
emby_config.emby_url=http://himym-qms-emby-compat:18096
```

The resulting upstream request is `/emby/Items/{id}/PlaybackInfo`, never `/emby/emby/Items/...`. Production currently uses an upstream without a base path: `http://192.168.8.146:58096`.

## Verify and Build

Run only the focused module checks:

```sh
cd deploy/qmediasync-compat
go test ./...
docker compose -f compose.yaml config --quiet
QMS_EMBY_UPSTREAM=http://192.168.8.146:58096 docker compose -f compose.yaml build
```

The tests require exact preservation of query parameters and `X-Emby-Token`, exact path joining, `{}` injection for empty PlaybackInfo requests, and no changes to existing bodies or unrelated requests.

## Production Cutover

Preconditions:

- QMediaSync version is exactly `v0.14.23`.
- QMediaSync, Postgres, Emby, and Media Hub are healthy.
- `syncs` has no status `0` or `1` rows and upload/download task tables have no active rows.
- Record the current non-secret `emby_config.emby_url`; do not print API keys or other configuration.

Deploy the sidecar on the existing external QMediaSync network, then atomically replace only `emby_config.id=1`'s URL and restart QMediaSync. The sidecar must become healthy before the database update.

```sh
cd /volume1/docker/qmediasync-compat
QMS_EMBY_UPSTREAM=http://192.168.8.146:58096 docker compose -f compose.yaml up -d --build

# Run inside the Postgres container so existing credentials remain internal.
docker exec himym-qmediasync-postgres sh -c \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -c \
  "UPDATE emby_config SET emby_url = '\''http://himym-qms-emby-compat:18096'\'', updated_at = EXTRACT(EPOCH FROM NOW())::bigint \
   WHERE id = 1 AND emby_url = '\''http://192.168.8.146:58096'\'';"'

cd /volume1/docker/qmediasync-build
docker compose up -d --no-deps --force-recreate qmediasync
```

Acceptance requires all of the following:

- sidecar is Docker `healthy`
- QMediaSync main HTTP returns `200`
- QMediaSync emby302 root returns `302`
- Media Hub remains `v0.18.0` and Docker `healthy`
- a real movie or episode descriptor resolves to an external HTTPS URL
- QMediaSync logs no longer contain the prior `Object reference not set` PlaybackInfo parse error for that request

Never print or persist the resolved media URL during validation.

## Rollback

Restore the original Emby URL first, restart the official QMediaSync image, then remove the sidecar:

```sh
docker exec himym-qmediasync-postgres sh -c \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -c \
  "UPDATE emby_config SET emby_url = '\''http://192.168.8.146:58096'\'', updated_at = EXTRACT(EPOCH FROM NOW())::bigint \
   WHERE id = 1 AND emby_url = '\''http://himym-qms-emby-compat:18096'\'';"'

cd /volume1/docker/qmediasync-build
docker compose up -d --no-deps --force-recreate qmediasync
cd /volume1/docker/qmediasync-compat
docker compose -f compose.yaml down
```

No database schema or media data is changed.

## Rejected Binary Patch

A three-byte QMediaSync binary patch that changed the first empty PlaybackInfo request from POST to GET was tested in production and rejected: the current Emby server returns the same JSON error for empty GET and empty POST requests. That binary and its patch recipe are intentionally not stored in this repository.
