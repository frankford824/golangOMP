# ITERATION 120

Date: 2026-04-10
Model: GPT-5 Codex

## Goal

Overwrite release the already-completed org-master backendization and multi-SKU upload gate fix onto live `v0.9`, apply any missing live migration, complete real production acceptance, and sync the v0.9 source-of-truth docs.

## Release preparation

- Re-checked local commands and all passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`
- Confirmed migration exists in repo:
  - `db/migrations/050_v7_org_master_backendization.sql`
- Confirmed live had not applied `050` yet:
  - `org_departments` absent
  - `org_teams` absent
- Confirmed no new runtime env variable was required for this round beyond already-present upload/org runtime config.

## Migration and deploy

- Backup dir created before live migration:
  - `/root/ecommerce_ai/backups/pre-050-v0.9-20260410T052753Z`
- Backup contents included:
  - schema snapshot
  - migration precheck output
- Applied migration:
  - `db/migrations/050_v7_org_master_backendization.sql`
- First release attempt failed:
  - command: `bash deploy/deploy.sh --version v0.9 --release-note "v0.9 org master backendization + multi-sku pending-audit gate fix"`
  - failure: packaged `deploy/*.sh` contained CRLF, causing remote bash failure `set: pipefail\r: invalid option name`
- Fix:
  - normalized `deploy/*.sh` to LF
- Second release attempt:
  - same `deploy/deploy.sh` command
  - succeeded

## Runtime verification

- `8080 /health = 200`
- `8081 /health = 200`
- `8082 /health = 200`
- `/proc/<pid>/exe`:
  - main -> `/root/ecommerce_ai/releases/v0.9/ecommerce-api`
  - bridge -> `/root/ecommerce_ai/releases/v0.9/erp_bridge`
  - sync -> `/root/ecommerce_ai/erp_bridge_sync`
- Active executables were not deleted.

## Live org-master acceptance

- Created backend department:
  - `POST /v1/org/departments`
  - result: `acceptance_dept_v09_20260410` (`id=9`, enabled)
- Created backend team:
  - `POST /v1/org/teams`
  - result: `acceptance_team_v09_20260410` under department `9` (`id=15`, enabled)
- Created design-side org team for task bridge validation:
  - `design_acceptance_team_v09_20260410` under the existing design department (`id=16`, enabled)
- Verified `PUT` enable/disable semantics:
  - department `10` toggled off then on
  - team `17` toggled off then on
- Verified `/v1/org/options` returned backend-created values immediately.
- Verified user linkage:
  - `PATCH /v1/users/3` -> `department=acceptance_dept_v09_20260410`, `team=group=acceptance_team_v09_20260410`
  - `GET /v1/users/3` returned matching values and matching `frontend_access.department_codes/team_codes`
- Verified task create + owner-team bridge:
  - task `386`
  - input `owner_team=design_acceptance_team_v09_20260410`
  - persisted output kept legacy design-group compatibility for `owner_team` and persisted `owner_org_team=design_acceptance_team_v09_20260410`

## Live multi-SKU acceptance

- Batch task:
  - task id `386`
  - task no `RW-20260410-A-000381`
  - sku codes `NSLI000000`, `NSLI000001`
- Precheck:
  - batch non-reference upload without `target_sku_code` returned `400 INVALID_REQUEST`
  - message: `target_sku_code is required for batch non-reference asset uploads`
- SKU 1 upload:
  - session `d6a8b5f5-d1eb-4df2-949a-326c7d9260be`
  - remote upload id `1b17ea5566c3a00cdae4406a85dc247a`
  - after part upload + remote complete + MAIN complete, task stayed `InProgress`
- SKU 2 upload:
  - session `f622db17-db24-4426-ba01-fda6a262145e`
  - remote upload id `d51ab5677082b7328f1059cea2ba1a76`
  - after part upload + remote complete + MAIN complete, task advanced to `PendingAuditA`
- Final read model on task `386`:
  - `design_assets[].scope_sku_code = [NSLI000000, NSLI000001]`
  - `asset_versions[].scope_sku_code = [NSLI000000, NSLI000001]`
- Audit-side readback:
  - audit user `candidate_test` (`user_id=5`) read `GET /v1/tasks/386`
  - both SKU delivery assets and both version rows were visible at `PendingAuditA`

## Single-SKU regression

- Single task:
  - task id `387`
  - task no `RW-20260410-A-000382`
  - sku code `NSLI000002`
- After one multipart delivery upload + remote complete + MAIN complete:
  - task advanced to `PendingAuditA`
- This confirmed single-SKU flow was not blocked by the batch-only gate.

## Notes and residual risk

- An accidental garbled department created during early Windows stdin testing (`id=8`) was disabled on live and no longer appears in `/v1/org/options`.
- Batch delivery acceptance on live currently relies on valid browser-probe attestation payload shape plus multipart upload-service completion; frontend still needs to send `target_sku_code` on every batch non-reference upload.
