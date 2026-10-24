# Completed SKU cost maintenance

Scope: main-ops `vue/` and shared Go backend. The current route and field
authorities remain `transport/http.go` and `docs/api/openapi.yaml`.

- `/cost-rules` defaults to **编码成本**. **方案与绑定** retains scheme editing,
  binding checks, reconciliation and historical recalculation runs.
- The list reads canonical `task_sku_items` whose code has a completed/archived
  system task. Unrelated ERP-only products are excluded. Server keyset pagination
  orders task IDs and SKU sequence; the UI loads 20/50/100 rows and refreshes only
  its current page every 15 seconds, pausing during editing and in hidden tabs.
- Confirmation approves the displayed local price; it does not resend identity.
  Modification sends only changed fields and requires one of `area_incorrect`,
  `style_binding_incorrect`, `cost_incorrect`, `other`. Other requires text.
  Both `catalog.manage` and `erp.manage`, including task scope, are required.
- The default list shows **待确认** only. **已确认** and **全部编码** remain
  available as a separate review filter from ERP delivery status. The server
  applies the same current-audit predicate before choosing keyset page boundaries,
  scanning bounded batches under a five-second read budget. Completed confirmations
  disappear immediately from the pending view, including if its refresh fails.
- `reviewed` derives from the latest committed maintenance audit matching the
  current business values. An unchanged reviewed row shows **已确认** instead of
  another confirmation button, including while ERP delivery is pending and after
  reload. **修改** remains available. ERP convergence alone never implies human
  review. Changed values or an ERP conflict allow a fresh confirmation.
- The server checks the displayed local token and a direct ERP price baseline,
  locks/rechecks canonical data, then commits the change, current read mirrors,
  calculation snapshot, task event and dedicated maintenance log together.
  A stale price or specification returns 409 without a write. Retry requires
  a fresh review. Missing/unavailable ERP is never treated as a zero price.
- System commit is immediately effective. The existing cost worker delivers
  price-only updates and marks convergence only after direct ERP readback.
  Name/specification changes also queue the existing base-data worker. Their
  pending/failure state is shown separately from successful cost readback. ERP
  changes enter explicit conflict review; the drawer can adopt the displayed
  ERP price after another direct baseline check. No last-writer-wins overwrite.
- Width/height use centimetres; area uses square metres. Existing structured
  cost inputs use the established editor, preserving face/layout/piece semantics.
  Source-image dimensions are never used. A style change affects one SKU and
  requires an existing explicit style-to-scheme binding.
- Migration 144 creates an append-only log with SKU/reason/time indexes and
  structured before/after evidence. It never updates historical snapshots.
- Migration 145 canonicalizes previously task-only completed original-product
  codes, choosing the latest completed task per code and retaining original
  prices, dimensions and filing status. Existing canonical owners win. New
  historical states are quarantined for explicit review, not sent to ERP.
  Future original-product completion establishes the same identity inside the
  existing finalization transaction, without introducing a new completion path.
- Migration 146 removes only unknown/null dimension keys from newly canonicalized
  records and schedules a read-only ERP baseline observation for unreviewed
  historical codes. It never approves a price. New completion writes omit unknown
  dimensions directly, so the existing strict numeric read model remains valid.

Validation: full `scripts/agent-check.sh`; focused frontend unit tests and
`npm run build:prod`; isolated MySQL `TestCostMaintenanceIntegration`; desktop
and mobile browser checks for editing, log viewing and overflow. Integration
tests require `TEST_COST_SYNC_DSN` targeting `codex_cost_sync_test_*` and use
synthetic fixture data only.

Scheme maintenance: the sidebar uses the effective unified model name before old
fee-item names, searches every rule name and defaults to showing unbound schemes.
The binding dialog offers ERP-style pagination and selection up to 100 styles.
Existing bindings to another scheme require an explicit move review; partial
failures show progress and retain unprocessed selections. Bindings use the existing
POST/PATCH contracts and do not reprice historical SKUs. Legacy price multipliers
accept decimal factors such as 1.1 without a mismatched HTML step base, and the
saved factor is shown in the rule summary.
