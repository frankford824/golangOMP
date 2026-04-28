package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type taskAssetLifecycleRepo struct{ db *DB }

func NewTaskAssetLifecycleRepo(db *DB) repo.TaskAssetLifecycleRepo {
	return &taskAssetLifecycleRepo{db: db}
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
	return reindexTaskSearchDocuments(ctx, sqlTx, taskIDs)
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
	return reindexTaskSearchDocuments(ctx, sqlTx, taskIDs)
}

func (r *taskAssetLifecycleRepo) SoftDelete(ctx context.Context, tx repo.Tx, update repo.TaskAssetLifecycleUpdate) error {
	sqlTx := Unwrap(tx)
	taskIDs, err := taskIDsByAssetID(ctx, sqlTx, update.AssetID)
	if err != nil {
		return err
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE task_assets
		   SET deleted_at = ?, storage_key = NULL
		 WHERE asset_id = ? AND deleted_at IS NULL`,
		update.Now, update.AssetID)
	if err != nil {
		return fmt.Errorf("soft delete task asset: %w", err)
	}
	if err := requireAffected(res, "soft delete task asset"); err != nil {
		return err
	}
	return reindexTaskSearchDocuments(ctx, sqlTx, taskIDs)
}

func (r *taskAssetLifecycleRepo) MarkAutoCleaned(ctx context.Context, tx repo.Tx, versionID int64, cleanedAt time.Time) error {
	sqlTx := Unwrap(tx)
	taskID, err := taskIDByAssetVersionID(ctx, sqlTx, versionID)
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
		return reindexTaskSearchDocument(ctx, sqlTx, taskID)
	}
	return nil
}

func (r *taskAssetLifecycleRepo) ListEligibleForCleanup(ctx context.Context, cutoff time.Time, limit int) ([]*repo.TaskAssetCleanupCandidate, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT ta.asset_id, ta.id, ta.task_id, ta.source_task_module_id, COALESCE(ta.storage_key, ''), ta.source_module_key, t.updated_at
		  FROM task_assets ta
		  JOIN tasks t ON t.id = ta.task_id
		 WHERE ta.deleted_at IS NULL
		   AND ta.cleaned_at IS NULL
		   AND COALESCE(ta.storage_key, '') <> ''
		   AND t.task_status IN (?, ?, ?)
		   AND t.updated_at < ?
		 ORDER BY t.updated_at ASC, ta.id ASC
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
