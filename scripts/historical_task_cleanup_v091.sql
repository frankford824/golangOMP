START TRANSACTION;

-- Retained historical existing-product tasks: backfill product_id when an exact local products.sku_code match now exists.
UPDATE tasks t
JOIN products p ON p.sku_code = t.sku_code
SET t.product_id = p.id,
    t.updated_at = UTC_TIMESTAMP()
WHERE t.id IN (106,114,115,122,128,130,131,134,137,139,142,144)
  AND t.product_id IS NULL;

-- Explicit test / acceptance / demo / case tasks selected for deletion.
DELETE FROM cost_override_events
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM cost_override_event_sequences
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM cost_override_reviews
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM cost_override_finance_flags
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM audit_handovers
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM audit_records
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM outsource_orders
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM procurement_records
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM warehouse_receipts
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM task_event_logs
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM task_event_sequences
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM asset_storage_refs
WHERE asset_id IN (
    SELECT id
    FROM (
        SELECT id
        FROM design_assets
        WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136)
    ) AS design_assets_to_delete
)
   OR (
        owner_type = 'task_asset'
    AND owner_id IN (
        SELECT id
        FROM (
            SELECT id
            FROM task_assets
            WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136)
        ) AS task_assets_to_delete
    )
   );

DELETE FROM task_assets
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM design_assets
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM task_details
WHERE task_id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

DELETE FROM tasks
WHERE id IN (51,52,53,54,55,56,57,60,63,68,84,101,102,103,104,105,107,129,133,136);

COMMIT;
