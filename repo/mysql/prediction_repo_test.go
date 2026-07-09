package mysqlrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestPredictionRepoAssetSuggestionsRequireCanonicalAssetID(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "column-exists":
			if !strings.Contains(normalized, "FROM information_schema.columns") {
				return fmt.Errorf("column exists SQL missing information_schema lookup: %s", normalized)
			}
		case "asset-suggestions":
			for _, fragment := range []string{
				"SELECT ta.id AS task_asset_id, ta.asset_id AS asset_id",
				"ta.asset_id IS NOT NULL",
				"ta.id = COALESCE(da.current_version_id",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("asset suggestion SQL missing %q: %s", fragment, normalized)
				}
			}
			if strings.Contains(normalized, "COALESCE(ta.asset_id, ta.id)") {
				return fmt.Errorf("asset suggestion must not use task_asset id as asset target: %s", normalized)
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

	createdAt := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery("column-exists").
		WithArgs("task_assets", "deleted_at").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("column-exists").
		WithArgs("task_assets", "cleaned_at").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("asset-suggestions").
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{
			"task_asset_id", "asset_id", "file_name", "original_filename", "asset_type", "flow_review_status",
			"task_id", "task_no", "product_name_snapshot", "created_at",
		}).AddRow(
			int64(11),
			int64(77),
			"delivery.png",
			"交付图.png",
			"delivery",
			"approved",
			int64(42),
			"RW-42",
			"寿宴手举牌",
			createdAt,
		))

	repo := NewPredictionRepo(New(db))
	suggestions, err := repo.AssetSuggestions(context.Background(), domain.RequestActor{ID: 291}, "", 20)
	if err != nil {
		t.Fatalf("AssetSuggestions() error = %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("suggestions = %d, want 1", len(suggestions))
	}
	got := suggestions[0]
	if got.TargetType != "asset" || got.TargetID != "77" {
		t.Fatalf("target = %s/%s, want asset/77", got.TargetType, got.TargetID)
	}
	if got.Metadata["asset_id"] != "77" || got.Metadata["task_asset_id"] != "11" || got.Metadata["task_id"] != "42" {
		t.Fatalf("metadata = %+v, want canonical asset and task_asset ids", got.Metadata)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPredictionRepoTaskCreateSuggestionsGroupsDerivedColumns(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		if expectedSQL != "task-create-suggestions" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		for _, fragment := range []string{
			"FROM ( SELECT COALESCE(NULLIF(td.category_name, ''), NULLIF(td.category, ''), '未分类') AS category_name",
			"t.created_at AS created_at FROM tasks t LEFT JOIN task_details td ON td.task_id = t.id WHERE",
			") task_create_candidates GROUP BY category_name, category_code, material, spec_text, size_text, process_text, task_type",
		} {
			if !strings.Contains(normalized, fragment) {
				return fmt.Errorf("task-create SQL missing %q: %s", fragment, normalized)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	lastUsed := time.Date(2026, 7, 9, 7, 0, 0, 0, time.UTC)
	mock.ExpectQuery("task-create-suggestions").
		WithArgs(8).
		WillReturnRows(sqlmock.NewRows([]string{
			"category_name", "category_code", "material", "spec_text", "size_text", "process_text", "task_type", "use_count", "last_used_at",
		}).AddRow(
			"常规KT板",
			"KT",
			"KT板",
			"单面",
			"20x30cm",
			"覆膜",
			"normal",
			3,
			lastUsed,
		))

	repo := NewPredictionRepo(New(db))
	suggestions, err := repo.TaskCreateSuggestions(context.Background(), domain.RequestActor{ID: 291}, "", "", 8)
	if err != nil {
		t.Fatalf("TaskCreateSuggestions() error = %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("suggestions = %d, want 1", len(suggestions))
	}
	if suggestions[0].Title != "常规KT板 / KT板 / 20x30cm / 覆膜" {
		t.Fatalf("title = %q", suggestions[0].Title)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
