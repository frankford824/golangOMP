#!/usr/bin/env bash
# Publish local vue/dist-asset to jst_ecs:/var/www/assets.yongbo.cloud.
# This is intentionally separate from publish-front.sh so the workbench App
# cannot overwrite the main operation system frontend.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FRONT="$ROOT/vue/dist-asset"
SSH_HOST="${SSH_HOST:-jst_ecs}"
REMOTE_WEB="/var/www/assets.yongbo.cloud"
REMOTE_BACKUP_PARENT="/var/www/backups"

SKIP_CHECKS=false
SKIP_VERIFY=false
DRY_RUN=false

usage() {
  echo "Usage: $0 [--skip-checks] [--skip-verify] [--dry-run] [--host HOST]"
  echo "  Env: SSH_HOST (default: jst_ecs)"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-checks) SKIP_CHECKS=true ;;
    --skip-verify) SKIP_VERIFY=true ;;
    --dry-run) DRY_RUN=true ;;
    --host) SSH_HOST="$2"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1"; usage; exit 1 ;;
  esac
  shift
done

die() { echo "[publish-asset-front] ERROR: $*" >&2; exit 1; }
step() { echo "[publish-asset-front] $*"; }

if [[ "$SKIP_CHECKS" != "true" ]]; then
  step "Local check: vue/dist-asset ..."
  [[ -d "$FRONT" ]] || die "Missing directory: $FRONT"
  [[ -f "$FRONT/asset.html" ]] || die "Missing file: $FRONT/asset.html"
  [[ -d "$FRONT/assets" ]] || die "Missing directory: $FRONT/assets"
  if grep -qE 'localhost|127\.0\.0\.1' "$FRONT/asset.html"; then
    die "asset.html contains localhost or 127.0.0.1"
  fi
  step "Local checks passed"
fi

command -v ssh >/dev/null || die "ssh is required"
command -v scp >/dev/null || die "scp is required"

if [[ "$DRY_RUN" == "true" ]]; then
  step "DryRun: Host=$SSH_HOST source=$FRONT target=$REMOTE_WEB"
  exit 0
fi

TS="$(ssh "$SSH_HOST" 'date -u +%Y%m%dT%H%M%SZ' | tr -d '\r\n')"
[[ -n "$TS" ]] || die "Could not read remote timestamp"

BACKUP="$REMOTE_BACKUP_PARENT/assets.yongbo.cloud_${TS}"
STAGING="/tmp/assets.yongbo.cloud_dist_${TS}"

step "Remote backup: $BACKUP"
ssh "$SSH_HOST" "mkdir -p \"$BACKUP\" \"$REMOTE_WEB\" \"$STAGING\" && cp -a \"$REMOTE_WEB\"/. \"$BACKUP\"/ || true"

step "Upload to staging: $STAGING"
if command -v rsync >/dev/null 2>&1; then
  rsync -av --delete "$FRONT/" "$SSH_HOST:$STAGING/"
else
  scp -r "$FRONT"/* "$SSH_HOST:$STAGING/"
fi

step "Sync to web root, chmod, nginx reload"
ssh "$SSH_HOST" "rsync -a --delete \"$STAGING\"/ \"$REMOTE_WEB\"/ && chmod -R a+rX \"$REMOTE_WEB\" && nginx -t && systemctl reload nginx"

step "Done. Backup kept at: $BACKUP"

if [[ "$SKIP_VERIFY" != "true" ]] && command -v curl >/dev/null 2>&1; then
  step "HTTP probe: assets.yongbo.cloud ..."
  code="$(curl -sS -o /dev/null -w '%{http_code}' "https://assets.yongbo.cloud/")"
  [[ "$code" == "200" ]] || echo "[publish-asset-front] WARN: home HTTP $code" >&2
  code="$(curl -sS -o /dev/null -w '%{http_code}' -X POST "https://assets.yongbo.cloud/v1/auth/login" -H "Content-Type: application/json" -d '{}')"
  [[ "$code" != "404" ]] || echo "[publish-asset-front] WARN: POST /v1/auth/login returned 404; check Nginx /v1 proxy" >&2
  step "HTTP probe done; use a browser and a real account for functional smoke test."
fi
