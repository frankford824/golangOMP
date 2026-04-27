# V1.2-D P4 · ERP pagination wrapper report

> Date: 2026-04-26 PT
> Scope: ERP-style JST user list wrapper
> Baseline: `tmp/v1_2_d_p3_after_schema.json`
> Output: `tmp/v1_2_d_p4_audit.json`

## Summary

| metric | after P3 | after P4 | delta |
|---|---:|---:|---:|
| total_paths | 242 | 242 | 0 |
| clean | 115 | 117 | +2 |
| drift | 42 | 40 | -2 |
| unmapped | 65 | 65 | 0 |
| known_gap | 20 | 20 | 0 |

The original P4 estimate listed 7 ERP-style pagination paths. After P3's receiver-aware return inference, only 2 remained real `JSTUserListResponse` wrapper paths:

| method | path | action | verdict |
|---|---|---|---|
| GET | `/v1/erp/users` | document `data.JSTUserListResponse` | clean |
| GET | `/v1/admin/jst-users` | document `data.JSTUserListResponse` | clean |

## Contract Edits

Added OpenAPI components:

- `JSTUser`
- `JSTUserListResponse`

Both paths now document the real handler payload:

```json
{
  "data": {
    "current_page": "1",
    "page_size": "50",
    "count": "123",
    "pages": "3",
    "datas": []
  }
}
```

## Frontend Doc Sync

| file | action |
|---|---|
| `docs/frontend/V1_API_ERP.md` | add V1.2-D P4 revision note and `/v1/erp/users` response schema |
| `docs/frontend/V1_API_USERS.md` | add V1.2-D P4 revision note and `/v1/admin/jst-users` response schema |

## Verification

```text
go run ./tools/contract_audit \
  --transport transport/http.go \
  --handlers transport/handler \
  --domain domain \
  --openapi docs/api/openapi.yaml \
  --output tmp/v1_2_d_p4_audit.json \
  --markdown tmp/v1_2_d_p4_audit.md
summary: clean=117 drift=40 unmapped=65 known_gap=20

python yaml.safe_load docs/api/openapi.yaml
PASS

go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
openapi validate: 0 error 0 warning
```
