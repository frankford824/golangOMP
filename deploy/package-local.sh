#!/usr/bin/env bash
set -euo pipefail

DEPLOY_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/lib.sh
. "$DEPLOY_SCRIPT_DIR/lib.sh"

VERSION="dev"
OUTPUT_ROOT="dist"
SKIP_TESTS="false"

while [ $# -gt 0 ]; do
  case "$1" in
    --version)
      VERSION="$2"
      shift 2
      ;;
    --output-root)
      OUTPUT_ROOT="$2"
      shift 2
      ;;
    --skip-tests)
      SKIP_TESTS="true"
      shift
      ;;
    *)
      fail "Unknown argument: $1"
      ;;
  esac
done

ROOT="$(repo_root)"
load_local_deploy_env "$ROOT"
HISTORY_PATH="$ROOT/deploy/release-history.log"
ensure_release_history_file "$HISTORY_PATH"
package_release \
  "$ROOT" \
  "$VERSION" \
  "$OUTPUT_ROOT" \
  "$SKIP_TESTS" \
  "$(history_value "$HISTORY_PATH" artifact_prefix)" \
  "$(history_value "$HISTORY_PATH" bridge_runtime_base_url)"

printf 'Packaged MAIN and bridge to %s\n' "$PACKAGE_STAGE_ROOT"
printf 'Tarball artifact: %s\n' "$PACKAGE_ARTIFACT_PATH"
