package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/domain"
	"workflow/repo"
)

func TestWorkbenchSyncUsesEffectiveGrantsAndPreservesProtectedStates(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		if !strings.Contains(actual, expected) {
			return fmt.Errorf("missing %s", expected)
		}
		if expected == "INSERT INTO app_memberships" {
			for _, required := range []string{"auth_user_role_assignments", "auth_org_role_policies", "ar.archived_at IS NULL", "ap.enabled = 1", "u.status = 'active'", "status = IF(app_memberships.status = 'pending', 'active', app_memberships.status)"} {
				if !strings.Contains(actual, required) {
					return fmt.Errorf("missing guard %s", required)
				}
			}
		}
		if expected == "INSERT INTO asset_workbench_profiles" && (strings.Contains(actual, "phone") || strings.Contains(actual, "job_grade") || strings.Contains(actual, "alipay")) {
			return errors.New("must not copy PII or assign grades")
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, _ := db.Begin()
	mock.ExpectExec("INSERT INTO app_memberships").WithArgs(int64(1), int64(331), int64(331)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO asset_workbench_profiles").WithArgs(int64(1), int64(1), int64(331), int64(331)).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := syncWorkbenchMemberships(context.Background(), &MySQLTx{tx: tx}, 331, 1); err != nil {
		t.Fatal(err)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMainOpsAssignmentsAndMembershipAreOneTransaction(t *testing.T) {
	for _, failSync := range []bool{false, true} {
		t.Run(fmt.Sprint(failSync), func(t *testing.T) {
			db, mock, _ := sqlmock.New()
			defer db.Close()
			mdb := New(db)
			policy := NewAccessPolicyRepo(mdb)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT policy_revision").WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(2))
			mock.ExpectExec("DELETE s FROM auth_assignment_scope_subjects").WithArgs(int64(331)).WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec("DELETE FROM auth_user_role_assignments").WithArgs(int64(331)).WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec("INSERT INTO auth_user_role_assignments").WithArgs(int64(331), domain.AccessScopeSelf, int64(1), int64(10)).WillReturnResult(sqlmock.NewResult(4, 1))
			mock.ExpectExec("INSERT INTO app_memberships").WithArgs(int64(1), int64(331), int64(331)).WillReturnResult(sqlmock.NewResult(1, 1))
			profile := mock.ExpectExec("INSERT INTO asset_workbench_profiles").WithArgs(int64(1), int64(1), int64(331), int64(331))
			if failSync {
				profile.WillReturnError(errors.New("profile failed"))
				mock.ExpectRollback()
			} else {
				profile.WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			}
			err := mdb.RunInTx(context.Background(), func(tx repo.Tx) error {
				return policy.ReplaceUserAssignments(context.Background(), tx, 331, 1, []domain.AccessAssignment{{RoleID: 10, ScopeMode: domain.AccessScopeSelf}})
			})
			if (err != nil) != failSync {
				t.Fatalf("error=%v failSync=%v", err, failSync)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkbenchRoleWriteRejectsStaleOrInheritedBeforeWriting(t *testing.T) {
	for _, inherited := range []bool{false, true} {
		t.Run(fmt.Sprint(inherited), func(t *testing.T) {
			db, mock, _ := sqlmock.New()
			defer db.Close()
			mock.ExpectBegin()
			tx, _ := db.Begin()
			mock.ExpectQuery("SELECT policy_revision").WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(8))
			expected := int64(7)
			if inherited {
				expected = 8
				mock.ExpectQuery("SELECT COUNT.*FROM users").WithArgs(int64(331)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			}
			err := (&userRepo{db: New(db)}).ReplaceWorkbenchRoles(context.Background(), &MySQLTx{tx: tx}, 331, 1, []domain.Role{domain.RoleAssetSubmitter}, &expected, "test")
			if err == nil {
				t.Fatal("expected conflict")
			}
			if !inherited && !errors.Is(err, repo.ErrConflict) {
				t.Fatal(err)
			}
			mock.ExpectRollback()
			_ = tx.Rollback()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkbenchMembersIncludeNeverLoggedInGranteesWithoutLegacyRoles(t *testing.T) {
	db, mock, _ := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		if strings.Contains(actual, "user_roles") {
			return errors.New("legacy roles leaked into authority")
		}
		if !strings.Contains(actual, "auth_org_role_policies") || !strings.Contains(actual, "asset_workbench.use") {
			return errors.New("missing effective grants")
		}
		return nil
	})))
	defer db.Close()
	mock.ExpectQuery("count").WithArgs("active").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	now := time.Now()
	mock.ExpectQuery("list").WithArgs("active", 20, 0).WillReturnRows(sqlmock.NewRows([]string{"id", "username", "display_name", "real_name", "worker_type", "job_grade", "status", "pii", "identity", "roles", "created", "updated", "revision", "external"}).AddRow(331, "test", "Test", "Test", "", "", "active", false, "admin", "AssetSubmitter,AssetManager", now, now, 8, false))
	items, total, err := NewAssetWorkbenchRepo(New(db)).ListMembers(context.Background(), repo.AssetWorkbenchMemberFilter{Status: "active", Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(items) != 1 || items[0].PolicyRevision != 8 {
		t.Fatalf("items=%+v total=%d err=%v", items, total, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWorkbenchMembershipReadThroughDoesNotRequireLogin(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	mock.ExpectQuery("FROM app_memberships").WithArgs(domain.AssetWorkbenchAppCode, int64(331)).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT EXISTS.*FROM users").WithArgs(int64(331)).WillReturnRows(sqlmock.NewRows([]string{"granted"}).AddRow(true))
	item, err := NewAssetWorkbenchRepo(New(db)).GetMembership(context.Background(), domain.AssetWorkbenchAppCode, 331)
	if err != nil || item.Status != domain.AppMembershipStatusActive {
		t.Fatalf("item=%+v err=%v", item, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
