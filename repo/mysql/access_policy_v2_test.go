package mysqlrepo

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestEnsureExplicitRoleAssignmentUsesStableRoleCodeAndCallerTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewAccessPolicyRepo(New(db))
	mock.ExpectBegin()
	sqlTx, err := db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tx := &MySQLTx{tx: sqlTx}
	mock.ExpectQuery("SELECT id\\s+FROM auth_roles").WithArgs("member").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(8)))
	mock.ExpectExec("INSERT INTO auth_user_role_assignments").WithArgs(int64(42), int64(8), domain.AccessScopeSelf).WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repository.EnsureExplicitRoleAssignment(context.Background(), tx, 42, "member", domain.AccessScopeSelf); err != nil {
		t.Fatalf("EnsureExplicitRoleAssignment() error = %v", err)
	}
	mock.ExpectRollback()
	if err := sqlTx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEffectiveAccessInjectsOrgPolicySelectedSubject(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewAccessPolicyRepo(New(db))
	mock.ExpectQuery("SELECT policy_revision").WillReturnRows(sqlmock.NewRows([]string{"policy_revision"}).AddRow(int64(4)))
	mock.ExpectQuery("SELECT a.id").WithArgs(int64(7), int64(7)).WillReturnRows(sqlmock.NewRows([]string{
		"id", "user_id", "role_id", "role_code", "role_name", "scope_mode", "source_type", "source_ref_id", "version", "policy_subject_type", "policy_subject_id",
	}).AddRow(int64(0), int64(7), int64(20), "department_designer", "Department designer", "selected_org", "org_policy", int64(30), int64(1), "department", int64(101)))
	mock.ExpectQuery("SELECT rp.role_id").WithArgs(int64(20)).WillReturnRows(sqlmock.NewRows([]string{"role_id", "permission_code"}).AddRow(int64(20), string(domain.PermissionAssetView)))

	view, err := repository.EffectiveAccess(context.Background(), 7)
	if err != nil {
		t.Fatalf("EffectiveAccess() error = %v", err)
	}
	if len(view.Assignments) != 1 || len(view.Assignments[0].Subjects) != 1 {
		t.Fatalf("assignments = %+v", view.Assignments)
	}
	subject := view.Assignments[0].Subjects[0]
	if subject.SubjectType != domain.AccessSubjectDepartment || subject.SubjectID != 101 {
		t.Fatalf("selected org subject = %+v", subject)
	}
	departmentID := int64(101)
	actor := domain.RequestActor{ID: 7, DepartmentID: &departmentID, Permissions: view.Permissions, EffectiveAccess: view}
	if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetView, domain.TaskAccessSubject{TaskID: 1, CreatorID: 8, OwnerDepartmentID: &departmentID}) {
		t.Fatal("org-policy selected_org did not authorize its own department")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
