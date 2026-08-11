-- Migration: 136_warehouse_global_asset_access.sql
-- Warehouse operators need a global, read/download-only asset scope. The
-- legacy Warehouse identity role was intentionally excluded from explicit
-- access migration 124, which left cloud-warehouse accounts restricted to
-- self/organization scope and hid valid SKU resources owned by operations.

INSERT INTO auth_roles (code, name, description, system_protected)
VALUES (
  'warehouse_asset_reader',
  '云仓全局资产读取',
  '系统维护的云仓资产查看与下载角色，不包含任务或权限管理能力',
  1
)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  description = VALUES(description),
  system_protected = 1,
  archived_at = NULL,
  version = version + 1;

SET @warehouse_asset_reader_role_id := (
  SELECT id FROM auth_roles WHERE code = 'warehouse_asset_reader' LIMIT 1
);

INSERT IGNORE INTO auth_role_permissions (role_id, permission_code, task_types)
SELECT @warehouse_asset_reader_role_id, permission.code, NULL
FROM auth_permissions permission
WHERE permission.enabled = 1
  AND permission.code IN ('asset.view', 'asset.download');

-- Repair every current legacy Warehouse identity without granting the broader
-- operations role or any task-management capability.
INSERT IGNORE INTO auth_user_role_assignments (
  user_id,
  role_id,
  scope_mode,
  source_type,
  source_ref_id,
  assigned_by
)
SELECT DISTINCT
  legacy_role.user_id,
  @warehouse_asset_reader_role_id,
  'global',
  'migration',
  136,
  NULL
FROM user_roles legacy_role
JOIN users user ON user.id = legacy_role.user_id AND user.status = 'active'
WHERE legacy_role.role = 'Warehouse';

-- Keep future cloud-warehouse accounts independent of legacy role hydration.
-- The policy follows the stable organization row, so moving a user out of the
-- department automatically removes this organization-derived grant.
INSERT INTO auth_org_role_policies (
  subject_type,
  subject_id,
  role_id,
  scope_mode,
  enabled,
  version,
  reason,
  created_by,
  updated_by
)
SELECT
  'department',
  department.id,
  @warehouse_asset_reader_role_id,
  'global',
  1,
  0,
  '云仓生产操作需要全局查看并下载 SKU 资产',
  migration_actor.id,
  migration_actor.id
FROM org_departments department
JOIN (
  SELECT COALESCE(
    MIN(CASE WHEN is_config_super_admin = 1 AND status = 'active' THEN id END),
    MIN(CASE WHEN status = 'active' THEN id END)
  ) AS id
  FROM users
) migration_actor ON migration_actor.id IS NOT NULL
WHERE department.name = '云仓部'
ON DUPLICATE KEY UPDATE
  scope_mode = 'global',
  enabled = 1,
  version = version + 1,
  reason = VALUES(reason),
  updated_by = VALUES(updated_by);

UPDATE auth_policy_state
SET policy_revision = policy_revision + 1
WHERE singleton_id = 1;

-- ROLLBACK-BEGIN
DELETE policy
FROM auth_org_role_policies policy
JOIN auth_roles role ON role.id = policy.role_id
WHERE role.code = 'warehouse_asset_reader';

DELETE assignment
FROM auth_user_role_assignments assignment
JOIN auth_roles role ON role.id = assignment.role_id
WHERE role.code = 'warehouse_asset_reader'
  AND assignment.source_type = 'migration'
  AND assignment.source_ref_id = 136;

DELETE permission
FROM auth_role_permissions permission
JOIN auth_roles role ON role.id = permission.role_id
WHERE role.code = 'warehouse_asset_reader';

DELETE FROM auth_roles WHERE code = 'warehouse_asset_reader';

UPDATE auth_policy_state
SET policy_revision = policy_revision + 1
WHERE singleton_id = 1;
-- ROLLBACK-END
