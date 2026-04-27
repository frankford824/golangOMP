# PHASE_AUTO_021 - Category Selection / Cost Prefill Integration

## Why This Phase Now
- Step 20 already landed category center and cost-rule center skeletons, but they are still mostly passive configuration surfaces.
- The current biggest PRD gap is no direct business path from task classification to cost assistance, so category and cost-rule data are not yet driving daily task actions.
- ERP mapping is explicitly deferred until the skeleton contracts are first consumed by real task and procurement workflows.

## Current Context
- `CURRENT_STATE.md` before this round reports Step 20 complete and OpenAPI `v0.17.0`.
- Stable V7 foundations already present:
  - three task types with converged task mainline
  - dedicated procurement persistence and lifecycle
  - warehouse handoff / receive / complete flow
  - category center skeleton
  - cost-rule center skeleton
  - minimal `POST /v1/cost-rules/preview`
- Current gaps this phase must close:
  - `PATCH /v1/tasks/{id}/business-info` does not yet create a direct category-selection to cost-prefill path
  - preview results are not persisted as structured task-side prefill state
  - procurement-facing task read models do not clearly surface internal cost, rule provenance, and manual override state

## Goals
- Extend task business-info maintenance so category selection plus minimal size/area/quantity/process inputs can trigger cost preview and cost prefill.
- Persist a clear task-side boundary between:
  - system prefill
  - manual override
  - rule provenance
  - manual-review requirement
- Surface procurement-facing summary fields that make purchase price, internal estimated/prefilled cost, and rule provenance clearer for `purchase_task`.
- Keep category center and cost-rule center as lightweight configurable skeletons rather than turning them into ERP integration or a full pricing engine.
- Synchronize OpenAPI and state documents so the frontend can consume the new contract directly.

## Allowed Scope
- `cmd/`
- `db/migrations/`
- `domain/`
- `repo/`
- `service/`
- `transport/`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/`
- `docs/phases/`
- `璁捐娴佽浆鑷姩鍖栫鐞嗙郴缁焈V7.0_閲嶆瀯鐗坃鎶€鏈疄鏂借鏍?md`

## Forbidden Scope
- Real ERP integration or ERP mapping workers
- NAS / upload implementation
- Strict `whole_hash` verification
- Real auth / RBAC enforcement
- Full finance module, BI, KPI, or export-center implementation
- Complex formula engine or general rule-expression parser
- Full category multi-level expansion

## Expected File Changes
- Add a migration extending `task_details` with minimal cost-prefill input/output fields.
- Extend task detail and procurement summary read models with prefill / override / provenance fields.
- Refactor cost preview logic into reusable helpers so task business-info updates can trigger preview without duplicating rule behavior.
- Add task-service tests for:
  - category-driven system prefill
  - manual override persistence
  - manual-review fallback
- Sync phase, iteration, state, handover, OpenAPI, and V7 spec docs to Step 21.

## Required API / DB Changes
- API:
  - extend `PATCH /v1/tasks/{id}/business-info` request with minimal preview inputs and manual-override fields
  - extend `TaskDetail` response contract with persisted prefill / override / provenance state
  - extend procurement-facing summary contract with internal cost and provenance fields
- DB / migration:
  - extend `task_details` with minimal preview-input fields
  - extend `task_details` with estimated-cost / manual-override / manual-review persistence fields

## Success Criteria
- `PATCH /v1/tasks/{id}/business-info` can persist category linkage plus width/height/area/quantity/process inputs.
- Business-info update can trigger cost preview and automatically prefill internal cost when the skeleton rules can estimate it.
- Task data clearly distinguishes `estimated_cost` from current `cost_price` and records whether the latter is manually overridden.
- `purchase_task` read models expose clearer procurement-facing cost/provenance fields without moving procurement ownership back into task details.
- `go test ./...` passes.
- `CURRENT_STATE.md`, `docs/iterations/ITERATION_021.md`, and `docs/api/openapi.yaml` are synchronized.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_021.md`
- `docs/api/openapi.yaml`

Optional but expected for this round:
- `MODEL_HANDOVER.md`
- `璁捐娴佽浆鑷姩鍖栫鐞嗙郴缁焈V7.0_閲嶆瀯鐗坃鎶€鏈疄鏂借鏍?md`

## Risks
- The preview contract must stay explicit that this is still a skeleton and not a full formula engine.
- Auto-prefill behavior must not silently blur the boundary between system estimate and manual override.
- Purchase-task summary enhancements must remain presentation-oriented and not collapse procurement data back into task-detail ownership.

## Completion Output Format
1. Phase path
2. Changed files
3. DB / migration changes
4. API / OpenAPI changes
5. Auto-correction notes
6. Verification
7. Risks / remaining gaps
8. Next recommended phase
