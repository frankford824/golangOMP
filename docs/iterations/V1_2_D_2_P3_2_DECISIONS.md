# V1.2-D-2 P3 · documented/mounted residual decisions

- date: 2026-04-26 PT
- scope: P3.1 infra exclusion + P3.2 mounted business route documentation
- rule: `transport/http.go` remains Tier-0; OpenAPI follows mounted business routes.

## Summary

| bucket | count | action |
|---|---:|---|
| infra/non-business | 5 | excluded from `contract_audit` path set: `/health`, `/healthz`, `/ping`, `/internal/*`, `/jst/*` |
| mounted business but missing OpenAPI | 7 | added OpenAPI path entries |
| documented but not mounted | 0 | none after P3 |

## Business Route Decisions

| method | path | transport evidence | handler | decision |
|---|---|---|---|---|
| GET | `/v1/rule-templates` | `transport/http.go:475` | `RuleTemplateHandler.List` | document response `data[] -> RuleTemplate` |
| GET | `/v1/rule-templates/{type}` | `transport/http.go:476` | `RuleTemplateHandler.GetByType` | document response `data -> RuleTemplate` |
| PUT | `/v1/rule-templates/{type}` | `transport/http.go:477` | `RuleTemplateHandler.Put` | document response `data -> RuleTemplate` |
| POST | `/v1/incidents/{id}/assign` | `transport/http.go:208` | `IncidentHandler.Assign` | document empty success response |
| POST | `/v1/incidents/{id}/resolve` | `transport/http.go:209` | `IncidentHandler.Resolve` | document empty success response |
| POST | `/v1/sku/preview_code` | `transport/http.go:189` | `SKUHandler.PreviewCode` | document empty success response |
| PUT | `/v1/policies/{id}` | `transport/http.go:216` | `PolicyHandler.Update` | document empty success response |

## P3 Audit

- output: `tmp/v1_2_d_2_p3_audit.json`
- result after P3: `missing_in_openapi=0`, `missing_in_code=0`
- remaining work moved to P4: response field drift only.
