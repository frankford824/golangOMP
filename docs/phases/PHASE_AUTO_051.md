# PHASE_AUTO_051

## Why This Phase Now
- Post-Step-50 audit shows the safest next move is still inside the upload/storage placeholder seam.
- `upload_requests` already support create/get/advance/bind, but they still lack a paginated management view.
- Adding list/filter visibility closes an obvious internal management gap without choosing real upload/storage infrastructure.

## Current Context
- Current CURRENT_STATE before this phase: Step 50 complete
- Current OpenAPI version before this phase: `0.46.0`
- Latest iteration before this phase: `docs/iterations/ITERATION_050.md`
- Current execution audit file: `docs/phases/PHASE_AUDIT_051.md`

## Goals
- Add one internal placeholder paginated list route for upload requests.
- Support lightweight management filters:
  - `owner_type`
  - `owner_id`
  - `task_asset_type`
  - `status`
- Keep upload-request lifecycle semantics unchanged:
  - `requested`
  - `bound`
  - `expired`
  - `cancelled`
- Keep real upload/storage infrastructure explicitly deferred.

## Allowed Scope
- Additive upload-request query/list contracts in repo/service/handler/router/OpenAPI
- Focused upload-request tests
- State / iteration / handover synchronization

## Forbidden Scope
- Real upload sessions or byte transfer
- Signed URL generation
- NAS / object-storage integration
- File delivery / download system
- Task-asset lifecycle redesign
- Export / integration runtime work
- Real auth/org/runtime permission redesign

## Expected File Changes
- `docs/phases/PHASE_AUDIT_051.md`
- `docs/phases/PHASE_AUTO_051.md`
- `repo/interfaces.go`
- `repo/mysql/upload_request.go`
- `service/asset_upload_service.go`
- `service/asset_upload_service_test.go`
- `service/task_step04_service_test.go`
- `transport/handler/asset_upload.go`
- `transport/http.go`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `docs/iterations/ITERATION_051.md`

## Required API / DB Changes
- DB:
  - none
- API:
  - add `GET /v1/assets/upload-requests`
  - internal placeholder only
  - paginated list over existing `UploadRequest` read model

## Success Criteria
- `GET /v1/assets/upload-requests` returns paginated upload-request records.
- Filter validation is explicit and aligned with current placeholder enums.
- Existing create/get/advance/bind semantics remain unchanged.
- OpenAPI + CURRENT_STATE + iteration/handover docs are synchronized to Step 51 and `v0.47.0`.

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_051.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`

## Risks
- The list route improves management visibility only; it does not imply a real upload platform.
- Later real upload/session/storage systems may still require a different operational model behind the same placeholder seam.

## Completion Output Format
1. Changed files
2. DB / migration changes
3. API changes
4. Correction notes
5. Risks / known gaps
6. Suggested next step
