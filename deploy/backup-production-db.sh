#!/usr/bin/env bash
# Create a consistent, compressed pre-deploy MySQL backup on the production
# host. Intended for deploy-on-host.sh and the self-hosted release workflow.
set -euo pipefail

BASE_DIR="/root/ecommerce_ai"
VERSION=""
KEEP_BACKUPS="3"

while [ $# -gt 0 ]; do
  case "$1" in
    --base-dir) BASE_DIR="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --keep) KEEP_BACKUPS="$2"; shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

[ -n "$VERSION" ] || { echo "--version is required" >&2; exit 1; }
[[ "$VERSION" =~ ^v[0-9]+\.[0-9]+$ ]] || { echo "invalid version: $VERSION" >&2; exit 1; }
[[ "$KEEP_BACKUPS" =~ ^[1-9][0-9]*$ ]] || { echo "--keep must be a positive integer" >&2; exit 1; }

ENV_FILE="$BASE_DIR/shared/main.env"
[ -f "$ENV_FILE" ] || { echo "missing env: $ENV_FILE" >&2; exit 1; }
command -v mysqldump >/dev/null 2>&1 || { echo "mysqldump is required" >&2; exit 1; }
command -v gzip >/dev/null 2>&1 || { echo "gzip is required" >&2; exit 1; }

set -a
# shellcheck source=/dev/null
. "$ENV_FILE"
set +a

: "${DB_HOST:?DB_HOST is required}"
: "${DB_USER:?DB_USER is required}"
: "${DB_NAME:?DB_NAME is required}"
DB_PORT="${DB_PORT:-3306}"

umask 077
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_root="$BASE_DIR/backups/production-release"
final_path="$backup_root/${timestamp}_${VERSION}_${DB_NAME}.sql.gz"
tmp_path="${final_path}.partial"
mkdir -p "$backup_root"

cleanup() {
  if [ -f "$tmp_path" ]; then
    rm -f -- "$tmp_path"
  fi
}
trap cleanup EXIT INT TERM

export MYSQL_PWD="${DB_PASS:-}"
mysqldump \
  --single-transaction \
  --quick \
  --hex-blob \
  --triggers \
  --set-gtid-purged=OFF \
  --no-tablespaces \
  --default-character-set=utf8mb4 \
  -h"$DB_HOST" \
  -P"$DB_PORT" \
  -u"$DB_USER" \
  "$DB_NAME" | gzip -1 >"$tmp_path"
unset MYSQL_PWD

[ -s "$tmp_path" ] || { echo "backup is empty" >&2; exit 1; }
gzip -t "$tmp_path"
mv "$tmp_path" "$final_path"
sha256sum "$final_path" >"${final_path}.sha256"

mapfile -t backups < <(find "$backup_root" -maxdepth 1 -type f -name '*_v*_*.sql.gz' -printf '%T@ %p\n' | sort -nr | awk '{print $2}')
if [ "${#backups[@]}" -gt "$KEEP_BACKUPS" ]; then
  for old in "${backups[@]:$KEEP_BACKUPS}"; do
    rm -f -- "$old" "${old}.sha256"
  done
fi

printf 'BACKUP_PATH=%s\n' "$final_path"
printf 'BACKUP_SHA256=%s\n' "$(awk '{print $1}' "${final_path}.sha256")"
printf 'BACKUP_BYTES=%s\n' "$(stat -c '%s' "$final_path")"
