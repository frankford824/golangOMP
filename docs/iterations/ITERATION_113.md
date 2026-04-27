# ITERATION_113

**Date:** 2026-04-07  
**Goal:** Keep NAS private-network direct upload/download strategy, enforce external explicit deny for multipart/direct large-file lanes, and close frontend menu vs route-role mismatch.

## Scope

- Keep existing strategy unchanged:
  - intranet/VPN users can keep browser direct NAS upload/download.
  - external users must not receive unusable private NAS browser endpoints.
- Add backend environment gate for multipart upload session issuance.
- Align `frontend_access` with actual route-role contract so department-only membership no longer surfaces task menus.
- Overwrite deploy to existing `v0.8` only.

## Root causes (confirmed)

### Upload

- NAS upload service returns browser URLs under private address (`http://192.168.0.125:8089/...`).
- Without backend environment gating, external HTTPS page users still got private URLs and hit browser network errors.
- This violated current policy ("private-network only"), because external requests should have been denied explicitly.

### Menu/permission mismatch

- Department-level frontend access previously could contribute business menus/pages/actions.
- Real route authorization (`/v1/tasks` etc.) still depends on workflow roles (`Ops/Designer/Audit/Warehouse/...`).
- Result: some users could see task menus but still get `403` on route access.

## Code/config changes

- `transport/handler/upload_network_access.go` (new):
  - added request-source IP extraction + CIDR allowlist evaluation.
  - supports `X-Real-IP`, `X-Forwarded-For`, and `RemoteAddr`.
- `transport/handler/task_asset_center.go`:
  - multipart create now gated by source network policy.
  - external disallowed source returns machine-readable:
    - `code=UPLOAD_ENV_NOT_ALLOWED`
  - added private-network download gate for `download_mode=private_network`.
  - added minimal gate logs for allow/deny diagnostics.
- `domain/errors.go`:
  - added `ErrCodeUploadEnvNotAllowed`.
- `transport/handler/response.go`
- `transport/auth_placeholder.go`
  - mapped `UPLOAD_ENV_NOT_ALLOWED` to HTTP `403`.
- `config/config.go`:
  - upload direct-access policy envs loaded:
    - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_POLICY_ENABLED` (default `true`)
    - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_CIDRS` (default private CIDRs)
- `cmd/server/main.go`
- `cmd/api/main.go`
  - wired upload access policy config into task-asset-center handler.
- `config/frontend_access.json`
- `domain/frontend_access.go`
- `service/identity_service.go`
  - department access now contributes scopes only, not business menus/pages/actions.
  - default authenticated UI remains minimal (`dashboard`, `dashboard_home`, `profile_me`).
- `service/upload_service_client.go`
- `service/upload_service_client_test.go`
  - kept browser multipart base rebasing support.
  - reverted proxy-path token suppression; multipart headers continue including `X-Internal-Token` for current direct contract.
- `deploy/main.env.example`:
  - reset browser multipart base to private NAS address (`http://192.168.0.125:8089`).
  - documented direct-access CIDR gate envs.
- `deploy/nginx/yongbo.cloud.conf`
- `deploy/nginx/yongbo.cloud.production.conf`
  - removed `/upload/` public same-origin proxy blocks (not part of current policy).

## Local verification

Executed and passed:

- `go test ./service ./transport/handler`
- `go build ./cmd/server`
- `go build ./repo/mysql ./service ./transport/handler`
- `go test ./repo/mysql`

Additional targeted pass:

- new handler tests for:
  - external multipart deny
  - internal multipart allow
  - small-upload bypass of multipart gate
  - private-network download external deny
  - direct download external allow

## Deploy

- Overwrite deploy to existing line only:
  - `bash deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 private-network upload-download gate + frontend_access-role alignment"`
- Entrypoint unchanged:
  - `./cmd/server`

Post-deploy runtime checks reported healthy:

- `8080 /health = 200`
- `8081 /health = 200`
- `8082 /health = 200`
- Active executables:
  - main: `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - bridge: `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - sync: `/root/ecommerce_ai/erp_bridge_sync`

## Live acceptance

### Upload gate

- External (`https://yongbo.cloud`):
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/multipart` returns:
    - HTTP `403`
    - `error.code=UPLOAD_ENV_NOT_ALLOWED`
    - includes `source_ip`, `allowed_cidrs`, `policy=private_network_only`
  - no multipart browser URL is returned to external caller.

- Internal-like source (`jst_ecs` local call to `127.0.0.1:8080`):
  - multipart session creation still returns private NAS URLs:
    - `remote.base_url=http://192.168.0.125:8089`
    - `part_upload_url_template/complete_url/abort_url` all under same private base.

### Multipart upload complete on `jst_ecs`

- Attempting to push part data from `jst_ecs` to `192.168.0.125:8089` failed with connectivity timeout.
- This indicates `jst_ecs` host itself is not inside NAS LAN route.
- Contract result remains correct:
  - external explicit deny is now active.
  - private direct lane remains available only where actual LAN/VPN reachability exists.

### Menu/permission consistency

Live account probes:

- `Member` + unassigned:
  - menus: `["dashboard"]`
  - `/v1/tasks` -> `403 PERMISSION_DENIED`
- `Member+Ops` with formal ops org:
  - menus include task entries (`task_create/task_board/task_list/...`)
  - `/v1/tasks` -> `200`
- `Member+Designer`:
  - menus include designer workspace/task list
  - `/v1/tasks` -> `200`
- `Member` but in ops department/team (no `Ops` role):
  - menus stay minimal (`dashboard`)
  - `/v1/tasks` -> `403`

This confirms department-only users no longer get task menus without workflow role.

## Account opening minimum template

- Ops account:
  - org: `运营部` + concrete team (e.g. `运营三组`)
  - roles: `[Member, Ops]`
- Designer account:
  - roles: `[Member, Designer]`
- Audit account:
  - roles: `[Member, Audit_A]` or `[Member, Audit_B]`
- Warehouse account:
  - roles: `[Member, Warehouse]`
- Management account:
  - `DepartmentAdmin + managed_departments`
  - or `TeamLead + managed_teams`

## Remaining risks / unfinished

- This iteration is a minimum closure under current strategy, not a final identity-governance redesign.
- `X-Internal-Token` is still present in browser multipart contract for direct path; long-term hardening should move token issuance/injection to short-lived/sessionized model.
- No live sample with existing `source` asset was found during this run to re-probe external private-network download deny against real source-file history; handler-level gate and tests are in place.
