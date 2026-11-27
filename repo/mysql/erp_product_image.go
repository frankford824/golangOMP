package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type erpProductImageRepo struct{ db *DB }

func NewERPProductImageRepo(db *DB) repo.ERPProductImageRepo {
	return &erpProductImageRepo{db: db}
}

func (r *erpProductImageRepo) NextSKU(ctx context.Context, now time.Time) (string, error) {
	var sku string
	err := r.db.db.QueryRowContext(ctx, `
		SELECT sku_code FROM erp_product_image_checks
		WHERE next_check_at <= ? AND (leased_until IS NULL OR leased_until <= ?)
		ORDER BY next_check_at, sku_code LIMIT 1`, now, now).Scan(&sku)
	if !errors.Is(err, sql.ErrNoRows) {
		return sku, err
	}
	err = r.db.db.QueryRowContext(ctx, `
		SELECT p.sku_code FROM erp_product_sync_records p
		WHERE p.sku_code <> '' AND p.image_required = 1
		  AND (p.base_sync_status = 'synced' OR p.erp_sync_status = 'synced'
		       OR p.last_image_synced_at IS NOT NULL)
		  AND NOT EXISTS (
		    SELECT 1 FROM erp_product_image_checks c WHERE c.sku_code = p.sku_code
		  )
		ORDER BY p.sku_code LIMIT 1`).Scan(&sku)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return sku, err
}

// ImageGapScanSKUs walks the same ERP-owned catalogue as the ordinary queue.
// A gap already held for review remains on its existing retry schedule.
func (r *erpProductImageRepo) ImageGapScanSKUs(ctx context.Context, after string, now time.Time, limit int) ([]string, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT DISTINCT p.sku_code FROM erp_product_sync_records p
		LEFT JOIN erp_product_image_checks c ON c.sku_code = p.sku_code
		WHERE p.sku_code > ? AND p.image_required = 1
		  AND (p.base_sync_status = 'synced' OR p.erp_sync_status = 'synced'
		       OR p.last_image_synced_at IS NOT NULL)
		  AND (c.leased_until IS NULL OR c.leased_until <= ?)
		  AND (c.sku_code IS NULL OR c.next_check_at <= ? OR c.last_error NOT LIKE 'ERP 单边缺图暂缓%')
		ORDER BY p.sku_code LIMIT ?`, after, now, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var sku string
		if err := rows.Scan(&sku); err != nil {
			return nil, err
		}
		result = append(result, sku)
	}
	return result, rows.Err()
}

func (r *erpProductImageRepo) Acquire(ctx context.Context, sku, token string, now, until time.Time) (bool, error) {
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO erp_product_image_checks (sku_code, next_check_at, lease_token, leased_until)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  lease_token = IF(leased_until IS NULL OR leased_until <= ?, VALUES(lease_token), lease_token),
		  next_check_at = IF(lease_token = VALUES(lease_token), VALUES(next_check_at), next_check_at),
		  leased_until = IF(lease_token = VALUES(lease_token), VALUES(leased_until), leased_until)`,
		sku, until, token, until, now)
	if err != nil {
		return false, err
	}
	var owner string
	err = r.db.db.QueryRowContext(ctx, `SELECT lease_token FROM erp_product_image_checks WHERE sku_code = ?`, sku).Scan(&owner)
	return owner == token, err
}

func (r *erpProductImageRepo) RecordReferences(ctx context.Context, sku, token string, refs []repo.ERPProductImageReference, replace bool) error {
	return r.db.RunInTx(ctx, func(tx repo.Tx) error {
		db := Unwrap(tx)
		var owner string
		if err := db.QueryRowContext(ctx, `
			SELECT lease_token FROM erp_product_image_checks
			WHERE sku_code = ? AND leased_until > UTC_TIMESTAMP(6) FOR UPDATE`, sku).Scan(&owner); err != nil {
			return err
		}
		if owner != token {
			return repo.ErrConflict
		}
		if replace {
			if _, err := db.ExecContext(ctx, `DELETE FROM erp_product_image_references WHERE sku_code = ?`, sku); err != nil {
				return err
			}
		}
		for _, ref := range refs {
			var version int64
			var parent sql.NullInt64
			var storage sql.NullString
			if err := db.QueryRowContext(ctx, `
				SELECT id, source_asset_version_id, storage_ref_id FROM task_assets WHERE id = ? AND deleted_at IS NULL AND cleaned_at IS NULL
				  AND access_revoked_at IS NULL AND object_deleted_at IS NULL
				FOR UPDATE`, ref.VersionID).Scan(&version, &parent, &storage); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("ERP image version %d is unavailable: %w", ref.VersionID, err)
				}
				return err
			}
			if parent.Valid {
				var sourceID int64
				err := db.QueryRowContext(ctx, `SELECT id FROM task_assets WHERE id = ? FOR UPDATE`, parent.Int64).Scan(&sourceID)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return err
				}
			}
			if _, err := db.ExecContext(ctx, `
				INSERT INTO erp_product_image_references (sku_code, asset_version_id, expires_at, source_version_id, storage_ref_id)
				VALUES (?, ?, ?, ?, ?)
				ON DUPLICATE KEY UPDATE expires_at = COALESCE(VALUES(expires_at), expires_at),
				    source_version_id = VALUES(source_version_id), storage_ref_id = VALUES(storage_ref_id)`,
				sku, version, ref.ExpiresAt, parent, storage); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *erpProductImageRepo) Finish(ctx context.Context, sku, token string, next time.Time, issue string) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE erp_product_image_checks
		SET next_check_at = IF(next_check_at < leased_until, LEAST(next_check_at, ?), ?),
		    last_error = ?, checked_at = IF(? = '', UTC_TIMESTAMP(6), checked_at),
		    lease_token = '', leased_until = NULL
		WHERE sku_code = ? AND lease_token = ?`, next, next, issue, issue, sku, token)
	return err
}

func (r *erpProductImageRepo) FindReference(ctx context.Context, taskID int64, sku string, ref domain.ReferenceFileRef) (*domain.TaskAsset, error) {
	if ref.RefID == "" && ref.StorageKey == "" {
		return nil, nil
	}
	row := r.db.db.QueryRowContext(ctx, `
		SELECT `+taskAssetSelectCols+`
		FROM task_assets ta LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		WHERE ta.task_id = ? AND ta.asset_type = 'reference'
		  AND (ta.scope_sku_code IS NULL OR ta.scope_sku_code = '' OR BINARY ta.scope_sku_code = ?)
		  AND ta.binding_state NOT IN ('staged', 'discarded')
		  AND ta.deleted_at IS NULL AND ta.cleaned_at IS NULL AND ta.access_revoked_at IS NULL AND ta.object_deleted_at IS NULL
		  AND ((? <> '' AND ta.storage_ref_id = ?) OR (? <> '' AND BINARY COALESCE(NULLIF(ta.storage_key, ''), asr.ref_key) = ?))
		ORDER BY (BINARY ta.scope_sku_code = ?) DESC, ta.id DESC LIMIT 1`,
		taskID, sku, ref.RefID, ref.RefID, ref.StorageKey, ref.StorageKey, sku)
	item, err := scanTaskAsset(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (r *erpProductImageRepo) ActiveAsset(ctx context.Context, id int64) (*domain.TaskAsset, error) {
	return r.db.readERPProductImageAsset(ctx, id)
}

func (d *DB) readERPProductImageAsset(ctx context.Context, id int64) (*domain.TaskAsset, error) {
	row := d.db.QueryRowContext(ctx, `
		SELECT `+taskAssetSelectCols+`
		FROM task_assets ta LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		WHERE ta.id = ? AND ta.deleted_at IS NULL AND ta.cleaned_at IS NULL
		  AND ta.access_revoked_at IS NULL AND ta.object_deleted_at IS NULL
		  AND ta.binding_state NOT IN ('staged', 'discarded')
		  AND COALESCE(asr.status, '') NOT IN ('archived', 'historical_unavailable')`, id)
	return scanTaskAsset(row)
}

func (r *taskAssetRepo) GetERPProductImageAsset(ctx context.Context, id int64) (*domain.TaskAsset, error) {
	return r.db.readERPProductImageAsset(ctx, id)
}

func (r *erpProductImageRepo) Target(ctx context.Context, sku string) (repo.ERPProductImageTarget, error) {
	var target repo.ERPProductImageTarget
	var tasks, versions int
	err := r.db.db.QueryRowContext(ctx, `
		SELECT COALESCE(MIN(task_id), 0), COALESCE(MIN(task_no), ''), COALESCE(MIN(image_asset_version_id), 0),
		       COUNT(DISTINCT task_id), COUNT(DISTINCT image_asset_version_id)
		FROM erp_product_sync_records WHERE sku_code = ? AND image_required = 1`, sku).
		Scan(&target.TaskID, &target.TaskNo, &target.VersionID, &tasks, &versions)
	if tasks != 1 {
		return repo.ERPProductImageTarget{}, err
	}
	if versions > 1 {
		target.VersionID = 0
	}
	err = r.db.db.QueryRowContext(ctx, `
		SELECT NOT EXISTS(SELECT 1 FROM erp_product_sync_records WHERE task_id = ? AND sku_code <> '' AND sku_code <> ?)
		       AND NOT EXISTS(SELECT 1 FROM task_sku_items WHERE task_id = ? AND sku_code <> '' AND sku_code <> ?)`,
		target.TaskID, sku, target.TaskID, sku).Scan(&target.SingleSKU)
	return target, err
}

// ERP-owned assets are pinned before ERP writes and remain pinned until an
// authoritative readback releases them. Unchecked legacy task swaps fail closed.
const unreferencedERPProductImageSQL = `
  AND NOT EXISTS (
    SELECT 1 FROM erp_product_image_references pin WHERE pin.asset_version_id = asset.id
  )
  AND NOT EXISTS (
    SELECT 1 FROM erp_product_image_references pin WHERE pin.source_version_id = asset.id
  )
  AND NOT EXISTS (
    SELECT 1 FROM erp_product_image_references pin WHERE pin.storage_ref_id = asset.storage_ref_id
  )
  AND NOT EXISTS (
    SELECT 1 FROM erp_product_sync_records pm
    LEFT JOIN erp_product_image_checks image_check ON image_check.sku_code = pm.sku_code
    WHERE pm.task_id = asset.task_id AND pm.sku_code <> '' AND pm.image_required = 1
      AND (pm.base_sync_status = 'synced' OR pm.erp_sync_status = 'synced' OR pm.last_image_synced_at IS NOT NULL)
      AND (image_check.checked_at IS NULL OR image_check.last_error <> '')
  )`

func (r *erpProductImageRepo) RequestReference(ctx context.Context, sku string, request repo.ERPProductImageRequest, at time.Time) error {
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO erp_product_image_checks (sku_code, next_check_at, requested_task_id, requested_ref_id)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE next_check_at = LEAST(next_check_at, VALUES(next_check_at)),
		  requested_task_id = VALUES(requested_task_id), requested_ref_id = VALUES(requested_ref_id)`,
		sku, at, request.TaskID, request.RefID)
	return err
}

func (r *erpProductImageRepo) PendingReference(ctx context.Context, sku string) (repo.ERPProductImageRequest, error) {
	var request repo.ERPProductImageRequest
	err := r.db.db.QueryRowContext(ctx, `
		SELECT COALESCE(requested_task_id, 0), COALESCE(requested_ref_id, '')
		FROM erp_product_image_checks WHERE sku_code = ?`, sku).Scan(&request.TaskID, &request.RefID)
	return request, err
}

func (r *erpProductImageRepo) ClearReference(ctx context.Context, sku string, request repo.ERPProductImageRequest) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE erp_product_image_checks SET requested_task_id = NULL, requested_ref_id = NULL
		WHERE sku_code = ? AND requested_task_id = ? AND requested_ref_id = ?`, sku, request.TaskID, request.RefID)
	return err
}

func (r *erpProductImageRepo) FindLegacyObject(ctx context.Context, sku, key string) (int64, error) {
	var id int64
	selectID := func() error {
		return r.db.db.QueryRowContext(ctx, `
			SELECT id FROM erp_product_image_legacy_objects
			WHERE sku_code = ? AND object_key_hash = UNHEX(SHA2(?, 256)) AND BINARY object_key = ?`,
			sku, key, key).Scan(&id)
	}
	if err := selectID(); !errors.Is(err, sql.ErrNoRows) {
		return id, err
	}
	_, err := r.db.db.ExecContext(ctx, `
		INSERT IGNORE INTO erp_product_image_legacy_objects (sku_code, object_key, object_key_hash)
		VALUES (?, ?, UNHEX(SHA2(?, 256)))`, sku, key, key)
	if err != nil {
		return 0, err
	}
	err = selectID()
	return id, err
}

func (r *erpProductImageRepo) LegacyObjectKey(ctx context.Context, id int64) (string, error) {
	var key string
	err := r.db.db.QueryRowContext(ctx, `SELECT object_key FROM erp_product_image_legacy_objects WHERE id = ?`, id).Scan(&key)
	return key, err
}

func (r *erpProductImageRepo) LegacyObjectKeyForSKU(ctx context.Context, id int64, sku string) (string, error) {
	var key string
	err := r.db.db.QueryRowContext(ctx, `
		SELECT object_key FROM erp_product_image_legacy_objects
		WHERE id = ? AND BINARY sku_code = ?`, id, sku).Scan(&key)
	return key, err
}

func (r *erpProductImageRepo) HasRegisteredObject(ctx context.Context, taskID int64, key string) (bool, error) {
	var found bool
	err := r.db.db.QueryRowContext(ctx, `
		SELECT EXISTS(
		  SELECT 1 FROM task_assets ta LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		  WHERE ta.task_id = ? AND (BINARY ta.storage_key = ? OR BINARY asr.ref_key = ?)
		)`, taskID, key, key).Scan(&found)
	return found, err
}

func (r *erpProductImageRepo) Schedule(ctx context.Context, sku string, at time.Time) error {
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO erp_product_image_checks (sku_code, next_check_at) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE next_check_at = LEAST(next_check_at, VALUES(next_check_at))`, sku, at)
	return err
}

func (r *erpProductImageRepo) ActiveObjectVersions(ctx context.Context, sku, key string) ([]int64, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT DISTINCT ta.id
		FROM erp_product_sync_records pm JOIN task_assets ta ON ta.task_id = pm.task_id
		LEFT JOIN asset_storage_refs ar ON ar.ref_id = ta.storage_ref_id
		WHERE pm.sku_code = ? AND (BINARY ta.storage_key = ? OR BINARY ar.ref_key = ?)
		  AND ta.deleted_at IS NULL AND ta.cleaned_at IS NULL AND ta.access_revoked_at IS NULL AND ta.object_deleted_at IS NULL
		  AND ta.binding_state NOT IN ('staged', 'discarded')
		ORDER BY ta.id`, sku, key, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var versions []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		versions = append(versions, id)
	}
	return versions, rows.Err()
}

func (r *erpProductImageRepo) RequiresERPImageSet(ctx context.Context, sku string) (bool, error) {
	var found bool
	err := r.db.db.QueryRowContext(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM erp_product_sync_records p
		  JOIN tasks t ON t.id = p.task_id
		  JOIN task_asset_groups g ON g.task_id = p.task_id
		  JOIN task_asset_group_revisions rev
		    ON rev.id = COALESCE(g.finalized_revision_id, g.working_revision_id)
		  LEFT JOIN task_sku_items si ON si.id = g.task_sku_item_id
		  WHERE p.sku_code = ? AND BINARY p.sku_code = ?
		    AND rev.mode = 'set'
		    AND ((g.scope_kind = 'sku' AND (BINARY si.sku_code = ? OR g.task_sku_item_id = p.task_sku_item_id))
		      OR (g.scope_kind = 'task' AND BINARY COALESCE(NULLIF(t.primary_sku_code, ''), t.sku_code) = ?))
		)`, sku, sku, sku, sku).Scan(&found)
	return found, err
}
