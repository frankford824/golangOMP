# API Usage Guide

This file is only an entry pointer. It intentionally contains no duplicated route or role contract.

Current integration authority, in order:

1. `transport/http.go` — mounted routes.
2. `docs/api/openapi.yaml` — request and response contracts.
3. `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` — route-family governance and implementation status.

Generated frontend-facing route summaries are under `docs/frontend/` and must be regenerated from OpenAPI.
The pre-V8 guide is preserved at `docs/archive/legacy_specs/API_USAGE_GUIDE_pre_v8.md` for audit only.
