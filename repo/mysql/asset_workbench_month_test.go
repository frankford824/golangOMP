package mysqlrepo

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/repo"
)

func TestSettlementMonthLockHoldsUntilCommitOrRollback(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "rollback"}[fail], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT CONCAT").WithArgs("2026-10").WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("aw-test:2026-10"))
			mock.ExpectQuery(`SELECT GET_LOCK`).WithArgs("aw-test:2026-10").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(1))
			mock.ExpectBegin()
			if fail {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			mock.ExpectQuery(`SELECT RELEASE_LOCK`).WithArgs("aw-test:2026-10").WillReturnRows(sqlmock.NewRows([]string{"released"}).AddRow(1))
			callbackErr := errors.New("batch appeared")
			err = NewAssetWorkbenchRepo(New(db)).RunSettlementMonthTx(context.Background(), "2026-10", func(repo.Tx) error {
				if fail {
					return callbackErr
				}
				return nil
			})
			if fail && !errors.Is(err, callbackErr) || !fail && err != nil {
				t.Fatalf("err=%v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSettlementMonthLockTimeoutDoesNotStartTransaction(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	mock.ExpectQuery("SELECT CONCAT").WithArgs("2026-10").WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("aw-test:2026-10"))
	mock.ExpectQuery(`SELECT GET_LOCK`).WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(0))
	err := NewAssetWorkbenchRepo(New(db)).RunSettlementMonthTx(context.Background(), "2026-10", func(repo.Tx) error { t.Fatal("callback ran without lock"); return nil })
	if err == nil {
		t.Fatal("expected lock conflict")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSettlementMonthStatesUseWholeMonthNotPayee(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	mock.ExpectQuery(`SELECT business_month, MAX\(locked\) FROM`).WillReturnRows(sqlmock.NewRows([]string{"month", "locked"}).AddRow("2026-07", 0).AddRow("2026-09", 1).AddRow("2026-10", 1))
	states, err := NewAssetWorkbenchRepo(New(db)).ListSettlementMonthStates(context.Background())
	if err != nil || states["2026-07"] || !states["2026-10"] || !states["2026-09"] {
		t.Fatalf("states=%v err=%v", states, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
