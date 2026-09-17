-- Production-confirmed PP adhesive pricing:
-- base material (matte or gloss lamination) = 8 CNY/m2;
-- explicit punching = +1 CNY per piece.
-- The fixed-area rule deliberately returns no automatic estimate when area is
-- absent; the service keeps such records in manual review.

INSERT INTO cost_rules (
  rule_name, rule_version, category_id, category_code, product_family,
  rule_type, base_price, priority, is_active, governance_note, source, remark
)
SELECT
  '常规PP背胶面积成本', 1, c.id, 'PP_STICKY', 'paper',
  'fixed_unit_price', 8.000, 10, 1,
  '按确认报价表：哑光膜/亮光膜均按8元/平方米；无面积时禁止自动计价。',
  'production_verified_20260917', 'PP背胶面积成本修复'
FROM categories c
WHERE c.category_code = 'PP_STICKY'
  AND NOT EXISTS (
    SELECT 1 FROM cost_rules r
    WHERE r.category_code = 'PP_STICKY'
      AND r.rule_name = '常规PP背胶面积成本'
      AND r.rule_version = 1
  );

INSERT INTO cost_rules (
  rule_name, rule_version, category_id, category_code, product_family,
  rule_type, special_process_keyword, special_process_price,
  priority, is_active, governance_note, source, remark
)
SELECT
  '常规PP背胶打孔附加', 1, c.id, 'PP_STICKY', 'paper',
  'special_process_surcharge', '打孔', 1.000,
  20, 1, '按确认报价表：仅明确打孔时加1元/份。',
  'production_verified_20260917', 'PP背胶打孔附加修复'
FROM categories c
WHERE c.category_code = 'PP_STICKY'
  AND NOT EXISTS (
    SELECT 1 FROM cost_rules r
    WHERE r.category_code = 'PP_STICKY'
      AND r.rule_name = '常规PP背胶打孔附加'
      AND r.rule_version = 1
  );

INSERT INTO cost_rule_bindings (
  i_id_raw, normalized_i_id, rule_group, display_name, source,
  is_active, created_by, updated_by
)
SELECT
  '常规PP背胶', '常规PP背胶', 'PP_STICKY', '常规PP背胶',
  'production_verified_20260917', 1, 1, 1
WHERE NOT EXISTS (
  SELECT 1 FROM cost_rule_bindings b
  WHERE b.normalized_i_id = '常规PP背胶' AND b.is_active = 1
);

-- ROLLBACK
-- UPDATE cost_rule_bindings SET is_active = 0
-- WHERE normalized_i_id = '常规PP背胶' AND source = 'production_verified_20260917';
-- UPDATE cost_rules SET is_active = 0
-- WHERE category_code = 'PP_STICKY' AND source = 'production_verified_20260917';
