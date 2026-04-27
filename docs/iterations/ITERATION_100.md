# ITERATION_100

## Phase
Task-create reference small escaped-storage-key closure on live `v0.8`

## Input Context
- User reported stable repro on:
  - `POST /v1/tasks/reference-upload`
  - `500 INTERNAL_ERROR`
  - `message = internal error during probe task-create reference stored file`
  - latest trace `cba34f59-5f24-4280-9fea-c2b7e2d1eeee`
- Architecture baseline already fixed and must not change:
  - `reference = small`
  - `delivery/source/preview = multipart`
  - small reference uses `/upload/files`
  - small reference does **not** call NAS `complete`
  - success requires stored size/hash verification

## Goals
- Locate the real stable failure point using the provided trace.
- Fix the failure without changing the small-vs-multipart architecture split.
- Self-test, overwrite-publish onto existing `v0.8`, and complete live verification.

## Files Changed
- `domain/url_path.go`
- `service/upload_service_client.go`
- `transport/handler/asset_files.go`
- `service/task_create_reference_upload_service.go`
- `service/task_asset_center_read_model.go`
- `service/upload_service_client_test.go`
- `transport/handler/asset_files_test.go`
- `service/task_create_reference_upload_service_test.go`
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `ITERATION_INDEX.md`

## Root Cause
- The failing chain stayed on the correct server-to-server base URL:
  - `UPLOAD_SERVICE_BASE_URL=http://100.111.214.38:8089`
  - browser multipart base stayed `http://192.168.0.125:8089`
- The provided trace proved upload itself succeeded:
  - `/upload/files` returned a real `storage_key`
  - `storage_key` was `tasks/task-create-reference/assets/PRECREATE-REFERENCE/v1/derived/💚97% ... .jpg`
- The actual defect was local URL construction, not remote probe I/O:
  - MAIN built `"/files/{storage_key}"` directly
  - the raw `%` in the filename was treated as an invalid URL escape
  - all three probe attempts failed before HTTP dispatch with:
    - `invalid upload service path ... invalid URL escape "% \xf0"`
- Therefore the stable failure class was path escaping for `%`/UTF-8 storage keys.
- The earlier short-lived visibility race remains a separate storage-side behavior, but it was not the root cause of the stable repro.

## What Changed
- Added shared escaped-path helpers in `domain/url_path.go`.
- Updated `service/upload_service_client.go`:
  - probe URLs now escape storage keys by path segment before resolving against base URL
- Updated `transport/handler/asset_files.go`:
  - upstream proxy URLs now escape storage keys by path segment
  - proxy request logs no longer print the raw internal token value
- Updated `service/task_create_reference_upload_service.go`:
  - returned `public_url` / `lan_url` / `tailscale_url` are now escaped
- Updated `service/task_asset_center_read_model.go`:
  - delivery/preview/reference access URLs are now escaped for `%`/UTF-8 file names
- Kept the previously added bounded probe retry intact:
  - still only on reference small
  - still no `complete` call
  - still hard-fails on empty storage key, invalid metadata, size mismatch, and hash mismatch

## API Changes
- No API contract change.
- `POST /v1/tasks/reference-upload` still returns the same `ReferenceFileRef` shape on success.
- `POST /v1/tasks` still accepts those returned `reference_file_refs`.
- OpenAPI did not change.

## Local Verification
- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed

## Live Rollout
- Required repo deploy entrypoint used:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 fix escaped storage-key probe and proxy urls"`
- Result:
  - overwrite publish onto existing `v0.8`
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/3503354/exe -> /root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/proc/3503402/exe -> /root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `/proc/3503547/exe -> /root/ecommerce_ai/erp_bridge_sync`
  - active executables were not deleted

## Live Acceptance
- Historical failing evidence remained explicit:
  - trace `cba34f59-5f24-4280-9fea-c2b7e2d1eeee`
  - `/upload/files` returned a valid `storage_key`
  - probe attempts `1/3`, `2/3`, `3/3` all failed before request dispatch with `invalid URL escape "% \xf0"`
- After overwrite, `POST /v1/tasks/reference-upload` with filename `💚97% 能量充满啦.png` returned `201`:
  - trace `502f127d-02de-4c51-8446-99899df5530b`
  - `asset_id=569c7113-bde2-4ced-a20b-964336ac8b05`
  - `upload_request_id=43b241d6-dd05-42ea-ae4f-4f05433afd6f`
- Live logs for that trace showed:
  - correct `storage_key`
  - probe host stayed `100.111.214.38:8089`
  - `probe_status=200`
  - matching stored size/hash
  - escaped returned `public_url` / `lan_url` / `tailscale_url`
- Public proxy read passed on the escaped returned URL:
  - `GET /v1/assets/files/.../%F0%9F%92%9A97%25...png` -> `200`
  - body bytes `137253`
- Using that returned `reference_file_refs`, `POST /v1/tasks` for `new_product_development` returned `201`:
  - `task_id=169`
  - `task_no=RW-20260401-A-000164`
- Regression reads passed:
  - `GET /v1/tasks?page=1&page_size=5` -> `200`
  - existing batch task `167` remained readable with `is_batch_task=true`
  - `owner_team`, `owner_department`, `owner_org_team` still returned on live detail reads
- No forced live failure sample was executed after fix:
  - the original stable failure already had real evidence from the provided trace
  - storage-side race reproduction is still not appropriate on the shared live environment

## Risks / Known Gaps
- The stable user-reported failure is closed by escaped URL/path construction.
- The separate storage-side visibility race still belongs to NAS/upload-service if it reappears in logs.
- `original_product_development` defer-create was not re-run live in this round.
