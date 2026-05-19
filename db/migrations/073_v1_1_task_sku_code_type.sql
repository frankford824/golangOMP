-- Migration: 073_v1_1_task_sku_code_type.sql
-- Persist the product-code generation lane used for new automatic task SKUs.
-- Historical SKU strings are not rewritten.

ALTER TABLE task_details
  ADD COLUMN sku_code_type VARCHAR(32) NOT NULL DEFAULT 'regular'
    COMMENT 'Automatic SKU code type: regular or customization' AFTER product_channel;

ALTER TABLE task_sku_items
  ADD COLUMN sku_code_type VARCHAR(32) NOT NULL DEFAULT 'regular'
    COMMENT 'Automatic SKU code type: regular or customization' AFTER dedupe_key;

CREATE INDEX idx_task_details_sku_code_type ON task_details (sku_code_type);
CREATE INDEX idx_task_sku_items_sku_code_type ON task_sku_items (sku_code_type);

UPDATE rule_templates
SET
  config_json = '{"enabled":true,"prefixes":{"regular":"CG","customization":"DZ"},"category_short_code_length":1,"seq_length":6,"rule_types":["task_product_code"],"archived_rule_types":["new_sku"],"replacement":"product_code_sequences"}',
  updated_at = NOW(6)
WHERE template_type = 'product-code';
