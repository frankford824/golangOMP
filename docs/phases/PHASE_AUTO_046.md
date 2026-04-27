# PHASE_AUTO_046

## Why This Phase Now
- Step 45 stabilized shared boundary vocabulary across export, integration, and storage/upload.
- Among the remaining placeholder seams, `upload_requests` has the clearest small runtime gap:
  - create/get exists
  - bind-on-task-asset exists
  - explicit cancel/expire lifecycle does not exist yet
- This is the safest next step because it deepens one placeholder runtime loop without committing the repo to real storage/upload infrastructure.

## Current Context
- Current CURRENT_STATE before this phase: Step 45 complete
- Current OpenAPI version before this phase: `0.41.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_045.md`
- Current phase audit file: `docs/phases/PHASE_AUDIT_046.md`

## Goals
- Add an explicit internal placeholder lifecycle advance path for upload requests
- Support small terminal actions:
  - `cancel`
  - `expire`
- Expose additive upload-request lifecycle readiness fields:
  - `can_bind`
  - `can_cancel`
  - `can_expire`
- Keep task-asset binding as the only path that moves an upload request into `bound`

## Allowed Scope
- Additive upload-request domain/read-model fields
- Additive upload-request repo/service/handler logic
- One internal placeholder route:
  - `POST /v1/assets/upload-requests/:id/advance`
- Focused tests
- Required document synchronization

## Forbidden Scope
- Real multipart upload
- Real object-storage / NAS / signed URL / CDN integration
- Real file hash verification beyond current metadata hints
- Task-asset workflow redesign
- Export/integration deeper runtime work
- Approval / finance / KPI / report work

## Expected File Changes
- Update upload-request domain model / hydration
- Add upload-request lifecycle update contract in repo layer
- Add upload-request advance service method
- Add upload-request advance handler and route registration
- Add focused service tests
- Update `docs/api/openapi.yaml`
- Update `CURRENT_STATE.md`
- Update `MODEL_HANDOVER.md`
- Add `docs/iterations/ITERATION_046.md`

## Required API / DB Changes
- No new DB tables or migrations
- Add one internal placeholder route:
  - `POST /v1/assets/upload-requests/:id/advance`
- Add additive upload-request read-model fields:
  - `can_bind`
  - `can_cancel`
  - `can_expire`

## Success Criteria
- Upload requests can move from `requested` to `cancelled` or `expired` through one explicit internal route
- Task-asset binding still moves `requested` to `bound` and remains the only binding path
- Terminal upload requests reject further lifecycle advancement
- OpenAPI and state docs describe the new lifecycle clearly without claiming real upload/storage infrastructure

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_046.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`

## Risks
- Later real upload/storage integration may need more than `requested|bound|expired|cancelled`
- Expiry remains a manual/internal placeholder action, not a background expirer or lease manager

## Completion Output Format
1. Changed files
2. DB / migration changes
3. API changes
4. Correction notes
5. Risks / known gaps
6. Suggested next step
