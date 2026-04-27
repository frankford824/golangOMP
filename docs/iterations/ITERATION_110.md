# ITERATION_110

**Date:** 2026-04-02  
**Goal:** Complete a combined closure for (1) `rule_templates/product-code` runtime audit, (2) backend-only default task product-code generation, and (3) prepare-product-codes capability with strict uniqueness for new/purchase task create flows.

## Audit conclusion (`rule_templates` / `product-code`)

Runtime code search was executed over `service`, `repo`, `transport`, `cmd`, `domain`, `db/migrations` with keywords:

- `rule_templates`
- `rule-templates`
- `product-code`
- `cost-pricing`
- `short-name`
- `code-rules`
- `generate-sku`
- `template_key`
- `rule_template`
- `code_rule`

Findings:

1. `rule_templates` table/repo/service/handler still exist in runtime and `/v1/rule-templates` routes are still exposed.
2. `product-code` under `rule_templates` is now deprecated in runtime:
   - list excludes `product-code`
   - `GET/PUT /v1/rule-templates/product-code` now return `INVALID_REQUEST` deprecation errors
3. `code-rules` module is still active and exposes:
   - `GET /v1/code-rules`
   - `GET /v1/code-rules/{id}/preview`
   - `POST /v1/code-rules/generate-sku`
4. Task create mainline for `new_product_development` + `purchase_task` no longer depends on frontend-selected `rule_templates/product-code` or `code-rules` selection for `sku_code`.

Final classification:

- Still running: `rule_templates` (`cost-pricing`, `short-name`), `code-rules`
- Legacy/deprecated: `rule_templates/product-code`
- Safe to deprecate now: `GET/PUT /v1/rule-templates/product-code` behavior (kept route, disabled key)
- Requires migration before full route removal: entire `rule-templates` route group (still needed by non-product-code templates)

## Backend default product-code rule (authoritative source)

For task create/product-code generation:

- prefix: `NS`
- middle: `category_code` (from create payload / batch item)
- suffix: 6-digit zero-padded sequence

Example: `NSKT000000`

Scope:

- enabled: `new_product_development`, `purchase_task`
- not enabled: `original_product_development` existing-product binding flow

Important business wording:

- In current task domain, this generated value is written to `sku_code` and treated as task product code / SKU code in the same field.

## Uniqueness mechanism

New migration and allocator were introduced:

- `db/migrations/048_v7_product_code_sequences.sql`
- new table: `product_code_sequences`
- unique key: `(prefix, category_code)`

Allocation strategy:

1. transaction
2. upsert `(prefix, category_code)` row
3. `SELECT ... FOR UPDATE` on sequence row
4. allocate contiguous range `[start, start+count-1]`
5. `UPDATE next_value = next_value + count`

Additional uniqueness guardrails:

- in-request duplicate prevention for batch generation
- existing SKU pre-check for manual inputs
- final DB unique protection via `task_sku_items.uq_task_sku_items_sku_code`
- create-tx duplicate mapping to `INVALID_REQUEST` on MySQL 1062

## API and behavior changes

### Existing create mainline (`POST /v1/tasks`)

- frontend no longer needs to select/pass code-rule or rule-template for new/purchase task SKU generation.
- when `sku_code` is omitted:
  - single new/purchase create auto-generates on backend
  - batch create auto-generates for each batch item

### New prepare endpoint

- `POST /v1/tasks/prepare-product-codes`
- input: `task_type`, `category_code` + optional `count` OR `batch_items[].category_code`
- output: `codes[].{index, category_code, sku_code}`

Note: create remains final source of truth; prepare endpoint is an assist/pre-generation API for frontend UX.

## Frontend contract after this iteration

Frontend should stop doing:

- configuring/selecting `rule_templates/product-code`
- configuring/selecting code-rules for task `sku_code` generation
- local rule assembly for product code format

Frontend should do:

1. call `POST /v1/tasks` directly and rely on backend auto-generation (recommended default).
2. optionally call `POST /v1/tasks/prepare-product-codes` for pre-display before submit.

Read paths for generated code:

- single task: top-level `data.sku_code`
- batch task:
  - compatibility primary field: `data.sku_code` and `data.primary_sku_code` (first line)
  - each child line: `data.sku_items[].sku_code`

## Code changes

Added:

- `db/migrations/048_v7_product_code_sequences.sql`
- `repo/mysql/product_code_sequence.go`
- `service/task_product_code.go`
- `service/task_product_code_test.go`
- `service/rule_template_service_test.go`
- `transport/handler/task_prepare_product_codes_test.go`

Updated:

- `repo/interfaces.go`
- `service/task_service.go`
- `service/task_batch_create.go`
- `service/rule_template_service.go`
- `transport/handler/task.go`
- `transport/http.go`
- `cmd/server/main.go`
- `cmd/api/main.go`
- `docs/api/openapi.yaml`
- `service/task_prd_service_test.go`
- `service/task_filing_flow_test.go`
- `service/task_owner_team_test.go`

## Local verification

Required commands:

- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed
- `go test ./repo/mysql` -> passed

Concurrency-focused test:

- `go test ./service -run TestTaskServicePrepareProductCodesBatchAndConcurrentUnique -count=1` -> passed

Race note:

- `go test -race ...` could not run on this host: `-race requires cgo; enable cgo by setting CGO_ENABLED=1`

## Deploy / online acceptance

- Not executed in this iteration session.
- Existing release line requirement remains: overwrite deploy to `v0.8` only when runtime promotion is requested.

## Risks / known gaps

1. `rule-templates` routes are still exposed for `cost-pricing` and `short-name`; full route removal was intentionally not done in this iteration.
2. `code-rules` module still exists for other numbering domains and backward compatibility; task new/purchase SKU generation has been converged to backend default allocator.
3. Prepare API allocates real sequence numbers by design; if clients prefetch and do not submit create, sequence gaps are expected and acceptable under uniqueness-first policy.
