# V1.2 Contract Audit

`contract_audit` is the first V1 field-contract guard. It loads `transport/http.go`
with Go AST and `docs/api/openapi.yaml` with `gopkg.in/yaml.v3`, emits stable JSON,
and exits non-zero when `--fail-on-drift=true` and drift is present.

Current V1.2 scope:

- OpenAPI operation inventory and top-level response field extraction.
- JSON tag extraction helper for Go structs.
- Deterministic field-set diff helpers used by CI guard tests.

Run:

```bash
go run ./tools/contract_audit \
  --transport transport/http.go \
  --handlers transport/handler \
  --domain domain \
  --openapi docs/api/openapi.yaml \
  --output docs/iterations/V1_2_CONTRACT_AUDIT_v1.json \
  --markdown docs/iterations/V1_2_CONTRACT_AUDIT_v1.md \
  --fail-on-drift true
```

