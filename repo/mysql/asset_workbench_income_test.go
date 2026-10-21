package mysqlrepo

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"workflow/domain"
	"workflow/repo"
)

func TestPersonalIncomeSnapshotIsOwnerScopedUnpaginatedAndIncludesGenerated(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`FROM asset_workbench_submission_items WHERE payee_user_id = \? AND entry_kind = \? AND settlement_status = \? AND current_settlement_batch_id IS NULL AND qc_status <> \? ORDER BY id$`).WithArgs(77, "normal", "unsettled", "voided").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`FROM asset_workbench_settlement_supplements WHERE payee_user_id = \? ORDER BY business_month DESC, id DESC$`).WithArgs(77).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`FROM asset_workbench_error_records WHERE payee_user_id = \? ORDER BY id$`).WithArgs(77).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`WHERE si.payee_user_id = \? AND sb.status IN \(\?, \?\) ORDER BY si.id$`).WithArgs(77, "generated", "confirmed").WillReturnRows(sqlmock.NewRows([]string{"id", "batch_id", "item_type", "submission_item_id", "payee_user_id", "paid_to_user_id", "business_month", "amount", "quantity", "unit_price", "direction", "source_ref_type", "source_ref_id", "snapshot_json", "payout_snapshot_json", "created_at", "status"}).AddRow(1, 2, "gross_piecework", 3, 77, nil, "2026-09", 10, 1, 10, "credit", "submission_item", 3, nil, nil, time.Now(), "generated"))
	mock.ExpectCommit()
	got, err := NewAssetWorkbenchRepo(New(db)).ReadPersonalIncome(context.Background(), 77)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Ledger) != 1 || got.Ledger[0].BatchStatus != "generated" || got.Ledger[0].Item.Amount != 10 {
		t.Fatalf("snapshot=%+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVisibilityUpdateIsOwnerLockedAndDoesNotWriteMoney(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{true: "missing", false: "saved"}[missing], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			lock := mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM asset_workbench_profiles WHERE user_id = ? FOR UPDATE`)).WithArgs(77)
			if missing {
				lock.WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
			} else {
				lock.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(8))
				mock.ExpectExec(regexp.QuoteMeta(`UPDATE asset_workbench_profiles SET settlement_visible = ?, updated_by = ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?`)).WithArgs(false, 1, 77).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			mysqlDB := New(db)
			err = mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
				return NewAssetWorkbenchRepo(mysqlDB).SetSettlementVisibility(context.Background(), tx, 77, false, 1)
			})
			if missing && err != sql.ErrNoRows {
				t.Fatalf("err=%v", err)
			}
			if !missing && err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReservedSourcesExcludeCancelledBatches(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`WHERE business_month = \? AND item_type IN \(\?, \?\) AND batch_id IN \(SELECT id FROM asset_workbench_settlement_batches WHERE status IN \(\?, \?\)\) ORDER BY id$`).WithArgs("2026-10", domain.AssetWorkbenchItemTypeErrorDeduction, domain.AssetWorkbenchItemTypeWelfare, "generated", "confirmed").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, err = NewAssetWorkbenchRepo(New(db)).ListReservedSettlementSources(context.Background(), nil, "2026-10")
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
