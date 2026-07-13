package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type taskAssetLifecycleRepo struct{ db *DB }

func NewTaskAssetLifecycleRepo(db *DB) repo.TaskAssetLifecycleRepo {
	return &taskAssetLifecycleRepo{db: db}
}

// ListResourceDeletionStorageKeys returns both the resource's own objects and
// backend-derived preview/thumb objects. Deleting only the parent resource
// would otherwise leave the old preview bytes addressable in object storage.
func (r *taskAssetLifecycleRepo) ListResourceDeletionStorageKeys(ctx context.Context, assetID int64) ([]string, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT DISTINCT COALESCE(ta.storage_key, '')
		  FROM task_assets ta
		  JOIN design_assets da ON da.id = ta.asset_id
		 WHERE (da.id = ? OR da.source_asset_id = ?)
		   AND ta.deleted_at IS NULL
		   AND COALESCE(ta.storage_key, '') <> ''
		 ORDER BY COALESCE(ta.storage_key, '')`, assetID, assetID)
	if err != nil {
		return nil, fmt.Errorf("list resource deletion storage keys: %w", err)
	}
	defer rows.Close()
	keys := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan resource deletion storage key: %w", err)
		}
		if key = strings.TrimSpace(key); key != "" {
			keys = append(keys, key)
		}
	}
	return keys, rows.Err()
}

func (r *taskAssetLifecycleRepo) ListSupersededEligibleForCleanup(ctx context.Context, cutoff time.Time, limit int) ([]*repo.TaskAssetCleanupCandidate, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT ta.asset_id, ta.id, ta.task_id, ta.source_task_module_id, COALESCE(ta.storage_key, ''), ta.source_module_key, t.updated_at,
		       COALESCE(GROUP_CONCAT(NULLIF(derived.storage_key, '') SEPARATOR '\n'), '') AS related_storage_keys
		  FROM task_assets ta
		  JOIN tasks t ON t.id = ta.task_id
		  LEFT JOIN task_assets derived
		    ON derived.source_asset_version_id = ta.id
		   AND derived.cleaned_at IS NULL
		   AND derived.deleted_at IS NULL
		   AND COALESCE(derived.storage_key, '') <> ''
		 WHERE ta.deleted_at IS NULL
		   AND ta.cleaned_at IS NULL
		   AND COALESCE(ta.storage_key, '') <> ''
		   AND ta.flow_review_status = ?
		   AND ta.cleanup_after_at IS NOT NULL
		   AND ta.cleanup_after_at <= ?
		   AND ta.id <> COALESCE((
		        SELECT da.current_version_id FROM design_assets da WHERE da.id = ta.asset_id
		   ), 0)
		   AND EXISTS (
		        SELECT 1 FROM task_assets newer
		         WHERE newer.id = ta.superseded_by_version_id
		           AND newer.deleted_at IS NULL
		           AND newer.cleaned_at IS NULL
		   )
		 GROUP BY ta.asset_id, ta.id, ta.task_id, ta.source_task_module_id, ta.storage_key, ta.source_module_key, t.updated_at
		 ORDER BY ta.cleanup_after_at ASC, ta.id ASC
		 LIMIT ?`,
		string(domain.TaskAssetFlowReviewStatusSuperseded), cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list superseded cleanup candidates: %w", err)
	}
	defer rows.Close()
	var out []*repo.TaskAssetCleanupCandidate
	for rows.Next() {
		var c repo.TaskAssetCleanupCandidate
		var assetID, moduleID sql.NullInt64
		var related string
		if err := rows.Scan(&assetID, &c.VersionID, &c.TaskID, &moduleID, &c.StorageKey, &c.SourceModuleKey, &c.TaskUpdatedAt, &related); err != nil {
			return nil, fmt.Errorf("scan superseded cleanup candidate: %w", err)
		}
		if assetID.Valid {
			c.AssetID = assetID.Int64
		}
		c.SourceTaskModuleID = fromNullInt64(moduleID)
		c.RelatedStorageKeys = splitCleanupStorageKeys(related)
		c.CleanupReason = "superseded"
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (r *taskAssetLifecycleRepo) MarkSupersededAutoCleaned(ctx context.Context, tx repo.Tx, versionID int64, cleanedAt time.Time) error {
	sqlTx := Unwrap(tx)
	taskID, err := taskIDByAssetVersionID(ctx, sqlTx, versionID)
	if err != nil {
		return err
	}
	assetIDs, err := assetIDsByAssetVersionOrSourceID(ctx, sqlTx, versionID)
	if err != nil {
		return err
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE task_assets
		   SET is_archived = 1,
		       cleaned_at = ?,
		       storage_key = NULL,
		       flow_review_status = ?
		 WHERE (id = ? OR source_asset_version_id = ?)
		   AND cleaned_at IS NULL
		   AND deleted_at IS NULL`,
		cleanedAt, string(domain.TaskAssetFlowReviewStatusCleaned), versionID, versionID)
	if err != nil {
		return fmt.Errorf("mark superseded task asset auto cleaned: %w", err)
	}
	if _, err := res.RowsAffected(); err != nil {
		return err
	}
	if taskID > 0 {
		if err := reindexTaskSearchDocument(ctx, sqlTx, taskID); err != nil {
			return err
		}
	}
	return reindexAssetSearchDocuments(ctx, sqlTx, assetIDs)
}

func splitCleanupStorageKeys(raw string) []string {
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func (r *taskAssetLifecycleRepo) Archive(ctx context.Context, tx repo.Tx, update repo.TaskAssetLifecycleUpdate) error {
	sqlTx := Unwrap(tx)
	taskIDs, err := taskIDsByAssetID(ctx, sqlTx, update.AssetID)
	if err != nil {
		return err
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE task_assets
		   SET is_archived = 1, archived_at = ?, archived_by = ?
		 WHERE asset_id = ? AND deleted_at IS NULL AND cleaned_at IS NULL`,
		update.Now, update.ActorID, update.AssetID)
	if err != nil {
		return fmt.Errorf("archive task asset: %w", err)
	}
	if err := requireAffected(res, "archive task asset"); err != nil {
		return err
	}
	if err := reindexTaskSearchDocuments(ctx, sqlTx, taskIDs); err != nil {
		return err
	}
	return reindexAssetSearchDocument(ctx, sqlTx, update.AssetID)
}

func (r *taskAssetLifecycleRepo) Restore(ctx context.Context, tx repo.Tx, update repo.TaskAssetLifecycleUpdate) error {
	sqlTx := Unwrap(tx)
	taskIDs, err := taskIDsByAssetID(ctx, sqlTx, update.AssetID)
	if err != nil {
		return err
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE task_assets
		   SET is_archived = 0, archived_at = NULL, archived_by = NULL
		 WHERE asset_id = ? AND deleted_at IS NULL AND cleaned_at IS NULL`,
		update.AssetID)
	if err != nil {
		return fmt.Errorf("restore task asset: %w", err)
	}
	if err := requireAffected(res, "restore task asset"); err != nil {
		return err
	}
	if err := reindexTaskSearchDocuments(ctx, sqlTx, taskIDs); err != nil {
		return err
	}
	return reindexAssetSearchDocument(ctx, sqlTx, update.AssetID)
}

func (r *taskAssetLifecycleRepo) SoftDelete(ctx context.Context, tx repo.Tx, update repo.TaskAssetLifecycleUpdate) error {
	sqlTx := Unwrap(tx)
	taskIDs, err := taskIDsByAssetID(ctx, sqlTx, update.AssetID)
	if err != nil {
		return err
	}
	assetIDs, err := resourceAndDerivedAssetIDs(ctx, sqlTx, update.AssetID)
	if err != nil {
		return err
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE task_assets ta
		JOIN design_assets da ON da.id = ta.asset_id
		   SET ta.deleted_at = ?, ta.storage_key = NULL
		 WHERE (da.id = ? OR da.source_asset_id = ?)
		   AND ta.deleted_at IS NULL`,
		update.Now, update.AssetID, update.AssetID)
	if err != nil {
		return fmt.Errorf("soft delete task asset: %w", err)
	}
	if err := requireAffected(res, "soft delete task asset"); err != nil {
		return err
	}
	if err := reindexTaskSearchDocuments(ctx, sqlTx, taskIDs); err != nil {
		return err
	}
	return reindexAssetSearchDocuments(ctx, sqlTx, assetIDs)
}

func resourceAndDerivedAssetIDs(ctx context.Context, q taskSearchDocumentSQL, assetID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id
		  FROM design_assets
		 WHERE id = ? OR source_asset_id = ?
		 ORDER BY id`, assetID, assetID)
	if err != nil {
		return nil, fmt.Errorf("list resource and derived asset ids: %w", err)
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan resource and derived asset id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *taskAssetLifecycleRepo) MarkAutoCleaned(ctx context.Context, tx repo.Tx, versionID int64, cleanedAt time.Time) error {
	sqlTx := Unwrap(tx)
	taskID, err := taskIDByAssetVersionID(ctx, sqlTx, versionID)
	if err != nil {
		return err
	}
	assetID, err := assetIDByAssetVersionID(ctx, sqlTx, versionID)
	if err != nil {
		return err
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE task_assets
		   SET is_archived = 1, cleaned_at = ?, storage_key = NULL
		 WHERE id = ? AND cleaned_at IS NULL AND deleted_at IS NULL`,
		cleanedAt, versionID)
	if err != nil {
		return fmt.Errorf("mark task asset auto cleaned: %w", err)
	}
	if _, err = res.RowsAffected(); err != nil {
		return err
	}
	if taskID > 0 {
		if err := reindexTaskSearchDocument(ctx, sqlTx, taskID); err != nil {
			return err
		}
	}
	return reindexAssetSearchDocument(ctx, sqlTx, assetID)
}

func (r *taskAssetLifecycleRepo) ListEligibleForCleanup(ctx context.Context, cutoff time.Time, limit int) ([]*repo.TaskAssetCleanupCandidate, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT ta.asset_id, ta.id, ta.task_id, ta.source_task_module_id, COALESCE(ta.storage_key, ''), ta.source_module_key,
		       COALESCE(ta.uploaded_at, ta.created_at)
		  FROM task_assets ta
		  JOIN tasks t ON t.id = ta.task_id
		 WHERE ta.deleted_at IS NULL
		   AND ta.cleaned_at IS NULL
		   AND COALESCE(ta.storage_key, '') <> ''
		   AND t.task_status IN (?, ?, ?)
		   AND COALESCE(ta.uploaded_at, ta.created_at) < ?
		   AND ta.id <> COALESCE((
		        SELECT da.current_version_id FROM design_assets da WHERE da.id = ta.asset_id
		   ), 0)
		 ORDER BY COALESCE(ta.uploaded_at, ta.created_at) ASC, ta.id ASC
		 LIMIT ?`,
		string(domain.TaskStatusCompleted), string(domain.TaskStatusCancelled), string(domain.TaskStatusArchived), cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list cleanup candidates: %w", err)
	}
	defer rows.Close()
	var out []*repo.TaskAssetCleanupCandidate
	for rows.Next() {
		var c repo.TaskAssetCleanupCandidate
		var assetID sql.NullInt64
		var moduleID sql.NullInt64
		if err := rows.Scan(&assetID, &c.VersionID, &c.TaskID, &moduleID, &c.StorageKey, &c.SourceModuleKey, &c.TaskUpdatedAt); err != nil {
			return nil, fmt.Errorf("scan cleanup candidate: %w", err)
		}
		if assetID.Valid {
			c.AssetID = assetID.Int64
		}
		c.SourceTaskModuleID = fromNullInt64(moduleID)
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (r *taskAssetLifecycleRepo) GetCurrentForUpdate(ctx context.Context, tx repo.Tx, assetID int64) (*repo.TaskAssetSearchRow, error) {
	row := Unwrap(tx).QueryRowContext(ctx, taskAssetSearchSelect+taskAssetSearchFrom+`
		WHERE da.id = ?
		  AND ta.id = COALESCE(da.current_version_id, (
		      SELECT ta2.id FROM task_assets ta2 WHERE ta2.asset_id = da.id ORDER BY ta2.asset_version_no DESC, ta2.id DESC LIMIT 1
		  ))
		FOR UPDATE`, assetID)
	return scanTaskAssetSearchRow(row)
}

// ResolveOrCreateLifecycleEventModule keeps delete auditing available for old
// tasks that predate task_modules. Existing modules are never overwritten;
// only a hidden closed placeholder is inserted when the exact source module
// does not exist yet.
func (r *taskAssetLifecycleRepo) ResolveOrCreateLifecycleEventModule(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string) (int64, error) {
	moduleKey = strings.TrimSpace(moduleKey)
	if taskID <= 0 || moduleKey == "" {
		return 0, fmt.Errorf("resolve lifecycle event module: task_id and module_key are required")
	}
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO task_modules (task_id, module_key, state, data)
		VALUES (?, ?, 'closed', JSON_OBJECT('backfill_placeholder', TRUE, 'source', 'asset_lifecycle'))
		ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)`, taskID, moduleKey)
	if err != nil {
		return 0, fmt.Errorf("resolve lifecycle event module: %w", err)
	}
	moduleID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("resolve lifecycle event module id: %w", err)
	}
	if moduleID <= 0 {
		return 0, fmt.Errorf("resolve lifecycle event module: no module id returned")
	}
	return moduleID, nil
}

func (r *taskAssetLifecycleRepo) InsertLifecycleEvent(ctx context.Context, tx repo.Tx, moduleID int64, eventType domain.ModuleEventType, actorID *int64, payload interface{}) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal lifecycle payload: %w", err)
	}
	_, err = Unwrap(tx).ExecContext(ctx, `
		INSERT INTO task_module_events (task_module_id, event_type, actor_id, actor_snapshot, payload)
		VALUES (?, ?, ?, JSON_OBJECT(), ?)`,
		moduleID, string(eventType), toNullInt64(actorID), jsonOrObject(raw))
	if err != nil {
		return fmt.Errorf("insert asset lifecycle event: %w", err)
	}
	return nil
}

func requireAffected(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s affected rows: %w", op, err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
