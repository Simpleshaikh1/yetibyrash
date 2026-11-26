#!/bin/sh
# Restores a backup into the database at RESTORE_DATABASE_URL.
#
#   restore.sh                      # the newest backup
#   restore.sh yetibyrash-2026-12-01T0200Z.sql.gz
#
# Restore into a NEW, empty database first and check it before pointing the shop at it.
# Needs the same R2 settings as backup.sh, plus RESTORE_DATABASE_URL.
set -eu

: "${RESTORE_DATABASE_URL:?RESTORE_DATABASE_URL is not set (the database to restore INTO)}"
: "${BACKUP_BUCKET:?BACKUP_BUCKET is not set}"

export RCLONE_CONFIG_R2_TYPE=s3
export RCLONE_CONFIG_R2_PROVIDER="${R2_PROVIDER:-Cloudflare}"
export RCLONE_CONFIG_R2_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID"
export RCLONE_CONFIG_R2_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY"
export RCLONE_CONFIG_R2_ENDPOINT="${R2_ENDPOINT:-https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com}"
export RCLONE_CONFIG_R2_NO_CHECK_BUCKET=true

NAME="${1:-}"
if [ -z "$NAME" ]; then
  NAME=$(rclone lsf "r2:${BACKUP_BUCKET}/db/" | sort | tail -n 1)
fi
[ -n "$NAME" ] || { echo "No backups found." >&2; exit 1; }

echo "Restoring $NAME ..."
rclone cat "r2:${BACKUP_BUCKET}/db/${NAME}" | gunzip | psql --quiet --set ON_ERROR_STOP=1 "$RESTORE_DATABASE_URL"
echo "Restored. Quick check:"
psql "$RESTORE_DATABASE_URL" -c "SELECT
  (SELECT COUNT(*) FROM products) AS products,
  (SELECT COUNT(*) FROM orders)   AS orders,
  (SELECT COUNT(*) FROM staff)    AS staff,
  (SELECT MAX(version_id) FROM goose_db_version) AS migration;"
