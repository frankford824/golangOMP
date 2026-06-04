package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestProductManagementRefreshReadModelPreservesProductSyncStatus(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL != "product-management-refresh" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		required := []string{
			"WHEN erp_product_sync_records.erp_sync_status = 'synced'",
			"NOT (erp_product_sync_records.cost_price <=> VALUES(cost_price))",
			"THEN 'pending_sync'",
			"ELSE erp_product_sync_records.erp_sync_status",
		}
		for _, fragment := range required {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("refresh SQL missing %q", fragment)
			}
		}
		if strings.Contains(normalized, "IN ('queued', 'failed', 'cooling_down')") {
			return fmt.Errorf("refresh SQL must not preserve stale failed status over product sync success")
		}
		duplicateIndex := strings.Index(normalized, "ON DUPLICATE KEY UPDATE")
		if duplicateIndex < 0 {
			return fmt.Errorf("refresh SQL missing duplicate update")
		}
		duplicateClause := normalized[duplicateIndex:]
		statusIndex := strings.Index(duplicateClause, "erp_sync_status = CASE")
		costIndex := strings.Index(duplicateClause, "cost_price = VALUES(cost_price)")
		if statusIndex < 0 || costIndex < 0 || statusIndex > costIndex {
			return fmt.Errorf("refresh SQL must evaluate sync status before overwriting cost_price")
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec("product-management-refresh").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("product-management-refresh").WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewProductManagementRepo(New(db))
	if err := repo.RefreshReadModel(context.Background()); err != nil {
		t.Fatalf("RefreshReadModel() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
