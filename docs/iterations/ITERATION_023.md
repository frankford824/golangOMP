# ITERATION_023 - Mapped Product Search Integration

**Date**: 2026-03-10  
**Scope**: PHASE_AUTO_023

## 1. Goals

- Let local product search formally consume `search_entry_code`, `category_id/category_code`, and active `category_erp_mappings`.
- Turn "total category code = first-level search entry" into an executable local ERP product-positioning path.
- Keep real ERP API lookup deferred while making result provenance explicit for frontend联调.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 22 complete and OpenAPI `v0.19.0`.
- Step 22 already made category center and `category_erp_mappings` explicit repository-truth contracts.
- The largest remaining gap was that `GET /v1/products/search` still only used legacy `keyword + category LIKE` filtering and did not consume the new mapping layer.
- This round stayed out of scope for:
  - real ERP API lookup
  - sync enhancement
  - full second/third-level category tree implementation
  - full search-engine behavior
  - NAS / upload / strict `whole_hash`

## 3. Files Changed

### Code

- `cmd/server/main.go`
- `domain/product_search.go`
- `repo/interfaces.go`
- `repo/mysql/category_mapping.go`
- `repo/mysql/product.go`
- `service/category_mapping_service_test.go`
- `service/product_service.go`
- `service/product_service_test.go`
- `transport/handler/product.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_023.md`
- `docs/phases/PHASE_AUTO_023.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- None.
- This round deliberately stayed inside the existing local ERP product table plus Step 22 category/mapping schema.

## 5. API Changes

### `GET /v1/products/search`

- Kept legacy query compatibility:
  - `keyword`
  - `category`
- Added mapped local ERP positioning query support:
  - `category_id`
  - `category_code`
  - `search_entry_code`
  - `mapping_match`
  - `secondary_key`
  - `secondary_value`
  - `tertiary_key`
  - `tertiary_value`
- Clarified contract semantics:
  - this is a local ERP positioning layer over already-synced `products`
  - it consumes local `category_erp_mappings`
  - it does not call real ERP APIs

### Product-search result provenance

- Search results now expose:
  - `matched_category_code`
  - `matched_search_entry_code`
  - `matched_mapping_rule`
- `matched_mapping_rule` now makes the applied mapping visible through:
  - `mapping_id`
  - `category_code`
  - `search_entry_code`
  - `erp_match_type`
  - `erp_match_value`
  - reserved `secondary_*` / `tertiary_*` fields
  - `is_primary`
  - `priority`

### OpenAPI

- Version updated from `0.19.0` to `0.20.0`.
- Clarified that mapped product search is a non-real-time local ERP positioning layer, not a real ERP lookup API.

## 6. Design Decisions

- Kept mapped product search on top of the existing local `products` table instead of introducing any new sync or lookup mechanism.
- Resolved category-driven search through `category -> search_entry_code -> active mappings -> local products`, which makes the first-level search-entry model executable without pretending deeper ERP taxonomy is finished.
- Used `mapping_match=primary|all` instead of adding a full rule-selection system:
  - `primary` is the default when mapped search is used
  - `all` allows broader active mapping consumption
- Chose a clear fallback boundary:
  - prefer exact category mappings when they exist
  - otherwise fall back to search-entry-wide mappings
- Limited secondary/tertiary support to lightweight reserved filtering only; no full multi-level tree or search engine is claimed.

## 7. Correction Notes

- `CURRENT_STATE.md` and `docs/api/openapi.yaml` correctly reflected Step 22 before this round, but still described `GET /v1/products/search` as a legacy `category LIKE` path only.
- This iteration reconciled repository-truth docs with the new mapped-search execution layer and result provenance contract.

## 8. Verification

- Ran `gofmt -w` on all touched Go files.
- Added service tests covering:
  - child category fallback to first-level search-entry mappings
  - exact category mapping preference under `mapping_match=all`
  - validation for mismatched `search_entry_code`
- Ran:
  - `go test ./service/...`
  - `go test ./...`

## 9. Risks / Known Gaps

- Product search still relies on straightforward SQL predicates over the local `products` table; this is not a full search engine.
- Secondary/tertiary support is still only lightweight reserved-field consumption.
- Original-product task creation / selector flows still need to call this mapped local search path directly.
- Real ERP lookup behavior still requires future validation against live ERP data and APIs.

## 10. Suggested Next Step

- Keep real ERP integration deferred.
- The next reasonable phase is to connect original-product task creation / product selection flows directly to the mapped local product-search contract, so category-driven ERP positioning is used end-to-end in task entry.
