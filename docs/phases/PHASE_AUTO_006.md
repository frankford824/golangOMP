# PHASE_AUTO_006 - ERP Sync Placeholder / Contract

## Why This Phase Now
- `products` has already been established as the ERP master-data entry point, but the repository still lacked a runnable sync placeholder.
- The worker infrastructure already exists, so this is the smallest Phase-06 addition that closes a known infrastructure gap without expanding into RBAC or real external integration.
- This phase matches the auto-phase priority for foundation placeholders and avoids mixing multiple major themes into one round.

## Current Context
- Current `CURRENT_STATE.md`: Step 05 complete, Step 06 listed as ERP placeholder / RBAC placeholder / document consolidation candidate.
- Current OpenAPI version: `0.6.0`
- Latest iterations: `ITERATION_005.md`, `ITERATION_004.md`
- Stable business mainline already present:
  - task create / assign / submit-design
  - task detail aggregate
  - audit / outsource / warehouse flows
  - Step 05 frontend-oriented query enhancement
- Main missing gap for this phase:
  - no ERP sync worker placeholder
  - no sync run history
  - no internal sync status / trigger contract

## Goals
- Add a runnable ERP sync placeholder based on a local stub file.
- Add sync run history and internal status/trigger APIs.
- Keep the implementation explicitly internal and placeholder-only.

## Allowed Scope
- `config/`
- `domain/`
- `repo/`
- `repo/mysql/`
- `service/`
- `transport/handler/`
- `transport/http.go`
- `workers/`
- `db/migrations/`
- `docs/phases/`
- `docs/iterations/`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`

## Forbidden Scope
- Real ERP HTTP/SDK integration
- RBAC enforcement and auth middleware
- NAS / real file upload / whole_hash hardening
- Task / Audit / Warehouse business semantics changes
- Marking internal ERP sync APIs as ready for frontend

## Expected File Changes
- Add ERP sync domain types, provider, service, worker, handler, migration, stub file, and tests.
- Extend product repository with batch upsert support.
- Add `docs/phases/PHASE_AUTO_006.md` and `docs/iterations/ITERATION_006.md`.
- Update `CURRENT_STATE.md` and `docs/api/openapi.yaml`.

## Required API / DB Changes
- Add internal placeholder APIs:
  - `GET /v1/products/sync/status`
  - `POST /v1/products/sync/run`
- Add migration:
  - `erp_sync_runs`
- Reuse existing `products` table; no product schema change in DB.

## Success Criteria
- `go test ./...` passes.
- ERP sync placeholder can:
  - read local stub data
  - upsert products by `erp_product_id`
  - persist sync run history
  - expose status and manual trigger APIs
- OpenAPI, iteration memory, and current state are synchronized to Step 06.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_006.md`
- `docs/api/openapi.yaml`

## Risks
- Sync APIs are intentionally unauthenticated for now and must remain clearly marked as internal placeholder only.
- Stub-file-driven behavior may be mistaken for real ERP integration unless documentation stays explicit.

## Completion Output Format
1. Changed files
2. DB / migration changes
3. API changes
4. Auto-correction notes
5. Risks / remaining gaps
6. Next iteration suggestion
