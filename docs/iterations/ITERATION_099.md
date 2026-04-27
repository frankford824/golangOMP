# ITERATION_099

## Phase
Task-create reference small probe retry closure on live `v0.8`

## Input Context
- `CURRENT_STATE.md` / `MODEL_HANDOVER.md` already fixed the reference-small architecture baseline:
  - `reference = small`
  - MAIN uses `/upload/files`
  - MAIN does not call NAS `complete`
  - MAIN must probe stored bytes and verify size/hash before returning success
- User-reported current symptom:
  - `POST /v1/tasks/reference-upload`
  - `500 INTERNAL_ERROR`
  - `message = internal error during probe task-create reference stored file`
- Truth-source files investigated:
  - `transport/handler/task_create_reference_upload.go`
  - `service/task_create_reference_upload_service.go`
  - `service/upload_service_client.go`
  - `transport/handler/asset_files.go`

## Goals
- Locate the real failure point in the reference-small upload -> probe chain.
- Fix the failure without changing the small-vs-multipart architecture split.
- Self-test, overwrite-publish onto existing `v0.8`, and complete live verification.

## Files Changed
- `service/task_create_reference_upload_service.go`
- `service/upload_service_client.go`
- `service/task_create_reference_upload_service_test.go`
- `service/task_asset_center_service_test.go`
- `transport/handler/task_reference_upload_contract_test.go`
- `deploy/*.sh`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `ITERATION_INDEX.md`

## Root Cause
- The failing chain was confirmed to stay on the correct service-to-service base URL:
  - `UPLOAD_SERVICE_BASE_URL=http://100.111.214.38:8089`
  - browser multipart base stayed `http://192.168.0.125:8089`
- Live historical evidence showed the small upload result itself was valid:
  - `/upload/files` returned a real `storage_key`
  - MAIN selected the correct probe URL under `http://100.111.214.38:8089/files/{storage_key}`
- The real defect was an upload-to-probe visibility race, not a host/path contract mix-up:
  - archived traces such as `ref-upload-fix2-20260320-p1hRH` and `3774ace0-d32c-4130-9cd3-bf952a6ef50d` showed `probe` hitting the correct URL on the correct host and receiving `200`, but with `bytes_read=0` and `content_length=0`
  - that meant MAIN was probing before the newly written object became stably readable on NAS
  - with the old code, MAIN failed immediately on that first transient probe result
- Current user-facing `internal error during probe task-create reference stored file` is consistent with the same race family when the first probe attempt returns a temporary transport / 404 / 5xx error instead of a zero-byte `200`

## What Changed
- Added a narrow probe retry loop only on the task-create reference small path:
  - default `3` attempts
  - linear short backoff `200ms`, `400ms`
  - retries only for transient probe request failures (`404`, `409`, `425`, `429`, `5xx`, net/op errors) and `200` responses that still expose an empty stored object (`bytes_read=0` and `content_length=0`)
- Kept the original hard failure behavior for real bad states:
  - empty `storage_key`
  - incomplete probe metadata
  - stored size mismatch
  - stored hash mismatch
- Added bounded probe observability:
  - `upload_service_base_url`
  - `selected_probe_host`
  - `storage_key`
  - `filename`
  - `expected_size`
  - `expected_sha256`
  - per-attempt `probe_status` / `probe_error` / `retry_reason`
  - final mismatch logs
- Added upload-service probe logs with explicit `base_url` and `selected_probe_host`
- Kept the external contract unchanged:
  - no OpenAPI change
  - no change to `reference=small`
  - no change to multipart `remote.headers`
  - no `complete` call added back to the small path

## API Changes
- No API contract change.
- `POST /v1/tasks/reference-upload` still returns the same `ReferenceFileRef` object on success.
- Failures still return `INTERNAL_ERROR` with `trace_id`.

## Local Verification
- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed

## Live Rollout
- Initial deploy attempt using the required repo entrypoint failed because repo-local `deploy/*.sh` still had CRLF line endings and local `bash` rejected `set -euo pipefail`
- Normalized `deploy/*.sh` to LF and reran the same required entrypoint successfully:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task-create reference small probe retry and diagnostics"`
- Result:
  - overwrite publish onto existing `v0.8`
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/3484605/exe -> /root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/proc/3484628/exe -> /root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `/proc/3484806/exe -> /root/ecommerce_ai/erp_bridge_sync`
  - active executables were not deleted

## Live Acceptance
- Re-logged in on live `8080` and obtained a real bearer token.
- `POST /v1/tasks/reference-upload` with `p1hRH.png` returned `201` after overwrite:
  - trace `14d2ffd0-c52a-452d-a0d5-e3f87096eabf`
  - `asset_id=5d749472-deb2-4f89-8660-eb7eeef0c227`
  - `upload_request_id=2da9078d-cd94-47ce-8621-3f5197651751`
- Live logs for that trace showed the repaired chain end-to-end:
  - upload result carried `storage_key=tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/p1hRH.png`
  - probe host stayed `100.111.214.38:8089`
  - probe attempt `1/3` succeeded with matching size/hash
- Using that returned `reference_file_refs`, `POST /v1/tasks` for `new_product_development` returned `201`:
  - `task_id=168`
  - `task_no=RW-20260401-A-000163`
- Regression checks passed:
  - `GET /v1/tasks?page=1&page_size=5` -> `200`
  - existing batch task detail still readable:
    - sampled `task_id=167`
    - `is_batch_task=true`
    - `batch_item_count=2`
  - canonical ownership fields still present on live reads:
    - `owner_team`
    - `owner_department`
    - `owner_org_team`
- No live probe-failure sample was forced in this round:
  - safely reproducing a true probe race would require intentionally destabilizing NAS visibility or corrupting the stored object
  - that was not acceptable on the shared live environment

## Correction Notes
- The business fix stayed narrow.
- The only deploy-side correction was LF normalization for repo-local `deploy/*.sh` so the required existing package/deploy entrypoints could execute from this control node.

## Risks / Known Gaps
- The root cause sits on a short-lived NAS/upload-service visibility race outside MAIN.
- MAIN now tolerates the observed transient window, but it does not eliminate the underlying storage-side timing behavior.
- Live failure-path acceptance for a real probe race was not re-created intentionally on production.
- `original_product_development` defer-create was not re-run live in this round; regression confidence comes from preserved code boundaries and the required local package tests.

## Ready for Frontend
- `POST /v1/tasks/reference-upload` remains frontend-ready with the same success payload and error envelope.
- `POST /v1/tasks` continues to accept `reference_file_refs` objects returned by the upload route.

## Suggested Next Step
- If probe races continue to appear in logs, inspect the NAS/upload-service write visibility path directly and add storage-side durability/visibility guarantees instead of growing MAIN retries further.
