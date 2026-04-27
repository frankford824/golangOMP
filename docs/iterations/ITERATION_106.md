# ITERATION_106

## Phase
Destructive reset (keep admin) + user-org patch contract closure + primary-SKU nested-field confirmation + overwrite publish to existing `v0.8` + full-loop verification

## Goal
- Reset the test environment to a clean baseline while keeping Admin/SuperAdmin and system skeleton.
- Close backend user-org patch semantics for `department/team/group`.
- Confirm the real primary-SKU nested object field path from code + live payload (no guessing).
- Overwrite publish to existing `v0.8` with runtime changes.
- Produce an end-to-end closed-loop verification report.

## Phase A Findings (Real Inventory)

### Keep set (must preserve)
- Admin/SuperAdmin users and role bindings.
- Base org/role/config sources:
  - auth/org settings (`config/auth_identity.json` load path),
  - `/v1/org/options`,
  - `/v1/roles`,
  - frontend access config,
  - categories/cost rules/code rules/rule templates, etc.
- Runtime/deploy skeleton:
  - `./cmd/server` entrypoint,
  - existing deploy/package scripts,
  - existing release line `v0.8`,
  - migrations/docs/repo artifacts.

### Clear set (must reset)
- Task chain and related records:
  - `tasks`, `task_details`, `task_sku_items`,
  - `procurement_records`, `procurement_record_items`,
  - `task_event_logs`, `task_event_sequences`,
  - audit/warehouse/outsource/cost-override records.
- Asset/upload chain:
  - `task_assets`, `design_assets`,
  - `upload_requests`, `asset_storage_refs`,
  - plus optional `asset_versions` when present.
- Log/integration/export traces:
  - `permission_logs`, `server_logs`,
  - integration/export/runtime event tables.
- Non-admin test users and their role/session records.
- Server/NAS/local test logs/tmp/cache objects.

### Org master-data status
- `/v1/org/options` is backend-generated from server auth settings (`identityService.authSettings`), not frontend-local-only data.
- No dedicated DB CRUD for org tree exists in current minimal architecture.
- This round keeps "backend config as authority" (Case A), and aligns user patch/read contracts to it.

## Phase B Execution (Destructive Reset)

### Scripts and fixes applied
- Updated:
  - `scripts/test_env_destructive_reset_keep_admin.sql`
  - `scripts/test_env_destructive_reset_keep_admin.sh`
- Key fixes:
  - backup naming standardized to `..._pre_reset_keep_admin`,
  - optional `asset_versions` cleanup with safe table-existence guards,
  - restart args aligned to current `start-bridge.sh` / `start-sync.sh` contract,
  - post-reset API verification block rewritten to robust remote heredoc mode.

### Final successful reset run
- UTC timestamp: `20260402T041507Z`
- Server backup:
  - `/root/ecommerce_ai/backups/20260402T041507Z_pre_reset_keep_admin`
- NAS backup:
  - `/volume1/homes/yongbo/asset-upload-service/backups/20260402T041507Z_pre_reset_keep_admin`

### Backup evidence
- `/root/ecommerce_ai/backups/20260402T041507Z_pre_reset_keep_admin/full_db.sql` (`1.1G`)
- `/root/ecommerce_ai/backups/20260402T041507Z_pre_reset_keep_admin/key_tables.sql` (`38K`)
- reset result:
  - `keep_admin_count=4`
  - task/asset/procurement/upload/permission/integration counters all `0`
  - `users_after_reset=4`
  - `user_sessions_after_reset=0`

### Post-reset endpoint verification
- `/v1/auth/me` => `200`
- `/v1/org/options` => `200`
- `/v1/roles` => `200`
- `/v1/tasks?page=1&page_size=20` => empty (`total=0`)

## Phase C Backend Org/User Closure

### Runtime code changes
- `transport/handler/user_admin.go`
  - `PATCH /v1/users/{id}` now accepts `group` as compatibility alias.
- `service/identity_service.go`
  - `UpdateUserParams` now supports `Group`.
  - Unified team/group normalization:
    - if both provided, must match.
  - Ungrouped semantic closure:
    - `team/group = "ungrouped"` => normalize to unassigned pool (`department=未分配`, `team=未分配池` from configured options).
    - if patch sets `department=未分配` without team/group, backend auto-fills configured unassigned pool team.
  - Added helpers:
    - `resolveTeamPatchInput`
    - `defaultUnassignedPoolTeam`

### Tests added
- `service/identity_service_test.go`
  - `TestIdentityServiceUpdateUserSupportsGroupAliasAndUngrouped`
  - `TestIdentityServiceUpdateUserRejectsTeamGroupConflict`

### Contract/docs update
- `docs/api/openapi.yaml`
  - added `/v1/org/options` path contract,
  - added `OrgOptions` / `ConfiguredUserAssignment` schemas,
  - updated `PATCH /v1/users/{id}` request schema to real partial-update fields:
    - `display_name`, `status`, `department`, `team`, `group`, `email`, `mobile`, `managed_departments`, `managed_teams`,
  - documented unassigned-pool / `ungrouped` semantics.

## Phase D Primary-SKU Nested Object Confirmation (No Guessing)

### Real endpoint and path
- Confirmed from code + live payload:
  - list: `GET /v1/tasks`
    - `item.product_selection.erp_product`
  - detail (light read-model): `GET /v1/tasks/{id}`
    - `data.product_selection.erp_product`
- Not in `task_detail.product_selection` for current read-model payload.

### Exact nested fields (live)
- `product_selection.erp_product.product_id`
- `product_selection.erp_product.sku_id`
- `product_selection.erp_product.sku_code`
- `product_selection.erp_product.product_name` (and `name`)

### Source field clarification
- There is no `product_selection.erp_product.source` field.
- Source/provenance is carried by sibling:
  - `product_selection.source_match_type`
  - plus `source_match_rule`, `source_search_entry_code`.

### Live sample (task `328`)
- `product_selection.erp_product.sku_code = "HSC19163"`
- `product_selection.erp_product.product_id = "HSC19163"`
- `product_selection.erp_product.product_name = "常规kt板/开业手举牌/蒸蒸日上-大吉大利/宽30cm/6个装"`
- `product_selection.source_match_type = "erp_bridge_keyword_search"`

## Required Local Verification
- `go test ./service ./transport/handler` => passed
- `go build ./cmd/server` => passed
- `go build ./repo/mysql ./service ./transport/handler` => passed
- `go test ./repo/mysql` => passed

## Publish (Overwrite Existing v0.8)
- Command:
  - `bash ./deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 reset+org-user patch+contract alignment"`
- `deploy/release-history.log`:
  - packaged/uploaded/deployed at `2026-04-02T04:08:23Z ... 2026-04-02T04:08:42Z`
  - artifact sha256: `c4c16fe3e656c3fa92ea51ff65369d166ae90496aafb726c2e49d59bb05a81c4`
- Entry point remains `./cmd/server`.

## Post-Deploy Runtime Verification
- `8080 /health = 200`
- `8081 /health = 200`
- `8082 /health = 200`
- `/proc/<pid>/exe` from runtime check:
  - `8080 -> /root/ecommerce_ai/releases/v0.8/ecommerce-api` (`exe_deleted=false`)
  - `8081 -> /root/ecommerce_ai/releases/v0.8/erp_bridge` (`exe_deleted=false`)
  - `8082 -> /root/ecommerce_ai/erp_bridge_sync` (`exe_deleted=false`)

## Full Closed-Loop Online Verification

### Mainline full-chain acceptance
- Script:
  - `/tmp/iteration106_live_verify.py` (copied from `scripts/iteration105_live_verify.py`)
- Result:
  - `/tmp/iteration106_live_verify_result.json`
  - local copy: `tmp/iteration106_live_verify_result.json`
  - summary: `101/101` passed, `failed_checks=0`
- Covered:
  - original/new(batch)/purchase create,
  - reference upload + design upload/preview/download,
  - canonical ownership + assign/reassign,
  - audit A/B + warehouse + close->Completed,
  - permission allow/deny + operation/task-event logs.

### User-org patch online verification
- Script:
  - `scripts/iteration106_org_patch_verify.py`
- Result:
  - `/tmp/iteration106_org_patch_verify_result.json`
  - local copy: `tmp/iteration106_org_patch_verify_result.json`
  - summary: `6/6` passed, including:
    - group alias patch success,
    - `ungrouped` -> unassigned pool normalization success,
    - final user readback consistent (`department/team/group`).

## Known Boundaries (Honest)
- Legacy `owner_team` is still present as compatibility field.
- Current model is still not full ABAC rollout.
- Org master-data management remains minimal (server-config authority, not full org platform CRUD).
- Historical task canonical ownership may still be incomplete for older data.
