# ITERATION 001 — V7 Domain Skeleton

**Date**: 2026-03-06
**Scope**: STEP_01 — Task / Product / CodeRule domain objects, minimal repo/service/handler skeleton, DB migration script.

---

## 1. Goals

- Add V7 domain entities: Product, Task, TaskDetail, CodeRule.
- Add V7 enums: TaskStatus, TaskType, TaskSourceMode, TaskPriority, CodeRuleType, ResetCycle.
- Provide minimal compilable repo, service, handler for Product / Task / CodeRule.
- Register new routes in the router.
- Write DB migration 001 (additive — no V6 tables dropped).
- Update openapi.yaml with all new endpoints.

---

## 2. Changed Files

### New files
| File | Purpose |
|---|---|
| `domain/enums_v7.go` | V7-specific enums (TaskStatus, CodeRuleType, etc.) |
| `domain/product.go` | Product entity |
| `domain/task.go` | Task + TaskDetail entities |
| `domain/code_rule.go` | CodeRule + CodePreview entities |
| `repo/mysql/product.go` | Product MySQL repo (Search, GetByID) |
| `repo/mysql/task.go` | Task MySQL repo (Create, GetByID, List) |
| `repo/mysql/code_rule.go` | CodeRule MySQL repo (ListAll, GetByID, GetEnabledByType, NextSeq) |
| `service/product_service.go` | ProductService (Search, GetByID) |
| `service/task_service.go` | TaskService (Create, List, GetByID) |
| `service/code_rule_service.go` | CodeRuleService (List, Preview, GenerateCode, GenerateSKU) |
| `transport/handler/product.go` | ProductHandler (Search, GetByID) |
| `transport/handler/task.go` | TaskHandler (Create, List, GetByID) |
| `transport/handler/code_rule.go` | CodeRuleHandler (List, Preview, GenerateSKU) |
| `db/migrations/001_v7_tables.sql` | Additive DB migration |
| `docs/iterations/ITERATION_001.md` | This file |

### Modified files
| File | Change |
|---|---|
| `repo/interfaces.go` | Added ProductRepo, TaskRepo, CodeRuleRepo + filter types |
| `transport/http.go` | Added productH/taskH/codeRuleH params + 7 new routes |
| `cmd/server/main.go` | Wired V7 repos → services → handlers |
| `transport/handler/sku.go` | Added parseInt / parseInt64 helpers |
| `docs/api/openapi.yaml` | Full rewrite with V7 + V6 legacy paths |

---

## 3. Database Changes

### New Tables
| Table | Key Constraint |
|---|---|
| `products` | UNIQUE(`erp_product_id`) |
| `tasks` | UNIQUE(`task_no`) |
| `task_details` | UNIQUE(`task_id`), FK → tasks |
| `code_rules` | — |
| `code_rule_sequences` | PK(`rule_id`), FK → code_rules |

### Seed Data
- 4 default code rules inserted (task_no, new_sku, outsource_no, handover_no).

### V6 Tables: Unchanged
`skus`, `asset_versions`, `distribution_jobs`, `event_logs`, `audit_actions`, `incidents`, `sku_sequences` — all preserved.

---

## 4. API Changes

### New Endpoints (V7)
| Method | Path | Handler |
|---|---|---|
| GET | `/v1/products/search` | ProductHandler.Search |
| GET | `/v1/products/:id` | ProductHandler.GetByID |
| POST | `/v1/tasks` | TaskHandler.Create |
| GET | `/v1/tasks` | TaskHandler.List |
| GET | `/v1/tasks/:id` | TaskHandler.GetByID |
| GET | `/v1/code-rules` | CodeRuleHandler.List |
| GET | `/v1/code-rules/:id/preview` | CodeRuleHandler.Preview |
| POST | `/v1/code-rules/generate-sku` | CodeRuleHandler.GenerateSKU |

### V6 Endpoints: All preserved
No existing endpoints were removed or modified.

---

## 5. Key Design Decisions

1. **Additive migration**: V6 tables untouched. V7 tables are new.
2. **CodeRule sequence counter**: Uses `code_rule_sequences` table with SELECT FOR UPDATE inside a transaction — same pattern as `sku_sequences`.
3. **Task.sku_code gate**: Enforced in TaskService.Create; empty sku_code → 400.
4. **Preview vs. generate separation**: Preview uses seq=0 and does not touch `code_rule_sequences`.
5. **infraError reuse**: Shared package-level helper from `service/sku.go`.

---

## 6. Incomplete / Next Steps

- `task_assets`, `audit_records`, `audit_handovers`, `outsource_orders`, `warehouse_receipts` tables — next iterations.
- TaskService.Create does not yet write to `event_logs` (V6 EventLog is SKU-scoped; V7 needs task-scoped event log design).
- No authentication/RBAC middleware yet.
- ERP sync worker (pulls from ERP every 5 min into `products` table) not yet implemented.
- Frontend stores (Pinia) — separate workstream.

---

## 7. Next Iteration (STEP_02 suggestion)

- Add `audit_records` + `audit_handovers` domain + repo + service + handler skeleton.
- Add TaskService event_log integration (extend EventRepo or add task-scoped log).
- Add `outsource_orders` domain skeleton.
