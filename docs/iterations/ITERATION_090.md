# ITERATION_090

Title: Task read-model design-asset visibility closure, `v0.8` overwrite publish, and live verification without `submit-design`

Date: 2026-03-23
Model: GPT-5 Codex

## 1. Background
- The round goal was fixed before execution:
  - do not re-investigate the upload chain itself
  - do not force frontend to blindly call `submit-design` in `PendingAuditA`
  - make `GET /v1/tasks/{id}` correctly reflect the persisted fact that upload-complete already wrote the design version
- Existing evidence already showed the problem shape:
  - upload succeeded
  - frontend later only requested `GET /v1/tasks/{id}`
  - frontend did not call `POST /v1/tasks/{id}/submit-design`
  - task detail response still lacked `asset_versions`

## 2. Root cause conclusion
- The gap was in the MAIN task read-model aggregation path, not in the upload-complete persistence chain.
- `upload-session complete` already persists the version fact:
  - `design_assets` root record
  - `task_assets` version row
  - `design_assets.current_version_id`
- But `GET /v1/tasks/{id}` previously did not aggregate those asset-center facts into the task read model.
- That created the false appearance that uploaded versions only became visible after `submit-design`.

## 3. Backend patch
- Read-model output contract additions:
  - `domain/query_views.go`
    - add `design_assets`
    - add compatibility projection `asset_versions`
- Main task read-model aggregation:
  - `service/task_service.go`
    - inject design-asset read dependencies into `taskService`
    - add `WithTaskDesignAssetReadModel(...)`
    - aggregate `design_assets` and flattened `asset_versions` into `GET /v1/tasks/{id}`
  - `service/task_design_asset_read_model.go`
    - new helper that reuses the asset-center hydration logic and flattens versions for the task read model
- Runtime wiring:
  - `cmd/server/main.go`
  - `cmd/api/main.go`
- Contract documentation:
  - `docs/api/openapi.yaml`
    - `TaskReadModel.design_assets`
    - `TaskReadModel.asset_versions`
    - explicit note that visibility is driven by upload-complete persistence, not by later `submit-design`
- Test updates:
  - `service/task_read_model_asset_versions_test.go`
    - adds targeted coverage for completed-upload visibility without `submit-design`
  - `service/task_prd_service_test.go`
    - fixes test stub ID backfill so the new read-model test path does not fail on fixture drift

## 4. Verification before release
- Verified changed files and tests for this round:
  - `domain/query_views.go`
  - `service/task_service.go`
  - `service/task_design_asset_read_model.go`
  - `cmd/server/main.go`
  - `cmd/api/main.go`
  - `docs/api/openapi.yaml`
  - `service/task_read_model_asset_versions_test.go`
  - `service/task_prd_service_test.go`
- Local build / test status:
  - `go build ./cmd/server ./cmd/api` passed
  - `go test -c ./service` passed
  - direct local execution of generated `.test.exe` binaries remained blocked by host Application Control, so live server validation was used as the runtime closure

## 5. Release closure
- Real release entrypoint used:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task read model reflects upload-complete design_assets asset_versions without submit-design"`
- Deployment target:
  - `jst_ecs:/root/ecommerce_ai/releases/v0.8`
- Deployment result:
  - overwrite publish to `/root/ecommerce_ai/releases/v0.8` completed
  - `/root/ecommerce_ai/current` points to `/root/ecommerce_ai/releases/v0.8`
  - three-service runtime verification after cutover reported:
    - `8080 main status=ok`
    - `8081 bridge status=ok`
    - `8082 sync status=ok`
    - `OVERALL_OK=true`
- Release-tooling side fix applied during closure:
  - normalized `deploy/*.sh` to LF so Bash release scripts run correctly from the Windows control node

## 6. Live verification
- Chosen real task:
  - `task_id=140`
  - `task_no=RW-20260320-A-000134`
- Why this task was chosen:
  - it has `task.asset.upload_session.completed`
  - it has `task.asset.version.created`
  - it has persisted `design_assets` and `task_assets`
  - it has no `task.design.submitted`
- Live verification method:
  - obtain real bearer token through `POST /v1/auth/login`
  - call live MAIN `GET /v1/tasks/140`
- Live response summary after publish:
  - `task_status = InProgress`
  - `design_assets_count = 4`
  - `asset_versions_count = 4`
  - returned `design_assets[].current_version.id` values:
    - `22`
    - `24`
    - `29`
    - `30`
  - returned `asset_versions[].id` values:
    - `22`
    - `24`
    - `29`
    - `30`
- Final live conclusion:
  - task detail now exposes uploaded version facts even when no `submit-design` event exists for the task

## 7. Final boundary
- `upload-session complete` is the persistence boundary for design-version facts.
- `GET /v1/tasks/{id}` is the read boundary and must surface persisted `design_assets` / `asset_versions` immediately after upload-complete succeeds.
- `submit-design` remains a business-action boundary:
  - explicit design submission
  - workflow transition / audit semantics
  - legacy/manual submission path where still needed
- `submit-design` is no longer the visibility switch for uploaded design versions.
