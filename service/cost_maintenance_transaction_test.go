package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"workflow/domain"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
)

func TestCostMaintenanceTransactionOnlyReplaysRolledBackDeadlocks(t *testing.T) {
	for _, test := range []struct {
		name               string
		number             uint16
		failures, attempts int
	}{
		{"deadlock recovery", 1213, 2, 3}, {"bounded recovery", 1213, 9, 4}, {"lock timeout is not a deadlock", 1205, 1, 1}, {"duplicate is not a deadlock", 1062, 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for i := 0; i < test.attempts; i++ {
				mock.ExpectBegin()
				if i < test.failures {
					mock.ExpectRollback()
				} else {
					mock.ExpectCommit()
				}
			}
			svc := &taskService{txRunner: mysqlrepo.New(db)}
			calls, effects := 0, 0
			err = svc.runCostMaintenanceTransaction(context.Background(), 14927, func(tx repo.Tx) error {
				calls++
				tx.(*mysqlrepo.MySQLTx).AfterCommit(func() { effects++ })
				if calls <= test.failures {
					return fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: test.number})
				}
				return nil
			})
			if calls != test.attempts {
				t.Fatal("unexpected retries", calls, err)
			}
			if test.failures < test.attempts {
				if err != nil || effects != 1 {
					t.Fatalf("success/effects=%v/%d", err, effects)
				}
			} else if err == nil || effects != 0 {
				t.Fatalf("failed transaction committed effects: %v %d", err, effects)
			}
			if test.number == 1213 && test.failures > test.attempts {
				var app *domain.AppError
				if !errors.As(err, &app) || app.Code != domain.ErrCodeConflict {
					t.Fatal("exhaustion did not produce an explicit uncommitted conflict", err)
				}
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCostMaintenanceDoesNotReplayAmbiguousCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(errors.New("network lost during commit"))
	svc := &taskService{txRunner: mysqlrepo.New(db)}
	calls := 0
	err = svc.runCostMaintenanceTransaction(context.Background(), 1, func(repo.Tx) error { calls++; return nil })
	if err == nil || calls != 1 {
		t.Fatal("ambiguous commit was replayed", calls, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
