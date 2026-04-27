# PHASE_AUTO_023 - Mapped Product Search Integration

## Why This Phase Now
- Step 22 already made `search_entry_code`, `is_search_entry`, and `category_erp_mappings` explicit repository-truth contracts.
- The main remaining PRD gap is no longer category modeling; it is the missing execution layer from category selection to local ERP product positioning.
- Current repository truth already identifies the biggest missing loop as:
  - `products/search` still does not consume `search_entry_code + category_erp_mappings`
  - "total category code = first-level search entry" is modeled but not yet executable in local product search
  - real ERP API lookup is intentionally still deferred

## Current Context
- `CURRENT_STATE.md` before this round reports Step 22 complete and OpenAPI `v0.19.0`.
- Stable contracts that must not regress:
  - category center with `search_entry_code` and `is_search_entry`
  - `category_erp_mappings` skeleton with reserved secondary/tertiary fields
  - task/business-info category selection
  - cost prefill and procurement-summary exposure
- Current product-search gap:
  - `GET /v1/products/search` still only uses `keyword` plus legacy `category LIKE`
  - original-product selection still has no explicit first-level entry narrowing over local ERP data

## Goals
- Make local product search consume category-center search-entry semantics and `category_erp_mappings`.
- Support `search_entry_code`, `category_id`, `category_code`, and mapping-driven local ERP positioning on `GET /v1/products/search`.
- Keep primary mapping as the main supported path, while allowing only lightweight reserved secondary/tertiary condition consumption.
- Return explicit matched-positioning metadata so frontend and later联调 can see which mapping rule located each product.
- Keep this phase strictly local-data-based; no real ERP API lookup, sync expansion, or full search-engine work.

## Allowed Scope
- `cmd/`
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
- Real ERP API integration or live ERP lookup
- ERP sync enhancement beyond current local stub/file data
- NAS / upload implementation
- Strict `whole_hash` verification
- Full second/third-level category tree implementation
- Full search engine, index redesign, or materialized search projection
- Real auth / RBAC enforcement
- Finance / BI / export-center real modules

## Expected File Changes
- Extend product search query/response contracts in handler/service/repo/OpenAPI.
- Add local mapping-consumption logic so product search can narrow against local ERP products via active category mappings.
- Add explicit matched-result metadata for product search responses.
- Add or update service tests covering:
  - first-level search-entry fallback from category to mapping
  - primary/all mapping consumption behavior
  - reserved secondary/tertiary lightweight filtering boundaries
- Sync state, iteration, OpenAPI, handover, and V7 spec text to Step 23.

## Required API / DB Changes
- API:
  - extend `GET /v1/products/search` query support with:
    - `category_id`
    - `category_code`
    - `search_entry_code`
    - `mapping_match`
    - optional lightweight `secondary_*` / `tertiary_*` reserved condition filters
  - extend product-search response items with:
    - `matched_category_code`
    - `matched_search_entry_code`
    - `matched_mapping_rule`
- DB / migration:
  - none in this round

## Success Criteria
- Local product search can resolve category selection into a first-level `search_entry_code`.
- Product search can consume active local `category_erp_mappings` without calling real ERP APIs.
- Primary mappings are the default supported path; broader active-mapping search is optional and explicit.
- Reserved secondary/tertiary mapping fields are only lightly consumed; no full hierarchy/search-tree claim is made.
- Search results expose matched positioning metadata for observability and frontend联调.
- `go test ./...` passes.
- `CURRENT_STATE.md`, `docs/iterations/ITERATION_023.md`, and `docs/api/openapi.yaml` are synchronized.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_023.md`
- `docs/api/openapi.yaml`

Expected in this round:
- `MODEL_HANDOVER.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Product search must stay clearly labeled as a local ERP positioning layer, not real ERP lookup.
- Category-specific mappings versus search-entry-wide mappings need a clear fallback rule to avoid future ambiguity.
- Secondary/tertiary reserved fields must remain obviously partial support, not a claimed full multi-level taxonomy.

## Completion Output Format
1. Phase path
2. Changed files
3. DB / migration changes
4. API / OpenAPI changes
5. Auto-correction notes
6. Verification
7. Risks / remaining gaps
8. Next recommended phase
