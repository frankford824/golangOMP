# ITERATION_076

## Goal

Strictly complete one thing this round:

1. wire `8081` Bridge remote write path to Jushuitan official OpenWeb with the already-validated signing/credential rules from `8082`
2. finish real remote acceptance in `hybrid` mode and preserve `v0.4` runtime safety

## Scope

- No frontend changes
- No unrelated optimization/refactor
- No version bump (still `v0.4`)
- Keep `8080/8081/8082` stable throughout

## Confirmed Facts

- `8081` remote write path now uses OpenWeb signing:
  - `sign = md5(app_secret + sorted(key+value))`
  - signed keys include `app_key/access_token/timestamp/charset/version/biz` (exclude `sign`)
- `8081` write route mapping to official OpenWeb is explicit in code/config/docs:
  - `POST /v1/erp/products/upsert` -> `/open/webapi/itemapi/itemsku/itemskubatchupload`
  - `POST /v1/erp/products/shelve/batch` -> `/open/webapi/wmsapi/openshelve/skubatchshelve`
  - `POST /v1/erp/products/unshelve/batch` -> `/open/webapi/wmsapi/openoffshelve/skubatchoffshelve`
  - `POST /v1/erp/inventory/virtual-qty` -> `/open/webapi/itemapi/iteminventory/batchupdatewmsvirtualqtys`
- Live bridge was switched to:
  - `ERP_REMOTE_MODE=hybrid`
  - `ERP_REMOTE_BASE_URL=https://openapi.jushuitan.com`
  - `ERP_REMOTE_AUTH_MODE=openweb`
- Real remote hit evidence is confirmed:
  - bridge logs include `remote_erp_openweb_request_completed`
  - target URL is official OpenWeb upsert endpoint
  - response status `200`
- Real hybrid fallback evidence is confirmed:
  - shelve batch got OpenWeb business rejection `code=190` (missing API permission)
  - bridge logs include `erp_remote_shelve_batch_failed_fallback_local`
  - then `erp_remote_shelve_batch_fallback_local_success`
  - API response to caller remains `200` with local `sync_log_id`
- Permission model re-verification after remote/hybrid integration:
  - `Admin` ERP read/write: `200`
  - `Ops` ERP read/write: `200`
  - roleless ERP read/write: `403`
- Post-rollout service/runtime safety is intact:
  - `8080` pid `3515875`, health OK
  - `8081` pid `3515839`, health OK
  - `8082` pid `3516136`, health OK
  - service executable links are not deleted

## Inferences

- OpenWeb remote writeback is now truly connected for bridge upsert path (official endpoint reached and returned success).
- Remaining write capabilities are currently limited by upstream API authorization scope, not by local signing/wiring.
- `hybrid + fallback` is the correct live strategy under current external permission constraints.

## Verification Steps

1. Reused/ported OpenWeb signing and request assembly rules from `8082` reference implementation.
2. Implemented OpenWeb auth mode in bridge remote client and bound route-level write operations to official OpenWeb paths.
3. Added unit tests for OpenWeb sign generation and biz-payload mapping in bridge service package.
4. Updated bridge env examples and config defaults for OpenWeb/hybrid deployment.
5. Deployed with existing `v0.4` rollout flow (no version change).
6. Ran real-token acceptance calls against live `8081`:
   - verified remote upsert hits official OpenWeb and succeeds
   - verified shelve remote failure path falls back locally in hybrid mode
7. Re-ran minimal regression:
   - `8081` health/read/write + role grading
   - `8080` health/tasks
   - `8082` health

## External Remote Acceptance Status

- Attempted: yes
- Completed: yes (for official remote connectivity and at least one successful write path)
- Current limitation:
  - some write APIs are subject to upstream OpenWeb permission scope (`code=190` observed on shelve batch in this round)
- Live final mode after this round: `hybrid`

## Files Changed

- `service/erp_bridge_remote_client.go`
- `service/erp_bridge_service.go`
- `service/erp_bridge_remote_client_openweb_test.go`
- `config/config.go`
- `cmd/server/main.go`
- `deploy/bridge.env.example`
- `dist/ecommerce-ai-v0.4-linux-amd64/bridge.env.example`
- `docs/api/openapi.yaml`
- `CURRENT_STATE.md`
- `MODEL_v0.4_memory.md`
- `MODEL_HANDOVER.md`
- `docs/iterations/ITERATION_076.md`
- `ITERATION_INDEX.md`

