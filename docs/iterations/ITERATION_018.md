# ITERATION_018 - Candidate Scan Optimization Assessment / Indexing-or-Projection Prework

**Date**: 2026-03-09  
**Scope**: STEP_18

## 1. Goals

- Assess the remaining read-model cost behind `GET /v1/tasks`, `GET /v1/task-board/summary`, and `GET /v1/task-board/queues`.
- Distinguish which hotspots merit immediate light optimization, which should wait for future index/projection work, and which are acceptable derived-cost debt for now.
- Keep all existing board/list/filter/query-template contracts stable.

## 2. Input Context

- `CURRENT_STATE.md` before this iteration reported Step 17 complete and OpenAPI `v0.15.3`.
- Step 17 had already narrowed task-board candidate scans onto a dedicated repo/read-model path, so the main remaining gap was no longer preset fan-out but candidate-scan predicate cost.
- This round remained explicitly out of scope for:
  - real auth / RBAC enforcement
  - real ERP / NAS / upload work
  - strict `whole_hash` verification
  - queue ownership persistence
  - saved preferences or personal inbox persistence

## 3. Files Changed

### Code

- `repo/mysql/task.go`
- `repo/mysql/task_test.go`

### Documents

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/api/openapi.yaml`
- `docs/iterations/ITERATION_018.md`
- `docs/phases/PHASE_AUTO_018.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## 4. DB / Migration Changes

- No new table.
- No migration added.
- No index rollout added in this round.

## 5. API Changes

### External contract

- No public route rename.
- No public query-field rename for:
  - `GET /v1/tasks`
  - `GET /v1/task-board/summary`
  - `GET /v1/task-board/queues`
- No task-board queue payload change:
  - `queue_key`
  - `filters`
  - `normalized_filters`
  - `query_template`
  - `count`
  - sample-task and queue-task payloads

### OpenAPI

- Version updated from `0.15.3` to `0.15.4`.
- Added Step 18 implementation notes describing the optimization-assessment outcome and the small internal latest-asset projection improvement.

## 6. Assessment Outcome

### Highest remaining candidate-scan predicate cost

- `sub_status_code` without `sub_status_scope`
  - broadest scan-side predicate because it expands into OR matching across multiple derived sub-status lanes
- `warehouse_blocking_reason_code`
  - expensive because it expands into multiple derived business conditions, including final-asset checks for design tasks
- `warehouse_prepare_ready`
  - effectively the negation of the combined warehouse-blocking conditions, so it inherits most of that derived cost

### Medium candidate-scan predicate cost

- `coordination_status`
  - one derived CASE over task type, procurement state, warehouse state, and task status
- `main_status`
  - one derived CASE over task status, filed state, and warehouse state

### Lower candidate-scan predicate cost

- `warehouse_receive_ready`
  - small CASE expression over `task_status` plus warehouse-receipt existence

### Projection-heavy but not current scan driver

- `closable` / `cannot_close_reasons`
  - still meaningful per-row workflow projection cost on list/read-model hydration
  - currently not a board-candidate-scan pushdown predicate, so not the first place to spend index/materialization effort

## 7. Small Optimization Applied

- Replaced repeated latest-asset scalar subquery usage with one joined latest-asset projection in `repo/mysql/task.go`.
- The join now derives one latest asset row per task from `task_assets` using the existing `(task_id, version_no)` uniqueness contract and reuses that result for:
  - `latest_asset_type`
  - design-lane `sub_status`
  - `warehouse_blocking_reason_code=missing_final_asset`
  - `warehouse_prepare_ready`

## 8. Optimization Classification

### Worth doing now

- Deduplicate latest-asset computation across list and board candidate scans.
  - Implemented in this round.

### Future candidates after data-volume validation

- Composite indexes supporting stable task list / board ordering and common narrowing patterns, most likely around:
  - `tasks.updated_at`
  - `tasks.task_status`
  - `tasks.task_type`
- Projection or materialization for broad derived workflow predicates if:
  - `sub_status_code` without scope becomes common
  - `warehouse_blocking_reason_code` and `warehouse_prepare_ready` dominate board traffic
- A dedicated read projection if queue ownership/preferences are revisited later and real per-user inbox behavior becomes necessary.

### Acceptable technical debt for now

- `main_status` and `coordination_status` remaining as derived CASE expressions.
- `closable` / `cannot_close_reasons` remaining per-row derived workflow projection.
- `keyword` using straightforward `LIKE` matching.

## 9. Design Decisions

- Prefer explicit hotspot classification before building indexes or projections.
- Limit this round's implementation to one internal, contract-safe optimization instead of broad schema work.
- Keep board/list convergence semantics unchanged so future optimization decisions stay grounded in a stable API contract.

## 10. Correction Notes

- No code/OpenAPI public-contract drift was found before implementation.
- Repository-truth docs were advanced from Step 17 / OpenAPI `v0.15.3` to Step 18 / OpenAPI `v0.15.4` after the assessment and internal query optimization were completed.

## 11. Verification

- Added repo-level tests proving latest-asset projection now uses a joined alias instead of repeated scalar subqueries.
- Ran:
  - `gofmt -w repo/mysql/task.go repo/mysql/task_test.go`
  - `go test ./repo/mysql/...`
  - `go test ./service/...`
  - `go test ./...`

## 12. Risks / Known Gaps

- This round classifies likely index/projection targets, but exact future choices still need scale validation.
- `sub_status_code` without scope and warehouse-readiness/blocking predicates remain the most likely pressure points if data volume grows.
- Real auth, ERP, NAS/upload, ownership persistence, saved preferences, and strict `whole_hash` validation remain deliberately out of scope.

## 13. Suggested Next Step

- Do not start broad index or materialized-view work by default.
- Revisit targeted index/projection prework only when:
  - data volume or measured latency shows pressure on unscoped `sub_status_code`, `warehouse_blocking_reason_code`, or `warehouse_prepare_ready`
  - or a future queue-ownership/preferences phase proves that repeated candidate scans need a narrower persisted read substrate
