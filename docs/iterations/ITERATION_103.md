# ITERATION_103

## Phase
MAIN task detail reference/design image contract investigation and confirmation

## Scope
- Investigated the actual read path of `GET /v1/tasks/{id}` for:
  - reference images
  - design assets and versions
  - legacy compatibility arrays
- Confirmed contract/document alignment with current truth-source boundaries.
- Added targeted tests and docs clarifications without changing runtime behavior.

## Root Cause
- The observed "image missing" reports were caused by reading legacy fields/arrays:
  - new task create intentionally persists `task_details.reference_images_json = []`
  - formal references are persisted in `task_details.reference_file_refs_json`
  - `GET /v1/tasks/{id}` returns formal `reference_file_refs` from that formal JSON field
- Design-asset visibility is already sourced from upload-complete persistence:
  - `design_assets` comes from `design_assets` roots
  - `asset_versions` comes from `task_assets` versions under each root
  - no `submit-design` dependency is required for visibility
- `/v1/assets/files/*` is a file proxy path, not a task-detail source of truth.

## Code-Level Findings
- `GET /v1/tasks/{id}`:
  - handler: `transport/handler/task.go` (`TaskHandler.GetByID`)
  - service read model: `service/task_service.go` (`loadTaskReadModel`, `enrichTaskReadModelDetail`)
  - reference source:
    - primary: `task_details.reference_file_refs_json`
    - legacy fallback: `task_details.reference_images_json` (compatibility only)
  - design source:
    - `service/task_design_asset_read_model.go` -> asset-center read model
    - `service/task_asset_center_service.go` / `service/task_asset_center_read_model.go`
    - persisted from upload-complete into `design_assets` + `task_assets`
- Create/upload chain:
  - `POST /v1/tasks` rejects `reference_images` and accepts `reference_file_refs`
  - `createSingleTask`/`createBatchTask` keep `ReferenceImagesJSON = "[]"` and persist `ReferenceFileRefsJSON`
  - `POST /v1/tasks/reference-upload` returns reference objects for `reference_file_refs`

## Files Changed
- Tests:
  - `service/reference_images_test.go`
- Docs:
  - `docs/api/openapi.yaml`
  - `CURRENT_STATE.md`
  - `MODEL_HANDOVER.md`
  - `ITERATION_INDEX.md`
  - `docs/iterations/ITERATION_103.md`
- Runtime:
  - none

## Local Verification
- Passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`

## Publish
- Not required.
- Reason: no runtime code change (tests/docs only).

## Live Verification
- Not executed in this round.
- Reason: no runtime code change, no deploy required.

## Explicit Boundary
- Formal task-detail contract remains:
  - references: `reference_file_refs`
  - design assets: `design_assets`
  - design versions: `asset_versions`
- Legacy compatibility remains readable but non-canonical:
  - `reference_images_json` fallback path still exists for old data only
  - it is not restored as a write-time source of truth
