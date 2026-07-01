-- Migration: 099_v1_13_experience_reason_tags_seed.sql
-- Seed the first controlled reason tags for low-friction AI suggestion feedback.
-- Side-channel only: this does not change task, asset, audit, or ERP state.

INSERT INTO experience_reason_tags (
  scene,
  code,
  name,
  tag_group,
  severity,
  version,
  enabled,
  sort_order,
  created_by,
  updated_by
)
SELECT
  seed.scene,
  seed.code,
  seed.name,
  seed.tag_group,
  seed.severity,
  1,
  1,
  seed.sort_order,
  0,
  0
FROM (
  SELECT 'ai_suggestion_feedback' AS scene, 'spec_mismatch' AS code, '规格不匹配' AS name, 'feedback_reason' AS tag_group, 'medium' AS severity, 10 AS sort_order
  UNION ALL SELECT 'ai_suggestion_feedback', 'asset_mismatch', '资产不匹配', 'feedback_reason', 'medium', 20
  UNION ALL SELECT 'ai_suggestion_feedback', 'stage_not_applicable', '阶段不适用', 'feedback_reason', 'medium', 30
  UNION ALL SELECT 'ai_suggestion_feedback', 'missing_context', '缺少上下文', 'feedback_reason', 'medium', 40
  UNION ALL SELECT 'ai_suggestion_feedback', 'outdated', '信息过期', 'feedback_reason', 'low', 50
  UNION ALL SELECT 'ai_suggestion_feedback', 'customer_special_case', '客户特殊要求', 'feedback_reason', 'high', 60
  UNION ALL SELECT 'ai_suggestion_feedback', 'already_handled', '已由人工处理', 'feedback_reason', 'low', 70
  UNION ALL SELECT 'ai_suggestion_feedback', 'not_relevant', '不相关', 'feedback_reason', 'medium', 80
) AS seed
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  tag_group = VALUES(tag_group),
  severity = VALUES(severity),
  enabled = 1,
  sort_order = VALUES(sort_order),
  updated_by = VALUES(updated_by);

-- ROLLBACK-BEGIN
UPDATE experience_reason_tags
SET enabled = 0, deleted_at = COALESCE(deleted_at, NOW()), updated_by = 0
WHERE scene = 'ai_suggestion_feedback'
  AND code IN (
    'spec_mismatch',
    'asset_mismatch',
    'stage_not_applicable',
    'missing_context',
    'outdated',
    'customer_special_case',
    'already_handled',
    'not_relevant'
  );
-- ROLLBACK-END
