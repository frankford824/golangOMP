# ITERATION_025 - Product Selection Read-Model Integration

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_025

## 1. Goals

- Make `product_selection` a first-class read-model object instead of a detail-only add-on.
- Expose stable `product_selection` summary on task list, task board, and procurement summary layers.
- Preserve full task read/detail provenance so frontend can replay how an original product was located without rebuilding matched/source fields client-side.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 24 complete and OpenAPI `v0.21.0`.
- Step 24 already completed the write-side handoff from mapped local ERP product search into task-side persistence through additive `product_selection`.
- The main remaining PRD/frontend gap was read-side inconsistency:
  - `GET /v1/tasks/{id}` and `/detail` exposed `product_selection`
  - `GET /v1/tasks`, task-board payloads, and `procurement_summary` did not yet expose one stable `product_selection` summary contract
  - frontend would still need to merge `matched_*`, `source_*`, and selected-product fields itself in some pages
- This round remained out of scope for:
  - real ERP API lookup
  - sync enhancement
  - full second/third-level category tree implementation
  - full search engine work
  - NAS / upload / strict `whole_hash`

## 3. Files Changed

### Code

- `domain/procurement.go`
- `domain/query_views.go`
- `domain/task_product_selection.go`
- `repo/mysql/task.go`
- `service/procurement_summary.go`
- `service/task_product_selection.go`
- `service/task_query.go`
- `service/task_board_service_test.go`
- `service/task_prd_service_test.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_025.md`
- `docs/phases/PHASE_AUTO_025.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- No new migration in this round.
- Reused Step 24 task-detail persistence fields:
  - `source_product_id`
  - `source_product_name`
  - `source_search_entry_code`
  - `source_match_type`
  - `source_match_rule`
  - `matched_category_code`
  - `matched_search_entry_code`
  - `matched_mapping_rule_json`

## 5. API Changes

### `GET /v1/tasks`

- Task list items now expose lightweight:
  - `product_selection`
- This summary view carries stable frontend-facing fields:
  - `selected_product_id`
  - `selected_product_name`
  - `selected_product_sku_code`
  - `matched_category_code`
  - `matched_search_entry_code`
  - `source_product_id`
  - `source_product_name`
  - `source_match_type`
  - `source_match_rule`
  - `source_search_entry_code`

### `GET /v1/task-board/summary` and `GET /v1/task-board/queues`

- Board sample tasks and queue tasks now inherit the same task-item `product_selection` summary contract used by `GET /v1/tasks`.

### `procurement_summary`

- `procurement_summary` now also exposes lightweight:
  - `product_selection`
- This lets purchase-facing pages consume stable original-product provenance without drilling into detail first.

### `GET /v1/tasks/{id}` and `GET /v1/tasks/{id}/detail`

- Read/detail endpoints keep full:
  - `product_selection`
- Full provenance still includes:
  - `matched_mapping_rule`

### OpenAPI

- Version updated from `0.21.0` to `0.22.0`.
- Added explicit schema layering:
  - `TaskProductSelectionSummary`
  - `TaskProductSelectionContext`
- Clarified that summary views are for stable frontend consumption, while full provenance remains detail/read-only.
- Clarified that all of this remains local ERP positioning provenance, not real ERP validation.

## 6. Design Decisions

- Reused one stable field name, `product_selection`, across all read models instead of inventing page-specific aliases.
- Split read models by weight rather than by naming:
  - summary for list / board / procurement summary
  - full provenance for read / detail
- Kept `matched_mapping_rule` out of list/board/procurement summary so lightweight views stay lightweight.
- Let backend assemble provenance from persisted task-detail fields so frontend no longer has to reconstruct the original-product path manually.

## 7. Correction Notes

- Prior repository-truth docs mainly described `product_selection` as a task write contract plus task read/detail traceability.
- This iteration reconciled code, OpenAPI, and handover docs so list, board, and procurement summary exposure are now part of the documented ready-for-frontend contract.

## 8. Verification

- Ran `gofmt -w` on all touched Go files.
- Added/updated tests covering:
  - task-list `product_selection` summary hydration
  - procurement-summary `product_selection` exposure
  - task-board propagation of task-item `product_selection`
- Ran:
  - `go test ./service/...`
  - `go test ./...`

## 9. Risks / Known Gaps

- `product_selection` still reflects persisted local mapped-search provenance supplied or confirmed by task-side flows; it does not validate against a real ERP API.
- Board/list pages now have stable provenance summaries, but there are still no dedicated filters for `source_search_entry_code` or `source_match_type`.
- Secondary/tertiary refinement remains reserved-field-based only; this iteration does not introduce a full search tree or search engine.

## 10. Suggested Next Step

- Keep real ERP integration deferred.
- The next reasonable phase is to decide whether frontend-facing list/board filters or badges should be added around `product_selection` summary fields while still avoiding full secondary/tertiary search expansion.
