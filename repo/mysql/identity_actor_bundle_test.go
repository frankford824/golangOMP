package mysqlrepo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestResolveActorBundleUsesParameterizedSingleStatements(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repository := NewUserSessionRepo(New(db)).(*userSessionRepo)
	now := time.Date(2026, 8, 11, 6, 5, 0, 123456000, time.UTC)
	tokenHash := "token-hash-123"

	mock.ExpectQuery("SELECT session_id, user_id, token_hash").
		WithArgs(tokenHash).
		WillReturnRows(sqlmock.NewRows([]string{
			"session_id", "user_id", "token_hash", "expires_at", "last_seen_at", "revoked_at", "created_at",
		}).AddRow("session-1", int64(292), tokenHash, now.Add(time.Hour), nil, nil, now.Add(-time.Hour)))
	mock.ExpectQuery("(?s)SELECT\\s+id, username.*FROM users WHERE id = \\?").
		WithArgs(int64(292)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "employee_no", "display_name", "department", "department_id", "team", "team_id",
			"managed_departments_json", "managed_teams_json", "mobile", "email", "avatar_url", "password_hash", "status",
			"employment_type", "is_config_super_admin", "last_login_at", "created_at", "updated_at", "jst_u_id", "jst_raw_snapshot_json",
		}).AddRow(
			int64(292), "zhouqian", nil, "周谦", "运营部", int64(1), "淘系运营三部", int64(2),
			"[]", "[]", "", "", nil, "hash", "active", "full_time", false, nil, now, now, nil, nil,
		))
	mock.ExpectQuery("SELECT role FROM user_roles WHERE user_id = \\? ORDER BY role ASC").
		WithArgs(int64(292)).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("operator"))
	mock.ExpectExec("UPDATE user_sessions").
		WithArgs(now, tokenHash).
		WillReturnResult(sqlmock.NewResult(0, 1))

	session, user, roles, err := repository.ResolveActorBundle(context.Background(), tokenHash, now)
	if err != nil {
		t.Fatalf("ResolveActorBundle() error = %v", err)
	}
	if session == nil || session.UserID != 292 || session.TokenHash != tokenHash {
		t.Fatalf("session = %+v", session)
	}
	if user == nil || user.ID != 292 || user.Username != "zhouqian" {
		t.Fatalf("user = %+v", user)
	}
	if len(roles) != 1 || roles[0] != "operator" {
		t.Fatalf("roles = %#v", roles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveActorBundleReturnsNilForUnknownToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repository := NewUserSessionRepo(New(db)).(*userSessionRepo)
	mock.ExpectQuery("SELECT session_id, user_id, token_hash").
		WithArgs("unknown").
		WillReturnError(sql.ErrNoRows)

	session, user, roles, err := repository.ResolveActorBundle(context.Background(), "unknown", time.Now())
	if err != nil {
		t.Fatalf("ResolveActorBundle() error = %v", err)
	}
	if session != nil || user != nil || roles != nil {
		t.Fatalf("bundle = (%+v, %+v, %#v), want all nil", session, user, roles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
