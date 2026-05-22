-- Migration: 075_v1_2_cost_rule_tax_multiplier_refresh.sql
-- Align existing seeded cost rules with the current tax-inclusive pricing rules.

UPDATE cost_rules
SET tax_multiplier = 1.1000,
    updated_at = CURRENT_TIMESTAMP
WHERE source = 'phase_020_sample'
  AND (
    (category_code = 'KT_RED' AND rule_name = '红色KT板基础单价')
    OR (category_code = 'KT_GOLD' AND rule_name = '金色KT板基础单价')
    OR (category_code = 'KT_STANDARD_FILM' AND rule_name = '常规覆膜KT板基础单价')
    OR (category_code = 'KT_CUSTOM_FILM' AND rule_name = '定制覆膜KT板基础单价')
    OR (category_code = 'PHOTO_CLOTH_STANDARD' AND rule_name IN ('常规写真布基础单价', '常规写真布小面积附加'))
    OR (category_code = 'PHOTO_CLOTH_CUSTOM' AND rule_name IN ('定制写真布基础单价', '定制写真布小面积附加'))
    OR (category_code = 'FLAG_CLOTH_STANDARD' AND rule_name = '常规旗帜布基础单价')
    OR (category_code = 'FLAG_CLOTH_SEWED' AND rule_name = '车缝旗帜布基础单价')
  );
