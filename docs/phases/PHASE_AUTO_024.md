# PHASE_AUTO_024 - Task Creation Product Picker Integration

## Why This Phase Now
- Step 23 already made mapped local ERP product search executable through `search_entry_code + category_erp_mappings`.
- The largest remaining PRD gap is no longer search capability itself; it is the missing handoff from mapped product search into original-product task creation and later task-side traceability.
- Repository truth already identifies the next reasonable phase as wiring mapped local product search into original-product selection while keeping real ERP lookup deferred.

## Current Context
- `CURRENT_STATE.md` before this round reports Step 23 complete and OpenAPI `v0.20.0`.
- Stable contracts that must not regress:
  - category center with `search_entry_code` and `is_search_entry`
  - `category_erp_mappings` as the independent local ERP positioning layer
  - `GET /v1/products/search` mapped-search filters and matched-result metadata
  - task/business-info category and cost-prefill contracts
- Current original-product gap:
  - `POST /v1/tasks` still only binds `product_id/sku_code/product_name_snapshot`
  - `PATCH /v1/tasks/{id}/business-info` cannot fully persist original-product picker provenance
  - task read/detail contracts cannot fully answer which category/search entry/mapping located the chosen existing product

## Goals
- Let original-product task creation formally consume mapped local product-search results as a backend contract.
- Add one explicit product-picker context contract so task creation and business-info can persist how an existing product was located.
- Make original-product selection traceable through category/search-entry/mapping provenance without introducing real ERP APIs or a full search tree.

## Allowed Scope
- `db/migrations/`
- `domain/`
- `repo/`
- `service/`
- `transport/`
- `cmd/`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/`
- `docs/phases/`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real ERP API integration or live ERP lookup
- ERP sync enhancement beyond current local stub/file data
- NAS / upload implementation
- Strict `whole_hash` verification
- Full second/third-level category tree implementation
- Full search engine, indexing redesign, or materialized product search projection
- Real auth / RBAC enforcement
- Finance / BI / export-center real modules

## Expected File Changes
- Add one additive task-side product-picker provenance persistence migration.
- Extend task create and task business-info request/response contracts with an explicit original-product picker context object.
- Persist mapped-search provenance onto task-side detail records while keeping task root product binding stable.
- Expose product-picker provenance on task read/detail responses.
- Add or update tests covering:
  - original-product task creation with mapped-search picker context
  - business-info rebinding/enrichment for existing-product tasks
  - legacy existing-product creation fallback without mapped provenance

## Required API / DB Changes
- API:
  - extend `POST /v1/tasks` with an additive `product_selection` object for original-product picker integration
  - extend `PATCH /v1/tasks/{id}/business-info` with the same additive `product_selection` object
  - expose persisted picker provenance on task read/detail contracts
- DB / migration:
  - add task-detail persistence for existing-product source tracing:
    - selected source product identity
    - source search-entry code
    - source match type / rule
    - matched category / search-entry
    - matched mapping-rule snapshot

## Success Criteria
- Original-product task creation can consume mapped local product-search context without breaking the legacy create contract.
- Existing-product task business-info can persist or refine selected-product provenance.
- Task read/detail responses can explain which category/search entry/mapping located the selected existing product.
- Mapped product search remains clearly documented as a local ERP positioning layer, not a real ERP lookup API.
- `go test ./...` passes.
- `CURRENT_STATE.md`, `docs/iterations/ITERATION_024.md`, and `docs/api/openapi.yaml` are synchronized.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_024.md`
- `docs/api/openapi.yaml`

Expected in this round:
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Task-side picker provenance must not be mistaken for real ERP lookup capability.
- Additive task create support must preserve current clients that still send only legacy `product_id/sku_code/product_name_snapshot`.
- Selection traceability should stay explicit enough for later ERP validation without overcommitting to a finished multi-level search taxonomy.

## Completion Output Format
1. Phase path
2. Changed files
3. DB / migration changes
4. API / OpenAPI changes
5. Auto-correction notes
6. Verification
7. Risks / remaining gaps
8. Next recommended phase
