#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEPLOY="$ROOT/deploy/deploy-on-host.sh"
BACKUP="$ROOT/deploy/backup-production-db.sh"
PUBLISH="$ROOT/deploy/publish-front-on-host.sh"
TMP_ROOT="$(mktemp -d)"
PACKAGE_TEST_VERSION="v98765.4321"
PACKAGE_TEST_NAME="ecommerce-ai-${PACKAGE_TEST_VERSION}-linux-amd64.tar.gz"

cleanup() {
  rm -rf "$TMP_ROOT"
  rm -f "$ROOT/dist/$PACKAGE_TEST_NAME"
}
trap cleanup EXIT

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

mkdir -p "$TMP_ROOT/package-root" "$ROOT/dist"
printf 'package-one\n' >"$ROOT/dist/$PACKAGE_TEST_NAME"
bash "$DEPLOY" \
  --mode package \
  --version "$PACKAGE_TEST_VERSION" \
  --package-root "$TMP_ROOT/package-root" \
  --base-dir "$TMP_ROOT/runtime" >/dev/null
cmp -s \
  "$ROOT/dist/$PACKAGE_TEST_NAME" \
  "$TMP_ROOT/runtime/packages/$PACKAGE_TEST_NAME" ||
  fail "persisted package differs from source archive"
(
  cd "$TMP_ROOT/runtime/packages"
  sha256sum -c "${PACKAGE_TEST_NAME}.sha256" >/dev/null
) || fail "persisted package checksum is invalid"

printf 'package-two\n' >"$ROOT/dist/$PACKAGE_TEST_NAME"
expect_failure \
  "package version already exists with different content" \
  bash "$DEPLOY" \
    --mode package \
    --version "$PACKAGE_TEST_VERSION" \
    --package-root "$TMP_ROOT/package-root" \
    --base-dir "$TMP_ROOT/runtime"

echo "[self-hosted-release-test] PASS"
