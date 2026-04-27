# ITERATION_077

## Goal

Only continue one mainline:

1. close the remaining Bridge(8081) OpenWeb write acceptance for:
   - `POST /v1/erp/products/shelve/batch`
   - `POST /v1/erp/products/unshelve/batch`
   - `POST /v1/erp/inventory/virtual-qty`
2. keep `hybrid` stable and observable
3. keep `8080/8081/8082` live-safe under `v0.4` (no version bump)

## Confirmed Facts

- Runtime topology remains stable and healthy after deploy:
  - `8080` pid `3527927`, health `200`
  - `8081` pid `3527959`, health `200`
  - `8082` pid `3528131`, health `200`
  - three-service check reports `exe_deleted=false`
- Bridge live mode remains:
  - `ERP_REMOTE_MODE=hybrid`
  - `ERP_REMOTE_BASE_URL=https://openapi.jushuitan.com`
  - `ERP_REMOTE_AUTH_MODE=openweb`
- OpenWeb mapping remains:
  - upsert -> `/open/webapi/itemapi/itemsku/itemskubatchupload`
  - shelve batch -> `/open/webapi/wmsapi/openshelve/skubatchshelve`
  - unshelve batch -> `/open/webapi/wmsapi/openoffshelve/skubatchoffshelve`
  - virtual qty -> `/open/webapi/itemapi/iteminventory/batchupdatewmsvirtualqtys`
- Code-side fix completed for OpenWeb biz payload shape:
  - shelve/unshelve now send `items` structure (and keep `sku_codes` compatibility field)
  - virtual qty now sends richer list item fields (`sku_id`, `virtual_qty`, `qty`, optional warehouse fields)
- Code-side fix completed for virtual qty false-positive success:
  - OpenWeb body `msg=未获取到有效的传入数据` (even when `code=0`) is now treated as business rejection
  - in hybrid mode, this now triggers fallback-local instead of being reported as remote success
- New observability log point added:
  - `remote_erp_openweb_request_started`
- Real upstream responses after fix are captured:
  - shelve:
    - official URL hit: `/open/webapi/wmsapi/openshelve/skubatchshelve`
    - response: `code=100`, `msg=上架仓位不能为空`
  - unshelve:
    - official URL hit: `/open/webapi/wmsapi/openoffshelve/skubatchoffshelve`
    - response: `code=100`, `msg=指定箱不存在`
  - virtual qty:
    - official URL hit: `/open/webapi/itemapi/iteminventory/batchupdatewmsvirtualqtys`
    - raw upstream body includes `code=0`, `msg=未获取到有效的传入数据`, `data=null`
    - bridge now classifies this as remote business rejection and falls back locally
- Hybrid fallback path is confirmed for all three remaining writes:
  - `erp_remote_*_failed_fallback_local`
  - `erp_remote_*_fallback_local_success`
  - client response remains `200` with local `sync_log_id`, `message=stored locally`
- Role boundary remains intact after this round:
  - Admin/Ops: ERP read/write routes `200`
  - Roleless: ERP read/write routes `403`

## Inferences

- Remaining blockers are no longer generic "remote not wired"; they are upstream business constraints per API:
  - shelve currently blocked by required shelve-location/slot context (`上架仓位不能为空`)
  - unshelve currently blocked by target container/box context (`指定箱不存在`)
  - virtual qty currently blocked by upstream accepting request but returning "no valid input data"
- Upsert remote remains formally verified and stable.
- Under current upstream constraints, `hybrid` remains the correct live mode.

## Verification Steps

1. Updated Bridge OpenWeb biz mapping and virtual-response validation logic.
2. Added/updated unit tests for OpenWeb biz generation and validation.
3. Deployed to live with `--version v0.4` (no version change).
4. Re-ran remote acceptance against live `8081` with real session tokens:
   - read routes (`products`, `categories`, `sync-logs`, `sync-log detail`)
   - write routes (`upsert`, `shelve`, `unshelve`, `virtual-qty`)
   - multi-role checks (Admin, Ops, roleless)
5. Captured latest Bridge log markers for:
   - `remote_erp_openweb_request_started`
   - `remote_erp_openweb_request_completed`
   - `remote_erp_openweb_business_error`
   - fallback trigger/success logs

## External Remote Acceptance Status (Remaining Writes)

- upsert: remote success confirmed (`status_code=200`)
- shelve batch: remote blocked by upstream business rule (`code=100`, `上架仓位不能为空`) -> fallback local
- unshelve batch: remote blocked by upstream business rule (`code=100`, `指定箱不存在`) -> fallback local
- virtual qty: remote returns non-effective payload semantics (`code=0`, `msg=未获取到有效的传入数据`) -> bridge classifies as reject -> fallback local

## Files Changed

- `service/erp_bridge_remote_client.go`
- `service/erp_bridge_remote_client_openweb_test.go`
- `docs/iterations/ITERATION_077.md`
- `ITERATION_INDEX.md`
- `CURRENT_STATE.md`
- `MODEL_v0.4_memory.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `tmp/iteration_077_remote_acceptance.json`
