package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type productManagementRepo struct{ db *DB }

func NewProductManagementRepo(db *DB) repo.ProductManagementRepo {
	return &productManagementRepo{db: db}
}

const productManagementSelectCols = `
	id, record_key, task_id, task_sku_item_id, task_no, task_type, source_mode,
	sku_code, product_i_id, erp_i_id, category_name, product_family,
	product_name, cost_price, creator_id, creator_name, task_created_at,
	image_source, image_selection_mode, image_asset_id, image_asset_version_id,
	image_filename, image_mime_type, image_missing_reason, image_sync_source,
	erp_sync_status, base_sync_status, image_sync_status,
	last_erp_checked_at, last_erp_synced_at, last_base_synced_at, last_image_synced_at,
	sync_cooldown_until, last_sync_error, base_sync_error, image_sync_error, image_required,
	created_at, updated_at`

func (r *productManagementRepo) RefreshReadModel(ctx context.Context) error {
	if err := r.refreshMainTaskRecords(ctx); err != nil {
		return err
	}
	return r.refreshSKUItemRecords(ctx)
}

func (r *productManagementRepo) refreshMainTaskRecords(ctx context.Context) error {
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO erp_product_sync_records (
		  record_key, task_id, task_sku_item_id, task_no, task_type, source_mode,
		  sku_code, product_i_id, erp_i_id, category_name, product_family,
		  product_name, cost_price, creator_id, creator_name, task_created_at,
		  erp_sync_status, base_sync_status, image_sync_status, last_erp_synced_at, last_base_synced_at
		)
		SELECT
		  CONCAT('task:', t.id, ':main'),
		  t.id,
		  NULL,
		  COALESCE(t.task_no, ''),
		  COALESCE(t.task_type, ''),
		  COALESCE(t.source_mode, ''),
		  COALESCE(t.sku_code, ''),
		  COALESCE(
		    NULLIF(CASE WHEN JSON_VALID(td.product_selection_snapshot_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.i_id')) ELSE '' END, ''),
		    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.product.i_id')) ELSE '' END, ''),
		    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')) ELSE '' END, ''),
		    ''
		  ),
		  COALESCE(
		    NULLIF(CASE WHEN JSON_VALID(td.product_selection_snapshot_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.i_id')) ELSE '' END, ''),
		    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.product.i_id')) ELSE '' END, ''),
		    NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')) ELSE '' END, ''),
		    ''
		  ),
		  COALESCE(NULLIF(td.category_name, ''), NULLIF(td.category, ''), ''),
		  COALESCE(NULLIF(td.category_name, ''), NULLIF(td.category, ''), ''),
		  COALESCE(NULLIF(td.product_short_name, ''), NULLIF(t.product_name_snapshot, ''), ''),
		  td.cost_price,
		  COALESCE(t.creator_id, 0),
		  COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username, ''), ''),
		  t.created_at,
		  CASE
		    WHEN td.filing_status = 'filed' THEN 'synced'
		    WHEN td.filing_status = 'filing_failed' THEN 'failed'
		    ELSE 'pending_sync'
		  END,
		  CASE
		    WHEN td.filing_status = 'filed' THEN 'synced'
		    WHEN td.filing_status = 'filing_failed' THEN 'failed'
		    ELSE 'pending_sync'
		  END,
		  'waiting_image',
		  td.last_filed_at,
		  td.last_filed_at
		FROM tasks t
		JOIN task_details td ON td.task_id = t.id
		LEFT JOIN users u ON u.id = t.creator_id
		WHERE COALESCE(t.sku_code, '') <> ''
		  AND NOT EXISTS (
		    SELECT 1 FROM task_sku_items tsi WHERE tsi.task_id = t.id
		  )
		ON DUPLICATE KEY UPDATE
		  task_no = VALUES(task_no),
		  task_type = VALUES(task_type),
		  source_mode = VALUES(source_mode),
		  erp_sync_status = CASE
		    WHEN VALUES(erp_sync_status) = 'synced' THEN 'synced'
		    WHEN erp_product_sync_records.erp_sync_status IN ('queued', 'cooling_down', 'syncing') THEN erp_product_sync_records.erp_sync_status
		    WHEN erp_product_sync_records.erp_sync_status = 'synced'
		      AND (
		        NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		        OR NOT (erp_product_sync_records.product_i_id <=> VALUES(product_i_id))
		        OR NOT (erp_product_sync_records.product_name <=> VALUES(product_name))
		        OR NOT (erp_product_sync_records.cost_price <=> VALUES(cost_price))
		      ) THEN 'pending_sync'
		    ELSE erp_product_sync_records.erp_sync_status
		  END,
		  base_sync_status = CASE
		    WHEN VALUES(base_sync_status) = 'synced' THEN 'synced'
		    WHEN erp_product_sync_records.base_sync_status IN ('queued', 'cooling_down', 'syncing') THEN erp_product_sync_records.base_sync_status
		    WHEN erp_product_sync_records.base_sync_status = 'synced'
		      AND (
		        NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		        OR NOT (erp_product_sync_records.erp_i_id <=> VALUES(erp_i_id))
		        OR NOT (erp_product_sync_records.product_name <=> VALUES(product_name))
		        OR NOT (erp_product_sync_records.cost_price <=> VALUES(cost_price))
		      ) THEN 'pending_sync'
		    ELSE erp_product_sync_records.base_sync_status
		  END,
		  sku_code = VALUES(sku_code),
		  product_i_id = VALUES(product_i_id),
		  erp_i_id = VALUES(erp_i_id),
		  category_name = VALUES(category_name),
		  product_family = VALUES(product_family),
		  product_name = VALUES(product_name),
		  cost_price = VALUES(cost_price),
		  creator_id = VALUES(creator_id),
		  creator_name = VALUES(creator_name),
		  task_created_at = VALUES(task_created_at),
		  last_erp_synced_at = COALESCE(VALUES(last_erp_synced_at), erp_product_sync_records.last_erp_synced_at),
		  last_base_synced_at = COALESCE(VALUES(last_base_synced_at), erp_product_sync_records.last_base_synced_at)`)
	if err != nil {
		return fmt.Errorf("refresh product management main task records: %w", err)
	}
	return nil
}

func (r *productManagementRepo) refreshSKUItemRecords(ctx context.Context) error {
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO erp_product_sync_records (
		  record_key, task_id, task_sku_item_id, task_no, task_type, source_mode,
		  sku_code, product_i_id, erp_i_id, category_name, product_family,
		  product_name, cost_price, creator_id, creator_name, task_created_at,
		  erp_sync_status, base_sync_status, image_sync_status, last_erp_synced_at, last_base_synced_at
		)
		SELECT
		  CONCAT('task:', t.id, ':sku:', tsi.id),
		  t.id,
		  tsi.id,
		  COALESCE(t.task_no, ''),
		  COALESCE(t.task_type, ''),
		  COALESCE(t.source_mode, ''),
		  COALESCE(tsi.sku_code, ''),
		  COALESCE(
		    NULLIF(CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.product_i_id')) ELSE '' END, ''),
		    NULLIF(CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.i_id')) ELSE '' END, ''),
		    ''
		  ),
		  COALESCE(
		    NULLIF(CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.product_i_id')) ELSE '' END, ''),
		    NULLIF(CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.i_id')) ELSE '' END, ''),
		    ''
		  ),
		  COALESCE(NULLIF(td.category_name, ''), NULLIF(td.category, ''), ''),
		  COALESCE(NULLIF(CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.product_family')) ELSE '' END, ''), NULLIF(td.category_name, ''), NULLIF(td.category, ''), ''),
		  COALESCE(NULLIF(tsi.product_short_name, ''), NULLIF(tsi.product_name_snapshot, ''), NULLIF(t.product_name_snapshot, ''), ''),
		  tsi.cost_price,
		  COALESCE(t.creator_id, 0),
		  COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username, ''), ''),
		  t.created_at,
		  CASE
		    WHEN COALESCE(tsi.erp_sync_status, tsi.filing_status) = 'filed' THEN 'synced'
		    WHEN COALESCE(tsi.erp_sync_status, tsi.filing_status) = 'filing_failed' THEN 'failed'
		    ELSE 'pending_sync'
		  END,
		  CASE
		    WHEN COALESCE(tsi.erp_sync_status, tsi.filing_status) = 'filed' THEN 'synced'
		    WHEN COALESCE(tsi.erp_sync_status, tsi.filing_status) = 'filing_failed' THEN 'failed'
		    ELSE 'pending_sync'
		  END,
		  'waiting_image',
		  tsi.last_filed_at,
		  tsi.last_filed_at
		FROM task_sku_items tsi
		JOIN tasks t ON t.id = tsi.task_id
		JOIN task_details td ON td.task_id = t.id
		LEFT JOIN users u ON u.id = t.creator_id
		WHERE COALESCE(tsi.sku_code, '') <> ''
		ON DUPLICATE KEY UPDATE
		  task_no = VALUES(task_no),
		  task_type = VALUES(task_type),
		  source_mode = VALUES(source_mode),
		  erp_sync_status = CASE
		    WHEN VALUES(erp_sync_status) = 'synced' THEN 'synced'
		    WHEN erp_product_sync_records.erp_sync_status IN ('queued', 'cooling_down', 'syncing') THEN erp_product_sync_records.erp_sync_status
		    WHEN erp_product_sync_records.erp_sync_status = 'synced'
		      AND (
		        NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		        OR NOT (erp_product_sync_records.product_i_id <=> VALUES(product_i_id))
		        OR NOT (erp_product_sync_records.product_name <=> VALUES(product_name))
		        OR NOT (erp_product_sync_records.cost_price <=> VALUES(cost_price))
		      ) THEN 'pending_sync'
		    ELSE erp_product_sync_records.erp_sync_status
		  END,
		  base_sync_status = CASE
		    WHEN VALUES(base_sync_status) = 'synced' THEN 'synced'
		    WHEN erp_product_sync_records.base_sync_status IN ('queued', 'cooling_down', 'syncing') THEN erp_product_sync_records.base_sync_status
		    WHEN erp_product_sync_records.base_sync_status = 'synced'
		      AND (
		        NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		        OR NOT (erp_product_sync_records.erp_i_id <=> VALUES(erp_i_id))
		        OR NOT (erp_product_sync_records.product_name <=> VALUES(product_name))
		        OR NOT (erp_product_sync_records.cost_price <=> VALUES(cost_price))
		      ) THEN 'pending_sync'
		    ELSE erp_product_sync_records.base_sync_status
		  END,
		  sku_code = VALUES(sku_code),
		  product_i_id = VALUES(product_i_id),
		  erp_i_id = VALUES(erp_i_id),
		  category_name = VALUES(category_name),
		  product_family = VALUES(product_family),
		  product_name = VALUES(product_name),
		  cost_price = VALUES(cost_price),
		  creator_id = VALUES(creator_id),
		  creator_name = VALUES(creator_name),
		  task_created_at = VALUES(task_created_at),
		  last_erp_synced_at = COALESCE(VALUES(last_erp_synced_at), erp_product_sync_records.last_erp_synced_at),
		  last_base_synced_at = COALESCE(VALUES(last_base_synced_at), erp_product_sync_records.last_base_synced_at)`)
	if err != nil {
		return fmt.Errorf("refresh product management sku item records: %w", err)
	}
	return nil
}

func (r *productManagementRepo) List(ctx context.Context, filter repo.ProductManagementListFilter) ([]*domain.ProductManagementRecord, int64, error) {
	filter.Page, filter.PageSize = normalizePage(filter.Page, filter.PageSize)
	where, args := buildProductManagementWhere(filter)

	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM erp_product_sync_records `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count product management records: %w", err)
	}
	args = append(args, (filter.Page-1)*filter.PageSize, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, `SELECT `+productManagementSelectCols+` FROM erp_product_sync_records `+where+`
		ORDER BY updated_at DESC, task_created_at DESC, id DESC
		LIMIT ?, ?`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list product management records: %w", err)
	}
	defer rows.Close()
	return scanProductManagementRows(rows, total)
}

func (r *productManagementRepo) GetByID(ctx context.Context, id int64) (*domain.ProductManagementRecord, error) {
	row := r.db.db.QueryRowContext(ctx, `SELECT `+productManagementSelectCols+` FROM erp_product_sync_records WHERE id = ?`, id)
	return scanProductManagementRecord(row)
}

func (r *productManagementRepo) GetByTaskID(ctx context.Context, taskID int64) ([]*domain.ProductManagementRecord, error) {
	rows, err := r.db.db.QueryContext(ctx, `SELECT `+productManagementSelectCols+` FROM erp_product_sync_records WHERE task_id = ? ORDER BY task_sku_item_id IS NULL DESC, id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list product management records by task: %w", err)
	}
	defer rows.Close()
	items, _, err := scanProductManagementRows(rows, 0)
	return items, err
}

func (r *productManagementRepo) ClaimQueuedSyncRecords(ctx context.Context, limit int, claimToken string, now time.Time) ([]*domain.ProductManagementRecord, error) {
	limit = normalizeProductManagementClaimLimit(limit)
	claimToken = strings.TrimSpace(claimToken)
	if claimToken == "" {
		return nil, fmt.Errorf("claim token is required")
	}
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin product management sync claim: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `
		UPDATE erp_product_sync_records
		   SET erp_sync_status = 'syncing',
		       base_sync_status = CASE
		         WHEN base_sync_status IN ('queued', 'cooling_down')
		           OR (base_sync_status = 'syncing' AND (last_erp_checked_at IS NULL OR last_erp_checked_at <= DATE_SUB(?, INTERVAL 10 MINUTE)))
		         THEN 'syncing'
		         ELSE base_sync_status
		       END,
		       image_sync_status = CASE
		         WHEN image_sync_status IN ('queued', 'cooling_down')
		           OR (image_sync_status = 'syncing' AND (last_erp_checked_at IS NULL OR last_erp_checked_at <= DATE_SUB(?, INTERVAL 10 MINUTE)))
		         THEN 'syncing'
		         ELSE image_sync_status
		       END,
		       sync_claim_token = ?,
		       last_erp_checked_at = ?,
		       last_sync_error = ''
		 WHERE erp_sync_status = 'queued'
		    OR (erp_sync_status = 'cooling_down' AND (sync_cooldown_until IS NULL OR sync_cooldown_until <= ?))
		    OR (erp_sync_status = 'syncing' AND (last_erp_checked_at IS NULL OR last_erp_checked_at <= DATE_SUB(?, INTERVAL 10 MINUTE)))
		 ORDER BY COALESCE(last_erp_checked_at, created_at), updated_at, id
		 LIMIT ?`,
		now, now, claimToken, now, now, now, limit,
	); err != nil {
		return nil, fmt.Errorf("claim product management sync records: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+productManagementSelectCols+` FROM erp_product_sync_records WHERE sync_claim_token = ? ORDER BY last_erp_checked_at, id`, claimToken)
	if err != nil {
		return nil, fmt.Errorf("list claimed product management sync records: %w", err)
	}
	items, _, scanErr := scanProductManagementRows(rows, 0)
	closeErr := rows.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit product management sync claim: %w", err)
	}
	committed = true
	return items, nil
}

func (r *productManagementRepo) UpdateImage(ctx context.Context, tx repo.Tx, id int64, patch repo.ProductManagementImagePatch) error {
	sqlTx := Unwrap(tx)
	imageSyncSource := patch.ImageSyncSource
	if imageSyncSource == "" {
		imageSyncSource = patch.ImageSource
	}
	imageSyncStatus := patch.ImageSyncStatus
	if imageSyncStatus == "" {
		imageSyncStatus = domain.ProductManagementERPSyncStatusWaitingImage
		if patch.ImageAssetID != nil && *patch.ImageAssetID > 0 {
			imageSyncStatus = domain.ProductManagementERPSyncStatusPendingSync
		}
	}
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE erp_product_sync_records
		   SET image_source = ?,
		       image_selection_mode = ?,
		       image_asset_id = ?,
		       image_asset_version_id = ?,
		       image_filename = ?,
		       image_mime_type = ?,
		       image_missing_reason = ?,
		       image_sync_source = ?,
		       image_sync_status = ?,
		       image_sync_error = CASE WHEN ? = 'waiting_image' THEN image_missing_reason ELSE '' END,
		       erp_sync_status = CASE WHEN erp_sync_status = 'synced' THEN 'pending_sync' ELSE erp_sync_status END
		 WHERE id = ?`,
		string(patch.ImageSource),
		string(patch.ImageSelectionMode),
		toNullInt64(patch.ImageAssetID),
		toNullInt64(patch.ImageAssetVersionID),
		strings.TrimSpace(patch.ImageFilename),
		strings.TrimSpace(patch.ImageMimeType),
		strings.TrimSpace(patch.ImageMissingReason),
		string(imageSyncSource),
		string(imageSyncStatus),
		string(imageSyncStatus),
		id,
	)
	if err != nil {
		return fmt.Errorf("update product management image: %w", err)
	}
	return nil
}

func (r *productManagementRepo) UpdateSyncStatus(ctx context.Context, tx repo.Tx, id int64, patch repo.ProductManagementSyncPatch) error {
	sqlTx := Unwrap(tx)
	baseStatus := patch.BaseStatus
	if baseStatus == "" {
		baseStatus = patch.Status
	}
	imageStatus := patch.ImageStatus
	if imageStatus == "" {
		imageStatus = patch.Status
	}
	baseErr := patch.BaseSyncError
	if baseErr == "" {
		baseErr = patch.LastSyncError
	}
	imageErr := patch.ImageSyncError
	if imageErr == "" {
		imageErr = patch.LastSyncError
	}
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE erp_product_sync_records
		   SET erp_sync_status = ?,
		       base_sync_status = ?,
		       image_sync_status = ?,
		       last_erp_checked_at = ?,
		       last_erp_synced_at = COALESCE(?, last_erp_synced_at),
		       last_base_synced_at = COALESCE(?, last_base_synced_at),
		       last_image_synced_at = COALESCE(?, last_image_synced_at),
		       sync_cooldown_until = ?,
		       sync_claim_token = '',
		       last_sync_error = ?,
		       base_sync_error = ?,
		       image_sync_error = ?
		 WHERE id = ?`,
		string(patch.Status),
		string(baseStatus),
		string(imageStatus),
		toNullTime(patch.LastERPCheckedAt),
		toNullTime(patch.LastERPSyncedAt),
		toNullTime(patch.LastBaseSyncedAt),
		toNullTime(patch.LastImageSyncedAt),
		toNullTime(patch.SyncCooldownUntil),
		strings.TrimSpace(patch.LastSyncError),
		strings.TrimSpace(baseErr),
		strings.TrimSpace(imageErr),
		id,
	)
	if err != nil {
		return fmt.Errorf("update product management sync status: %w", err)
	}
	return nil
}

func (r *productManagementRepo) UpdateBaseSyncStatus(ctx context.Context, tx repo.Tx, id int64, patch repo.ProductManagementSyncPatch) error {
	sqlTx := Unwrap(tx)
	status := patch.BaseStatus
	if status == "" {
		status = patch.Status
	}
	overallStatus := patch.Status
	if overallStatus == "" {
		overallStatus = status
	}
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE erp_product_sync_records
		   SET erp_sync_status = ?,
		       base_sync_status = ?,
		       last_erp_checked_at = ?,
		       last_erp_synced_at = COALESCE(?, last_erp_synced_at),
		       last_base_synced_at = COALESCE(?, last_base_synced_at),
		       sync_cooldown_until = ?,
		       sync_claim_token = '',
		       last_sync_error = ?,
		       base_sync_error = ?
		 WHERE id = ?`,
		string(overallStatus),
		string(status),
		toNullTime(patch.LastERPCheckedAt),
		toNullTime(patch.LastERPSyncedAt),
		toNullTime(patch.LastBaseSyncedAt),
		toNullTime(patch.SyncCooldownUntil),
		strings.TrimSpace(patch.LastSyncError),
		strings.TrimSpace(patch.BaseSyncError),
		id,
	)
	if err != nil {
		return fmt.Errorf("update product management base sync status: %w", err)
	}
	return nil
}

func (r *productManagementRepo) UpdateImageSyncStatus(ctx context.Context, tx repo.Tx, id int64, patch repo.ProductManagementSyncPatch) error {
	sqlTx := Unwrap(tx)
	status := patch.ImageStatus
	if status == "" {
		status = patch.Status
	}
	overallStatus := patch.Status
	if overallStatus == "" {
		overallStatus = status
	}
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE erp_product_sync_records
		   SET erp_sync_status = ?,
		       image_sync_status = ?,
		       last_erp_checked_at = ?,
		       last_erp_synced_at = COALESCE(?, last_erp_synced_at),
		       last_image_synced_at = COALESCE(?, last_image_synced_at),
		       sync_cooldown_until = ?,
		       sync_claim_token = '',
		       last_sync_error = ?,
		       image_sync_error = ?
		 WHERE id = ?`,
		string(overallStatus),
		string(status),
		toNullTime(patch.LastERPCheckedAt),
		toNullTime(patch.LastERPSyncedAt),
		toNullTime(patch.LastImageSyncedAt),
		toNullTime(patch.SyncCooldownUntil),
		strings.TrimSpace(patch.LastSyncError),
		strings.TrimSpace(patch.ImageSyncError),
		id,
	)
	if err != nil {
		return fmt.Errorf("update product management image sync status: %w", err)
	}
	return nil
}

func normalizeProductManagementClaimLimit(limit int) int {
	if limit < 1 {
		return 10
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func buildProductManagementWhere(filter repo.ProductManagementListFilter) (string, []interface{}) {
	clauses := []string{"1 = 1"}
	args := make([]interface{}, 0, 12)
	keyword := strings.TrimSpace(filter.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		clauses = append(clauses, `(
			sku_code LIKE ?
			OR task_no LIKE ?
			OR product_i_id LIKE ?
			OR erp_i_id LIKE ?
			OR product_name LIKE ?
			OR category_name LIKE ?
			OR creator_name LIKE ?
			OR CAST(creator_id AS CHAR) = ?
		)`)
		args = append(args, like, like, like, like, like, like, like, keyword)
	}
	if source := strings.TrimSpace(filter.ImageSource); source != "" {
		clauses = append(clauses, "image_source = ?")
		args = append(args, source)
	}
	if status := strings.TrimSpace(filter.SyncStatus); status != "" {
		clauses = append(clauses, "erp_sync_status = ?")
		args = append(args, status)
	}
	if status := strings.TrimSpace(filter.BaseSyncStatus); status != "" {
		clauses = append(clauses, "base_sync_status = ?")
		args = append(args, status)
	}
	if status := strings.TrimSpace(filter.ImageSyncStatus); status != "" {
		clauses = append(clauses, "image_sync_status = ?")
		args = append(args, status)
	}
	switch strings.TrimSpace(filter.CostStatus) {
	case "missing":
		clauses = append(clauses, "(cost_price IS NULL OR cost_price <= 0)")
	case "ready":
		clauses = append(clauses, "cost_price IS NOT NULL AND cost_price > 0")
	}
	if filter.CreatorID != nil && *filter.CreatorID > 0 {
		clauses = append(clauses, "creator_id = ?")
		args = append(args, *filter.CreatorID)
	}
	if strings.TrimSpace(filter.IssueScope) == "attention" {
		clauses = append(clauses, `(
			cost_price IS NULL
			OR cost_price <= 0
			OR base_sync_status IN ('pending_sync', 'failed', 'queued', 'cooling_down', 'syncing')
			OR image_sync_status IN ('pending_sync', 'waiting_image', 'failed', 'queued', 'cooling_down', 'syncing')
		)`)
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func scanProductManagementRows(rows *sql.Rows, total int64) ([]*domain.ProductManagementRecord, int64, error) {
	var items []*domain.ProductManagementRecord
	for rows.Next() {
		item, err := scanProductManagementRecordScanner(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

type productManagementScanner interface {
	Scan(dest ...interface{}) error
}

func scanProductManagementRecord(row *sql.Row) (*domain.ProductManagementRecord, error) {
	item, err := scanProductManagementRecordScanner(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func scanProductManagementRecordScanner(scanner productManagementScanner) (*domain.ProductManagementRecord, error) {
	var item domain.ProductManagementRecord
	var taskSKUItemID, imageAssetID, imageAssetVersionID sql.NullInt64
	var costPrice sql.NullFloat64
	var lastERPCheckedAt, lastERPSyncedAt, lastBaseSyncedAt, lastImageSyncedAt, syncCooldownUntil sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.RecordKey,
		&item.TaskID,
		&taskSKUItemID,
		&item.TaskNo,
		&item.TaskType,
		&item.SourceMode,
		&item.SKUCode,
		&item.ProductIID,
		&item.ERPIID,
		&item.CategoryName,
		&item.ProductFamily,
		&item.ProductName,
		&costPrice,
		&item.CreatorID,
		&item.CreatorName,
		&item.TaskCreatedAt,
		&item.ImageSource,
		&item.ImageSelectionMode,
		&imageAssetID,
		&imageAssetVersionID,
		&item.ImageFilename,
		&item.ImageMimeType,
		&item.ImageMissingReason,
		&item.ImageSyncSource,
		&item.ERPSyncStatus,
		&item.BaseSyncStatus,
		&item.ImageSyncStatus,
		&lastERPCheckedAt,
		&lastERPSyncedAt,
		&lastBaseSyncedAt,
		&lastImageSyncedAt,
		&syncCooldownUntil,
		&item.LastSyncError,
		&item.BaseSyncError,
		&item.ImageSyncError,
		&item.ImageRequired,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan product management record: %w", err)
	}
	item.TaskSKUItemID = fromNullInt64(taskSKUItemID)
	item.CostPrice = fromNullFloat64(costPrice)
	item.ImageAssetID = fromNullInt64(imageAssetID)
	item.ImageAssetVersionID = fromNullInt64(imageAssetVersionID)
	item.LastERPCheckedAt = fromNullTime(lastERPCheckedAt)
	item.LastERPSyncedAt = fromNullTime(lastERPSyncedAt)
	item.LastBaseSyncedAt = fromNullTime(lastBaseSyncedAt)
	item.LastImageSyncedAt = fromNullTime(lastImageSyncedAt)
	item.SyncCooldownUntil = fromNullTime(syncCooldownUntil)
	if strings.TrimSpace(item.ERPIID) == "" {
		item.ERPIID = strings.TrimSpace(item.ProductIID)
	}
	item.ImageSourceLabel = domain.ProductManagementImageSourceLabel(item.ImageSource)
	return &item, nil
}
