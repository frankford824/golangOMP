package mysqlrepo

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestDecodeTaskTypesJSONFailsClosed(t *testing.T) {
	for _, raw := range [][]byte{[]byte(`{"broken"`), []byte(`["not_a_task_type"]`)} {
		if values, err := decodeTaskTypesJSON(raw); err == nil {
			t.Fatalf("decodeTaskTypesJSON(%q) = %#v, nil; want error", raw, values)
		}
	}
}

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
	mock.ExpectQuery("SELECT rp.role_id").WithArgs(int64(20)).WillReturnRows(sqlmock.NewRows([]string{"role_id", "permission_code", "task_types"}).AddRow(int64(20), string(domain.PermissionAssetView), nil))

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

func TestEffectiveAccessManyHydratesUsersInOneProjectionQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewAccessPolicyRepo(New(db))
	mock.ExpectQuery("SELECT policy_revision").WillReturnRows(sqlmock.NewRows([]string{"policy_revision"}).AddRow(int64(9)))
	columns := []string{
		"user_id", "assignment_id", "role_id", "role_code", "role_name", "scope_mode", "source_type", "source_ref_id", "version",
		"subject_type", "subject_id", "subject_name", "permission_code", "task_types",
	}
	mock.ExpectQuery("WITH effective_assignments").WithArgs(
		int64(7), int64(8), int64(7), int64(8), int64(7), int64(8),
	).WillReturnRows(sqlmock.NewRows(columns).
		AddRow(int64(7), int64(21), int64(5), "designer", "Designer", "selected_org", "direct", int64(0), int64(2), "department", int64(101), "Design", string(domain.PermissionTaskView), []byte(`["new_product_development"]`)).
		AddRow(int64(7), int64(21), int64(5), "designer", "Designer", "selected_org", "direct", int64(0), int64(2), "team", int64(202), "A Team", string(domain.PermissionTaskView), []byte(`["new_product_development"]`)).
		AddRow(int64(8), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))

	views, err := repository.EffectiveAccessMany(context.Background(), []int64{8, 7, 7})
	if err != nil {
		t.Fatalf("EffectiveAccessMany() error = %v", err)
	}
	if len(views) != 2 || views[7] == nil || views[8] == nil {
		t.Fatalf("views = %+v", views)
	}
	if len(views[7].Assignments) != 1 || len(views[7].Assignments[0].Subjects) != 2 {
		t.Fatalf("user 7 assignments = %+v", views[7].Assignments)
	}
	if len(views[7].Permissions) != 1 || views[7].Permissions[0] != domain.PermissionTaskView || len(views[7].Sources) != 1 {
		t.Fatalf("user 7 permissions/sources = %+v/%+v", views[7].Permissions, views[7].Sources)
	}
	if views[8].PolicyRevision != 9 || len(views[8].Permissions) != 0 || len(views[8].Assignments) != 0 {
		t.Fatalf("user 8 effective access = %+v", views[8])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
