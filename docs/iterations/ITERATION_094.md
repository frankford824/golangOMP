# ITERATION_094

Title: MAIN self-test, overwrite publish to existing `v0.8`, live migration `046` closure, and batch-SKU task-create re-verification

Date: 2026-03-31
Model: GPT-5 Codex

## 1. Background and goal
- This round was explicitly a real delivery closure, not a code-only patch:
  - run required local self-test
  - build/package current MAIN backend
  - overwrite publish onto existing `v0.8`
  - verify 8080/8081/8082 runtime health
  - verify the batch-SKU task-create mainline live
- The release line was fixed:
  - no `v0.9`
  - no `v1.0`
  - live line remains overwrite-published `v0.8`
- Production entrypoint was also fixed:
  - `cmd/server/main.go`
  - `cmd/api` remained compatibility-only

## 2. Local verification before publish
- Required commands all passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
- Additional targeted regression also passed:
  - `go test ./service -run "TaskPRD|TaskBatch|Create"`
  - `go test ./transport/handler -run "TestTaskHandlerCreateParsesBatchItems|TestTaskHandlerCreateBatchErrorIncludesViolations|TestTaskHandlerCreateBatchResponseIncludesSKUItems"`
  - `go test ./repo/mysql -run "Test.*Task.*"`
- Migration and contract presence were confirmed locally:
  - migration file exists: `db/migrations/046_v7_task_batch_sku_items.sql`
  - `docs/api/openapi.yaml` already exposed:
    - `is_batch_task`
    - `batch_item_count`
    - `batch_mode`
    - `primary_sku_code`
    - `sku_generation_status`
    - `sku_items`

## 3. Packaging and publish action
- A packaging blocker was found first:
  - several `deploy/*.sh` files were still using CRLF
  - Linux bash failed inside the existing package flow
- This round normalized `deploy/*.sh` to LF and then re-used the repository deploy system.
- Local package command:
  - `bash ./deploy/package-local.sh --version v0.8 --skip-tests`
- Local package result:
  - artifact: `dist/ecommerce-ai-v0.8-linux-amd64.tar.gz`
  - SHA-256: `4baea8036dcf3e8f7cb4c41ac18416946210c002a383e0f3011323c476bae845`
  - `PACKAGE_INFO.json` still resolved entrypoint to `./cmd/server`
- Remote cutover target remained:
  - `/root/ecommerce_ai/releases/v0.8`

## 4. Why publish was not one clean wrapper call
- The repository `deploy/deploy.sh` wrapper started correctly but its Windows/WSL remote phase was unstable on this control node.
- This round still stayed on repository-managed deploy assets only:
  - local `deploy/package-local.sh`
  - packaged remote `deploy/remote-deploy.sh`
  - packaged remote `deploy/verify-runtime.sh`
- Result:
  - existing `v0.8` was overwritten in place
  - `8080` and `8081` binaries were replaced
  - `8082` binary was not replaced

## 5. First live verification failure and root cause
- Initial post-publish live acceptance was not falsely marked as success.
- Live result before schema fix:
  - single `new_product_development` create -> `500`
  - batch `new_product_development` create -> `500`
  - batch `purchase_task` create -> `500`
  - batch `original_product_development` reject -> expected `400`
- Live log evidence from `/root/ecommerce_ai/logs/ecommerce-api-20260331T012125Z.log`:
  - `create task: insert task: Error 1054 (42S22): Unknown column 'is_batch_task' in 'field list'`
- Conclusion:
  - current binary was live
  - current live DB schema was not
  - migration `046` had not yet been applied on server

## 6. Live schema closure in this round
- Backup created before mutation:
  - `/root/ecommerce_ai/backups/20260331T012734Z_task_batch_schema_046/tasks_procurement_before.sql`
- Applied live migration:
  - `/root/ecommerce_ai/releases/v0.8/db/migrations/046_v7_task_batch_sku_items.sql`
- This added:
  - `tasks.is_batch_task`
  - `tasks.batch_item_count`
  - `tasks.batch_mode`
  - `tasks.primary_sku_code`
  - `tasks.sku_generation_status`
  - `task_sku_items`
  - `procurement_record_items`

## 7. Runtime verification after overwrite + migration
- Health:
  - `8080 /health` = `200`
  - `8081 /health` = `200`
  - `8082 /health` = `200`
- Runtime pointers:
  - 8080 PID `3186035` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - 8081 PID `3186057` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - 8082 PID `3186156` -> `/root/ecommerce_ai/erp_bridge_sync`
- Deleted-binary state:
  - all active `/proc/<pid>/exe` pointers were verified as non-deleted
- SHA-256:
  - `ecommerce-api` -> `16dcbcc6bcc53c97d6a3abad138c44939171f06126c791e38087bbebd9f2a721`
  - `erp_bridge` -> `16dcbcc6bcc53c97d6a3abad138c44939171f06126c791e38087bbebd9f2a721`
  - unchanged `erp_bridge_sync` -> `2264a80cc8318d08828fcf29a6f7ddaa3ea69804ab13dc5b71e293d97afc82b8`
- 8082 truth:
  - verification triggered auto-recovery
  - sync was restored successfully
  - binary stayed unchanged

## 8. Live API acceptance after schema fix
- Real bearer session:
  - `POST /v1/auth/login` -> `200`
- Post-fix list sanity:
  - `GET /v1/tasks?page=1&page_size=5` -> `200`
- Live-safe create verification:
  - single `new_product_development` -> `201`, task `147`
  - batch `new_product_development` -> `201`, task `148`
  - batch `purchase_task` -> `201`, task `149`
  - batch `original_product_development` -> `400 INVALID_REQUEST`
- Detail/readback verification:
  - `GET /v1/tasks/147` -> `200`
    - `is_batch_task=false`
    - `batch_item_count=1`
    - `batch_mode=single`
    - `primary_sku_code=SKU-000029`
    - `sku_generation_status=completed`
    - `sku_items` present
  - `GET /v1/tasks/148` -> `200`
    - `is_batch_task=true`
    - `batch_item_count=2`
    - `batch_mode=multi_sku`
    - `primary_sku_code=CDXN31092831A`
    - `sku_generation_status=completed`
    - `sku_items` present with two rows
  - `GET /v1/tasks/149` -> `200`
    - `is_batch_task=true`
    - `batch_item_count=2`
    - `batch_mode=multi_sku`
    - `primary_sku_code=CDXP31092831A`
    - `sku_generation_status=completed`
    - `sku_items` present with two rows
- Rejection verification:
  - original batch create returned `400 INVALID_REQUEST`
  - machine-readable violations confirmed:
    - `batch_sku_mode`
    - `batch_items`
    - `batch_not_supported_for_task_type`

## 9. Scope summary
- Binaries replaced in this round:
  - `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `/root/ecommerce_ai/releases/v0.8/erp_bridge`
- Binary not replaced in this round:
  - `/root/ecommerce_ai/erp_bridge_sync`
- OpenAPI note:
  - `docs/api/openapi.yaml` was already aligned before this round
  - no OpenAPI content edit was required in this iteration

## 10. Remaining risks / honest follow-up
- The local `deploy/deploy.sh` remote wrapper is still unstable on this Windows control node and should be debugged separately if repeatable fully-managed local cutover is required.
- This round left explicit live verification tasks in DB:
  - `147`, `148`, `149`
- The most important truth from this iteration is:
  - overwrite publish alone did not finish the closure
  - live effectiveness required both the binary overwrite and live migration `046`
