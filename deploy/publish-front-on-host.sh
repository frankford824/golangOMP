#!/usr/bin/env bash
# Publish already-built main-ops and asset-workbench artifacts from a
# self-hosted runner that is installed on the production ECS host.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MAIN_DIST="$ROOT/vue/dist"
ASSET_DIST="$ROOT/vue/dist-asset"
MAIN_WEB_ROOT="/var/www/yongbo.cloud"
ASSET_WEB_ROOT="/var/www/assets.yongbo.cloud"
BACKUP_PARENT="/var/www/backups"
EXPECTED_COMMIT="$(git -C "$ROOT" rev-parse HEAD)"
PUBLISH_MAIN="true"
PUBLISH_ASSET="true"
RELOAD_NGINX="true"
DRY_RUN="false"

usage() {
  cat <<'EOF'
Usage: deploy/publish-front-on-host.sh [options]

Options:
  --main-dist PATH       main-ops build directory (default: vue/dist)
  --asset-dist PATH      asset-workbench build directory (default: vue/dist-asset)
  --main-web-root PATH   main-ops web root
  --asset-web-root PATH  asset-workbench web root
  --backup-parent PATH   backup parent directory
  --expected-commit SHA  required artifact commit
  --main-only            publish only main-ops
  --asset-only           publish only asset-workbench
  --skip-nginx-reload    sync artifacts without reloading Nginx
  --dry-run              validate artifacts and print destinations only
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --main-dist) MAIN_DIST="$2"; shift 2 ;;
    --asset-dist) ASSET_DIST="$2"; shift 2 ;;
    --main-web-root) MAIN_WEB_ROOT="$2"; shift 2 ;;
    --asset-web-root) ASSET_WEB_ROOT="$2"; shift 2 ;;
    --backup-parent) BACKUP_PARENT="$2"; shift 2 ;;
    --expected-commit) EXPECTED_COMMIT="$2"; shift 2 ;;
    --main-only) PUBLISH_ASSET="false"; shift ;;
    --asset-only) PUBLISH_MAIN="false"; shift ;;
    --skip-nginx-reload) RELOAD_NGINX="false"; shift ;;
    --dry-run) DRY_RUN="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 1 ;;
  esac
done

die() { echo "[publish-on-host] ERROR: $*" >&2; exit 1; }
step() { echo "[publish-on-host] $*"; }

command -v node >/dev/null 2>&1 || die "node is required"
command -v rsync >/dev/null 2>&1 || die "rsync is required"
command -v curl >/dev/null 2>&1 || die "curl is required"

validate_artifact() {
  local app="$1"
  local dir="$2"
  node "$SCRIPT_DIR/verify-static-artifact.mjs" \
    --app "$app" \
    --dir "$dir" \
    --expected-commit "$EXPECTED_COMMIT"
}

if [ "$PUBLISH_MAIN" = "true" ]; then
  validate_artifact main-ops "$MAIN_DIST"
fi
if [ "$PUBLISH_ASSET" = "true" ]; then
  validate_artifact asset-workbench "$ASSET_DIST"
fi

if [ "$DRY_RUN" = "true" ]; then
  step "dry-run main=$PUBLISH_MAIN:$MAIN_DIST->$MAIN_WEB_ROOT asset=$PUBLISH_ASSET:$ASSET_DIST->$ASSET_WEB_ROOT"
  exit 0
fi

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
work_root="$(mktemp -d "${TMPDIR:-/tmp}/yongbo-front-publish-${timestamp}-XXXXXX")"
main_backup="$BACKUP_PARENT/yongbo.cloud_${timestamp}"
asset_backup="$BACKUP_PARENT/assets.yongbo.cloud_${timestamp}"
main_changed="false"
asset_changed="false"
completed="false"

rollback() {
  local status=$?
  trap - EXIT INT TERM
  if [ "$completed" != "true" ]; then
    echo "[publish-on-host] publication failed; restoring web roots" >&2
    if [ "$main_changed" = "true" ] && [ -d "$main_backup" ]; then
      rsync -a --delete "$main_backup/" "$MAIN_WEB_ROOT/" || true
    fi
    if [ "$asset_changed" = "true" ] && [ -d "$asset_backup" ]; then
      rsync -a --delete "$asset_backup/" "$ASSET_WEB_ROOT/" || true
    fi
    if [ "$RELOAD_NGINX" = "true" ]; then
      nginx -t >/dev/null 2>&1 && systemctl reload nginx >/dev/null 2>&1 || true
    fi
  fi
  rm -rf "$work_root"
  exit "$status"
}
trap rollback EXIT INT TERM

mkdir -p "$BACKUP_PARENT" "$MAIN_WEB_ROOT" "$ASSET_WEB_ROOT"

if [ "$PUBLISH_MAIN" = "true" ]; then
  step "backing up main-ops to $main_backup"
  mkdir -p "$main_backup" "$work_root/main"
  cp -a "$MAIN_WEB_ROOT/." "$main_backup/"
  rsync -a --delete "$MAIN_DIST/" "$work_root/main/"
  rsync -a --delete "$work_root/main/" "$MAIN_WEB_ROOT/"
  chmod -R a+rX "$MAIN_WEB_ROOT"
  main_changed="true"
fi

if [ "$PUBLISH_ASSET" = "true" ]; then
  step "backing up asset-workbench to $asset_backup"
  mkdir -p "$asset_backup" "$work_root/asset"
  cp -a "$ASSET_WEB_ROOT/." "$asset_backup/"
  rsync -a --delete "$ASSET_DIST/" "$work_root/asset/"
  rsync -a --delete "$work_root/asset/" "$ASSET_WEB_ROOT/"
  chmod -R a+rX "$ASSET_WEB_ROOT"
  asset_changed="true"
fi

if [ "$RELOAD_NGINX" = "true" ]; then
  step "validating and reloading Nginx"
  nginx -t
  systemctl reload nginx
fi

if [ "$PUBLISH_MAIN" = "true" ]; then
  code="$(curl --max-time 10 -sS -o /dev/null -w '%{http_code}' https://yongbo.cloud/)"
  [ "$code" = "200" ] || die "main-ops probe returned HTTP $code"
fi
if [ "$PUBLISH_ASSET" = "true" ]; then
  code="$(curl --max-time 10 -sS -o /dev/null -w '%{http_code}' https://assets.yongbo.cloud/)"
  [ "$code" = "200" ] || die "asset-workbench probe returned HTTP $code"
fi

completed="true"
step "publication complete commit=$EXPECTED_COMMIT"
step "main backup: $main_backup"
step "asset backup: $asset_backup"
