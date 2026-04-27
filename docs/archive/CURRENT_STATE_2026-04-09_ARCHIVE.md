# CURRENT_STATE

## 2026-04-09 v0.9 consolidation authority
- Before using any older section in this file as a current contract claim, read `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`.
- This file remains a historical state log. It is no longer the standalone authority for official-vs-compatibility route classification.

## 2026-04-08 user management backend closure on existing `v0.8` (iteration 117, latest live truth source)
- This section is the latest live truth source for MAIN user-management backend capability.
- Formal backend capability after this round:
  - `GET /v1/users`
    - server-side pagination
    - `keyword` search over `username` / `display_name`
    - server-side filters: `status` / `role` / `department` / `team`
    - response includes `id` / `username` / `display_name` / `department` / `team` / `roles` / `status` / `frontend_access`
  - `GET /v1/users/{id}`
    - returns user base fields, org binding, current roles, status, and computed `frontend_access`
  - `POST /v1/users`
    - admin-managed formal create path
    - validates `department/team` against `/v1/org/options`
    - validates `roles` against the workflow role catalog
    - persists initial password hash and returns created user
  - `PUT /v1/users/{id}/password`
    - admin-managed password reset path
    - updates local password hash and returns user
  - `PATCH /v1/users/{id}`
    - existing formal update path remains the disable/enable mechanism through `status=active|disabled`
  - role management remains formal and backend-owned:
    - `GET /v1/roles`
    - `PUT /v1/users/{id}/roles` for full-set replacement
    - `POST /v1/users/{id}/roles` for additive grant
    - `DELETE /v1/users/{id}/roles/{role}` for single-role removal

### Delete / disable semantics
- There is still no physical delete or soft-delete tombstone endpoint in MAIN.
- Formal frontend action for “删除/禁用” should map to:
  - `PATCH /v1/users/{id}` with `{"status":"disabled"}`
- Disabled-user runtime semantics:
  - login returns `403 PERMISSION_DENIED`
  - existing session-backed request resolution still re-checks user status, so disabled users do not keep normal authenticated access

### Password semantics
- User self-service password change remains:
  - `PUT /v1/auth/password`
- Admin-managed password reset is now formalized:
  - `PUT /v1/users/{id}/password`
- This minimal reset path updates password hash only and does not revoke already-issued session tokens.

### Organization / permission truth-source consistency
- `/v1/org/options` remains the account-org truth source for user management writes.
- This round did not introduce any second org catalog or second role catalog.
- `frontend_access` continues to be computed from the persisted role set plus canonical user `department/team`, so:
  - backend route auth and frontend menu/page/action hints stay on the same role truth source
  - role updates are no longer expected to be faked by frontend local state
- Task-side truth sources were not changed in this round:
  - task create `owner_team` compatibility bridge remains separate and unchanged
  - canonical task ownership fields and action authorization remain on their existing truth sources

### Local verification
- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed
- `go test ./repo/mysql` -> passed
- New regression coverage added for:
  - managed user create
  - managed password reset
  - user-list `department/team/role/keyword` filtering
  - disabled-user login denial
  - handler binding for create/reset/list filters

### Live acceptance captured in this iteration
- Deploy:
  - overwritten onto existing `v0.8` with current deploy script
  - runtime verify passed on `8080` / `8081` / `8082`
  - `/proc/<pid>/exe` remained healthy and not deleted
- Live user-management acceptance through `https://yongbo.cloud`:
  - admin login `200`
  - `/v1/auth/me` `200`
  - `/v1/org/options` `200`
  - `POST /v1/users` created live user `id=177`
  - `GET /v1/users` with `keyword+department+team+role+page/page_size` returned the created user and `pagination.total=1`
  - `GET /v1/roles` returned `17` catalog entries
  - `GET /v1/users/177` returned roles plus `frontend_access`
  - `PUT /v1/users/177/roles` changed live roles from `Ops` to `Designer`
  - new user `/v1/auth/me` reflected the new `Designer` role and matching `frontend_access` pages/actions
  - `PUT /v1/users/177/password` reset password successfully
  - old password login returned `401 UNAUTHORIZED`
  - new password login returned `200`
  - `PATCH /v1/users/177` with `status=disabled` returned disabled user
  - disabled user login returned `403 PERMISSION_DENIED`

## 2026-04-08 attested NAS probe-driven upload/download gate on existing `v0.8` (iteration 116, latest live truth source)
- This section supersedes the earlier allowlist-driven upload gate notes as the latest live truth for large-file browser-direct admission:
  - same browser entry remains `https://yongbo.cloud`
  - large-file multipart upload and private-network download are now **probe-driven**
  - browser success probe must carry NAS-signed `attestation`; MAIN no longer trusts bare frontend-reported `reachable/method/url/status`
  - legacy source-IP CIDR/public allowlists remain diagnostic-only and are no longer the primary gate
  - `POST /v1/tasks/reference-upload` remains available for small external-safe uploads

### Effective gate contract
- NAS upload service now exposes lightweight browser probe:
  - `GET http://192.168.0.125:8089/upload/ping`
  - response returns:
    - `reachable`
    - `method`
    - `url`
    - `checked_at`
    - `status_code`
    - signed `attestation`
- MAIN multipart/private-download gate now requires:
  - fresh probe evidence
  - expected probe method/url match
  - success status
  - valid NAS-signed `attestation`
- Deny contract:
  - `HTTP 403`
  - `error.code=UPLOAD_ENV_NOT_ALLOWED`
  - details include:
    - `policy=probe_driven_browser_nas_reachability`
    - `reason`
    - `probe_reachable`
    - `probe_method`
    - `probe_url`
    - `probe_checked_at`
    - `probe_status_code`
    - `probe_attestation_present`
    - `probe_attested`
    - `source_ip`
  - `expected_probe_url` is intentionally not echoed in deny details, so missing-probe denies do not leak the NAS private URL

### Live acceptance captured in this iteration
- NAS probe endpoint:
  - rebuilt live NAS upload service on `synology-dsm`
  - `curl -H 'Host: 192.168.0.125:8089' http://127.0.0.1:8089/upload/ping` returned:
    - `200`
    - signed `attestation`
    - `url=http://192.168.0.125:8089/upload/ping`
- Multipart create through `https://yongbo.cloud`:
  - missing `network_probe` -> `403 UPLOAD_ENV_NOT_ALLOWED`, `reason=probe_missing`
  - forged success probe without `attestation` -> `403 UPLOAD_ENV_NOT_ALLOWED`, `reason=probe_attestation_missing`
  - valid attested probe -> `201`, returned NAS private multipart base:
    - `remote.base_url=http://192.168.0.125:8089`
- Private-network download:
  - missing probe headers -> `403 UPLOAD_ENV_NOT_ALLOWED`, `reason=probe_missing`
  - forged success probe headers without attestation -> `403 UPLOAD_ENV_NOT_ALLOWED`, `reason=probe_attestation_missing`
  - valid attested probe headers -> `200`, `download_mode=private_network`
- External-safe small upload:
  - `POST /v1/tasks/reference-upload` still returned `201`
- SKU-scoped asset contract remained intact on live:
  - existing task detail still returns `sku_items[].reference_file_refs`
  - existing task detail still returns `design_assets[].scope_sku_code`
  - existing task detail still returns `asset_versions[].scope_sku_code`

### Organization consistency re-verified
- `/v1/org/options` still returns `运营三组` under `运营部`
- live create re-verified with valid `new_product_development` payload plus:
  - `owner_team="运营三组"`
  - `category_code=CAT-PROBE-001`
  - `material_mode=preset`
  - `material=纸`
- create returned `201` and normalized ownership correctly:
  - `owner_team="内贸运营组"`
  - `owner_department="运营部"`
  - `owner_org_team="运营三组"`
- The earlier local `invalid_owner_team` probe was caused by local script encoding corruption, not by live backend bridge regression.

## 2026-04-08 yongbo.cloud single-domain upload gate live verification + fixed-strategy freeze (iteration 115, latest live truth source)
- This section supersedes older upload-gate investigation notes as the latest live truth for the single-domain rule:
  - same browser entry remains `https://yongbo.cloud`
  - office/intranet-allowed sources may create multipart source/design upload sessions and receive NAS private-network direct-upload plans
  - external sources are explicitly denied for multipart/private-network large-file lanes with machine-readable `UPLOAD_ENV_NOT_ALLOWED`
  - external sources still keep `POST /v1/tasks/reference-upload`

### Current source IP resolution chain
- MAIN upload/download gate currently resolves request source in this order:
  - `X-Real-IP`
  - `X-Forwarded-For`
    - current code takes the last valid token in the header chain
  - `RemoteAddr`
- Under the current live `yongbo.cloud` Nginx, `/v1` forwards:
  - `Host`
  - `X-Real-IP $remote_addr`
  - `X-Forwarded-For $proxy_add_x_forwarded_for`
  - `X-Forwarded-Proto $scheme`
- Because Nginx always sets `X-Real-IP`, current live classification is effectively driven by `X-Real-IP`.

### Confirmed root cause of historical misclassification
- The root cause was not missing Nginx forwarding and not a frontend caller problem.
- Real office users reached MAIN as public office egress IP `222.95.254.125`, not as RFC1918 private address.
- Before the live runtime env was updated with office public egress allowlist, the gate only trusted private CIDRs, so office traffic was classified as external.
- Historical deny evidence remained in live logs:
  - `2026-04-07 14:20:38 +0800` `source_ip="222.95.254.125"` `allowed=false`
  - `2026-04-08 10:31:29 +0800` `source_ip="222.95.254.125"` `allowed=false`
- Current live env now contains:
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_IPS=222.95.254.125`

### Final fixed strategy
- Allow multipart/private-network large-file lanes when request source matches either:
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_CIDRS`
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_IPS`
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_CIDRS`
- Deny all other public sources with:
  - `HTTP 403`
  - `error.code=UPLOAD_ENV_NOT_ALLOWED`
  - details:
    - `source_ip`
    - `policy=private_network_or_configured_public_ip`
    - `allowed_cidrs`
    - `allowed_public_ips`
    - `allowed_public_cidrs`
    - `reason`
- Reference-small upload is intentionally exempt:
  - `POST /v1/tasks/reference-upload` continues to work for external users
- Download gate stays aligned with upload gate:
  - only `download_mode=private_network` responses are source-IP gated
  - public/direct-compatible download lanes stay unchanged

### Live acceptance re-verified in this iteration
- Office source sample:
  - real browser/API traffic through `https://yongbo.cloud` still appears at MAIN as `source_ip=222.95.254.125`
  - Nginx access examples:
    - `POST /v1/tasks/371/asset-center/upload-sessions` -> `201` at `2026-04-08 13:38:31 +0800`
    - `POST /v1/tasks/reference-upload` -> `201` at `2026-04-08 13:37:07 +0800`
  - MAIN gate logs now show:
    - `source_ip="222.95.254.125"` `allowed=true` `reason="source_x_real_ip_matched_allowed_public"`
- External source sample:
  - real external verification host: `openclaw` public IP `8.222.174.253`
  - `POST https://yongbo.cloud/v1/tasks/375/asset-center/upload-sessions/multipart` returned:
    - `403`
    - `error.code=UPLOAD_ENV_NOT_ALLOWED`
    - `details.source_ip=8.222.174.253`
    - no NAS private multipart plan was returned
  - `POST https://yongbo.cloud/v1/tasks/reference-upload` from the same external host returned:
    - `201`
    - valid reference file ref object
- Office-allowed response body was re-checked against live MAIN:
  - with `X-Real-IP: 222.95.254.125`, `POST /v1/tasks/375/asset-center/upload-sessions/multipart` returned `201`
  - response still carried NAS private browser plan:
    - `remote.base_url=http://192.168.0.125:8089`
    - `part_upload_url_template/complete_url/abort_url` under the same private base
- Download gate was re-checked against live MAIN on real private-network asset metadata:
  - task `375` asset `58`
  - office-allowed source `222.95.254.125` -> `200`, `download_mode=private_network`
  - external source `8.8.8.8` -> `403`, `UPLOAD_ENV_NOT_ALLOWED`
- Existing flow evidence remains valid:
  - task `373` still shows completed multipart delivery asset persisted on live
  - task status is now `PendingAuditA`, proving upload completion can continue into downstream workflow

### Operational freeze / anti-regression rule
- Keep `https://yongbo.cloud` as the only browser entry; do not split into separate intranet/extranet sites.
- Maintain the office public egress allowlist in `/root/ecommerce_ai/shared/main.env` whenever company出口 IP/CIDR changes.
- If a CDN/WAF/other proxy layer is added in front of Nginx later, revisit trusted-proxy / real-IP handling before assuming the current `X-Real-IP` precedence remains correct.
- No new runtime binary change was required in this verification iteration; live `v0.8` from iteration 114 already matched the intended behavior once allowlist config was present.

## 2026-04-08 batch SKU reference/design scope formalization + office-egress upload allowlist + owner-team truth-source convergence (iteration 114, latest live truth source)
- Latest live truth for the three pending backend ambiguities:
  - batch task references/design assets are now formally **SKU-scoped where business data is SKU-scoped**;
  - browser-direct multipart/private-network download is allowed from private office/VPN ranges **or** configured office public egress IP/CIDR allowlists;
  - task create `owner_team` compatibility now derives from the same configured auth org catalog that powers `/v1/org/options`, instead of a separately frozen hardcoded bridge.

### Problem 1: batch task reference/design contract is now explicit
- Previous ambiguity:
  - `batch_items[].reference_file_refs` were merged only into mother-task `task_details.reference_file_refs_json`.
  - `sku_items[].reference_file_refs` was not a formal persisted/read-model field.
  - design uploads were task-level only; no formal SKU scope field existed on upload session / asset root / asset version.
- Current backend contract:
  - top-level `reference_file_refs` remains the mother-task union summary for compatibility.
  - per-SKU reference truth is now persisted and returned on:
    - `task_sku_items.reference_file_refs_json`
    - `GET /v1/tasks/{id}` -> `sku_items[].reference_file_refs`
  - design upload session accepts:
    - `target_sku_code`
  - SKU-scoped design persistence/read model is now exposed on:
    - `upload_requests.target_sku_code`
    - `design_assets.scope_sku_code`
    - `task_assets.scope_sku_code`
    - `GET /v1/tasks/{id}` -> `design_assets[].scope_sku_code`
    - `GET /v1/tasks/{id}` -> `asset_versions[].scope_sku_code`
- Frontend implication:
  - batch Tab switch must read `sku_items[productIndex].reference_file_refs` for references.
  - batch design list must filter by `asset.scope_sku_code` / `version.scope_sku_code == active sku_code`.
  - top-level `reference_file_refs` must be treated only as mother-task summary, not as per-SKU substitution.
- Historical-data boundary:
  - migration `049` does **not** backfill old batch tasks from mother-task union into per-SKU rows, because historical provenance is ambiguous.
  - pre-049 historical batch tasks may still show empty `sku_items[].reference_file_refs` unless they are recreated/reuploaded under the new contract.

### Problem 2: upload/download environment gate now matches real office access
- Previous ambiguity:
  - direct browser multipart gate trusted only private CIDRs, so office users accessing `https://yongbo.cloud` through public company egress were misclassified as external.
- Current runtime policy:
  - allow when source IP matches:
    - private office/VPN CIDRs (`UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_CIDRS`)
    - configured office public IP allowlist (`UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_IPS`, legacy alias `UPLOAD_ALLOWED_PUBLIC_IPS`)
    - configured office public CIDR allowlist (`UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_CIDRS`, legacy alias `UPLOAD_ALLOWED_PUBLIC_CIDRS`)
  - deny all other public sources with:
    - `HTTP 403`
    - `error.code=UPLOAD_ENV_NOT_ALLOWED`
    - details:
      - `source_ip`
      - `policy=private_network_or_configured_public_ip`
      - `allowed_cidrs`
      - `allowed_public_ips`
      - `allowed_public_cidrs`
      - `reason`
- Scope:
  - same policy is enforced for multipart upload session creation.
  - same policy is enforced for asset download when `download_mode=private_network`.
  - public/small/direct-compatible lanes remain unchanged.

### Problem 3: org/options, owner_team bridge, and permission truth sources are further converged
- Previous ambiguity:
  - `/v1/org/options` came from configured auth org tree, but task create owner-team compatibility still depended on a separately frozen hardcoded mapping set.
- Current repo truth:
  - `service.ConfigureTaskOrgCatalog(cfg.Auth)` now builds the task-side compatibility catalog from runtime auth settings.
  - task create normalization, canonical ownership inference, and legacy-bridge checks all read the same runtime-derived catalog.
  - result:
    - configured org-team values with deterministic legacy mapping can pass create validation and persist canonical ownership together.
    - `owner team must be a valid configured team` regressions are reduced to truly unmapped/invalid inputs, not stale bridge config.
- Permission/menu truth:
  - `frontend_access` vs route-role alignment from iteration 113 remains in force:
    - business menus stay role-driven
    - department-only membership stays minimal
    - formal business accounts must still carry workflow roles (`Ops`, `Designer`, `Audit_A/B`, `Warehouse`, etc.)

### Local verification (iteration 114)
- Passed:
  - `go test ./service ./transport/handler` with `GOTMPDIR` / `GOCACHE` pointed inside the repo (workaround for host Windows App Control on system temp test binaries)
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`
- Targeted coverage added/passed:
  - batch per-SKU refs persisted + read-model returned on `sku_items[].reference_file_refs`
  - batch design upload session persisted `target_sku_code` and projected `scope_sku_code`
  - office public egress IP allowlist accepted for multipart upload and private-network download
  - configured org-team values normalized into valid create-time legacy `owner_team`

### Deploy/runtime truth (iteration 114)
- Backup + migration:
  - backup dir:
    - `/root/ecommerce_ai/backups/iter114_20260408T051352Z`
  - live migration applied:
    - `049_v7_batch_sku_asset_scope.sql`
  - live runtime config updated:
    - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_IPS=222.95.254.125`
- Overwrite deploy:
  - existing line only:
    - `v0.8`
  - entrypoint unchanged:
    - `./cmd/server`
  - deployed artifact:
    - `991268c62615a2efa9cea37fc8915e3063af224056cf2f2641e81445f0933b11`
- Health/runtime:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/<pid>/exe`:
    - main -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
    - bridge -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
    - sync -> `/root/ecommerce_ai/erp_bridge_sync`

### Live acceptance (iteration 114)
- Problem 1:
  - batch task `374` created with two different per-SKU refs.
  - live response returned:
    - `sku_items[0].sku_code=NSKT000071` with only ref `3b7657a8-319d-49ac-aada-f0876b7bf13a`
    - `sku_items[1].sku_code=NSKT000072` with only ref `d53e2f96-93b5-4183-9df0-1ae24250be7e`
    - top-level `reference_file_refs` remained mother-task union summary containing both refs.
  - batch task `373` multipart delivery upload completed after real NAS part PUT + NAS `/complete` + MAIN `/complete`.
  - live result returned:
    - `design_assets[].scope_sku_code = ["NSKT000070"]`
    - `asset_versions[].scope_sku_code = ["NSKT000070"]`
- Problem 2:
  - live public-domain multipart create via `https://yongbo.cloud` on task `373` succeeded and returned:
    - `target_sku_code=NSKT000070`
    - browser direct NAS URLs under `http://192.168.0.125:8089`
  - source asset download on task `375`, asset `58`:
    - simulated external `X-Real-IP: 8.8.8.8` -> `403 UPLOAD_ENV_NOT_ALLOWED`
    - simulated office egress `X-Real-IP: 222.95.254.125` -> `200`, `download_mode=private_network`
- Problem 3:
  - live batch task `372` and `373` accepted create payload with `owner_team="运营三组"`.
  - persisted/read back:
    - `owner_team="内贸运营组"`
    - `owner_department="运营部"`
    - `owner_org_team="运营三组"`

## 2026-04-07 private-network upload/download gate + frontend_access permission alignment closure on existing `v0.8` (iteration 113, latest truth source)
- Latest truth for keeping current NAS strategy:
  - intranet/VPN browser direct upload/download remains allowed.
  - external users are explicitly denied for multipart/private-network large-file lanes.

### Upload root cause and policy closure
- Root cause:
  - upload service returns browser URLs under private NAS host (`http://192.168.0.125:8089/...`).
  - external page users previously still received those URLs, then failed with browser network errors.
- Final behavior:
  - multipart session issuance is backend-gated by source IP CIDR policy.
  - disallowed source returns:
    - `HTTP 403`
    - `error.code=UPLOAD_ENV_NOT_ALLOWED`
    - machine-readable details (`source_ip`, `allowed_cidrs`, `policy`, `reason`).
  - no multipart browser URL is returned to external caller.
- Current runtime defaults:
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_POLICY_ENABLED=true`
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_CIDRS=127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,100.64.0.0/10,::1/128,fc00::/7`
  - `UPLOAD_SERVICE_BROWSER_MULTIPART_BASE_URL=http://192.168.0.125:8089`

### Nginx/deploy strategy truth
- This iteration explicitly does **not** move to public same-origin `/upload`.
- Repo nginx templates (`deploy/nginx/yongbo.cloud*.conf`) no longer include `/upload/` public reverse-proxy block.
- `deploy/main.env.example` is aligned to private-network direct browser base + CIDR-gate envs.

### Download policy closure
- Task asset download endpoints now enforce the same private-network gate when:
  - `download_mode=private_network`.
- External disallowed source receives explicit `UPLOAD_ENV_NOT_ALLOWED` instead of private-network-only ambiguity.
- Direct/public-compatible download mode remains unchanged.

### frontend_access vs route-role alignment closure
- Department-level frontend access is now scope-only and no longer grants business menus/pages/actions.
- Business menus remain role-driven (`Ops/Designer/Audit/Warehouse/...`) and now align with route auth.
- Result:
  - department/team without workflow role no longer gets task menus.
  - avoids menu-visible-but-route-403 mismatch for this lane.

### Live acceptance (iteration 113)
- External check (`https://yongbo.cloud`):
  - `POST /v1/tasks/{id}/asset-center/upload-sessions/multipart` -> `403`
  - `error.code=UPLOAD_ENV_NOT_ALLOWED`
  - details include external `source_ip`.
- Intranet-like source check (`jst_ecs` localhost call to `127.0.0.1:8080`):
  - multipart session creation succeeded and returned:
    - `remote.base_url=http://192.168.0.125:8089`
    - private NAS part/complete/abort URLs.
- End-to-end part upload from `jst_ecs` to `192.168.0.125:8089` failed by connectivity timeout:
  - indicates `jst_ecs` host is not in LAN route to NAS.
  - policy behavior remains correct (external denied, private route required for direct upload).

### Role/menu live probes
- `Member` + `未分配/未分配池`:
  - menus only `dashboard`
  - `/v1/tasks` -> `403 PERMISSION_DENIED`
- `Member+Ops` + `运营部/运营三组`:
  - task menus visible
  - `/v1/tasks` -> `200`
- `Member+Designer`:
  - design/task menus visible
  - `/v1/tasks` -> `200`
- `Member` but `运营部/运营三组` without `Ops`:
  - menus still minimal
  - `/v1/tasks` -> `403`

### Account opening minimum template (current)
- Ops:
  - org: `运营部` + concrete team (example: `运营三组`)
  - roles: `[Member, Ops]`
- Designer:
  - roles: `[Member, Designer]`
- Audit:
  - roles: `[Member, Audit_A]` or `[Member, Audit_B]`
- Warehouse:
  - roles: `[Member, Warehouse]`
- Management:
  - `DepartmentAdmin + managed_departments`
  - or `TeamLead + managed_teams`

### Deployment/runtime truth
- Overwrite deploy to existing line:
  - `v0.8` only, entrypoint unchanged `./cmd/server`.
- Runtime health after overwrite:
  - `8080=200`, `8081=200`, `8082=200`
  - active executables resolved under expected release/binary paths and not deleted.

## 2026-04-02 category short-code coding-rule closure on existing `v0.8` (iteration 112, latest truth source)
- Latest truth for moving default task product-code from raw `category_code` to two-letter uppercase short code.

### Why old format happened
- Previous default generator used:
  - `NS + category_code + 6-digit sequence`
- So `KT_STANDARD` produced:
  - `NSKT_STANDARD000060`

### New runtime rule (live)
- Format:
  - `NS + category_short_code(2 uppercase letters) + 6-digit sequence`
  - regex: `^NS[A-Z]{2}[0-9]{6}$`
- Short-code priority:
  - explicit map first (`KT_STANDARD -> KT`)
  - else first two alphabet letters from `category_code` (uppercased)
  - else deterministic fallback to two uppercase letters
- Scope:
  - enabled: `new_product_development`, `purchase_task`
  - not enabled: `original_product_development`

### Sequence/uniqueness truth
- Allocation key switched to short-code scope:
  - `(prefix, category_short_code)`
- Different `category_code` values collapsing to same short code share one sequence lane.
- On first allocation for a lane (`next_value=0`), allocator bootstraps from historical `task_sku_items` max suffix under `NS + short_code + 6-digit` before incrementing.

### Deploy truth
- Existing deploy chain only:
  - `deploy/deploy.sh`
- Existing release line only:
  - overwritten `v0.8` (no new release line)
- Entrypoint unchanged:
  - `./cmd/server`
- Release evidence (`deploy/release-history.log`):
  - deployed artifact sha256: `9dd601696fcad8c719a437526d99c4d6b5cf9b2dc5ece94a5b14c41321935b60`
- Migration:
  - no new migration was applied in iteration 112; `048_v7_product_code_sequences.sql` remained already-applied from iteration 111.

### Runtime verification (live)
- Health:
  - `8080 /health = 200` (external probe)
  - `8081 /health = 200` (remote localhost probe)
  - `8082 /health = 200` (remote localhost probe)
- PID + executable:
  - main pid `3838150` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - bridge pid `3838173` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - sync pid `3838987` -> `/root/ecommerce_ai/erp_bridge_sync`
- Active executable targets are not `(deleted)`.

### Live acceptance truth
- Deprecated template route:
  - `GET /v1/rule-templates/product-code` -> `400 INVALID_REQUEST`
  - message: `rule_templates/product-code is deprecated; use default backend task product-code generation`
  - `GET /v1/rule-templates` -> `200`, current list returns no `product-code`.
- Prepare endpoint:
  - `POST /v1/tasks/prepare-product-codes` with `KT_STANDARD`, `count=3`
  - returned: `NSKT000017`, `NSKT000018`, `NSKT000019`
  - regex match: pass; in-batch duplicate: none
- New single create:
  - `task_id=361`
  - `sku_code=NSKT000028`
  - regex match: pass
- Purchase single create:
  - `task_id=362`
  - `sku_code=NSKT000029`
  - regex match: pass
- Batch create:
  - `task_id=365`
  - `primary_sku_code=NSKT000032`
  - `sku_items[].sku_code = [NSKT000032, NSKT000033]`
  - all regex pass; duplicate: none
- Same-short-code collision lane:
  - `KT_STANDARD` create -> `task_id=363`, `sku_code=NSKT000030`
  - `K-T-standard` create -> `task_id=364`, `sku_code=NSKT000031`
  - duplicate: none
- Lightweight concurrency:
  - method: 8 parallel `prepare-product-codes` requests (`count=1`)
  - returned: `NSKT000020` ... `NSKT000027`
  - duplicate: none; error count: 0

### Other live regression spot checks
- `GET /v1/tasks?page=1&page_size=5` -> `200`
- Assign action:
  - `POST /v1/tasks/361/assign` -> `200` (`InProgress`)
- Canonical ownership remains on read:
  - `owner_team`, `owner_department`, `owner_org_team`
- Detail response lane remains:
  - `design_assets` present
  - `asset_versions` present

### Probe correction record (honest failure log)
- First create probes in this run failed due request-shape issues, not generator defects:
  - missing `owner_team`
  - invalid/legacy owner fields
  - batch item required-field omissions (`product_short_name/material_mode/design_requirement`)
- After correcting request payload to current contract, new/purchase/batch acceptance passed.

### Frontend collaboration truth
- Frontend must not configure/use `rule_templates/product-code`.
- Frontend must not compute `category_short_code` client-side.
- Task create mainline:
  - `POST /v1/tasks` (backend auto-generates for new/purchase types)
- Optional pre-display:
  - `POST /v1/tasks/prepare-product-codes`
- Read fields:
  - single/purchase: `data.sku_code`
  - batch: `data.sku_items[].sku_code`
  - compatibility fallback: `data.sku_code` / `data.primary_sku_code`

### Remaining boundaries
- This iteration formalizes two-letter short-code default generation on live; it is not a full/global numbering-platform finalization.
- `product_code_sequences` physical schema column remains named `category_code` while runtime semantics now use short code as key value.

## 2026-04-02 overwrite deploy + live coding-rule acceptance closure on existing `v0.8` (iteration 111, archived previous truth source)
- Latest truth for promoting ITERATION_110 default task product-code runtime to live `v0.8` and completing live acceptance evidence.

### Deployment truth
- Existing chain only (`deploy/deploy.sh`), version pinned to existing line `v0.8`, entrypoint unchanged `./cmd/server`.
- First run:
  - deploy stage completed but runtime-verify failed from remote CRLF script issue (`pipefail^M`).
- Re-run:
  - `--skip-tests --skip-runtime-verify` deploy succeeded.
- Final repair run:
  - after one failed remote CRLF conversion attempt, one more overwrite deploy restored scripts and runtime package state.
- Release evidence (`deploy/release-history.log`):
  - latest deployed artifact sha256: `8fea0be9a4fcfa5a3324c47ca885146033675c58ed0348f09650d605c5a02bd8`.

### Migration truth (`048_v7_product_code_sequences.sql`)
- Live pre-check found:
  - `task_sku_items` + `uq_task_sku_items_sku_code` already existed.
  - `product_code_sequences` was missing.
- Applied with backup-first:
  - backup dir: `/root/ecommerce_ai/backups/iter110_048_20260402T082531Z`
  - migration applied from `/root/ecommerce_ai/releases/v0.8/db/migrations/048_v7_product_code_sequences.sql`
  - post-check: `product_code_sequences` exists with unique `(prefix, category_code)`.

### Runtime verification (live)
- `8080 /health = 200`
- `8081 /health = 200`
- `8082 /health = 200`
- Active executables:
  - main: `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - bridge: `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - sync: `/root/ecommerce_ai/erp_bridge_sync`
- All active executables are not deleted.

### Live coding-rule acceptance truth
- `rule_templates/product-code`:
  - `GET /v1/rule-templates/product-code` -> `400`
  - message: `rule_templates/product-code is deprecated; use default backend task product-code generation`
  - `GET /v1/rule-templates` does not return `product-code`.
- Default rule now verified on live:
  - pattern: `NS + category_code + 6-digit sequence`
  - active scopes: `new_product_development`, `purchase_task`
  - non-scope: `original_product_development`.
- New single:
  - `task_id=352`
  - `sku_code=NSKT_STANDARD000022`
- Purchase single:
  - `task_id=353`
  - `sku_code=NSKT_STANDARD000023`
- Batch:
  - `task_id=354`
  - `primary_sku_code=NSKT_STANDARD000024`
  - item codes: `NSKT_STANDARD000024`, `NSKT_STANDARD000025`
  - duplicate: no
- Prepare endpoint:
  - `POST /v1/tasks/prepare-product-codes` returned `NSKT_STANDARD000026~000028` (no duplicate).
- Lightweight concurrency:
  - 8 parallel prepare calls returned `NSKT_STANDARD000029~000036`
  - duplicate: no
  - error count: 0.

### Other live regression spot checks
- `GET /v1/tasks?page=1&page_size=5` -> `200`.
- Assign action:
  - `POST /v1/tasks/352/assign` -> `200`, task status now `InProgress`.
- Canonical ownership fields still returned on task read:
  - `owner_team`, `owner_department`, `owner_org_team`.
- Detail design lane still present:
  - `design_assets`, `asset_versions` present on `/v1/tasks/{id}/detail`.
- Current detail reference lane observed on live:
  - `task_detail.reference_file_refs_json` present (`[]` in checked lanes),
  - top-level detail `reference_file_refs` not observed in this run.

### Frontend collaboration truth
- Frontend should not configure/use `rule_templates/product-code`.
- Frontend should not client-generate task product codes.
- Task create mainline remains `POST /v1/tasks`; backend generates SKU for new/purchase task types.
- Optional pre-display uses `POST /v1/tasks/prepare-product-codes`.
- Read fields:
  - single/purchase: `data.sku_code`
  - batch: `data.sku_items[].sku_code`
  - compatibility fallback: `data.sku_code` / `data.primary_sku_code`.

### Remaining boundaries
- This iteration brings default task product-code to live with evidence; it is not a full/global numbering-platform finalization.
- Deploy/runtime verify scripts on this control node still have CRLF risk and need ops hygiene.

## 2026-04-02 overwrite deploy + live acceptance closure on existing `v0.8` (iteration 109, latest truth source)
- Latest truth for promotion of the already-finished batch reference fix to live `v0.8`.

### Deployment result
- Used existing deploy chain only (`deploy/deploy.sh`, entrypoint unchanged: `./cmd/server`).
- First publish attempt hit remote runtime-verify script CRLF issue (`pipefail^M`) after deploy stage.
- Re-ran deploy with existing script and `--skip-runtime-verify`; overwrite to existing `v0.8` succeeded.
- Release evidence (`deploy/release-history.log`):
  - final deployed artifact sha256: `b7e43bde56f23a6a17b7a6b9796e8e0a7006d5c720c7e44b475cc9aaa1b4c017`.

### Runtime verification (live)
- `8080 /health = 200`
- `8081 /health = 200`
- `8082 /health = 200` (sync process restarted after CRLF normalization on remote scripts)
- Active executables:
  - main: `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - bridge: `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - sync: `/root/ecommerce_ai/erp_bridge_sync`
- No active executable points to `(deleted)`.

### Live acceptance evidence
- Single task with refs:
  - `task_id=343`, detail `reference_file_refs` present and non-empty.
- Batch task with refs only in `batch_items[].reference_file_refs`:
  - `task_id=344`, create `201`;
  - detail top-level `reference_file_refs` non-empty with expected ids
    (`18c4f798-ba8d-4526-aff2-b288154f08ee`, `b8a85ff5-5da3-4a8a-a09f-1c273b64afe6`).
- Empty-ref task:
  - `task_id=345`, detail includes `reference_file_refs: []` (field present).
- Design regression check:
  - `task_id=339`, `design_assets=1`, `asset_versions=1`.
- Mainline spot checks:
  - batch SKU create lane passed (`task_id=344`)
  - assign action passed (`POST /v1/tasks/345/assign` -> `200`, now `InProgress`, `designer_id=5`)
  - list route passed (`GET /v1/tasks?page=1&page_size=5` -> `200`)
  - canonical ownership fields remain on detail (`owner_team`, `owner_department`, `owner_org_team`).

### Contract status
- Formal detail fields remain:
  - `reference_file_refs`
  - `design_assets`
  - `asset_versions`
- No rollback to legacy `reference_images` or old `/v1/assets` as primary detail source.
- `sku_items[].reference_file_refs` is still not a formal response contract.

## 2026-04-02 batch task reference merge + stable `reference_file_refs` JSON (iteration 108, latest truth source)
- Latest truth for **batch create** reference persistence and **GET /v1/tasks/{id}** reference field shape.

### Additional root cause (beyond iteration 107 design-asset fallback)
- Batch clients often attach `reference_file_refs` only under **`batch_items[]`**. The HTTP handler previously mapped only top-level `reference_file_refs` into `CreateTaskParams`, so the mother task was created with an empty `task_details.reference_file_refs_json` while uploads were valid.
- `TaskReadModel.reference_file_refs` used `json:",omitempty"`, so an empty reference list **disappeared from JSON**; some frontends treat “field missing” like “no reference images”.

### Runtime / contract fixes (108)
- `service/task_batch_create.go`: `mergeBatchItemReferenceFileRefsIntoTask` folds `batch_items[].reference_file_refs` into the top-level list (deduped by `asset_id`) before validation and persistence.
- `service/task_service.go`: run merge immediately after `normalizeCreateTaskRequest`; `enrichTaskReadModelDetail` always sets a non-nil slice (empty when none).
- `domain/query_views.go`: removed `omitempty` on `TaskReadModel.reference_file_refs` so responses always include `"reference_file_refs": []` when there are no refs.
- `transport/handler/task.go`: `batch_items[].reference_file_refs` mapped into service params.
- `docs/api/openapi.yaml`: `CreateTaskBatchItem.reference_file_refs` + clarified `TaskReadModel.reference_file_refs` always present.

### Tests added (108)
- `service/task_batch_reference_file_refs_test.go`
  - `TestMergeBatchItemReferenceFileRefsIntoTask`
  - `TestTaskServiceCreateBatchMergesItemLevelReferenceFileRefsWithValidation`
  - `TestTaskReadModelReferenceFileRefsAlwaysSlice`

### Local verification (108)
- `go build ./...` succeeded on the development host.
- `go test ./service ...` could not be executed here: Windows Application Control blocked `service.test.exe` (policy). Re-run tests on an unrestricted runner or CI.

### Publish / live (108)
- Not executed from this session (no deploy script run). Prior `v0.8` artifact from iteration 107 remains the last recorded overwrite until a new deploy is performed.

## 2026-04-02 batch task detail image return fix (iteration 107)
- Truth source for **design_assets / asset_versions** fallback when `design_assets` roots are empty.

### Root cause (confirmed)
- `reference_file_refs` for batch tasks is persisted at mother-task detail level (`task_details.reference_file_refs_json`), not `task_sku_items` level.
- `GET /v1/tasks/{id}` and `/v1/tasks/{id}/detail` both reuse `loadTaskDesignAssetReadModel` for `design_assets` + `asset_versions`.
- Existing read-model implementation returned early when `design_assets` roots were empty.
  - Result: if task-level `task_assets` versions existed but `design_assets` roots were missing, detail response returned empty `design_assets`/`asset_versions`.
  - Single tasks usually had complete roots, so they looked normal; affected batch tasks showed missing image preview data.

### Runtime fix
- Updated `service/task_design_asset_read_model.go`:
  - keep existing formal root-first path (`design_assets` as primary source)
  - add minimal fallback aggregation when roots are empty:
    - task-level `task_assets` versions are grouped by `asset_id`
    - synthetic read-model roots are built only for response projection
    - `design_assets` and `asset_versions` are hydrated with the same derived fields/roles logic
- No contract rollback:
  - formal fields remain `reference_file_refs`, `design_assets`, `asset_versions`
  - old `reference_images`/old `/v1/assets` were not promoted to main truth source

### Tests added
- `service/task_design_asset_read_model_test.go`
  - `TestLoadTaskDesignAssetReadModelFallsBackWhenRootsMissing`
- `service/task_read_model_asset_versions_test.go`
  - `TestTaskReadModelBatchIncludesReferenceFileRefsAndFallbackAssetVersions`
  - asserts `reference_file_refs_json` precedence over legacy `reference_images_json`
- `service/task_detail_asset_versions_test.go`
  - `TestTaskDetailAggregateBatchIncludesFallbackAssetVersions`

### Local verification
- Passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`

### Publish/live
- Executed overwrite publish to existing `v0.8`:
  - `bash ./deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 batch task detail image fallback projection fix"`
- Release evidence (`deploy/release-history.log`):
  - deployed at `2026-04-02T04:44:38Z`
  - artifact sha256: `791de8fac0082de48ebdbb9e511586574f9e7c3feabdd0f288011c1031bbcfce`
- Runtime checks after deploy:
  - `/health` on `8080` => `{"status":"ok"}`
  - `/proc/<pid>/exe` for main PID (`3774193`) => `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
- Live task-detail image verification (real upload-complete path):
  - created single task `338` and batch task `339`
  - both details returned:
    - `reference_file_refs_count=1`
    - `design_assets_count=1`
    - `asset_versions_count=1`
    - `preview_public_url_ok=true`

## 2026-04-02 destructive reset + user-org patch closure + primary-SKU nested-field confirmation on existing v0.8 (latest truth source)
- This section is the latest truth source for iteration 106.

### Runtime/code and contract changes completed
- Runtime:
  - `service/identity_service.go`
    - `PATCH /v1/users/{id}` backend now supports `group` alias (with `team` consistency guard).
    - `ungrouped` semantic supported: `team/group="ungrouped"` normalizes to configured unassigned pool (`department=未分配`, `team=未分配池`).
    - when patching `department=未分配` without `team/group`, backend auto-fills configured unassigned pool team.
  - `transport/handler/user_admin.go`
    - request contract accepts `group` and forwards it to identity service.
- Tests:
  - `service/identity_service_test.go`
    - added `TestIdentityServiceUpdateUserSupportsGroupAliasAndUngrouped`
    - added `TestIdentityServiceUpdateUserRejectsTeamGroupConflict`
- OpenAPI:
  - `docs/api/openapi.yaml`
    - added `/v1/org/options` path contract.
    - added `OrgOptions` and `ConfiguredUserAssignment` schemas.
    - aligned `PATCH /v1/users/{id}` request body with real partial-update fields (`department/team/group/...`).
    - clarified primary nested SKU snapshot read path and source-field semantics (`source_match_type`, not `erp_product.source`).

### Destructive reset execution (completed)
- Final successful run:
  - `scripts/test_env_destructive_reset_keep_admin.sh`
  - UTC: `20260402T041507Z`
- Backup directories:
  - server: `/root/ecommerce_ai/backups/20260402T041507Z_pre_reset_keep_admin`
  - NAS: `/volume1/homes/yongbo/asset-upload-service/backups/20260402T041507Z_pre_reset_keep_admin`
- Backup artifacts:
  - full DB dump `full_db.sql` (~`1.1G`)
  - key tables dump `key_tables.sql` (~`38K`)
- Reset SQL result highlights:
  - `keep_admin_count=4`
  - `tasks/task_details/task_assets/design_assets/procurement/task_event/upload/permission/integration` => `0`
  - `users_after_reset=4`
  - `user_sessions_after_reset=0`
- Post-reset API checks:
  - `/v1/auth/me` `200`
  - `/v1/org/options` `200`
  - `/v1/roles` `200`
  - `/v1/tasks?page=1&page_size=20` => empty

### Primary-SKU nested object (confirmed from live payload)
- Confirmed read paths:
  - list: `GET /v1/tasks` -> `item.product_selection.erp_product`
  - detail: `GET /v1/tasks/{id}` -> `data.product_selection.erp_product`
- Confirmed nested fields include:
  - `product_id`, `sku_id`, `sku_code`, `product_name` (and `name`)
- Source/provenance is not under `erp_product.source`.
  - Use `product_selection.source_match_type` (+ `source_match_rule` / `source_search_entry_code`).
- Live sample (`task_id=328`) confirmed:
  - `product_selection.erp_product.sku_code = "HSC19163"`
  - `product_selection.source_match_type = "erp_bridge_keyword_search"`

### Required local verification (completed)
- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed
- `go test ./repo/mysql` -> passed

### Publish to existing v0.8 (completed)
- Command:
  - `bash ./deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 reset+org-user patch+contract alignment"`
- Release evidence (`deploy/release-history.log`):
  - deployed at `2026-04-02T04:08:42Z`
  - artifact sha256: `c4c16fe3e656c3fa92ea51ff65369d166ae90496aafb726c2e49d59bb05a81c4`
- Runtime checks after deploy:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - main/bridge executables point to `/root/ecommerce_ai/releases/v0.8/*` and are not deleted.

### Closed-loop live verification (completed)
- Full-chain script result:
  - `/tmp/iteration106_live_verify_result.json`
  - local copy: `tmp/iteration106_live_verify_result.json`
  - summary: `101/101` passed.
- User-org patch script result:
  - `/tmp/iteration106_org_patch_verify_result.json`
  - local copy: `tmp/iteration106_org_patch_verify_result.json`
  - summary: `6/6` passed.

### Remaining explicit boundaries
- `owner_team` compatibility field still exists.
- This is still not full ABAC.
- Org management remains minimal (server-config authority, no full org platform CRUD).
- Historical canonical ownership can still be incomplete on old tasks.

## 2026-04-02 MAIN performance optimization + full live e2e acceptance on existing v0.8 (latest truth source)
- This section is the latest truth source for iteration 105: real hotspot optimization on MAIN, overwrite publish to existing `v0.8`, and full-chain live acceptance.

### Runtime/code changes completed
- Performance-focused runtime changes (no business-contract change):
  - `service/task_design_asset_read_model.go`
    - task detail design asset read-model now hydrates versions by one `ListByTaskID` fetch plus in-memory grouping/sorting by `asset_id`, removing per-asset repeated lookups.
  - `service/task_data_scope_guard.go`
    - data-scope resolution now reuses actor context org fields first and only falls back to `userRepo.GetByID` when actor scope is absent, reducing repeated user read overhead.
  - `repo/mysql/identity.go`
    - added `ListRolesByUserIDs(ctx, []int64)` to batch role reads for user-list scenarios.
  - `service/identity_service.go`
    - `/v1/users` role attachment path now prefers batched role hydration via `ListRolesByUserIDs`.
    - `/v1/org/options` switched to lightweight in-process once-cache + defensive clone on return.
- Verification tests added/updated for these optimizations:
  - `service/task_design_asset_read_model_test.go`
  - `service/identity_service_test.go`
- No create/action/upload/detail permission contract semantics were changed.
- No migration/index/schema file added in this round.

### Required local verification (completed)
- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed
- `go test ./repo/mysql` -> passed

### Publish to existing v0.8 (completed)
- Release line and entrypoint preserved:
  - version: `v0.8` (overwrite in place)
  - runtime entrypoint: `./cmd/server` (unchanged)
- Deploy command used:
  - `bash ./deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 performance optimization: task detail/user role/scope"`
- Release evidence:
  - `deploy/release-history.log` contains `packaged/uploaded/deployed` entries at `2026-04-02T02:57:47Z ... 2026-04-02T02:58:13Z`
  - artifact sha256: `8ec7b4ea4e9c6d20e26252d134a982aecb2327386946d6372da5e6c88a1eff8a`
- Post-deploy runtime health verified:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/3745752/exe -> /root/ecommerce_ai/releases/v0.8/ecommerce-api` (not deleted)
  - `/proc/3745781/exe -> /root/ecommerce_ai/releases/v0.8/erp_bridge` (not deleted)
  - `/proc/3745951/exe -> /root/ecommerce_ai/erp_bridge_sync` (not deleted)

### Full live acceptance (completed end-to-end)
- Authoritative result artifact:
  - `tmp/iteration105_live_verify_result.json`
  - remote source: `/tmp/iteration105_live_verify_result.json` on `jst_ecs`
- Latest full run:
  - `started_at=2026-04-02T03:28:25Z`
  - `finished_at=2026-04-02T03:28:33Z`
  - checks: `101/101` passed
- Covered modules and chain:
  - Admin/auth/org/roles:
    - login, `/v1/auth/me`, `/v1/org/options`, `/v1/roles`
  - task create:
    - `original_product_development` defer create
    - `new_product_development` single + batch
    - `purchase_task` single + batch
    - create rejects: duplicate batch items, original batch forbidden, invalid team, machine-readable violations
  - reference image:
    - `POST /v1/tasks/reference-upload` (small reference, probe/readable)
    - detail visibility via `reference_file_refs`
    - preview/download routes
  - design assets:
    - multipart session create/upload/remote complete/main complete
    - detail returns `design_assets` + `asset_versions` immediately after upload-complete
    - preview/download route verification
  - list/detail/ownership:
    - `owner_team` + canonical `owner_department`/`owner_org_team` verified in list/detail
    - batch fields verified (`is_batch_task`, `batch_item_count`, `batch_mode`, `primary_sku_code`, `sku_generation_status`, `sku_items`)
  - action flow:
    - assign/reassign allow + out-of-scope deny + stage deny
    - audit A/B allow + out-of-scope deny + stage mismatch deny
    - warehouse allow + out-of-scope deny + stage mismatch deny
    - close allow to `Completed` + repeat close deny
  - logs/audit:
    - task events contain:
      - `task.created`, `task.assigned`, `task.reassigned`,
      - `task.audit.claimed`, `task.audit.approved`,
      - `task.warehouse.received`, `task.warehouse.completed`,
      - `task.closed`
    - permission logs have both allow and deny (`permission_log_scan.ok=true`)
    - operation logs non-empty
  - observed deny codes:
    - `audit_stage_mismatch`
    - `task_not_closable`
    - `task_out_of_team_scope`
    - `task_status_not_actionable`
    - `warehouse_stage_mismatch`

### Performance sample (latest)
- Latest sample from acceptance run (`n=8` each):
  - `GET /v1/tasks`: avg `10.99ms`, p50 `9.79`, p95 `13.84`
  - `GET /v1/tasks/{id}`: avg `8.73ms`, p50 `8.58`, p95 `9.67`
  - `POST /v1/tasks`: avg `256.40ms`, p50 `235.97`, p95 `334.13`
  - `POST /v1/tasks/{id}/assign`: avg `12.53ms`, p50 `11.54`, p95 `15.16`
  - `POST /v1/tasks/reference-upload`: avg `63.24ms`, p50 `63.48`, p95 `65.59`
  - `GET /v1/org/options`: avg `5.93ms`, p50 `5.83`, p95 `6.40`
  - `GET /v1/roles`: avg `5.77ms`, p50 `5.65`, p95 `6.25`
  - `GET /v1/users`: avg `7.94ms`, p50 `7.97`, p95 `8.44`
- This round preserved runtime stability under repeated sampling and full-chain action execution.

### Data/env notes
- This iteration did **not** perform another destructive reset.
- New acceptance entities were created and kept:
  - users: ids `141..152`
  - tasks: ids `301..315`
  - uploads:
    - reference asset `9b83eeb8-2553-4b43-8003-52cc1dfd8611`
    - design delivery session `2270604d-3e00-403a-a6f3-9e9943af4c01` on task `311`

## 2026-04-02 destructive test reset keep-admin baseline (latest truth source)
- This section is the latest truth source for the high-risk reset that returns live `v0.8` to a clean re-test baseline while preserving admin and system foundations.

### Reset type and boundary
- This round is a **destructive test reset** (data cleanup), not a schema rebuild.
- Preserved (explicitly not deleted):
  - Admin/SuperAdmin login chain:
    - kept users: `admin`, `testuser_fix`, `candidate_test`, `test_01`
    - kept their `user_roles`
  - Base org/role/config surfaces:
    - `/v1/org/options` source config
    - `/v1/roles` role catalog path
    - base config tables such as `categories`, `cost_rules`, `code_rules`, `rule_templates`
  - Engineering/runtime skeleton:
    - `/root/ecommerce_ai/releases/v0.8` binaries
    - deploy scripts
    - migration files
    - repo/docs/handover files

### Backup first (completed before delete)
- Server backup dir:
  - `/root/ecommerce_ai/backups/20260402T022844Z_pre_test_reset_keep_admin`
- NAS backup dir:
  - `/volume1/homes/yongbo/asset-upload-service/backups/20260402T022844Z_pre_test_reset_keep_admin`
- Backup artifacts include:
  - full DB dump: `full_db.sql` (~1.1G)
  - key tables dump: `key_tables.sql` (~8.4M)
  - pre-reset service/log/tmp snapshots
  - reset SQL + SQL result + post-reset counts/health/users/API verification files
  - NAS pre-reset directory/file snapshot + `upload.db.pre`

### DB cleanup scope (executed)
- Cleared task/business chains:
  - `tasks`, `task_details`, `task_sku_items`
  - `task_event_logs`, `task_event_sequences`
  - `task_assets`, `design_assets`
  - `procurement_records`, `procurement_record_items`
  - `audit_records`, `audit_handovers`, `warehouse_receipts`, `outsource_orders`
  - `cost_override_events`, `cost_override_event_sequences`, `cost_override_reviews`, `cost_override_finance_flags`
- Cleared asset/upload/meta:
  - `upload_requests`, `asset_storage_refs`
- Cleared export/integration/log traces:
  - `export_jobs`, `export_job_events`, `export_job_event_sequences`, `export_job_attempts`, `export_job_dispatches`
  - `integration_call_logs`, `integration_call_executions`
  - `permission_logs`, `server_logs`, `erp_sync_runs`
  - optional live runtime log table: `erp_sync_log` (present on this environment and cleared)
  - `distribution_jobs`, `job_attempts`, `event_logs`, `sku_sequences`, `workbench_preferences`
- Auth/session cleanup:
  - `user_sessions` cleared during reset
  - non-admin test users removed; Admin/SuperAdmin users retained by SQL guard

### File/cache/log cleanup scope (executed)
- Server:
  - cleared `/root/ecommerce_ai/logs/*` and `/root/ecommerce_ai/tmp/*`
  - truncated `/root/ecommerce_ai/server.log`
- NAS upload service data:
  - cleared task/upload test objects under:
    - `/volume1/docker/asset-upload/data/uploads/tasks/*`
    - `/volume1/docker/asset-upload/data/uploads/nas/design-assets/*`
    - `/volume1/docker/asset-upload/data/uploads/nas/file/*`
    - `/volume1/docker/asset-upload/data/uploads/.sessions/*`
  - cleared upload metadata rows:
    - sqlite `upload_sessions = 0`
    - sqlite `file_meta = 0`
  - kept upload-service root and directory skeleton
- Local workspace:
  - cleared local cache/tmp artifacts in:
    - `.gocache`, `.gomodcache`, `.gotmp`, `.tmp`, `.tmp-go`, `.tmp_go`, `.tmp_gotest`, `tmp`
  - removed top-level temp files matching `tmp_*` / `.tmp_*`
  - did not delete source, docs, migrations, deploy/release artifacts

### Post-reset verification
- Service health:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
- Admin and base endpoints:
  - `POST /v1/auth/login` (admin) succeeded
  - `GET /v1/auth/me` succeeded
  - `GET /v1/org/options` succeeded
  - `GET /v1/roles` succeeded
- Business data reset checks:
  - `GET /v1/tasks?page=1&page_size=20` returned empty list (`total=0`)
  - `GET /v1/operation-logs?page=1&page_size=20` returned empty list
  - `GET /v1/export-jobs?page=1&page_size=20` returned empty list
  - `GET /v1/integration/call-logs?page=1&page_size=20` returned empty list
  - DB post-reset core counts confirm task/asset/procurement/export/integration tables are `0`
- Logs/cache check:
  - server `logs` cleaned, then only fresh restart logs reappeared
  - NAS task/upload test objects are `0` after cleanup

### Residual conservative keeps
- Intentionally preserved:
  - base config/rule data (`categories`, `cost_rules`, `code_rules`, `rule_templates`, etc.)
  - Admin/SuperAdmin user set (4 accounts as conservative safety keep)
- Runtime verification itself generated fresh auth/session/permission traces after reset.
  - This is expected post-reset new activity, not historical residue.

## 2026-04-02 task detail reference/design image contract investigation (latest truth source)
- This section is the latest truth source for task-detail image return behavior.

### Final conclusion
- `GET /v1/tasks/{id}` formal image fields are:
  - references: `reference_file_refs`
  - design asset roots: `design_assets`
  - design versions: `asset_versions`
- Legacy arrays/fields are not canonical truth:
  - `task_details.reference_images_json` is not the formal new-create write target
  - `/v1/assets/*` is not the task-detail canonical projection surface (`/v1/assets/files/*` is only a read proxy path)

### Why old arrays are often empty
- New create flow rejects `reference_images` and uses `reference_file_refs`.
- Create persistence intentionally keeps:
  - `task_details.reference_images_json = []`
  - `task_details.reference_file_refs_json = <formal refs>`
- Therefore "legacy array empty" for new tasks is expected compatibility behavior, not a detail-image regression.

### Code-path confirmation
- `GET /v1/tasks/{id}`:
  - handler: `transport/handler/task.go` -> `TaskHandler.GetByID`
  - service: `service/task_service.go` -> `loadTaskReadModel` / `enrichTaskReadModelDetail`
  - references:
    - primary source: `task_details.reference_file_refs_json`
    - compatibility fallback only: `task_details.reference_images_json`
  - design assets/versions:
    - `service/task_design_asset_read_model.go` delegates to asset-center hydrator
    - `service/task_asset_center_service.go` + `service/task_asset_center_read_model.go`
    - data source is upload-complete persisted `design_assets` + `task_assets`

### Regression and contract checks completed
- Tests updated:
  - `service/reference_images_test.go`
    - create with valid `reference_file_refs` now also asserts `GetByID().reference_file_refs` returns expected refs while legacy `reference_images_json` stays `[]`
    - confirms formal-over-legacy precedence and legacy fallback boundary
- Required local checks passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`

### Runtime/publish
- No runtime code changes in this round (tests + docs + openapi clarification only).
- No deploy/publish required.

## 2026-04-01 task action org gating for audit warehouse close (latest truth source)
- This section is the latest truth source for the 2026-04-01 MAIN task-action minimum org gating closure on live `v0.8`.

### Scope completed in this round
- Extended the existing canonical ownership + minimum org visibility + assign/reassign authorizer line into these live task actions:
  - audit:
    - `POST /v1/tasks/:id/audit/claim`
    - `POST /v1/tasks/:id/audit/approve`
    - `POST /v1/tasks/:id/audit/reject`
    - `POST /v1/tasks/:id/audit/transfer`
    - `POST /v1/tasks/:id/audit/handover`
    - `POST /v1/tasks/:id/audit/takeover`
  - warehouse:
    - `POST /v1/tasks/:id/warehouse/receive`
    - `POST /v1/tasks/:id/warehouse/reject`
    - `POST /v1/tasks/:id/warehouse/complete`
  - close:
    - `POST /v1/tasks/:id/close`
- The unified action layer now combines:
  - required business/management role
  - canonical ownership scope
  - stage/status gate
  - current-handler gate where the action semantics require it
- Machine-readable permission denials are now emitted from the shared task-action authorizer for these paths, instead of each service scattering its own role/status/handler checks.

### Final rule shape
- `view_all` roles:
  - `Admin`
  - `SuperAdmin`
  - `RoleAdmin`
  - `HRAdmin`
  - may cross department/team scope
  - still may **not** cross illegal workflow status
- department-scoped management:
  - `DepartmentAdmin`
  - `DesignDirector`
  - requires `task.owner_department` match
- team-scoped management:
  - `TeamLead`
  - requires `task.owner_org_team` match
- audit business roles:
  - `Audit_A` only acts on `PendingAuditA`
  - `Audit_B` only acts on `PendingAuditB`
  - non-management actors still need current-handler match for approve/reject/transfer/handover, and claim is limited to unassigned-or-current-handler
- warehouse business roles:
  - `Warehouse` is status-gated to `PendingWarehouseReceive`
  - non-management actors still need current-handler match for reject/complete, and receive is limited to unassigned-or-current-handler
- close:
  - only `PendingClose` is action-authorizable
  - org scope still applies
  - closability readiness remains a second gate after permission

### Explicit route boundary
- There is no current MAIN route for:
  - audit `submit`
  - audit `return`
  - warehouse `reopen`
  - warehouse `return`
  - task `reopen`
  - pending-close confirm
  - reject-close
- Therefore this round did **not** invent those actions or document them as implemented.

### Local verification completed
- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed
- `go test ./repo/mysql` -> passed

### Live rollout notes
- Required repo deploy entrypoint used:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task action org gating for audit warehouse close"`
- Runtime verification passed:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/3532025/exe -> /root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/proc/3532047/exe -> /root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `/proc/3532217/exe -> /root/ecommerce_ai/erp_bridge_sync`
  - active executables were not deleted

### Live acceptance completed
- Audit A:
  - out-of-scope actor `iter102_audit_out_1775029502` (`Audit_A`,`Audit_B`,`TeamLead`, team `运营三组`) on task `163` returned:
    - `403 PERMISSION_DENIED`
    - `deny_code=task_out_of_team_scope`
    - `matched_rule=audit_a_scope_or_handler`
  - in-scope actor `iter102_probe_1775029501` (`Audit_A`,`Audit_B`,`TeamLead`, team `运营一组`) approved task `165`:
    - `POST /v1/tasks/165/audit/approve`
    - stage `A`
    - `next_status=PendingAuditB`
    - `200`
    - follow-up detail showed `task_status=PendingAuditB`
- Audit B:
  - same out-of-scope actor on task `165` returned:
    - `403 PERMISSION_DENIED`
    - `deny_code=task_out_of_team_scope`
    - `matched_rule=audit_b_scope_or_handler`
  - same in-scope actor approved task `165`:
    - stage `B`
    - `next_status=PendingWarehouseReceive`
    - `200`
    - follow-up detail showed `task_status=PendingWarehouseReceive`
- Warehouse:
  - out-of-scope actor `iter102_ops_out_1775029504` (`DepartmentAdmin`,`Warehouse`, department `设计部`) on task `165` returned:
    - `403 PERMISSION_DENIED`
    - `deny_code=task_out_of_department_scope`
    - `matched_rule=warehouse_receive_scope`
  - in-scope actor `iter102_ops_in_1775029503` (`DepartmentAdmin`,`Warehouse`, department `运营部`) received task `165`:
    - `POST /v1/tasks/165/warehouse/receive` -> `201`
    - receipt status `received`
  - same in-scope actor on wrong-stage task `163` returned:
    - `403 PERMISSION_DENIED`
    - `deny_code=warehouse_stage_mismatch`
  - same in-scope actor completed task `165`:
    - `POST /v1/tasks/165/warehouse/complete` -> `200`
    - follow-up detail showed `task_status=PendingClose`
- Close:
  - out-of-scope actor `iter102_ops_out_1775029504` on closable task `137` returned:
    - `403 PERMISSION_DENIED`
    - `deny_code=task_out_of_department_scope`
  - in-scope actor `iter102_ops_in_1775029503` on wrong-status task `163` returned:
    - `403 PERMISSION_DENIED`
    - `deny_code=task_not_closable`
  - same in-scope actor closed task `137`:
    - `POST /v1/tasks/137/close` -> `200`
    - follow-up detail showed `task_status=Completed`
  - supplemental note:
    - a plain `Member` actor hit route-role denial on `/close` before the shared authorizer, so that sample did not carry authorizer `deny_code`
- Regression:
  - `/assign` still kept the prior split semantics:
    - task `172` reassign `41 -> 42 -> 41` both returned `200`, status stayed `InProgress`
    - task `163` `POST /assign` returned `403 PERMISSION_DENIED` with `deny_code=task_not_reassignable`
  - `/v1/tasks` list/detail still returned canonical ownership:
    - task `163` list/detail both exposed `owner_team=内贸运营组`
    - `owner_department=运营部`
    - `owner_org_team=运营一组`
- Temporary live verification users `52-56` were disabled after acceptance.

### Remaining boundary
- This is still minimum task-action gating, not full ABAC.
- Legacy `owner_team` is still present for compatibility and has not been retired.
- Historical tasks with empty canonical ownership fields still exist; those tasks fall back to the minimum currently available scope facts.
- Close readiness is still separate from permission:
  - `PendingClose` may pass authorization and still fail business readiness if workflow closability is false.
- Unified authorizer coverage is still limited to the actions listed above; absent routes remain absent.

## 2026-04-01 task-create reference small escaped-storage-key closure (latest truth source)
- This section is the latest truth source for the 2026-04-01 MAIN reference-small probe/proxy repair on live `v0.8`.

### Scope completed in this round
- Kept the archived upload architecture unchanged:
  - `reference = small`
  - `delivery/source/preview = multipart`
  - small reference still uses `/upload/files`
  - small reference still does **not** call NAS `complete`
  - success still requires stored size/hash verification
- Repaired the stable probe failure reproduced by `trace_id=cba34f59-5f24-4280-9fea-c2b7e2d1eeee`.
- Extended the same fix to the public asset-file proxy and returned HTTP file URLs so the uploaded reference stays readable after success.

### Root cause
- Live/server-side investigation proved MAIN was already selecting the correct server-to-server host:
  - `UPLOAD_SERVICE_BASE_URL=http://100.111.214.38:8089`
  - browser multipart host stayed `http://192.168.0.125:8089`
- The stable defect was not a browser-host/probe-host mix-up and not a small-path `complete` regression.
- The failing trace showed the actual fault:
  - `/upload/files` returned a valid `storage_key`
  - the `storage_key` contained raw `%` plus UTF-8 characters in the filename
  - MAIN then tried to build probe path `"/files/{storage_key}"` via `url.Parse`
  - probe never sent an HTTP request because local URL parsing failed first with `invalid URL escape "% \xf0"`
- Therefore the true stable failure was path escaping, specifically unescaped `%`/UTF-8 characters in `storage_key` values reused as HTTP path segments.
- The earlier short probe-visibility race remains a separate storage-side behavior, but it was not the root cause of the user-reported stable repro.

### Current runtime behavior
- MAIN now escapes storage keys by path segment before building:
  - server-to-server probe URLs
  - `/v1/assets/files/*` upstream proxy URLs
  - returned `public_url` / `lan_url` / `tailscale_url`
- The bounded small-probe retry remains in place:
  - default `3` attempts
  - short linear backoff from `200ms`
  - retries only transient probe failures and transient empty stored-object reads
- MAIN still fails immediately for non-transient bad states:
  - empty `storage_key`
  - incomplete probe metadata
  - stored size mismatch
  - stored hash mismatch

### Explicit boundaries kept
- Do not describe this as a change to the small-vs-multipart split.
- Do not describe this as a return to NAS `complete` for small reference.
- Do not describe this as "probe failures are ignored".
- Do not describe this as a multipart contract change:
  - browser multipart still depends on `remote.headers`
  - `remote.headers` still needs `X-Internal-Token`

### Local verification completed
- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed

### Live rollout notes
- Required repo deploy entrypoint used:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 fix escaped storage-key probe and proxy urls"`
- Runtime verification after overwrite passed:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/3503354/exe -> /root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/proc/3503402/exe -> /root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `/proc/3503547/exe -> /root/ecommerce_ai/erp_bridge_sync`
  - active executables were not deleted

### Live acceptance completed
- Original failing evidence stayed reproducible in historical live logs:
  - `trace_id=cba34f59-5f24-4280-9fea-c2b7e2d1eeee`
  - `/upload/files` returned a real `storage_key`
  - probe attempts `1/3`, `2/3`, `3/3` all failed before request dispatch with `invalid upload service path ... invalid URL escape "% \xf0"`
- After overwrite, `POST /v1/tasks/reference-upload` with filename `💚97% 能量充满啦.png` returned `201`:
  - trace `502f127d-02de-4c51-8446-99899df5530b`
  - `asset_id=569c7113-bde2-4ced-a20b-964336ac8b05`
  - `upload_request_id=43b241d6-dd05-42ea-ae4f-4f05433afd6f`
- The repaired logs on live `v0.8` showed:
  - correct `storage_key`
  - probe host `100.111.214.38:8089`
  - `probe_status=200`
  - matching stored size/hash
  - escaped `public_url` / `lan_url` / `tailscale_url`
- Public proxy read also passed on the escaped returned URL:
  - `GET /v1/assets/files/.../%F0%9F%92%9A97%25...png` -> `200`
  - body bytes `137253`
- Using that returned `reference_file_refs`, `POST /v1/tasks` created a live `new_product_development` task successfully:
  - `task_id=169`
  - `task_no=RW-20260401-A-000164`
- Regression reads also passed:
  - `GET /v1/tasks?page=1&page_size=5` -> `200`
  - existing batch task `167` remained readable with `is_batch_task=true`
  - live responses still exposed `owner_team`, `owner_department`, and `owner_org_team`

### Open boundary after this round
- MAIN now correctly escapes storage keys and still keeps the bounded retry added for short visibility races.
- The storage-side visibility race remains a separate NAS/upload-service concern, but the user-reported stable repro was closed by the URL/path fix.

## 2026-04-01 task-create reference small probe retry closure (latest truth source)
- This section is the latest truth source for the 2026-04-01 MAIN reference-small probe repair on live `v0.8`.

### Scope completed in this round
- Kept the already archived upload architecture unchanged:
  - `reference = small`
  - `delivery/source/preview = multipart`
  - small reference still uses `/upload/files`
  - small reference still does **not** call NAS `complete`
  - success still requires stored size/hash verification
- Repaired the reference-small upload after-upload probe failure lane.
- Added the minimum bounded observability needed to distinguish:
  - probe request failure
  - transient empty stored object
  - probe metadata invalid
  - stored size mismatch
  - stored hash mismatch

### Root cause
- The live/server-side investigation confirmed that MAIN was already selecting the correct server-to-server host:
  - `UPLOAD_SERVICE_BASE_URL=http://100.111.214.38:8089`
  - browser multipart host stayed `http://192.168.0.125:8089`
- The defect was not a browser-host/probe-host mix-up and not a small-path `complete` regression.
- Archived live traces proved the real failure mode:
  - upload result returned a valid `storage_key`
  - MAIN probed the correct `http://100.111.214.38:8089/files/{storage_key}` URL
  - but some probes still saw a transient empty object (`200` with `bytes_read=0` and `content_length=0`) immediately after `/upload/files` returned
- That made the failure a short NAS/upload-service visibility race on the newly written object.

### Current runtime behavior
- Task-create reference small uploads now use a narrow probe retry window before final verification:
  - default `3` attempts
  - short linear backoff from `200ms`
  - retries only for transient probe request failures and transient empty stored-object reads
- MAIN still fails immediately for non-transient bad states:
  - empty `storage_key`
  - incomplete probe metadata
  - stored size mismatch
  - stored hash mismatch
- New probe logs now include:
  - `trace_id`
  - `upload_mode=reference_small`
  - `upload_service_base_url`
  - `selected_probe_host`
  - `storage_key`
  - `filename`
  - `expected_size`
  - `expected_sha256`
  - per-attempt `probe_status` / `probe_error` / `retry_reason`

### Explicit boundaries kept
- Do not describe this as a change to the small-vs-multipart split.
- Do not describe this as a return to NAS `complete` for small reference.
- Do not describe this as “probe failures are ignored”.
- Do not describe this as a multipart contract change:
  - browser multipart still depends on `remote.headers`
  - `remote.headers` still needs `X-Internal-Token`

### Local verification completed
- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed

### Live rollout notes
- Required repo deploy entrypoint used:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task-create reference small probe retry and diagnostics"`
- The first packaging attempt failed because repo-local `deploy/*.sh` still had CRLF line endings on this Windows control node.
- After normalizing `deploy/*.sh` to LF, the same deploy entrypoint completed successfully onto existing `v0.8`.
- Runtime verification after overwrite passed:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/3484605/exe -> /root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/proc/3484628/exe -> /root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `/proc/3484806/exe -> /root/ecommerce_ai/erp_bridge_sync`
  - active executables were not deleted

### Live acceptance completed
- `POST /v1/tasks/reference-upload` returned `201` after overwrite:
  - trace `14d2ffd0-c52a-452d-a0d5-e3f87096eabf`
  - `asset_id=5d749472-deb2-4f89-8660-eb7eeef0c227`
- The repaired logs on live `v0.8` showed:
  - correct `storage_key`
  - probe host `100.111.214.38:8089`
  - probe attempt `1/3`
  - matching stored size/hash
- Using that returned `reference_file_refs`, `POST /v1/tasks` created a live `new_product_development` task successfully:
  - `task_id=168`
  - `task_no=RW-20260401-A-000163`
- Regression reads also passed:
  - `GET /v1/tasks?page=1&page_size=5` -> `200`
  - existing batch task `167` remained readable with `is_batch_task=true`, `batch_item_count=2`
  - live responses still exposed `owner_team`, `owner_department`, and `owner_org_team`
- A live forced probe-failure sample was not executed:
  - reproducing the race safely would require intentionally destabilizing NAS visibility or corrupting stored bytes on the shared environment

### Open boundary after this round
- MAIN now masks the observed short race window with a bounded retry, but the underlying storage-side visibility race still belongs to the NAS/upload-service side.
- `original_product_development` defer-create was not re-run live in this round.

## 2026-03-31 task action minimum org-scoped authorization closure (latest truth source)
- This section is the latest truth source for the 2026-03-31 MAIN task-action organization round.

### Scope completed in this round
- Kept the already-landed account org / role minimum closure unchanged:
  - `/v1/org/options`
  - `/v1/auth/me`
  - `/v1/users`
  - `/v1/roles`
  - `frontend_access`
- Kept the already-landed task ownership / visibility baseline unchanged:
  - legacy `tasks.owner_team`
  - canonical `tasks.owner_department`
  - canonical `tasks.owner_org_team`
  - minimal list/detail visibility over canonical ownership
  - batch SKU create behavior
  - original defer create behavior
- Added a minimum task action authorization layer on top of the canonical ownership model.

### Current runtime behavior
- Task action authorization is now evaluated as `role gate + minimum org scope + workflow/handler/status checks`.
- The shared task action authorizer now covers these runtime actions:
  - `create`
  - `read_detail`
  - `update_business_info`
  - `assign`
  - `submit_design`
  - task asset upload-session `create` / `complete` / `cancel`
  - audit `claim` / `approve` / `reject` / `transfer` / `handover` / `takeover`
  - warehouse `prepare` / `receive` / `reject` / `complete`
  - `close`
  - procurement `update` / `advance`
- Scope evaluation is intentionally minimal and explicit:
  - `Admin` / `SuperAdmin` / `RoleAdmin` / `HRAdmin` use `view_all`
  - `DepartmentAdmin` / `DesignDirector` are limited by canonical `owner_department`
  - `TeamLead` is limited by canonical `owner_org_team`
  - node roles such as `Designer`, `Audit_A`, `Audit_B`, `Warehouse`, `Ops` still require matching workflow role semantics and, where configured, current handler / designer / creator linkage
- Denied actions still return `PERMISSION_DENIED`, and now include machine-readable `error.details` such as:
  - `deny_code=missing_required_role`
  - `deny_code=task_out_of_department_scope`
  - `deny_code=task_out_of_team_scope`
  - `deny_code=task_not_assigned_to_actor`
  - `deny_code=task_status_not_actionable`
- Authorization decisions now emit bounded auth logs with:
  - `actor_id`
  - `actor_roles`
  - `action`
  - `task_id`
  - `owner_department`
  - `owner_org_team`
  - `scope_source`
  - allow/deny outcome

### Explicit boundaries kept
- This is not a full ABAC engine.
- This is not a generic policy DSL or org-tree inheritance platform.
- Legacy `owner_team` is still retained for compatibility and is not retired in this round.
- Ordinary members still do not get a complete row-level org policy:
  - they rely on the existing minimal self/handler/designer-related behavior
- Not every task-adjacent endpoint is fully unified under the new task action layer yet:
  - batch remind remains on the prior route/business gate path
  - audit handover listing remains read-oriented
  - mock / compatibility-only asset routes are not the primary authorization truth source

### Local verification completed
- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed
- `go test ./repo/mysql` was additionally executed for repository regressions in this round

### Live rollout notes
- This round was overwrite-published onto existing `v0.8` using:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task action minimum org authorization"`
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task action minimum org authorization fix"`
- The first live action-verification pass exposed a real authorization defect:
  - `TeamLead(运营三组)` could still assign an `owner_org_team=运营一组` task because generic same-department scope was being accepted
  - non-management workflow-role denial ordering also preferred org mismatch over handler mismatch too early
- After the shared authorizer fix and second overwrite publish, live verification passed:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/<pid>/exe` resolved to the expected `v0.8` binaries
  - detail read allow/deny passed
  - department-scoped write allow/deny passed
  - team-scoped assign allow/deny passed
  - handler/self-related submit-design allow/deny passed

## 2026-03-31 task canonical org ownership and minimum task visibility closure (latest truth source)
- This section is the latest truth source for the 2026-03-31 MAIN task/org formal connection round on live `v0.8`.

### Scope completed in this round
- Keep the already-live account org / role minimum closure unchanged:
  - `/v1/org/options`
  - `/v1/auth/me`
  - `/v1/users`
  - `/v1/users/{id}`
  - `/v1/roles`
  - `/v1/permission-logs`
  - `frontend_access`
- Keep the already-live task fixes unchanged:
  - batch SKU create for `new_product_development`
  - batch SKU create for `purchase_task`
  - `original_product_development` still rejects batch SKU create
  - create/detail/list still expose batch read fields
  - create-time `owner_team` compatibility bridge still accepts mapped org-team values such as `运营三组`
- Add formal task-side canonical org ownership without deleting legacy task ownership:
  - `tasks.owner_team` remains the legacy compatibility field
  - `tasks.owner_department` is now the canonical task owner department
  - `tasks.owner_org_team` is now the canonical task owner org team

### Current runtime behavior
- New task create now writes both ownership layers:
  - legacy `owner_team`
  - canonical `owner_department`
  - canonical `owner_org_team` when deterministic
- Read models now expose all three fields on:
  - `GET /v1/tasks`
  - `GET /v1/tasks/{id}`
  - `GET /v1/tasks/{id}/detail` through nested `task`
- Canonical ownership filtering is now available on `GET /v1/tasks`:
  - `owner_department`
  - `owner_org_team`
- Minimum list/detail visibility is now connected to the org/role line through canonical task ownership:
  - view-all roles such as `Admin`, `SuperAdmin`, `RoleAdmin`, `HRAdmin` still see all tasks
  - `DepartmentAdmin` and `DesignDirector` are filtered by task `owner_department`
  - `TeamLead` is filtered by task `owner_org_team`
  - ordinary members still rely on the pre-existing self-related visibility checks

### Explicit boundaries kept
- This round does not replace `/v1/org/options` or `frontend_access`.
- This round does not introduce a full org tree platform.
- This round does not introduce a full ABAC or row-level permission engine.
- This round does not fully retire legacy task `owner_team`.
- Historical tasks are not fully backfilled:
  - safe minimal migration backfilled only deterministic department-level cases
  - ambiguous historical org-team ownership remains empty in canonical columns

### Storage and migration facts
- New migration: `047_v7_task_canonical_org_ownership.sql`
- Added columns:
  - `tasks.owner_department`
  - `tasks.owner_org_team`
- Added indexes:
  - `idx_tasks_owner_department`
  - `idx_tasks_owner_org_team`
- Backfill policy is intentionally narrow:
  - only deterministic legacy-team -> department mappings are backfilled
  - no ambiguous legacy-team -> org-team mass rewrite is performed

### Live rollout notes
- First live verification after the initial overwrite publish exposed a real schema gap:
  - `GET /v1/tasks` returned `500`
  - live DB had not yet applied `047_v7_task_canonical_org_ownership.sql`
- Backup was created before the live schema mutation:
  - `/root/ecommerce_ai/backups/20260331T033855Z_task_canonical_org_047`
- Live migration was then applied from:
  - `/root/ecommerce_ai/releases/v0.8/db/migrations/047_v7_task_canonical_org_ownership.sql`
- A second overwrite publish was required after that:
  - the first runtime cut still omitted empty `owner_department` / `owner_org_team` JSON fields because of `omitempty`
  - the final live binary now returns those fields stably on task list/detail payloads
- Final live verification passed:
  - `8080`, `8081`, `8082` health all returned `200`
  - original/new/purchase create with `运营三组` all returned canonical ownership
  - `/v1/tasks` list/detail returned canonical ownership fields
  - minimal department/team visibility filtering behaved as implemented

## 2026-03-31 MAIN batch-SKU overwrite publish, live schema closure, and verified `v0.8` effectiveness (latest truth source)
- This section is the latest truth source for the 2026-03-31 MAIN self-test + overwrite publish round on the existing live release line `v0.8`.

### Delivery boundary
- `Design Target`:
  - overwrite-publish current MAIN backend onto existing `v0.8`
  - keep production entrypoint locked to `cmd/server/main.go`
  - validate batch-SKU task create mainline live on `8080`
  - keep `8082` binary unchanged unless an explicit replacement is actually required
- `Code Implemented`:
  - repository batch-SKU create chain was already present in code before this round:
    - `db/migrations/046_v7_task_batch_sku_items.sql`
    - task create / read-model / OpenAPI batch-SKU fields
  - this round additionally normalized `deploy/*.sh` line endings to LF so the existing package/deploy scripts execute correctly from Linux bash during packaging and remote runtime operations
- `Server Verified`:
  - required local tests/builds passed
  - local package artifact for `v0.8` was built from `./cmd/server`
  - server overwrite publish completed onto `/root/ecommerce_ai/releases/v0.8`
  - live DB schema gap was detected and closed by applying migration `046`
- `Live Effective`:
  - yes, after overwrite publish plus live schema fix
  - 8080/8081/8082 all healthy
  - batch-SKU create/read behavior is live on `v0.8`

### Local verification executed before publish
- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed
- additional targeted regression:
  - `go test ./service -run "TaskPRD|TaskBatch|Create"` -> passed
  - `go test ./transport/handler -run "TestTaskHandlerCreateParsesBatchItems|TestTaskHandlerCreateBatchErrorIncludesViolations|TestTaskHandlerCreateBatchResponseIncludesSKUItems"` -> passed
  - `go test ./repo/mysql -run "Test.*Task.*"` -> passed
- migration/OpenAPI alignment confirmed locally:
  - migration file exists: `db/migrations/046_v7_task_batch_sku_items.sql`
  - `docs/api/openapi.yaml` already contained:
    - `is_batch_task`
    - `batch_item_count`
    - `batch_mode`
    - `primary_sku_code`
    - `sku_generation_status`
    - `sku_items`

### Package and overwrite publish result
- Local package entrypoint used:
  - `bash ./deploy/package-local.sh --version v0.8 --skip-tests`
- Local package result:
  - artifact: `dist/ecommerce-ai-v0.8-linux-amd64.tar.gz`
  - artifact SHA-256: `4baea8036dcf3e8f7cb4c41ac18416946210c002a383e0f3011323c476bae845`
  - `PACKAGE_INFO.json` resolved entrypoint remained `./cmd/server`
- Managed remote cutover result:
  - target release dir remained `/root/ecommerce_ai/releases/v0.8`
  - replaced binaries:
    - `8080` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
    - `8081` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - not replaced:
    - `8082` binary remained `/root/ecommerce_ai/erp_bridge_sync`
- Important control-plane note:
  - the repository `deploy.sh` wrapper started correctly but its Windows/WSL remote phase was unstable on this control node
  - this round therefore still used repository-managed assets only:
    - local `deploy/package-local.sh`
    - remote packaged `deploy/remote-deploy.sh`
    - remote `deploy/verify-runtime.sh`
  - no new release line was created; live line remained overwrite-published `v0.8`

### Live schema gap discovered during first verification
- First live acceptance against the freshly overwritten binary exposed a real DB/schema mismatch:
  - single new-product create returned `500`
  - batch new-product create returned `500`
  - batch purchase-task create returned `500`
  - original-product batch reject already returned the expected `400`
- Live log evidence on `/root/ecommerce_ai/logs/ecommerce-api-20260331T012125Z.log` showed:
  - `create task: insert task: Error 1054 (42S22): Unknown column 'is_batch_task' in 'field list'`
- Conclusion:
  - the overwrite publish itself succeeded
  - live DB had not yet applied `046_v7_task_batch_sku_items.sql`

### Live schema fix applied in this round
- Backup created before schema mutation:
  - `/root/ecommerce_ai/backups/20260331T012734Z_task_batch_schema_046/tasks_procurement_before.sql`
- Applied live migration:
  - `/root/ecommerce_ai/releases/v0.8/db/migrations/046_v7_task_batch_sku_items.sql`
- This closed the missing live schema required by the new `tasks` columns and by:
  - `task_sku_items`
  - `procurement_record_items`

### Current live runtime truth after overwrite + migration
- Health:
  - `http://127.0.0.1:8080/health` -> `200`
  - `http://127.0.0.1:8081/health` -> `200`
  - `http://127.0.0.1:8082/health` -> `200`
- Runtime pointers:
  - 8080 PID `3186035` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - 8081 PID `3186057` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - 8082 PID `3186156` -> `/root/ecommerce_ai/erp_bridge_sync`
- Deleted-binary check:
  - 8080 `/proc/3186035/exe` is not deleted
  - 8081 `/proc/3186057/exe` is not deleted
  - 8082 `/proc/3186156/exe` is not deleted
- SHA-256:
  - `ecommerce-api` -> `16dcbcc6bcc53c97d6a3abad138c44939171f06126c791e38087bbebd9f2a721`
  - `erp_bridge` -> `16dcbcc6bcc53c97d6a3abad138c44939171f06126c791e38087bbebd9f2a721`
  - unchanged `erp_bridge_sync` -> `2264a80cc8318d08828fcf29a6f7ddaa3ea69804ab13dc5b71e293d97afc82b8`
- 8082 recovery truth:
  - `verify-runtime.sh --auto-recover-8082` was triggered
  - sync was restarted successfully after the publish path
  - binary remained unchanged

### Live batch-task acceptance evidence
- Real bearer session obtained through `POST /v1/auth/login` -> `200`
- `GET /v1/tasks?page=1&page_size=5` -> `200` after migration `046` was applied
- Live-safe task-create verification:
  - single `new_product_development` create -> `201`
    - task id `147`
    - `is_batch_task=false`
    - `batch_item_count=1`
    - `batch_mode=single`
    - `primary_sku_code=SKU-000029`
    - `sku_generation_status=completed`
    - `GET /v1/tasks/147` -> `200`
  - batch `new_product_development` create -> `201`
    - task id `148`
    - `is_batch_task=true`
    - `batch_item_count=2`
    - `batch_mode=multi_sku`
    - `primary_sku_code=CDXN31092831A`
    - `sku_generation_status=completed`
    - `GET /v1/tasks/148` -> `200`
    - returned `sku_items[0..1]`
  - batch `purchase_task` create -> `201`
    - task id `149`
    - `is_batch_task=true`
    - `batch_item_count=2`
    - `batch_mode=multi_sku`
    - `primary_sku_code=CDXP31092831A`
    - `sku_generation_status=completed`
    - `GET /v1/tasks/149` -> `200`
    - returned `sku_items[0..1]`
  - batch `original_product_development` reject -> `400 INVALID_REQUEST`
    - violation `batch_not_supported_for_task_type` on:
      - `batch_sku_mode`
      - `batch_items`

### Current remaining risk / follow-up
- The repo-local `deploy/deploy.sh` remote wrapper is still unstable on this Windows control node; the actual live overwrite was completed by using the packaged repository `remote-deploy.sh` directly on server after local packaging.
- This round created explicit live verification tasks:
  - `147`
  - `148`
  - `149`
- `docs/api/openapi.yaml` already matched the batch-SKU contract before this round, so no OpenAPI content change was required here.

## 2026-03-23 non-explicit historical normalization audit and wider regression closure (latest truth source)
- This section is the latest truth source for the post-`ITERATION_092` follow-up round.
- This round was audit-only and regression-only:
  - no upload-chain code change
  - no task-detail aggregate-chain change
  - no org/permission-chain change
  - no second-round bulk delete
- Evidence source for every conclusion in this section:
  - live DB read on host `223.4.249.11` against MySQL `jst_erp`
  - live session-backed regression on `http://127.0.0.1:8080`

### What was re-audited
- Historical task and related-data audit was widened from explicit `test/demo/accept/case` markers to non-explicit suspicious residue.
- Focus areas rechecked:
  - `existing_product` weak consistency
  - old ERP snapshot residue
  - old org-field residue
  - old enum / old status residue
  - weak relation anomalies
  - old read-model compatibility residue

### Negative findings confirmed by DB
- Task/product weak consistency remained closed after `ITERATION_092`:
  - `existing_product` + `product_id IS NULL` = `0`
  - `tasks.product_id -> products.id` missing FK target = `0`
  - `existing_product` exact `tasks.sku_code != products.sku_code` mismatch = `0`
  - `new_product` task carrying unexpected `product_id` = `0`
- Audited task JSON fields remained valid:
  - `product_selection_snapshot_json` invalid JSON = `0`
  - `matched_mapping_rule_json` invalid JSON = `0`
  - `reference_file_refs_json` invalid JSON = `0`
- Weak relation checks remained clean on the core task graph:
  - missing `creator/designer/current_handler` user refs = `0`
  - missing `task_details` = `0`
  - orphan `task_assets` = `0`
  - orphan `design_assets` = `0`
  - `asset_storage_refs(owner_type=task_asset)` missing `task_assets.owner_id` = `0`
- Current task enums remained on the current mainline:
  - `source_mode`: only `existing_product`, `new_product`
  - `task_type`: only `original_product_development`, `new_product_development`, `purchase_task`
  - `task_status`: only `PendingAssign`, `InProgress`, `Completed`, `PendingAuditA`, `PendingClose`
  - `filing_status`: only `pending_filing`, `filed`, `not_filed`

### Candidate boundary after the widened audit
- `保留并归一`:
  - `asset_storage_refs` has `9` deterministic normalization candidates on tasks `144` and `145`
  - exact pattern:
    - `owner_type=task_asset`
    - `asset_storage_refs.asset_id = task_assets.id`
    - but the correct current design-asset target is `task_assets.asset_id`
  - affected `asset_storage_refs.ref_id`:
    - `9a2f3635-94eb-4f05-bb51-0397646b7ad9`
    - `a21568b5-c098-4bfb-acbc-1ed2d1968379`
    - `93f6557e-7a89-47b4-83c3-c3835384e59b`
    - `70ea67f6-9aad-4d1d-99c3-65eb381c9950`
    - `1c54dc1d-3ee2-4067-8f58-8132b0d4444d`
    - `088c9bdd-4843-4fa0-ae0d-4912ae38b541`
    - `ff1a4601-05bf-465a-b246-fdae510fc757`
    - `ffff03bf-2d48-4eca-a21f-57803dbc250f`
    - `9f841583-d77b-404c-bfaa-cd47623ebbd5`
  - deterministic mapping already proven in DB:
    - wrong `asset_id 44~52`
    - correct `task_assets.asset_id 35~43`
  - this is a normalization candidate, not a delete candidate
- `需人工确认`:
  - task IDs:
    - `95,96,97,106,112,113,114,115,116,117,118,119,120,122,124,125,128,130,131,132,134,135,137,138,139,140,142,144,145`
  - common reasons:
    - business-like task content but suspicious verification lineage
    - linked `reference_file_refs`
    - linked `task_assets` / `design_assets`
    - active workflow state (`InProgress` / `PendingClose` / `PendingAuditA`)
    - tied to historical verification actors such as `test_a`, `test_01`, `一流测试`, `ops_remote_0317`, `bb_designer3`
  - these objects are intentionally not auto-deletable in this round
- `明确可删`:
  - task IDs:
    - `47,48,49,58,59,61,62,69,71,72,73,74,75,76,98,99,100,111,121,123,126`
  - supporting evidence from live DB:
    - task content itself shows synthetic verification semantics such as `live verify`, `黑盒V04`, `Verify`, `BRIDGE-REMOTE-CHECK`, `ERP Stub`, `Roleless Verify`, `验收defer路径`, `ERP acceptance`, `reference image small verify`, `测试新品`, `Step87`
    - all currently have `design_asset_count = 0`
    - almost all also have `reference_file_refs = 0` and `task_assets = 0`
    - two blackbox tasks (`58`, `61`) still carry one `task_asset` row each, so any later delete must cascade carefully

### Old org-field audit boundary
- Live `/v1/org/options` currently exposes the new account-org catalog:
  - `7` departments
  - `14` teams
- Live `users` still contain `27` legacy org-field rows outside that new catalog, concentrated in:
  - blank department/team = `4`
  - `人力行政中心 / 人力行政组` = `5`
  - `设计部 / 设计组` = `5`
  - `内贸运营部 / 内贸运营组` = `8`
  - `采购仓储部 / 采购仓储组` = `2`
  - `总经办 / 总经办组` = `3`
- Important boundary:
  - task `owner_team` must **not** be treated the same as account-org `team`
  - repo code still keeps legacy task teams as the `owner_team` compatibility source (`domain.DefaultDepartmentTeams`, `domain.ValidTeam`, `service.validateCreateTaskEntry`)
  - therefore all current `66` tasks still using legacy `owner_team` values is a compatibility fact, not deletion evidence

### Live wider regression result
- Full remaining task-set scan was re-run after the widened audit:
  - `66` live tasks scanned
  - `GET /v1/tasks/{id}` non-`200` = `0`
  - `GET /v1/tasks/{id}/product-info` non-`200` = `0`
  - `GET /v1/tasks/{id}/cost-info` non-`200` = `0`
- Task-list regression widened beyond the prior round:
  - `GET /v1/tasks?page=1&page_size=20` = `200`
  - `GET /v1/tasks?page=2&page_size=20` = `200`
  - `GET /v1/tasks?page=3&page_size=20` = `200`
  - `GET /v1/tasks?page=4&page_size=20` = `200`
  - `GET /v1/tasks?page=1&page_size=20&task_type=original_product_development` = `200`
  - `GET /v1/tasks?page=1&page_size=20&task_type=purchase_task` = `200`
- Org / permission regression also widened and stayed intact:
  - admin session:
    - `GET /v1/org/options` = `200`
    - `GET /v1/roles` = `200`
    - `GET /v1/users?page=1&page_size=20` = `200`
    - `GET /v1/permission-logs?page=1&page_size=5` = `200`
  - `Ops` session:
    - same four management routes = `403`
  - roleless session:
    - same four management routes = `403`

### Current risk assessment and recommended order
- Do not start a second-round bulk delete directly from the candidate list.
- Recommended order if the next round proceeds:
  1. back up DB again
  2. normalize the `9` deterministic `asset_storage_refs` rows
  3. manually confirm the `29` business-like / linked / active suspicious tasks
  4. only then delete the `21` clear-delete tasks with full dependent-row cleanup
  5. re-run the same full `66`-task detail/product-info/cost-info scan plus task-list/org regressions
- This round intentionally stops at boundary confirmation and evidence archival.

## 2026-03-23 historical task 500 and dirty-data cleanup closure (latest truth source)
- This section is the latest truth source for the historical task read-model `500` cleanup round on MAIN `v0.8`.
- Scope fixed for this round:
  - audit all historical task samples that had reported `500` on `GET /v1/tasks/{id}`, `GET /v1/tasks/{id}/product-info`, and `GET /v1/tasks/{id}/cost-info`
  - classify retained historical compatibility data vs. explicit test/demo/acceptance residue
  - back up live DB before any mutation
  - repair retained historical task references
  - clean only explicit marker test tasks
  - re-run live consistency and endpoint verification
- Explicitly out of scope for this round:
  - upload-chain changes
  - org/permission expansion
  - new org tree / ABAC / row-level work

### Historical 500 audit result
- Live current-state scan after the prior backend fixes:
  - all current tasks were re-scanned on:
    - `GET /v1/tasks/{id}`
    - `GET /v1/tasks/{id}/product-info`
    - `GET /v1/tasks/{id}/cost-info`
  - current live result before cleanup already showed `0` active `500`s across the full current task set
- Historical log audit still confirmed the earlier failure clusters:
  - old `GET /v1/tasks/84` and related `product-info/cost-info` `500`
  - old `GET /v1/tasks/136~141/cost-info` repeated `500`
- Database audit conclusion:
  - invalid JSON was not present in the audited task JSON fields
  - `tasks/task_details/task_assets/design_assets` structural orphans were not present before cleanup
  - the main retained historical risk pattern was:
    - `existing_product` tasks with `product_id IS NULL` but a valid historical ERP snapshot still present
  - the main removable residue pattern was:
    - explicit test/demo/acceptance/case tasks identified by task content markers, not merely by who created them

### Backup truth
- Backup was created before any live mutation at:
  - `/root/ecommerce_ai/backups/20260323T120120Z_historical_task_cleanup_v091`
- Backup contents:
  - full DB snapshot: `jst_erp_full_before.sql.gz`
  - key table snapshot: `key_tables_before.sql.gz`
  - candidate boundary export: `candidate_boundaries_before.tsv`

### Keep / fix / delete boundary
- Keep boundary:
  - historical business-like tasks without explicit test/demo markers
  - historical `existing_product` tasks that still need compatibility preservation
- Fix boundary:
  - retained `existing_product` tasks where `products.sku_code = tasks.sku_code` now matches exactly
  - these tasks were repaired by backfilling `tasks.product_id`
- Delete boundary:
  - only explicit marker tasks where `sku_code` or `product_name_snapshot` clearly contained:
    - `accept`
    - `demo`
    - `case`
    - `test`
  - test-account ownership alone was not treated as sufficient deletion evidence

### Live cleanup result
- `tasks` count changed:
  - `86 -> 66`
- Explicit marker test tasks removed:
  - `20`
- Retained historical tasks repaired by `product_id` backfill:
  - `12`
- Post-cleanup consistency checks:
  - `existing_product_missing_product_id = 0`
  - `missing_detail = 0`
  - `orphan_detail = 0`
  - `orphan_task_assets = 0`
  - `orphan_design_assets = 0`
  - `asset_storage_refs(owner_type=task_asset)` also rechecked against `task_assets` and remained `0` orphan

### Live verification result
- After backup + cleanup, the live task set was re-scanned again on:
  - `GET /v1/tasks/{id}`
  - `GET /v1/tasks/{id}/product-info`
  - `GET /v1/tasks/{id}/cost-info`
- Post-cleanup live result:
  - `0` current `500`s across all `66` remaining live tasks
- Retained historical samples were verified live:
  - tasks `106, 114, 137, 139, 142, 144` now return consistent `task.product_id` and `product-info.product_id`
- Deleted explicit test samples were verified live:
  - tasks `51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136` now return `404`

### Current boundary after cleanup
- Historical task read-model `500` closure is now backed by:
  - full task-set scan
  - DB audit
  - backup evidence
  - targeted repair
  - targeted cleanup
  - live regression verification
- The org/permission minimum closure remains frozen and unchanged by this round.
- Upload, multipart, reference-small, and task asset visibility chains were not modified in this round.

## 2026-03-23 account permission and org-management minimum closure (latest truth source)
- This section is the latest truth source for the minimal account/role/department/team closure on MAIN `v0.8`.
- Scope fixed for this round:
  - minimal account permission closure
  - minimal department/team organization closure
  - minimal user/role management closure
  - minimal data-scope expression through `frontend_access`
  - permission-log traceability for key identity mutations
- Explicitly out of scope for this round:
  - full org-tree platform
  - complete ABAC or row-level permission engine
  - visual permission-point platform
  - login/register mainline rewrite

### Current live truth
- Local MAIN repo remains the only control plane.
- Live release line remains overwrite-published `v0.8`.
- Current MAIN live binary target remains `/root/ecommerce_ai/releases/v0.8/ecommerce-api`.
- 2026-03-23 overwrite deploy completed through the existing repository deploy entrypoint.
- Post-deploy runtime verification reported:
  - `8080 main status=ok`
  - `8081 bridge status=ok`
  - `8082 sync status=ok`
  - `OVERALL_OK=true`

### Current minimum organization model
- Fixed departments now exposed by backend org options:
  - `人事部`
  - `设计部`
  - `运营部`
  - `采购部`
  - `仓储部`
  - `烘焙仓储部`
  - `未分配`
- Fixed first-version teams now exposed by backend org options:
  - `人事部 -> 人事管理组`
  - `设计部 -> 定制美工组, 设计审核组`
  - `运营部 -> 运营一组 ... 运营七组`
  - `采购部 -> 采购组`
  - `仓储部 -> 仓储组`
  - `烘焙仓储部 -> 烘焙仓储组`
  - `未分配 -> 未分配池`
- Backend rule:
  - user has one primary `department`
  - user has one primary `team`
  - `team` must belong to `department`
  - cross-department responsibility is expressed by `managed_departments` / `managed_teams`, not by multiple primary org records

### Current minimum role and scope model
- Role catalog now includes the minimal management roles:
  - `SuperAdmin`
  - `HRAdmin`
  - `OrgAdmin`
  - `RoleAdmin`
  - `DepartmentAdmin`
  - `TeamLead`
  - `DesignDirector`
  - `DesignReviewer`
  - `Member`
- Existing workflow/business roles remain intact for compatibility:
  - `Admin`
  - `Ops`
  - `Designer`
  - `Audit_A`
  - `Audit_B`
  - `Warehouse`
  - `Outsource`
  - `ERP`
- `frontend_access` now returns a stable minimum shape across `/v1/auth/me`, `/v1/users`, and `/v1/users/{id}`:
  - `roles`
  - `scopes`
  - `menus/pages/actions`
  - `view_all`
  - `department_codes`
  - `team_codes`
  - `managed_departments`
  - `managed_teams`

### Unassigned pool rule
- Unassigned pool is now explicit and enabled in backend config:
  - `department = 未分配`
  - `team = 未分配池`
- New/self-registered users can be placed into this pool without inheriting formal business department scope.
- JST-imported users without formal org mapping also fall back to the unassigned pool.
- Formal dispatch out of the pool is performed through user patching by `SuperAdmin` / `HRAdmin` / `OrgAdmin` class management roles.

### Audit and live verification result
- Live `GET /v1/auth/me` returns:
  - `roles`
  - `department`
  - `team`
  - `frontend_access`
- Live `GET /v1/roles` returned the expanded 17-role catalog.
- Live `GET /v1/users` and `GET /v1/users/{id}` returned the unified org/role/frontend fields.
- Live `GET /v1/org/options` returned 7 departments, department-team mappings, role catalog summary, and `unassigned_pool_enabled=true`.
- Live safe-sample verification completed on a newly registered unassigned-pool user:
  - register into `未分配 / 未分配池`
  - patch into `运营部 / 运营一组`
  - assign roles `Member + TeamLead`
  - toggle status `disabled -> active`
  - read back detail and permission logs successfully
- Live permission-log action evidence now covers the required minimum mutation classes:
  - `register`
  - `user_pool_assigned`
  - `user_org_changed`
  - `user_scope_changed`
  - `role_assigned`
  - `user_status_changed`

### Current boundary
- This round is minimum usable management closure, not a full organization platform.
- Existing business mainlines remain preserved:
  - reference-small fixed chain
  - multipart `remote.headers.X-Internal-Token` fixed chain
  - `GET /v1/tasks/{id}` design-asset aggregation fix
  - upload / complete / submit-design / task-detail mainline
- Existing legacy users may still carry historical department/team values until later cleanup or explicit reassignment.
- Current live backend is sufficient for backend/frontend management-side joint debugging, but not yet a full historical-data cleanup.

## 2026-03-20 MAIN server closure baseline (latest truth source)
- This section supersedes older notes below for the current three-endpoint definition, reference-small closure, multipart browser-direct contract, and `v0.8` overwrite deploy status.
- Local MAIN repo is the only control plane.
- Fixed three endpoints for the current round:
  - local MAIN engineering workspace
  - server live `jst_ecs`
  - NAS `synology-dsm`
- Do not restate the current "three endpoints" as including frontend code. Frontend is not in the local workspace for this round and no frontend local build/change is part of the current closure.

### Control-plane rules
- Windows local control node must not enable SSH connection multiplexing:
  - `ControlMaster`
  - `ControlPersist`
  - `ControlPath`
- Keep the existing repository deploy entrypoints as the only valid packaging/deploy path:
  - `deploy/package-local.sh`
  - `deploy/deploy.sh`
  - `deploy/remote-deploy.sh`

### Current upload contract
- Current mode split:
  - `reference` = `small`
  - `delivery` = `multipart`
  - `source` = `multipart`
  - `preview` = `multipart`
- Browser multipart host:
  - `http://192.168.0.125:8089`
- MAIN server-to-server / probe host:
  - `http://100.111.214.38:8089`

### Reference-small final archived conclusion
- `task-create reference` small upload uses `/upload/files` return data as the canonical result.
- MAIN does not call NAS `complete` for the small path.
- MAIN must verify stored size/hash after landing.
- Verification mismatch must fail immediately.
- MAIN must not generate a successful ref when stored size/hash verification fails.
- `reference-upload` 0-byte issue is closed and archived under this rule set.

### Multipart final archived conclusion
- The final root cause of the PSD multipart issue was not the three-endpoint contract itself, not the reference-small path, and not a pure CORS-only cause.
- Final root cause:
  - MAIN created `RemoteUploadSessionPlan` without `remote.headers.X-Internal-Token`.
- Effective fix:
  - MAIN service injects `remote.headers` when creating `RemoteUploadSessionPlan`.
  - At minimum, `remote.headers` must include `X-Internal-Token`.
- Contract rule:
  - This header group is returned by MAIN to the browser.
  - The browser reuses the returned `remote.headers` on:
    - `PUT part_upload`
    - `POST complete`
    - `POST abort`
  - Token must not be hardcoded in frontend.
  - This is part of the MAIN-to-NAS multipart direct-upload contract.

### Current live status
- Current live release line remains overwrite-published `v0.8`.
- Current MAIN live binary target is `/root/ecommerce_ai/releases/v0.8/ecommerce-api`.
- 2026-03-20 overwrite deploy completed again through the repository deploy scripts.
- Live verification after overwrite confirmed:
  - `remote.headers.X-Internal-Token` is present in the multipart create-session response.
  - direct NAS `PUT /parts/1` returned `200`
  - direct NAS `POST /abort` returned `200`
  - prior failure signatures `ERR_CONNECTION_RESET` and `abort 401` are no longer present in the verified path.

## 2026-03-20 MAIN authoritative baseline (supersedes older conflicting notes below)
- This section is the current truth source for MAIN entrypoints, three-endpoint collaboration, and the `reference-upload` 0-byte incident closure.
- Any older note below that says current live is `v0.5`, `v0.6`, or a future `v0.9` is historical only and must not be used as the current live baseline.
- Any older note below that says `delivery` currently uses small upload, or that task-create `reference` small upload must call NAS `complete`, is deprecated.
- Any older note below that implies `/v1/assets/files/*` caused the 0-byte reference incident is deprecated.

### Effective entrypoints
- Runtime/build entrypoint: `cmd/server/main.go`
- Route registration entrypoint: `transport/http.go`
- Deploy/package entrypoints:
  - `deploy/deploy.sh`
  - `deploy/remote-deploy.sh`
  - `deploy/package-local.sh`
- Production packaging/deploy is locked to `./cmd/server`.
- `cmd/api` is deprecated compatibility-only code and is not a current production build or deploy entrypoint.

### Live deploy truth
- Current live release line remains overwrite-published `v0.8`.
- Current MAIN live binary target is `/root/ecommerce_ai/releases/v0.8/ecommerce-api`.
- Current deploy script directory is `/root/ecommerce_ai/releases/v0.8/deploy`.
- Current log directory is `/root/ecommerce_ai/logs`.
- Live verification must use:
  - `/proc/<pid>/exe`
  - `sha256`
  - `/health`
- Current server deploy should prefer the existing MAIN deploy-script system and must not be documented as a `v0.9` cutover.

### Three-endpoint control plane truth
- Local MAIN repo is the only control plane.
- Server alias: `jst_ecs`
- NAS alias: `synology-dsm`
- Future server and NAS collaboration must be coordinated from the local MAIN workspace.
- Windows local control node keeps SSH keepalive but must not enable:
  - `ControlMaster`
  - `ControlPersist`
  - `ControlPath`
- Reason: `OpenSSH_for_Windows_9.5p2` was verified to fail with `getsockname failed: Not a socket` and `Unknown error`.
- Linux/macOS control nodes may enable SSH multiplexing as an optional optimization.
- Standard tmux sessions:
  - server: `main-live`
  - NAS: `nas-upload`
- NAS tmux entry command:
  - `ssh synology-dsm "source ~/.bashrc >/dev/null 2>&1; tmux new -As nas-upload"`

### Current upload/download truth
- MAIN service-to-service NAS calls use `UPLOAD_SERVICE_BASE_URL`.
- Browser multipart direct upload must use `http://192.168.0.125:8089`.
- Current mode split is:
  - `reference` = `small`
  - `delivery` = `multipart`
  - `source` = `multipart`
  - `preview` = `multipart`
- `/v1/assets/files/*` is a read proxy only and not the root cause of the 0-byte reference issue.

## 2026-03-20 task-create reference 0-byte incident closed and archived

### Symptom
- `POST /v1/tasks/reference-upload` returned `201` and a structurally valid `ReferenceFileRef`.
- Newly uploaded reference `public_url` reads returned `200` with `Content-Length=0` and empty body.
- Historical references remained normal.

### Single confirmed root cause
- The issue was not caused by MAIN request-body forwarding.
- The issue was not caused by a global NAS `/files/*` read failure.
- The single root cause was NAS small upload plus `complete` pseudo-success:
  - remote manual `create + /upload/files + /complete` could also land a 0-byte file
  - remote manual `create + /upload/files` landed the file correctly

### Implemented fix
- `task-create reference` small uploads no longer call NAS `complete`.
- MAIN now uses `/upload/files` return data as the canonical small-upload result:
  - `file_id`
  - `storage_key`
  - `file_size`
- MAIN probes the stored object before binding success.
- MAIN rejects size/hash mismatch immediately.
- MAIN no longer allows a 0-byte physical file to be wrapped as a successful `ReferenceFileRef`.

### Acceptance
- New reference samples now land on NAS with correct size/hash.
- MAIN `public_url` downloads now return correct size/hash for new references.
- Creating tasks with the new refs succeeds and detail readback is normal.
- Historical references do not regress.
- Current routing remains:
  - `reference` still uses `small`
  - asset-center multipart browser host remains `192.168.0.125:8089`

### Mandatory rule
> `task-create reference` 的 small 上传链路以 `/upload/files` 返回结果为准，不再调用 NAS `complete`；MAIN 必须对落盘结果做 size/hash 校验，校验失败直接报错，不得生成成功 ref。

- Plain English: the task-create reference small-upload path must trust `/upload/files`, must not call NAS `complete`, and must fail immediately on stored size/hash mismatch instead of returning a successful ref.

## 2026-03-19 task filing policy upgrade (backend implemented)
- Scope: `original_product_development`, `new_product_development`, `purchase_task` filing/ERP sync strategy upgraded from legacy `business-info + filed_at` single boundary to backend state-machine + auto triggers + idempotency.
- Delivery boundary for this entry:
  - `Design Target`: all 3 task types follow auto filing policy by trigger source + state machine.
  - `Code Implemented`: backend trigger/state/idempotency/retry/read-model/openapi changes are merged.
  - `Server Verified`: code-level compile and package-level tests were executed in local dev environment.
  - `Live Effective`: not declared in this entry (requires explicit deploy + live verification evidence).
- New filing states in use: `not_filed`, `pending_filing`, `filing`, `filed`, `filing_failed`.
- New persisted fields (task_details): `filing_trigger_source`, `last_filing_attempt_at`, `last_filed_at`, `erp_sync_required`, `erp_sync_version`, `last_filing_payload_hash`, `last_filing_payload_json` (legacy `filed_at` kept for compatibility).
- Trigger points now implemented:
  - original task: audit final approve auto trigger; warehouse complete precheck auto trigger (non-blocking warning strategy)
  - new task: create auto-evaluate; business-info patch auto re-evaluate
  - purchase task: create auto-evaluate; procurement update/advance auto re-evaluate
- Idempotency rule: same task + same effective payload hash will skip duplicate ERP upsert; payload change allows next sync version and re-sync.
- Retry support implemented:
  - `GET /v1/tasks/{id}/filing-status`
  - `POST /v1/tasks/{id}/filing/retry`
- OpenAPI and read model exposure updated for frontend filing status display:
  - `filing_status`, `filing_error_message`, `missing_fields`, `missing_fields_summary_cn`, `last_filed_at`, `erp_sync_required`.

## 2026-03-19 reference_images 限制热修复（v0.8 已覆盖）
- 根因：`reference_images` 直传是把 base64/字符串数组直接写进 `task_details.reference_images_json`。旧规则与早期 `TEXT` 列容量不匹配时，超大请求会在 create tx 内触发 `Data too long for column 'reference_images_json'`，并被包装成 `internal error during create task tx` 500。
- 新规则：`reference_images` 统一为单张 `<= 3MB`、最多 `3` 张；不再保留会误伤的旧 `512KB` 总量限制，改为兼容 `3MB * 3` 的总量校验。
- 事务前校验：handler 与 service 共用同一套校验；在进入 create tx 前完成数量/大小校验，并在序列化 `reference_images_json` 前再次校验，避免超限图片进入 DB insert。
- 存储调整：新增 migration `044_v7_reference_images_mediumtext.sql`，将 `task_details.reference_images_json` 扩容为 `MEDIUMTEXT`，确保合法上限内的直传请求不会因列容量不足再触发 DB 500。
- 错误契约：超限统一返回 `400 INVALID_REQUEST`，`message=reference_images exceed upload limit`，并返回 `actual_count`、`max_count`、`max_single_bytes`、`oversized_indexes`、`suggestion=use asset-center upload / reference_file_refs`。
- 前端建议：小图可继续走 `reference_images`；超过单张 `3MB` 或超过 `3` 张时，默认走 asset-center 上传，并在创建/编辑时传 `reference_file_refs`。

## 2026-03-19 v0.8 Overwrite Hotfix Deployed
- Release mode: overwrite deploy on existing `v0.8`; no `v0.9` was created.
- Binary coverage: replaced `releases/v0.8/ecommerce-api` and `releases/v0.8/erp_bridge`.
- 8082 status: `erp_bridge_sync` binary was not replaced in this round.
- Runtime verification: 8080 and 8081 restarted onto `releases/v0.8/*`; 8082 was restored after `stop-bridge.sh` also matched `erp_bridge_sync`, but its binary remained unchanged.
- Original create-chain verification on live `223.4.249.11:8080`:
- Case 1 `design_requirement -> change_request` alias: `201`.
- Case 2 `is_outsource` alias: `201`, response `need_outsource=true`.
- Case 3 `product_selection + defer_local_product_binding`: `201`.
- Case 4 illegal original-task fields: `400`, `code=INVALID_REQUEST`, `message=task_type field whitelist validation failed`, `details.invalid_fields`, and `violations` returned.

## v0.8 — 商品主数据 live 真相源切换完成版（2026-03-18）

**v0.8 = 商品主数据 live 真相源切换完成版**。本轮不是普通修 bug，而是：
- 真正切了主链（8081 OpenWeb 主链已实证）
- 真正让 products 变成副本承接（从 20 增至 7470）
- 真正验证了 OpenWeb 命中（remote_ok、fallback_used=false）

### 已实证事实（live 223.4.249.11）
| 事实 | 状态 |
|------|------|
| 8081 `remote_ok` | 已实证 |
| 8081 `fallback_used=false` | 已实证 |
| 8080 `ERP_SYNC_SOURCE_MODE=jst` | 已实证 |
| `JSTOpenWebProductProvider` 驱动 sync | 已实证 |
| products 从 20 增至 7470 | 已实证 |
| HQT21413 样本刷新与 `sync_role=8080_products_replica_from_openweb` 写入 | 已实证 |

当前 live 运行态：8080 PID 3876002、8081 PID 3876047、8082 PID 3876205；health 全 200。

---

## ITERATION_082 — ERP 商品主数据四层职责收口（2026-03-18）

### 四层职责（最终定义）
| 层 | 职责 | 非职责 |
|----|------|--------|
| **8081 商品查询** | 原品选品搜索、ERP 详情；`remote`/`hybrid` 下 **OpenWeb POST `ERP_REMOTE_SKU_QUERY_PATH` 优先**（主链）；local/fallback 仅兜底 | 品类树主数据、任务长期承接 |
| **8080 products** | JST/OpenWeb 同步副本、ERP 映射缓存、任务/成本/商品维护读写的本地承接表 | 选品搜索唯一真相源、原品创建唯一硬前置（允许 defer + ERP snapshot） |
| **8082 jst_inventory** | JST 同步驻留原始表、证据、对账、品类索引抽取来源（本仓库无表实现，见部署侧） | 前台商品搜索主表、原品开发创建主表 |
| **品类/分类维度** | 业务分类主语义 = **款式编码（i_id）**；`GET /v1/categories`（主）、`/v1/erp/categories`（辅）来自**本地可配置映射层**（当前含 31 行样例，非生产真实分类库） | 对 `jst_inventory` 全表扫描当品类 API；将聚水潭 `category` 默认当业务分类 |

### 行为变更（证据向）
- **hybrid 回退**：仅在网络超时、5xx 等「可恢复上游故障」时回退本地 `products`；OpenWeb 业务码、JSON 解析失败、`AUTH!=openweb`、远程 SKU 未命中 **不回退**（日志 `erp_bridge_product_*`）。
- **8081 启动**：`remote`/`hybrid` 强制 `ERP_REMOTE_BASE_URL` + `ERP_REMOTE_AUTH_MODE=openweb`。
- **原品 defer**：`product_id=null` 时详情仍返回 `product`（`status=erp_snapshot`，见 `task_detail_service`）。
- **验收清单**：`docs/ERP_REAL_LINK_VERIFICATION.md`。
- **真相源统一说明**：`docs/TRUTH_SOURCE_ALIGNMENT.md`（ITERATION_083）。

## ITERATION_081A — 原品开发创建任务 500（ERP 绑定解析）专项修复（2026-03-18）

### 日志证据与根因
- 线上 `trace_id=9837a36e-ebaa-4262-9a91-6bc1ff3d7a47` 命中 `/root/ecommerce_ai/logs/ecommerce-api-20260318T032624Z.log`，确认 `POST /v1/tasks` 返回 `500`。
- 补充服务内错误日志后，复现同类请求拿到真实报错：
  - `create task: insert task_detail: Error 1136 (21S01): Column count doesn't match value count at row 1`
- 结论：500 的直接根因是 `repo/mysql/task.go` 中 `task_details` 插入 SQL 列数与占位符数不一致（57 列 vs 55 占位符），属于 DB 写入失败链路。

### 本轮最小修复
- 修复 `repo/mysql/task.go`：补齐 `task_details` INSERT 占位符到 57，和列数一致，消除 `create task tx` 500。
- 修复 `transport/handler/task.go`：创建入口增加原品开发商品绑定归一优先级与路径日志。
  - 优先级：`top.product_id` -> `product_selection.erp_product.product_id` -> `product_selection.erp_product.sku_code` -> `top.sku_code`
  - 支持 `product_id=null` 且仅传 `sku_code` 的归一绑定。
- 修复 `service/erp_bridge_service.go`：`EnsureLocalProduct` 增加 `sku_code` 作为回退绑定键（在 `product_id/sku_id` 为空时）。

### 复测结果（生产入口）
- Case A（`product_id=null` + `product_selection.erp_product.product_id/sku_code`）：
  - `201 Created`，成功创建任务，`product_id` 归一为本地 `485`。
- Case B（`product_id=null` + 仅顶层 `sku_code`）：
  - `201 Created`，成功创建任务，`product_id` 归一为本地 `485`。
- 两个 case 均已消除 `internal error during create task tx`。

## ITERATION_081 — 最小阻塞修复 + 重部署 + 服务器复验（2026-03-18）

### 本轮定位
- 本轮是**实操验收与收口核对**，不是继续扩功能。
- 以线上 `223.4.249.11`（8080/8081/8082）真实响应与文件流为唯一判据。

### 已完成并有证据
- 服务器运行态核验：三服务在线，health 全 200。
- DB 核验：补齐 041/042/043 对应列，且已复核存在。
- 任务创建三类型（original/new/purchase）实测均可 `201` 创建并可读详情。
- 资产中心：reference/source(含伪 PSD)/delivery 上传会话与 complete 成功，版本列表可见，source/PSD 的受控访问语义正确。
- OpenAPI：`docs/api/openapi.yaml` 从解析失败修复为可全量解析；补齐批量与资产下载关键路径及缺失 schema 命名。

### 本轮阻塞修复结果（已复验）
- 批量路由恢复：
  - `POST /v1/tasks/batch/remind` 实测 `200`
  - `POST /v1/tasks/batch/assign` 实测命中批量 handler（空 body 返回 `batchAssignTaskReq` 校验错误；有效 body 返回 `200` + 逐项结果）
- per-task 商品/成本 5 接口恢复：
  - `GET/PATCH /v1/tasks/{id}/product-info`：`200`
  - `GET/PATCH /v1/tasks/{id}/cost-info`：`200`
  - `POST /v1/tasks/{id}/cost-quote/preview`：按数据前置条件返回 `400`（非路由 404）
- 资产下载接口恢复：
  - `GET /v1/tasks/{id}/assets/{asset_id}/download`：`200`
  - `GET /v1/tasks/{id}/assets/{asset_id}/versions/{version_id}/download`：`200`
- delivery 推审闭环恢复：
  - 在 `PendingAssign` 任务完成 delivery upload session 后，任务状态推进为 `PendingAuditA`

### 最小修复记录（本轮执行）
- 路由最小修复：将 `POST /v1/tasks/batch/assign` 与 `POST /v1/tasks/batch/remind` 注册顺序前移到 `/:id/assign` 之前，避免被参数路由吞掉。
- delivery 推审最小修复：asset-center complete delivery 时，自动推进 `PendingAuditA` 的适用状态扩展到
  - `PendingAssign`
  - `Assigned`
  - `InProgress`
  - `RejectedByAuditA`
  - `RejectedByAuditB`
- 针对补列后 `POST /v1/tasks` 出现 500 的兼容性问题，已在服务器将以下新列由 `TEXT NOT NULL` 调整为可空，恢复线上创建链路：
  - `task_details.filing_error_message`
  - `task_details.note`
  - `task_details.reference_file_refs_json`
- 该修复仅为运行兼容，不改变业务流程语义。

### 当前结论
- 五模块整体状态：**达到可收口**（本轮指定阻塞项已修复并通过服务器复验）。
- 当前运行版本：`v0.6`，MAIN 进程 `PID=3777316`，`/proc/3777316/exe -> /root/ecommerce_ai/releases/v0.6/ecommerce-api`，`health=200`。

## v0.5 已发布 (2026-03-17, ITERATION_080)

### 发布事实
- **发布目标机**：`223.4.249.11`
- **发布目录**：`/root/ecommerce_ai/releases/v0.5`
- **当前线上版本**：v0.5

### 三服务运行状态（已验收）
| 服务 | 端口 | PID | 二进制路径/名称 | health |
|------|------|-----|------------------|--------|
| MAIN | 8080 | 3589336 | `/root/ecommerce_ai/releases/v0.5/ecommerce-api` | 200 |
| Bridge | 8081 | 3589373 | `/root/ecommerce_ai/releases/v0.5/erp_bridge` | 200 |
| Sync | 8082 | 3589421 | `erp_bridge_sync` | 200 |

### Migration 执行情况
- **038**：`users` 表新增 `jst_u_id`、`jst_raw_snapshot_json` — **已执行，且为 v0.5 启动前置条件**
- **039**：`rule_templates` 表 + 3 条种子数据 — 已执行
- **040**：`server_logs` 表 — 已执行

**重要**：migration **038 是 v0.5 启动前置条件**。v0.5 代码会查询 `users.jst_u_id` / `users.jst_raw_snapshot_json`，若只执行 039/040 而漏掉 038，MAIN 启动后会立即退出。部署时必须按 038 → 039 → 040 顺序执行迁移后再启动服务。

### 部署中遇到并已解决的问题
- 初次部署时 MAIN 启动立即退出。
- **原因**：v0.5 代码查询 `users.jst_u_id` / `users.jst_raw_snapshot_json`，但 migration 038 尚未执行。
- **处理**：补齐 migration 038 后，服务恢复正常。

### 本轮已完成 API 验证范围
以下接口已在 v0.5 环境中完成验证：
- 登录
- `GET /v1/auth/me`（/me）
- `GET /v1/rule-templates`（rule-templates）
- `GET /v1/server-logs`、`POST /v1/server-logs/clean`（server-logs）
- `GET /v1/tasks`、`POST /v1/tasks`、`GET /v1/tasks/{id}`（tasks）

### v0.5 收口能力范围（代码与文档）
- 任务创建/详情：assignee_id、reference_file_refs、note、creator_name 等字段补齐；设计师指派回显；创建时传 designer_id 直接进入 InProgress，设置 current_handler_id
- 审核流程：delivery 上传后自动 PendingAuditA；submit-design 后进入 PendingAuditA；claim/reject/approve 收口
- 规则及模板：主菜单「规则及模板」；rule_templates 表 + API（cost-pricing, product-code, short-name）
- 组织与权限：user_admin/org_admin/role_admin/logs_center 归入同一 section
- 参考图：validateReferenceImages 已统一为单张 `<= 3MB`、最多 `3` 张；超限会在 create tx 前返回 `400 INVALID_REQUEST`，并提示走 asset-center / `reference_file_refs`
- 服务器日志：server_logs 表；GET/POST /v1/server-logs；5xx 自动入库；返回前脱敏；Admin 权限
- 文档：openapi.yaml、FRONTEND_ALIGNMENT_v0.5.md 已收口为 v0.5 联调基准

## v0.4 JST 用户同步预埋 (2026-03-17, ITERATION_079)
- **预埋能力**：仅提供查询与导入，不改变主业务用户/权限/登录逻辑。
- Bridge(8081) 新增 JST getcompanyusers 适配：`GET /v1/erp/users` -> `/open/webapi/userapi/company/getcompanyusers`
- MAIN Admin 接口：`GET /v1/admin/jst-users`、`POST /v1/admin/jst-users/import-preview`、`POST /v1/admin/jst-users/import`
- 本地 users 表扩展：`jst_u_id`、`jst_raw_snapshot_json`（migration 038）
- 导入策略：匹配 jst_u_id > loginId(若存在) > username；新建用户 status=disabled，密码随机 hash（不可直接登录）
- 角色映射：默认 `write_roles=false`，不写入 user_roles
- JST 仅作数据源，不接管鉴权、不自动同步

## v0.4 Bridge Semantic & Route Acceptance (2026-03-17, ITERATION_078)
- Live deployment still uses shared `cmd/server` source and `v0.4` binaries (no version bump).
- Bridge live mode remains `ERP_REMOTE_MODE=hybrid` with OpenWeb credentials/signing:
  - `ERP_REMOTE_BASE_URL=https://openapi.jushuitan.com`
  - `ERP_REMOTE_AUTH_MODE=openweb`
  - signer: `md5(app_secret + sorted(key+value))`, keys = `app_key/access_token/timestamp/charset/version/biz` (`sign` excluded)
- Semantic contract alignment now verified through live write responses:
  - `sku_id` = 商品唯一编码（JST 商品编码）
  - `i_id` = 款式/分类维度（JST 款式编码）
  - `name` = 商品名称
  - `short_name` = 商品简称（模板能力已接入服务层）
  - `wms_co_id` = 仓库维度
- New Bridge routes are live and verified (post-deploy):
  - `POST /v1/erp/products/style/update` (`200` for Admin/Ops, `403` for roleless)
  - `GET /v1/erp/warehouses` (`200` for Admin/Ops, `403` for roleless)
- 11-warehouse contract is live on `/v1/erp/warehouses`, covering all required `wms_co_id` mappings.
- Upsert/style dual write-path behavior is now explicit in response payload:
  - upsert result `route=itemskubatchupload`
  - style update result `route=itemupload`
- Shelve/unshelve/virtual-qty remain stable under hybrid fallback:
  - accepted response with `sync_log_id` and `message=stored locally`
  - sync logs keep warehouse context (`wms_co_id`, plus `bin_id/carry_id/box_no` payload fields)
- Permission regression remains correct:
  - `Admin` and `Ops` keep ERP read/write `200`
  - roleless remains blocked (`403`)
- Runtime/deploy safety after this rollout:
  - `8080/8081/8082` all healthy
  - check-three-services reports all `exe_deleted=false`
  - current pids: MAIN `3546054`, Bridge `3546082`, Sync `3546261`

## Current Phase
- **v0.5 已发布**。当前线上版本为 v0.5，三服务运行于 223.4.249.11，migrations 038/039/040 已执行，API 验收已完成。
- V7 Migration Step 69 complete

### Current Live Snapshot (Authoritative)
- **当前线上版本为 v0.5**。发布事实、三服务状态、migration 顺序及 038 前置条件以本文顶部「v0.5 已发布」一节为权威依据。
- Bridge/ERP 相关历史结论（如 ITERATION_078 语义契约、style-update/warehouses 等）仍适用于当前 8081/8082 行为；若与顶部 v0.5 发布事实冲突，以顶部为准。

### Historical Archive (Evidence Retained, Not Current Live)

#### Archive A — ITERATION_075 Stage Record (superseded by ITERATION_076)
- v0.4 ERP bridge acceptance and permission hardening (2026-03-17):
  - external ERP formal remote writeback acceptance is still **not completed** in live because `ERP_REMOTE_BASE_URL` and remote auth/sign credentials are still missing in `/root/ecommerce_ai/shared/bridge.env`
  - live bridge mode stays `ERP_REMOTE_MODE=local` (safe mode retained, no unstable remote config left online)
  - `/v1/erp/*` permission policy was tightened in `transport/http.go`:
    - read routes require one of `Ops/Designer/Audit_A/Audit_B/Warehouse/Outsource/ERP/Admin`
    - write routes require one of `Ops/Warehouse/ERP/Admin`
    - sync-log routes require one of `Ops/Warehouse/ERP/Admin`
  - runtime evidence after deploy:
    - real `Admin` and `Ops` sessions both pass all listed `/v1/erp/*` routes (`200`)
    - real roleless session now gets `403` on read/write probes (`/v1/erp/products`, `/v1/erp/categories`, `/v1/erp/products/upsert`)
  - deployment check passed with no runtime breakage:
    - `8081` pid `3507876`, exe not deleted
    - `8080` pid `3507849`, `/v1/tasks` still `200`
    - `8082` pid `3508034`, `/health` and `/internal/jst/ping` both `200`
- v0.4 ERP Bridge remote-ready update (2026-03-17): Bridge service (8081) is online with runtime mode switch (`ERP_REMOTE_MODE=local|remote|hybrid`) and a new external-ERP upsert client (`service/erp_bridge_remote_client.go`) supporting configurable base URL/path, auth/signature headers, timeout/retry, and structured logging.
- Live mode is currently `ERP_REMOTE_MODE=local` on server `223.4.249.11`, so MAIN -> Bridge -> local MySQL remains the active write path; external ERP production API is **not yet confirmed connected**.
- Bridge env (`/root/ecommerce_ai/shared/bridge.env`) now includes remote-mode keys (`ERP_REMOTE_BASE_URL`, `ERP_REMOTE_AUTH_MODE`, retry/timeouts, fallback switch), allowing no-code mode flip for later联调.
- v0.4 ERP Bridge route restoration remains valid: `POST /v1/erp/products/upsert` 404 gap has been fixed and MAIN -> Bridge call path is stable.

#### Archive B — MAIN ERPSyncWorker Historical Recovery Record
- v0.4 MAIN ERPSyncWorker verification/recovery (2026-03-17): old `cmd/api/main.go` cron-based `*/10 * * * * -> syncSvc.IncrementalSync(10)` is confirmed historical only and is not the live sync owner. The live owner remains MAIN(8080) `ERPSyncWorker` plus `/v1/products/sync/*`.
- Live evidence before recovery: `GET /v1/products/sync/status` returned `scheduler_enabled=true`, `interval_seconds=300`, `source_mode=stub`, latest scheduled run `status=noop`; authenticated `POST /v1/products/sync/run` also returned `status=noop`.
- Confirmed root cause: live MAIN process `/proc/3416820/cwd` was `/root`, while `ERP_SYNC_STUB_FILE` remained relative (`config/erp_products_stub.json`) and the actual packaged stub file lived under `/root/ecommerce_ai/releases/v0.4/config/erp_products_stub.json`. That runtime working-directory mismatch made the stub provider resolve to a missing file path and continuously record `noop`.
- Fix applied in repo and redeployed in-place to `v0.4`: `deploy/run-with-env.sh` now changes into the resolved binary directory before exec so packaged relative config files resolve correctly; `/v1/products/sync/status` also now exposes `resolved_stub_file` and `stub_file_exists` for direct operability.
- Live result after recovery:
  - new MAIN pid `3450797`
  - `/proc/3450797/exe -> /root/ecommerce_ai/releases/v0.4/ecommerce-api`
  - `/proc/3450797/cwd -> /root/ecommerce_ai/releases/v0.4`
  - authenticated manual sync now returns `status=success`, `total_received=2`, `total_upserted=2`
  - first post-deploy scheduled run also recovered: `trigger_mode=scheduled`, `status=success`, `started_at=2026-03-17T10:58:38+08:00`, `total_upserted=2`
  - `/v1/products/search` and `/v1/erp/products` both return the recovered stub-backed product rows (`ERP-10001`, `ERP-10002`)
- Current boundary after verification:
  - MAIN `ERPSyncWorker` owns scheduled local product-cache sync and `erp_sync_runs` history.
  - MAIN local product cache is still the backing data source for compatibility routes `/v1/products/search` and `/v1/products/{id}`.
  - Bridge(8081) owns adapter query semantics and mutation execution; in current live `ERP_REMOTE_MODE=local`, Bridge query/write paths still read/write the shared local `products` table via `localERPBridgeClient`.
  - external ERP production connectivity is still **not confirmed**; Bridge remains local mode in live.
- Recommended future alignment: keep MAIN as scheduler/cache owner, but when external ERP formal query access is ready, move `ERPSyncWorker` source from `stub` to a Bridge-owned query/export contract so MAIN no longer couples directly to upstream ERP semantics.

#### Archive C — Other Historical Milestones
- v0.4 closure validated (2026-03-16): 3-round blackbox E2E passed (original_product_development, new_product_development, purchase_task); ERP filing made non-blocking; TaskListItem fields completed; all 3 task creation rules verified aligned with PRD
- v0.4 convergence phase close-out recorded
- v0.4 fixed release (2026-03-13): API docs (API_USAGE_GUIDE.md, API_INTEGRATION_GUIDE.md) now auto-generated per release; frontend_access.json is canonical for menus/pages/actions; deploy supports --version for pinning; check-remote-db.sh for DB integration readiness
- Deploy SSH key upgrade (2026-03-13): deploy chain now defaults to `DEPLOY_AUTH_MODE=key` with batch-mode SSH/SCP. Use `deploy/setup-ssh-key.ps1` from Windows IDE / PowerShell or `deploy/setup-ssh-key.sh` from bash to authorize `~/.ssh/id_deploy_ecommerce`; `DEPLOY_AUTH_MODE=password` + `DEPLOY_PASSWORD` remains compatibility fallback only.
- Main flow end-to-end readiness (2026-03-14): all mainline flows (auth, permission, logs, task, upload, audit, warehouse) verified as code-complete; asset access policy (LAN/Tailscale/public) now formally populates lan_url/tailscale_url/public_url in DesignAssetVersion responses
- Task create rules formalized (2026-03-14): three task types (original_product_development, new_product_development, purchase_task) now have explicit field-level validation with owner_team required for all; see docs/TASK_CREATE_RULES.md
- ERP search stabilization (2026-03-14): `/v1/erp/products`, `/v1/erp/products/{id}`, and `/v1/erp/categories` now define a stable result-page contract for frontend integration. MAIN now treats `product_id` as the facade lookup key, normalizes best-effort `sku_code/category_name/category_code/image_url/product_short_name`, rejects invalid exact category filters with empty results instead of browse fallback, and applies compatible detail fallback lookup so list -> detail -> task binding remains stable. Supported boundary is keyword contains-match, exact sku, exact category, and browse only; global search, suggestions, and sorting remain unsupported. See docs/API_USAGE_GUIDE.md and docs/ERP_SEARCH_CAPABILITY.md.

## Main Flow End-to-End Status (2026-03-14)

### Auth / Identity
- register-options: READY (GET /v1/auth/register-options)
- register: READY (POST /v1/auth/register) — supports admin_key for department admin registration
- login: READY (POST /v1/auth/login)
- /me: READY (GET /v1/auth/me) — returns user + roles + frontend_access
- change password: READY (PUT /v1/auth/password)
- admin_key business rules:
  - no admin_key → registered as member
  - valid department admin key → registered as dept_admin
  - super_admin not via self-registration; managed by auth_identity.json config
- frontend_access: stable structure, driven by role + department + identity merge

### Permission Management
- user list: READY (GET /v1/users)
- user detail: READY (GET /v1/users/:id)
- role assignment: READY (POST/PUT/DELETE /v1/users/:id/roles)
- frontend_access reflects is_department_admin / is_super_admin correctly

### Logs
- permission-logs: READY (GET /v1/permission-logs)
- operation-logs: READY (GET /v1/operation-logs)
- HR admin center can see both

### Task Main Flow
- create: READY (POST /v1/tasks)
- list: READY (GET /v1/tasks)
- detail: READY (GET /v1/tasks/:id/detail)
- board: READY (GET /v1/task-board/summary, /queues)
- assign: READY (POST /v1/tasks/:id/assign)

### Upload (reference / delivery / source)
- reference small upload: READY (POST /v1/tasks/:id/assets/upload-sessions, asset_type=reference)
- delivery upload: READY (POST /v1/tasks/:id/assets/upload-sessions, asset_type=delivery)
- source multipart upload: READY (POST /v1/tasks/:id/assets/upload-sessions, upload_mode=multipart, asset_type=source)
- complete: READY (POST .../complete)
- cancel: READY (POST .../cancel or .../abort)

### Audit
- claim: READY (POST /v1/tasks/:id/audit/claim)
- approve: READY (POST /v1/tasks/:id/audit/approve)
- reject: READY (POST /v1/tasks/:id/audit/reject)
- transfer/handover/takeover: READY
- approved_version: derived from task status + latest delivery version

### Warehouse
- receive: READY (POST /v1/tasks/:id/warehouse/receive)
- reject: READY (POST /v1/tasks/:id/warehouse/reject)
- complete: READY (POST /v1/tasks/:id/warehouse/complete)
- warehouse_ready_version: derived from task status + latest delivery version
- receipt list: READY (GET /v1/warehouse/receipts)

### Asset Access Policy (LAN / Tailscale / Public)
- Config: NAS_LAN_HOST (default 192.168.0.125), NAS_TAILSCALE_ADDR (default 100.111.214.38:8089)
- source files: lan_url=smb://..., tailscale_url=http://..., public_url=null, public_download_allowed=false
- delivery/reference/preview: lan_url, tailscale_url, public_url all populated, public_download_allowed=true
- access_hint: contains locator information for source files
- source_file_requires_private_network: true for source files
- See docs/ASSET_ACCESS_POLICY.md for full specification

## Current Stage Development Priority Decision
- this is the current-stage development priority decision, not a temporary verbal note
- current priority order is now:
  - mainline feature development first
  - integration, verification, release, and deployment first
  - compatibility retirement / legacy retirement / architecture-cleanup work no longer leads short-term execution
- retirement work is moved behind current mainline delivery and should proceed in:
  - engineering review windows
  - version close-out windows
  - post-release governance windows
- `docs/MAIN_BRIDGE_RESPONSIBILITY_MATRIX.md`, `docs/V0_4_MAIN_BRIDGE_CONVERGENCE_PLAN.md`, and `docs/V0_4_COMPATIBILITY_RETIREMENT_CHECKLIST.md` remain the governance baseline for later cleanup, but they are not the current mainline driver

## v0.4 Convergence Close-out
- completed in v0.4 so far:
  - canonical MAIN production entrypoint is locked to `./cmd/server`
  - `cmd/api` is demoted to compatibility-only / non-production status
  - MAIN versus Bridge ownership language is aligned:
    - MAIN = public business application service
    - Bridge = ERP/JST adapter and mutation boundary
  - legacy compatibility surface classification is explicit:
    - `keep temporarily`
    - `compatibility only`
    - `replaced by canonical MAIN`
    - `retire after v0.4`
  - sync/runtime terminology is aligned so v0.4 still keeps current MAIN-owned sync/runtime continuity
  - post-v0.4 retirement sequencing is documented in `docs/V0_4_COMPATIBILITY_RETIREMENT_CHECKLIST.md`
- not yet retired or removed:
  - no broad compatibility-route deletion has been performed
  - `GET /v1/products/search`, `GET /v1/products/{id}`, and `POST /v1/integration/call-logs/{id}/advance` remain on disk as compatibility surfaces
  - `cmd/api` remains on disk only as a compatibility remnant, not as a production entrypoint
  - same-host Bridge loopback assumptions and side-by-side validation remain for rollback-safe runtime continuity
- next likely implementation focus after v0.4:
  - current mainline feature development
  - integration, verification, release readiness, and deployment execution
  - retirement work only when review / close-out / governance windows explicitly open
- first real retirement executed after v0.4:
  - retired the wording-only mixed cache/live ERP compatibility remnant from active repository guidance
  - no live routes, sync/runtime behavior, Bridge dependency, or rollback-sensitive path was removed in this step

## v0.4 Runtime Terminology
- current runtime reality:
  - `live MAIN` = the public business application service on `8080`
  - for the smallest safe v0.4 convergence path, MAIN still owns the current sync/runtime/background behavior used by this repo, including `ERPSyncWorker` and `/v1/products/sync/*`
  - `Bridge` = the ERP/JST adapter runtime on `8081`; it owns adapter query semantics and ERP mutation execution, not MAIN business routes or generic MAIN sync/background ownership
  - `candidate MAIN` = a side-by-side validation instance of the same MAIN service on `18080`; it is not a second long-term runtime role
- target architecture and deferred work:
  - future extraction of MAIN-owned sync/runtime/background concerns, if needed, is deferred after v0.4 and is not implied by current repository wording
- compatibility-only behavior:
  - legacy `8080` compatibility surfaces and same-host loopback Bridge assumptions remain rollback-safe continuity only; they are not the target architecture

## Latest Increment
- STEP_70:
  - MAIN <-> NAS Upload Service real integration was verified on `2026-03-14` against the live NAS endpoint:
    - `UPLOAD_SERVICE_BASE_URL=http://100.111.214.38:8089`
    - `UPLOAD_SERVICE_INTERNAL_TOKEN=nas-upload-token-2026`
    - `UPLOAD_SERVICE_TIMEOUT=30s`
    - `UPLOAD_SERVICE_ENABLED=true`
    - `UPLOAD_STORAGE_PROVIDER=nas`
    - live MAIN runtime env was updated at `/root/ecommerce_ai/shared/main.env`
  - connectivity from the live MAIN host is now explicitly verified:
    - `GET /health` on the NAS Upload Service returns `200`
    - internal-token authenticated requests return non-auth responses on real endpoints
  - real upload chains now verified through MAIN business APIs plus real NAS byte transfer:
    - `reference` small upload:
      - MAIN `POST /v1/tasks/{id}/assets/upload-sessions`
      - direct NAS small-file upload
      - MAIN `POST /v1/tasks/{id}/assets/upload-sessions/{session_id}/complete`
      - `GET /v1/tasks/{id}/assets`
      - `GET /v1/tasks/{id}/assets/{asset_id}/versions`
    - `delivery` small upload:
      - real NAS small-file upload completed through MAIN
      - asset/version query verified
      - `approved_version` / `warehouse_ready_version` derivation verified on a `PendingWarehouseReceive` task
    - `source` multipart upload:
      - real create-session
      - real get-session
      - real part upload
      - real complete
      - resulting source asset-version persisted and queryable
  - real business-loop persistence is now verified:
    - `design_assets.current_version_id` updates correctly
    - `task.asset.upload_session.created`
    - `task.asset.version.created`
    - `task.asset.upload_session.completed`
    - are written into `task_event_logs`
  - minimal MAIN fixes discovered only by real integration:
    - `upload_requests` insert placeholder count corrected in `repo/mysql/upload_request.go`
    - upload-session sync no longer treats MySQL `rows affected = 0` as a missing row when the session record already exists
    - asset-version JSON now keeps `lan_url`, `tailscale_url`, and `public_url` in the response structure even when they are null
    - live DB schema had to be aligned with the additive Step 67-69 asset-center migrations before the real MAIN flow could succeed
  - frontend recommendation is now explicit:
    - first: `reference` small upload on `POST /v1/tasks/{id}/assets/upload-sessions`
    - second: `delivery` small upload
    - third: `source` multipart upload
    - `preview` remains auxiliary-only and is not a primary frontend upload path in the current phase
- STEP_69:
  - MAIN design-asset semantics are now formally canonicalized to `reference`, `source`, `delivery`, and `preview`:
    - legacy `original` input is normalized to `source`
    - legacy `draft` / `revised` / `final` / `outsource_return` inputs are normalized to `delivery`
    - DB migration `036_v7_design_asset_flow_semantics.sql` backfills canonical asset types and `task_assets.upload_mode`
  - task asset center now exposes business-readable flow pointers directly on each `design_asset`:
    - `current_version`
    - `approved_version`
    - `warehouse_ready_version`
    - `approved_version_id`
    - `warehouse_ready_version_id`
  - asset-version payloads now explicitly express access/flow semantics needed by design, audit, and warehouse:
    - `upload_mode`
    - `task_no`
    - `asset_no`
    - `is_source_file`
    - `is_delivery_file`
    - `is_preview_file`
    - `source_access_mode`
    - `access_policy`
    - `preview_available`
    - `approved_for_flow`
    - `warehouse_ready`
    - `current_version_role`
    - `lan_url`
    - `tailscale_url`
    - `public_url`
    - `public_download_allowed`
    - `direct_download_preferred`
    - `source_file_requires_private_network`
    - `preview_public_allowed`
    - `access_hint`
    - `notes`
  - business rules are now explicit in MAIN:
    - task-creation small-file uploads default to `reference` and must use `small`
    - PSD / PSB / AI and similar editable files are treated as `source` with controlled-access metadata; MAIN returns locating information even when browser public download is disabled
    - `delivery` is the formal audit / warehouse flow image; automatic PSD->JPG conversion remains auxiliary-only and is not the business truth path
    - warehouse-facing read models now return `warehouse_ready_version`, and task workflow readiness now keys off canonical `delivery` semantics rather than the old `final` literal
- STEP_68:
  - MAIN now formally integrates with the NAS-side independent Go upload service as the real file-transfer/storage boundary:
    - `service/upload_service_client.go` is now a real HTTP client instead of a local stub seam
    - upload-service config now includes:
      - `UPLOAD_SERVICE_BASE_URL`
      - `UPLOAD_SERVICE_TIMEOUT`
      - `UPLOAD_SERVICE_ENABLED`
      - `UPLOAD_SERVICE_INTERNAL_TOKEN`
      - `UPLOAD_STORAGE_PROVIDER=nas`
    - `UPLOAD_SERVICE_AUTH_TOKEN` remains a backward-compatible legacy env alias only
  - task-scoped asset-center business models are now persisted with remote upload/file identity:
    - additive `task_assets.remote_file_id`
    - additive `upload_requests.remote_file_id`
    - additive `upload_requests.last_synced_at`
  - MAIN now exposes `/v1/tasks/{id}/assets/*` as the primary frontend asset-center contract:
    - `GET /v1/tasks/{id}/assets`
    - `GET /v1/tasks/{id}/assets/{asset_id}/versions`
    - `POST /v1/tasks/{id}/assets/upload-sessions`
    - `GET /v1/tasks/{id}/assets/upload-sessions/{session_id}`
    - `POST /v1/tasks/{id}/assets/upload-sessions/{session_id}/complete`
    - `POST /v1/tasks/{id}/assets/upload-sessions/{session_id}/abort`
    - `POST /v1/tasks/{id}/assets/upload`
    - `GET /v1/tasks/{id}/assets/timeline` remains the legacy task-asset timeline compatibility view
    - prior `/asset-center/*` routes are kept as compatibility aliases
  - upload completion is now a closed business loop inside MAIN:
    - remote NAS completion/file-meta is mapped into one `asset_version` record (persisted on `task_assets`)
    - `design_assets.current_version_id` is updated
    - task audit/event logs now include:
      - `task.asset.upload_session.created`
      - `task.asset.version.created`
      - `task.asset.upload_session.completed`
      - `task.asset.upload_session.cancelled`
  - MAIN responsibilities stay narrow:
    - task ownership
    - asset / version / upload-session metadata
    - frontend aggregation API
    - audit / operation-log traceability
  - MAIN still does not do:
    - real byte upload proxying as the primary path
    - multipart chunk receive / merge
    - direct NAS disk management
    - PSD / AI parsing
    - preview generation pipeline
- STEP_67:
  - MAIN now has a dedicated task-scoped design asset center preparation layer for later NAS upload-service integration:
    - `design_assets` root records via DB migration `034_v7_design_asset_center_boundary.sql`
    - additive `task_assets` version fields:
      - `asset_id`
      - `asset_version_no`
      - `original_filename`
      - `storage_key`
      - `upload_status`
      - `preview_status`
      - `uploaded_at`
    - additive `upload_requests` session fields:
      - `task_id`
      - `asset_id`
      - `upload_mode`
      - `expected_size`
      - `storage_provider`
      - `session_status`
      - `remote_upload_id`
      - `created_by`
      - `expires_at`
  - MAIN now exposes frontend-linkable asset-center APIs:
    - `GET /v1/tasks/{id}/asset-center/assets`
    - `GET /v1/tasks/{id}/asset-center/assets/{asset_id}/versions`
    - `POST /v1/tasks/{id}/asset-center/upload-sessions/small`
    - `POST /v1/tasks/{id}/asset-center/upload-sessions/multipart`
    - `GET /v1/tasks/{id}/asset-center/upload-sessions/{session_id}`
    - `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/complete`
    - `POST /v1/tasks/{id}/asset-center/upload-sessions/{session_id}/cancel`
  - MAIN now owns stable business metadata and aggregation only:
    - task binding
    - asset / asset-version / upload-session lifecycle
    - audit/task-event traceability for create / complete / cancel
    - remote upload-service client seam and config
  - MAIN still does not do:
    - NAS byte transfer
    - multipart chunk receive/merge
    - local large-file persistence
    - PSD/PSB/AI preview generation
    - thumbnail/preview artifact pipeline
- STEP_66:
  - auth/org mainline now supports fixed department-team registration options through `GET /v1/auth/register-options`
  - register/login/me payloads now additionally carry organization and frontend gating fields:
    - `user.team`
    - compatibility `user.group`
    - `frontend_access.team`
    - explicit `frontend_access.roles`
    - explicit `frontend_access.scopes`
    - explicit `frontend_access.menus`
    - explicit `frontend_access.pages`
    - explicit `frontend_access.actions`
    - explicit `frontend_access.modules`
  - `users.team` is now persisted through DB migration `031_v7_org_team_auth_extension.sql`
  - register validation now additionally enforces:
    - fixed department enum
    - optional team must belong to the selected department
    - mobile required
    - optional email format validation
  - configured super-admin bootstrap remains file-driven and now includes department/team placement
  - HR-visible org/admin read scope is now minimally usable for frontend联调:
    - `GET /v1/roles`
    - `GET /v1/access-rules`
    - `GET /v1/users`
    - `GET /v1/users/{id}`
    - `GET /v1/permission-logs`
    - `GET /v1/operation-logs`
  - `GET /v1/operation-logs` now aggregates:
    - task events
    - export-job events
    - integration call logs
  - ERP query visibility is now explicitly all-authenticated-user scope:
    - `GET /v1/erp/products`
    - `GET /v1/erp/products/{id}`
    - `GET /v1/erp/categories`
- STEP_65:
  - v0.4 legacy compatibility surface inventory is now explicit at repository level
  - remaining old 8080-style public surface is now classified into four states:
    - `keep temporarily`
    - `compatibility only`
    - `replaced by canonical MAIN`
    - `retire after v0.4`
  - the minimum convergence inventory now explicitly names:
    - root operational probes `/health`, `/ping`
    - local ERP-cache compatibility reads `/v1/products/search`, `/v1/products/{id}`
    - legacy sync continuity `/v1/products/sync/status`, `/v1/products/sync/run`, `ERPSyncWorker`
    - compatibility advance route `POST /v1/integration/call-logs/{id}/advance`
    - replaced legacy audit route `POST /v1/audit`
    - post-v0.4 retirement candidates `/v1/sku/*`, `/v1/agent/*`, `/v1/incidents`, `/v1/policies`
    - rollback/runtime continuity assumption `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`
  - no broad legacy route deletion was performed in this step
- STEP_64:
  - v0.4 MAIN versus Bridge ownership language is now aligned at repository level without broad runtime refactoring
  - MAIN is now explicitly documented as the public business application surface:
    - owns public business routes and workflow/business state
    - may expose user-facing Bridge-backed ERP query facade routes under `/v1/erp/*`
  - Bridge is now explicitly documented as the ERP/JST adapter and ERP mutation boundary:
    - owns adapter query semantics
    - owns ERP mutation execution
    - raw ERP mutation APIs do not belong on MAIN
  - the filing write boundary remains intentionally narrow and unchanged:
    - `PATCH /v1/tasks/{id}/business-info` remains the only recognized MAIN-to-Bridge write trigger
    - Bridge `POST /erp/products/upsert` remains Bridge-owned execution
    - no new MAIN mutation surface was added
- STEP_63:
  - v0.4 convergence entrypoint demotion is now explicit without broad code deletion
  - `./cmd/server` remains the only canonical production MAIN entrypoint
  - `./cmd/api` is now explicitly compatibility-only / deprecated:
    - retained on disk only for narrow rollback-safe compatibility during v0.4
    - not a production packaging candidate
    - not a production deploy candidate
  - deployment/release documentation now also clarifies historical context:
    - `deploy/release-history.log` records showing `entrypoint=./cmd/api` on 2026-03-12 are legacy pre-convergence history
    - current production guidance remains `./cmd/server` only
- STEP_62:
  - Linux/bash deployment workflow now supports a true non-disruptive side-by-side validation mode without widening business scope
  - managed deploy entrypoint `deploy/deploy.sh` now accepts `--parallel`
  - parallel deploy behavior is now explicit:
    - deploys into `releases/<version>` without switching live symlinks
    - does not stop live MAIN
    - does not stop live Bridge
    - does not overwrite live shared env files in place
    - starts only a candidate MAIN instance on isolated port `18080` by default
    - keeps the candidate MAIN dependency pinned to the live Bridge loopback at `http://127.0.0.1:8081`
    - writes isolated candidate runtime files for env, pid, log, and deploy state
  - runtime helper hardening for candidate startup is now additive:
    - `deploy/start-main.sh` supports explicit binary/env/pid/log paths
    - `deploy/stop-main.sh` supports explicit pid-file and process-match targeting
    - `deploy/remote-deploy.sh` now distinguishes cutover mode from safe validation mode
  - deployment docs are now explicit about:
    - normal cutover deployment
    - side-by-side validation deployment
    - side-by-side verification without cutover
- STEP_61:
  - Unified deployment and packaging workflow is now standardized from this version forward without adding business features
  - fixed managed release source of truth now exists:
    - `deploy/release-history.log`
    - baseline managed deployment version is `v0.1`
    - future versions auto-increment as `v0.1`, `v0.2`, `v0.3`, ...
  - one-command deploy entrypoint is now explicit:
    - `deploy/deploy.sh`
    - local-only packaging remains available through `deploy/package-local.sh`
  - package/deploy automation is now version-aware and repeatable:
    - builds Linux release binaries as `ecommerce-api` and `erp_bridge`
    - creates one versioned tar.gz artifact under `dist/`
    - uploads over SSH/SCP
    - deploys into predictable Linux release directories under `/root/ecommerce_ai`
    - reuses one stable runtime env file on the server
    - starts the services automatically only when real runtime env files already exist
  - remote runtime helper scripts are now packaged with each release:
    - `deploy/remote-deploy.sh`
    - `deploy/start-main.sh`
    - `deploy/stop-main.sh`
    - `deploy/start-bridge.sh`
    - `deploy/stop-bridge.sh`
    - `deploy/verify-runtime.sh`
  - runtime assumptions remain explicit and unchanged:
    - deployment layout now matches Linux host reality under `/root/ecommerce_ai`
    - MAIN-to-Bridge runtime default remains `http://127.0.0.1:8081`
    - no new CI/CD, orchestration, or supervision platform was introduced
- STEP_60:
  - Narrow integration pass completed against the current Bridge behavior and deployment target assumptions
  - live probe result from this environment remains honest:
    - `223.4.249.11:8081` accepted the TCP connection
    - HTTP probes to Bridge paths returned an empty reply / closed connection instead of JSON
    - public-IP Bridge ingress therefore remains a runtime environment issue, not a main-project contract change
  - deployment truthfulness is improved for same-host rollout:
    - default `ERP_BRIDGE_BASE_URL` now points to `http://127.0.0.1:8081`
    - explicit env override is still supported for non-loopback Bridge routing
    - local package/deploy artifacts are now prepared under `deploy/`
  - local build/package preparation is now explicit:
    - `deploy/package-local.sh`
    - `deploy/main.env.example`
    - `deploy/LOCAL_PACKAGE_DEPLOY.md`
  - regression coverage now also locks the Bridge base-url default/override behavior
- STEP_59:
  - Step E now closes the next narrow audit/warehouse/logging slice without reopening Step A-D
  - audit flow ownership is more coherent and business-safe:
    - `submit-design` now clears stale designer ownership before audit and also supports re-entry from `RejectedByAuditB`
    - audit approve now clears current handler for the next stage instead of leaving stale audit ownership behind
    - audit reject now routes back to the designer handler for truthful rework ownership
    - audit handover now clears current handler until explicit `takeover`
    - pending handovers now block further claim/approve/reject/transfer actions until takeover resolves them
  - warehouse flow is more truthful and reusable:
    - warehouse receive now reuses a previously rejected receipt after task re-prepare instead of dead-ending on receipt existence
    - warehouse receive now sets the current handler to the receiver
    - warehouse reject no longer sends every task to generic `Blocked`
    - purchase-task warehouse reject now returns to `PendingAssign`
    - design/audit task warehouse reject now returns to `RejectedByAuditB` with designer ownership for rework
    - warehouse complete now clears current handler while moving tasks to `PendingClose`
  - workflow/read-model truthfulness improved:
    - `available_actions` now allow resubmit from `RejectedByAuditB`
    - `available_actions` now allow warehouse receive again when a task has been re-prepared after a rejected receipt
    - warehouse sub-status now returns `pending_receive` after re-prepare even if the latest stored receipt was previously rejected
    - purchase-task procurement coordination no longer reports `handed_to_warehouse` just because a rejected warehouse receipt exists
  - task-event traceability is more closed for real debugging:
    - `POST /v1/tasks` now appends `task.created`
    - key task events across create / assign / submit-design / audit / procurement / warehouse / close now carry richer before/after task-status, handler, and result context in payloads
- STEP_58:
  - Step D task entry is now minimally usable across all three task types without widening into Step E
  - `/v1/tasks` create semantics are now explicit by task type and source mode:
    - `original_product_development` remains `existing_product` only and keeps ERP-backed selection binding as the primary path
    - `new_product_development` is `new_product` only, requires `product_name_snapshot`, and auto-generates SKU from the enabled `new_sku` rule when omitted
    - `purchase_task` now supports clean entry without design/audit assumptions; `existing_product` binds selected SKU, `new_product` auto-generates SKU when omitted
  - create-task validation is clearer and machine-readable through additive `error.details.violations`
  - `purchase_task` creation now initializes a draft `procurement_records` row so read/list/detail models expose procurement entry state immediately
  - task entry read models now project a cleaner initial workflow shape:
    - design/audit remain `not_required`/`not_triggered` for purchase entry
    - procurement starts from explicit `draft` / `preparing` instead of null state
- STEP_57:
  - Step C now reaches the narrow business-safe ERP filing boundary on top of the Step A/B auth baseline
  - `PATCH /v1/tasks/{id}/business-info` is now the only Bridge write path:
    - only when `filed_at` is set
    - only for `source_mode=existing_product`
    - only with ERP-backed `product_selection.erp_product`
  - existing-product filing now calls Bridge `POST /erp/products/upsert` before local filing persistence
  - ERP-backed `product_selection` binding is stricter:
    - Bridge selections now always ensure a local cached/bound `products` row
    - caller-supplied `selected_product_id` must match the resolved Bridge binding when both are provided
  - Bridge filing traces now reuse internal integration call logs with connector:
    - `erp_bridge_product_upsert`
    - resource type `task_erp_filing`
  - Bridge filing failures now return structured main-project errors with:
    - retry-hint/upstream context from the Bridge layer
    - additive `integration_call_log_id` when the trace record was created
  - Bridge query surface remains unchanged and query-first:
    - `GET /v1/erp/products`
    - `GET /v1/erp/products/{id}`
    - `GET /v1/erp/categories`

## Completed
- V6 to V7 migration package created
- STEP_01: Product / Task / CodeRule minimal skeleton + DB migration 001
- STEP_02: AuditRecord / AuditHandover / OutsourceOrder / TaskEvent skeleton + DB migration 002
- STEP_02 hardening: task event payloads return raw JSON, `/v1/tasks/:id/events` returns 404 for missing tasks, and `takeover` validates task/handover ownership
- STEP_03:
  - `warehouse_receipts` + DB migration 003
  - `/v1/tasks/:id/detail` aggregate detail API
  - `/v1/tasks/:id/audit/handovers` query API
  - `/v1/warehouse/receipts`
  - `/v1/tasks/:id/warehouse/receive`
  - `/v1/tasks/:id/warehouse/reject`
  - `/v1/tasks/:id/warehouse/complete`
- STEP_04:
  - `task_assets` + DB migration 004
  - task create initial status adjusted to `PendingAssign`
  - `/v1/tasks/:id/assign`
  - `/v1/tasks/:id/submit-design`
  - `/v1/tasks/:id/assets`
  - `/v1/tasks/:id/assets/mock-upload`
- STEP_05:
  - `GET /v1/tasks` query enhancement + unified pagination
  - `GET /v1/tasks/:id/detail` aggregate enhancement with `assets` + `available_actions`
  - `GET /v1/products/search` query enhancement + unified pagination
  - `GET /v1/outsource-orders` query enhancement + unified pagination
  - `GET /v1/warehouse/receipts` query enhancement + unified pagination
- STEP_06:
  - ERP sync placeholder config + stub source
  - `products` batch upsert by `erp_product_id`
  - `erp_sync_runs` + DB migration 005
  - scheduled ERP sync placeholder worker
  - `GET /v1/products/sync/status`
  - `POST /v1/products/sync/run`
- STEP_07:
  - RBAC placeholder role constants
  - request actor / middleware placeholder
  - V7 route readiness + required-role placeholder metadata
  - docs consolidation:
    - `docs/V7_API_READY.md`
    - `docs/V7_FRONTEND_INTEGRATION_ORDER.md`
    - `docs/V7_MODEL_HANDOVER_APPENDIX.md`
- STEP_08:
  - PRD V2.0 task-type contract switched to:
    - `original_product_development`
    - `new_product_development`
    - `purchase_task`
  - task business-info / cost-maintenance fields + DB migration 006
  - `PATCH /v1/tasks/:id/business-info`
  - `POST /v1/tasks/:id/warehouse/prepare`
  - task list/detail workflow projections:
    - `main_status`
    - `sub_status`
    - `warehouse_blocking_reasons`
    - `cannot_close_reasons`
  - purchase-task direct warehouse handoff without designer assignment / design submit / audit
- STEP_09:
  - explicit `PendingClose` task state introduced for the PRD mainline
  - `POST /v1/tasks/:id/close`
  - `GET /v1/tasks/:id` upgraded from base entity to read model with `workflow`
  - `warehouse/complete` now moves tasks to `PendingClose` instead of `Completed`
  - workflow readiness reasons upgraded to structured `{ code, message }` objects
  - `workflow.closable` added while keeping `workflow.can_close` for compatibility
- STEP_10:
  - `workflow.sub_status` upgraded from loose strings to structured `{ code, label, source }`
  - `main_status` / persisted `task_status` / structured `sub_status` responsibilities clarified in docs and OpenAPI
  - `procurement_records` + DB migration 007
  - `PATCH /v1/tasks/:id/procurement`
  - `GET /v1/tasks/:id` and `GET /v1/tasks/:id/detail` now expose nullable `procurement`
  - purchase-task warehouse/close readiness now depends on dedicated procurement persistence instead of `task_details.procurement_price`
- STEP_11:
  - `GET /v1/tasks` now supports projected `main_status` filtering
  - `GET /v1/tasks` now supports structured `sub_status_code` filtering, with optional `sub_status_scope`
  - `workflow.sub_status` contract is kept aligned across `/v1/tasks`, `/v1/tasks/:id`, and `/v1/tasks/:id/detail`
  - `procurement_summary` now returns on list/read/detail task views
  - `procurement_records` upgraded with quantity + DB migration 008
  - purchase-task procurement lifecycle upgraded from readiness skeleton to minimal flow:
    - `draft`
    - `prepared`
    - `in_progress`
    - `completed`
  - `POST /v1/tasks/:id/procurement/advance`
  - purchase-task warehouse readiness now requires procurement quantity plus procurement status allowing warehouse handoff
  - purchase-task close readiness now requires procurement completion while preserving `close` / `closable` / `cannot_close_reasons`
- STEP_12:
  - `procurement_summary` now exposes derived procurement-to-warehouse coordination fields on list/read/detail task views:
    - `coordination_status`
    - `coordination_label`
    - `warehouse_status`
    - `warehouse_prepare_ready`
    - `warehouse_receive_ready`
  - purchase-task warehouse prepare is now gated by procurement completion instead of any warehouse-eligible procurement activity
  - purchase-task procurement/warehouse coordination now explicitly distinguishes:
    - awaiting arrival
    - ready for warehouse
    - handed to warehouse
  - projected procurement sub-status semantics were aligned with the stricter service-layer coordination rules
- STEP_13:
  - frontend-ready task-board / inbox aggregation added:
    - `GET /v1/task-board/summary`
    - `GET /v1/task-board/queues`
  - task-board responses now expose stable queue metadata:
    - `queue_key`
    - `queue_name`
    - `queue_description`
    - `filters`
    - `count`
  - `/v1/task-board/summary` now returns sample tasks per preset queue
  - `/v1/task-board/queues` now returns paginated task lists per preset queue
  - minimal role-oriented queue pools now exist for:
    - operations
    - designer
    - audit
    - procurement
    - warehouse
  - preset queues now cover:
    - `ops_pending_material`
    - `design_pending_submit`
    - `audit_pending_review`
    - `procurement_pending_followup`
    - `awaiting_arrival`
    - `warehouse_pending_prepare`
    - `warehouse_pending_receive`
    - `pending_close`
- STEP_14:
  - task list / task-board filters now converge on a shared task query filter contract
  - `GET /v1/tasks` now additionally supports board-reusable filter dimensions:
    - `coordination_status`
    - `warehouse_prepare_ready`
    - `warehouse_receive_ready`
    - `warehouse_blocking_reason_code`
  - `GET /v1/tasks` query params now accept multi-value board/list convergence semantics for:
    - `status`
    - `task_type`
    - `source_mode`
    - `main_status`
    - `sub_status_code`
    - `coordination_status`
    - `warehouse_blocking_reason_code`
  - task-board queue payloads now expose stable board-to-list handoff fields:
    - `normalized_filters`
    - `query_template`
  - preset queue definitions now reuse the same list-side filter matcher used by `/v1/tasks`
  - board-level query parsing now reuses the same filter field names and semantics as task list queries
- STEP_15:
  - converged `/v1/tasks` filters now execute through one direct repo/read-model query path instead of service-layer segmented fan-out
  - repo-level multi-value predicates now directly cover:
    - `status`
    - `task_type`
    - `source_mode`
    - `main_status`
    - `sub_status_code`
    - `coordination_status`
    - `warehouse_blocking_reason_code`
  - derived read-model predicate pushdown now covers:
    - `coordination_status`
    - `warehouse_prepare_ready`
    - `warehouse_receive_ready`
    - `warehouse_blocking_reason_code`
  - board/list external filter names and semantics remain stable; `normalized_filters` and `query_template` contracts do not change
- STEP_16:
  - task-board preset aggregation now builds summary and queue payloads from one shared board-level candidate task pool instead of calling the list path once per preset queue
  - task-board summary and task-board queues now share one aggregation implementation for:
    - preset filtering
    - counts
    - sample-task selection
    - per-queue paginated task slicing
  - board summary, board queues, and `/v1/tasks` now share one consistent filter interpretation path:
    - repo/read-model predicates constrain the base candidate pool
    - preset queues reuse the same filter matcher semantics for final board partitioning
  - external board contracts stay stable:
    - `queue_key`
    - `queue_name`
    - `filters`
    - `normalized_filters`
    - `query_template`
    - `count`
    - sample tasks / queue task lists
- STEP_17:
  - broad task-board candidate scans now use a dedicated repo/read-model-backed candidate scan entry instead of paging through the generic `/v1/tasks` list path
  - the new board candidate scan pushes down:
    - shared global board/list filters
    - the union of selected preset queue predicates
  - task-board summary and task-board queues still share one candidate pool per request, but that pool is now narrowed before service-level partitioning
  - remaining task-board fan-out is now explicitly bounded as business-required final queue shaping:
    - overlapping preset queue membership
    - stable counts
    - sample-task selection
    - per-queue pagination slicing
  - external board/list contracts remain stable:
    - `queue_key`
    - `filters`
    - `normalized_filters`
    - `query_template`
    - `count`
    - sample tasks / queue task lists
- STEP_18:
  - candidate-scan and read-model cost assessment is now recorded for:
    - `GET /v1/tasks`
    - `GET /v1/task-board/summary`
    - `GET /v1/task-board/queues`
  - current hotspot classification now explicitly distinguishes:
    - immediate light optimization candidates
    - later index / projection / materialization candidates
    - acceptable derived SQL / projection debt
  - the heaviest remaining candidate-scan predicates are now documented as:
    - unscoped `sub_status_code`
    - `warehouse_blocking_reason_code`
    - `warehouse_prepare_ready`
  - a small internal optimization now removes repeated latest-asset scalar subqueries by joining one latest task-asset projection per task
  - public board/list/filter/query-template contracts remain unchanged
- STEP_19:
  - preset task-board queues now expose lightweight ownership-hint metadata:
    - `suggested_roles`
    - `suggested_actor_type`
    - `default_visibility`
    - `ownership_hint`
  - queue ownership metadata is advisory only and does not enforce auth, permissions, assignment, or queue visibility
  - lightweight saved workbench preferences are now available through:
    - `GET /v1/workbench/preferences`
    - `PATCH /v1/workbench/preferences`
  - saved preferences are now scoped to the current session-backed user on the mainline HTTP path
  - workbench preferences currently support:
    - `default_queue_key`
    - `pinned_queue_keys`
    - `default_filters`
    - `default_page_size`
    - `default_sort`
  - workbench bootstrap responses now include direct config for:
    - filters schema
    - supported page sizes / sorts
    - preset queue metadata with board/list drill-down fields and ownership hints
- STEP_20:
  - configurable category center skeleton added with dedicated persistence, admin APIs, and sample initialization mappings
  - configurable cost-rule center skeleton added with dedicated persistence, admin APIs, sample initialization mappings, and minimal preview support
  - coded-style values such as `HBJ/HBZ/HCP/HLZ/HPJ/HQT/HSC/HZS` are now treated as valid first-level category entries instead of free-text noise
  - `task_details` now carries standard category linkage and cost-rule provenance fields:
    - `category_id`
    - `category_code`
    - `category_name`
    - `cost_rule_id`
    - `cost_rule_name`
    - `cost_rule_source`
  - `PATCH /v1/tasks/:id/business-info` can now associate tasks with total category codes and internal cost-rule provenance without moving those concerns into procurement or remarks
  - category / cost-rule sample mappings are now available in:
    - `config/category_seed.json`
    - `config/cost_rule_seed.json`
  - minimal category center APIs:
    - `GET /v1/categories`
    - `GET /v1/categories/search`
    - `GET /v1/categories/:id`
    - `POST /v1/categories`
    - `PATCH /v1/categories/:id`
  - minimal cost-rule center APIs:
    - `GET /v1/cost-rules`
    - `GET /v1/cost-rules/:id`
    - `GET /v1/cost-rules/:id/history`
    - `POST /v1/cost-rules`
    - `PATCH /v1/cost-rules/:id`
    - `POST /v1/cost-rules/preview`
  - preview currently supports skeleton evaluation for:
    - `fixed_unit_price`
    - `area_threshold_surcharge`
    - `minimum_billable_area`
    - `special_process_surcharge`
    - limited `size_based_formula` (`print_side:*`)
    - `manual_quote`
- STEP_21:
  - `PATCH /v1/tasks/:id/business-info` now supports direct category-to-cost-prefill usage with minimal preview inputs:
    - `width`
    - `height`
    - `area`
    - `quantity`
    - `process`
  - task-side persisted cost-prefill / override boundary added on `task_details`:
    - `estimated_cost`
    - `requires_manual_review`
    - `manual_cost_override`
    - `manual_cost_override_reason`
  - business-info update now reuses skeleton cost-preview behavior so category plus minimal inputs can prefill internal `cost_price`
  - purchase-task read/list/detail procurement summaries now surface:
    - `category_code`
    - `category_name`
    - `cost_price`
    - `estimated_cost`
    - `cost_rule_name`
    - `cost_rule_source`
    - `requires_manual_review`
    - `manual_cost_override`
    - `manual_cost_override_reason`
  - system prefill and manual override are now explicit business data behaviors; they are not treated as permission controls
- STEP_22:
  - category center now explicitly models first-level ERP search-entry semantics through:
    - `search_entry_code`
    - `is_search_entry`
  - “总分类编码 = 一级搜索入口” is now a persisted category contract instead of a documentation-only rule
  - independent ERP-positioning skeleton added through:
    - `category_erp_mappings`
  - reserved second/third-level refinement fields now exist on mappings:
    - `secondary_condition_key`
    - `secondary_condition_value`
    - `tertiary_condition_key`
    - `tertiary_condition_value`
  - sample category-to-ERP mapping skeleton records are now available in:
    - `config/category_erp_mapping_seed.json`
  - minimal category-mapping APIs:
    - `GET /v1/category-mappings`
    - `GET /v1/category-mappings/search`
    - `GET /v1/category-mappings/:id`
    - `POST /v1/category-mappings`
    - `PATCH /v1/category-mappings/:id`
- STEP_23:
  - `GET /v1/products/search` now supports mapped local ERP positioning through:
    - `category_id`
    - `category_code`
    - `search_entry_code`
    - `mapping_match`
    - lightweight reserved `secondary_key/secondary_value`
    - lightweight reserved `tertiary_key/tertiary_value`
  - product search now consumes active local `category_erp_mappings` instead of only legacy `products.category LIKE`
  - category search flow is now executable as:
    - selected category -> `search_entry_code`
    - active local mappings under that first-level entry
    - local ERP product narrowing over synced `products`
  - current mapping-consumption rule is:
    - default to primary mappings when mapped search is used
    - allow explicit `mapping_match=all` for broader active-rule consumption
    - prefer exact category mappings when present
    - otherwise fall back to search-entry-wide mappings
  - product search results now expose positioning provenance through:
    - `matched_category_code`
    - `matched_search_entry_code`
    - `matched_mapping_rule`
- STEP_24:
  - original-product task entry now accepts additive `product_selection` provenance on:
    - `POST /v1/tasks`
    - `PATCH /v1/tasks/:id/business-info`
  - existing-product task selection can now persist:
    - selected product identity and SKU snapshot
    - `matched_category_code`
    - `matched_search_entry_code`
    - `matched_mapping_rule`
    - `source_match_type`
    - `source_match_rule`
    - `source_search_entry_code`
  - task read/detail contracts now expose top-level `product_selection` for existing-product traceability
  - business-info updates may now rebind existing-product tasks while keeping mapped-search provenance on `task_details`
  - legacy existing-product tasks without mapped provenance still expose a minimal fallback trace context instead of returning nothing
- STEP_25:
  - `product_selection` is now a first-class task read-model object instead of a detail-only add-on
  - `GET /v1/tasks` now returns lightweight task-item `product_selection` summary
  - `GET /v1/task-board/summary` and `GET /v1/task-board/queues` now surface the same task-item `product_selection` summary through sample tasks and queue tasks
  - `procurement_summary` now also exposes lightweight `product_selection` summary for purchase-facing pages
  - `GET /v1/tasks/:id` and `GET /v1/tasks/:id/detail` keep full `product_selection` provenance with `matched_mapping_rule`
  - frontend no longer needs to reconstruct original-product provenance from scattered `matched_*` / `source_*` fields
- STEP_26:
  - export center skeleton added with dedicated `export_jobs` persistence + DB migration 014
  - frontend-ready export-center APIs added:
    - `GET /v1/export-templates`
    - `POST /v1/export-jobs`
    - `GET /v1/export-jobs`
    - `GET /v1/export-jobs/:id`
  - export jobs now persist:
    - `export_job_id`
    - `template_key`
    - `export_type`
    - `source_query_type`
    - `source_filters`
    - `normalized_filters`
    - `query_template`
    - `requested_by`
    - `status`
    - `result_ref`
    - `created_at`
    - `finished_at`
    - `remark`
  - current export-center sources are:
    - task list query state
    - task-board queue handoff state (`queue_key` + `query_template` / `normalized_filters`)
    - procurement-summary task query state
    - warehouse receipt list filters
  - `result_ref` is now an explicit placeholder metadata contract only; it is not a real file-system, NAS, or object-storage integration
- STEP_27:
  - export job lifecycle skeleton added with explicit status progression:
    - `queued`
    - `running`
    - `ready`
    - `failed`
    - `cancelled`
  - internal/admin lifecycle advancement endpoint added:
    - `POST /v1/export-jobs/:id/advance`
  - export jobs now persist explicit lifecycle timestamp through DB migration 015:
    - `status_updated_at`
  - export-job list/detail views now expose stable frontend lifecycle read fields:
    - `progress_hint`
    - `latest_status_at`
    - `download_ready`
  - `result_ref` is now a structured placeholder download-handoff object:
    - `ref_type`
    - `ref_key`
    - `file_name`
    - `mime_type`
    - `expires_at`
    - `is_placeholder`
    - `note`
  - export center now provides a minimal closed loop without real storage:
    - created as `queued`
    - manually advanced to `running`
    - manually advanced to `ready` or `failed`
    - exposes placeholder download handoff metadata when ready
- STEP_28:
  - export job lifecycle audit trace added with DB migration 016:
    - `export_job_events`
    - `export_job_event_sequences`
  - export-job lifecycle actions now append durable timeline events in the same transaction:
    - `export_job.created`
    - `export_job.advanced_to_running`
    - `export_job.advanced_to_ready`
    - `export_job.advanced_to_failed`
    - `export_job.advanced_to_cancelled`
    - `export_job.advanced_to_queued`
    - `export_job.result_ref_updated`
  - frontend-ready timeline query added:
    - `GET /v1/export-jobs/:id/events`
  - export-job list/detail contracts now expose lightweight audit summaries:
    - `event_count`
    - `latest_event`
  - event payload is audit context only:
    - not a full runner log
    - not a storage-delivery log
    - not proof of real file generation
- STEP_29:
  - placeholder download claim/read boundary added for ready export jobs:
    - `POST /v1/export-jobs/:id/claim-download`
    - `GET /v1/export-jobs/:id/download`
  - claim/read now return structured placeholder handoff metadata including:
    - `export_job_id`
    - `result_ref`
    - `file_name`
    - `mime_type`
    - `is_placeholder`
    - `expires_at`
    - `download_ready`
    - `note`
  - claim/read now reuse `export_job_events` instead of creating a separate audit log:
    - `export_job.download_claimed`
    - `export_job.download_read`
  - claim/read remain ready-only placeholder handoff actions:
    - not file-byte download
    - not signed URL delivery
    - not NAS or object-storage integration
- STEP_30:
  - placeholder handoff expiry/refresh semantics added for ready export jobs:
    - `POST /v1/export-jobs/:id/refresh-download`
  - claim/read now require ready and not-expired handoff:
    - ready + not expired => claim/read allowed
    - ready + expired => claim/read rejected with refresh-required placeholder semantics
    - non-ready => claim/read still rejected
  - export-job list/detail now expose handoff state hints:
    - `is_expired`
    - `can_refresh`
  - handoff responses now expose enforced expiry/refresh state:
    - `is_expired`
    - `can_refresh`
  - expiry/refresh continue to reuse `export_job_events`:
    - `export_job.download_expired`
    - `export_job.download_refreshed`
    - `export_job.result_ref_updated` remains the material handoff-change event on refresh
- STEP_31:
  - explicit placeholder runner-initiation boundary added for export jobs:
    - `POST /v1/export-jobs/:id/start`
  - `queued -> running` is now formalized as a start contract:
    - only `queued` jobs can start
    - `running|ready|failed|cancelled` cannot be started again
    - `POST /v1/export-jobs/:id/advance` `action=start` now reuses the same start helper for backward compatibility
  - export-job list/detail now expose runner-initiation hints:
    - `can_start`
    - `start_mode`
    - `execution_mode`
    - `latest_runner_event`
  - export-job event chain now includes explicit start-boundary events:
    - `export_job.runner_initiated`
    - `export_job.started`
    - `export_job.advanced_to_running` remains for backward-compatible lifecycle consumers
- STEP_32:
  - placeholder execution-attempt visibility added for export jobs:
    - `export_job_attempts` + DB migration 017
    - `GET /v1/export-jobs/:id/attempts`
  - export-job list/detail now expose:
    - `attempt_count`
    - `latest_attempt`
    - `can_retry`
  - `/start` now creates one durable attempt record per successful initiation
  - running-attempt terminal results are now written separately from job lifecycle through:
    - `export_job.attempt_succeeded`
    - `export_job.attempt_failed`
    - `export_job.attempt_cancelled`
  - placeholder runner-adapter visibility is now explicit on attempt records through:
    - `trigger_source`
    - `execution_mode`
    - `adapter_key`
    - `adapter_note`
- STEP_33:
  - placeholder adapter-dispatch handoff added for export jobs:
    - `export_job_dispatches` + DB migration 018
    - `dispatch_id` added to `export_job_attempts`
    - `GET /v1/export-jobs/:id/dispatches`
    - `POST /v1/export-jobs/:id/dispatches`
    - `POST /v1/export-jobs/:id/dispatches/:dispatch_id/advance`
  - dispatch state is now explicitly modeled as:
    - `submitted`
    - `received`
    - `rejected`
    - `expired`
    - `not_executed`
  - export-job event chain now also records dispatch handoff events through:
    - `export_job.dispatch_submitted`
    - `export_job.dispatch_received`
    - `export_job.dispatch_rejected`
    - `export_job.dispatch_expired`
    - `export_job.dispatch_not_executed`
  - `/start` now consumes a received dispatch or auto-creates one placeholder submitted/received handoff for backward-compatible initiation
- STEP_34:
  - export-job list/detail read models now also expose:
    - `dispatch_count`
    - `latest_dispatch`
    - `can_dispatch`
    - `can_redispatch`
    - `latest_dispatch_event`
  - `can_start` is now hardened to respect blocking submitted dispatch state instead of only checking job lifecycle status
  - export-job read models now explicitly separate:
    - job lifecycle state
    - latest dispatch handoff state
    - latest execution-attempt state
  - current dispatch/attempt read-model integration remains placeholder-only and does not imply a real scheduler queue, adapter callback, or worker lease
- STEP_35:
  - integration center / API call log skeleton added:
    - `integration_call_logs` + DB migration 019
    - `GET /v1/integration/connectors`
    - `POST /v1/integration/call-logs`
    - `GET /v1/integration/call-logs`
    - `GET /v1/integration/call-logs/:id`
    - `POST /v1/integration/call-logs/:id/advance`
  - current integration call-log lifecycle is:
    - `queued`
    - `sent`
    - `succeeded`
    - `failed`
    - `cancelled`
  - integration call-log read models now expose:
    - `progress_hint`
    - `latest_status_at`
    - `started_at`
    - `finished_at`
    - `can_replay`
  - current connector catalog is static, with mostly placeholder connectors plus one narrow ERP filing trace connector:
    - `erp_product_stub`
    - `erp_bridge_product_upsert`
    - `export_adapter_bridge`
- STEP_36:
  - placeholder auth / RBAC route boundary is now hardened over current debug actor headers
  - V7 routes carrying `withAccessMeta(...)` now reject actors that do not satisfy route `required_roles`
  - current auth mode is now:
    - `debug_header_role_enforced`
  - current mainline requests without bearer or explicit debug headers no longer synthesize request actor ID `1`
  - `system_fallback` remains limited to explicit placeholder helpers and non-request fallback usage
  - route-level role checks currently allow:
    - any explicitly required role
    - `Admin` as a placeholder override
  - current hardening remains intentionally narrow:
    - no login/session system
    - no org tree
    - no department/team visibility trimming
    - no task-level data scope policy
- STEP_37:
  - placeholder asset storage/upload boundary added with DB migration 020:
    - `upload_requests`
    - `asset_storage_refs`
  - internal placeholder upload-intent APIs added:
    - `POST /v1/assets/upload-requests`
    - `GET /v1/assets/upload-requests/:id`
  - `task_assets` now add additive boundary fields:
    - `upload_request_id`
    - `storage_ref_id`
    - `mime_type`
    - `file_size`
    - nested `storage_ref`
  - `POST /v1/tasks/:id/submit-design` and `POST /v1/tasks/:id/assets/mock-upload` may now consume `upload_request_id`
  - task-asset writes now auto-create placeholder `storage_ref` metadata even when no upload request is supplied
  - legacy `file_path` / `whole_hash` remain accepted for compatibility and are now treated as migration-era metadata rather than the long-term storage boundary
- STEP_38:
  - integration execution boundary added with DB migration 021:
    - `integration_call_executions`
  - internal placeholder execution APIs added:
    - `GET /v1/integration/call-logs/:id/executions`
    - `POST /v1/integration/call-logs/:id/executions`
    - `POST /v1/integration/call-logs/:id/executions/:execution_id/advance`
  - integration call-log read models now additionally expose:
    - `execution_count`
    - `latest_execution`
    - `can_retry`
    - backward-compatible `can_replay`
  - call-log lifecycle and execution lifecycle are now explicitly layered:
    - call log = request envelope / business call intent
    - execution = one placeholder execution attempt beneath that call log
  - `POST /v1/integration/call-logs/:id/advance` remains for compatibility but now reuses execution semantics for `sent|succeeded|failed|cancelled`
- STEP_39:
  - export runner / storage / delivery planning boundary hardened additively on export-job read models
  - export-job list/detail now additionally expose:
    - `adapter_mode`
    - `storage_mode`
    - `delivery_mode`
    - `execution_boundary`
    - `storage_boundary`
    - `delivery_boundary`
  - current responsibility layering is now explicit:
    - start execution = `POST /v1/export-jobs/:id/start`
    - dispatch handoff = `export_job_dispatches`
    - one concrete execution try = `export_job_attempts`
    - placeholder result generation = export-job lifecycle advance into terminal placeholder states
    - placeholder storage representation = `result_ref`
    - placeholder delivery handoff = claim/read/refresh routes
  - future replacement seams are now documented in code and OpenAPI without introducing real runner, storage, or download infrastructure
- STEP_40:
  - cost-rule governance / versioning / override hardening added on top of the existing configurable skeleton
  - `cost_rules` now expose governed version lineage and effective-window semantics through:
    - `rule_version`
    - `supersedes_rule_id`
    - `superseded_by_rule_id`
    - `governance_note`
    - `governance_status`
  - `POST /v1/cost-rules/preview` now additionally returns:
    - `matched_rule_id`
    - `matched_rule_version`
    - `governance_status`
  - task-side cost-prefill persistence on `task_details` now explicitly snapshots:
    - `matched_rule_version`
    - `prefill_source`
    - `prefill_at`
  - manual override persistence on `task_details` now explicitly snapshots:
    - `override_actor`
    - `override_at`
  - purchase-task `procurement_summary` now surfaces those additive cost-governance trace fields
  - current history policy is now explicit:
    - new rule changes affect future preview / future prefill only
    - existing task snapshots are not auto-recomputed
- STEP_41:
  - cost-governance audit / history read-model hardening added on top of the Step 40 governed skeleton
  - `cost_rules` now additionally expose derived lineage read models:
    - `version_chain_summary`
    - `previous_version`
    - `next_version`
    - `supersession_depth`
  - new read-only cost-rule lineage endpoint:
    - `GET /v1/cost-rules/:id/history`
  - `GET /v1/tasks/:id`, `GET /v1/tasks/:id/detail`, and purchase-task `procurement_summary` now additionally expose:
    - `matched_rule_governance`
    - `override_summary`
  - task-side cost governance reads now explicitly separate:
    - the historical matched rule snapshot
    - the current latest rule in that lineage
    - the lightweight override summary derived from task business-info events
  - Step 40 write semantics are unchanged:
    - new rule changes affect future preview / future prefill only
    - existing task snapshots are not auto-recomputed
- STEP_42:
  - override / governance audit stream hardening added on top of the Step 40 / Step 41 governed skeleton
  - dedicated governance audit persistence added through:
    - `cost_override_events`
    - `cost_override_event_sequences`
  - `PATCH /v1/tasks/:id/business-info` now appends dedicated override audit events when override state changes while keeping `task_event_logs` as the general task event layer
  - dedicated read-only task override timeline endpoint added:
    - `GET /v1/tasks/:id/cost-overrides`
  - `GET /v1/tasks/:id`, `GET /v1/tasks/:id/detail`, and purchase-task `procurement_summary` now additionally expose:
    - `governance_audit_summary`
  - `override_summary` is preserved as the stable lightweight summary contract, but it now prefers the dedicated override audit stream when available and falls back to `task_event_logs` for older rows
  - current boundary remains explicit:
    - this is a governance-audit skeleton
    - not a real approval flow
    - not a finance system
    - not an ERP cost writeback layer
- STEP_43:
  - approval / finance placeholder boundary added above the dedicated override audit skeleton
  - dedicated placeholder persistence added through:
    - `cost_override_reviews`
    - `cost_override_finance_flags`
  - `GET /v1/tasks/:id`, `GET /v1/tasks/:id/detail`, `GET /v1/tasks/:id/cost-overrides`, and purchase-task `procurement_summary` now additionally expose:
    - `override_governance_boundary`
  - task-side governance layering is now explicit:
    - rule lineage / history = cost-rule governance layer
    - matched rule snapshot = task-side historical hit layer
    - override summary = stable lightweight consumer summary
    - governance audit summary / timeline = dedicated override audit layer
    - approval / finance placeholder = post-override governance handoff boundary
  - minimal internal placeholder write actions added:
    - `POST /v1/tasks/:id/cost-overrides/:event_id/review`
    - `POST /v1/tasks/:id/cost-overrides/:event_id/finance-mark`
  - current boundary remains explicit:
    - not a real approval workflow
    - not a real finance / accounting system
    - not an ERP cost writeback layer
    - not settlement / reconciliation / invoice capability
- STEP_44:
  - cost governance boundary read-model consolidation completed above the Step 43 placeholder write boundary
  - `override_governance_boundary` now carries one unified frontend-readable read model across `GET /v1/tasks/:id`, `GET /v1/tasks/:id/detail`, purchase-task `procurement_summary`, and `GET /v1/tasks/:id/cost-overrides`
  - the unified boundary now additionally exposes:
    - `governance_boundary_summary`
    - `approval_placeholder_summary`
    - `finance_placeholder_summary`
    - `latest_review_action`
    - `latest_finance_action`
    - `latest_boundary_actor`
    - `latest_boundary_at`
  - current read-model layering is now stable and explicit:
    - rule history / lineage = rule governance layer
    - matched rule snapshot / prefill trace = task-side historical hit layer
    - override audit = `cost_override_events`
    - approval placeholder = `cost_override_reviews`
    - finance placeholder = `cost_override_finance_flags`
    - boundary summary = unified read-model aggregation over approval / finance placeholder layers
  - current boundary remains explicit:
    - not a real approval workflow
    - not a finance / accounting / settlement system
    - not an ERP cost writeback layer
- STEP_45:
  - cross-center adapter-boundary consolidation completed across export center, integration center, and asset storage/upload placeholder boundaries
  - shared terminology is now explicit without merging center tables:
    - `adapter_mode`
    - `execution_mode`
    - `dispatch_mode`
    - `storage_mode`
    - `delivery_mode`
  - shared minimal read-model summaries now exist for cross-center reuse:
    - `adapter_ref_summary`
    - `resource_ref_summary`
    - `handoff_ref_summary`
  - current reuse relationship is now explicit:
    - export job keeps lifecycle / dispatch / attempt / result / delivery semantics, and now adds unified `dispatch_mode` plus shared adapter/resource/handoff summaries
    - integration call log / execution keep connector + execution semantics, and now reuse shared adapter/handoff summaries plus unified `adapter_mode` / `dispatch_mode`
    - upload requests and `asset_storage_refs` keep storage-center-specific semantics, and now reuse shared adapter/resource/handoff summaries plus unified `adapter_mode` / `dispatch_mode` / `storage_mode`
  - current consolidation remains read-model/documentation language only:
    - not a real runner platform
    - not real file upload or storage
    - not real external execution
    - not a table merge across centers
- STEP_46:
  - upload-request placeholder lifecycle hardened without introducing real upload/storage infrastructure
  - internal placeholder upload-request lifecycle API added:
    - `POST /v1/assets/upload-requests/:id/advance`
  - upload requests now expose additive lifecycle readiness fields:
    - `can_bind`
    - `can_cancel`
    - `can_expire`
  - explicit internal lifecycle actions now exist for:
    - `cancel`
    - `expire`
  - `bound` remains reserved for task-asset binding only:
    - `POST /v1/tasks/:id/submit-design`
    - `POST /v1/tasks/:id/assets/mock-upload`
  - current lifecycle hardening remains placeholder-only:
    - not a real byte upload session
    - not a signed URL flow
    - not NAS or object-storage integration
    - not a background expiry worker
- STEP_47:
  - integration execution replay / retry hardening added without introducing real external execution
  - internal placeholder execution action APIs added:
    - `POST /v1/integration/call-logs/:id/retry`
    - `POST /v1/integration/call-logs/:id/replay`
  - retry / replay now stay layered on the same execution boundary:
    - `retry` = create a new execution attempt only when the latest visible failed execution is retryable
    - `replay` = create a new execution attempt to re-drive the recorded call-log envelope, including succeeded or cancelled outcomes
  - integration executions now additionally expose:
    - `action_type`
  - integration call-log read models now additionally expose:
    - `retry_count`
    - `replay_count`
    - `latest_retry_action`
    - `latest_replay_action`
    - `retryability_reason`
    - `replayability_reason`
  - current history relation remains execution-centric:
    - no new retry/replay event stream was added
    - retry/replay traceability currently reuses persisted `integration_call_executions` plus action-specific `trigger_source` / `action_type`
  - current hardening remains placeholder-only:
    - not a real ERP / HTTP / SDK executor
    - not a callback processor
    - not a retry scheduler
    - not a signature/auth negotiation layer
    - not a message queue or async worker platform
- STEP_48:
  - export dispatch / attempt admission hardening added without introducing real scheduler or runner infrastructure
  - export-job list/detail read models now additionally expose admission-reason fields:
    - `can_start_reason`
    - `can_attempt`
    - `can_attempt_reason`
    - `can_dispatch_reason`
    - `can_redispatch_reason`
    - `dispatchability_reason`
    - `attemptability_reason`
    - `latest_admission_decision`
  - dispatch / attempt list records now additionally expose minimal admission hints:
    - dispatch:
      - `start_admissible`
      - `start_admission_reason`
    - attempt:
      - `blocks_new_attempt`
      - `next_attempt_admission_reason`
  - admission rules are now explicitly machine-readable across queued/running/ready/failed/cancelled states:
    - dispatch admission remains queued-only and blocks on unresolved latest `submitted|received` dispatch
    - start/attempt admission now explicitly reports compatibility-path semantics:
      - latest `received` dispatch can be consumed directly
      - no startable dispatch may still trigger compatibility auto-placeholder dispatch creation at start boundary
  - internal invalid-state errors now include admission reason details consistent with list/detail read models
  - current hardening remains placeholder-only:
    - not a real async runner or scheduler
    - not a real queue/lease/heartbeat system
    - not real file generation/storage/download infrastructure
- STEP_49:
  - unified auth/org/visibility policy scaffolding added on top of existing route-level placeholder auth enforcement
  - cross-center reusable policy structures now exist:
    - `policy_scope_summary`
    - `resource_access_policy`
    - `action_policy_summary`
  - task/export/integration/cost/upload read models now additively expose:
    - `policy_mode`
    - `visible_to_roles`
    - `action_roles`
    - `policy_scope_summary`
  - task center coverage now includes:
    - task list/read/detail
    - task-board summary/queues
    - procurement summary
  - export center coverage now includes:
    - export job policy summary with frontend/internal/admin action-role hints
  - integration center coverage now includes:
    - call-log and execution policy summaries
  - cost governance coverage now includes:
    - override governance boundary policy summary
  - upload/storage boundary coverage now includes:
    - upload request policy summary
  - existing route-level role contract remains in place:
    - preferred auth is bearer session
    - debug compatibility headers remain `X-Debug-Actor-Id`, `X-Debug-Actor-Roles`
    - route metadata middleware remains `withAccessMeta(...)`
    - route-level role enforcement behavior is preserved
  - current phase remains scaffolding-only:
    - not a real identity/login/SSO system
    - not a real org sync or hierarchy policy engine
    - not final fine-grained RBAC/ABAC
    - not a full approval permission system redesign
- STEP_50:
  - unified KPI/finance/report platform entry boundary scaffolding added across task/procurement/cost governance/export read models
  - reusable cross-center entry structures now exist:
    - `platform_entry_boundary`
    - `kpi_entry_summary`
    - `finance_entry_summary`
    - `report_entry_summary`
  - task center read-model coverage now includes:
    - task list item
    - task read model
    - task detail aggregate
  - procurement/cost/export coverage now includes:
    - `procurement_summary`
    - `override_governance_boundary`
    - `export_job`
  - each entry summary now explicitly distinguishes:
    - existing source read-model fields
    - placeholder-only future platform fields
    - current `eligible_now` boundary readiness
  - this phase remains entry-boundary scaffolding only:
    - not real KPI/BI computation
    - not real finance/accounting/reconciliation/settlement/invoice workflows
    - not a real report-generation platform
- STEP_51:
  - post-Step-50 prioritization audit completed in `docs/phases/PHASE_AUDIT_051.md`
  - upload/storage placeholder management deepened through:
    - `GET /v1/assets/upload-requests`
  - upload-request internal management filters now support:
    - `owner_type`
    - `owner_id`
    - `task_asset_type`
    - `status`
  - upload-request management remains internal placeholder only:
    - no real upload session allocator
    - no signed URL system
    - no NAS / object storage
    - no byte-transfer confirmation
    - not a real data warehouse or analytics engine
- STEP_52:
  - mainline priority corrected back to Step A / Step B instead of continuing placeholder platform deepening
  - minimal real identity/auth support added with DB migration 025:
    - `users`
    - `user_roles`
    - `user_sessions`
    - `permission_logs`
  - added auth APIs:
    - `POST /v1/auth/register`
    - `POST /v1/auth/login`
    - `GET /v1/auth/me`
  - added user / role / permission admin APIs:
    - `GET /v1/roles`
    - `GET /v1/users`
    - `GET /v1/users/:id`
    - `PATCH /v1/users/:id`
    - `PUT /v1/users/:id/roles`
    - `GET /v1/permission-logs`
  - request actor resolution now prefers bearer session tokens and still keeps debug-header compatibility
  - route-level permission checks now append durable permission access logs
  - key task-flow write APIs now default actor ids from the authenticated request actor instead of forcing explicit request-body ids
- STEP_56:
  - Step B minimal route/role clarity and auditability hardening added with DB migration 027:
    - persisted permission-log route policy context
    - actor-username and method-searchable permission-log reads
  - ready-for-frontend route checks now require session-backed actors before role matching
  - internal/mock placeholder route checks still allow debug-header compatibility
  - admin inspection API now exposes the protected route-role contract:
    - `GET /v1/access-rules`
  - workbench preferences are now fully session-backed on both route and service paths
- STEP_57:
  - Step C now reaches the narrow ERP filing boundary without expanding into broad ERP docking
  - `PATCH /v1/tasks/{id}/business-info` now treats `filed_at` as the only Bridge product-upsert boundary:
    - applies only to `source_mode=existing_product`
    - requires ERP-backed `product_selection.erp_product`
    - calls Bridge `POST /erp/products/upsert` before local filing persistence
  - Bridge-backed `product_selection` binding is stricter:
    - Bridge selections now always ensure/bind a local `products` row even if caller also sends `selected_product_id`
    - mismatched `selected_product_id` plus ERP snapshot is now rejected instead of being silently trusted
  - Bridge filing traces now reuse integration call logs through connector `erp_bridge_product_upsert`
  - business-info task events now also snapshot narrow ERP filing result context:
    - `integration_call_log_id`
    - Bridge `sync_log_id` when available
    - upstream status/message summary when available
- STEP_53:
  - ERP Bridge query surface added behind configurable `ERP_BRIDGE_BASE_URL`
  - frontend-ready ERP query APIs now exist:
    - `GET /v1/erp/products`
    - `GET /v1/erp/products/{id}`
    - `GET /v1/erp/categories`
  - ERP Bridge keyword search is now the primary original-product picker entry
  - bridge categories are auxiliary only and do not block mainline selection when incomplete
  - selected ERP Bridge products can now flow into:
    - `POST /v1/tasks`
    - `PATCH /v1/tasks/{id}/business-info`
  - backend now auto-caches/binds selected ERP Bridge products into local `products`
  - task-side `product_selection` now additively persists ERP Bridge external snapshot fields:
    - external product id
    - external sku id
    - category name
    - image url
    - price
  - local mapped search remains available through `GET /v1/products/search`, but it is no longer the recommended first entry for bridge-backed original-product selection
- STEP_54:
  - ERP Bridge query layer hardened without changing the upstream ERP Bridge service
  - `GET /v1/erp/products` now normalizes additive filters and pagination:
    - `q`
    - compatibility `keyword`
    - `sku_code`
    - `category_id`
    - `category_name`
    - compatibility `category`
  - `GET /v1/erp/products` now returns additive `normalized_filters`
  - ERP Bridge client response normalization now tolerates broader envelope/list/detail variants and merges duplicate rows more safely
  - ERP Bridge failures now expose timeout / retry-hint diagnostics in internal error details
  - ERP Bridge request logging now records duration/status for internal observability
  - task-side `product_selection.erp_product` snapshot persistence is now hardened to:
    - merge with prior local cached snapshot fields when rebinding
    - backfill missing non-identity fields from local product/task context
    - preserve backward compatibility for older partial snapshots
- STEP_55:
  - Step A actor/auth hardening continued without expanding into broader RBAC/SSO work
  - normal request middleware no longer synthesizes an implicit actor `1` system fallback for no-header mainline requests
  - bearer/session-backed identity remains first and now is the only accepted path for `GET /v1/auth/me`
  - debug-header actors remain compatibility-only and no longer satisfy authenticated-user semantics on `/v1/auth/me`
  - user-scoped workbench preference read/write is now session-backed on the mainline HTTP path
  - ready-for-frontend actor-id derivation helpers now only auto-fill from real session actors; debug actors keep that implicit fallback only on non-frontend/internal placeholder routes
  - route/docs/test coverage was synchronized for the tightened actor precedence

## In Progress
- None

## Next Step
- No automatic next step selected in this round.
- Post-Step-57 priority should stay bounded:
  - if Step C needs another increment, keep it on live Bridge verification / filing robustness only
  - otherwise move to Step D task-entry/SKU mainline work
  - keep broad ERP docking / WMS / finance / upload / org hierarchy work explicitly deferred

## API Source of Truth
- `docs/api/openapi.yaml` (v0.67.0)

## Latest Iteration
- `docs/iterations/ITERATION_067.md`

## Readiness Classification (V7)
### Ready for Frontend
- `GET /v1/products/search`
- `GET /v1/products/:id`
- `GET /v1/categories`
- `GET /v1/categories/search`
- `GET /v1/categories/:id`
- `POST /v1/categories`
- `PATCH /v1/categories/:id`
- `GET /v1/category-mappings`
- `GET /v1/category-mappings/search`
- `GET /v1/category-mappings/:id`
- `POST /v1/category-mappings`
- `PATCH /v1/category-mappings/:id`
- `GET /v1/cost-rules`
- `GET /v1/cost-rules/:id`
- `GET /v1/cost-rules/:id/history`
- `POST /v1/cost-rules`
- `PATCH /v1/cost-rules/:id`
- `POST /v1/cost-rules/preview`
- `POST /v1/tasks`
- `GET /v1/tasks`
- `GET /v1/tasks/:id`
- `GET /v1/task-board/summary`
- `GET /v1/task-board/queues`
- `GET /v1/workbench/preferences`
- `PATCH /v1/workbench/preferences`
- `GET /v1/export-templates`
- `POST /v1/export-jobs`
- `GET /v1/export-jobs`
- `GET /v1/export-jobs/:id`
- `GET /v1/export-jobs/:id/events`
- `POST /v1/export-jobs/:id/claim-download`
- `GET /v1/export-jobs/:id/download`
- `POST /v1/export-jobs/:id/refresh-download`
- `PATCH /v1/tasks/:id/business-info`
- `PATCH /v1/tasks/:id/procurement`
- `POST /v1/tasks/:id/procurement/advance`
- `GET /v1/tasks/:id/detail`
- `POST /v1/tasks/:id/close`
- `POST /v1/tasks/:id/assign`
- `POST /v1/tasks/:id/submit-design`
- `POST /v1/tasks/:id/warehouse/prepare`
- `GET /v1/tasks/:id/assets`
- `POST /v1/tasks/:id/audit/claim`
- `POST /v1/tasks/:id/audit/approve`
- `POST /v1/tasks/:id/audit/reject`
- `POST /v1/tasks/:id/audit/transfer`
- `POST /v1/tasks/:id/audit/handover`
- `GET /v1/tasks/:id/audit/handovers`
- `POST /v1/tasks/:id/audit/takeover`
- `POST /v1/tasks/:id/outsource`
- `GET /v1/outsource-orders`
- `GET /v1/warehouse/receipts`
- `POST /v1/tasks/:id/warehouse/receive`
- `POST /v1/tasks/:id/warehouse/reject`
- `POST /v1/tasks/:id/warehouse/complete`
- `GET /v1/tasks/:id/events`
- `GET /v1/code-rules`
- `GET /v1/code-rules/:id/preview`
- `POST /v1/code-rules/generate-sku`

### Internal Placeholder
- `GET /v1/products/sync/status`
- `POST /v1/products/sync/run`
- `GET /v1/assets/upload-requests`
- `POST /v1/assets/upload-requests`
- `GET /v1/assets/upload-requests/:id`
- `POST /v1/assets/upload-requests/:id/advance`
- `GET /v1/integration/connectors`
- `POST /v1/integration/call-logs`
- `GET /v1/integration/call-logs`
- `GET /v1/integration/call-logs/:id`
- `GET /v1/integration/call-logs/:id/executions`
- `POST /v1/integration/call-logs/:id/executions`
- `POST /v1/integration/call-logs/:id/retry`
- `POST /v1/integration/call-logs/:id/replay`
- `POST /v1/integration/call-logs/:id/executions/:execution_id/advance`
- `POST /v1/integration/call-logs/:id/advance`
- `GET /v1/export-jobs/:id/dispatches`
- `POST /v1/export-jobs/:id/dispatches`
- `POST /v1/export-jobs/:id/dispatches/:dispatch_id/advance`
- `GET /v1/export-jobs/:id/attempts`
- `POST /v1/export-jobs/:id/start`
- `POST /v1/export-jobs/:id/advance`

### Mock / Placeholder Only
- `POST /v1/tasks/:id/assets/mock-upload`

## V7 Endpoints (live)
### Step-01
- GET `/v1/products/search`
- GET `/v1/products/:id`
- POST `/v1/tasks`
- GET `/v1/tasks`
- GET `/v1/tasks/:id`
- GET `/v1/code-rules`
- GET `/v1/code-rules/:id/preview`
- POST `/v1/code-rules/generate-sku`

### Step-20
- GET `/v1/categories`
- GET `/v1/categories/search`
- GET `/v1/categories/{id}`
- POST `/v1/categories`
- PATCH `/v1/categories/{id}`
- GET `/v1/cost-rules`
- GET `/v1/cost-rules/{id}`
- GET `/v1/cost-rules/{id}/history`
- POST `/v1/cost-rules`
- PATCH `/v1/cost-rules/{id}`
- POST `/v1/cost-rules/preview`
- `PATCH /v1/tasks/{id}/business-info` now also persists:
  - `category_id`
  - `category_code`
  - `category_name`
  - `width`
  - `height`
  - `area`
  - `quantity`
  - `process`
  - `estimated_cost`
  - `cost_rule_id`
  - `cost_rule_name`
  - `cost_rule_source`
  - `requires_manual_review`
  - `manual_cost_override`
  - `manual_cost_override_reason`
- `PATCH /v1/tasks/{id}/business-info` now also performs skeleton cost prefill:
  - category + minimal inputs can update `estimated_cost`
  - when no manual override is active, `cost_price` follows the system prefill result
  - manual override remains a business field behavior, not a permission concept
- category center and cost-rule center are both configurable skeletons:
  - valid first-level coded-style entries are supported
  - later ERP mapping / hierarchy / richer formulas remain future extensions

### Step-22
- GET `/v1/category-mappings`
- GET `/v1/category-mappings/search`
- GET `/v1/category-mappings/{id}`
- POST `/v1/category-mappings`
- PATCH `/v1/category-mappings/{id}`
- `GET /v1/categories`, `POST /v1/categories`, and `PATCH /v1/categories/{id}` now also expose:
  - `search_entry_code`
  - `is_search_entry`
- first-level total category code is now explicitly modeled as the ERP search-entry key:
  - top-level categories must keep `search_entry_code == category_code`
  - top-level categories must keep `is_search_entry=true`
  - child categories inherit the parent `search_entry_code`
- category-to-ERP mapping skeleton is now a dedicated persistence/API boundary:
  - `category_id`
  - `category_code`
  - `search_entry_code`
  - `erp_match_type`
  - `erp_match_value`
  - `secondary_condition_key`
  - `secondary_condition_value`
  - `tertiary_condition_key`
  - `tertiary_condition_value`
  - `is_primary`
  - `is_active`
  - `priority`
- current mapping skeleton is readiness-marked for frontend/admin configuration only; it does not execute real ERP lookup

### Step-23
- `GET /v1/products/search` now supports mapped local ERP positioning filters:
  - `category_id`
  - `category_code`
  - `search_entry_code`
  - `mapping_match`
  - `secondary_key`
  - `secondary_value`
  - `tertiary_key`
  - `tertiary_value`
- mapped product search is still local-data-based only:
  - it resolves category-center search-entry semantics
  - it consumes active local `category_erp_mappings`
  - it filters already-synced local `products`
  - it does not call real ERP APIs
- mapped-search fallback boundary is now:
  - exact category mappings are preferred when they exist
  - otherwise the search falls back to first-level `search_entry_code` mappings
- product search responses now expose positioning provenance through:
  - `matched_category_code`
  - `matched_search_entry_code`
  - `matched_mapping_rule`

### Step-24
- `POST /v1/tasks` and `PATCH /v1/tasks/{id}/business-info` now accept additive `product_selection`
- current existing-product task persistence now keeps:
  - selected product identity and SKU snapshot
  - `matched_category_code`
  - `matched_search_entry_code`
  - `matched_mapping_rule`
  - `source_match_type`
  - `source_match_rule`
  - `source_search_entry_code`
- `GET /v1/tasks/{id}` and `GET /v1/tasks/{id}/detail` now expose top-level full `product_selection` provenance

### Step-25
- `GET /v1/tasks` now exposes lightweight `product_selection` summary on task items
- `GET /v1/task-board/summary` and `GET /v1/task-board/queues` now expose the same lightweight task-item `product_selection` summary
- `procurement_summary` now also carries lightweight `product_selection` summary
- task read/detail endpoints keep the full provenance object including `matched_mapping_rule`

### Step-26
- GET `/v1/export-templates`
- POST `/v1/export-jobs`
- GET `/v1/export-jobs`
- GET `/v1/export-jobs/{id}`
- export center now persists export jobs over stable read-model sources without generating real files:
  - task-query list state
  - task-board queue handoff state
  - procurement-summary task query state
  - warehouse receipt list filters
- current export-center result references are placeholder metadata only:
  - no NAS path
  - no object storage URL
  - no real file-generation execution

### Step-27
- POST `/v1/export-jobs/{id}/advance`
- export jobs now expose minimal lifecycle state for frontend/admin visibility:
  - `status`
  - `progress_hint`
  - `latest_status_at`
  - `download_ready`
- current lifecycle contract is:
  - `queued`
  - `running`
  - `ready`
  - `failed`
  - optional `cancelled`
- current `result_ref` is now structured placeholder handoff metadata:
  - `ref_type`
  - `ref_key`
  - `file_name`
  - `mime_type`
  - `expires_at`
  - `is_placeholder`
  - `note`
- `POST /v1/export-jobs/{id}/advance` is internal/admin only and is not a real runner, scheduler, or file-delivery service
- `queued -> running` start semantics are now superseded by the explicit Step-31 start boundary even though `advance action=start` remains backward compatible

### Step-28
- GET `/v1/export-jobs/{id}/events`
- export jobs now expose lightweight lifecycle-audit summaries on list/detail:
  - `event_count`
  - `latest_event`
- export jobs may also expose runner-boundary summary context through:
  - `latest_runner_event`
- export-job timeline events now durably record:
  - `created`
  - `runner_initiated`
  - `started`
  - `advanced_to_running`
  - `advanced_to_ready`
  - `advanced_to_failed`
  - `advanced_to_cancelled`
  - `advanced_to_queued`
  - `result_ref_updated`
- event payload remains audit context only:
  - not a full runner log stream
  - not a storage or download telemetry channel

### Step-29
- POST `/v1/export-jobs/{id}/claim-download`
- GET `/v1/export-jobs/{id}/download`
- ready export jobs now expose a minimal placeholder handoff claim/read boundary:
  - claim records placeholder download takeover intent
  - read returns current structured handoff metadata
- claim/read remain placeholder-only:
  - not file bytes
  - not signed URLs
  - not NAS or object-storage references
- claim/read reuse the export-job event chain through:
  - `download_claimed`
  - `download_read`

### Step-30
- POST `/v1/export-jobs/{id}/refresh-download`
- placeholder handoff expiry is now enforced on ready export jobs:
  - ready + not expired => claim/read allowed
  - ready + expired => claim/read rejected until refresh
- export-job list/detail now expose:
  - `is_expired`
  - `can_refresh`
- handoff responses now expose:
  - `is_expired`
  - `can_refresh`
- refresh rotates placeholder handoff state through:
  - refreshed `result_ref.ref_key`
  - refreshed `expires_at`
- expiry/refresh reuse the export-job event chain through:
  - `download_expired`
  - `download_refreshed`
  - `result_ref_updated`

### Step-31
- POST `/v1/export-jobs/{id}/start`
- export jobs now expose explicit placeholder runner-initiation hints on list/detail:
  - `can_start`
  - `start_mode`
  - `execution_mode`
  - `latest_runner_event`
- current start contract is:
  - only `queued` export jobs can start
  - successful start formalizes the `queued -> running` boundary
  - `running|ready|failed|cancelled` cannot be started again
  - `POST /v1/export-jobs/{id}/advance` `action=start` still works, but it now reuses the same start helper for backward compatibility
- explicit start-boundary events now supplement the lifecycle timeline through:
  - `runner_initiated`
  - `started`
  - `advanced_to_running` remains the generic lifecycle event for existing consumers
- `POST /v1/export-jobs/{id}/start` is internal/admin only and is not a real async runner, scheduler, worker-lease API, or file-generation platform

### Step-32
- GET `/v1/export-jobs/{id}/attempts`
- export jobs now expose placeholder execution-attempt summaries on list/detail:
  - `attempt_count`
  - `latest_attempt`
  - `can_retry`
- current attempt records now separate one concrete start execution from the export-job lifecycle through:
  - `attempt_id`
  - `attempt_no`
  - `trigger_source`
  - `execution_mode`
  - `adapter_key`
  - `status`
  - `started_at`
  - `finished_at`
  - `error_message`
  - `adapter_note`
- current attempt-result events now supplement the shared export-job timeline through:
  - `attempt_succeeded`
  - `attempt_failed`
  - `attempt_cancelled`
- `GET /v1/export-jobs/{id}/attempts` is internal/admin only and is not a real scheduler, worker lease, or runner telemetry stream

### Step-33
- GET `/v1/export-jobs/{id}/dispatches`
- POST `/v1/export-jobs/{id}/dispatches`
- POST `/v1/export-jobs/{id}/dispatches/{dispatch_id}/advance`
- current dispatch records now make the placeholder adapter handoff explicit through:
  - `dispatch_id`
  - `dispatch_no`
  - `trigger_source`
  - `execution_mode`
  - `adapter_key`
  - `status`
  - `submitted_at`
  - `received_at`
  - `finished_at`
  - `expires_at`
  - `status_reason`
  - `adapter_note`
- current dispatch events now supplement the shared export-job timeline through:
  - `dispatch_submitted`
  - `dispatch_received`
  - `dispatch_rejected`
  - `dispatch_expired`
  - `dispatch_not_executed`
- dispatch visibility is internal/admin only and is not a real scheduler queue, dispatch callback, or worker handoff protocol

### Step-34
- export jobs now expose dispatch-side read-model summaries on list/detail:
  - `dispatch_count`
  - `latest_dispatch`
  - `can_dispatch`
  - `can_redispatch`
  - `latest_dispatch_event`
- `can_start` now remains false when the latest placeholder dispatch is still `submitted`
- export-job read models now explicitly separate:
  - lifecycle state
  - dispatch state
  - attempt state
- these dispatch read-model summaries are still placeholder-only and must not be interpreted as a real scheduler or distributed runner platform

### Step-35
- GET `/v1/integration/connectors`
- POST `/v1/integration/call-logs`
- GET `/v1/integration/call-logs`
- GET `/v1/integration/call-logs/{id}`
- GET `/v1/integration/call-logs/{id}/executions`
- POST `/v1/integration/call-logs/{id}/executions`
- POST `/v1/integration/call-logs/{id}/retry`
- POST `/v1/integration/call-logs/{id}/replay`
- POST `/v1/integration/call-logs/{id}/executions/{execution_id}/advance`
- POST `/v1/integration/call-logs/{id}/advance`
- current integration call-log records now expose:
  - `connector_key`
  - `operation_key`
  - `direction`
  - `resource_type`
  - `resource_id`
  - `status`
  - `progress_hint`
  - `request_payload`
  - `response_payload`
  - `error_message`
  - `latest_status_at`
  - `started_at`
  - `finished_at`
  - `execution_count`
  - `latest_execution`
  - `can_retry`
  - `can_replay`
  - `retry_count`
  - `replay_count`
  - `latest_retry_action`
  - `latest_replay_action`
  - `retryability_reason`
  - `replayability_reason`
- integration center is internal/admin only and is not a real ERP executor, retry queue, callback processor, or distributed integration platform

### Step-02
- POST `/v1/tasks/:id/audit/claim`
- POST `/v1/tasks/:id/audit/approve`
- POST `/v1/tasks/:id/audit/reject`
- POST `/v1/tasks/:id/audit/transfer`
- POST `/v1/tasks/:id/audit/handover`
- POST `/v1/tasks/:id/audit/takeover`
- POST `/v1/tasks/:id/outsource`
- GET `/v1/outsource-orders`
- GET `/v1/tasks/:id/events`

### Step-03
- GET `/v1/tasks/:id/detail`
- GET `/v1/tasks/:id/audit/handovers`
- GET `/v1/warehouse/receipts`
- POST `/v1/tasks/:id/warehouse/receive`
- POST `/v1/tasks/:id/warehouse/reject`
- POST `/v1/tasks/:id/warehouse/complete`

### Step-04
- POST `/v1/tasks/:id/assign`
- POST `/v1/tasks/:id/submit-design`
- GET `/v1/tasks/:id/assets`
- POST `/v1/tasks/:id/assets/mock-upload`

### Step-05
- GET `/v1/tasks` (enhanced filters + pagination + warehouse/asset projections)
- GET `/v1/tasks/:id/detail` (assets + available_actions)
- GET `/v1/tasks/:id` (superseded in Step-09 by read model + workflow snapshot)
- GET `/v1/products/search` (pagination)
- GET `/v1/outsource-orders` (vendor/task/status filters + pagination)
- GET `/v1/warehouse/receipts` (receiver/task/status filters + pagination)

### Step-06
- GET `/v1/products/sync/status` (internal ERP placeholder status; not ready for frontend)
- POST `/v1/products/sync/run` (internal ERP placeholder manual trigger; not ready for frontend)

### Step-07
- V7 routes now expose placeholder response headers:
  - `X-Workflow-Auth-Mode`
  - `X-Workflow-API-Readiness`
  - `X-Workflow-Required-Roles`
- V7 request actor placeholder accepts:
  - `X-Debug-Actor-Id`
  - `X-Debug-Actor-Roles`
- Step-07 route role metadata is advisory only and not enforced

### Step-08
- PATCH `/v1/tasks/:id/business-info`
- POST `/v1/tasks/:id/warehouse/prepare`
- `task_type` public contract now uses:
  - `original_product_development`
  - `new_product_development`
  - `purchase_task`
- `purchase_task` cannot use:
  - `POST /v1/tasks/:id/assign`
  - `POST /v1/tasks/:id/submit-design`
- `purchase_task` can move from business-info maintenance directly to `PendingWarehouseReceive`
- V7 task list/detail now expose workflow projection fields:
  - `workflow.main_status`
  - `workflow.sub_status`
  - `workflow.can_prepare_warehouse`
  - `workflow.warehouse_blocking_reasons`
  - `workflow.can_close`
  - `workflow.cannot_close_reasons`

### Step-09
- POST `/v1/tasks/:id/close`
- GET `/v1/tasks/:id` now returns a task read model with `workflow`
- `warehouse/complete` now moves `task_status` to `PendingClose`
- standalone close moves `task_status` from `PendingClose` to `Completed`
- workflow readiness reason arrays now return structured objects:
  - `code`
  - `message`
- V7 workflow query contracts now expose:
  - `workflow.closable`
  - `workflow.can_close` (compatibility alias)

### Step-10
- PATCH `/v1/tasks/:id/procurement`
- `workflow.sub_status.*` now returns structured:
  - `code`
  - `label`
  - `source`
- `GET /v1/tasks/:id` now exposes nullable `procurement`
- `GET /v1/tasks/:id/detail` now exposes nullable `procurement`
- `purchase_task` readiness no longer depends on the public `task_details.procurement_price` contract

### Step-11
- GET `/v1/tasks` now supports projected workflow filters:
  - `main_status`
  - `sub_status_code`
  - `sub_status_scope`
- POST `/v1/tasks/:id/procurement/advance`
- `GET /v1/tasks`, `GET /v1/tasks/:id`, and `GET /v1/tasks/:id/detail` now expose nullable `procurement_summary`
- `procurement_records.status` now uses:
  - `draft`
  - `prepared`
  - `in_progress`
  - `completed`

### Step-12
- `procurement_summary` now additionally exposes:
  - `coordination_status`
  - `coordination_label`
  - `warehouse_status`
  - `warehouse_prepare_ready`
  - `warehouse_receive_ready`
- purchase-task procurement coordination now uses derived semantics:
  - `in_progress` -> awaiting arrival
  - `completed` -> ready for warehouse
  - warehouse handoff / receipt presence -> handed to warehouse
- `POST /v1/tasks/:id/warehouse/prepare` for `purchase_task` now requires procurement status `completed`

### Step-13
- GET `/v1/task-board/summary`
- GET `/v1/task-board/queues`
- task-board / inbox aggregation is now frontend-ready and derived from:
  - `workflow.main_status`
  - `workflow.sub_status`
  - `procurement_summary.coordination_status`
- task-board queues now expose:
  - `queue_key`
  - `queue_name`
  - `queue_description`
  - `filters`
  - `count`
- task-board queues now additionally expose lightweight ownership hints:
  - `suggested_roles`
  - `suggested_actor_type`
  - `default_visibility`
  - `ownership_hint`
- `/v1/task-board/summary` returns sample tasks per preset queue
- `/v1/task-board/queues` returns paginated tasks per preset queue
- GET `/v1/workbench/preferences` and PATCH `/v1/workbench/preferences` now expose placeholder-actor-scoped saved workbench defaults plus direct queue/config bootstrap payloads
- preset queues currently include:
  - `ops_pending_material`
  - `design_pending_submit`
  - `audit_pending_review`
  - `procurement_pending_followup`
  - `awaiting_arrival`
  - `warehouse_pending_prepare`
  - `warehouse_pending_receive`
  - `pending_close`

### Step-14
- `GET /v1/tasks` and task-board queues now share a converged filter contract over:
  - `status`
  - `task_type`
  - `source_mode`
  - `main_status`
  - `sub_status_scope`
  - `sub_status_code`
  - `coordination_status`
  - `warehouse_prepare_ready`
  - `warehouse_receive_ready`
  - `warehouse_blocking_reason_code`
- the converged task query fields accept multi-value filtering semantics where appropriate
- task-board queue payloads now additionally expose:
  - `normalized_filters`
  - `query_template`
- board queue filters are now designed to be consumed directly by `/v1/tasks` drill-down flows without frontend-only rule translation

### Step-15
- converged `/v1/tasks` filters now execute through direct repo/read-model predicates instead of service-layer segmented fan-out
- repo/read-model predicate pushdown now directly covers:
  - `status`
  - `task_type`
  - `source_mode`
  - `main_status`
  - `sub_status_code`
  - `coordination_status`
  - `warehouse_prepare_ready`
  - `warehouse_receive_ready`
  - `warehouse_blocking_reason_code`
- task-board external filter names and drill-down payloads stay stable while `/v1/tasks` execution hardens internally

### Step-16
- task-board summary and task-board queues now aggregate from one shared board-level candidate task pool per request
- preset queues are no longer collected through queue-by-queue list fan-out
- board aggregation now shares one implementation for:
  - summary counts
  - sample tasks
  - queue task lists
  - per-queue pagination slicing
- board aggregation still relies on the same `/v1/tasks`-aligned filter semantics and stable queue payload contract

### Step-17
- task-board summary and task-board queues now obtain their shared candidate pool from a dedicated repo/read-model board candidate scan
- board candidate scans now push down:
  - global board/list converged filters
  - the union of selected preset queue predicates
- service-memory partitioning is now limited to business-required final queue shaping rather than broad candidate collection
- task-board external queue/filter/query-template contracts remain unchanged

## V6 Endpoints (preserved)
- All V6 routes remain intact; no removals

## New Tables
### Migration 001
- `products`
- `tasks`
- `task_details`
- `code_rules`
- `code_rule_sequences`

### Migration 002
- `audit_records`
- `audit_handovers`
- `outsource_orders`
- `task_event_logs`
- `task_event_sequences`

### Migration 003
- `warehouse_receipts`

### Migration 004
- `task_assets`

### Migration 005
- `erp_sync_runs`

### Migration 006
- additive columns on `task_details`:
  - `category`
  - `spec_text`
  - `material`
  - `size_text`
  - `craft_text`
  - `procurement_price`
  - `cost_price`
  - `filed_at`

### Migration 007
- `procurement_records`

### Migration 008
- additive column on `procurement_records`:
  - `quantity`
- normalized `procurement_records.status` values:
  - `preparing -> draft`
  - `ready -> prepared`

### Migration 009
- `workbench_preferences`

### Migration 010
- `categories`
- `cost_rules`
- additive columns on `task_details`:
  - `category_id`
  - `category_code`
  - `category_name`
  - `cost_rule_id`
  - `cost_rule_name`
  - `cost_rule_source`

### Migration 011
- additive columns on `task_details`:
  - `width`
  - `height`
  - `area`
  - `quantity`
  - `process`
  - `estimated_cost`
  - `requires_manual_review`
  - `manual_cost_override`
  - `manual_cost_override_reason`

### Migration 012
- additive columns on `categories`:
  - `search_entry_code`
  - `is_search_entry`
- `category_erp_mappings`

### Migration 013
- additive columns on `task_details`:
  - `source_product_id`
  - `source_product_name`
  - `source_search_entry_code`
  - `source_match_type`
  - `source_match_rule`
  - `matched_category_code`
  - `matched_search_entry_code`
  - `matched_mapping_rule_json`

### Migration 014
- `export_jobs`

### Migration 015
- additive column on `export_jobs`:
  - `status_updated_at`

### Migration 016
- `export_job_events`
- `export_job_event_sequences`

### Migration 017
- `export_job_attempts`

### Migration 018
- `export_job_dispatches`

### Migration 019
- `integration_call_logs`

### Migration 020
- `upload_requests`
- `asset_storage_refs`
- additive columns on `task_assets`:
  - `upload_request_id`
  - `storage_ref_id`
  - `mime_type`
  - `file_size`

### Migration 021
- `integration_call_executions`

### Migration 022
- additive columns on `cost_rules`:
  - `rule_version`
  - `supersedes_rule_id`
  - `governance_note`
- additive columns on `task_details`:
  - `matched_rule_version`
  - `prefill_source`
  - `prefill_at`
  - `override_actor`
  - `override_at`

## Step-04 Rules
- New tasks now start in `PendingAssign`
- `assign` only allows tasks in `PendingAssign`
- `assign` sets both `designer_id` and `current_handler_id`, then moves task to `InProgress`
- `submit-design` requires existing `designer_id`
- `submit-design` only allows `InProgress` or `RejectedByAuditA`
- `submit-design` creates a `task_assets` row and moves task to `PendingAuditA`
- `mock-upload` creates a `task_assets` row but does not change task status
- `task_assets.version_no` is a per-task monotonic sequence ordered by `version_no ASC`
- `assign`, `submit-design`, and `mock-upload` all append `task_event_logs`

## Step-05 Rules
- V7 list query responses standardized to `{ "data": [...], "pagination": {...} }` for:
  - `/v1/tasks`
  - `/v1/products/search`
  - `/v1/outsource-orders`
  - `/v1/warehouse/receipts`
- V7 single-object queries in this step remain `{ "data": {...} }`
- `/v1/tasks/{id}` was base-entity-only in Step-05, but now returns a read model with `workflow`
- `/v1/tasks/{id}/detail` is the frontend aggregate detail endpoint
- `/v1/tasks/{id}/detail.assets` is ordered by `version_no ASC`
- `/v1/tasks/{id}/detail.available_actions` is frontend suggestion only and derived from `task_status` plus `warehouse_receipt`
- `/v1/tasks` supports filters:
  - `status`
  - `task_type`
  - `source_mode`
  - `creator_id`
  - `designer_id`
  - `need_outsource`
  - `overdue`
  - `keyword`
- `/v1/tasks.latest_asset_type` is derived from highest `task_assets.version_no`, then latest `created_at`
- `/v1/products/search.category` is matched against `products.category`
- `/v1/outsource-orders.vendor` is fuzzy matched on `vendor_name`
- `/v1/warehouse/receipts.receiver_id` filters current receipt owner
- V7 query endpoints intentionally still using pre-Step-05 `data`-only list shape:
  - `/v1/tasks/{id}/assets`
  - `/v1/tasks/{id}/audit/handovers`
  - `/v1/tasks/{id}/events`
  - `/v1/code-rules`

## Step-06 Rules
- ERP sync source mode is currently `stub` only
- `ERP_SYNC_STUB_FILE` is read as a local JSON array source
- Missing stub file is recorded as sync status `noop`, not `failed`
- Invalid stub file content is recorded as sync status `failed`
- Product sync upsert key is `products.erp_product_id`
- Product sync overwrites:
  - `sku_code`
  - `product_name`
  - `category`
  - `spec_json`
  - `status`
  - `source_updated_at`
- Product sync always refreshes `sync_time` on successful upsert
- Step-06 ERP sync APIs are internal placeholder APIs only
- Step-06 ERP sync APIs are not part of the ready-for-frontend endpoint set

## Step-07 Rules
- Current placeholder auth mode is `debug_header_role_enforced`
- Mainline requests without bearer or debug headers no longer synthesize request actor ID `1`
- `system_fallback` remains legacy/explicit placeholder behavior only
- V7 role constants currently include:
  - `Ops`
  - `Designer`
  - `Audit_A`
  - `Audit_B`
  - `Warehouse`
  - `Outsource`
  - `ERP`
  - `Admin`
- Route-level required roles are now enforced on V7 routes that carry `withAccessMeta(...)`
- `Admin` currently acts as a placeholder override across those route-level role checks
- This phase still does not implement login/session, org hierarchy, or data-visibility filtering
- `POST /v1/tasks/{id}/assets/mock-upload` remains mock / placeholder only

## Step-08 Rules
- `task_type` is now required on task creation
- `original_product_development` is treated as `source_mode=existing_product`; frontend should not send `source_mode`
- `new_product_development` is treated as `source_mode=new_product`; frontend should not send `source_mode`
- `purchase_task` may use either `existing_product` or `new_product`
- `purchase_task` cannot be assigned to a designer and cannot submit design assets
- Warehouse handoff is now explicit via `POST /v1/tasks/{id}/warehouse/prepare`
- Warehouse handoff evaluates PRD-aligned blocking reasons before moving to `PendingWarehouseReceive`
- Business-info / generic cost maintenance now lands in `task_details` and feeds workflow readiness:
  - `category`
  - `spec_text`
  - `material`
  - `size_text`
  - `craft_text`
  - `cost_price`
  - `filed_at`

## Step-09 Rules
- `PendingClose` is now an explicit persisted `task_status`
- Warehouse completion is no longer a close surrogate:
  - `POST /v1/tasks/{id}/warehouse/complete` transitions `PendingWarehouseReceive -> PendingClose`
  - `POST /v1/tasks/{id}/close` transitions `PendingClose -> Completed`
- `GET /v1/tasks/{id}` now returns the lightweight read model plus `workflow`
- `workflow.cannot_close_reasons` and `workflow.warehouse_blocking_reasons` now return structured objects:
  - `code`
  - `message`
- `workflow.closable` is the stable frontend-facing close-readiness flag
- `workflow.can_close` is retained as a compatibility alias in Step-09
- Close errors now return server-side readiness context in `error.details`:
  - `task_type`
  - `workflow`
  - `closable`
  - `cannot_close_reasons`

## Step-10 Rules
- `task_status` remains the persisted operational state used for transitions and event sourcing
- `workflow.main_status` is the stable PRD mainline projection exposed to frontend clients
- `workflow.sub_status.*` now returns structured objects:
  - `code`
  - `label`
  - `source`
- purchase-task procurement preparation is persisted in `procurement_records`, not in the public `task_details` contract
- `PATCH /v1/tasks/{id}/business-info` no longer owns procurement data
- `PATCH /v1/tasks/{id}/procurement` creates or updates the dedicated purchase-preparation record
- purchase-task readiness now requires:
  - generic business info in `task_details`
  - procurement record present
  - procurement price present
  - procurement status in `ready|completed`
- `GET /v1/tasks`, `GET /v1/tasks/{id}`, and `GET /v1/tasks/{id}/detail` now share the same structured `workflow.sub_status` contract
- `GET /v1/tasks/{id}` and `GET /v1/tasks/{id}/detail` expose nullable `procurement`

## Step-11 Rules
- `GET /v1/tasks.main_status` filters the projected `workflow.main_status` contract instead of raw `task_status`
- `GET /v1/tasks.sub_status_code` filters the projected structured `workflow.sub_status.*.code`
- `GET /v1/tasks.sub_status_scope` optionally narrows sub-status matching to one lane:
  - `design`
  - `audit`
  - `procurement`
  - `warehouse`
  - `outsource`
  - `production`
- list/read/detail task queries now expose `procurement_summary` for frontend-friendly procurement overview
- `PATCH /v1/tasks/{id}/procurement` maintains procurement draft data and explicit status fields
- `POST /v1/tasks/{id}/procurement/advance` performs the minimal procurement lifecycle transitions:
  - `prepare`
  - `start`
  - `complete`
  - `reopen`
- purchase-task procurement status now persists as:
  - `draft`
  - `prepared`
  - `in_progress`
  - `completed`
- purchase-task warehouse readiness now requires:
  - procurement record present
  - procurement price present
  - procurement quantity present
  - procurement status `completed`
- purchase-task close readiness now requires procurement status `completed`

## Step-20 Rules
- Category center is now a dedicated configurable skeleton instead of a note-level convention.
- First-level coded-style entries such as `HBJ/HBZ/HCP/HLZ/HPJ/HQT/HSC/HZS` are valid category-center records and valid first-level search-entry categories.
- Category hierarchy remains intentionally lightweight in this phase:
  - `parent_id`
  - `level`
  - no full tree-management workflow yet
- Cost-rule center is now a dedicated configurable skeleton instead of service hardcoding or Excel-note branching.
- Supported rule types currently include:
  - `fixed_unit_price`
  - `area_threshold_surcharge`
  - `minimum_billable_area`
  - `size_based_formula`
  - `manual_quote`
  - `special_process_surcharge`
- `manual_quote` must be used for rules that are not machine-calculable yet.
- `POST /v1/cost-rules/preview` currently supports:
  - estimated calculation for fixed-price / threshold / minimum-area / special-process combinations
  - narrow formula support for `print_side:*`
  - manual-review fallback for unsupported size-based formulas and manual-quote categories
- `PATCH /v1/tasks/{id}/business-info` may now persist:
  - legacy `category`
  - structured `category_id/category_code/category_name`
  - `cost_rule_id/cost_rule_name/cost_rule_source`
- Procurement remains the owner of purchase-side pricing:
  - `procurement_price`
- Business-info remains the owner of internal-cost-side pricing and provenance:
  - `cost_price`
  - `cost_rule_*`
- Current sample initialization artifacts live in:
  - `config/category_seed.json`
  - `config/cost_rule_seed.json`

## Step-21 Rules
- `PATCH /v1/tasks/{id}/business-info` is now the direct business boundary for:
  - category selection
  - minimal cost-prefill inputs
  - system-estimated internal cost persistence
  - manual override persistence
- Supported minimal cost-prefill inputs are:
  - `width`
  - `height`
  - `area`
  - `quantity`
  - `process`
- Persisted cost-prefill / override outputs are:
  - `estimated_cost`
  - `cost_price`
  - `cost_rule_id`
  - `cost_rule_name`
  - `cost_rule_source`
  - `requires_manual_review`
  - `manual_cost_override`
  - `manual_cost_override_reason`
- Current business boundary is:
  - system prefill writes `estimated_cost`
  - current effective internal cost is read from `cost_price`
  - when `manual_cost_override=false`, `cost_price` normally follows `estimated_cost`
  - when `manual_cost_override=true`, `cost_price` is a business override and `estimated_cost` remains the last system estimate
- `manual_cost_override` is not a permission-system flag and does not imply any RBAC enforcement.
- Procurement remains the owner of `procurement_price`, while procurement-facing summaries now explicitly expose internal cost-side signals for `purchase_task`.

## Step-22 Rules
- Category center now explicitly models first-level ERP lookup semantics through:
  - `search_entry_code`
  - `is_search_entry`
- Current first-level search-entry contract is:
  - top-level categories must use `search_entry_code == category_code`
  - top-level categories must use `is_search_entry=true`
  - child categories inherit the same first-level `search_entry_code`
  - child categories cannot become first-level search entries in this phase
- `category_erp_mappings` is now the dedicated category-to-ERP positioning skeleton.
- Supported `erp_match_type` values are:
  - `category_code`
  - `product_family`
  - `sku_prefix`
  - `keyword`
  - `external_id`
- Reserved later-refinement fields are:
  - `secondary_condition_key`
  - `secondary_condition_value`
  - `tertiary_condition_key`
  - `tertiary_condition_value`
- Current task/business-info and cost-prefill contracts remain unchanged:
  - future ERP positioning should resolve from `task_details.category_* -> categories.search_entry_code -> category_erp_mappings`
- Current product search still uses the existing `products.category` filter path; mapping skeleton consumption by product search remains a later phase.

## Step-23 Rules
- `GET /v1/products/search` is now the local ERP positioning execution layer for:
  - `search_entry_code`
  - `category_id`
  - `category_code`
  - active `category_erp_mappings`
- Current mapped-search execution order is:
  - resolve `category_id/category_code` to `categories.search_entry_code`
  - load active local mappings under that first-level search entry
  - optionally narrow by reserved `secondary_*` / `tertiary_*` query pairs
  - default to primary mappings unless `mapping_match=all`
  - prefer exact category mappings when present, otherwise fall back to search-entry-wide mappings
- Current mapped-search result contract exposes:
  - `matched_category_code`
  - `matched_search_entry_code`
  - `matched_mapping_rule`
- Current mapped-search scope is intentionally limited:
  - no real ERP API lookup
  - no sync enhancement
  - no full second/third-level category tree
  - no full search-engine behavior
- Legacy `category` fuzzy filtering remains available for compatibility, but mapped search is now the intended path for category-center-driven ERP product positioning.

## Step-24 Rules
- Original-product task entry is now allowed to pass an additive `product_selection` object through:
  - `POST /v1/tasks`
  - `PATCH /v1/tasks/{id}/business-info`
- `product_selection` is the formal handoff object from mapped local product search into task-side persistence:
  - selected product identity
  - selected SKU snapshot
  - `matched_category_code`
  - `matched_search_entry_code`
  - `matched_mapping_rule`
  - `source_match_type`
  - `source_match_rule`
  - `source_search_entry_code`
- Current original-product integration boundary is:
  - `GET /v1/products/search` still does the local ERP positioning
  - task create / business-info now persist the chosen result and its provenance
  - task read/detail now expose that provenance through top-level `product_selection`
- Legacy compatibility remains intentional:
  - `product_id/sku_code/product_name_snapshot` are still accepted on task create
  - when older existing-product rows lack mapped provenance, the read contract falls back to a minimal legacy selection trace instead of returning nothing
- Current Step-24 scope is still intentionally limited:
  - no real ERP API lookup
  - no sync enhancement
  - no full second/third-level category tree
  - no full search-engine behavior

## Step-25 Rules
- `product_selection` is now a first-class read-model contract across:
  - `GET /v1/tasks`
  - `GET /v1/task-board/summary`
  - `GET /v1/task-board/queues`
  - `GET /v1/tasks/{id}`
  - `GET /v1/tasks/{id}/detail`
- Current layering is explicit:
  - task list / board task items expose lightweight `product_selection` summary
  - `procurement_summary` exposes lightweight `product_selection` summary
  - task read / detail keep full `product_selection` provenance including `matched_mapping_rule`
- Field naming should stay aligned across these views:
  - selected product identity and SKU
  - `matched_category_code`
  - `matched_search_entry_code`
  - `source_match_type`
  - `source_match_rule`
  - `source_search_entry_code`
- Frontend should directly consume read-model provenance instead of rebuilding original-product traceability from scattered `matched_*` / `source_*` fields.
- Current Step-25 scope remains intentionally limited:
  - no real ERP API lookup
  - no sync enhancement
  - no full second/third-level category tree
  - no full search-engine behavior

## Step-26 Rules
- Export center must consume existing stable read models instead of inventing a parallel reporting query language.
- Current frontend-ready export-center APIs are:
  - `GET /v1/export-templates`
  - `POST /v1/export-jobs`
  - `GET /v1/export-jobs`
  - `GET /v1/export-jobs/{id}`
- Current `export_type` values are:
  - `task_list`
  - `task_board_queue`
  - `procurement_summary`
  - `warehouse_receipts`
- Current `source_query_type` values are:
  - `task_query`
  - `task_board_queue`
  - `procurement_summary`
  - `warehouse_receipts`
- Task-query-derived export jobs must persist `query_template` and may also persist `normalized_filters`.
- Task-board export jobs must at least persist:
  - `source_filters.queue_key`
  - current `query_template`
  - optional `normalized_filters`
- Warehouse export jobs currently persist warehouse-list filter fields through `source_filters` rather than task-query contracts.
- `result_ref` is placeholder metadata only:
  - it is structured placeholder download-handoff metadata
  - it is not a generated file path
  - it is not a signed download URL
  - it does not imply NAS or object-storage integration has landed
- Current static export template catalog is code-defined only; no `export_templates` table exists in this phase.
- Current export job lifecycle model is intentionally minimal:
  - `queued`
  - `running`
  - `ready`
  - `failed`
  - `cancelled`
- Export-job read models now expose:
  - `progress_hint`
  - `latest_status_at`
  - `download_ready`
- internal placeholder export-runner initiation is now exposed through:
  - `POST /v1/export-jobs/{id}/start`
- `POST /v1/export-jobs/{id}/advance` remains internal/admin skeleton only:
  - manual lifecycle progression
  - not a real async runner
  - not a real file service
- Current Step-26 scope remains intentionally limited:
  - no real file generation
  - no NAS / upload / download-agent integration
  - no full template engine
  - no async scheduling platform
  - no BI / finance / ERP reporting modules

## Step-27 Rules
- Export-center list/detail contracts now expose stable minimal lifecycle state through:
  - `status`
  - `progress_hint`
  - `latest_status_at`
  - `download_ready`
  - `can_start`
  - `start_mode`
  - `execution_mode`
- Current export lifecycle transitions are:
  - `queued -> running`
  - `running -> ready`
  - `queued|running -> failed`
  - `queued|running -> cancelled`
  - `failed|cancelled -> queued`
- `result_ref` is now the placeholder download-handoff contract and currently exposes:
  - `ref_type`
  - `ref_key`
  - `file_name`
  - `mime_type`
  - `expires_at`
  - `is_placeholder`
  - `note`
- `ready` means placeholder handoff metadata is available; it does not mean a real file system or signed download URL exists.
- `POST /v1/export-jobs/{id}/start` is now the formal placeholder runner-initiation boundary for `queued -> running`.
- `POST /v1/export-jobs/{id}/advance` remains an internal/admin skeleton endpoint only and must not be marked ready-for-frontend.
- Real file generation, storage delivery, and async execution remain out of scope in this phase.

## Step-28 Rules
- Export-job lifecycle trace must stay as a dedicated event chain rather than being squeezed into `remark` or inferred only from timestamps.
- Current frontend-ready export timeline API is:
  - `GET /v1/export-jobs/{id}/events`
- Export-job list/detail contracts may expose lightweight audit summaries only:
  - `event_count`
  - `latest_event`
  - `latest_runner_event`
- Current export-job event coverage includes:
  - `export_job.created`
  - `export_job.runner_initiated`
  - `export_job.started`
  - `export_job.advanced_to_running`
  - `export_job.advanced_to_ready`
  - `export_job.advanced_to_failed`
  - `export_job.advanced_to_cancelled`
  - `export_job.advanced_to_queued`
  - `export_job.result_ref_updated`
- Event payload must be interpreted as audit context only:
  - not a full runner log stream
  - not real file-generation telemetry
  - not proof that a download endpoint or storage integration exists
- Lifecycle state changes and export-job audit events must be written atomically in the same transaction.
- This step remains intentionally limited:
  - no real file generation
  - no real download endpoint
  - no signed URL delivery
  - no NAS / object-storage integration
  - no async runner platform

## Step-29 Rules
- Current frontend-ready placeholder download-handoff APIs are:
  - `POST /v1/export-jobs/{id}/claim-download`
  - `GET /v1/export-jobs/{id}/download`
- Claim/read are valid only when:
  - export job status is `ready`
  - `download_ready=true`
- Current handoff response returns structured placeholder metadata including:
  - `export_job_id`
  - `result_ref`
  - `file_name`
  - `mime_type`
  - `is_placeholder`
  - `expires_at`
  - `download_ready`
  - `note`
- Current handoff response may also expose placeholder access audit context:
  - `claimed_at`
  - `claimed_by_actor_id`
  - `claimed_by_actor_type`
  - `last_read_at`
  - `last_read_by_actor_id`
  - `last_read_by_actor_type`
- Claim/read must reuse the existing export-job event chain rather than creating a parallel audit log:
  - `export_job.download_claimed`
  - `export_job.download_read`
- `GET /v1/export-jobs/{id}/download` is a placeholder handoff read endpoint, not a real file-byte delivery endpoint.
- This step remains intentionally limited:
  - no real file generation
  - no real file-byte download
  - no signed URL delivery
  - no NAS / object-storage integration
  - no async runner platform

## Step-31 Rules
- `POST /v1/export-jobs/{id}/start` is the formal placeholder runner-initiation boundary.
- Current start semantics are:
  - only `queued` export jobs can start
  - start formalizes the `queued -> running` boundary
  - `running|ready|failed|cancelled` cannot be started again
  - `POST /v1/export-jobs/{id}/advance` `action=start` remains backward compatible but must reuse the same start semantics
- Export-job list/detail contracts now expose:
  - `can_start`
  - `start_mode`
  - `execution_mode`
  - `latest_runner_event`
- Current explicit runner-boundary events include:
  - `export_job.runner_initiated`
  - `export_job.started`
- These events must still be interpreted as audit context only:
  - not a worker lease
  - not a scheduler callback contract
  - not a real runner log stream
  - not proof that file generation or delivery infrastructure exists
- This step remains intentionally limited:
  - no real async runner platform
  - no real file generation
  - no real download delivery
  - no signed URL issuance
  - no NAS / object-storage integration

## Step-32 Rules
- Export-job lifecycle and execution-attempt lifecycle must stay separate:
  - export job state still answers whether the business object is `queued|running|ready|failed|cancelled`
  - attempt state answers what happened in one concrete start attempt
- Current internal/admin attempt inspection API is:
  - `GET /v1/export-jobs/{id}/attempts`
- Export-job list/detail contracts now expose:
  - `attempt_count`
  - `latest_attempt`
  - `can_retry`
- Current attempt state contract includes:
  - `running`
  - `succeeded`
  - `failed`
  - `cancelled`
- Current placeholder runner-adapter boundary is visible through attempt records:
  - `trigger_source`
  - `execution_mode`
  - `adapter_key`
  - `adapter_note`
- Current attempt-result events supplement the shared export-job event chain through:
  - `export_job.attempt_succeeded`
  - `export_job.attempt_failed`
  - `export_job.attempt_cancelled`
- Attempt visibility must remain placeholder-only:
  - not a real scheduler queue
  - not a worker lease or heartbeat contract
  - not a full runner log stream
  - not proof that real file generation, storage, or delivery exists

## Step-33 Rules
- Export-job dispatch lifecycle must stay separate from both export-job lifecycle and execution-attempt lifecycle:
  - export job state still answers whether the business object is `queued|running|ready|failed|cancelled`
  - dispatch state answers what happened at the placeholder adapter handoff boundary
  - attempt state answers what happened after one concrete start execution began
- Current internal/admin dispatch inspection / operation APIs are:
  - `GET /v1/export-jobs/{id}/dispatches`
  - `POST /v1/export-jobs/{id}/dispatches`
  - `POST /v1/export-jobs/{id}/dispatches/{dispatch_id}/advance`
- Current dispatch state contract includes:
  - `submitted`
  - `received`
  - `rejected`
  - `expired`
  - `not_executed`
- Start may not cross a blocking latest `submitted` dispatch until that dispatch is received or otherwise resolved.
- Dispatch visibility must remain placeholder-only:
  - not a real scheduler queue item
  - not a worker lease or claim
  - not a callback contract proving background execution
  - not proof that real file generation, storage, or delivery exists

## Step-34 Rules
- Export-job list/detail contracts now expose dispatch-side summaries through:
  - `dispatch_count`
  - `latest_dispatch`
  - `can_dispatch`
  - `can_redispatch`
  - `latest_dispatch_event`
- `can_start` must stay aligned with dispatch-aware start validation:
  - `queued` alone is not enough
  - latest `submitted` dispatch still blocks start
- Export-job read-model hardening must preserve the current layering:
  - lifecycle fields remain the stable business-object state
  - dispatch fields remain placeholder handoff state
  - attempt fields remain placeholder execution state
- Dispatch read-model visibility must remain aligned across code, docs, and OpenAPI and must not be described as a real scheduler platform.

## Step-35 Rules
- Integration call-log lifecycle must stay separate from real external-system execution:
  - call-log state only records placeholder intended/observed status
  - it does not prove a real ERP or external API request occurred
- Current internal/admin integration-center APIs are:
  - `GET /v1/integration/connectors`
  - `POST /v1/integration/call-logs`
  - `GET /v1/integration/call-logs`
  - `GET /v1/integration/call-logs/{id}`
  - `GET /v1/integration/call-logs/{id}/executions`
  - `POST /v1/integration/call-logs/{id}/executions`
  - `POST /v1/integration/call-logs/{id}/retry`
  - `POST /v1/integration/call-logs/{id}/replay`
  - `POST /v1/integration/call-logs/{id}/executions/{execution_id}/advance`
  - `POST /v1/integration/call-logs/{id}/advance`
- Current integration call-log state contract includes:
  - `queued`
  - `sent`
  - `succeeded`
  - `failed`
  - `cancelled`
- Current integration execution state contract now includes:
  - `prepared`
  - `dispatched`
  - `received`
  - `completed`
  - `failed`
  - `cancelled`
- Integration center must remain mostly placeholder-oriented in this phase:
  - Step 57 may reuse it for narrow ERP filing trace records only
  - not a real ERP SDK / HTTP executor
  - not a callback processor

## Step-38 Rules
- Call-log lifecycle and execution lifecycle must remain layered:
  - call log = request envelope and latest business-visible outcome summary
  - execution = one concrete placeholder execution attempt beneath that envelope
- `latest_execution` and `execution_count` are read-model summaries only:
  - they are not proof that a real external request completed
  - they do not imply a real retry scheduler or async platform exists
- `retry` and `replay` must stay separated on the execution boundary:
  - `retry` creates a new execution attempt only for retryable failed outcomes
  - `replay` creates a new execution attempt to re-drive a recorded call-log envelope and may be allowed after succeeded, failed, or cancelled outcomes
  - `retry_count` / `replay_count` and latest retry/replay action summaries are derived from persisted executions only
  - `retryability_reason` / `replayability_reason` are advisory read-model hints only and do not imply an actual retry engine exists
- `POST /v1/integration/call-logs/{id}/advance` remains compatibility-only:
  - `queued` may directly requeue the parent call log
  - `sent|succeeded|failed|cancelled` must reuse the explicit execution boundary instead of creating a second lifecycle implementation
- Step 38 remains intentionally limited:
  - no real ERP / HTTP / SDK execution
  - no callback processor
  - no retry scheduler
  - no signature/auth negotiation with external systems
  - no message queue or async worker platform

## Step-47 Rules
- Step 47 is replay/retry hardening only; it does not introduce a real ERP / HTTP / SDK executor, callback processor, retry scheduler, signature/auth layer, or async platform.
- Current retry / replay admission boundary is:
  - `POST /v1/integration/call-logs/{id}/retry`
  - `POST /v1/integration/call-logs/{id}/replay`
- Current semantics must stay explicit:
  - `retry` is for retryable failed executions
  - `replay` is for re-driving the recorded call-log envelope through a new execution attempt
  - both actions create a new `integration_call_executions` row rather than introducing a second action table
- Current execution traceability is:
  - `action_type` and `trigger_source` distinguish `start` / `retry` / `replay` / compatibility executions
  - `latest_retry_action` / `latest_replay_action` summarize the newest matching execution result
  - no separate retry/replay event stream exists yet
- Current state admission defaults are:
  - `failed` + retryable latest execution => retry allowed, replay allowed
  - `succeeded` => retry denied, replay allowed
  - `cancelled` => retry denied, replay allowed
  - `queued` / `sent` or unresolved latest execution => neither retry nor replay allowed

## Step-39 Rules
- Export center must now keep five layers visibly separate:
  - export job lifecycle = business-visible export object state
  - dispatch = placeholder adapter handoff state
  - attempt = one concrete placeholder execution try
  - storage = placeholder result representation through `result_ref`
  - delivery = placeholder claim/read/refresh handoff over `result_ref`
- Export-job list/detail read models now additionally expose planning-only boundary fields:
  - `adapter_mode`
  - `storage_mode`
  - `delivery_mode`
  - `execution_boundary`
  - `storage_boundary`
  - `delivery_boundary`
- These new boundary fields are planning/hardening skeletons only:
  - they do not prove a real runner exists
  - they do not prove a real file was generated
  - they do not prove a real storage object or download service exists
- Current responsibility split is:
  - start execution -> `POST /v1/export-jobs/{id}/start`
  - adapter handoff -> `export_job_dispatches`
  - execution try -> `export_job_attempts`
  - placeholder result-state minting -> export-job lifecycle advance
  - placeholder storage representation -> `result_ref`
  - placeholder delivery handoff -> `claim-download` / `download` / `refresh-download`
- Future real infrastructure must replace the boundary layers beneath those contracts rather than collapsing lifecycle, dispatch, attempt, storage, and delivery back into one field or one endpoint family.

## Step-40 Rules
- Cost-rule governance is still intentionally table-hardening on `cost_rules`; this phase does not introduce a separate `cost_rule_versions` subsystem.
- Current governed cost-rule row semantics are:
  - `rule_version` = additive lineage number on the current row
  - `supersedes_rule_id` = backward link to the immediately prior governed row when used
  - `superseded_by_rule_id` = derived forward link exposed on reads
  - `effective_from/effective_to/is_active` = current effective-window boundary
  - `governance_note` = lightweight governance context only
  - `governance_status` = derived effective-window status, not approval state
- Current `governance_status` values are:
  - `inactive`
  - `scheduled`
  - `effective`
  - `expired`
- `POST /v1/cost-rules/preview` now explicitly returns governed hit trace through:
  - `matched_rule_id`
  - `matched_rule_version`
  - `rule_source`
  - `governance_status`
  - existing `explanation`
- `PATCH /v1/tasks/{id}/business-info` now explicitly snapshots task-side prefill governance through:
  - `matched_rule_version`
  - `prefill_source`
  - `prefill_at`
- `PATCH /v1/tasks/{id}/business-info` now explicitly snapshots manual override governance through:
  - `manual_cost_override`
  - `manual_cost_override_reason`
  - `override_actor`
  - `override_at`
- Override governance remains intentionally lightweight:
  - no real auth identity platform
  - no real approval workflow
  - no field-level permission engine
- Current history stability policy is:
  - later rule changes do not auto-recompute old tasks
  - historical task-side cost state remains the last persisted snapshot
  - new rule changes affect future preview and future business-info prefill only
- Procurement remains separate from this governance work:
  - `procurement_price` still belongs to procurement records
  - cost-rule governance fields remain under task/business-info and procurement summary read models only

## Step-41 Rules
- Step 41 is read-model hardening on top of Step 40; it does not change the Step 40 write boundary.
- Cost-rule lineage can now be read through:
  - additive `CostRule` read fields:
    - `version_chain_summary`
    - `previous_version`
    - `next_version`
    - `supersession_depth`
  - dedicated lineage endpoint:
    - `GET /v1/cost-rules/{id}/history`
- Task-side cost governance now has explicit read-model layering:
  - `matched_rule_governance.matched_rule` = historical matched snapshot
  - `matched_rule_governance.current_rule` = latest reachable rule in the same lineage at read time
  - `override_summary` = lightweight override history summary derived from `task_event_logs`
- Current override audit scope remains intentionally lightweight:
  - latest task-detail fields still store the current override snapshot
  - task events provide enough history for summary-grade read models
  - no separate approval flow or dedicated override-history table is introduced in this phase
- Current rule-history and task-history policy remains:
  - later rule changes still affect future preview/prefill only
  - already-persisted task-side hits are not auto-recomputed
  - read models must distinguish historical matched rule from current latest rule state

## Step-42 Rules
- Step 42 is governance-audit hardening on top of Step 40 / Step 41; it does not introduce approval flow, finance integration, or ERP cost writeback.
- Dedicated override governance audit now persists through:
  - `cost_override_events`
  - `cost_override_event_sequences`
- `PATCH /v1/tasks/{id}/business-info` now writes two layers when override state changes:
  - `task_event_logs` remains the general task event stream
  - `cost_override_events` is the governance-specific audit layer
- Current dedicated override audit event contract includes:
  - `event_id`
  - `task_id`
  - `task_detail_id`
  - `sequence`
  - `event_type`
  - `category_code`
  - `matched_rule_id`
  - `matched_rule_version`
  - `matched_rule_source`
  - `governance_status`
  - `previous_estimated_cost`
  - `previous_cost_price`
  - `override_cost`
  - `result_cost_price`
  - `override_reason`
  - `override_actor`
  - `override_at`
  - `source`
  - `note`
- Current audit event types are:
  - `override_applied`
  - `override_updated`
  - `override_released`
- Current read-model layering must stay explicit:
  - rule lineage = cost-rule governance/history layer
  - matched rule snapshot = task-side historical hit layer
  - override summary = stable lightweight consumer summary
  - governance audit summary / timeline = dedicated override audit layer
- `GET /v1/tasks/{id}/cost-overrides` is read-only timeline visibility for the dedicated override audit skeleton.
- `override_summary` remains backward-compatible:
  - it now prefers `cost_override_events`
  - it falls back to `task_event_logs` when older tasks have no dedicated override audit events
- Current boundary remains explicit:
  - not a real approval workflow
  - not a real finance ledger
  - not a real ERP cost writeback contract

## Step-49 Rules
- Step 49 is policy scaffolding only; it must not be described as a real identity, org, or permission platform rollout.
- Current policy scaffolding layer is additive and read-model oriented:
  - `policy_mode`
  - `visible_to_roles`
  - `action_roles`
  - `policy_scope_summary`
- Current cross-center coverage is:
  - task center (task list/read/detail/board/procurement summary)
  - export center (export job)
  - integration center (call log / execution)
  - cost governance boundary (override governance boundary)
  - upload/storage boundary (upload request)
- Current policy language is default-role summary only:
  - it describes route-aligned default visibility and action intent
  - it does not perform runtime field-level, row-level, or org-tree data trimming
- Existing route-level role-gate contract must stay aligned:
  - request headers:
    - `X-Debug-Actor-Id`
    - `X-Debug-Actor-Roles`
  - middleware contract:
    - `withAccessMeta(...)`
  - response headers:
    - `X-Workflow-Auth-Mode`
    - `X-Workflow-API-Readiness`
    - `X-Workflow-Required-Roles`
- Step 49 must explicitly defer:
  - real login/session/SSO
  - real org hierarchy sync
  - final fine-grained RBAC/ABAC
  - full approval permission system implementation

## Step-50 Rules
- Step 50 is KPI/finance/report entry-boundary scaffolding only; it must not be described as a real BI, finance/accounting, or report-platform rollout.
- Current unified platform-entry language is additive and read-model oriented:
  - `platform_entry_boundary`
  - `kpi_entry_summary`
  - `finance_entry_summary`
  - `report_entry_summary`
- Current cross-center coverage is:
  - task center (`task_list_item`, `task_read_model`, `task_detail_aggregate`)
  - procurement summary (`procurement_summary`)
  - cost governance boundary (`override_governance_boundary`)
  - export center (`export_job`)
- Current entry summaries are boundary declarations only:
  - they enumerate current source read-model fields
  - they enumerate placeholder-only future fields
  - they expose `eligible_now` and conditional readiness hints
  - they do not execute real KPI calculations
  - they do not execute real accounting/ledger/reconciliation/settlement/invoice logic
  - they do not execute real report generation or analytics jobs
- Step 50 must explicitly defer:
  - real KPI/BI statistical engines
  - real finance/accounting posting systems
  - real report-generation engines
  - real data warehouse / analytics infrastructure
  - real ERP finance integration
  - real settlement/reconciliation/invoice pipelines

## Step-63 Rules
- Step 63 is a frontend-linkable auth payload/config hardening pass only; it must not be described as a full org/permission platform rebuild.
- Current auth mainline now additionally includes:
  - fixed department enum validation on register
  - required `mobile` plus optional `email`
  - config-backed department-admin key matching during register
  - config-backed default super-admin bootstrap instead of "first registered user becomes Admin"
  - `PUT /v1/auth/password` for current-user password change
- Current persisted auth/user profile boundary now additionally includes:
  - `users.department`
  - `users.mobile`
  - `users.email`
  - `users.is_config_super_admin`
  - current phone uniqueness policy through `uq_users_mobile`
- Current auth configuration boundary is explicit file-driven:
  - `config/auth_identity.json`
  - `config/frontend_access.json`
  - `AUTH_SETTINGS_FILE`
  - `FRONTEND_ACCESS_SETTINGS_FILE`
- Current frontend visibility contract now additionally includes:
  - `WorkflowUser.account`
  - `WorkflowUser.name`
  - `WorkflowUser.department`
  - `WorkflowUser.mobile`
  - `WorkflowUser.phone`
  - `WorkflowUser.email`
  - `frontend_access.is_department_admin`
  - `frontend_access.department`
  - `frontend_access.managed_departments`
- Current department-admin behavior is intentionally narrow:
  - promotion is registration-time only via configured department key match
  - effective marker role is `DepartmentAdmin`
  - current frontend visibility can key off `roles`, `is_department_admin`, and `managed_departments`
  - no org tree, row filtering, or department data isolation is introduced yet
- Step 63 must explicitly defer:
  - org hierarchy / organization tree
  - ABAC / row-level / field-level visibility redesign
  - full admin UI for auth configuration
  - external identity provider / SSO rollout

## Step-66 Rules
- Step 66 is the minimal frontend-linkable org/auth completion slice on top of Step 63; it must not be described as a full organization platform or ABAC rollout.
- Current auth/org mainline now additionally includes:
  - `GET /v1/auth/register-options` for fixed department/team register-form initialization
  - persisted `users.team`
  - registration-time team-to-department relation validation
  - explicit `frontend_access.roles/scopes/menus/pages/actions/modules`
  - `GET /v1/operation-logs` aggregated read model for frontline-process visibility
- Current fixed organization scope is intentionally narrow:
  - department = fixed configured enum
  - team = flat per-department configured list
  - no organization tree
  - no team-admin role
  - no row-level department data isolation platform
- Current HR visibility scope is intentionally read-oriented:
  - HR can read org/user/permission/operation visibility endpoints for frontend联调
  - admin write operations remain role-managed separately
- Current ERP visibility contract is explicit:
  - all authenticated users can read `/v1/erp/products`
  - all authenticated users can read `/v1/erp/products/{id}`
  - all authenticated users can read `/v1/erp/categories`
- Step 66 must explicitly defer:
  - organization tree / recursive departments
  - team-admin role
  - full permission-management UI
  - ABAC / row-level / field-level visibility platform
  - cross-department workflow-data scoping redesign

## Step-52 Rules
- Step 52 is minimal real identity/auth and permission-log hardening only; it must not be described as full SSO, org sync, or deep RBAC/ABAC rollout.
- Current real auth coverage now includes:
  - `POST /v1/auth/register`
  - `POST /v1/auth/login`
  - `GET /v1/auth/me`
  - persisted `users`, `user_roles`, `user_sessions`
- Current user/permission admin coverage now includes:
  - `GET /v1/roles`
  - `GET /v1/users`
  - `GET /v1/users/:id`
  - `PATCH /v1/users/:id`
  - `POST /v1/users/:id/roles`
  - `PUT /v1/users/:id/roles`
  - `DELETE /v1/users/:id/roles/:role`
  - `GET /v1/permission-logs`
- Current session model is intentionally minimal:
  - bearer token backed by persisted session rows
  - local password-hash login only
  - config-managed super admins bootstrap as `Admin`
  - regular registered users start without `Admin` unless separately role-assigned
- Current auth response contract now includes frontend-facing permission visibility:
  - `POST /v1/auth/register` -> `data.user.frontend_access.*`
  - `POST /v1/auth/login` -> `data.user.frontend_access.*`
  - `GET /v1/auth/me` -> `data.frontend_access.*`
  - `frontend_access.is_super_admin`
  - `frontend_access.permission_flags`
  - `frontend_access.page_keys`
  - `frontend_access.menu_keys`
  - `frontend_access.module_keys`
  - `frontend_access.access_scopes`
- Current permission logging scope now covers both route access and key auth/role actions:
  - actor id / source / auth mode
  - actor username
  - action type
  - target user id / username / roles
  - actor roles / required roles
  - method / route path
  - route readiness / session-required / debug-compatible policy context
  - granted / denied result
- Current role-assignment behavior is frontend-linkable and audit-backed:
  - admins can add roles with `POST /v1/users/:id/roles`
  - admins can replace roles with `PUT /v1/users/:id/roles`
  - admins can remove one role with `DELETE /v1/users/:id/roles/:role`
  - the backend now prevents removing the last remaining `Admin` role from the system
  - role changes are reflected on the next `GET /v1/auth/me` without requiring a fresh login
- Step 56 clarifies current Step B authorization scope:
  - `ready_for_frontend` route auth is now session-backed by default
  - debug-header role checks remain compatibility-only on internal/mock placeholder routes
  - admins can inspect route-role requirements through `GET /v1/access-rules`
  - this is still not a full RBAC/ABAC redesign
- Step 52 must explicitly defer:
  - SSO / OAuth / external identity provider integration
  - org hierarchy sync
  - field-level / row-level ABAC
  - refresh-token / multi-device session governance platform
  - final permission-engine redesign

## Step-51 Rules
- Step 51 is upload-request management query hardening only; it must not be described as a real upload platform rollout.
- Current upload-request internal management coverage now includes:
  - `GET /v1/assets/upload-requests`
  - `POST /v1/assets/upload-requests`
  - `GET /v1/assets/upload-requests/{id}`
  - `POST /v1/assets/upload-requests/{id}/advance`
- Current list/filter semantics are limited to placeholder management fields:
  - `owner_type`
  - `owner_id`
  - `task_asset_type`
  - `status`
  - pagination
- Current upload-request list/read models remain boundary-only:
  - they expose placeholder lifecycle and policy summaries
  - they do not imply upload byte transfer
  - they do not imply signed URL issuance
  - they do not imply NAS/object-storage backing
- Step 51 must explicitly defer:
  - real upload session allocation
  - real multipart/chunk/resume flows
  - real signed URL / presigned policy generation
  - real NAS / object-storage integration
  - real file delivery / file verification services

## Step-46 Rules
- Step 46 is upload-request lifecycle hardening only; it does not introduce a real upload platform, byte-transfer session, signed URL system, NAS integration, or object-storage adapter.
- Current upload-request lifecycle semantics are:
  - `requested`
  - `bound`
  - `expired`
  - `cancelled`
- Current explicit lifecycle actions are:
  - `POST /v1/assets/upload-requests/:id/advance` with `cancel`
  - `POST /v1/assets/upload-requests/:id/advance` with `expire`
- `bound` must remain reserved for task-asset binding through:
  - `POST /v1/tasks/:id/submit-design`
  - `POST /v1/tasks/:id/assets/mock-upload`
- `can_bind` / `can_cancel` / `can_expire` are additive read-model hints only:
  - they are not proof of a real upload allocator
  - they are not proof of a background expiry worker
  - they are not proof that bytes exist in external storage

## Step-45 Rules
- Step 45 is cross-center adapter-boundary consolidation only; it unifies terminology and shared read-model summaries across export, integration, and storage/upload without merging their tables or introducing real infrastructure.
- Current shared terminology boundaries are:
  - `adapter_mode` = the center-level adapter strategy / layering choice
  - `execution_mode` = the center-specific execution trigger style that remains owned by export or integration
  - `dispatch_mode` = the cross-boundary handoff progression style
  - `storage_mode` = the placeholder resource-representation style
  - `delivery_mode` = the consumer-facing handoff style and currently remains export-center-specific
- Current shared reference summaries are:
  - `adapter_ref_summary` = minimal adapter / connector / storage-adapter identity
  - `resource_ref_summary` = minimal result/storage resource reference metadata
  - `handoff_ref_summary` = minimal dispatch / execution / upload-request handoff reference
- Current center-specific semantics must stay explicit:
  - export keeps lifecycle / dispatch / attempt / result / delivery layers
  - integration keeps call-log / execution layers
  - storage/upload keeps upload-request / storage-ref layers
- These shared summaries are additive language only:
  - not a scheduler protocol
  - not a storage API contract
  - not a queue platform
  - not proof that export/integration/storage are implemented by one backend subsystem

## Step-44 Rules
- Step 44 is governance-boundary read-model consolidation on top of Step 43; it does not introduce a real approval workflow, finance system, accounting contract, or ERP writeback.
- Current read-model layering must stay explicit:
  - rule history = cost-rule governance / lineage layer
  - matched snapshot / prefill trace = task-side historical rule-hit layer
  - override audit = `cost_override_events`
  - approval placeholder = `cost_override_reviews`
  - finance placeholder = `cost_override_finance_flags`
  - boundary summary = stable read-model aggregation over approval / finance placeholder layers
- `override_governance_boundary` is now the unified boundary object across:
  - `GET /v1/tasks/{id}`
  - `GET /v1/tasks/{id}/detail`
  - purchase-task `procurement_summary`
  - `GET /v1/tasks/{id}/cost-overrides`
- Current unified boundary contract now additionally exposes:
  - `governance_boundary_summary`
  - `approval_placeholder_summary`
  - `finance_placeholder_summary`
  - `latest_review_action`
  - `latest_finance_action`
  - `latest_boundary_actor`
  - `latest_boundary_at`
- These read fields are stable frontend-facing summaries only:
  - they do not imply a real approval system
  - they do not imply a real finance / accounting system
  - they do not imply ERP cost writeback
- Current internal placeholder write actions remain unchanged:
  - `POST /v1/tasks/{id}/cost-overrides/{event_id}/review`
  - `POST /v1/tasks/{id}/cost-overrides/{event_id}/finance-mark`

## Step-43 Rules
- Step 43 is approval / finance placeholder-boundary hardening on top of Step 42; it does not introduce a real approval workflow, finance system, accounting contract, or ERP writeback.
- Dedicated placeholder persistence now exists through:
  - `cost_override_reviews`
  - `cost_override_finance_flags`
- Current layering must stay explicit:
  - rule history = cost-rule governance / lineage layer
  - override audit = `cost_override_events`
  - approval placeholder = `cost_override_reviews`
  - finance placeholder = `cost_override_finance_flags`
- Task read/detail and purchase-task `procurement_summary` now additionally expose:
  - `override_governance_boundary`
- `GET /v1/tasks/{id}/cost-overrides` now exposes:
  - per-event `override_governance_boundary`
  - top-level latest/current `override_governance_boundary`
- Current placeholder boundary contract can express:
  - whether review is required
  - current placeholder review status
  - whether finance follow-up is required
  - current placeholder finance status
  - whether the override is ready to enter the future finance-facing layer
- Current internal placeholder write actions are:
  - `POST /v1/tasks/{id}/cost-overrides/{event_id}/review`
  - `POST /v1/tasks/{id}/cost-overrides/{event_id}/finance-mark`
- Current boundary remains explicit:
  - not a real identity approval chain
  - not a ledger / reconciliation / settlement / invoice system
  - not a real ERP cost writeback contract

## Step-36 Rules
- Route-level role enforcement now applies to V7 routes that declare `required_roles` through `withAccessMeta(...)`.
  - Current mainline actor source contract is:
    - preferred request auth: `Authorization: Bearer <token>`
    - debug compatibility headers:
      - `X-Debug-Actor-Id`
      - `X-Debug-Actor-Roles`
    - missing headers no longer imply a synthetic authenticated actor
  - Current auth mode contract is:
    - `debug_header_role_enforced`
    - `session_token_role_enforced`
  - Current enforcement rule is:
  - actor must satisfy at least one declared route role
  - `Admin` is accepted as a placeholder override
- Step 36 remains intentionally limited:
  - no login/session module
  - no org/department/team model
  - no task-level visibility trimming
  - no field-level permission system

## Step-37 Rules
- `task_assets` remain the business asset timeline; do not turn them into a real file-service table.
- `upload_requests` are placeholder upload-intent records only:
  - they do not upload bytes
  - they do not allocate signed URLs
  - they do not prove a real file exists
  - Step 46 only adds explicit `cancel` / `expire` placeholder lifecycle actions plus additive `can_bind` / `can_cancel` / `can_expire` read-model hints
- `asset_storage_refs` are placeholder storage-reference records only:
  - they carry adapter/type/key/metadata semantics
  - they do not prove NAS/object-storage integration exists
  - they do not replace export `result_ref`, but they should stay semantically aligned with it
- `POST /v1/tasks/{id}/submit-design` and `POST /v1/tasks/{id}/assets/mock-upload` may bind an `upload_request_id`, but they must still work without one by auto-creating placeholder `storage_ref` metadata.
- legacy `file_path` / `whole_hash` remain additive compatibility fields in this phase:
  - they are no longer the preferred long-term storage boundary
  - `whole_hash` remains checksum-hint metadata only, not strict content verification
- Step 37 remains intentionally limited:
  - no real file upload
  - no chunking / resume
  - no signed URL issuance

## Step-67 Rules
- `design_assets` are now the task-scoped asset roots for the design asset center:
  - they group business assets by `asset_type`
  - they track `current_version_id`
  - they do not store bytes
- `task_assets` now additionally act as the asset-version persistence layer beneath that root:
  - legacy task-timeline semantics remain compatible
  - asset-center versions are identified through `asset_id + asset_version_no`
- `upload_requests` continue to be the persisted upload handoff/session table:
  - asset-center APIs now project them as `upload_session`
  - they may carry `remote_upload_id`, `upload_mode`, and `expires_at`
  - they still do not prove MAIN uploaded or stored bytes
- the new upload-service client boundary is preparatory only:
  - MAIN calls an abstraction for small-file and multipart flows
  - the current implementation is stub/placeholder
  - later NAS-side Go service should plug in beneath this contract rather than collapsing task logic and storage logic back together
  - no NAS / object storage
  - no CDN / download service
  - not a retry scheduler
  - not a distributed integration worker platform

## Step-30 Rules
- Current frontend-ready placeholder download-handoff APIs are:
  - `POST /v1/export-jobs/{id}/claim-download`
  - `GET /v1/export-jobs/{id}/download`
  - `POST /v1/export-jobs/{id}/refresh-download`
- Claim/read are now valid only when:
  - export job status is `ready`
  - `download_ready=true`
  - current placeholder handoff is not expired
- Ready but expired handoff must be treated distinctly from non-ready jobs:
  - claim/read return a clear placeholder-expired invalid-state response
  - response details expose `is_expired=true`
  - response details expose `can_refresh=true`
- Export-job list/detail read models now expose:
  - `is_expired`
  - `can_refresh`
- Current handoff response now exposes:
  - `claim_available`
  - `read_available`
  - `is_expired`
  - `can_refresh`
- Refresh is currently allowed only when:
  - export job status is `ready`
  - `download_ready=true`
  - current placeholder handoff is expired
- Refresh updates placeholder handoff metadata by:
  - rotating `result_ref.ref_key`
  - updating `expires_at`
  - keeping this as placeholder metadata only, not signed URL renewal or real storage refresh
- Expiry/refresh must stay inside the existing export-job event chain:
  - `export_job.download_expired`
  - `export_job.download_refreshed`
  - `export_job.result_ref_updated`
- `download_ready` remains lifecycle readiness, not active-access readiness; expired ready jobs may still have `download_ready=true`.
- This step remains intentionally limited:
  - no real file generation
  - no real file-byte download
  - no signed URL delivery
  - no NAS / object-storage integration
  - no async runner platform

## Step-12 Rules
- `procurement_summary` on `/v1/tasks`, `/v1/tasks/{id}`, and `/v1/tasks/{id}/detail` now carries stable procurement-to-warehouse coordination fields:
  - `coordination_status`
  - `coordination_label`
  - `warehouse_status`
  - `warehouse_prepare_ready`
  - `warehouse_receive_ready`
- purchase-task coordination status is derived as:
  - draft / prepared -> `preparing`
  - in_progress -> `awaiting_arrival`
  - completed before handoff -> `ready_for_warehouse`
  - pending warehouse / receipt present -> `handed_to_warehouse`
  - warehouse completed / pending close / closed -> `warehouse_completed`
- purchase-task procurement lane sub-status is now interpreted as:
  - `pending_inbound` -> awaiting arrival
  - `ready` -> ready for warehouse
- purchase-task warehouse prepare now requires:
  - business info present
  - procurement record present
  - procurement price present
  - procurement quantity present
  - procurement status `completed`

## Known Gaps
- Minimal real login/session plus route-level role enforcement now exists, but there is still no org hierarchy, SSO, row-level visibility trimming, or deep RBAC/ABAC system
- Step 50 now adds unified KPI/finance/report entry boundaries on task/procurement/cost/export read models, but there is still:
  - no real KPI/BI computation engine
  - no real finance/accounting/reconciliation/settlement/invoice system
  - no real report-generation platform
  - no real data warehouse or analytics engine behind these entry summaries
- Task asset storage/upload adapter boundary now uses the same shared adapter/resource/handoff summary language as export/integration, but there is still no real file upload, NAS integration, object storage, signed URL flow, or file-service delivery path
- `whole_hash` is still passthrough metadata without strict verification
- Upload-request management visibility now supports paginated owner/type/status filtering, but task-asset list query/filter capabilities are still basic
- Some STEP_05 filters use straightforward SQL `LIKE` or derived predicates and are not index-optimized yet
- Converged `/v1/tasks` filter execution is now pushed into repo/read-model predicates, but those derived SQL expressions are still mostly not backed by dedicated indexes or materialized read models
- Step-18 assessment now classifies the remaining candidate-scan hotspots as:
  - heaviest:
    - unscoped `sub_status_code`
    - `warehouse_blocking_reason_code`
    - `warehouse_prepare_ready`
  - medium:
    - `coordination_status`
    - `main_status`
  - lighter:
    - `warehouse_receive_ready`
- Task-board summary/queue aggregation no longer fans out per preset queue and no longer pages through the generic list path for board candidate collection, but the repo/read-model board candidate scan still relies on derived SQL expressions and is not yet backed by dedicated indexes or materialized views
- Step-18 already applied one light optimization by joining latest task-asset projection once per task, but this is intentionally not a broad indexing or materialization phase
- Remaining task-board fan-out now splits into:
  - business-required: final overlapping queue partitioning, stable counts, sample-task selection, and per-queue pagination slicing
  - later-optimizable: predicate/index/materialized-view costs inside the repo/read-model board candidate scan
- `closable` / `cannot_close_reasons` remain per-row workflow projection cost on list/read-model hydration, but they are not currently the main board candidate-scan driver
- ERP Bridge query integration now exists for original-product selection, and Step 57 adds one narrow business-info filing upsert boundary, but there is still:
  - no broad ERP writeback platform beyond that single `filed_at`-driven product upsert path
  - no procurement / WMS / finance docking behind that query layer
  - no callback / retry scheduler / circuit-breaker platform beyond current timeout + retry-hint diagnostics and request logging
- Category center, `category_erp_mappings`, ERP Bridge query routes, and task-side `product_selection` now cover both local mapped search and bridge-backed original-product selection, but deeper second/third-level search consumption and broader ERP master-data integration are still future work
- Cost-rule center now exists, but it is still a skeleton:
  - no general-purpose formula-expression engine
  - no full pricing-audit/versioning workflow
  - some `size_based_formula` cases still fall back to manual review
  - missing area/size inputs can still force manual review instead of estimate generation
- Task-side override governance now has a dedicated audit skeleton plus a unified placeholder-boundary read model, but there is still:
  - no real approval flow or approval actor model
  - no backfill of historical override audit rows from older `task_event_logs`
  - no finance-side posting or ERP cost writeback beneath the placeholder boundary summaries
- Stored `task_status` still mixes legacy operational states with PRD mainline semantics; Step-10 stabilizes the read contract but does not yet redesign the persisted status model end-to-end
- Procurement now has a minimal lifecycle plus derived warehouse-coordination summary, but there is still no full supplier lifecycle, inbound receipt loop, or ERP procurement integration
- Task-board queues remain preset-derived only; Step 19 adds advisory ownership hints and lightweight placeholder-actor-scoped saved preferences, but there is still no real auth-backed queue ownership, permission trimming, or full per-user inbox persistence
- Export center now has a minimal lifecycle, lifecycle audit trace, placeholder dispatch/attempt seams, explicit dispatch/start/attempt/redispatch admission reasons, explicit planning-only runner/storage/delivery boundaries, placeholder download expiry/refresh boundary, and shared cross-center adapter/resource/handoff summaries, but there is still:
  - no real file-generation execution
  - no storage/NAS integration
  - no signed download delivery or download-token flow
  - no real async runner or scheduler driving export jobs automatically beyond the explicit placeholder `POST /v1/export-jobs/{id}/start` initiation boundary and the current planning-only dispatch/attempt seam
  - no real runner-log stream or delivery telemetry beyond the current lifecycle and handoff audit trace
  - no background expirer; placeholder expiry is still detected lazily on access/refresh attempts
- Integration center now has a placeholder connector catalog, API call-log skeleton, execution-attempt boundary, and shared cross-center adapter/handoff summaries, but there is still:
  - no real ERP or external API execution
  - no retry scheduler or callback processor
  - no signature/auth negotiation with external systems
  - no delivery guarantee or idempotency protocol beyond stored placeholder call logs and execution attempts

## Step-73 Rules (8081 Bridge Adapter Completion)
- Current v0.4 runtime target remains three-service shape:
  - `8080` = MAIN business layer
  - `8081` = Bridge ERP/JST adapter layer
  - `8082` = resident JST sync service
- 8081 Bridge ERP adapter surface now includes:
  - `GET /v1/erp/products`
  - `GET /v1/erp/products/{id}`
  - `GET /v1/erp/categories`
  - `POST /v1/erp/products/upsert`
  - `GET /v1/erp/sync-logs`
  - `GET /v1/erp/sync-logs/{id}`
  - `POST /v1/erp/products/shelve/batch`
  - `POST /v1/erp/products/unshelve/batch`
  - `POST /v1/erp/inventory/virtual-qty`
- Missing-404 gap for sync-log/shelve/unshelve/virtual-qty is closed in current repo code path (`transport/http.go` + handler/service/client chain).
- local Bridge mutation path now records integration connector logs for:
  - `erp_bridge_product_upsert`
  - `erp_bridge_product_shelve_batch`
  - `erp_bridge_product_unshelve_batch`
  - `erp_bridge_inventory_virtual_qty`
- `GET /v1/erp/sync-logs` now serves mutation observability from Bridge-side integration call-log data (or remote client path in remote/hybrid mode).
- Bridge remote/hybrid path coverage now includes:
  - upsert
  - shelve batch
  - unshelve batch
  - virtual qty
  - sync-log list/detail
- 8082 is explicitly preserved as a resident JST sync service and is not merged into 8081.
- MAIN should continue converging toward Bridge-only ERP/JST business calls and must not directly own JST/OpenWeb adapter details.

## Step-74 Rules (Live Runtime Verification and 8082 Recovery)
- Verified on live host `223.4.249.11` through SSH localhost probing.
- 8081 runtime evidence:
  - `GET /health` = `200`
  - ERP adapter routes (query + write + newly added mutation/sync-log routes) return `401` under session-backed auth policy, proving mounted route/middleware chain and closing prior `404` gap.
- 8082 runtime evidence:
  - Initial state: `8082` not listening and previous `erp_sync.pid` was stale.
  - Recovery action: `/root/ecommerce_ai/scripts/start-sync.sh --base-dir /root/ecommerce_ai`
  - After recovery: `GET /health` = `200`, `GET /internal/jst/ping` = `200`, `POST /jst/sync/inc` = `200`.
- Coexistence evidence:
  - `ss -ltnp` shows simultaneous listeners for `8080` (`ecommerce-api`), `8081` (`erp_bridge`), and `8082` (`erp_bridge_sync`).
  - `/proc/<pid>/exe` symlinks are intact (no `(deleted)` suffix).
- Operational note:
  - Current `deploy/deploy.sh` cutover flow starts `8080` and `8081`, but does not auto-start resident `8082` sync service.
  - Post-deploy checklist must include explicit 8082 process health/start verification.

## Step-75 Rules (Bridge Token Acceptance and Three-Service Recovery Automation)
- Verified on live host `223.4.249.11` through SSH localhost probing with a real bearer session obtained from `POST /v1/auth/login`.
- Confirmed 8081 ERP success-path acceptance with real token:
  - Login returned `200` and issued a persisted bearer session; `GET /v1/auth/me` returned `200`.
  - `GET /v1/erp/categories` returned `200` with category data.
  - `POST /v1/erp/products/upsert` returned `200` for test product `bridge-accept-1773724718` / `BRIDGE-ACCEPT-1773724718`, with `sync_log_id=5`.
  - `GET /v1/erp/products?sku_code=BRIDGE-ACCEPT-1773724718&page=1&page_size=5` returned `200` with one matched product.
  - `GET /v1/erp/products/bridge-accept-1773724718` returned `200`.
  - `POST /v1/erp/products/shelve/batch` returned `200` with `sync_log_id=6`.
  - `POST /v1/erp/products/unshelve/batch` returned `200` with `sync_log_id=7`.
  - `POST /v1/erp/inventory/virtual-qty` returned `200` with `sync_log_id=8`.
  - `GET /v1/erp/sync-logs?page=1&page_size=20` returned `200` with mutation call-log data.
  - `GET /v1/erp/sync-logs/5` returned `200` with request/response payload snapshots for the upsert call.
- Confirmed 8081 ERP failure-path acceptance:
  - Unauthenticated `GET /v1/erp/products` returned `401 UNAUTHORIZED`.
  - Unauthenticated `POST /v1/erp/products/upsert` returned `401 UNAUTHORIZED`.
  - `GET /v1/erp/products?page=x&page_size=1` returned `400 INVALID_REQUEST`.
  - `POST /v1/erp/products/upsert` with `{}` returned `400 INVALID_REQUEST`.
  - `POST /v1/erp/products/shelve/batch` with empty items returned `400 INVALID_REQUEST`.
  - `POST /v1/erp/inventory/virtual-qty` with empty items returned `400 INVALID_REQUEST`.
  - Missing product detail and missing sync-log detail returned `404 NOT_FOUND`.
- Confirmed 8081 request-log evidence:
  - Bridge log file `/root/ecommerce_ai/logs/erp_bridge-20260317T042205Z.log` contains `http_request` entries for `/v1/erp/products/upsert` (`401`, `400`, `200`), `/v1/erp/sync-logs` (`200`), `/v1/erp/sync-logs/5` (`200`), `/v1/erp/products/shelve/batch` (`400`, `200`), `/v1/erp/products/unshelve/batch` (`200`), and `/v1/erp/inventory/virtual-qty` (`400`, `200`).
- Confirmed deploy/runtime root cause for 8082 drift:
  - `deploy/remote-deploy.sh` cutover `--start-services` branch only stops/starts `8080` and `8081`.
  - Historical packaged helper list did not include `start-sync.sh`, `stop-sync.sh`, or a three-service runtime checker.
  - Historical `deploy/verify-runtime.sh` only checked `8080` TCP/auth state and did not inspect or recover `8082`.
- Added runtime automation:
  - New `deploy/check-three-services.sh` checks `8080/8081/8082` health, pid existence, TCP listeners, and `/proc/<pid>/exe` deleted state, emits human-readable lines plus machine-readable `KEY=VALUE` and `JSON_SUMMARY`.
  - Added `deploy/start-sync.sh` and `deploy/stop-sync.sh` to source control so future packages carry the same 8082 lifecycle helpers already used on the host.
  - `deploy/verify-runtime.sh` now delegates to the three-service checker.
  - `deploy/deploy.sh` cutover post-verify now passes `--auto-recover-8082`; parallel deploy explicitly skips the three-service check to avoid changing candidate-port verification semantics.
- Live runtime-recovery verification:
  - Normal `check-three-services.sh --auto-recover-8082` run reported all three services `status=ok`, `OVERALL_OK=true`, and no `(deleted)` executables.
  - After `stop-sync.sh --base-dir /root/ecommerce_ai`, `curl http://127.0.0.1:8082/health` returned `000`.
  - Re-running `check-three-services.sh --base-dir /root/ecommerce_ai --auto-recover-8082` reported `SYNC_RECOVER_TRIGGERED=true`, `SYNC_RECOVER_SUCCESS=true`, and restarted `erp_bridge_sync` as pid `3498341`.
  - After recovery, `GET /health`, `GET /internal/jst/ping`, and `POST /jst/sync/inc` on `8082` all returned `200`.
## 2026-03-19 task-create reference upload closure
- `POST /v1/tasks` reference-image contract is now closed:
  - formal input: `reference_file_refs`
  - formal element type: reference object array
  - deprecated input: `reference_images`
  - runtime behavior: any `reference_images` field in create payload returns `400 INVALID_REQUEST`
- New formal pre-task reference upload path:
  - `POST /v1/tasks/reference-upload`
  - request content-type: `multipart/form-data`
  - file field name: `file`
  - frontend passes the returned ref object directly into `POST /v1/tasks.reference_file_refs`
- Compatibility pre-task asset-center path remains available:
  - `POST /v1/task-create/asset-center/upload-sessions`
  - `POST /v1/task-create/asset-center/upload-sessions/{session_id}/complete`
- Backend source validation is enforced:
  - ref must exist in `asset_storage_refs`
  - ref must point back to a completed `upload_requests` record
  - upload must be `reference` type, `bound`, and `completed`
  - ref owner must match the task creator on create
- New task creation no longer serializes base64 reference images into `task_details.reference_images_json`; new creates write `[]` there and only persist `reference_file_refs_json`.
- This supersedes the same-day hotfix note that still allowed `reference_images` as a small-image create path.

## 2026-03-23 task read-model asset-version visibility closure
- Final boundary for this round:
  - `POST /v1/tasks/{id}/assets/upload-sessions/{session_id}/complete` is the persistence boundary for uploaded design-version facts.
  - once upload-complete has successfully written `design_assets`, `task_assets`, and `design_assets.current_version_id`, `GET /v1/tasks/{id}` must return those facts through `design_assets` and `asset_versions`.
  - `GET /v1/tasks/{id}` must not depend on a later `POST /v1/tasks/{id}/submit-design` call to make uploaded versions visible.
  - `submit-design` remains a business workflow action for explicit design submission / audit transition semantics; it is no longer the visibility trigger for uploaded version facts.
- Backend patch files verified in this round:
  - `domain/query_views.go`
  - `service/task_service.go`
  - `service/task_design_asset_read_model.go`
  - `cmd/server/main.go`
  - `cmd/api/main.go`
  - `docs/api/openapi.yaml`
  - `service/task_read_model_asset_versions_test.go`
  - `service/task_prd_service_test.go`
- Local verification completed before publish:
  - `go build ./cmd/server ./cmd/api` passed.
  - `go test -c ./service` passed.
  - targeted read-model coverage was added in `service/task_read_model_asset_versions_test.go`.
  - direct local execution of generated `.test.exe` remained blocked by host Application Control, so runtime test proof for this round is the live server verification below.
- Formal release closure:
  - real publish entrypoint used: `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task read model reflects upload-complete design_assets asset_versions without submit-design"`
  - release target: `jst_ecs:/root/ecommerce_ai/releases/v0.8`
  - live cutover result: `/root/ecommerce_ai/current -> /root/ecommerce_ai/releases/v0.8`
  - live runtime result: `8080/8081/8082` all `status=ok`, `OVERALL_OK=true`
- Live verification closure:
  - verified real task: `task_id=140`, `task_no=RW-20260320-A-000134`
  - DB evidence for the chosen task:
    - `task.asset.upload_session.completed` exists
    - `task.asset.version.created` exists
    - no `task.design.submitted` event exists
  - live `GET /v1/tasks/140` after publish returned:
    - `task_status = InProgress`
    - `design_assets_count = 4`
    - `asset_versions_count = 4`
    - every returned `design_asset.current_version.id` matched a returned `asset_versions[].id`
  - this closes the original read-model gap: uploaded versions are now visible from task detail without forcing frontend blind retries of `submit-design`.

## 2026-03-30 batch SKU create closure
- `POST /v1/tasks` now supports one mother task with multiple `task_sku_items` for:
  - `new_product_development`
  - `purchase_task`
- `original_product_development` remains existing-product only and rejects `batch_sku_mode=multiple` / `batch_items` with `400 INVALID_REQUEST` plus machine-readable `error.details.violations`.
- Read models now expose additive task batch fields:
  - `is_batch_task`
  - `batch_item_count`
  - `batch_mode`
  - `primary_sku_code`
  - `sku_generation_status`
  - `sku_items`
- Purchase-task create still initializes the task-level `procurement_records(draft)` row, and multi/single SKU create now also initializes `procurement_record_items(draft)` aligned to `task_sku_items`.
- `owner_team` remains on the legacy compatibility enum path and is still not tied to `/v1/org/options`.

## 2026-03-31 owner_team create compatibility closure
- Boundary remains unchanged:
  - task `owner_team` is still the task-side legacy compatibility field
  - account org `/v1/org/options` is still the account-side department/team source
  - this round did **not** unify those two models
- Create-time compatibility is now explicit in MAIN:
  - service create normalization first keeps legacy owner-team values unchanged
  - if the request carries a supported org-team value with a deterministic task mapping, create normalizes it into the legacy task `owner_team`
  - unsupported/unknown team strings still fail with `400 INVALID_REQUEST` and `violations[].code=invalid_owner_team`
- owner_team compatibility guardrail:
  - task-side `owner_team` is still a legacy compatibility field; this is **not** org-model unification
  - `/v1/org/options` teams must **not** be auto-accepted by task create
  - create-time compatibility now comes from an explicit fixed mapping list in `service/task_owner_team.go`, not from auto-deriving every org team under a department
  - supported org-team samples fixed for regression are:
    - `运营一组` -> `内贸运营组`
    - `运营三组` -> `内贸运营组`
    - `运营七组` -> `内贸运营组`
    - `定制美工组` -> `设计组`
    - `设计审核组` -> `设计组`
    - `采购组` -> `采购仓储组`
    - `仓储组` -> `采购仓储组`
    - `烘焙仓储组` -> `采购仓储组`
  - any newly introduced org team that should be accepted by task create must also add explicit task mapping and regression coverage
- Code path:
  - task create validation still runs in `service.validateCreateTaskEntry`
  - legacy allowed task owner teams still come from `domain.DefaultDepartmentTeams` / `domain.ValidTeam`
  - create-time compatibility bridge now runs in `service.normalizeOwnerTeamForTaskCreate`
  - read-only mapping introspection is available from `service.ListTaskOwnerTeamCompatMappings()`
- Minimal debug log remains fixed on the create path:
  - `trace_id`
  - `task_type`
  - `raw_owner_team`
  - `normalized_owner_team`
  - `owner_team_mapping_applied`
  - `mapping_source`
- Required local verification passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
- Overwrite publish stayed on the existing release line:
  - command: `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 owner_team create compatibility fix"`
  - target remained `/root/ecommerce_ai/releases/v0.8`
- Live runtime verification after overwrite:
  - `8080 /health` = `200`
  - `8081 /health` = `200`
  - `8082 /health` = `200`
  - active exe pointers:
    - `8080` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
    - `8081` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
    - `8082` -> `/root/ecommerce_ai/erp_bridge_sync`
  - no active `/proc/<pid>/exe` pointer was in `(deleted)` state
- Live task-create acceptance after overwrite:
  - `original_product_development` + defer-local-binding payload + `owner_team="运营三组"` -> `201`, task `150`, stored/read `owner_team="内贸运营组"`
  - `new_product_development` + `owner_team="运营三组"` -> `201`, task `151`, stored/read `owner_team="内贸运营组"`
  - `purchase_task` + `owner_team="运营三组"` -> `201`, task `152`, stored/read `owner_team="内贸运营组"`
  - illegal team (`不存在的组`) still returned `400 INVALID_REQUEST` with `violations[].field=owner_team` and `violations[].code=invalid_owner_team`
- Live log evidence also confirmed the bridge path:
  - successful compat requests logged `mapping_source=org_team_compat`
  - illegal team requests logged `mapping_source=invalid`

## 2026-03-31 owner_team compatibility guardrail hardening
- Guardrail scope:
  - this round did **not** unify the org model
  - task-side `owner_team` remains the legacy compatibility field
  - `/v1/org/options` must still not be treated as task `owner_team` truth
- Runtime hardening in MAIN:
  - create-time compat mapping is now an explicit fixed list in `service/task_owner_team.go`
  - newly added org teams are no longer auto-accepted just because they appear under an org department
  - read-only introspection is available through `service.ListTaskOwnerTeamCompatMappings()`
- Fixed compat samples now frozen by code + tests:
  - `运营一组` -> `内贸运营组`
  - `运营三组` -> `内贸运营组`
  - `运营七组` -> `内贸运营组`
  - `定制美工组` -> `设计组`
  - `设计审核组` -> `设计组`
  - `采购组` -> `采购仓储组`
  - `仓储组` -> `采购仓储组`
  - `烘焙仓储组` -> `采购仓储组`
- Guardrail coverage added:
  - service table-driven normalization + validation coverage for direct / compat / invalid inputs
  - create regression retained for original / new / purchase compat success plus invalid-team rejection
  - batch regression added for `new_product_development`, `purchase_task`, and original-batch reject with compat owner-team input
  - create-path log coverage now asserts `mapping_source=legacy_direct|org_team_compat|invalid`
- Local verification passed:
  - `go test ./service -run "OwnerTeam|OriginalProductWithOrgTeamCompatOwnerTeamPasses|NewProductWithOrgTeamCompatOwnerTeamPasses|PurchaseTaskWithOrgTeamCompatOwnerTeamPasses"`
  - `go test ./transport/handler -run "OwnerTeam"`
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
- Overwrite publish completed on the existing release line:
  - command: `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 owner_team compatibility guardrail hardening"`
  - target remained `/root/ecommerce_ai/releases/v0.8`
- Live verification after hardening:
  - `8080 /health` = `200`
  - `8081 /health` = `200`
  - `8082 /health` = `200`
  - `/proc/<pid>/exe` targets stayed normal:
    - `8080` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
    - `8081` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
    - `8082` -> `/root/ecommerce_ai/erp_bridge_sync`
  - live create with `owner_team="运营三组"` still succeeded: `new_product_development` -> `201`, task `156`, returned `owner_team="内贸运营组"`
  - live create with `owner_team="不存在的组"` still failed: `400 INVALID_REQUEST`, `violations[].field=owner_team`, `violations[].code=invalid_owner_team`
## 2026-04-01 task assign/reassign status-gating closure
- Root cause confirmed on live before code change:
  - task `170` was already `InProgress`
  - `designer_id=41`
  - `current_handler_id=41`
  - `owner_department=运营部`
  - `owner_org_team=运营一组`
  - old `/v1/tasks/{id}/assign` only implemented `PendingAssign -> InProgress`, so the request failed at status gating before org scope evaluation
- Final rule now fixed and explicit:
  - `PendingAssign`:
    - semantic action = `assign`
    - existing Ops/management path still applies
    - success sets `designer_id` + `current_handler_id`
    - status transitions to `InProgress`
  - `InProgress`:
    - semantic action = `reassign`
    - only management roles may act:
      - `Admin`
      - `SuperAdmin`
      - `RoleAdmin`
      - `HRAdmin`
      - `DepartmentAdmin`
      - `TeamLead`
      - `DesignDirector`
    - existing canonical org scope must still match
    - success updates `designer_id` + `current_handler_id`
    - status remains `InProgress`
  - audit / warehouse / close style states remain denied with `deny_code=task_not_reassignable`
- Runtime code changed in:
  - `service/task_action_rules.go`
  - `service/task_action_authorizer.go`
  - `service/task_assignment_service.go`
  - `domain/audit.go`
- Regression tests added/updated in:
  - `service/task_action_authorizer_test.go`
  - `service/task_step04_service_test.go`
  - `transport/handler/task_action_authorization_test.go`
- Local verification for this round:
  - `go test ./service ./transport/handler` -> passed
  - `go build ./cmd/server` -> passed
  - `go build ./repo/mysql ./service ./transport/handler` -> passed
  - `go test ./repo/mysql` -> blocked by local Application Control policy on `mysql.test.exe`
- Overwrite publish stayed on existing `v0.8`:
  - command: `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task assign reassign status gating fix"`
  - release history shows deploy completed at `2026-04-01T07:16:42Z`
- Live verification after overwrite:
  - `8080 /health` = `200`
  - `8081 /health` = `200`
  - `8082 /health` = `200`
  - `/proc/3519769/exe` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/proc/3519812/exe` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `/proc/3519962/exe` -> `/root/ecommerce_ai/erp_bridge_sync`
  - active executables were not deleted
- Live acceptance samples:
  - task `170` scope mismatch:
    - temporary `TeamLead` in `运营三组` -> `POST /v1/tasks/170/assign` returned `403`
    - deny details: `action=reassign`, `deny_code=task_out_of_team_scope`
  - task `170` reassign success:
    - temporary `TeamLead` in `运营一组` -> `POST /v1/tasks/170/assign` to designer `42` returned `200`
    - follow-up detail read showed `assignee_id=42`, `assignee_name=iter098_designer_b_74931810`, `current_handler_id=42`, `task_status=InProgress`
    - same actor then reassigned task `170` back to designer `41` so live sample ended restored
  - task `169` assign success:
    - before: `PendingAssign`, no assignee/current handler
    - admin assign to designer `42` returned `200`
    - follow-up detail read showed `task_status=InProgress`, `assignee_id=42`, `current_handler_id=42`
  - task `165` forbidden status:
    - current status `PendingAuditA`
    - admin `POST /v1/tasks/165/assign` returned `403`
    - deny details: `action=reassign`, `deny_code=task_not_reassignable`
- Live event/log evidence:
  - `task_event_logs`:
    - task `169` sequence `3` -> `task.assigned`, `action=assign`, `designer_id=42`
    - task `170` sequence `4` -> `task.reassigned`, `previous_designer_id=41`, `designer_id=42`
    - task `170` sequence `5` -> `task.reassigned`, `previous_designer_id=42`, `designer_id=41`
  - server logs now include:
    - `task_action_auth action=assign|reassign ...`
    - `task_assignment trace_id=... task_id=... action=assign|reassign ... previous_designer_id=... new_designer_id=... previous_status=... resulting_status=... allow=... deny_reason=...`
- Temporary live verification users:
  - created users `50` and `51`
  - both were disabled after acceptance
- Boundary kept explicit:
  - this round fixes assign/reassign status gating only
  - it does not introduce a standalone `/reassign` route
  - it does not open reassignment in audit / warehouse / close stages
  - it is not full task-action ABAC

## 2026-04-02 task product-code mainline closure (`rule_templates` audit + backend default allocator)
- Runtime audit executed in code directories (`service/repo/transport/cmd/domain/db/migrations`) with keywords:
  - `rule_templates`, `rule-templates`, `product-code`, `cost-pricing`, `short-name`
  - `code-rules`, `generate-sku`, `template_key`, `rule_template`, `code_rule`
- Audit conclusion:
  - `rule_templates` runtime module still exists and `/v1/rule-templates` routes are still exposed.
  - `rule_templates/product-code` is deprecated in runtime:
    - `List` filters it out.
    - `GET/PUT /v1/rule-templates/product-code` return `INVALID_REQUEST` deprecation errors.
  - `code-rules` runtime module remains active (`/v1/code-rules`, `/preview`, `/generate-sku`), but task create no longer depends on frontend rule selection.
- Backend default task product-code rule is now fixed:
  - `NS + category_code + 6-digit sequence`
  - example: `NSKT000000`
  - applied to `new_product_development` and `purchase_task`
  - `original_product_development` existing-product lane not switched to this generator
- New backend allocator and uniqueness guardrails:
  - migration `048_v7_product_code_sequences.sql`
  - new table `product_code_sequences` with unique key `(prefix, category_code)`
  - transactional range allocation with row lock (`SELECT ... FOR UPDATE`)
  - create-tx final duplicate guard via existing `task_sku_items.uq_task_sku_items_sku_code`
- New frontend-facing pre-generation capability:
  - `POST /v1/tasks/prepare-product-codes`
  - supports `count` and `batch_items[].category_code`
  - returns ordered `codes[].index/category_code/sku_code`
  - create remains final source of truth
- Frontend collaboration contract:
  - stop configuring `rule_templates/product-code`
  - stop configuring code-rules for task create SKU generation
  - rely on backend auto generation in `POST /v1/tasks` (recommended)
  - optional pre-display via `POST /v1/tasks/prepare-product-codes`
- Field-reading contract (task detail/list/create response model):
  - single: `data.sku_code`
  - batch: `data.sku_code` + `data.primary_sku_code` (compat) and `data.sku_items[].sku_code`
- Local verification in this round:
  - `go test ./service ./transport/handler` passed
  - `go build ./cmd/server` passed
  - `go build ./repo/mysql ./service ./transport/handler` passed
  - `go test ./repo/mysql` passed
  - concurrency test `TestTaskServicePrepareProductCodesBatchAndConcurrentUnique` passed
  - `-race` unavailable on this host (`CGO_ENABLED=1` required)
