package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type userRepo struct{ db *DB }

func NewUserRepo(db *DB) repo.UserRepo { return &userRepo{db: db} }

func (r *userRepo) Count(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return total, nil
}

func (r *userRepo) CountByRole(ctx context.Context, role domain.Role) (int64, error) {
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_roles WHERE role = ?`, role).Scan(&total); err != nil {
		return 0, fmt.Errorf("count users by role: %w", err)
	}
	return total, nil
}

func (r *userRepo) CountByDepartment(ctx context.Context, department string) (int64, error) {
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE department = ?`, strings.TrimSpace(department)).Scan(&total); err != nil {
		return 0, fmt.Errorf("count users by department: %w", err)
	}
	return total, nil
}

func (r *userRepo) CountByTeam(ctx context.Context, team string) (int64, error) {
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE team = ?`, strings.TrimSpace(team)).Scan(&total); err != nil {
		return 0, fmt.Errorf("count users by team: %w", err)
	}
	return total, nil
}

func (r *userRepo) Create(ctx context.Context, tx repo.Tx, user *domain.User) (int64, error) {
	result, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO users (username, display_name, department, team, managed_departments_json, managed_teams_json, mobile, email, password_hash, status, employment_type, is_config_super_admin, last_login_at, created_at, updated_at, jst_u_id, jst_raw_snapshot_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.Username,
		user.DisplayName,
		user.Department,
		user.Team,
		marshalStringSlice(user.ManagedDepartments),
		marshalStringSlice(user.ManagedTeams),
		user.Mobile,
		user.Email,
		user.PasswordHash,
		user.Status,
		user.EmploymentType,
		user.IsConfigSuperAdmin,
		toNullTime(user.LastLoginAt),
		user.CreatedAt,
		user.UpdatedAt,
		toNullInt64(user.JstUID),
		nullString(user.JstRawSnapshotJSON),
	)
	if err != nil {
		return 0, fmt.Errorf("insert user: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("user last insert id: %w", err)
	}
	return id, nil
}

func (r *userRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, department, team, managed_departments_json, managed_teams_json, mobile, email, password_hash, status, employment_type, is_config_super_admin, last_login_at, created_at, updated_at, jst_u_id, jst_raw_snapshot_json
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (r *userRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, department, team, managed_departments_json, managed_teams_json, mobile, email, password_hash, status, employment_type, is_config_super_admin, last_login_at, created_at, updated_at, jst_u_id, jst_raw_snapshot_json
		FROM users WHERE LOWER(username) = LOWER(?)`, username)
	return scanUser(row)
}

func (r *userRepo) GetByMobile(ctx context.Context, mobile string) (*domain.User, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, department, team, managed_departments_json, managed_teams_json, mobile, email, password_hash, status, employment_type, is_config_super_admin, last_login_at, created_at, updated_at, jst_u_id, jst_raw_snapshot_json
		FROM users WHERE mobile = ?`, mobile)
	return scanUser(row)
}

func (r *userRepo) GetByJstUID(ctx context.Context, jstUID int64) (*domain.User, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, department, team, managed_departments_json, managed_teams_json, mobile, email, password_hash, status, employment_type, is_config_super_admin, last_login_at, created_at, updated_at, jst_u_id, jst_raw_snapshot_json
		FROM users WHERE jst_u_id = ?`, jstUID)
	return scanUser(row)
}

func (r *userRepo) List(ctx context.Context, filter repo.UserListFilter) ([]*domain.User, int64, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	where := []string{"1=1"}
	args := make([]interface{}, 0, 7)
	if filter.Role != nil && string(*filter.Role) != "" {
		where = append(where, "EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id = users.id AND ur.role = ?)")
		args = append(args, *filter.Role)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		where = append(where, "(users.username LIKE ? OR users.display_name LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	if filter.Status != nil && filter.Status.Valid() {
		where = append(where, "users.status = ?")
		args = append(args, *filter.Status)
	}
	if filter.Department != nil && strings.TrimSpace(string(*filter.Department)) != "" {
		where = append(where, "users.department = ?")
		args = append(args, *filter.Department)
	}
	if team := strings.TrimSpace(filter.Team); team != "" {
		where = append(where, "users.team = ?")
		args = append(args, team)
	}

	countQuery := `SELECT COUNT(*) FROM users WHERE ` + strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, username, display_name, department, team, managed_departments_json, managed_teams_json, mobile, email, password_hash, status, employment_type, is_config_super_admin, last_login_at, created_at, updated_at, jst_u_id, jst_raw_snapshot_json
		FROM users
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}
	return users, total, rows.Err()
}

// ListActiveByRole returns every user with status='active' that carries the
// given role. It has no department/team/keyword filters and no pagination —
// callers should use this ONLY for assignment-candidate-pool lookups (e.g.
// `/v1/users/designers` dropdown) where the result set is expected to be
// small and where the route layer is responsible for access control.
func (r *userRepo) ListActiveByRole(ctx context.Context, role domain.Role) ([]*domain.User, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, username, display_name, department, team, managed_departments_json, managed_teams_json, mobile, email, password_hash, status, employment_type, is_config_super_admin, last_login_at, created_at, updated_at, jst_u_id, jst_raw_snapshot_json
		FROM users
		WHERE status = ?
		  AND EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id = users.id AND ur.role = ?)
		ORDER BY id DESC`,
		domain.UserStatusActive, role,
	)
	if err != nil {
		return nil, fmt.Errorf("list active users by role: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list active users by role rows: %w", err)
	}
	return users, nil
}

func (r *userRepo) Update(ctx context.Context, tx repo.Tx, user *domain.User) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE users
		SET display_name = ?, department = ?, team = ?, managed_departments_json = ?, managed_teams_json = ?, mobile = ?, email = ?, status = ?, employment_type = ?, is_config_super_admin = ?, updated_at = ?
		WHERE id = ?`,
		user.DisplayName,
		user.Department,
		user.Team,
		marshalStringSlice(user.ManagedDepartments),
		marshalStringSlice(user.ManagedTeams),
		user.Mobile,
		user.Email,
		user.Status,
		user.EmploymentType,
		user.IsConfigSuperAdmin,
		user.UpdatedAt,
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func (r *userRepo) UpdateJstFields(ctx context.Context, tx repo.Tx, userID int64, displayName, status, department, team string, managedDepartments, managedTeams []string, jstRawSnapshot string, jstUID *int64, lastLoginAt *time.Time) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE users
		SET display_name = ?, status = ?, department = ?, team = ?, managed_departments_json = ?, managed_teams_json = ?, jst_u_id = ?, jst_raw_snapshot_json = ?, last_login_at = ?, updated_at = ?
		WHERE id = ?`,
		displayName, status, department, team, marshalStringSlice(managedDepartments), marshalStringSlice(managedTeams), toNullInt64(jstUID), nullString(jstRawSnapshot), toNullTime(lastLoginAt), time.Now().UTC(), userID,
	)
	if err != nil {
		return fmt.Errorf("update user jst fields: %w", err)
	}
	return nil
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (r *userRepo) UpdatePassword(ctx context.Context, tx repo.Tx, userID int64, passwordHash string, updatedAt time.Time) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		passwordHash, updatedAt, userID,
	)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	return nil
}

func (r *userRepo) UpdateLastLogin(ctx context.Context, tx repo.Tx, userID int64, at time.Time) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE users SET last_login_at = ?, updated_at = ? WHERE id = ?`,
		at, at, userID,
	)
	if err != nil {
		return fmt.Errorf("update user last_login_at: %w", err)
	}
	return nil
}

func (r *userRepo) ReplaceRoles(ctx context.Context, tx repo.Tx, userID int64, roles []domain.Role) error {
	sqlTx := Unwrap(tx)
	if _, err := sqlTx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete user roles: %w", err)
	}
	roles = domain.NormalizeRoleValues(roles)
	for _, role := range roles {
		if _, err := sqlTx.ExecContext(ctx, `
			INSERT INTO user_roles (user_id, role, created_at) VALUES (?, ?, ?)`,
			userID, role, time.Now().UTC(),
		); err != nil {
			return fmt.Errorf("insert user role: %w", err)
		}
	}
	return nil
}

func (r *userRepo) ListRoles(ctx context.Context, userID int64) ([]domain.Role, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT role FROM user_roles WHERE user_id = ? ORDER BY role ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user roles: %w", err)
	}
	defer rows.Close()

	roles := make([]string, 0)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("scan user role: %w", err)
		}
		roles = append(roles, role)
	}
	return domain.NormalizeRoles(roles), rows.Err()
}

// ListRolesRaw returns the raw role strings persisted on user_roles without
// applying NormalizeRoles. It is consumed by ResolveRequestActor via an
// optional type assertion to observe dropped unknown strings for the
// actor_role_hydration_degraded telemetry signal. It is intentionally not
// part of the UserRepo interface so stubs may opt in without forcing every
// test/mocked implementation to add the method.
func (r *userRepo) ListRolesRaw(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT role FROM user_roles WHERE user_id = ? ORDER BY role ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list raw user roles: %w", err)
	}
	defer rows.Close()

	roles := make([]string, 0)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("scan raw user role: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

// ListRolesByUserIDs batches role reads for user list endpoints.
// It is intentionally not part of the generic UserRepo interface so
// service-layer callers can adopt it via type assertion without affecting stubs.
func (r *userRepo) ListRolesByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]domain.Role, error) {
	unique := make([]int64, 0, len(userIDs))
	seen := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		unique = append(unique, userID)
	}
	rolesByUser := make(map[int64][]domain.Role, len(unique))
	if len(unique) == 0 {
		return rolesByUser, nil
	}

	placeholders := make([]string, 0, len(unique))
	args := make([]interface{}, 0, len(unique))
	for _, userID := range unique {
		placeholders = append(placeholders, "?")
		args = append(args, userID)
	}
	query := `
		SELECT user_id, role
		FROM user_roles
		WHERE user_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY user_id ASC, role ASC`
	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list user roles by ids: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID int64
		var role string
		if err := rows.Scan(&userID, &role); err != nil {
			return nil, fmt.Errorf("scan user role by ids: %w", err)
		}
		rolesByUser[userID] = append(rolesByUser[userID], domain.Role(strings.TrimSpace(role)))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for userID, roles := range rolesByUser {
		rolesByUser[userID] = domain.NormalizeRoleValues(roles)
	}
	return rolesByUser, nil
}

func (r *userRepo) ListConfigManagedAdmins(ctx context.Context) ([]*domain.User, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, username, display_name, department, team, managed_departments_json, managed_teams_json, mobile, email, password_hash, status, employment_type, is_config_super_admin, last_login_at, created_at, updated_at, jst_u_id, jst_raw_snapshot_json
		FROM users
		WHERE is_config_super_admin = 1
		ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list config managed admins: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

type userSessionRepo struct{ db *DB }

func NewUserSessionRepo(db *DB) repo.UserSessionRepo { return &userSessionRepo{db: db} }

func (r *userSessionRepo) Create(ctx context.Context, tx repo.Tx, session *domain.UserSession) (*domain.UserSession, error) {
	_, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO user_sessions (session_id, user_id, token_hash, expires_at, last_seen_at, revoked_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		session.SessionID,
		session.UserID,
		session.TokenHash,
		session.ExpiresAt,
		toNullTime(session.LastSeenAt),
		toNullTime(session.RevokedAt),
		session.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user session: %w", err)
	}
	return session, nil
}

func (r *userSessionRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.UserSession, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT session_id, user_id, token_hash, expires_at, last_seen_at, revoked_at, created_at
		FROM user_sessions WHERE token_hash = ?`, tokenHash)

	var session domain.UserSession
	var lastSeenAt sql.NullTime
	var revokedAt sql.NullTime
	if err := row.Scan(
		&session.SessionID,
		&session.UserID,
		&session.TokenHash,
		&session.ExpiresAt,
		&lastSeenAt,
		&revokedAt,
		&session.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan user session: %w", err)
	}
	session.LastSeenAt = fromNullTime(lastSeenAt)
	session.RevokedAt = fromNullTime(revokedAt)
	return &session, nil
}

func (r *userSessionRepo) Touch(ctx context.Context, sessionID string, at time.Time) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE user_sessions SET last_seen_at = ? WHERE session_id = ?`, at, sessionID)
	if err != nil {
		return fmt.Errorf("touch user session: %w", err)
	}
	return nil
}

type permissionLogRepo struct{ db *DB }

func NewPermissionLogRepo(db *DB) repo.PermissionLogRepo { return &permissionLogRepo{db: db} }

func (r *permissionLogRepo) Create(ctx context.Context, entry *domain.PermissionLog) error {
	return r.create(ctx, nil, entry)
}

func (r *permissionLogRepo) CreateTx(ctx context.Context, tx repo.Tx, entry *domain.PermissionLog) error {
	return r.create(ctx, tx, entry)
}

func (r *permissionLogRepo) create(ctx context.Context, tx repo.Tx, entry *domain.PermissionLog) error {
	actorRolesJSON, err := json.Marshal(domain.NormalizeRoleValues(entry.ActorRoles))
	if err != nil {
		return fmt.Errorf("marshal permission log actor roles: %w", err)
	}
	targetRolesJSON, err := json.Marshal(domain.NormalizeRoleValues(entry.TargetRoles))
	if err != nil {
		return fmt.Errorf("marshal permission log target roles: %w", err)
	}
	requiredRolesJSON, err := json.Marshal(domain.NormalizeRoleValues(entry.RequiredRoles))
	if err != nil {
		return fmt.Errorf("marshal permission log required roles: %w", err)
	}
	exec := interface {
		ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	}(r.db.db)
	if tx != nil {
		exec = Unwrap(tx)
	}
	_, err = exec.ExecContext(ctx, `
		INSERT INTO permission_logs (
			actor_id, actor_username, actor_source, auth_mode, readiness, session_required, debug_compatible, actor_roles_json,
			action_type, target_user_id, target_username, target_roles_json,
			method, route_path, required_roles_json, granted, reason, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		toNullInt64(entry.ActorID),
		entry.ActorUsername,
		entry.ActorSource,
		entry.AuthMode,
		entry.Readiness,
		entry.SessionRequired,
		entry.DebugCompatible,
		string(actorRolesJSON),
		entry.ActionType,
		toNullInt64(entry.TargetUserID),
		entry.TargetUsername,
		string(targetRolesJSON),
		entry.Method,
		entry.RoutePath,
		string(requiredRolesJSON),
		entry.Granted,
		entry.Reason,
		entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert permission log: %w", err)
	}
	return nil
}

func (r *permissionLogRepo) List(ctx context.Context, filter repo.PermissionLogListFilter) ([]*domain.PermissionLog, int64, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	where := []string{"1=1"}
	args := make([]interface{}, 0, 4)
	if filter.ActorID != nil {
		where = append(where, "actor_id = ?")
		args = append(args, *filter.ActorID)
	}
	if actorUsername := strings.TrimSpace(filter.ActorUsername); actorUsername != "" {
		where = append(where, "actor_username LIKE ?")
		args = append(args, "%"+actorUsername+"%")
	}
	if actionType := strings.TrimSpace(filter.ActionType); actionType != "" {
		where = append(where, "action_type = ?")
		args = append(args, actionType)
	}
	if filter.TargetUserID != nil {
		where = append(where, "target_user_id = ?")
		args = append(args, *filter.TargetUserID)
	}
	if targetUsername := strings.TrimSpace(filter.TargetUsername); targetUsername != "" {
		where = append(where, "target_username LIKE ?")
		args = append(args, "%"+targetUsername+"%")
	}
	if filter.Granted != nil {
		where = append(where, "granted = ?")
		args = append(args, *filter.Granted)
	}
	if method := strings.TrimSpace(filter.Method); method != "" {
		where = append(where, "method = ?")
		args = append(args, strings.ToUpper(method))
	}
	if routePath := strings.TrimSpace(filter.RoutePath); routePath != "" {
		where = append(where, "route_path LIKE ?")
		args = append(args, "%"+routePath+"%")
	}

	countQuery := `SELECT COUNT(*) FROM permission_logs WHERE ` + strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count permission logs: %w", err)
	}

	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, actor_id, actor_username, actor_source, auth_mode, readiness, session_required, debug_compatible, actor_roles_json,
		       action_type, target_user_id, target_username, target_roles_json,
		       method, route_path, required_roles_json, granted, reason, created_at
		FROM permission_logs
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list permission logs: %w", err)
	}
	defer rows.Close()

	logs := make([]*domain.PermissionLog, 0)
	for rows.Next() {
		var entry domain.PermissionLog
		var actorID sql.NullInt64
		var targetUserID sql.NullInt64
		var actorRolesJSON string
		var targetRolesJSON string
		var requiredRolesJSON string
		if err := rows.Scan(
			&entry.ID,
			&actorID,
			&entry.ActorUsername,
			&entry.ActorSource,
			&entry.AuthMode,
			&entry.Readiness,
			&entry.SessionRequired,
			&entry.DebugCompatible,
			&actorRolesJSON,
			&entry.ActionType,
			&targetUserID,
			&entry.TargetUsername,
			&targetRolesJSON,
			&entry.Method,
			&entry.RoutePath,
			&requiredRolesJSON,
			&entry.Granted,
			&entry.Reason,
			&entry.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan permission log: %w", err)
		}
		entry.ActorID = fromNullInt64(actorID)
		entry.TargetUserID = fromNullInt64(targetUserID)
		entry.ActorRoles, err = unmarshalOptionalRoles(actorRolesJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("unmarshal permission log actor roles: %w", err)
		}
		entry.TargetRoles, err = unmarshalOptionalRoles(targetRolesJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("unmarshal permission log target roles: %w", err)
		}
		entry.RequiredRoles, err = unmarshalOptionalRoles(requiredRolesJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("unmarshal permission log required roles: %w", err)
		}
		logs = append(logs, &entry)
	}
	return logs, total, rows.Err()
}

type userScanner interface {
	Scan(dest ...interface{}) error
}

func scanUser(scanner userScanner) (*domain.User, error) {
	var user domain.User
	var lastLoginAt sql.NullTime
	var jstUID sql.NullInt64
	var jstRaw sql.NullString
	var employmentType sql.NullString
	var managedDepartmentsJSON sql.NullString
	var managedTeamsJSON sql.NullString
	if err := scanner.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.Department,
		&user.Team,
		&managedDepartmentsJSON,
		&managedTeamsJSON,
		&user.Mobile,
		&user.Email,
		&user.PasswordHash,
		&user.Status,
		&employmentType,
		&user.IsConfigSuperAdmin,
		&lastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&jstUID,
		&jstRaw,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	user.LastLoginAt = fromNullTime(lastLoginAt)
	if jstUID.Valid {
		user.JstUID = &jstUID.Int64
	}
	if jstRaw.Valid {
		user.JstRawSnapshotJSON = jstRaw.String
	}
	user.ManagedDepartments = unmarshalStringSlice(managedDepartmentsJSON.String)
	user.ManagedTeams = unmarshalStringSlice(managedTeamsJSON.String)
	user.EmploymentType = domain.EmploymentType(strings.TrimSpace(employmentType.String))
	if !user.EmploymentType.Valid() {
		user.EmploymentType = domain.EmploymentTypeFullTime
	}
	return &user, nil
}

func marshalStringSlice(values []string) interface{} {
	if len(values) == 0 {
		return nil
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return nil
	}
	return string(raw)
}

func unmarshalStringSlice(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return values
}
