package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type taskAssetLifecycleRepo struct{ db *DB }

var _ repo.TaskAssetLifecycleObjectDeletionRepo = (*taskAssetLifecycleRepo)(nil)

func NewTaskAssetLifecycleRepo(db *DB) repo.TaskAssetLifecycleObjectDeletionRepo {
	return &taskAssetLifecycleRepo{db: db}
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
		   AND NOT EXISTS (
		        SELECT 1
		          FROM task_asset_group_revisions revision_ref
		         WHERE revision_ref.source_task_asset_id = ta.id
		            OR revision_ref.source_task_asset_id IN (
		                 SELECT derived_ref.id
		                   FROM task_assets derived_ref
		                  WHERE derived_ref.source_asset_version_id = ta.id
		            )
		   )
		   AND NOT EXISTS (
		        SELECT 1
		          FROM task_asset_group_revision_items item_ref
		         WHERE item_ref.task_asset_id = ta.id
		            OR item_ref.task_asset_id IN (
		                 SELECT derived_ref.id
		                   FROM task_assets derived_ref
		                  WHERE derived_ref.source_asset_version_id = ta.id
		            )
		   )
		   AND NOT EXISTS (
		        SELECT 1
		          FROM task_asset_group_revision_references frozen_ref
		         WHERE frozen_ref.formal_task_asset_id = ta.id
		            OR frozen_ref.formal_task_asset_id IN (
		                 SELECT derived_ref.id
		                   FROM task_assets derived_ref
		                  WHERE derived_ref.source_asset_version_id = ta.id
		            )
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

// LockGenericDeleteGuard locks one authoritative deletion set: the requested
// design asset, every backend-derived design asset linked through
// source_asset_id, and all of their live task-asset versions. Binding,
// revision, publication, outbox and soft-delete operations must reuse the
// returned task-asset IDs so no derived object escapes validation.
func (r *taskAssetLifecycleRepo) LockGenericDeleteGuard(ctx context.Context, tx repo.Tx, assetID int64) (*repo.TaskAssetDeleteGuard, error) {
	sqlTx := Unwrap(tx)
	assetRows, err := sqlTx.QueryContext(ctx, `
		SELECT id
		  FROM design_assets
		 WHERE id = ? OR source_asset_id = ?
		 ORDER BY id
		 FOR UPDATE`, assetID, assetID)
	if err != nil {
		return nil, fmt.Errorf("lock generic delete design assets: %w", err)
	}
	guard := &repo.TaskAssetDeleteGuard{AllStagedUnbound: true}
	for assetRows.Next() {
		var designAssetID int64
		if err := assetRows.Scan(&designAssetID); err != nil {
			_ = assetRows.Close()
			return nil, fmt.Errorf("scan generic delete design asset: %w", err)
		}
		guard.DesignAssetIDs = append(guard.DesignAssetIDs, designAssetID)
	}
	if err := assetRows.Close(); err != nil {
		return nil, err
	}
	if len(guard.DesignAssetIDs) == 0 {
		return nil, sql.ErrNoRows
	}
	designMarks, designArgs := int64MutationArgs(guard.DesignAssetIDs)
	rows, err := sqlTx.QueryContext(ctx, `
		SELECT ta.id, ta.binding_state, ta.bound_group_id, ta.bound_role
		  FROM task_assets ta
		 WHERE ta.asset_id IN (`+designMarks+`)
		   AND ta.deleted_at IS NULL
		   AND ta.cleaned_at IS NULL
		 ORDER BY ta.id
		 FOR UPDATE`, designArgs...)
	if err != nil {
		return nil, fmt.Errorf("lock generic delete asset versions: %w", err)
	}
	for rows.Next() {
		var taskAssetID int64
		var bindingState string
		var boundGroupID sql.NullInt64
		var boundRole sql.NullString
		if err := rows.Scan(&taskAssetID, &bindingState, &boundGroupID, &boundRole); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan generic delete asset version: %w", err)
		}
		guard.TaskAssetIDs = append(guard.TaskAssetIDs, taskAssetID)
		if strings.TrimSpace(bindingState) != "staged" || boundGroupID.Valid || strings.TrimSpace(boundRole.String) != "" {
			guard.AllStagedUnbound = false
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(guard.TaskAssetIDs) == 0 {
		return nil, sql.ErrNoRows
	}

	inClause, args := int64MutationArgs(guard.TaskAssetIDs)
	revisionQueries := []string{
		`SELECT id FROM task_asset_group_revisions WHERE source_task_asset_id IN (` + inClause + `) ORDER BY id FOR UPDATE`,
		`SELECT revision_id FROM task_asset_group_revision_items WHERE task_asset_id IN (` + inClause + `) ORDER BY revision_id FOR UPDATE`,
		`SELECT revision_id FROM task_asset_group_revision_references WHERE formal_task_asset_id IN (` + inClause + `) ORDER BY revision_id FOR UPDATE`,
	}
	revisionIDs := map[int64]struct{}{}
	for _, query := range revisionQueries {
		refRows, err := sqlTx.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("lock generic delete revision references: %w", err)
		}
		for refRows.Next() {
			var revisionID int64
			if err := refRows.Scan(&revisionID); err != nil {
				_ = refRows.Close()
				return nil, fmt.Errorf("scan generic delete revision reference: %w", err)
			}
			revisionIDs[revisionID] = struct{}{}
		}
		if err := refRows.Close(); err != nil {
			return nil, err
		}
	}
	for revisionID := range revisionIDs {
		guard.RevisionReferenceIDs = append(guard.RevisionReferenceIDs, revisionID)
	}
	sort.Slice(guard.RevisionReferenceIDs, func(i, j int) bool { return guard.RevisionReferenceIDs[i] < guard.RevisionReferenceIDs[j] })

	pinRows, err := sqlTx.QueryContext(ctx, `
		SELECT id
		  FROM asset_workbench_client_materials
		 WHERE source_type = 'system' AND asset_id IN (`+designMarks+`)
		 ORDER BY id
		 FOR UPDATE`, designArgs...)
	if err != nil {
		return nil, fmt.Errorf("lock generic delete publication pins: %w", err)
	}
	for pinRows.Next() {
		var pinID int64
		if err := pinRows.Scan(&pinID); err != nil {
			_ = pinRows.Close()
			return nil, fmt.Errorf("scan generic delete publication pin: %w", err)
		}
		guard.PublicationPinIDs = append(guard.PublicationPinIDs, pinID)
	}
	if err := pinRows.Close(); err != nil {
		return nil, err
	}
	return guard, nil
}

// EnqueueObjectDeletions snapshots every object key owned by the resource and
// its backend-derived previews while the delete transaction still holds the
// current resource lock. Physical object deletion is performed asynchronously;
// the request path never calls object storage before the database commit.
func (r *taskAssetLifecycleRepo) EnqueueObjectDeletions(ctx context.Context, tx repo.Tx, taskAssetIDs []int64) error {
	return enqueueTaskAssetObjectDeletions(ctx, Unwrap(tx), taskAssetIDs)
}

// LockCleanupObjectIDs freezes the exact root + derived object set before the
// cleanup transaction snapshots deletion outbox rows and clears live pointers.
// The worker, never the cleanup request, performs physical deletion.
func (r *taskAssetLifecycleRepo) LockCleanupObjectIDs(ctx context.Context, tx repo.Tx, versionID int64) ([]int64, error) {
	rows, err := Unwrap(tx).QueryContext(ctx, `
		SELECT id
		FROM task_assets
		WHERE (id = ? OR source_asset_version_id = ?)
		  AND deleted_at IS NULL
		  AND cleaned_at IS NULL
		ORDER BY id
		FOR UPDATE`, versionID, versionID)
	if err != nil {
		return nil, fmt.Errorf("lock cleanup object versions: %w", err)
	}
	ids := make([]int64, 0, 4)
	rootFound := false
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
		rootFound = rootFound || id == versionID
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if !rootFound {
		return nil, repo.ErrConflict
	}
	referenced, err := cleanupObjectSetHasRevisionReferences(ctx, Unwrap(tx), ids)
	if err != nil {
		return nil, err
	}
	if referenced {
		return nil, repo.ErrConflict
	}
	return ids, nil
}

func cleanupObjectSetHasRevisionReferences(ctx context.Context, sqlTx *sql.Tx, taskAssetIDs []int64) (bool, error) {
	marks, args := int64MutationArgs(taskAssetIDs)
	if marks == "" {
		return false, nil
	}
	queryArgs := make([]interface{}, 0, len(args)*3)
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, args...)
	var referenced bool
	err := sqlTx.QueryRowContext(ctx, `
		SELECT
		  EXISTS (
		    SELECT 1 FROM task_asset_group_revisions
		     WHERE source_task_asset_id IN (`+marks+`)
		  )
		  OR EXISTS (
		    SELECT 1 FROM task_asset_group_revision_items
		     WHERE task_asset_id IN (`+marks+`)
		  )
		  OR EXISTS (
		    SELECT 1 FROM task_asset_group_revision_references
		     WHERE formal_task_asset_id IN (`+marks+`)
		  )`, queryArgs...).Scan(&referenced)
	if err != nil {
		return false, fmt.Errorf("check cleanup revision references: %w", err)
	}
	return referenced, nil
}

func enqueueTaskAssetObjectDeletions(ctx context.Context, sqlTx *sql.Tx, taskAssetIDs []int64) error {
	marks, args := int64MutationArgs(taskAssetIDs)
	if marks == "" {
		return sql.ErrNoRows
	}
	// task_assets.storage_key is the primary pointer, while the exact
	// task_assets.storage_ref_id relation freezes the backend adapter and the
	// upload-boundary fallback key. Never infer an adapter from owner ids or send
	// an unclassified key to OSS. The authoritative ID set already contains
	// derived preview/design-thumb versions locked by LockGenericDeleteGuard.
	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO asset_object_deletion_outbox (
		    task_asset_id, storage_ref_id, storage_adapter, storage_is_placeholder, storage_key, dedupe_key
		)
		SELECT object_keys.task_asset_id,
		       object_keys.storage_ref_id,
		       object_keys.storage_adapter,
		       object_keys.storage_is_placeholder,
		       object_keys.storage_key,
		       CONCAT('asset-delete:task-asset:', object_keys.task_asset_id, ':', SHA2(object_keys.storage_key, 256))
		  FROM (
		        SELECT ta.id AS task_asset_id,
		               storage_ref.ref_id AS storage_ref_id,
		               COALESCE(NULLIF(TRIM(storage_ref.storage_adapter), ''), 'unknown') AS storage_adapter,
		               COALESCE(storage_ref.is_placeholder, 0) AS storage_is_placeholder,
		               TRIM(ta.storage_key) AS storage_key
		          FROM task_assets ta
		          LEFT JOIN asset_storage_refs storage_ref ON storage_ref.ref_id = ta.storage_ref_id
		         WHERE ta.id IN (`+marks+`)
		           AND ta.deleted_at IS NULL
		           AND COALESCE(TRIM(ta.storage_key), '') <> ''
		        UNION
		        SELECT ta.id AS task_asset_id,
		               storage_ref.ref_id AS storage_ref_id,
		               COALESCE(NULLIF(TRIM(storage_ref.storage_adapter), ''), 'unknown') AS storage_adapter,
		               storage_ref.is_placeholder AS storage_is_placeholder,
		               TRIM(storage_ref.ref_key) AS storage_key
		          FROM task_assets ta
		          JOIN asset_storage_refs storage_ref ON storage_ref.ref_id = ta.storage_ref_id
		         WHERE ta.id IN (`+marks+`)
		           AND ta.deleted_at IS NULL
		           AND COALESCE(TRIM(storage_ref.ref_key), '') <> ''
		       ) object_keys
		ON DUPLICATE KEY UPDATE
		  task_asset_id = VALUES(task_asset_id),
		  storage_ref_id = VALUES(storage_ref_id),
		  storage_adapter = VALUES(storage_adapter),
		  storage_is_placeholder = VALUES(storage_is_placeholder)`, append(args, args...)...)
	if err != nil {
		return fmt.Errorf("enqueue resource object deletions: %w", err)
	}
	return nil
}

func (r *taskAssetLifecycleRepo) ClaimObjectDeletions(ctx context.Context, tx repo.Tx, leaseToken string, now, leaseUntil time.Time, limit int) ([]repo.AssetObjectDeletionOutboxItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	sqlTx := Unwrap(tx)
	rows, err := sqlTx.QueryContext(ctx, `
		SELECT id, task_asset_id, storage_ref_id, storage_adapter, storage_is_placeholder, storage_key, attempt
		  FROM asset_object_deletion_outbox
		 WHERE (
		       (status IN ('pending', 'retry') AND (next_retry_at IS NULL OR next_retry_at <= ?))
		       OR (status = 'processing' AND lease_until IS NOT NULL AND lease_until <= ?)
		 )
		 ORDER BY id
		 LIMIT ?
		 FOR UPDATE SKIP LOCKED`, now, now, limit)
	if err != nil {
		return nil, fmt.Errorf("claim object deletion outbox: %w", err)
	}
	items := make([]repo.AssetObjectDeletionOutboxItem, 0, limit)
	for rows.Next() {
		var item repo.AssetObjectDeletionOutboxItem
		var taskAssetID sql.NullInt64
		var storageRefID, storageAdapter sql.NullString
		if err := rows.Scan(&item.ID, &taskAssetID, &storageRefID, &storageAdapter, &item.StorageIsPlaceholder, &item.StorageKey, &item.Attempt); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan object deletion outbox: %w", err)
		}
		item.TaskAssetID = fromNullInt64(taskAssetID)
		if storageRefID.Valid {
			value := strings.TrimSpace(storageRefID.String)
			if value != "" {
				item.StorageRefID = &value
			}
		}
		item.StorageAdapter = domain.AssetStorageAdapter(strings.TrimSpace(storageAdapter.String))
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return items, nil
	}
	marks := make([]string, len(items))
	args := make([]interface{}, 0, len(items)+2)
	args = append(args, leaseToken, leaseUntil)
	for index := range items {
		marks[index] = "?"
		args = append(args, items[index].ID)
		items[index].Attempt++
	}
	result, err := sqlTx.ExecContext(ctx, `
		UPDATE asset_object_deletion_outbox
		   SET status = 'processing', attempt = attempt + 1,
		       next_retry_at = NULL, lease_token = ?, lease_until = ?
		 WHERE id IN (`+strings.Join(marks, ",")+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("lease object deletion outbox: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != int64(len(items)) {
		return nil, repo.ErrConflict
	}
	return items, nil
}

func (r *taskAssetLifecycleRepo) MarkObjectDeletionSucceeded(ctx context.Context, tx repo.Tx, item repo.AssetObjectDeletionOutboxItem, leaseToken string, deletedAt time.Time) error {
	sqlTx := Unwrap(tx)
	result, err := sqlTx.ExecContext(ctx, `
		UPDATE asset_object_deletion_outbox
		   SET status = 'succeeded', next_retry_at = NULL,
		       lease_token = NULL, lease_until = NULL, last_error = NULL,
		       alert_status = 'none'
		 WHERE id = ? AND status = 'processing' AND lease_token = ?`, item.ID, leaseToken)
	if err != nil {
		return fmt.Errorf("complete object deletion outbox: %w", err)
	}
	if err := requireAffected(result, "complete object deletion outbox"); err != nil {
		return err
	}
	if item.TaskAssetID != nil {
		if _, err := sqlTx.ExecContext(ctx, `
			UPDATE task_assets
			   SET object_deleted_at = COALESCE(object_deleted_at, ?), storage_key = NULL
			 WHERE id = ?
			   AND NOT EXISTS (
			       SELECT 1
			         FROM asset_object_deletion_outbox remaining
			        WHERE remaining.task_asset_id = ?
			          AND remaining.status <> 'succeeded'
			   )`, deletedAt, *item.TaskAssetID, *item.TaskAssetID); err != nil {
			return fmt.Errorf("mark task asset object deleted: %w", err)
		}
	}
	return nil
}

func (r *taskAssetLifecycleRepo) MarkObjectDeletionRetry(ctx context.Context, tx repo.Tx, item repo.AssetObjectDeletionOutboxItem, leaseToken, lastError string, nextRetryAt time.Time, alert bool) error {
	result, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_object_deletion_outbox
		   SET status = 'retry', next_retry_at = ?,
		       lease_token = NULL, lease_until = NULL, last_error = ?,
		       alert_status = CASE WHEN ? THEN 'required' ELSE alert_status END
		 WHERE id = ? AND status = 'processing' AND lease_token = ?`,
		nextRetryAt, lastError, alert, item.ID, leaseToken)
	if err != nil {
		return fmt.Errorf("retry object deletion outbox: %w", err)
	}
	return requireAffected(result, "retry object deletion outbox")
}

func (r *taskAssetLifecycleRepo) SoftDelete(ctx context.Context, tx repo.Tx, update repo.TaskAssetLifecycleUpdate) error {
	sqlTx := Unwrap(tx)
	marks, args := int64MutationArgs(update.TaskAssetIDs)
	if marks == "" {
		return sql.ErrNoRows
	}
	rows, err := sqlTx.QueryContext(ctx, `SELECT DISTINCT task_id, asset_id FROM task_assets WHERE id IN (`+marks+`)`, args...)
	if err != nil {
		return err
	}
	taskIDSet := map[int64]struct{}{}
	assetIDSet := map[int64]struct{}{}
	for rows.Next() {
		var taskID int64
		var designAssetID sql.NullInt64
		if err := rows.Scan(&taskID, &designAssetID); err != nil {
			_ = rows.Close()
			return err
		}
		taskIDSet[taskID] = struct{}{}
		if designAssetID.Valid {
			assetIDSet[designAssetID.Int64] = struct{}{}
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	taskIDs := sortedInt64Set(taskIDSet)
	assetIDs := sortedInt64Set(assetIDSet)
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE task_assets
		   SET deleted_at = ?,
		       binding_state = 'discarded',
		       access_revoked_at = ?,
		       access_revoked_reason = ?,
		       storage_key = NULL
		 WHERE id IN (`+marks+`)
		   AND deleted_at IS NULL`,
		append([]interface{}{update.Now, update.Now, "generic_delete: " + strings.TrimSpace(update.Reason)}, args...)...)
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

func int64MutationArgs(ids []int64) (string, []interface{}) {
	marks := make([]string, 0, len(ids))
	args := make([]interface{}, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		marks = append(marks, "?")
		args = append(args, id)
	}
	return strings.Join(marks, ","), args
}

func sortedInt64Set(values map[int64]struct{}) []int64 {
	items := make([]int64, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Slice(items, func(i, j int) bool { return items[i] < items[j] })
	return items
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
