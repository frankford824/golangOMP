# CLAUDE.md

> AUTHORITY ONLY
> 1. `transport/http.go`
> 2. `docs/api/openapi.yaml`
> 3. `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`
>
> Historical background only: `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`.
> Do not treat `CURRENT_STATE.md`, `MODEL_HANDOVER.md`, `docs/archive/*`,
> `docs/iterations/*`, legacy specs, model-memory files, or prompts as current spec.

This file is an assistant guidance note. It is not the backend specification.

## Current Repo Baseline

- V1 current authority is centralized in `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`.
- Route existence is decided by `transport/http.go`.
- Request/response field contracts are decided by `docs/api/openapi.yaml`.
- New frontend or new integrations must start from the V1 SoT route families and the generated frontend docs under `docs/frontend/`.
- Compatibility and deprecated surfaces remain documented only for migration safety.

## Reading Order

1. `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`
3. `transport/http.go`
4. `docs/iterations/V1_2_RETRO_REPORT.md`
5. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md` for historical background only

## Non-Authoritative Materials

- `CURRENT_STATE.md` and `MODEL_HANDOVER.md` are historical entry files.
- `docs/archive/*` and `docs/iterations/*` are archive or evidence only unless restated in the V1 SoT.
- `docs/archive/legacy_specs/*` and `docs/archive/model_memory/*` are never current API specs.
- `prompts/*` are execution history, not current contract authority.

## Working Rule

When documents disagree:

1. `transport/http.go` decides what is mounted.
2. `docs/api/openapi.yaml` decides the current request/response contract.
3. `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` decides route family, governance state, and milestone pointers.
4. All other documents are evidence or history.
