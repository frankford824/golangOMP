-- Migration: 130_activate_legacy_purchase_sku_planning_rule.sql
-- Activate planning SKU generation with the last production purchase-task
-- numbering semantics: CG/DZ + one deterministic category letter + 6 digits.
-- Historical SKU strings and immutable revision 1 are not rewritten.

SET @planning_rule_count := (
  SELECT COUNT(*)
  FROM code_rules
  WHERE rule_type = 'sku_planning'
);

SET @planning_rule_id := (
  SELECT MIN(id)
  FROM code_rules
  WHERE rule_type = 'sku_planning'
);

SET @planning_previous_revision_id := (
  SELECT MIN(id)
  FROM code_rule_revisions
  WHERE rule_id = @planning_rule_id
    AND version_no = 1
);

SET @planning_placeholder_count := (
  SELECT COUNT(*)
  FROM code_rules rule_row
  JOIN code_rule_revisions revision_row
    ON revision_row.rule_id = rule_row.id
   AND revision_row.id = rule_row.active_revision_id
  WHERE rule_row.id = @planning_rule_id
    AND rule_row.is_enabled = 0
    AND revision_row.version_no = 1
    AND revision_row.prefix = 'SKU'
    AND revision_row.separator_text = '-'
    AND revision_row.seq_length = 6
    AND revision_row.reset_cycle = 'none'
    AND revision_row.dimension_mode = 'none'
    AND JSON_EXTRACT(revision_row.config_json, '$.requires_admin_activation') = TRUE
);

SET @planning_activated_count := (
  SELECT COUNT(*)
  FROM code_rules rule_row
  JOIN code_rule_revisions revision_row
    ON revision_row.rule_id = rule_row.id
   AND revision_row.id = rule_row.active_revision_id
  WHERE rule_row.id = @planning_rule_id
    AND rule_row.is_enabled = 1
    AND revision_row.version_no = 2
    AND revision_row.prefix = ''
    AND revision_row.separator_text = ''
    AND revision_row.seq_length = 6
    AND revision_row.reset_cycle = 'none'
    AND revision_row.dimension_mode = 'category_code'
    AND JSON_UNQUOTE(JSON_EXTRACT(revision_row.config_json, '$.strategy')) = 'legacy_task_product_code_v1'
    AND JSON_UNQUOTE(JSON_EXTRACT(revision_row.config_json, '$.prefixes.regular')) = 'CG'
    AND JSON_UNQUOTE(JSON_EXTRACT(revision_row.config_json, '$.prefixes.customization')) = 'DZ'
    AND JSON_EXTRACT(revision_row.config_json, '$.category_short_code_length') = 1
    AND JSON_EXTRACT(revision_row.config_json, '$.sequence_length') = 6
);

SET @planning_guard_sql := IF(
  @planning_rule_count = 1
    AND (@planning_placeholder_count = 1 OR @planning_activated_count = 1),
  'SELECT 1',
  'SELECT * FROM __planning_sku_rule_state_mismatch__'
);
PREPARE planning_guard_stmt FROM @planning_guard_sql;
EXECUTE planning_guard_stmt;
DEALLOCATE PREPARE planning_guard_stmt;

INSERT INTO code_rule_revisions (
  rule_id,
  version_no,
  prefix,
  date_format,
  site_code,
  biz_code,
  separator_text,
  seq_length,
  reset_cycle,
  dimension_mode,
  config_json,
  created_by
)
SELECT
  @planning_rule_id,
  2,
  '',
  '',
  '',
  '',
  '',
  6,
  'none',
  'category_code',
  JSON_OBJECT(
    'strategy', 'legacy_task_product_code_v1',
    'prefixes', JSON_OBJECT('regular', 'CG', 'customization', 'DZ'),
    'category_short_code_length', 1,
    'sequence_length', 6,
    'sequence_store', 'product_code_sequences',
    'approved_by_reviewer_id', 1,
    'supersedes_rule_revision_id', @planning_previous_revision_id
  ),
  (SELECT id FROM users WHERE id = 1 LIMIT 1)
WHERE NOT EXISTS (
  SELECT 1
  FROM code_rule_revisions
  WHERE rule_id = @planning_rule_id
    AND version_no = 2
);

SET @planning_revision_id := (
  SELECT id
  FROM code_rule_revisions
  WHERE rule_id = @planning_rule_id
    AND version_no = 2
);

SET @planning_revision_fingerprint_count := (
  SELECT COUNT(*)
  FROM code_rule_revisions
  WHERE id = @planning_revision_id
    AND prefix = ''
    AND separator_text = ''
    AND seq_length = 6
    AND reset_cycle = 'none'
    AND dimension_mode = 'category_code'
    AND JSON_UNQUOTE(JSON_EXTRACT(config_json, '$.strategy')) = 'legacy_task_product_code_v1'
    AND JSON_UNQUOTE(JSON_EXTRACT(config_json, '$.prefixes.regular')) = 'CG'
    AND JSON_UNQUOTE(JSON_EXTRACT(config_json, '$.prefixes.customization')) = 'DZ'
    AND JSON_EXTRACT(config_json, '$.category_short_code_length') = 1
    AND JSON_EXTRACT(config_json, '$.sequence_length') = 6
    AND JSON_EXTRACT(config_json, '$.supersedes_rule_revision_id') = @planning_previous_revision_id
);

SET @planning_revision_guard_sql := IF(
  @planning_revision_fingerprint_count = 1,
  'SELECT 1',
  'SELECT * FROM __planning_sku_revision_fingerprint_mismatch__'
);
PREPARE planning_revision_guard_stmt FROM @planning_revision_guard_sql;
EXECUTE planning_revision_guard_stmt;
DEALLOCATE PREPARE planning_revision_guard_stmt;

UPDATE code_rules
SET
  rule_name = '策划 SKU 编号规则（旧采购任务口径）',
  prefix = 'CG',
  date_format = '',
  site_code = '',
  biz_code = '',
  seq_length = 6,
  reset_cycle = 'none',
  is_enabled = 1,
  active_revision_id = @planning_revision_id,
  config_json = JSON_OBJECT(
    'strategy', 'legacy_task_product_code_v1',
    'prefixes', JSON_OBJECT('regular', 'CG', 'customization', 'DZ'),
    'category_short_code_length', 1,
    'sequence_length', 6,
    'sequence_store', 'product_code_sequences',
    'approved_by_reviewer_id', 1,
    'supersedes_rule_revision_id', @planning_previous_revision_id
  ),
  updated_at = CURRENT_TIMESTAMP
WHERE id = @planning_rule_id;

-- ROLLBACK-BEGIN
UPDATE code_rules rule_row
JOIN code_rule_revisions revision_row
  ON revision_row.rule_id = rule_row.id
 AND revision_row.version_no = 1
SET
  rule_row.rule_name = '策划 SKU 编号规则（待配置）',
  rule_row.prefix = 'SKU',
  rule_row.date_format = '',
  rule_row.site_code = '',
  rule_row.biz_code = '',
  rule_row.seq_length = 6,
  rule_row.reset_cycle = 'none',
  rule_row.is_enabled = 0,
  rule_row.active_revision_id = revision_row.id,
  rule_row.config_json = JSON_OBJECT('dimension_mode', 'none', 'requires_admin_activation', TRUE),
  rule_row.updated_at = CURRENT_TIMESTAMP
WHERE rule_row.rule_type = 'sku_planning';

DELETE revision_row
FROM code_rule_revisions revision_row
JOIN code_rules rule_row ON rule_row.id = revision_row.rule_id
WHERE rule_row.rule_type = 'sku_planning'
  AND revision_row.version_no = 2
  AND NOT EXISTS (
    SELECT 1
    FROM task_planning_settings settings_row
    WHERE settings_row.code_rule_revision_id = revision_row.id
  );
-- ROLLBACK-END
