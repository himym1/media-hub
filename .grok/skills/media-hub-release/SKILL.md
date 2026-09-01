---
name: media-hub-release
description: >-
  Commit, tag, package, and deploy Media Hub: GitHub release builds the
  signed Android APK first, then the NAS Docker image, then compose up.
  Use when the user says 提交打包部署, 发版, release Media Hub, deploy media hub,
  发布 0.x, 更新 NAS, or runs /media-hub-release. Do not create a second
  release skill — the hang is docker layer cache, not missing docs.
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
- After a user-facing fix is complete, ship the next patch without asking.

## Where releases actually stall

The long wait is the self-hosted NAS job `.github/workflows/release.yml`, not `git push`.

1. CI used to rewrite the backend `RUN go mod download` line on every tag. That cache-busts the module layer even when `go.mod` / `go.sum` did not change.
2. `proxy.golang.org` and `sum.golang.org` time out from the NAS. `goproxy.cn` IPv4 resolves to an overseas CDN and crawls when the build container does not use Clash. Use `https://mirrors.aliyun.com/goproxy/,direct`. Clash mixed port `:7890` is an HTTP proxy, not a GOPROXY; pass it as backend `HTTP_PROXY` when it is listening. `gcr.io` distroless cannot be pulled; swap only the final stage to `alpine:3.20` + uid `65532`.
3. The image step is wrapped in `timeout 20m`. A stalled `go mod download` sits the full 20 minutes, then exits 124, and used to skip the APK as well.

A new skill will not make module downloads faster. Keep one skill (this file). Do not rewrite the module `RUN` in CI or in a manual NAS build. Pass `--build-arg GOPROXY=https://mirrors.aliyun.com/goproxy/,direct --build-arg GOSUMDB=off` and `--network=host`. If `:7890` is up, also pass `HTTP_PROXY` / `HTTPS_PROXY` so Go uses Clash. After the first cached module layer, later tags should reuse it.

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

GitHub Actions workflow `.github/workflows/release.yml` on `v*.*.*` (self-hosted `[self-hosted, linux, x64, media-hub]`):

1. Signed APK + `latest.json` into `/volume1/docker/media-hub/releases` first, so a hung image does not block the app update
2. `docker build --network=host` with Aliyun GOPROXY / GOSUMDB build-args and Clash `HTTP_PROXY` when `:7890` is up; only the final distroless stage is rewritten to alpine
3. NAS `backup.sh` + `MEDIA_HUB_IMAGE_TAG` compose up + local health on `:18080`

```bash
gh run watch --exit-status $(gh run list --workflow=release.yml --branch "v$VERSION" --limit 1 --json databaseId -q '.[0].databaseId')
```

If the job is still in `Build amd64 image on NAS` after a few minutes and `go.mod` did not change, the module layer did not cache. Do not sit the full 20 minutes guessing. Inspect the runner log. Do not rewrite the module `RUN` as a "fix".

`workflow_dispatch` with `skip_image=true` is APK-only. Use that when the image is already on the NAS.

## 5. Deploy NAS

If the compose step in CI succeeded, skip this section. Only smoke the public origin.

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

Last resort if Actions export is unavailable: build the tagged source on the NAS (amd64, Docker legacy builder, no `buildx`/`--progress`). Swap only distroless → alpine. Do not rewrite the Go module `RUN`.

```bash
git archive --format=tar "v$VERSION" | ssh himym 'set -euo pipefail
rm -rf /tmp/media-hub-src && mkdir -p /tmp/media-hub-src
tar -C /tmp/media-hub-src -xf -
python3 - <<"PY"
from pathlib import Path
p = Path("/tmp/media-hub-src/Dockerfile")
t = p.read_text()
t = t.replace(
    "FROM gcr.io/distroless/static-debian12:nonroot\n",
    "FROM alpine:3.20\nRUN apk add --no-cache ca-certificates tzdata \\\n && addgroup -g 65532 -S nonroot \\\n && adduser -u 65532 -S -G nonroot -H -D nonroot\n",
    1,
)
p.write_text(t)
PY
cd /tmp/media-hub-src
docker build --network=host --build-arg VERSION=v'"$VERSION"' \
  --build-arg GOPROXY=https://mirrors.aliyun.com/goproxy/,direct --build-arg GOSUMDB=off \
  --build-arg HTTP_PROXY=http://127.0.0.1:7890 \
  --build-arg HTTPS_PROXY=http://127.0.0.1:7890 \
  --build-arg NO_PROXY=localhost,127.0.0.1 \
  -t ghcr.io/himym1/media-hub:'"$VERSION"' .
cd /volume1/docker/media-hub
MEDIA_HUB_IMAGE_TAG='"$VERSION"' docker compose up -d
curl --fail http://127.0.0.1:18080/api/v1/health
rm -rf /tmp/media-hub-src
'
```

`proxy.golang.org` and `gcr.io` time out from the NAS. Use Aliyun GOPROXY plus Clash mixed port; do not set `GOPROXY` to the Clash URL. `alpine:3.20` is already on the NAS; keep UID/GID `65532` so compose `user:` still matches. Report this fallback in the release note — it is not the GHCR distroless image.

If `.env` already contains `MEDIA_HUB_IMAGE_TAG`, edit that file so the next reboot keeps the tag.

## 6. Place Android APK

If CI published the APK, skip this section.

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

APK-only retry:

```bash
gh workflow run release.yml -f version="$VERSION" -f skip_image=true
```

## 7. Smoke

- `curl --fail https://media.himym.us.ci/api/v1/health`
- Confirm Web loads and login still works (do not log secrets)
- Confirm `/api/v1/client/android/releases/latest` returns the new `versionCode` when authenticated

## Report

Version, commit SHA, tag, image digest if available, NAS health, APK path, and anything not deployed. If the image is NAS alpine, say so — it is not the GHCR distroless image.
