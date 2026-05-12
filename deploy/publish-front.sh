#!/usr/bin/env bash
# Publish local dist/front to jst_ecs:/var/www/yongbo.cloud.
# Requires ssh and scp locally. Remote host must provide rsync and nginx.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FRONT="$ROOT/dist/front"
SSH_HOST="${SSH_HOST:-jst_ecs}"
REMOTE_WEB="/var/www/yongbo.cloud"
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

die() { echo "[publish-front] ERROR: $*" >&2; exit 1; }
step() { echo "[publish-front] $*"; }

if [[ "$SKIP_CHECKS" != "true" ]]; then
  step "Local check: dist/front ..."
  [[ -d "$FRONT" ]] || die "Missing directory: $FRONT"
  [[ -f "$FRONT/index.html" ]] || die "Missing file: $FRONT/index.html"
  [[ -d "$FRONT/assets" ]] || die "Missing directory: $FRONT/assets"
  if grep -qE 'localhost|127\.0\.0\.1' "$FRONT/index.html"; then
    die "index.html contains localhost or 127.0.0.1"
  fi
  main_js=""
  main_js=$(grep -oE 'src="/assets/[^"]+\.js"' "$FRONT/index.html" | head -1 | sed 's/src="//;s/"$//' || true)
  if [[ -n "$main_js" ]]; then
    rel="${main_js#/}"
    path="$FRONT/$rel"
    if [[ -f "$path" ]]; then
      if grep -qE 'https?://localhost|https?://127\.0\.0\.1|127\.0\.0\.1:[0-9]+' "$path"; then
        echo "[publish-front] WARN: main bundle contains localhost/127.0.0.1 literals; confirm API is still /v1" >&2
      fi
      if ! grep -q '"/v1' "$path"; then
        echo "[publish-front] WARN: main bundle has no literal \"/v1\"; verify API base" >&2
      fi
    fi
  fi
  if ! grep -rEl 'beian\.miit\.gov\.cn|2026007026' "$FRONT" --include='*.html' --include='*.js' --include='*.css' -q 2>/dev/null; then
    echo "[publish-front] WARN: ICP footer or beian link not detected; confirm the page shows the required filing text." >&2
  fi
  step "Local checks passed"
fi

command -v ssh >/dev/null || die "ssh is required"
command -v scp >/dev/null || die "scp is required"

if [[ "$DRY_RUN" == "true" ]]; then
  step "DryRun: Host=$SSH_HOST source=$FRONT target=$REMOTE_WEB"
  exit 0
fi

step "Fetch remote UTC timestamp ..."
TS="$(ssh "$SSH_HOST" 'date -u +%Y%m%dT%H%M%SZ' | tr -d '\r\n')"
[[ -n "$TS" ]] || die "Could not read remote timestamp"

BACKUP="$REMOTE_BACKUP_PARENT/yongbo.cloud_${TS}"
STAGING="/tmp/yongbo.cloud_dist_${TS}"

step "Remote backup: $BACKUP"
ssh "$SSH_HOST" "mkdir -p \"$BACKUP\" && cp -a \"$REMOTE_WEB\"/. \"$BACKUP\"/ && mkdir -p \"$STAGING\""

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
  step "HTTP probe: yongbo.cloud ..."
  code="$(curl -sS -o /dev/null -w '%{http_code}' "https://yongbo.cloud/")"
  [[ "$code" == "200" ]] || echo "[publish-front] WARN: home HTTP $code" >&2
  code="$(curl -sS -o /dev/null -w '%{http_code}' "https://yongbo.cloud/login")"
  [[ "$code" == "200" ]] || echo "[publish-front] WARN: /login HTTP $code" >&2
  code="$(curl -sS -o /dev/null -w '%{http_code}' -X POST "https://yongbo.cloud/v1/auth/login" -H "Content-Type: application/json" -d '{}')"
  [[ "$code" != "404" ]] || echo "[publish-front] WARN: POST /v1/auth/login returned 404; check Nginx /v1 proxy" >&2
  step "HTTP probe done; use a browser and a real account for functional smoke test."
fi
