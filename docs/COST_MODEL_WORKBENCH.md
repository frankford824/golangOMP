# Unified cost model workbench

Implementation scope: main-ops frontend and shared backend. Existing endpoints
remain authoritative in `transport/http.go` and `docs/api/openapi.yaml`.

## Operator SOP

1. Select a material/rule group in `/cost-rules`. Search also finds bound style codes.
2. Open **统一计价方案**. Confirm the business unit price and coefficient; choose
   area, piece, set, or manual quote. Optional thickness tiers use exact matching.
   Slotting, punching, lamination and double-sided work have explicit prices and
   units; the engine does not infer them from a product name.
3. Use the bottom-right floating calculator to test the unsaved configuration with known examples.
   Save the reviewed scheme and bind exact ERP/style codes. Unbound names cannot
   activate a unified model on a task.
4. For each SKU supply one sales unit's material usage: single-piece dimensions,
   a list of faces, total unfolded dimensions, or already-summed area. Total,
   layout and face areas are never multiplied by piece count again. Order quantity
   is not the cost unit. Complex inputs can be maintained in the task SKU editor.
5. Use **更新记录** to preview historical changes before applying or syncing them.
   Saving a rule alone does not rewrite historical costs or ERP records.

Formula: billed quantity × material price × coefficient, plus individually
priced process lines and optional small-area surcharge. Output includes input,
line items, rule/version and missing-field reasons. Draft previews do not write.

The left rail defaults to schemes with active style bindings; the middle panel
contains editable prices and binding search. The right panel paginates current
task SKUs under those active bindings (not historical cost matches), including
SKUs whose ERP filing is still pending. Its costs are stored amounts, not live
recalculations of an unsaved scheme. Catalog administration permission is required.
Legacy print prices are edited as single/double-sided prices, never raw formulas.
Internal priority and version identifiers are not operator form fields.

## Safety and rollout boundaries

- Model formulas are bounded JSON configurations (`cost_model`), not executable
  expressions. Editing a model formula creates a successor version.
- Missing specifications, unmatched thicknesses, unconfirmed configured processes,
  or conflicting models yield no automatic cost. A surcharge alone is not a price.
- Unknown/unreviewed costs do not block SKU identity filing; the cost field is
  omitted. ERP defaults and externally changed prices remain unconfirmed until
  an explicit decision. Image-only sync must not become a pricing source.
- Existing production schemes may retain multiple price components. Restore
  them with the audited operator tool below; do not require users to recreate
  existing tariffs. A manually created structured model is a separate option,
  not a prerequisite for an existing scheme to work.
- Migration 140 enlarges formula storage to TEXT; it does not change rates,
  bindings or historical amounts. Existing stored expressions remain intact.
- Local visual fixture: `/tests/fixtures/cost-model-workbench.html` on Vite dev
  server. It uses labelled fake data and prohibits saving; it is not a production
  calculation test or a production build entry.

## Restore the existing production tariffs

`cmd/tools/restore-cost-rules` previews the effective legacy price components
and exact style bindings. Apply requires the reviewed plan SHA256, database name
and a new recovery file. It creates successor rule versions without changing
rates, thresholds, formulas or process charges. Existing explicit bindings win;
opaque codes and missing tariff families are reported instead of guessed.
The existing **方案与绑定** view displays these schemes and their individual
editable charges. Re-entering the prices in a new model is not required.

`cmd/tools/repair-legacy-costs` previews affected, initially unpriced new-product
SKUs through the same bound-rule calculator. Human overrides, explicit cost
decisions, later ERP price changes and insufficient per-SKU specifications are
excluded. Apply requires the report SHA256 and an operator ID, rechecks ERP and
local revisions, records a durable journal and calculation snapshot, and queues
the normal cost-only worker. It never rewrites product identity or audit history.
Unreviewed ERP imports that cannot be recalculated retain their numeric value
but lose the false manual-confirmation flag and remain explicitly unconfirmed.
