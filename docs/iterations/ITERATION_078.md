# ITERATION_078

## Goal

Align Bridge(8081) with current real ERP business semantics and complete live acceptance for:

1. semantic chain alignment: `sku_id / i_id / name / short_name / wms_co_id`
2. dual write-path support:
   - product profile upsert (`itemskubatchupload`)
   - item style update (`itemupload`)
3. warehouse dimension support (`wms_co_id`) and 11-warehouse list contract
4. keep live `v0.4` stable across `8080/8081/8082` (no version bump)

## Confirmed Facts

- Live deployment was rebuilt and redeployed in place with `--version v0.4` (no version bump).
- Post-deploy runtime check passed:
  - `8080` health `200`, pid `3546054`, exe not deleted
  - `8081` health `200`, pid `3546082`, exe not deleted
  - `8082` health `200`, pid `3546261`, exe not deleted
- New Bridge routes are now live (post-deploy verification):
  - `POST /v1/erp/products/style/update` no longer 404; Admin/Ops return `200`
  - `GET /v1/erp/warehouses` no longer 404; Admin/Ops return `200`
- `upsert` write acceptance shows aligned semantic fields in live response:
  - `sku_id`, `i_id`, `name`, `short_name`, `s_price`, `wms_co_id`
  - `route=itemskubatchupload`
- `item_style_update` write acceptance is live and returns:
  - `sku_id`, `i_id`, `name`, `short_name`
  - `route=itemupload`
- Warehouse list contract now returns all 11 required warehouses with `wms_co_id`.
- Batch mutation payloads in live sync logs now carry warehouse-context fields (`wms_co_id`, `bin_id`, `carry_id`, `box_no`) for shelve/unshelve/virtual-qty paths.
- Role-based access remains correct after rollout:
  - Admin: ERP read/write `200`
  - Ops: ERP read/write `200`
  - Roleless: ERP read/write `403`
- Current write behavior in live `hybrid`:
  - upsert: remote route behavior visible (`route=itemskubatchupload`)
  - item_style_update: remote route behavior visible (`route=itemupload`)
  - shelve/unshelve/virtual-qty: accepted with local fallback evidence (`message=stored locally`, sync_log_id generated)

## Inferences

- The Bridge semantic contract is now exposed end-to-end for MAIN consumption, including the newly required style-update and warehouse-listing capabilities.
- Remaining uncertainty for full remote success of `shelve/unshelve/virtual-qty` is still constrained by upstream business prerequisites (warehouse slot/container/effective inventory payload constraints), not by route availability.
- Keeping live in `hybrid` with fallback remains the safest mode for current production stability.

## Verification Steps

1. Ran local acceptance script before deploy and confirmed old live mismatch (`style/update` and `warehouses` were 404).
2. Rebuilt and redeployed to live with:
   - `bash ./deploy/deploy.sh --version v0.4 --release-note "..."`
3. Verified three-service runtime + process safety (`health`, pid, listener, `/proc/<pid>/exe`).
4. Re-ran acceptance script after deploy and confirmed:
   - style-update route `200` (Admin/Ops)
   - warehouses route `200` (Admin/Ops) with 11 rows
   - roleless permission boundary `403`
   - write/read route health unchanged for 8080/8081/8082.

## Files Changed

- `docs/iterations/ITERATION_078.md`
- `ITERATION_INDEX.md`
- `CURRENT_STATE.md`
- `MODEL_v0.4_memory.md`
- `MODEL_HANDOVER.md`
- `docs/api/openapi.yaml`
- `tmp/iteration_078_acceptance_postdeploy.json`
