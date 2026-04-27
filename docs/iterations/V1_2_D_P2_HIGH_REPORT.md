# V1.2-D P2 · HIGH drift triage report

> Date: 2026-04-26 PT
> Scope: TaskReadModel embedded-field drift
> Baseline: `tmp/v1_2_d_p1_audit.json`
> Output: `tmp/v1_2_d_p2_audit.json`

## Summary

No OpenAPI or handler edits were required in P2. P1.1 made anonymous embedded struct expansion explicit in `contract_audit`, so the two TaskReadModel paths now compare the flattened `domain.Task` fields plus `reference_file_refs`.

| metric | after P1 | after P2 | delta |
|---|---:|---:|---:|
| total_paths | 242 | 242 | 0 |
| clean | 87 | 87 | 0 |
| drift | 68 | 68 | 0 |
| unmapped | 67 | 67 | 0 |
| known_gap | 20 | 20 | 0 |

## Path Verdicts

| method | path | verdict | action |
|---|---|---|---|
| GET | `/v1/tasks/:id` | clean | no schema change |
| POST | `/v1/tasks/:id/close` | clean | no schema change |

## Verification

```text
go run ./tools/contract_audit \
  --transport transport/http.go \
  --handlers transport/handler \
  --domain domain \
  --openapi docs/api/openapi.yaml \
  --output tmp/v1_2_d_p2_audit.json \
  --markdown tmp/v1_2_d_p2_audit.md
summary: clean=87 drift=68 unmapped=67 known_gap=20
```
