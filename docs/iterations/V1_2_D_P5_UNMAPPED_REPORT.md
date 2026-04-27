# V1.2-D P5 · unmapped triage report

> Date: 2026-04-26 PT
> Scope: unmapped handler reduction and known_gap registration
> Baseline: `tmp/v1_2_d_p4_audit.json`
> Output: `tmp/v1_2_d_p5_audit.json`

## Summary

| metric | after P4 | after P5 | delta |
|---|---:|---:|---:|
| total_paths | 242 | 242 | 0 |
| clean | 117 | 127 | +10 |
| drift | 40 | 40 | 0 |
| unmapped | 65 | 5 | -60 |
| known_gap | 20 | 70 | +50 |

P5 did not edit business handlers. It improved `contract_audit` recognition for existing response forms and registered intentional non-field-audit surfaces as known gaps.

## Tool Enhancements

| enhancement | effect |
|---|---|
| `c.JSON(...)` payload extraction | recognizes explicit `gin.H` data envelopes and top-level side fields |
| `c.Status(204)` recognition | classifies no-content handlers as `clean_empty` when OpenAPI has no JSON fields |
| NewRouter local handler constructor inference | resolves locally constructed handlers such as `taskBatchExcelH` |
| stream response detection | registers file/data/stream responses as `stream_response` known gaps |
| delegated response detection | registers thin wrapper handlers that delegate response writing to helper methods |
| known_gap routing | moves reserved, inline/middleware, dynamic payload, stream, and delegated-response cases out of `unmapped` |

## Known Gap Classes

| class | count |
|---|---:|
| mounted_not_documented | 14 |
| documented_not_mounted | 6 |
| reserved_route | 10 |
| inline_or_middleware_route | 11 |
| dynamic_payload_documented | 18 |
| stream_response | 1 |
| delegated_handler_response | 10 |

## Remaining Unmapped

| method | path | handler | reason |
|---|---|---|---|
| GET | `/v1/assets/search` | `TaskAssetCenterHandler.SearchGlobalAssets` | response expression type not inferred |
| GET | `/v1/erp/products` | `ERPBridgeHandler.SearchProducts` | response expression type not inferred |
| GET | `/v1/erp/sync-logs` | `ERPBridgeHandler.ListSyncLogs` | response expression type not inferred |
| GET | `/v1/me/task-drafts` | `TaskDraftHandler.MyList` | response struct not found: TaskDraftListItem |
| POST | `/v1/agent/ack_job` | `AgentHandler.AckJob` | response expression type not inferred |

## Verification

```text
go test ./tools/contract_audit/... -count=1
PASS

go run ./tools/contract_audit \
  --transport transport/http.go \
  --handlers transport/handler \
  --domain domain \
  --openapi docs/api/openapi.yaml \
  --output tmp/v1_2_d_p5_audit.json \
  --markdown tmp/v1_2_d_p5_audit.md
summary: clean=127 drift=40 unmapped=5 known_gap=70
```

## P6 Risk

P5 satisfies the unmapped threshold (`5 <= 25`) and does not increase drift, but P6 final `--fail-on-drift true` is still blocked by 40 remaining drift paths. Those require explicit OpenAPI schema closure or a narrower V1.2-D continuation decision.
