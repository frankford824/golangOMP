# ITERATION_085

## Date
2026-03-19

## Goal
Unify `reference_images` direct-upload limits, stop oversized create requests before create tx, eliminate `reference_images_json` DB overflow for original/new/purchase task creation, and overwrite-deploy the fix onto `v0.8`.

## Root Cause
- `reference_images` direct-create payloads were serialized into `task_details.reference_images_json`.
- The old guardrail (`200KB` single, `512KB` total, `5` images) lived mainly in handler code and did not fully protect every create path before DB write.
- When oversized payloads slipped into the transaction path, MySQL could fail with `Data too long for column 'reference_images_json'`, and the API surfaced `internal error during create task tx` as a `500`.
- The storage column still used the earlier `TEXT` capacity, which was not aligned with the new business requirement of `3MB * 3`.

## Code Changes
- Added shared validation in [service/reference_images.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/reference_images.go):
  - single image `<= 3MB`
  - max `3` images
  - total limit aligned to `3MB * 3`
  - standardized `400 INVALID_REQUEST` payload
  - pre-DB JSON size guard
- Updated [transport/handler/task.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/transport/handler/task.go) to use the shared validator before calling service create.
- Updated [service/task_service.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/task_service.go) to validate again before create tx and to validate the final `reference_images_json` serialization before insert.
- Added migration [db/migrations/044_v7_reference_images_mediumtext.sql](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/db/migrations/044_v7_reference_images_mediumtext.sql) to change `task_details.reference_images_json` from `TEXT` to `MEDIUMTEXT`.

## Test Coverage
- Added [service/reference_images_test.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/service/reference_images_test.go):
  - Case A: 1 image smaller than `3MB` passes
  - Case B: 3 images, including an exact `3MB` boundary image, pass
  - Case E: oversized image is rejected before `RunInTx`, so no tx-side `Data too long` path is reached
- Extended [transport/handler/task_erp_bridge_test.go](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/transport/handler/task_erp_bridge_test.go):
  - Case C: 4 images returns `400 INVALID_REQUEST`
  - Case D: any image `> 3MB` returns `400 INVALID_REQUEST` and reports `oversized_indexes`

## Documentation
- Updated [CURRENT_STATE.md](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/CURRENT_STATE.md)
- Updated [MODEL_HANDOVER.md](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/MODEL_HANDOVER.md)
- Updated [docs/FRONTEND_ALIGNMENT_v0.5.md](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/docs/FRONTEND_ALIGNMENT_v0.5.md)
- Updated [docs/TASK_CREATE_RULES.md](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/docs/TASK_CREATE_RULES.md)
- Updated [docs/api/openapi.yaml](/c:/Users/wsfwk/Downloads/yongboWorkflow/go/docs/api/openapi.yaml)

## Deployment Target
- Release line stays `v0.8`.
- This iteration is intended to overwrite the existing `v0.8` deployment rather than create `v0.9`.

## Deployment Result
- Executed `bash ./deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 reference_images upload limit hotfix"`.
- MAIN and Bridge were rebuilt from `./cmd/server` and overwritten onto `releases/v0.8`.
- Post-deploy runtime check passed:
  - MAIN `8080` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - Bridge `8081` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - Sync `8082` recovered and stayed healthy on the unchanged `erp_bridge_sync` binary
- Applied migration `044_v7_reference_images_mediumtext.sql` on live DB and verified `task_details.reference_images_json` is now `mediumtext`.

## Minimal Live Verification
- Login: `POST /v1/auth/login` with `admin / <ADMIN_PASSWORD>` -> `200`.
- Original create with one small `reference_images` data URI -> `201`, task created successfully (`task_id=111`).
- Original create with one oversized `reference_images` data URI (`3145729` bytes) -> `400 INVALID_REQUEST`.
- Oversized live response now returns:
  - `message=reference_images exceed upload limit`
  - `details.actual_count=1`
  - `details.max_count=3`
  - `details.max_single_bytes=3145728`
  - `details.oversized_indexes=[0]`
  - `details.suggestion=use asset-center upload / reference_file_refs`
- Verified outcome: the oversized request no longer returns `500`, and no `internal error during create task tx` fallback was observed in the API response path.
