# PHASE_AUTO_053

## Why This Phase Now
- The mainline had already regained auth/actor basics in Step 52, but original-product selection still depended on local cached search only.
- ERP Bridge query APIs were already available upstream, and keyword search had become the practical entry because bridge category coverage was known incomplete.
- The next bounded mainline gain was therefore to consume ERP Bridge queries in the main project without expanding into ERP writeback or broader integration work.

## Current Context
- Current CURRENT_STATE before this phase: Step 52 complete
- Current OpenAPI version before this phase: `0.48.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_052.md`
- Mainline focus: original-product picker / task create / business-info rebind

## Goals
- Add configurable ERP Bridge client access through `ERP_BRIDGE_BASE_URL`.
- Expose frontend-ready ERP Bridge query APIs for products/detail/categories.
- Make keyword ERP product search the primary original-product entry.
- Allow selected ERP Bridge products to flow into task create / business-info via additive `product_selection`.
- Keep bridge categories auxiliary only and non-blocking.

## Allowed Scope
- Main-project config, client, service, handler, router, DTO, and task-selection persistence changes
- Additive task-side selection snapshot persistence
- Focused tests for bridge client normalization and task mainline binding
- OpenAPI/state/handover/iteration synchronization

## Forbidden Scope
- ERP Bridge service changes
- Real ERP writeback or SKU writeback
- WMS / procurement / inbound integration docking
- Upload / NAS / object storage work
- Finance / BI / report deepening
- Frontend large-scale redesign
- Category-tree completion work

## Expected File Changes
- `config/config.go`
- `service/erp_bridge_client.go`
- `service/erp_bridge_service.go`
- `transport/handler/erp_bridge.go`
- `transport/http.go`
- `service/task_service.go`
- `service/task_product_selection.go`
- `domain/erp_bridge.go`
- `domain/task_product_selection.go`
- `repo/interfaces.go`
- `repo/mysql/product.go`
- `repo/mysql/task.go`
- `db/migrations/026_v7_erp_bridge_selection_snapshot.sql`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/iterations/ITERATION_053.md`
- `docs/phases/PHASE_AUTO_053.md`

## Required API / DB Changes
- API:
  - add `/v1/erp/products`
  - add `/v1/erp/products/{id}`
  - add `/v1/erp/categories`
  - extend task `product_selection` with additive `erp_product` snapshot fields
- DB:
  - add `task_details.product_selection_snapshot_json`

## Success Criteria
- Main project can reach ERP Bridge through configuration rather than hardcoded business URLs.
- Original-product mainline can search ERP products, fetch product detail, and read categories through the main project.
- Task create and business-info rebind can accept a selected ERP Bridge product and keep it traceable inside `product_selection`.
- Bridge category incompleteness does not block keyword search flows.
- OpenAPI and handover/state docs clearly describe keyword search as primary and categories as auxiliary.
