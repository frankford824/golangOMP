package mysqlrepo

import (
	"context"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

// Both applications resolve authorization from auth_*; user_roles is not an
// authorization source for the workbench. The outer user alias is code-owned.
func workbenchGrantedRoleSQL(userAlias string) string {
	return `SELECT a.role_id FROM auth_user_role_assignments a WHERE a.user_id = ` + userAlias + `.id
		UNION SELECT p.role_id FROM auth_org_role_policies p WHERE p.enabled = 1 AND (
		(p.subject_type = 'department' AND p.subject_id = ` + userAlias + `.department_id) OR
		(p.subject_type = 'team' AND p.subject_id = ` + userAlias + `.team_id))`
}

func workbenchPermissionSQL(userAlias string, permission domain.PermissionCode) string {
	return `EXISTS (SELECT 1 FROM auth_roles ar
		JOIN auth_role_permissions arp ON arp.role_id = ar.id
		JOIN auth_permissions ap ON ap.code = arp.permission_code AND ap.enabled = 1
		WHERE ar.archived_at IS NULL AND arp.permission_code = '` + string(permission) + `'
		AND ar.id IN (` + workbenchGrantedRoleSQL(userAlias) + `))`
}

func workbenchRoleCaseSQL() string {
	return `CASE ar.code WHEN 'asset_submitter' THEN 'AssetSubmitter'
		WHEN 'asset_manager' THEN 'AssetManager' WHEN 'asset_template_admin' THEN 'AssetTemplateAdmin'
		WHEN 'asset_settlement' THEN 'AssetSettlement' WHEN 'asset_profile_admin' THEN 'HRAdmin'
		WHEN 'super_admin' THEN 'SuperAdmin' END`
}

// Existing grants without a materialized membership are immediately visible.
// An explicit disabled/merged state always wins over an authorization grant.
func workbenchMembershipStatusSQL() string {
	return `COALESCE(am.status, CASE WHEN u.status = 'active' AND ` + workbenchPermissionSQL("u", domain.PermissionAssetWorkbenchUse) + ` THEN 'active' ELSE '' END)`
}

// Eagerly materialize membership in the SAME transaction as a policy mutation.
// Never restore disabled/merged memberships, or reopen a disabled user. Pending
// applications are approved by an explicit authorization grant. No PII is copied.
func syncWorkbenchMemberships(ctx context.Context, tx repo.Tx, userID, actorID int64) error {
	_, err := Unwrap(tx).ExecContext(ctx, `INSERT INTO app_memberships
		(app_code, user_id, status, identity_type, source, opened_by)
		SELECT 'asset_workbench', u.id, 'active', 'staff', 'main_ops_opened', NULLIF(?, 0)
		FROM users u WHERE u.status = 'active' AND (? = 0 OR u.id = ?)
		AND `+workbenchPermissionSQL("u", domain.PermissionAssetWorkbenchUse)+`
		ON DUPLICATE KEY UPDATE
		opened_by = IF(app_memberships.status = 'pending', VALUES(opened_by), app_memberships.opened_by),
		updated_at = IF(app_memberships.status = 'pending', CURRENT_TIMESTAMP, app_memberships.updated_at),
		status = IF(app_memberships.status = 'pending', 'active', app_memberships.status)`, actorID, userID, userID)
	if err != nil {
		return fmt.Errorf("synchronize workbench membership: %w", err)
	}
	_, err = Unwrap(tx).ExecContext(ctx, `INSERT INTO asset_workbench_profiles (user_id, real_name, created_by, updated_by)
		SELECT u.id, COALESCE(NULLIF(u.display_name, ''), u.username, ''), NULLIF(?,0), NULLIF(?,0)
		FROM users u JOIN app_memberships am ON am.user_id = u.id AND am.app_code = 'asset_workbench' AND am.status = 'active'
		WHERE u.status = 'active' AND (? = 0 OR u.id = ?) AND `+workbenchPermissionSQL("u", domain.PermissionAssetWorkbenchUse)+`
		ON DUPLICATE KEY UPDATE user_id = VALUES(user_id)`, actorID, actorID, userID, userID)
	if err != nil {
		return fmt.Errorf("ensure pending workbench profile: %w", err)
	}
	return nil
}

// ReconcileWorkbenchAccess repairs historical missing rows using current grants.
// The caller must explicitly authorize and audit the transaction. It never
// copies PII, changes grants, grades, prices or restores disabled/merged states.
func (r *AccessPolicyRepo) ReconcileWorkbenchAccess(ctx context.Context, tx repo.Tx, actorID int64) error {
	if _, err := r.LockPolicyRevision(ctx, tx); err != nil {
		return err
	}
	return syncWorkbenchMemberships(ctx, tx, 0, actorID)
}

func (r *userRepo) ListWorkbenchRoles(ctx context.Context, userID int64) ([]domain.Role, error) {
	rows, err := r.db.db.QueryContext(ctx, `SELECT DISTINCT `+workbenchRoleCaseSQL()+`
		FROM users u JOIN auth_roles ar ON ar.archived_at IS NULL AND ar.id IN (`+workbenchGrantedRoleSQL("u")+`)
		WHERE u.id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []domain.Role
	for rows.Next() {
		var value *string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if value != nil {
			roles = append(roles, domain.Role(*value))
		}
	}
	return domain.NormalizeRoleValues(roles), rows.Err()
}

// Only the four workbench-owned DIRECT assignments are edited here. Other
// applications' grants, custom roles, inherited policies, and selected-org
// scope subjects are preserved. The global revision serializes both writers.
func (r *userRepo) ReplaceWorkbenchRoles(ctx context.Context, tx repo.Tx, userID, actorID int64, roles []domain.Role, expected *int64, reason string) error {
	policy := NewAccessPolicyRepo(r.db)
	revision, err := policy.LockPolicyRevision(ctx, tx)
	if err != nil {
		return err
	}
	if expected != nil && *expected != revision {
		return repo.ErrConflict
	}
	selected := map[string]bool{}
	for _, role := range roles {
		switch role {
		case domain.RoleAssetSubmitter:
			selected["asset_submitter"] = true
		case domain.RoleAssetManager:
			selected["asset_manager"] = true
		case domain.RoleAssetTemplateAdmin:
			selected["asset_template_admin"] = true
		case domain.RoleAssetSettlement:
			selected["asset_settlement"] = true
		}
	}
	if expected != nil {
		// Inherited roles must be edited at their policy source, not silently
		// copied to direct assignments or seemingly unchecked in this screen.
		var inherited int
		if err := Unwrap(tx).QueryRowContext(ctx, `SELECT COUNT(*) FROM users u
			JOIN auth_org_role_policies p ON p.enabled = 1 AND ((p.subject_type = 'department' AND p.subject_id = u.department_id) OR (p.subject_type = 'team' AND p.subject_id = u.team_id))
			JOIN auth_roles ar ON ar.id = p.role_id AND ar.archived_at IS NULL
			WHERE u.id = ? AND ar.code IN ('asset_submitter','asset_manager','asset_template_admin','asset_settlement')`, userID).Scan(&inherited); err != nil {
			return err
		}
		if inherited > 0 {
			return domain.NewAppError(domain.ErrCodeConflict, "该成员有组织继承角色，请在运营系统的组织权限中调整。", nil)
		}
	}
	for _, code := range []string{"asset_submitter", "asset_manager", "asset_template_admin", "asset_settlement"} {
		if selected[code] {
			var available int
			if err := Unwrap(tx).QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_roles WHERE code = ? AND archived_at IS NULL`, code).Scan(&available); err != nil {
				return err
			}
			if available != 1 {
				return domain.NewAppError(domain.ErrCodeConflict, "工作台角色已归档，请先在运营系统检查角色配置。", nil)
			}
			scope := domain.AccessScopeSelf
			if code == "asset_manager" {
				scope = domain.AccessScopeGlobal
			}
			_, err := Unwrap(tx).ExecContext(ctx, `INSERT INTO auth_user_role_assignments
				(user_id, role_id, scope_mode, source_type, source_ref_id, version, assigned_by)
				SELECT u.id, ar.id, ?, 'direct', 0, 0, ? FROM users u JOIN auth_roles ar ON ar.code = ? AND ar.archived_at IS NULL
				WHERE u.id = ? AND NOT EXISTS (SELECT 1 FROM auth_user_role_assignments a WHERE a.user_id = u.id AND a.role_id = ar.id)
				AND NOT EXISTS (SELECT 1 FROM auth_org_role_policies p WHERE p.enabled = 1 AND p.role_id = ar.id AND
				((p.subject_type = 'department' AND p.subject_id = u.department_id) OR (p.subject_type = 'team' AND p.subject_id = u.team_id)))`, scope, actorID, code, userID)
			if err != nil {
				return fmt.Errorf("grant workbench role %s: %w", code, err)
			}
		} else {
			if _, err := Unwrap(tx).ExecContext(ctx, `DELETE s FROM auth_assignment_scope_subjects s
				JOIN auth_user_role_assignments a ON a.id = s.assignment_id JOIN auth_roles ar ON ar.id = a.role_id
				WHERE a.user_id = ? AND a.source_type IN ('direct','migration') AND ar.code = ?`, userID, code); err != nil {
				return err
			}
			if _, err := Unwrap(tx).ExecContext(ctx, `DELETE a FROM auth_user_role_assignments a JOIN auth_roles ar ON ar.id = a.role_id
				WHERE a.user_id = ? AND a.source_type IN ('direct','migration') AND ar.code = ?`, userID, code); err != nil {
				return err
			}
		}
	}
	if err := syncWorkbenchMemberships(ctx, tx, userID, actorID); err != nil {
		return err
	}
	_, err = policy.BumpPolicyRevision(ctx, tx, revision, actorID, "user.workbench_roles_replaced", "user", fmt.Sprint(userID), strings.TrimSpace(reason), nil, roles)
	return err
}
