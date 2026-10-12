package service

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/domain"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
)

func TestWorkbenchRegistrationProvisionFailureRollsBackIdentityAndRoles(t *testing.T) {
	for _, stage := range []string{"profile", "membership"} {
		t.Run(stage, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mysql := mysqlrepo.New(db)
			sessions := &identitySessionRepoStub{}
			svc := NewIdentityService(mysqlrepo.NewUserRepo(mysql), sessions, &identityPermissionLogRepoStub{}, mysql,
				WithIdentityAccessAssignmentWriter(&identityAccessAssignmentWriterStub{}))
			mock.ExpectQuery("FROM users WHERE LOWER\\(username\\)").WillReturnRows(sqlmock.NewRows([]string{"id"}))
			mock.ExpectQuery("FROM users WHERE mobile").WillReturnRows(sqlmock.NewRows([]string{"id"}))
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO users").WillReturnResult(sqlmock.NewResult(77, 1))
			mock.ExpectExec("DELETE FROM user_roles").WithArgs(int64(77)).WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec("INSERT INTO user_roles").WillReturnResult(sqlmock.NewResult(0, 1))
			if stage == "membership" {
				mock.ExpectExec("INSERT INTO registration_profile_probe").WillReturnResult(sqlmock.NewResult(1, 1))
			}
			mock.ExpectExec("INSERT INTO registration_" + stage + "_probe").WillReturnError(errors.New("provision failure"))
			mock.ExpectRollback()
			result, appErr := svc.RegisterAssetWorkbenchUser(context.Background(), RegisterAssetWorkbenchUserParams{
				Username: "atomic-worker", DisplayName: "注册测试", Mobile: "13800000991", Password: "Pass1234",
				EmploymentType: domain.EmploymentTypeFullTime,
				Provision: func(ctx context.Context, tx repo.Tx, user *domain.User) error {
					if user.EmploymentType != domain.EmploymentTypeFullTime {
						t.Fatal("fulltime registration lost employment type")
					}
					if stage == "membership" {
						if _, err := mysqlrepo.Unwrap(tx).ExecContext(ctx, "INSERT INTO registration_profile_probe VALUES (?)", user.ID); err != nil {
							return err
						}
					}
					_, err := mysqlrepo.Unwrap(tx).ExecContext(ctx, "INSERT INTO registration_"+stage+"_probe VALUES (?)", user.ID)
					return err
				},
			})
			if result != nil || appErr == nil {
				t.Fatalf("result=%v error=%v, want rollback", result, appErr)
			}
			if len(sessions.sessions) != 0 {
				t.Fatal("failed registration issued a session")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
