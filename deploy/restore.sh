#!/bin/sh
set -eu

root=${1:-/volume1/docker/media-hub}
backup=${2:?usage: restore.sh ROOT BACKUP_DIRECTORY}
cd "$root"

test -f "$backup/media-hub.db"
docker compose down
cp -p "$backup/media-hub.db" "$root/data/media-hub.db"
if [ -f "$backup/media-hub.env" ]; then
  cp -p "$backup/media-hub.env" "$root/.env"
  chmod 600 "$root/.env"
fi
if [ -f "$backup/compose.mikan-egress.yaml" ]; then
  cp -p "$backup/compose.mikan-egress.yaml" "$root/compose.mikan-egress.yaml"
fi
docker compose up -d
