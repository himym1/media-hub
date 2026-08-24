#!/bin/sh
set -eu

root=${1:-/volume1/docker/media-hub}
mkdir -p "$root/data" "$root/releases" "$root/backups" "$root/tunnel"
chmod 700 "$root/data" "$root/releases" "$root/backups" "$root/tunnel"

uid=${MEDIA_HUB_UID:-65532}
gid=${MEDIA_HUB_GID:-65532}
chown "$uid:$gid" "$root/data"

install -m 644 deploy/compose.yaml "$root/compose.yaml"
install -m 644 deploy/compose.mikan-egress.yaml "$root/compose.mikan-egress.yaml"
install -m 644 deploy/compose.strm.yaml "$root/compose.strm.yaml"
install -m 755 deploy/backup.sh "$root/backup.sh"
install -m 755 deploy/restore.sh "$root/restore.sh"

if [ ! -f "$root/.env" ]; then
  printf '%s\n' "Create $root/.env from backend/.env.example and chmod 600 before deployment." >&2
fi
