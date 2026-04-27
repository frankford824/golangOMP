# ITERATION_089

Title: Reference-small archival, multipart `remote.headers/X-Internal-Token` archival, and `v0.8` overwrite deploy closure

Date: 2026-03-20
Model: GPT-5 Codex

## 1. Background
- This iteration closed the current MAIN backend round under the fixed three-endpoint model:
  - local MAIN engineering workspace
  - server live `jst_ecs`
  - NAS `synology-dsm`
- Scope for this round was backend-only:
  - verify MAIN server-side code
  - run required tests and real package/deploy entrypoints
  - overwrite-publish to `v0.8`
  - archive the final rules into handover documents
- Frontend code was not in the local workspace and no frontend local build/change was part of this iteration.

## 2. Three-endpoint definition and collaboration principle
- Local MAIN repo is the only control plane.
- All coordination for server live and NAS direct-upload behavior is driven from the local MAIN workspace.
- Windows local control node must not enable SSH multiplexing:
  - `ControlMaster`
  - `ControlPersist`
  - `ControlPath`
- Current upload split remains:
  - `reference = small`
  - `delivery = multipart`
  - `source = multipart`
  - `preview = multipart`
- Current host split remains:
  - browser multipart host = `http://192.168.0.125:8089`
  - MAIN server-to-server / probe host = `http://100.111.214.38:8089`

## 3. Reference-small fix conclusion
- The previously archived `reference-upload` 0-byte issue remains closed.
- Final rule is unchanged and was re-verified not to be accidentally broken in this round:
  - `task-create reference` small upload treats `/upload/files` return data as canonical
  - MAIN does not call NAS `complete`
  - MAIN probes stored object state
  - MAIN verifies stored size/hash
  - size/hash mismatch fails immediately
  - MAIN does not emit a successful ref for a bad landed object

## 4. Multipart fault phenomenon
- The failing path was the browser-direct multipart upload from MAIN session creation to NAS direct `PUT/abort`.
- Observed historical symptoms included:
  - browser-side `ERR_CONNECTION_RESET`
  - NAS `abort 401`
  - multipart create-session response missing the header needed for NAS internal auth

## 5. Narrowing process
- Investigation first suspected the overall three-endpoint contract or a MAIN/NAS contract drift.
- That suspicion was narrowed down because:
  - the reference-small path was already independently closed
  - the issue reproduced specifically on multipart direct-upload behavior
- Investigation then narrowed onto NAS CORS / Origin behavior.
- That explanation was still incomplete because it did not explain the repeated internal-auth failure pattern on multipart actions.
- Final narrowing showed the real missing contract field:
  - `remote.headers.X-Internal-Token` was absent from the `RemoteUploadSessionPlan` returned by MAIN.

## 6. Final root cause
- Final root cause was in MAIN backend session planning:
  - when MAIN created `RemoteUploadSessionPlan`, it did not inject `remote.headers.X-Internal-Token`
- Therefore the browser direct-upload path lacked the required internal-auth header for NAS multipart actions.

## 7. MAIN backend fix points
- MAIN backend fix was implemented in `service/upload_service_client.go`.
- `applyBrowserHeaders(plan)` now writes the browser-reused multipart headers into `RemoteUploadSessionPlan.Headers`.
- At minimum, the returned headers include:
  - `X-Internal-Token`
- `service/task_asset_center_service.go` keeps the `RemoteUploadSessionPlan` on the response chain.
- `transport/handler/task_asset_center.go` returns the service result without stripping `remote.headers`.
- Related tests were updated and passed:
  - `service/upload_service_client_test.go`
  - `service/task_asset_center_service_test.go`
- Reference-small logic in `service/task_create_reference_upload_service.go` was checked again and not regressed in this round.
- The backend intent stayed correct:
  - MAIN issues the upload plan
  - browser reuses `remote.headers`
  - MAIN does not proxy multipart file bytes on behalf of the browser
  - token is not hardcoded in frontend

## 8. Server `v0.8` overwrite deploy actions
- Real repository packaging/deploy entrypoints were used:
  - `bash ./deploy/package-local.sh --version v0.8`
  - `bash ./deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 multipart remote.headers x-internal-token archival closure"`
- The package was rebuilt locally and overwrite-published to:
  - `/root/ecommerce_ai/releases/v0.8`
- Deploy flow used the repository's existing upload/cutover/restart/verify path.
- Runtime verification after deploy confirmed the standard three-service state:
  - `8080` MAIN healthy
  - `8081` bridge healthy
  - `8082` sync healthy
- Release pointer and running MAIN binary were verified on live:
  - `/root/ecommerce_ai/current -> /root/ecommerce_ai/releases/v0.8`
  - running `ecommerce-api` points to `/root/ecommerce_ai/releases/v0.8/ecommerce-api`

## 9. Live verification results
- Required local backend test passed:
  - `go test ./service -run "TestUploadServiceClient|TestTaskAssetCenterServiceCreate"`
- Full package flow passed through `deploy/package-local.sh`, including repository test coverage.
- Live multipart create-session verification was executed against the MAIN live service after deploy.
- Verified live response sample showed:
  - `remote.base_url = "http://192.168.0.125:8089"`
  - `remote.headers = { "X-Internal-Token": "nas-upload-token-2026" }`
- Follow-up direct NAS calls using the returned header succeeded:
  - `PUT /parts/1 = 200`
  - `POST /abort = 200`
- Verified conclusion:
  - `hasXInternalToken: true`
  - new `v0.8` live behavior is effective

## 10. Current final rules
- Local MAIN repo remains the only control plane.
- Fixed three endpoints for the current round are:
  - local MAIN engineering workspace
  - server live `jst_ecs`
  - NAS `synology-dsm`
- Windows local control node must not use SSH multiplexing.
- `reference` continues to use `small`.
- `delivery/source/preview` continue to use `multipart`.
- Browser multipart direct-upload contract now explicitly requires MAIN to return `remote.headers`.
- Minimum required multipart browser header is:
  - `X-Internal-Token`
- The browser reuses the returned header set on:
  - `PUT part_upload`
  - `POST complete`
  - `POST abort`
- Token must not be hardcoded in frontend.

## 11. Next to-do
- Database testing
- Dirty-data cleanup
- Preparation for full manual regression testing
