package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

type taskSearchDocumentSQL interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

type mysqlSchemaCacheKey struct {
	kind   string
	table  string
	column string
}

type mysqlSchemaCacheEntry struct {
	exists    bool
	expiresAt time.Time
}

const (
	mysqlSchemaPresencePositiveTTL = 10 * time.Minute
	mysqlSchemaPresenceNegativeTTL = 30 * time.Second
)

var mysqlSchemaPresenceCache sync.Map

func taskSearchDocumentsTableExists(ctx context.Context, q taskSearchDocumentSQL) bool {
	return mysqlTableExists(ctx, q, "task_search_documents")
}

func mysqlTableExists(ctx context.Context, q taskSearchDocumentSQL, table string) bool {
	table = strings.TrimSpace(table)
	if table == "" {
		return false
	}
	key := mysqlSchemaCacheKey{kind: "table", table: table}
	if exists, ok := loadMySQLSchemaPresenceCache(key); ok {
		return exists
	}
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	defer cancelQuery()
	var n int
	err := q.QueryRowContext(queryCtx, `
		SELECT COUNT(*)
		  FROM information_schema.tables
		 WHERE table_schema = DATABASE()
		   AND table_name = ?`, table).Scan(&n)
	if err != nil {
		return false
	}
	exists := n > 0
	storeMySQLSchemaPresenceCache(key, exists)
	return exists
}

func mysqlColumnExists(ctx context.Context, q taskSearchDocumentSQL, table, column string) bool {
	exists, err := mysqlColumnExistsChecked(ctx, q, table, column)
	return err == nil && exists
}

func mysqlColumnExistsChecked(ctx context.Context, q taskSearchDocumentSQL, table, column string) (bool, error) {
	table = strings.TrimSpace(table)
	column = strings.TrimSpace(column)
	if table == "" || column == "" {
		return false, nil
	}
	key := mysqlSchemaCacheKey{kind: "column", table: table, column: column}
	if exists, ok := loadMySQLSchemaPresenceCache(key); ok {
		return exists, nil
	}
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	defer cancelQuery()
	var n int
	err := q.QueryRowContext(queryCtx, `
		SELECT COUNT(*)
		  FROM information_schema.columns
		 WHERE table_schema = DATABASE()
		   AND table_name = ?
		   AND column_name = ?`, table, column).Scan(&n)
	if err != nil {
		return false, err
	}
	exists := n > 0
	storeMySQLSchemaPresenceCache(key, exists)
	return exists, nil
}

func loadMySQLSchemaPresenceCache(key mysqlSchemaCacheKey) (bool, bool) {
	cached, ok := mysqlSchemaPresenceCache.Load(key)
	if !ok {
		return false, false
	}
	entry, ok := cached.(mysqlSchemaCacheEntry)
	if !ok {
		mysqlSchemaPresenceCache.Delete(key)
		return false, false
	}
	if time.Now().After(entry.expiresAt) {
		mysqlSchemaPresenceCache.Delete(key)
		return false, false
	}
	return entry.exists, true
}

func storeMySQLSchemaPresenceCache(key mysqlSchemaCacheKey, exists bool) {
	ttl := mysqlSchemaPresenceNegativeTTL
	if exists {
		ttl = mysqlSchemaPresencePositiveTTL
	}
	mysqlSchemaPresenceCache.Store(key, mysqlSchemaCacheEntry{
		exists:    exists,
		expiresAt: time.Now().Add(ttl),
	})
}

func taskAssetsActiveSQL(ctx context.Context, q taskSearchDocumentSQL, alias string) string {
	where, err := taskAssetsActiveSQLChecked(ctx, q, alias)
	if err != nil {
		return "1=0"
	}
	return where
}

func taskAssetsActiveSQLChecked(ctx context.Context, q taskSearchDocumentSQL, alias string) (string, error) {
	prefix := strings.TrimSpace(alias)
	if prefix != "" {
		prefix += "."
	}
	hasDeletedAt, err := mysqlColumnExistsChecked(ctx, q, "task_assets", "deleted_at")
	if err != nil {
		return "", fmt.Errorf("inspect task_assets.deleted_at: %w", err)
	}
	hasCleanedAt, err := mysqlColumnExistsChecked(ctx, q, "task_assets", "cleaned_at")
	if err != nil {
		return "", fmt.Errorf("inspect task_assets.cleaned_at: %w", err)
	}
	switch {
	case hasDeletedAt && hasCleanedAt:
		return fmt.Sprintf("COALESCE(%sdeleted_at, %scleaned_at) IS NULL", prefix, prefix), nil
	case hasDeletedAt:
		return fmt.Sprintf("%sdeleted_at IS NULL", prefix), nil
	case hasCleanedAt:
		return fmt.Sprintf("%scleaned_at IS NULL", prefix), nil
	default:
		return "1=1", nil
	}
}

func reindexTaskSearchDocument(ctx context.Context, q taskSearchDocumentSQL, taskID int64) error {
	if err := reindexTaskSearchDocumentProjection(ctx, q, taskID); err != nil {
		return err
	}
	return enqueueTaskSearchReindex(ctx, q, taskID)
}

// reindexTaskSearchDocumentProjection refreshes the MySQL exact-search
// projection without producing another search_reindex_outbox item. The async
// outbox consumer must use this core helper to avoid recursively resetting the
// row it is currently processing.
func reindexTaskSearchDocumentProjection(ctx context.Context, q taskSearchDocumentSQL, taskID int64) error {
	if taskID <= 0 {
		return nil
	}
	if !taskSearchDocumentsTableExists(ctx, q) {
		return nil
	}
	if _, err := q.ExecContext(ctx, `SET SESSION group_concat_max_len = 1048576`); err != nil {
		return fmt.Errorf("set task search group_concat_max_len: %w", err)
	}
	activeAssetWhere, err := taskAssetsActiveSQLChecked(ctx, q, "")
	if err != nil {
		return err
	}
	return upsertTaskSearchDocumentProjection(ctx, q, taskID, activeAssetWhere)
}

// upsertTaskSearchDocumentProjection is the single canonical task projection
// implementation. Callers that rebuild a batch must set group_concat_max_len
// and resolve activeAssetWhere once before invoking it for each task.
func upsertTaskSearchDocumentProjection(ctx context.Context, q taskSearchDocumentSQL, taskID int64, activeAssetWhere string) error {
	query := strings.Replace(`
			INSERT INTO task_search_documents (
		  task_id, task_no, product_name_snapshot, sku_code, primary_sku_code, product_i_id,
		  task_type, task_status, priority, owner_department, owner_team, owner_org_team,
		  creator_id, creator_name, requester_id, requester_name, designer_id, designer_name,
		  current_handler_id, current_handler_name, created_at, updated_at, deadline_at, asset_text, search_text
		)
		SELECT
		  t.id,
		  t.task_no,
		  COALESCE(t.product_name_snapshot, ''),
		  COALESCE(t.sku_code, ''),
		  COALESCE(t.primary_sku_code, ''),
		  COALESCE(
		    NULLIF(td.category, ''),
		    NULLIF(td.category_name, ''),
		    NULLIF(CASE WHEN JSON_VALID(td.product_selection_snapshot_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.i_id')) ELSE '' END, ''),
		    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.product.i_id')) ELSE '' END, ''),
		    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')) ELSE '' END, ''),
		    ''
		  ),
		  COALESCE(t.task_type, ''),
		  COALESCE(t.task_status, ''),
		  COALESCE(t.priority, ''),
		  COALESCE(t.owner_department, ''),
		  COALESCE(t.owner_team, ''),
		  COALESCE(t.owner_org_team, ''),
		  t.creator_id,
		  COALESCE(NULLIF(creator.display_name, ''), creator.username, ''),
		  t.requester_id,
		  COALESCE(NULLIF(requester.display_name, ''), requester.username, ''),
		  t.designer_id,
		  COALESCE(NULLIF(designer.display_name, ''), designer.username, ''),
		  t.current_handler_id,
		  COALESCE(NULLIF(handler.display_name, ''), handler.username, ''),
		  t.created_at,
		  t.updated_at,
		  t.deadline_at,
		  COALESCE(assets.asset_text, ''),
		  CONCAT_WS(' ',
		    t.id, t.task_no, t.product_name_snapshot, t.sku_code, t.primary_sku_code,
		    t.task_type, t.task_status, t.priority, t.owner_department, t.owner_team, t.owner_org_team,
		    COALESCE(NULLIF(td.category, ''), NULLIF(td.category_name, ''), ''),
		    td.category_code, td.product_short_name, td.demand_text, td.copy_text, td.remark,
		    td.change_request, td.design_requirement, td.material, td.spec_text, td.size_text,
		    td.craft_text, td.process, td.reference_link,
		    COALESCE(NULLIF(creator.display_name, ''), creator.username, ''),
		    COALESCE(NULLIF(requester.display_name, ''), requester.username, ''),
		    COALESCE(NULLIF(designer.display_name, ''), designer.username, ''),
		    COALESCE(NULLIF(handler.display_name, ''), handler.username, ''),
		    DATE_FORMAT(t.created_at, '%Y-%m-%d'), DATE_FORMAT(t.created_at, '%Y%m%d'),
		    DATE_FORMAT(t.deadline_at, '%Y-%m-%d'), COALESCE(assets.asset_text, ''),
		    COALESCE(sku_items.sku_item_text, ''),
		    COALESCE(planning.planning_text, '')
		  )
		FROM tasks t
		LEFT JOIN task_details td ON td.task_id = t.id
		LEFT JOIN users creator ON creator.id = t.creator_id
		LEFT JOIN users requester ON requester.id = t.requester_id
		LEFT JOIN users designer ON designer.id = t.designer_id
		LEFT JOIN users handler ON handler.id = t.current_handler_id
		LEFT JOIN (
			  SELECT task_id, GROUP_CONCAT(CONCAT_WS(' ', scope_sku_code, file_name, original_filename, asset_type, source_module_key) ORDER BY id SEPARATOR ' ') AS asset_text
			  FROM task_assets
			  WHERE task_id = ? AND {{active_asset_where}}
			    AND asset_type NOT IN ('preview', 'design_thumb')
			  GROUP BY task_id
			) assets ON assets.task_id = t.id
		LEFT JOIN (
		  SELECT tsi.task_id,
		         GROUP_CONCAT(CONCAT_WS(' ', tsi.sku_code, tsi.product_name_snapshot,
		           tsi.product_short_name, tsi.design_requirement, tsi.product_i_id,
		           CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.product_i_id')) ELSE '' END)
		           ORDER BY tsi.sequence_no, tsi.id SEPARATOR ' ') AS sku_item_text
		  FROM task_sku_items tsi
		  WHERE tsi.task_id = ?
		  GROUP BY tsi.task_id
		) sku_items ON sku_items.task_id = t.id
		LEFT JOIN (
		  SELECT tsi.task_id,
		         GROUP_CONCAT(CONCAT_WS(' ', tsi.sku_code, revision.description_spec, revision.note,
		           revision.erp_product_i_id, revision.erp_product_name,
		           COALESCE((SELECT latest.status FROM task_erp_outbox latest
		                     WHERE latest.task_sku_item_id = tsi.id
		                     ORDER BY latest.generation DESC, latest.id DESC LIMIT 1), ''))
		           ORDER BY tsi.id, revision.id SEPARATOR ' ') AS planning_text
		  FROM task_sku_items tsi
		  JOIN task_planning_sku_details planning_detail ON planning_detail.task_sku_item_id = tsi.id
		  JOIN task_planning_sku_revisions revision ON revision.id = planning_detail.current_revision_id
		  WHERE tsi.task_id = ?
		  GROUP BY tsi.task_id
		) planning ON planning.task_id = t.id
		WHERE t.id = ?
		ON DUPLICATE KEY UPDATE
		  task_no = VALUES(task_no),
		  product_name_snapshot = VALUES(product_name_snapshot),
		  sku_code = VALUES(sku_code),
		  primary_sku_code = VALUES(primary_sku_code),
		  product_i_id = VALUES(product_i_id),
		  task_type = VALUES(task_type),
		  task_status = VALUES(task_status),
		  priority = VALUES(priority),
		  owner_department = VALUES(owner_department),
		  owner_team = VALUES(owner_team),
		  owner_org_team = VALUES(owner_org_team),
		  creator_id = VALUES(creator_id),
		  creator_name = VALUES(creator_name),
		  requester_id = VALUES(requester_id),
		  requester_name = VALUES(requester_name),
		  designer_id = VALUES(designer_id),
		  designer_name = VALUES(designer_name),
		  current_handler_id = VALUES(current_handler_id),
		  current_handler_name = VALUES(current_handler_name),
		  created_at = VALUES(created_at),
			  updated_at = VALUES(updated_at),
			  deadline_at = VALUES(deadline_at),
			  asset_text = VALUES(asset_text),
			  search_text = VALUES(search_text)`, "{{active_asset_where}}", activeAssetWhere, 1)
	_, err := q.ExecContext(ctx, query,
		taskID,
		taskID,
		taskID,
		taskID,
	)
	if err != nil {
		return fmt.Errorf("reindex task search document: %w", err)
	}
	return nil
}

// TaskSearchProjectionRebuild reports the exact before/after row counts for a
// full task_search_documents rebuild.
type TaskSearchProjectionRebuild struct {
	SourceRows int64
	BeforeRows int64
	AfterRows  int64
	Changed    bool
}

// RebuildAllTaskSearchDocumentProjections atomically rebuilds every task
// document through the same canonical upsert used by incremental writes. It
// deliberately does not enqueue search_reindex_outbox rows: this command is
// rebuilding the source projection itself, not scheduling another rebuild.
func RebuildAllTaskSearchDocumentProjections(ctx context.Context, db *sql.DB, dryRun bool) (TaskSearchProjectionRebuild, error) {
	result := TaskSearchProjectionRebuild{}
	if db == nil {
		return result, fmt.Errorf("task search rebuild database is nil")
	}
	if !taskSearchDocumentsTableExists(ctx, db) {
		return result, fmt.Errorf("task_search_documents table is missing")
	}
	if dryRun {
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks`).Scan(&result.SourceRows); err != nil {
			return result, fmt.Errorf("count task search source rows: %w", err)
		}
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_search_documents`).Scan(&result.BeforeRows); err != nil {
			return result, fmt.Errorf("count task search documents: %w", err)
		}
		return result, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin task search rebuild: %w", err)
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks`).Scan(&result.SourceRows); err != nil {
		return result, fmt.Errorf("count task search source rows: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_search_documents`).Scan(&result.BeforeRows); err != nil {
		return result, fmt.Errorf("count task search documents: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM tasks ORDER BY id`)
	if err != nil {
		return result, fmt.Errorf("list task search source ids: %w", err)
	}
	taskIDs := make([]int64, 0, result.SourceRows)
	for rows.Next() {
		var taskID int64
		if err := rows.Scan(&taskID); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan task search source id: %w", err)
		}
		taskIDs = append(taskIDs, taskID)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close task search source ids: %w", err)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("list task search source ids: %w", err)
	}
	if int64(len(taskIDs)) != result.SourceRows {
		return result, fmt.Errorf("task search source count drifted: counted=%d listed=%d", result.SourceRows, len(taskIDs))
	}
	if _, err := tx.ExecContext(ctx, `SET SESSION group_concat_max_len = 1048576`); err != nil {
		return result, fmt.Errorf("set task search group_concat_max_len: %w", err)
	}
	activeAssetWhere, err := taskAssetsActiveSQLChecked(ctx, tx, "")
	if err != nil {
		return result, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_search_documents`); err != nil {
		return result, fmt.Errorf("delete task search documents: %w", err)
	}
	for _, taskID := range taskIDs {
		if err := upsertTaskSearchDocumentProjection(ctx, tx, taskID, activeAssetWhere); err != nil {
			return result, fmt.Errorf("rebuild task search document %d: %w", taskID, err)
		}
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_search_documents`).Scan(&result.AfterRows); err != nil {
		return result, fmt.Errorf("count rebuilt task search documents: %w", err)
	}
	if result.AfterRows != result.SourceRows {
		return result, fmt.Errorf("task search rebuild row mismatch: source=%d after=%d", result.SourceRows, result.AfterRows)
	}
	var missingOrOrphan int64
	if err := tx.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM tasks t LEFT JOIN task_search_documents d ON d.task_id=t.id WHERE d.task_id IS NULL)
		+ (SELECT COUNT(*) FROM task_search_documents d LEFT JOIN tasks t ON t.id=d.task_id WHERE t.id IS NULL)`).Scan(&missingOrOrphan); err != nil {
		return result, fmt.Errorf("verify rebuilt task search ids: %w", err)
	}
	if missingOrOrphan != 0 {
		return result, fmt.Errorf("task search rebuild id mismatch: missing_or_orphan=%d", missingOrOrphan)
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit task search rebuild: %w", err)
	}
	result.Changed = true
	return result, nil
}

func enqueueTaskSearchReindex(ctx context.Context, q taskSearchDocumentSQL, taskID int64) error {
	if taskID <= 0 {
		return nil
	}
	result, err := q.ExecContext(ctx, `
		INSERT IGNORE INTO search_reindex_outbox (entity_type, entity_id, dedupe_key)
		SELECT 'task', d.task_id,
		       CONCAT('task:', d.task_id, ':', SHA2(CONCAT_WS('|', d.task_no, d.search_text, UNIX_TIMESTAMP(d.updated_at)), 256))
		FROM task_search_documents d
		WHERE d.task_id = ?`, taskID)
	if err != nil {
		return fmt.Errorf("enqueue task search reindex: %w", err)
	}
	if rows, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("read task search reindex enqueue result: %w", err)
	} else if rows > 1 {
		return fmt.Errorf("enqueue task search reindex: unexpected affected rows %d", rows)
	}
	return nil
}

// enqueueTaskSearchReindexForAssetMutation records an asset-version-specific
// refresh without reading the current task_search_documents row. The asset id
// makes the dedupe key monotonic for this mutation, so a stale or missing
// projection cannot suppress the repair job.
func enqueueTaskSearchReindexForAssetMutation(ctx context.Context, q taskSearchDocumentSQL, taskID, taskAssetID int64) error {
	if taskID <= 0 || taskAssetID <= 0 {
		return nil
	}
	dedupeKey := fmt.Sprintf("task:%d:asset:%d", taskID, taskAssetID)
	result, err := q.ExecContext(ctx, `
		INSERT IGNORE INTO search_reindex_outbox (entity_type, entity_id, dedupe_key)
		VALUES ('task', ?, ?)`, taskID, dedupeKey)
	if err != nil {
		return fmt.Errorf("enqueue task search reindex for asset mutation: %w", err)
	}
	if rows, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("read task asset search reindex enqueue result: %w", err)
	} else if rows > 1 {
		return fmt.Errorf("enqueue task search reindex for asset mutation: unexpected affected rows %d", rows)
	}
	return nil
}

func reindexTaskSearchDocuments(ctx context.Context, q taskSearchDocumentSQL, taskIDs []int64) error {
	seen := map[int64]struct{}{}
	for _, taskID := range taskIDs {
		if taskID <= 0 {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		if err := reindexTaskSearchDocument(ctx, q, taskID); err != nil {
			return err
		}
	}
	return nil
}

func taskIDsByAssetID(ctx context.Context, q taskSearchDocumentSQL, assetID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT task_id FROM task_assets WHERE asset_id = ?`, assetID)
	if err != nil {
		return nil, fmt.Errorf("list task ids by asset id: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var taskID int64
		if err := rows.Scan(&taskID); err != nil {
			return nil, fmt.Errorf("scan task id by asset id: %w", err)
		}
		out = append(out, taskID)
	}
	return out, rows.Err()
}

func taskIDByAssetVersionID(ctx context.Context, q taskSearchDocumentSQL, versionID int64) (int64, error) {
	var taskID int64
	err := q.QueryRowContext(ctx, `SELECT task_id FROM task_assets WHERE id = ?`, versionID).Scan(&taskID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get task id by asset version id: %w", err)
	}
	return taskID, nil
}

func assetIDByAssetVersionID(ctx context.Context, q taskSearchDocumentSQL, versionID int64) (int64, error) {
	var assetID sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT asset_id FROM task_assets WHERE id = ?`, versionID).Scan(&assetID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get asset id by asset version id: %w", err)
	}
	if !assetID.Valid {
		return 0, nil
	}
	return assetID.Int64, nil
}

func assetIDsByAssetVersionOrSourceID(ctx context.Context, q taskSearchDocumentSQL, versionID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT DISTINCT asset_id
		  FROM task_assets
		 WHERE (id = ? OR source_asset_version_id = ?)
		   AND asset_id IS NOT NULL`, versionID, versionID)
	if err != nil {
		return nil, fmt.Errorf("list asset ids by asset version/source id: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var assetID int64
		if err := rows.Scan(&assetID); err != nil {
			return nil, fmt.Errorf("scan asset id by asset version/source id: %w", err)
		}
		out = append(out, assetID)
	}
	return out, rows.Err()
}

func reindexAssetSearchDocuments(ctx context.Context, q taskSearchDocumentSQL, assetIDs []int64) error {
	seen := map[int64]struct{}{}
	for _, assetID := range assetIDs {
		if assetID <= 0 {
			continue
		}
		if _, ok := seen[assetID]; ok {
			continue
		}
		seen[assetID] = struct{}{}
		if err := reindexAssetSearchDocument(ctx, q, assetID); err != nil {
			return err
		}
	}
	return nil
}
