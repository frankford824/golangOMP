-- Migration: 132_simplify_access_roles_without_narrowing.sql
-- Collapse the assignable role catalog without narrowing any effective access.
--
-- Final active role families:
--   hidden system baseline: member
--   management: super_admin, department_admin
--   main business: operations, auditor, designer, customization_operator
--   asset workbench: unchanged
--
-- Historical assignments remain for audit, but their roles are archived only
-- after a broader-or-equal migration overlay has been inserted.

UPDATE auth_roles
SET name = CASE code
      WHEN 'member' THEN '系统基础角色'
      WHEN 'super_admin' THEN '超级管理员'
      WHEN 'operations' THEN '运营'
      WHEN 'auditor' THEN '审核'
      WHEN 'designer' THEN '常规设计师'
      WHEN 'customization_operator' THEN '定制设计师'
      WHEN 'department_admin' THEN '部门管理员'
      ELSE name
    END,
    description = CASE code
      WHEN 'member' THEN '系统自动赋予的隐藏基础角色'
      WHEN 'super_admin' THEN '全局系统、权限与 ERP 管理'
      WHEN 'operations' THEN '任务创建、指派与运营处理'
      WHEN 'auditor' THEN '任务审核、交班与重开'
      WHEN 'designer' THEN '常规设计处理与源文件上传'
      WHEN 'customization_operator' THEN '定制设计处理与源文件上传'
      WHEN 'department_admin' THEN '管理授权范围内的部门任务与资源'
      ELSE description
    END,
    version = version + 1
WHERE code IN (
  'member',
  'super_admin',
  'operations',
  'auditor',
  'designer',
  'customization_operator',
  'department_admin'
);

-- A former design director could reopen tasks. Granting reopen to the retained
-- auditor role preserves that operation when design directors are converted
-- to department_admin + designer + auditor.
INSERT IGNORE INTO auth_role_permissions (role_id, permission_code, task_types)
SELECT id, 'task.reopen', NULL
FROM auth_roles
WHERE code = 'auditor'
  AND archived_at IS NULL;

-- Team leads and design directors become department administrators.
-- own_department is deliberately broader than own_team; if either historical
-- management role was global, the merged department-admin overlay stays
-- global. Aggregating both source roles avoids a narrower team-lead row
-- masking a broader design-director row for the same user.
INSERT IGNORE INTO auth_user_role_assignments (
  user_id,
  role_id,
  scope_mode,
  source_type,
  source_ref_id,
  assigned_by
)
SELECT DISTINCT
  source_assignment.user_id,
  target_role.id,
  CASE
    WHEN MAX(source_assignment.scope_mode = 'global') = 1 THEN 'global'
    ELSE 'own_department'
  END,
  'migration',
  132,
  NULL
FROM auth_user_role_assignments source_assignment
JOIN auth_roles source_role
  ON source_role.id = source_assignment.role_id
 AND source_role.code IN ('team_lead', 'design_director')
JOIN auth_roles target_role
  ON target_role.code = 'department_admin'
 AND target_role.archived_at IS NULL
GROUP BY source_assignment.user_id, target_role.id;

-- Design directors become department administrators while retaining the
-- complete design/audit/reopen operation set through active business roles.
-- All current design-director assignments belong to the visual design
-- department, so their retained design lane is the normal designer lane.
INSERT IGNORE INTO auth_user_role_assignments (
  user_id,
  role_id,
  scope_mode,
  source_type,
  source_ref_id,
  assigned_by
)
SELECT DISTINCT
  source_assignment.user_id,
  target_role.id,
  CASE
    WHEN MAX(source_assignment.scope_mode = 'global') = 1 THEN 'global'
    ELSE 'own_department'
  END,
  'migration',
  132,
  NULL
FROM auth_user_role_assignments source_assignment
JOIN auth_roles source_role
  ON source_role.id = source_assignment.role_id
 AND source_role.code = 'design_director'
JOIN auth_roles target_role
  ON target_role.code IN ('designer', 'auditor')
 AND target_role.archived_at IS NULL
GROUP BY source_assignment.user_id, target_role.id;

-- Removing access_admin cannot revoke access.manage. Existing holders are
-- promoted to the protected super-admin role before access_admin is archived.
-- The same rule makes any late ERP-operator assignment safe before that role
-- is retired.
INSERT IGNORE INTO auth_user_role_assignments (
  user_id,
  role_id,
  scope_mode,
  source_type,
  source_ref_id,
  assigned_by
)
SELECT DISTINCT
  source_assignment.user_id,
  target_role.id,
  'global',
  'migration',
  132,
  NULL
FROM auth_user_role_assignments source_assignment
JOIN auth_roles source_role
  ON source_role.id = source_assignment.role_id
 AND source_role.code IN ('access_admin', 'erp_operator')
JOIN auth_roles target_role
  ON target_role.code = 'super_admin'
 AND target_role.archived_at IS NULL;

-- Keep historical rows and assignments for auditability. Archived roles are
-- excluded from effective access and from the assignable-role API surface.
UPDATE auth_roles
SET archived_at = COALESCE(archived_at, UTC_TIMESTAMP()),
    version = version + 1
WHERE code IN ('team_lead', 'design_director', 'access_admin', 'erp_operator')
  AND archived_at IS NULL;

UPDATE auth_policy_state
SET policy_revision = policy_revision + 1
WHERE singleton_id = 1;

-- ROLLBACK-BEGIN
UPDATE auth_roles
SET archived_at = NULL,
    version = version + 1
WHERE code IN ('team_lead', 'design_director', 'access_admin', 'erp_operator');

DELETE FROM auth_user_role_assignments
WHERE source_type = 'migration'
  AND source_ref_id = 132;

DELETE role_permission
FROM auth_role_permissions role_permission
JOIN auth_roles role ON role.id = role_permission.role_id
WHERE role.code = 'auditor'
  AND role_permission.permission_code = 'task.reopen';

UPDATE auth_roles
SET name = CASE code
      WHEN 'member' THEN '普通成员'
      WHEN 'super_admin' THEN '管理员'
      WHEN 'operations' THEN '运营'
      WHEN 'auditor' THEN '审核'
      WHEN 'designer' THEN '设计'
      WHEN 'customization_operator' THEN '定制设计'
      WHEN 'department_admin' THEN '部门管理员'
      ELSE name
    END,
    description = CASE code
      WHEN 'member' THEN '系统默认基础角色'
      WHEN 'super_admin' THEN '全局管理角色（系统保护）'
      WHEN 'operations' THEN '任务创建、指派与运营处理'
      WHEN 'auditor' THEN '任务审核与交班'
      WHEN 'designer' THEN '设计处理与源文件上传'
      WHEN 'customization_operator' THEN '定制设计内部处理'
      WHEN 'department_admin' THEN '管理本人稳定部门范围内的任务与资源'
      ELSE description
    END,
    version = version + 1
WHERE code IN (
  'member',
  'super_admin',
  'operations',
  'auditor',
  'designer',
  'customization_operator',
  'department_admin'
);

UPDATE auth_policy_state
SET policy_revision = policy_revision + 1
WHERE singleton_id = 1;
-- ROLLBACK-END
