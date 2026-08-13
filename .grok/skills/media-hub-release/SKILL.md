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

If `.env` is the source of truth and already contains `MEDIA_HUB_IMAGE_TAG`, prefer editing that file so the next reboot keeps the tag.

## 6. Place Android APK

```bash
tmp=$(mktemp -d)
gh release download "v$VERSION" -D "$tmp" -p "media-hub-*.apk" -p latest.json
scp "$tmp"/media-hub-*.apk "$tmp"/latest.json himym:/volume1/docker/media-hub/releases/
ssh himym 'ls -l /volume1/docker/media-hub/releases'
```

## 7. Smoke

- `curl --fail https://media.himym.us.ci/api/v1/health`
- Confirm Web loads and login still works (do not log secrets)
- Confirm `/api/v1/client/android/releases/latest` returns the new `versionCode` when authenticated

## Report

Version, commit SHA, tag, image digest if available, NAS health, APK path, and anything not deployed.
