package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestListWarehouseAutoReleaseCandidatesRequiresERPReadySKU(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL != "warehouse-auto-release-candidates" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		required := []string{
			"FROM task_sku_items tsi_ready",
			"FROM task_sku_items tsi_block",
			"tsi_block.filing_status = ?",
			"COALESCE(tsi_block.erp_sync_status, tsi_block.filing_status) = ?",
			"COALESCE(tsi_block.erp_sync_required, 0) = 0",
			"FROM erp_product_sync_records pm_ready",
			"pm_ready.erp_sync_status = ?",
			"FROM erp_product_sync_records pm_block",
			"COALESCE(pm_block.erp_sync_status, '') <> ?",
			"ORDER BY t.updated_at DESC, t.id DESC",
		}
		for _, fragment := range required {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("warehouse auto-release SQL missing %q", fragment)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("warehouse-auto-release-candidates").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1380)))

	repo := &taskAssetRepo{db: New(db)}
	candidates, err := repo.ListWarehouseAutoReleaseCandidates(context.Background(), time.Now(), 10)
	if err != nil {
		t.Fatalf("ListWarehouseAutoReleaseCandidates() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0] != 1380 {
		t.Fatalf("candidates = %+v", candidates)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
