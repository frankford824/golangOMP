#!/usr/bin/env bash
# agent-check.sh — consolidated full-gate verification for AI coding agents.
#
# Runs the checks that AGENTS.md §After Editing requires plus repository-local
# migration guards:
#   1. go vet ./...
#   2. go build ./...
#   3. go test ./... -count=1
#   4. experience migration guard
#   5. openapi-validate docs/api/openapi.yaml
#   6. contract_audit (drift gate)
#
# Each step prints a banner. The first failing step exits non-zero;
# subsequent steps are skipped so the failure is obvious in the log.
#
# Environment overrides:
#   GO_BIN       — path to go binary (default: `go`, falls back to /home/wsfwk/go/bin/go)
#   AGENT_CHECK_SKIP_TESTS=1   — skip step 3 (only when iterating fast on docs/openapi)
#   AGENT_CHECK_AUDIT_OUT      — output path for contract_audit JSON (default: tmp/agent_check_audit.json)

set -euo pipefail

go_bin="${GO_BIN:-go}"
if ! command -v "$go_bin" >/dev/null 2>&1 && [[ -x /home/wsfwk/go/bin/go ]]; then
  go_bin=/home/wsfwk/go/bin/go
fi

audit_out="${AGENT_CHECK_AUDIT_OUT:-tmp/agent_check_audit.json}"
mkdir -p "$(dirname "$audit_out")"

step() {
  echo ""
  echo "==> [$1] $2"
}

fail() {
  echo ""
  echo "FAIL at step [$1]. Stop and fix the cause. Do not bypass." >&2
  exit 1
}

step "1/6" "go vet ./..."
"$go_bin" vet ./... || fail "1/6 go vet"

step "2/6" "go build ./..."
"$go_bin" build ./... || fail "2/6 go build"

if [[ "${AGENT_CHECK_SKIP_TESTS:-0}" == "1" ]]; then
  step "3/6" "go test ./... -count=1   [SKIPPED via AGENT_CHECK_SKIP_TESTS=1]"
else
  step "3/6" "go test ./... -count=1"
  "$go_bin" test ./... -count=1 || fail "3/6 go test"
fi

step "4/6" "experience migration guard"
python3 scripts/check_experience_migrations.py || fail "4/6 experience migration guard"

step "5/6" "openapi-validate docs/api/openapi.yaml"
"$go_bin" run ./cmd/tools/openapi-validate docs/api/openapi.yaml || fail "5/6 openapi-validate"

step "6/6" "contract_audit --fail-on-drift true   (output: $audit_out)"
"$go_bin" run ./tools/contract_audit \
  --transport transport/http.go \
  --handlers transport/handler \
  --domain domain \
  --openapi docs/api/openapi.yaml \
  --output "$audit_out" \
  --fail-on-drift true || fail "6/6 contract_audit (drift detected — read $audit_out)"

echo ""
echo "PASS — all 6 checks green."
