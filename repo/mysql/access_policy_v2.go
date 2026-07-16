package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type AccessPolicyRepo struct{ db *DB }

func NewAccessPolicyRepo(db *DB) *AccessPolicyRepo { return &AccessPolicyRepo{db: db} }

func (r *AccessPolicyRepo) ListPermissions(ctx context.Context) ([]domain.AccessPermission, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT code, module, name, description, risk_level, enabled
		FROM auth_permissions
		ORDER BY module, code`)
	if err != nil {
		return nil, fmt.Errorf("list auth permissions: %w", err)
	}
	defer rows.Close()
	out := make([]domain.AccessPermission, 0)
	for rows.Next() {
		var item domain.AccessPermission
		if err := rows.Scan(&item.Code, &item.Module, &item.Name, &item.Description, &item.RiskLevel, &item.Enabled); err != nil {
			return nil, fmt.Errorf("scan auth permission: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *AccessPolicyRepo) ListRoles(ctx context.Context, includeArchived bool) ([]domain.AccessRole, error) {
	where := "WHERE r.archived_at IS NULL"
	if includeArchived {
		where = ""
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT r.id, r.code, r.name, r.description, r.system_protected, r.archived_at, r.version,
		       COALESCE(GROUP_CONCAT(rp.permission_code ORDER BY rp.permission_code SEPARATOR ','), '')
		FROM auth_roles r
		LEFT JOIN auth_role_permissions rp ON rp.role_id = r.id
		`+where+`
		GROUP BY r.id, r.code, r.name, r.description, r.system_protected, r.archived_at, r.version
		ORDER BY r.system_protected DESC, r.name, r.id`)
	if err != nil {
		return nil, fmt.Errorf("list auth roles: %w", err)
	}
	defer rows.Close()
	out := make([]domain.AccessRole, 0)
	for rows.Next() {
		item, err := scanAccessRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *AccessPolicyRepo) GetRole(ctx context.Context, id int64) (*domain.AccessRole, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT r.id, r.code, r.name, r.description, r.system_protected, r.archived_at, r.version,
		       COALESCE(GROUP_CONCAT(rp.permission_code ORDER BY rp.permission_code SEPARATOR ','), '')
		FROM auth_roles r
		LEFT JOIN auth_role_permissions rp ON rp.role_id = r.id
		WHERE r.id = ?
		GROUP BY r.id, r.code, r.name, r.description, r.system_protected, r.archived_at, r.version`, id)
	item, err := scanAccessRole(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	return item, err
}

type accessRoleScanner interface{ Scan(...interface{}) error }

func scanAccessRole(row accessRoleScanner) (*domain.AccessRole, error) {
	var item domain.AccessRole
	var archived sql.NullTime
	var permissions string
	if err := row.Scan(&item.ID, &item.Code, &item.Name, &item.Description, &item.SystemProtected, &archived, &item.Version, &permissions); err != nil {
		return nil, err
	}
	if archived.Valid {
		item.ArchivedAt = &archived.Time
	}
	item.Permissions = splitPermissionCodes(permissions)
	return &item, nil
}

func splitPermissionCodes(raw string) []domain.PermissionCode {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []domain.PermissionCode{}
	}
	parts := strings.Split(raw, ",")
	out := make([]domain.PermissionCode, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, domain.PermissionCode(value))
		}
	}
	return out
}

func (r *AccessPolicyRepo) GetPolicyRevision(ctx context.Context) (int64, error) {
	var revision int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT policy_revision FROM auth_policy_state WHERE singleton_id = 1`).Scan(&revision); err != nil {
		return 0, fmt.Errorf("read access policy revision: %w", err)
	}
	return revision, nil
}

func (r *AccessPolicyRepo) LockPolicyRevision(ctx context.Context, tx repo.Tx) (int64, error) {
	var revision int64
	if err := Unwrap(tx).QueryRowContext(ctx, `SELECT policy_revision FROM auth_policy_state WHERE singleton_id = 1 FOR UPDATE`).Scan(&revision); err != nil {
		return 0, fmt.Errorf("lock access policy revision: %w", err)
	}
	return revision, nil
}

func (r *AccessPolicyRepo) BumpPolicyRevision(ctx context.Context, tx repo.Tx, expected int64, actorID int64, action, targetType, targetID, reason string, before, after interface{}) (int64, error) {
	current, err := r.LockPolicyRevision(ctx, tx)
	if err != nil {
		return 0, err
	}
	if current != expected {
		return 0, repo.ErrConflict
	}
	next := current + 1
	if _, err := Unwrap(tx).ExecContext(ctx, `UPDATE auth_policy_state SET policy_revision = ? WHERE singleton_id = 1`, next); err != nil {
		return 0, fmt.Errorf("update access policy revision: %w", err)
	}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	if _, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO auth_policy_events
		  (policy_revision, actor_id, action, target_type, target_id, reason, before_json, after_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		next, actorID, action, targetType, targetID, reason, nullJSON(beforeJSON), nullJSON(afterJSON)); err != nil {
		return 0, fmt.Errorf("insert access policy event: %w", err)
	}
	return next, nil
}

func nullJSON(raw []byte) interface{} {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}

func (r *AccessPolicyRepo) CreateRole(ctx context.Context, tx repo.Tx, role *domain.AccessRole, actorID int64) (int64, error) {
	result, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO auth_roles (code, name, description, system_protected, version, created_by, updated_by)
		VALUES (?, ?, ?, 0, 0, ?, ?)`, strings.TrimSpace(role.Code), strings.TrimSpace(role.Name), strings.TrimSpace(role.Description), actorID, actorID)
	if err != nil {
		return 0, fmt.Errorf("insert auth role: %w", err)
	}
	return result.LastInsertId()
}

func (r *AccessPolicyRepo) UpdateRole(ctx context.Context, tx repo.Tx, role *domain.AccessRole, expectedVersion, actorID int64) (bool, error) {
	result, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE auth_roles
		SET name = ?, description = ?, version = version + 1, updated_by = ?
		WHERE id = ? AND version = ? AND archived_at IS NULL`,
		strings.TrimSpace(role.Name), strings.TrimSpace(role.Description), actorID, role.ID, expectedVersion)
	if err != nil {
		return false, fmt.Errorf("update auth role: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (r *AccessPolicyRepo) ArchiveRole(ctx context.Context, tx repo.Tx, roleID, expectedVersion, actorID int64) (bool, error) {
	result, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE auth_roles
		SET archived_at = CURRENT_TIMESTAMP, version = version + 1, updated_by = ?
		WHERE id = ? AND version = ? AND system_protected = 0 AND archived_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM auth_user_role_assignments a WHERE a.role_id = auth_roles.id)`, actorID, roleID, expectedVersion)
	if err != nil {
		return false, fmt.Errorf("archive auth role: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (r *AccessPolicyRepo) ReplaceRolePermissions(ctx context.Context, tx repo.Tx, roleID int64, permissions []domain.PermissionCode) error {
	if _, err := Unwrap(tx).ExecContext(ctx, `DELETE FROM auth_role_permissions WHERE role_id = ?`, roleID); err != nil {
		return fmt.Errorf("delete role permissions: %w", err)
	}
	for _, permission := range permissions {
		if _, err := Unwrap(tx).ExecContext(ctx, `
			INSERT INTO auth_role_permissions (role_id, permission_code)
			SELECT ?, code FROM auth_permissions WHERE code = ? AND enabled = 1`, roleID, permission); err != nil {
			return fmt.Errorf("insert role permission %s: %w", permission, err)
		}
	}
	return nil
}

func (r *AccessPolicyRepo) ReplaceUserAssignments(ctx context.Context, tx repo.Tx, userID, actorID int64, assignments []domain.AccessAssignment) error {
	if _, err := Unwrap(tx).ExecContext(ctx, `
		DELETE s FROM auth_assignment_scope_subjects s
		JOIN auth_user_role_assignments a ON a.id = s.assignment_id
		WHERE a.user_id = ? AND a.source_type IN ('direct','migration')`, userID); err != nil {
		return fmt.Errorf("delete assignment scopes: %w", err)
	}
	if _, err := Unwrap(tx).ExecContext(ctx, `DELETE FROM auth_user_role_assignments WHERE user_id = ? AND source_type IN ('direct','migration')`, userID); err != nil {
		return fmt.Errorf("delete user assignments: %w", err)
	}
	for _, assignment := range assignments {
		result, err := Unwrap(tx).ExecContext(ctx, `
			INSERT INTO auth_user_role_assignments
			  (user_id, role_id, scope_mode, source_type, version, assigned_by)
			SELECT ?, r.id, ?, 'direct', 0, ?
			FROM auth_roles r WHERE r.id = ? AND r.archived_at IS NULL`, userID, assignment.ScopeMode, actorID, assignment.RoleID)
		if err != nil {
			return fmt.Errorf("insert user assignment: %w", err)
		}
		assignmentID, err := result.LastInsertId()
		if err != nil || assignmentID == 0 {
			return fmt.Errorf("assigned role does not exist or is archived")
		}
		for _, subject := range assignment.Subjects {
			if _, err := Unwrap(tx).ExecContext(ctx, `
				INSERT INTO auth_assignment_scope_subjects (assignment_id, subject_type, subject_id)
				VALUES (?, ?, ?)`, assignmentID, subject.SubjectType, subject.SubjectID); err != nil {
				return fmt.Errorf("insert assignment scope subject: %w", err)
			}
		}
	}
	return nil
}

func (r *AccessPolicyRepo) EffectiveAccess(ctx context.Context, userID int64) (*domain.EffectiveAccess, error) {
	revision, err := r.GetPolicyRevision(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT a.id, a.user_id, r.id, r.code, r.name, a.scope_mode, a.source_type, a.source_ref_id, a.version,
		       NULL AS policy_subject_type, NULL AS policy_subject_id
		FROM auth_user_role_assignments a
		JOIN auth_roles r ON r.id = a.role_id AND r.archived_at IS NULL
		WHERE a.user_id = ?
		UNION ALL
		SELECT 0, u.id, r.id, r.code, r.name, p.scope_mode, 'org_policy', p.id, p.version,
		       p.subject_type, p.subject_id
		FROM users u
		JOIN auth_org_role_policies p ON p.enabled = 1 AND (
		  (p.subject_type = 'department' AND p.subject_id = u.department_id)
		  OR (p.subject_type = 'team' AND p.subject_id = u.team_id)
		)
		JOIN auth_roles r ON r.id = p.role_id AND r.archived_at IS NULL
		WHERE u.id = ?`, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("list effective access assignments: %w", err)
	}
	defer rows.Close()
	assignments := make([]domain.AccessAssignment, 0)
	roleIDs := make([]int64, 0)
	seenRole := map[int64]struct{}{}
	for rows.Next() {
		var item domain.AccessAssignment
		var sourceRef, policySubjectID sql.NullInt64
		var policySubjectType sql.NullString
		if err := rows.Scan(&item.ID, &item.UserID, &item.RoleID, &item.RoleCode, &item.RoleName, &item.ScopeMode, &item.SourceType, &sourceRef, &item.Version, &policySubjectType, &policySubjectID); err != nil {
			return nil, fmt.Errorf("scan effective access assignment: %w", err)
		}
		if sourceRef.Valid {
			item.SourceRef = &sourceRef.Int64
		}
		item.Subjects = []domain.AccessScopeSubject{}
		if item.ID > 0 && item.ScopeMode == domain.AccessScopeSelectedOrg {
			subjects, err := r.listAssignmentSubjects(ctx, item.ID)
			if err != nil {
				return nil, err
			}
			item.Subjects = subjects
		}
		if item.SourceType == "org_policy" && item.ScopeMode == domain.AccessScopeSelectedOrg && policySubjectType.Valid && policySubjectID.Valid {
			// An organization policy's selected_org scope is intentionally the
			// organization node on which that policy is defined. Multi-node scopes
			// belong to direct user assignments.
			item.Subjects = []domain.AccessScopeSubject{{
				SubjectType: domain.AccessSubjectType(policySubjectType.String),
				SubjectID:   policySubjectID.Int64,
			}}
		}
		assignments = append(assignments, item)
		if _, ok := seenRole[item.RoleID]; !ok {
			seenRole[item.RoleID] = struct{}{}
			roleIDs = append(roleIDs, item.RoleID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	permissions, notes, err := r.permissionsForRoles(ctx, roleIDs, assignments)
	if err != nil {
		return nil, err
	}
	return &domain.EffectiveAccess{UserID: userID, PolicyRevision: revision, Permissions: permissions, Assignments: assignments, Sources: notes}, nil
}

func (r *AccessPolicyRepo) listAssignmentSubjects(ctx context.Context, assignmentID int64) ([]domain.AccessScopeSubject, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT s.subject_type, s.subject_id,
		       CASE WHEN s.subject_type = 'department' THEN d.name ELSE t.name END
		FROM auth_assignment_scope_subjects s
		LEFT JOIN org_departments d ON s.subject_type = 'department' AND d.id = s.subject_id
		LEFT JOIN org_teams t ON s.subject_type = 'team' AND t.id = s.subject_id
		WHERE s.assignment_id = ?
		ORDER BY s.subject_type, s.subject_id`, assignmentID)
	if err != nil {
		return nil, fmt.Errorf("list assignment subjects: %w", err)
	}
	defer rows.Close()
	out := make([]domain.AccessScopeSubject, 0)
	for rows.Next() {
		var item domain.AccessScopeSubject
		if err := rows.Scan(&item.SubjectType, &item.SubjectID, &item.SubjectName); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *AccessPolicyRepo) permissionsForRoles(ctx context.Context, roleIDs []int64, assignments []domain.AccessAssignment) ([]domain.PermissionCode, []domain.EffectiveAccessNote, error) {
	if len(roleIDs) == 0 {
		return []domain.PermissionCode{}, []domain.EffectiveAccessNote{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(roleIDs)), ",")
	args := make([]interface{}, 0, len(roleIDs))
	for _, id := range roleIDs {
		args = append(args, id)
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT rp.role_id, rp.permission_code
		FROM auth_role_permissions rp
		JOIN auth_permissions p ON p.code = rp.permission_code AND p.enabled = 1
		WHERE rp.role_id IN (`+placeholders+`)
		ORDER BY rp.permission_code`, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list effective permissions: %w", err)
	}
	defer rows.Close()
	permissions := make([]domain.PermissionCode, 0)
	notes := make([]domain.EffectiveAccessNote, 0)
	seen := map[domain.PermissionCode]struct{}{}
	assignmentsByRole := map[int64][]domain.AccessAssignment{}
	for _, assignment := range assignments {
		assignmentsByRole[assignment.RoleID] = append(assignmentsByRole[assignment.RoleID], assignment)
	}
	for rows.Next() {
		var roleID int64
		var permission domain.PermissionCode
		if err := rows.Scan(&roleID, &permission); err != nil {
			return nil, nil, err
		}
		for _, assignment := range assignmentsByRole[roleID] {
			notes = append(notes, domain.EffectiveAccessNote{Permission: permission, RoleID: roleID, RoleCode: assignment.RoleCode, SourceType: assignment.SourceType, ScopeMode: assignment.ScopeMode})
		}
		if _, ok := seen[permission]; !ok {
			seen[permission] = struct{}{}
			permissions = append(permissions, permission)
		}
	}
	sort.Slice(permissions, func(i, j int) bool { return permissions[i] < permissions[j] })
	return permissions, notes, rows.Err()
}

func (r *AccessPolicyRepo) ListPolicyEvents(ctx context.Context, limit int) ([]domain.AccessPolicyEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, policy_revision, actor_id, action, target_type, target_id, reason, before_json, after_json, created_at
		FROM auth_policy_events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list auth policy events: %w", err)
	}
	defer rows.Close()
	out := make([]domain.AccessPolicyEvent, 0)
	for rows.Next() {
		var item domain.AccessPolicyEvent
		var before, after []byte
		if err := rows.Scan(&item.ID, &item.PolicyRevision, &item.ActorID, &item.Action, &item.TargetType, &item.TargetID, &item.Reason, &before, &after, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(before) > 0 {
			_ = json.Unmarshal(before, &item.Before)
		}
		if len(after) > 0 {
			_ = json.Unmarshal(after, &item.After)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *AccessPolicyRepo) CountUsersWithRoleCode(ctx context.Context, roleCode string) (int64, error) {
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT a.user_id)
		FROM auth_user_role_assignments a
		JOIN auth_roles r ON r.id = a.role_id
		WHERE r.code = ? AND r.archived_at IS NULL`, strings.TrimSpace(roleCode)).Scan(&total); err != nil {
		return 0, fmt.Errorf("count users with auth role: %w", err)
	}
	return total, nil
}

func (r *AccessPolicyRepo) GetOrgPolicy(ctx context.Context, subjectType domain.AccessSubjectType, subjectID int64) ([]domain.AccessOrgPolicy, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, subject_type, subject_id, role_id, scope_mode, enabled, version, reason
		FROM auth_org_role_policies
		WHERE subject_type = ? AND subject_id = ?
		ORDER BY id`, subjectType, subjectID)
	if err != nil {
		return nil, fmt.Errorf("list org access policies: %w", err)
	}
	defer rows.Close()
	out := make([]domain.AccessOrgPolicy, 0)
	for rows.Next() {
		var item domain.AccessOrgPolicy
		if err := rows.Scan(&item.ID, &item.SubjectType, &item.SubjectID, &item.RoleID, &item.ScopeMode, &item.Enabled, &item.Version, &item.Reason); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *AccessPolicyRepo) ReplaceOrgPolicies(ctx context.Context, tx repo.Tx, subjectType domain.AccessSubjectType, subjectID, actorID int64, policies []domain.AccessOrgPolicy) error {
	if _, err := Unwrap(tx).ExecContext(ctx, `DELETE FROM auth_org_role_policies WHERE subject_type = ? AND subject_id = ?`, subjectType, subjectID); err != nil {
		return fmt.Errorf("delete org access policies: %w", err)
	}
	for _, policy := range policies {
		if _, err := Unwrap(tx).ExecContext(ctx, `
			INSERT INTO auth_org_role_policies
			  (subject_type, subject_id, role_id, scope_mode, enabled, version, reason, created_by, updated_by)
			SELECT ?, ?, r.id, ?, ?, 0, ?, ?, ?
			FROM auth_roles r WHERE r.id = ? AND r.archived_at IS NULL`,
			subjectType, subjectID, policy.ScopeMode, policy.Enabled, strings.TrimSpace(policy.Reason), actorID, actorID, policy.RoleID); err != nil {
			return fmt.Errorf("insert org access policy: %w", err)
		}
	}
	return nil
}
