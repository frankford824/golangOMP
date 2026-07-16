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
	table = strings.TrimSpace(table)
	column = strings.TrimSpace(column)
	if table == "" || column == "" {
		return false
	}
	key := mysqlSchemaCacheKey{kind: "column", table: table, column: column}
	if exists, ok := loadMySQLSchemaPresenceCache(key); ok {
		return exists
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
		return false
	}
	exists := n > 0
	storeMySQLSchemaPresenceCache(key, exists)
	return exists
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
	prefix := strings.TrimSpace(alias)
	if prefix != "" {
		prefix += "."
	}
	hasDeletedAt := mysqlColumnExists(ctx, q, "task_assets", "deleted_at")
	hasCleanedAt := mysqlColumnExists(ctx, q, "task_assets", "cleaned_at")
	switch {
	case hasDeletedAt && hasCleanedAt:
		return fmt.Sprintf("COALESCE(%sdeleted_at, %scleaned_at) IS NULL", prefix, prefix)
	case hasDeletedAt:
		return fmt.Sprintf("%sdeleted_at IS NULL", prefix)
	case hasCleanedAt:
		return fmt.Sprintf("%scleaned_at IS NULL", prefix)
	default:
		return "1=1"
	}
}

func reindexTaskSearchDocument(ctx context.Context, q taskSearchDocumentSQL, taskID int64) error {
	if taskID <= 0 {
		return nil
	}
	if !taskSearchDocumentsTableExists(ctx, q) {
		return nil
	}
	if _, err := q.ExecContext(ctx, `SET SESSION group_concat_max_len = 1048576`); err != nil {
		return fmt.Errorf("set task search group_concat_max_len: %w", err)
	}
	activeAssetWhere := taskAssetsActiveSQL(ctx, q, "")
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
		    COALESCE(planning.planning_text, '')
		  )
		FROM tasks t
		LEFT JOIN task_details td ON td.task_id = t.id
		LEFT JOIN users creator ON creator.id = t.creator_id
		LEFT JOIN users requester ON requester.id = t.requester_id
		LEFT JOIN users designer ON designer.id = t.designer_id
		LEFT JOIN users handler ON handler.id = t.current_handler_id
		LEFT JOIN (
			  SELECT task_id, GROUP_CONCAT(CONCAT_WS(' ', file_name, original_filename, storage_key, source_module_key) SEPARATOR ' ') AS asset_text
			  FROM task_assets
			  WHERE task_id = ? AND {{active_asset_where}}
			  GROUP BY task_id
			) assets ON assets.task_id = t.id
		LEFT JOIN (
		  SELECT tsi.task_id,
		         GROUP_CONCAT(CONCAT_WS(' ', tsi.sku_code, revision.description_spec, revision.note,
		           revision.erp_product_i_id, revision.erp_product_name,
		           COALESCE((SELECT latest.status FROM task_erp_outbox latest
		                     WHERE latest.task_sku_item_id = tsi.id
		                     ORDER BY latest.generation DESC, latest.id DESC LIMIT 1), '')) SEPARATOR ' ') AS planning_text
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
	)
	if err != nil {
		return fmt.Errorf("reindex task search document: %w", err)
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
