# CLAUDE.md

> AUTHORITY ONLY
> 1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
> 2. `docs/api/openapi.yaml`
> 3. `transport/http.go`
>
> Start model handoff with `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`.
> Do not treat `CURRENT_STATE.md`, `MODEL_HANDOVER.md`, `docs/archive/*`,
> `docs/iterations/*`, legacy specs, or model-memory files as current spec.

This file is an assistant guidance note. It is not the backend specification.

## Current Repo Baseline

- MAIN v0.9 authority lives only in the authority set above.
- New frontend or new integrations must start on the canonical MAIN families:
  - `/v1/auth/*`
  - `/v1/users*`
  - `/v1/erp/products*`
  - `/v1/tasks*`
  - `/v1/tasks/{id}/asset-center/*`
- Compatibility and deprecated surfaces remain documented only for migration safety.

## Reading Order

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `transport/http.go`
5. One focused current guide if needed:
   - `docs/TASK_CREATE_RULES.md`
   - `docs/API_USAGE_GUIDE.md`
   - `docs/ASSET_UPLOAD_INTEGRATION.md`
   - `docs/V7_FRONTEND_INTEGRATION_ORDER.md`

## Non-Authoritative Materials

- `CURRENT_STATE.md` and `MODEL_HANDOVER.md` are index-only historical entry files.
- `docs/archive/*` and `docs/iterations/*` are archive or evidence only.
- `docs/archive/legacy_specs/*` and `docs/archive/model_memory/*` are never current API specs.

## Working Rule

When documentation conflicts:

1. `transport/http.go` decides what is mounted.
2. `docs/api/openapi.yaml` decides the current request/response contract.
3. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md` decides route class, successor path, and compatibility policy.
