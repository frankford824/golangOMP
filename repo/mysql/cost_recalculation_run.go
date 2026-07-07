package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type costRecalculationRunRepo struct{ db *DB }

func NewCostRecalculationRunRepo(db *DB) repo.CostRecalculationRunRepo {
	return &costRecalculationRunRepo{db: db}
}

func (r *costRecalculationRunRepo) CreateRun(ctx context.Context, tx repo.Tx, run *domain.CostRecalculationRun) (int64, error) {
	if run == nil {
		return 0, fmt.Errorf("cost recalculation run is required")
	}
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO cost_recalculation_runs (
		  run_no, status, mode, filters_json, summary_json, created_by, applied_by, erp_synced_by,
		  previewed_at, applied_at, erp_synced_at, cancelled_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(run.RunNo),
		string(run.Status),
		strings.TrimSpace(run.Mode),
		toNullJSONString(run.FiltersJSON),
		toNullJSONString(run.SummaryJSON),
		toNullInt64(run.CreatedBy),
		toNullInt64(run.AppliedBy),
		toNullInt64(run.ERPSyncedBy),
		toNullTime(run.PreviewedAt),
		toNullTime(run.AppliedAt),
		toNullTime(run.ERPSyncedAt),
		toNullTime(run.CancelledAt),
	)
	if err != nil {
		return 0, fmt.Errorf("insert cost recalculation run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("cost recalculation run id: %w", err)
	}
	return id, nil
}

func (r *costRecalculationRunRepo) GetRun(ctx context.Context, id int64) (*domain.CostRecalculationRun, error) {
	row := r.db.db.QueryRowContext(ctx, costRecalculationRunSelectSQL()+` WHERE id = ?`, id)
	return scanCostRecalculationRun(row)
}

func (r *costRecalculationRunRepo) ListRuns(ctx context.Context, filter repo.CostRecalculationRunFilter) ([]*domain.CostRecalculationRun, int64, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	where := []string{"1=1"}
	args := make([]interface{}, 0, 3)
	if status := strings.TrimSpace(filter.Status); status != "" {
		where = append(where, "status = ?")
		args = append(args, status)
	}
	if mode := strings.TrimSpace(filter.Mode); mode != "" {
		where = append(where, "mode = ?")
		args = append(args, mode)
	}
	if filter.CreatedBy != nil && *filter.CreatedBy > 0 {
		where = append(where, "created_by = ?")
		args = append(args, *filter.CreatedBy)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cost_recalculation_runs WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count cost recalculation runs: %w", err)
	}
	queryArgs := append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, costRecalculationRunSelectSQL()+` WHERE `+whereSQL+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list cost recalculation runs: %w", err)
	}
	defer rows.Close()
	items := make([]*domain.CostRecalculationRun, 0)
	for rows.Next() {
		item, err := scanCostRecalculationRun(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate cost recalculation runs: %w", err)
	}
	return items, total, nil
}

func (r *costRecalculationRunRepo) UpdateRun(ctx context.Context, tx repo.Tx, run *domain.CostRecalculationRun) error {
	if run == nil || run.ID <= 0 {
		return fmt.Errorf("cost recalculation run id is required")
	}
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE cost_recalculation_runs
		   SET status = ?, summary_json = ?, applied_by = ?, erp_synced_by = ?,
		       previewed_at = ?, applied_at = ?, erp_synced_at = ?, cancelled_at = ?
		 WHERE id = ?`,
		string(run.Status),
		toNullJSONString(run.SummaryJSON),
		toNullInt64(run.AppliedBy),
		toNullInt64(run.ERPSyncedBy),
		toNullTime(run.PreviewedAt),
		toNullTime(run.AppliedAt),
		toNullTime(run.ERPSyncedAt),
		toNullTime(run.CancelledAt),
		run.ID,
	)
	if err != nil {
		return fmt.Errorf("update cost recalculation run: %w", err)
	}
	return nil
}

func (r *costRecalculationRunRepo) MarkRunApplying(ctx context.Context, tx repo.Tx, runID int64) (bool, error) {
	sqlTx := Unwrap(tx)
	result, err := sqlTx.ExecContext(ctx, `
		UPDATE cost_recalculation_runs
		   SET status = ?
		 WHERE id = ? AND status = ?`,
		string(domain.CostRunStatusApplying),
		runID,
		string(domain.CostRunStatusPreviewed),
	)
	if err != nil {
		return false, fmt.Errorf("mark cost recalculation run applying: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("cost recalculation applying rows affected: %w", err)
	}
	return count > 0, nil
}

func (r *costRecalculationRunRepo) DeleteRunItems(ctx context.Context, tx repo.Tx, runID int64) error {
	sqlTx := Unwrap(tx)
	if _, err := sqlTx.ExecContext(ctx, `DELETE FROM cost_recalculation_run_items WHERE run_id = ?`, runID); err != nil {
		return fmt.Errorf("delete cost recalculation run items: %w", err)
	}
	return nil
}

func (r *costRecalculationRunRepo) InsertRunItems(ctx context.Context, tx repo.Tx, items []*domain.CostRecalculationRunItem) error {
	if len(items) == 0 {
		return nil
	}
	sqlTx := Unwrap(tx)
	stmt, err := sqlTx.PrepareContext(ctx, `
		INSERT INTO cost_recalculation_run_items (
		  run_id, product_management_record_id, task_id, task_no, task_sku_item_id, sku_code,
		  erp_i_id, product_i_id, normalized_i_id, old_cost_price, new_cost_price, cost_delta,
		  old_rule_id, new_rule_id, new_rule_version, match_mode, status, skip_reason,
		  conflict_reason, preview_snapshot_json, apply_snapshot_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare cost recalculation run item insert: %w", err)
	}
	defer stmt.Close()
	for _, item := range items {
		if item == nil {
			continue
		}
		res, err := stmt.ExecContext(ctx,
			item.RunID,
			item.ProductManagementRecordID,
			toNullInt64(item.TaskID),
			strings.TrimSpace(item.TaskNo),
			toNullInt64(item.TaskSKUItemID),
			strings.TrimSpace(item.SKUCode),
			strings.TrimSpace(item.ERPIID),
			strings.TrimSpace(item.ProductIID),
			strings.TrimSpace(item.NormalizedIID),
			toNullFloat64(item.OldCostPrice),
			toNullFloat64(item.NewCostPrice),
			toNullFloat64(item.CostDelta),
			toNullInt64(item.OldRuleID),
			toNullInt64(item.NewRuleID),
			toNullInt(item.NewRuleVersion),
			strings.TrimSpace(item.MatchMode),
			string(item.Status),
			strings.TrimSpace(item.SkipReason),
			strings.TrimSpace(item.ConflictReason),
			toNullJSONString(item.PreviewSnapshotJSON),
			toNullJSONString(item.ApplySnapshotJSON),
		)
		if err != nil {
			return fmt.Errorf("insert cost recalculation run item: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("cost recalculation run item id: %w", err)
		}
		item.ID = id
	}
	return nil
}

func (r *costRecalculationRunRepo) ListRunItems(ctx context.Context, filter repo.CostRecalculationRunItemFilter) ([]*domain.CostRecalculationRunItem, int64, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	where := []string{"run_id = ?"}
	args := []interface{}{filter.RunID}
	if status := strings.TrimSpace(filter.Status); status != "" {
		where = append(where, "status = ?")
		args = append(args, status)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cost_recalculation_run_items WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count cost recalculation run items: %w", err)
	}
	queryArgs := append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, costRecalculationRunItemSelectSQL()+` WHERE `+whereSQL+` ORDER BY ABS(COALESCE(cost_delta, 0)) DESC, id ASC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list cost recalculation run items: %w", err)
	}
	defer rows.Close()
	items := make([]*domain.CostRecalculationRunItem, 0)
	for rows.Next() {
		item, err := scanCostRecalculationRunItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate cost recalculation run items: %w", err)
	}
	return items, total, nil
}

func (r *costRecalculationRunRepo) ListRunItemsForUpdate(ctx context.Context, tx repo.Tx, runID int64) ([]*domain.CostRecalculationRunItem, error) {
	sqlTx := Unwrap(tx)
	rows, err := sqlTx.QueryContext(ctx, costRecalculationRunItemSelectSQL()+` WHERE run_id = ? ORDER BY id ASC FOR UPDATE`, runID)
	if err != nil {
		return nil, fmt.Errorf("list cost recalculation run items for update: %w", err)
	}
	defer rows.Close()
	items := make([]*domain.CostRecalculationRunItem, 0)
	for rows.Next() {
		item, err := scanCostRecalculationRunItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cost recalculation run items for update: %w", err)
	}
	return items, nil
}

func (r *costRecalculationRunRepo) UpdateRunItem(ctx context.Context, tx repo.Tx, item *domain.CostRecalculationRunItem) error {
	if item == nil || item.ID <= 0 {
		return fmt.Errorf("cost recalculation run item id is required")
	}
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE cost_recalculation_run_items
		   SET old_cost_price = ?, new_cost_price = ?, cost_delta = ?, old_rule_id = ?,
		       new_rule_id = ?, new_rule_version = ?, match_mode = ?, status = ?,
		       skip_reason = ?, conflict_reason = ?, preview_snapshot_json = ?, apply_snapshot_json = ?
		 WHERE id = ?`,
		toNullFloat64(item.OldCostPrice),
		toNullFloat64(item.NewCostPrice),
		toNullFloat64(item.CostDelta),
		toNullInt64(item.OldRuleID),
		toNullInt64(item.NewRuleID),
		toNullInt(item.NewRuleVersion),
		strings.TrimSpace(item.MatchMode),
		string(item.Status),
		strings.TrimSpace(item.SkipReason),
		strings.TrimSpace(item.ConflictReason),
		toNullJSONString(item.PreviewSnapshotJSON),
		toNullJSONString(item.ApplySnapshotJSON),
		item.ID,
	)
	if err != nil {
		return fmt.Errorf("update cost recalculation run item: %w", err)
	}
	return nil
}

func (r *costRecalculationRunRepo) HasOpenRunForRecord(ctx context.Context, tx repo.Tx, excludingRunID int64, recordID int64) (bool, error) {
	sqlTx := Unwrap(tx)
	var exists int
	err := sqlTx.QueryRowContext(ctx, `
		SELECT 1
		  FROM cost_recalculation_run_items i
		  JOIN cost_recalculation_runs r ON r.id = i.run_id
		 WHERE i.product_management_record_id = ?
		   AND i.run_id <> ?
		   AND r.status IN ('previewing','previewed','applying','partially_applied','applied','erp_syncing')
		   AND i.status NOT IN ('skipped','conflict','failed','erp_failed','erp_synced')
		 LIMIT 1`, recordID, excludingRunID).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check open cost recalculation run item: %w", err)
	}
	return true, nil
}

func (r *costRecalculationRunRepo) MarkERPQueuedItemsForRun(ctx context.Context, tx repo.Tx, runID int64, recordIDs []int64) (int64, error) {
	if len(recordIDs) == 0 {
		return 0, nil
	}
	sqlTx := Unwrap(tx)
	placeholders := make([]string, 0, len(recordIDs))
	args := make([]interface{}, 0, len(recordIDs)+1)
	args = append(args, string(domain.CostRunItemStatusERPQueued), runID, string(domain.CostRunItemStatusApplied))
	for _, id := range recordIDs {
		if id <= 0 {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	if len(placeholders) == 0 {
		return 0, nil
	}
	result, err := sqlTx.ExecContext(ctx, `
		UPDATE cost_recalculation_run_items
		   SET status = ?
		 WHERE run_id = ? AND status = ? AND product_management_record_id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	if err != nil {
		return 0, fmt.Errorf("mark cost recalculation run items erp queued: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("cost recalculation erp queued rows affected: %w", err)
	}
	return count, nil
}

func (r *costRecalculationRunRepo) MarkERPResultForProductManagementRecord(ctx context.Context, tx repo.Tx, recordID int64, status domain.CostRecalculationRunItemStatus, message string) error {
	if recordID <= 0 {
		return nil
	}
	sqlTx := Unwrap(tx)
	if _, err := sqlTx.ExecContext(ctx, `
		UPDATE cost_recalculation_run_items i
		JOIN cost_recalculation_runs r ON r.id = i.run_id
		   SET i.status = ?,
		       i.conflict_reason = CASE WHEN ? = 'erp_failed' THEN ? ELSE i.conflict_reason END
		 WHERE i.product_management_record_id = ?
		   AND i.status = 'erp_queued'
		   AND r.status = 'erp_syncing'`,
		string(status),
		string(status),
		strings.TrimSpace(message),
		recordID,
	); err != nil {
		return fmt.Errorf("mark cost recalculation erp result: %w", err)
	}
	return r.refreshERPRunStatuses(ctx, tx)
}

func (r *costRecalculationRunRepo) refreshERPRunStatuses(ctx context.Context, tx repo.Tx) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE cost_recalculation_runs r
		JOIN (
		  SELECT run_id,
		         COUNT(*) AS total_count,
		         COUNT(DISTINCT task_id) AS task_count,
		         SUM(status = 'previewed') AS previewed_count,
		         SUM(status = 'applied') AS applied_count,
		         SUM(status = 'skipped') AS skipped_count,
		         SUM(status = 'conflict') AS conflict_count,
		         SUM(status = 'failed') AS item_failed_count,
		         SUM(status = 'erp_queued') AS queued_count,
		         SUM(status = 'erp_failed') AS failed_count,
		         SUM(status = 'erp_synced') AS synced_count
		    FROM cost_recalculation_run_items
		   GROUP BY run_id
		) x ON x.run_id = r.id
		   SET r.status = CASE
		         WHEN x.queued_count > 0 THEN r.status
		         WHEN x.failed_count > 0 AND x.synced_count > 0 THEN 'partially_erp_synced'
		         WHEN x.failed_count > 0 AND x.synced_count = 0 THEN 'partially_erp_synced'
		         WHEN x.synced_count > 0 THEN 'erp_synced'
		         ELSE r.status
		       END,
		       r.erp_synced_at = CASE
		         WHEN x.queued_count = 0 AND (x.synced_count > 0 OR x.failed_count > 0) THEN COALESCE(r.erp_synced_at, UTC_TIMESTAMP())
		         ELSE r.erp_synced_at
		       END,
		       r.summary_json = JSON_OBJECT(
		         'total_count', x.total_count,
		         'previewed_count', x.previewed_count,
		         'applied_count', x.applied_count,
		         'skipped_count', x.skipped_count,
		         'conflict_count', x.conflict_count,
		         'failed_count', x.item_failed_count,
		         'erp_queued_count', x.queued_count,
		         'erp_synced_count', x.synced_count,
		         'erp_failed_count', x.failed_count,
		         'erp_syncable_count', x.applied_count,
		         'task_count', x.task_count,
		         'confirm_message', CONCAT('将重算 ', x.previewed_count, ' 个 SKU，涉及 ', x.task_count, ' 个任务，跳过 ', x.skipped_count, ' 条人工覆盖/需人工报价，冲突 ', x.conflict_count, ' 条'),
		         'confirmation_text', CONCAT('将重算 ', x.previewed_count, ' 个 SKU，涉及 ', x.task_count, ' 个任务，跳过 ', x.skipped_count, ' 条人工覆盖/需人工报价，冲突 ', x.conflict_count, ' 条'),
		         'erp_sync_message', CONCAT('可同步 ERP ', x.applied_count, ' 条，已入队 ', x.queued_count, ' 条，成功 ', x.synced_count, ' 条，失败 ', x.failed_count, ' 条')
		       )
		 WHERE r.status = 'erp_syncing'`)
	if err != nil {
		return fmt.Errorf("refresh cost recalculation erp run statuses: %w", err)
	}
	return nil
}

func costRecalculationRunSelectSQL() string {
	return `SELECT id, run_no, status, mode, filters_json, summary_json, created_by, applied_by, erp_synced_by,
	              created_at, previewed_at, applied_at, erp_synced_at, cancelled_at, updated_at
	         FROM cost_recalculation_runs`
}

func costRecalculationRunItemSelectSQL() string {
	return `SELECT id, run_id, product_management_record_id, task_id, task_no, task_sku_item_id, sku_code,
	              erp_i_id, product_i_id, normalized_i_id, old_cost_price, new_cost_price, cost_delta,
	              old_rule_id, new_rule_id, new_rule_version, match_mode, status, skip_reason,
	              conflict_reason, preview_snapshot_json, apply_snapshot_json, created_at, updated_at
	         FROM cost_recalculation_run_items`
}

func scanCostRecalculationRun(scanner interface{ Scan(...interface{}) error }) (*domain.CostRecalculationRun, error) {
	var item domain.CostRecalculationRun
	var filtersJSON, summaryJSON sql.NullString
	var createdBy, appliedBy, erpSyncedBy sql.NullInt64
	var previewedAt, appliedAt, erpSyncedAt, cancelledAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.RunNo,
		&item.Status,
		&item.Mode,
		&filtersJSON,
		&summaryJSON,
		&createdBy,
		&appliedBy,
		&erpSyncedBy,
		&item.CreatedAt,
		&previewedAt,
		&appliedAt,
		&erpSyncedAt,
		&cancelledAt,
		&item.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan cost recalculation run: %w", err)
	}
	item.FiltersJSON = rawJSONFromNullString(filtersJSON)
	item.SummaryJSON = rawJSONFromNullString(summaryJSON)
	item.CreatedBy = fromNullInt64(createdBy)
	item.AppliedBy = fromNullInt64(appliedBy)
	item.ERPSyncedBy = fromNullInt64(erpSyncedBy)
	item.PreviewedAt = fromNullTime(previewedAt)
	item.AppliedAt = fromNullTime(appliedAt)
	item.ERPSyncedAt = fromNullTime(erpSyncedAt)
	item.CancelledAt = fromNullTime(cancelledAt)
	decodeCostRunSummary(&item)
	return &item, nil
}

func scanCostRecalculationRunItem(scanner interface{ Scan(...interface{}) error }) (*domain.CostRecalculationRunItem, error) {
	var item domain.CostRecalculationRunItem
	var taskID, taskSKUItemID, oldRuleID, newRuleID sql.NullInt64
	var oldCost, newCost, costDelta sql.NullFloat64
	var newRuleVersion sql.NullInt64
	var previewJSON, applyJSON sql.NullString
	if err := scanner.Scan(
		&item.ID,
		&item.RunID,
		&item.ProductManagementRecordID,
		&taskID,
		&item.TaskNo,
		&taskSKUItemID,
		&item.SKUCode,
		&item.ERPIID,
		&item.ProductIID,
		&item.NormalizedIID,
		&oldCost,
		&newCost,
		&costDelta,
		&oldRuleID,
		&newRuleID,
		&newRuleVersion,
		&item.MatchMode,
		&item.Status,
		&item.SkipReason,
		&item.ConflictReason,
		&previewJSON,
		&applyJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan cost recalculation run item: %w", err)
	}
	item.TaskID = fromNullInt64(taskID)
	item.TaskSKUItemID = fromNullInt64(taskSKUItemID)
	item.OldCostPrice = fromNullFloat64(oldCost)
	item.NewCostPrice = fromNullFloat64(newCost)
	item.CostDelta = fromNullFloat64(costDelta)
	item.OldRuleID = fromNullInt64(oldRuleID)
	item.NewRuleID = fromNullInt64(newRuleID)
	item.NewRuleVersion = fromNullInt(newRuleVersion)
	item.PreviewSnapshotJSON = rawJSONFromNullString(previewJSON)
	item.ApplySnapshotJSON = rawJSONFromNullString(applyJSON)
	return &item, nil
}

func rawJSONFromNullString(value sql.NullString) []byte {
	if !value.Valid {
		return nil
	}
	raw := strings.TrimSpace(value.String)
	if raw == "" {
		return nil
	}
	return []byte(raw)
}

func decodeCostRunSummary(run *domain.CostRecalculationRun) {
	if run == nil || len(run.SummaryJSON) == 0 {
		return
	}
	var summary domain.CostRecalculationRunSummary
	if err := json.Unmarshal(run.SummaryJSON, &summary); err == nil {
		run.Summary = &summary
	}
}
