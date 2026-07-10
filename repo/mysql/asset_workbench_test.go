package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
	"workflow/repo"
)

func TestGetUploadSessionForUpdateUsesRowLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(assetWorkbenchUploadSessionSelect() + ` WHERE session_id = ? FOR UPDATE`)).
		WithArgs("upload-session-1").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()

	mysqlDB := New(db)
	workbenchRepo := NewAssetWorkbenchRepo(mysqlDB)
	if err := mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		item, err := workbenchRepo.GetUploadSessionForUpdate(context.Background(), tx, "upload-session-1")
		if item != nil || !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("GetUploadSessionForUpdate() = (%+v, %v), want (nil, sql.ErrNoRows)", item, err)
		}
		return nil
	}); err != nil {
		t.Fatalf("RunInTx() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestLockPriceMatrixDimensionLocksParentDimensionBeforeExistingRules(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO asset_workbench_price_matrix_dimensions (
			worker_type, job_grade, difficulty_class
		) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE updated_at = updated_at`)).
		WithArgs("parttime", "J1", "A").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id
		FROM asset_workbench_price_matrix_dimensions
		WHERE worker_type = ? AND job_grade = ? AND difficulty_class = ?
		FOR UPDATE`)).
		WithArgs("parttime", "J1", "A").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta(assetWorkbenchPriceMatrixSelect()+`
		WHERE worker_type = ? AND job_grade = ? AND difficulty_class = ?
		ORDER BY effective_from ASC, id ASC
		FOR UPDATE`)).
		WithArgs("parttime", "J1", "A").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "worker_type", "job_grade", "difficulty_class", "unit_price", "effective_from", "effective_to", "enabled",
			"revision_no", "created_by", "remark", "created_at", "updated_at",
		}))
	mock.ExpectCommit()

	mysqlDB := New(db)
	workbenchRepo := NewAssetWorkbenchRepo(mysqlDB)
	if err := mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		items, err := workbenchRepo.LockPriceMatrixDimension(context.Background(), tx, "parttime", "J1", "A")
		if err != nil {
			return err
		}
		if len(items) != 0 {
			t.Fatalf("items = %+v, want empty", items)
		}
		return nil
	}); err != nil {
		t.Fatalf("LockPriceMatrixDimension() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestLockSettleableItemsExcludesSupplementSubmissionItems(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(assetWorkbenchSubmissionItemSelect()+`
		WHERE business_month = ?
		  AND entry_kind = ?
		  AND pricing_status = ?
		  AND qc_status IN (?, ?)
		  AND settlement_status = ?
		  AND current_settlement_batch_id IS NULL
		ORDER BY payee_user_id ASC, id ASC
		FOR UPDATE`)).
		WithArgs(
			"2026-07",
			domain.AssetWorkbenchSubmissionEntryKindNormal,
			domain.AssetWorkbenchPricingStatusPriced,
			domain.AssetWorkbenchSubmissionStatusSubmitted,
			domain.AssetWorkbenchSubmissionStatusChecked,
			domain.AssetWorkbenchSettlementStatusUnsettled,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectCommit()

	mysqlDB := New(db)
	workbenchRepo := NewAssetWorkbenchRepo(mysqlDB)
	if err := mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		items, err := workbenchRepo.LockSettleableItems(context.Background(), tx, "2026-07")
		if err != nil {
			return err
		}
		if len(items) != 0 {
			t.Fatalf("items = %+v, want empty", items)
		}
		return nil
	}); err != nil {
		t.Fatalf("LockSettleableItems() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestSetPriceMatrixEffectiveToUpdatesDate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	closed := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE asset_workbench_price_matrix
		SET effective_to = ?, updated_at = NOW()
		WHERE id = ?`)).
		WithArgs(sql.NullTime{Time: closed, Valid: true}, int64(101)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(assetWorkbenchPriceMatrixSelect() + ` WHERE id = ?`)).
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "worker_type", "job_grade", "difficulty_class", "unit_price", "effective_from", "effective_to", "enabled",
			"revision_no", "created_by", "remark", "created_at", "updated_at",
		}).AddRow(int64(101), "parttime", "J1", "A", 12.5, now, closed, true, 1, int64(7), "", now, now))
	mock.ExpectCommit()

	mysqlDB := New(db)
	workbenchRepo := NewAssetWorkbenchRepo(mysqlDB)
	if err := mysqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		item, err := workbenchRepo.SetPriceMatrixEffectiveTo(context.Background(), tx, 101, &closed)
		if err != nil {
			return err
		}
		if item.EffectiveTo == nil || !item.EffectiveTo.Equal(closed) {
			t.Fatalf("EffectiveTo = %+v, want %s", item.EffectiveTo, closed)
		}
		return nil
	}); err != nil {
		t.Fatalf("SetPriceMatrixEffectiveTo() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestAssetWorkbenchSubmissionOrderBySupportsFileAndTimeSorts(t *testing.T) {
	tests := []struct {
		name     string
		orderBy  string
		orderDir string
		want     string
	}{
		{
			name:     "file type desc",
			orderBy:  "file_type",
			orderDir: "desc",
			want:     "(SELECT MIN(LOWER(f.file_type)) FROM asset_workbench_submission_files f WHERE f.submission_id = s.id AND f.deleted_at IS NULL) DESC, s.submitted_at DESC, s.id DESC",
		},
		{
			name:     "filename asc alias",
			orderBy:  "filename",
			orderDir: "asc",
			want:     "(SELECT MIN(LOWER(COALESCE(NULLIF(f.display_name, ''), f.original_filename))) FROM asset_workbench_submission_files f WHERE f.submission_id = s.id AND f.deleted_at IS NULL) ASC, s.submitted_at DESC, s.id DESC",
		},
		{
			name:     "created at asc",
			orderBy:  "created_at",
			orderDir: "asc",
			want:     "s.created_at ASC, s.id ASC",
		},
		{
			name:     "default",
			orderBy:  "unknown",
			orderDir: "asc",
			want:     "s.submitted_at DESC, s.id DESC",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := assetWorkbenchSubmissionOrderBy(tt.orderBy, tt.orderDir); got != tt.want {
				t.Fatalf("assetWorkbenchSubmissionOrderBy(%q, %q) = %q, want %q", tt.orderBy, tt.orderDir, got, tt.want)
			}
		})
	}
}

func TestListEventsPagesIDsBeforeHydratingLightweightRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 10, 11, 42, 37, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM asset_workbench_events e WHERE 1=1`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT e.id FROM asset_workbench_events e WHERE 1=1
		ORDER BY e.id DESC
		LIMIT ? OFFSET ?`)).
		WithArgs(50, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(3902)).AddRow(int64(3901)))
	mock.ExpectQuery(regexp.QuoteMeta(assetWorkbenchEventListSelect()+`
		WHERE e.id IN (?,?)
		ORDER BY FIELD(e.id, ?,?)`)).
		WithArgs(int64(3902), int64(3901), int64(3902), int64(3901)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "actor_user_id", "event_type", "entity_type", "entity_id",
			"before_json", "after_json", "reason", "created_at", "actor_username", "actor_display_name",
		}).
			AddRow(int64(3902), int64(302), domain.AssetWorkbenchEventFileMoved, domain.AssetWorkbenchEntitySubmissionFile, int64(564), nil, nil, "移动到 A 类", now, "test-admin", "测试管理员").
			AddRow(int64(3901), int64(302), domain.AssetWorkbenchEventItemRepriced, domain.AssetWorkbenchEntitySubmissionItem, int64(450), nil, nil, "移动后重新计价", now.Add(-time.Second), "test-admin", "测试管理员"))

	workbenchRepo := NewAssetWorkbenchRepo(New(db))
	items, total, err := workbenchRepo.ListEvents(context.Background(), repo.AssetWorkbenchEventFilter{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("ListEvents() total=%d items=%d, want 2", total, len(items))
	}
	if items[0].ID != 3902 || items[0].ActorDisplayName != "测试管理员" || len(items[0].Before) != 0 || len(items[0].After) != 0 {
		t.Fatalf("first event = %+v, want lightweight moved event with actor", items[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
