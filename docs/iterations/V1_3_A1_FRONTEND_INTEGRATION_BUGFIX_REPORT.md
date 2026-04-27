# V1.3-A1 Frontend Integration Bugfix Diagnosis

Date: 2026-04-27

Status: diagnosis complete; waiting for architect verify and backend trace log for Issue 3.

## Scope

Readonly diagnosis for four frontend integration issues. No business Go files, OpenAPI, or frontend docs were modified.

Evidence files:

- `tmp/v1_3_a1_issue1_evidence.md`
- `tmp/v1_3_a1_issue2_evidence.md`
- `tmp/v1_3_a1_issue3_evidence.md`
- `tmp/v1_3_a1_issue4_evidence.md`

## Decision Table

| Issue | Root cause status | Root cause | Owner | Impact | Next execution |
| --- | --- | --- | --- | --- | --- |
| 1. Draft reference image 404 | Mostly determined | Draft payload is persisted raw and can carry expired signed `download_url`; download endpoint can refresh by asset id | Frontend primary; backend optional hardening | P1 | Frontend refresh via `GET /v1/assets/{asset_id}/download`; optional backend draft scrub in follow-up |
| 2. Reports overview 404 | Determined | `/v1/reports/l1/overview` is not mounted and not in OpenAPI | Frontend | P1 | Frontend use `/cards`, `/throughput`, `/module-dwell`; do not add backend overview in V1.x bugfix |
| 3. Task create 500 | Pending trace log | Transaction failure collapsed to `internal error during create task tx`; exact DB/repo error requires trace log | Backend diagnosis pending user log | P0 | User must provide trace-id log; then split V1.3-A1.1 fix |
| 4. Asset list URL consistency | Determined as contract decision | Lists expose metadata/current version shape; fresh access belongs to single download/preview endpoints | Frontend primary; backend no change recommended | P2 | Keep lists metadata-only; frontend calls single fresh access endpoint on demand |

## Decision Matrix

| Decision | Recommendation | Rationale |
| --- | --- | --- |
| Draft URL handling | Frontend refreshes by `asset_id`; backend may later scrub draft URLs | Signed URL expiry is expected; draft service intentionally stores WIP payload raw |
| Report overview | Do not add `/overview` in V1.3-A1 | New aggregate endpoint is a feature, not a bugfix; current three routes are canonical |
| Task create 500 | Do not code-fix before log | The code already logs `create_task_tx_failed err=...`; exact transaction error is required |
| Asset list URLs | Do not add root-level fresh URLs to lists | Avoid list-time presign fan-out and stale URL persistence; use single access endpoints |

## Issue 3 Log Request

Required:

```bash
grep '83ba7d26-385b-4bea-99b7-db0925be2975' /path/to/backend.log
```

The key evidence is the `create_task_tx_failed err=...` line and nearby `create_task_product_selection_*` lines.

## Verification

Performed:

- Static route check against `transport/http.go`.
- Static contract check against `docs/api/openapi.yaml`.
- Static service/repo read for task drafts, asset download, asset lists, and task creation transaction mapping.

Not performed:

- Live reproduction of `POST /v1/tasks`, because `cmd/server` has no sqlite/file dev mode and requires MySQL plus Redis with matching fixture data.

## Terminator

V1_3_A1_FRONTEND_BUGFIX_DIAGNOSED_PENDING_LOG_AND_VERIFY
