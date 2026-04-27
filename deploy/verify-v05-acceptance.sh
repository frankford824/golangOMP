#!/usr/bin/env bash
# v0.5 API acceptance verification script.
# Usage: BASE_URL=http://127.0.0.1:8080 AUTH_TOKEN="Bearer <token>" bash deploy/verify-v05-acceptance.sh
# Or: bash deploy/verify-v05-acceptance.sh --base-url http://127.0.0.1:8080 --token "Bearer xxx"
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
AUTH_TOKEN="${AUTH_TOKEN:-}"
while [ $# -gt 0 ]; do
  case "$1" in
    --base-url) BASE_URL="$2"; shift 2 ;;
    --token)    AUTH_TOKEN="$2"; shift 2 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

if [ -z "$AUTH_TOKEN" ]; then
  echo "Set AUTH_TOKEN (Bearer token) or pass --token" >&2
  exit 1
fi

CURL_OPTS=(-s -w "\n%{http_code}" -H "Authorization: $AUTH_TOKEN" -H "Content-Type: application/json")
PASS=0
FAIL=0

run() {
  local name="$1"
  local method="$2"
  local path="$3"
  local data="${4:-}"
  local want_code="${5:-200}"
  local out
  local code

  if [ -n "$data" ]; then
    out=$(curl "${CURL_OPTS[@]}" -X "$method" -d "$data" "$BASE_URL$path" 2>/dev/null || true)
  else
    out=$(curl "${CURL_OPTS[@]}" -X "$method" "$BASE_URL$path" 2>/dev/null || true)
  fi
  code=$(echo "$out" | tail -n1)
  if [ "$code" = "$want_code" ]; then
    echo "  [PASS] $name (HTTP $code)"
    ((PASS++)) || true
    return 0
  else
    echo "  [FAIL] $name (got $code, want $want_code)"
    echo "    body: $(echo "$out" | head -n-1 | head -c 200)"
    ((FAIL++)) || true
    return 1
  fi
}

echo "=== v0.5 Acceptance Verification ==="
echo "BASE_URL=$BASE_URL"
echo ""

# A. Designers list
echo "--- A. GET /v1/users/designers ---"
run "designers list" GET "/v1/users/designers" "" 200
echo ""

# B. Server logs (Admin)
echo "--- B. Server logs (Admin) ---"
run "server-logs list" GET "/v1/server-logs?page=1&page_size=5" "" 200
run "server-logs clean" POST "/v1/server-logs/clean" '{"older_than_hours":720,"reason":"v05-acceptance-test"}' 200
echo ""

# C. Rule templates
echo "--- C. Rule templates ---"
run "rule-templates list" GET "/v1/rule-templates" "" 200
run "rule-templates cost-pricing" GET "/v1/rule-templates/cost-pricing" "" 200
echo ""

# D. Task create with designer_id (requires valid owner_team, due_at)
echo "--- D. Task create (minimal) ---"
# owner_team must be one of: 娴滃搫濮忕悰灞炬杺缂?鐠佹崘顓哥紒?閸愬懓閿ゆ潻鎰儉缂?闁插洩鍠樻禒鎾冲亶缂?閹崵绮￠崝鐐电矋
TASK_JSON='{"task_type":"new_product_development","owner_team":"鐠佹崘顓哥紒?,"due_at":"2026-12-31T23:59:59Z","product_name_snapshot":"v05-test","design_requirement":"test","material_mode":"preset","product_short_name":"v05"}'
run "task create minimal" POST "/v1/tasks" "$TASK_JSON" 201
echo ""

# E. Reference image validation (illegal: too many)
echo "--- E. Reference image validation (illegal) ---"
# 6 refs = over limit
ILLEGAL_JSON='{"task_type":"new_product_development","owner_team":"鐠佹崘顓哥紒?,"due_at":"2026-12-31T23:59:59Z","product_name_snapshot":"v05","design_requirement":"x","material_mode":"preset","product_short_name":"v05","reference_file_refs":["a","b","c","d","e","f"]}'
run "task create with 6 refs -> 400" POST "/v1/tasks" "$ILLEGAL_JSON" 400
echo ""

echo "=== Summary ==="
echo "PASS: $PASS  FAIL: $FAIL"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
