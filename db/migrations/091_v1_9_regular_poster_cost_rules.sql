-- Migration: 091_v1_9_regular_poster_cost_rules.sql
-- Promote regular poster from photo-cloth aliasing to its own cost-rule category.

INSERT IGNORE INTO categories (
  category_code, category_name, display_name, parent_id, level_no, search_entry_code,
  is_search_entry, category_type, is_active, sort_order, source, remark
) VALUES (
  'POSTER_STANDARD', '常规海报', '常规海报', NULL, 1, 'POSTER_STANDARD',
  1, 'cloth', 1, 95, 'phase_020_sample', '常规海报独立成本规则入口'
);

INSERT IGNORE INTO cost_rules (
  rule_name, category_code, product_family, rule_type, base_price, tax_multiplier, min_area,
  area_threshold, surcharge_amount, special_process_keyword, special_process_price, formula_expression,
  priority, is_active, source, remark
) VALUES
  ('常规海报基础单价', 'POSTER_STANDARD', 'cloth', 'fixed_unit_price', 5.000, 1.1000, NULL, NULL, NULL, '', NULL, '', 10, 1, 'phase_020_sample', '海报按面积计价'),
  ('常规海报小面积附加', 'POSTER_STANDARD', 'cloth', 'area_threshold_surcharge', NULL, 1.1000, NULL, 0.1500, 3.000, '', NULL, '', 20, 1, 'phase_020_sample', '单件面积低于阈值加价');
