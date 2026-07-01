package mysqlrepo

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/repo"
)

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
			want:     "(SELECT MIN(LOWER(f.file_type)) FROM asset_workbench_submission_files f WHERE f.submission_id = s.id) DESC, s.submitted_at DESC, s.id DESC",
		},
		{
			name:     "filename asc alias",
			orderBy:  "filename",
			orderDir: "asc",
			want:     "(SELECT MIN(LOWER(f.original_filename)) FROM asset_workbench_submission_files f WHERE f.submission_id = s.id) ASC, s.submitted_at DESC, s.id DESC",
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
