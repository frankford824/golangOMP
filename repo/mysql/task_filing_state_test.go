package mysqlrepo

import (
	"context"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"workflow/domain"
)

func TestFilingAckDoesNotOverwriteCostOrDimensions(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_, q string) error {
		if strings.Contains(q, "cost_price") || strings.Contains(q, "width") || strings.Contains(q, "manual_cost_override") {
			return fmt.Errorf("filing ACK contains stale business fields")
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, _ := db.Begin()
	mock.ExpectExec("filing-only").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	r := &taskRepo{db: &DB{db: db}}
	if err := r.UpdateDetailFilingOnly(context.Background(), &MySQLTx{tx: tx}, &domain.TaskDetail{TaskID: 1, FilingStatus: domain.FilingStatusFiled}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
