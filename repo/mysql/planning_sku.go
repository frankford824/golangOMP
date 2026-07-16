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

type PlanningSKURepo struct{ db *DB }

func NewPlanningSKURepo(db *DB) *PlanningSKURepo { return &PlanningSKURepo{db: db} }

func (r *PlanningSKURepo) GetUniqueActiveRuleForUpdate(ctx context.Context, tx repo.Tx, ruleType domain.CodeRuleType) (*domain.CodeRuleRevision, error) {
	rows, err := Unwrap(tx).QueryContext(ctx, `
		SELECT rev.id, rev.rule_id, rev.version_no, rev.prefix, rev.date_format,
		       rev.site_code, rev.biz_code, rev.separator_text, rev.seq_length,
		       rev.reset_cycle, rev.dimension_mode, rev.created_at
		FROM code_rules rule_row
		JOIN code_rule_revisions rev ON rev.id = rule_row.active_revision_id AND rev.rule_id = rule_row.id
		WHERE rule_row.rule_type = ? AND rule_row.is_enabled = 1
		ORDER BY rule_row.id FOR UPDATE`, string(ruleType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.CodeRuleRevision
	for rows.Next() {
		var item domain.CodeRuleRevision
		if err := rows.Scan(&item.ID, &item.RuleID, &item.VersionNo, &item.Prefix, &item.DateFormat,
			&item.SiteCode, &item.BizCode, &item.Separator, &item.SequenceLength,
			&item.ResetCycle, &item.DimensionMode, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, repo.ErrNotFound
	}
	if len(out) != 1 {
		return nil, repo.ErrConflict
	}
	return out[0], nil
}

func (r *PlanningSKURepo) AllocateRuleRange(ctx context.Context, tx repo.Tx, revisionID int64, dimensionKey, periodKey string, count int) (int64, error) {
	if count <= 0 {
		return 0, fmt.Errorf("allocation count must be positive")
	}
	sqlTx := Unwrap(tx)
	dimensionKey = strings.TrimSpace(dimensionKey)
	periodKey = strings.TrimSpace(periodKey)
	if _, err := sqlTx.ExecContext(ctx, `
		INSERT INTO code_rule_revision_sequences (rule_revision_id, dimension_key, period_key, next_value)
		VALUES (?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE rule_revision_id = VALUES(rule_revision_id)`, revisionID, dimensionKey, periodKey); err != nil {
		return 0, err
	}
	var start int64
	if err := sqlTx.QueryRowContext(ctx, `
		SELECT next_value FROM code_rule_revision_sequences
		WHERE rule_revision_id = ? AND dimension_key = ? AND period_key = ? FOR UPDATE`,
		revisionID, dimensionKey, periodKey).Scan(&start); err != nil {
		return 0, err
	}
	if start <= 0 {
		start = 1
	}
	if _, err := sqlTx.ExecContext(ctx, `
		UPDATE code_rule_revision_sequences SET next_value = ?
		WHERE rule_revision_id = ? AND dimension_key = ? AND period_key = ?`,
		start+int64(count), revisionID, dimensionKey, periodKey); err != nil {
		return 0, err
	}
	return start, nil
}

func (r *PlanningSKURepo) FindCreateResult(ctx context.Context, actorID int64, clientCreateID string) (*domain.PlanningSKUCreateResult, error) {
	var taskID int64
	err := r.db.db.QueryRowContext(ctx, `
		SELECT task_id FROM task_planning_settings
		WHERE created_by = ? AND client_create_id = ?`, actorID, strings.TrimSpace(clientCreateID)).Scan(&taskID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.LoadCreateResult(ctx, taskID)
}

func (r *PlanningSKURepo) GetTaskAccessSubject(ctx context.Context, taskID int64) (domain.TaskAccessSubject, error) {
	var item domain.TaskAccessSubject
	var requesterID, designerID, handlerID, departmentID, teamID sql.NullInt64
	err := r.db.db.QueryRowContext(ctx, `
		SELECT id, creator_id, requester_id, designer_id, current_handler_id, owner_department_id, owner_team_id
		FROM tasks WHERE id = ? AND task_type = 'sku_planning'`, taskID).
		Scan(&item.TaskID, &item.CreatorID, &requesterID, &designerID, &handlerID, &departmentID, &teamID)
	if err == sql.ErrNoRows {
		return item, repo.ErrNotFound
	}
	item.RequesterID = fromNullInt64(requesterID)
	item.DesignerID = fromNullInt64(designerID)
	item.CurrentHandlerID = fromNullInt64(handlerID)
	item.OwnerDepartmentID = fromNullInt64(departmentID)
	item.OwnerTeamID = fromNullInt64(teamID)
	return item, err
}

func (r *PlanningSKURepo) CreateSettings(ctx context.Context, tx repo.Tx, settings domain.PlanningSKUSettings) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO task_planning_settings
		  (task_id, erp_sync_mode, code_rule_revision_id, client_create_id, created_by)
		VALUES (?, ?, ?, ?, ?)`, settings.TaskID, string(settings.ERPSyncMode), settings.CodeRuleRevisionID,
		strings.TrimSpace(settings.ClientCreateID), settings.CreatedBy)
	return err
}

func (r *PlanningSKURepo) ValidatePlanningImage(ctx context.Context, tx repo.Tx, refID, clientCreateID, clientItemID string, actorID int64) (bool, error) {
	if strings.TrimSpace(refID) == "" {
		return true, nil
	}
	var count int
	err := Unwrap(tx).QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM asset_storage_refs asr
		JOIN upload_requests ur ON ur.request_id = asr.upload_request_id
		WHERE asr.ref_id = ? AND asr.owner_type = 'planning_sku_create' AND asr.owner_id = ?
		  AND asr.status = 'recorded' AND ur.client_create_id = ? AND ur.client_item_id = ?`,
		strings.TrimSpace(refID), actorID, strings.TrimSpace(clientCreateID), strings.TrimSpace(clientItemID)).Scan(&count)
	return count == 1, err
}

func (r *PlanningSKURepo) CreateRevision(ctx context.Context, tx repo.Tx, taskSKUItemID int64, input domain.PlanningSKUItemInput, version int, actorID int64, reason string) (*domain.PlanningSKURevision, error) {
	sqlTx := Unwrap(tx)
	result, err := sqlTx.ExecContext(ctx, `
		INSERT INTO task_planning_sku_revisions
		  (task_sku_item_id, version_no, description_spec, quantity, target_price, currency,
		   note, reference_url, erp_product_i_id, erp_product_name, reason, created_by)
		VALUES (?, ?, ?, ?, ?, 'CNY', ?, ?, ?, ?, ?, ?)`,
		taskSKUItemID, version, strings.TrimSpace(input.DescriptionSpec), input.Quantity,
		nullPlanningPrice(input.TargetPrice), strings.TrimSpace(input.Note), strings.TrimSpace(input.ReferenceURL),
		strings.TrimSpace(input.ERPProductIID), strings.TrimSpace(input.ERPProductName), strings.TrimSpace(reason), actorID)
	if err != nil {
		return nil, err
	}
	revisionID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	if version == 1 {
		if _, err := sqlTx.ExecContext(ctx, `
			INSERT INTO task_planning_sku_details (task_sku_item_id, current_revision_id, lock_version)
			VALUES (?, ?, 0)`, taskSKUItemID, revisionID); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(input.ImageUploadRef) != "" {
		if _, err := sqlTx.ExecContext(ctx, `
			INSERT INTO task_planning_sku_revision_images (revision_id, storage_ref_id) VALUES (?, ?)`, revisionID, strings.TrimSpace(input.ImageUploadRef)); err != nil {
			return nil, err
		}
		if _, err := sqlTx.ExecContext(ctx, `
			UPDATE asset_storage_refs SET owner_type = 'planning_sku_revision_image', owner_id = ?
			WHERE ref_id = ? AND owner_type = 'planning_sku_create' AND owner_id = ?`, revisionID, strings.TrimSpace(input.ImageUploadRef), actorID); err != nil {
			return nil, err
		}
	}
	return &domain.PlanningSKURevision{
		ID: revisionID, TaskSKUItemID: taskSKUItemID, VersionNo: version,
		DescriptionSpec: strings.TrimSpace(input.DescriptionSpec), Quantity: input.Quantity,
		TargetPrice: input.TargetPrice, Currency: "CNY", Note: strings.TrimSpace(input.Note),
		ReferenceURL: strings.TrimSpace(input.ReferenceURL), ERPProductIID: strings.TrimSpace(input.ERPProductIID),
		ERPProductName: strings.TrimSpace(input.ERPProductName), ProductImageRefID: strings.TrimSpace(input.ImageUploadRef),
		Reason: strings.TrimSpace(reason), CreatedBy: actorID,
	}, nil
}

func (r *PlanningSKURepo) EnqueueERP(ctx context.Context, tx repo.Tx, taskID, taskSKUItemID int64, jobType string, generation int, payload interface{}) error {
	raw, err := jsonMarshal(payload)
	if err != nil {
		return err
	}
	dedupe := fmt.Sprintf("%s:%d:%d:%d", jobType, taskID, taskSKUItemID, generation)
	_, err = Unwrap(tx).ExecContext(ctx, `
		INSERT INTO task_erp_outbox
		  (task_id, task_sku_item_id, job_type, generation, dedupe_key, payload_json)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE dedupe_key = dedupe_key`, taskID, taskSKUItemID, jobType, generation, dedupe, raw)
	return err
}

func (r *PlanningSKURepo) LoadCreateResult(ctx context.Context, taskID int64) (*domain.PlanningSKUCreateResult, error) {
	var out domain.PlanningSKUCreateResult
	if err := r.db.db.QueryRowContext(ctx, `SELECT id, task_no, task_status, workflow_revision FROM tasks WHERE id = ? AND task_type = 'sku_planning'`, taskID).
		Scan(&out.TaskID, &out.TaskNo, &out.TaskStatus, &out.WorkflowRevision); err != nil {
		if err == sql.ErrNoRows {
			return nil, repo.ErrNotFound
		}
		return nil, err
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT tsi.id, tsi.sequence_no, tsi.sku_code, tsi.quantity,
		       rev.id, rev.version_no, rev.description_spec, rev.quantity, rev.target_price,
		       rev.currency, rev.note, rev.reference_url, rev.erp_product_i_id, rev.erp_product_name,
		       COALESCE(img.storage_ref_id, ''), rev.reason, rev.created_by, rev.created_at,
		       COALESCE(outbox.status, '')
		FROM task_sku_items tsi
		JOIN task_planning_sku_details detail ON detail.task_sku_item_id = tsi.id
		JOIN task_planning_sku_revisions rev ON rev.id = detail.current_revision_id
		LEFT JOIN task_planning_sku_revision_images img ON img.revision_id = rev.id
		LEFT JOIN task_erp_outbox outbox ON outbox.id = (
		  SELECT latest.id FROM task_erp_outbox latest
		  WHERE latest.task_sku_item_id = tsi.id AND latest.job_type IN ('planning_sku_sync','planning_sku_resync')
		  ORDER BY latest.generation DESC, latest.id DESC LIMIT 1
		)
		WHERE tsi.task_id = ? ORDER BY tsi.sequence_no`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.PlanningSKUResultItem
		var revision domain.PlanningSKURevision
		var quantity sql.NullInt64
		var targetPrice sql.NullString
		var erpStatus string
		if err := rows.Scan(&item.TaskSKUItemID, &item.SequenceNo, &item.SKUCode, &quantity,
			&revision.ID, &revision.VersionNo, &revision.DescriptionSpec, &revision.Quantity, &targetPrice,
			&revision.Currency, &revision.Note, &revision.ReferenceURL, &revision.ERPProductIID, &revision.ERPProductName,
			&revision.ProductImageRefID, &revision.Reason, &revision.CreatedBy, &revision.CreatedAt, &erpStatus); err != nil {
			return nil, err
		}
		revision.TaskSKUItemID = item.TaskSKUItemID
		if targetPrice.Valid {
			value := targetPrice.String
			revision.TargetPrice = &value
		}
		item.Quantity = quantity.Int64
		item.ERPStatus = domain.FilingStatus(erpStatus)
		item.Revision = &revision
		out.Items = append(out.Items, item)
	}
	return &out, rows.Err()
}

func (r *PlanningSKURepo) GetUpdateLock(ctx context.Context, tx repo.Tx, taskID, itemID int64) (*domain.PlanningSKUUpdateLock, error) {
	var item domain.PlanningSKUUpdateLock
	var targetPrice sql.NullString
	var imageRef string
	err := Unwrap(tx).QueryRowContext(ctx, `
		SELECT t.id, tsi.id, tsi.sku_code, detail.lock_version, settings.erp_sync_mode,
		       rev.id, rev.version_no, rev.description_spec, rev.quantity, rev.target_price,
		       rev.currency, rev.note, rev.reference_url, rev.erp_product_i_id, rev.erp_product_name,
		       COALESCE(img.storage_ref_id, ''), rev.reason, rev.created_by, rev.created_at
		FROM tasks t
		JOIN task_planning_settings settings ON settings.task_id = t.id
		JOIN task_sku_items tsi ON tsi.task_id = t.id
		JOIN task_planning_sku_details detail ON detail.task_sku_item_id = tsi.id
		JOIN task_planning_sku_revisions rev ON rev.id = detail.current_revision_id
		LEFT JOIN task_planning_sku_revision_images img ON img.revision_id = rev.id
		WHERE t.id = ? AND tsi.id = ? AND t.task_type = 'sku_planning' AND t.task_status = 'Completed'
		FOR UPDATE`, taskID, itemID).Scan(
		&item.TaskID, &item.TaskSKUItemID, &item.SKUCode, &item.LockVersion, &item.ERPSyncMode,
		&item.CurrentRevision.ID, &item.CurrentRevision.VersionNo, &item.CurrentRevision.DescriptionSpec,
		&item.CurrentRevision.Quantity, &targetPrice, &item.CurrentRevision.Currency, &item.CurrentRevision.Note,
		&item.CurrentRevision.ReferenceURL, &item.CurrentRevision.ERPProductIID, &item.CurrentRevision.ERPProductName,
		&imageRef, &item.CurrentRevision.Reason, &item.CurrentRevision.CreatedBy, &item.CurrentRevision.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.CurrentRevision.TaskSKUItemID = item.TaskSKUItemID
	item.CurrentRevision.ProductImageRefID = imageRef
	if targetPrice.Valid {
		value := targetPrice.String
		item.CurrentRevision.TargetPrice = &value
	}
	return &item, nil
}

func (r *PlanningSKURepo) UpdateRevision(ctx context.Context, tx repo.Tx, lock domain.PlanningSKUUpdateLock, request domain.UpdatePlanningSKURequest, actorID int64) (*domain.PlanningSKURevision, error) {
	input := domain.PlanningSKUItemInput{
		DescriptionSpec: request.DescriptionSpec, Quantity: request.Quantity, TargetPrice: request.TargetPrice,
		Note: request.Note, ReferenceURL: request.ReferenceURL,
		ERPProductIID: lock.CurrentRevision.ERPProductIID, ERPProductName: lock.CurrentRevision.ERPProductName,
		ImageUploadRef: request.ImageUploadRef,
	}
	if input.ImageUploadRef == "" && !request.RemoveImage {
		input.ImageUploadRef = lock.CurrentRevision.ProductImageRefID
	}
	// Existing revision images remain immutable. Reusing one only pins the same
	// object to the new revision and does not change its storage ownership.
	sqlTx := Unwrap(tx)
	result, err := sqlTx.ExecContext(ctx, `
		INSERT INTO task_planning_sku_revisions
		  (task_sku_item_id, version_no, description_spec, quantity, target_price, currency,
		   note, reference_url, erp_product_i_id, erp_product_name, reason, created_by)
		VALUES (?, ?, ?, ?, ?, 'CNY', ?, ?, ?, ?, ?, ?)`,
		lock.TaskSKUItemID, lock.CurrentRevision.VersionNo+1, strings.TrimSpace(input.DescriptionSpec), input.Quantity,
		nullPlanningPrice(input.TargetPrice), strings.TrimSpace(input.Note), strings.TrimSpace(input.ReferenceURL),
		input.ERPProductIID, input.ERPProductName, strings.TrimSpace(request.Reason), actorID)
	if err != nil {
		return nil, err
	}
	revisionID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	if input.ImageUploadRef != "" {
		if _, err := sqlTx.ExecContext(ctx, `INSERT INTO task_planning_sku_revision_images (revision_id, storage_ref_id) VALUES (?, ?)`, revisionID, input.ImageUploadRef); err != nil {
			return nil, err
		}
		if input.ImageUploadRef != lock.CurrentRevision.ProductImageRefID {
			if _, err := sqlTx.ExecContext(ctx, `
				UPDATE asset_storage_refs SET owner_type = 'planning_sku_revision_image', owner_id = ?
				WHERE ref_id = ? AND owner_type = 'planning_sku_create' AND owner_id = ?`, revisionID, input.ImageUploadRef, actorID); err != nil {
				return nil, err
			}
		}
	}
	result, err = sqlTx.ExecContext(ctx, `
		UPDATE task_planning_sku_details
		SET current_revision_id = ?, lock_version = lock_version + 1
		WHERE task_sku_item_id = ? AND lock_version = ?`, revisionID, lock.TaskSKUItemID, request.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return nil, repo.ErrConflict
	}
	if _, err := sqlTx.ExecContext(ctx, `UPDATE task_sku_items SET quantity = ? WHERE id = ? AND task_id = ?`, request.Quantity, lock.TaskSKUItemID, lock.TaskID); err != nil {
		return nil, err
	}
	return &domain.PlanningSKURevision{
		ID: revisionID, TaskSKUItemID: lock.TaskSKUItemID, VersionNo: lock.CurrentRevision.VersionNo + 1,
		DescriptionSpec: strings.TrimSpace(input.DescriptionSpec), Quantity: input.Quantity, TargetPrice: input.TargetPrice,
		Currency: "CNY", Note: strings.TrimSpace(input.Note), ReferenceURL: strings.TrimSpace(input.ReferenceURL),
		ERPProductIID: input.ERPProductIID, ERPProductName: input.ERPProductName, ProductImageRefID: input.ImageUploadRef,
		Reason: strings.TrimSpace(request.Reason), CreatedBy: actorID,
	}, nil
}

func (r *PlanningSKURepo) ListExportRows(ctx context.Context, taskIDs, itemIDs []int64) ([]domain.PlanningSKUExportRow, error) {
	where := []string{"t.task_type = 'sku_planning'"}
	args := make([]interface{}, 0, len(taskIDs)+len(itemIDs))
	if len(taskIDs) > 0 {
		where = append(where, "t.id IN ("+sqlPlaceholders(len(taskIDs))+")")
		for _, id := range taskIDs {
			args = append(args, id)
		}
	}
	if len(itemIDs) > 0 {
		where = append(where, "tsi.id IN ("+sqlPlaceholders(len(itemIDs))+")")
		for _, id := range itemIDs {
			args = append(args, id)
		}
	}
	if len(taskIDs) == 0 && len(itemIDs) == 0 {
		return nil, fmt.Errorf("an export selection is required")
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT t.id, t.task_no, tsi.sequence_no, tsi.id, tsi.sku_code,
		       COALESCE(img.storage_ref_id, ''), rev.description_spec, rev.quantity, rev.target_price,
		       rev.note, rev.reference_url, COALESCE(outbox.status, ''),
		       COALESCE(NULLIF(u.display_name, ''), u.username), t.updated_at
		FROM tasks t
		JOIN users u ON u.id = t.creator_id
		JOIN task_sku_items tsi ON tsi.task_id = t.id
		JOIN task_planning_sku_details detail ON detail.task_sku_item_id = tsi.id
		JOIN task_planning_sku_revisions rev ON rev.id = detail.current_revision_id
		LEFT JOIN task_planning_sku_revision_images img ON img.revision_id = rev.id
		LEFT JOIN task_erp_outbox outbox ON outbox.id = (
		  SELECT latest.id FROM task_erp_outbox latest WHERE latest.task_sku_item_id = tsi.id
		  ORDER BY latest.generation DESC, latest.id DESC LIMIT 1
		)
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY t.id, tsi.sequence_no LIMIT 5001`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.PlanningSKUExportRow, 0)
	for rows.Next() {
		var item domain.PlanningSKUExportRow
		var target sql.NullString
		var completed sql.NullTime
		if err := rows.Scan(&item.TaskID, &item.TaskNo, &item.SequenceNo, &item.TaskSKUItemID, &item.SKUCode,
			&item.ImageRefID, &item.DescriptionSpec, &item.Quantity, &target, &item.Note, &item.ReferenceURL,
			&item.ERPStatus, &item.CreatorName, &completed); err != nil {
			return nil, err
		}
		if target.Valid {
			value := target.String
			item.TargetPrice = &value
		}
		item.CompletedAt = fromNullTime(completed)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PlanningSKURepo) EnqueueTaskERP(ctx context.Context, tx repo.Tx, taskID int64, jobType string) (int, error) {
	rows, err := Unwrap(tx).QueryContext(ctx, `
		SELECT tsi.id, tsi.sku_code, rev.id, rev.erp_product_i_id, rev.erp_product_name,
		       COALESCE(img.storage_ref_id, ''),
		       COALESCE((SELECT MAX(generation) FROM task_erp_outbox existing WHERE existing.task_sku_item_id = tsi.id), 0) + 1
		FROM tasks t
		JOIN task_sku_items tsi ON tsi.task_id = t.id
		JOIN task_planning_sku_details detail ON detail.task_sku_item_id = tsi.id
		JOIN task_planning_sku_revisions rev ON rev.id = detail.current_revision_id
		LEFT JOIN task_planning_sku_revision_images img ON img.revision_id = rev.id
		WHERE t.id = ? AND t.task_type = 'sku_planning' AND t.task_status = 'Completed'
		ORDER BY tsi.sequence_no FOR UPDATE`, taskID)
	if err != nil {
		return 0, err
	}
	type queuedItem struct {
		itemID, revisionID    int64
		sku, iid, name, image string
		generation            int
	}
	items := make([]queuedItem, 0)
	for rows.Next() {
		var item queuedItem
		if err := rows.Scan(&item.itemID, &item.sku, &item.revisionID, &item.iid, &item.name, &item.image, &item.generation); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, repo.ErrNotFound
	}
	for _, item := range items {
		if err := r.EnqueueERP(ctx, tx, taskID, item.itemID, jobType, item.generation, map[string]interface{}{
			"task_id": taskID, "task_sku_item_id": item.itemID, "sku_code": item.sku,
			"revision_id": item.revisionID, "erp_product_i_id": item.iid,
			"erp_product_name": item.name, "image_ref_id": item.image,
		}); err != nil {
			return 0, err
		}
	}
	return len(items), nil
}

func (r *PlanningSKURepo) ReindexTask(ctx context.Context, tx repo.Tx, taskID int64) error {
	return reindexTaskSearchDocument(ctx, Unwrap(tx), taskID)
}

func sqlPlaceholders(count int) string {
	if count <= 0 {
		return "NULL"
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}

func nullPlanningPrice(value *string) interface{} {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return strings.TrimSpace(*value)
}

func jsonMarshal(value interface{}) ([]byte, error) {
	return json.Marshal(value)
}
