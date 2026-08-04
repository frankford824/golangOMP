#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: freeze-production-snapshot.sh <output.sql.gz> [ssh-host]

Streams one MySQL --single-transaction production snapshot over SSH directly
to the requested local file. The remote command only reads the database and
does not create a server-side backup file. Credentials remain in the remote
environment and are never printed.
EOF
}

if [[ $# -lt 1 || $# -gt 2 ]]; then
  usage >&2
  exit 64
fi

output_path=$1
ssh_host=${2:-jst_ecs}
output_dir=$(dirname -- "$output_path")
output_name=$(basename -- "$output_path")
partial_path="$output_dir/.${output_name}.partial"

mkdir -p -- "$output_dir"
if [[ -e "$output_path" || -e "$partial_path" ]]; then
  echo "refusing to overwrite existing snapshot: $output_path" >&2
  exit 73
fi

cleanup() {
  if [[ -e "$partial_path" ]]; then
    rm -f -- "$partial_path"
  fi
}
trap cleanup EXIT

ssh "$ssh_host" 'set -euo pipefail
  cd /root/ecommerce_ai
  . ./shared/main.env
  export MYSQL_PWD="$DB_PASS"
  mysqldump \
    -h"$DB_HOST" \
    -P"${DB_PORT:-3306}" \
    -u"$DB_USER" \
    --single-transaction \
    --quick \
    --routines \
    --triggers \
    --events \
    --hex-blob \
    --set-gtid-purged=OFF \
    --no-tablespaces \
    --source-data=2 \
    "$DB_NAME" \
  | gzip -1' >"$partial_path"

test -s "$partial_path"
mv -- "$partial_path" "$output_path"
trap - EXIT

sha256sum -- "$output_path"
