# ITERATION_074

## Goal

Close the last live-runtime gap for the current `8080 MAIN + 8081 Bridge + 8082 JST sync` coexistence mode by:

1. completing real bearer-token acceptance for all `8081 /v1/erp/*` Bridge routes
2. adding a reusable post-release three-service runtime check with `8082` auto-recovery

## Scope

- No frontend changes
- No unrelated business-logic refactor
- No version bump
- Keep `8082` as an independent resident JST sync service

## Confirmed Facts

- Live host verified through SSH localhost probing: `223.4.249.11`
- `POST /v1/auth/login` issued a real bearer token; `GET /v1/auth/me` returned `200`
- `8081` success-path acceptance completed with real token:
  - `GET /v1/erp/products`
  - `GET /v1/erp/products/{id}`
  - `GET /v1/erp/categories`
  - `POST /v1/erp/products/upsert`
  - `GET /v1/erp/sync-logs`
  - `GET /v1/erp/sync-logs/{id}`
  - `POST /v1/erp/products/shelve/batch`
  - `POST /v1/erp/products/unshelve/batch`
  - `POST /v1/erp/inventory/virtual-qty`
- Real acceptance test objects:
  - `product_id = bridge-accept-1773724718`
  - `sku_id = bridge-accept-sku-1773724718`
  - `sku_code = BRIDGE-ACCEPT-1773724718`
  - `sync_log_id = 5 / 6 / 7 / 8`
- Failure-path acceptance also completed:
  - unauthenticated read/write -> `401`
  - invalid page / empty payload -> `400`
  - missing product / missing sync log -> `404`
- Bridge runtime log evidence exists in `/root/ecommerce_ai/logs/erp_bridge-20260317T042205Z.log`
- Current deploy-flow gap root cause is confirmed:
  - `deploy/remote-deploy.sh` cutover `--start-services` only manages `8080/8081`
  - prior packaged helper list did not carry `start-sync.sh` / `stop-sync.sh`
  - prior `deploy/verify-runtime.sh` did not inspect or recover `8082`
- New runtime scripts were added:
  - `deploy/check-three-services.sh`
  - `deploy/start-sync.sh`
  - `deploy/stop-sync.sh`
- `deploy/verify-runtime.sh` now invokes the three-service check
- `deploy/deploy.sh` cutover post-verify now passes `--auto-recover-8082`
- Real recovery drill completed:
  - stopped `8082`
  - confirmed `/health` -> `000`
  - ran `check-three-services.sh --auto-recover-8082`
  - script reported `SYNC_RECOVER_TRIGGERED=true`
  - script reported `SYNC_RECOVER_SUCCESS=true`
  - `8082` restarted as pid `3498341`
  - after recovery `/health`, `/internal/jst/ping`, `/jst/sync/inc` all returned `200`

## Inferences

- The current safest release integration mode is to keep `8082` out of the cutover start/stop branch and instead enforce a post-release runtime check plus targeted recovery. This avoids changing the already-working `8080/8081` cutover semantics.
- Parallel deploy verification should skip the three-service checker, otherwise a candidate-port verification could fail because of unrelated live-service drift.

## Verification Steps

1. Logged in on live host and captured a real bearer token
2. Executed all `8081 /v1/erp/*` success-path calls with that token
3. Executed representative `401` / `400` / `404` failure-path checks
4. Confirmed Bridge `http_request` log lines for the exercised routes
5. Added three-service runtime scripts and wired them into deploy verification
6. Uploaded updated runtime scripts to `/root/ecommerce_ai/scripts`
7. Ran `check-three-services.sh --base-dir /root/ecommerce_ai --auto-recover-8082` under normal healthy state
8. Stopped `8082` with `stop-sync.sh`
9. Re-ran the three-service checker and confirmed automatic restart
10. Re-checked `8082` health, JST ping, and incremental sync trigger
11. Re-ran `verify-runtime.sh` to confirm integrated post-release flow behavior

## Files Changed

- `deploy/check-three-services.sh`
- `deploy/start-sync.sh`
- `deploy/stop-sync.sh`
- `deploy/verify-runtime.sh`
- `deploy/deploy.sh`
- `deploy/lib.sh`
- `CURRENT_STATE.md`
- `MODEL_v0.4_memory.md`
- `MODEL_HANDOVER.md`
- `docs/iterations/ITERATION_074.md`
- `ITERATION_INDEX.md`

## Result

- `8081` is no longer only “auth-gated and mounted”; all current ERP query/write/log routes have real token-backed acceptance evidence.
- `8082` now has a repeatable post-release runtime check and targeted self-recovery path.
- The live `8080/8081/8082` coexistence shape is now in a maintainable operational state.
