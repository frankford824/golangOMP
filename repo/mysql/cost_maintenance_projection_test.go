package mysqlrepo

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/domain"
	"workflow/repo"
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

func TestCostMaintenanceFiltersByExactCodesBeforePaging(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("s.sku_code IN (?, ?) AND (s.task_id<? OR (s.task_id=? AND s.sequence_no<?)) AND COALESCE(cs.status,'baseline')=? ORDER BY s.task_id DESC,s.sequence_no DESC LIMIT ?")).
		WithArgs("DZD000047", "DZD000043", 0, 0, 90, "synced", 3).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, err = NewCostMaintenanceRepo(New(db)).List(context.Background(), repo.CostMaintenanceFilter{
		SKUCodes: []string{"DZD000047", "DZD000043"}, Status: "synced", BeforeID: 90, PageSize: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
