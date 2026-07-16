-- Migration: 124_explicit_access_platform.sql
-- Explicit capability, role and organization-scope authorization.
-- Organization names remain display-only; grants reference stable department/team ids.

ALTER TABLE users
  ADD COLUMN department_id BIGINT NULL AFTER department,
  ADD COLUMN team_id BIGINT NULL AFTER department_id,
  ADD KEY idx_users_department_id (department_id),
  ADD KEY idx_users_team_id (team_id),
  ADD CONSTRAINT fk_users_department_id FOREIGN KEY (department_id) REFERENCES org_departments(id),
  ADD CONSTRAINT fk_users_team_id FOREIGN KEY (team_id) REFERENCES org_teams(id);

UPDATE users u
LEFT JOIN (
  SELECT name, MIN(id) AS id
  FROM org_departments
  GROUP BY name
  HAVING COUNT(*) = 1
) d ON d.name = u.department
LEFT JOIN (
  SELECT department_id, name, MIN(id) AS id
  FROM org_teams
  GROUP BY department_id, name
  HAVING COUNT(*) = 1
) t ON t.name = u.team AND t.department_id = COALESCE(u.department_id, d.id)
SET u.department_id = COALESCE(u.department_id, d.id),
    u.team_id = COALESCE(u.team_id, t.id)
WHERE u.department_id IS NULL OR u.team_id IS NULL;

ALTER TABLE tasks
  ADD COLUMN owner_department_id BIGINT NULL AFTER owner_department,
  ADD COLUMN owner_team_id BIGINT NULL AFTER owner_org_team,
  ADD KEY idx_tasks_owner_department_id (owner_department_id),
  ADD KEY idx_tasks_owner_team_id (owner_team_id),
  ADD CONSTRAINT fk_tasks_owner_department_id FOREIGN KEY (owner_department_id) REFERENCES org_departments(id),
  ADD CONSTRAINT fk_tasks_owner_team_id FOREIGN KEY (owner_team_id) REFERENCES org_teams(id);

UPDATE tasks t
LEFT JOIN (
  SELECT name, MIN(id) AS id
  FROM org_departments
  GROUP BY name
  HAVING COUNT(*) = 1
) d ON d.name = t.owner_department
LEFT JOIN (
  SELECT department_id, name, MIN(id) AS id
  FROM org_teams
  GROUP BY department_id, name
  HAVING COUNT(*) = 1
) ot ON ot.name = NULLIF(TRIM(t.owner_org_team), '') AND ot.department_id = COALESCE(t.owner_department_id, d.id)
LEFT JOIN (
  SELECT department_id, name, MIN(id) AS id
  FROM org_teams
  GROUP BY department_id, name
  HAVING COUNT(*) = 1
) lt ON lt.name = NULLIF(TRIM(t.owner_team), '') AND lt.department_id = COALESCE(t.owner_department_id, d.id)
SET t.owner_department_id = COALESCE(t.owner_department_id, d.id),
    t.owner_team_id = COALESCE(t.owner_team_id, CASE
      WHEN NULLIF(TRIM(t.owner_org_team), '') IS NOT NULL AND NULLIF(TRIM(t.owner_team), '') IS NOT NULL
        THEN CASE WHEN ot.id = lt.id THEN ot.id ELSE NULL END
      WHEN NULLIF(TRIM(t.owner_org_team), '') IS NOT NULL THEN ot.id
      WHEN NULLIF(TRIM(t.owner_team), '') IS NOT NULL THEN lt.id
      ELSE NULL
    END)
WHERE t.owner_department_id IS NULL OR t.owner_team_id IS NULL;

CREATE TABLE auth_permissions (
  code VARCHAR(96) NOT NULL,
  module VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  risk_level VARCHAR(16) NOT NULL DEFAULT 'normal',
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (code),
  KEY idx_auth_permissions_module_enabled (module, enabled),
  CONSTRAINT chk_auth_permissions_risk CHECK (risk_level IN ('normal','high'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Code-owned authorization capability catalog';

CREATE TABLE auth_roles (
  id BIGINT NOT NULL AUTO_INCREMENT,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  system_protected TINYINT(1) NOT NULL DEFAULT 0,
  archived_at DATETIME NULL,
  version BIGINT NOT NULL DEFAULT 0,
  created_by BIGINT NULL,
  updated_by BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_auth_roles_code (code),
  KEY idx_auth_roles_active (archived_at, name),
  CONSTRAINT fk_auth_roles_created_by FOREIGN KEY (created_by) REFERENCES users(id),
  CONSTRAINT fk_auth_roles_updated_by FOREIGN KEY (updated_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Administrator-managed business roles';

CREATE TABLE auth_role_permissions (
  role_id BIGINT NOT NULL,
  permission_code VARCHAR(96) NOT NULL,
  created_by BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (role_id, permission_code),
  KEY idx_auth_role_permissions_permission (permission_code, role_id),
  CONSTRAINT fk_auth_role_permissions_role FOREIGN KEY (role_id) REFERENCES auth_roles(id),
  CONSTRAINT fk_auth_role_permissions_permission FOREIGN KEY (permission_code) REFERENCES auth_permissions(code),
  CONSTRAINT fk_auth_role_permissions_created_by FOREIGN KEY (created_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Role capability allow-list';

CREATE TABLE auth_user_role_assignments (
  id BIGINT NOT NULL AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  scope_mode VARCHAR(32) NOT NULL,
  source_type VARCHAR(32) NOT NULL DEFAULT 'direct',
  source_ref_id BIGINT NOT NULL DEFAULT 0,
  version BIGINT NOT NULL DEFAULT 0,
  assigned_by BIGINT NULL,
  assigned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_auth_user_role_assignment (user_id, role_id, source_type, source_ref_id),
  KEY idx_auth_assignments_user (user_id, source_type),
  KEY idx_auth_assignments_role (role_id, user_id),
  CONSTRAINT fk_auth_assignments_user FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT fk_auth_assignments_role FOREIGN KEY (role_id) REFERENCES auth_roles(id),
  CONSTRAINT fk_auth_assignments_actor FOREIGN KEY (assigned_by) REFERENCES users(id),
  CONSTRAINT chk_auth_assignments_scope CHECK (scope_mode IN ('self','own_department','own_team','selected_org','global')),
  CONSTRAINT chk_auth_assignments_source CHECK (source_type IN ('direct','org_policy','migration'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Atomic user role and data-scope assignments';

CREATE TABLE auth_assignment_scope_subjects (
  assignment_id BIGINT NOT NULL,
  subject_type VARCHAR(16) NOT NULL,
  subject_id BIGINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (assignment_id, subject_type, subject_id),
  KEY idx_auth_scope_subject_lookup (subject_type, subject_id, assignment_id),
  CONSTRAINT fk_auth_scope_subject_assignment FOREIGN KEY (assignment_id) REFERENCES auth_user_role_assignments(id) ON DELETE CASCADE,
  CONSTRAINT chk_auth_scope_subject_type CHECK (subject_type IN ('department','team'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Selected department/team subjects for an assignment';

CREATE TABLE auth_org_role_policies (
  id BIGINT NOT NULL AUTO_INCREMENT,
  subject_type VARCHAR(16) NOT NULL,
  subject_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  scope_mode VARCHAR(32) NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 0,
  version BIGINT NOT NULL DEFAULT 0,
  reason VARCHAR(512) NOT NULL,
  created_by BIGINT NOT NULL,
  updated_by BIGINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_auth_org_role_policy (subject_type, subject_id, role_id),
  KEY idx_auth_org_role_policy_active (subject_type, subject_id, enabled),
  CONSTRAINT fk_auth_org_policy_role FOREIGN KEY (role_id) REFERENCES auth_roles(id),
  CONSTRAINT fk_auth_org_policy_created_by FOREIGN KEY (created_by) REFERENCES users(id),
  CONSTRAINT fk_auth_org_policy_updated_by FOREIGN KEY (updated_by) REFERENCES users(id),
  CONSTRAINT chk_auth_org_policy_subject CHECK (subject_type IN ('department','team')),
  CONSTRAINT chk_auth_org_policy_scope CHECK (scope_mode IN ('self','own_department','own_team','selected_org','global'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Explicit opt-in organization role policies';

CREATE TABLE auth_policy_events (
  id BIGINT NOT NULL AUTO_INCREMENT,
  policy_revision BIGINT NOT NULL,
  actor_id BIGINT NOT NULL,
  action VARCHAR(64) NOT NULL,
  target_type VARCHAR(32) NOT NULL,
  target_id VARCHAR(128) NOT NULL,
  reason VARCHAR(512) NOT NULL,
  before_json JSON NULL,
  after_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_auth_policy_event_revision (policy_revision),
  KEY idx_auth_policy_events_target (target_type, target_id, id),
  KEY idx_auth_policy_events_actor (actor_id, id),
  CONSTRAINT fk_auth_policy_event_actor FOREIGN KEY (actor_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Immutable access policy audit stream';

CREATE TABLE auth_policy_state (
  singleton_id TINYINT NOT NULL,
  policy_revision BIGINT NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (singleton_id),
  CONSTRAINT chk_auth_policy_singleton CHECK (singleton_id = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Global access policy revision';

INSERT INTO auth_policy_state (singleton_id, policy_revision) VALUES (1, 0);

INSERT INTO auth_permissions (code, module, name, description, risk_level) VALUES
  ('task.view','task','查看任务','查看授权范围内的任务','normal'),
  ('task.create','task','创建任务','创建业务任务','normal'),
  ('task.design.submit','task','提交设计','提交设计资源进入审核','normal'),
  ('task.audit.decision','task','审核决策','通过或打回设计','high'),
  ('task.reopen','task','重开任务','重开已结单任务','high'),
  ('task.manage','task','维护任务','分配、维护业务信息及执行任务节点动作','high'),
  ('planning_sku.view','planning_sku','查看策划 SKU','查看策划 SKU 任务','normal'),
  ('planning_sku.create','planning_sku','创建策划 SKU','单条或批量创建策划 SKU','normal'),
  ('planning_sku.edit','planning_sku','修正策划 SKU','结单后受控修正','high'),
  ('planning_sku.export','planning_sku','导出策划 SKU','导出策划 SKU 表格','normal'),
  ('planning_sku.erp_sync','planning_sku','同步策划 SKU 到 ERP','创建时请求 ERP 同步','high'),
  ('planning_sku.erp_retry','planning_sku','重试策划 SKU ERP','重试或重同步 ERP','high'),
  ('asset.view','asset','查看资产','查看授权范围内的资源组','normal'),
  ('asset.download','asset','下载资产','下载资源组文件','normal'),
  ('asset.export','asset','批量导出资产','批量打包资源组','normal'),
  ('asset.publish','asset','发布客户素材','发布固定最终修订','high'),
  ('asset.manage','asset','维护资产','上传、归档及维护资产业务数据','high'),
  ('asset_workbench.use','asset_workbench','使用素材工作台','查看个人素材、提交记录和通知','normal'),
  ('asset_workbench.submit','asset_workbench','提交素材','上传和提交外部素材','normal'),
  ('asset_workbench.members.manage','asset_workbench','管理工作台成员','维护工作台访问和分组成员','high'),
  ('asset_workbench.profiles.manage','asset_workbench','管理人员档案','维护工作台人员档案','high'),
  ('asset_workbench.groups.manage','asset_workbench','管理计价人员组','维护工作台计价人员组及成员','high'),
  ('asset_workbench.drive.manage','asset_workbench','管理上传目录','维护素材上传目录','high'),
  ('asset_workbench.batch.manage','asset_workbench','管理批处理任务','查看和维护素材批处理任务','high'),
  ('asset_workbench.templates.manage','asset_workbench','管理计价模板','维护难度、价格、扣款和福利规则','high'),
  ('asset_workbench.qc.manage','asset_workbench','管理素材质检','执行质检、作废和重新计价','high'),
  ('asset_workbench.settlement.manage','asset_workbench','管理素材结算','维护结算批次、调整和补录','high'),
  ('asset_workbench.audit.view','asset_workbench','查看工作台审计','查看素材工作台业务操作记录','normal'),
  ('catalog.view','catalog','查看产品资料','查看 ERP 产品、分类和业务规则','normal'),
  ('catalog.manage','catalog','维护产品资料','维护分类、成本规则和编号规则','high'),
  ('erp.manage','erp','管理 ERP 同步','管理 ERP 建档、重试和告警','high'),
  ('account.use','account','使用个人工作台','维护个人资料、通知和工作台偏好','normal'),
  ('report.view','report','查看经营报表','查看经营指标和分析结果','high'),
  ('system.manage','system','管理系统运行','维护系统配置、日志和集成','high'),
  ('access_policy.view','access','查看权限策略','查看角色、能力和有效权限','normal'),
  ('access_policy.manage','access','管理权限策略','维护角色、授权和组织策略','high');

INSERT INTO auth_roles (code, name, description, system_protected) VALUES
  ('member','成员','系统默认基础角色',1),
  ('super_admin','超级管理员','系统保护的全局管理角色',1),
  ('operations','运营','任务创建和运营处理',0),
  ('designer','设计师','设计处理和资源提交',0),
  ('customization_operator','定制设计','定制设计内部处理',0),
  ('auditor','审核员','统一任务审核',0),
  ('asset_submitter','素材提交员','客户素材查看与提交',0),
  ('asset_manager','素材管理员','素材管理、导出和发布',0),
  ('asset_profile_admin','素材人员管理员','维护素材工作台人员档案',0),
  ('asset_template_admin','素材规则管理员','维护素材工作台计价规则',0),
  ('asset_settlement','素材结算员','维护素材工作台质检与结算',0),
  ('department_admin','部门管理员','管理本人稳定部门范围内的任务与资源',0),
  ('team_lead','团队负责人','管理本人稳定团队范围内的任务与资源',0),
  ('design_director','设计负责人','管理本人稳定部门范围内的设计与审核',0),
  ('erp_operator','ERP 操作员','ERP 同步管理',0),
  ('access_admin','权限管理员','显式权限策略维护',0);

INSERT INTO auth_role_permissions (role_id, permission_code)
SELECT r.id, p.code
FROM auth_roles r
JOIN auth_permissions p ON
  r.code = 'super_admin'
  OR (r.code = 'member' AND p.code IN ('account.use','task.view','planning_sku.view','asset.view','catalog.view'))
  OR (r.code = 'operations' AND p.code IN ('account.use','task.view','task.create','task.manage','planning_sku.view','planning_sku.create','planning_sku.export','asset.view','asset.download','catalog.view'))
  OR (r.code IN ('designer','customization_operator') AND p.code IN ('account.use','task.view','task.design.submit','asset.view','asset.download','asset.manage','catalog.view'))
  OR (r.code = 'auditor' AND p.code IN ('account.use','task.view','task.audit.decision','asset.view','asset.download','catalog.view'))
  OR (r.code = 'asset_submitter' AND p.code IN ('account.use','asset.view','asset.download','asset_workbench.use','asset_workbench.submit'))
  OR (r.code = 'asset_manager' AND p.code IN ('account.use','asset.view','asset.download','asset.export','asset.publish','asset.manage','asset_workbench.use','asset_workbench.submit','asset_workbench.members.manage','asset_workbench.groups.manage','asset_workbench.drive.manage','asset_workbench.batch.manage','asset_workbench.audit.view'))
  OR (r.code = 'asset_profile_admin' AND p.code IN ('account.use','asset_workbench.use','asset_workbench.profiles.manage'))
  OR (r.code = 'asset_template_admin' AND p.code IN ('account.use','asset_workbench.use','asset_workbench.groups.manage','asset_workbench.templates.manage','asset_workbench.audit.view'))
  OR (r.code = 'asset_settlement' AND p.code IN ('account.use','asset_workbench.use','asset_workbench.qc.manage','asset_workbench.settlement.manage','asset_workbench.audit.view'))
  OR (r.code = 'department_admin' AND p.code IN ('account.use','task.view','task.create','task.manage','planning_sku.view','planning_sku.create','planning_sku.edit','planning_sku.export','asset.view','asset.download','asset.manage','catalog.view','report.view'))
  OR (r.code = 'team_lead' AND p.code IN ('account.use','task.view','task.manage','planning_sku.view','planning_sku.create','planning_sku.export','asset.view','asset.download','catalog.view'))
  OR (r.code = 'design_director' AND p.code IN ('account.use','task.view','task.manage','task.design.submit','task.audit.decision','task.reopen','asset.view','asset.download','asset.manage','catalog.view'))
  OR (r.code = 'erp_operator' AND p.code IN ('account.use','planning_sku.view','planning_sku.erp_sync','planning_sku.erp_retry','catalog.view','catalog.manage','erp.manage'))
  OR (r.code = 'access_admin' AND p.code IN ('access_policy.view','access_policy.manage'));

INSERT INTO auth_user_role_assignments (user_id, role_id, scope_mode, source_type)
SELECT DISTINCT ur.user_id, r.id,
  CASE
    WHEN r.code IN ('super_admin','access_admin') THEN 'global'
    WHEN r.code IN ('operations','auditor','asset_manager','erp_operator') THEN 'global'
    WHEN r.code IN ('department_admin','design_director') THEN 'own_department'
    WHEN r.code = 'team_lead' THEN 'own_team'
    ELSE 'self'
  END,
  'migration'
FROM user_roles ur
JOIN auth_roles r ON r.code = CASE ur.role
  WHEN 'Member' THEN 'member'
  WHEN 'SuperAdmin' THEN 'super_admin'
  WHEN 'Admin' THEN 'access_admin'
  WHEN 'RoleAdmin' THEN 'access_admin'
  WHEN 'Ops' THEN 'operations'
  WHEN 'Designer' THEN 'designer'
  WHEN 'CustomizationOperator' THEN 'customization_operator'
  WHEN 'Audit_A' THEN 'auditor'
  WHEN 'Audit_B' THEN 'auditor'
  WHEN 'DesignReviewer' THEN 'auditor'
  WHEN 'CustomizationReviewer' THEN 'auditor'
  WHEN 'AssetSubmitter' THEN 'asset_submitter'
  WHEN 'AssetManager' THEN 'asset_manager'
  WHEN 'AssetTemplateAdmin' THEN 'asset_template_admin'
  WHEN 'AssetSettlement' THEN 'asset_settlement'
  WHEN 'HRAdmin' THEN 'asset_profile_admin'
  WHEN 'DepartmentAdmin' THEN 'department_admin'
  WHEN 'TeamLead' THEN 'team_lead'
  WHEN 'DesignDirector' THEN 'design_director'
  WHEN 'ERP' THEN 'erp_operator'
  ELSE '__manual_review__'
END;

INSERT IGNORE INTO auth_user_role_assignments (user_id, role_id, scope_mode, source_type)
SELECT u.id, r.id, 'self', 'migration'
FROM users u
JOIN auth_roles r ON r.code = 'member'
;

-- ROLLBACK-BEGIN
DROP TABLE IF EXISTS auth_policy_events;
DROP TABLE IF EXISTS auth_org_role_policies;
DROP TABLE IF EXISTS auth_assignment_scope_subjects;
DROP TABLE IF EXISTS auth_user_role_assignments;
DROP TABLE IF EXISTS auth_role_permissions;
DROP TABLE IF EXISTS auth_roles;
DROP TABLE IF EXISTS auth_permissions;
DROP TABLE IF EXISTS auth_policy_state;
ALTER TABLE tasks DROP FOREIGN KEY fk_tasks_owner_team_id;
ALTER TABLE tasks DROP FOREIGN KEY fk_tasks_owner_department_id;
ALTER TABLE tasks DROP INDEX idx_tasks_owner_team_id;
ALTER TABLE tasks DROP INDEX idx_tasks_owner_department_id;
ALTER TABLE tasks DROP COLUMN owner_team_id;
ALTER TABLE tasks DROP COLUMN owner_department_id;
ALTER TABLE users DROP FOREIGN KEY fk_users_team_id;
ALTER TABLE users DROP FOREIGN KEY fk_users_department_id;
ALTER TABLE users DROP INDEX idx_users_team_id;
ALTER TABLE users DROP INDEX idx_users_department_id;
ALTER TABLE users DROP COLUMN team_id;
ALTER TABLE users DROP COLUMN department_id;
-- ROLLBACK-END
