#!/usr/bin/env bash
# Generates small release-document entrypoints without duplicating API contracts.
set -euo pipefail

VERSION="${1:-dev}"
OUTPUT_DIR="${2:-}"
if [ -z "$OUTPUT_DIR" ]; then
  echo "Usage: $0 VERSION OUTPUT_DIR" >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"
GEN_TIME="$(date -u +"%Y-%m-%d %H:%M:%S UTC")"

write_guide() {
  local target="$1"
  local title="$2"
  cat >"$target" <<DOC
# ${title}

- Release: ${VERSION}
- Generated: ${GEN_TIME}

This release document intentionally does not copy route, role, workflow, or field definitions.

Current authority, in order:

1. \`transport/http.go\` for mounted routes.
2. \`docs/api/openapi.yaml\` for request and response contracts.
3. \`docs/V1_BACKEND_SOURCE_OF_TRUTH.md\` for route-family governance.

Frontend integration summaries are generated from OpenAPI under \`docs/frontend/\`.
DOC
}

write_guide "$OUTPUT_DIR/API_USAGE_GUIDE.md" "${VERSION} API Usage Entry"
write_guide "$OUTPUT_DIR/API_INTEGRATION_GUIDE.md" "${VERSION} API Integration Entry"

echo "Generated authority-pointer release docs in $OUTPUT_DIR"
