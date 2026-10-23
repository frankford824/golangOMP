package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

// Explicitly opt-in. Every fixture, policy change and audit row is rolled back;
// no real employee's grants, profile or pay records are touched.
func TestWorkbenchAccessMySQLLifecycle(t *testing.T) {
	dsn := os.Getenv("WORKBENCH_ACCESS_TEST_DSN")
	if dsn == "" {
		t.Skip("WORKBENCH_ACCESS_TEST_DSN not configured; opt-in rollback integration")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("invalid test MySQL configuration")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	mdb := New(db)
	wrapped, tx, err := mdb.BeginTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	policy := NewAccessPolicyRepo(mdb)
	revision, err := policy.LockPolicyRevision(ctx, wrapped)
	if err != nil {
		t.Fatal(err)
	}
	exec := func(query string, args ...interface{}) sql.Result {
		result, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	username := fmt.Sprintf("aw-access-rollback-%d", time.Now().UnixNano())
	users := &userRepo{db: mdb}
	userID, err := users.Create(ctx, wrapped, &domain.User{Username: username, DisplayName: "回滚测试账号", PasswordHash: "!no-login-test-fixture", Status: domain.UserStatusActive, EmploymentType: domain.EmploymentTypePartTime, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	roleID := func(code string) int64 {
		var id int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM auth_roles WHERE code = ? AND archived_at IS NULL`, code).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	submitterID, managerID, designerID := roleID("asset_submitter"), roleID("asset_manager"), roleID("designer")
	assignments := []domain.AccessAssignment{{RoleID: submitterID, ScopeMode: domain.AccessScopeSelf}, {RoleID: managerID, ScopeMode: domain.AccessScopeGlobal}, {RoleID: designerID, ScopeMode: domain.AccessScopeGlobal}}
	if err := policy.ReplaceUserAssignments(ctx, wrapped, userID, userID, assignments); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO user_roles (user_id,role) VALUES (?,'AssetSettlement')`, userID)
	member := func() *domain.AssetWorkbenchMember {
		value, err := scanAssetWorkbenchMember(tx.QueryRowContext(ctx, assetWorkbenchMemberSelect()+` WHERE u.id = ?`, userID))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	hasRole := func(roles []domain.Role, wanted domain.Role) bool {
		for _, role := range roles {
			if role == wanted {
				return true
			}
		}
		return false
	}
	first := member()
	if first.Status != "active" || !hasRole(first.Roles, domain.RoleAssetManager) || hasRole(first.Roles, domain.RoleAssetSettlement) {
		t.Fatalf("initial canonical projection=%+v", first)
	}
	var profileStatus, workerType string
	if err := tx.QueryRowContext(ctx, `SELECT status,worker_type FROM asset_workbench_profiles WHERE user_id = ?`, userID).Scan(&profileStatus, &workerType); err != nil {
		t.Fatal(err)
	}
	if profileStatus != "pending" || workerType != "" {
		t.Fatalf("profile assigned unapproved pricing: %s/%s", profileStatus, workerType)
	}
	if err := users.ReplaceWorkbenchRoles(ctx, wrapped, userID, userID, []domain.Role{domain.RoleAssetSubmitter, domain.RoleAssetSettlement}, &revision, "rollback integration"); err != nil {
		t.Fatal(err)
	}
	second := member()
	if hasRole(second.Roles, domain.RoleAssetManager) || !hasRole(second.Roles, domain.RoleAssetSettlement) || second.PolicyRevision != revision+1 {
		t.Fatalf("reverse role synchronization=%+v", second)
	}
	var preserved int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_user_role_assignments WHERE user_id = ? AND role_id = ?`, userID, designerID).Scan(&preserved); err != nil || preserved != 1 {
		t.Fatalf("non-workbench grant changed: count=%d err=%v", preserved, err)
	}
	if err := users.ReplaceWorkbenchRoles(ctx, wrapped, userID, userID, []domain.Role{domain.RoleAssetManager}, &revision, "stale writer"); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("stale save error=%v", err)
	}
	for _, status := range []string{"disabled", "merged"} {
		exec(`UPDATE app_memberships SET status = ? WHERE app_code='asset_workbench' AND user_id = ?`, status, userID)
		if err := policy.ReplaceUserAssignments(ctx, wrapped, userID, userID, assignments); err != nil {
			t.Fatal(err)
		}
		if got := member().Status; got != status {
			t.Fatalf("role save restored %s to %s", status, got)
		}
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username = ?`, username).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("rollback cleanup count=%d err=%v", remaining, err)
	}
	t.Log("PASS: eager first-login-independent membership/profile, canonical bidirectional roles, unrelated-role preservation, shared revision conflict, disabled/merged protection, fixture rollback")
}
