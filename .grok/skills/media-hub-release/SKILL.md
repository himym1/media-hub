---
name: media-hub-release
description: >-
  Commit, tag, package, and deploy Media Hub: GitHub release builds the
  Docker image (Web+API) and signed Android APK, then NAS compose is upgraded.
  Use when the user says 提交打包部署, 发版, release Media Hub, deploy media hub,
  发布 0.x, 更新 NAS, or runs /media-hub-release.
---

# Media Hub release

One private release path. Web is baked into the image. Android is the signed APK from the same tag.

## Facts

- Repo remote: `origin` = `https://github.com/himym1/media-hub.git`
- Image: `ghcr.io/himym1/media-hub:<version>` (never `latest`)
- Tag: `vMAJOR.MINOR.PATCH` on `main`
- NAS SSH: `himym`
- NAS dir: `/volume1/docker/media-hub`
- Public origin: `https://media.himym.us.ci`
- Android `versionCode` = `major * 1000000 + minor * 1000 + patch` (example `0.9.0` → `9000`)
- APK name on NAS: `releases/media-hub-<versionCode>.apk` plus `releases/latest.json`
- Do not commit `.env`, keystores, tokens, cookies, or NAS exports
- Do not drop SQLite tables
- Backup NAS before `compose up`

## 1. Version

```bash
git fetch --tags origin
git tag --sort=-v:refname | head -5
```

Pick the next semver. Breaking API or removed client features → minor or major. Otherwise patch.

Write the version into:

- `api/openapi.yaml` → `info.version`
- `android/app/build.gradle.kts` defaults `MEDIA_HUB_VERSION_NAME` / `MEDIA_HUB_VERSION_CODE` (local fallback only; CI derives from the tag)

Leave `backend/cmd/server/main.go` `version` as `X.Y.Z-dev`. The image build sets `-X main.version=vX.Y.Z` from the tag.

## 2. Verify

From repo root:

```bash
go -C backend test ./...
go -C backend vet ./...
pnpm --dir web test
pnpm --dir web lint
pnpm --dir web build
cd android && ./gradlew :app:testDebugUnitTest :app:assembleDebug :app:lintDebug
ruby scripts/check-openapi.rb api/openapi.yaml
```

Stop on the first failure.

## 3. Commit and tag

Conventional commit. Typical release:

```text
feat: <user-facing summary>

- <web>
- <android>
- <backend/api>
```

```bash
git add -A
git status
git diff --cached --stat
git commit -m "$(cat <<'EOF'
feat: ...

- ...
EOF
)"
git push origin HEAD:main
git tag "v$VERSION"
git push origin "v$VERSION"
```

Do not amend a pushed release tag.

## 4. Package

GitHub Actions workflow `.github/workflows/release.yml` on `v*.*.*`:

- image job → `ghcr.io/himym1/media-hub:$VERSION` (amd64+arm64)
- android job → signed APK + `latest.json` attached to the GitHub release

```bash
gh run watch --exit-status $(gh run list --workflow=release.yml --branch "v$VERSION" --limit 1 --json databaseId -q '.[0].databaseId')
gh release view "v$VERSION"
```

Wait until both jobs succeed and the release has `media-hub-<code>.apk` and `latest.json`.

## 5. Deploy NAS

NAS `himym` has no GHCR credentials by default. `docker compose pull` returns `unauthorized` until a `read:packages` token is logged in:

```bash
printf '%s' "$GHCR_READ_TOKEN" | ssh himym 'docker login ghcr.io -u himym1 --password-stdin'
```

Keep that token on the NAS docker config, not in git. Then:

```bash
ssh himym 'set -euo pipefail
cd /volume1/docker/media-hub
./backup.sh
grep -E "^MEDIA_HUB_IMAGE_TAG=" .env >/dev/null && \
  sed -i.bak -E "s/^MEDIA_HUB_IMAGE_TAG=.*/MEDIA_HUB_IMAGE_TAG='"$VERSION"'/" .env || \
  printf "\nMEDIA_HUB_IMAGE_TAG=%s\n" "'"$VERSION"'" >> .env
MEDIA_HUB_IMAGE_TAG='"$VERSION"' docker compose pull
MEDIA_HUB_IMAGE_TAG='"$VERSION"' docker compose up -d
docker compose ps
curl --fail http://127.0.0.1:18080/api/v1/health
'
```

If pull is unauthorized, export the official `linux/amd64` image from GitHub Actions (same repo token can read GHCR) and load it:

```bash
gh workflow run export-private-image.yml -f version="$VERSION"
gh run watch --exit-status $(gh run list --workflow=export-private-image.yml --limit 1 --json databaseId -q '.[0].databaseId')
art=$(mktemp -d)
gh run download $(gh run list --workflow=export-private-image.yml --limit 1 --json databaseId -q '.[0].databaseId') -n "media-hub-$VERSION-linux-amd64" -D "$art"
gzip -dc "$art/image.tar.gz" | ssh himym 'docker load'
ssh himym 'cd /volume1/docker/media-hub && MEDIA_HUB_IMAGE_TAG='"$VERSION"' docker compose up -d && curl --fail http://127.0.0.1:18080/api/v1/health'
```

Last resort if Actions export is unavailable: build the tagged source on the NAS (amd64, Docker legacy builder, no `buildx`/`--progress`):

```bash
git archive --format=tar "v$VERSION" | ssh himym 'set -euo pipefail
rm -rf /tmp/media-hub-src && mkdir -p /tmp/media-hub-src
tar -C /tmp/media-hub-src -xf -
python3 - <<"PY"
from pathlib import Path
p = Path("/tmp/media-hub-src/Dockerfile")
t = p.read_text()
t = t.replace(
    "FROM golang:1.25-bookworm AS backend\n",
    "FROM golang:1.25-bookworm AS backend\nENV GOPROXY=https://goproxy.cn,direct\nENV GOSUMDB=off\n",
    1,
)
t = t.replace(
    "FROM gcr.io/distroless/static-debian12:nonroot\n",
    "FROM alpine:3.20\nRUN apk add --no-cache ca-certificates tzdata \\\n && addgroup -g 65532 -S nonroot \\\n && adduser -u 65532 -S -G nonroot -H -D nonroot\n",
    1,
)
p.write_text(t)
PY
cd /tmp/media-hub-src
docker build --build-arg VERSION=v'"$VERSION"' -t ghcr.io/himym1/media-hub:'"$VERSION"' .
cd /volume1/docker/media-hub
MEDIA_HUB_IMAGE_TAG='"$VERSION"' docker compose up -d
curl --fail http://127.0.0.1:18080/api/v1/health
rm -rf /tmp/media-hub-src
'
```

`proxy.golang.org` and `gcr.io` time out from the NAS. `goproxy.cn` works. `alpine:3.20` is already on the NAS; keep UID/GID `65532` so compose `user:` still matches. Report this fallback in the release note — it is not the GHCR distroless image.

If `.env` already contains `MEDIA_HUB_IMAGE_TAG`, edit that file so the next reboot keeps the tag.

## 6. Place Android APK

NAS SFTP cannot `mkdir` `/volume1/docker/media-hub/releases/` (the dir exists; `scp` still fails). Pipe files over SSH:

```bash
tmp=$(mktemp -d)
gh release download "v$VERSION" -D "$tmp" -p "media-hub-*.apk" -p latest.json
apk=$(echo "$tmp"/media-hub-*.apk)
ssh himym "cat > /volume1/docker/media-hub/releases/$(basename "$apk")" < "$apk"
ssh himym 'cat > /volume1/docker/media-hub/releases/latest.json' < "$tmp/latest.json"
ssh himym 'chmod 444 /volume1/docker/media-hub/releases/media-hub-*.apk /volume1/docker/media-hub/releases/latest.json
ls -l /volume1/docker/media-hub/releases'
```

## 7. Smoke

- `curl --fail https://media.himym.us.ci/api/v1/health`
- Confirm Web loads and login still works (do not log secrets)
- Confirm `/api/v1/client/android/releases/latest` returns the new `versionCode` when authenticated

## Report

Version, commit SHA, tag, image digest if available, NAS health, APK path, and anything not deployed.
