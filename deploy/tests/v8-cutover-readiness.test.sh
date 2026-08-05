#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
READINESS="$ROOT/deploy/check-v8-cutover-readiness.sh"
REMOTE_DEPLOY="$ROOT/deploy/remote-deploy.sh"
TMP_ROOT="$(mktemp -d)"
BASE_DIR="$TMP_ROOT/runtime"
REPORT_DIR="$BASE_DIR/migrations/run-1"
EXPECTED_COMMIT="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

cleanup() {
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

fail_test() {
  echo "[v8-cutover-readiness-test] FAIL: $*" >&2
  exit 1
}

expect_failure() {
  local expected="$1"
  shift
  local output
  if output="$("$@" 2>&1)"; then
    fail_test "command unexpectedly succeeded: $*"
  fi
  grep -Fq -- "$expected" <<<"$output" ||
    fail_test "missing failure message '$expected': $output"
}

mkdir -p "$BASE_DIR/shared" "$REPORT_DIR" "$TMP_ROOT/bin"
cat >"$BASE_DIR/shared/main.env" <<'EOF'
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=test
DB_PASS=test
DB_NAME=jst_erp
EOF

FAKE_MYSQL="$TMP_ROOT/bin/mysql"
cat >"$FAKE_MYSQL" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
query="${*: -1}"
case "$query" in
  *information_schema.tables*|*information_schema.columns*) printf '1\n' ;;
  *resource_counts*) printf '%s\n' "${FAKE_RESOURCE_MISMATCHES:-0}" ;;
  *) printf '0\n' ;;
esac
EOF
chmod +x "$FAKE_MYSQL"

SOURCE_REPORT="$REPORT_DIR/source-apply.json"
WORKFLOW_APPLY_REPORT="$REPORT_DIR/workflow-apply.json"
WORKFLOW_POST_REPORT="$REPORT_DIR/workflow-postapply.json"
cat >"$SOURCE_REPORT" <<'EOF'
{
  "schema_version": 1,
  "mode": "apply",
  "status": "PASS",
  "database": "jst_erp",
  "database_transaction_committed": true
}
EOF
cat >"$WORKFLOW_APPLY_REPORT" <<'EOF'
{
  "mode": "apply",
  "database": "jst_erp",
  "mapping_file_sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "mapping_canonical_sha256": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
  "mapping_resource_count": 10,
  "mapping_planning_count": 2,
  "counts": {
    "tasks_without_resource_groups": 0,
    "tasks_without_stable_department": 0,
    "users_without_stable_department": 0,
    "legacy_planning_candidates": 0,
    "migration_incomplete_groups": 0
  },
  "manual_task_ids": [],
  "manual_task_issues": [],
  "manual_resource_group_ids": [],
  "manual_access_user_ids": [],
  "manual_access_issues": [],
  "manual_org_issues": [],
  "manual_resource_issues": [],
  "mapping_candidate_issues": []
}
EOF
sed 's/"mode": "apply"/"mode": "dry-run"/' "$WORKFLOW_APPLY_REPORT" >"$WORKFLOW_POST_REPORT"

write_marker() {
  cat >"$BASE_DIR/shared/v8-cutover-approved.env" <<EOF
READINESS_FORMAT=v8-cutover-readiness-v1
APPROVED_COMMIT=$EXPECTED_COMMIT
SOURCE_BUNDLE_APPLY_REPORT=$SOURCE_REPORT
SOURCE_BUNDLE_APPLY_SHA256=$(sha256sum "$SOURCE_REPORT" | awk '{print $1}')
WORKFLOW_APPLY_REPORT=$WORKFLOW_APPLY_REPORT
WORKFLOW_APPLY_SHA256=$(sha256sum "$WORKFLOW_APPLY_REPORT" | awk '{print $1}')
WORKFLOW_POSTAPPLY_REPORT=$WORKFLOW_POST_REPORT
WORKFLOW_POSTAPPLY_SHA256=$(sha256sum "$WORKFLOW_POST_REPORT" | awk '{print $1}')
EOF
}

write_marker
MYSQL_BIN="$FAKE_MYSQL" \
  bash "$READINESS" --base-dir "$BASE_DIR" --expected-commit "$EXPECTED_COMMIT" >/dev/null

expect_failure \
  "package is" \
  env MYSQL_BIN="$FAKE_MYSQL" bash "$READINESS" \
    --base-dir "$BASE_DIR" \
    --expected-commit "dddddddddddddddddddddddddddddddddddddddd"

python3 - "$WORKFLOW_POST_REPORT" <<'PY'
import json
import sys
from pathlib import Path

path = Path(sys.argv[1])
value = json.loads(path.read_text(encoding="utf-8"))
value["counts"]["tasks_without_resource_groups"] = 1
path.write_text(json.dumps(value), encoding="utf-8")
PY
write_marker
expect_failure \
  "tasks_without_resource_groups must be zero" \
  env MYSQL_BIN="$FAKE_MYSQL" bash "$READINESS" \
    --base-dir "$BASE_DIR" \
    --expected-commit "$EXPECTED_COMMIT"

sed -i 's/"tasks_without_resource_groups": 1/"tasks_without_resource_groups": 0/' "$WORKFLOW_POST_REPORT"
write_marker
printf '\n' >>"$SOURCE_REPORT"
expect_failure \
  "SOURCE_BUNDLE_APPLY_REPORT SHA-256 mismatch" \
  env MYSQL_BIN="$FAKE_MYSQL" bash "$READINESS" \
    --base-dir "$BASE_DIR" \
    --expected-commit "$EXPECTED_COMMIT"

write_marker
expect_failure \
  "resource_group_mismatches=1" \
  env MYSQL_BIN="$FAKE_MYSQL" FAKE_RESOURCE_MISMATCHES=1 bash "$READINESS" \
    --base-dir "$BASE_DIR" \
    --expected-commit "$EXPECTED_COMMIT"

migration_line="$(grep -n 'run-pending-migrations.sh' "$REMOTE_DEPLOY" | head -1 | cut -d: -f1)"
readiness_line="$(grep -n 'check-v8-cutover-readiness.sh' "$REMOTE_DEPLOY" | head -1 | cut -d: -f1)"
stop_line="$(grep -n 'stop-main.sh' "$REMOTE_DEPLOY" | tail -1 | cut -d: -f1)"
[ "$migration_line" -lt "$readiness_line" ] ||
  fail_test "readiness gate must run after pending schema migrations"
[ "$readiness_line" -lt "$stop_line" ] ||
  fail_test "readiness gate must run before stopping live MAIN"

echo "[v8-cutover-readiness-test] PASS"
