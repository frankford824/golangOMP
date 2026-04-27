# ITERATION_022 - ERP Category Mapping Skeleton

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_022

## 1. Goals

- Push category center from task-side selection into explicit ERP product-positioning semantics.
- Make “总分类编码 = 一级搜索入口” a real persisted category contract instead of a documentation-only rule.
- Add an independent `category_erp_mappings` skeleton with CRUD/search APIs and reserved secondary/tertiary refinement fields.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 21 complete and OpenAPI `v0.18.0`.
- Step 21 already made category and cost-rule skeletons active in task/business-info and procurement summaries.
- This round explicitly stayed out of scope for:
  - real ERP integration or live ERP lookup
  - sync enhancement beyond the existing stub ERP product source
  - NAS / upload work
  - strict `whole_hash` verification
  - full second/third-level category-tree implementation
  - a full search engine or formula engine

## 3. Files Changed

### Code

- `cmd/server/main.go`
- `config/category_erp_mapping_seed.json`
- `db/migrations/012_v7_category_erp_mapping_skeleton.sql`
- `domain/category.go`
- `domain/category_mapping.go`
- `repo/interfaces.go`
- `repo/mysql/category.go`
- `repo/mysql/category_mapping.go`
- `service/category_mapping_service.go`
- `service/category_mapping_service_test.go`
- `service/category_service.go`
- `service/cost_rule_service_test.go`
- `transport/handler/category.go`
- `transport/handler/category_filters.go`
- `transport/handler/category_mapping.go`
- `transport/http.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_022.md`
- `docs/phases/PHASE_AUTO_022.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- Added `db/migrations/012_v7_category_erp_mapping_skeleton.sql`.
- Extended `categories` with explicit first-level ERP search-entry semantics:
  - `search_entry_code`
  - `is_search_entry`
- Added `category_erp_mappings` with skeleton ERP-positioning fields:
  - `category_id`
  - `category_code`
  - `search_entry_code`
  - `erp_match_type`
  - `erp_match_value`
  - `secondary_condition_key`
  - `secondary_condition_value`
  - `tertiary_condition_key`
  - `tertiary_condition_value`
  - `is_primary`
  - `is_active`
  - `priority`
  - `source`
  - `remark`
- Seeded sample skeleton mappings and added `config/category_erp_mapping_seed.json` as repository sample reference.

## 5. API Changes

### Category center

- `Category` payloads now expose:
  - `search_entry_code`
  - `is_search_entry`
- `POST /v1/categories` and `PATCH /v1/categories/{id}` now accept the same fields.
- Service validation now makes the first-level-entry contract explicit:
  - top-level categories must keep `search_entry_code == category_code`
  - top-level categories must keep `is_search_entry=true`
  - child categories inherit the parent `search_entry_code`
  - child categories cannot be marked as first-level search entries

### Category-to-ERP mapping skeleton

- Added:
  - `GET /v1/category-mappings`
  - `GET /v1/category-mappings/search`
  - `GET /v1/category-mappings/{id}`
  - `POST /v1/category-mappings`
  - `PATCH /v1/category-mappings/{id}`
- Mapping schema now supports:
  - `category_id`
  - `category_code`
  - `search_entry_code`
  - `erp_match_type`
  - `erp_match_value`
  - `is_primary`
  - `is_active`
  - `priority`
  - `source`
  - `remark`
- Reserved fields for later second/third-level refinement are now explicit:
  - `secondary_condition_key`
  - `secondary_condition_value`
  - `tertiary_condition_key`
  - `tertiary_condition_value`

### OpenAPI

- Version updated from `0.18.0` to `0.19.0`.
- Clarified that category center now carries explicit first-level ERP search-entry semantics.
- Clarified that category-mapping APIs are positioning skeletons only and do not execute real ERP lookup yet.

## 6. Design Decisions

- Used `search_entry_code` plus `is_search_entry` on category records instead of relying only on `level=1`, because the PRD gap was semantic explicitness, not hierarchy depth.
- Kept `category_erp_mappings` independent from `products` so category-to-ERP positioning can evolve without mutating ERP product master data.
- Reserved secondary/tertiary condition fields directly on the mapping record instead of inventing a full second/third-level taxonomy this round.
- Kept task/business-info and cost-prefill contracts unchanged; future task-side ERP positioning is expected to resolve from `task_details.category_* -> categories.search_entry_code -> category_erp_mappings`.

## 7. Correction Notes

- `docs/V7_API_READY.md` and `docs/V7_MODEL_HANDOVER_APPENDIX.md` were materially stale before this round:
  - they still reported Step 19-era status and old OpenAPI versions
  - they omitted Step 20 and Step 21 category/cost/task-prefill work
- This iteration corrected those repository-truth documents while adding Step 22.

## 8. Verification

- Ran `gofmt -w` on all touched Go files.
- Added service tests covering:
  - top-level search-entry defaults
  - child-category search-entry inheritance
  - category-to-ERP mapping creation and validation
- Ran:
  - `go test ./service/...`
  - `go test ./...`

## 9. Risks / Known Gaps

- `category_erp_mappings` is still only a positioning skeleton; no real ERP API or product lookup execution is wired yet.
- Reserved secondary/tertiary fields are standard placeholders, not a finished second/third-level category system.
- Product search still uses the current `products.category` fuzzy filter; later work must decide how mapping skeletons actually constrain ERP product search/read paths.

## 10. Suggested Next Step

- Keep real ERP integration deferred.
- The next reasonable phase is to let ERP product search or original-product selection consume `search_entry_code + category_erp_mappings` as a non-real-time positioning layer, while still avoiding full synchronization or live ERP calls.
