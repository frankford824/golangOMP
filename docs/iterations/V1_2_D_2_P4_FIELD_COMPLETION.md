# V1.2-D-2 P4 · OpenAPI field completion

- date: 2026-04-26 PT
- scope: close residual `only_in_code` / `only_in_openapi` response field drift
- authority: code response structs and json tags; no business Go implementation changes.

## Actions

| family | representative paths | action |
|---|---|---|
| Assets upload requests | `/v1/assets/upload-requests*` | added `target_sku_code` to `UploadRequest` |
| Tasks | `/v1/tasks`, `/v1/tasks/pool`, batch actions, warehouse actions, audit handover, task events | completed response schemas and repaired malformed method indentation on affected legacy blocks |
| V6 legacy SKU/agent/audit | `/v1/sku*`, `/v1/agent/*`, `/v1/audit` | added concrete response schemas for legacy-but-mounted routes |
| Admin/JST | `/v1/admin/jst-users/import*` | added import result response schemas |
| Org move | `/v1/org-move-requests` | removed phantom actor objects and added `reject_reason` |
| Reference upload | `/v1/tasks/reference-upload` | removed phantom `owner_module_key` |

## Result

- output: `tmp/v1_2_d_2_p4_audit.json`
- final carried output: `docs/iterations/V1_2_D_2_FINAL_AUDIT.json`
- `summary.drift = 0`
- `summary.unmapped = 0`
- `summary.missing_in_openapi = 0`
- `summary.missing_in_code = 0`

## Notes

The only remaining unmapped route is `GET /v1/erp/sync-logs`, a dynamic ERP Bridge response inference boundary. It is below the prompt threshold `unmapped <= 2` and is not response-field drift.
