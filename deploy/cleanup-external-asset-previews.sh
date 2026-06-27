#!/usr/bin/env bash
# Dry-run first cleanup helper for legacy external asset OSS preview objects.
# It exports DB rows before any delete and only deletes when --execute is passed.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT_DIR="$ROOT/dist/external-asset-cleanup"
EXECUTE=false
MYSQL_CMD="${MYSQL_CMD:-mysql}"
OSSUTIL_CMD="${OSSUTIL_CMD:-ossutil64}"
OSS_BUCKET_URI="${OSS_BUCKET_URI:-}"

usage() {
  echo "Usage: $0 --bucket oss://bucket [--execute] [--out DIR]"
  echo "  Env: MYSQL_DSN or MYSQL_PWD/mysql defaults via MYSQL_CMD; OSSUTIL_CMD; OSS_BUCKET_URI"
  echo "  Default mode is dry-run: export rows and print object keys only."
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bucket) OSS_BUCKET_URI="$2"; shift ;;
    --execute) EXECUTE=true ;;
    --out) OUT_DIR="$2"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage; exit 1 ;;
  esac
  shift
done

die() { echo "[external-preview-cleanup] ERROR: $*" >&2; exit 1; }
step() { echo "[external-preview-cleanup] $*"; }

command -v "$MYSQL_CMD" >/dev/null || die "mysql client is required"
mkdir -p "$OUT_DIR"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
RECORDS_TSV="$OUT_DIR/external_asset_records_${TS}.tsv"
RUNS_TSV="$OUT_DIR/external_asset_sync_runs_${TS}.tsv"
KEYS_TXT="$OUT_DIR/external_preview_keys_${TS}.txt"

step "Export external_asset_records -> $RECORDS_TSV"
"$MYSQL_CMD" --batch --raw --execute \
  "SELECT * FROM external_asset_records ORDER BY id" > "$RECORDS_TSV"

step "Export external_asset_sync_runs -> $RUNS_TSV"
"$MYSQL_CMD" --batch --raw --execute \
  "SELECT * FROM external_asset_sync_runs ORDER BY id" > "$RUNS_TSV"

step "Export ready preview keys -> $KEYS_TXT"
"$MYSQL_CMD" --batch --raw --skip-column-names --execute \
  "SELECT oss_preview_key FROM external_asset_records WHERE COALESCE(oss_preview_key, '') <> '' ORDER BY id" > "$KEYS_TXT"

COUNT="$(wc -l < "$KEYS_TXT" | tr -d ' ')"
step "Preview key count: $COUNT"

if [[ "$COUNT" == "0" ]]; then
  step "Nothing to clean."
  exit 0
fi

if [[ "$EXECUTE" != "true" ]]; then
  step "Dry-run complete. Review $KEYS_TXT, then rerun with --execute to delete preview objects only."
  exit 0
fi

[[ -n "$OSS_BUCKET_URI" ]] || die "--bucket or OSS_BUCKET_URI is required for execute mode"
command -v "$OSSUTIL_CMD" >/dev/null || die "ossutil64 is required for execute mode"

step "Deleting preview objects from $OSS_BUCKET_URI"
while IFS= read -r key; do
  [[ -n "$key" ]] || continue
  "$OSSUTIL_CMD" rm "${OSS_BUCKET_URI%/}/$key"
done < "$KEYS_TXT"

step "Deleted preview objects listed in $KEYS_TXT. Original objects were not touched."
