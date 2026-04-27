# ITERATION_108

**Date:** 2026-04-02  
**Goal:** Close batch-task “no reference preview” when refs are sent per `batch_items[]`, and make `GET /v1/tasks/{id}` always expose `reference_file_refs` as a JSON array (including empty).

## Root cause

1. **Create path:** For `batch_sku_mode=multiple`, clients may send `reference_file_refs` only on each `batch_item`. The handler only forwarded the top-level array into `CreateTaskParams`, so `task_details.reference_file_refs_json` stayed `[]` even though uploads succeeded.
2. **Read path:** `TaskReadModel.reference_file_refs` used `json:",omitempty"`. An empty list was omitted from JSON, which is easy to misread as “no contract field” on the frontend.

Iteration 107 already addressed **design** preview gaps (empty `design_assets` roots + existing `task_assets`) and is orthogonal to this iteration.

## Formal contract (unchanged field names)

| Concern | Where to read |
|--------|----------------|
| Reference images (mother task) | `GET /v1/tasks/{id}` → **`reference_file_refs`** (always an array; union of top-level + all batch line refs after create) |
| Design roots + versions | Same response → **`design_assets`**, **`asset_versions`** (task-level; iteration 107 fallback still applies) |
| Per-SKU line metadata | `sku_items[]` — **no** `reference_file_refs` column today; line-level refs are merged into the mother task’s `reference_file_refs` |

## Code changes

- `service/task_batch_create.go` — `mergeBatchItemReferenceFileRefsIntoTask`
- `service/task_service.go` — call merge after normalize; `enrichTaskReadModelDetail` non-nil slice
- `service/task_batch_reference_file_refs_test.go` — new tests
- `domain/query_views.go` — drop `omitempty` on `TaskReadModel.reference_file_refs`
- `transport/handler/task.go` — `createTaskBatchItemReq.reference_file_refs` + mapping
- `docs/api/openapi.yaml` — batch item property + read-model description

## Local verification

- `go build ./...` OK.
- `go test` blocked on host Application Control for `*.test.exe`; run `go test ./service ./transport/handler ./repo/mysql` elsewhere.

## Deploy

- Not run in this iteration session. Use existing `./deploy/deploy.sh --version v0.8` when promoting.

## Risks / follow-ups

- If a product needs **distinct** reference sets per batch line in the API response (not merged), that would require a new persisted column on `task_sku_items` or a parallel JSON column; current scope intentionally merges to the mother task to match existing storage.
