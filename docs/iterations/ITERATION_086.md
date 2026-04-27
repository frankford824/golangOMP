# ITERATION_086

## Goal
- Close the task-create reference-image chain onto one formal contract:
  - `POST /v1/tasks` only accepts `reference_file_refs`
  - `reference_images` returns `400 INVALID_REQUEST`
  - refs must come from completed pre-task asset-center uploads

## Code
- Added pre-task asset-center upload endpoints under `/v1/task-create/asset-center/upload-sessions`.
- Added backend validation for `reference_file_refs` against `asset_storage_refs` + `upload_requests`.
- Removed new-create dependence on `reference_images_json`; new task creates now persist `reference_file_refs_json` only.

## Tests
- `go test ./service`
- `go test ./cmd/server ./cmd/api ./transport`
- `go test -c -o .tmp_gotest/handler.test.exe ./transport/handler`
- `cmd /c ".\\.tmp_gotest\\handler.test.exe -test.v"`

## Deploy
- Overwrote deploy onto `v0.8` on `2026-03-19`.
- Release history:
  - `release|v0.8|...|packaged|...|4fbc4a0c1974cefe1491f98ba084ada8ef94820e77ce8593faef71f245313043|...`
  - `release|v0.8|...|uploaded|...|4fbc4a0c1974cefe1491f98ba084ada8ef94820e77ce8593faef71f245313043|...`
  - `release|v0.8|...|deployed|...|4fbc4a0c1974cefe1491f98ba084ada8ef94820e77ce8593faef71f245313043|...`
- Runtime verification passed:
  - `verify-runtime.sh` reported `OVERALL_OK=true`
  - MAIN `8080` health `200`
  - Bridge `8081` health `200`
  - Sync `8082` health `200`
- Live API verification passed:
  - valid completed `reference_file_ref` -> `POST /v1/tasks` returned `201`, `task_id=112`
  - any `reference_images` field -> `400 INVALID_REQUEST`
  - forged ref -> `400 INVALID_REQUEST`
  - uncompleted session id / incomplete ref source -> `400 INVALID_REQUEST`
  - large base64 `reference_images` payload -> `400 INVALID_REQUEST`, no create-tx `500`
