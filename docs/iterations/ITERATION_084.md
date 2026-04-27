# ITERATION_084

## Date
2026-03-19

## Goal
Overwrite-deploy the existing `v0.8` release so live keeps the same release name while picking up the original-product create-chain fixes:
- whitelist false-reject fix for `original_product_development`
- `design_requirement -> change_request` alias normalization
- original-path compatibility for `product_selection + defer_local_product_binding`
- machine-readable `invalid_fields` and `violations`

## Deployment Facts
- Local verification before deploy:
- `go test ./service/...` passed.
- `go build ./...` passed.
- Linux amd64 release binaries were rebuilt from `./cmd/server` as `ecommerce-api` and `erp_bridge`.
- Remote overwrite target remained `releases/v0.8`.
- Replaced binaries:
- `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
- `/root/ecommerce_ai/releases/v0.8/erp_bridge`
- Backup files created before overwrite:
- `/root/ecommerce_ai/releases/v0.8/ecommerce-api.bak.20260319092020`
- `/root/ecommerce_ai/releases/v0.8/erp_bridge.bak.20260319092020`
- `8082` binary was not replaced.

## Runtime Result
- 8080 restarted and now runs `/root/ecommerce_ai/releases/v0.8/ecommerce-api`.
- 8081 restarted and now runs `/root/ecommerce_ai/releases/v0.8/erp_bridge`.
- Existing `stop-bridge.sh` matched `erp_bridge_sync` and also stopped 8082 during restart; 8082 was then restored with `start-sync.sh`.
- Restored 8082 still runs the unchanged `/root/ecommerce_ai/erp_bridge_sync` binary.
- Final health checks after recovery:
- `http://127.0.0.1:8080/health` => `{"status":"ok"}`
- `http://127.0.0.1:8081/health` => `{"status":"ok"}`
- `http://127.0.0.1:8082/health` => `{"status":"ok"}`

## Minimal Live Verification
- Case 1: original create with only `design_requirement` alias and ERP snapshot defer path => `201`, trace `8e00e291-9e7a-4acb-8735-9824867fb7a8`.
- Case 2: original create with `is_outsource=true` alias => `201`, trace `58871bce-4c9b-4658-9aca-6ca1b5008a8d`, response `need_outsource=true`.
- Case 3: original create with `product_id=null` plus `product_selection.erp_product` and `defer_local_product_binding=true` => `201`, trace `d2ba2f8d-6d2b-46f9-b51b-0ea96ef81c2c`.
- Case 4: original create mixed with `material_mode`, `material`, `product_channel` => `400`, trace `50cacb39-8d4e-46ba-9ba2-5d372ce58785`, `code=INVALID_REQUEST`, `message=task_type field whitelist validation failed`, `details.invalid_fields`, `violations`.

## Version Position
- This iteration is a `v0.8` overwrite deployment record only.
- It is not `v0.9`.
