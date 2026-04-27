# ITERATION_114

**Date:** 2026-04-08  
**Goal:** Backend-only closure for batch SKU-scoped reference/design contracts, office-egress upload allowlist, and `/v1/org/options` to task `owner_team` compatibility convergence.

## Scope

- Formalize batch-task reference/design behavior at SKU scope instead of leaving task-vs-SKU semantics ambiguous.
- Keep NAS browser-direct large-file strategy private, but stop misclassifying real office users who access `yongbo.cloud` through public company egress.
- Converge task-create owner-team compatibility with the configured auth org catalog so `/v1/org/options` and create validation no longer drift.
- Update migration, OpenAPI, and handover docs. No frontend code changes in this iteration.

## Root causes (confirmed)

### Problem 1

- `batch_items[].reference_file_refs` were only merged into mother-task `task_details.reference_file_refs_json`.
- `sku_items[].reference_file_refs` was not a persisted/read-model contract.
- design upload sessions and persisted assets had no formal SKU scope field, so frontend could not reliably distinguish "task-level" vs "SKU-level" design data.

### Problem 2

- multipart/private-network download allow logic only trusted private CIDRs from request source IP parsing.
- Real office users hitting `https://yongbo.cloud` appear as public company egress IPs, not `192.168.*`, so valid office traffic was rejected as external.

### Problem 3

- `/v1/org/options` truth came from configured auth settings.
- task create legacy `owner_team` compatibility still depended on a separate hardcoded bridge set.
- Result: configured org-team values could still fail create validation when the hardcoded bridge lagged behind the auth org tree.

## Final backend solution

### Problem 1

- Added formal per-SKU reference persistence:
  - `task_sku_items.reference_file_refs_json`
  - `GET /v1/tasks/{id}` -> `sku_items[].reference_file_refs`
- Kept top-level `reference_file_refs` as mother-task union summary for compatibility.
- Added formal SKU-scoped design upload path:
  - create upload session accepts `target_sku_code`
  - backend validates target SKU belongs to the task
  - persisted/read back via:
    - `upload_requests.target_sku_code`
    - `design_assets.scope_sku_code`
    - `task_assets.scope_sku_code`
    - `design_assets[].scope_sku_code`
    - `asset_versions[].scope_sku_code`

### Problem 2

- Kept existing private-network/NAS direct strategy.
- Added explicit office public egress allowlists:
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_IPS`
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_CIDRS`
  - legacy aliases still supported:
    - `UPLOAD_ALLOWED_PUBLIC_IPS`
    - `UPLOAD_ALLOWED_PUBLIC_CIDRS`
- Final allow rule:
  - private CIDR match => allow
  - configured office public IP/CIDR match => allow
  - everything else => `403 UPLOAD_ENV_NOT_ALLOWED`
- Same rule now covers:
  - multipart upload session creation
  - `download_mode=private_network` download responses

### Problem 3

- Replaced hardcoded task-org compatibility lookup with runtime-derived catalog from `cfg.Auth`.
- Task create normalization, canonical ownership inference, and legacy owner-team checks now all read the same configured catalog.
- Existing role-driven `frontend_access` vs route auth alignment from iteration 113 remains the effective permission/menu rule.

## Code / schema changes

- `service/task_org_catalog.go`
  - new runtime task-org compatibility catalog builder.
- `service/task_owner_team.go`
- `service/task_org_ownership.go`
  - switched legacy/canonical mapping lookups to runtime catalog.
- `service/task_asset_center_service.go`
- `service/task_asset_center_read_model.go`
- `service/task_design_asset_read_model.go`
- `service/task_batch_create.go`
- `service/task_sku_read_model.go`
  - wired batch SKU reference persistence/read model and design SKU scope propagation.
- `transport/handler/upload_network_access.go`
- `transport/handler/task_asset_center.go`
  - added public office egress allowlist policy and machine-readable deny details.
- `config/config.go`
- `cmd/server/main.go`
- `cmd/api/main.go`
  - loaded new upload-policy envs and runtime org catalog.
- `repo/mysql/task.go`
- `repo/mysql/task_asset.go`
- `repo/mysql/design_asset.go`
- `repo/mysql/upload_request.go`
  - persisted/read new SKU scope fields.
- `db/migrations/049_v7_batch_sku_asset_scope.sql`
  - added `task_sku_items.reference_file_refs_json`
  - added `design_assets.scope_sku_code`
  - added `task_assets.scope_sku_code`
  - added `upload_requests.target_sku_code`

## OpenAPI / docs

- `docs/api/openapi.yaml`
  - documented:
    - `sku_items[].reference_file_refs`
    - `design_assets[].scope_sku_code`
    - `asset_versions[].scope_sku_code`
    - upload-session `target_sku_code`
    - upload gate `UPLOAD_ENV_NOT_ALLOWED` semantics
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `ITERATION_INDEX.md`
- `deploy/main.env.example`

## Local verification

Passed:

- `go test ./service ./transport/handler`
  - with `GOTMPDIR` / `GOCACHE` pointed inside the repo because host Windows App Control blocks temp test executables under system temp.
- `go build ./cmd/server`
- `go build ./repo/mysql ./service ./transport/handler`
- `go test ./repo/mysql`

Targeted tests passed:

- `TestTaskServiceCreateBatchMergesItemLevelReferenceFileRefsWithValidation`
- `TestTaskReadModelBatchIncludesReferenceFileRefsAndFallbackAssetVersions`
- `TestTaskAssetCenterServiceCreateAndCompleteMultipartUploadSessionWithTargetSKUCode`
- `TestConfigureTaskOrgCatalogAlignsConfiguredOrgTeamsWithTaskCreate`
- `TestTaskServiceCreateBatchOwnerTeamCompatRegression`
- `TestTaskAssetCenterHandlerCreateMultipartUploadSessionDeniedForExternalSource`
- `TestTaskAssetCenterHandlerCreateMultipartUploadSessionAllowedForConfiguredOfficePublicIP`
- `TestTaskAssetCenterHandlerDownloadPrivateNetworkDeniedForExternalSource`
- `TestTaskAssetCenterHandlerDownloadPrivateNetworkAllowedForConfiguredOfficePublicIP`

## Deploy / live acceptance

- Backup before migration:
  - `/root/ecommerce_ai/backups/iter114_20260408T051352Z`
- Applied live migration:
  - `049_v7_batch_sku_asset_scope.sql`
- Updated live runtime env:
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_IPS=222.95.254.125`
- Overwrite deploy:
  - `bash deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 batch-sku-scope + office-egress-allowlist + runtime-org-bridge"`
  - entrypoint unchanged: `./cmd/server`
  - final deployed artifact sha:
    - `991268c62615a2efa9cea37fc8915e3063af224056cf2f2641e81445f0933b11`
- Runtime checks passed:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/<pid>/exe`:
    - main -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
    - bridge -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
    - sync -> `/root/ecommerce_ai/erp_bridge_sync`
- Live acceptance passed:
  - task `372` create with `owner_team="运营三组"` succeeded and persisted:
    - `owner_team="内贸运营组"`
    - `owner_department="运营部"`
    - `owner_org_team="运营三组"`
  - task `374` proved per-SKU refs:
    - `sku_items[0].reference_file_refs[0].asset_id=3b7657a8-319d-49ac-aada-f0876b7bf13a`
    - `sku_items[1].reference_file_refs[0].asset_id=d53e2f96-93b5-4183-9df0-1ae24250be7e`
  - task `373` delivery multipart upload completed with real NAS part upload + NAS `/complete` + MAIN `/complete`:
    - `design_assets[].scope_sku_code=["NSKT000070"]`
    - `asset_versions[].scope_sku_code=["NSKT000070"]`
  - task `373` multipart create through `https://yongbo.cloud` succeeded:
    - `target_sku_code=NSKT000070`
  - task `375` source download gate:
    - `X-Real-IP: 8.8.8.8` -> `403 UPLOAD_ENV_NOT_ALLOWED`
    - `X-Real-IP: 222.95.254.125` -> `200` with `download_mode=private_network`

## Remaining risks / unfinished

- Historical batch tasks are not backfilled from mother-task union refs into per-SKU refs, because old data does not encode true SKU provenance.
- Windows-hosted deploy initially hit CRLF shell-script line endings in `deploy/*.sh`; line endings were normalized to LF before the successful overwrite deploy.
