# ITERATION_116

Title: Probe-driven NAS browser admission with signed attestation on existing `v0.8`

Date: 2026-04-08
Model: GPT-5 Codex

## Goal
- Keep scheme A:
  - same browser entry `https://yongbo.cloud`
  - intranet users still upload large files directly to NAS private address
  - external users must not receive unusable NAS private multipart/download plans
  - external small reference upload must still work
- Replace the admission truth source:
  - from office public-IP allowlist
  - to browser NAS probe + backend verification
- Do not regress:
  - `sku_items[].reference_file_refs`
  - `design_assets[].scope_sku_code`
  - `asset_versions[].scope_sku_code`
  - `/v1/org/options`
  - owner-team bridge
  - `frontend_access` / route-role alignment

## Why allowlist was no longer acceptable
- Office public egress IP is unstable and operationally brittle.
- Allowlist falsely denied real office users whenever egress changed.
- Allowlist also did not prove that the current browser could actually reach NAS.
- Frontend needed a pre-flight answer before requesting multipart session, otherwise external browsers still received unusable private-network endpoints.

## Implemented solution

### MAIN
- Added `network_probe.attestation` to multipart create contract.
- Added `X-Network-Probe-Attestation` to private-network download contract.
- Multipart/private download gate now validates:
  - probe presence
  - `reachable`
  - method/url match
  - freshness
  - success status
  - NAS-signed attestation
- Legacy CIDR/public-IP settings remain diagnostic-only in deny details and logs.
- Missing/forged success probe now returns:
  - `403`
  - `error.code=UPLOAD_ENV_NOT_ALLOWED`
  - no leaked `expected_probe_url`

### NAS upload service
- Added lightweight `GET /upload/ping`.
- Endpoint is browser-accessible and does not create upload resources.
- Successful probe returns:
  - `reachable`
  - `method`
  - `url`
  - `checked_at`
  - `status_code`
  - signed `attestation`
- If NAS attestation secret is not configured, probe returns failure instead of false success.

## Code changes

### MAIN repo
- `domain/task_asset_center.go`
  - added `NetworkProbeEvidence.Attestation`
  - audit payload only records `attestation_present`, not the token itself
- `transport/handler/network_probe_attestation.go`
  - added HMAC sign/verify helper for NAS probe attestation
- `transport/handler/upload_network_access.go`
  - switched gate from probe-report-only to signed-attestation validation
- `transport/handler/task_asset_center.go`
  - multipart create accepts `network_probe.attestation`
  - private download reads `X-Network-Probe-Attestation`
  - deny details now expose attestation diagnostics
- `config/config.go`
  - added `UPLOAD_SERVICE_BROWSER_PROBE_ATTESTATION_SECRET`
- `cmd/server/main.go`
- `cmd/api/main.go`
  - wired probe attestation secret into handler policy
- Tests:
  - `transport/handler/network_probe_attestation_test.go`
  - updated `transport/handler/task_asset_center_upload_policy_test.go`
  - updated `service/task_asset_center_service_test.go`
  - updated `config/config_test.go`

### NAS upload-service repo on `synology-dsm`
- `internal/handler/upload.go`
  - added `BrowserProbe`
- `internal/handler/probe_attestation.go`
  - added attestation signer
- `internal/handler/middleware.go`
  - `/upload/ping` exempted from internal token auth
- `internal/handler/router.go`
  - routed `/upload/ping`
- `internal/config/config.go`
  - added `BROWSER_PROBE_TOKEN_SECRET`
- `docker-compose.yml`
  - wired `BROWSER_PROBE_TOKEN_SECRET`
- Tests:
  - `internal/handler/probe_test.go`

## Local verification

### MAIN required commands
- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed
- `go test ./repo/mysql` -> passed

### MAIN additional checks
- `go test ./config` -> passed

### NAS local checks from local control node
- `go test ./internal/handler -run "BrowserProbe|CORS"` -> passed
- `go build ./cmd/server` -> passed
- `go test ./...` -> not fully runnable on this local host because existing sqlite tests require CGO and local environment is `CGO_ENABLED=0`

## Deploy

### NAS
- Synced local patched upload-service repo to:
  - `/volume1/homes/yongbo/asset-upload-service`
- Rebuilt/restarted with:
  - `/usr/local/bin/docker compose up -d --build --force-recreate`
- Live container status:
  - `asset-upload-service` up on `8089`

### MAIN
- Updated live env:
  - `/root/ecommerce_ai/shared/main.env`
  - `UPLOAD_SERVICE_BROWSER_PROBE_ATTESTATION_SECRET=<UPLOAD_SERVICE_BROWSER_PROBE_ATTESTATION_SECRET>`
- Overwrite deploy:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 probe-driven attested NAS browser probe gate"`
- Runtime verification after deploy:
  - `8080 /health = 200`
  - `8081 /health = 200`
  - `8082 /health = 200`
  - `/proc/<pid>/exe` healthy and not deleted

## Live acceptance

### Probe / multipart
- NAS `GET /upload/ping` on live NAS returned `200` with signed `attestation`.
- `POST /v1/tasks/375/asset-center/upload-sessions/multipart`
  - missing probe -> `403 probe_missing`
  - forged success probe without attestation -> `403 probe_attestation_missing`
  - valid attested probe -> `201`, `remote.base_url=http://192.168.0.125:8089`

### Download
- `GET /v1/tasks/375/asset-center/assets/58/download`
  - missing probe -> `403 probe_missing`
  - forged success probe without attestation -> `403 probe_attestation_missing`
  - valid attested probe headers -> `200`, `download_mode=private_network`

### External-safe small upload
- `POST /v1/tasks/reference-upload` still returned `201`

### SKU contract regression
- Existing live task detail still exposes:
  - `sku_items[].reference_file_refs`
  - `design_assets[].scope_sku_code`
  - `asset_versions[].scope_sku_code`

### Organization/owner-team regression
- `/v1/org/options` still returns `运营三组`.
- Live create re-verified with valid payload plus `owner_team="运营三组"` returned `201`.
- Returned ownership was normalized as intended:
  - `owner_team="内贸运营组"`
  - `owner_department="运营部"`
  - `owner_org_team="运营三组"`
- The earlier `invalid_owner_team` result was caused by local control-node script encoding corruption, not by live backend regression.

## Documentation updates
- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `ITERATION_INDEX.md`
- `docs/iterations/ITERATION_116.md`
- `docs/api/openapi.yaml`
- `docs/ASSET_UPLOAD_INTEGRATION.md`
- `deploy/main.env.example`

## Final boundary
- Main strategy is now probe-driven with NAS-signed attestation.
- Legacy allowlist is no longer the primary logic.
- Download chain was switched with the same probe/attestation model in this round.
- Owner-team bridge consistency remained intact in live runtime after re-verification.
