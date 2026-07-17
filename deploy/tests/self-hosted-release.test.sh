#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEPLOY="$ROOT/deploy/deploy-on-host.sh"
BACKUP="$ROOT/deploy/backup-production-db.sh"
PUBLISH="$ROOT/deploy/publish-front-on-host.sh"

fail() {
  echo "[self-hosted-release-test] FAIL: $*" >&2
  exit 1
}

expect_failure() {
  local expected="$1"
  shift
  local output
  if output="$("$@" 2>&1)"; then
    fail "command unexpectedly succeeded: $*"
  fi
  grep -Fq -- "$expected" <<<"$output" ||
    fail "missing failure message '$expected' from: $*"
}

for script in "$DEPLOY" "$BACKUP" "$PUBLISH"; do
  [ -x "$script" ] || fail "script is not executable: $script"
  bash -n "$script"
done

bash "$DEPLOY" --mode validate
expect_failure "unsupported mode" bash "$DEPLOY" --mode unsupported
expect_failure "--version is required" bash "$DEPLOY" --mode package
expect_failure "version must match" bash "$DEPLOY" --mode package --version latest
expect_failure "production mode requires" bash "$DEPLOY" --mode production --version v99.99
expect_failure "invalid version" bash "$BACKUP" --version latest

echo "[self-hosted-release-test] PASS"
