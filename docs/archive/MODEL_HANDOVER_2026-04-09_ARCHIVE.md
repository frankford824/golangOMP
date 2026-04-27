# MODEL_HANDOVER

## 2026-04-09 v0.9 consolidation authority
- Before treating any older section here as the active contract, read `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`.
- This file remains the historical handover archive. Official v0.9 API status, canonical field naming, and compatibility governance now live in the v0.9 source-of-truth doc plus `transport/http.go`.

## 2026-04-08 iteration 117 handover: user-management backend closure on existing `v0.8`
- Treat this as the latest handover truth source for MAIN user-management backend capability.

### What changed
- Added formal admin-managed user creation:
  - `POST /v1/users`
  - validates `department/team` against `/v1/org/options`
  - validates `roles` against the backend role catalog
  - persists initial password hash and returns created user with final `frontend_access`
- Added formal admin-managed password reset:
  - `PUT /v1/users/{id}/password`
- Extended formal user list filtering:
  - `GET /v1/users`
  - now supports `department` / `team` / `role` in addition to existing `keyword` / `status` / pagination
- Kept disable semantics on the existing formal update path:
  - `PATCH /v1/users/{id}` with `status=disabled`

### Current supported backend contract
- User list:
  - `GET /v1/users`
  - filters: `keyword`, `status`, `role`, `department`, `team`, `page`, `page_size`
  - returns: `id`, `username`, `display_name`, `department`, `team`, `roles`, `status`, `frontend_access`, timestamps
- User detail:
  - `GET /v1/users/{id}`
  - returns base profile, org binding, current roles, status, and computed `frontend_access`
- Role management:
  - `GET /v1/roles`
  - `PUT /v1/users/{id}/roles` = full-set replacement
  - `POST /v1/users/{id}/roles` = additive grant
  - `DELETE /v1/users/{id}/roles/{role}` = single-role removal
- Password:
  - self-change: `PUT /v1/auth/password`
  - admin reset: `PUT /v1/users/{id}/password`
- Disable:
  - `PATCH /v1/users/{id}` with `status`
- Physical delete:
  - still not implemented
  - frontend should treat “删除用户” as disable unless a future formal delete semantic is added

### Truth-source boundaries
- `/v1/org/options` remains the only org source used for user-management create/update validation.
- The backend still computes `frontend_access` from persisted roles plus canonical user `department/team`.
- This round did not change:
  - `/v1/auth/me`
  - task create `owner_team` compatibility bridge
  - canonical task ownership
  - task action authorization routes
- Result:
  - user role changes continue to align frontend menus/pages/actions with backend route-role truth
  - no new parallel org/role truth source was introduced

### Verification summary
- Local:
  - `go test ./service ./transport/handler` passed
  - `go build ./cmd/server` passed
  - `go build ./repo/mysql ./service ./transport/handler` passed
  - `go test ./repo/mysql` passed
- New regression tests cover:
  - managed create
  - managed reset password
  - filtered user list
  - disabled-user login denial
  - handler request binding for list/create/reset
- Live on `v0.8`:
  - overwrite deploy completed successfully
  - `8080` / `8081` / `8082` health checks passed
  - `/proc/<pid>/exe` paths remained healthy
  - admin login, `/v1/auth/me`, `/v1/org/options`, user create, filtered list, role replacement, password reset, and disable-login denial all succeeded on live runtime

### Remaining boundary / risk
- No physical delete endpoint exists yet.
- Admin password reset does not revoke already-issued sessions.
- Task create/org bridge regression in this round was covered by local regression/builds only; no new live task-create write probe was executed during this user-management iteration.

## 2026-04-08 iteration 116 handover: probe-driven gate now requires NAS-signed attestation
- Treat this as the latest handover truth source for large-file browser-direct admission under `https://yongbo.cloud`.

### What changed
- Main strategy is no longer office public IP allowlist.
- Large-file multipart upload and private-network download are now unlocked only by:
  - browser probe to NAS `GET /upload/ping`
  - frontend pass-through of probe payload
  - backend verification of NAS-signed `attestation`
- Legacy CIDR/public-IP lists are still recorded for diagnostics, but are no longer the admission truth source.

### Runtime contract
- NAS upload service:
  - `/upload/ping` is lightweight, browser-accessible, and does not create upload resources
  - success returns signed `attestation`
  - missing probe secret returns probe failure instead of false success
- MAIN:
  - multipart create requires `network_probe.attestation` on success probes
  - private-network download requires `X-Network-Probe-Attestation` alongside the existing `X-Network-Probe-*` headers
  - forged success probes without attestation are explicitly denied with `UPLOAD_ENV_NOT_ALLOWED`

### Live verification summary
- NAS `/upload/ping` live response:
  - `200`
  - `url=http://192.168.0.125:8089/upload/ping`
  - signed `attestation`
- Multipart create:
  - no probe -> `403 probe_missing`
  - fake success without attestation -> `403 probe_attestation_missing`
  - valid attested probe -> `201` with NAS private multipart plan
- Private download:
  - no probe -> `403 probe_missing`
  - fake success without attestation -> `403 probe_attestation_missing`
  - valid attested probe -> `200 private_network`
- Small external-safe lane unchanged:
  - `POST /v1/tasks/reference-upload` still returned `201`

### Organization consistency
- `/v1/org/options` still exposes `运营三组`.
- Live create with a valid `new_product_development` payload plus `owner_team="运营三组"` returned `201` and normalized to:
  - `owner_team="内贸运营组"`
  - `owner_department="运营部"`
  - `owner_org_team="运营三组"`
- The earlier local `invalid_owner_team` probe was an encoding artifact from the local control node, not a live backend regression.

## 2026-04-08 iteration 115 handover: yongbo.cloud single-domain upload gate re-verification + fixed-strategy freeze
- Treat this section as the latest handover truth source for the upload/download environment gate under the single public entry `https://yongbo.cloud`.

### Current request-source resolution
- MAIN currently resolves source IP in this order:
  - `X-Real-IP`
  - `X-Forwarded-For`
    - current code uses the last valid token
  - `RemoteAddr`
- Current live Nginx forwards `/v1` with:
  - `Host`
  - `X-Real-IP $remote_addr`
  - `X-Forwarded-For $proxy_add_x_forwarded_for`
  - `X-Forwarded-Proto $scheme`
- Result:
  - live `yongbo.cloud` classification is effectively based on `X-Real-IP`
  - current office browser traffic is seen by MAIN as public egress `222.95.254.125`

### Historical root cause
- Office users were previously misclassified because they did not arrive as RFC1918 private IPs.
- Before live allowlist was populated, office traffic from `222.95.254.125` failed the gate and was denied as external.
- Nginx forwarding itself was already correct; the missing piece was runtime allowlist population.

### Final fixed rule
- Allow multipart/private-network large-file lanes when source IP matches either:
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
- Scope of the rule:
  - multipart upload session creation
  - private-network source/download responses
- Explicit exemption:
  - `POST /v1/tasks/reference-upload` remains available to external users

### Live samples captured in this iteration
- Office/intranet-allowed source:
  - `source_ip=222.95.254.125`
  - log reason now: `source_x_real_ip_matched_allowed_public`
  - office replay on live MAIN for task `375` returned multipart `201` with NAS private plan:
    - `remote.base_url=http://192.168.0.125:8089`
- External real source:
  - verification host `openclaw`, public IP `8.222.174.253`
  - real call to `https://yongbo.cloud/v1/tasks/375/asset-center/upload-sessions/multipart` returned:
    - `403`
    - `UPLOAD_ENV_NOT_ALLOWED`
    - `details.source_ip=8.222.174.253`
  - same external host still succeeded on:
    - `POST /v1/tasks/reference-upload` -> `201`
- Download alignment:
  - task `375` asset `58`
  - office source `222.95.254.125` -> `200 private_network`
  - external source `8.8.8.8` -> `403 UPLOAD_ENV_NOT_ALLOWED`

### Runtime truth
- Live env key:
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_IPS=222.95.254.125`
- No new runtime code or deploy was required in this iteration.
- Existing live runtime remained:
  - main `8080` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - bridge `8081` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - sync `8082` -> `/root/ecommerce_ai/erp_bridge_sync`

### Anti-regression note
- Do not split the site into separate public/intranet browser entries.
- Future office出口 IP/CIDR changes must be reflected in `shared/main.env`.
- If any upstream CDN/WAF/proxy is introduced later, re-evaluate trusted proxy handling before relying on current `X-Real-IP` precedence.

## 2026-04-08 iteration 114 handover: batch SKU-scoped reference/design contract + office-egress upload allowlist + runtime org bridge convergence
- Treat this section as latest live handover truth source.

### Problem 1 final backend contract
- Batch references:
  - top-level `reference_file_refs` remains the mother-task union summary for compatibility.
  - formal per-SKU refs now live on:
    - `task_sku_items.reference_file_refs_json`
    - `GET /v1/tasks/{id}` -> `sku_items[].reference_file_refs`
- Batch design assets:
  - upload session create accepts `target_sku_code`.
  - validated target SKU must belong to the task.
  - persisted scope fields:
    - `upload_requests.target_sku_code`
    - `design_assets.scope_sku_code`
    - `task_assets.scope_sku_code`
  - read-model fields:
    - `design_assets[].scope_sku_code`
    - `asset_versions[].scope_sku_code`
- Frontend consequence:
  - batch SKU tab switch must be keyed by `sku_items[].sku_code`.
  - references come from the active SKU item's `reference_file_refs`.
  - designs come from `design_assets/asset_versions` filtered by `scope_sku_code`.
  - top-level `reference_file_refs` is no longer a valid per-SKU substitute.

### Problem 2 final upload/download gate
- Direct browser multipart/private-network download is now allowed when source IP matches either:
  - configured private CIDRs
  - configured office public IP allowlist
  - configured office public CIDR allowlist
- Runtime envs:
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_CIDRS`
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_IPS`
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_CIDRS`
  - legacy aliases still read:
    - `UPLOAD_ALLOWED_PUBLIC_IPS`
    - `UPLOAD_ALLOWED_PUBLIC_CIDRS`
- Deny contract:
  - `HTTP 403`
  - `error.code=UPLOAD_ENV_NOT_ALLOWED`
  - details include:
    - `source_ip`
    - `policy=private_network_or_configured_public_ip`
    - `allowed_cidrs`
    - `allowed_public_ips`
    - `allowed_public_cidrs`
    - `reason`

### Problem 3 final org/permission truth-source convergence
- `/v1/org/options` remains the account-org source from auth settings.
- Task create compatibility bridge is no longer separately hardcoded:
  - `service.ConfigureTaskOrgCatalog(cfg.Auth)` derives:
    - org-team -> legacy `owner_team`
    - org-team -> canonical department
    - department -> supported legacy owner teams
- Result:
  - deterministic configured org-team values can now pass create validation and persist canonical ownership together.
  - `invalid_owner_team` should now mean truly unmapped/unsupported input, not stale hardcoded bridge drift.
- `frontend_access` vs route-role alignment remains the latest rule from iteration 113:
  - menus/pages/actions are role-driven
  - department-only membership is intentionally minimal
  - formal business accounts still require workflow roles

### Files changed in this iteration
- Runtime / contract:
  - `service/task_org_catalog.go`
  - `service/task_owner_team.go`
  - `service/task_org_ownership.go`
  - `service/task_asset_center_service.go`
  - `service/task_asset_center_read_model.go`
  - `service/task_design_asset_read_model.go`
  - `service/task_batch_create.go`
  - `service/task_sku_read_model.go`
  - `transport/handler/upload_network_access.go`
  - `transport/handler/task_asset_center.go`
  - `config/config.go`
  - `cmd/server/main.go`
  - `cmd/api/main.go`
- Repo / schema:
  - `repo/mysql/task.go`
  - `repo/mysql/task_asset.go`
  - `repo/mysql/design_asset.go`
  - `repo/mysql/upload_request.go`
  - `db/migrations/049_v7_batch_sku_asset_scope.sql`
- Docs/config:
  - `docs/api/openapi.yaml`
  - `deploy/main.env.example`

### Local verification
- Passed:
  - `go test ./service ./transport/handler` (with repo-local `GOTMPDIR` / `GOCACHE`)
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`
- Targeted tests:
  - `TestTaskServiceCreateBatchMergesItemLevelReferenceFileRefsWithValidation`
  - `TestTaskReadModelBatchIncludesReferenceFileRefsAndFallbackAssetVersions`
  - `TestTaskAssetCenterServiceCreateAndCompleteMultipartUploadSessionWithTargetSKUCode`
  - `TestConfigureTaskOrgCatalogAlignsConfiguredOrgTeamsWithTaskCreate`
  - `TestTaskServiceCreateBatchOwnerTeamCompatRegression`
  - `TestTaskAssetCenterHandlerCreateMultipartUploadSessionDeniedForExternalSource`
  - `TestTaskAssetCenterHandlerCreateMultipartUploadSessionAllowedForConfiguredOfficePublicIP`
  - `TestTaskAssetCenterHandlerDownloadPrivateNetworkDeniedForExternalSource`
  - `TestTaskAssetCenterHandlerDownloadPrivateNetworkAllowedForConfiguredOfficePublicIP`

### Remaining boundary
- Migration `049` does not attempt historical per-SKU backfill from mother-task union refs because provenance is ambiguous.

### Deploy / live acceptance truth
- Backup before migration:
  - `/root/ecommerce_ai/backups/iter114_20260408T051352Z`
- Applied migration:
  - `049_v7_batch_sku_asset_scope.sql`
- Updated live runtime env:
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_IPS=222.95.254.125`
- Overwrite deployed existing `v0.8`:
  - artifact sha:
    - `991268c62615a2efa9cea37fc8915e3063af224056cf2f2641e81445f0933b11`
  - entrypoint unchanged:
    - `./cmd/server`
- Runtime verification:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/<pid>/exe`:
    - main -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
    - bridge -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
    - sync -> `/root/ecommerce_ai/erp_bridge_sync`
- Live acceptance:
  - task `374` proved per-SKU refs:
    - `NSKT000071` -> ref `3b7657a8-319d-49ac-aada-f0876b7bf13a`
    - `NSKT000072` -> ref `d53e2f96-93b5-4183-9df0-1ae24250be7e`
  - task `373` proved SKU-scoped design asset persistence:
    - `design_assets[].scope_sku_code = ["NSKT000070"]`
    - `asset_versions[].scope_sku_code = ["NSKT000070"]`
  - task `375` source download gate proved:
    - `X-Real-IP: 8.8.8.8` -> `403 UPLOAD_ENV_NOT_ALLOWED`
    - `X-Real-IP: 222.95.254.125` -> `200 private_network`
  - tasks `372` / `373` proved org-team create bridge:
    - input `owner_team="运营三组"`
    - persisted `owner_team="内贸运营组"`
    - canonical ownership `owner_department="运营部"`, `owner_org_team="运营三组"`

## 2026-04-07 iteration 113 handover: private-network direct upload/download policy enforcement + frontend_access role alignment on existing `v0.8`
- Treat this section as latest handover truth source.

### Final policy boundary
- Keep existing NAS direct strategy:
  - intranet/VPN can use browser direct NAS upload/download.
  - external must not get private NAS browser URLs for large-file direct lanes.
- This iteration does **not** enable public same-origin `/upload` upload path.

### Upload behavior after fix
- Multipart session issue path (`POST /v1/tasks/{id}/asset-center/upload-sessions/multipart`) now enforces source-network gate.
- External disallowed source returns:
  - `HTTP 403`
  - `error.code=UPLOAD_ENV_NOT_ALLOWED`
  - details include `source_ip`, `allowed_cidrs`, `policy=private_network_only`, `reason`.
- Intranet-like source still receives private browser targets:
  - `remote.base_url=http://192.168.0.125:8089`
  - `part_upload_url_template`, `complete_url`, `abort_url` under same base.

### Download behavior after fix
- Download endpoints now apply the same gate when `download_mode=private_network`.
- External disallowed source is explicitly rejected (`UPLOAD_ENV_NOT_ALLOWED`) instead of returning ambiguous private-network-only hints.
- Direct/public-compatible download mode is unchanged.

### Runtime config truth
- Added runtime config support:
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_POLICY_ENABLED` (default `true`)
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_CIDRS` (default private CIDRs)
- Browser multipart base remains private-network:
  - `UPLOAD_SERVICE_BROWSER_MULTIPART_BASE_URL=http://192.168.0.125:8089`
- Server-to-server upload base remains:
  - `UPLOAD_SERVICE_BASE_URL=http://100.111.214.38:8089`

### Security note
- Current direct-browser multipart contract still carries `X-Internal-Token` in response headers for NAS direct path.
- This iteration keeps compatibility; long-term hardening should migrate to proxy-injected or short-lived session token model.

### frontend_access vs route-role closure
- Department frontend access now contributes scopes only.
- Business menus/pages/actions remain role-driven, matching route auth requirements.
- This closes department-only menu leakage that previously produced menu-visible-but-route-403 mismatch.

### Live verification highlights
- External (`https://yongbo.cloud`) multipart session call returns explicit `403 UPLOAD_ENV_NOT_ALLOWED`.
- Intranet-like source (`jst_ecs` localhost -> `127.0.0.1:8080`) still gets private NAS multipart URLs.
- Direct part upload from `jst_ecs` to `192.168.0.125:8089` timed out, confirming `jst_ecs` is not on NAS LAN route; this is environmental reachability, not API gate regression.
- Role/menu probes:
  - unassigned `Member` -> minimal menu, `/v1/tasks=403`
  - formal `Member+Ops` -> task menu visible, `/v1/tasks=200`
  - formal `Member+Designer` -> design/task menus visible, `/v1/tasks=200`
  - ops-department `Member` only -> minimal menu, `/v1/tasks=403`

### Account opening minimum templates (current)
- Ops:
  - `department=运营部`, concrete team (example `运营三组`)
  - roles `[Member, Ops]`
- Designer:
  - roles `[Member, Designer]`
- Audit:
  - roles `[Member, Audit_A]` or `[Member, Audit_B]`
- Warehouse:
  - roles `[Member, Warehouse]`
- Management:
  - `DepartmentAdmin + managed_departments`
  - or `TeamLead + managed_teams`

### Deploy/runtime truth
- Overwrite publish to existing `v0.8` only.
- Entrypoint unchanged:
  - `./cmd/server`
- Post-deploy runtime checks remained healthy:
  - `8080/8081/8082 /health = 200`
  - active `/proc/<pid>/exe` links valid and not deleted.

## 2026-04-02 iteration 112 handover: two-letter short-code product-code live closure on existing `v0.8`
- Treat this section as latest handover truth source.

### Scope
- Move default task product-code middle segment from raw `category_code` to backend-owned two-letter uppercase short code.
- Keep single/batch/concurrency uniqueness stable after the format switch.
- Keep deprecated `rule_templates/product-code` out of create-task mainline.

### Why old format was wrong
- Previous runtime formatter used:
  - `NS + category_code + 6-digit sequence`
- So `KT_STANDARD` became:
  - `NSKT_STANDARD000060`

### Final runtime rule
- Format:
  - `NS + category_short_code(2 uppercase letters) + 6-digit sequence`
  - regex: `^NS[A-Z]{2}[0-9]{6}$`
- Short-code priority:
  - explicit mapping first (`KT_STANDARD -> KT`)
  - fallback extraction of first two alphabet letters from `category_code` (uppercased)
  - deterministic fallback letters when fewer than two alphabet letters exist
- Effective task-type scope:
  - enabled: `new_product_development`, `purchase_task`
  - disabled: `original_product_development`

### Uniqueness design (critical)
- Sequence allocation now uses short-code lane:
  - `(prefix, category_short_code)`
- This prevents collisions when different `category_code` values collapse to one short code.
- First-lane bootstrap:
  - when `product_code_sequences.next_value=0`, allocator scans existing `task_sku_items` for `NS+short_code+6-digit` max suffix and starts from `max+1`.

### Deploy truth
- Existing script chain only:
  - `deploy/deploy.sh --version v0.8 --skip-tests --skip-runtime-verify --release-note \"overwrite v0.8 category short code NS??xxxxxx rule\"`
- Existing line overwrite only:
  - `v0.8` (no new release line)
- Entrypoint unchanged:
  - `./cmd/server`
- Release artifact (`deploy/release-history.log`):
  - `9dd601696fcad8c719a437526d99c4d6b5cf9b2dc5ece94a5b14c41321935b60`
- Migration status:
  - iteration 112 did not apply new migrations;
  - iteration 111 had already applied `048_v7_product_code_sequences.sql`.

### Runtime verification truth
- Health:
  - `8080 /health = 200` (external probe)
  - `8081 /health = 200` (remote localhost probe)
  - `8082 /health = 200` (remote localhost probe)
- Process executable links:
  - main pid `3838150` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - bridge pid `3838173` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - sync pid `3838987` -> `/root/ecommerce_ai/erp_bridge_sync`
- Active executable targets are not `(deleted)`.

### Live acceptance truth
- Deprecated template route:
  - `GET /v1/rule-templates/product-code` -> `400 INVALID_REQUEST`
  - message: `rule_templates/product-code is deprecated; use default backend task product-code generation`
  - `GET /v1/rule-templates` -> `200` and list currently has no `product-code`.
- Prepare endpoint:
  - request: `POST /v1/tasks/prepare-product-codes` with `{task_type:new_product_development, category_code:KT_STANDARD, count:3}`
  - response codes: `NSKT000017`, `NSKT000018`, `NSKT000019`
  - regex pass + no in-batch duplicate.
- New single:
  - `task_id=361`, `sku_code=NSKT000028`
- Purchase single:
  - `task_id=362`, `sku_code=NSKT000029`
- Batch:
  - `task_id=365`
  - `primary_sku_code=NSKT000032`
  - `sku_items[].sku_code = [NSKT000032, NSKT000033]`
  - regex pass + no duplicate.
- Same-short-code lane:
  - `KT_STANDARD` create -> `task_id=363`, `sku_code=NSKT000030`
  - `K-T-standard` create -> `task_id=364`, `sku_code=NSKT000031`
  - no duplicate.
- Lightweight concurrency:
  - 8 parallel prepare calls returned `NSKT000020 ... NSKT000027`
  - duplicate `no`, error count `0`.
- Other mainline checks:
  - `GET /v1/tasks?page=1&page_size=5` -> `200`
  - `POST /v1/tasks/361/assign` -> `200` (`InProgress`)
  - task read/detail still carries ownership + `design_assets` + `asset_versions`.

### Honest correction log
- First probes in this run hit validation failures:
  - missing `owner_team`
  - invalid owner-department payload
  - missing required batch item fields
- After adjusting requests to current create contract, all required acceptance lanes passed.

### Frontend final wording
- Do not configure `rule_templates/product-code`.
- Do not compute short code in frontend.
- Use `POST /v1/tasks` for new/purchase creation and read returned codes.
- Optional pre-display:
  - `POST /v1/tasks/prepare-product-codes`
- Read fields:
  - single/purchase: `data.sku_code`
  - batch: `data.sku_items[].sku_code`
  - compatibility: `data.primary_sku_code`, `data.sku_code`.

### Remaining boundaries
- This iteration closes live short-code rollout + acceptance, not full/global numbering-platform finalization.
- `product_code_sequences` table keeps column name `category_code`; runtime now stores/uses short code in that slot.

## 2026-04-02 iteration 111 handover: ITERATION_110 live promotion on existing `v0.8` + coding-rule acceptance closure (archived previous truth source)
- Treat this section as previous handover truth source.

### Scope
- Promote already-finished ITERATION_110 runtime changes to live existing `v0.8`.
- Validate live behavior for:
  - deprecated `rule_templates/product-code`
  - default SKU generation for new/purchase task create
  - batch item auto-generation
  - prepare endpoint
  - lightweight concurrency uniqueness.

### Deploy and migration truth
- Deploy path: existing `deploy/deploy.sh`, fixed `--version v0.8`, entrypoint unchanged `./cmd/server`.
- First deploy attempt:
  - reached deploy stage but runtime-verify failed from CRLF script (`pipefail^M`).
- Re-run deploy (`--skip-tests --skip-runtime-verify`) succeeded.
- One extra overwrite deploy was executed to restore scripts after a failed remote CRLF conversion attempt.
- Final deployed artifact (`release-history.log`): `8fea0be9a4fcfa5a3324c47ca885146033675c58ed0348f09650d605c5a02bd8`.

- Live DB migration status:
  - `048_v7_product_code_sequences.sql` was not yet applied on live.
  - backup-first then apply:
    - backup dir: `/root/ecommerce_ai/backups/iter110_048_20260402T082531Z`
    - applied migration path: `/root/ecommerce_ai/releases/v0.8/db/migrations/048_v7_product_code_sequences.sql`
  - post-check: `product_code_sequences` exists with unique key `(prefix, category_code)`.

### Runtime verification truth
- Health: `8080=200`, `8081=200`, `8082=200`.
- Active executables:
  - main -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - bridge -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - sync -> `/root/ecommerce_ai/erp_bridge_sync`
- No active executable points to `(deleted)`.

### Live acceptance truth
- Deprecated route behavior:
  - `GET /v1/rule-templates/product-code` -> `400`
  - message: `rule_templates/product-code is deprecated; use default backend task product-code generation`
  - `GET /v1/rule-templates` no longer lists `product-code`.
- New single auto-code:
  - `task_id=352`, `sku_code=NSKT_STANDARD000022`.
- Purchase single auto-code:
  - `task_id=353`, `sku_code=NSKT_STANDARD000023`.
- Batch auto-code:
  - `task_id=354`
  - `primary_sku_code=NSKT_STANDARD000024`
  - `sku_items[].sku_code = [NSKT_STANDARD000024, NSKT_STANDARD000025]`
  - no duplicate.
- Prepare endpoint:
  - `POST /v1/tasks/prepare-product-codes`
  - sample return: `NSKT_STANDARD000026, 000027, 000028`.
- Lightweight concurrency:
  - 8 parallel prepare calls returned `NSKT_STANDARD000029~000036`
  - duplicate `no`, error count `0`.
- Other mainline checks:
  - `GET /v1/tasks?page=1&page_size=5` -> `200`
  - `POST /v1/tasks/352/assign` -> `200` (`InProgress`)
  - canonical ownership fields still present in task read
  - detail read still includes `design_assets` + `asset_versions`.

### Contract/handover notes
- Default task product-code rule (live-verified):
  - `NS + category_code + 6-digit sequence`.
- Scope:
  - enabled: `new_product_development`, `purchase_task`
  - not enabled: `original_product_development`.
- Frontend must not select/configure `rule_templates/product-code` nor client-build task SKU code.
- Optional pre-display path remains:
  - `POST /v1/tasks/prepare-product-codes`.
- Current live detail reference lane observed:
  - `task_detail.reference_file_refs_json` present,
  - top-level `reference_file_refs` not observed in this run.

### Remaining boundaries
- This closure is live promotion + acceptance of default task coding behavior, not a full numbering-platform finalization.
- CRLF risk on remote script files remains an ops/deploy hygiene concern.

## 2026-04-02 iteration 109 handover: overwrite publish to existing `v0.8` + full live acceptance
- Treat this as latest handover truth source.

### Scope
- No new contract expansion in this iteration.
- Promote already-completed batch reference fix (iteration 108) to live `v0.8`.
- Complete real live acceptance and document result.

### Deploy truth
- Script path: existing `deploy/deploy.sh` only.
- Version line: overwrite existing `v0.8` (no new release line).
- Entry point: unchanged `./cmd/server`.
- First run:
  - deploy stage completed, but runtime verify failed from remote CRLF script issue (`pipefail^M`).
- Second run:
  - re-ran `deploy/deploy.sh --version v0.8 --skip-tests --skip-runtime-verify ...`
  - final deploy succeeded.
- Release evidence:
  - `deploy/release-history.log` final deployed artifact sha:
    `b7e43bde56f23a6a17b7a6b9796e8e0a7006d5c720c7e44b475cc9aaa1b4c017`.

### Runtime verification truth
- Health:
  - `8080=200`, `8081=200`, `8082=200`.
- Process executable links:
  - main -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - bridge -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - sync -> `/root/ecommerce_ai/erp_bridge_sync`
- No `(deleted)` executable in active pids.

### Live acceptance truth
- Single with reference:
  - `task_id=343`, detail `reference_file_refs` non-empty.
- Batch key lane (refs only in `batch_items[].reference_file_refs`):
  - `task_id=344`, create `201`;
  - detail top-level `reference_file_refs` includes expected ids:
    - `18c4f798-ba8d-4526-aff2-b288154f08ee`
    - `b8a85ff5-5da3-4a8a-a09f-1c273b64afe6`
- Empty reference lane:
  - `task_id=345`, detail includes `reference_file_refs: []`.
- Design regression lane:
  - `task_id=339`, `design_assets=1`, `asset_versions=1`.
- Mainline spot checks:
  - batch SKU create passed (`task_id=344`)
  - assign action passed on `task_id=345` (`200`, now `InProgress`, `designer_id=5`)
  - list route `GET /v1/tasks?page=1&page_size=5` returned `200`.

### Contract boundaries (must keep)
- Formal detail read contract remains:
  - `reference_file_refs`
  - `design_assets`
  - `asset_versions`
- Do not promote legacy `reference_images`/legacy `/v1/assets` back to primary frontend detail source.
- `sku_items[].reference_file_refs` is still **not** formal output contract.

## 2026-04-02 iteration 108 handover: batch line `reference_file_refs` merge + stable task read-model JSON
- Treat this section as the latest handover truth source.

### Root cause
- Batch UIs often send `reference_file_refs` under **`batch_items[]` only**. The handler previously ignored those fields, so `task_details.reference_file_refs_json` stayed empty while uploads were valid (single-task flows that send top-level refs looked fine).
- `TaskReadModel.reference_file_refs` used `json:",omitempty"`, so **empty lists disappeared** from JSON and some clients treated that as “no references”.

### Runtime change
- `service/task_batch_create.go` — `mergeBatchItemReferenceFileRefsIntoTask` merges item-level refs with top-level refs (dedupe by `asset_id`) before `validateReferenceFileRefs` and mother-task insert.
- `service/task_service.go` — invoke merge right after `normalizeCreateTaskRequest`; `enrichTaskReadModelDetail` always assigns a non-nil slice.
- `domain/query_views.go` — `TaskReadModel.reference_file_refs` no longer `omitempty` (always encodes as JSON array).
- `transport/handler/task.go` — map `batch_items[].reference_file_refs`.

### Tests added
- `service/task_batch_reference_file_refs_test.go` — merge unit test, validated batch create with per-item refs, JSON key presence for empty refs.

### Local checks
- `go build ./...` succeeded.
- Full `go test` not confirmed on the locked-down Windows host (Application Control blocked `service.test.exe`).

### Deploy/live
- Not run in iteration 108 session; promote with existing `deploy/deploy.sh --version v0.8` when ready.

## 2026-04-02 iteration 107 handover: batch task detail image return alignment
- Truth source for **`design_assets` / `asset_versions`** when `design_assets` roots are empty (task-level `task_assets` fallback).

### Root cause
- Batch task references are mother-task-level (`task_details.reference_file_refs_json`), not SKU-item-level.
- `GET /v1/tasks/{id}` and `GET /v1/tasks/{id}/detail` both depend on `service/loadTaskDesignAssetReadModel` for `design_assets` + `asset_versions`.
- Previous behavior exited early when `design_assets` roots were empty, even if task-level `task_assets` versions existed.
- This produced single/batch divergence in affected data lanes (single commonly had complete roots; some batch tasks did not).

### Runtime change
- `service/task_design_asset_read_model.go`
  - Added fallback read-model aggregation when roots are empty:
    - build response-level asset groups from task-level versions (`task_assets`) by `asset_id`
    - reuse the same derived-field and role hydration logic
  - Formal contract remains unchanged:
    - references: `reference_file_refs`
    - design roots: `design_assets`
    - versions: `asset_versions`
  - No rollback to legacy `reference_images` or old `/v1/assets` as primary truth source.

### Tests added
- `service/task_design_asset_read_model_test.go`
  - `TestLoadTaskDesignAssetReadModelFallsBackWhenRootsMissing`
- `service/task_read_model_asset_versions_test.go`
  - `TestTaskReadModelBatchIncludesReferenceFileRefsAndFallbackAssetVersions`
  - includes formal-over-legacy reference assertion
- `service/task_detail_asset_versions_test.go`
  - `TestTaskDetailAggregateBatchIncludesFallbackAssetVersions`

### Local checks (all passed)
- `go test ./service ./transport/handler`
- `go build ./cmd/server`
- `go build ./repo/mysql ./service ./transport/handler`
- `go test ./repo/mysql`

### Deploy/live
- Overwrite deploy executed on existing `v0.8` with entrypoint `./cmd/server`:
  - `bash ./deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 batch task detail image fallback projection fix"`
- `deploy/release-history.log` evidence:
  - deployed at `2026-04-02T04:44:38Z`
  - artifact sha256: `791de8fac0082de48ebdbb9e511586574f9e7c3feabdd0f288011c1031bbcfce`
- Runtime checks:
  - `http://127.0.0.1:8080/health` => `{"status":"ok"}`
  - `/proc/3774193/exe` => `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
- Live single/batch detail verification (real upload-complete):
  - single `task_id=338`: `reference_file_refs=1`, `design_assets=1`, `asset_versions=1`, preview URL reachable
  - batch `task_id=339`: `reference_file_refs=1`, `design_assets=1`, `asset_versions=1`, preview URL reachable

## 2026-04-02 iteration 106 handover: destructive reset + user-org patch closure + primary-SKU nested-field confirmation on v0.8
- Treat this section as the latest handover truth source.

### What changed in runtime
- `service/identity_service.go`
  - `UpdateUserParams` now supports `Group` alias input.
  - team/group normalization now enforces:
    - if both provided they must match,
    - `team/group = "ungrouped"` maps to configured unassigned pool (`department=未分配`, `team=未分配池`),
    - when patching to unassigned department without explicit team/group, backend auto-fills configured unassigned pool team.
- `transport/handler/user_admin.go`
  - `PATCH /v1/users/{id}` request accepts `group` and forwards it.
- Tests:
  - `service/identity_service_test.go`
    - `TestIdentityServiceUpdateUserSupportsGroupAliasAndUngrouped`
    - `TestIdentityServiceUpdateUserRejectsTeamGroupConflict`

### What changed in scripts/contracts
- Reset scripts hardened:
  - `scripts/test_env_destructive_reset_keep_admin.sql`
  - `scripts/test_env_destructive_reset_keep_admin.sh`
  - backup naming now `..._pre_reset_keep_admin`
  - optional-table guards for `asset_versions`
  - restart step aligned with current deploy script arguments.
- OpenAPI aligned with runtime:
  - `docs/api/openapi.yaml`
    - `/v1/org/options` path added,
    - `OrgOptions` + `ConfiguredUserAssignment` schemas added,
    - `PATCH /v1/users/{id}` request fields aligned to real partial-update semantics,
    - `product_selection.erp_product` source-field clarification (`source_match_type` is the source indicator).

### Destructive reset truth
- Final successful reset run:
  - `scripts/test_env_destructive_reset_keep_admin.sh`
  - UTC: `20260402T041507Z`
- Backup paths:
  - server: `/root/ecommerce_ai/backups/20260402T041507Z_pre_reset_keep_admin`
  - NAS: `/volume1/homes/yongbo/asset-upload-service/backups/20260402T041507Z_pre_reset_keep_admin`
- Key reset result:
  - keep-admin count `4`
  - task/asset/procurement/upload/log/integration test data cleared to `0`
  - users kept `4`, sessions reset to `0`
  - post-reset `/v1/auth/me`, `/v1/org/options`, `/v1/roles` all `200`
  - `/v1/tasks` empty.

### Primary-SKU nested object truth (confirmed)
- Real read path:
  - list: `item.product_selection.erp_product`
  - detail: `data.product_selection.erp_product`
- Real nested fields include:
  - `product_id`, `sku_id`, `sku_code`, `product_name`/`name`.
- Source/provenance indicator is sibling field:
  - `product_selection.source_match_type` (not `erp_product.source`).
- Live sample (task `328`) confirmed `source_match_type=erp_bridge_keyword_search`.

### Local required checks (all passed)
- `go test ./service ./transport/handler`
- `go build ./cmd/server`
- `go build ./repo/mysql ./service ./transport/handler`
- `go test ./repo/mysql`

### Publish truth (overwrite existing v0.8)
- Command:
  - `bash ./deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 reset+org-user patch+contract alignment"`
- `deploy/release-history.log`:
  - deployed at `2026-04-02T04:08:42Z`
  - sha256: `c4c16fe3e656c3fa92ea51ff65369d166ae90496aafb726c2e49d59bb05a81c4`
- Runtime status:
  - `8080/8081/8082` health all `200`
  - `/proc/<pid>/exe`:
    - `8080 -> /root/ecommerce_ai/releases/v0.8/ecommerce-api`
    - `8081 -> /root/ecommerce_ai/releases/v0.8/erp_bridge`
    - `8082 -> /root/ecommerce_ai/erp_bridge_sync`
  - all `exe_deleted=false`.

### Closed-loop verification truth
- Full chain:
  - `/tmp/iteration106_live_verify_result.json`
  - local copy `tmp/iteration106_live_verify_result.json`
  - summary `101/101` passed.
- User-org patch closure:
  - `/tmp/iteration106_org_patch_verify_result.json`
  - local copy `tmp/iteration106_org_patch_verify_result.json`
  - summary `6/6` passed.

### Boundaries to keep explicit
- Legacy `owner_team` is still compatibility-active.
- Not full ABAC yet.
- Org management is still minimal (server-config authority, no full CRUD platform).
- Historical task canonical ownership may still be incomplete.

## 2026-04-02 iteration 105 handover: performance optimization + full live acceptance on v0.8
- Treat this section as the latest handover truth source.

### What changed in runtime
- `service/task_design_asset_read_model.go`
  - task detail design-assets hydration now batches `task_assets` by task and groups in memory by `asset_id`, reducing repeated per-asset reads.
- `service/task_data_scope_guard.go`
  - data-scope resolution now prefers actor context org fields and only falls back to repo user hydration when actor scope is empty.
- `repo/mysql/identity.go`
  - added batched role query `ListRolesByUserIDs(ctx, []int64)`.
- `service/identity_service.go`
  - `/v1/users` role assembly now uses batched role hydration when repo supports it.
  - `/v1/org/options` now returns a cloned in-memory cached object (once cache), reducing repeated object rebuild overhead.

### What did not change
- No business contract change for create/action/ownership/upload/detail fields.
- No migration/index/schema change in this iteration.
- Runtime entrypoint remains `./cmd/server`.
- Release line remains existing overwrite `v0.8`.

### Local required checks (all passed)
- `go test ./service ./transport/handler`
- `go build ./cmd/server`
- `go build ./repo/mysql ./service ./transport/handler`
- `go test ./repo/mysql`

### Deploy truth
- Command:
  - `bash ./deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 performance optimization: task detail/user role/scope"`
- Evidence:
  - `deploy/release-history.log` has packaged/uploaded/deployed records at `2026-04-02T02:57:47Z` to `2026-04-02T02:58:13Z`
- Health/runtime:
  - `8080/8081/8082 /health = 200`
  - live executables:
    - `8080 -> /root/ecommerce_ai/releases/v0.8/ecommerce-api`
    - `8081 -> /root/ecommerce_ai/releases/v0.8/erp_bridge`
    - `8082 -> /root/ecommerce_ai/erp_bridge_sync`
  - none are deleted executables

### Full live acceptance truth
- Script:
  - `scripts/iteration105_live_verify.py`
- Result artifact:
  - local: `tmp/iteration105_live_verify_result.json`
  - remote: `/tmp/iteration105_live_verify_result.json`
- Latest run:
  - `started_at=2026-04-02T03:28:25Z`
  - `finished_at=2026-04-02T03:28:33Z`
  - `101/101` checks passed
- Coverage:
  - admin/auth/org/roles
  - create chains (original defer + new single/batch + purchase single/batch + create deny validations)
  - reference small upload + detail/preview/download
  - design multipart upload session + complete + detail `design_assets/asset_versions` + preview/download
  - list/detail canonical ownership fields
  - full action chain assign/reassign -> audit A/B -> warehouse -> close -> completed
  - permission deny coverage and operation/task-event logs
- Confirmed event chain includes `task.closed`.
- Confirmed permission logs include both allow and deny traces.

### Entities created during acceptance (kept)
- users: ids `141..152` (`iter105_*`)
- tasks: ids `301..315`
- uploads:
  - reference asset `9b83eeb8-2553-4b43-8003-52cc1dfd8611`
  - design delivery session `2270604d-3e00-403a-a6f3-9e9943af4c01` (task `311`)

### Known boundaries to keep explicit
- This is still not full ABAC unification across every route.
- Legacy `owner_team` is still returned for compatibility and has not been removed.
- Historical tasks can still have incomplete canonical ownership data.
- Performance work in this iteration is hotspot mitigation (batching/cache/reducing duplicate reads), not deep query-plan or architecture rework.

## 2026-04-02 destructive test reset keep-admin handover
- Treat this section as the latest handover truth source for the live destructive reset baseline on `v0.8`.

### What was done
- Executed a high-risk data reset with backup-first discipline:
  - server backup: `/root/ecommerce_ai/backups/20260402T022844Z_pre_test_reset_keep_admin`
  - NAS backup: `/volume1/homes/yongbo/asset-upload-service/backups/20260402T022844Z_pre_test_reset_keep_admin`
- Stopped `8080/8081/8082`, ran SQL data cleanup, cleaned server/NAS/local cache+tmp+logs, then restarted and re-verified services.

### Keep vs clear contract
- Preserved:
  - Admin/SuperAdmin login chain and role bindings
  - base org/roles/options/config behavior
  - release binaries, deploy scripts, migration files, repo/docs skeleton
- Cleared:
  - task/asset/procurement/audit/warehouse/outsource business data
  - upload/storage metadata (`upload_requests`, `asset_storage_refs`)
  - export/integration/operation traces and runtime log tables
  - server/NAS/local temporary and log artifacts in scoped test-data paths

### Admin guard and user result
- SQL guard explicitly refuses reset if keep-admin set resolves to empty.
- Retained users after reset:
  - `admin`
  - `testuser_fix`
  - `candidate_test`
  - `test_01`
- Non-admin test users were removed.
- `user_sessions` was cleared during reset; new sessions were then created by post-reset verification logins.

### Verified post-reset state
- Service health:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
- Admin/base APIs:
  - login success
  - `/v1/auth/me` OK
  - `/v1/org/options` OK
  - `/v1/roles` OK
- Data-empty checks:
  - `/v1/tasks?page=1&page_size=20` -> empty
  - `/v1/operation-logs` -> empty
  - `/v1/export-jobs` -> empty
  - `/v1/integration/call-logs` -> empty
  - DB post-reset counts show task/asset/procurement/export/integration core tables are zero

### Residuals to remember
- Permission/session traces seen after reset are from the verification actions themselves.
- Base config/rule data is intentionally preserved (not part of test-data wipe).
- NAS upload-service root skeleton is preserved; scoped task/upload test objects were deleted.

## 2026-04-02 task detail image return contract handover
- Treat this section as the latest handover truth source for task-detail reference/design image fields.

### What the next model should assume
- `GET /v1/tasks/{id}` formal image fields are:
  - `reference_file_refs`
  - `design_assets`
  - `asset_versions`
- `task_details.reference_images_json` is compatibility-only for old data reads.
- `/v1/assets/*` is not the task-detail canonical projection surface; `/v1/assets/files/*` remains a file proxy route only.

### Confirmed runtime behavior
- New create flow:
  - `POST /v1/tasks` rejects `reference_images`
  - formal references are passed by `reference_file_refs`
  - persistence keeps `reference_images_json = []` and writes `reference_file_refs_json`
- Read flow:
  - `enrichTaskReadModelDetail` reads `reference_file_refs_json` first
  - only if formal refs are empty does it fallback to `reference_images_json`
- Design asset read model:
  - `design_assets` + `asset_versions` are hydrated from upload-complete persisted `design_assets`/`task_assets`
  - visibility does not require `submit-design`

### Changes in this round
- Runtime code:
  - none
- Tests:
  - `service/reference_images_test.go` now explicitly verifies:
    - formal refs are returned by `GET /v1/tasks/{id}` while legacy array is empty
    - formal-over-legacy precedence
    - legacy fallback still readable for old records
- Docs/openapi:
  - clarified formal-vs-legacy contract wording so frontend/callers do not rely on legacy empty arrays

### Verification completed
- `go test ./service ./transport/handler`
- `go build ./cmd/server`
- `go build ./repo/mysql ./service ./transport/handler`
- `go test ./repo/mysql`

### Publish/live
- No runtime change, so no overwrite publish and no live acceptance run in this round.

## 2026-04-01 task action org gating for audit warehouse close handover
- Treat this section as the latest handover truth source for the MAIN task-action minimum org gating closure on live `v0.8`.

### What the next model should assume
- The existing minimum org authorization line is now extended beyond assign/reassign into these routed task actions:
  - audit claim / approve / reject / transfer / handover / takeover
  - warehouse receive / reject / complete
  - close
- The stable shared entry remains:
  - `service/task_action_rules.go`
  - `service/task_action_authorizer.go`
  - `service/task_action_scope.go`
- Do **not** revert this back to route-role-only checks or service-local ad hoc checks.
- Do **not** treat `Admin` or other view-all roles as status-bypass roles.

### What changed in code
- Audit actions are now stage-resolved inside the authorizer:
  - generic audit actions resolve to `audit_a_*`, `audit_b_*`, or outsource-review variants
  - wrong requested stage returns `audit_stage_mismatch`
- Audit and warehouse services now rely on the shared authorizer for:
  - required role
  - canonical ownership scope
  - current-handler requirements
  - machine-readable deny codes
- Warehouse load path no longer pre-rejects by status before the shared authorizer can emit `warehouse_stage_mismatch`.
- Close uses the same unified authorizer line and keeps readiness as a second business gate after permission.

### Final rule boundary
- `view_all` roles may cross org scope but still cannot cross invalid status.
- `DepartmentAdmin` / `DesignDirector` require department ownership match.
- `TeamLead` requires canonical `owner_org_team` match.
- `Audit_A` is constrained to `PendingAuditA`.
- `Audit_B` is constrained to `PendingAuditB`.
- Non-management audit approve/reject/transfer/handover require current-handler match.
- `Warehouse` is constrained to `PendingWarehouseReceive`.
- Non-management warehouse reject/complete require current-handler match.
- `close` is permission-gated only on `PendingClose`; closability remains a second business-readiness gate.

### Explicit route boundary
- No separate routed action currently exists for:
  - audit `submit`
  - audit `return`
  - warehouse `reopen`
  - warehouse `return`
  - task `reopen`
  - pending-close confirm
  - reject-close
- Do not claim those are unified until the routes actually exist.

### Verification expectation
- Required local checks completed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`
- Required publish completed:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task action org gating for audit warehouse close"`
- Runtime verification completed:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/3532025/exe -> /root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/proc/3532047/exe -> /root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `/proc/3532217/exe -> /root/ecommerce_ai/erp_bridge_sync`

### What actually happened on live `v0.8`
- Temporary scoped verification users were created through real `/v1/auth/register` and then role-mutated through `/v1/users/:id/roles`:
  - `iter102_probe_1775029501` -> `Audit_A`, `Audit_B`, `TeamLead`, team `运营一组`
  - `iter102_audit_out_1775029502` -> `Audit_A`, `Audit_B`, `TeamLead`, team `运营三组`
  - `iter102_ops_in_1775029503` -> `DepartmentAdmin`, `Warehouse`, department `运营部`
  - `iter102_ops_out_1775029504` -> `DepartmentAdmin`, `Warehouse`, department `设计部`
  - these temporary users were disabled after acceptance
- Audit acceptance:
  - task `163` out-of-scope audit A deny -> `task_out_of_team_scope`
  - task `165` in-scope audit A approve -> `PendingAuditB`
  - task `165` out-of-scope audit B deny -> `task_out_of_team_scope`
  - task `165` in-scope audit B approve -> `PendingWarehouseReceive`
- Warehouse acceptance:
  - task `165` out-of-scope receive deny -> `task_out_of_department_scope`
  - task `165` in-scope receive success -> receipt `received`
  - task `163` wrong-stage receive deny -> `warehouse_stage_mismatch`
  - task `165` in-scope complete success -> `PendingClose`
- Close acceptance:
  - task `137` out-of-scope close deny -> `task_out_of_department_scope`
  - task `163` wrong-status close deny -> `task_not_closable`
  - task `137` in-scope close success -> `Completed`
- Assign regression acceptance:
  - task `172` reassign `41 -> 42 -> 41` both succeeded
  - task `163` assign remained denied with `task_not_reassignable`
- Canonical ownership regression acceptance:
  - live list/detail still returned `owner_team`, `owner_department`, `owner_org_team`

### Remaining boundary
- This is still not complete task-action ABAC.
- Legacy `owner_team` still exists and is still returned.
- Historical tasks with empty canonical ownership remain edge-boundary cases.
- Some route-role middleware still rejects role-missing calls before the shared authorizer can emit an action-specific `deny_code`.

## 2026-04-01 task-create reference small escaped-storage-key handover
- Treat this section as the latest handover truth source for the task-create reference-small probe/proxy repair on live `v0.8`.

### What the next model should assume
- The reference upload architecture did **not** change:
  - `reference = small`
  - `delivery/source/preview = multipart`
  - small reference still uses `/upload/files`
  - small reference still does not call NAS `complete`
  - success still depends on stored size/hash verification
- MAIN service-to-service NAS traffic still uses:
  - `UPLOAD_SERVICE_BASE_URL=http://100.111.214.38:8089`
- Browser multipart direct upload still uses:
  - `http://192.168.0.125:8089`

### What actually broke
- The stable user-reported repro was not host selection and not a small-path `complete` regression.
- Historical live trace `cba34f59-5f24-4280-9fea-c2b7e2d1eeee` showed:
  - `/upload/files` returned a valid `storage_key`
  - the `storage_key` filename contained raw `%` plus UTF-8 characters
  - MAIN then tried to parse `"/files/{storage_key}"` locally before sending the probe
  - all three probe attempts failed before request dispatch with `invalid URL escape "% \xf0"`
- Therefore the true stable failure was unescaped storage-key reuse in HTTP path construction.
- The earlier short-lived visibility race remains a separate storage-side behavior and the bounded retry remains useful for that lane, but it was not the root cause of the reported stable failure.

### What changed in code
- Added shared escaped-path helpers in:
  - `domain/url_path.go`
- Updated runtime URL construction in:
  - `service/upload_service_client.go`
  - `transport/handler/asset_files.go`
  - `service/task_create_reference_upload_service.go`
  - `service/task_asset_center_read_model.go`
- Result:
  - probe URLs now escape storage keys by path segment
  - asset-file proxy upstream URLs now escape storage keys by path segment
  - returned `public_url` / `lan_url` / `tailscale_url` are now escaped and readable for `%`/UTF-8 filenames
- The previously added bounded probe retry remains:
  - only on reference small
  - still no `complete` call
  - still hard-fails on empty storage key, invalid metadata, size mismatch, and hash mismatch
- Logging improvement retained:
  - `trace_id`
  - `upload_service_base_url`
  - `selected_probe_host`
  - `storage_key`
  - `filename`
  - `expected_size`
  - `expected_sha256`
  - per-attempt `probe_status` / `probe_error`
- Sensitive token values are not logged from the asset-file proxy request log anymore.

### Boundaries that must stay explicit
- Do not describe this as a multipart migration.
- Do not describe this as reintroducing small-path `complete`.
- Do not describe this as "probe can fail and success still returns".
- Do not claim OpenAPI changed:
  - this round changed implementation correctness and URL safety only

### Verification expectation
- Required local checks completed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
- Required live checks completed after overwrite publish:
  - `8080 /health`
  - `8081 /health`
  - `8082 /health`
  - `/proc/<pid>/exe`
  - one live `POST /v1/tasks/reference-upload` with `%`/UTF-8 filename
  - one live `GET` on the escaped returned `public_url`
  - one live `new_product_development` create using returned `reference_file_refs`
  - one live `/v1/tasks` list read
  - one live existing batch-task detail read

### What actually happened on live `v0.8`
- Required repo deploy entrypoint was used:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 fix escaped storage-key probe and proxy urls"`
- Runtime verification passed:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/3503354/exe -> /root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/proc/3503402/exe -> /root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `/proc/3503547/exe -> /root/ecommerce_ai/erp_bridge_sync`
  - active executables were not deleted
- Live acceptance passed:
  - failing trace `cba34f59-5f24-4280-9fea-c2b7e2d1eeee` was pinned to pre-request URL parse failure on raw `%`/UTF-8 `storage_key`
  - `POST /v1/tasks/reference-upload` with filename `💚97% 能量充满啦.png` returned `201`
  - success trace `502f127d-02de-4c51-8446-99899df5530b`
  - returned ref `asset_id=569c7113-bde2-4ced-a20b-964336ac8b05`
  - live logs showed correct probe host and matching stored size/hash
  - escaped returned `public_url` was readable through MAIN proxy (`200`, `137253` bytes)
  - `POST /v1/tasks` with that ref created live task `169 / RW-20260401-A-000164`
  - `GET /v1/tasks?page=1&page_size=5` remained healthy
  - existing batch task `167` remained readable
  - canonical ownership fields stayed present on live reads

### Remaining boundary
- The user-reported stable failure is closed by escaped URL/path construction.
- The separate storage-side visibility race still belongs to NAS/upload-service if it reappears in logs.

## 2026-04-01 task-create reference small probe retry handover
- Treat this section as the latest handover truth source for the task-create reference-small repair on live `v0.8`.

### What the next model should assume
- The reference upload architecture did **not** change:
  - `reference = small`
  - `delivery/source/preview = multipart`
  - small reference still uses `/upload/files`
  - small reference still does not call NAS `complete`
  - success still depends on stored size/hash verification
- MAIN service-to-service NAS traffic still uses:
  - `UPLOAD_SERVICE_BASE_URL=http://100.111.214.38:8089`
- Browser multipart direct upload still uses:
  - `http://192.168.0.125:8089`

### What actually broke
- The real fault was not host selection and not path rebasing.
- Archived live traces showed:
  - `/upload/files` returned a valid `storage_key`
  - MAIN probed the correct `http://100.111.214.38:8089/files/{storage_key}` URL
  - some immediate probes still saw a transient empty object (`200`, `bytes_read=0`, `content_length=0`)
- That means the bug was a short-lived write-visibility race between upload completion and immediate server-to-server probe.
- The frontend-visible `internal error during probe task-create reference stored file` can happen in the same race family when the first probe attempt returns a temporary HTTP/network failure instead of an empty `200`.

### What changed in code
- `service/task_create_reference_upload_service.go`
  - added a narrow retry loop only around the task-create reference small probe
  - retries only transient probe failures and transient empty stored-object reads
  - keeps hard failure on empty storage key, invalid probe metadata, size mismatch, and hash mismatch
  - adds probe-attempt diagnostics:
    - `upload_service_base_url`
    - `selected_probe_host`
    - `storage_key`
    - `filename`
    - `expected_size`
    - `expected_sha256`
    - `probe_status` / `probe_error` / `retry_reason`
- `service/upload_service_client.go`
  - probe logs now include `base_url` and `selected_probe_host`
- Tests updated in:
  - `service/task_create_reference_upload_service_test.go`
  - `service/task_asset_center_service_test.go`
  - `transport/handler/task_reference_upload_contract_test.go`

### Boundaries that must stay explicit
- Do not describe this as a multipart migration.
- Do not describe this as reintroducing small-path `complete`.
- Do not describe this as “probe can fail and success still returns”.
- Do not claim OpenAPI changed:
  - this round changed implementation stability and observability only

### Verification expectation
- Required local checks completed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
- Required live checks completed after overwrite publish:
  - `8080 /health`
  - `8081 /health`
  - `8082 /health`
  - `/proc/<pid>/exe`
  - one live `POST /v1/tasks/reference-upload`
  - one live `new_product_development` create using returned `reference_file_refs`
  - one live `/v1/tasks` list read
  - one live existing batch-task detail read

### What actually happened on live `v0.8`
- Required repo deploy entrypoint was used:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task-create reference small probe retry and diagnostics"`
- Local bash packaging initially failed because repo-local `deploy/*.sh` still had CRLF line endings on this control node.
- After normalizing `deploy/*.sh` to LF, the same deploy entrypoint overwrite-published `v0.8` successfully.
- Runtime verification passed:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/3484605/exe -> /root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/proc/3484628/exe -> /root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `/proc/3484806/exe -> /root/ecommerce_ai/erp_bridge_sync`
  - active executables were not deleted
- Live acceptance passed:
  - `POST /v1/tasks/reference-upload` -> `201`
  - trace `14d2ffd0-c52a-452d-a0d5-e3f87096eabf`
  - returned ref `asset_id=5d749472-deb2-4f89-8660-eb7eeef0c227`
  - live logs showed correct probe host and size/hash verification on attempt `1/3`
  - `POST /v1/tasks` with that ref created live task `168 / RW-20260401-A-000163`
  - `GET /v1/tasks?page=1&page_size=5` remained healthy
  - existing batch task `167` remained readable
  - canonical ownership fields stayed present on live reads
- No forced live probe-failure sample was run because reproducing the race safely would require intentionally destabilizing NAS visibility or stored bytes on the shared live environment.

## 2026-03-31 task action minimum org-scoped authorization handover
- Treat this section as the latest handover truth source for the task action organization round.

### What the next model should assume
- Live release line is still intended to stay on overwrite-published `v0.8`.
- Task ownership is still dual-track:
  - legacy compatibility field: `tasks.owner_team`
  - canonical task org fields: `tasks.owner_department`, `tasks.owner_org_team`
- This round builds on canonical ownership and minimum list/detail visibility; it does not replace them.
- This round is still not a full ABAC engine and still not a generic policy platform.

### What changed in code
- Added a shared task action authorization layer in `service/`:
  - `task_action_scope.go`
  - `task_action_rules.go`
  - `task_action_authorizer.go`
- Request-actor resolution now carries richer org context:
  - `department`
  - `team`
  - `managed_departments`
  - `managed_teams`
  - `frontend_access`
- The shared authorizer now gates these task actions:
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
- Decision model is intentionally small and explainable:
  - first check required business / management role
  - then evaluate minimum scope over canonical `owner_department` / `owner_org_team`
  - then apply status and handler/designer/creator checks where the action requires it
- Denials stay on `PERMISSION_DENIED` and now add machine-readable details:
  - `deny_code`
  - `deny_reason`
  - `matched_rule`
  - `scope_source`

### Scope model now in effect
- `Admin` / `SuperAdmin` / `RoleAdmin` / `HRAdmin` use `view_all`.
- `DepartmentAdmin` / `DesignDirector` are bounded by canonical `owner_department`.
- `TeamLead` is bounded by canonical `owner_org_team`.
- Workflow roles such as `Designer`, `Audit_A`, `Audit_B`, `Warehouse`, and `Ops` keep node semantics:
  - they do not get global action power from role name alone
  - handler/designer/creator matches still matter for the smaller non-management path
- Manager overrides are intentionally minimal:
  - a scoped department/team manager can act inside the matched canonical scope
  - out-of-scope access denies with department/team-specific `deny_code`

### Boundaries that must stay explicit
- Do not describe this as full ABAC.
- Do not describe this as a reusable generic resource/action policy engine.
- Do not describe ordinary member behavior as complete org-scoped authorization.
- Do not claim every task-adjacent route has been unified:
  - batch remind is still on older behavior
  - some compatibility/mock asset routes remain thin aliases over the main actions
  - audit handover listing is still read-only/read-path behavior
- Do not remove legacy `owner_team` until downstream consumers migrate.

### Verification expectation
- Required local checks for this round are:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`
- Required live checks, if this runtime cut is published, are:
  - `8080 /health`
  - `8081 /health`
  - `8082 /health`
  - `/proc/<pid>/exe`
  - one detail-read allow/deny sample
  - one department-scope allow/deny task action
  - one team-scope allow/deny task action
  - one handler/self allow/deny task action

### What actually happened on live `v0.8`
- The first overwrite publish completed and service health stayed green, but live action verification exposed a real authorization gap:
  - `TeamLead` could still operate across teams inside the same department because generic department scope was being accepted for a team-scoped manager path
  - non-management workflow-role deny ordering could return org-scope denial before handler-mismatch denial
- A second code fix tightened role-aware scope usage and deny ordering inside the shared task action authorizer.
- The second overwrite publish onto the same `v0.8` line succeeded.
- Final live verification then passed for:
  - health on `8080` / `8081` / `8082`
  - `/proc/<pid>/exe`
  - detail read allow/deny
  - department-scoped business-info allow/deny
  - team-scoped assign allow/deny
  - handler/self-related submit-design allow/deny

## 2026-03-31 canonical task org ownership and minimum task visibility handover
- Treat this section as the latest handover truth source for the task/org formal connection round.

### What the next model should assume
- Live release line is still overwrite-published `v0.8`.
- Task ownership is now intentionally dual-track:
  - legacy compatibility field: `tasks.owner_team`
  - canonical task org fields: `tasks.owner_department`, `tasks.owner_org_team`
- The previous create-time `owner_team` compatibility bridge still exists and must not be removed casually.
- The new canonical ownership fields are the task-side source for further org-aware work.
- This is still not a full org-model unification and not a full ABAC engine.

### What changed in code
- Added migration `047_v7_task_canonical_org_ownership.sql`.
- `POST /v1/tasks` now resolves and persists:
  - normalized legacy `owner_team`
  - canonical `owner_department`
  - canonical `owner_org_team`
- `GET /v1/tasks` and `GET /v1/tasks/{id}` now expose canonical task ownership in addition to legacy `owner_team`.
- `GET /v1/tasks` now accepts canonical ownership filters:
  - `owner_department`
  - `owner_org_team`
- Minimum task visibility is now wired to canonical ownership:
  - view-all roles still bypass org filtering
  - department-scoped management roles filter on `owner_department`
  - team-scoped management roles filter on `owner_org_team`
  - member/self-related behavior remains intentionally minimal
- Scope resolution now loads full user org data from `userRepo`; previous actor-only scope resolution was not enough for reliable department/team filtering.

### Mapping and backfill policy
- If create input is a supported org team such as `运营三组`, backend:
  - normalizes legacy `owner_team`
  - writes canonical department/team
- If create input is a legacy `owner_team`, backend only backfills canonical values when the mapping is deterministic.
- Historical data is not fully rewritten:
  - safe deterministic department-level backfill is allowed
  - ambiguous historical org-team ownership remains null/empty in canonical columns

### Verification and release expectation
- Required local checks for this round are:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
- Additional repo-layer regression should also be run because query SQL and scans changed:
  - `go test ./repo/mysql`
- Required live checks after overwrite publish:
  - `8080 /health`
  - `8081 /health`
  - `8082 /health`
  - `/proc/<pid>/exe`
  - original/new/purchase create with org-team input such as `运营三组`
  - `/v1/tasks` canonical ownership fields
  - minimum org-scoped list filtering

### What actually happened on live `v0.8`
- First overwrite publish succeeded at runtime level but initial acceptance immediately exposed a live schema gap:
  - `GET /v1/tasks` returned `500`
  - cause: `047_v7_task_canonical_org_ownership.sql` was not yet applied on live DB
- Before mutating live schema, a backup was created:
  - `/root/ecommerce_ai/backups/20260331T033855Z_task_canonical_org_047`
- Live migration was then applied from the released artifact:
  - `/root/ecommerce_ai/releases/v0.8/db/migrations/047_v7_task_canonical_org_ownership.sql`
- A second overwrite publish followed because the first runtime cut still had `omitempty` on task ownership JSON fields, which made empty canonical fields disappear from some list/detail responses.
- Final live verification result:
  - `8080 /health` = `200`
  - `8081 /health` = `200`
  - `8082 /health` = `200`
  - `/proc/<pid>/exe` resolved to:
    - `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
    - `/root/ecommerce_ai/releases/v0.8/erp_bridge`
    - `/root/ecommerce_ai/erp_bridge_sync`
  - original/new/purchase create with `运营三组` succeeded and returned:
    - legacy `owner_team=内贸运营组`
    - canonical `owner_department=运营部`
    - canonical `owner_org_team=运营三组`
  - additional live visibility verification also passed:
    - view-all admin saw all verification tasks
    - `DepartmentAdmin` in `运营部` saw ops-department tasks but not design-department tasks
    - `TeamLead` in `运营一组` saw only `运营一组` tasks and not `运营三组` / design tasks

### Boundaries that must remain explicit
- Do not describe this as the final org platform.
- Do not describe this as full row-level visibility or ABAC.
- Do not claim historical tasks are fully backfilled unless a later round really does that work.
- Do not remove legacy `owner_team` until the rest of the task workflow and external consumers are migrated.

## 2026-03-31 batch-SKU `v0.8` overwrite publish handover
- Treat this section as the latest handover truth source for the 2026-03-31 MAIN self-test + overwrite publish round.

### What the next model should assume
- Current live release line is still overwrite-published `v0.8`.
- Current MAIN live binary target is still:
  - `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
- Current bridge live binary target is:
  - `/root/ecommerce_ai/releases/v0.8/erp_bridge`
- Current sync runtime stayed unchanged:
  - `/root/ecommerce_ai/erp_bridge_sync`
- Production entrypoint remains locked to:
  - `cmd/server/main.go`
- `cmd/api` remains compatibility-only and was not used for this publish.

### What was actually done
- Local verification executed before publish:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - targeted task batch regressions in `service`, `transport/handler`, and `repo/mysql`
- Local package was built through:
  - `deploy/package-local.sh --version v0.8 --skip-tests`
- Deploy scripts in `deploy/*.sh` were normalized to LF so repository-managed bash scripts run correctly during local packaging and remote runtime actions.
- Overwrite publish completed onto existing `v0.8`.
- First live acceptance then exposed a real DB schema gap:
  - live log showed `Unknown column 'is_batch_task' in 'field list'`
- Before fixing schema, this round created a rollback boundary:
  - `/root/ecommerce_ai/backups/20260331T012734Z_task_batch_schema_046/tasks_procurement_before.sql`
- Live migration then applied:
  - `/root/ecommerce_ai/releases/v0.8/db/migrations/046_v7_task_batch_sku_items.sql`

### Why the deploy path looked slightly different from the happy-path wrapper
- The repository `deploy.sh` wrapper started normally on the local control node, but its Windows/WSL remote phase was unstable.
- This round still stayed inside repository-managed deployment assets:
  - local package: `deploy/package-local.sh`
  - remote cutover: packaged `deploy/remote-deploy.sh`
  - runtime verification: packaged `deploy/verify-runtime.sh`
- No ad-hoc binary copy/move layout was introduced and no new release line was created.

### Live state the next model can trust
- Health:
  - `8080` `/health` = `200`
  - `8081` `/health` = `200`
  - `8082` `/health` = `200`
- Runtime pointers:
  - 8080 PID `3186035` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - 8081 PID `3186057` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - 8082 PID `3186156` -> `/root/ecommerce_ai/erp_bridge_sync`
- Deleted-binary state:
  - all three active `/proc/<pid>/exe` pointers were verified as non-deleted
- SHA-256:
  - `/root/ecommerce_ai/releases/v0.8/ecommerce-api` -> `16dcbcc6bcc53c97d6a3abad138c44939171f06126c791e38087bbebd9f2a721`
  - `/root/ecommerce_ai/releases/v0.8/erp_bridge` -> `16dcbcc6bcc53c97d6a3abad138c44939171f06126c791e38087bbebd9f2a721`
  - unchanged `/root/ecommerce_ai/erp_bridge_sync` -> `2264a80cc8318d08828fcf29a6f7ddaa3ea69804ab13dc5b71e293d97afc82b8`
- 8082 truth:
  - publish verification triggered auto-recovery for 8082
  - sync was restored successfully
  - 8082 binary itself was not replaced in this round

### Live API evidence
- Real bearer session:
  - `POST /v1/auth/login` -> `200`
- Post-migration list sanity:
  - `GET /v1/tasks?page=1&page_size=5` -> `200`
- Live-safe create verification:
  - single `new_product_development` -> `201`, task `147`
  - batch `new_product_development` -> `201`, task `148`
  - batch `purchase_task` -> `201`, task `149`
  - batch `original_product_development` -> `400 INVALID_REQUEST` with `batch_not_supported_for_task_type`
- Live readback verification:
  - `GET /v1/tasks/147` -> batch fields present with single-SKU values
  - `GET /v1/tasks/148` -> additive fields present:
    - `is_batch_task`
    - `batch_item_count`
    - `batch_mode`
    - `primary_sku_code`
    - `sku_generation_status`
    - `sku_items`
  - `GET /v1/tasks/149` -> same additive batch fields present

### Remaining caution
- This round left explicit live verification tasks in DB:
  - `147`, `148`, `149`
- If a later cleanup round wants a pristine production dataset, these IDs should be handled explicitly instead of being forgotten.
- The current important historical fact is:
  - code publish alone was not enough
  - live effectiveness required both overwrite publish and migration `046`

## 2026-03-23 non-explicit historical normalization audit handover
- Treat this section as the latest handover truth source after `ITERATION_092`.
- This round was evidence-only:
  - no backend code change
  - no deploy
  - no DB mutation
  - no second-round delete yet

### What was rechecked
- Live DB audit widened from explicit `test/demo/accept/case` markers to non-explicit suspicious residue.
- Live regression widened from task detail/product-info/cost-info to include:
  - task list pagination / task-type filters
  - `/v1/org/options`
  - `/v1/roles`
  - `/v1/users`
  - `/v1/permission-logs`

### Findings the next model should trust
- The first-round cleanup did not regress the live read path:
  - all `66` remaining tasks still return `200` on:
    - `GET /v1/tasks/{id}`
    - `GET /v1/tasks/{id}/product-info`
    - `GET /v1/tasks/{id}/cost-info`
- Wider task-list regression also passed:
  - `/v1/tasks` pages `1~4` = `200`
  - `/v1/tasks?task_type=original_product_development` = `200`
  - `/v1/tasks?task_type=purchase_task` = `200`
- Org / permission boundary stayed intact:
  - admin bearer session gets `200` on `/v1/org/options`, `/v1/roles`, `/v1/users`, `/v1/permission-logs`
  - `Ops` and roleless sessions get `403` on the same management routes

### DB-side negatives already ruled out
- `existing_product` weak consistency remains closed:
  - missing `product_id` = `0`
  - missing `products` FK target = `0`
  - bound `sku_code` mismatch = `0`
- Audited JSON payloads remain valid:
  - `product_selection_snapshot_json`
  - `matched_mapping_rule_json`
  - `reference_file_refs_json`
- Core task relations remain structurally clean:
  - missing `creator/designer/current_handler` = `0`
  - orphan `task_assets` / `design_assets` = `0`

### Candidate buckets frozen by this round
- `保留并归一`:
  - `9` deterministic `asset_storage_refs` rows on tasks `144/145`
  - wrong pattern: `asset_storage_refs.asset_id = task_assets.id`
  - correct target already present: `task_assets.asset_id = design_assets.id`
- `需人工确认`:
  - task IDs:
    - `95,96,97,106,112,113,114,115,116,117,118,119,120,122,124,125,128,130,131,132,134,135,137,138,139,140,142,144,145`
  - these are business-like, linked, or still active; do not bulk-delete from suspicion alone
- `明确可删`:
  - task IDs:
    - `47,48,49,58,59,61,62,69,71,72,73,74,75,76,98,99,100,111,121,123,126`
  - supporting signals:
    - `live verify`
    - `黑盒V04`
    - `Verify`
    - `BRIDGE-REMOTE-CHECK`
    - `ERP Stub`
    - `Roleless Verify`
    - `验收defer路径`
    - `ERP acceptance`
    - `reference image small verify`
    - `测试新品`
    - `Step87`

### Important org-field boundary
- Current account-org truth comes from live `/v1/org/options`:
  - departments/teams are the new minimum org model
- But task `owner_team` is still a deliberate legacy compatibility field in code:
  - see `domain.DefaultDepartmentTeams`
  - see `domain.ValidTeam`
  - see `service.validateCreateTaskEntry`
- Therefore:
  - legacy `users.department/team` can be audited as historical org residue
  - legacy `tasks.owner_team` must not be mass-normalized inside this cleanup lane

### Recommended next order
1. backup DB again before any mutation
2. normalize the `9` deterministic `asset_storage_refs` rows first
3. manually confirm the `29` active/business-like suspicious tasks
4. only then consider deleting the `21` clear-delete tasks with full dependent cleanup
5. repeat the same full live regression set after any mutation

### Still out of scope
- upload-chain redesign
- task detail aggregate redesign
- org/permission model redesign
- second-round bulk delete in this iteration

## 2026-03-23 historical task cleanup and read-model 500 handover
- Treat this section as the latest handover truth source for the historical task `500` audit/cleanup round.
- This round did not reopen upload-chain or org/permission design. It was limited to:
  - historical task read-model `500` evidence collection
  - dirty-data classification
  - backup before mutation
  - retained-task repair
  - explicit test/demo/case residue cleanup

### What was actually audited
- Historical log evidence confirmed earlier `500` clusters on:
  - `GET /v1/tasks/84`
  - `GET /v1/tasks/84/product-info`
  - `GET /v1/tasks/84/cost-info`
  - `GET /v1/tasks/136~141/cost-info`
- Current live task-set audit then re-scanned all remaining tasks on:
  - `GET /v1/tasks/{id}`
  - `GET /v1/tasks/{id}/product-info`
  - `GET /v1/tasks/{id}/cost-info`
- Current live result after backend hardening plus this cleanup round:
  - no remaining active `500` in the scanned task set

### Dirty-data patterns found
- Pattern A: retained historical compatibility data
  - `existing_product` tasks with `product_id IS NULL`
  - but `product_selection_snapshot_json` still present and readable
  - this is a keep/fix pattern, not a delete pattern
- Pattern B: explicit test/demo/acceptance/case residue
  - task content itself carried clear markers such as:
    - `accept`
    - `demo`
    - `case`
    - `test`
  - this was the delete boundary
- Pattern C: structural DB integrity
  - no `tasks/task_details` or core task-asset structural orphans were found before mutation
  - audited JSON fields were valid in the current live DB snapshot

### Keep / fix / delete rules
- Keep:
  - business-like historical tasks without explicit test/demo markers
  - snapshot-based historical existing-product tasks that still carry valid ERP snapshot context
- Fix:
  - retained historical tasks with `source_mode=existing_product` and exact `products.sku_code = tasks.sku_code`
  - these were repaired by backfilling `tasks.product_id`
- Delete:
  - only explicit marker tasks where the task content itself showed demo/acceptance/case/test semantics
  - test-account ownership by itself was not treated as enough evidence to delete

### Backup and rollback boundary
- Backup path before mutation:
  - `/root/ecommerce_ai/backups/20260323T120120Z_historical_task_cleanup_v091`
- Files created there:
  - `jst_erp_full_before.sql.gz`
  - `key_tables_before.sql.gz`
  - `candidate_boundaries_before.tsv`
- Cleanup SQL archived in repo:
  - `scripts/historical_task_cleanup_v091.sql`

### Actual live data actions taken
- Repaired retained historical tasks:
  - `12` tasks had `tasks.product_id` backfilled from exact `products.sku_code` match
- Removed explicit marker test tasks:
  - `20`
- Post-cleanup live state:
  - total tasks `86 -> 66`
  - explicit marker task count `20 -> 0`
  - `existing_product_missing_product_id -> 0`

### Verification the next model should trust
- Retained repaired samples verified live:
  - task IDs `106, 114, 137, 139, 142, 144`
  - `GET /v1/tasks/{id}` and `GET /v1/tasks/{id}/product-info` now agree on `product_id`
- Deleted explicit test samples verified live:
  - task IDs `51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136`
  - live read returns `404`
- Full remaining task-set scan verified:
  - `GET /v1/tasks/{id}` = no `500`
  - `GET /v1/tasks/{id}/product-info` = no `500`
  - `GET /v1/tasks/{id}/cost-info` = no `500`

### What remains deferred after this round
- Broader historical-data normalization beyond the explicit marker boundary
- Additional regression expansion around non-explicit legacy test users
- Any future upload-chain or org/permission work remains separate and must not be mixed back into this cleanup lane

## 2026-03-23 minimal organization and permission model on `v0.8`
- Treat this section as the latest handover truth source for account permission, department/team ownership, user-role management, unassigned-pool semantics, and minimum data-scope expression.
- This round intentionally stops at a minimum usable closure:
  - no full org-tree editor
  - no complete ABAC platform
  - no deep row-level visibility engine
  - no auth mainline rewrite

### Organization model
- Fixed departments:
  - `人事部`
  - `设计部`
  - `运营部`
  - `采购部`
  - `仓储部`
  - `烘焙仓储部`
  - `未分配`
- Fixed first-version teams:
  - `人事部 -> 人事管理组`
  - `设计部 -> 定制美工组, 设计审核组`
  - `运营部 -> 运营一组 ... 运营七组`
  - `采购部 -> 采购组`
  - `仓储部 -> 仓储组`
  - `烘焙仓储部 -> 烘焙仓储组`
  - `未分配 -> 未分配池`
- Backend invariants:
  - user owns one primary `department`
  - user owns one primary `team`
  - `team` must be valid under the selected `department`
  - cross-org responsibility is represented by `managed_departments` / `managed_teams`

### Role model
- Minimum management roles now supported:
  - `SuperAdmin`
  - `HRAdmin`
  - `OrgAdmin`
  - `RoleAdmin`
  - `DepartmentAdmin`
  - `TeamLead`
  - `DesignDirector`
  - `DesignReviewer`
  - `Member`
- Compatibility workflow roles remain active:
  - `Admin`
  - `Ops`
  - `Designer`
  - `Audit_A`
  - `Audit_B`
  - `Warehouse`
  - `Outsource`
  - `ERP`
- Admin-class safety rule:
  - removing the last active `Admin` / `SuperAdmin` is blocked
  - disabling the last active `Admin` / `SuperAdmin` is blocked

### First-version responsibility mapping
- Responsibility is expressed by org fields + role assignments + managed scope, not by hardcoded per-name branch logic.
- Default configured mappings shipped in backend config:
  - `刘芸菲`
    - `department = 人事部`
    - `team = 人事管理组`
    - `roles = HRAdmin + OrgAdmin`
    - `managed_departments = ["人事部"]`
  - `王亚琳`
    - `department = 设计部`
    - `team = 设计审核组`
    - `roles = DepartmentAdmin + DesignDirector`
    - `managed_departments = ["设计部"]`
  - `马雨琪`
    - `department = 设计部`
    - `team = 设计审核组`
    - `roles = DesignReviewer`
  - `章鹏鹏`
    - `department = 设计部`
    - `team = 定制美工组`
    - `roles = TeamLead`
    - `managed_teams = ["定制美工组"]`
  - `方晓兵`
    - `department = 采购部`
    - `team = 采购组`
    - `roles = DepartmentAdmin`
    - `managed_departments = ["采购部","仓储部","烘焙仓储部"]`
- Live note:
  - these mappings are configured and applied when matching users exist
  - current live database still contains historical users with older department/team values, so a full historical reassignment is still a later cleanup task

### Unassigned-pool rule
- Unassigned pool is an explicit first-class state:
  - `department = 未分配`
  - `team = 未分配池`
- Pool users do not receive formal business department scope by default.
- Pool users can be reassigned later by management roles through `PATCH /v1/users/{id}` and role updates.
- JST-import users without formal mapping also land here first.

### Minimum `frontend_access` / data-scope expression
- `/v1/auth/me`, `/v1/users`, and `/v1/users/{id}` now expose a unified minimum shape:
  - `roles`
  - `scopes`
  - `menus`
  - `pages`
  - `actions`
  - `view_all`
  - `department_codes`
  - `team_codes`
  - `managed_departments`
  - `managed_teams`
- Current intended expression examples:
  - `SuperAdmin` / `Admin` class can surface `view_all=true`
  - department owners surface department-scoped visibility through departments and managed departments
  - team owners surface managed-team visibility through `managed_teams`
  - unassigned-pool users remain at minimum authenticated/self scope
- This is not a full row-level resolver. It is the minimum stable expression needed for management-side integration.

### API surface now expected by the next model
- Unified user/org/role fields are returned by:
  - `GET /v1/auth/me`
  - `GET /v1/users`
  - `GET /v1/users/{id}`
  - `GET /v1/roles`
  - `GET /v1/permission-logs`
- Added minimum org configuration read endpoint:
  - `GET /v1/org/options`
- User mutation supports minimum org and scope management through:
  - `PATCH /v1/users/{id}`
    - `display_name`
    - `status`
    - `department`
    - `team`
    - `email`
    - `mobile`
    - `managed_departments`
    - `managed_teams`

### Audit-log boundary
- Permission logs now provide minimum traceability for:
  - role changes
  - department/team reassignment
  - managed-scope changes
  - user status enable/disable
  - pool-to-formal-org assignment
- Live-verified action types:
  - `role_assigned`
  - `user_org_changed`
  - `user_scope_changed`
  - `user_status_changed`
  - `user_pool_assigned`
  - `register`

### Deferred after this round
- Full org tree and rich org editing UI
- Complex ABAC / line-by-line visibility engine
- Deep historical task visibility cleanup and broader row-level permissions
- Historical task `500` cases and dirty-data cleanup remain the next practical cleanup priority
- Wider regression test expansion beyond the current auth/users/org minimum
- Full legacy user department/team normalization

## 2026-03-23 task detail asset-visibility final boundary
- Treat this section as the latest truth source for task-detail asset visibility.
- Final backend rule:
  - once `POST /v1/tasks/{id}/assets/upload-sessions/{session_id}/complete` has successfully persisted `design_assets`, `task_assets`, and `design_assets.current_version_id`, `GET /v1/tasks/{id}` must expose those facts through `design_assets` and `asset_versions`
  - `GET /v1/tasks/{id}` must not rely on a later `POST /v1/tasks/{id}/submit-design` call to surface uploaded design versions
  - frontend must not be forced to issue blind `submit-design` retries merely to recover missing task-detail versions
- `submit-design` final semantic boundary:
  - explicit design submission business action
  - audit/workflow transition semantics
  - legacy/manual path where a business submission still needs to be recorded
  - not the read-model visibility trigger for uploaded versions
- Backend patch files for this closure:
  - `domain/query_views.go`
  - `service/task_service.go`
  - `service/task_design_asset_read_model.go`
  - `cmd/server/main.go`
  - `cmd/api/main.go`
  - `docs/api/openapi.yaml`
  - `service/task_read_model_asset_versions_test.go`
  - `service/task_prd_service_test.go`
- Publish closure:
  - overwrite-published to `jst_ecs:/root/ecommerce_ai/releases/v0.8`
  - live pointer confirmed: `/root/ecommerce_ai/current -> /root/ecommerce_ai/releases/v0.8`
  - live runtime confirmed: `8080/8081/8082` all healthy after deploy
- Live verification closure:
  - verified task: `task_id=140`, `task_no=RW-20260320-A-000134`
  - DB evidence:
    - `task.asset.upload_session.completed` exists
    - `task.asset.version.created` exists
    - no `task.design.submitted` exists
  - live `GET /v1/tasks/140` returned:
    - `design_assets_count = 4`
    - `asset_versions_count = 4`
    - `design_assets[].current_version.id = [22, 24, 29, 30]`
    - `asset_versions[].id = [22, 24, 29, 30]`
- Local verification status:
  - `go build ./cmd/server ./cmd/api` passed
  - `go test -c ./service` passed
  - local `.test.exe` execution remained blocked by host Application Control, so the live server verification above is the runtime proof for this closure

## 2026-03-20 MAIN server closure handover baseline
- This section is the latest handover truth source for the current three-endpoint definition, backend upload contracts, and overwrite-published `v0.8` live status.
- Local MAIN repo remains the only control plane for this round.
- Fixed three endpoints:
  - local MAIN engineering workspace
  - server live `jst_ecs`
  - NAS `synology-dsm`
- Frontend is not part of the local workspace in this round; do not describe the current "three endpoints" as including frontend.

### Control-plane rules
- Windows local control node must not enable:
  - `ControlMaster`
  - `ControlPersist`
  - `ControlPath`
- Use only the repository packaging/deploy entrypoints:
  - `deploy/package-local.sh`
  - `deploy/deploy.sh`
  - `deploy/remote-deploy.sh`

### Current live/publish truth
- Live release line remains overwrite-published `v0.8`.
- Current MAIN live binary target:
  - `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
- 2026-03-20 overwrite deployment was executed again from the local MAIN control plane and verified live.
- Verified live multipart create-session response now contains:
  - `remote.headers.X-Internal-Token`
- Verified direct NAS browser-style calls using the returned headers:
  - `PUT /parts/1` = `200`
  - `POST /abort` = `200`

### Reference-small final rule
- `task-create reference` small upload must treat `/upload/files` as the canonical result.
- MAIN must not call NAS `complete` on that small-upload path.
- MAIN must verify landed size/hash before emitting success.
- Verification failure must return an error and must not produce a successful `ReferenceFileRef`.

### Multipart final rule
- Current direct-upload contract for `delivery/source/preview = multipart` requires MAIN to return `remote.headers` in `RemoteUploadSessionPlan`.
- At minimum, `remote.headers` must include `X-Internal-Token`.
- The browser must reuse the returned `remote.headers` on:
  - `PUT part_upload`
  - `POST complete`
  - `POST abort`
- Token must not be hardcoded in frontend.
- This rule belongs to the MAIN server and NAS direct-upload collaboration contract.

### Current operating mode
- `reference = small`
- `delivery/source/preview = multipart`
- browser multipart host = `http://192.168.0.125:8089`
- MAIN server-to-server / probe host = `http://100.111.214.38:8089`

### Next priority
- Next priority is no longer multipart-contract diagnosis.
- Next priority is:
  - database testing
  - dirty-data cleanup
  - preparation for full manual regression testing

## 2026-03-20 authoritative handover baseline
- Treat this section as the current handover truth source for MAIN entrypoints, three-endpoint collaboration, and the `reference-upload` 0-byte fix.
- Do not fall back to older notes that describe:
  - current live as `v0.5`, `v0.6`, or a new `v0.9`
  - task-create `reference` small upload as a flow that must call NAS `complete`
  - `delivery` as the current small-upload mode
  - `/v1/assets/files/*` as the root cause of the 0-byte reference issue

### Current effective entrypoints
- Production runtime/build entrypoint: `cmd/server/main.go`
- Route registration: `transport/http.go`
- Deploy/package entrypoints:
  - `deploy/deploy.sh`
  - `deploy/remote-deploy.sh`
  - `deploy/package-local.sh`
- Production packaging remains locked to `./cmd/server`.
- `cmd/api` is deprecated compatibility-only and must not be treated as a production build/deploy entry.

### Current live/server truth
- Live release line remains overwrite-published `v0.8`.
- MAIN live binary target:
  - `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
- Server deploy scripts:
  - `/root/ecommerce_ai/releases/v0.8/deploy`
- Logs:
  - `/root/ecommerce_ai/logs`
- Live verification must continue to use:
  - `/proc/<pid>/exe`
  - `sha256`
  - `/health`

### Current three-endpoint truth
- Local MAIN repo is the only control plane.
- Server alias: `jst_ecs`
- NAS alias: `synology-dsm`
- Future collaboration must be driven from the local MAIN workspace.
- Windows local control node must keep keepalive but must not enable:
  - `ControlMaster`
  - `ControlPersist`
  - `ControlPath`
- Reason:
  - `OpenSSH_for_Windows_9.5p2` was verified to fail with `getsockname failed: Not a socket` and `Unknown error`
- Linux/macOS may enable SSH multiplexing optionally.
- Standard tmux sessions:
  - server: `main-live`
  - NAS: `nas-upload`
- NAS tmux entry command:
  - `ssh synology-dsm "source ~/.bashrc >/dev/null 2>&1; tmux new -As nas-upload"`

### Current upload/read-chain truth
- MAIN service-to-service NAS calls use `UPLOAD_SERVICE_BASE_URL`.
- Browser multipart direct upload uses `http://192.168.0.125:8089`.
- Current mode split:
  - `reference` = `small`
  - `delivery` = `multipart`
  - `source` = `multipart`
  - `preview` = `multipart`
- `/v1/assets/files/*` is a read proxy only.

## 2026-03-20 task-create reference 0-byte fix handover

### Symptom
- `POST /v1/tasks/reference-upload` returned `201`.
- Returned `ReferenceFileRef` looked structurally valid.
- Newly uploaded reference `public_url` reads returned `200` with zero bytes.
- Historical references still read normally.

### Single confirmed root cause
- Not a MAIN proxy-body forwarding bug.
- Not a global NAS `/files/*` failure.
- Single root cause:
  - NAS small upload plus `complete` pseudo-success

### Effective fix behavior
- For task-create `reference` small uploads, MAIN does not call NAS `complete`.
- MAIN treats `/upload/files` return data as the canonical result for the small path.
- MAIN probes the stored object and verifies stored size/hash before binding success.
- MAIN rejects mismatches and does not emit a successful ref for a 0-byte object.

### Mandatory rule
> `task-create reference` 的 small 上传链路以 `/upload/files` 返回结果为准，不再调用 NAS `complete`；MAIN 必须对落盘结果做 size/hash 校验，校验失败直接报错，不得生成成功 ref。

- Plain English: the task-create reference small-upload path must trust `/upload/files`, must not call NAS `complete`, and must fail immediately on stored size/hash mismatch instead of returning a successful ref.

### What the next model should assume
- The issue is closed.
- The local MAIN workspace is already the formal three-endpoint control plane.
- Current deploy language must stay on overwrite-published `v0.8`.
- Any future troubleshooting should start from the current code path in:
  - `transport/handler/task_create_reference_upload.go`
  - `service/task_create_reference_upload_service.go`
  - `service/upload_service_client.go`
  - `transport/handler/asset_files.go`

## 2026-03-19 task filing policy upgrade (step 87 backend)
- Scope covers `original_product_development`, `new_product_development`, `purchase_task`.
- Legacy single trigger boundary (`PATCH /v1/tasks/{id}/business-info` + `filed_at`) is no longer the only path.
- New backend trigger sources are active in code:
  - `create`
  - `business_info_patch`
  - `procurement_update`
  - `procurement_advance`
  - `audit_final_approved`
  - `warehouse_complete_precheck`
  - `manual_retry`
  - `legacy_filed_at` (compat)
- Filing state machine in use:
  - `not_filed` -> `pending_filing` -> `filing` -> `filed`
  - failure path: `filing_failed`
- Idempotency rule:
  - same task + same payload hash does not repeat ERP upsert
  - payload change increments `erp_sync_version` and allows next sync
- Read model/API surface added for frontend:
  - `filing_status`, `filing_error_message`, `missing_fields`, `missing_fields_summary_cn`, `last_filed_at`, `erp_sync_required`
  - `GET /v1/tasks/{id}/filing-status`
  - `POST /v1/tasks/{id}/filing/retry`
- Delivery boundary for this handover note:
  - `Design Target`: done
  - `Code Implemented`: done
  - `Server Verified`: local compile/test level done
  - `Live Effective`: not declared (requires deploy + online evidence)

## 2026-03-19 reference_images limit hotfix
- Root cause: `reference_images` direct-create payloads were still serialized into `task_details.reference_images_json`. When an oversized base64 payload slipped past the early guard, MySQL could still fail inside create tx with `Data too long for column 'reference_images_json'`, which surfaced as `internal error during create task tx`.
- Backend rule is now unified to `<= 3MB` per image and max `3` images. The old `200KB / 512KB / 5 images` rule is retired.
- Validation now happens before create tx in both handler and service, and the final `reference_images_json` serialization path re-checks limits before DB write.
- Error contract is stable: `400 INVALID_REQUEST`, `message=reference_images exceed upload limit`, and details include `actual_count`, `max_count`, `max_single_bytes`, `oversized_indexes`, `suggestion=use asset-center upload / reference_file_refs`.
- Storage guard was aligned by adding migration `044_v7_reference_images_mediumtext.sql` to change `task_details.reference_images_json` from `TEXT` to `MEDIUMTEXT`.
- Frontend guidance: keep `reference_images` only for small images within the limit; larger files or more images should go through asset-center and be passed as `reference_file_refs`.

## 2026-03-19 v0.8 Overwrite Hotfix
- This turn did not introduce a new release line. It overwrote the existing `v0.8` release binaries in place.
- Scope of overwrite:
- `releases/v0.8/ecommerce-api`
- `releases/v0.8/erp_bridge`
- Scope explicitly not changed:
- `erp_bridge_sync` / port `8082` binary was not replaced.
- Live verification completed after overwrite:
- 8080 PID now points to `releases/v0.8/ecommerce-api`.
- 8081 PID now points to `releases/v0.8/erp_bridge`.
- 8082 was recovered after the existing `stop-bridge.sh` pattern also stopped sync, but it still runs the unchanged `/root/ecommerce_ai/erp_bridge_sync` binary.
- Original-product create-chain hotfix is live on `v0.8`:
- `design_requirement` alias accepted on original create.
- `is_outsource` alias accepted on original create.
- `product_selection.defer_local_product_binding=true` accepted with ERP snapshot create path.
- illegal original-task fields still fail with machine-readable `invalid_fields` and `violations`.

## v0.8 — 商品主数据 live 真相源切换完成版（2026-03-18）

**v0.8 = 商品主数据 live 真相源切换完成版**。已实证事实：
- 8081 `remote_ok`、`fallback_used=false` 已实证
- 8080 `ERP_SYNC_SOURCE_MODE=jst` 已实证
- `JSTOpenWebProductProvider` 已实证
- products 从 20 增至 7470 已实证
- HQT21413 样本已完成刷新与副本标记写入

### 下一阶段优先级（详见 `docs/NEXT_PHASE_ROADMAP.md`）

1. **第一优先级**：原品开发 / 商品 / 成本联调闭环 — original_product_development defer/非 defer、product-info/cost-info、filing/upsert、前端详情与读模型一致性
2. **第二优先级**：设计资产中心闭环 — download/version-download、delivery 推审、真实上传下载联调
3. **第三优先级**：版本口径统一 — 将 v0.5 命名文档收成 v0.8 或统一版本

---

## ITERATION_082 — ERP 商品主数据四层职责（2026-03-18）

### 架构一句话
选品搜索主链在 **8081 OpenWeb**（local/fallback 仅兜底）；**8080 `products`** 是副本/缓存/承接表（非搜索唯一真相源）；**8082 `jst_inventory`** 是同步驻留/证据层（勿当前台搜索源）；**业务分类主语义 = 款式编码（i_id）**；全局品类用 **`/v1/categories`**（本地可配置映射层，当前含样例数据），勿扫库存大表。详见 `docs/TRUTH_SOURCE_ALIGNMENT.md`。

### 实现锚点
- Hybrid 回退与日志：`service/erp_bridge_remote_client.go`（`erp_bridge_product_search`、`erp_bridge_product_by_id`）。
- 8081 启动校验：`cmd/server/main.go`（remote/hybrid + openweb）。
- Trace 进上下文：`domain/context_trace.go`、`transport/http.go`。
- defer 详情 `product` 合成：`service/task_detail_service.go`。
- JST 同步 spec：`service/erp_sync_jst_provider.go`。

### 部署后必跑
- `docs/ERP_REAL_LINK_VERIFICATION.md`（A/B/C/D）。

## ITERATION_081A — 原品开发创建任务 500（ERP 绑定解析）专项修复（2026-03-18）

### 关键结论
- 本次 `original_product_development` 的 500 已确认不是 ERP 选择映射空值直接触发，而是 `task_details` 插入 SQL 占位符数量错误导致的事务失败。
- 同时补齐了原品开发商品绑定归一链路，`product_id=null` 场景下可从 ERP 选择或 SKU 回退解析本地 `product_id`，避免空绑定导致后续异常。

### 线上证据
- 历史报错 trace：`9837a36e-ebaa-4262-9a91-6bc1ff3d7a47`（`POST /v1/tasks` -> 500）。
- 复现后日志明确错误：
  - `create task: insert task_detail: Error 1136 (21S01): Column count doesn't match value count at row 1`
- 绑定归一日志（新增）可见解析路径：
  - `binding_path=product_selection.erp_product.product_id`（Case A）
  - `binding_path=top.sku_code`（Case B）

### 修复点
- `repo/mysql/task.go`
  - `INSERT INTO task_details` 占位符由 55 补齐为 57，与列数一致。
- `transport/handler/task.go`
  - 创建入口商品绑定归一优先级：
    - `top.product_id`
    - `product_selection.erp_product.product_id`
    - `product_selection.erp_product.sku_code`
    - `top.sku_code`
  - 新增绑定路径日志与失败日志（含 trace_id/task_type/关键绑定字段）。
- `service/erp_bridge_service.go`
  - `EnsureLocalProduct` 绑定键回退新增 `sku_code`。
- `service/task_service.go`
  - 增加 `create_task_tx_failed` 事务失败日志，便于后续排障追溯。

### 复测（生产）
- Case A：`product_id=null` + `product_selection.erp_product.product_id/sku_code` -> `201`，本地 `product_id=485`，任务创建成功。
- Case B：`product_id=null` + 仅 `sku_code` -> `201`，本地 `product_id=485`，任务创建成功。
- 结论：该问题链路下 500 已消除。

## ITERATION_081 — 最小阻塞修复 + 重部署 + 服务器复验（2026-03-18）

### 核心结论（先看这个）
- 当前五模块**已可收口**（按本轮指定阻塞项）。
- 阻塞根因已确认为“线上运行二进制未对齐当前路由/逻辑实现”，并已通过 v0.6 重部署修复。

### 已拿到的硬证据
- 三服务运行与健康：8080/8081/8082 全在线，health=200。
- MAIN 新进程：`PID=3777316`，`/proc/3777316/exe -> /root/ecommerce_ai/releases/v0.6/ecommerce-api`。
- DB：041/042/043 对应列已补齐并复核。
- 三类任务创建（original/new/purchase）均已实测 `201`。
- 资产中心文件流已实测：reference/source/伪PSD/delivery 的 create-session + complete 均成功；source/PSD `preview_available=false`、`source_controlled` 语义成立；历史版本（v1/v2）可见。
- OpenAPI：`docs/api/openapi.yaml` 已修复语法断点并可全量 parse，且补齐批量与资产下载路径 + 缺失 schema 名称。
- 本轮阻塞复验：
  - `POST /v1/tasks/batch/remind` -> `200`
  - `POST /v1/tasks/batch/assign` -> 命中批量 handler（`200` + items；空 body 返回 `batchAssignTaskReq` 校验错误而非 `invalid task id`）
  - `GET/PATCH /v1/tasks/{id}/product-info`、`GET/PATCH /v1/tasks/{id}/cost-info` -> `200`
  - `POST /v1/tasks/{id}/cost-quote/preview` -> `400`（前置条件缺失，非路由 404）
  - 资产 download/version-download -> `200`
  - delivery complete 后任务状态从 `PendingAssign` 推进至 `PendingAuditA`

### 本轮最小修复（已执行）
- 为解决“补列后创建任务 500”，在服务器上做了兼容性 DDL 调整（不改业务语义）：
  - `task_details.filing_error_message` -> `TEXT NULL`
  - `task_details.note` -> `TEXT NULL`
  - `task_details.reference_file_refs_json` -> `TEXT NULL`
- 修复后 `POST /v1/tasks` 已恢复可用并复验通过。

### 下一轮关注（非本轮阻塞）
1. 排查部分历史任务在 `GET /v1/tasks/{id}` / `product-info` / `cost-info` 上的 `500`（读模型历史脏数据兼容）。
2. DataScopeResolver 的“范围裁剪差异”继续补证（当前已验证角色门禁）。

## ITERATION_080 — v0.5 已发布 (2026-03-17)

### 真实发布事实
- **发布目标机**：`223.4.249.11`
- **发布目录**：`/root/ecommerce_ai/releases/v0.5`
- **当前线上版本**：v0.5
- **三服务运行状态**：
  - MAIN：端口 8080，PID 3589336，二进制 `/root/ecommerce_ai/releases/v0.5/ecommerce-api`，health=200
  - Bridge：端口 8081，PID 3589373，二进制 `/root/ecommerce_ai/releases/v0.5/erp_bridge`，health=200
  - Sync：端口 8082，PID 3589421，二进制名 `erp_bridge_sync`，health=200

### 部署顺序（必须遵守）
1. **Migration 038**（v0.5 启动前置条件）：`users` 表新增 `jst_u_id`、`jst_raw_snapshot_json`
2. **Migration 039**：`rule_templates` 表 + 种子数据
3. **Migration 040**：`server_logs` 表
4. **deploy**：部署 v0.5 二进制与配置
5. **三服务健康检查**：8080/8081/8082 health=200
6. **API 验收**：登录、/me、rule-templates、server-logs、tasks 等

**不能只执行 039/040 而漏掉 038**。漏跑 038 会导致 MAIN 启动时查询不存在的 `users.jst_u_id`/`jst_raw_snapshot_json` 而立即退出（本次部署已遇到并靠补齐 038 修复）。

### 生产入口与弃用说明
- **Canonical production 入口**：`cmd/server`（构建 ecommerce-api / erp_bridge / erp_bridge_sync）
- **已弃用**：`cmd/api` 不作为当前发布入口，不要用于 v0.5 部署

### 已完成（代码与文档）
- 任务创建：支持 assignee_id、reference_file_refs、note、need_outsource、requester_id；创建后返回完整 TaskReadModel；传 designer_id 时任务直接进入 InProgress，设置 current_handler_id
- 任务详情：enrichTaskReadModelDetail 填充 assignee_id、assignee_name、design_requirement、note、reference_file_refs、creator_name
- 审核流程：delivery 上传完成后自动进入 PendingAuditA；submit-design 后进入 PendingAuditA；409/400 状态机收口
- 规则及模板：主菜单改名；rule_templates 表 + GET/PUT API（cost-pricing, product-code, short-name）
- 组织与权限：user_admin/org_admin/role_admin/logs_center 归入同一 section；org_admin→组织结构，role_admin→角色管理
- 参考图：validateReferenceImages 已统一为单张 `<= 3MB`、最多 `3` 张；超限会在 create tx 前返回 `400 INVALID_REQUEST`，并提示走 asset-center / `reference_file_refs`
- 服务器日志：server_logs 表；GET /v1/server-logs、POST /v1/server-logs/clean；5xx 自动入库；脱敏
- Migrations：**038**（users 扩展）、**039** rule_templates、**040** server_logs
- 文档：openapi.yaml、FRONTEND_ALIGNMENT_v0.5.md 已收口为 v0.5 联调基准

---

## 收口补丁执行版进展（2026-03-18）

本轮已完成并通过全量 `go test ./...`：

- per-task 商品/成本接口 5 个端点：
  - `GET/PATCH /v1/tasks/{id}/product-info`
  - `GET/PATCH /v1/tasks/{id}/cost-info`
  - `POST /v1/tasks/{id}/cost-quote/preview`
- `missing_fields` 双轨输出：`missing_fields` + `missing_fields_summary_cn` 同源计算
- 资产下载结构化响应与预览约束：
  - 下载接口返回 `AssetDownloadInfo`
  - PSD/source 预览受 `preview_available + download_mode` 共同约束
- 批量催办事件：`task.reminded` payload 含 `batch_request_id`
- `filing_status` 状态机正式化（含 `filing_error_message` 与历史 `filed_at` 兼容）
- `note/reference_file_refs` 独立字段与读取回退逻辑
- DataScope 最小闭环：
  - `DataScopeResolver` 接口 + 角色推导实现
  - task list / board 查询注入可见性裁剪
  - task detail 增加可见性校验
- `TaskListItem` 关键业务字段已从 `json:"-"` 暴露为前端可读字段
- 新增 migration：
  - `041_v7_task_filing_status.sql`
  - `042_v7_task_note_reference_file_refs.sql`
  - `043_v7_task_cost_price_source.sql`

## ITERATION_079 — JST 用户同步预埋 (2026-03-17)
- 预埋能力：查询 JST 商家用户、手动导入；不改变主业务用户/权限/登录逻辑。
- Bridge 新增：`GET /v1/erp/users` -> `/open/webapi/userapi/company/getcompanyusers`
- MAIN Admin：`GET /v1/admin/jst-users`、`POST /v1/admin/jst-users/import-preview`、`POST /v1/admin/jst-users/import`
- 本地 users：`jst_u_id`、`jst_raw_snapshot_json`（migration 038）
- 导入：jst_u_id > loginId > username；新建 disabled + 随机密码；角色默认不写入
- JST 仅作数据源，不接管鉴权

## ITERATION_078 — Bridge Semantic Alignment + Route Rollout (2026-03-17)
- Scope focused on real-business semantic correction and live acceptance:
  - align `sku_id / i_id / name / short_name / wms_co_id`
  - enable both write paths (`upsert` + `item_style_update`)
  - expose 11-warehouse `wms_co_id` contract route
  - keep `v0.4`/`8080/8081/8082` stable with no version bump

### Confirmed facts
- Rebuilt and redeployed with `--version v0.4`; runtime check passed.
- Current live process state:
  - MAIN(8080) pid `3546054`, health `200`
  - Bridge(8081) pid `3546082`, health `200`
  - Sync(8082) pid `3546261`, health `200`
  - no deleted executable for active pids
- Post-deploy acceptance confirms route availability:
  - `POST /v1/erp/products/style/update` is live (`200` Admin/Ops, `403` roleless)
  - `GET /v1/erp/warehouses` is live (`200` Admin/Ops, `403` roleless)
- Upsert/style responses now carry aligned fields and explicit route marker:
  - upsert returns `sku_id/i_id/name/short_name/s_price/wms_co_id`, `route=itemskubatchupload`
  - style update returns `sku_id/i_id/name/short_name`, `route=itemupload`
- Warehouse contract is live with full 11 records (`wms_co_id` complete list).
- Shelve/unshelve/virtual-qty writes remain stable under hybrid fallback:
  - response keeps `status=accepted`, `message=stored locally`, with `sync_log_id`
  - sync log payload includes `wms_co_id`, and for shelf operations includes `bin_id/carry_id/box_no`
- Role policy remains clear and enforced:
  - ERP read routes: role-scoped
  - ERP write routes: `Ops/Warehouse/ERP/Admin`
  - roleless blocked on read/write

### Inferences
- Bridge contract surface is now sufficient for MAIN follow-up ERP growth without needing another adapter-contract rewrite.
- Remaining remote closure risk is upstream business-context sufficiency, not missing route/contract implementation.
- `hybrid + fallback` remains the correct live posture.

### Recommended next actions
1. Re-run remote acceptance for shelve/unshelve with upstream-confirmed valid slot/container context.
2. Re-run virtual-qty with upstream-confirmed minimal accepted payload template.
3. Keep documenting remote evidence per endpoint as `confirmed_facts` before any "fully connected" conclusion.

## ITERATION_077 — 8081 Bridge Remaining Write Acceptance (2026-03-17)
- Scope strictly focused on the three remaining Bridge writes (`shelve/unshelve/virtual-qty`) under OpenWeb `remote/hybrid`.
- Shared entrypoint remains `cmd/server/main.go` (8080/8081 common binary path), no version bump.

### Confirmed facts
- 8081 remote client now uses OpenWeb signing/credential rules:
  - `sign = md5(app_secret + sorted(key+value))`
  - signed keys: `app_key/access_token/timestamp/charset/version/biz`
  - request content-type: `application/x-www-form-urlencoded;charset=utf-8`
- Official interface mappings are now explicit in code and docs:
  - upsert -> `/open/webapi/itemapi/itemsku/itemskubatchupload`
  - shelve batch -> `/open/webapi/wmsapi/openshelve/skubatchshelve`
  - unshelve batch -> `/open/webapi/wmsapi/openoffshelve/skubatchoffshelve`
  - virtual qty -> `/open/webapi/itemapi/iteminventory/batchupdatewmsvirtualqtys`
- Live bridge env switched to `hybrid` and OpenWeb credentials are configured.
- Real remote hit evidence exists:
  - bridge log contains `remote_erp_openweb_request_completed`
  - URL confirms official target `https://openapi.jushuitan.com/open/webapi/itemapi/itemsku/itemskubatchupload`
  - status_code `200`
- Real remote-fail + fallback evidence exists for all remaining writes:
  - shelve batch:
    - official URL hit: `/open/webapi/wmsapi/openshelve/skubatchshelve`
    - upstream response: `code=100`, `msg=上架仓位不能为空`
    - fallback chain: `erp_remote_shelve_batch_failed_fallback_local` -> `erp_remote_shelve_batch_fallback_local_success`
  - unshelve batch:
    - official URL hit: `/open/webapi/wmsapi/openoffshelve/skubatchoffshelve`
    - upstream response: `code=100`, `msg=指定箱不存在`
    - fallback chain: `erp_remote_unshelve_batch_failed_fallback_local` -> `erp_remote_unshelve_batch_fallback_local_success`
  - virtual qty:
    - official URL hit: `/open/webapi/itemapi/iteminventory/batchupdatewmsvirtualqtys`
    - upstream raw body: `code=0`, `msg=未获取到有效的传入数据`, `data=null`
    - bridge now classifies this as business reject and triggers:
      - `erp_remote_virtual_inventory_failed_fallback_local` -> `erp_remote_virtual_inventory_fallback_local_success`
- Observability enhanced:
  - `remote_erp_openweb_request_started` log marker added before outbound call.
- Permission regression after this change remains correct:
  - Admin and Ops pass ERP read/write (`200`)
  - roleless blocked on ERP read/write (`403`)
- Runtime safety check passed post rollout:
  - `8080/8081/8082` all healthy
  - no deleted executable for active pids
  - current pids: MAIN `3527927`, Bridge `3527959`, Sync `3528131`

### Inferences
- Upsert remote writeback path is officially reachable and validated via bridge-initiated traffic.
- Remaining write blockers are now narrowed to upstream business constraints (required warehouse slot/container/valid inventory payload semantics), not remote wiring/signature reachability.
- Keeping live in `hybrid` mode is currently the safest production posture.

### Recommended next actions
1. With business/integration side, confirm and provide valid upstream context fields:
   - shelve: required shelf location context for `skubatchshelve`
   - unshelve: valid box/container context for `skubatchoffshelve`
   - virtual qty: exact minimal accepted payload for `batchupdatewmsvirtualqtys`
2. Re-run real acceptance on those three write routes with current hybrid mode and preserve log evidence (`started/completed/business_error/fallback`).
3. Keep fallback enabled until upstream acceptance is confirmed by real success responses.

> 注：下方章节包含历史迭代记录（如 ITERATION_075 的阶段性结论），当前状态以本文件顶部 ITERATION_077 为准。

## v0.4 Bridge Remote Acceptance + ERP Permission Grading (2026-03-17)
- Round objective was strictly narrowed to:
  1) external ERP remote writeback acceptance feasibility on live `8081`
  2) multi-role runtime verification for `/v1/erp/*`
- External ERP acceptance result:
  - live bridge remains `ERP_REMOTE_MODE=local`
  - `ERP_REMOTE_BASE_URL` is still empty
  - remote auth/sign credentials are still empty (`ERP_REMOTE_AUTH_MODE=none`, token/app_key/app_secret/access_token unset)
  - therefore no safe switch to live `hybrid/remote` in this round
  - conclusion: external ERP formal writeback is still **not accepted** (no real upstream response evidence)
- Permission verification result:
  - real `Admin` and real `Ops` sessions were verified against all listed `/v1/erp/*` routes on live `8081`; all returned `200`
  - pre-fix probe confirmed roleless valid session could still call ERP read/write routes (`200`) -> policy too open
- Minimal fix applied and deployed in-place to `v0.4`:
  - file: `transport/http.go` only
  - read routes now require one of `Ops/Designer/Audit_A/Audit_B/Warehouse/Outsource/ERP/Admin`
  - write routes now require one of `Ops/Warehouse/ERP/Admin`
  - sync-log routes now require one of `Ops/Warehouse/ERP/Admin`
- Post-fix runtime proof:
  - `Admin` + `Ops` still pass all required ERP routes (`200`)
  - roleless valid session now receives `403` on tested ERP read/write routes
- Deploy/runtime safety after fix:
  - deployed with `--version v0.4` (no version bump)
  - `8080/8081/8082` all healthy
  - current pids: MAIN `3507849`, Bridge `3507876`, Sync `3508034`
  - `/proc/<pid>/exe` not deleted for all three services
- Recommended next action (blocked external dependency):
  1) obtain external ERP formal `base_url`
  2) obtain auth/sign credentials and confirmed timestamp/nonce/signature contract
  3) verify whitelist/network reachability
  4) then run `hybrid` first and capture remote hit / upstream response / fallback evidence in one acceptance batch

## v0.4 MAIN ERPSyncWorker Verification and Recovery (2026-03-17)
- Scope was MAIN(8080) only: verify the real state of `ERPSyncWorker`, recover only if broken, and clarify MAIN/Bridge/external-ERP boundaries.
- Confirmed facts before fix:
  - old `cmd/api/main.go` 10-minute sync (`*/10 * * * * -> syncSvc.IncrementalSync(10)`) is historical only
  - live owner remains MAIN `ERPSyncWorker` + `/v1/products/sync/*`
  - live `/v1/products/sync/status` returned `scheduler_enabled=true`, `interval_seconds=300`, `source_mode=stub`, latest run `status=noop`
  - authenticated manual run also returned `status=noop`
- Confirmed root cause:
  - live MAIN `/proc/<pid>/cwd = /root`
  - stub path remained relative: `config/erp_products_stub.json`
  - packaged stub file existed only inside release/current config directory
  - result: worker stayed alive and scheduled, but resolved the stub path incorrectly and persisted `noop`
- Fix:
  - updated `deploy/run-with-env.sh` so runtime starts from the resolved binary directory
  - added `resolved_stub_file` + `stub_file_exists` to `ERPSyncStatus`
- Live verification after deploy:
  - new MAIN pid: `3450797`
  - `/proc/3450797/exe -> /root/ecommerce_ai/releases/v0.4/ecommerce-api`
  - `/proc/3450797/cwd -> /root/ecommerce_ai/releases/v0.4`
  - manual `/v1/products/sync/run` now returns `success` with `total_received=2`, `total_upserted=2`
  - first post-deploy scheduled run now also returns `success` (`started_at=2026-03-17T10:58:38+08:00`, `total_upserted=2`)
  - `/v1/products/search` and `/v1/erp/products` both read the recovered stub-backed rows
- Boundary conclusion:
  - MAIN keeps scheduler/cache/runtime ownership
  - Bridge keeps adapter query semantics and mutation execution
  - external ERP formal connectivity is still not confirmed; live Bridge remains `local` mode
  - future recommended path is `MAIN ERPSyncWorker -> Bridge query/export contract -> MAIN products`, not direct MAIN-to-external-ERP coupling
- Recommended first reads for next model on this topic:
  - `docs/iterations/ITERATION_072.md`
  - `deploy/run-with-env.sh`
  - `service/erp_sync_service.go`
  - `workers/erp_sync_worker.go`
  - `service/erp_bridge_local_client.go`
  - `service/erp_bridge_service.go`

## v0.4 Bridge External ERP Remote-Ready (2026-03-17)
- Scope constrained to Bridge(8081): added external-ERP upsert client capability without changing frontend and without changing MAIN(8080) routing contract.
- New runtime switch is now config-driven (`ERP_REMOTE_MODE=local|remote|hybrid`) in `config/config.go`, wired in `cmd/server/main.go`, with new client implementation in `service/erp_bridge_remote_client.go`.
- Remote client now supports: configurable base URL and upsert path, bearer/app-sign auth headers, timeout, retry with backoff, structured logs, and upstream error mapping compatible with existing bridge error envelope.
- Local safety path is retained:
  - `local` mode keeps existing `localERPBridgeClient`
  - `hybrid` mode can fallback to local by `ERP_REMOTE_FALLBACK_LOCAL_ON_ERROR=true`
- Live server verification after bridge-only binary replacement:
  - bridge PID: 3423714
  - `/proc/3423714/exe -> /root/ecommerce_ai/releases/v0.4/erp_bridge` (not deleted)
  - `/health` on 8081 returns 200
  - MAIN -> Bridge upsert remains 200
  - response still `message=stored locally` because live mode is `local`
- External ERP is **not yet confirmed connected** (no real upstream formal API response evidence yet).

## v0.4 ERP Bridge Restoration (2026-03-17)
- ERP Bridge service (port 8081) restored to production and verified running
- Root cause: `POST /v1/erp/products/upsert` route was not registered in router — all MAIN -> Bridge filing calls returned 404
- Fix: added `UpsertProduct` handler to `ERPBridgeHandler` + registered route in `transport/http.go`
- MAIN -> Bridge -> DB writeback chain verified with authenticated requests (status 200)
- Full blackbox flow tested: login -> task create (with ERP product selection) -> business info update (with `filed_at`) -> Bridge upsert succeeds
- Non-blocking fallback retained as safety net; it no longer triggers under normal operation
- Files changed: `transport/handler/erp_bridge.go`, `transport/http.go`
- See `MODEL_v0.4_memory.md` section 10 for deployment verification checklist

## v0.4 Closure (2026-03-16)
- 3-round blackbox E2E testing completed: all 3 task types pass full lifecycle (create -> design -> audit -> warehouse -> close)
- ERP filing made non-blocking: `performERPBridgeFiling` UpsertProduct errors no longer block business-info updates
- TaskListItem augmented with `owner_team`, `priority`, `created_at`, `is_outsource`
- Keyword search extended to include task ID
- See `MODEL_v0.4_memory.md` for comprehensive handover state

## Main Flow E2E Readiness (2026-03-14)
- all mainline flows verified as code-complete and integration-ready
- auth (register with admin_key, login, /me, change password): READY
- permission management (user CRUD, role assignment): READY
- logs (permission-logs, operation-logs): READY
- task flow (create, list, detail, board, assign): READY
- task create rules v0.4 update (2026-03-14):
  - three task types now have formalized creation rules with field-level validation
  - `owner_team` (所属组) is now **required** for all three task types (original, new, purchase)
  - `due_at` (任务截止时间) is now **required** for all three task types
  - `owner_team` validation enforces against configured teams from auth_identity.json
  - original_product_development: requires product_id + change_request + owner_team + due_at
  - new_product_development: requires category_code + material_mode + product_name + product_short_name + design_requirement + owner_team + due_at
  - purchase_task: requires purchase_sku + product_name + cost_price_mode + quantity + base_sale_price + owner_team + due_at
  - material_mode supports preset/other with conditional material/material_other validation
  - cost_price_mode supports manual/template with conditional cost_price validation
  - source_mode is now auto-inferred from task_type when omitted
  - priority defaults to low (was normal)
  - reference docs: docs/TASK_CREATE_RULES.md
- upload (reference/delivery/source via NAS upload service): READY
- audit (claim, approve, reject, transfer, handover, takeover): READY
- warehouse (receive, reject, complete, receipt list): READY
- asset access policy: LAN/Tailscale/public URLs now populated in DesignAssetVersion responses
- register admin_key supported: yes (admin_key or secret_key field)
- department admin registration via key: yes (dept_admin role assigned)
- super admin via config only: yes (auth_identity.json super_admins)
- reference docs: docs/FRONTEND_MAIN_FLOW_CHECKLIST.md, docs/ASSET_ACCESS_POLICY.md

## Current Stage Development Priority Decision
- this is the current-stage development priority decision, not a temporary verbal note
- current priority order is now:
  - mainline feature development first
  - integration, verification, release, and deployment first
  - compatibility retirement / legacy retirement / architecture-cleanup work no longer leads short-term execution
- retirement work is now expected to continue only in:
  - engineering review windows
  - version close-out windows
  - post-release governance windows
- existing v0.4 governance documents remain valid baselines:
  - `docs/MAIN_BRIDGE_RESPONSIBILITY_MATRIX.md`
  - `docs/V0_4_MAIN_BRIDGE_CONVERGENCE_PLAN.md`
  - `docs/V0_4_COMPATIBILITY_RETIREMENT_CHECKLIST.md`
- short-term delivery should not use retirement as the mainline development track

## v0.4 Convergence Close-out
- completed in v0.4 so far:
  - production MAIN is locked to `./cmd/server`
  - `cmd/api` is demoted from production use and kept only as a compatibility remnant
  - MAIN is now the explicit public business application service for v0.4
  - Bridge is now the explicit ERP/JST adapter and mutation boundary for v0.4
  - legacy compatibility surfaces are explicitly classified and bounded
  - sync/runtime terminology is aligned so MAIN still owns the current sync/runtime continuity needed for the smallest safe convergence path
  - post-v0.4 retirement sequencing now lives in `docs/V0_4_COMPATIBILITY_RETIREMENT_CHECKLIST.md`
- not yet retired or removed:
  - compatibility routes and legacy remnants are still present where rollback-safe continuity requires them
  - no broad compatibility-surface deletion has been performed in v0.4
  - same-host Bridge loopback and candidate-MAIN validation remain the documented runtime continuity model
- next likely focus after v0.4:
  - mainline feature delivery, integration, verification, release, and deployment
  - retirement work only during review / close-out / governance windows after mainline needs are covered
  - no new architecture track is implied by this handover

## Current Architecture Direction
- Keep V6 infra core
- Add V7 Task-centric business layer aligned to PRD V2.0 mainline:
  - `Task` is the only business root
  - task types are `original_product_development`, `new_product_development`, `purchase_task`
  - Step 69 formalizes canonical design-asset business semantics inside the existing `Task -> Asset -> AssetVersion` model:
    - `reference` = task-creation / design-reference small files; they default to small upload and never become the warehouse-ready truth
    - `source` = PSD / PSB / AI / editable originals; they are controlled-access metadata objects, not default public-download files
    - `delivery` = JPG / PNG / formal audit + warehouse circulation image; this is the current business-truth flow asset
    - `preview` = auxiliary preview only; it must not replace `delivery` as formal truth
    - automatic PSD/AI conversion remains optional preview assistance only and is not the primary business path
    - asset-center payloads now expose `current_version`, `approved_version`, and `warehouse_ready_version` directly
    - warehouse-facing read models should prioritize `warehouse_ready_version`
    - source-file payloads must keep locating metadata visible even if browser-side public download is disabled:
      - `task_no`
      - `asset_no`
      - `version_no`
      - `original_filename`
      - `uploaded_by`
      - `uploaded_at`
      - `storage_key`
      - `source_access_mode`
      - `lan_url`
      - `tailscale_url`
      - `public_download_allowed`
      - `access_hint`
  - Step 68 formalizes MAIN <-> NAS Upload Service responsibility separation:
    - MAIN = task business ownership + asset/version/upload-session metadata + audit/logging + frontend aggregation
    - NAS Upload Service = small-file upload, multipart session allocation, part-upload orchestration, complete/abort, physical NAS storage, and file metadata
    - primary frontend asset-center API surface is now `/v1/tasks/{id}/assets/*`
    - legacy `/v1/tasks/{id}/asset-center/*` routes remain compatibility aliases only
    - recommended frontend flow is:
      - call MAIN `POST /v1/tasks/{id}/assets/upload-sessions`
      - upload bytes directly to NAS Upload Service using returned remote instructions
      - call MAIN complete/abort so business metadata and audit traces are persisted
    - `task_assets` remain the append-only asset-version persistence layer for the task business timeline
    - `design_assets` remain the task-scoped asset roots/current-version pointers
    - `upload_requests` remain the MAIN-side upload-session business view and remote-session correlation table
    - preview fields stay placeholder-only for later PSD/AI preview pipeline work
  - Real integration verification status as of `2026-03-14`:
    - live NAS endpoint in use: `http://100.111.214.38:8089`
    - live MAIN runtime now reads:
      - `UPLOAD_SERVICE_ENABLED=true`
      - `UPLOAD_STORAGE_PROVIDER=nas`
      - `UPLOAD_SERVICE_BASE_URL=http://100.111.214.38:8089`
      - `UPLOAD_SERVICE_INTERNAL_TOKEN=nas-upload-token-2026`
      - `UPLOAD_SERVICE_TIMEOUT=30s`
    - real verified flows:
      - `reference` small upload
      - `delivery` small upload
      - `source` multipart upload
    - real verified business closure:
      - asset/version persistence
      - `current_version` pointer update
      - `approved_version` / `warehouse_ready_version` derivation for `delivery` on an approved warehouse-stage task
      - task event / operation-log persistence
    - frontend minimum integration order should be:
      - `reference` small upload first
      - `delivery` small upload second
      - `source` multipart third
    - `preview` remains contract-only / auxiliary semantics and is still not a real upload-production path
  - Step 67 adds the MAIN-side design asset upload-center preparation layer without moving real file transfer into MAIN:
    - new task-scoped `design_assets` roots define the business asset boundary
    - existing `task_assets` now also carry additive asset-version fields and remain compatible with the older task timeline contract
    - existing `upload_requests` now also act as persisted upload-session records for the new asset-center APIs
    - frontend-facing asset-center routes now exist for asset list, version list, upload-session create/read/complete/cancel
    - a dedicated upload-service client abstraction now isolates later NAS-side Go service integration from task business logic
    - MAIN still does not receive bytes, merge multipart chunks, mount NAS, or generate PSD/preview artifacts in this step
  - Step 62 hardens the Linux/bash deployment workflow for non-disruptive side-by-side validation without widening product scope:
    - `deploy/deploy.sh --parallel` now performs a candidate-only deployment path
    - candidate releases still unpack under `/root/ecommerce_ai/releases/<version>`
    - parallel mode does not switch:
      - `/root/ecommerce_ai/current`
      - `/root/ecommerce_ai/ecommerce-api`
      - `/root/ecommerce_ai/erp_bridge`
    - parallel mode does not stop live MAIN or live Bridge
    - parallel mode does not overwrite live shared env files in place
    - candidate MAIN now starts from the version directory with isolated:
      - env file
      - pid file
      - log file
      - deploy state file
      - verification port, default `18080`
    - candidate Bridge dependency stays fixed at same-host loopback `http://127.0.0.1:8081`
    - cutover remains a separate concern from side-by-side validation
  - Deploy SSH key upgrade: deployment now defaults to `DEPLOY_AUTH_MODE=key` with batch-mode SSH/SCP. Run `deploy/setup-ssh-key.ps1` from Windows IDE / PowerShell or `deploy/setup-ssh-key.sh` from bash to authorize `~/.ssh/id_deploy_ecommerce`; `deploy.sh` then runs without `DEPLOY_PASSWORD` or `sshpass`. `DEPLOY_AUTH_MODE=password` + `DEPLOY_PASSWORD` remains a compatibility fallback.
  - Step 61 standardizes deployment packaging and release bookkeeping on top of the completed mainline without widening product scope:
    - managed deployment entrypoint is now `deploy/deploy.sh`
    - managed release history source of truth is now `deploy/release-history.log`
    - deploy defaults to SSH key passwordless; one-time setup via `deploy/setup-ssh-key.ps1` (Windows IDE / PowerShell) or `deploy/setup-ssh-key.sh` (bash); `DEPLOY_AUTH_MODE=password` + `DEPLOY_PASSWORD` retained as explicit fallback
    - baseline managed deployment version is `v0.1`
    - future package/deploy versions auto-increment by minor step from that file instead of relying on git tags
    - packages now include remote deploy/start/stop/verify bash helpers for the Linux host layout
    - remote layout now assumes:
      - `/root/ecommerce_ai/releases/<version>`
      - `/root/ecommerce_ai/shared/main.env`
      - `/root/ecommerce_ai/shared/bridge.env`
      - `/root/ecommerce_ai/logs`
      - `/root/ecommerce_ai/scripts`
      - stable runtime symlinks `/root/ecommerce_ai/ecommerce-api` and `/root/ecommerce_ai/erp_bridge`
    - Bridge runtime default remains same-host loopback `http://127.0.0.1:8081`
    - this is still not a CI/CD rollout, service-supervision redesign, or infrastructure-platform expansion
  - Step 60 is a narrow integration/package-readiness pass on top of the completed Step A-E mainline:
    - no new platform feature was added
    - live Bridge probes against `223.4.249.11` still returned empty HTTP replies from this environment, so public-IP ingress remains a runtime concern to verify on-host
    - default `ERP_BRIDGE_BASE_URL` now assumes same-host deployment and points to `http://127.0.0.1:8081`
    - explicit packaging/deploy assets now live in `deploy/package-local.sh`, `deploy/main.env.example`, and `deploy/LOCAL_PACKAGE_DEPLOY.md`
  - Step 59 closes the next narrow Step E slice around audit/warehouse/logging instead of widening platform scaffolding:
    - `task.created` now starts the task event stream at entry
    - key task events across create / assign / submit-design / audit / procurement / warehouse / close now carry richer before/after task-status, handler, and result context
    - `submit-design` now clears stale designer ownership before audit and also supports re-entry from `RejectedByAuditB`
    - audit approve clears current handler for the next stage
    - audit reject routes back to designer ownership for truthful rework
    - audit handover clears current handler and blocks further audit actions until explicit takeover
    - warehouse receive can reuse a previously rejected receipt after task re-prepare and sets current handler to the receiver
    - warehouse reject no longer collapses all tasks into generic `Blocked`:
      - `purchase_task` returns to `PendingAssign`
      - design/audit tasks return to `RejectedByAuditB` with designer ownership for rework
    - warehouse complete clears current handler while moving the task to `PendingClose`
    - procurement-to-warehouse coordination must treat rejected warehouse receipts as unresolved/rework state rather than always as active handoff
  - Step 58 narrows task entry around explicit task-type/source-mode rules instead of one vague create contract:
    - `original_product_development` stays `existing_product` only
    - `new_product_development` stays `new_product` only
    - `purchase_task` may start from either source mode without inheriting design/audit assumptions at entry
  - SKU ownership at task entry is now source-mode driven:
    - `existing_product` binds SKU from the selected product
    - `new_product` auto-generates SKU from the enabled `new_sku` code rule when omitted
  - `purchase_task` create now initializes a draft `procurement_records` row so read/list/detail models expose procurement state immediately after entry
  - create-task validation now returns additive machine-readable `error.details.violations` for entry-contract mismatches
  - warehouse completion and close are now split:
    - warehouse completion writes explicit `PendingClose`
    - standalone close writes `Completed`
  - close/warehouse readiness must use structured machine-readable reasons: `{ code, message }`
  - `workflow.sub_status` is now a structured contract: `{ code, label, source }`
  - `/v1/tasks` now supports converged board/list filtering over projected `main_status`, structured `sub_status_code` / `sub_status_scope`, procurement coordination, and warehouse readiness-related fields
  - purchase preparation now persists in `procurement_records`, exposed via nullable `procurement` plus frontend-friendly `procurement_summary`
  - procurement lifecycle skeleton is now:
    - `draft`
    - `prepared`
    - `in_progress`
    - `completed`
  - purchase-task warehouse prepare now requires procurement completion
  - `procurement_summary` now carries derived coordination semantics such as awaiting-arrival, ready-for-warehouse, and handed-to-warehouse
  - procurement lifecycle transitions use `POST /v1/tasks/{id}/procurement/advance`
  - task-board / inbox aggregation is now available through:
    - `GET /v1/task-board/summary`
    - `GET /v1/task-board/queues`
  - board queues are derived from `workflow.main_status`, `workflow.sub_status`, and `procurement_summary.coordination_status`
  - task-board queue contracts now expose `normalized_filters` and `query_template` so frontend can drill into `/v1/tasks` without rebuilding queue conditions
  - converged `/v1/tasks` filters now resolve through direct repo/read-model predicates for projected workflow, procurement coordination, warehouse readiness, and warehouse blocking-reason matching instead of service-layer segmented fan-out
  - task-board summary and task-board queues now aggregate from one shared board-level candidate pool per request instead of preset-by-preset list fan-out
  - board summary, board queues, and `/v1/tasks` now share a tighter aggregation relationship:
    - repo/read-model predicates constrain the board candidate pool
    - preset queues reuse the same filter matcher semantics for final partitioning
  - broad board candidate scans now use a dedicated repo/read-model board candidate scan entry instead of paging through the generic task-list path
  - board candidate scans now push down the union of selected preset queue predicates before service-level partitioning
  - remaining task-board fan-out is now intentionally limited to business-required final queue shaping rather than broad candidate collection
  - Step 18 now classifies the remaining candidate-scan hotspots instead of jumping straight into indexes or materialized projections
  - the heaviest remaining scan-side predicates are:
    - unscoped `sub_status_code`
    - `warehouse_blocking_reason_code`
    - `warehouse_prepare_ready`
  - a light read-model optimization now joins one latest task-asset projection per task instead of repeating the latest-asset scalar subquery across list and board candidate scans
  - `main_status` / `coordination_status` remain acceptable derived CASE predicates for now, while `closable` / `cannot_close_reasons` remain per-row projection cost rather than current candidate-scan pushdown hotspots
  - broad index / projection / materialized-view work is still intentionally deferred until later scale validation
  - Step 19 adds lightweight task-board ownership hints:
    - `suggested_roles`
    - `suggested_actor_type`
    - `default_visibility`
    - `ownership_hint`
  - those queue ownership fields are advisory only and do not enforce permissions, claims, assignment, or queue visibility
  - workbench bootstrap / saved preferences are now available through:
    - `GET /v1/workbench/preferences`
    - `PATCH /v1/workbench/preferences`
  - workbench preferences are now session-backed only on the mainline HTTP path
  - admin route-access inspection is now available through:
    - `GET /v1/access-rules`
  - permission access logs now persist route-policy context plus session actor username for Step B auditability
  - Step 20 adds configurable category-center skeleton APIs:
    - `GET /v1/categories`
    - `GET /v1/categories/search`
    - `GET /v1/categories/{id}`
    - `POST /v1/categories`
    - `PATCH /v1/categories/{id}`
  - Step 20 adds configurable cost-rule-center skeleton APIs:
    - `GET /v1/cost-rules`
    - `GET /v1/cost-rules/{id}`
    - `POST /v1/cost-rules`
    - `PATCH /v1/cost-rules/{id}`
    - `POST /v1/cost-rules/preview`
  - coded-style values such as `HBJ/HBZ/HCP/HLZ/HPJ/HQT/HSC/HZS` are now valid first-level category-center entries
  - task business info can now persist structured category linkage and internal cost-rule provenance through:
    - `category_id`
    - `category_code`
    - `category_name`
    - `cost_rule_id`
    - `cost_rule_name`
    - `cost_rule_source`
  - cost-rule preview is intentionally skeleton-only:
    - fixed/threshold/min-area/process rules can estimate now
    - unsupported size-formula cases and `manual_quote` still return manual review
  - Step 21 turns that skeleton into a direct task usage path:
    - `PATCH /v1/tasks/{id}/business-info` now accepts minimal cost-prefill inputs:
      - `width`
      - `height`
      - `area`
      - `quantity`
      - `process`
    - task business info now persists:
      - `estimated_cost`
      - `requires_manual_review`
      - `manual_cost_override`
      - `manual_cost_override_reason`
    - when skeleton preview can estimate, business info now prefills internal `cost_price`
    - `purchase_task` procurement-facing summaries now surface internal cost + provenance signals without moving procurement ownership back into task details
  - Step 22 turns category center into an explicit ERP positioning skeleton:
    - category records now expose:
      - `search_entry_code`
      - `is_search_entry`
    - top-level total category code is now the explicit first-level ERP search entry
    - independent category-to-ERP positioning APIs now exist:
      - `GET /v1/category-mappings`
      - `GET /v1/category-mappings/search`
      - `GET /v1/category-mappings/{id}`
      - `POST /v1/category-mappings`
      - `PATCH /v1/category-mappings/{id}`
    - mapping records now reserve later second/third-level refinement through:
      - `secondary_condition_key`
      - `secondary_condition_value`
      - `tertiary_condition_key`
      - `tertiary_condition_value`
    - real ERP lookup execution is still deferred; this round only defines the positioning skeleton
  - Step 23 turns that positioning skeleton into an executable local product-search layer:
    - `GET /v1/products/search` now accepts:
      - `category_id`
      - `category_code`
      - `search_entry_code`
      - `mapping_match`
      - lightweight reserved `secondary_*` / `tertiary_*` query pairs
    - mapped search now resolves:
      - selected category -> `search_entry_code`
      - active local `category_erp_mappings`
      - local synced `products`
    - mapped search currently prefers exact category mappings and otherwise falls back to search-entry-wide mappings
    - search results now expose:
      - `matched_category_code`
      - `matched_search_entry_code`
      - `matched_mapping_rule`
    - real ERP lookup execution is still deferred; Step 23 is a non-real-time local ERP positioning layer only
  - Step 24 turns that mapped-search contract into a task-entry contract:
    - `POST /v1/tasks` and `PATCH /v1/tasks/{id}/business-info` now accept additive `product_selection`
    - existing-product task selection can now persist:
      - selected product identity
      - selected SKU snapshot
      - `matched_category_code`
      - `matched_search_entry_code`
      - `matched_mapping_rule`
      - `source_match_type`
      - `source_match_rule`
      - `source_search_entry_code`
    - `GET /v1/tasks/{id}` and `GET /v1/tasks/{id}/detail` now expose top-level `product_selection`
    - business-info updates may rebind existing-product tasks while keeping mapped-search provenance
    - real ERP lookup execution is still deferred; Step 24 only completes local picker integration and traceability
  - Step 25 turns `product_selection` into a first-class read-model contract:
    - `GET /v1/tasks` task items now expose lightweight `product_selection` summary
    - `GET /v1/task-board/summary` and `GET /v1/task-board/queues` inherit the same task-item `product_selection` summary contract
    - `procurement_summary` now also carries lightweight `product_selection` summary for stable purchase-facing consumption
    - `GET /v1/tasks/{id}` and `GET /v1/tasks/{id}/detail` keep full `product_selection` provenance with `matched_mapping_rule`
    - frontend should consume read-model provenance directly instead of reconstructing original-product traceability from scattered `matched_*` / `source_*` fields
  - Step 53 connects the real ERP Bridge query surface to the original-product mainline:
    - frontend-ready ERP Bridge query APIs now exist:
      - `GET /v1/erp/products`
      - `GET /v1/erp/products/{id}`
      - `GET /v1/erp/categories`
    - keyword search is now the primary original-product entry because bridge category coverage is currently incomplete for cases such as `定制车缝`
    - bridge categories are auxiliary only and must not block the selection path
    - when frontend submits `product_selection.erp_product`, backend now:
      - caches/binds that ERP Bridge product into local `products`
      - keeps task-side `product_selection` additive
      - persists external product id / sku id / category / image / price snapshot
    - local mapped search remains valid through `GET /v1/products/search`, but it is no longer the recommended first entry for bridge-backed original-product picking
  - Step 54 hardens that ERP Bridge mainline without changing the upstream service:
    - `GET /v1/erp/products` now accepts additive normalized filters:
      - `q`
      - compatibility `keyword`
      - `sku_code`
      - `category_id`
      - `category_name`
      - compatibility `category`
    - `GET /v1/erp/products` now returns additive `normalized_filters`
    - bridge response normalization now tolerates broader envelope/list/detail variants and safely merges duplicate rows
    - bridge failures now surface timeout / retry-hint diagnostics in internal error details
    - bridge requests now emit lightweight duration/status logs for internal observability
    - task-side `product_selection.erp_product` persistence now merges with prior cached snapshot fields and backfills missing non-identity fields from local binding/task context
  - Step 55 hardens Step A actor precedence without expanding into a broader auth redesign:
    - normal request middleware no longer synthesizes the mainline request actor as `system_fallback` user `1`
    - `GET /v1/auth/me` now accepts only bearer/session-backed identity
    - debug headers remain route-compatibility fallback only and do not satisfy authenticated-user semantics on `/v1/auth/me`
    - workbench preference scope is now session-backed on the mainline HTTP path
    - ready-for-frontend request-body actor-id defaults now only derive from real session actors; debug fallback remains limited to non-frontend/internal placeholder routes
  - Step 56 begins Step B without expanding into a full RBAC redesign:
    - `ready_for_frontend` role-gated routes now require bearer/session-backed actors before role checks
    - `internal_placeholder` and `mock_placeholder_only` routes still keep narrow debug-header role compatibility
    - admin users can inspect the protected route-role contract through `GET /v1/access-rules`
    - permission logs now persist `actor_username`, route `readiness`, `session_required`, `debug_compatible`, and clearer decision reasons
  - Step 57 lands the next narrow Step C increment without distorting task ownership:
    - `PATCH /v1/tasks/{id}/business-info` is now the only ERP Bridge product-upsert boundary
    - that filing boundary is active only when `filed_at` is set for `source_mode=existing_product`
    - filing now requires ERP-backed `product_selection.erp_product` instead of a legacy local-only binding
    - ERP-backed `product_selection` now always resolves/ensures one local bound `products` row
    - mismatched `selected_product_id` plus ERP snapshot is now rejected instead of being silently trusted
    - internal/admin filing trace now reuses integration connector `erp_bridge_product_upsert`
  - Step 26 adds an export-center skeleton over existing stable read models:
    - `GET /v1/export-templates`
    - `POST /v1/export-jobs`
    - `GET /v1/export-jobs`
    - `GET /v1/export-jobs/{id}`
    - export jobs now persist:
      - export type
      - source query type
      - source filters
      - task-query `query_template` / `normalized_filters` when applicable
      - placeholder request actor
      - export status
      - placeholder `result_ref`
    - current export sources are intentionally limited to stable list/board/procurement/warehouse query state
    - real file generation, storage, NAS, and async execution are still deferred
  - Step 27 keeps real storage deferred but adds a minimal export-job lifecycle skeleton:
    - `POST /v1/export-jobs/{id}/advance`
    - lifecycle statuses are now:
      - `queued`
      - `running`
      - `ready`
      - `failed`
      - optional `cancelled`
    - export-job read models now expose:
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
    - `ready` only means placeholder handoff metadata is available; it does not mean real file delivery exists
  - Step 28 adds export-job lifecycle audit trace without expanding into real download/storage:
    - `GET /v1/export-jobs/{id}/events`
    - export-job list/detail now expose lightweight audit summaries:
      - `event_count`
      - `latest_event`
    - export jobs now write durable lifecycle events:
      - `export_job.created`
      - `export_job.advanced_to_running`
      - `export_job.advanced_to_ready`
      - `export_job.advanced_to_failed`
      - `export_job.advanced_to_cancelled`
      - `export_job.advanced_to_queued`
      - `export_job.result_ref_updated`
    - event payload is audit context only and is not a runner log stream
  - Step 29 adds a frontend-ready placeholder download claim/read boundary:
    - `POST /v1/export-jobs/{id}/claim-download`
    - `GET /v1/export-jobs/{id}/download`
    - these routes only work for `ready` export jobs
    - they return structured handoff metadata, not file bytes
    - claim/read activity is appended into the same export-job event chain through:
      - `export_job.download_claimed`
      - `export_job.download_read`
  - Step 30 adds enforced placeholder handoff expiry/refresh semantics:
    - `POST /v1/export-jobs/{id}/refresh-download`
    - ready + not expired => claim/read allowed
    - ready + expired => claim/read rejected until refresh
    - export-job list/detail now expose:
      - `is_expired`
      - `can_refresh`
    - handoff responses now expose:
      - `is_expired`
      - `can_refresh`
    - expiry/refresh activity is appended into the same export-job event chain through:
      - `export_job.download_expired`
      - `export_job.download_refreshed`
  - Step 31 adds explicit export runner-initiation semantics without introducing a real async platform:
    - `POST /v1/export-jobs/{id}/start`
    - export-job list/detail now also expose:
      - `can_start`
      - `start_mode`
      - `execution_mode`
      - `latest_runner_event`
    - current explicit runner-boundary events now include:
      - `export_job.runner_initiated`
      - `export_job.started`
    - `POST /v1/export-jobs/{id}/advance` `action=start` is still accepted, but only as backward-compatible reuse of the same start helper
  - Step 32 adds placeholder execution-attempt and runner-adapter visibility without introducing a real async platform:
    - `GET /v1/export-jobs/{id}/attempts`
    - export-job list/detail now also expose:
      - `attempt_count`
      - `latest_attempt`
      - `can_retry`
    - current attempt records now expose:
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
    - current attempt-result events now include:
      - `export_job.attempt_succeeded`
      - `export_job.attempt_failed`
      - `export_job.attempt_cancelled`
  - Step 33 adds placeholder adapter-dispatch handoff visibility without introducing a real scheduler platform:
    - `GET /v1/export-jobs/{id}/dispatches`
    - `POST /v1/export-jobs/{id}/dispatches`
    - `POST /v1/export-jobs/{id}/dispatches/{dispatch_id}/advance`
    - current dispatch records now expose:
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
    - current dispatch events now include:
      - `export_job.dispatch_submitted`
      - `export_job.dispatch_received`
      - `export_job.dispatch_rejected`
      - `export_job.dispatch_expired`
      - `export_job.dispatch_not_executed`
  - Step 34 hardens export job read models around that dispatch layer:
    - export-job list/detail now also expose:
      - `dispatch_count`
      - `latest_dispatch`
      - `can_dispatch`
      - `can_redispatch`
      - `latest_dispatch_event`
    - `can_start` is now dispatch-aware and remains false while the latest dispatch is still `submitted`
    - export-job read models now keep lifecycle, dispatch, and attempt semantics visibly separate
  - Step 35 adds a narrow integration-center / API call log skeleton without introducing real external execution:
    - `GET /v1/integration/connectors`
    - `POST /v1/integration/call-logs`
    - `GET /v1/integration/call-logs`
    - `GET /v1/integration/call-logs/{id}`
    - `POST /v1/integration/call-logs/{id}/advance`
    - current connector catalog is static, with mostly placeholder connectors plus one narrow ERP filing trace connector:
      - `erp_product_stub`
      - `erp_bridge_product_upsert`
      - `export_adapter_bridge`
    - current integration call-log lifecycle is:
      - `queued`
      - `sent`
      - `succeeded`
      - `failed`
      - `cancelled`
    - current integration call-log read models now expose:
      - `progress_hint`
      - `latest_status_at`
      - `started_at`
      - `finished_at`
      - `can_replay`
  - Step 36 hardens placeholder route auth without introducing a real identity platform:
    - V7 routes carrying `withAccessMeta(...)` now enforce `required_roles`
    - request identity still comes only from:
      - `X-Debug-Actor-Id`
      - `X-Debug-Actor-Roles`
    - current auth mode is now:
      - `debug_header_role_enforced`
    - `Admin` is currently accepted as a placeholder route-level override
    - login/session/org/visibility data scope still remain future work
  - Step 37 adds a placeholder task-asset storage/upload adapter boundary without introducing a real file system:
    - internal placeholder upload-intent APIs now exist:
      - `POST /v1/assets/upload-requests`
      - `GET /v1/assets/upload-requests/{id}`
    - task assets now expose additive boundary fields:
      - `upload_request_id`
      - `storage_ref_id`
      - `mime_type`
      - `file_size`
      - nested `storage_ref`
    - task-asset writes now auto-create placeholder `asset_storage_refs`
    - `submit-design` and `mock-upload` may optionally bind a prior `upload_request_id`
    - legacy `file_path` / `whole_hash` stay as compatibility metadata only; they are no longer the preferred long-term storage boundary
  - Step 38 hardens integration center around an explicit execution boundary without introducing real external execution:
    - internal placeholder execution APIs now exist:
      - `GET /v1/integration/call-logs/{id}/executions`
      - `POST /v1/integration/call-logs/{id}/executions`
      - `POST /v1/integration/call-logs/{id}/executions/{execution_id}/advance`
    - integration call-log read models now additionally expose:
      - `execution_count`
      - `latest_execution`
      - `can_retry`
      - backward-compatible `can_replay`
    - current integration execution state contract is:
      - `prepared`
      - `dispatched`
      - `received`
      - `completed`
      - `failed`
      - `cancelled`
    - `POST /v1/integration/call-logs/{id}/advance` remains compatibility-only and now reuses execution semantics for non-queued transitions
  - Step 47 hardens retry/replay semantics on top of that execution boundary without introducing real external execution:
    - internal placeholder execution action APIs now exist:
      - `POST /v1/integration/call-logs/{id}/retry`
      - `POST /v1/integration/call-logs/{id}/replay`
    - integration executions now additionally expose:
      - `action_type`
    - integration call-log read models now additionally expose:
      - `retry_count`
      - `replay_count`
      - `latest_retry_action`
      - `latest_replay_action`
      - `retryability_reason`
      - `replayability_reason`
    - current placeholder semantics are:
      - `retry` = new execution only for retryable failed outcomes
      - `replay` = new execution that re-drives the recorded call-log envelope, including succeeded/cancelled outcomes
    - retry/replay history still reuses `integration_call_executions`; no separate action-event stream exists yet
  - Step 48 hardens export dispatch/start/attempt admission semantics without introducing real scheduler or runner infrastructure:
    - export-job list/detail now additionally expose:
      - `can_start_reason`
      - `can_attempt`
      - `can_attempt_reason`
      - `can_dispatch_reason`
      - `can_redispatch_reason`
      - `dispatchability_reason`
      - `attemptability_reason`
      - `latest_admission_decision`
    - dispatch records now additionally expose:
      - `start_admissible`
      - `start_admission_reason`
    - attempt records now additionally expose:
      - `blocks_new_attempt`
      - `next_attempt_admission_reason`
    - start compatibility policy is now explicit:
      - latest `received` dispatch can be consumed directly
      - if no startable dispatch exists, `/start` may still auto-create one placeholder submitted+received dispatch for backward-compatible initiation
    - admission rejection details are now aligned across API errors and list/detail read models
  - Step 49 adds unified auth/org/visibility policy scaffolding without introducing real identity/org systems:
    - cross-center reusable policy summaries now exist through:
      - `policy_scope_summary`
      - `resource_access_policy`
      - `action_policy_summary`
    - task/export/integration/cost/upload read models now additionally expose:
      - `policy_mode`
      - `visible_to_roles`
      - `action_roles`
      - `policy_scope_summary`
    - this remains additive policy language only:
      - no real login/session/SSO
      - no real org hierarchy sync
      - no final fine-grained RBAC/ABAC evaluator
      - no full approval permission system redesign
    - existing route-level contract remains stable:
      - `X-Debug-Actor-*` request headers
      - `withAccessMeta(...)` route metadata + enforcement
  - Step 50 adds unified KPI/finance/report platform entry-boundary scaffolding without introducing real BI/finance/report systems:
    - cross-center reusable platform-entry summaries now exist through:
      - `platform_entry_boundary`
      - `kpi_entry_summary`
      - `finance_entry_summary`
      - `report_entry_summary`
    - current coverage is:
      - task center (`task_list_item` / `task_read_model` / `task_detail_aggregate`)
      - procurement summary (`procurement_summary`)
      - cost governance boundary (`override_governance_boundary`)
      - export center (`export_job`)
    - current entry summaries now explicitly expose:
      - source read-model fields reused by future platform docking
      - placeholder-only future fields reserved for later platform integrations
      - `eligible_now` boundary hints
    - this remains boundary language only:
      - no real KPI/BI engine
      - no real finance/accounting/reconciliation/settlement/invoice engine
      - no real report generation engine
      - no real data warehouse / analytics engine
  - Step 51 deepens upload/storage placeholder management visibility without introducing real upload/storage infrastructure:
    - internal placeholder upload-request management route now exists:
      - `GET /v1/assets/upload-requests`
    - current upload-request filters are:
      - `owner_type`
      - `owner_id`
      - `task_asset_type`
      - `status`
    - this remains management/read visibility only:
      - no real upload session allocator
      - no signed URL issuance
      - no NAS/object-storage integration
      - no file-byte confirmation
  - Step 52 brings the repo back to the mainline by adding minimal real identity/auth support without introducing SSO or deep policy engines:
    - real auth APIs now exist:
      - `POST /v1/auth/register`
      - `POST /v1/auth/login`
      - `GET /v1/auth/me`
    - admin user / role / permission-log APIs now exist:
      - `GET /v1/roles`
      - `GET /v1/users`
      - `GET /v1/users/{id}`
      - `PATCH /v1/users/{id}`
      - `POST /v1/users/{id}/roles`
      - `PUT /v1/users/{id}/roles`
      - `DELETE /v1/users/{id}/roles/{role}`
      - `GET /v1/permission-logs`
    - current persisted auth boundary is:
      - `users`
      - `user_roles`
      - `user_sessions`
      - `permission_logs`
    - request actor resolution now prefers `Authorization: Bearer <token>` and still keeps `X-Debug-Actor-*` compatibility
    - current frontend auth contract now includes:
      - `AuthSession.session_id`
      - register/login -> `data.user.frontend_access.*`
      - current-user -> `data.frontend_access.*`
      - `frontend_access.is_super_admin`
      - `frontend_access.permission_flags`
      - `frontend_access.page_keys`
      - `frontend_access.menu_keys`
      - `frontend_access.module_keys`
      - `frontend_access.access_scopes`
    - current permission logs now cover:
      - route access decisions
      - register
      - login
      - login failure
      - role assignment
      - role removal
      - optional target-user context for admin changes
    - key task-flow write APIs now default actor ids from the authenticated request actor when legacy explicit ids are omitted
    - this remains intentionally limited:
      - no SSO
      - no org hierarchy sync
      - no final RBAC/ABAC engine
      - no external identity provider integration
  - Step 63 keeps that minimal auth baseline but upgrades it for frontend联调 without broadening the platform scope:
    - `POST /v1/auth/register` now accepts `account`/`name`/`department`/`mobile`/optional `email`/`password`/optional admin key
    - `PUT /v1/auth/password` now exists for current-user password change
    - `users` now additionally persist `department`, `mobile`, `email`, and `is_config_super_admin`
    - phone uniqueness is currently enforced through `uq_users_mobile`
    - config-backed auth/bootstrap inputs now live in:
      - `config/auth_identity.json`
      - `config/frontend_access.json`
    - config-managed super admins now replace the old first-user-admin bootstrap behavior
    - current frontend auth contract now additionally includes:
      - `WorkflowUser.account`
      - `WorkflowUser.name`
      - `WorkflowUser.department`
      - `WorkflowUser.mobile`
      - `WorkflowUser.phone`
      - `WorkflowUser.email`
      - `frontend_access.is_department_admin`
      - `frontend_access.department`
      - `frontend_access.managed_departments`
    - current department-admin behavior is intentionally narrow:
      - registration-time promotion via configured department key
      - marker role `DepartmentAdmin`
      - frontend-gating hints only; no org tree, row filtering, or data-isolation redesign
  - Step 66 continues the auth baseline into a minimally usable department-team org slice without broadening into a full org platform:
    - `GET /v1/auth/register-options` now exposes fixed department/team register-form options
    - `users` now additionally persist `team`
    - register validation now enforces that optional `team` belongs to the selected department
    - current frontend auth contract now additionally includes:
      - `WorkflowUser.team`
      - compatibility `WorkflowUser.group`
      - `frontend_access.team`
      - `frontend_access.roles`
      - `frontend_access.scopes`
      - `frontend_access.menus`
      - `frontend_access.pages`
      - `frontend_access.actions`
      - `frontend_access.modules`
    - HR-visible admin/read scope is now minimally linkable for frontend integration:
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
    - ERP query routes are now explicitly visible to all authenticated users rather than a role subset
    - current org scope remains intentionally narrow:
      - no org tree
      - no team-admin role
      - no row-level ABAC/data-isolation platform
  - Step 39 hardens export center around explicit planning-only runner / storage / delivery boundaries without introducing real infrastructure:
    - export-job list/detail now additionally expose:
      - `adapter_mode`
      - `storage_mode`
      - `delivery_mode`
      - `execution_boundary`
      - `storage_boundary`
      - `delivery_boundary`
    - current responsibility split is now explicit:
      - start execution -> `POST /v1/export-jobs/{id}/start`
      - dispatch handoff -> `export_job_dispatches`
      - one execution try -> `export_job_attempts`
      - placeholder result generation -> export-job lifecycle advance
      - placeholder storage representation -> `result_ref`
      - placeholder delivery handoff -> `claim-download` / `download` / `refresh-download`
    - future real runner / storage / delivery work should replace those layers beneath the current contracts rather than redefining lifecycle, dispatch, attempt, or `result_ref`
  - Step 40 hardens the cost-rule skeleton around governance/versioning/override trace without introducing a separate version engine or approval flow:
    - cost-rule list/detail now additionally expose:
      - `rule_version`
      - `supersedes_rule_id`
      - `superseded_by_rule_id`
      - `governance_note`
      - `governance_status`
    - cost preview now additionally exposes:
      - `matched_rule_id`
      - `matched_rule_version`
      - `governance_status`
    - task business-info persistence now additionally snapshots:
      - `matched_rule_version`
      - `prefill_source`
      - `prefill_at`
      - `override_actor`
      - `override_at`
    - historical task-side cost results remain snapshot-based:
      - later rule changes affect future preview/prefill only
      - old task rows are not auto-recomputed
  - Step 41 hardens the cost-governance read side without changing Step 40 write semantics:
    - cost-rule list/detail now additionally expose:
      - `version_chain_summary`
      - `previous_version`
      - `next_version`
      - `supersession_depth`
    - new read-only lineage endpoint:
      - `GET /v1/cost-rules/{id}/history`
    - task read/detail and `procurement_summary` now additionally expose:
      - `matched_rule_governance`
      - `override_summary`
    - `matched_rule_governance` must separate:
      - the historical matched rule snapshot
      - the current latest rule in that lineage
    - `override_summary` remains a lightweight summary derived from task business-info events:
      - not a real approval audit stream
      - not a real identity or approver model
  - Step 42 hardens task-side override governance into a dedicated audit skeleton without introducing approval flow or finance integration:
    - dedicated governance audit persistence now exists through:
      - `cost_override_events`
      - `cost_override_event_sequences`
    - `PATCH /v1/tasks/{id}/business-info` still writes `task_event_logs` as the general task event stream, but now also appends dedicated override audit events when override state changes
    - new read-only task timeline endpoint:
      - `GET /v1/tasks/{id}/cost-overrides`
    - task read/detail and `procurement_summary` now additionally expose:
      - `governance_audit_summary`
    - `override_summary` remains the stable lightweight summary contract:
      - prefer the dedicated override audit stream when available
      - fall back to older task business-info events when no dedicated rows exist yet
  - Step 43 adds approval / finance placeholder boundaries above the dedicated override audit layer without introducing real approval workflow or finance/accounting systems:
    - dedicated placeholder persistence now exists through:
      - `cost_override_reviews`
      - `cost_override_finance_flags`
    - task read/detail, purchase-task `procurement_summary`, and `/v1/tasks/{id}/cost-overrides` now additionally expose:
      - `override_governance_boundary`
    - internal placeholder write actions now exist through:
      - `POST /v1/tasks/{id}/cost-overrides/{event_id}/review`
      - `POST /v1/tasks/{id}/cost-overrides/{event_id}/finance-mark`
    - current layering must stay explicit:
      - rule history != override audit != approval placeholder != finance placeholder
    - this still remains a placeholder boundary only:
      - not a real approval workflow
      - not a finance / accounting / reconciliation / settlement / invoice system
      - not an ERP cost writeback interface
  - Step 44 consolidates that placeholder boundary into one stable read model without expanding the Step 43 write surface:
    - `override_governance_boundary` remains the unified boundary object across task read/detail, purchase-task `procurement_summary`, and `/v1/tasks/{id}/cost-overrides`
    - the boundary now additionally exposes:
      - `governance_boundary_summary`
      - `approval_placeholder_summary`
      - `finance_placeholder_summary`
      - `latest_review_action`
      - `latest_finance_action`
      - `latest_boundary_actor`
      - `latest_boundary_at`
    - current layering must stay explicit:
      - rule history / lineage = governed rule layer
      - matched snapshot / prefill trace = task-side historical hit layer
      - override audit = `cost_override_events`
      - approval placeholder = `cost_override_reviews`
      - finance placeholder = `cost_override_finance_flags`
      - boundary summary = stable read-model aggregation above those placeholder layers
    - these new read fields are frontend-facing summaries only:
      - not a real approval workflow
      - not a real finance / accounting / settlement system
      - not an ERP cost writeback contract
  - Step 45 consolidates cross-center adapter-boundary terminology across export, integration, and storage/upload without merging their tables or introducing real infrastructure:
    - shared terminology is now explicit:
      - `adapter_mode`
      - `execution_mode`
      - `dispatch_mode`
      - `storage_mode`
      - `delivery_mode`
    - shared minimal summaries now exist:
      - `adapter_ref_summary`
      - `resource_ref_summary`
      - `handoff_ref_summary`
    - current reuse relationship must stay explicit:
      - export keeps lifecycle / dispatch / attempt / result / delivery semantics and now adds unified `dispatch_mode` plus shared summaries
      - integration keeps connector / call-log / execution semantics and now reuses shared adapter / handoff summaries plus unified `adapter_mode` / `dispatch_mode`
      - storage/upload keeps upload-request / storage-ref semantics and now reuses shared adapter / resource / handoff summaries plus unified `adapter_mode` / `dispatch_mode` / `storage_mode`
    - this is still language/read-model consolidation only:
      - not a real runner platform
      - not real storage/upload infrastructure
      - not real external execution
      - not a cross-center table merge
  - Step 46 deepens upload-request lifecycle semantics without introducing real upload/storage infrastructure:
    - internal placeholder lifecycle API now exists:
      - `POST /v1/assets/upload-requests/{id}/advance`
    - supported explicit lifecycle actions are:
      - `cancel`
      - `expire`
    - upload-request reads now additionally expose:
      - `can_bind`
      - `can_cancel`
      - `can_expire`
    - `bound` remains reserved for task-asset binding only
    - not a real byte-upload session
    - not signed-URL allocation
    - not NAS / object-storage integration
  - Step 67 adds the asset-center integration-preparation layer on top of those placeholder boundaries:
    - `design_assets` are business asset roots only
    - `task_assets` are the append-only asset-version persistence layer for that root
    - `upload_requests` are still the persisted handoff/session table and are now projected to frontend as `upload_session`
    - the new upload-service client seam is where the later NAS-side Go upload service must connect
    - do not collapse task business rules and remote storage transport back into one service or one table
  - Step 47 deepens integration execution troubleshooting without introducing real external execution:
    - retry/replay remain internal/admin-only execution-boundary actions
    - both actions create new placeholder executions instead of introducing a second action subsystem
    - later real executor, callback, and scheduler work must attach beneath or beside `integration_call_executions`, not replace the call-log / execution layering

## Read These First
1. CURRENT_STATE.md
2. docs/NEXT_PHASE_ROADMAP.md（v0.8 后优先级）
3. docs/iterations/latest iteration
4. docs/api/openapi.yaml
5. docs/V7_MODEL_HANDOVER_APPENDIX.md
6. Go_V6_to_V7_迁移执行包.md

## Non-Negotiables
- Task is aggregate root
- SKU determined during task creation
- existing_product binds ERP SKU
- new_product generates SKU by rule
- purchase_task must not be forced through designer assignment or audit
- warehouse readiness and close readiness must expose machine-readable blocking reasons with stable `code`
- `task_status` is persisted operational state; `workflow.main_status` is the PRD mainline projection; `workflow.sub_status` is the stable structured sub-line contract
- projected workflow filters must stay aligned with the actual `workflow.main_status` / `workflow.sub_status` derivation logic
- board/list convergence filters must stay aligned with the actual `workflow`, `procurement_summary`, and warehouse-readiness derivation logic
- `PATCH /v1/tasks/{id}/business-info` must not be reused as the source of truth for procurement preparation
- `PATCH /v1/tasks/{id}/business-info` is the correct boundary for category linkage, cost-prefill inputs, internal cost-rule provenance, estimated cost, and manual override state; do not move those fields into procurement or remarks
- `PATCH /v1/tasks/{id}/procurement` and `POST /v1/tasks/{id}/procurement/advance` jointly define the minimal procurement boundary; do not move this back into `task_details`
- first-level coded-style category entries are legitimate business categories, not cleanup targets
- category center must stay configurable and ERP-mapping-friendly; do not collapse it back into free-text task fields
- top-level category code is the first-level ERP search entry; do not relegate this back to documentation-only convention
- `search_entry_code` / `is_search_entry` are semantic model fields, not optional UI hints
- `category_erp_mappings` is the correct independent boundary for category-to-ERP positioning; do not hardcode this into task detail, procurement, or product master rows
- `GET /v1/products/search` is now allowed to consume `category_erp_mappings`, but only against already-synced local ERP product data; do not present this as real ERP API lookup
- `GET /v1/erp/products` / `GET /v1/erp/products/{id}` are now the real ERP Bridge query boundary for the original-product picker; prefer keyword search here first
- `/v1/erp/products` additive filter normalization must stay aligned across handler, service, OpenAPI, and frontend expectations:
  - `q` remains the canonical keyword field
  - `keyword` is compatibility-only and normalizes into `q` when `q` is absent
  - `sku_code` / `category_id` / `category_name` are additive only and must not replace keyword-first entry semantics
  - response `normalized_filters` is an echo of the effective backend query contract, not a second search result source
- `GET /v1/erp/categories` is auxiliary only in the current phase; do not make incomplete bridge category coverage a blocker for original-product picking
- task-side existing-product traceability must flow through `product_selection`; do not spread original-product picker provenance across ad hoc remark fields or frontend-only state
- `product_selection` is allowed to rebind existing-product tasks through business-info for both:
  - local mapped-search provenance
  - ERP Bridge-selected products that have been cached/bound into local `products`
- task-side `product_selection.erp_product` is additive snapshot data only; it preserves external id / sku / image / price and does not replace local task binding
- ERP Bridge write usage is now narrow and explicit:
  - `product_selection.erp_product` itself is not the writeback
  - Bridge `products/upsert` is called only from `PATCH /v1/tasks/{id}/business-info` when `filed_at` is set on an `existing_product` task
  - legacy local-only existing-product bindings must be reselected through the ERP-backed picker before filing
  - current filing trace is visible internally through integration call logs under connector `erp_bridge_product_upsert`
- task-side `product_selection.erp_product` must remain backward-compatible:
  - missing non-identity fields may be backfilled from prior cached snapshot data
  - missing name / sku / category fields may be backfilled from the resolved local product or task binding context
  - later hardening must not silently drop older partial snapshots
- ERP Bridge observability in the current phase is limited and additive:
  - request duration / status logging
  - timeout / retry-hint diagnostics on failures
  - no callback processor, retry scheduler, circuit-breaker platform, or external APM dependency is implied
- **ERP 主链可观测性约束（长期保护）**：主链打通后不得删除：
  - `ERP_REMOTE_MODE=hybrid|remote` 时：必须保留 `erp_bridge_product_search` / `erp_bridge_product_by_id`（含 `result`、`fallback_used`、`fallback_reason`）及 `remote_erp_openweb_request_started/completed`
  - `ERP_SYNC_SOURCE_MODE=jst` 时：必须保留 `erp_sync_run_start/finish`（含 provider、page、upsert、sample_sku、rate_limit/retry 日志）
- `product_selection` naming must stay aligned across list, board, procurement summary, read, and detail contracts:
  - list/board/procurement-summary use the lightweight summary shape
  - read/detail keep the full provenance shape with `matched_mapping_rule`
- current mapped product-search fallback is intentional:
  - prefer exact category mappings when present
  - otherwise fall back to first-level `search_entry_code` mappings
- second/third-level ERP search refinement is reserved through explicit mapping condition fields; do not claim a full tree/search engine has landed yet
- cost-rule center must stay configuration-driven; do not translate Excel-style samples into hardcoded service `if/else` branches
- `estimated_cost` and `cost_price` must stay semantically distinct:
  - `estimated_cost` is system preview/prefill output
  - `cost_price` is the current effective internal cost
  - `manual_cost_override` indicates business override state and is not an auth concept
- cost-rule governance in the current phase is row-hardening, not a separate rule-engine platform:
  - `rule_version` / `supersedes_rule_id` / `superseded_by_rule_id` / `governance_note` / `governance_status` are additive governance metadata only
  - they must not be described as a full approval flow, formula DSL, or finance system
  - later rule changes must not silently rewrite historical task-side snapshots
- task-side cost governance trace must stay aligned across code, docs, and OpenAPI:
  - preview exposes the governed matched rule/version/source/status
  - business-info persists the last task-side prefill snapshot through `matched_rule_version` / `prefill_source` / `prefill_at`
  - manual override persists lightweight business trace through `manual_cost_override_reason` / `override_actor` / `override_at`
  - `override_actor` is a lightweight placeholder trace only and must not be described as a real auth identity or approval approver
- task-side governance read models must keep three layers separate:
  - historical matched rule snapshot = what the task last hit or persisted
  - current lineage state = what the latest reachable governed rule is now
  - override summary = lightweight business adjustment trace derived from task events
- task-side override governance now adds a fourth layer that must stay separate:
  - dedicated override audit stream = governance-specific timeline of override apply/update/release events
  - `task_event_logs` still remain the general task event stream and must not be re-described as the governance-specific audit layer
- task-side approval / finance placeholder now adds a fifth layer that must stay separate:
  - approval placeholder = lightweight post-override review requirement/status boundary
  - finance placeholder = lightweight post-review finance-facing handoff boundary
  - they must not be collapsed back into rule history, override summary, or override audit
- Step 41 read-model hardening does not change the history policy:
  - later rule changes still affect future preview/prefill only
  - historical task rows are still not auto-recomputed
- Step 42 still does not change the history policy:
  - later rule changes still affect future preview/prefill only
  - dedicated override audit events explain later human cost changes, not retroactive rule recomputation
  - no historical backfill from older `task_event_logs` into `cost_override_events` is implied by this phase
- purchase-task procurement-to-warehouse coordination must stay derivable from task + procurement + warehouse state without introducing a second source of truth
- task-board presets must reuse the same projected workflow / procurement summary semantics as `/v1/tasks`, not invent parallel queue-state logic
- external converged filter names and semantics must stay stable even if repo/read-model predicate execution is hardened internally
- task-board aggregation hardening must preserve `queue_key`, `queue_name`, `filters`, `normalized_filters`, `query_template`, `count`, and sample-task/task-list payload basics for existing frontend consumers
- queue ownership metadata must remain hint-only until a future real auth / ownership phase explicitly changes that contract
- workbench preferences must stay lightweight and user-scoped; the current mainline HTTP contract is now session-backed only
- placeholder route auth must stay aligned across code, docs, and OpenAPI:
  - V7 `required_roles` are no longer advisory-only
  - `ready_for_frontend` route checks now require bearer session tokens before role matching
  - debug actor headers remain compatibility fallback only on internal/mock placeholder routes
  - `GET /v1/auth/me` is now bearer-session-only
  - user-scoped workbench preferences now require a bearer session
  - admins can inspect the current route-role contract through `GET /v1/access-rules`
  - `Admin` currently acts as a placeholder override at the route boundary
  - this phase must not be misdescribed as full org/department/team visibility enforcement
- Step 49 policy scaffolding must stay aligned across code, docs, and OpenAPI:
  - `policy_mode` / `visible_to_roles` / `action_roles` / `policy_scope_summary` are additive read-model hints only
  - these fields summarize default route-aligned visibility/action intent
  - these fields are not a runtime row-level or field-level policy engine
  - these fields must not be described as real login/SSO/org sync or final RBAC/ABAC rollout
- Step 50 platform-entry scaffolding must stay aligned across code, docs, and OpenAPI:
  - `platform_entry_boundary` / `kpi_entry_summary` / `finance_entry_summary` / `report_entry_summary` are additive read-model entry hints only
  - `eligible_now` and source-field lists are boundary semantics, not downstream execution guarantees
  - these fields must not be described as real KPI/BI computation, real accounting/settlement flows, or real report-generation pipelines
  - these fields must not be described as real data warehouse or analytics-engine readiness
- board candidate scan hardening must preserve the public task-board contract while making the candidate-scan boundary explicit enough for later index/materialized-view decisions
- export center must consume existing stable read-model contracts instead of inventing a parallel reporting query language
- `query_template` / `normalized_filters` semantics used by export jobs must stay aligned with the same task-list and task-board contracts the frontend already consumes
- `result_ref` in the current export center is placeholder download-handoff metadata only; do not present it as a real file path, signed URL, NAS handle, or completed storage integration
- task asset upload/storage boundary must stay aligned across code, docs, and OpenAPI:
  - `upload_requests` are placeholder upload-intent records only
  - `asset_storage_refs` are placeholder reference metadata only
  - task assets remain the business timeline and should reference that boundary rather than re-absorbing file-service semantics
  - task-asset `storage_ref` and export `result_ref` should stay semantically aligned as placeholder adapter/type/key/metadata contracts, but they are intentionally not merged yet
  - no real file upload, NAS, object storage, signed URL, CDN, or strict whole-hash verification should be claimed in this phase
  - Step 46 only adds explicit placeholder lifecycle control over `requested -> cancelled|expired`
  - Step 51 only adds internal paginated upload-request list/filter visibility; it does not introduce a real upload-management platform
  - `bound` must remain the result of task-asset binding, not a separate upload-request-only route
  - `can_bind` / `can_cancel` / `can_expire` are read-model hints only; they are not proof that a real upload allocator or background expirer exists
- Step 45 cross-center adapter-boundary consolidation must stay aligned across code, docs, and OpenAPI:
  - shared terminology is additive language only and must not erase center-specific semantics
  - `adapter_mode` describes the boundary strategy, not infrastructure readiness
  - `execution_mode` remains center-specific for export/integration execution seams
  - `dispatch_mode` is the shared handoff progression term across dispatch/execution/upload-request records
  - `storage_mode` is the shared resource-representation term and does not force export `result_ref` and `asset_storage_refs` to share one table
  - `delivery_mode` currently remains export-center-specific and must not be projected onto integration/storage where no equivalent consumer-delivery seam exists yet
  - `adapter_ref_summary`, `resource_ref_summary`, and `handoff_ref_summary` are read-model summaries only; they are not a scheduler protocol, queue protocol, or file-service abstraction by themselves
- `POST /v1/export-jobs/{id}/start` is the formal placeholder runner-initiation boundary for `queued -> running`; do not describe it as a real async runner, scheduler, worker-lease API, or file-generation platform
- `GET /v1/export-jobs/{id}/attempts` is internal/admin placeholder visibility only; do not mark it ready-for-frontend and do not describe it as a real scheduler, worker-lease, heartbeat, or runner telemetry API
- `POST /v1/export-jobs/{id}/advance` is internal/admin skeleton only; do not mark it ready-for-frontend and do not describe it as a real runner or scheduler
- export-job lifecycle semantics must stay aligned across code, docs, and OpenAPI:
  - `queued`
  - `running`
  - `ready`
  - `failed`
  - optional `cancelled`
- export-job event trace must stay aligned across code, docs, and OpenAPI:
  - `GET /v1/export-jobs/{id}/events` is the timeline boundary
  - `event_count` / `latest_event` are summaries only
  - `latest_runner_event` is also a summary only
  - `export_job.runner_initiated` / `export_job.started` are explicit start-boundary audit events
  - `export_job.attempt_succeeded` / `export_job.attempt_failed` / `export_job.attempt_cancelled` are explicit attempt-result audit events
  - event payload is audit context only, not a full runner log
- export-job attempt semantics must stay aligned across code, docs, and OpenAPI:
  - execution-attempt state must not replace export-job lifecycle state
  - `latest_attempt` is a summary of one concrete execution attempt, not the job lifecycle itself
  - `can_attempt` / `can_attempt_reason` must express current start-attempt admission in a machine-readable way
  - `attemptability_reason` is the current attempt-side admission alias on list/detail read models
  - `latest_admission_decision` is a summary hint only and not a second persisted event stream
  - attempt-list hints (`blocks_new_attempt` / `next_attempt_admission_reason`) are attempt-status-derived only; they do not replace job-level admission checks
  - `can_retry` currently means the job is back in `queued` and already has historical attempts
  - `adapter_key` / `adapter_note` expose the placeholder runner-adapter seam only; they are not proof of a real scheduler or distributed worker platform
- export-job dispatch semantics must stay aligned across code, docs, and OpenAPI:
  - dispatch state must not replace export-job lifecycle state or execution-attempt state
  - `latest_dispatch` is a summary of one placeholder adapter handoff, not proof that execution started
  - `dispatch_count` counts placeholder handoff records only; it is not a scheduler queue depth metric
  - `can_dispatch` / `can_redispatch` plus `can_dispatch_reason` / `can_redispatch_reason` are admission hints only and do not imply a real scheduler platform
  - `dispatchability_reason` is the current dispatch-side admission alias on list/detail read models
  - dispatch-list hints (`start_admissible` / `start_admission_reason`) are dispatch-status-derived only; they do not replace job-level start admission checks
  - latest `submitted` dispatch must keep `can_start=false` until the dispatch is received or otherwise resolved
  - when no startable dispatch exists, `/start` may still use backward-compatible auto-placeholder dispatch creation; this is explicit compatibility behavior, not a real scheduler fallback
- export-job planning-boundary semantics must stay aligned across code, docs, and OpenAPI:
  - `execution_boundary` clarifies current start / dispatch / attempt / placeholder result-generation layering only
  - `storage_boundary` clarifies that `result_ref` is the current placeholder storage representation only
  - `delivery_boundary` clarifies that claim/read/refresh are the current placeholder delivery seam only
  - `adapter_mode` / `storage_mode` / `delivery_mode` are planning hints only and must not be described as proof of real infrastructure readiness
- integration-center call-log semantics must stay aligned across code, docs, and OpenAPI:
  - call-log state must not be described as proof that a real external request happened
  - `can_retry` and `can_replay` are distinct placeholder admission hints and do not imply a real retry/replay engine exists
  - `retryability_reason` / `replayability_reason` must explain why the current call log can or cannot accept each action
  - static connector metadata is descriptive only and does not imply SDK/auth/callback infrastructure has landed
- integration-center execution semantics must stay aligned across code, docs, and OpenAPI:
  - execution state must not replace call-log lifecycle state
  - `latest_execution` is a summary of one concrete placeholder execution attempt, not proof that a real external request was delivered
  - `execution_count` counts persisted placeholder execution attempts only; it is not queue depth or throughput telemetry
  - `action_type` / `trigger_source` must keep `start` / `retry` / `replay` / compatibility executions distinguishable on the same execution boundary
  - `latest_retry_action` / `latest_replay_action` plus `retry_count` / `replay_count` must stay derived from persisted executions rather than a second action-history subsystem
  - `POST /v1/integration/call-logs/{id}/advance` must remain a compatibility facade over execution semantics rather than a second independent execution lifecycle
- placeholder download handoff semantics must stay aligned across code, docs, and OpenAPI:
  - `POST /v1/export-jobs/{id}/claim-download` is a handoff claim action, not a real download
  - `GET /v1/export-jobs/{id}/download` is a handoff read action, not a byte-stream file endpoint
  - `POST /v1/export-jobs/{id}/refresh-download` is a placeholder handoff refresh action, not signed-URL renewal or storage reissue
  - claim/read are valid only when export job status is `ready` and the current placeholder handoff is not expired
  - ready + expired handoff must surface `is_expired` / `can_refresh` semantics rather than being treated like non-ready lifecycle state
  - refresh is currently allowed only for expired ready handoff and must rotate `result_ref.ref_key`
  - claim/read must reuse `export_job_events`; do not create a second audit trail
  - expiry/refresh must also reuse `export_job_events`; do not add a second handoff audit subsystem
- openapi.yaml must be updated every API change
- every iteration leaves a written record
- V7 readiness/internal/mock classification follows docs/api/openapi.yaml and docs/V7_API_READY.md

## 2026-03-17 Bridge 补齐补充（ITERATION_073）
- 三服务形态明确保持：
  - 8080 MAIN（业务）
  - 8081 Bridge（统一 ERP/JST 适配层）
  - 8082 JST sync（常驻同步服务）
- 本轮补齐的 8081 Bridge 路由面：
  - `GET /v1/erp/sync-logs`
  - `GET /v1/erp/sync-logs/{id}`
  - `POST /v1/erp/products/shelve/batch`
  - `POST /v1/erp/products/unshelve/batch`
  - `POST /v1/erp/inventory/virtual-qty`
- 代码链路已落在 route + handler + service + client（非空壳）：
  - local 模式下写入会记录 integration call log，sync-log 可读
  - remote/hybrid 模式补齐了上述 mutation/sync-log 的 path 化远端调用能力
- integration connector 扩展：
  - `erp_bridge_product_shelve_batch`
  - `erp_bridge_product_unshelve_batch`
  - `erp_bridge_inventory_virtual_qty`
- OpenAPI 已显式区分 8081 Bridge 与 8082 JST sync 契约：
  - 8081：`/v1/erp/*`
  - 8082：`/internal/jst/ping`、`/jst/sync/inc`（标注为 8082 runtime role）
- 关键约束继续有效：
  - 不废弃 8082
  - 不让 MAIN 直接耦合 JST/OpenWeb 细节
  - 版本保持 `v0.4`（不升版本号）

## 2026-03-17 线上验证补充（ITERATION_073 后续）
- 远端 `223.4.249.11` 验证结论：
  - 8081 `/health` = `200`
  - 8081 ERP 路由（含新增 `sync-logs/shelve/unshelve/virtual-qty`）在无会话下统一返回 `401`，说明链路存在且受鉴权保护（非 `404`）
- 8082 状态修复：
  - 初查 `8082` 未监听（`HTTP:000`），`run/erp_sync.pid` 为陈旧 pid
  - 执行 `/root/ecommerce_ai/scripts/start-sync.sh --base-dir /root/ecommerce_ai` 后恢复
  - 恢复后：`/health`、`/internal/jst/ping`、`/jst/sync/inc` 均返回 `200`
- 三服务共存证据：
  - `ss -ltnp` 显示 `8080(ecommerce-api)`、`8081(erp_bridge)`、`8082(erp_bridge_sync)` 同时监听
  - `/proc/<pid>/exe` 均无 `(deleted)`
- 运维注意：
  - 当前发布脚本 cutover 流程自动管理 8080/8081，不自动拉起 8082
  - 发布后应增加 8082 状态检查与必要时 `start-sync.sh` 恢复动作

## 2026-03-17 交接补充（ITERATION_074）
- 本轮只做了两件事，且都已在远端 `223.4.249.11` 实测：
  - 用真实 bearer token 完成 8081 全部 ERP 路由 success/failure acceptance
  - 补三服务巡检与 8082 自恢复，并完成停服后自动拉起演练
- 8081 真实会话验收结果：
  - 登录：`POST /v1/auth/login` 返回 `200` 并签发真实 session token
  - 当前用户：`GET /v1/auth/me` 返回 `200`
  - 成功路径全部打通：
    - `GET /v1/erp/products`
    - `GET /v1/erp/products/{id}`
    - `GET /v1/erp/categories`
    - `POST /v1/erp/products/upsert`
    - `GET /v1/erp/sync-logs`
    - `GET /v1/erp/sync-logs/{id}`
    - `POST /v1/erp/products/shelve/batch`
    - `POST /v1/erp/products/unshelve/batch`
    - `POST /v1/erp/inventory/virtual-qty`
  - 本轮测试对象：
    - `product_id=bridge-accept-1773724718`
    - `sku_code=BRIDGE-ACCEPT-1773724718`
    - `sync_log_id=5/6/7/8`
  - 失败路径已确认合理：
    - 未认证 -> `401`
    - 参数非法/空 payload -> `400`
    - 不存在资源 -> `404`
- 8081 日志证据：
  - Bridge 日志 `/root/ecommerce_ai/logs/erp_bridge-20260317T042205Z.log` 已出现对应 `http_request` 记录
  - 已看到：
    - `/v1/erp/products/upsert` 的 `401/400/200`
    - `/v1/erp/sync-logs` 的 `200`
    - `/v1/erp/sync-logs/5` 的 `200`
    - `/v1/erp/products/shelve/batch` 的 `400/200`
    - `/v1/erp/products/unshelve/batch` 的 `200`
    - `/v1/erp/inventory/virtual-qty` 的 `400/200`
- 当前为什么 8082 不会自动拉起：
  - `deploy/remote-deploy.sh` 的 cutover `--start-services` 只启停 `8080/8081`
  - 之前仓内 deploy helper 列表没有 `start-sync.sh` / `stop-sync.sh`
  - 之前 `deploy/verify-runtime.sh` 只做 `8080` tcp/auth 检查，不检查也不恢复 `8082`
- 本轮已补的运维脚本：
  - `deploy/check-three-services.sh`
  - `deploy/start-sync.sh`
  - `deploy/stop-sync.sh`
- 接入方式：
  - `deploy/verify-runtime.sh` 现在会调用 `check-three-services.sh`
  - `deploy/deploy.sh` 的 cutover 发布后验证会传入 `--auto-recover-8082`
  - parallel deploy 显式跳过三服务巡检，避免改变候选端口验证语义
- 三服务巡检能力（已实测）：
  - 检查 `8080/8081/8082 /health`
  - 检查 pid 是否存在
  - 检查 TCP 监听
  - 检查 `/proc/<pid>/exe` 是否 `(deleted)`
  - 输出人类可读摘要、`KEY=VALUE`、`JSON_SUMMARY`
- 8082 自恢复演练（已实测）：
  - 先执行 `stop-sync.sh --base-dir /root/ecommerce_ai`
  - 确认 `8082 /health` = `000`
  - 再执行 `check-three-services.sh --base-dir /root/ecommerce_ai --auto-recover-8082`
  - 输出显示：
    - `SYNC_RECOVER_TRIGGERED=true`
    - `SYNC_RECOVER_SUCCESS=true`
    - 新 pid = `3498341`
  - 恢复后再次确认：
    - `GET /health` = `200`
    - `GET /internal/jst/ping` = `200`
    - `POST /jst/sync/inc` = `200`
- 当前推荐发布后固定动作：
  - 首选：直接执行 `bash /root/ecommerce_ai/scripts/verify-runtime.sh --base-dir /root/ecommerce_ai --base-url http://127.0.0.1:8080 --bridge-url http://127.0.0.1:8081 --sync-url http://127.0.0.1:8082 --auto-recover-8082`
  - 或者只跑三服务巡检：`bash /root/ecommerce_ai/scripts/check-three-services.sh --base-dir /root/ecommerce_ai --auto-recover-8082`
## 2026-03-19 task-create reference upload closure
- Formal create-task contract:
  - `POST /v1/tasks` only accepts `reference_file_refs` for reference images.
  - Any create payload containing `reference_images` now fails with `400 INVALID_REQUEST`.
- Formal ref acquisition path:
  - `POST /v1/task-create/asset-center/upload-sessions`
  - upload bytes with the returned remote plan
  - `POST /v1/task-create/asset-center/upload-sessions/{session_id}/complete`
  - take `reference_file_ref` from the completion response and place it into `reference_file_refs`
- Ref validation is now backend-enforced on create:
  - ref must exist
  - ref must come from the task-create asset-center owner space
  - its upload request must be `reference`, `bound`, and `completed`
  - owner must match `creator_id`
- New creates no longer push base64 image payloads into `reference_images_json`, so the old `Data too long for column 'reference_images_json'` create-tx failure path is closed.

## 2026-03-31 owner_team create compatibility closure
- Keep this boundary fixed:
  - `/v1/org/options` remains the account department/team truth source
  - task `owner_team` remains the task-side legacy compatibility field
  - this round did not merge those models and did not rewrite historical `tasks.owner_team`
- New create-time bridge now exists in MAIN:
  - legacy task owner-team values still pass directly
  - supported org-team values with deterministic task mappings are normalized at create time into legacy task owner-team values
  - unsupported team strings still fail with `INVALID_REQUEST` + `violations[].code=invalid_owner_team`
- owner_team compatibility guardrail:
  - `/v1/org/options` teams must not be treated as task `owner_team` values automatically
  - create-time compatibility is now fixed by an explicit mapping list in `service/task_owner_team.go`, not by deriving every org team from department truth
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
  - this remains compatibility only; it is not a full org-model unification
- Code anchors:
  - legacy owner-team source: `domain.DefaultDepartmentTeams`, `domain.ValidTeam`
  - create-time compat bridge: `service.normalizeOwnerTeamForTaskCreate`
  - read-only compat mapping helper: `service.ListTaskOwnerTeamCompatMappings()`
  - create validation remains in `service.validateCreateTaskEntry`
- Added create-path debug log fields:
  - `trace_id`
  - `task_type`
  - `raw_owner_team`
  - `normalized_owner_team`
  - `owner_team_mapping_applied`
  - `mapping_source`
- Local verification for this round passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
- Release/deploy closure:
  - command: `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 owner_team create compatibility fix"`
  - live target remained overwrite-published `v0.8`
  - runtime health after publish:
    - `8080 /health` = `200`
    - `8081 /health` = `200`
    - `8082 /health` = `200`
  - active exe pointers:
    - `8080` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
    - `8081` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
    - `8082` -> `/root/ecommerce_ai/erp_bridge_sync`
  - no active executable was in `(deleted)` state
- Live acceptance summary:
  - `original_product_development` + defer-local-binding + `owner_team="运营三组"` -> `201`, task `150`
  - `new_product_development` + `owner_team="运营三组"` -> `201`, task `151`
  - `purchase_task` + `owner_team="运营三组"` -> `201`, task `152`
  - invalid team (`不存在的组`) still returned `400 INVALID_REQUEST` with `field=owner_team` and `code=invalid_owner_team`
- Live log proof:
  - compat requests logged `mapping_source=org_team_compat`
  - illegal requests logged `mapping_source=invalid`

## 2026-03-31 owner_team compatibility guardrail hardening
- Keep this boundary fixed:
  - task-side `owner_team` is still the legacy compatibility field
  - `/v1/org/options` is still the account-org source only
  - this is still **not** a full org-model unification
- Runtime guardrail now tightened:
  - `service/task_owner_team.go` uses an explicit fixed compat mapping list instead of deriving acceptance from the org tree
  - a newly added org team will no longer become task-create-valid unless a mapping is added intentionally
  - read-only mapping introspection is available through `service.ListTaskOwnerTeamCompatMappings()`
- Fixed compat mapping samples:
  - `运营一组` -> `内贸运营组`
  - `运营三组` -> `内贸运营组`
  - `运营七组` -> `内贸运营组`
  - `定制美工组` -> `设计组`
  - `设计审核组` -> `设计组`
  - `采购组` -> `采购仓储组`
  - `仓储组` -> `采购仓储组`
  - `烘焙仓储组` -> `采购仓储组`
- Required regression intent for future changes:
  - any new org team that should be accepted by task create must also add an explicit compat mapping
  - the same change must add or update regression coverage
  - `/v1/org/options` output must never be auto-reused as task `owner_team` validation truth
- Verification closure for this hardening round:
  - `go test ./service -run "OwnerTeam|OriginalProductWithOrgTeamCompatOwnerTeamPasses|NewProductWithOrgTeamCompatOwnerTeamPasses|PurchaseTaskWithOrgTeamCompatOwnerTeamPasses"`
  - `go test ./transport/handler -run "OwnerTeam"`
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - overwrite publish command: `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 owner_team compatibility guardrail hardening"`
  - live health remained `200` on `8080`, `8081`, `8082`
  - live compat create with `owner_team="运营三组"` still succeeded: task `156`, normalized owner-team `内贸运营组`
  - live invalid create with `owner_team="不存在的组"` still failed with `INVALID_REQUEST` and `invalid_owner_team`
## 2026-04-01 task assign/reassign status-gating closure
- Root cause that matters for handover:
  - live task `170` was already `InProgress`, with `designer_id=41`, `current_handler_id=41`
  - old `/v1/tasks/{id}/assign` had no reassign branch
  - both `service/task_action_rules.go` and `service/task_assignment_service.go` effectively forced assign to `PendingAssign` only
- Current runtime rule:
  - `PendingAssign` -> semantic action `assign`
    - existing Ops/management org-scoped rule stays intact
    - success writes `designer_id` + `current_handler_id`
    - status becomes `InProgress`
  - `InProgress` -> semantic action `reassign`
    - only management roles may act:
      - `Admin`
      - `SuperAdmin`
      - `RoleAdmin`
      - `HRAdmin`
      - `DepartmentAdmin`
      - `TeamLead`
      - `DesignDirector`
    - canonical org scope must still match
    - success writes `designer_id` + `current_handler_id`
    - status stays `InProgress`
  - audit / warehouse / close style states remain denied with `deny_code=task_not_reassignable`
- Code anchors for this round:
  - `service/task_action_rules.go`
  - `service/task_action_authorizer.go`
  - `service/task_assignment_service.go`
  - `domain/audit.go`
- New live evidence after overwrite-publish to existing `v0.8`:
  - task `170`:
    - `TeamLead(运营三组)` -> `/assign` returned `403`, `deny_code=task_out_of_team_scope`
    - `TeamLead(运营一组)` -> `/assign` to designer `42` returned `200`, detail then showed `assignee_id=42`, `current_handler_id=42`, status stayed `InProgress`
    - same actor then reassigned task `170` back to designer `41`
  - task `169`:
    - before `PendingAssign`
    - admin assign to designer `42` returned `200`
    - follow-up detail showed `task_status=InProgress`, `assignee_id=42`, `current_handler_id=42`
  - task `165` (`PendingAuditA`):
    - admin `/assign` returned `403`, `deny_code=task_not_reassignable`
- Event/log anchors:
  - `task_event_logs` now carry:
    - task `169` sequence `3` -> `task.assigned`
    - task `170` sequences `4` and `5` -> `task.reassigned`
  - server log now carries:
    - `task_action_auth action=assign|reassign ...`
    - `task_assignment trace_id=... task_id=... action=assign|reassign ... previous_designer_id=... new_designer_id=... previous_status=... resulting_status=... allow=... deny_reason=...`
- Local verification status:
  - passed:
    - `go test ./service ./transport/handler`
    - `go build ./cmd/server`
    - `go build ./repo/mysql ./service ./transport/handler`
  - blocked by host policy:
    - `go test ./repo/mysql`
    - reason: local Application Control blocked `mysql.test.exe`
- Publish record:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 task assign reassign status gating fix"`
  - release history deploy completion: `2026-04-01T07:16:42Z`
- Scope boundary kept explicit:
  - no standalone `/reassign` route yet
  - no reopen/reassign support for audit / warehouse / close states
  - still not full task-action ABAC

## 2026-04-02 handover: task product-code convergence and `rule_templates/product-code` deprecation
- What was audited:
  - exact code search in runtime paths for:
    - `rule_templates`, `rule-templates`, `product-code`, `cost-pricing`, `short-name`
    - `code-rules`, `generate-sku`, `template_key`, `rule_template`, `code_rule`
- Audit result:
  - `rule_templates` route/repo/service still exists.
  - `product-code` template is no longer an active runtime configuration key:
    - excluded from list output
    - `GET/PUT /v1/rule-templates/product-code` explicitly return deprecation errors
  - `code-rules` remains as a separate module, but create-task product-code generation for new/purchase tasks is now backend-default and no longer frontend-selected.
- Final decision:
  - keep `rule-templates` compatibility surface for `cost-pricing` and `short-name`
  - deprecate `rule_templates/product-code` usage
  - converge task product-code generation to backend default allocator

### Default rule and scope
- Rule: `NS + category_code + 6-digit sequence`
- Example: `NSKT000000`
- Applied when create task type is:
  - `new_product_development`
  - `purchase_task`
- Not applied to:
  - `original_product_development` existing-product lane

### Storage and naming note
- In current MAIN task domain, generated "product code" is written into `sku_code`.
- For batch tasks, per-line values are in `task_sku_items.sku_code` and exposed via `sku_items[].sku_code`.

### Uniqueness guarantee implementation
- New migration: `048_v7_product_code_sequences.sql`
- New table: `product_code_sequences`
  - unique key: `(prefix, category_code)`
  - `next_value` allocator by category
- Allocation path:
  - tx + upsert row + `SELECT ... FOR UPDATE` + range increment
- Protection layers:
  - no duplicate within one request/batch payload
  - no duplicate across concurrent requests via row-level locking
  - no duplicate across tasks/types in same namespace by shared allocator
  - final DB unique guard at `task_sku_items.uq_task_sku_items_sku_code`

### API contract changes
- Existing create (`POST /v1/tasks`):
  - frontend no longer needs code-rule/template selection for new/purchase SKU generation
  - when sku missing, backend auto-generates
  - batch mode auto-generates per child item
- New optional pre-generation endpoint:
  - `POST /v1/tasks/prepare-product-codes`
  - request: `task_type`, `category_code`, `count` or `batch_items[].category_code`
  - response: `codes[].index/category_code/sku_code`
- OpenAPI updated accordingly in `docs/api/openapi.yaml`.

### Frontend collaboration final wording
- Remove/disable frontend dependency on:
  - `rule_templates/product-code`
  - create-time code-rule selection for task SKU
- Preferred create flow:
  1. call `POST /v1/tasks` directly and read generated codes from response
  2. optionally call `POST /v1/tasks/prepare-product-codes` for pre-display
- Response read paths:
  - single task: `data.sku_code`
  - batch task: `data.sku_code`, `data.primary_sku_code`, `data.sku_items[].sku_code`

### Local verification in this round
- Passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
  - `go test ./repo/mysql`
  - `go test ./service -run TestTaskServicePrepareProductCodesBatchAndConcurrentUnique -count=1`
- Not available on this host:
  - `go test -race ...` (requires `CGO_ENABLED=1`)

### Release/online status
- No deploy/online acceptance executed in this iteration session.
- If promoted, keep overwrite publish on existing `v0.8` only.
