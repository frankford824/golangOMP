-- Migration: 128_access_operations.sql
-- Introduce task operation codes and task-type matrices without collapsing
-- existing business roles or widening fine-grained planning/workbench grants.

SET @has_task_types := (
    SELECT COUNT(1)
    FROM information_schema.COLUMNS
    WHERE table_schema = DATABASE()
      AND table_name = 'auth_role_permissions'
      AND column_name = 'task_types'
);
SET @sql := IF(@has_task_types = 0,
    'ALTER TABLE auth_role_permissions ADD COLUMN task_types JSON NULL AFTER permission_code',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- Only one-to-one or explicitly split task operations are renamed. Planning,
-- asset publication/export and workbench administration stay fine grained so
-- a read/submit grant cannot silently become a manage grant.
INSERT INTO auth_permissions (code, module, name, description, risk_level) VALUES
  ('task.assign','task','指派任务','将待处理任务指派给处理人','high'),
  ('task.reassign','task','改派任务','改派当前处理人或设计师','high'),
  ('task.terminate','task','终止任务','终止进行中的任务','high'),
  ('task.upload_source','task','上传源文件','上传设计源文件并提交审核','normal'),
  ('task.audit','task','审核任务','通过或打回设计','high'),
  ('task.audit_handover','task','审核交班','审核人员交班与接手','high'),
  ('access.view','access','查看权限','查看角色与有效权限','normal'),
  ('access.manage','access','管理权限','维护角色、授权和组织策略','high')
ON DUPLICATE KEY UPDATE
  module = VALUES(module),
  name = VALUES(name),
  description = VALUES(description),
  risk_level = VALUES(risk_level),
  enabled = 1;

-- INSERT DISTINCT first, then delete the old rows. Updating primary-key values
-- in place would collide when one role owns more than one source operation.
INSERT IGNORE INTO auth_role_permissions
  (role_id, permission_code, task_types, created_by, created_at)
SELECT rp.role_id, map.new_code, rp.task_types, rp.created_by, rp.created_at
FROM auth_role_permissions rp
JOIN (
  SELECT 'task.design.submit' AS old_code, 'task.upload_source' AS new_code UNION ALL
  SELECT 'task.audit.decision', 'task.audit' UNION ALL
  SELECT 'task.manage', 'task.assign' UNION ALL
  SELECT 'task.manage', 'task.reassign' UNION ALL
  SELECT 'task.manage', 'task.terminate' UNION ALL
  SELECT 'access_policy.view', 'access.view' UNION ALL
  SELECT 'access_policy.manage', 'access.manage'
) map ON map.old_code = rp.permission_code;

-- Handover is deliberately separate from audit decision. Preserve it only for
-- the known built-in audit roles; custom roles require an explicit grant.
INSERT IGNORE INTO auth_role_permissions (role_id, permission_code, task_types)
SELECT r.id, 'task.audit_handover', NULL
FROM auth_roles r
WHERE r.code IN ('super_admin', 'auditor', 'design_director')
  AND r.archived_at IS NULL;

DELETE rp
FROM auth_role_permissions rp
WHERE rp.permission_code IN (
  'task.design.submit',
  'task.audit.decision',
  'task.manage',
  'access_policy.view',
  'access_policy.manage'
);

DELETE FROM auth_permissions
WHERE code IN (
  'task.design.submit',
  'task.audit.decision',
  'task.manage',
  'access_policy.view',
  'access_policy.manage'
);

UPDATE auth_permissions
SET name = CASE code
  WHEN 'task.view' THEN '查看任务'
  WHEN 'task.create' THEN '创建任务'
  WHEN 'task.reopen' THEN '重开任务'
  WHEN 'asset.view' THEN '查看资产'
  WHEN 'asset.download' THEN '下载资产'
  WHEN 'asset.export' THEN '批量导出资产'
  WHEN 'asset.publish' THEN '发布客户素材'
  WHEN 'asset.manage' THEN '维护资产'
  WHEN 'catalog.view' THEN '查看产品资料'
  WHEN 'catalog.manage' THEN '维护产品资料'
  WHEN 'erp.manage' THEN '管理 ERP'
  WHEN 'account.use' THEN '使用个人工作台'
  WHEN 'report.view' THEN '查看经营报表'
  WHEN 'system.manage' THEN '管理系统'
  ELSE name
END,
description = CASE code
  WHEN 'task.view' THEN '查看授权范围内的任务'
  WHEN 'task.create' THEN '创建业务任务（可按任务类型限制）'
  WHEN 'task.reopen' THEN '重开已结单任务'
  WHEN 'asset.view' THEN '查看授权范围内的资源组'
  WHEN 'asset.download' THEN '下载资源组文件'
  WHEN 'asset.export' THEN '批量打包授权范围内的资源组'
  WHEN 'asset.publish' THEN '发布固定最终修订到客户素材'
  WHEN 'asset.manage' THEN '上传、归档及维护资产业务数据'
  WHEN 'catalog.view' THEN '查看产品分类与业务规则'
  WHEN 'catalog.manage' THEN '维护分类、成本规则和编号规则'
  WHEN 'erp.manage' THEN '管理 ERP 建档、重试和告警'
  WHEN 'account.use' THEN '维护个人资料、通知和工作台偏好'
  WHEN 'report.view' THEN '查看经营指标和分析结果'
  WHEN 'system.manage' THEN '维护系统配置、日志和集成'
  ELSE description
END
WHERE code IN (
  'task.view','task.create','task.reopen','asset.view','asset.download','asset.export','asset.publish','asset.manage',
  'catalog.view','catalog.manage','erp.manage','account.use','report.view','system.manage'
);

-- Role identities and assignments remain unchanged. In particular,
-- access_admin is not promoted to protected super_admin, and submitters are not
-- merged into asset managers.
UPDATE auth_roles SET name = '管理员', description = '全局管理角色（系统保护）' WHERE code = 'super_admin';
UPDATE auth_roles SET name = '普通成员', description = '系统默认基础角色' WHERE code = 'member';
UPDATE auth_roles SET name = '运营', description = '任务创建、指派与运营处理' WHERE code = 'operations';
UPDATE auth_roles SET name = '设计', description = '设计处理与源文件上传' WHERE code = 'designer';
UPDATE auth_roles SET name = '审核', description = '任务审核与交班' WHERE code = 'auditor';

UPDATE auth_policy_state
SET policy_revision = policy_revision + 1
WHERE singleton_id = 1;

-- ROLLBACK-BEGIN
INSERT INTO auth_permissions (code, module, name, description, risk_level) VALUES
  ('task.design.submit','task','提交设计','提交设计资源进入审核','normal'),
  ('task.audit.decision','task','审核决策','通过或打回设计','high'),
  ('task.manage','task','维护任务','分配、维护业务信息及执行任务节点动作','high'),
  ('access_policy.view','access','查看权限策略','查看角色、能力和有效权限','normal'),
  ('access_policy.manage','access','管理权限策略','维护角色、授权和组织策略','high')
ON DUPLICATE KEY UPDATE enabled = 1;

INSERT IGNORE INTO auth_role_permissions (role_id, permission_code, created_by, created_at)
SELECT rp.role_id, map.old_code, rp.created_by, rp.created_at
FROM auth_role_permissions rp
JOIN (
  SELECT 'task.upload_source' AS new_code, 'task.design.submit' AS old_code UNION ALL
  SELECT 'task.audit', 'task.audit.decision' UNION ALL
  SELECT 'task.assign', 'task.manage' UNION ALL
  SELECT 'task.reassign', 'task.manage' UNION ALL
  SELECT 'task.terminate', 'task.manage' UNION ALL
  SELECT 'access.view', 'access_policy.view' UNION ALL
  SELECT 'access.manage', 'access_policy.manage'
) map ON map.new_code = rp.permission_code;

DELETE FROM auth_role_permissions
WHERE permission_code IN (
  'task.assign','task.reassign','task.terminate','task.upload_source','task.audit','task.audit_handover',
  'access.view','access.manage'
);

DELETE FROM auth_permissions
WHERE code IN (
  'task.assign','task.reassign','task.terminate','task.upload_source','task.audit','task.audit_handover',
  'access.view','access.manage'
);

ALTER TABLE auth_role_permissions DROP COLUMN task_types;
-- ROLLBACK-END
