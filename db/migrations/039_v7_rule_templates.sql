-- Rule templates for cost-pricing, product-code, short-name (v0.5)
CREATE TABLE IF NOT EXISTS rule_templates (
  id          BIGINT       NOT NULL AUTO_INCREMENT PRIMARY KEY,
  template_type VARCHAR(64) NOT NULL COMMENT 'cost-pricing, product-code, short-name',
  config_json TEXT         NOT NULL COMMENT 'JSON config per type',
  created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_rule_templates_type (template_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Seed default configs
INSERT INTO rule_templates (template_type, config_json) VALUES
  ('cost-pricing', '{"enabled":true,"base_price_mode":"manual","formula_expression":"","params":{}}'),
  ('product-code', '{"enabled":true,"prefix":"","date_format":"20060102","seq_length":6,"rule_types":["task_no","new_sku"]}'),
  ('short-name', '{"enabled":true,"max_length":32,"default_template":"{name}-{i_id}","scene_templates":{"new_product_development":"{name}-{i_id}","purchase_task":"{name}-{i_id}","original_product_update":"{name}-{i_id}"}}')
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP;
