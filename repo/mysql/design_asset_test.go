package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/repo"
)

func TestNextAssetNoLocksTaskBeforeAssetRange(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing_task", true: "missing_task"}[missing], func(t *testing.T) {
			raw, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer raw.Close()
			db := New(raw)
			mock.ExpectBegin()
			rows := sqlmock.NewRows([]string{"id"})
			if !missing {
				rows.AddRow(4908)
			}
			mock.ExpectQuery(`SELECT id FROM tasks WHERE id = \? FOR UPDATE`).WithArgs(int64(4908)).WillReturnRows(rows)
			if missing {
				mock.ExpectRollback()
			} else {
				mock.ExpectQuery(`SELECT COALESCE\(MAX\(CASE`).WithArgs(int64(4908)).WillReturnRows(sqlmock.NewRows([]string{"max_sequence"}).AddRow(1047))
				mock.ExpectCommit()
			}
			var number string
			err = db.RunInTx(context.Background(), func(tx repo.Tx) error {
				var err error
				number, err = NewDesignAssetRepo(db).NextAssetNo(context.Background(), tx, 4908)
				return err
			})
			if missing {
				if !errors.Is(err, sql.ErrNoRows) || number != "" {
					t.Fatalf("number=%s err=%v", number, err)
				}
			} else if err != nil || number != "AST-1048" {
				t.Fatalf("number=%s err=%v", number, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNextAssetNoUsesMaximumSuffixInsteadOfRowCount(t *testing.T) {
	raw, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	db := New(raw)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM tasks WHERE id = \? FOR UPDATE`).WithArgs(int64(441)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(441))
	// The task has five rows but AST-0002 is missing and AST-0006 exists.
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(CASE`).WithArgs(int64(441)).WillReturnRows(sqlmock.NewRows([]string{"max_sequence"}).AddRow(6))
	mock.ExpectCommit()
	var number string
	err = db.RunInTx(context.Background(), func(tx repo.Tx) error {
		var allocErr error
		number, allocErr = NewDesignAssetRepo(db).NextAssetNo(context.Background(), tx, 441)
		return allocErr
	})
	if err != nil || number != "AST-0007" {
		t.Fatalf("number=%s err=%v", number, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
