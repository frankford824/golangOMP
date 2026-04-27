#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GO_BIN="${GO_BIN:-/home/wsfwk/go/bin/go}"
REMOTE_ROOT="/root/ecommerce_ai/r3_5"

cd "$ROOT"

echo "== 1. Upload R3.5 scripts =="
ssh jst_ecs "mkdir -p '$REMOTE_ROOT/bin' '$REMOTE_ROOT/sql'"
scp scripts/r35/setup_test_db.sh scripts/r35/build_test_dsn.sh jst_ecs:"$REMOTE_ROOT"/

echo "== 2. Setup test DB on jst_ecs =="
ssh jst_ecs "bash '$REMOTE_ROOT/setup_test_db.sh'"

echo "== 3. Build linux binaries =="
GOOS=linux GOARCH=amd64 "$GO_BIN" build -o /tmp/r2_forward ./cmd/tools/migrate_v1_forward
GOOS=linux GOARCH=amd64 "$GO_BIN" build -o /tmp/r2_backfill ./cmd/tools/migrate_v1_backfill
GOOS=linux GOARCH=amd64 "$GO_BIN" build -o /tmp/r2_rollback ./cmd/tools/migrate_v1_rollback
scp /tmp/r2_forward /tmp/r2_backfill /tmp/r2_rollback jst_ecs:"$REMOTE_ROOT"/bin/
scp db/migrations/059_*.sql db/migrations/060_*.sql db/migrations/061_*.sql \
    db/migrations/062_*.sql db/migrations/063_*.sql db/migrations/064_*.sql \
    db/migrations/065_*.sql db/migrations/066_*.sql db/migrations/067_*.sql \
    db/migrations/068_*.sql jst_ecs:"$REMOTE_ROOT"/sql/

echo "== 4. R2 forward + backfill on test DB =="
ssh jst_ecs "cd /root/ecommerce_ai && . ./shared/main.env && \
  DSN=\"\${DB_USER}:\${DB_PASS}@tcp(\${DB_HOST}:\${DB_PORT})/jst_erp_r3_test?parseTime=true&multiStatements=true\" && \
  '$REMOTE_ROOT/bin/r2_forward' --dsn=\"\$DSN\" --sql-dir='$REMOTE_ROOT/sql' --r35-mode=true && \
  '$REMOTE_ROOT/bin/r2_backfill' --dsn=\"\$DSN\" --r35-mode=true"

echo "== 5. Intentional guard attack must exit 4 =="
ssh jst_ecs "cd /root/ecommerce_ai && . ./shared/main.env && \
  DSN=\"\${DB_USER}:\${DB_PASS}@tcp(\${DB_HOST}:\${DB_PORT})/jst_erp?parseTime=true\" && \
  set +e; '$REMOTE_ROOT/bin/r2_forward' --dsn=\"\$DSN\" --sql-dir='$REMOTE_ROOT/sql' --r35-mode=true 2>&1; code=\$?; echo exit=\$code; test \"\$code\" = 4"

echo "== 6. Run local integration tests through SSH tunnel =="
TUNNEL_PORT="${TUNNEL_PORT:-33306}"
REMOTE_DB_ADDR="$(ssh jst_ecs 'cd /root/ecommerce_ai && . ./shared/main.env && echo "${DB_HOST}:${DB_PORT}"')"
ssh -N -L "${TUNNEL_PORT}:${REMOTE_DB_ADDR}" jst_ecs &
TUNNEL_PID=$!
trap 'kill "$TUNNEL_PID" >/dev/null 2>&1 || true' EXIT
sleep 2

REMOTE_DSN="$(ssh jst_ecs "bash '$REMOTE_ROOT/build_test_dsn.sh'")"
LOCAL_DSN="$(printf '%s' "$REMOTE_DSN" | sed -E "s/@tcp\\([^)]*\\)/@tcp(127.0.0.1:${TUNNEL_PORT})/")"

MYSQL_DSN="$LOCAL_DSN" R35_MODE=1 "$GO_BIN" test ./cmd/tools/internal/v1migrate/... -run TestGuardR35DSN -v
MYSQL_DSN="$LOCAL_DSN" R35_MODE=1 "$GO_BIN" test ./service/task_pool/... -tags=integration -run TestClaimCAS_100Concurrent_MySQL -v -count=1
MYSQL_DSN="$LOCAL_DSN" R35_MODE=1 "$GO_BIN" test ./... -tags=integration -run "Integration|TestClaimCAS_TwoClaims_MySQL" -v -count=1
"$GO_BIN" test ./... -count=1
"$GO_BIN" run ./cmd/tools/openapi-validate docs/api/openapi.yaml

echo "DONE"
