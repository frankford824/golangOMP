package mysqlrepo

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/domain"
)

func TestMaintenanceProjectionUpdatesMatchingPrimaryKeys(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM erp_product_sync_records WHERE task_sku_item_id=?")).WithArgs(int64(14927)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(52).AddRow(53))
	price := 13.068
	mock.ExpectExec(`UPDATE erp_product_sync_records SET cost_price=.*WHERE id IN \(\?,\?\)`).WithArgs(&price, "CGK006299", int64(52), int64(53)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	err = NewCostMaintenanceRepo(New(db)).Project(context.Background(), &MySQLTx{tx: tx}, &domain.TaskSKUItem{ID: 14927, SKUCode: "CGK006299", CostPrice: &price})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
