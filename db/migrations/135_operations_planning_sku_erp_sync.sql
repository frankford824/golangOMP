-- Migration: 135_operations_planning_sku_erp_sync.sql
-- Planning-SKU creation is a single business operation: generating the SKU
-- must enqueue the matching ERP filing. Operations users already own
-- planning_sku.create, so grant the narrower create-time ERP sync capability
-- without granting ERP administration or retry management.

INSERT IGNORE INTO auth_role_permissions (role_id, permission_code, task_types)
SELECT id, 'planning_sku.erp_sync', NULL
FROM auth_roles
WHERE code = 'operations'
  AND archived_at IS NULL;

UPDATE auth_roles
SET version = version + 1
WHERE code = 'operations'
  AND archived_at IS NULL;

UPDATE auth_policy_state
SET policy_revision = policy_revision + 1
WHERE singleton_id = 1;

-- ROLLBACK-BEGIN
DELETE role_permission
FROM auth_role_permissions role_permission
JOIN auth_roles role ON role.id = role_permission.role_id
WHERE role.code = 'operations'
  AND role_permission.permission_code = 'planning_sku.erp_sync';

UPDATE auth_roles
SET version = version + 1
WHERE code = 'operations'
  AND archived_at IS NULL;

UPDATE auth_policy_state
SET policy_revision = policy_revision + 1
WHERE singleton_id = 1;
-- ROLLBACK-END
