# ITERATION_112

**Date:** 2026-04-02  
**Goal:** Switch default task product-code middle segment from raw `category_code` to two-letter uppercase category short code, keep uniqueness under single/batch/concurrency, and overwrite deploy to existing `v0.8`.

## Why old format was wrong

Previous default generator directly formatted:

- `NS + category_code + 6-digit sequence`

So `KT_STANDARD` produced values like:

- `NSKT_STANDARD000060`

This violated the required compact format.

## New short-code rule (backend-owned)

Final format:

- `NS + category_short_code(2 uppercase letters) + 6-digit sequence`
- regex: `^NS[A-Z]{2}[0-9]{6}$`

Short-code generation priority:

1. Explicit map first:
   - `KT_STANDARD -> KT`
2. Else extract first two alphabet letters from `category_code`, uppercase:
   - `kt_standard -> KT`
   - `K-T-standard -> KT`
   - `A1B2 -> AB`
3. If fewer than two letters:
   - deterministic fallback letters (stable hash-based, non-random), same input always same output.

## Uniqueness strategy (critical)

### Core change

Sequence allocation dimension was changed logically to:

- `(prefix, category_short_code)`

Implementation detail:

- `product_code_sequences` table is reused.
- allocator now receives short code as allocation key.
- this guarantees different `category_code` values that collapse to the same short code share one sequence lane.

### Additional safety

When sequence row is first seen with `next_value=0`, allocator bootstraps from existing `task_sku_items`:

- scans max existing numeric suffix for `prefix+short_code`
- starts from `max+1`

This avoids collisions with historical already-issued `NS??xxxxxx` values.

## Code changes

- `service/task_product_code.go`
  - added short-code derivation + deterministic fallback
  - formatting switched to `NS + short_code + 6-digit`
  - allocation key switched from normalized `category_code` to `category_short_code`
- `repo/interfaces.go`
  - allocator interface comment/signature semantics updated to short-code scope
- `repo/mysql/product_code_sequence.go`
  - allocator now treats second dimension as short code
  - added bootstrap-from-existing-sku max suffix logic
- `service/task_product_code_test.go`
  - added mapping tests (`KT_STANDARD -> KT`, extract rules, fallback stability)
  - added regex test (`^NS[A-Z]{2}[0-9]{6}$`)
  - added prepare/create consistency test
  - existing single/batch/concurrency tests updated to `KT_STANDARD` lanes
- `repo/mysql/product_code_sequence_test.go` (new)
  - tests bootstrap behavior and non-bootstrap path with sqlmock
- `docs/api/openapi.yaml`
  - updated create/prepare documentation to short-code rule and short-code-scoped sequence wording.

## Local verification

Executed:

- `go test ./service ./transport/handler` -> pass
- `go build ./cmd/server` -> pass
- `go build ./repo/mysql ./service ./transport/handler` -> pass
- `go test ./repo/mysql` -> pass

## Deploy to existing `v0.8`

Used existing script chain only:

- `bash deploy/deploy.sh --version v0.8 --skip-tests --skip-runtime-verify --release-note "overwrite v0.8 category short code NS??xxxxxx rule"`

Release history evidence:

- deployed artifact sha256:
  - `9dd601696fcad8c719a437526d99c4d6b5cf9b2dc5ece94a5b14c41321935b60`

## Live acceptance

Environment: real bearer session on `http://223.4.249.11:8080`.

Deprecated template route:

- `GET /v1/rule-templates/product-code` -> `400 INVALID_REQUEST`
- message:
  - `rule_templates/product-code is deprecated; use default backend task product-code generation`
- `GET /v1/rule-templates` -> `200` and list currently has no `product-code`.

Prepare:

- `POST /v1/tasks/prepare-product-codes` with `KT_STANDARD` returned:
  - `NSKT000017`, `NSKT000018`, `NSKT000019`
- all matched regex and no in-batch duplicates.

New single:

- `task_id=361`
- `sku_code=NSKT000028`
- regex matched.

Purchase single:

- `task_id=362`
- `sku_code=NSKT000029`
- regex matched.

Batch:

- `task_id=365`
- `primary_sku_code=NSKT000032`
- task-level `sku_code=NSKT000032`
- item codes:
  - `NSKT000032`
  - `NSKT000033`
- all matched regex and unique.

Same-short-code collision lane:

- `KT_STANDARD` create -> `task_id=363`, `sku_code=NSKT000030`
- `K-T-standard` create -> `task_id=364`, `sku_code=NSKT000031`
- both success, no duplicate.

Lightweight concurrency:

- 8 parallel prepare requests returned:
  - `NSKT000020` ... `NSKT000027`
- no duplicates, no errors.

Other regression probes:

- `GET /v1/tasks?page=1&page_size=5` -> `200`
- `POST /v1/tasks/361/assign` -> `200` (`InProgress`)
- `GET /v1/tasks/365/detail` includes:
  - `design_assets`
  - `asset_versions`
  - `task_detail`

Runtime health + executable checks:

- `8080 /health = 200` (external probe)
- `8081 /health = 200` (remote localhost probe)
- `8082 /health = 200` (remote localhost probe)
- pids:
  - main `3838150`
  - bridge `3838173`
  - sync `3838987`
- `/proc/<pid>/exe`:
  - main -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - bridge -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - sync -> `/root/ecommerce_ai/erp_bridge_sync`
- active executable targets are not deleted.

Probe correction log (honest):

- First probe attempt failed due request payload issues (missing `owner_team`, invalid owner payload, missing required batch item fields).
- After correcting payload to current live create contract, all required acceptance lanes passed.

## Frontend final wording

- Frontend must not compute short code itself.
- Frontend must not configure `rule_templates/product-code`.
- Use:
  - `POST /v1/tasks` for creation (new/purchase auto-code)
  - optional `POST /v1/tasks/prepare-product-codes` for pre-display
- Read:
  - single/purchase: `data.sku_code`
  - batch: `data.sku_items[].sku_code`
  - compatibility: `data.primary_sku_code`, `data.sku_code`.
