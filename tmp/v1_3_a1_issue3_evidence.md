# V1.3-A1 Issue 3 Evidence - POST /v1/tasks 500

## Finding

Root cause is not determinable without the backend log for trace id `83ba7d26-385b-4bea-99b7-db0925be2975`.

The visible error message maps to `service/task_service.go` transaction failure handling: `mapTaskCreateTxError` only converts MySQL duplicate-key 1062 to a 400; other transaction errors become `INTERNAL_ERROR` with message `internal error during create task tx`.

## Static path

Relevant create path:

- `transport/http.go` mounts `POST /v1/tasks` to `taskH.Create`.
- `transport/handler/task.go` binds `createTaskReq`, logs `create_task_entry`, validates `product_selection`, normalizes existing-product binding, then calls `h.svc.Create`.
- `service/task_service.go` normalizes request, validates `reference_file_refs`, handles `defer_local_product_binding`, validates task type fields, resolves SKU, creates task no, and enters `createTaskWithBatchSkuItemsTx`.
- Any non-1062 transaction error is logged as `create_task_tx_failed err=...` and returned as `internal error during create task tx`.

## Local reproduction status

No local runtime reproduction was completed in this diagnostic round:

- `cmd/server/main.go` has no sqlite/file dev mode.
- Server startup requires MySQL (`MYSQL_DSN`) and Redis; default config points to `127.0.0.1` services.
- A faithful repro requires the same DB rows for `asset_storage_refs`, `upload_requests`, user/session/role, product/ERP bridge state, and code-rule/task sequence tables.

## Most likely hypotheses from static analysis

1. Reference ref validation passes, then flat reference insert or task/detail/SKU insert fails due missing or inconsistent asset mapping. The payload uses `reference_file_refs[0]` with both `asset_id` and `ref_id`; `domain.ReferenceFileRef.CanonicalID()` accepts aliases, but DB-side FK or unique constraints can still fail in transaction.
2. Deferred ERP product binding produces `product_id = NULL` for `original_product_development`, then a downstream repo insert or task SKU item constraint rejects a nullable field that validation allows. The service explicitly supports `defer_local_product_binding=true` with sufficient `erp_product`, so a DB/schema mismatch would surface as tx 500.
3. Code-rule/task-number/SKU sequence or task SKU item insertion failed under this payload. The user report says all task types 500, which points to a shared create transaction dependency rather than one task-type whitelist branch.

## Required log

Please provide the full backend log lines for:

```bash
grep '83ba7d26-385b-4bea-99b7-db0925be2975' /path/to/backend.log
```

The useful line is expected to include `create_task_tx_failed err=...` and may include the preceding `create_task_product_selection_*` lines. Without that error string or stack, code changes would be guesswork.
