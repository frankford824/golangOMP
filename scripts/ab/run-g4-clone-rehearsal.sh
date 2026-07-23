#!/usr/bin/env bash
# Destructive only to an explicitly confirmed local clone database.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DB=""; DSN_FILE=""; MAPPING=""; RUN_DIR=""; EXECUTE=false
TOOL="go run ./cmd/tools/workflow-groups-migrate"
MAX_STEP=600; MAX_TOTAL=1800

die() { printf 'ERROR: %s\n' "$*" >&2; exit 2; }
while [[ $# -gt 0 ]]; do
  case "$1" in
    --confirm-clone-database) DB="$2"; shift 2 ;;
    --dsn-file) DSN_FILE="$2"; shift 2 ;;
    --mapping-file) MAPPING="$2"; shift 2 ;;
    --run-dir) RUN_DIR="$2"; shift 2 ;;
    --tool) TOOL="$2"; shift 2 ;;
    --execute-clone-writes) EXECUTE=true; shift ;;
    --max-step-seconds) MAX_STEP="$2"; shift 2 ;;
    --max-total-seconds) MAX_TOTAL="$2"; shift 2 ;;
    *) die "unknown argument: $1" ;;
  esac
done
[[ "$EXECUTE" == true ]] || die "--execute-clone-writes is required"
[[ "$DB" =~ ^ab_[A-Za-z0-9_]+$ ]] || die "clone database must start with ab_"
[[ -f "$DSN_FILE" && -f "$MAPPING" ]] || die "dsn and mapping files are required"
[[ -n "$RUN_DIR" && ! -e "$RUN_DIR" ]] || die "run directory must be new"
[[ "$MAX_STEP" =~ ^[0-9]+$ && "$MAX_TOTAL" =~ ^[0-9]+$ ]] || die "invalid timing limit"

MYSQL_DSN="$(<"$DSN_FILE")"
[[ "$MYSQL_DSN" == *"@tcp(127.0.0.1:"* || "$MYSQL_DSN" == *"@tcp(localhost:"* ]] || die "DSN host must be local"
[[ "$MYSQL_DSN" == *"/$DB"* ]] || die "DSN database does not match confirmation"
export MYSQL_DSN
mkdir -p "$RUN_DIR/snapshot"
: > "$RUN_DIR/steps.tsv"
printf 'step\texit_code\telapsed_seconds\n' > "$RUN_DIR/steps.tsv"

run_step() {
  local name="$1"; shift
  local start end code=0
  start="$(date +%s)"
  printf 'START %s %s\n' "$name" "$(date -u +%FT%TZ)" >> "$RUN_DIR/commands.log"
  timeout --signal=TERM "${MAX_STEP}s" "$@" >"$RUN_DIR/$name.stdout" 2>"$RUN_DIR/$name.stderr" || code=$?
  end="$(date +%s)"
  printf '%s\t%s\t%s\n' "$name" "$code" "$((end-start))" >> "$RUN_DIR/steps.tsv"
  printf 'END %s exit=%s elapsed=%ss\n' "$name" "$code" "$((end-start))" >> "$RUN_DIR/commands.log"
  [[ "$code" -eq 0 ]] || return "$code"
}

read -r -a tool_parts <<< "$TOOL"
common=("${tool_parts[@]}" --mapping-file "$MAPPING" --confirm-database "$DB")
overall_start="$(date +%s)"
run_step dry_run_before "${common[@]}" --dry-run=true --report-file "$RUN_DIR/dry-run-before.json"
run_step apply "${common[@]}" --dry-run=false --apply --snapshot-dir "$RUN_DIR/snapshot" --report-file "$RUN_DIR/apply-report.json"
run_step idempotent_apply "${common[@]}" --dry-run=false --apply --snapshot-dir "$RUN_DIR/snapshot" --report-file "$RUN_DIR/idempotent-report.json"
run_step validate_after_apply "${common[@]}" --dry-run=true --report-file "$RUN_DIR/validate-after-apply.json"
run_step rollback "${common[@]}" --dry-run=false --rollback --snapshot-dir "$RUN_DIR/snapshot"
run_step validate_after_rollback "${common[@]}" --dry-run=true --report-file "$RUN_DIR/validate-after-rollback.json"
overall_end="$(date +%s)"
python3 "$ROOT/scripts/ab/summarize_g4.py" --steps "$RUN_DIR/steps.tsv" \
  --total-seconds "$((overall_end-overall_start))" --max-step "$MAX_STEP" \
  --max-total "$MAX_TOTAL" --snapshot "$RUN_DIR/snapshot/workflow-groups-snapshot.json" \
  --output "$RUN_DIR/g4-report.json"
sha256sum "$MAPPING" "$RUN_DIR"/*.json "$RUN_DIR/snapshot/workflow-groups-snapshot.json" > "$RUN_DIR/evidence.sha256"
