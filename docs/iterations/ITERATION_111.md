# ITERATION_111

**Date:** 2026-04-02  
**Goal:** Based on ITERATION_110 local completion, overwrite deploy to existing `v0.8`, run live coding-rule acceptance, and publish truth-source closure.

## 1) Pre-deploy local checks

Executed exactly-required local checks:

- `go test ./service ./transport/handler` -> `ok`
- `go build ./cmd/server` -> pass
- `go build ./repo/mysql ./service ./transport/handler` -> pass
- `go test ./repo/mysql` -> `ok`

Targeted code-path checks confirmed:

- `db/migrations/048_v7_product_code_sequences.sql` exists and defines `product_code_sequences` with unique `(prefix, category_code)`.
- `repo/mysql/product_code_sequence.go` uses TX allocator with `SELECT ... FOR UPDATE` and range allocation.
- `service/task_product_code.go` fixed backend default: `NS + category_code + 6-digit sequence`.
- `service/task_service.go` and `service/task_batch_create.go` wire auto-generation for `new_product_development` and `purchase_task` (single + batch items).
- `transport/http.go` exposes `POST /v1/tasks/prepare-product-codes`.
- `service/rule_template_service.go` keeps `/v1/rule-templates/product-code` route but returns explicit deprecation errors.

## 2) Deploy to existing `v0.8`

Used existing script chain only (`deploy/deploy.sh`), version pinned to existing line:

- `bash deploy/deploy.sh --version v0.8 --release-note "overwrite v0.8 ITERATION_110 task default product-code live acceptance"`

### First run (real result)

- Packaging/upload/deploy completed.
- Runtime verify step failed because remote script still had CRLF:
  - `check-three-services.sh: set: pipefail^M: invalid option name`
- `release-history.log` recorded both `deployed` and final `failed` for this attempt.

### Second run (re-run after failure)

- Re-ran existing script:
  - `bash deploy/deploy.sh --version v0.8 --skip-tests --skip-runtime-verify --release-note "overwrite v0.8 ITERATION_110 rerun after runtime-verify CRLF failure"`
- Result: `deployed` on existing `v0.8`.

### Third run (restore after failed remote CRLF edit attempt)

- One remote CRLF conversion attempt was executed incorrectly and touched deploy scripts; immediately recovered by one more overwrite deploy:
  - `bash deploy/deploy.sh --version v0.8 --skip-tests --skip-runtime-verify --release-note "overwrite v0.8 ITERATION_110 rerun to restore deploy scripts after CRLF fix attempt"`
- Final deployed artifact sha256 in `deploy/release-history.log`:
  - `8fea0be9a4fcfa5a3324c47ca885146033675c58ed0348f09650d605c5a02bd8`

## 3) Migration 048 live status (backup -> apply -> verify)

Pre-check on live DB showed:

- `task_sku_items` + `uq_task_sku_items_sku_code` already existed.
- `product_code_sequences` **did not exist** yet.

So migration was applied with backup-first:

1. Backup created:
   - `/root/ecommerce_ai/backups/iter110_048_20260402T082531Z/full_db_before_048.sql`
2. Applied:
   - `/root/ecommerce_ai/releases/v0.8/db/migrations/048_v7_product_code_sequences.sql`
3. Verified:
   - table `product_code_sequences` exists
   - unique key `uq_product_code_sequences_prefix_category (prefix, category_code)` exists

## 4) Runtime verification after deploy

Live checks on target host:

- `8080 /health = 200` (`{"status":"ok"}`)
- `8081 /health = 200` (`{"status":"ok"}`)
- `8082 /health = 200` (`{"status":"ok"}`)

PID/exe:

- `ecommerce-api` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
- `erp_bridge` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
- `erp_sync` -> `/root/ecommerce_ai/erp_bridge_sync`
- all active executables: `DELETED=no`

## 5) Live coding-rule acceptance (A~G)

Authenticated with real bearer session from `POST /v1/auth/login` (`admin`).

### A. `/v1/rule-templates/product-code`

- `GET /v1/rule-templates/product-code` -> `400`
- message:
  - `rule_templates/product-code is deprecated; use default backend task product-code generation`
- `GET /v1/rule-templates` -> `200`, list no longer contains `product-code`.
- Live create flows below still succeeded, proving this deprecated route no longer gates create mainline.

### B. new product single auto-code

- Create -> `201`
- `task_id=352`
- `sku_code=NSKT_STANDARD000022`
- format matched: `NS + KT_STANDARD + 6 digits`

### C. purchase single auto-code

- Create -> `201`
- `task_id=353`
- returned code field: `sku_code`
- value: `NSKT_STANDARD000023`
- format matched.

### D. batch auto-code

- Create -> `201`
- `task_id=354`
- `primary_sku_code=NSKT_STANDARD000024`
- `sku_items[].sku_code`:
  - `NSKT_STANDARD000024`
  - `NSKT_STANDARD000025`
- duplicate check: `no`
- task-level `sku_code` and `primary_sku_code` both `NSKT_STANDARD000024`.

### E. prepare endpoint acceptance

- Path: `POST /v1/tasks/prepare-product-codes`
- Request sample:
  ```json
  {
    "task_type": "new_product_development",
    "category_code": "KT_STANDARD",
    "count": 3
  }
  ```
- Response sample codes:
  - `NSKT_STANDARD000026`
  - `NSKT_STANDARD000027`
  - `NSKT_STANDARD000028`
- In-batch duplicate: `no`

### F. lightweight concurrency uniqueness

- Method: parallel `POST /v1/tasks/prepare-product-codes` (`count=1`)
- Concurrency: `8`
- Returned:
  - `NSKT_STANDARD000029` ... `NSKT_STANDARD000036`
- duplicate: `no`
- conflict/error count: `0`

### G. Other mainline regression spot checks

- `GET /v1/tasks?page=1&page_size=5` -> `200`
- assign action:
  - `POST /v1/tasks/352/assign` -> `200`
  - task status became `InProgress`
- canonical ownership still present on read:
  - `owner_team=设计组`
  - `owner_department=设计部`
  - `owner_org_team=""` (empty string lane)
- detail design fields still present:
  - `design_assets` exists
  - `asset_versions` exists
- reference detail lane in current live payload:
  - `task_detail.reference_file_refs_json` exists (returned `[]` in checked tasks)
  - top-level `reference_file_refs` was not present in current `/detail` payload.

## 6) Frontend collaboration final wording

Keep:

- Do **not** configure `rule_templates/product-code`.
- Do **not** assemble task SKU rule client-side.
- Create path stays `POST /v1/tasks` (backend auto-generate for `new_product_development` and `purchase_task`).
- Optional preview/pre-generation path:
  - `POST /v1/tasks/prepare-product-codes`

Read fields:

- single/purchase: `data.sku_code`
- batch: `data.sku_items[].sku_code`
- primary fallback: `data.primary_sku_code` and `data.sku_code`

## 7) Remaining boundaries / risks

- This iteration formalized default task product-code on live and completed live acceptance; it does **not** claim global numbering-platform finalization.
- Deploy runtime-verify scripts on this control node still have CRLF risk; deploy itself is successful, but script line-ending hygiene remains an ops checklist item.
- `erp_sync` (`8082`) is still operated via `/root/ecommerce_ai/erp_bridge_sync` compatibility binary path.
- `/v1/tasks/{id}/detail` reference payload currently observed as `task_detail.reference_file_refs_json`; no top-level `reference_file_refs` observed in this live verification lane.
