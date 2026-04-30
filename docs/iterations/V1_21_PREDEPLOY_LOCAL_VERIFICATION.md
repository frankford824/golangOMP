# v1.21-prod -> HEAD Predeploy Local Verification

Date: 2026-04-30
Base: `v1.21-prod`
Local target before this report: `49bae0d`

## Verdict

Local executable verification is green, but production release is not yet
externally cleared.

This branch can be treated as a locally verified functional release candidate.
It must not be deployed over online v1.21 until migration dry-run, frontend
contract acknowledgement, and staging smoke verification are completed.

## Local Gate

Final full gate command:

```bash
./scripts/agent-check.sh
```

Result: PASS.

The script completed all five steps:

- `go vet ./...`
- `go build ./...`
- `go test ./... -count=1`
- `go run ./cmd/tools/openapi-validate docs/api/openapi.yaml`
- `go run ./tools/contract_audit ... --fail-on-drift true`

Final audit summary from `tmp/agent_check_audit.json`:

```text
total_paths=234 clean=169 drift=0 unmapped=0 known_gap=65 missing_in_openapi=0 missing_in_code=0
```

The command emitted only the existing local Go environment warning that GOPATH
and GOROOT are both `/home/wsfwk/go`; this did not fail the gate.

## Static Findings

### Migration 070 Is A Deploy-Order Blocker

`db/migrations/070_v1_1_task_sku_item_filing_projection.sql` adds SKU filing
projection columns:

- `filing_status`
- `erp_sync_status`
- `erp_sync_required`
- `erp_sync_version`
- `last_filed_at`
- `filing_error_message`

Current code references these columns in `domain/task_sku_item.go` and
`repo/mysql/task.go`, including SKU item SELECT/UPDATE paths. Therefore
migration 070 must run before the new binary serves traffic. Deploying the code
first can produce unknown-column SQL failures.

External status: not verified locally against a production-size table. A staging
clone dry-run is required before release.

### Migration 069 Has Runtime Fallback But Needs MySQL Compatibility Check

`db/migrations/069_v1_1_task_search_documents.sql` creates
`task_search_documents` with a FULLTEXT index and `utf8mb4_0900_ai_ci`.
Current search repository code checks table existence and can fall back when the
table is missing.

External status: production MySQL version, collation support, and DDL runtime
cost are Unknown from this repository. Confirm on staging before release.

### OpenAPI / Frontend Docs Alignment

Static contract checks found:

- `/v1/erp/iids` exists in OpenAPI and generated frontend docs.
- `/v1/assets/{asset_id}` exists in OpenAPI and generated frontend docs.
- `/v1/assets/{id}` is not an OpenAPI path key.
- Priority enums in OpenAPI are `low`, `normal`, `high`, `critical`; no
  `urgent` reference remains in OpenAPI or generated frontend docs.
- `task_pending_audit` is present in OpenAPI and generated frontend docs.
- Pool and other list endpoints expose `page_size` / `pagination` in the
  generated docs.

One docs-only residual was fixed during this verification: the proxy file route
description still mentioned `/v1/assets/{id}/download` and
`/v1/assets/{id}/preview`. It now uses the canonical
`/v1/assets/{asset_id}/...` wording, and frontend docs were regenerated.

## Required External Verification

Before production deploy, run these checks outside the local repository:

1. Migration dry-run on a staging clone with production-like row counts.
2. Confirm MySQL supports `utf8mb4_0900_ai_ci`, FULLTEXT DDL, and the required
   online/offline ALTER behavior for `task_sku_items`.
3. Confirm no online clients still submit task priority `urgent`.
4. Confirm frontend accepts the changed contracts:
   - task create/product info `i_id` / `product_i_id`
   - `/v1/erp/iids`
   - `/v1/tasks/pool` paginated response
   - `task_pending_audit`
   - global asset path parameter name `asset_id`
   - batch Excel parse/template semantics
5. Run staging smoke for the changed user-visible flows:
   - create new-product task with i_id
   - parse and submit batch Excel with row i_id and embedded image
   - ERP filing success and retry/failure visibility
   - retouch design submit auto-completes the retouch task
   - pool listing, concurrent claim conflict, and assignment notification
   - asset read/download/preview with `{asset_id}`
   - global search with and without `task_search_documents`

## Release Gate Status

- Local code/build/test/contract gate: PASS.
- Local OpenAPI/frontend-doc drift check: PASS after docs-only wording fix.
- Migration dry-run: NOT RUN.
- Staging smoke: NOT RUN.
- Frontend runtime compatibility: NOT VERIFIED.

Release decision: hold production deploy until the three external items above
are completed.
