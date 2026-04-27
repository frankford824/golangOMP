# PHASE_AUTO_022 - ERP Category Mapping Skeleton

## Why This Phase Now
- Step 21 already turned category center and cost-rule center into active task-side inputs, so category is no longer just passive configuration.
- The next PRD gap is not more pricing logic; it is the missing bridge from category selection to ERP product positioning.
- Repository truth before this round already identified the main deficit as:
  - category center not formally entering ERP positioning
  - total category code not explicitly modeled as the first-level search entry
  - no independent category-to-ERP mapping skeleton

## Current Context
- `CURRENT_STATE.md` before this round reports Step 21 complete and OpenAPI `v0.18.0`.
- Stable V7 foundations already present:
  - task/business-info category selection
  - task-side cost preview / prefill
  - procurement summary cost / provenance exposure
  - configurable category center skeleton
  - configurable cost-rule center skeleton
- Current gaps this phase must close:
  - category records do not explicitly model first-level ERP search-entry semantics
  - category and ERP product positioning still have no independent mapping domain
  - later second/third-level ERP search refinement has no reserved standard structure

## Goals
- Explicitly model `search_entry_code` and `is_search_entry` on category center records so “总分类编码 = 一级搜索入口” is no longer implicit.
- Add an independent `category_erp_mappings` skeleton with admin/query APIs and persistence.
- Reserve standard secondary/tertiary condition fields for later ERP search refinement without implementing a full multi-level category tree or real ERP lookup.
- Keep current task/business-info category / cost-prefill contracts stable and non-regressive.
- Synchronize repository-truth documents and OpenAPI to Step 22 / `v0.19.0`.

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
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real ERP API integration or sync enhancement
- Real ERP lookup execution inside task or product search
- NAS / upload implementation
- Strict `whole_hash` verification
- Real auth / RBAC enforcement
- Full finance module, BI, KPI, or export-center implementation
- Full category second/third-level tree implementation
- Full search engine or formula engine

## Expected File Changes
- Add a migration extending `categories` with explicit search-entry semantics and creating `category_erp_mappings`.
- Add new domain/repo/service/handler/router skeletons for category-to-ERP mappings.
- Extend category API payloads with explicit first-level search-entry fields.
- Add service tests covering:
  - top-level category search-entry semantics
  - child-category inheritance boundary
  - category-to-ERP mapping creation / validation
- Sync phase, iteration, state, handover, OpenAPI, and V7 docs to Step 22.

## Required API / DB Changes
- API:
  - extend category contracts with `search_entry_code` and `is_search_entry`
  - add:
    - `GET /v1/category-mappings`
    - `GET /v1/category-mappings/search`
    - `GET /v1/category-mappings/{id}`
    - `POST /v1/category-mappings`
    - `PATCH /v1/category-mappings/{id}`
- DB / migration:
  - extend `categories` with explicit first-level search-entry fields
  - add `category_erp_mappings` with reserved secondary/tertiary search-condition fields

## Success Criteria
- Category records explicitly persist whether they are first-level ERP search entries and which `search_entry_code` they belong to.
- Top-level categories enforce `search_entry_code == category_code` and `is_search_entry=true`.
- Independent category-to-ERP mapping skeleton exists with persistence, CRUD/search APIs, and OpenAPI coverage.
- Mapping schema supports:
  - `category_id`
  - `category_code`
  - `search_entry_code`
  - `erp_match_type`
  - `erp_match_value`
  - `is_primary`
  - `is_active`
  - `priority`
  - `remark`
- Reserved secondary/tertiary condition fields are present without claiming full second/third-level hierarchy support.
- `go test ./...` passes.
- `CURRENT_STATE.md`, `docs/iterations/ITERATION_022.md`, and `docs/api/openapi.yaml` are synchronized.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_022.md`
- `docs/api/openapi.yaml`

Expected in this round because repository truth was stale:
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Mapping APIs must stay clearly labeled as skeleton positioning contracts, not real ERP lookup.
- Explicit first-level search-entry modeling must not accidentally imply a finished second/third-level taxonomy.
- Category, cost-rule, and task-side prefill contracts must not drift while adding the ERP-positioning layer.

## Completion Output Format
1. Phase path
2. Changed files
3. DB / migration changes
4. API / OpenAPI changes
5. Auto-correction notes
6. Verification
7. Risks / remaining gaps
8. Next recommended phase
