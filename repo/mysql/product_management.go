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
	cost_snapshot.calculation_snapshot_json, cost_snapshot.created_at,
	pm_tsi.variant_json, pm_tsi.quantity,
	pm_td.spec_text, pm_td.size_text, pm_td.width, pm_td.height, pm_td.area, pm_td.quantity`

const productManagementCostTraceJoin = `
	LEFT JOIN omp_sku_cost_snapshots cost_snapshot
	  ON cost_snapshot.id = pm.latest_cost_snapshot_id`

const productManagementDimensionJoin = `
	LEFT JOIN task_details pm_td ON pm_td.task_id = pm.task_id
	LEFT JOIN task_sku_items pm_tsi ON pm.task_sku_item_id IS NOT NULL AND pm_tsi.id = pm.task_sku_item_id`

func (r *productManagementRepo) RefreshReadModel(ctx context.Context) error {
	if err := r.refreshMainTaskRecords(ctx); err != nil {
		return err
	}
	if err := r.refreshSKUItemRecords(ctx); err != nil {
		return err
	}
	return r.refreshMaterializedFields(ctx)
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
		  latest_cost_snapshot_id = CASE
		    WHEN NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		      OR NOT (erp_product_sync_records.task_id <=> VALUES(task_id))
		      OR NOT (erp_product_sync_records.task_sku_item_id <=> VALUES(task_sku_item_id))
		    THEN NULL ELSE erp_product_sync_records.latest_cost_snapshot_id
		  END,
		  latest_erp_trace_id = CASE
		    WHEN NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		      OR NOT (erp_product_sync_records.task_id <=> VALUES(task_id))
		      OR NOT (erp_product_sync_records.task_sku_item_id <=> VALUES(task_sku_item_id))
		    THEN NULL ELSE erp_product_sync_records.latest_erp_trace_id
		  END,
		  combo_search_text = CASE
		    WHEN NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		    THEN NULL ELSE erp_product_sync_records.combo_search_text
		  END,
		  cost_legacy_alias_fallback = CASE
		    WHEN NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		      OR NOT (erp_product_sync_records.task_id <=> VALUES(task_id))
		      OR NOT (erp_product_sync_records.task_sku_item_id <=> VALUES(task_sku_item_id))
		    THEN 0 ELSE erp_product_sync_records.cost_legacy_alias_fallback
		  END,
		  cost_area_spec_abnormal = CASE
		    WHEN NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		      OR NOT (erp_product_sync_records.task_id <=> VALUES(task_id))
		      OR NOT (erp_product_sync_records.task_sku_item_id <=> VALUES(task_sku_item_id))
		    THEN 0 ELSE erp_product_sync_records.cost_area_spec_abnormal
		  END,
		  erp_sync_status = CASE
		    WHEN erp_product_sync_records.erp_sync_status IN ('queued', 'cooling_down', 'syncing') THEN erp_product_sync_records.erp_sync_status
		    WHEN erp_product_sync_records.erp_sync_status IN ('synced', 'failed')
		      AND (
		        NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		        OR NOT (erp_product_sync_records.product_i_id <=> VALUES(product_i_id))
		        OR NOT (erp_product_sync_records.product_name <=> VALUES(product_name))
		        OR NOT (erp_product_sync_records.cost_price <=> VALUES(cost_price))
		      ) THEN 'pending_sync'
		    WHEN VALUES(erp_sync_status) = 'synced' THEN 'synced'
		    WHEN VALUES(erp_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.erp_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at THEN 'pending_sync'
		    ELSE erp_product_sync_records.erp_sync_status
		  END,
		  base_sync_status = CASE
		    WHEN erp_product_sync_records.base_sync_status IN ('queued', 'cooling_down', 'syncing') THEN erp_product_sync_records.base_sync_status
		    WHEN erp_product_sync_records.base_sync_status IN ('synced', 'failed')
		      AND (
		        NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		        OR NOT (erp_product_sync_records.erp_i_id <=> VALUES(erp_i_id))
		        OR NOT (erp_product_sync_records.product_name <=> VALUES(product_name))
		        OR NOT (erp_product_sync_records.cost_price <=> VALUES(cost_price))
		      ) THEN 'pending_sync'
		    WHEN VALUES(base_sync_status) = 'synced' THEN 'synced'
		    WHEN VALUES(base_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.base_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at THEN 'pending_sync'
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
			    NULLIF(CASE WHEN COALESCE(t.is_batch_task, 0) = 0 AND JSON_VALID(td.product_selection_snapshot_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.i_id')) ELSE '' END, ''),
			    NULLIF(CASE WHEN COALESCE(t.is_batch_task, 0) = 0 AND JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.product.i_id')) ELSE '' END, ''),
			    NULLIF(CASE WHEN COALESCE(t.is_batch_task, 0) = 0 AND JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')) ELSE '' END, ''),
			    ''
			  ),
			  COALESCE(
			    NULLIF(CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.product_i_id')) ELSE '' END, ''),
			    NULLIF(CASE WHEN JSON_VALID(tsi.variant_json) THEN JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.i_id')) ELSE '' END, ''),
			    NULLIF(CASE WHEN COALESCE(t.is_batch_task, 0) = 0 AND JSON_VALID(td.product_selection_snapshot_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.i_id')) ELSE '' END, ''),
			    NULLIF(CASE WHEN COALESCE(t.is_batch_task, 0) = 0 AND JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.product.i_id')) ELSE '' END, ''),
			    NULLIF(CASE WHEN COALESCE(t.is_batch_task, 0) = 0 AND JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')) ELSE '' END, ''),
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
		  latest_cost_snapshot_id = CASE
		    WHEN NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		      OR NOT (erp_product_sync_records.task_id <=> VALUES(task_id))
		      OR NOT (erp_product_sync_records.task_sku_item_id <=> VALUES(task_sku_item_id))
		    THEN NULL ELSE erp_product_sync_records.latest_cost_snapshot_id
		  END,
		  latest_erp_trace_id = CASE
		    WHEN NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		      OR NOT (erp_product_sync_records.task_id <=> VALUES(task_id))
		      OR NOT (erp_product_sync_records.task_sku_item_id <=> VALUES(task_sku_item_id))
		    THEN NULL ELSE erp_product_sync_records.latest_erp_trace_id
		  END,
		  combo_search_text = CASE
		    WHEN NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		    THEN NULL ELSE erp_product_sync_records.combo_search_text
		  END,
		  cost_legacy_alias_fallback = CASE
		    WHEN NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		      OR NOT (erp_product_sync_records.task_id <=> VALUES(task_id))
		      OR NOT (erp_product_sync_records.task_sku_item_id <=> VALUES(task_sku_item_id))
		    THEN 0 ELSE erp_product_sync_records.cost_legacy_alias_fallback
		  END,
		  cost_area_spec_abnormal = CASE
		    WHEN NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		      OR NOT (erp_product_sync_records.task_id <=> VALUES(task_id))
		      OR NOT (erp_product_sync_records.task_sku_item_id <=> VALUES(task_sku_item_id))
		    THEN 0 ELSE erp_product_sync_records.cost_area_spec_abnormal
		  END,
		  erp_sync_status = CASE
		    WHEN erp_product_sync_records.erp_sync_status IN ('queued', 'cooling_down', 'syncing') THEN erp_product_sync_records.erp_sync_status
		    WHEN erp_product_sync_records.erp_sync_status IN ('synced', 'failed')
		      AND (
		        NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		        OR NOT (erp_product_sync_records.product_i_id <=> VALUES(product_i_id))
		        OR NOT (erp_product_sync_records.product_name <=> VALUES(product_name))
		        OR NOT (erp_product_sync_records.cost_price <=> VALUES(cost_price))
		      ) THEN 'pending_sync'
		    WHEN VALUES(erp_sync_status) = 'synced' THEN 'synced'
		    WHEN VALUES(erp_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.erp_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at THEN 'pending_sync'
		    ELSE erp_product_sync_records.erp_sync_status
		  END,
		  base_sync_status = CASE
		    WHEN erp_product_sync_records.base_sync_status IN ('queued', 'cooling_down', 'syncing') THEN erp_product_sync_records.base_sync_status
		    WHEN erp_product_sync_records.base_sync_status IN ('synced', 'failed')
		      AND (
		        NOT (erp_product_sync_records.sku_code <=> VALUES(sku_code))
		        OR NOT (erp_product_sync_records.erp_i_id <=> VALUES(erp_i_id))
		        OR NOT (erp_product_sync_records.product_name <=> VALUES(product_name))
		        OR NOT (erp_product_sync_records.cost_price <=> VALUES(cost_price))
		      ) THEN 'pending_sync'
		    WHEN VALUES(base_sync_status) = 'synced' THEN 'synced'
		    WHEN VALUES(base_sync_status) = 'pending_sync'
		      AND erp_product_sync_records.base_sync_status = 'failed'
		      AND VALUES(updated_at) > erp_product_sync_records.updated_at THEN 'pending_sync'
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

func (r *productManagementRepo) refreshMaterializedFields(ctx context.Context) error {
	if _, err := r.db.db.ExecContext(ctx, `SET SESSION group_concat_max_len = 1048576`); err != nil {
		return fmt.Errorf("prepare product management materialized fields: %w", err)
	}
	if _, err := r.db.db.ExecContext(ctx, `
		UPDATE erp_product_sync_records pm
		   SET pm.latest_cost_snapshot_id = (
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
		       )
		 WHERE pm.latest_cost_snapshot_id IS NULL`); err != nil {
		return fmt.Errorf("refresh product management latest cost snapshot: %w", err)
	}
	if _, err := r.db.db.ExecContext(ctx, `
		UPDATE erp_product_sync_records pm
		   SET pm.latest_erp_trace_id = (
		         SELECT l.id
		           FROM omp_sku_erp_trace_logs l
		          WHERE l.sku_code = pm.sku_code
		            AND (
		              (pm.task_sku_item_id IS NOT NULL AND l.task_sku_item_id = pm.task_sku_item_id)
		              OR (pm.task_sku_item_id IS NULL AND l.task_id = pm.task_id AND l.task_sku_item_id IS NULL)
		              OR l.task_id = pm.task_id
		              OR l.task_id IS NULL
		            )
		          ORDER BY l.created_at DESC, l.id DESC
		          LIMIT 1
		       )
		 WHERE pm.latest_erp_trace_id IS NULL`); err != nil {
		return fmt.Errorf("refresh product management latest erp trace: %w", err)
	}
	if _, err := r.db.db.ExecContext(ctx, `
		UPDATE erp_product_sync_records pm
		LEFT JOIN (
		    SELECT
		      limited.child_sku_code,
		      GROUP_CONCAT(DISTINCT limited.search_token ORDER BY limited.search_token SEPARATOR ' ') AS combo_search_text
		      FROM (
		        SELECT ranked.child_sku_code, ranked.search_token
		          FROM (
		            SELECT
		              rel.child_sku_code,
		              LEFT(CONCAT_WS(' ', rel.combo_sku_code, rec.erp_i_id, rec.name, rec.short_name), 256) AS search_token,
		              ROW_NUMBER() OVER (PARTITION BY rel.child_sku_code ORDER BY rel.combo_sku_code, COALESCE(rec.erp_i_id, '')) AS rn
		              FROM omp_sku_combo_relations rel
		              LEFT JOIN omp_sku_combo_records rec ON rec.combo_sku_code = rel.combo_sku_code
		          ) ranked
		         WHERE ranked.rn <= 200
		      ) limited
		     GROUP BY limited.child_sku_code
		) combo ON combo.child_sku_code = pm.sku_code
		   SET pm.combo_search_text = COALESCE(combo.combo_search_text, '')
		 WHERE pm.combo_search_text IS NULL`); err != nil {
		return fmt.Errorf("refresh product management combo search text: %w", err)
	}
	if _, err := r.db.db.ExecContext(ctx, productManagementRefreshCostFlagsSQL()); err != nil {
		return fmt.Errorf("refresh product management cost flags: %w", err)
	}
	return nil
}

func productManagementRefreshCostFlagsSQL() string {
	return `
		UPDATE erp_product_sync_records pm
		LEFT JOIN omp_sku_cost_snapshots cost_snapshot ON cost_snapshot.id = pm.latest_cost_snapshot_id
		LEFT JOIN task_details pm_td ON pm_td.task_id = pm.task_id
		LEFT JOIN task_sku_items pm_tsi ON pm.task_sku_item_id IS NOT NULL AND pm_tsi.id = pm.task_sku_item_id
		   SET pm.cost_legacy_alias_fallback = CASE
		         WHEN JSON_VALID(cost_snapshot.calculation_snapshot_json)
		          AND JSON_UNQUOTE(JSON_EXTRACT(cost_snapshot.calculation_snapshot_json, '$.legacy_alias_fallback')) = 'true'
		         THEN 1 ELSE 0 END,
		       pm.cost_area_spec_abnormal = CASE
		         WHEN pm.cost_price IS NOT NULL AND pm.cost_price > 0
		          AND COALESCE(pm_td.area, 0) <= 0
		          AND (COALESCE(pm_td.width, 0) <= 0 OR COALESCE(pm_td.height, 0) <= 0)
		          AND (
		            pm.task_sku_item_id IS NULL
		            OR NOT JSON_VALID(pm_tsi.variant_json)
		            OR (
		                 COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(pm_tsi.variant_json, '$.area')) AS DECIMAL(12,4)), 0) <= 0
		             AND COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(pm_tsi.variant_json, '$.width')) AS DECIMAL(12,4)), 0) <= 0
		             AND COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(pm_tsi.variant_json, '$.height')) AS DECIMAL(12,4)), 0) <= 0
		            )
		          )
		         THEN 1 ELSE 0 END`
}

func (r *productManagementRepo) List(ctx context.Context, filter repo.ProductManagementListFilter) ([]*domain.ProductManagementRecord, int64, error) {
	filter.Page, filter.PageSize = normalizePage(filter.Page, filter.PageSize)
	useComboFullText := strings.TrimSpace(filter.Keyword) != "" && mysqlColumnExists(ctx, r.db.db, "erp_product_sync_records", "combo_search_text")
	where, args := buildProductManagementWhereWithOptions(filter, productManagementWhereOptions{UseComboFullText: useComboFullText})
	items, total, err := r.listWithProductManagementWhere(ctx, filter, where, args)
	if err != nil && strings.TrimSpace(filter.Keyword) != "" && isMySQLFullTextIndexMissing(err) {
		where, args = buildProductManagementWhere(filter)
		return r.listWithProductManagementWhere(ctx, filter, where, args)
	}
	return items, total, err
}

func (r *productManagementRepo) listWithProductManagementWhere(ctx context.Context, filter repo.ProductManagementListFilter, where string, args []interface{}) ([]*domain.ProductManagementRecord, int64, error) {
	var total int64
	countCtx, cancelCount := mysqlReadQueryContext(ctx)
	err := r.db.db.QueryRowContext(countCtx, `SELECT COUNT(*) FROM erp_product_sync_records pm `+where, args...).Scan(&total)
	cancelCount()
	if err != nil {
		return nil, 0, fmt.Errorf("count product management records: %w", err)
	}
	listArgs := append([]interface{}{}, args...)
	listArgs = append(listArgs, (filter.Page-1)*filter.PageSize, filter.PageSize)
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, `SELECT `+productManagementSelectCols+` FROM erp_product_sync_records pm `+productManagementCostTraceJoin+productManagementDimensionJoin+` `+where+`
		ORDER BY pm.updated_at DESC, pm.task_created_at DESC, pm.id DESC
		LIMIT ?, ?`, listArgs...)
	if err != nil {
		cancelQuery()
		return nil, 0, fmt.Errorf("list product management records: %w", err)
	}
	defer cancelQuery()
	defer rows.Close()
	return scanProductManagementRows(rows, total)
}

func (r *productManagementRepo) CostDashboard(ctx context.Context) (*domain.ProductCostDashboardResponse, error) {
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	row := r.db.db.QueryRowContext(queryCtx, `
		SELECT
		  COUNT(*) AS total_records,
		  COALESCE(SUM(cost_missing), 0) AS cost_missing,
		  COALESCE(SUM(manual_quote), 0) AS manual_quote,
		  COALESCE(SUM(erp_mismatch), 0) AS erp_mismatch,
		  COALESCE(SUM(rule_version_outdated), 0) AS rule_version_outdated,
		  COALESCE(SUM(legacy_alias_fallback), 0) AS legacy_alias_fallback,
		  COALESCE(SUM(area_spec_abnormal), 0) AS area_spec_abnormal,
		  COALESCE(SUM(CASE WHEN cost_missing = 1 OR manual_quote = 1 OR erp_mismatch = 1
		                      OR rule_version_outdated = 1 OR legacy_alias_fallback = 1
		                      OR area_spec_abnormal = 1 THEN 1 ELSE 0 END), 0) AS issue_total
		FROM (
		  SELECT
		    pm.id,
		    CASE WHEN pm.cost_price IS NULL OR pm.cost_price <= 0 THEN 1 ELSE 0 END AS cost_missing,
		    CASE WHEN COALESCE(cost_snapshot.requires_manual_review, 0) = 1 THEN 1 ELSE 0 END AS manual_quote,
		    CASE
		      WHEN pm.erp_sync_status = 'failed' OR pm.base_sync_status = 'failed'
		        OR latest_erp_trace.status = 'failed'
		        OR (
		          JSON_VALID(latest_erp_trace.response_payload_json)
		          AND JSON_UNQUOTE(JSON_EXTRACT(latest_erp_trace.response_payload_json, '$.cost_verification.status')) = 'mismatched'
		        )
		      THEN 1 ELSE 0
		    END AS erp_mismatch,
		    CASE
		      WHEN cost_snapshot.matched_rule_version IS NOT NULL
		       AND latest_rule.latest_rule_version IS NOT NULL
		       AND cost_snapshot.matched_rule_version < latest_rule.latest_rule_version
		      THEN 1 ELSE 0
		    END AS rule_version_outdated,
		    CASE WHEN COALESCE(pm.cost_legacy_alias_fallback, 0) = 1 THEN 1 ELSE 0 END AS legacy_alias_fallback,
		    CASE WHEN COALESCE(pm.cost_area_spec_abnormal, 0) = 1 THEN 1 ELSE 0 END AS area_spec_abnormal
		  FROM erp_product_sync_records pm
		  `+productManagementCostTraceJoin+`
		  LEFT JOIN cost_rules snapshot_rule ON snapshot_rule.id = cost_snapshot.cost_rule_id
		  LEFT JOIN (
		    SELECT category_code, MAX(rule_version) AS latest_rule_version
		      FROM cost_rules
		     WHERE is_active = 1
		     GROUP BY category_code
		  ) latest_rule ON latest_rule.category_code = COALESCE(
		    NULLIF(CASE WHEN JSON_VALID(cost_snapshot.calculation_snapshot_json)
		      THEN JSON_UNQUOTE(JSON_EXTRACT(cost_snapshot.calculation_snapshot_json, '$.rule_group')) ELSE '' END, ''),
		    snapshot_rule.category_code
		  )
		  LEFT JOIN omp_sku_erp_trace_logs latest_erp_trace
		    ON latest_erp_trace.id = pm.latest_erp_trace_id
		) flags`)
	var totalRecords, costMissing, manualQuote, erpMismatch, ruleVersionOutdated, legacyAliasFallback, areaSpecAbnormal, issueTotal int64
	err := row.Scan(&totalRecords, &costMissing, &manualQuote, &erpMismatch, &ruleVersionOutdated, &legacyAliasFallback, &areaSpecAbnormal, &issueTotal)
	cancelQuery()
	if err != nil {
		return nil, fmt.Errorf("get product cost dashboard: %w", err)
	}
	trend, err := r.costLegacyFallbackTrend(ctx)
	if err != nil {
		return nil, fmt.Errorf("get product cost legacy fallback trend: %w", err)
	}
	ratio := 0.0
	if totalRecords > 0 {
		ratio = float64(legacyAliasFallback) / float64(totalRecords)
	}
	return &domain.ProductCostDashboardResponse{
		TotalRecords:        totalRecords,
		IssueTotal:          issueTotal,
		LegacyFallbackCount: legacyAliasFallback,
		LegacyFallbackRatio: ratio,
		LegacyFallbackTrend: trend,
		Groups: []domain.ProductCostIssueGroup{
			{Key: "cannot_calculate", Label: "算不出来的", Count: costMissing + manualQuote},
			{Key: "possibly_wrong", Label: "可能算错的", Count: erpMismatch + ruleVersionOutdated + legacyAliasFallback},
			{Key: "looks_abnormal", Label: "看着不对劲的", Count: areaSpecAbnormal},
		},
		Tags: []domain.ProductCostIssueTag{
			{Key: "cost_missing", Label: "成本缺失", Group: "cannot_calculate", Count: costMissing},
			{Key: "manual_quote", Label: "需人工报价", Group: "cannot_calculate", Count: manualQuote},
			{Key: "erp_mismatch", Label: "ERP 不一致", Group: "possibly_wrong", Count: erpMismatch},
			{Key: "rule_version_outdated", Label: "规则版本过旧", Group: "possibly_wrong", Count: ruleVersionOutdated, Tooltip: "最新成本快照版本低于当前命中规则组的最新版本"},
			{Key: "unbound_iid", Label: "未关联款式", Group: "possibly_wrong", Count: legacyAliasFallback, Tooltip: "当前成本由名称或文本猜测规则产生"},
			{Key: "area_spec_abnormal", Label: "面积/规格异常", Group: "looks_abnormal", Count: areaSpecAbnormal},
		},
		GeneratedAt: time.Now().UTC(),
	}, nil
}

func (r *productManagementRepo) costLegacyFallbackTrend(ctx context.Context) ([]domain.ProductCostLegacyFallbackTrendItem, error) {
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, `
		SELECT
		  DATE(cost_snapshot.created_at) AS snapshot_date,
		  COUNT(*) AS total_records,
		  COALESCE(SUM(CASE WHEN COALESCE(pm.cost_legacy_alias_fallback, 0) = 1 THEN 1 ELSE 0 END), 0) AS legacy_alias_fallback
		FROM erp_product_sync_records pm
		`+productManagementCostTraceJoin+`
		WHERE cost_snapshot.created_at IS NOT NULL
		  AND cost_snapshot.created_at >= DATE_SUB(CURRENT_DATE, INTERVAL 29 DAY)
		GROUP BY DATE(cost_snapshot.created_at)
		ORDER BY snapshot_date ASC`)
	if err != nil {
		cancelQuery()
		return nil, err
	}
	defer cancelQuery()
	defer rows.Close()
	trend := make([]domain.ProductCostLegacyFallbackTrendItem, 0, 30)
	for rows.Next() {
		var snapshotDate sql.NullTime
		var item domain.ProductCostLegacyFallbackTrendItem
		if err := rows.Scan(&snapshotDate, &item.TotalRecords, &item.LegacyFallbackCount); err != nil {
			return nil, err
		}
		if snapshotDate.Valid {
			item.Date = snapshotDate.Time.Format("2006-01-02")
		}
		if item.TotalRecords > 0 {
			item.LegacyFallbackRatio = float64(item.LegacyFallbackCount) / float64(item.TotalRecords)
		}
		trend = append(trend, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return trend, nil
}

func (r *productManagementRepo) GetByID(ctx context.Context, id int64) (*domain.ProductManagementRecord, error) {
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	row := r.db.db.QueryRowContext(queryCtx, `SELECT `+productManagementSelectCols+` FROM erp_product_sync_records pm `+productManagementCostTraceJoin+productManagementDimensionJoin+` WHERE pm.id = ?`, id)
	item, err := scanProductManagementRecord(row)
	cancelQuery()
	return item, err
}

func (r *productManagementRepo) GetByTaskID(ctx context.Context, taskID int64) ([]*domain.ProductManagementRecord, error) {
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, `SELECT `+productManagementSelectCols+` FROM erp_product_sync_records pm `+productManagementCostTraceJoin+productManagementDimensionJoin+` WHERE pm.task_id = ? ORDER BY pm.task_sku_item_id IS NULL DESC, pm.id ASC`, taskID)
	if err != nil {
		cancelQuery()
		return nil, fmt.Errorf("list product management records by task: %w", err)
	}
	defer cancelQuery()
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
	rows, err := tx.QueryContext(ctx, `SELECT `+productManagementSelectCols+` FROM erp_product_sync_records pm `+productManagementCostTraceJoin+productManagementDimensionJoin+` WHERE pm.sync_claim_token = ? ORDER BY pm.last_erp_checked_at, pm.id`, claimToken)
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

type productManagementWhereOptions struct {
	UseComboFullText bool
}

func buildProductManagementWhere(filter repo.ProductManagementListFilter) (string, []interface{}) {
	return buildProductManagementWhereWithOptions(filter, productManagementWhereOptions{})
}

func buildProductManagementWhereWithOptions(filter repo.ProductManagementListFilter, options productManagementWhereOptions) (string, []interface{}) {
	clauses := []string{"1 = 1"}
	args := make([]interface{}, 0, 12)
	keyword := strings.TrimSpace(filter.Keyword)
	if keyword != "" {
		kw := normalizeSearchKeyword(keyword)
		textClauses := []string{
			"pm.product_name LIKE ?",
			"pm.category_name LIKE ?",
			"pm.creator_name LIKE ?",
		}
		textArgs := []interface{}{kw.Like, kw.Like, kw.Like}
		if kw.HasInt64 {
			textClauses = append(textClauses, "pm.creator_id = ?")
			textArgs = append(textArgs, kw.Int64)
		}

		comboClause := ""
		comboArgs := make([]interface{}, 0, 6)
		if options.UseComboFullText {
			comboClause = `MATCH(pm.combo_search_text) AGAINST (? IN NATURAL LANGUAGE MODE)`
			comboArgs = append(comboArgs, keyword)
		} else {
			comboClause = `EXISTS (
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
			)`
			comboArgs = append(comboArgs, kw.Upper, kw.Upper, kw.Upper+"%", kw.Upper+"%", kw.Like, kw.Like)
		}

		if kw.IsCode {
			exactClauses := []string{
				"pm.sku_code = ?",
				"pm.task_no = ?",
				"pm.product_i_id = ?",
				"pm.erp_i_id = ?",
			}
			fallbackClauses := append(textClauses,
				"pm.sku_code LIKE ?",
				"pm.task_no LIKE ?",
				"pm.product_i_id LIKE ?",
				"pm.erp_i_id LIKE ?",
				comboClause,
			)
			directMatchGuard := `NOT EXISTS (
			  SELECT 1
			    FROM erp_product_sync_records direct_pm
			   WHERE direct_pm.sku_code = ?
			      OR direct_pm.task_no = ?
			      OR direct_pm.product_i_id = ?
			      OR direct_pm.erp_i_id = ?
			)`
			clauses = append(clauses,
				"(("+strings.Join(exactClauses, " OR ")+") OR ("+directMatchGuard+" AND ("+strings.Join(fallbackClauses, " OR ")+")))",
			)
			args = append(args,
				kw.Upper, kw.Upper, kw.Upper, kw.Upper,
				kw.Upper, kw.Upper, kw.Upper, kw.Upper,
			)
			args = append(args, textArgs...)
			args = append(args, kw.Upper+"%", kw.Upper+"%", kw.Upper+"%", kw.Upper+"%")
			args = append(args, comboArgs...)
		} else {
			keywordClauses := append(textClauses,
				"pm.sku_code LIKE ?",
				"pm.task_no LIKE ?",
				"pm.product_i_id LIKE ?",
				"pm.erp_i_id LIKE ?",
				comboClause,
			)
			keywordArgs := append(textArgs, kw.Like, kw.Like, kw.Like, kw.Like)
			keywordArgs = append(keywordArgs, comboArgs...)
			clauses = append(clauses, "("+strings.Join(keywordClauses, " OR ")+")")
			args = append(args, keywordArgs...)
		}
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
	var dimensionSKUQuantity, dimensionTaskQuantity sql.NullInt64
	var dimensionTaskWidth, dimensionTaskHeight, dimensionTaskArea sql.NullFloat64
	var lastERPCheckedAt, lastERPSyncedAt, lastBaseSyncedAt, lastImageSyncedAt, syncCooldownUntil sql.NullTime
	var costRuleName, costRuleSource, costPrefillSource, costManualOverrideReason sql.NullString
	var costInputSnapshot, costCalculationSnapshot sql.NullString
	var dimensionVariantJSON, dimensionTaskSpecText, dimensionTaskSizeText sql.NullString
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
		&dimensionVariantJSON,
		&dimensionSKUQuantity,
		&dimensionTaskSpecText,
		&dimensionTaskSizeText,
		&dimensionTaskWidth,
		&dimensionTaskHeight,
		&dimensionTaskArea,
		&dimensionTaskQuantity,
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
	item.DimensionVariantJSON = productManagementRawJSON(dimensionVariantJSON)
	item.DimensionTaskSpecText = strings.TrimSpace(dimensionTaskSpecText.String)
	item.DimensionTaskSizeText = strings.TrimSpace(dimensionTaskSizeText.String)
	item.DimensionTaskWidthM = fromNullFloat64(dimensionTaskWidth)
	item.DimensionTaskHeightM = fromNullFloat64(dimensionTaskHeight)
	item.DimensionTaskAreaM2 = fromNullFloat64(dimensionTaskArea)
	item.DimensionSKUQuantity = productManagementFloatFromNullInt64(dimensionSKUQuantity)
	item.DimensionTaskQuantity = productManagementFloatFromNullInt64(dimensionTaskQuantity)
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

func productManagementFloatFromNullInt64(value sql.NullInt64) *float64 {
	if !value.Valid || value.Int64 <= 0 {
		return nil
	}
	converted := float64(value.Int64)
	return &converted
}
