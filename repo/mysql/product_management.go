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

type productManagementRepo struct{ db *DB }

func NewProductManagementRepo(db *DB) repo.ProductManagementRepo {
	return &productManagementRepo{db: db}
}

const productManagementSelectCols = `
	pm.id, pm.record_key, pm.task_id, pm.task_sku_item_id, pm.task_no, pm.task_type, pm.source_mode,
	pm.sku_code, pm.product_i_id, pm.erp_i_id, pm.category_name, pm.product_family,
	pm.product_name, pm.cost_price, pm.creator_id, pm.creator_name, pm.task_created_at,
	pm.image_source, pm.image_selection_mode, pm.image_asset_id, pm.image_asset_version_id,
	pm.image_filename, pm.image_mime_type, pm.image_missing_reason, pm.image_sync_source,
	pm.erp_sync_status, pm.base_sync_status, pm.image_sync_status,
	pm.last_erp_checked_at, pm.last_erp_synced_at, pm.last_base_synced_at, pm.last_image_synced_at,
	pm.sync_cooldown_until, pm.last_sync_error, pm.base_sync_error, pm.image_sync_error, pm.image_required,
	pm.created_at, pm.updated_at,
	cost_snapshot.cost_rule_name, cost_snapshot.cost_rule_source, cost_snapshot.matched_rule_version,
	cost_snapshot.prefill_source, cost_snapshot.requires_manual_review, cost_snapshot.manual_cost_override,
	cost_snapshot.manual_cost_override_reason, cost_snapshot.input_snapshot_json,
	cost_snapshot.calculation_snapshot_json, cost_snapshot.created_at`

const productManagementCostTraceJoin = `
	LEFT JOIN omp_sku_cost_snapshots cost_snapshot
	  ON cost_snapshot.id = (
	    SELECT s.id
	      FROM omp_sku_cost_snapshots s
	     WHERE s.sku_code = pm.sku_code
	       AND (
	         (pm.task_sku_item_id IS NOT NULL AND s.task_sku_item_id = pm.task_sku_item_id)
	         OR (pm.task_sku_item_id IS NULL AND s.task_id = pm.task_id AND s.task_sku_item_id IS NULL)
	         OR s.task_id = pm.task_id
	         OR s.task_id IS NULL
	       )
	     ORDER BY
	       CASE
	         WHEN pm.task_sku_item_id IS NOT NULL AND s.task_sku_item_id = pm.task_sku_item_id THEN 0
	         WHEN pm.task_sku_item_id IS NULL AND s.task_id = pm.task_id AND s.task_sku_item_id IS NULL THEN 1
	         WHEN s.task_id = pm.task_id THEN 2
	         ELSE 3
	       END,
	       s.created_at DESC,
	       s.id DESC
	     LIMIT 1
	  )`

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
		  erp_sync_status, base_sync_status, image_sync_status, last_erp_synced_at, last_base_synced_at,
		  updated_at
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
		  td.last_filed_at,
		  GREATEST(t.updated_at, td.updated_at)
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
		  last_sync_error = CASE
		    WHEN VALUES(erp_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.erp_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at THEN ''
		    ELSE erp_product_sync_records.last_sync_error
		  END,
		  base_sync_error = CASE
		    WHEN VALUES(base_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.base_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at THEN ''
		    ELSE erp_product_sync_records.base_sync_error
		  END,
		  sync_cooldown_until = CASE
		    WHEN (
		      VALUES(erp_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.erp_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at
		    ) OR (
		      VALUES(base_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.base_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at
		    ) THEN NULL
		    ELSE erp_product_sync_records.sync_cooldown_until
		  END,
		  erp_sync_status = CASE
		    WHEN VALUES(erp_sync_status) = 'synced' THEN 'synced'
		    WHEN erp_product_sync_records.erp_sync_status IN ('queued', 'cooling_down', 'syncing') THEN erp_product_sync_records.erp_sync_status
		    WHEN VALUES(erp_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.erp_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at THEN 'pending_sync'
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
		    WHEN VALUES(base_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.base_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at THEN 'pending_sync'
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
		  erp_sync_status, base_sync_status, image_sync_status, last_erp_synced_at, last_base_synced_at,
		  updated_at
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
		  tsi.last_filed_at,
		  tsi.updated_at
		FROM task_sku_items tsi
		JOIN tasks t ON t.id = tsi.task_id
		JOIN task_details td ON td.task_id = t.id
		LEFT JOIN users u ON u.id = t.creator_id
		WHERE COALESCE(tsi.sku_code, '') <> ''
		ON DUPLICATE KEY UPDATE
		  task_no = VALUES(task_no),
		  task_type = VALUES(task_type),
		  source_mode = VALUES(source_mode),
		  last_sync_error = CASE
		    WHEN VALUES(erp_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.erp_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at THEN ''
		    ELSE erp_product_sync_records.last_sync_error
		  END,
		  base_sync_error = CASE
		    WHEN VALUES(base_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.base_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at THEN ''
		    ELSE erp_product_sync_records.base_sync_error
		  END,
		  sync_cooldown_until = CASE
		    WHEN (
		      VALUES(erp_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.erp_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at
		    ) OR (
		      VALUES(base_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.base_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at
		    ) THEN NULL
		    ELSE erp_product_sync_records.sync_cooldown_until
		  END,
		  erp_sync_status = CASE
		    WHEN VALUES(erp_sync_status) = 'synced' THEN 'synced'
		    WHEN erp_product_sync_records.erp_sync_status IN ('queued', 'cooling_down', 'syncing') THEN erp_product_sync_records.erp_sync_status
		    WHEN VALUES(erp_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.erp_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at THEN 'pending_sync'
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
		    WHEN VALUES(base_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.base_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at THEN 'pending_sync'
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
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM erp_product_sync_records pm `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count product management records: %w", err)
	}
	args = append(args, (filter.Page-1)*filter.PageSize, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, `SELECT `+productManagementSelectCols+` FROM erp_product_sync_records pm `+productManagementCostTraceJoin+` `+where+`
		ORDER BY pm.updated_at DESC, pm.task_created_at DESC, pm.id DESC
		LIMIT ?, ?`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list product management records: %w", err)
	}
	defer rows.Close()
	return scanProductManagementRows(rows, total)
}

func (r *productManagementRepo) GetByID(ctx context.Context, id int64) (*domain.ProductManagementRecord, error) {
	row := r.db.db.QueryRowContext(ctx, `SELECT `+productManagementSelectCols+` FROM erp_product_sync_records pm `+productManagementCostTraceJoin+` WHERE pm.id = ?`, id)
	return scanProductManagementRecord(row)
}

func (r *productManagementRepo) GetByTaskID(ctx context.Context, taskID int64) ([]*domain.ProductManagementRecord, error) {
	rows, err := r.db.db.QueryContext(ctx, `SELECT `+productManagementSelectCols+` FROM erp_product_sync_records pm `+productManagementCostTraceJoin+` WHERE pm.task_id = ? ORDER BY pm.task_sku_item_id IS NULL DESC, pm.id ASC`, taskID)
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
		    OR base_sync_status = 'queued'
		    OR (base_sync_status = 'cooling_down' AND (sync_cooldown_until IS NULL OR sync_cooldown_until <= ?))
		    OR (base_sync_status = 'syncing' AND (last_erp_checked_at IS NULL OR last_erp_checked_at <= DATE_SUB(?, INTERVAL 10 MINUTE)))
		    OR image_sync_status = 'queued'
		    OR (image_sync_status = 'cooling_down' AND (sync_cooldown_until IS NULL OR sync_cooldown_until <= ?))
		    OR (image_sync_status = 'syncing' AND (last_erp_checked_at IS NULL OR last_erp_checked_at <= DATE_SUB(?, INTERVAL 10 MINUTE)))
		 ORDER BY CASE
		            WHEN erp_sync_status = 'queued' OR base_sync_status = 'queued' OR image_sync_status = 'queued' THEN 0
		            WHEN erp_sync_status = 'syncing' OR base_sync_status = 'syncing' OR image_sync_status = 'syncing' THEN 1
		            ELSE 2
		          END,
		          COALESCE(last_erp_checked_at, created_at),
		          updated_at,
		          id
		 LIMIT ?`,
		now, now, claimToken, now, now, now, now, now, now, now, limit,
	); err != nil {
		return nil, fmt.Errorf("claim product management sync records: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+productManagementSelectCols+` FROM erp_product_sync_records pm `+productManagementCostTraceJoin+` WHERE pm.sync_claim_token = ? ORDER BY pm.last_erp_checked_at, pm.id`, claimToken)
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

func (r *productManagementRepo) QueuePendingBaseSyncByTaskID(ctx context.Context, tx repo.Tx, taskID int64, now time.Time, cooldownUntil time.Time) (int64, error) {
	if taskID <= 0 {
		return 0, nil
	}
	sqlTx := Unwrap(tx)
	result, err := sqlTx.ExecContext(ctx, `
		UPDATE erp_product_sync_records
		   SET erp_sync_status = 'queued',
		       base_sync_status = 'queued',
		       last_erp_checked_at = ?,
		       sync_cooldown_until = ?,
		       sync_claim_token = '',
		       last_sync_error = '',
		       base_sync_error = '',
		       updated_at = CURRENT_TIMESTAMP
		 WHERE task_id = ?
		   AND COALESCE(sku_code, '') <> ''
		   AND COALESCE(product_name, '') <> ''
		   AND COALESCE(NULLIF(erp_i_id, ''), NULLIF(product_i_id, ''), NULLIF(product_family, ''), NULLIF(category_name, '')) IS NOT NULL
		   AND base_sync_status IN ('pending_sync', 'failed')
		   AND erp_sync_status NOT IN ('queued', 'cooling_down', 'syncing')`,
		now,
		cooldownUntil,
		taskID,
	)
	if err != nil {
		return 0, fmt.Errorf("queue product management base sync by task: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read queued product management base sync count: %w", err)
	}
	return count, nil
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
		       image_sync_status = CASE
		         WHEN ? = 'pending_sync' AND erp_sync_status IN ('queued', 'cooling_down', 'syncing') THEN erp_sync_status
		         ELSE ?
		       END,
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

func (r *productManagementRepo) MarkBaseSyncProjectionSynced(ctx context.Context, tx repo.Tx, taskID int64, taskSKUItemID *int64, now time.Time) error {
	if taskID <= 0 {
		return nil
	}
	sqlTx := Unwrap(tx)
	if taskSKUItemID != nil && *taskSKUItemID > 0 {
		if _, err := sqlTx.ExecContext(ctx, `
			UPDATE task_sku_items
			   SET sku_status = 'filed',
			       filing_status = 'filed',
			       erp_sync_status = 'filed',
			       erp_sync_required = 0,
			       erp_sync_version = CASE
			         WHEN filing_status <> 'filed' OR erp_sync_status <> 'filed' OR erp_sync_required <> 0
			         THEN erp_sync_version + 1
			         ELSE erp_sync_version
			       END,
			       last_filed_at = ?,
			       filing_error_message = '',
			       updated_at = CURRENT_TIMESTAMP
			 WHERE task_id = ? AND id = ?`,
			now,
			taskID,
			*taskSKUItemID,
		); err != nil {
			return fmt.Errorf("mark task_sku_item product management base sync filed: %w", err)
		}
	}
	if _, err := sqlTx.ExecContext(ctx, `
		UPDATE task_details td
		   SET filing_status = 'filed',
		       erp_sync_required = 0,
		       erp_sync_version = CASE
		         WHEN td.filing_status <> 'filed' OR COALESCE(td.erp_sync_required, 0) <> 0
		         THEN td.erp_sync_version + 1
		         ELSE td.erp_sync_version
		       END,
		       last_filing_attempt_at = ?,
		       last_filed_at = ?,
		       filing_error_message = '',
		       filed_at = COALESCE(td.filed_at, ?),
		       updated_at = CURRENT_TIMESTAMP
		 WHERE td.task_id = ?
		   AND EXISTS (
		     SELECT 1
		       FROM erp_product_sync_records pm
		      WHERE pm.task_id = td.task_id
		   )
		   AND NOT EXISTS (
		     SELECT 1
		       FROM erp_product_sync_records pm
		      WHERE pm.task_id = td.task_id
		        AND pm.base_sync_status <> 'synced'
		   )
		   AND (
		     td.filing_status <> 'filed'
		     OR COALESCE(td.erp_sync_required, 0) <> 0
		     OR td.last_filed_at IS NULL
		   )`,
		now,
		now,
		now,
		taskID,
	); err != nil {
		return fmt.Errorf("mark task product management base sync filed: %w", err)
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
		kw := normalizeSearchKeyword(keyword)
		keywordClauses := []string{
			"pm.product_name LIKE ?",
			"pm.category_name LIKE ?",
			"pm.creator_name LIKE ?",
		}
		keywordArgs := []interface{}{kw.Like, kw.Like, kw.Like}
		if kw.HasInt64 {
			keywordClauses = append(keywordClauses, "pm.creator_id = ?")
			keywordArgs = append(keywordArgs, kw.Int64)
		}
		if kw.IsCode {
			keywordClauses = append(keywordClauses,
				"pm.sku_code = ?",
				"pm.task_no = ?",
				"pm.product_i_id = ?",
				"pm.erp_i_id = ?",
				"pm.sku_code LIKE ?",
				"pm.task_no LIKE ?",
				"pm.product_i_id LIKE ?",
				"pm.erp_i_id LIKE ?",
			)
			keywordArgs = append(keywordArgs, kw.Upper, kw.Upper, kw.Upper, kw.Upper, kw.Upper+"%", kw.Upper+"%", kw.Upper+"%", kw.Upper+"%")
		} else {
			keywordClauses = append(keywordClauses,
				"pm.sku_code LIKE ?",
				"pm.task_no LIKE ?",
				"pm.product_i_id LIKE ?",
				"pm.erp_i_id LIKE ?",
			)
			keywordArgs = append(keywordArgs, kw.Like, kw.Like, kw.Like, kw.Like)
		}
		keywordClauses = append(keywordClauses, `EXISTS (
			  SELECT 1
			    FROM omp_sku_combo_relations rel
				    LEFT JOIN omp_sku_combo_records rec ON rec.combo_sku_code = rel.combo_sku_code
				   WHERE rel.child_sku_code = pm.sku_code
			     AND (
			       rel.combo_sku_code = ?
			       OR COALESCE(rec.erp_i_id, '') = ?
			       OR rel.combo_sku_code LIKE ?
			       OR COALESCE(rec.erp_i_id, '') LIKE ?
			       OR rec.name LIKE ?
			       OR rec.short_name LIKE ?
			     )
			)`)
		keywordArgs = append(keywordArgs, kw.Upper, kw.Upper, kw.Upper+"%", kw.Upper+"%", kw.Like, kw.Like)
		clauses = append(clauses, "("+strings.Join(keywordClauses, " OR ")+")")
		args = append(args, keywordArgs...)
	}
	if source := strings.TrimSpace(filter.ImageSource); source != "" {
		clauses = append(clauses, "pm.image_source = ?")
		args = append(args, source)
	}
	if status := strings.TrimSpace(filter.SyncStatus); status != "" {
		clauses = append(clauses, "pm.erp_sync_status = ?")
		args = append(args, status)
	}
	if status := strings.TrimSpace(filter.BaseSyncStatus); status != "" {
		clauses = append(clauses, "pm.base_sync_status = ?")
		args = append(args, status)
	}
	if status := strings.TrimSpace(filter.ImageSyncStatus); status != "" {
		if status == string(domain.ProductManagementERPSyncStatusSynced) {
			clauses = append(clauses, "pm.image_sync_status = ? AND pm.last_image_synced_at IS NOT NULL")
			args = append(args, status)
		} else if status == string(domain.ProductManagementERPSyncStatusPendingSync) {
			clauses = append(clauses, "(pm.image_sync_status = ? OR (pm.image_sync_status = 'synced' AND pm.last_image_synced_at IS NULL))")
			args = append(args, status)
		} else {
			clauses = append(clauses, "pm.image_sync_status = ?")
			args = append(args, status)
		}
	}
	switch strings.TrimSpace(filter.CostStatus) {
	case "missing":
		clauses = append(clauses, "(pm.cost_price IS NULL OR pm.cost_price <= 0)")
	case "ready":
		clauses = append(clauses, "pm.cost_price IS NOT NULL AND pm.cost_price > 0")
	}
	if filter.CreatorID != nil && *filter.CreatorID > 0 {
		clauses = append(clauses, "pm.creator_id = ?")
		args = append(args, *filter.CreatorID)
	}
	if shouldApplyProductManagementAttentionScope(filter) {
		clauses = append(clauses, `(
			pm.cost_price IS NULL
			OR pm.cost_price <= 0
			OR pm.base_sync_status IN ('pending_sync', 'failed', 'queued', 'cooling_down', 'syncing')
			OR pm.image_sync_status IN ('pending_sync', 'waiting_image', 'failed', 'queued', 'cooling_down', 'syncing')
			OR (pm.image_sync_status = 'synced' AND pm.last_image_synced_at IS NULL)
		)`)
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func shouldApplyProductManagementAttentionScope(filter repo.ProductManagementListFilter) bool {
	if strings.TrimSpace(filter.IssueScope) != "attention" {
		return false
	}
	return strings.TrimSpace(filter.SyncStatus) != string(domain.ProductManagementERPSyncStatusSynced) &&
		strings.TrimSpace(filter.BaseSyncStatus) != string(domain.ProductManagementERPSyncStatusSynced) &&
		strings.TrimSpace(filter.ImageSyncStatus) != string(domain.ProductManagementERPSyncStatusSynced)
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
	var costMatchedRuleVersion, costRequiresManualReview, costManualOverride sql.NullInt64
	var costPrice sql.NullFloat64
	var lastERPCheckedAt, lastERPSyncedAt, lastBaseSyncedAt, lastImageSyncedAt, syncCooldownUntil sql.NullTime
	var costRuleName, costRuleSource, costPrefillSource, costManualOverrideReason sql.NullString
	var costInputSnapshot, costCalculationSnapshot sql.NullString
	var costSnapshotAt sql.NullTime
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
		&costRuleName,
		&costRuleSource,
		&costMatchedRuleVersion,
		&costPrefillSource,
		&costRequiresManualReview,
		&costManualOverride,
		&costManualOverrideReason,
		&costInputSnapshot,
		&costCalculationSnapshot,
		&costSnapshotAt,
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
	item.CostTrace = productManagementCostTraceFromRow(
		costRuleName,
		costRuleSource,
		costMatchedRuleVersion,
		costPrefillSource,
		costRequiresManualReview,
		costManualOverride,
		costManualOverrideReason,
		costInputSnapshot,
		costCalculationSnapshot,
		costSnapshotAt,
	)
	if strings.TrimSpace(item.ERPIID) == "" {
		item.ERPIID = strings.TrimSpace(item.ProductIID)
	}
	item.ImageSourceLabel = domain.ProductManagementImageSourceLabel(item.ImageSource)
	return &item, nil
}

func productManagementCostTraceFromRow(
	ruleName sql.NullString,
	ruleSource sql.NullString,
	matchedRuleVersion sql.NullInt64,
	prefillSource sql.NullString,
	requiresManualReview sql.NullInt64,
	manualCostOverride sql.NullInt64,
	manualCostOverrideReason sql.NullString,
	inputSnapshot sql.NullString,
	calculationSnapshot sql.NullString,
	snapshotAt sql.NullTime,
) *domain.ProductManagementCostTrace {
	if !ruleName.Valid && !ruleSource.Valid && !inputSnapshot.Valid && !calculationSnapshot.Valid {
		return nil
	}
	trace := &domain.ProductManagementCostTrace{
		RuleName:                 strings.TrimSpace(ruleName.String),
		RuleSource:               strings.TrimSpace(ruleSource.String),
		MatchedRuleVersion:       fromNullInt(matchedRuleVersion),
		PrefillSource:            strings.TrimSpace(prefillSource.String),
		RequiresManualReview:     requiresManualReview.Valid && requiresManualReview.Int64 != 0,
		ManualCostOverride:       manualCostOverride.Valid && manualCostOverride.Int64 != 0,
		ManualCostOverrideReason: strings.TrimSpace(manualCostOverrideReason.String),
		InputSnapshot:            productManagementRawJSON(inputSnapshot),
		CalculationSnapshot:      productManagementRawJSON(calculationSnapshot),
		SnapshotAt:               fromNullTime(snapshotAt),
	}
	if trace.RuleName == "" && trace.RuleSource == "" && len(trace.InputSnapshot) == 0 && len(trace.CalculationSnapshot) == 0 {
		return nil
	}
	return trace
}

func productManagementRawJSON(value sql.NullString) json.RawMessage {
	if !value.Valid {
		return nil
	}
	raw := strings.TrimSpace(value.String)
	if raw == "" {
		return nil
	}
	return json.RawMessage(raw)
}
