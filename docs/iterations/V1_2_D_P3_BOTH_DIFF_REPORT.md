# V1.2-D P3 · both_diff triage report

> Date: 2026-04-26 PT
> Scope: cross-family `both_diff` closure
> Baseline: `tmp/v1_2_d_p2_audit.json`
> Output: `tmp/v1_2_d_p3_after_schema.json`

## Summary

| metric | after P2 | after P3 | delta |
|---|---:|---:|---:|
| total_paths | 242 | 242 | 0 |
| clean | 87 | 115 | +28 |
| drift | 68 | 42 | -26 |
| unmapped | 67 | 65 | -2 |
| both_diff | 29 | 0 | -29 |
| known_gap | 20 | 20 | 0 |

P3 split into two concrete fixes:

- Tool inference fix: method return lookup is now receiver/interface-aware, and multi-return assignments map by result position. This removed the false `WarehouseReceipt` / `TaskReadModel` classifications on category, cost, task list, product, upload-session, and related handlers.
- Contract fix: `WorkflowUser` response schema now follows `domain.User`, adding `jst_u_id`, `managed_departments`, and `managed_teams`, and removing response `avatar`.

## Contract Edits

| component | action |
|---|---|
| `WorkflowUser` | remove response `avatar` placeholder |
| `WorkflowUser` | add `managed_departments[]` |
| `WorkflowUser` | add `managed_teams[]` |
| `WorkflowUser` | add nullable `jst_u_id` |

Affected paths:

| method | path | verdict |
|---|---|---|
| GET | `/v1/auth/me` | clean |
| GET | `/v1/users` | clean |
| GET | `/v1/users/:id` | clean |

## Frontend Doc Sync

| file | action |
|---|---|
| `docs/frontend/V1_API_AUTH.md` | add V1.2-D P3 revision note and `WorkflowUser` response-field note |
| `docs/frontend/V1_API_USERS.md` | add V1.2-D P3 revision note and list/detail `WorkflowUser` response-field notes |

## Verification

```text
go test ./tools/contract_audit/... -count=1
PASS

go run ./tools/contract_audit \
  --transport transport/http.go \
  --handlers transport/handler \
  --domain domain \
  --openapi docs/api/openapi.yaml \
  --output tmp/v1_2_d_p3_after_schema.json \
  --markdown tmp/v1_2_d_p3_after_schema.md
summary: clean=115 drift=42 unmapped=65 known_gap=20

python yaml.safe_load docs/api/openapi.yaml
PASS

go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
openapi validate: 0 error 0 warning
```
