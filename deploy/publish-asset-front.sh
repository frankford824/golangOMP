#!/usr/bin/env bash
# Publish local vue/dist-asset to jst_ecs:/var/www/assets.yongbo.cloud.
# This is intentionally separate from publish-front.sh so the workbench App
# cannot overwrite the main operation system frontend.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FRONT="$ROOT/vue/dist-asset"
VERIFY_STATIC_ARTIFACT="$ROOT/deploy/verify-static-artifact.mjs"
SSH_HOST="${SSH_HOST:-jst_ecs}"
REMOTE_WEB="/var/www/assets.yongbo.cloud"
REMOTE_BACKUP_PARENT="/var/www/backups"
EXPECTED_COMMIT="$(git -C "$ROOT" rev-parse HEAD)"

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

guard_asset_artifact() {
  command -v node >/dev/null || die "node is required for static artifact manifest validation"
  node "$VERIFY_STATIC_ARTIFACT" --app asset-workbench --dir "$FRONT" --expected-commit "$EXPECTED_COMMIT" || die "static artifact manifest validation failed"
  [[ -d "$FRONT" ]] || die "Missing directory: $FRONT"
  [[ -f "$FRONT/asset.html" ]] || die "Missing file: $FRONT/asset.html"
  [[ -d "$FRONT/assets" ]] || die "Missing directory: $FRONT/assets"
  [[ ! -f "$FRONT/index.html" ]] || die "vue/dist-asset contains index.html; refusing to publish a main-ops shaped bundle to assets.yongbo.cloud"
  if ! grep -q 'asset-workbench-app' "$FRONT/asset.html"; then
    die "vue/dist-asset/asset.html does not contain the asset-workbench mount node"
  fi
  if ! grep -qE 'src="/assets/asset-[^"]+\.js"' "$FRONT/asset.html"; then
    die "vue/dist-asset/asset.html does not reference an asset-workbench entry bundle"
  fi
  if grep -qE '<title>[[:space:]]*永箔运营管理系统[[:space:]]*</title>|id="app"' "$FRONT/asset.html"; then
    die "vue/dist-asset/asset.html looks like the main-ops entry; rebuild asset-workbench before publishing"
  fi
}

step "Artifact identity guard: asset-workbench frontend ..."
guard_asset_artifact

if [[ "$SKIP_CHECKS" != "true" ]]; then
  step "Local check: vue/dist-asset ..."
  if grep -qE 'localhost|127\.0\.0\.1' "$FRONT/asset.html"; then
    die "asset.html contains localhost or 127.0.0.1"
  fi
  step "Local checks passed"
fi

if [[ "$DRY_RUN" == "true" ]]; then
  step "DryRun: Host=$SSH_HOST source=$FRONT target=$REMOTE_WEB"
  exit 0
fi

command -v ssh >/dev/null || die "ssh is required"
command -v scp >/dev/null || die "scp is required"

TS="$(ssh "$SSH_HOST" 'date -u +%Y%m%dT%H%M%SZ' | tr -d '\r\n')"
[[ -n "$TS" ]] || die "Could not read remote timestamp"

BACKUP="$REMOTE_BACKUP_PARENT/assets.yongbo.cloud_${TS}"
STAGING="/tmp/assets.yongbo.cloud_dist_${TS}"

step "Remote backup: $BACKUP"
ssh "$SSH_HOST" "mkdir -p \"$BACKUP\" \"$REMOTE_WEB\" \"$STAGING\" && cp -a \"$REMOTE_WEB\"/. \"$BACKUP\"/"

step "Upload to staging: $STAGING"
if command -v rsync >/dev/null 2>&1; then
  rsync -av --delete "$FRONT/" "$SSH_HOST:$STAGING/"
else
  scp -r "$FRONT"/* "$SSH_HOST:$STAGING/"
fi

step "Remote artifact guard: asset-workbench staging ..."
ssh "$SSH_HOST" "test -f \"$STAGING/asset.html\" && test -d \"$STAGING/assets\" && test -f \"$STAGING/static-artifact-manifest.json\" && test ! -f \"$STAGING/index.html\" && grep -q 'asset-workbench-app' \"$STAGING/asset.html\" && grep -Eq 'src=\"/assets/asset-[^\"]+\\.js\"' \"$STAGING/asset.html\" && ! grep -q 'id=\"app\"' \"$STAGING/asset.html\" && grep -Eq '\"app\"[[:space:]]*:[[:space:]]*\"asset-workbench\"' \"$STAGING/static-artifact-manifest.json\" && grep -Eq '\"entry\"[[:space:]]*:[[:space:]]*\"asset.html\"' \"$STAGING/static-artifact-manifest.json\" && grep -Eq '\"targetHost\"[[:space:]]*:[[:space:]]*\"assets.yongbo.cloud\"' \"$STAGING/static-artifact-manifest.json\" && grep -Eq '\"targetWebRoot\"[[:space:]]*:[[:space:]]*\"/var/www/assets.yongbo.cloud\"' \"$STAGING/static-artifact-manifest.json\" && grep -Eq '\"gitCommit\"[[:space:]]*:[[:space:]]*\"$EXPECTED_COMMIT\"' \"$STAGING/static-artifact-manifest.json\"" \
  || die "staged artifact failed the asset-workbench identity guard"

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
