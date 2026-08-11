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
	session, err := scanSingleUserSessionResult(r.db.db.QueryRowContext(ctx, `
		SELECT session_id, user_id, token_hash, expires_at, last_seen_at, revoked_at, created_at
		FROM user_sessions
		WHERE token_hash = ?`, tokenHash))
	if err != nil || session == nil {
		if err != nil {
			return nil, nil, nil, fmt.Errorf("resolve actor bundle session: %w", err)
		}
		return nil, nil, nil, nil
	}

	user, err := scanUser(r.db.db.QueryRowContext(ctx, `SELECT `+userSelectColumns+` FROM users WHERE id = ?`, session.UserID))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve actor bundle user: %w", err)
	}

	rows, err := r.db.db.QueryContext(ctx, `SELECT role FROM user_roles WHERE user_id = ? ORDER BY role ASC`, session.UserID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve actor bundle roles: %w", err)
	}
	defer rows.Close()
	roles, err := scanRawRoleRows(rows)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve actor bundle roles: %w", err)
	}

	if _, err := r.db.db.ExecContext(ctx, `
		UPDATE user_sessions
		SET last_seen_at = ?
		WHERE token_hash = ?
		  AND revoked_at IS NULL
		  AND expires_at > UTC_TIMESTAMP(6)`, at.UTC(), tokenHash); err != nil {
		return nil, nil, nil, fmt.Errorf("resolve actor bundle touch session: %w", err)
	}
	return session, user, roles, nil
}

func scanSingleUserSessionResult(scanner userScanner) (*domain.UserSession, error) {
	var session domain.UserSession
	var lastSeenAt sql.NullTime
	var revokedAt sql.NullTime
	if err := scanner.Scan(
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
