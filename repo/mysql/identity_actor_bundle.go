package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
)

func (r *userSessionRepo) ResolveActorBundle(ctx context.Context, tokenHash string, at time.Time) (*domain.UserSession, *domain.User, []string, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	if tokenHash == "" {
		return nil, nil, nil, nil
	}
	atLiteral := at.UTC().Format("2006-01-02 15:04:05.999999")
	query := fmt.Sprintf(`
		SELECT session_id, user_id, token_hash, expires_at, last_seen_at, revoked_at, created_at
		FROM user_sessions WHERE token_hash = '%[1]s';

		SELECT id, username, display_name, department, team, managed_departments_json, managed_teams_json, mobile, email, password_hash, status, employment_type, is_config_super_admin, last_login_at, created_at, updated_at, jst_u_id, jst_raw_snapshot_json
		FROM users
		WHERE id = (SELECT user_id FROM user_sessions WHERE token_hash = '%[1]s');

		SELECT role
		FROM user_roles
		WHERE user_id = (SELECT user_id FROM user_sessions WHERE token_hash = '%[1]s')
		ORDER BY role ASC;

		UPDATE user_sessions
		SET last_seen_at = '%[2]s'
		WHERE token_hash = '%[1]s'
		  AND revoked_at IS NULL
		  AND expires_at > UTC_TIMESTAMP(6)`, tokenHash, atLiteral)

	rows, err := r.db.db.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve actor bundle: %w", err)
	}
	defer rows.Close()

	session, err := scanSingleUserSessionResult(rows)
	if err != nil || session == nil {
		return session, nil, nil, err
	}
	if !rows.NextResultSet() {
		return session, nil, nil, rows.Err()
	}
	user, err := scanSingleUserResult(rows)
	if err != nil {
		return nil, nil, nil, err
	}
	if !rows.NextResultSet() {
		return session, user, nil, rows.Err()
	}
	roles, err := scanRawRoleRows(rows)
	if err != nil {
		return nil, nil, nil, err
	}
	return session, user, roles, rows.Err()
}

func scanSingleUserSessionResult(rows *sql.Rows) (*domain.UserSession, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	var session domain.UserSession
	var lastSeenAt sql.NullTime
	var revokedAt sql.NullTime
	if err := rows.Scan(
		&session.SessionID,
		&session.UserID,
		&session.TokenHash,
		&session.ExpiresAt,
		&lastSeenAt,
		&revokedAt,
		&session.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan user session: %w", err)
	}
	session.LastSeenAt = fromNullTime(lastSeenAt)
	session.RevokedAt = fromNullTime(revokedAt)
	return &session, rows.Err()
}

func scanSingleUserResult(rows *sql.Rows) (*domain.User, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	user, err := scanUser(rows)
	if err != nil {
		return nil, err
	}
	return user, rows.Err()
}

func scanRawRoleRows(rows *sql.Rows) ([]string, error) {
	roles := make([]string, 0)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("scan user role: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}
