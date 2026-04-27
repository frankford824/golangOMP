# PHASE_AUTO_028 - Export Job Lifecycle Audit Trace

## Why This Phase Now
- Step 27 completed export-job persistence, lifecycle statuses, and placeholder download handoff, but export jobs still do not leave a dedicated lifecycle event trail.
- The next blocking gap is observability and maintainability, not real file delivery. Without a stable event chain, later runner, download handoff, or storage work will scatter lifecycle semantics across row fields and ad hoc logs.
- Repository truth already points to lifecycle audit trace as the narrowest next phase that strengthens export center without expanding into real file generation or async platform work.

## Current Context
- `CURRENT_STATE.md` reports export center skeleton plus lifecycle advancement through `POST /v1/export-jobs/{id}/advance`.
- OpenAPI version is `0.24.0`.
- Existing export-center APIs:
  - `GET /v1/export-templates`
  - `POST /v1/export-jobs`
  - `GET /v1/export-jobs`
  - `GET /v1/export-jobs/{id}`
  - `POST /v1/export-jobs/{id}/advance` (internal/admin skeleton)
- Current main gaps:
  - no dedicated audit trace for export-job lifecycle
  - no timeline query for state progression, failure reasons, or placeholder handoff changes
  - no stable event summary on job detail for quick troubleshooting

## Goals
- Add an export-job lifecycle audit-trace skeleton with durable event persistence.
- Record key lifecycle transitions and placeholder handoff mutations without changing existing export-job contracts.
- Expose one timeline query so frontend/internal troubleshooting can inspect export-job history without reading raw DB rows.
- Add small event summaries to export-job detail/list responses where useful, without embedding the full timeline into list pages.

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
- Real file generation
- Real download endpoint or signed URL delivery
- NAS, object storage, or upload/download-agent integration
- Full async scheduling / orchestration platform
- BI / KPI / finance export expansion
- Full auth / permission trimming
- Real ERP or other external-system integration

## Expected File Changes
- Add export-job event domain models and event type constants.
- Add one migration for export-job lifecycle event persistence.
- Extend export repo/service logic to append audit-trace events atomically with lifecycle mutations.
- Add timeline query endpoint and lightweight event summary fields on export jobs.
- Add or update tests covering event creation, lifecycle event sequencing, and timeline reads.
- Sync state, iteration, OpenAPI, handover, and V7 appendix/spec documents.

## Required API / DB Changes
- API:
  - keep existing export create/list/detail contracts stable
  - add `GET /v1/export-jobs/{id}/events`
  - decide and document readiness level explicitly; expected to be frontend-ready for troubleshooting/timeline display
- DB:
  - add dedicated export-job event table/read model, for example `export_job_events`
  - persist at least:
    - `event_id`
    - `export_job_id`
    - `event_type`
    - `from_status`
    - `to_status`
    - `actor_id`
    - `actor_type`
    - `note`
    - `payload`
    - `created_at`
  - optionally add event-count / latest-event summary helpers only if they materially simplify reads

## Success Criteria
- Export jobs durably record lifecycle audit events for:
  - `created`
  - `advanced_to_running`
  - `advanced_to_ready`
  - `advanced_to_failed`
  - `advanced_to_cancelled`
  - `result_ref_updated` when handoff metadata materially changes
- `GET /v1/export-jobs/{id}/events` returns a stable timeline ordered oldest to newest.
- Export-job detail exposes a lightweight event summary such as `latest_event` and `event_count`.
- Existing lifecycle/status/result-ref contracts remain backward compatible.
- OpenAPI clearly marks event payload as audit context only, not a runner log stream.
- `go test ./...` passes.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_028.md`
- `docs/api/openapi.yaml`

Expected in this round:
- `MODEL_HANDOVER.md`
- `docs/V7_API_READY.md`
- `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- `设计流转自动化管理系统_V7.0_重构版_技术实施规格.md`

## Risks
- Event semantics can drift if status changes and result-ref mutations are not emitted atomically in the same transaction.
- Timeline payload must stay lightweight; otherwise consumers may misread it as full runner logging.
- The new events endpoint must not imply that real download or storage integration already exists.

## Completion Output Format
1. Phase path
2. Changed files
3. DB / migration changes
4. API / OpenAPI changes
5. Auto-correction notes
6. Verification
7. Risks / remaining gaps
8. Next recommended phase
