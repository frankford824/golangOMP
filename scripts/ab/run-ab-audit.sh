#!/usr/bin/env bash
# Safe evidence runner for a two-clone database audit. It is intentionally
# plan-only by default: no network or database command runs without a flag.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SQL_DIR="$ROOT/scripts/ab/sql"
RENDER_BIN="$ROOT/scripts/ab/render_evidence.py"
MANIFEST_LOADER="$ROOT/scripts/ab/manifest_loader.py"
MODE=""
RUN_ID=""
SOURCE_DB=""
TARGET_DB=""
SOURCE_HOST="127.0.0.1"
SOURCE_PORT="3306"
TARGET_HOST="127.0.0.1"
TARGET_PORT="3306"
EVIDENCE_ROOT="$ROOT/tmp/v8-ab"
MYSQL_BIN="${MYSQL_BIN:-mysql}"
SOURCE_DEFAULTS_FILE="${AB_AUDIT_MYSQL_SOURCE_DEFAULTS_FILE:-}"
TARGET_DEFAULTS_FILE="${AB_AUDIT_MYSQL_TARGET_DEFAULTS_FILE:-}"
API_A_URL=""
API_B_URL=""
EXECUTE_READONLY=false
ALLOW_RESTORE=false
RESTORE_FILE=""
ALLOW_APPLY_MIGRATIONS=false
MIGRATION_FILE=""
MANIFEST_JSONL=""
MANIFEST_SHA256=""
SNAPSHOT_SHA256=""

usage() {
  cat <<'USAGE'
Usage:
  scripts/ab/run-ab-audit.sh --mode readonly|clone --run-id ID \
    --source-db DB --target-db DB [options]

Default behavior only writes a local plan and checksums under tmp/v8-ab.
It never contacts MySQL, HTTP, Docker, or production by default.

Read-only execution:
  --execute-readonly                 Run only SELECT/SHOW evidence SQL.
  --source-host HOST --source-port P --target-host HOST --target-port P
  --source-defaults-file FILE        mysql defaults file; never logged.
  --target-defaults-file FILE        mysql defaults file; never logged.
  --api-a-url URL --api-b-url URL    Optional GET-only API probes.

Formal clone evidence also requires:
  --snapshot-sha256 SHA256
  --manifest-jsonl FILE --manifest-sha256 SHA256

This runner never writes a database. Restore/migration/apply/rollback are
performed by the dedicated tools with their own database confirmation guards.
USAGE
}

die() { printf 'ERROR: %s\n' "$*" >&2; exit 2; }
log() { printf '%s\n' "$*" | tee -a "$COMMAND_LOG"; }
sha256() { sha256sum "$1" | awk '{print $1}'; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="$2"; shift 2 ;;
    --run-id) RUN_ID="$2"; shift 2 ;;
    --source-db) SOURCE_DB="$2"; shift 2 ;;
    --target-db) TARGET_DB="$2"; shift 2 ;;
    --source-host) SOURCE_HOST="$2"; shift 2 ;;
    --source-port) SOURCE_PORT="$2"; shift 2 ;;
    --target-host) TARGET_HOST="$2"; shift 2 ;;
    --target-port) TARGET_PORT="$2"; shift 2 ;;
    --evidence-root) EVIDENCE_ROOT="$2"; shift 2 ;;
    --mysql-bin) MYSQL_BIN="$2"; shift 2 ;;
    --source-defaults-file) SOURCE_DEFAULTS_FILE="$2"; shift 2 ;;
    --target-defaults-file) TARGET_DEFAULTS_FILE="$2"; shift 2 ;;
    --api-a-url) API_A_URL="$2"; shift 2 ;;
    --api-b-url) API_B_URL="$2"; shift 2 ;;
    --execute-readonly) EXECUTE_READONLY=true; shift ;;
    --allow-restore) ALLOW_RESTORE=true; shift ;;
    --restore-file) RESTORE_FILE="$2"; shift 2 ;;
    --allow-apply-migrations) ALLOW_APPLY_MIGRATIONS=true; shift ;;
    --migration-file) MIGRATION_FILE="$2"; shift 2 ;;
    --manifest-jsonl) MANIFEST_JSONL="$2"; shift 2 ;;
    --manifest-sha256) MANIFEST_SHA256="$2"; shift 2 ;;
    --snapshot-sha256) SNAPSHOT_SHA256="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

[[ "$MODE" == "readonly" || "$MODE" == "clone" ]] || die "--mode readonly|clone is required"
[[ "$RUN_ID" =~ ^[a-z0-9][a-z0-9_-]{2,63}$ ]] || die "invalid --run-id (lowercase letters, digits, _ and - only)"
[[ "$SOURCE_DB" =~ ^[A-Za-z0-9_]+$ ]] || die "invalid --source-db"
[[ "$TARGET_DB" =~ ^[A-Za-z0-9_]+$ ]] || die "invalid --target-db"
[[ "$SOURCE_PORT" =~ ^[0-9]{2,5}$ && "$TARGET_PORT" =~ ^[0-9]{2,5}$ ]] || die "invalid MySQL port"

RUN_DIR="$EVIDENCE_ROOT/$RUN_ID"
[[ ! -e "$RUN_DIR" ]] || die "run directory already exists; choose a new run-id"
COMMAND_LOG="$RUN_DIR/commands.log"
mkdir -p "$RUN_DIR/sql/source" "$RUN_DIR/sql/target" "$RUN_DIR/api"
mkdir -p "$RUN_DIR/parity"
: > "$COMMAND_LOG"

if [[ "$ALLOW_RESTORE" == true || "$ALLOW_APPLY_MIGRATIONS" == true || -n "$RESTORE_FILE" || -n "$MIGRATION_FILE" ]]; then
  die "database writes are not supported by the A/B evidence runner; use the confirmed clone migration workflow"
fi
if [[ -n "$MANIFEST_JSONL" || -n "$MANIFEST_SHA256" ]]; then
  [[ -n "$MANIFEST_JSONL" && -n "$MANIFEST_SHA256" ]] || die "manifest requires both --manifest-jsonl and --manifest-sha256"
  [[ "$MODE" == "clone" ]] || die "manifest staging is clone mode only; source is never altered"
  python3 "$MANIFEST_LOADER" "$MANIFEST_JSONL" "$MANIFEST_SHA256" "$RUN_ID" "$RUN_DIR/clone-manifest-temp.sql"
  cp "$MANIFEST_JSONL" "$RUN_DIR/reviewed-manifest.jsonl"
fi
if [[ "$EXECUTE_READONLY" == true ]]; then
  [[ "$MODE" == "clone" ]] || die "formal SQL execution requires --mode clone"
  [[ "$SNAPSHOT_SHA256" =~ ^[0-9a-f]{64}$ ]] || die "--snapshot-sha256 is required for formal SQL execution"
  [[ -n "$MANIFEST_JSONL" ]] || die "reviewed manifest is required for formal SQL execution"
  [[ "$SOURCE_HOST:$SOURCE_PORT/$SOURCE_DB" != "$TARGET_HOST:$TARGET_PORT/$TARGET_DB" ]] || die "source and target clones must be distinct"
  case "$SOURCE_HOST" in 127.0.0.1|localhost|host.docker.internal) ;; *) die "source clone host must be local" ;; esac
  case "$TARGET_HOST" in 127.0.0.1|localhost|host.docker.internal) ;; *) die "target clone host must be local" ;; esac
fi

GIT_HEAD="$(git -C "$ROOT" rev-parse HEAD)"
OPENAPI_HASH="$(sha256 "$ROOT/docs/api/openapi.yaml")"
WORKTREE_HASH="$({
  git -C "$ROOT" diff --binary HEAD
  while IFS= read -r -d '' untracked; do
    printf '%s %s\n' "$untracked" "$(sha256 "$ROOT/$untracked")"
  done < <(git -C "$ROOT" ls-files --others --exclude-standard -z | sort -z)
} | sha256sum | awk '{print $1}')"
{
  printf 'run_id=%s\nmode=%s\nsource_db=%s\ntarget_db=%s\n' "$RUN_ID" "$MODE" "$SOURCE_DB" "$TARGET_DB"
  printf 'source_endpoint=%s:%s\ntarget_endpoint=%s:%s\n' "$SOURCE_HOST" "$SOURCE_PORT" "$TARGET_HOST" "$TARGET_PORT"
  printf 'execute_readonly=%s\nsnapshot_sha256=%s\n' "$EXECUTE_READONLY" "$SNAPSHOT_SHA256"
  printf 'git_head=%s\nworktree_diff_sha256=%s\nopenapi_sha256=%s\n' "$GIT_HEAD" "$WORKTREE_HASH" "$OPENAPI_HASH"
} > "$RUN_DIR/manifest.env"
sha256sum "$RUN_DIR/manifest.env" | awk '{print $1}' > "$RUN_DIR/manifest.sha256"
python3 "$ROOT/scripts/ab/initialize_run.py" \
  --run-dir "$RUN_DIR" --run-id "$RUN_ID" --mode "$MODE" \
  --source-db "$SOURCE_DB" --target-db "$TARGET_DB" \
  --git-head "$GIT_HEAD" --worktree-hash "$WORKTREE_HASH" --openapi-hash "$OPENAPI_HASH" \
  --snapshot-hash "$SNAPSHOT_SHA256" --review-manifest-hash "$MANIFEST_SHA256"

for sql in "$SQL_DIR"/*.sql; do
  base="$(basename "$sql")"
  cp "$sql" "$RUN_DIR/sql/$base"
done
sha256sum "$RUN_DIR"/sql/*.sql "$ROOT/docs/api/openapi.yaml" > "$RUN_DIR/input.sha256"
log "PLAN mode=$MODE run_id=$RUN_ID source=$SOURCE_HOST:$SOURCE_PORT/$SOURCE_DB target=$TARGET_HOST:$TARGET_PORT/$TARGET_DB"
log "PLAN evidence=$RUN_DIR"

mysql_readonly() {
  local side="$1" host="$2" port="$3" db="$4" defaults="$5" out_dir="$6"
  local combined="$out_dir/combined.tsv" stderr_file="$out_dir/mysql.stderr" sql base
  local -a args=("$MYSQL_BIN")
  # MySQL only reads --defaults-extra-file when it is the first option. Its
  # contents and path are intentionally absent from commands.log.
  [[ -n "$defaults" ]] && args+=(--defaults-extra-file="$defaults")
  args+=(--batch --raw --host "$host" --port "$port" --database "$db")
  log "READONLY mysql side=$side adapter=$(basename "$MYSQL_BIN") database=$db gates=00-12"
  if ! {
    printf 'SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci;\n'
    printf "SET @ab_side = '%s'; SET @ab_run_id = '%s';\n" "$side" "$RUN_ID"
    # TEMPORARY table DDL/DML is session-local and cannot alter either clone.
    # All persistent reads begin only after the explicit read-only transaction.
    cat "$RUN_DIR/clone-manifest-temp.sql"
    printf 'SET SESSION TRANSACTION READ ONLY; START TRANSACTION READ ONLY;\n'
    for sql in "$RUN_DIR/sql"/*.sql; do
      base="$(basename "$sql" .sql)"
      printf "SELECT '__AB_GATE__%s' AS ab_gate_marker;\n" "$base"
      # The external A snapshot may predate the V8 resource/planning tables.
      # Run only common-schema baseline queries there; emit a typed empty
      # result for B-only gates so the evidence envelope is stable.
      if [[ "$side" == A && "$base" != 00_* && "$base" != 01_* && "$base" != 07_* ]]; then
        printf "SELECT CAST(NULL AS CHAR) AS violation_code, CAST(NULL AS CHAR) AS entity_key, CAST(NULL AS CHAR) AS detail WHERE 1=0;\n"
      else
        cat "$sql"
      fi
      printf '\n'
    done
    printf 'ROLLBACK;\n'
  } | "${args[@]}" >"$combined" 2>"$stderr_file"; then
    chmod 600 "$stderr_file"
    return 2
  fi
  chmod 600 "$stderr_file"
  python3 "$RENDER_BIN" split-markers "$combined" "$out_dir" || return 3
  for sql in "$RUN_DIR/sql"/*.sql; do
    base="$(basename "$sql" .sql)"
    [[ -f "$out_dir/$base.tsv" ]] || return 3
    python3 "$RENDER_BIN" render "$out_dir/$base.tsv" "$out_dir/$base.csv" "$out_dir/$base.json" "$side" "$base" >/dev/null || return 4
    sha256sum "$out_dir/$base.tsv" "$out_dir/$base.csv" "$out_dir/$base.json" >"$out_dir/$base.sha256" || return 5
  done
}

api_probe() {
  local label="$1" base_url="$2"
  [[ -z "$base_url" ]] && return 0
  command -v curl >/dev/null 2>&1 || die "curl is required for API probes"
  for path in /health /v1/version; do
    log "READONLY curl label=$label GET=$base_url$path"
    curl --fail-with-body --max-time 10 --silent --show-error "$base_url$path" > "$RUN_DIR/api/${label}_$(tr '/' '_' <<<"${path#/}").body"
  done
}

if [[ "$EXECUTE_READONLY" == true ]]; then
  command -v "$MYSQL_BIN" >/dev/null 2>&1 || die "mysql client is required for --execute-readonly"
  failed_sides=()
  if ! mysql_readonly A "$SOURCE_HOST" "$SOURCE_PORT" "$SOURCE_DB" "$SOURCE_DEFAULTS_FILE" "$RUN_DIR/sql/source"; then
    failed_sides+=(A)
  fi
  if ! mysql_readonly B "$TARGET_HOST" "$TARGET_PORT" "$TARGET_DB" "$TARGET_DEFAULTS_FILE" "$RUN_DIR/sql/target"; then
    failed_sides+=(B)
  fi
  if (( ${#failed_sides[@]} > 0 )); then
    failed_csv="$(IFS=,; printf '%s' "${failed_sides[*]}")"
    python3 "$RENDER_BIN" execution-failure "$RUN_ID" "$failed_csv" "$RUN_DIR/gate_report.json" || true
    sha256sum "$RUN_DIR/gate_report.json" >"$RUN_DIR/gate_report.sha256"
    find "$RUN_DIR" -type f ! -name evidence.sha256 -print0 | sort -z | xargs -0 sha256sum >"$RUN_DIR/evidence.sha256"
    die "one or more MySQL read-only sessions failed; see run-scoped evidence"
  fi
  # Old external state and migrated V8 state are intentionally different. Only
  # immutable legacy-event evidence is direct hash parity; other gates are
  # judged independently against their approved environment manifest.
  base="07_event_history_checksum"
  python3 "$RENDER_BIN" compare "$RUN_DIR/sql/source/$base.json" "$RUN_DIR/sql/target/$base.json" "$RUN_DIR/parity/$base.json" > "$RUN_DIR/parity/$base.sha256"
  report_status=0
  python3 "$RENDER_BIN" gate-report "$RUN_ID" "$RUN_DIR/sql/source" "$RUN_DIR/sql/target" "$RUN_DIR/parity" "$RUN_DIR/gate_report.json" || report_status=$?
  sha256sum "$RUN_DIR/gate_report.json" >"$RUN_DIR/gate_report.sha256"
  if ! api_probe a "$API_A_URL"; then
    python3 "$RENDER_BIN" mark-blocked "$RUN_DIR/gate_report.json" runner.api_probe_failed "A GET-only API probe failed"
    report_status=1
  fi
  if ! api_probe b "$API_B_URL"; then
    python3 "$RENDER_BIN" mark-blocked "$RUN_DIR/gate_report.json" runner.api_probe_failed "B GET-only API probe failed"
    report_status=1
  fi
  sha256sum "$RUN_DIR/gate_report.json" >"$RUN_DIR/gate_report.sha256"
fi

log "FINALIZING evidence checksums"
find "$RUN_DIR" -type f ! -name evidence.sha256 -print0 | sort -z | xargs -0 sha256sum > "$RUN_DIR/evidence.sha256"
printf 'DONE evidence_sha256=%s\n' "$RUN_DIR/evidence.sha256"
if [[ "$EXECUTE_READONLY" == true && "${report_status:-0}" -ne 0 ]]; then
  exit "$report_status"
fi
