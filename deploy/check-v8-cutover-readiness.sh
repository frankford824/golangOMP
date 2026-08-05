#!/usr/bin/env bash
# Fail-closed V8 production cutover gate. It validates the exact reviewed
# evidence set and the live database after schema migration, before MAIN is
# stopped or the candidate backend is allowed to accept production traffic.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_DIR="/root/ecommerce_ai"
MARKER_PATH=""
EXPECTED_COMMIT=""

usage() {
  cat <<'EOF'
Usage: deploy/check-v8-cutover-readiness.sh --expected-commit SHA [options]

Options:
  --base-dir PATH          runtime base directory (default: /root/ecommerce_ai)
  --marker PATH            reviewed evidence marker
  --expected-commit SHA    exact 40-character release commit
EOF
}

fail() {
  echo "[v8-cutover-readiness] ERROR: $*" >&2
  exit 1
}

while [ $# -gt 0 ]; do
  case "$1" in
    --base-dir) BASE_DIR="$2"; shift 2 ;;
    --marker) MARKER_PATH="$2"; shift 2 ;;
    --expected-commit) EXPECTED_COMMIT="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done

MARKER_PATH="${MARKER_PATH:-$BASE_DIR/shared/v8-cutover-approved.env}"
[[ "$EXPECTED_COMMIT" =~ ^[0-9a-f]{40}$ ]] ||
  fail "--expected-commit must be a lowercase 40-character git SHA"
[ -f "$MARKER_PATH" ] || fail "reviewed cutover marker not found: $MARKER_PATH"
[ ! -L "$MARKER_PATH" ] || fail "reviewed cutover marker must not be a symlink"

if ! awk '
  /^[[:space:]]*($|#)/ { next }
  /^(READINESS_FORMAT|APPROVED_COMMIT|SOURCE_BUNDLE_APPLY_REPORT|SOURCE_BUNDLE_APPLY_SHA256|WORKFLOW_APPLY_REPORT|WORKFLOW_APPLY_SHA256|WORKFLOW_POSTAPPLY_REPORT|WORKFLOW_POSTAPPLY_SHA256)=/ { next }
  { exit 1 }
' "$MARKER_PATH"; then
  fail "cutover marker contains an unknown or malformed key"
fi

marker_value() {
  local key="$1"
  local count
  local value
  count="$(grep -c "^${key}=" "$MARKER_PATH" || true)"
  [ "$count" = "1" ] || fail "cutover marker must contain exactly one ${key}"
  value="$(sed -n "s/^${key}=//p" "$MARKER_PATH")"
  [ -n "$value" ] || fail "cutover marker value is empty: ${key}"
  printf '%s' "$value"
}

format="$(marker_value READINESS_FORMAT)"
approved_commit="$(marker_value APPROVED_COMMIT)"
[ "$format" = "v8-cutover-readiness-v1" ] ||
  fail "unsupported READINESS_FORMAT: $format"
[ "$approved_commit" = "$EXPECTED_COMMIT" ] ||
  fail "cutover marker approves $approved_commit, package is $EXPECTED_COMMIT"

base_real="$(readlink -f "$BASE_DIR")"
[ -n "$base_real" ] && [ -d "$base_real" ] ||
  fail "runtime base directory does not exist: $BASE_DIR"

resolve_report() {
  local key="$1"
  local path
  local real
  path="$(marker_value "$key")"
  [[ "$path" = /* ]] || fail "${key} must be an absolute path"
  [ -f "$path" ] || fail "${key} is not a regular file: $path"
  real="$(readlink -f "$path")"
  case "$real" in
    "$base_real"/*) ;;
    *) fail "${key} must resolve inside $base_real" ;;
  esac
  printf '%s' "$real"
}

verify_report_hash() {
  local path_key="$1"
  local sha_key="$2"
  local path
  local expected
  local actual
  path="$(resolve_report "$path_key")"
  expected="$(marker_value "$sha_key")"
  [[ "$expected" =~ ^[0-9a-f]{64}$ ]] ||
    fail "${sha_key} must be a lowercase SHA-256"
  actual="$(sha256sum "$path" | awk '{print $1}')"
  [ "$actual" = "$expected" ] ||
    fail "${path_key} SHA-256 mismatch"
  printf '%s' "$path"
}

source_report="$(verify_report_hash SOURCE_BUNDLE_APPLY_REPORT SOURCE_BUNDLE_APPLY_SHA256)"
workflow_apply_report="$(verify_report_hash WORKFLOW_APPLY_REPORT WORKFLOW_APPLY_SHA256)"
workflow_postapply_report="$(verify_report_hash WORKFLOW_POSTAPPLY_REPORT WORKFLOW_POSTAPPLY_SHA256)"

command -v python3 >/dev/null 2>&1 || fail "python3 is required to validate cutover reports"
python3 - "$source_report" "$workflow_apply_report" "$workflow_postapply_report" <<'PY'
import json
import sys
from pathlib import Path


def fail(message: str) -> None:
    raise SystemExit(f"[v8-cutover-readiness] ERROR: {message}")


def load(path: str, label: str) -> dict:
    try:
        value = json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"{label} is not valid JSON: {exc}")
    if not isinstance(value, dict):
        fail(f"{label} must be a JSON object")
    return value


source = load(sys.argv[1], "source-bundle apply report")
workflow_apply = load(sys.argv[2], "workflow-groups apply report")
workflow_post = load(sys.argv[3], "workflow-groups postapply report")

if source.get("schema_version") != 1:
    fail("source-bundle apply report schema_version must be 1")
if source.get("mode") != "apply" or source.get("status") != "PASS":
    fail("source-bundle apply report must be a PASS apply")
if source.get("database_transaction_committed") is not True:
    fail("source-bundle apply transaction is not committed")

for label, report, expected_mode in (
    ("workflow-groups apply report", workflow_apply, "apply"),
    ("workflow-groups postapply report", workflow_post, "dry-run"),
):
    if report.get("mode") != expected_mode:
        fail(f"{label} mode must be {expected_mode}")
    for key in (
        "manual_task_ids",
        "manual_task_issues",
        "manual_resource_group_ids",
        "manual_access_user_ids",
        "manual_access_issues",
        "manual_org_issues",
        "manual_resource_issues",
        "mapping_candidate_issues",
    ):
        value = report.get(key, [])
        if not isinstance(value, list) or value:
            fail(f"{label} contains blockers in {key}")

    counts = report.get("counts")
    if not isinstance(counts, dict):
        fail(f"{label} counts must be an object")
    for key in (
        "tasks_without_resource_groups",
        "tasks_without_stable_department",
        "users_without_stable_department",
        "legacy_planning_candidates",
        "migration_incomplete_groups",
    ):
        if counts.get(key) != 0:
            fail(f"{label} count {key} must be zero, got {counts.get(key)!r}")

for key in ("mapping_file_sha256", "mapping_canonical_sha256"):
    apply_value = workflow_apply.get(key)
    post_value = workflow_post.get(key)
    if not isinstance(apply_value, str) or len(apply_value) != 64:
        fail(f"workflow-groups apply report has invalid {key}")
    if apply_value != post_value:
        fail(f"workflow-groups apply/postapply {key} mismatch")

for key in ("mapping_resource_count", "mapping_planning_count"):
    if workflow_apply.get(key) != workflow_post.get(key):
        fail(f"workflow-groups apply/postapply {key} mismatch")

databases = {
    source.get("database"),
    workflow_apply.get("database"),
    workflow_post.get("database"),
}
if len(databases) != 1 or not next(iter(databases), ""):
    fail("source-bundle and workflow-groups reports target different databases")
PY

# This check runs after pending schema migrations and validates live production
# state. A reviewed report cannot substitute for the current database.
bash "$SCRIPT_DIR/check-remote-db.sh" --base-dir "$BASE_DIR"

echo "[v8-cutover-readiness] PASS commit=$EXPECTED_COMMIT"
