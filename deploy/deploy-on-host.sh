#!/usr/bin/env bash
# Package and deploy from a GitHub self-hosted runner installed on the target
# ECS host. No SSH/SCP round trip is used.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MODE="validate"
VERSION=""
BASE_DIR="/root/ecommerce_ai"
PARALLEL_PORT="18080"
PACKAGE_ROOT=""
PUBLISH_FRONTEND="false"
CONFIRM_PRODUCTION=""

usage() {
  cat <<'EOF'
Usage: deploy/deploy-on-host.sh --mode validate|package|candidate|production [options]

Options:
  --version VERSION            required for package/candidate/production
  --package-root PATH          use an existing unpacked package directory
  --base-dir PATH              runtime base directory
  --parallel-port PORT         candidate port (default: 18080)
  --publish-frontend           publish both built frontends after production cutover
  --confirm-production VALUE   must equal PRODUCTION for production mode
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --mode) MODE="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --package-root) PACKAGE_ROOT="$2"; shift 2 ;;
    --base-dir) BASE_DIR="$2"; shift 2 ;;
    --parallel-port) PARALLEL_PORT="$2"; shift 2 ;;
    --publish-frontend) PUBLISH_FRONTEND="true"; shift ;;
    --confirm-production) CONFIRM_PRODUCTION="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 1 ;;
  esac
done

die() { echo "[deploy-on-host] ERROR: $*" >&2; exit 1; }
step() { echo "[deploy-on-host] $*"; }

case "$MODE" in
  validate) exit 0 ;;
  package|candidate|production) ;;
  *) die "unsupported mode: $MODE" ;;
esac

[ -n "$VERSION" ] || die "--version is required for mode $MODE"
[[ "$VERSION" =~ ^v[0-9]+\.[0-9]+$ ]] || die "version must match v<major>.<minor>"

if [ "$MODE" = "production" ]; then
  [ "$CONFIRM_PRODUCTION" = "PRODUCTION" ] || die "production mode requires --confirm-production PRODUCTION"

  step "production preflight"
  [ -z "$(git -C "$ROOT" status --porcelain=v1 --untracked-files=all)" ] || die "working tree is not clean"

  # The v8 workflow/data migration remains an explicit release gate. The marker
  # is created only after the production-equivalent rehearsal and reviewed
  # workflow-groups migration have completed.
  cutover_marker="$BASE_DIR/shared/v8-cutover-approved.env"
  [ -f "$cutover_marker" ] || die "missing reviewed cutover marker: $cutover_marker"
  grep -Fxq "APPROVED_COMMIT=$(git -C "$ROOT" rev-parse HEAD)" "$cutover_marker" ||
    die "cutover marker does not approve this commit"
fi

if [ -z "$PACKAGE_ROOT" ]; then
  step "packaging $VERSION"
  bash "$SCRIPT_DIR/package-local.sh" --version "$VERSION" --skip-tests
  # shellcheck source=deploy/lib.sh
  . "$SCRIPT_DIR/lib.sh"
  ensure_release_history_file "$ROOT/deploy/release-history.log"
  artifact_prefix="$(history_value "$ROOT/deploy/release-history.log" artifact_prefix)"
  PACKAGE_ROOT="$ROOT/dist/${artifact_prefix}-${VERSION}-linux-amd64"
fi

[ -d "$PACKAGE_ROOT" ] || die "package root not found: $PACKAGE_ROOT"

if [ "$MODE" = "package" ]; then
  # The runner workspace is cleaned by the next checkout. Persist package-mode
  # output outside that workspace so an independently reviewed archive remains
  # available for candidate or production promotion.
  # shellcheck source=deploy/lib.sh
  . "$SCRIPT_DIR/lib.sh"
  ensure_release_history_file "$ROOT/deploy/release-history.log"
  artifact_prefix="$(history_value "$ROOT/deploy/release-history.log" artifact_prefix)"
  archive_name="${artifact_prefix}-${VERSION}-linux-amd64.tar.gz"
  archive_source="$ROOT/dist/$archive_name"
  package_store="$BASE_DIR/packages"
  archive_destination="$package_store/$archive_name"
  checksum_destination="${archive_destination}.sha256"

  [ -f "$archive_source" ] || die "package archive not found: $archive_source"
  install -d -m 0755 "$package_store"
  if [ -f "$archive_destination" ]; then
    cmp -s "$archive_source" "$archive_destination" ||
      die "package version already exists with different content: $archive_destination"
  else
    archive_tmp="${archive_destination}.tmp.$$"
    trap 'rm -f "${archive_tmp:-}"' EXIT
    install -m 0644 "$archive_source" "$archive_tmp"
    mv -f "$archive_tmp" "$archive_destination"
    trap - EXIT
  fi
  (
    cd "$package_store"
    sha256sum "$archive_name" >"$(basename "$checksum_destination")"
  )
  step "package ready at $archive_destination"
  exit 0
fi

runtime_env="$BASE_DIR/shared/main.env"
bridge_env="$BASE_DIR/shared/bridge.env"
[ -f "$runtime_env" ] || die "runtime env not found: $runtime_env"
[ -f "$bridge_env" ] || die "bridge env not found: $bridge_env"

if [ "$MODE" = "candidate" ]; then
  step "starting parallel candidate $VERSION on $PARALLEL_PORT"
  bash "$PACKAGE_ROOT/deploy/remote-deploy.sh" \
    --package-root "$PACKAGE_ROOT" \
    --version "$VERSION" \
    --remote-base-dir "$BASE_DIR" \
    --runtime-env-path "$runtime_env" \
    --bridge-env-path "$bridge_env" \
    --parallel \
    --parallel-port "$PARALLEL_PORT" \
    --start-services
  curl --fail --max-time 10 "http://127.0.0.1:${PARALLEL_PORT}/health"
  step "candidate is healthy: http://127.0.0.1:${PARALLEL_PORT}/health"
  exit 0
fi

step "deploying production $VERSION"
bash "$SCRIPT_DIR/backup-production-db.sh" --base-dir "$BASE_DIR" --version "$VERSION"
bash "$PACKAGE_ROOT/deploy/remote-deploy.sh" \
  --package-root "$PACKAGE_ROOT" \
  --version "$VERSION" \
  --remote-base-dir "$BASE_DIR" \
  --runtime-env-path "$runtime_env" \
  --bridge-env-path "$bridge_env" \
  --start-services

curl --fail --max-time 10 http://127.0.0.1:8080/health

if [ "$PUBLISH_FRONTEND" = "true" ]; then
  bash "$SCRIPT_DIR/publish-front-on-host.sh" \
    --expected-commit "$(git -C "$ROOT" rev-parse HEAD)"
fi

step "production deployment complete version=$VERSION"
