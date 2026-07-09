package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
	"workflow/repo"
)

func TestSKUTraceAppendCostSnapshotPreservesMoreSpecificLatestSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "insert-cost-snapshot":
			if !strings.Contains(normalized, "INSERT INTO omp_sku_cost_snapshots") {
				return fmt.Errorf("cost snapshot insert SQL unexpected: %s", normalized)
			}
		case "sync-cost-snapshot":
			for _, fragment := range []string{
				"JOIN omp_sku_cost_snapshots cost_snapshot ON cost_snapshot.id = ?",
				"LEFT JOIN omp_sku_cost_snapshots current_snapshot ON current_snapshot.id = pm.latest_cost_snapshot_id",
				"pm.latest_cost_snapshot_id IS NULL",
				"current_snapshot.id IS NULL",
				"WHEN pm.task_sku_item_id IS NOT NULL AND cost_snapshot.task_sku_item_id = pm.task_sku_item_id THEN 0",
				"WHEN pm.task_sku_item_id IS NOT NULL AND current_snapshot.task_sku_item_id = pm.task_sku_item_id THEN 0",
				"cost_snapshot.created_at > current_snapshot.created_at",
				"cost_snapshot.created_at = current_snapshot.created_at AND cost_snapshot.id > current_snapshot.id",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("cost snapshot sync SQL missing %q: %s", fragment, normalized)
				}
			}
		default:
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("insert-cost-snapshot").WillReturnResult(sqlmock.NewResult(99, 1))
	mock.ExpectExec("sync-cost-snapshot").WithArgs(int64(99)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mysqlDB := New(db)
	traceRepo := NewSKUTraceRepo(mysqlDB)
	err = mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		_, err := traceRepo.AppendCostSnapshot(context.Background(), tx, &domain.OMPSKUCostSnapshot{
			SKUCode: "SKU001",
			SKUKind: domain.OMPSKUKindOrdinary,
		})
		return err
	})
	if err != nil {
		t.Fatalf("AppendCostSnapshot() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
