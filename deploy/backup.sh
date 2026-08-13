#!/bin/sh
set -eu

root=${1:-/volume1/docker/media-hub}
cd "$root"
stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup="$root/backups/$stamp"
mkdir -p "$backup"

running=false
if docker compose ps --status running --services | grep -qx media-hub; then
  running=true
  docker compose stop media-hub
fi
trap 'if [ "$running" = true ]; then docker compose start media-hub; fi' EXIT

if [ -f "$root/data/media-hub.db" ]; then
  cp -p "$root/data/media-hub.db" "$backup/media-hub.db"
fi
if [ -f "$root/.env" ]; then
  cp -p "$root/.env" "$backup/media-hub.env"
  chmod 600 "$backup/media-hub.env"
fi
cp -p "$root/compose.yaml" "$backup/compose.yaml"
if [ -f "$root/compose.mikan-egress.yaml" ]; then
  cp -p "$root/compose.mikan-egress.yaml" "$backup/compose.mikan-egress.yaml"
fi

if [ "$running" = true ]; then
  docker compose start media-hub
  running=false
fi
printf '%s\n' "Backup created at $backup"
