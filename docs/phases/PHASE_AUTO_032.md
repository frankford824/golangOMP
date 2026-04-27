# PHASE_AUTO_032 - Export Execution-Attempt / Runner-Adapter Visibility

## Why This Phase Now
- Step 31 already established an explicit placeholder `POST /v1/export-jobs/{id}/start` boundary for `queued -> running`.
- The next narrow gap is not real async execution, but the lack of one explicit model for "this specific start attempt" and the placeholder adapter layer behind it.
- Before any future scheduler, runner, storage, or delivery platform is introduced, export jobs need a separate execution-attempt record so job lifecycle and single-attempt execution are no longer conflated.

## Current Context
- `CURRENT_STATE.md` before this phase reports Step 31 complete.
- OpenAPI version before this phase is `0.28.0`.
- Export center already has:
  - export job persistence
  - lifecycle status progression
  - structured placeholder `result_ref`
  - lifecycle audit trace
  - placeholder claim/read boundary
  - placeholder refresh boundary
  - explicit placeholder start boundary
  - runner-related audit events
- Current main gaps:
  - no durable execution-attempt record per export job start
  - no explicit placeholder runner-adapter identity beyond event wording
  - no stable read model for `latest_attempt` / `attempt_count`
  - no dedicated inspection endpoint for attempt history

## Goals
- Add a minimal export execution-attempt persistence layer for export jobs.
- Make the placeholder runner-adapter boundary explicit and reusable without pretending a real async platform exists.
- Separate export job lifecycle state from attempt lifecycle state.
- Keep current lifecycle, audit trace, `result_ref`, claim/read, refresh, and `start` contracts backward compatible.
- Expose enough read-model context for frontend-ready job views and internal/admin attempt inspection.

## Allowed Scope
- `db/migrations/`
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
- Add one export-job attempt persistence table and MySQL repo implementation.
- Extend export-job read models with:
  - `attempt_count`
  - `latest_attempt`
  - `can_retry`
- Add one internal/admin attempt inspection endpoint:
  - `GET /v1/export-jobs/{id}/attempts`
- Make `POST /v1/export-jobs/{id}/start` create a new attempt record.
- Make running-attempt completion/failure/cancel flows update attempt status without regressing current job lifecycle behavior.
- Extend export-job event payloads and runner-event coverage so attempt context remains visible in the shared audit timeline.
- Sync OpenAPI, state, iteration, handover, and V7 appendix documents.

## Required API / DB Changes
- API:
  - keep `POST /v1/export-jobs/{id}/start` as the explicit placeholder runner-initiation boundary
  - add `GET /v1/export-jobs/{id}/attempts` for internal/admin attempt inspection
  - extend `GET /v1/export-jobs` and `GET /v1/export-jobs/{id}` with `attempt_count`, `latest_attempt`, and `can_retry`
- DB:
  - add `export_job_attempts`
  - store at least:
    - `attempt_id`
    - `export_job_id`
    - `attempt_no`
    - `trigger_source`
    - `execution_mode`
    - `status`
    - `started_at`
    - `finished_at`
    - `error_message`
    - `adapter_note`
    - `created_at`
- Placeholder adapter boundary:
  - make the current adapter identity explicit
  - do not introduce worker leases, queue claims, scheduler callbacks, or storage handles

## Success Criteria
- Every successful export-job start creates one new attempt record.
- Multiple start cycles for the same export job are represented as multiple attempts after requeue.
- Export job lifecycle and attempt lifecycle are separately readable.
- Running attempts are finalized when the export job becomes `ready`, `failed`, or `cancelled`.
- Export job detail/list responses expose `attempt_count` and `latest_attempt` without breaking existing fields.
- Attempt visibility does not replace the existing export-job event timeline; it complements it.
- OpenAPI clearly states that the adapter/attempt layer is placeholder-only and not a real async platform.
- `go test ./...` passes, or any environment-limited failures are explicitly recorded.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_032.md`
- `docs/api/openapi.yaml`

Expected in this round:
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- If attempt status and job status drift apart, the new layer will create confusion instead of clarity.
- If attempt records are treated like real runner telemetry, downstream readers may over-interpret placeholder behavior.
- If `can_retry` semantics are vague, frontend/internal tooling may assume retry is directly executable from terminal job states when requeue is still required.

## Completion Output Format
1. Phase path
2. Changed files
3. DB / migration changes
4. API / OpenAPI changes
5. Auto-correction notes
6. Verification
7. Risks / remaining gaps
8. Next recommended phase
