package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	"workflow/domain"
)

func (r *taskRepo) GetTaskDetailBundle(ctx context.Context, taskID int64, eventLimit int) (*domain.Task, *domain.TaskDetail, []*domain.TaskModule, []*domain.TaskModuleEvent, []*domain.ReferenceFileRefFlat, error) {
	bundle, err := r.GetTaskDetailReadBundle(ctx, taskID, eventLimit)
	if err != nil || bundle == nil {
		return nil, nil, nil, nil, nil, err
	}
	return bundle.Task, bundle.TaskDetail, bundle.Modules, bundle.Events, bundle.ReferenceFiles, nil
}

func (r *taskRepo) GetTaskDetailReadBundle(ctx context.Context, taskID int64, eventLimit int) (*domain.TaskDetailReadBundle, error) {
	if eventLimit <= 0 || eventLimit > 200 {
		eventLimit = 50
	}
	query := fmt.Sprintf(`
		SELECT id, task_no, source_mode, product_id, sku_code, product_name_snapshot,
		       task_type, operator_group_id, owner_team, owner_department, owner_org_team, owner_department_id, owner_team_id,
		       creator_id, requester_id, designer_id, current_handler_id,
		       task_status, workflow_revision, priority, deadline_at, need_outsource, is_outsource, COALESCE(business_lane, ''), customization_required, customization_source_type,
		       last_customization_operator_id, warehouse_reject_reason, warehouse_reject_category,
		       is_batch_task, batch_item_count, batch_mode, primary_sku_code, sku_generation_status,
		       created_at, updated_at
		FROM tasks WHERE id = %[1]d;

		SELECT id, task_id, demand_text, copy_text, style_keywords, remark, COALESCE(note, ''), risk_flags_json,
		       category, category_id, category_code, category_name,
		       source_product_id, source_product_name, source_search_entry_code, source_match_type, source_match_rule,
		       matched_category_code, matched_search_entry_code, matched_mapping_rule_json, product_selection_snapshot_json,
		       change_request, design_requirement, COALESCE(set_mode_hint, 0), product_short_name, material_mode, material_other,
		       cost_price_mode, base_sale_price, product_channel, reference_images_json, COALESCE(reference_file_refs_json, ''), reference_link,
		       spec_text, material, size_text, craft_text,
		       width, height, area, quantity, process,
		       procurement_price, cost_price, estimated_cost, cost_rule_id, cost_rule_name, cost_rule_source,
		       matched_rule_version, prefill_source, prefill_at,
		       requires_manual_review, manual_cost_override, manual_cost_override_reason, override_actor, override_at,
		       filing_status, COALESCE(filing_error_message, ''), COALESCE(filing_trigger_source, ''),
		       last_filing_attempt_at, last_filed_at, COALESCE(erp_sync_required, 0), COALESCE(erp_sync_version, 0),
		       COALESCE(last_filing_payload_hash, ''), COALESCE(last_filing_payload_json, ''), filed_at, created_at, updated_at
		FROM task_details WHERE task_id = %[1]d;

		SELECT id, task_id, module_key, state, pool_team_code, claimed_by, claimed_team_code,
		       claimed_at, actor_org_snapshot, entered_at, terminal_at, data, updated_at
		FROM task_modules
		WHERE task_id = %[1]d
		  AND COALESCE(JSON_EXTRACT(data, '$.backfill_placeholder'), CAST('false' AS JSON)) != CAST('true' AS JSON)
		ORDER BY FIELD(module_key, 'basic_info', 'customization', 'design', 'retouch', 'audit'), id;

		SELECT task_module_events.id, task_module_events.task_module_id, task_module_events.event_type,
		       task_module_events.from_state, task_module_events.to_state, task_module_events.actor_id,
		       task_module_events.actor_snapshot, task_module_events.payload, task_module_events.created_at
		FROM task_module_events
		JOIN task_modules tm ON tm.id = task_module_events.task_module_id
		WHERE tm.task_id = %[1]d
		ORDER BY task_module_events.created_at DESC, task_module_events.id DESC
		LIMIT %[2]d;

		SELECT rfr.id, rfr.task_id, rfr.sku_item_id,
		       rfr.retouch_requirement_id, rfr.ref_id,
		       rfr.owner_module_key, rfr.context, rfr.attached_at,
		       COALESCE(asr.ref_key, ''), COALESCE(asr.file_name, ''),
		       COALESCE(asr.mime_type, ''), asr.file_size,
		       COALESCE(asr.status, '')
		FROM reference_file_refs rfr
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = rfr.ref_id
		WHERE rfr.task_id = %[1]d
		ORDER BY rfr.owner_module_key, rfr.attached_at ASC, rfr.id ASC;

		SELECT id, task_id, sequence_no, sku_code, sku_status, product_id, erp_product_id,
		       filing_status, erp_sync_status, erp_sync_required, erp_sync_version, last_filed_at, COALESCE(filing_error_message, ''),
		       product_name_snapshot, product_short_name, category_code, material_mode,
		       cost_price_mode, quantity, base_sale_price, cost_price, estimated_cost, cost_rule_id, cost_rule_name,
		       cost_rule_source, matched_rule_version, prefill_source, prefill_at, requires_manual_review,
		       manual_cost_override, manual_cost_override_reason, override_actor, override_at,
		       design_requirement, COALESCE(set_mode_hint, 0), variant_json, COALESCE(reference_file_refs_json, ''),
		       dedupe_key, COALESCE(sku_code_type, ''), created_at, updated_at
		FROM task_sku_items
		WHERE task_id = %[1]d
		ORDER BY sequence_no ASC, id ASC;

		SELECT %[3]s
		FROM task_assets ta
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		WHERE ta.task_id = %[1]d
		ORDER BY ta.version_no ASC, ta.created_at ASC;

		SELECT id, task_id, description, sku_code, spec, remark, sort_order,
		       created_by, updated_by, created_at, updated_at
		FROM task_retouch_requirements
		WHERE task_id = %[1]d AND deleted_at IS NULL
		ORDER BY sort_order ASC, id ASC;

		SELECT u.id, COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username, ''), '')
		FROM users u
		JOIN tasks t ON t.id = %[1]d
		WHERE u.id IN (t.creator_id, t.requester_id, t.designer_id, t.current_handler_id)
		ORDER BY u.id`, taskID, eventLimit, taskAssetSelectCols)

	rows, err := r.db.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get task detail bundle: %w", err)
	}
	defer rows.Close()

	task, err := scanSingleTaskResult(rows)
	if err != nil || task == nil {
		return nil, err
	}
	bundle := &domain.TaskDetailReadBundle{Task: task, UserNames: map[int64]string{}}
	if !rows.NextResultSet() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return bundle, nil
	}
	detail, err := scanSingleTaskDetailResult(rows)
	if err != nil {
		return nil, err
	}
	bundle.TaskDetail = detail
	if !rows.NextResultSet() {
		return bundle, rows.Err()
	}
	modules, err := scanTaskModules(rows)
	if err != nil {
		return nil, err
	}
	bundle.Modules = modules
	if !rows.NextResultSet() {
		return bundle, rows.Err()
	}
	events, err := scanTaskModuleEvents(rows)
	if err != nil {
		return nil, err
	}
	bundle.Events = events
	if !rows.NextResultSet() {
		return bundle, rows.Err()
	}
	refs, err := scanReferenceFileRefFlatRows(rows)
	if err != nil {
		return nil, err
	}
	bundle.ReferenceFiles = refs
	if !rows.NextResultSet() {
		return bundle, rows.Err()
	}
	bundle.SKUItems, err = scanTaskSKUItemRows(rows)
	if err != nil {
		return nil, err
	}
	if !rows.NextResultSet() {
		return bundle, rows.Err()
	}
	bundle.TaskAssets, err = scanTaskAssetRows(rows)
	if err != nil {
		return nil, err
	}
	if !rows.NextResultSet() {
		return bundle, rows.Err()
	}
	bundle.RetouchRequirements, err = scanTaskRetouchRequirementRows(rows)
	if err != nil {
		return nil, err
	}
	if !rows.NextResultSet() {
		return bundle, rows.Err()
	}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		bundle.UserNames[id] = name
	}
	return bundle, rows.Err()
}

func scanTaskSKUItemRows(rows *sql.Rows) ([]*domain.TaskSKUItem, error) {
	items := make([]*domain.TaskSKUItem, 0)
	for rows.Next() {
		item, err := scanTaskSKUItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanTaskAssetRows(rows *sql.Rows) ([]*domain.TaskAsset, error) {
	items := make([]*domain.TaskAsset, 0)
	for rows.Next() {
		item, err := scanTaskAssetRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanTaskRetouchRequirementRows(rows *sql.Rows) ([]*domain.TaskRetouchRequirement, error) {
	items := make([]*domain.TaskRetouchRequirement, 0)
	for rows.Next() {
		var item domain.TaskRetouchRequirement
		var skuCode, spec, remark sql.NullString
		var createdBy, updatedBy sql.NullInt64
		if err := rows.Scan(&item.ID, &item.TaskID, &item.Description, &skuCode, &spec, &remark,
			&item.SortOrder, &createdBy, &updatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.SKUCode = skuCode.String
		item.Spec = spec.String
		item.Remark = remark.String
		item.CreatedBy = fromNullInt64(createdBy)
		item.UpdatedBy = fromNullInt64(updatedBy)
		items = append(items, &item)
	}
	return items, rows.Err()
}

func scanSingleTaskResult(rows *sql.Rows) (*domain.Task, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	task, err := scanTaskRow(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return task, nil
}

func scanSingleTaskDetailResult(rows *sql.Rows) (*domain.TaskDetail, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	detail, err := scanTaskDetailRow(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return detail, nil
}

func scanTaskDetailRow(scanner interface{ Scan(...interface{}) error }) (*domain.TaskDetail, error) {
	var detail domain.TaskDetail
	var categoryID, sourceProductID, costRuleID, matchedRuleVersion sql.NullInt64
	var quantity sql.NullInt64
	var procurementPrice, costPrice, width, height, area, estimatedCost, baseSalePrice sql.NullFloat64
	var prefillAt, overrideAt, filedAt, lastFilingAttemptAt, lastFiledAt sql.NullTime
	var erpSyncRequired sql.NullBool
	var erpSyncVersion sql.NullInt64
	err := scanner.Scan(
		&detail.ID, &detail.TaskID, &detail.DemandText, &detail.CopyText, &detail.StyleKeywords,
		&detail.Remark, &detail.Note, &detail.RiskFlagsJSON, &detail.Category, &categoryID, &detail.CategoryCode, &detail.CategoryName,
		&sourceProductID, &detail.SourceProductName, &detail.SourceSearchEntryCode, &detail.SourceMatchType, &detail.SourceMatchRule,
		&detail.MatchedCategoryCode, &detail.MatchedSearchEntryCode, &detail.MatchedMappingRuleJSON, &detail.ProductSelectionSnapshotJSON,
		&detail.ChangeRequest, &detail.DesignRequirement, &detail.SetModeHint, &detail.ProductShortName, &detail.MaterialMode, &detail.MaterialOther,
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

func scanReferenceFileRefFlatRows(rows *sql.Rows) ([]*domain.ReferenceFileRefFlat, error) {
	var out []*domain.ReferenceFileRefFlat
	for rows.Next() {
		var ref domain.ReferenceFileRefFlat
		var skuID, retouchRequirementID sql.NullInt64
		var contextValue sql.NullString
		var fileSize sql.NullInt64
		if err := rows.Scan(
			&ref.ID, &ref.TaskID, &skuID, &retouchRequirementID,
			&ref.RefID, &ref.OwnerModuleKey, &contextValue,
			&ref.AttachedAt, &ref.StorageKey, &ref.FileName,
			&ref.MimeType, &fileSize, &ref.StorageStatus,
		); err != nil {
			return nil, fmt.Errorf("scan reference_file_ref flat: %w", err)
		}
		ref.SKUItemID = fromNullInt64(skuID)
		ref.RetouchRequirementID = fromNullInt64(retouchRequirementID)
		ref.Context = fromNullString(contextValue)
		ref.FileSize = fromNullInt64(fileSize)
		out = append(out, &ref)
	}
	return out, rows.Err()
}
