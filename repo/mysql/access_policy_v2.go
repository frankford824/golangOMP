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
		SELECT r.id, r.code, r.name, r.description, r.system_protected, r.archived_at, r.version
		FROM auth_roles r
		`+where+`
		ORDER BY r.system_protected DESC, r.name, r.id`)
	if err != nil {
		return nil, fmt.Errorf("list auth roles: %w", err)
	}
	defer rows.Close()
	out := make([]domain.AccessRole, 0)
	for rows.Next() {
		item, err := scanAccessRoleHeader(rows)
		if err != nil {
			return nil, err
		}
		permissions, err := r.listRolePermissions(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		item.Permissions = permissions
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *AccessPolicyRepo) GetRole(ctx context.Context, id int64) (*domain.AccessRole, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT r.id, r.code, r.name, r.description, r.system_protected, r.archived_at, r.version
		FROM auth_roles r
		WHERE r.id = ?`, id)
	item, err := scanAccessRoleHeader(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	permissions, err := r.listRolePermissions(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	item.Permissions = permissions
	return item, nil
}

type accessRoleScanner interface{ Scan(...interface{}) error }

func scanAccessRoleHeader(row accessRoleScanner) (*domain.AccessRole, error) {
	var item domain.AccessRole
	var archived sql.NullTime
	if err := row.Scan(&item.ID, &item.Code, &item.Name, &item.Description, &item.SystemProtected, &archived, &item.Version); err != nil {
		return nil, err
	}
	if archived.Valid {
		item.ArchivedAt = &archived.Time
	}
	item.Permissions = []domain.AccessRolePermission{}
	return &item, nil
}

func (r *AccessPolicyRepo) listRolePermissions(ctx context.Context, roleID int64) ([]domain.AccessRolePermission, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT permission_code, task_types
		FROM auth_role_permissions
		WHERE role_id = ?
		ORDER BY permission_code`, roleID)
	if err != nil {
		return nil, fmt.Errorf("list role permissions: %w", err)
	}
	defer rows.Close()
	out := make([]domain.AccessRolePermission, 0)
	for rows.Next() {
		var item domain.AccessRolePermission
		var taskTypes []byte
		if err := rows.Scan(&item.Code, &taskTypes); err != nil {
			return nil, err
		}
		item.TaskTypes, err = decodeTaskTypesJSON(taskTypes)
		if err != nil {
			return nil, fmt.Errorf("decode task types for role %d permission %s: %w", roleID, item.Code, err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func decodeTaskTypesJSON(raw []byte) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	normalized := normalizeTaskTypeStrings(out)
	for _, item := range normalized {
		if !domain.AccessTaskTypeValid(domain.TaskType(item)) {
			return nil, fmt.Errorf("invalid task type %q", item)
		}
	}
	return normalized, nil
}

func encodeTaskTypesJSON(items []string) interface{} {
	normalized := normalizeTaskTypeStrings(items)
	if len(normalized) == 0 {
		return nil
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil
	}
	return raw
}

func normalizeTaskTypeStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
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

func (r *AccessPolicyRepo) ReplaceRolePermissions(ctx context.Context, tx repo.Tx, roleID int64, permissions []domain.AccessRolePermission) error {
	if _, err := Unwrap(tx).ExecContext(ctx, `DELETE FROM auth_role_permissions WHERE role_id = ?`, roleID); err != nil {
		return fmt.Errorf("delete role permissions: %w", err)
	}
	for _, permission := range permissions {
		taskTypes := encodeTaskTypesJSON(permission.TaskTypes)
		if !domain.PermissionSupportsTaskTypes(permission.Code) {
			taskTypes = nil
		}
		if _, err := Unwrap(tx).ExecContext(ctx, `
			INSERT INTO auth_role_permissions (role_id, permission_code, task_types)
			SELECT ?, code, ? FROM auth_permissions WHERE code = ? AND enabled = 1`, roleID, taskTypes, permission.Code); err != nil {
			return fmt.Errorf("insert role permission %s: %w", permission.Code, err)
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

// EnsureExplicitRoleAssignment is used only at identity creation/config-sync
// boundaries. It deliberately writes an explicit assignment by stable role
// code in the caller's transaction; organization display names and legacy
// user_roles never participate in the decision.
func (r *AccessPolicyRepo) EnsureExplicitRoleAssignment(ctx context.Context, tx repo.Tx, userID int64, roleCode string, scopeMode domain.AccessScopeMode) error {
	if userID <= 0 || strings.TrimSpace(roleCode) == "" || !scopeMode.Valid() {
		return fmt.Errorf("invalid explicit identity assignment")
	}
	var roleID int64
	if err := Unwrap(tx).QueryRowContext(ctx, `
		SELECT id
		FROM auth_roles
		WHERE code = ? AND archived_at IS NULL
		FOR SHARE`, strings.TrimSpace(roleCode)).Scan(&roleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("active auth role %q does not exist", strings.TrimSpace(roleCode))
		}
		return fmt.Errorf("lock explicit identity role %q: %w", strings.TrimSpace(roleCode), err)
	}
	if _, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO auth_user_role_assignments
		  (user_id, role_id, scope_mode, source_type, source_ref_id, version, assigned_by)
		VALUES (?, ?, ?, 'direct', 0, 0, NULL)
		ON DUPLICATE KEY UPDATE scope_mode = VALUES(scope_mode), version = version + 1`,
		userID, roleID, scopeMode); err != nil {
		return fmt.Errorf("ensure explicit identity assignment %q: %w", strings.TrimSpace(roleCode), err)
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

func (r *AccessPolicyRepo) EffectiveAccessMany(ctx context.Context, userIDs []int64) (map[int64]*domain.EffectiveAccess, error) {
	userIDs = uniquePositiveInt64s(userIDs)
	out := make(map[int64]*domain.EffectiveAccess, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	revision, err := r.GetPolicyRevision(ctx)
	if err != nil {
		return nil, err
	}
	marks := resourceGroupPlaceholders(len(userIDs))
	args := make([]interface{}, 0, len(userIDs)*3)
	for range 3 {
		for _, userID := range userIDs {
			args = append(args, userID)
		}
	}
	rows, err := r.db.db.QueryContext(ctx, `
		WITH effective_assignments AS (
		  SELECT a.id AS assignment_id, a.user_id, r.id AS role_id, r.code AS role_code, r.name AS role_name,
		         a.scope_mode, a.source_type, a.source_ref_id, a.version,
		         NULL AS policy_subject_type, NULL AS policy_subject_id
		  FROM auth_user_role_assignments a
		  JOIN auth_roles r ON r.id = a.role_id AND r.archived_at IS NULL
		  WHERE a.user_id IN (`+marks+`)
		  UNION ALL
		  SELECT 0, u.id, r.id, r.code, r.name, p.scope_mode, 'org_policy', p.id, p.version,
		         p.subject_type, p.subject_id
		  FROM users u
		  JOIN auth_org_role_policies p ON p.enabled = 1 AND (
		    (p.subject_type = 'department' AND p.subject_id = u.department_id)
		    OR (p.subject_type = 'team' AND p.subject_id = u.team_id)
		  )
		  JOIN auth_roles r ON r.id = p.role_id AND r.archived_at IS NULL
		  WHERE u.id IN (`+marks+`)
		)
		SELECT u.id,
		       ea.assignment_id, ea.role_id, ea.role_code, ea.role_name, ea.scope_mode,
		       ea.source_type, ea.source_ref_id, ea.version,
		       COALESCE(scope_subject.subject_type, ea.policy_subject_type) AS subject_type,
		       COALESCE(scope_subject.subject_id, ea.policy_subject_id) AS subject_id,
		       COALESCE(department.name, team.name) AS subject_name,
		       rp.permission_code, rp.task_types
		FROM users u
		LEFT JOIN effective_assignments ea ON ea.user_id = u.id
		LEFT JOIN auth_assignment_scope_subjects scope_subject
		  ON ea.assignment_id > 0 AND ea.scope_mode = 'selected_org' AND scope_subject.assignment_id = ea.assignment_id
		LEFT JOIN org_departments department
		  ON COALESCE(scope_subject.subject_type, ea.policy_subject_type) = 'department'
		 AND department.id = COALESCE(scope_subject.subject_id, ea.policy_subject_id)
		LEFT JOIN org_teams team
		  ON COALESCE(scope_subject.subject_type, ea.policy_subject_type) = 'team'
		 AND team.id = COALESCE(scope_subject.subject_id, ea.policy_subject_id)
		LEFT JOIN (
		  SELECT role_permission.role_id, role_permission.permission_code, role_permission.task_types
		  FROM auth_role_permissions role_permission
		  JOIN auth_permissions permission ON permission.code = role_permission.permission_code AND permission.enabled = 1
		) rp ON rp.role_id = ea.role_id
		WHERE u.id IN (`+marks+`)
		ORDER BY u.id, ea.role_id, ea.source_type, ea.source_ref_id, ea.assignment_id, subject_type, subject_id, rp.permission_code`, args...)
	if err != nil {
		return nil, fmt.Errorf("list effective access for users: %w", err)
	}
	defer rows.Close()
	for _, userID := range userIDs {
		out[userID] = &domain.EffectiveAccess{
			UserID: userID, PolicyRevision: revision,
			Permissions: []domain.PermissionCode{}, Assignments: []domain.AccessAssignment{}, Sources: []domain.EffectiveAccessNote{},
		}
	}
	assignmentIndexes := make(map[string]int)
	subjectSeen := make(map[string]struct{})
	permissionSeen := make(map[int64]map[domain.PermissionCode]struct{})
	noteSeen := make(map[string]struct{})
	for rows.Next() {
		var userID int64
		var assignmentID, roleID, sourceRef, version sql.NullInt64
		var roleCode, roleName, scopeMode, sourceType sql.NullString
		var subjectType, subjectName sql.NullString
		var subjectID sql.NullInt64
		var permission sql.NullString
		var taskTypesRaw []byte
		if err := rows.Scan(&userID, &assignmentID, &roleID, &roleCode, &roleName, &scopeMode,
			&sourceType, &sourceRef, &version, &subjectType, &subjectID, &subjectName, &permission, &taskTypesRaw); err != nil {
			return nil, fmt.Errorf("scan effective access for users: %w", err)
		}
		effective := out[userID]
		if effective == nil || !roleID.Valid {
			continue
		}
		assignmentKey := fmt.Sprintf("%d:%d:%d:%s:%d", userID, assignmentID.Int64, roleID.Int64, sourceType.String, sourceRef.Int64)
		assignmentIndex, exists := assignmentIndexes[assignmentKey]
		if !exists {
			assignment := domain.AccessAssignment{
				ID: assignmentID.Int64, UserID: userID, RoleID: roleID.Int64,
				RoleCode: roleCode.String, RoleName: roleName.String,
				ScopeMode: domain.AccessScopeMode(scopeMode.String), SourceType: sourceType.String,
				Version: version.Int64, Subjects: []domain.AccessScopeSubject{},
			}
			if sourceRef.Valid {
				assignment.SourceRef = &sourceRef.Int64
			}
			effective.Assignments = append(effective.Assignments, assignment)
			assignmentIndex = len(effective.Assignments) - 1
			assignmentIndexes[assignmentKey] = assignmentIndex
		}
		if effective.Assignments[assignmentIndex].ScopeMode == domain.AccessScopeSelectedOrg && subjectType.Valid && subjectID.Valid {
			key := fmt.Sprintf("%s:%s:%d", assignmentKey, subjectType.String, subjectID.Int64)
			if _, seen := subjectSeen[key]; !seen {
				subjectSeen[key] = struct{}{}
				effective.Assignments[assignmentIndex].Subjects = append(effective.Assignments[assignmentIndex].Subjects, domain.AccessScopeSubject{
					SubjectType: domain.AccessSubjectType(subjectType.String), SubjectID: subjectID.Int64, SubjectName: subjectName.String,
				})
			}
		}
		if !permission.Valid {
			continue
		}
		taskTypes, decodeErr := decodeTaskTypesJSON(taskTypesRaw)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode effective task types for user %d role %d permission %s: %w", userID, roleID.Int64, permission.String, decodeErr)
		}
		permissionCode := domain.PermissionCode(permission.String)
		if permissionSeen[userID] == nil {
			permissionSeen[userID] = make(map[domain.PermissionCode]struct{})
		}
		if _, seen := permissionSeen[userID][permissionCode]; !seen {
			permissionSeen[userID][permissionCode] = struct{}{}
			effective.Permissions = append(effective.Permissions, permissionCode)
		}
		noteKey := fmt.Sprintf("%s:%s:%s", assignmentKey, permissionCode, strings.Join(taskTypes, ","))
		if _, seen := noteSeen[noteKey]; !seen {
			noteSeen[noteKey] = struct{}{}
			effective.Sources = append(effective.Sources, domain.EffectiveAccessNote{
				Permission: permissionCode, RoleID: roleID.Int64, RoleCode: roleCode.String,
				SourceType: sourceType.String, ScopeMode: domain.AccessScopeMode(scopeMode.String), TaskTypes: taskTypes,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, effective := range out {
		sort.Slice(effective.Permissions, func(i, j int) bool { return effective.Permissions[i] < effective.Permissions[j] })
	}
	return out, nil
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
		SELECT rp.role_id, rp.permission_code, rp.task_types
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
		var taskTypesRaw []byte
		if err := rows.Scan(&roleID, &permission, &taskTypesRaw); err != nil {
			return nil, nil, err
		}
		taskTypes, err := decodeTaskTypesJSON(taskTypesRaw)
		if err != nil {
			return nil, nil, fmt.Errorf("decode effective task types for role %d permission %s: %w", roleID, permission, err)
		}
		for _, assignment := range assignmentsByRole[roleID] {
			notes = append(notes, domain.EffectiveAccessNote{
				Permission: permission, RoleID: roleID, RoleCode: assignment.RoleCode,
				SourceType: assignment.SourceType, ScopeMode: assignment.ScopeMode, TaskTypes: taskTypes,
			})
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
