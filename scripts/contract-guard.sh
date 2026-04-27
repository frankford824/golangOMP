#!/usr/bin/env bash
set -euo pipefail

changed="${CONTRACT_GUARD_CHANGED_FILES:-}"
if [[ -z "$changed" ]]; then
  changed="$(git diff --cached --name-only)"
fi

code_changed=0
contract_changed=0
while IFS= read -r file; do
  [[ -z "$file" ]] && continue
  if [[ "$file" =~ ^(transport/handler/.*\.go|service/.*\.go|domain/.*\.go)$ ]]; then
    code_changed=1
  fi
  if [[ "$file" == "docs/api/openapi.yaml" ]]; then
    contract_changed=1
  fi
done <<< "$changed"

msg="${CONTRACT_GUARD_COMMIT_MESSAGE:-}"
if [[ -z "$msg" ]]; then
  msg="$(git log -1 --pretty=%B 2>/dev/null || true)"
fi

if [[ "$code_changed" == 0 && "$contract_changed" == 0 ]]; then
  exit 0
fi

if [[ "$code_changed" == 1 && "$contract_changed" == 0 ]]; then
  if [[ "$msg" == *"[contract-skip-justified]"* ]]; then
    exit 0
  fi
  echo "contract-guard: code contract surface changed without docs/api/openapi.yaml" >&2
  echo "Add OpenAPI changes in the same commit or include [contract-skip-justified] for architect review." >&2
  exit 1
fi

go_bin="${GO_BIN:-go}"
if ! command -v "$go_bin" >/dev/null 2>&1 && [[ -x /home/wsfwk/go/bin/go ]]; then
  go_bin=/home/wsfwk/go/bin/go
fi

timeout 60s "$go_bin" run ./tools/contract_audit \
  --transport transport/http.go \
  --handlers transport/handler \
  --domain domain \
  --openapi docs/api/openapi.yaml \
  --output tmp/v1_2/contract_guard_audit.json \
  --fail-on-drift true
