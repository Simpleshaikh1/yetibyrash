#!/bin/sh
# Nightly database backup: dumps the whole database, compresses it, and stores it in an
# R2 bucket (separate from Render), keeping the last 30 days. Run by Render as a cron job.
#
# Needs: DATABASE_URL, R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, BACKUP_BUCKET
# Optional: R2_ENDPOINT (another S3 server, for testing), KEEP_DAYS (default 30)
set -eu

: "${DATABASE_URL:?DATABASE_URL is not set}"
: "${BACKUP_BUCKET:?BACKUP_BUCKET is not set}"
KEEP_DAYS="${KEEP_DAYS:-30}"

# rclone reads its R2 settings from these variables (no config file needed).
export RCLONE_CONFIG_R2_TYPE=s3
export RCLONE_CONFIG_R2_PROVIDER="${R2_PROVIDER:-Cloudflare}"
export RCLONE_CONFIG_R2_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID"
export RCLONE_CONFIG_R2_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY"
export RCLONE_CONFIG_R2_ENDPOINT="${R2_ENDPOINT:-https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com}"
export RCLONE_CONFIG_R2_NO_CHECK_BUCKET=true

STAMP=$(date -u +%Y-%m-%dT%H%MZ)
FILE="/tmp/yetibyrash-${STAMP}.sql.gz"

echo "Backing up the database..."
pg_dump --no-owner --no-privileges --clean --if-exists "$DATABASE_URL" | gzip -9 > "$FILE"
SIZE=$(wc -c < "$FILE")
if [ "$SIZE" -lt 1000 ]; then
  echo "Backup looks empty ($SIZE bytes); not uploading." >&2
  exit 1
fi

rclone copyto "$FILE" "r2:${BACKUP_BUCKET}/db/yetibyrash-${STAMP}.sql.gz"
echo "Stored db/yetibyrash-${STAMP}.sql.gz (${SIZE} bytes)"

rclone delete --min-age "${KEEP_DAYS}d" "r2:${BACKUP_BUCKET}/db/"
echo "Removed backups older than ${KEEP_DAYS} days."
rm -f "$FILE"
