# ITERATION_020 - Category Center Skeleton / Cost Rule Skeleton

**Date**: 2026-03-09  
**Scope**: STEP_20

## 1. Goals

- Add a configurable category-center skeleton that treats current business total-category codes and names as valid first-level category entries.
- Add a configurable cost-rule-center skeleton that models sample pricing experience as rules instead of service hardcoding.
- Add minimal task-side category linkage and cost-rule provenance storage so category/cost data stop drifting into free-text remarks.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 19 complete and OpenAPI `v0.16.0`.
- The repo already had stable task mainline, procurement, warehouse, task-board, ownership hint, and workbench preference contracts.
- This round explicitly stayed out of scope for:
  - real ERP integration
  - real auth / RBAC enforcement
  - NAS / upload work
  - full finance / BI / export-center implementation
  - a full formula-expression engine

## 3. Files Changed

### Code

- `cmd/server/main.go`
- `config/category_seed.json`
- `config/cost_rule_seed.json`
- `db/migrations/010_v7_category_cost_rule_skeleton.sql`
- `domain/category.go`
- `domain/cost_rule.go`
- `domain/task.go`
- `repo/interfaces.go`
- `repo/mysql/category.go`
- `repo/mysql/cost_rule.go`
- `repo/mysql/task.go`
- `service/category_service.go`
- `service/cost_rule_service.go`
- `service/cost_rule_service_test.go`
- `service/task_service.go`
- `transport/http.go`
- `transport/handler/category.go`
- `transport/handler/category_filters.go`
- `transport/handler/cost_rule.go`
- `transport/handler/task.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_020.md`
- `docs/phases/PHASE_AUTO_020.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- Added `db/migrations/010_v7_category_cost_rule_skeleton.sql`.
- New tables:
  - `categories`
  - `cost_rules`
- Extended `task_details` with:
  - `category_id`
  - `category_code`
  - `category_name`
  - `cost_rule_id`
  - `cost_rule_name`
  - `cost_rule_source`
- Seeded skeleton sample category and cost-rule data from the accepted business facts.

## 5. API Changes

### Category center

- Added:
  - `GET /v1/categories`
  - `GET /v1/categories/search`
  - `GET /v1/categories/{id}`
  - `POST /v1/categories`
  - `PATCH /v1/categories/{id}`
- The category center is documented as a configurable skeleton, not a full hierarchy system.
- Coded-style values such as `HBJ/HBZ/HCP/HLZ/HPJ/HQT/HSC/HZS` are explicitly treated as valid first-level categories.

### Cost-rule center

- Added:
  - `GET /v1/cost-rules`
  - `GET /v1/cost-rules/{id}`
  - `POST /v1/cost-rules`
  - `PATCH /v1/cost-rules/{id}`
  - `POST /v1/cost-rules/preview`
- `manual_quote` is now a first-class rule type for categories or cases that are not machine-calculable yet.
- Preview currently estimates:
  - fixed unit price
  - area-threshold surcharge
  - minimum billable area
  - special-process surcharge
  - narrow `print_side:*` size formula
- Preview falls back to `requires_manual_review=true` for:
  - `manual_quote`
  - unsupported `size_based_formula` cases

### Task business-info contract

- `PATCH /v1/tasks/{id}/business-info` now also supports:
  - `category_id`
  - `category_code`
  - `cost_rule_id`
  - `cost_rule_name`
  - `cost_rule_source`
- Purchase-side pricing stays under procurement (`procurement_price`).
- Internal cost-side pricing and provenance stay under business-info (`cost_price`, `cost_rule_*`).

### OpenAPI

- Version updated from `0.16.0` to `0.17.0`.
- Marked category center and cost-rule center as configurable skeletons.
- Marked `POST /v1/cost-rules/preview` as frontend-ready but skeleton-limited.

## 6. Design Decisions

- Kept category-center hierarchy intentionally lightweight through `parent_id` and `level` instead of forcing a full tree-management system in this phase.
- Treated coded-style total-category values as legitimate first-level entries because the business clarified they are the actual operating category entrances.
- Kept cost rules configuration-driven and avoided hardcoding Excel-like notes into service branches.
- Added task-side category/rule provenance fields without moving procurement ownership back into `task_details`.
- Used JSON sample artifacts plus migration seeds so the skeleton is both documented and immediately initializable.

## 7. Correction Notes

- Repository-truth docs correctly reflected Step 19 before implementation; no rollback/reconciliation round was needed first.
- Task-service constructor compatibility was preserved by adding `NewTaskServiceWithCatalog(...)` while keeping the old constructor available for existing tests and call sites.

## 8. Verification

- Ran `gofmt -w` on all touched Go files.
- Added service tests covering:
  - coded-style category creation
  - fixed/threshold/process preview behavior
  - `manual_quote` preview fallback
- Ran:
  - `go test ./service/... ./transport/... ./repo/mysql/...`
  - `go test ./...`

## 9. Risks / Known Gaps

- Cost-rule preview is intentionally not a full pricing engine; some `size_based_formula` cases still require manual review.
- Category center is ready for first-level total categories, but ERP mapping and second/third-level search expansion remain future work.
- Seed samples reflect accepted business facts for skeleton initialization, not a claim that the full production taxonomy or pricing corpus is already complete.

## 10. Suggested Next Step

- Keep real ERP integration and general formula-engine work deferred.
- The next reasonable phase is either:
  - category-center to ERP product mapping skeleton
  - task-side autofill/quote assistance that consumes the new category and cost-rule centers without changing their boundaries
