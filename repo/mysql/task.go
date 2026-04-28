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

type taskRepo struct{ db *DB }

func NewTaskRepo(db *DB) repo.TaskRepo { return &taskRepo{db: db} }

func (r *taskRepo) Create(ctx context.Context, tx repo.Tx, task *domain.Task, detail *domain.TaskDetail) (int64, error) {
	sqlTx := Unwrap(tx)
	filingStatus := detail.FilingStatus
	if !filingStatus.Valid() {
		if detail.FiledAt != nil {
			filingStatus = domain.FilingStatusFiled
		} else {
			filingStatus = domain.FilingStatusNotFiled
		}
	}

	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO tasks
		  (task_no, source_mode, product_id, sku_code, product_name_snapshot,
		   task_type, operator_group_id, owner_team, owner_department, owner_org_team, creator_id, requester_id, designer_id, current_handler_id,
		   task_status, priority, deadline_at, need_outsource, is_outsource, customization_required, customization_source_type,
		   last_customization_operator_id, warehouse_reject_reason, warehouse_reject_category,
		   is_batch_task, batch_item_count, batch_mode, primary_sku_code, sku_generation_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.TaskNo,
		string(task.SourceMode),
		toNullInt64(task.ProductID),
		task.SKUCode,
		task.ProductNameSnapshot,
		string(task.TaskType),
		toNullInt64(task.OperatorGroupID),
		task.OwnerTeam,
		sql.NullString{String: task.OwnerDepartment, Valid: strings.TrimSpace(task.OwnerDepartment) != ""},
		sql.NullString{String: task.OwnerOrgTeam, Valid: strings.TrimSpace(task.OwnerOrgTeam) != ""},
		task.CreatorID,
		toNullInt64(task.RequesterID),
		toNullInt64(task.DesignerID),
		toNullInt64(task.CurrentHandlerID),
		string(task.TaskStatus),
		string(task.Priority),
		toNullTime(task.DeadlineAt),
		task.NeedOutsource,
		task.IsOutsource,
		task.CustomizationRequired,
		string(task.CustomizationSourceType),
		toNullInt64(task.LastCustomizationOperatorID),
		task.WarehouseRejectReason,
		task.WarehouseRejectCategory,
		task.IsBatchTask,
		task.BatchItemCount,
		string(task.BatchMode),
		task.PrimarySKUCode,
		string(task.SKUGenerationStatus),
	)
	if err != nil {
		return 0, fmt.Errorf("insert task: %w", err)
	}
	taskID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id (task): %w", err)
	}

	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO task_details
		  (task_id, demand_text, copy_text, style_keywords, remark, note, risk_flags_json,
		   category, category_id, category_code, category_name,
		   source_product_id, source_product_name, source_search_entry_code, source_match_type, source_match_rule,
		   matched_category_code, matched_search_entry_code, matched_mapping_rule_json, product_selection_snapshot_json,
		   change_request, design_requirement, product_short_name, material_mode, material_other,
		   cost_price_mode, base_sale_price, product_channel, reference_images_json, reference_file_refs_json, reference_link,
		   spec_text, material, size_text, craft_text,
		   width, height, area, quantity, process,
		   procurement_price, cost_price, estimated_cost, cost_rule_id, cost_rule_name, cost_rule_source,
		   matched_rule_version, prefill_source, prefill_at,
		   requires_manual_review, manual_cost_override, manual_cost_override_reason, override_actor, override_at, filing_status, filing_error_message, filed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID,
		detail.DemandText,
		detail.CopyText,
		detail.StyleKeywords,
		detail.Remark,
		detail.Note,
		detail.RiskFlagsJSON,
		detail.Category,
		toNullInt64(detail.CategoryID),
		detail.CategoryCode,
		detail.CategoryName,
		toNullInt64(detail.SourceProductID),
		detail.SourceProductName,
		detail.SourceSearchEntryCode,
		detail.SourceMatchType,
		detail.SourceMatchRule,
		detail.MatchedCategoryCode,
		detail.MatchedSearchEntryCode,
		detail.MatchedMappingRuleJSON,
		detail.ProductSelectionSnapshotJSON,
		detail.ChangeRequest,
		detail.DesignRequirement,
		detail.ProductShortName,
		detail.MaterialMode,
		detail.MaterialOther,
		detail.CostPriceMode,
		toNullFloat64(detail.BaseSalePrice),
		detail.ProductChannel,
		detail.ReferenceImagesJSON,
		detail.ReferenceFileRefsJSON,
		detail.ReferenceLink,
		detail.SpecText,
		detail.Material,
		detail.SizeText,
		detail.CraftText,
		toNullFloat64(detail.Width),
		toNullFloat64(detail.Height),
		toNullFloat64(detail.Area),
		toNullInt64(detail.Quantity),
		detail.Process,
		toNullFloat64(detail.ProcurementPrice),
		toNullFloat64(detail.CostPrice),
		toNullFloat64(detail.EstimatedCost),
		toNullInt64(detail.CostRuleID),
		detail.CostRuleName,
		detail.CostRuleSource,
		toNullInt(detail.MatchedRuleVersion),
		detail.PrefillSource,
		toNullTime(detail.PrefillAt),
		detail.RequiresManualReview,
		detail.ManualCostOverride,
		detail.ManualCostOverrideReason,
		detail.OverrideActor,
		toNullTime(detail.OverrideAt),
		string(filingStatus),
		detail.FilingErrorMessage,
		toNullTime(detail.FiledAt),
	)
	if err != nil {
		return 0, fmt.Errorf("insert task_detail: %w", err)
	}
	if err := reindexTaskSearchDocument(ctx, sqlTx, taskID); err != nil {
		return 0, err
	}
	return taskID, nil
}

func (r *taskRepo) CreateSKUItems(ctx context.Context, tx repo.Tx, items []*domain.TaskSKUItem) error {
	if len(items) == 0 {
		return nil
	}
	sqlTx := Unwrap(tx)
	stmt, err := sqlTx.PrepareContext(ctx, `
		INSERT INTO task_sku_items
		  (task_id, sequence_no, sku_code, sku_status, product_id, erp_product_id,
		   product_name_snapshot, product_short_name, category_code, material_mode,
		   cost_price_mode, quantity, base_sale_price, design_requirement, variant_json, reference_file_refs_json, dedupe_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert task_sku_items: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		if item == nil {
			continue
		}
		res, err := stmt.ExecContext(
			ctx,
			item.TaskID,
			item.SequenceNo,
			item.SKUCode,
			string(item.SKUStatus),
			toNullInt64(item.ProductID),
			toNullStringPtr(item.ERPProductID),
			item.ProductNameSnapshot,
			item.ProductShortName,
			item.CategoryCode,
			item.MaterialMode,
			item.CostPriceMode,
			toNullInt64(item.Quantity),
			toNullFloat64(item.BaseSalePrice),
			item.DesignRequirement,
			toNullJSONString(item.VariantJSON),
			marshalReferenceFileRefs(item.ReferenceFileRefs),
			item.DedupeKey,
		)
		if err != nil {
			return fmt.Errorf("insert task_sku_item: %w", err)
		}
		if id, err := res.LastInsertId(); err == nil {
			item.ID = id
		}
	}
	return nil
}

func (r *taskRepo) GetByID(ctx context.Context, id int64) (*domain.Task, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, task_no, source_mode, product_id, sku_code, product_name_snapshot,
		       task_type, operator_group_id, owner_team, owner_department, owner_org_team, creator_id, requester_id, designer_id, current_handler_id,
		       task_status, priority, deadline_at, need_outsource, is_outsource, customization_required, customization_source_type,
		       last_customization_operator_id, warehouse_reject_reason, warehouse_reject_category,
		       is_batch_task, batch_item_count, batch_mode, primary_sku_code, sku_generation_status,
		       created_at, updated_at
		FROM tasks WHERE id = ?`, id)
	return scanTask(row)
}

func (r *taskRepo) GetDetailByTaskID(ctx context.Context, taskID int64) (*domain.TaskDetail, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, task_id, demand_text, copy_text, style_keywords, remark, COALESCE(note, ''), risk_flags_json,
		       category, category_id, category_code, category_name,
		       source_product_id, source_product_name, source_search_entry_code, source_match_type, source_match_rule,
		       matched_category_code, matched_search_entry_code, matched_mapping_rule_json, product_selection_snapshot_json,
		       change_request, design_requirement, product_short_name, material_mode, material_other,
		       cost_price_mode, base_sale_price, product_channel, reference_images_json, COALESCE(reference_file_refs_json, ''), reference_link,
		       spec_text, material, size_text, craft_text,
		       width, height, area, quantity, process,
		       procurement_price, cost_price, estimated_cost, cost_rule_id, cost_rule_name, cost_rule_source,
		       matched_rule_version, prefill_source, prefill_at,
		       requires_manual_review, manual_cost_override, manual_cost_override_reason, override_actor, override_at,
		       filing_status, COALESCE(filing_error_message, ''), COALESCE(filing_trigger_source, ''),
		       last_filing_attempt_at, last_filed_at, COALESCE(erp_sync_required, 0), COALESCE(erp_sync_version, 0),
		       COALESCE(last_filing_payload_hash, ''), COALESCE(last_filing_payload_json, ''), filed_at, created_at, updated_at
		FROM task_details WHERE task_id = ?`, taskID)

	var detail domain.TaskDetail
	var categoryID, sourceProductID, costRuleID, matchedRuleVersion sql.NullInt64
	var quantity sql.NullInt64
	var procurementPrice, costPrice, width, height, area, estimatedCost, baseSalePrice sql.NullFloat64
	var prefillAt, overrideAt, filedAt, lastFilingAttemptAt, lastFiledAt sql.NullTime
	var erpSyncRequired sql.NullBool
	var erpSyncVersion sql.NullInt64
	err := row.Scan(
		&detail.ID, &detail.TaskID, &detail.DemandText, &detail.CopyText, &detail.StyleKeywords,
		&detail.Remark, &detail.Note, &detail.RiskFlagsJSON, &detail.Category, &categoryID, &detail.CategoryCode, &detail.CategoryName,
		&sourceProductID, &detail.SourceProductName, &detail.SourceSearchEntryCode, &detail.SourceMatchType, &detail.SourceMatchRule,
		&detail.MatchedCategoryCode, &detail.MatchedSearchEntryCode, &detail.MatchedMappingRuleJSON, &detail.ProductSelectionSnapshotJSON,
		&detail.ChangeRequest, &detail.DesignRequirement, &detail.ProductShortName, &detail.MaterialMode, &detail.MaterialOther,
		&detail.CostPriceMode, &baseSalePrice, &detail.ProductChannel, &detail.ReferenceImagesJSON, &detail.ReferenceFileRefsJSON, &detail.ReferenceLink,
		&detail.SpecText,
		&detail.Material, &detail.SizeText, &detail.CraftText,
		&width, &height, &area, &quantity, &detail.Process,
		&procurementPrice, &costPrice, &estimatedCost, &costRuleID, &detail.CostRuleName, &detail.CostRuleSource,
		&matchedRuleVersion, &detail.PrefillSource, &prefillAt,
		&detail.RequiresManualReview, &detail.ManualCostOverride, &detail.ManualCostOverrideReason, &detail.OverrideActor, &overrideAt,
		&detail.FilingStatus, &detail.FilingErrorMessage, &detail.FilingTriggerSource,
		&lastFilingAttemptAt, &lastFiledAt, &erpSyncRequired, &erpSyncVersion,
		&detail.LastFilingPayloadHash, &detail.LastFilingPayloadJSON, &filedAt, &detail.CreatedAt, &detail.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan task_detail: %w", err)
	}
	detail.CategoryID = fromNullInt64(categoryID)
	detail.SourceProductID = fromNullInt64(sourceProductID)
	detail.BaseSalePrice = fromNullFloat64(baseSalePrice)
	detail.Width = fromNullFloat64(width)
	detail.Height = fromNullFloat64(height)
	detail.Area = fromNullFloat64(area)
	detail.Quantity = fromNullInt64(quantity)
	detail.ProcurementPrice = fromNullFloat64(procurementPrice)
	detail.CostPrice = fromNullFloat64(costPrice)
	detail.EstimatedCost = fromNullFloat64(estimatedCost)
	detail.CostRuleID = fromNullInt64(costRuleID)
	detail.MatchedRuleVersion = fromNullInt(matchedRuleVersion)
	detail.PrefillAt = fromNullTime(prefillAt)
	detail.OverrideAt = fromNullTime(overrideAt)
	detail.LastFilingAttemptAt = fromNullTime(lastFilingAttemptAt)
	detail.LastFiledAt = fromNullTime(lastFiledAt)
	if erpSyncRequired.Valid {
		detail.ERPSyncRequired = erpSyncRequired.Bool
	}
	if erpSyncVersion.Valid {
		detail.ERPSyncVersion = erpSyncVersion.Int64
	}
	detail.FiledAt = fromNullTime(filedAt)
	if !detail.FilingStatus.Valid() {
		if detail.FiledAt != nil {
			detail.FilingStatus = domain.FilingStatusFiled
		} else {
			detail.FilingStatus = domain.FilingStatusNotFiled
		}
	}
	return &detail, nil
}

func (r *taskRepo) GetSKUItemBySKUCode(ctx context.Context, skuCode string) (*domain.TaskSKUItem, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, task_id, sequence_no, sku_code, sku_status, product_id, erp_product_id,
		       product_name_snapshot, product_short_name, category_code, material_mode,
		       cost_price_mode, quantity, base_sale_price, design_requirement, variant_json, COALESCE(reference_file_refs_json, ''),
		       dedupe_key, created_at, updated_at
		FROM task_sku_items
		WHERE sku_code = ?`, skuCode)
	item, err := scanTaskSKUItem(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task_sku_item by sku_code: %w", err)
	}
	return item, nil
}

func (r *taskRepo) ListSKUItemsByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskSKUItem, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_id, sequence_no, sku_code, sku_status, product_id, erp_product_id,
		       product_name_snapshot, product_short_name, category_code, material_mode,
		       cost_price_mode, quantity, base_sale_price, design_requirement, variant_json, COALESCE(reference_file_refs_json, ''),
		       dedupe_key, created_at, updated_at
		FROM task_sku_items
		WHERE task_id = ?
		ORDER BY sequence_no ASC, id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task_sku_items: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.TaskSKUItem, 0)
	for rows.Next() {
		item, err := scanTaskSKUItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task_sku_items: %w", err)
	}
	return items, nil
}

func (r *taskRepo) List(ctx context.Context, filter repo.TaskListFilter) ([]*domain.TaskListItem, int64, error) {
	spec, err := buildTaskListQuerySpec(filter, nil)
	if err != nil {
		return nil, 0, err
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) %s WHERE %s`, spec.fromSQL, spec.whereSQL)
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, spec.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize

	query := fmt.Sprintf(`
		SELECT t.id, t.task_no, t.product_id, t.sku_code, t.product_name_snapshot,
		       t.task_type, t.source_mode, t.owner_team, COALESCE(t.owner_department, ''), COALESCE(t.owner_org_team, ''), t.priority, t.creator_id, t.requester_id, t.designer_id, t.current_handler_id,
		       COALESCE(requester_user.display_name, requester_user.username, ''), COALESCE(creator_user.display_name, creator_user.username, ''), COALESCE(designer_user.display_name, designer_user.username, ''), COALESCE(handler_user.display_name, handler_user.username, ''),
		       t.task_status, t.created_at, t.updated_at, t.deadline_at, t.need_outsource, t.is_outsource, t.customization_required, COALESCE(t.customization_source_type, ''),
		       t.last_customization_operator_id, COALESCE(t.warehouse_reject_reason, ''), COALESCE(t.warehouse_reject_category, ''),
		       t.is_batch_task, t.batch_item_count, t.batch_mode, COALESCE(t.primary_sku_code, ''),
		       td.category, td.category_code, td.category_name,
		       td.source_product_id, td.source_product_name, td.source_search_entry_code, td.source_match_type, td.source_match_rule,
		       td.matched_category_code, td.matched_search_entry_code, td.product_selection_snapshot_json,
		       td.spec_text, td.material, td.size_text, td.craft_text,
		       pr.procurement_price, pr.status AS procurement_status, pr.quantity, pr.supplier_name, pr.expected_delivery_at,
		       td.cost_price, td.estimated_cost, td.cost_rule_id, td.cost_rule_name, td.cost_rule_source,
		       td.matched_rule_version, td.prefill_source, td.prefill_at,
		       td.requires_manual_review, td.manual_cost_override, td.manual_cost_override_reason, td.override_actor, td.override_at,
		       td.filing_status, COALESCE(td.filing_error_message, ''), COALESCE(td.filing_trigger_source, ''),
		       td.last_filing_attempt_at, td.last_filed_at, COALESCE(td.erp_sync_required, 0), COALESCE(td.erp_sync_version, 0),
		       COALESCE(td.last_filing_payload_hash, ''), COALESCE(td.last_filing_payload_json, ''), td.filed_at,
		       wr.status AS warehouse_status,
		       %s AS latest_asset_type
		%s
		%s
		WHERE %s
		ORDER BY t.updated_at DESC, t.id DESC
		LIMIT ? OFFSET ?`, spec.latestAssetExpr, spec.fromSQL, taskActorNameJoins(), spec.whereSQL)
	pageArgs := append(append([]interface{}{}, spec.args...), pageSize, offset)

	rows, err := r.db.db.QueryContext(ctx, query, pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var items []*domain.TaskListItem
	for rows.Next() {
		item, err := scanTaskListItemRow(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *taskRepo) ListBoardCandidates(ctx context.Context, filter repo.TaskBoardCandidateFilter) ([]*domain.TaskListItem, error) {
	if len(filter.CandidateFilters) == 0 {
		return []*domain.TaskListItem{}, nil
	}

	spec, err := buildTaskListQuerySpec(filter.TaskListFilter, filter.CandidateFilters)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.task_no, t.product_id, t.sku_code, t.product_name_snapshot,
		       t.task_type, t.source_mode, t.owner_team, COALESCE(t.owner_department, ''), COALESCE(t.owner_org_team, ''), t.priority, t.creator_id, t.requester_id, t.designer_id, t.current_handler_id,
		       COALESCE(requester_user.display_name, requester_user.username, ''), COALESCE(creator_user.display_name, creator_user.username, ''), COALESCE(designer_user.display_name, designer_user.username, ''), COALESCE(handler_user.display_name, handler_user.username, ''),
		       t.task_status, t.created_at, t.updated_at, t.deadline_at, t.need_outsource, t.is_outsource, t.customization_required, COALESCE(t.customization_source_type, ''),
		       t.last_customization_operator_id, COALESCE(t.warehouse_reject_reason, ''), COALESCE(t.warehouse_reject_category, ''),
		       t.is_batch_task, t.batch_item_count, t.batch_mode, COALESCE(t.primary_sku_code, ''),
		       td.category, td.category_code, td.category_name,
		       td.source_product_id, td.source_product_name, td.source_search_entry_code, td.source_match_type, td.source_match_rule,
		       td.matched_category_code, td.matched_search_entry_code, td.product_selection_snapshot_json,
		       td.spec_text, td.material, td.size_text, td.craft_text,
		       pr.procurement_price, pr.status AS procurement_status, pr.quantity, pr.supplier_name, pr.expected_delivery_at,
		       td.cost_price, td.estimated_cost, td.cost_rule_id, td.cost_rule_name, td.cost_rule_source,
		       td.matched_rule_version, td.prefill_source, td.prefill_at,
		       td.requires_manual_review, td.manual_cost_override, td.manual_cost_override_reason, td.override_actor, td.override_at,
		       td.filing_status, COALESCE(td.filing_error_message, ''), COALESCE(td.filing_trigger_source, ''),
		       td.last_filing_attempt_at, td.last_filed_at, COALESCE(td.erp_sync_required, 0), COALESCE(td.erp_sync_version, 0),
		       COALESCE(td.last_filing_payload_hash, ''), COALESCE(td.last_filing_payload_json, ''), td.filed_at,
		       wr.status AS warehouse_status,
		       %s AS latest_asset_type
		%s
		%s
		WHERE %s
		ORDER BY t.updated_at DESC, t.id DESC`, spec.latestAssetExpr, spec.fromSQL, taskActorNameJoins(), spec.whereSQL)

	rows, err := r.db.db.QueryContext(ctx, query, spec.args...)
	if err != nil {
		return nil, fmt.Errorf("list board candidates: %w", err)
	}
	defer rows.Close()

	var items []*domain.TaskListItem
	for rows.Next() {
		item, err := scanTaskListItemRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func taskActorNameJoins() string {
	return `
		LEFT JOIN users requester_user ON requester_user.id = t.requester_id
		LEFT JOIN users creator_user ON creator_user.id = t.creator_id
		LEFT JOIN users designer_user ON designer_user.id = t.designer_id
		LEFT JOIN users handler_user ON handler_user.id = t.current_handler_id`
}

func (r *taskRepo) UpdateStatus(ctx context.Context, tx repo.Tx, id int64, status domain.TaskStatus) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE tasks SET task_status = ? WHERE id = ?`,
		string(status), id,
	)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	if err := reindexTaskSearchDocument(ctx, sqlTx, id); err != nil {
		return err
	}
	return nil
}

func (r *taskRepo) UpdateDetailBusinessInfo(ctx context.Context, tx repo.Tx, detail *domain.TaskDetail) error {
	sqlTx := Unwrap(tx)
	filingStatus := detail.FilingStatus
	if !filingStatus.Valid() {
		if detail.FiledAt != nil {
			filingStatus = domain.FilingStatusFiled
		} else {
			filingStatus = domain.FilingStatusNotFiled
		}
	}
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE task_details
		SET category = ?,
		    category_id = ?,
		    category_code = ?,
		    category_name = ?,
		    source_product_id = ?,
		    source_product_name = ?,
		    source_search_entry_code = ?,
		    source_match_type = ?,
		    source_match_rule = ?,
		    matched_category_code = ?,
		    matched_search_entry_code = ?,
		    matched_mapping_rule_json = ?,
		    product_selection_snapshot_json = ?,
		    note = ?,
		    reference_file_refs_json = ?,
		    reference_link = ?,
		    spec_text = ?,
		    material = ?,
		    size_text = ?,
		    craft_text = ?,
		    width = ?,
		    height = ?,
		    area = ?,
		    quantity = ?,
		    process = ?,
		    procurement_price = ?,
		    cost_price = ?,
		    estimated_cost = ?,
		    cost_rule_id = ?,
		    cost_rule_name = ?,
		    cost_rule_source = ?,
		    matched_rule_version = ?,
		    prefill_source = ?,
		    prefill_at = ?,
		    requires_manual_review = ?,
		    manual_cost_override = ?,
		    manual_cost_override_reason = ?,
		    override_actor = ?,
		    override_at = ?,
		    filing_status = ?,
		    filing_error_message = ?,
		    filing_trigger_source = ?,
		    last_filing_attempt_at = ?,
		    last_filed_at = ?,
		    erp_sync_required = ?,
		    erp_sync_version = ?,
		    last_filing_payload_hash = ?,
		    last_filing_payload_json = ?,
		    filed_at = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE task_id = ?`,
		detail.Category,
		toNullInt64(detail.CategoryID),
		detail.CategoryCode,
		detail.CategoryName,
		toNullInt64(detail.SourceProductID),
		detail.SourceProductName,
		detail.SourceSearchEntryCode,
		detail.SourceMatchType,
		detail.SourceMatchRule,
		detail.MatchedCategoryCode,
		detail.MatchedSearchEntryCode,
		detail.MatchedMappingRuleJSON,
		detail.ProductSelectionSnapshotJSON,
		detail.Note,
		detail.ReferenceFileRefsJSON,
		detail.ReferenceLink,
		detail.SpecText,
		detail.Material,
		detail.SizeText,
		detail.CraftText,
		toNullFloat64(detail.Width),
		toNullFloat64(detail.Height),
		toNullFloat64(detail.Area),
		toNullInt64(detail.Quantity),
		detail.Process,
		toNullFloat64(detail.ProcurementPrice),
		toNullFloat64(detail.CostPrice),
		toNullFloat64(detail.EstimatedCost),
		toNullInt64(detail.CostRuleID),
		detail.CostRuleName,
		detail.CostRuleSource,
		toNullInt(detail.MatchedRuleVersion),
		detail.PrefillSource,
		toNullTime(detail.PrefillAt),
		detail.RequiresManualReview,
		detail.ManualCostOverride,
		detail.ManualCostOverrideReason,
		detail.OverrideActor,
		toNullTime(detail.OverrideAt),
		string(filingStatus),
		detail.FilingErrorMessage,
		detail.FilingTriggerSource,
		toNullTime(detail.LastFilingAttemptAt),
		toNullTime(detail.LastFiledAt),
		detail.ERPSyncRequired,
		detail.ERPSyncVersion,
		detail.LastFilingPayloadHash,
		detail.LastFilingPayloadJSON,
		toNullTime(detail.FiledAt),
		detail.TaskID,
	)
	if err != nil {
		return fmt.Errorf("update task detail business info: %w", err)
	}
	if err := reindexTaskSearchDocument(ctx, sqlTx, detail.TaskID); err != nil {
		return err
	}
	return nil
}

func (r *taskRepo) UpdateProductBinding(ctx context.Context, tx repo.Tx, task *domain.Task) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE tasks SET product_id = ?, sku_code = ?, product_name_snapshot = ? WHERE id = ?`,
		toNullInt64(task.ProductID),
		task.SKUCode,
		task.ProductNameSnapshot,
		task.ID,
	)
	if err != nil {
		return fmt.Errorf("update task product binding: %w", err)
	}
	if err := reindexTaskSearchDocument(ctx, sqlTx, task.ID); err != nil {
		return err
	}
	return nil
}

func (r *taskRepo) UpdateDesigner(ctx context.Context, tx repo.Tx, id int64, designerID *int64) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE tasks SET designer_id = ? WHERE id = ?`,
		toNullInt64(designerID), id,
	)
	if err != nil {
		return fmt.Errorf("update task designer: %w", err)
	}
	if err := reindexTaskSearchDocument(ctx, sqlTx, id); err != nil {
		return err
	}
	return nil
}

func (r *taskRepo) ClaimPendingAssignment(ctx context.Context, tx repo.Tx, id int64, designerID int64, resultingStatus domain.TaskStatus) (bool, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE tasks
		   SET designer_id = ?,
		       current_handler_id = ?,
		       task_status = ?
		 WHERE id = ?
		   AND task_status = ?
		   AND designer_id IS NULL
		   AND current_handler_id IS NULL`,
		designerID,
		designerID,
		string(resultingStatus),
		id,
		string(domain.TaskStatusPendingAssign),
	)
	if err != nil {
		return false, fmt.Errorf("claim pending task assignment: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim pending task assignment rows affected: %w", err)
	}
	if affected == 1 {
		if err := reindexTaskSearchDocument(ctx, sqlTx, id); err != nil {
			return false, err
		}
	}
	return affected == 1, nil
}

func (r *taskRepo) UpdateHandler(ctx context.Context, tx repo.Tx, id int64, handlerID *int64) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE tasks SET current_handler_id = ? WHERE id = ?`,
		toNullInt64(handlerID), id,
	)
	if err != nil {
		return fmt.Errorf("update task handler: %w", err)
	}
	if err := reindexTaskSearchDocument(ctx, sqlTx, id); err != nil {
		return err
	}
	return nil
}

func (r *taskRepo) UpdateNeedOutsource(ctx context.Context, tx repo.Tx, id int64, needOutsource bool) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE tasks SET need_outsource = ? WHERE id = ?`,
		needOutsource, id,
	)
	if err != nil {
		return fmt.Errorf("update task need_outsource: %w", err)
	}
	return nil
}

func (r *taskRepo) UpdateCustomizationState(ctx context.Context, tx repo.Tx, id int64, lastOperatorID *int64, rejectReason, rejectCategory string) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE tasks
		 SET last_customization_operator_id = ?,
		     warehouse_reject_reason = ?,
		     warehouse_reject_category = ?
		 WHERE id = ?`,
		toNullInt64(lastOperatorID),
		rejectReason,
		rejectCategory,
		id,
	)
	if err != nil {
		return fmt.Errorf("update task customization state: %w", err)
	}
	if err := reindexTaskSearchDocument(ctx, sqlTx, id); err != nil {
		return err
	}
	return nil
}

func scanTask(row *sql.Row) (*domain.Task, error) {
	var t domain.Task
	var productID, operatorGroupID, requesterID, designerID, currentHandlerID, lastCustomizationOperatorID sql.NullInt64
	var deadlineAt sql.NullTime
	var ownerDepartment, ownerOrgTeam, customizationSourceType, warehouseRejectReason, warehouseRejectCategory sql.NullString
	var primarySKUCode sql.NullString
	err := row.Scan(
		&t.ID, &t.TaskNo, &t.SourceMode, &productID, &t.SKUCode, &t.ProductNameSnapshot,
		&t.TaskType, &operatorGroupID, &t.OwnerTeam, &ownerDepartment, &ownerOrgTeam, &t.CreatorID, &requesterID, &designerID, &currentHandlerID,
		&t.TaskStatus, &t.Priority, &deadlineAt, &t.NeedOutsource, &t.IsOutsource, &t.CustomizationRequired, &customizationSourceType,
		&lastCustomizationOperatorID, &warehouseRejectReason, &warehouseRejectCategory,
		&t.IsBatchTask, &t.BatchItemCount, &t.BatchMode, &primarySKUCode, &t.SKUGenerationStatus,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan task: %w", err)
	}
	t.ProductID = fromNullInt64(productID)
	t.OperatorGroupID = fromNullInt64(operatorGroupID)
	t.RequesterID = fromNullInt64(requesterID)
	t.DesignerID = fromNullInt64(designerID)
	t.CurrentHandlerID = fromNullInt64(currentHandlerID)
	t.LastCustomizationOperatorID = fromNullInt64(lastCustomizationOperatorID)
	if ownerDepartment.Valid {
		t.OwnerDepartment = ownerDepartment.String
	}
	if ownerOrgTeam.Valid {
		t.OwnerOrgTeam = ownerOrgTeam.String
	}
	t.DeadlineAt = fromNullTime(deadlineAt)
	if customizationSourceType.Valid {
		t.CustomizationSourceType = domain.CustomizationSourceType(customizationSourceType.String)
	}
	if warehouseRejectReason.Valid {
		t.WarehouseRejectReason = warehouseRejectReason.String
	}
	if warehouseRejectCategory.Valid {
		t.WarehouseRejectCategory = warehouseRejectCategory.String
	}
	if primarySKUCode.Valid {
		t.PrimarySKUCode = primarySKUCode.String
	}
	if !t.BatchMode.Valid() {
		t.BatchMode = domain.TaskBatchModeSingle
	}
	if t.PrimarySKUCode == "" {
		t.PrimarySKUCode = t.SKUCode
	}
	if t.BatchItemCount == 0 && t.SKUCode != "" {
		t.BatchItemCount = 1
	}
	if !t.SKUGenerationStatus.Valid() {
		if t.TaskType == domain.TaskTypeNewProductDevelopment || t.TaskType == domain.TaskTypePurchaseTask {
			t.SKUGenerationStatus = domain.TaskSKUGenerationStatusCompleted
		} else {
			t.SKUGenerationStatus = domain.TaskSKUGenerationStatusNotApplicable
		}
	}
	return &t, nil
}

func scanTaskRow(rows *sql.Rows) (*domain.Task, error) {
	var t domain.Task
	var productID, operatorGroupID, requesterID, designerID, currentHandlerID, lastCustomizationOperatorID sql.NullInt64
	var deadlineAt sql.NullTime
	var ownerDepartment, ownerOrgTeam, customizationSourceType, warehouseRejectReason, warehouseRejectCategory sql.NullString
	var primarySKUCode sql.NullString
	err := rows.Scan(
		&t.ID, &t.TaskNo, &t.SourceMode, &productID, &t.SKUCode, &t.ProductNameSnapshot,
		&t.TaskType, &operatorGroupID, &t.OwnerTeam, &ownerDepartment, &ownerOrgTeam, &t.CreatorID, &requesterID, &designerID, &currentHandlerID,
		&t.TaskStatus, &t.Priority, &deadlineAt, &t.NeedOutsource, &t.IsOutsource, &t.CustomizationRequired, &customizationSourceType,
		&lastCustomizationOperatorID, &warehouseRejectReason, &warehouseRejectCategory,
		&t.IsBatchTask, &t.BatchItemCount, &t.BatchMode, &primarySKUCode, &t.SKUGenerationStatus,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan task row: %w", err)
	}
	t.ProductID = fromNullInt64(productID)
	t.OperatorGroupID = fromNullInt64(operatorGroupID)
	t.RequesterID = fromNullInt64(requesterID)
	t.DesignerID = fromNullInt64(designerID)
	t.CurrentHandlerID = fromNullInt64(currentHandlerID)
	t.LastCustomizationOperatorID = fromNullInt64(lastCustomizationOperatorID)
	if ownerDepartment.Valid {
		t.OwnerDepartment = ownerDepartment.String
	}
	if ownerOrgTeam.Valid {
		t.OwnerOrgTeam = ownerOrgTeam.String
	}
	t.DeadlineAt = fromNullTime(deadlineAt)
	if customizationSourceType.Valid {
		t.CustomizationSourceType = domain.CustomizationSourceType(customizationSourceType.String)
	}
	if warehouseRejectReason.Valid {
		t.WarehouseRejectReason = warehouseRejectReason.String
	}
	if warehouseRejectCategory.Valid {
		t.WarehouseRejectCategory = warehouseRejectCategory.String
	}
	if primarySKUCode.Valid {
		t.PrimarySKUCode = primarySKUCode.String
	}
	if !t.BatchMode.Valid() {
		t.BatchMode = domain.TaskBatchModeSingle
	}
	if t.PrimarySKUCode == "" {
		t.PrimarySKUCode = t.SKUCode
	}
	if t.BatchItemCount == 0 && t.SKUCode != "" {
		t.BatchItemCount = 1
	}
	if !t.SKUGenerationStatus.Valid() {
		if t.TaskType == domain.TaskTypeNewProductDevelopment || t.TaskType == domain.TaskTypePurchaseTask {
			t.SKUGenerationStatus = domain.TaskSKUGenerationStatusCompleted
		} else {
			t.SKUGenerationStatus = domain.TaskSKUGenerationStatusNotApplicable
		}
	}
	return &t, nil
}

func scanTaskListItemRow(rows *sql.Rows) (*domain.TaskListItem, error) {
	var item domain.TaskListItem
	var productID, requesterID, designerID, currentHandlerID sql.NullInt64
	var sourceProductID sql.NullInt64
	var deadlineAt sql.NullTime
	var batchMode sql.NullString
	var primarySKUCode sql.NullString
	var customizationSourceType sql.NullString
	var lastCustomizationOperatorID sql.NullInt64
	var warehouseRejectReason sql.NullString
	var warehouseRejectCategory sql.NullString
	var warehouseStatus sql.NullString
	var latestAssetType sql.NullString
	var procurementStatus sql.NullString
	var procurementQuantity sql.NullInt64
	var supplierName sql.NullString
	var expectedDeliveryAt sql.NullTime
	var costRuleID, matchedRuleVersion sql.NullInt64
	var procurementPrice, costPrice, estimatedCost sql.NullFloat64
	var category, categoryCode, categoryName sql.NullString
	var sourceProductName, sourceSearchEntryCode, sourceMatchType, sourceMatchRule sql.NullString
	var matchedCategoryCode, matchedSearchEntryCode sql.NullString
	var productSelectionSnapshotJSON sql.NullString
	var specText, material, sizeText, craftText sql.NullString
	var costRuleName, costRuleSource, prefillSource, manualCostOverrideReason, overrideActor sql.NullString
	var requiresManualReview, manualCostOverride sql.NullBool
	var filingStatus sql.NullString
	var filingErrorMessage, filingTriggerSource, lastFilingPayloadHash, lastFilingPayloadJSON sql.NullString
	var requesterName, creatorName, designerName, currentHandlerName sql.NullString
	var erpSyncRequired sql.NullBool
	var erpSyncVersion sql.NullInt64
	var prefillAt, overrideAt, filedAt, lastFilingAttemptAt, lastFiledAt sql.NullTime
	if err := rows.Scan(
		&item.ID, &item.TaskNo, &productID, &item.SKUCode, &item.ProductNameSnapshot,
		&item.TaskType, &item.SourceMode, &item.OwnerTeam, &item.OwnerDepartment, &item.OwnerOrgTeam, &item.Priority, &item.CreatorID, &requesterID, &designerID, &currentHandlerID,
		&requesterName, &creatorName, &designerName, &currentHandlerName,
		&item.TaskStatus, &item.CreatedAt, &item.UpdatedAt, &deadlineAt, &item.NeedOutsource, &item.IsOutsource, &item.CustomizationRequired, &customizationSourceType,
		&lastCustomizationOperatorID, &warehouseRejectReason, &warehouseRejectCategory,
		&item.IsBatchTask, &item.BatchItemCount, &batchMode, &primarySKUCode,
		&category, &categoryCode, &categoryName,
		&sourceProductID, &sourceProductName, &sourceSearchEntryCode, &sourceMatchType, &sourceMatchRule,
		&matchedCategoryCode, &matchedSearchEntryCode, &productSelectionSnapshotJSON,
		&specText, &material, &sizeText, &craftText,
		&procurementPrice, &procurementStatus, &procurementQuantity, &supplierName, &expectedDeliveryAt,
		&costPrice, &estimatedCost, &costRuleID, &costRuleName, &costRuleSource,
		&matchedRuleVersion, &prefillSource, &prefillAt,
		&requiresManualReview, &manualCostOverride, &manualCostOverrideReason, &overrideActor, &overrideAt,
		&filingStatus, &filingErrorMessage, &filingTriggerSource, &lastFilingAttemptAt, &lastFiledAt, &erpSyncRequired, &erpSyncVersion,
		&lastFilingPayloadHash, &lastFilingPayloadJSON, &filedAt,
		&warehouseStatus, &latestAssetType,
	); err != nil {
		return nil, fmt.Errorf("scan task list item: %w", err)
	}
	item.ProductID = fromNullInt64(productID)
	item.RequesterID = fromNullInt64(requesterID)
	item.DesignerID = fromNullInt64(designerID)
	item.CurrentHandlerID = fromNullInt64(currentHandlerID)
	item.DeadlineAt = fromNullTime(deadlineAt)
	item.WorkflowLane = domain.WorkflowLaneFromCustomizationRequired(item.CustomizationRequired)
	item.LastCustomizationOperatorID = fromNullInt64(lastCustomizationOperatorID)
	if customizationSourceType.Valid {
		item.CustomizationSourceType = domain.CustomizationSourceType(customizationSourceType.String)
	}
	if warehouseRejectReason.Valid {
		item.WarehouseRejectReason = warehouseRejectReason.String
	}
	if warehouseRejectCategory.Valid {
		item.WarehouseRejectCategory = warehouseRejectCategory.String
	}
	if batchMode.Valid {
		item.BatchMode = domain.TaskBatchMode(batchMode.String)
	}
	if primarySKUCode.Valid {
		item.PrimarySKUCode = primarySKUCode.String
	}
	if !item.BatchMode.Valid() {
		item.BatchMode = domain.TaskBatchModeSingle
	}
	if item.PrimarySKUCode == "" {
		item.PrimarySKUCode = item.SKUCode
	}
	if item.BatchItemCount == 0 && item.SKUCode != "" {
		item.BatchItemCount = 1
	}
	if category.Valid {
		item.Category = category.String
	}
	if requesterName.Valid {
		item.RequesterName = requesterName.String
	}
	if creatorName.Valid {
		item.CreatorName = creatorName.String
	}
	if designerName.Valid {
		item.DesignerName = designerName.String
	}
	if currentHandlerName.Valid {
		item.CurrentHandlerName = currentHandlerName.String
	}
	if warehouseStatus.Valid {
		status := domain.WarehouseReceiptStatus(warehouseStatus.String)
		item.WarehouseStatus = &status
	}
	if latestAssetType.Valid {
		assetType := domain.TaskAssetType(latestAssetType.String)
		item.LatestAssetType = &assetType
	}
	item.ProcurementPrice = fromNullFloat64(procurementPrice)
	if categoryCode.Valid {
		item.CategoryCode = categoryCode.String
	}
	if categoryName.Valid {
		item.CategoryName = categoryName.String
	}
	item.SourceProductID = fromNullInt64(sourceProductID)
	if sourceProductName.Valid {
		item.SourceProductName = sourceProductName.String
	}
	if sourceSearchEntryCode.Valid {
		item.SourceSearchEntryCode = sourceSearchEntryCode.String
	}
	if sourceMatchType.Valid {
		item.SourceMatchType = sourceMatchType.String
	}
	if sourceMatchRule.Valid {
		item.SourceMatchRule = sourceMatchRule.String
	}
	if matchedCategoryCode.Valid {
		item.MatchedCategoryCode = matchedCategoryCode.String
	}
	if matchedSearchEntryCode.Valid {
		item.MatchedSearchEntryCode = matchedSearchEntryCode.String
	}
	if productSelectionSnapshotJSON.Valid {
		item.ProductSelectionSnapshotJSON = productSelectionSnapshotJSON.String
	}
	if specText.Valid {
		item.SpecText = specText.String
	}
	if material.Valid {
		item.Material = material.String
	}
	if sizeText.Valid {
		item.SizeText = sizeText.String
	}
	if craftText.Valid {
		item.CraftText = craftText.String
	}
	if procurementStatus.Valid {
		status := domain.ProcurementStatus(procurementStatus.String)
		item.ProcurementStatus = &status
	}
	item.ProcurementQuantity = fromNullInt64(procurementQuantity)
	if supplierName.Valid {
		item.SupplierName = supplierName.String
	}
	item.ExpectedDeliveryAt = fromNullTime(expectedDeliveryAt)
	item.CostPrice = fromNullFloat64(costPrice)
	item.EstimatedCost = fromNullFloat64(estimatedCost)
	item.CostRuleID = fromNullInt64(costRuleID)
	if costRuleName.Valid {
		item.CostRuleName = costRuleName.String
	}
	if costRuleSource.Valid {
		item.CostRuleSource = costRuleSource.String
	}
	item.MatchedRuleVersion = fromNullInt(matchedRuleVersion)
	if prefillSource.Valid {
		item.PrefillSource = prefillSource.String
	}
	item.PrefillAt = fromNullTime(prefillAt)
	if requiresManualReview.Valid {
		item.RequiresManualReview = requiresManualReview.Bool
	}
	if manualCostOverride.Valid {
		item.ManualCostOverride = manualCostOverride.Bool
	}
	if manualCostOverrideReason.Valid {
		item.ManualCostOverrideReason = manualCostOverrideReason.String
	}
	if overrideActor.Valid {
		item.OverrideActor = overrideActor.String
	}
	item.OverrideAt = fromNullTime(overrideAt)
	if filingStatus.Valid {
		item.FilingStatus = domain.FilingStatus(filingStatus.String)
	}
	if filingErrorMessage.Valid {
		item.FilingErrorMessage = filingErrorMessage.String
	}
	if filingTriggerSource.Valid {
		item.FilingTriggerSource = filingTriggerSource.String
	}
	item.LastFilingAttemptAt = fromNullTime(lastFilingAttemptAt)
	item.LastFiledAt = fromNullTime(lastFiledAt)
	if erpSyncRequired.Valid {
		item.ERPSyncRequired = erpSyncRequired.Bool
	}
	if erpSyncVersion.Valid {
		item.ERPSyncVersion = erpSyncVersion.Int64
	}
	if lastFilingPayloadHash.Valid {
		item.LastFilingPayloadHash = lastFilingPayloadHash.String
	}
	if lastFilingPayloadJSON.Valid {
		item.LastFilingPayloadJSON = lastFilingPayloadJSON.String
	}
	if !item.FilingStatus.Valid() {
		if filedAt.Valid {
			item.FilingStatus = domain.FilingStatusFiled
		} else {
			item.FilingStatus = domain.FilingStatusNotFiled
		}
	}
	item.FiledAt = fromNullTime(filedAt)
	return &item, nil
}

func scanTaskSKUItem(scanner interface{ Scan(...interface{}) error }) (*domain.TaskSKUItem, error) {
	var item domain.TaskSKUItem
	var productID sql.NullInt64
	var erpProductID sql.NullString
	var quantity sql.NullInt64
	var baseSalePrice sql.NullFloat64
	var variantJSON []byte
	var referenceFileRefsJSON sql.NullString
	if err := scanner.Scan(
		&item.ID,
		&item.TaskID,
		&item.SequenceNo,
		&item.SKUCode,
		&item.SKUStatus,
		&productID,
		&erpProductID,
		&item.ProductNameSnapshot,
		&item.ProductShortName,
		&item.CategoryCode,
		&item.MaterialMode,
		&item.CostPriceMode,
		&quantity,
		&baseSalePrice,
		&item.DesignRequirement,
		&variantJSON,
		&referenceFileRefsJSON,
		&item.DedupeKey,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("scan task_sku_item: %w", err)
	}
	item.ProductID = fromNullInt64(productID)
	item.ERPProductID = fromNullString(erpProductID)
	item.Quantity = fromNullInt64(quantity)
	item.BaseSalePrice = fromNullFloat64(baseSalePrice)
	if len(variantJSON) > 0 {
		item.VariantJSON = append(item.VariantJSON[:0], variantJSON...)
		item.ProductIID = productIIDFromVariantJSON(variantJSON)
	}
	if referenceFileRefsJSON.Valid {
		item.ReferenceFileRefs = domain.ParseReferenceFileRefsJSON(referenceFileRefsJSON.String)
	}
	if item.ReferenceFileRefs == nil {
		item.ReferenceFileRefs = []domain.ReferenceFileRef{}
	}
	return &item, nil
}

func productIIDFromVariantJSON(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"product_i_id", "i_id"} {
		if value, ok := obj[key]; ok {
			if text, ok := value.(string); ok {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func marshalReferenceFileRefs(refs []domain.ReferenceFileRef) string {
	normalized := domain.NormalizeReferenceFileRefs(refs)
	if len(normalized) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

type taskListQueryExpressions struct {
	mainStatusExpr            string
	latestAssetExpr           string
	subStatusExprs            map[domain.TaskSubStatusScope]string
	coordinationStatusExpr    string
	warehousePrepareReadyExpr string
	warehouseReceiveReadyExpr string
}

type taskListQuerySpec struct {
	fromSQL         string
	whereSQL        string
	args            []interface{}
	latestAssetExpr string
}

func buildTaskListQuerySpec(filter repo.TaskListFilter, candidateFilters []domain.TaskQueryFilterDefinition) (taskListQuerySpec, error) {
	exprs := taskListQueryExpressions{
		mainStatusExpr:            taskMainStatusExpr(),
		latestAssetExpr:           taskLatestAssetTypeExpr(),
		coordinationStatusExpr:    taskCoordinationStatusExpr(),
		warehouseReceiveReadyExpr: taskWarehouseReceiveReadyExpr(),
	}
	exprs.subStatusExprs = taskSubStatusExprs(exprs.latestAssetExpr)
	exprs.warehousePrepareReadyExpr = taskWarehousePrepareReadyExpr(exprs.latestAssetExpr)

	where := []string{"1=1"}
	args := []interface{}{}

	if err := appendTaskQueryDefinitionWhere(&where, &args, filter.TaskQueryFilterDefinition, exprs); err != nil {
		return taskListQuerySpec{}, err
	}
	if clause, clauseArgs, err := buildTaskBoardCandidateScopeClause(candidateFilters, exprs); err != nil {
		return taskListQuerySpec{}, err
	} else if clause != "" {
		where = append(where, clause)
		args = append(args, clauseArgs...)
	}

	if filter.CreatorID != nil {
		where = append(where, "t.creator_id = ?")
		args = append(args, *filter.CreatorID)
	}
	if filter.DesignerID != nil {
		where = append(where, "t.designer_id = ?")
		args = append(args, *filter.DesignerID)
	}
	if filter.NeedOutsource != nil {
		where = append(where, "t.need_outsource = ?")
		args = append(args, *filter.NeedOutsource)
	}
	if filter.Overdue != nil {
		overdueExpr := `(t.deadline_at IS NOT NULL AND t.deadline_at < ? AND t.task_status NOT IN (?, ?, ?))`
		if *filter.Overdue {
			where = append(where, overdueExpr)
			args = append(args, time.Now(), string(domain.TaskStatusCompleted), string(domain.TaskStatusArchived), string(domain.TaskStatusCancelled))
		} else {
			where = append(where, "NOT "+overdueExpr)
			args = append(args, time.Now(), string(domain.TaskStatusCompleted), string(domain.TaskStatusArchived), string(domain.TaskStatusCancelled))
		}
	}
	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		where = append(where, "(t.task_no LIKE ? OR t.sku_code LIKE ? OR t.product_name_snapshot LIKE ? OR t.owner_team LIKE ? OR COALESCE(t.owner_department, '') LIKE ? OR COALESCE(t.owner_org_team, '') LIKE ? OR CAST(t.id AS CHAR) = ?)")
		args = append(args, like, like, like, like, like, like, filter.Keyword)
	}
	appendTaskDataScopeWhere(&where, &args, filter)

	return taskListQuerySpec{
		fromSQL: `
		FROM tasks t
		LEFT JOIN task_details td ON td.task_id = t.id
		LEFT JOIN procurement_records pr ON pr.task_id = t.id
		LEFT JOIN warehouse_receipts wr ON wr.task_id = t.id
		` + taskLatestAssetJoinSQL(),
		whereSQL:        strings.Join(where, " AND "),
		args:            args,
		latestAssetExpr: exprs.latestAssetExpr,
	}, nil
}

func appendTaskDataScopeWhere(where *[]string, args *[]interface{}, filter repo.TaskListFilter) {
	if filter.ScopeViewAll {
		return
	}
	scopeClauses := make([]string, 0, 8)
	scopeArgs := make([]interface{}, 0, 32)

	if len(filter.ScopeUserIDs) > 0 {
		userIDs := int64ValuesToInterfaces(filter.ScopeUserIDs)
		if len(userIDs) > 0 {
			placeholders := strings.TrimRight(strings.Repeat("?,", len(userIDs)), ",")
			scopeClauses = append(scopeClauses,
				"(t.creator_id IN ("+placeholders+") OR t.designer_id IN ("+placeholders+") OR t.current_handler_id IN ("+placeholders+"))",
			)
			scopeArgs = append(scopeArgs, userIDs...)
			scopeArgs = append(scopeArgs, userIDs...)
			scopeArgs = append(scopeArgs, userIDs...)
		}
	}
	if len(filter.ScopeDepartmentCodes) > 0 {
		clause, clauseArgs := buildInClause("t.owner_department", stringsToSlice(filter.ScopeDepartmentCodes))
		if clause != "" {
			scopeClauses = append(scopeClauses, clause)
			scopeArgs = append(scopeArgs, clauseArgs...)
		}
	}
	if len(filter.ScopeTeamCodes) > 0 {
		clause, clauseArgs := buildInClause("t.owner_org_team", stringsToSlice(filter.ScopeTeamCodes))
		if clause != "" {
			scopeClauses = append(scopeClauses, clause)
			scopeArgs = append(scopeArgs, clauseArgs...)
		}
	}
	if len(filter.ScopeManagedDepartmentCodes) > 0 {
		clause, clauseArgs := buildManagedDepartmentScopeClause(stringsToSlice(filter.ScopeManagedDepartmentCodes))
		if clause != "" {
			scopeClauses = append(scopeClauses, clause)
			scopeArgs = append(scopeArgs, clauseArgs...)
		}
	}
	if len(filter.ScopeManagedTeamCodes) > 0 {
		clause, clauseArgs := buildManagedTeamScopeClause(stringsToSlice(filter.ScopeManagedTeamCodes))
		if clause != "" {
			scopeClauses = append(scopeClauses, clause)
			scopeArgs = append(scopeArgs, clauseArgs...)
		}
	}
	for _, visibility := range filter.ScopeStageVisibilities {
		statusClause, statusArgs := buildInClause("t.task_status", comparableValuesToStrings(visibility.Statuses))
		if statusClause == "" {
			continue
		}
		if visibility.Lane == nil {
			scopeClauses = append(scopeClauses, statusClause)
			scopeArgs = append(scopeArgs, statusArgs...)
			continue
		}
		laneClause := workflowLaneConditionSQL(*visibility.Lane)
		if laneClause == "" {
			scopeClauses = append(scopeClauses, statusClause)
			scopeArgs = append(scopeArgs, statusArgs...)
			continue
		}
		scopeClauses = append(scopeClauses, "("+statusClause+" AND "+laneClause+")")
		scopeArgs = append(scopeArgs, statusArgs...)
	}
	if len(scopeClauses) == 0 {
		*where = append(*where, "1=0")
		return
	}
	*where = append(*where, "("+strings.Join(scopeClauses, " OR ")+")")
	*args = append(*args, scopeArgs...)
}

func buildManagedDepartmentScopeClause(departments []string) (string, []interface{}) {
	return buildManagedUserScopeClause("department", "t.owner_department", departments)
}

func buildManagedTeamScopeClause(teams []string) (string, []interface{}) {
	return buildManagedUserScopeClause("team", "t.owner_org_team", teams)
}

func buildManagedUserScopeClause(userColumn, ownerExpr string, values []string) (string, []interface{}) {
	values = stringsToSlice(values)
	if len(values) == 0 {
		return "", nil
	}
	expressions := []string{
		ownerExpr,
		"(SELECT " + userColumn + " FROM users WHERE id = t.creator_id)",
		"(SELECT " + userColumn + " FROM users WHERE id = t.designer_id)",
		"(SELECT " + userColumn + " FROM users WHERE id = t.current_handler_id)",
	}
	clauses := make([]string, 0, len(expressions))
	args := make([]interface{}, 0, len(expressions)*len(values))
	for _, expr := range expressions {
		clause, clauseArgs := buildInClause(expr, values)
		if clause == "" {
			continue
		}
		clauses = append(clauses, clause)
		args = append(args, clauseArgs...)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

func workflowLaneConditionSQL(lane domain.WorkflowLane) string {
	switch lane {
	case domain.WorkflowLaneCustomization:
		return "t.customization_required = 1"
	case domain.WorkflowLaneNormal:
		return "t.customization_required = 0"
	default:
		return ""
	}
}

func int64ValuesToInterfaces(values []int64) []interface{} {
	out := make([]interface{}, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		out = append(out, value)
	}
	return out
}

func stringsToSlice(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func appendTaskQueryDefinitionWhere(where *[]string, args *[]interface{}, filter domain.TaskQueryFilterDefinition, exprs taskListQueryExpressions) error {
	appendInClause(where, args, "t.task_status", comparableValuesToStrings(filter.Statuses))
	appendInClause(where, args, "t.task_type", comparableValuesToStrings(filter.TaskTypes))
	appendInClause(where, args, "t.source_mode", comparableValuesToStrings(filter.SourceModes))
	appendWorkflowLaneClause(where, args, filter.WorkflowLanes)
	appendInClause(where, args, "t.owner_department", stringsToSlice(filter.OwnerDepartments))
	appendInClause(where, args, "t.owner_org_team", stringsToSlice(filter.OwnerOrgTeams))
	appendInClause(where, args, exprs.mainStatusExpr, comparableValuesToStrings(filter.MainStatuses))

	if len(filter.SubStatusCodes) > 0 {
		if filter.SubStatusScope != nil {
			scopeExpr, ok := exprs.subStatusExprs[*filter.SubStatusScope]
			if !ok {
				return fmt.Errorf("unsupported task sub-status scope: %s", *filter.SubStatusScope)
			}
			appendInClause(where, args, scopeExpr, comparableValuesToStrings(filter.SubStatusCodes))
		} else {
			codes := comparableValuesToStrings(filter.SubStatusCodes)
			scopeWhere := make([]string, 0, len(taskSubStatusScopeOrder()))
			scopeArgs := make([]interface{}, 0, len(taskSubStatusScopeOrder())*len(codes))
			for _, scope := range taskSubStatusScopeOrder() {
				scopeExpr, ok := exprs.subStatusExprs[scope]
				if !ok {
					continue
				}
				clause, clauseArgs := buildInClause(scopeExpr, codes)
				if clause == "" {
					continue
				}
				scopeWhere = append(scopeWhere, clause)
				scopeArgs = append(scopeArgs, clauseArgs...)
			}
			if len(scopeWhere) > 0 {
				*where = append(*where, "("+strings.Join(scopeWhere, " OR ")+")")
				*args = append(*args, scopeArgs...)
			}
		}
	}

	appendInClause(where, args, exprs.coordinationStatusExpr, comparableValuesToStrings(filter.CoordinationStatuses))
	if filter.WarehousePrepareReady != nil {
		*where = append(*where, exprs.warehousePrepareReadyExpr+" = ?")
		*args = append(*args, boolToInt(*filter.WarehousePrepareReady))
	}
	if filter.WarehouseReceiveReady != nil {
		*where = append(*where, exprs.warehouseReceiveReadyExpr+" = ?")
		*args = append(*args, boolToInt(*filter.WarehouseReceiveReady))
	}
	if len(filter.WarehouseBlockingReasonCodes) > 0 {
		conditions := make([]string, 0, len(filter.WarehouseBlockingReasonCodes))
		for _, code := range filter.WarehouseBlockingReasonCodes {
			condition := taskWarehouseBlockingReasonCondition(code, exprs.latestAssetExpr)
			if condition == "" {
				return fmt.Errorf("unsupported workflow reason code: %s", code)
			}
			conditions = append(conditions, "("+condition+")")
		}
		if len(conditions) > 0 {
			*where = append(*where, "("+strings.Join(conditions, " OR ")+")")
		}
	}
	return nil
}

func appendWorkflowLaneClause(where *[]string, args *[]interface{}, lanes []domain.WorkflowLane) {
	if len(lanes) == 0 {
		return
	}
	seen := map[domain.WorkflowLane]struct{}{}
	conditions := make([]string, 0, len(lanes))
	for _, lane := range lanes {
		if _, exists := seen[lane]; exists {
			continue
		}
		seen[lane] = struct{}{}
		switch lane {
		case domain.WorkflowLaneCustomization:
			conditions = append(conditions, "t.customization_required = 1")
		case domain.WorkflowLaneNormal:
			conditions = append(conditions, "t.customization_required = 0")
		}
	}
	if len(conditions) == 0 {
		return
	}
	*where = append(*where, "("+strings.Join(conditions, " OR ")+")")
}

func buildTaskBoardCandidateScopeClause(candidateFilters []domain.TaskQueryFilterDefinition, exprs taskListQueryExpressions) (string, []interface{}, error) {
	if len(candidateFilters) == 0 {
		return "", nil, nil
	}

	scopeClauses := make([]string, 0, len(candidateFilters))
	scopeArgs := make([]interface{}, 0)
	for _, candidate := range candidateFilters {
		candidateWhere := make([]string, 0, 8)
		candidateArgs := make([]interface{}, 0, 16)
		if err := appendTaskQueryDefinitionWhere(&candidateWhere, &candidateArgs, candidate, exprs); err != nil {
			return "", nil, err
		}
		if len(candidateWhere) == 0 {
			scopeClauses = append(scopeClauses, "1=1")
		} else {
			scopeClauses = append(scopeClauses, "("+strings.Join(candidateWhere, " AND ")+")")
		}
		scopeArgs = append(scopeArgs, candidateArgs...)
	}
	return "(" + strings.Join(scopeClauses, " OR ") + ")", scopeArgs, nil
}

func taskLatestAssetTypeExpr() string {
	return `CASE
		WHEN la.asset_type = 'original' THEN 'source'
		WHEN la.asset_type IN ('draft', 'revised', 'final', 'outsource_return') THEN 'delivery'
		ELSE la.asset_type
	END`
}

func taskLatestAssetJoinSQL() string {
	return `LEFT JOIN (
			SELECT ta.task_id, ta.asset_type
			FROM task_assets ta
			INNER JOIN (
				SELECT task_id, MAX(version_no) AS max_version_no
				FROM task_assets
				GROUP BY task_id
			) latest_task_asset
				ON latest_task_asset.task_id = ta.task_id
				AND latest_task_asset.max_version_no = ta.version_no
		) la ON la.task_id = t.id`
}

func appendInClause(where *[]string, args *[]interface{}, expr string, values []string) {
	clause, clauseArgs := buildInClause(expr, values)
	if clause == "" {
		return
	}
	*where = append(*where, clause)
	*args = append(*args, clauseArgs...)
}

func buildInClause(expr string, values []string) (string, []interface{}) {
	if len(values) == 0 {
		return "", nil
	}
	placeholders := make([]string, 0, len(values))
	args := make([]interface{}, 0, len(values))
	for _, value := range values {
		placeholders = append(placeholders, "?")
		args = append(args, value)
	}
	return expr + " IN (" + strings.Join(placeholders, ", ") + ")", args
}

func comparableValuesToStrings[T ~string](values []T) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	return out
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func taskSubStatusScopeOrder() []domain.TaskSubStatusScope {
	return []domain.TaskSubStatusScope{
		domain.TaskSubStatusScopeDesign,
		domain.TaskSubStatusScopeAudit,
		domain.TaskSubStatusScopeProcurement,
		domain.TaskSubStatusScopeWarehouse,
		domain.TaskSubStatusScopeCustomization,
		domain.TaskSubStatusScopeProduction,
	}
}

func taskMainStatusExpr() string {
	return `CASE
		WHEN t.task_status = 'Completed' THEN 'closed'
		WHEN t.task_status = 'PendingClose' THEN 'pending_close'
		WHEN wr.status = 'completed' THEN 'pending_close'
		WHEN wr.status = 'received' THEN 'warehouse_processing'
		WHEN t.task_status = 'PendingWarehouseReceive' THEN 'pending_warehouse_receive'
		WHEN td.filing_status = 'filed' OR (COALESCE(td.filing_status, '') = '' AND td.filed_at IS NOT NULL) THEN 'filed'
		ELSE 'created'
	END`
}

func taskCoordinationStatusExpr() string {
	return `CASE
		WHEN t.task_type <> 'purchase_task' THEN NULL
		WHEN pr.task_id IS NULL THEN 'preparing'
		WHEN t.task_status IN ('PendingClose', 'Completed') THEN 'warehouse_completed'
		WHEN wr.status = 'completed' THEN 'warehouse_completed'
		WHEN t.task_status = 'PendingWarehouseReceive' THEN 'handed_to_warehouse'
		WHEN wr.task_id IS NOT NULL THEN 'handed_to_warehouse'
		WHEN pr.status = 'completed' THEN 'ready_for_warehouse'
		WHEN pr.status = 'in_progress' THEN 'awaiting_arrival'
		ELSE 'preparing'
	END`
}

func taskWarehousePrepareReadyExpr(latestAssetExpr string) string {
	return fmt.Sprintf(`CASE
		WHEN %s THEN 0
		ELSE 1
	END`, taskWarehouseBlockingAnyCondition(latestAssetExpr))
}

func taskWarehouseReceiveReadyExpr() string {
	return `CASE
		WHEN t.task_status = 'PendingWarehouseReceive' AND wr.task_id IS NULL THEN 1
		ELSE 0
	END`
}

func taskWarehouseBlockingAnyCondition(latestAssetExpr string) string {
	conditions := []string{
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonTaskDetailMissing, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonFiledAtMissing, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonCategoryMissing, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonSpecMissing, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonCostPriceMissing, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonProcurementMissing, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonProcurementPriceMissing, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonProcurementQuantityMissing, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonProcurementNotReady, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonTaskAlreadyPendingWH, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonTaskAwaitingClose, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonTaskAlreadyClosed, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonTaskBlocked, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonWarehouseAlreadyReceived, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonWarehouseAlreadyDone, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonMissingFinalAsset, latestAssetExpr),
		taskWarehouseBlockingReasonCondition(domain.WorkflowReasonAuditNotApproved, latestAssetExpr),
	}
	return strings.Join(conditions, " OR ")
}

func taskWarehouseBlockingReasonCondition(code domain.WorkflowReasonCode, latestAssetExpr string) string {
	detailMissing := "td.task_id IS NULL"
	requiresBusinessInfo := "td.task_id IS NOT NULL"
	purchaseTask := "t.task_type = 'purchase_task'"
	requiresDesign := "t.task_type IN ('original_product_development', 'new_product_development')"

	switch code {
	case domain.WorkflowReasonTaskDetailMissing:
		return detailMissing
	case domain.WorkflowReasonFiledAtMissing:
		return requiresBusinessInfo + " AND COALESCE(td.filing_status, '') <> 'filed' AND NOT (COALESCE(td.filing_status, '') = '' AND td.filed_at IS NOT NULL)"
	case domain.WorkflowReasonCategoryMissing:
		return requiresBusinessInfo + " AND TRIM(COALESCE(td.category, '')) = ''"
	case domain.WorkflowReasonSpecMissing:
		return requiresBusinessInfo + " AND TRIM(COALESCE(td.spec_text, '')) = ''"
	case domain.WorkflowReasonCostPriceMissing:
		return requiresBusinessInfo + " AND td.cost_price IS NULL"
	case domain.WorkflowReasonProcurementMissing:
		return purchaseTask + " AND pr.task_id IS NULL"
	case domain.WorkflowReasonProcurementPriceMissing:
		return purchaseTask + " AND pr.task_id IS NOT NULL AND pr.procurement_price IS NULL"
	case domain.WorkflowReasonProcurementQuantityMissing:
		return purchaseTask + " AND pr.task_id IS NOT NULL AND (pr.quantity IS NULL OR pr.quantity <= 0)"
	case domain.WorkflowReasonProcurementNotReady:
		return purchaseTask + " AND pr.task_id IS NOT NULL AND pr.status <> 'completed'"
	case domain.WorkflowReasonTaskAlreadyPendingWH:
		return "t.task_status = 'PendingWarehouseReceive'"
	case domain.WorkflowReasonTaskAwaitingClose:
		return "t.task_status = 'PendingClose'"
	case domain.WorkflowReasonTaskAlreadyClosed:
		return "t.task_status = 'Completed'"
	case domain.WorkflowReasonTaskBlocked:
		return "t.task_status = 'Blocked'"
	case domain.WorkflowReasonWarehouseAlreadyReceived:
		return "wr.status = 'received'"
	case domain.WorkflowReasonWarehouseAlreadyDone:
		return "wr.status = 'completed'"
	case domain.WorkflowReasonMissingFinalAsset:
		return fmt.Sprintf("%s AND COALESCE(%s, '') <> 'delivery'", requiresDesign, latestAssetExpr)
	case domain.WorkflowReasonAuditNotApproved:
		return requiresDesign + " AND t.task_status NOT IN ('PendingWarehouseReceive', 'PendingClose', 'Completed')"
	default:
		return ""
	}
}

func taskSubStatusExprs(latestAssetExpr string) map[domain.TaskSubStatusScope]string {
	customizationExpr := `CASE
			WHEN t.customization_required = 0
			     AND t.need_outsource = 0
			     AND t.task_status NOT IN ('PendingOutsource', 'Outsourcing', 'PendingOutsourceReview') THEN 'not_triggered'
			WHEN t.task_status = 'PendingCustomizationReview' THEN 'pending_review'
			WHEN t.task_status IN ('PendingCustomizationProduction', 'PendingEffectRevision') THEN 'in_progress'
			WHEN t.task_status = 'PendingEffectReview' THEN 'pending_review'
			WHEN t.task_status = 'PendingProductionTransfer' THEN 'ready'
			WHEN t.task_status = 'PendingWarehouseQC' THEN 'pending_receive'
			WHEN t.task_status = 'RejectedByWarehouse' THEN 'rejected'
			WHEN t.task_status IN ('PendingOutsource', 'Outsourcing') THEN 'in_progress'
			WHEN t.task_status = 'PendingOutsourceReview' THEN 'pending_review'
			WHEN t.task_status IN ('PendingWarehouseReceive', 'PendingClose', 'Completed') THEN 'completed'
			ELSE 'not_triggered'
		END`
	return map[domain.TaskSubStatusScope]string{
		domain.TaskSubStatusScopeDesign: fmt.Sprintf(`CASE
			WHEN t.task_type = 'purchase_task' THEN 'not_required'
			WHEN t.task_status IN (
				'PendingCustomizationReview',
				'PendingCustomizationProduction',
				'PendingEffectReview',
				'PendingEffectRevision',
				'PendingProductionTransfer',
				'PendingWarehouseQC',
				'RejectedByWarehouse'
			) THEN 'not_required'
			WHEN t.task_status = 'PendingAssign' THEN 'pending_design'
			WHEN t.task_status = 'InProgress' THEN 'designing'
			WHEN t.task_status IN ('RejectedByAuditA', 'RejectedByAuditB', 'Blocked') THEN 'rework_required'
			WHEN t.task_status IN ('PendingAuditA', 'PendingAuditB', 'PendingOutsourceReview') THEN 'pending_audit'
			WHEN t.task_status IN ('PendingOutsource', 'Outsourcing') THEN 'outsourcing'
			WHEN t.task_status IN ('PendingWarehouseReceive', 'PendingClose', 'Completed') AND %s = 'delivery' THEN 'final_ready'
			WHEN %s = 'delivery' THEN 'final_ready'
			ELSE 'pending_design'
		END`, latestAssetExpr, latestAssetExpr),
		domain.TaskSubStatusScopeAudit: `CASE
			WHEN t.task_type = 'purchase_task' THEN 'not_triggered'
			WHEN t.task_status IN ('PendingAuditA', 'PendingAuditB', 'PendingOutsourceReview') THEN 'in_review'
			WHEN t.task_status IN ('RejectedByAuditA', 'RejectedByAuditB', 'Blocked') THEN 'rejected'
			WHEN t.task_status IN ('PendingOutsource', 'Outsourcing') THEN 'outsourced'
			WHEN t.task_status IN ('PendingWarehouseReceive', 'PendingClose', 'Completed') THEN 'approved'
			ELSE 'not_triggered'
		END`,
		domain.TaskSubStatusScopeProcurement: `CASE
			WHEN t.task_type <> 'purchase_task' THEN 'not_triggered'
			WHEN t.task_status IN ('PendingClose', 'Completed') THEN 'completed'
			WHEN wr.status = 'completed' THEN 'completed'
			WHEN pr.status IS NULL THEN 'not_started'
			WHEN pr.status = 'completed' THEN 'ready'
			WHEN pr.status = 'in_progress' THEN 'pending_inbound'
			WHEN pr.status = 'prepared' THEN 'preparing'
			ELSE 'preparing'
		END`,
		domain.TaskSubStatusScopeWarehouse: `CASE
			WHEN wr.status = 'completed' THEN 'completed'
			WHEN wr.status = 'received' THEN 'received'
			WHEN wr.status = 'rejected' THEN 'rejected'
			WHEN t.task_status = 'PendingWarehouseReceive' THEN 'pending_receive'
			ELSE 'not_triggered'
		END`,
		domain.TaskSubStatusScopeCustomization: customizationExpr,
		domain.TaskSubStatusScopeOutsource:     customizationExpr,
		domain.TaskSubStatusScopeProduction:    `'reserved'`,
	}
}
