# V1.2-D P1 · contract_audit tool delta

> Date: 2026-04-26 PT
> Scope: P1 tool enhancement only
> Baseline: `tmp/v1_2_d_baseline.json`
> Output: `tmp/v1_2_d_p1_audit.json`

## Summary

| metric | baseline | after P1 | delta |
|---|---:|---:|---:|
| total_paths | 242 | 242 | 0 |
| clean | 85 | 87 | +2 |
| drift | 71 | 68 | -3 |
| unmapped | 66 | 67 | +1 |
| known_gap | 20 | 20 | 0 |
| missing_in_openapi | 14 | 14 | 0 |
| missing_in_code | 6 | 6 | 0 |

`unmapped` increased by one because P1.3 now detects one real multi-exit inconsistency instead of silently using the last response exit:

- `OperationLogEntry+pagination | WarehouseReceipt+pagination`

This is intentional tool precision, not a contract regression.

## Implemented

| item | status | evidence |
|---|---|---|
| P1.1 anonymous embedded struct expansion | done | `TestMainFlow_AnonymousEmbedClean` |
| P1.2 pagination wrapper alignment | done | `TestMainFlow_PaginationWrapClean` |
| P1.3 multi respond exit collection | done | `TestMainFlow_MultiExitInconsistent` |

## Verification

```text
go test ./tools/contract_audit/... -count=1
ok workflow/tools/contract_audit

go run ./tools/contract_audit \
  --transport transport/http.go \
  --handlers transport/handler \
  --domain domain \
  --openapi docs/api/openapi.yaml \
  --output tmp/v1_2_d_p1_audit.json \
  --markdown tmp/v1_2_d_p1_audit.md
summary: clean=87 drift=68 unmapped=67 known_gap=20
```

## P2 Readiness

P1.1 made both TaskReadModel paths clean without OpenAPI edits:

| method | path | verdict |
|---|---|---|
| GET | `/v1/tasks/:id` | clean |
| POST | `/v1/tasks/:id/close` | clean |
