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
- Unknown/unreviewed costs keep new-product ERP filing pending. Audited manual
  prices remain eligible. ERP image-only sync must not become a pricing source.
- Legacy rules remain available during transition. Conversion copies only basic
  rate/minimum/small-area settings as a draft; legacy formula/process semantics
  must be explicitly reviewed. This release does not certify legacy sample rates,
  migrate all business tariffs, or repair historical prices automatically.
- Migration 140 enlarges formula storage to TEXT; it does not change rates,
  bindings or historical amounts. Existing stored expressions remain intact.
- Local visual fixture: `/tests/fixtures/cost-model-workbench.html` on Vite dev
  server. It uses labelled fake data and prohibits saving; it is not a production
  calculation test or a production build entry.
