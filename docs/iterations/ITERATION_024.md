# ITERATION_024 - Task Creation Product Picker Integration

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_024

## 1. Goals

- Wire mapped local ERP product search into original-product task creation and task-side persistence.
- Let `POST /v1/tasks` and `PATCH /v1/tasks/{id}/business-info` accept one explicit `product_selection` provenance object.
- Make the selected existing product traceable back to category/search-entry/mapping context without introducing real ERP APIs.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 23 complete and OpenAPI `v0.20.0`.
- Step 23 already made mapped local product search executable through:
  - `category_id`
  - `category_code`
  - `search_entry_code`
  - `mapping_match`
  - `matched_*` result provenance
- The main remaining PRD gap was that original-product task creation still only accepted legacy `product_id/sku_code/product_name_snapshot` binding and could not persist how that product had been located.
- This round stayed out of scope for:
  - real ERP API lookup
  - sync enhancement
  - full second/third-level category tree implementation
  - full search engine work
  - NAS / upload / strict `whole_hash`

## 3. Files Changed

### Code

- `db/migrations/013_v7_task_product_picker_integration.sql`
- `domain/query_views.go`
- `domain/task.go`
- `domain/task_detail_aggregate.go`
- `domain/task_product_selection.go`
- `repo/interfaces.go`
- `repo/mysql/task.go`
- `service/task_detail_service.go`
- `service/task_prd_service_test.go`
- `service/task_product_selection.go`
- `service/task_service.go`
- `service/task_step04_service_test.go`
- `transport/handler/task.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_024.md`
- `docs/phases/PHASE_AUTO_024.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

### Migration 013

- Extended `task_details` with persisted original-product picker provenance:
  - `source_product_id`
  - `source_product_name`
  - `source_search_entry_code`
  - `source_match_type`
  - `source_match_rule`
  - `matched_category_code`
  - `matched_search_entry_code`
  - `matched_mapping_rule_json`
- Added lightweight indexes for:
  - `source_product_id`
  - `source_search_entry_code + matched_search_entry_code`

## 5. API Changes

### `POST /v1/tasks`

- Added additive `product_selection` support for original-product picker flows.
- `sku_code` remains accepted directly, but can now also be supplied through:
  - `product_selection.selected_product_sku_code`
- `product_id/sku_code/product_name_snapshot` compatibility is preserved.

### `PATCH /v1/tasks/{id}/business-info`

- Added additive `product_selection` support so existing-product tasks can:
  - persist mapped-search provenance
  - later refine that provenance
  - rebind the selected existing product when needed

### Task read/detail traceability

- `GET /v1/tasks/{id}` now exposes top-level:
  - `product_selection`
- `GET /v1/tasks/{id}/detail` now also exposes top-level:
  - `product_selection`
- `TaskDetail` responses now include nested:
  - `product_selection`

### `product_selection` contract

- Current object carries:
  - `selected_product_id`
  - `selected_product_name`
  - `selected_product_sku_code`
  - `matched_category_code`
  - `matched_search_entry_code`
  - `matched_mapping_rule`
  - `source_product_id`
  - `source_product_name`
  - `source_match_type`
  - `source_match_rule`
  - `source_search_entry_code`

### OpenAPI

- Version updated from `0.20.0` to `0.21.0`.
- Clarified the handoff from `GET /v1/products/search` into task `product_selection`.
- Clarified that this remains a local ERP positioning flow, not a real ERP lookup API.

## 6. Design Decisions

- Kept the mapped-search execution boundary on `GET /v1/products/search`; task APIs only consume and persist its result/provenance.
- Persisted original-product picker provenance on `task_details` instead of hardcoding category-to-ERP matching into `tasks` or `products`.
- Used one explicit `product_selection` object instead of scattering more flat request fields across task create and business-info contracts.
- Preserved legacy task-create compatibility by keeping `product_id/sku_code/product_name_snapshot` accepted and auto-building a minimal legacy selection trace when mapped provenance is absent.
- Allowed business-info updates to rebind existing-product tasks, because otherwise original-product picker integration would remain read-only and incomplete.

## 7. Correction Notes

- `CURRENT_STATE.md`, `MODEL_HANDOVER.md`, `docs/V7_MODEL_HANDOVER_APPENDIX.md`, and `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md` still described original-product selector integration as future work before this round.
- This iteration reconciled repository-truth docs with the new task-side `product_selection` persistence and read-model exposure.

## 8. Verification

- Ran `gofmt -w` on all touched Go files.
- Added service tests covering:
  - mapped-search-backed original-product creation persistence
  - business-info existing-product rebinding plus provenance persistence
  - legacy existing-product selection fallback on read
- Ran:
  - `go test ./service/...`
  - `go test ./...`

## 9. Risks / Known Gaps

- `product_selection` still persists frontend-provided local mapped-search provenance; it does not independently validate against a real ERP API.
- Current fallback labels such as `mapped_product_search`, `manual_existing_product_binding`, and `legacy_existing_product_binding` are intentionally lightweight trace strings, not a finished enterprise taxonomy.
- Secondary/tertiary search refinement remains lightweight and reserved-field-based only.
- Frontend page flow is still future work even though the backend contract is now complete enough for integration.

## 10. Suggested Next Step

- Keep real ERP integration deferred.
- The next reasonable phase is to tighten frontend/detail consumption of persisted `product_selection` and decide whether any extra lightweight secondary/tertiary narrowing or validation is needed for original-product picker UX.
