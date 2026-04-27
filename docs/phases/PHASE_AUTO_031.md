# PHASE_AUTO_031 - Export Runner-Initiation Semantics

## Why This Phase Now
- Step 30 already delivered placeholder handoff expiry/refresh semantics over the export lifecycle.
- The narrowest remaining export-center gap is no longer handoff access, but the absence of a formal runner-initiation boundary.
- Before any future async runner, NAS, object storage, or delivery layer is introduced, `queued -> running` needs an explicit contract that separates initiation from later execution and handoff concerns.

## Current Context
- `CURRENT_STATE.md` before this phase reports Step 30 complete.
- OpenAPI version before this phase is `0.27.0`.
- Existing export-center APIs:
  - `GET /v1/export-templates`
  - `POST /v1/export-jobs`
  - `GET /v1/export-jobs`
  - `GET /v1/export-jobs/{id}`
  - `GET /v1/export-jobs/{id}/events`
  - `POST /v1/export-jobs/{id}/claim-download`
  - `GET /v1/export-jobs/{id}/download`
  - `POST /v1/export-jobs/{id}/refresh-download`
  - `POST /v1/export-jobs/{id}/advance` (internal/admin skeleton)
- Existing export-center capabilities:
  - export job persistence
  - lifecycle statuses
  - structured placeholder `result_ref`
  - lifecycle audit trace
  - placeholder claim/read boundary
  - enforced handoff expiry / refresh semantics
- Current main gaps:
  - no explicit runner-initiation interface
  - `queued -> running` is still mainly expressed through generic `advance` behavior
  - no stable read-model hint showing whether a queued job can still be started
  - no clear placeholder contract identifying which layer a future real runner should replace

## Goals
- Add an explicit placeholder runner-initiation boundary for export jobs.
- Formalize `queued -> running` as a start contract instead of leaving it implicit in generic lifecycle advance semantics.
- Add runner-oriented audit events such as `export_job.runner_initiated` and `export_job.started`.
- Expose lightweight read-model hints that help frontend or internal tooling understand start availability and latest runner activity.
- Keep current lifecycle, `result_ref`, claim/read, refresh, and audit-trace contracts backward compatible.

## Allowed Scope
- `domain/`
- `repo/`
- `service/`
- `transport/`
- `cmd/server/main.go`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `docs/iterations/`
- `docs/phases/`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Forbidden Scope
- Real async runner or scheduler platform
- Real file generation
- Real byte-stream download
- Signed URL delivery
- NAS or object-storage integration
- BI / KPI / finance export expansion
- Real permission trimming
- Real ERP or other external-system integration

## Expected File Changes
- Add one internal placeholder start/initiation endpoint for export jobs.
- Reuse the same service layer to keep `advance start` and the new start boundary aligned.
- Introduce explicit runner-initiation / started events without removing existing lifecycle trace semantics.
- Extend export-job list/detail contracts with lightweight start and runner-event hints.
- Add tests covering start boundary rules, duplicate-start rejection, event ordering, and backward-compatible `advance start` semantics.
- Sync state, iteration, OpenAPI, handover, and V7 spec documents.

## Required API / DB Changes
- API:
  - add `POST /v1/export-jobs/{id}/start` as an internal/admin placeholder runner-initiation boundary
  - keep `POST /v1/export-jobs/{id}/advance` for generic lifecycle actions and backward compatibility
  - clarify that start is allowed only when export job status is `queued`
  - clarify that `running|ready|failed|cancelled` cannot be started again
  - keep current claim/read/refresh contracts unchanged
- Read model:
  - add lightweight fields such as:
    - `can_start`
    - `start_mode`
    - `execution_mode`
    - `latest_runner_event`
- DB:
  - no new migration is expected in this round
  - reuse existing `export_jobs`
  - reuse existing `export_job_events`

## Success Criteria
- Export jobs have one explicit start/initiation boundary separate from generic later lifecycle changes.
- Only `queued` export jobs can be started.
- Start writes explicit audit events for initiation and start.
- `queued -> running` remains atomic with audit writes in the same transaction.
- `advance` remains backward compatible and does not regress current export lifecycle behavior.
- List/detail contracts expose enough state for tools to understand start affordance and latest runner-side placeholder activity.
- OpenAPI clearly marks the start boundary as internal placeholder initiation, not a real async runner platform.
- `go test ./...` passes, or any environment-limited failures are explicitly recorded.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_031.md`
- `docs/api/openapi.yaml`

Expected in this round:
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- If start semantics are duplicated inconsistently between `/start` and `/advance`, future runner replacement will still be unclear.
- If runner-initiation events are treated as real execution telemetry, downstream consumers may over-interpret placeholder behavior.
- If `can_start` is not derived purely from lifecycle state, tooling may misread queued jobs that are still valid to initiate.

## Completion Output Format
1. Phase path
2. Changed files
3. DB / migration changes
4. API / OpenAPI changes
5. Auto-correction notes
6. Verification
7. Risks / remaining gaps
8. Next recommended phase
