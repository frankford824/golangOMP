package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

// LockERPProductImageSet verifies the captured SKU and every member just before
// the image-only ERP call. Finalization and resource revocation share these rows.
func (r *TaskResourceGroupRepo) LockERPProductImageSet(ctx context.Context, tx repo.Tx, expected domain.TaskAssetGroup) error {
	revision := expected.FinalizedRevision
	if revision == nil {
		return repo.ErrConflict
	}
	db := Unwrap(tx)
	sku := expected.SKUCode
	// Match workflow lock order: task and SKU identity, then resource group
	// and finalized revision. The identity is protected until dispatch.
	switch expected.ScopeKind {
	case domain.TaskAssetGroupScopeSKU:
		if expected.TaskSKUItemID == nil {
			return repo.ErrConflict
		}
		var taskID int64
		if err := db.QueryRowContext(ctx, `SELECT id FROM tasks WHERE id = ? FOR UPDATE`, expected.TaskID).Scan(&taskID); err != nil {
			return err
		}
		var code string
		if err := db.QueryRowContext(ctx, `SELECT sku_code, task_id FROM task_sku_items WHERE id = ? FOR UPDATE`, *expected.TaskSKUItemID).Scan(&code, &taskID); err != nil {
			return err
		}
		if code != sku || taskID != expected.TaskID {
			return repo.ErrConflict
		}
	case domain.TaskAssetGroupScopeTask:
		if err := lockERPImageSetTaskSKU(ctx, db, expected.TaskID, sku); err != nil {
			return err
		}
	default:
		return repo.ErrConflict
	}
	locked, err := r.LockGroup(ctx, tx, expected.TaskID, expected.ID, expected.LockVersion)
	if err != nil {
		return err
	}
	if locked.ScopeKind != expected.ScopeKind || locked.MigrationIncomplete ||
		locked.FinalizedRevisionID == nil || *locked.FinalizedRevisionID != revision.ID ||
		!equalERPImageSetItemID(locked.TaskSKUItemID, expected.TaskSKUItemID) {
		return repo.ErrConflict
	}

	actual, err := r.GetRevisionForUpdate(ctx, tx, revision.ID)
	if err != nil {
		return err
	}
	if actual.GroupID != locked.ID || actual.Status != domain.TaskAssetGroupRevisionFinalized ||
		actual.Mode != domain.TaskAssetGroupModeSet || actual.FinalOutputWaived || len(actual.Items) != len(revision.Items) {
		return repo.ErrConflict
	}
	for i, item := range revision.Items {
		got := actual.Items[i]
		if item.ID != got.ID || item.RevisionID != got.RevisionID || item.TaskAssetID != got.TaskAssetID ||
			item.SortOrder != got.SortOrder || item.File == nil {
			return repo.ErrConflict
		}
		var code, key, filename, mime string
		var fileSize sql.NullInt64
		err = db.QueryRowContext(ctx, `
			SELECT COALESCE(a.scope_sku_code, ''), COALESCE(NULLIF(a.storage_key, ''), NULLIF(ref.ref_key, '')),
			       a.file_name, COALESCE(a.mime_type, ''), a.file_size
			FROM task_assets a JOIN asset_storage_refs ref ON ref.ref_id = a.storage_ref_id
			WHERE a.id = ? AND a.task_id = ? AND a.bound_group_id = ? AND a.bound_role = 'final'
			  AND a.asset_type = 'delivery' AND a.binding_state = 'bound'
			  AND a.deleted_at IS NULL AND a.cleaned_at IS NULL AND a.access_revoked_at IS NULL
			  AND a.object_deleted_at IS NULL AND a.is_archived = 0
			  AND ref.is_placeholder = 0 AND ref.status NOT IN ('archived', 'historical_unavailable')
			FOR UPDATE`, item.TaskAssetID, expected.TaskID, expected.ID).Scan(&code, &key, &filename, &mime, &fileSize)
		if err != nil {
			return fmt.Errorf("verify ERP set member %d: %w", item.TaskAssetID, err)
		}
		if (code != "" && code != sku) || key != item.File.StorageKey || filename != item.File.FileName ||
			mime != item.File.MimeType || !equalERPImageSetItemID(fromNullInt64(fileSize), item.File.FileSize) {
			return repo.ErrConflict
		}
	}
	return nil
}

func equalERPImageSetItemID(left, right *int64) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func lockERPImageSetTaskSKU(ctx context.Context, db *sql.Tx, taskID int64, sku string) error {
	var own, primary string
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(sku_code, ''), COALESCE(primary_sku_code, '') FROM tasks WHERE id = ? FOR UPDATE`, taskID).
		Scan(&own, &primary); err != nil {
		return err
	}
	if (own == "" && primary == "") || (own != "" && own != sku) || (primary != "" && primary != sku) {
		return repo.ErrConflict
	}
	rows, err := db.QueryContext(ctx, `SELECT sku_code FROM task_sku_items WHERE task_id = ? FOR UPDATE`, taskID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return err
		}
		if strings.TrimSpace(code) != "" && code != sku {
			return repo.ErrConflict
		}
	}
	return rows.Err()
}
