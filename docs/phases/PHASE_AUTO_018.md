# PHASE_AUTO_018 - Candidate Scan Optimization Assessment / Indexing-or-Projection Prework

## Why This Phase Now
- Step 17 already removed the broadest board candidate-scan waste by moving task-board collection onto a dedicated repo/read-model path.
- The remaining uncertainty is no longer preset-by-preset fan-out; it is the cost of the candidate scan itself, especially where derived SQL and wide read-model projection are still mixed together.
- Ownership persistence, saved preferences, or personal inbox storage would still be built on an insufficiently assessed scan/aggregation substrate if introduced now.

## Current Context
- `CURRENT_STATE.md` before this round reports Step 17 complete and OpenAPI `v0.15.3`.
- `GET /v1/tasks`, `GET /v1/task-board/summary`, and `GET /v1/task-board/queues` must keep their public board/list/filter/query-template contracts stable.
- Main gaps before implementation:
  - candidate scans still rely on derived SQL for projected workflow, procurement coordination, and warehouse-readiness semantics
  - it is not yet clearly classified which predicates justify immediate light optimization, which need future index/projection work, and which are acceptable debt for now
  - there is still a risk of overbuilding indexes or materialized views before the real hotspots are distinguished

## Goals
- Assess the current read-model cost shape behind:
  - `GET /v1/tasks`
  - `GET /v1/task-board/summary`
  - `GET /v1/task-board/queues`
- Explicitly classify:
  - light optimizations worth doing now
  - index/projection/materialization candidates that should wait for later scale validation
  - derived SQL / aggregation costs that are acceptable for now
- Land only a very small, contract-safe optimization if there is a clear hotspot with low implementation risk.

## Allowed Scope
- `repo/`
- `service/`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `docs/api/openapi.yaml`
- `docs/iterations/`
- `docs/phases/`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real authentication or permission enforcement
- Queue ownership persistence or personal inbox storage
- Saved workbench preferences
- Real ERP integration
- NAS / real upload integration
- Strict `whole_hash` verification
- Large index engineering
- Broad materialized-view / projection redesign
- BI / KPI / finance / export-center real modules

## Expected File Changes
- Document the candidate-scan hotspot assessment and optimization classification backlog.
- Keep `/v1/tasks` and task-board public contracts unchanged while tightening one low-risk internal read-model hotspot if justified.
- Add tests that lock the small optimization boundary.
- Sync state, phase, iteration, OpenAPI, handover, and V7 appendix/spec docs to Step 18.

## Required API / DB Changes
- API:
  - no path change
  - no query-field rename
  - no queue payload rename
  - OpenAPI patch-version update plus implementation-note-only documentation
- DB / migration:
  - no new table
  - no new migration in this round
  - no broad index rollout in this round

## Success Criteria
- The read-model cost behind task list and board candidate scans is explicitly classified.
- The heaviest remaining derived predicate paths are identified, especially for:
  - `main_status`
  - `sub_status_code`
  - `coordination_status`
  - `warehouse_prepare_ready`
  - `warehouse_receive_ready`
  - `warehouse_blocking_reason_code`
  - `closable` / `cannot_close_reasons`
- A very small internal optimization lands only if it is clearly beneficial and contract-safe.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_018.md`
- `docs/api/openapi.yaml`

Optional but expected for this round:
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Derived SQL cost is now better classified, but that does not by itself prove the exact future index or projection shape.
- A light optimization must not be mistaken for a signal that large-scale indexing or materialization is already justified.
- `closable` / `cannot_close_reasons` still remain per-row workflow projection cost even after candidate-scan improvements.

## Completion Output Format
1. Changed Files
2. DB / Migration Changes
3. API / OpenAPI Changes
4. Auto-Correction Notes
5. Verification
6. Risks / Remaining Gaps
7. Next Step
