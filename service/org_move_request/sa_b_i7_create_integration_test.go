//go:build integration

package org_move_request

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/testsupport/r35"
)

func TestSABI7_CreateOrgMoveRequest_PendingSuperAdminConfirm(t *testing.T) {
	db, svc := saBOpenOrgMoveTestDB(t)
	userID := int64(30071)
	actorID := int64(30072)
	var requestIDs []int64
	saBCleanupOrgMove(t, db, nil, userID, actorID)
	defer func() { saBCleanupOrgMove(t, db, requestIDs, userID, actorID) }()

	sourceID := saBDepartmentID(t, db, string(domain.DepartmentOperations))
	targetID := saBDepartmentID(t, db, string(domain.DepartmentCloudWarehouse))
	saBInsertOrgMoveUser(t, db, userID, "sab_i7_target", string(domain.DepartmentOperations), "淘系一组", []domain.Role{domain.RoleMember}, nil)
	saBInsertOrgMoveUser(t, db, actorID, "sab_i7_dept_admin", string(domain.DepartmentOperations), "淘系一组", []domain.Role{domain.RoleMember, domain.RoleDeptAdmin}, []string{string(domain.DepartmentOperations)})

	item, appErr := svc.Create(context.Background(), saBActor(actorID, "sab_i7_dept_admin", []domain.Role{domain.RoleDeptAdmin}, string(domain.DepartmentOperations), "淘系一组", []string{string(domain.DepartmentOperations)}), sourceID, CreateParams{
		UserID:             userID,
		TargetDepartmentID: &targetID,
		Reason:             "SA-B.1 I7 create",
	})
	if appErr != nil {
		t.Fatalf("Create org move request appErr=%v", appErr)
	}
	requestIDs = append(requestIDs, item.ID)

	var department string
	if err := db.QueryRow(`SELECT department FROM users WHERE id = ?`, userID).Scan(&department); err != nil {
		t.Fatalf("select user department: %v", err)
	}
	logCount := saBPermissionLogCount(t, db, "org_move_requested", userID)
	if item.State != domain.OrgMoveRequestStatePendingSuperAdminConfirm || department != string(domain.DepartmentOperations) || logCount == 0 {
		t.Fatalf("created request=%+v department=%q org_move_requested_logs=%d", item, department, logCount)
	}
}

func saBOpenOrgMoveTestDB(t *testing.T) (*sql.DB, Service) {
	t.Helper()
	db := r35.MustOpenTestDB(t)
	wrapped := mysqlrepo.New(db)
	userRepo := mysqlrepo.NewUserRepo(wrapped)
	orgRepo := mysqlrepo.NewOrgRepo(wrapped)
	requestRepo := mysqlrepo.NewOrgMoveRequestRepo(wrapped)
	logRepo := mysqlrepo.NewPermissionLogRepo(wrapped)
	return db, NewService(userRepo, orgRepo, requestRepo, logRepo, wrapped)
}

func saBCleanupOrgMove(t *testing.T, db *sql.DB, requestIDs []int64, userIDs ...int64) {
	t.Helper()
	if len(requestIDs) > 0 {
		args, in := saBInArgs(t, false, requestIDs...)
		_, _ = db.Exec(`DELETE FROM org_move_requests WHERE id IN (`+in+`)`, args...)
	}
	if len(userIDs) == 0 {
		return
	}
	args, in := saBInArgs(t, true, userIDs...)
	_, _ = db.Exec(`DELETE FROM permission_logs WHERE actor_id IN (`+in+`) OR target_user_id IN (`+in+`)`, append(args, args...)...)
	_, _ = db.Exec(`DELETE FROM org_move_requests WHERE user_id IN (`+in+`) OR requested_by IN (`+in+`) OR resolved_by IN (`+in+`)`, append(append(args, args...), args...)...)
	_, _ = db.Exec(`DELETE FROM user_sessions WHERE user_id IN (`+in+`)`, args...)
	_, _ = db.Exec(`DELETE FROM user_roles WHERE user_id IN (`+in+`)`, args...)
	_, _ = db.Exec(`DELETE FROM users WHERE id IN (`+in+`)`, args...)
}

func saBInArgs(t *testing.T, validateUserIDs bool, ids ...int64) ([]interface{}, string) {
	t.Helper()
	args := make([]interface{}, 0, len(ids))
	placeholders := make([]string, 0, len(ids))
	for _, id := range ids {
		if validateUserIDs && id < 30000 {
			t.Fatalf("SA-B fixture user id %d is below 30000", id)
		}
		args = append(args, id)
		placeholders = append(placeholders, "?")
	}
	return args, strings.Join(placeholders, ",")
}

func saBInsertOrgMoveUser(t *testing.T, db *sql.DB, id int64, username, department, team string, roles []domain.Role, managedDepartments []string) {
	t.Helper()
	if id < 30000 {
		t.Fatalf("SA-B fixture user id %d is below 30000", id)
	}
	if username == "" {
		username = fmt.Sprintf("sab_org_move_%d", id)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("ChangeMeAdmin123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	managedRaw, _ := json.Marshal(managedDepartments)
	var managed interface{}
	if len(managedDepartments) > 0 {
		managed = string(managedRaw)
	}
	_, err = db.Exec(`
		INSERT INTO users
			(id, username, display_name, department, team, managed_departments_json, managed_teams_json,
			 mobile, email, password_hash, status, employment_type, is_config_super_admin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, 'active', 'full_time', 0, NOW(6), NOW(6))`,
		id, username, username, department, team, managed, fmt.Sprintf("139%08d", id), username+"@example.test", string(hash))
	if err != nil {
		t.Fatalf("insert org move user %d: %v", id, err)
	}
	if len(roles) == 0 {
		roles = []domain.Role{domain.RoleMember}
	}
	for _, role := range domain.NormalizeRoleValues(roles) {
		if _, err := db.Exec(`INSERT INTO user_roles (user_id, role, created_at) VALUES (?, ?, NOW(6))`, id, role); err != nil {
			t.Fatalf("insert org move role %s for user %d: %v", role, id, err)
		}
	}
}

func saBDepartmentID(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`SELECT id FROM org_departments WHERE name = ? AND enabled = 1`, name).Scan(&id); err != nil {
		t.Fatalf("select department id for %s: %v", name, err)
	}
	return id
}

func saBActor(id int64, username string, roles []domain.Role, department, team string, managedDepartments []string) domain.RequestActor {
	return domain.RequestActor{
		ID:                 id,
		Username:           username,
		Roles:              domain.NormalizeRoleValues(roles),
		Department:         department,
		Team:               team,
		ManagedDepartments: managedDepartments,
		Source:             domain.RequestActorSourceSessionToken,
		AuthMode:           domain.AuthModeSessionTokenRoleEnforced,
	}
}

func saBPermissionLogCount(t *testing.T, db *sql.DB, action string, targetUserID int64) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM permission_logs WHERE action_type = ? AND target_user_id = ?`, action, targetUserID).Scan(&count); err != nil {
		t.Fatalf("count permission log action=%s target=%d: %v", action, targetUserID, err)
	}
	return count
}

func saBAppDenyCode(appErr *domain.AppError) string {
	if appErr == nil || appErr.Details == nil {
		return ""
	}
	switch details := appErr.Details.(type) {
	case map[string]string:
		return details["deny_code"]
	case map[string]interface{}:
		if value, ok := details["deny_code"].(string); ok {
			return value
		}
	}
	return ""
}
