package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
)

const supersededResourceCleanupReason = "resource_revision_superseded"

// SupersededResourceCleanupSummary reports the exact immutable resource
// objects selected by the current working/finalized revision pointers.
type SupersededResourceCleanupSummary struct {
	DryRun                bool  `json:"dry_run"`
	SelectedTaskAssets    int64 `json:"selected_task_assets"`
	SelectedBytes         int64 `json:"selected_bytes"`
	RevokedTaskAssets     int64 `json:"revoked_task_assets"`
	QueuedObjectDeletions int64 `json:"queued_object_deletions"`
}

// PurgeSupersededResourceObjects keeps revision/audit rows but revokes and
// queues deletion of every reference, source, and final object not used by any
// resource group's current working/finalized revision.
func PurgeSupersededResourceObjects(ctx context.Context, db *sql.DB, apply bool) (SupersededResourceCleanupSummary, error) {
	out := SupersededResourceCleanupSummary{DryRun: !apply}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return out, fmt.Errorf("begin superseded resource cleanup: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DROP TEMPORARY TABLE IF EXISTS purge_superseded_task_assets`); err != nil {
		return out, fmt.Errorf("drop stale superseded resource selection: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		CREATE TEMPORARY TABLE purge_superseded_task_assets (
		  task_asset_id BIGINT NOT NULL PRIMARY KEY
		) ENGINE=InnoDB`); err != nil {
		return out, fmt.Errorf("create superseded resource selection: %w", err)
	}

	// Finalization changes the group pointers under a row lock. Holding every
	// group row only during the apply transaction prevents a concurrent
	// finalization from turning a selected historical object into a current one.
	if apply {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM task_asset_groups ORDER BY id FOR UPDATE`)
		if err != nil {
			return out, fmt.Errorf("lock resource groups for cleanup: %w", err)
		}
		for rows.Next() {
			var ignoredID int64
			if err := rows.Scan(&ignoredID); err != nil {
				rows.Close()
				return out, fmt.Errorf("scan locked resource group: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return out, fmt.Errorf("iterate locked resource groups: %w", err)
		}
		if err := rows.Close(); err != nil {
			return out, fmt.Errorf("close locked resource groups: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, supersededResourceSelectionSQL); err != nil {
		return out, fmt.Errorf("select superseded resource objects: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(asset.file_size), 0)
		FROM purge_superseded_task_assets selected
		JOIN task_assets asset ON asset.id = selected.task_asset_id`).
		Scan(&out.SelectedTaskAssets, &out.SelectedBytes); err != nil {
		return out, fmt.Errorf("summarize superseded resource objects: %w", err)
	}

	if !apply {
		if _, err := tx.ExecContext(ctx, `DROP TEMPORARY TABLE purge_superseded_task_assets`); err != nil {
			return out, fmt.Errorf("drop superseded resource dry-run selection: %w", err)
		}
		if err := tx.Rollback(); err != nil {
			return out, fmt.Errorf("rollback superseded resource dry run: %w", err)
		}
		return out, nil
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE task_assets asset
		JOIN purge_superseded_task_assets selected ON selected.task_asset_id = asset.id
		SET asset.access_revoked_at = UTC_TIMESTAMP(6),
		    asset.access_revoked_reason = ?
		WHERE asset.access_revoked_at IS NULL
		  AND asset.object_deleted_at IS NULL`, supersededResourceCleanupReason)
	if err != nil {
		return out, fmt.Errorf("revoke superseded resource objects: %w", err)
	}
	out.RevokedTaskAssets, err = result.RowsAffected()
	if err != nil {
		return out, fmt.Errorf("count revoked superseded resource objects: %w", err)
	}
	if out.RevokedTaskAssets != out.SelectedTaskAssets {
		return out, fmt.Errorf(
			"superseded resource cleanup changed %d of %d selected task assets",
			out.RevokedTaskAssets, out.SelectedTaskAssets,
		)
	}

	rows, err := tx.QueryContext(ctx, `SELECT task_asset_id FROM purge_superseded_task_assets ORDER BY task_asset_id`)
	if err != nil {
		return out, fmt.Errorf("list superseded resource task assets: %w", err)
	}
	taskAssetIDs := make([]int64, 0, out.SelectedTaskAssets)
	for rows.Next() {
		var taskAssetID int64
		if err := rows.Scan(&taskAssetID); err != nil {
			rows.Close()
			return out, fmt.Errorf("scan superseded resource task asset: %w", err)
		}
		taskAssetIDs = append(taskAssetIDs, taskAssetID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, fmt.Errorf("iterate superseded resource task assets: %w", err)
	}
	if err := rows.Close(); err != nil {
		return out, fmt.Errorf("close superseded resource task assets: %w", err)
	}
	if len(taskAssetIDs) > 0 {
		if err := enqueueTaskAssetObjectDeletions(ctx, tx, taskAssetIDs); err != nil {
			return out, err
		}
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM asset_object_deletion_outbox deletion
		JOIN purge_superseded_task_assets selected ON selected.task_asset_id = deletion.task_asset_id
		WHERE deletion.status <> 'succeeded'`).Scan(&out.QueuedObjectDeletions); err != nil {
		return out, fmt.Errorf("count queued superseded resource deletions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TEMPORARY TABLE purge_superseded_task_assets`); err != nil {
		return out, fmt.Errorf("drop superseded resource apply selection: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return out, fmt.Errorf("commit superseded resource cleanup: %w", err)
	}
	return out, nil
}

const supersededResourceSelectionSQL = `
	INSERT INTO purge_superseded_task_assets (task_asset_id)
	SELECT candidate.task_asset_id
	FROM (
	  SELECT revision.source_task_asset_id AS task_asset_id
	  FROM task_asset_group_revisions revision
	  WHERE revision.source_task_asset_id IS NOT NULL
	  UNION
	  SELECT item.task_asset_id
	  FROM task_asset_group_revision_items item
	  UNION
	  SELECT reference.formal_task_asset_id
	  FROM task_asset_group_revision_references reference
	  WHERE reference.formal_task_asset_id IS NOT NULL
	) candidate
	JOIN task_assets asset ON asset.id = candidate.task_asset_id
	LEFT JOIN (
	  SELECT revision.source_task_asset_id AS task_asset_id
	  FROM task_asset_groups current_group
	  JOIN task_asset_group_revisions revision
	    ON revision.id IN (current_group.working_revision_id, current_group.finalized_revision_id)
	  WHERE revision.source_task_asset_id IS NOT NULL
	  UNION
	  SELECT item.task_asset_id
	  FROM task_asset_groups current_group
	  JOIN task_asset_group_revision_items item
	    ON item.revision_id IN (current_group.working_revision_id, current_group.finalized_revision_id)
	  UNION
	  SELECT reference.formal_task_asset_id
	  FROM task_asset_groups current_group
	  JOIN task_asset_group_revision_references reference
	    ON reference.revision_id IN (current_group.working_revision_id, current_group.finalized_revision_id)
	  WHERE reference.formal_task_asset_id IS NOT NULL
	) current_asset ON current_asset.task_asset_id = candidate.task_asset_id
	WHERE asset.deleted_at IS NULL
	  AND asset.cleaned_at IS NULL
	  AND asset.object_deleted_at IS NULL
	  AND asset.access_revoked_at IS NULL
	  AND current_asset.task_asset_id IS NULL`
