# SKU planning numbering rule activation

## Approved production rule

The owner approved native `sku_planning` creation to use the same numbering
scheme as the latest retired purchase-task flow:

```text
<CG|DZ><category letter><six-digit sequence>
```

- `CG`: regular SKU.
- `DZ`: customization SKU.
- category letter: the existing one-character category code selected by the
  operator.
- sequence: the shared `product_code_sequences` counter for the same prefix and
  category. Native planning must continue the existing sequence rather than
  create a parallel revision-scoped counter.

Examples: `CGC000065`, `DZH000012`.

Historical SKU identities are immutable. This approval changes only newly
created native planning SKUs; it does not rewrite migrated `NS*`, `CG*`, or
`DZ*` values.

## Activation migration

Migration `130_activate_legacy_purchase_sku_planning_rule.sql`:

1. Locks the single expected disabled `sku_planning` placeholder.
2. Verifies its immutable revision-1 fingerprint.
3. Inserts immutable revision 2 with strategy
   `legacy_task_product_code_v1`.
4. Enables the rule and points it to revision 2 atomically.
5. Fails closed when the placeholder has drifted or a competing rule exists.
6. Leaves historical task settings on their original frozen revision.

The migration locates the rule by business type and fingerprint; it does not
assume that production IDs match Clone B IDs.

## Runtime behavior

- Each request validates `category_code` and `sku_code_type`.
- Prefix/category groups are locked in deterministic order.
- Ranges are allocated through the existing `product_code_sequences` table.
- The original input order is preserved in the response.
- Idempotent retries return the original task without consuming new numbers.
- The create page shows the formula before submission and the exact assigned
  SKU values after the server commits the task.

## Production verification

After migration and before accepting the release:

1. Exactly one `sku_planning` rule is enabled and its active revision belongs
   to the same rule.
2. The active revision config is `legacy_task_product_code_v1` with `CG`, `DZ`,
   one category character, six digits, and `product_code_sequences`.
3. A controlled planning task shows the formula before submission and the
   exact returned SKU codes after creation.
4. The generated values do not collide with existing `task_sku_items.sku_code`.
5. Retrying the same `client_create_id` returns the same task and values.

Rollback is safe only before revision 2 has been referenced by a created task.
After allocation, correction must be a forward immutable rule revision; issued
SKU codes must never be reused.
