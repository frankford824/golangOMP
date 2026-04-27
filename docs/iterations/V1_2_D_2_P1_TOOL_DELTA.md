# V1.2-D-2 P1 · contract_audit tool delta

> Date: 2026-04-26 PT
> Baseline: `tmp/v1_2_d_2_baseline.json`
> Output: `tmp/v1_2_d_2_p1_audit.json`

## Summary

| metric | baseline | after P1 | delta |
|---|---:|---:|---:|
| total_paths | 242 | 242 | 0 |
| clean | 127 | 129 | +2 |
| drift | 40 | 39 | -1 |
| unmapped | 5 | 2 | -3 |
| known_gap | 70 | 72 | +2 |
| missing_in_openapi | 14 | 14 | 0 |
| missing_in_code | 6 | 6 | 0 |

## Implemented

| item | result |
|---|---|
| clean-empty verdict | `code_fields=[]` and `openapi_fields=[]` now returns `clean` |
| type alias support | aliases such as `TaskDraftListItem = TaskDraft` resolve to target fields |
| receiver/interface return lookup | qualified service aliases such as `orgmovesvc.Service.List` are indexed |
| function-local struct support | local response structs such as `designerItem` expose JSON tags |
| nil response support | `respondOK(c, nil)` maps to clean empty payload |
| explicit envelope classification | `gin.H` envelopes with non-inferable data are classified as `dynamic_payload_documented` |

## Remaining Unmapped

| method | path | handler | reason |
|---|---|---|---|
| GET | `/v1/erp/products` | `ERPBridgeHandler.SearchProducts` | response expression type not inferred |
| GET | `/v1/erp/sync-logs` | `ERPBridgeHandler.ListSyncLogs` | response expression type not inferred |

## Verification

```text
go test ./tools/contract_audit/... -count=1
PASS

go run ./tools/contract_audit \
  --transport transport/http.go \
  --handlers transport/handler \
  --domain domain \
  --openapi docs/api/openapi.yaml \
  --output tmp/v1_2_d_2_p1_audit.json \
  --markdown tmp/v1_2_d_2_p1_audit.md
summary: clean=129 drift=39 unmapped=2 known_gap=72
```
