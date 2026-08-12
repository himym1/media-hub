#!/bin/sh
set -eu

root=${1:-/volume1/docker/media-hub}
backup=${2:?usage: restore.sh ROOT BACKUP_DIRECTORY}

test -f "$backup/media-hub.db"
docker compose -f "$root/compose.yaml" --project-directory "$root" down
cp -p "$backup/media-hub.db" "$root/data/media-hub.db"
if [ -f "$backup/media-hub.env" ]; then
  cp -p "$backup/media-hub.env" "$root/.env"
  chmod 600 "$root/.env"
fi
docker compose -f "$root/compose.yaml" --project-directory "$root" up -d
