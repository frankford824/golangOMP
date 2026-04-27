# ITERATION_021 - Category Selection / Cost Prefill Integration

**Date**: 2026-03-10  
**Scope**: STEP_21

## 1. Goals

- Turn the new category center and cost-rule center skeletons into a direct task-side usage path.
- Let `PATCH /v1/tasks/{id}/business-info` accept minimal cost-prefill inputs and persist system prefill vs manual override boundaries.
- Make `purchase_task` read/list/detail procurement summaries expose internal cost and rule provenance more clearly without moving procurement ownership back into task details.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 20 complete and OpenAPI `v0.17.0`.
- Step 20 already delivered category-center and cost-rule-center skeleton APIs plus minimal preview support.
- This round explicitly stayed out of scope for:
  - real ERP integration
  - NAS / upload work
  - strict `whole_hash` verification
  - full finance / BI / export-center implementation
  - complete category tree expansion
  - a general-purpose formula engine

## 3. Files Changed

### Code

- `db/migrations/011_v7_task_cost_prefill_integration.sql`
- `domain/procurement.go`
- `domain/query_views.go`
- `domain/task.go`
- `repo/mysql/task.go`
- `service/cost_prefill.go`
- `service/cost_rule_service.go`
- `service/procurement_summary.go`
- `service/task_detail_service.go`
- `service/task_prd_service_test.go`
- `service/task_service.go`
- `transport/handler/task.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_021.md`
- `docs/phases/PHASE_AUTO_021.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- Added `db/migrations/011_v7_task_cost_prefill_integration.sql`.
- Extended `task_details` with minimal cost-prefill input fields:
  - `width`
  - `height`
  - `area`
  - `quantity`
  - `process`
- Extended `task_details` with persisted prefill / override state:
  - `estimated_cost`
  - `requires_manual_review`
  - `manual_cost_override`
  - `manual_cost_override_reason`

## 5. API Changes

### Task business-info integration

- `PATCH /v1/tasks/{id}/business-info` now also accepts:
  - `width`
  - `height`
  - `area`
  - `quantity`
  - `process`
  - `manual_cost_override`
  - `manual_cost_override_reason`
- The same route now persists:
  - `estimated_cost`
  - `requires_manual_review`
  - `manual_cost_override`
  - `manual_cost_override_reason`
- When category plus minimal inputs are present, business-info update now triggers the same skeleton cost-preview logic used by `POST /v1/cost-rules/preview`.

### Prefill contract

- `estimated_cost` is now the last system preview/prefill result.
- `cost_price` remains the current effective internal cost.
- When `manual_cost_override=false`, backend prefers synchronized system prefill into `cost_price`.
- When `manual_cost_override=true`, `cost_price` stays user-entered while `estimated_cost` remains the last system estimate.
- Manual override is explicitly documented as a business field behavior, not a permission-system behavior.

### Procurement integration

- `procurement_summary` on `GET /v1/tasks`, `GET /v1/tasks/{id}`, and `GET /v1/tasks/{id}/detail` now also exposes:
  - `category_code`
  - `category_name`
  - `cost_price`
  - `estimated_cost`
  - `cost_rule_name`
  - `cost_rule_source`
  - `requires_manual_review`
  - `manual_cost_override`
  - `manual_cost_override_reason`

### OpenAPI

- Version updated from `0.17.0` to `0.18.0`.
- Clarified preview vs prefill relationship:
  - `/v1/cost-rules/preview` is the stateless skeleton preview contract
  - `PATCH /v1/tasks/{id}/business-info` persists task-side prefill / override state
- Clarified that `manual_cost_override` is not an auth or RBAC feature.

## 6. Design Decisions

- Reused one shared skeleton preview path for both the stateless preview API and task-side business-info prefill so the rule behavior does not drift.
- Kept prefill inputs intentionally narrow (`width/height/area/quantity/process`) instead of expanding toward a full cost engine.
- Preserved the existing procurement ownership boundary:
  - procurement still owns `procurement_price`
  - business-info still owns internal cost / rule provenance / manual override state
- Exposed procurement-facing internal cost signals through `procurement_summary` instead of duplicating or relocating procurement data.

## 7. Correction Notes

- Repository-truth docs and implementation were consistent at Step 20 / OpenAPI `v0.17.0` before development, so no rollback/reconciliation-only round was needed first.
- One behavior correction was applied inside the shared skeleton preview path:
  - area-dependent fixed/threshold/min-area rules now require actual area-style input instead of silently defaulting billable area to `1`

## 8. Verification

- Ran `gofmt -w` on all touched Go files.
- Added service tests covering:
  - system cost prefill after category selection
  - separation of manual override from system estimate
  - manual-review fallback for `manual_quote`
  - procurement summary exposure of cost/provenance signals
- Ran:
  - `go test ./service/...`
  - `go test ./transport/... ./repo/mysql/...`
  - `go test ./...`

## 9. Risks / Known Gaps

- Cost preview is still intentionally a skeleton, not a full pricing engine.
- Some categories still require manual review because `manual_quote` or unsupported `size_based_formula` cases remain unresolved by design.
- ERP mapping is still deferred; this iteration only ensures category/cost skeletons are actually used in daily task/procurement flows first.

## 10. Suggested Next Step

- Keep full ERP integration and formula-engine expansion deferred.
- The next reasonable phase is category-to-ERP mapping skeleton work that consumes the now-active task-side category / cost-prefill usage path, rather than expanding the pricing engine further.
