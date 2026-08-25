package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLSchemaPresenceNegativeCacheExpiresAndReprobes(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	key := mysqlSchemaCacheKey{kind: "table", table: "asset_search_documents"}
	storeMySQLSchemaPresenceCache(key, false)
	if exists, ok := loadMySQLSchemaPresenceCache(key); !ok || exists {
		t.Fatalf("fresh negative cache entry = exists:%v ok:%v", exists, ok)
	}
	mysqlSchemaPresenceCache.Store(key, mysqlSchemaCacheEntry{exists: false, expiresAt: time.Now().Add(-time.Second)})
	if exists, ok := loadMySQLSchemaPresenceCache(key); ok || exists {
		t.Fatalf("expired negative cache entry = exists:%v ok:%v", exists, ok)
	}

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if expectedSQL != "schema-table" {
			return fmt.Errorf("unexpected SQL expectation %q", expectedSQL)
		}
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		if !strings.Contains(normalized, "information_schema.tables") || !strings.Contains(normalized, "table_name = ?") {
			return fmt.Errorf("table schema SQL unexpected: %s", normalized)
		}
		return nil
	})))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mysqlSchemaPresenceCache.Store(key, mysqlSchemaCacheEntry{exists: false, expiresAt: time.Now().Add(-time.Second)})
	mock.ExpectQuery("schema-table").
		WithArgs("asset_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	if !mysqlTableExists(context.Background(), db, "asset_search_documents") {
		t.Fatalf("mysqlTableExists() should re-probe expired negative cache")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestReindexTaskSearchDocumentRefreshesAssetDocumentsForTaskMetadata(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		normalized := strings.Join(strings.Fields(actualSQL), " ")
		switch expectedSQL {
		case "schema-table":
			if !strings.Contains(normalized, "information_schema.tables") || !strings.Contains(normalized, "table_name = ?") {
				return fmt.Errorf("table schema SQL unexpected: %s", normalized)
			}
		case "schema-column":
			if !strings.Contains(normalized, "information_schema.columns") || !strings.Contains(normalized, "column_name = ?") {
				return fmt.Errorf("column schema SQL unexpected: %s", normalized)
			}
		case "set-group-concat":
			if !strings.Contains(normalized, "SET SESSION group_concat_max_len") {
				return fmt.Errorf("group concat SQL unexpected: %s", normalized)
			}
		case "task-doc-upsert":
			for _, fragment := range []string{
				"INSERT INTO task_search_documents",
				"ORDER BY id SEPARATOR ' '",
				"tsi.product_name_snapshot",
				"COALESCE(sku_items.sku_item_text, '')",
				"ORDER BY tsi.id, revision.id SEPARATOR ' '",
				"ON DUPLICATE KEY UPDATE",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("task document SQL missing %q: %s", fragment, normalized)
				}
			}
		case "task-reindex-enqueue":
			for _, fragment := range []string{"INSERT IGNORE INTO search_reindex_outbox", "SHA2", "FROM task_search_documents"} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("task reindex enqueue SQL missing %q: %s", fragment, normalized)
				}
			}
		case "asset-doc-delete-by-task":
			if !strings.Contains(normalized, "DELETE FROM asset_search_documents WHERE task_id = ?") {
				return fmt.Errorf("asset document delete SQL unexpected: %s", normalized)
			}
		case "asset-doc-upsert-by-task":
			for _, fragment := range []string{
				"INSERT INTO asset_search_documents",
				"JOIN tasks t ON t.id = ta.task_id",
				"t.task_no",
				"t.product_name_snapshot",
				"WHERE ta.task_id = ?",
			} {
				if !strings.Contains(normalized, fragment) {
					return fmt.Errorf("asset document SQL missing %q: %s", fragment, normalized)
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

	taskID := int64(42)
	mock.ExpectQuery("schema-table").
		WithArgs("task_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("set-group-concat").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("schema-column").
		WithArgs("task_assets", "deleted_at").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("schema-column").
		WithArgs("task_assets", "cleaned_at").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("task-doc-upsert").
		WithArgs(taskID, taskID, taskID, taskID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("task-reindex-enqueue").
		WithArgs(taskID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := reindexTaskSearchDocument(context.Background(), db, taskID); err != nil {
		t.Fatalf("reindexTaskSearchDocument() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestRebuildAllTaskSearchDocumentProjectionsUsesCanonicalUpsert(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`information_schema\.tables`).WithArgs("task_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM tasks`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM task_search_documents`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT id FROM tasks ORDER BY id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11).AddRow(22))
	mock.ExpectExec(`SET SESSION group_concat_max_len`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`information_schema\.columns`).WithArgs("task_assets", "deleted_at").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`information_schema\.columns`).WithArgs("task_assets", "cleaned_at").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`DELETE FROM task_search_documents`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO task_search_documents[\s\S]+ORDER BY id SEPARATOR ' '[\s\S]+sku_item_text[\s\S]+ORDER BY tsi.id, revision.id SEPARATOR ' '`).
		WithArgs(int64(11), int64(11), int64(11), int64(11)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO task_search_documents[\s\S]+ORDER BY id SEPARATOR ' '[\s\S]+sku_item_text[\s\S]+ORDER BY tsi.id, revision.id SEPARATOR ' '`).
		WithArgs(int64(22), int64(22), int64(22), int64(22)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM task_search_documents`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery(`SELECT[\s\S]+LEFT JOIN task_search_documents[\s\S]+LEFT JOIN tasks`).
		WillReturnRows(sqlmock.NewRows([]string{"missing_or_orphan"}).AddRow(0))
	mock.ExpectCommit()

	result, err := RebuildAllTaskSearchDocumentProjections(context.Background(), db, false)
	if err != nil {
		t.Fatalf("RebuildAllTaskSearchDocumentProjections() error = %v", err)
	}
	if result.SourceRows != 2 || result.BeforeRows != 1 || result.AfterRows != 2 || !result.Changed {
		t.Fatalf("unexpected rebuild result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRebuildAllTaskSearchDocumentProjectionsDryRunDoesNotWrite(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`information_schema\.tables`).WithArgs("task_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM tasks`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM task_search_documents`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	result, err := RebuildAllTaskSearchDocumentProjections(context.Background(), db, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.SourceRows != 3 || result.BeforeRows != 2 || result.AfterRows != 0 || result.Changed {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRebuildAllTaskSearchDocumentProjectionsFailsClosedOnSchemaInspectionError(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`information_schema\.tables`).WithArgs("task_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM tasks`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM task_search_documents`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT id FROM tasks ORDER BY id`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectExec(`SET SESSION group_concat_max_len`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`information_schema\.columns`).WithArgs("task_assets", "deleted_at").
		WillReturnError(errors.New("schema permission denied"))
	mock.ExpectRollback()

	_, err = RebuildAllTaskSearchDocumentProjections(context.Background(), db, false)
	if err == nil || !strings.Contains(err.Error(), "inspect task_assets.deleted_at") {
		t.Fatalf("expected fail-closed schema inspection error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskSearchCanonicalUpsertIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var taskID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM tasks ORDER BY id LIMIT 1`).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `SET SESSION group_concat_max_len = 1048576`); err != nil {
		t.Fatal(err)
	}
	activeAssetWhere := taskAssetsActiveSQL(ctx, tx, "")
	if err := upsertTaskSearchDocumentProjection(ctx, tx, taskID, activeAssetWhere); err != nil {
		t.Fatalf("canonical task projection SQL failed: %v", err)
	}
	var rows int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_search_documents WHERE task_id=?`, taskID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("task search projection rows=%d, want 1", rows)
	}
}

func TestReindexTaskSearchDocumentProjectionDoesNotRecursivelyEnqueue(t *testing.T) {
	mysqlSchemaPresenceCache = sync.Map{}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	taskID := int64(73)
	mock.ExpectQuery(`information_schema\.tables`).
		WithArgs("task_search_documents").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`SET SESSION group_concat_max_len`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`information_schema\.columns`).WithArgs("task_assets", "deleted_at").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`information_schema\.columns`).WithArgs("task_assets", "cleaned_at").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO task_search_documents`).
		WithArgs(taskID, taskID, taskID, taskID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := reindexTaskSearchDocumentProjection(context.Background(), db, taskID); err != nil {
		t.Fatalf("reindexTaskSearchDocumentProjection() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected recursive enqueue: %v", err)
	}
}

func TestEnqueueTaskSearchReindexUsesContentVersionedDedupe(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec(`INSERT IGNORE INTO search_reindex_outbox[\s\S]+SHA2\(CONCAT_WS`).
		WithArgs(int64(91)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := enqueueTaskSearchReindex(context.Background(), db, 91); err != nil {
		t.Fatalf("enqueueTaskSearchReindex() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
