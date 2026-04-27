# V1.2-C · contract_audit Tool Rework Retro

- date: 2026-04-27
- terminator: `V1_2_C_AUDIT_TOOL_REWORKED`
- verdict: PENDING_ARCHITECT_VERIFY (codex 不自签 PASS)
- scope: only `tools/contract_audit/` + V1.2-C audit v2 outputs

## §1 Baseline

P1 baseline sha gate passed for the 12 frozen files. Key anchors:

- `transport/http.go`: `9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396`
- `docs/api/openapi.yaml`: `80730ec3d272e4124ab95244feb0c1daf499d4c0a032f47b70179cdd4189488f`
- `domain/task.go`: `658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b`
- `service/identity_service.go`: `00ec340a81738a75a88d3b0d32d834b49879bea7df6ac1baa0eb1932d1d47644`

## §2 Implementation Summary

- Replaced the v1 shell flow where `CodeFields` and `OpenAPIFields` shared the same OpenAPI source.
- Added `ParseTransportRoutes` using Go AST to extract mounted method/path/handler mappings.
- Added handler AST indexing and `ResolveHandlerResponseType` for `respondOK`, `respondCreated`, and `respondOKWithPagination` response expressions.
- Added struct-field extraction across `domain/`, `service/`, and `transport/handler/`.
- Extended OpenAPI extraction with `$ref`, `allOf`, and array `items` expansion.
- Added real verdict assignment and real `Summary.Drift`, `Summary.Unmapped`, and `Summary.KnownGap` counters.

## §3 Test Coverage

Required tests are present in `tools/contract_audit/main_test.go`:

- `TestMainFlow_TopLevelClean`
- `TestMainFlow_FieldDriftSeed`
- `TestMainFlow_OnlyInCode`
- `TestMainFlow_BothDiff`
- `TestParseTransportRoutes_RealRepo`
- `TestFailOnDriftExitCode`

Testdata lives under `tools/contract_audit/testdata/` with clean, only-openapi, only-code, and both-diff fixtures.

## §4 Real Repo Audit v2 Result

- total_paths: 242
- clean: 84
- drift: 72
- unmapped: 66
- known_gap: 20
- missing_in_openapi: 14
- missing_in_code: 6

Verdict distribution:

| verdict | count |
|---|---:|
| both_diff | 31 |
| clean | 84 |
| documented_not_found | 14 |
| mounted_not_found | 6 |
| only_in_code | 39 |
| only_in_openapi | 2 |
| unmapped_handler | 35 |
| unmapped_handler_dynamic_payload | 31 |

## §5 Known Drift / Unmapped

This round intentionally does not fix OpenAPI or business code. The tool now exposes real drift and unmapped cases for V1.2-D triage. Full row-level detail is in:

- `docs/iterations/V1_2_CONTRACT_AUDIT_v2.json`
- `docs/iterations/V1_2_CONTRACT_AUDIT_v2.md`

## §6 Verify Matrix

| # | check | result |
|---|---|---|
| 1 | business SHA anchors | PASS |
| 2 | no `_ = handlers` / `_ = domain` | PASS |
| 3 | no OpenAPI self-copy into code fields | PASS |
| 4 | `go vet ./...` | PASS |
| 5 | `go build ./...` | PASS |
| 6 | `go test ./tools/contract_audit/... -count=1 -race` | PASS |
| 7 | `TestMainFlow_TopLevelClean` | PASS |
| 8 | `TestMainFlow_FieldDriftSeed` | PASS |
| 9 | `TestMainFlow_OnlyInCode` | PASS |
| 10 | `TestMainFlow_BothDiff` | PASS |
| 11 | `TestParseTransportRoutes_RealRepo` | PASS |
| 12 | real repo unmapped < 0.5 * total | PASS |
| 13 | real repo total_paths >= 200 | PASS |
| 14 | runtime < 60s | PASS (~1s) |
| 15 | `--fail-on-drift true` exits 1 on drift fixture | PASS |
| 16 | real repo drift/unmapped explicit | PASS |
| 17 | V1.2-C retro report | PASS |
| 18 | v2 JSON/Markdown outputs | PASS |

Codex does not self-sign PASS. Architect verify remains required.
