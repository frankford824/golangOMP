package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"workflow/domain"
	"workflow/repo"
)

func TestProfileIdentityConflictNeverUpdatesAnotherUser(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, field := range []string{"phone", "id_card", "user_id"} {
			t.Run(field+map[bool]string{false: "/insert", true: "/update"}[existing], func(t *testing.T) {
				db, mock, err := sqlmock.New()
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				mock.ExpectBegin()
				lock := mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM asset_workbench_profiles WHERE user_id = ? FOR UPDATE`)).WithArgs(int64(77))
				if existing {
					lock.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(12))
					// Require an explicitly user-bound UPDATE, never ON DUPLICATE KEY UPDATE.
					mock.ExpectExec(`UPDATE asset_workbench_profiles SET.*WHERE user_id = \?`).WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'private-value' for key 'uq_aw_profile_" + field + "'"})
				} else {
					lock.WillReturnError(sql.ErrNoRows)
					mock.ExpectExec(`INSERT INTO asset_workbench_profiles \(.*\) VALUES \([?, ]+\)$`).WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'private-value' for key 'uq_aw_profile_" + field + "'"})
				}
				mock.ExpectRollback()
				mysqlDB := New(db)
				workbench := NewAssetWorkbenchRepo(mysqlDB)
				err = mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
					_, err := workbench.UpsertProfile(context.Background(), tx, &domain.AssetWorkbenchProfile{UserID: 77})
					return err
				})
				var appErr *domain.AppError
				if !errors.As(err, &appErr) || appErr.Code != domain.ErrCodeConflict {
					t.Fatalf("error=%v, want CONFLICT", err)
				}
				if strings.Contains(appErr.Error(), "private-value") {
					t.Fatal("duplicate ownership value leaked to client")
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestProfileInsertAndUpdateStayBoundToUserID(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "insert", true: "update"}[existing], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			lock := mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM asset_workbench_profiles WHERE user_id = ? FOR UPDATE`)).WithArgs(int64(77))
			if existing {
				lock.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(12))
				mock.ExpectExec(`UPDATE asset_workbench_profiles SET.*WHERE user_id = \?`).WithArgs("parttime", "J1", "测试", nil, "", "", nil, "", "", nil, false, "active", false, int64(1), int64(77)).WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				lock.WillReturnError(sql.ErrNoRows)
				mock.ExpectExec(`INSERT INTO asset_workbench_profiles \(.*\) VALUES \([?, ]+\)$`).WithArgs(int64(77), "parttime", "J1", "测试", nil, "", "", nil, "", "", nil, false, "active", false, int64(1), int64(1)).WillReturnResult(sqlmock.NewResult(12, 1))
			}
			now := time.Now()
			mock.ExpectQuery(regexp.QuoteMeta(assetWorkbenchProfileSelect() + ` WHERE user_id = ?`)).WithArgs(int64(77)).WillReturnRows(sqlmock.NewRows([]string{
				"id", "user_id", "worker_type", "job_grade", "real_name", "phone", "province", "city", "id_card", "gender", "alipay_account", "onboarded_at", "grade_hidden", "status", "pii_completed", "created_by", "updated_by", "created_at", "updated_at",
			}).AddRow(12, 77, "parttime", "J1", "测试", nil, "", "", nil, "", "", nil, false, "active", false, 1, 1, now, now))
			mock.ExpectCommit()
			mysqlDB := New(db)
			actorID := int64(1)
			err = mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
				profile, err := NewAssetWorkbenchRepo(mysqlDB).UpsertProfile(context.Background(), tx, &domain.AssetWorkbenchProfile{UserID: 77, WorkerType: "parttime", JobGrade: "J1", RealName: "测试", Status: "active", CreatedBy: &actorID, UpdatedBy: &actorID})
				if err == nil && (profile.UserID != 77 || profile.ID != 12) {
					t.Fatal("saved profile returned wrong user")
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
