-- Migration: 072_v1_1_archive_legacy_coderule_new_sku.sql
-- Archive the legacy CodeRule new_sku generator.
-- Current SKU/product-code allocation is owned by product_code_sequences:
--   NS + category short code + 6-digit sequence.

UPDATE code_rules
SET
  is_enabled = 0,
  rule_name = CONCAT('[ARCHIVED] ', REPLACE(rule_name, '[ARCHIVED] ', '')),
  config_json = JSON_SET(
    COALESCE(config_json, JSON_OBJECT()),
    '$.archived', true,
    '$.archived_reason', 'legacy CodeRule new_sku is replaced by task product-code allocation',
    '$.replacement', 'POST /v1/tasks/prepare-product-codes'
  ),
  updated_at = CURRENT_TIMESTAMP
WHERE rule_type = 'new_sku';

UPDATE rule_templates
SET
  config_json = '{"enabled":true,"prefix":"NS","seq_length":6,"rule_types":["task_product_code"],"archived_rule_types":["new_sku"],"replacement":"product_code_sequences"}',
  updated_at = CURRENT_TIMESTAMP
WHERE template_type = 'product-code';
