# PHASE_AUTO_020 - Category Center Skeleton / Cost Rule Skeleton

## Why This Phase Now
- Step 19 already stabilized the task mainline, procurement boundary, warehouse boundary, task-board contract, and workbench bootstrap layer.
- The latest PRD explicitly requires configurable category and cost-rule modules so task filing, procurement, finance, and later ERP mapping stop depending on free-text fields and note-level conventions.
- The most reasonable next increment is to add independent category/cost-rule skeletons plus minimal task-side linkage, without jumping ahead into ERP integration or a full rule-expression engine.

## Current Context
- `CURRENT_STATE.md` before this round reports Step 19 complete and OpenAPI `v0.16.0`.
- Stable V7 foundations already present:
  - task-centric mainline with three task types
  - dedicated procurement persistence and lifecycle
  - warehouse handoff / receive / complete flow
  - close readiness and machine-readable blocking reasons
  - task-board aggregation, ownership hints, and saved workbench preferences
- Current gaps this phase must close:
  - no independent `category` center yet
  - no independent `cost_rule` center yet
  - `task_details.category` is still a loose text field rather than a configurable first-level category entry
  - internal cost-rule provenance is not standardized for later finance/export/API reuse

## Goals
- Add a minimal category center skeleton that treats business sample codes and names as valid first-level categories.
- Add a minimal cost-rule center skeleton that models experience-based cost samples as configurable rules rather than service hardcoding.
- Add minimal task business-info linkage for category code / category identity / cost-rule source so classification and internal cost provenance have standard storage.
- Add sample category and cost-rule initialization artifacts derived from the provided business facts.
- Add OpenAPI contracts, readiness markers, and documentation stating these modules are extensible skeletons.

## Allowed Scope
- `cmd/`
- `config/`
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
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real ERP integration or ERP sync redesign
- NAS / upload work
- Strict `whole_hash` verification
- Real auth / RBAC enforcement
- Full finance system, BI, KPI, or export-center implementation
- Complex formula-expression engine or generic rule DSL parser
- Reworking the already-stable task-board or procurement/warehouse semantics beyond necessary task-side linkage

## Expected File Changes
- Add new V7 tables for `categories` and `cost_rules`.
- Add seed/config artifacts for initial category and cost-rule samples.
- Add category and cost-rule domain models, repo interfaces, MySQL repos, services, handlers, and routes.
- Extend task business-info persistence/read models with category linkage and cost-rule provenance fields.
- Add minimal cost-rule preview support and tests.
- Sync state, phase, iteration, OpenAPI, handover, and V7 spec docs to Step 20.

## Required API / DB Changes
- API:
  - `GET /v1/categories`
  - `GET /v1/categories/search`
  - `GET /v1/categories/{id}`
  - `POST /v1/categories`
  - `PATCH /v1/categories/{id}`
  - `GET /v1/cost-rules`
  - `GET /v1/cost-rules/{id}`
  - `POST /v1/cost-rules`
  - `PATCH /v1/cost-rules/{id}`
  - `POST /v1/cost-rules/preview`
- DB / migration:
  - add `categories`
  - add `cost_rules`
  - extend `task_details` with category linkage and cost-rule provenance fields
  - initialize sample category and cost-rule skeleton data

## Success Criteria
- Category center exists as a configurable first-level category skeleton and accepts coded-style categories as valid primary entries.
- Cost-rule center exists as a configurable skeleton with explicit support for `manual_quote`.
- Task business info can persist category linkage and cost-rule provenance without moving those concerns into procurement notes.
- Minimal preview behavior exists, or if any evaluation case stays intentionally partial, the API contract and docs state the skeleton limits clearly.
- `go test ./...` passes.
- `CURRENT_STATE.md`, `docs/iterations/ITERATION_020.md`, and `docs/api/openapi.yaml` are synchronized.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_020.md`
- `docs/api/openapi.yaml`

Optional but expected for this round:
- `MODEL_HANDOVER.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Some sample rule types are only partially machine-calculable in this phase, so preview semantics must stay explicit about skeleton limitations.
- Category linkage must improve structure without breaking current task workflow readiness rules or existing frontend assumptions around `task_details.category`.
- Seed samples derived from business facts must remain clearly presented as initialization guidance, not as a claim of final complete production taxonomy or full pricing automation.

## Completion Output Format
1. Phase
2. Why this phase now
3. Category center design summary
4. Cost rule skeleton summary
5. Excel sample mapping interpretation summary
6. Category seed mapping summary
7. Cost rule seed mapping summary
8. Changed Files
9. DB / Migration Changes
10. API / OpenAPI Changes
11. Auto-Correction / Verification
12. Risks / Remaining Gaps
13. Next
