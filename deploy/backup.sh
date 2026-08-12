#!/bin/sh
set -eu

root=${1:-/volume1/docker/media-hub}
stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup="$root/backups/$stamp"
mkdir -p "$backup"

running=false
if docker compose -f "$root/compose.yaml" --project-directory "$root" ps --status running --services | grep -qx media-hub; then
  running=true
  docker compose -f "$root/compose.yaml" --project-directory "$root" stop media-hub
fi
trap 'if [ "$running" = true ]; then docker compose -f "$root/compose.yaml" --project-directory "$root" start media-hub; fi' EXIT

if [ -f "$root/data/media-hub.db" ]; then
  cp -p "$root/data/media-hub.db" "$backup/media-hub.db"
fi
if [ -f "$root/.env" ]; then
  cp -p "$root/.env" "$backup/media-hub.env"
  chmod 600 "$backup/media-hub.env"
fi
cp -p "$root/compose.yaml" "$backup/compose.yaml"

if [ "$running" = true ]; then
  docker compose -f "$root/compose.yaml" --project-directory "$root" start media-hub
  running=false
fi
printf '%s\n' "Backup created at $backup"
