-- Migration: 076_v1_2_spray_cloth_cost_rules.sql
-- Add spray cloth cost rules so standard/custom spray cloth tasks can auto-prefill cost.

INSERT IGNORE INTO cost_rules (
  rule_name, category_code, product_family, rule_type, base_price, tax_multiplier, min_area,
  area_threshold, surcharge_amount, special_process_keyword, special_process_price, formula_expression,
  priority, is_active, source, remark
) VALUES
  ('常规喷绘布基础单价', 'SPRAY_CLOTH_STANDARD', 'cloth', 'fixed_unit_price', 4.00, 1.1000, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '喷绘布按面积计价'),
  ('定制喷绘布基础单价', 'SPRAY_CLOTH_CUSTOM', 'cloth', 'fixed_unit_price', 4.00, 1.1000, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '喷绘布按面积计价');
