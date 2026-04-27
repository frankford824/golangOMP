# ITERATION_115

**Date:** 2026-04-08  
**Goal:** Re-verify and freeze the final single-domain `https://yongbo.cloud` upload/download access strategy so office-allowed sources can use NAS large-file lanes while external sources are explicitly denied, without breaking external reference-small upload.

## Scope

- Re-investigate the current `source_ip` resolution chain inside MAIN.
- Confirm live `yongbo.cloud` Nginx forwarding behavior.
- Capture real office and external source samples instead of relying only on assumptions.
- Re-verify:
  - office-allowed multipart create
  - external multipart explicit deny
  - external `reference-upload` allow
  - private-network download office allow / external deny
- Sync truth-source docs only. No new runtime code path was required in this iteration.

## Confirmed source-IP chain

- MAIN upload/download gate currently resolves request source in this order:
  - `X-Real-IP`
  - `X-Forwarded-For`
    - current code reads the last valid token
  - `RemoteAddr`
- Current live Nginx for `yongbo.cloud` forwards `/v1` with:
  - `Host $host`
  - `X-Real-IP $remote_addr`
  - `X-Forwarded-For $proxy_add_x_forwarded_for`
  - `X-Forwarded-Proto $scheme`
- Because Nginx always sets `X-Real-IP`, live classification is effectively based on `X-Real-IP`.

## Root cause of the historical misclassification

- The problem was not that Nginx failed to pass the client IP.
- The problem was not that frontend called the wrong API.
- The real office browser source reached MAIN as public egress `222.95.254.125`, not as `192.168.x.x`.
- Before live runtime env was populated with office public allowlist, MAIN only trusted private CIDRs, so office traffic was denied as external.
- Historical live-deny evidence remained in logs:
  - `2026-04-07 14:20:38 +0800` `source_ip="222.95.254.125"` `allowed=false`
  - `2026-04-08 10:31:29 +0800` `source_ip="222.95.254.125"` `allowed=false`

## Final fixed strategy

- Keep the only browser entry:
  - `https://yongbo.cloud`
- Allow multipart/private-network large-file lanes when source IP matches either:
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_CIDRS`
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_IPS`
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_CIDRS`
- Live runtime currently uses:
  - `UPLOAD_SERVICE_BROWSER_DIRECT_ACCESS_ALLOWED_PUBLIC_IPS=222.95.254.125`
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
- Keep `POST /v1/tasks/reference-upload` outside this large-file gate so external users can still upload small reference images.
- Keep private-network download aligned with the same source-IP allow/deny rule.

## Live evidence captured in this iteration

### Office/intranet-allowed sample

- Real office/public-egress sample observed in live logs:
  - `222.95.254.125`
- Nginx access examples:
  - `2026-04-08 13:37:07 +0800` `POST /v1/tasks/reference-upload` -> `201`
  - `2026-04-08 13:38:31 +0800` `POST /v1/tasks/371/asset-center/upload-sessions` -> `201`
- MAIN gate logs now show:
  - `source_ip="222.95.254.125"` `allowed=true` `reason="source_x_real_ip_matched_allowed_public"`

### External real sample

- Real external verification host:
  - `openclaw`
  - public IP `8.222.174.253`
- Real external multipart probe:
  - `POST https://yongbo.cloud/v1/tasks/375/asset-center/upload-sessions/multipart`
  - result:
    - `403`
    - `error.code=UPLOAD_ENV_NOT_ALLOWED`
    - `details.source_ip=8.222.174.253`
    - no NAS private multipart plan returned
- Real external reference-small probe from the same host:
  - `POST https://yongbo.cloud/v1/tasks/reference-upload`
  - result:
    - `201`
    - returned valid reference file ref object

## Focused live verification results

### Multipart create

- Office-allowed replay against live MAIN with `X-Real-IP: 222.95.254.125` on task `375`:
  - `201`
  - returned NAS private direct-upload plan:
    - `remote.base_url=http://192.168.0.125:8089`
    - `part_upload_url_template=http://192.168.0.125:8089/upload/sessions/.../parts/{part_no}`
    - `complete_url=http://192.168.0.125:8089/upload/sessions/.../complete`
    - `abort_url=http://192.168.0.125:8089/upload/sessions/.../abort`
- Real external call from `8.222.174.253`:
  - `403`
  - `UPLOAD_ENV_NOT_ALLOWED`

### Reference small upload

- Real external call from `8.222.174.253`:
  - `POST /v1/tasks/reference-upload`
  - `201`
  - returned `asset_id/ref_id/upload_request_id/public_url`

### Download gate

- Task `375` source asset `58` is a real live private-network source asset:
  - `asset_type=source`
  - `source_access_mode=private_network_only`
  - `download_mode=private_network`
- Office-allowed replay (`X-Real-IP: 222.95.254.125`):
  - `GET /v1/tasks/375/asset-center/assets/58/download`
  - `200`
  - returned controlled download info (`lan_url` / `tailscale_url`, no public URL)
- External replay (`X-Real-IP: 8.8.8.8`):
  - same route returned `403`
  - `error.code=UPLOAD_ENV_NOT_ALLOWED`

### Existing downstream workflow evidence

- Task `373` still shows live completed multipart delivery asset persisted:
  - `design_assets[0].scope_sku_code=NSKT000070`
  - `asset_versions[0].upload_status=uploaded`
- Task `373` current status is:
  - `PendingAuditA`
- This confirms the large-file lane can complete upload and continue into downstream workflow states under the same single-domain entry.

## Files updated in this iteration

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `ITERATION_INDEX.md`
- `docs/iterations/ITERATION_115.md`

## Deploy / runtime result

- No new runtime code/config change was necessary in this iteration.
- No new overwrite deploy was performed.
- Live runtime already remained on existing `v0.8`:
  - main `8080` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - bridge `8081` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - sync `8082` -> `/root/ecommerce_ai/erp_bridge_sync`
- Runtime health confirmed:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`

## Remaining risks / unfinished

- Current live allowlist contains one confirmed office egress IP:
  - `222.95.254.125`
- If company network later adds or changes public egress IP/CIDR, `shared/main.env` must be updated or office users on the new egress will again be misclassified as external.
- Current source-IP precedence trusts `X-Real-IP` first because current deployment is single Nginx reverse proxy.
- If a CDN/WAF/extra proxy is introduced later, trusted-proxy / real-IP handling must be revisited before assuming the current precedence is still safe.
