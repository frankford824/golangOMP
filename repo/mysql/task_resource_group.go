package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type TaskResourceGroupRepo struct{ db *DB }

func NewTaskResourceGroupRepo(db *DB) *TaskResourceGroupRepo {
	return &TaskResourceGroupRepo{db: db}
}

func (r *TaskResourceGroupRepo) GetWorkflow(ctx context.Context, taskID int64) (*domain.TaskWorkflowLock, error) {
	var item domain.TaskWorkflowLock
	var handlerID, designerID, requesterID, departmentID, teamID sql.NullInt64
	err := r.db.db.QueryRowContext(ctx, `
		SELECT id, task_type, task_status, workflow_revision, current_handler_id, designer_id, customization_required,
		       creator_id, requester_id, owner_department_id, owner_team_id
		FROM tasks WHERE id = ?`, taskID).
		Scan(&item.TaskID, &item.TaskType, &item.Status, &item.WorkflowRevision, &handlerID, &designerID, &item.Customization,
			&item.CreatorID, &requesterID, &departmentID, &teamID)
	if err == sql.ErrNoRows {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read task workflow: %w", err)
	}
	item.CurrentHandlerID = fromNullInt64(handlerID)
	item.DesignerID = fromNullInt64(designerID)
	item.RequesterID = fromNullInt64(requesterID)
	item.OwnerDepartmentID = fromNullInt64(departmentID)
	item.OwnerTeamID = fromNullInt64(teamID)
	return &item, nil
}

func (r *TaskResourceGroupRepo) GetWorkflowForUpdate(ctx context.Context, tx repo.Tx, taskID int64) (*domain.TaskWorkflowLock, error) {
	var item domain.TaskWorkflowLock
	var handlerID, designerID, requesterID, departmentID, teamID sql.NullInt64
	err := Unwrap(tx).QueryRowContext(ctx, `
		SELECT id, task_type, task_status, workflow_revision, current_handler_id, designer_id, customization_required,
		       creator_id, requester_id, owner_department_id, owner_team_id
		FROM tasks WHERE id = ? FOR UPDATE`, taskID).
		Scan(&item.TaskID, &item.TaskType, &item.Status, &item.WorkflowRevision, &handlerID, &designerID, &item.Customization,
			&item.CreatorID, &requesterID, &departmentID, &teamID)
	if err == sql.ErrNoRows {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock task workflow: %w", err)
	}
	item.CurrentHandlerID = fromNullInt64(handlerID)
	item.DesignerID = fromNullInt64(designerID)
	item.RequesterID = fromNullInt64(requesterID)
	item.OwnerDepartmentID = fromNullInt64(departmentID)
	item.OwnerTeamID = fromNullInt64(teamID)
	return &item, nil
}

func (r *TaskResourceGroupRepo) EnsureGroupShells(ctx context.Context, tx repo.Tx, taskID int64, taskType domain.TaskType) error {
	sqlTx := Unwrap(tx)
	switch taskType {
	case domain.TaskTypeSKUPlanning, domain.TaskTypePurchaseTask:
		return nil
	case domain.TaskTypeRetouchTask:
		_, err := sqlTx.ExecContext(ctx, `
			INSERT INTO task_asset_groups (task_id, scope_kind, retouch_requirement_id)
			SELECT trr.task_id, 'retouch_requirement', trr.id
			FROM task_retouch_requirements trr
			WHERE trr.task_id = ?
			ON DUPLICATE KEY UPDATE updated_at = task_asset_groups.updated_at`, taskID)
		return err
	default:
		var skuCount int
		if err := sqlTx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_sku_items WHERE task_id = ?`, taskID).Scan(&skuCount); err != nil {
			return err
		}
		if skuCount > 0 {
			_, err := sqlTx.ExecContext(ctx, `
			INSERT INTO task_asset_groups (task_id, scope_kind, task_sku_item_id)
			SELECT tsi.task_id, 'sku', tsi.id
			FROM task_sku_items tsi
			WHERE tsi.task_id = ?
			ON DUPLICATE KEY UPDATE updated_at = task_asset_groups.updated_at`, taskID)
			return err
		}
		_, err := sqlTx.ExecContext(ctx, `
			INSERT INTO task_asset_groups (task_id, scope_kind)
			VALUES (?, 'task')
			ON DUPLICATE KEY UPDATE updated_at = task_asset_groups.updated_at`, taskID)
		return err
	}
}

func (r *TaskResourceGroupRepo) ExpectedResourceGroupCount(ctx context.Context, taskID int64, taskType domain.TaskType) (int64, error) {
	return expectedResourceGroupCount(ctx, r.db.db, taskID, taskType)
}

func (r *TaskResourceGroupRepo) ExpectedResourceGroupCountForUpdate(ctx context.Context, tx repo.Tx, taskID int64, taskType domain.TaskType) (int64, error) {
	return expectedResourceGroupCount(ctx, Unwrap(tx), taskID, taskType)
}

type resourceGroupCountQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func expectedResourceGroupCount(ctx context.Context, queryer resourceGroupCountQuerier, taskID int64, taskType domain.TaskType) (int64, error) {
	var count int64
	switch taskType {
	case domain.TaskTypeSKUPlanning, domain.TaskTypePurchaseTask:
		return 0, nil
	case domain.TaskTypeRetouchTask:
		if err := queryer.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_retouch_requirements WHERE task_id = ?`, taskID).Scan(&count); err != nil {
			return 0, err
		}
		return count, nil
	default:
		if err := queryer.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_sku_items WHERE task_id = ?`, taskID).Scan(&count); err != nil {
			return 0, err
		}
		if count == 0 {
			return 1, nil
		}
		return count, nil
	}
}

func (r *TaskResourceGroupRepo) ListByTaskID(ctx context.Context, taskID int64) ([]domain.TaskAssetGroup, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT g.id, g.task_id, g.scope_kind, g.task_sku_item_id, g.retouch_requirement_id,
		       g.working_revision_id, g.finalized_revision_id, g.lock_version,
		       g.migration_incomplete, g.migration_issue, g.created_at, g.updated_at,
		       t.task_no, COALESCE(tsi.sku_code, ''),
		       COALESCE(NULLIF(tsi.product_short_name, ''), NULLIF(tsi.product_name_snapshot, ''), NULLIF(t.product_name_snapshot, ''), ''),
		       t.creator_id, COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username, ''), ''), t.business_lane
		FROM task_asset_groups g
		JOIN tasks t ON t.id = g.task_id
		LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
		LEFT JOIN users u ON u.id = t.creator_id
		WHERE g.task_id = ?
		ORDER BY FIELD(g.scope_kind, 'task','sku','retouch_requirement'), g.scope_ref_id, g.id`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task resource groups: %w", err)
	}
	defer rows.Close()
	items := make([]domain.TaskAssetGroup, 0)
	for rows.Next() {
		var item domain.TaskAssetGroup
		var skuID, retouchID, workingID, finalizedID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.TaskID, &item.ScopeKind, &skuID, &retouchID, &workingID, &finalizedID,
			&item.LockVersion, &item.MigrationIncomplete, &item.MigrationIssue, &item.CreatedAt, &item.UpdatedAt,
			&item.TaskNo, &item.SKUCode, &item.ProductName, &item.CreatorID, &item.CreatorName, &item.BusinessLane); err != nil {
			return nil, err
		}
		item.TaskSKUItemID = fromNullInt64(skuID)
		item.RetouchRequirementID = fromNullInt64(retouchID)
		item.WorkingRevisionID = fromNullInt64(workingID)
		item.FinalizedRevisionID = fromNullInt64(finalizedID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.hydrateResourceGroupRevisions(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *TaskResourceGroupRepo) ListResourceGroups(ctx context.Context, params domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64, error) {
	where := []string{"g.finalized_revision_id IS NOT NULL"}
	args := make([]interface{}, 0, 12)
	if params.TaskID > 0 {
		where = append(where, "g.task_id = ?")
		args = append(args, params.TaskID)
	}
	if value := strings.TrimSpace(params.SKUCode); value != "" {
		where = append(where, "tsi.sku_code = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(params.TaskNo); value != "" {
		where = append(where, "t.task_no LIKE ?")
		args = append(args, "%"+value+"%")
	}
	if params.CreatorID != nil && *params.CreatorID > 0 {
		where = append(where, "t.creator_id = ?")
		args = append(args, *params.CreatorID)
	}
	switch params.ResourceRole {
	case domain.ResourceRoleFilterReference:
		where = append(where, `EXISTS (
			SELECT 1 FROM task_asset_group_revision_references rr
			WHERE rr.revision_id = g.finalized_revision_id
		)`)
	case domain.ResourceRoleFilterSource:
		where = append(where, `EXISTS (
			SELECT 1 FROM task_asset_group_revisions gr
			WHERE gr.id = g.finalized_revision_id AND gr.source_task_asset_id IS NOT NULL
		)`)
	case domain.ResourceRoleFilterFinal:
		where = append(where, `EXISTS (
			SELECT 1 FROM task_asset_group_revision_items ri
			WHERE ri.revision_id = g.finalized_revision_id
		)`)
	}
	if value := strings.TrimSpace(params.Query); value != "" {
		like := "%" + value + "%"
		where = append(where, `(
			t.task_no LIKE ? OR COALESCE(tsi.sku_code,'') LIKE ?
			OR EXISTS (
				SELECT 1 FROM task_asset_group_search_documents doc
				WHERE doc.group_id=g.id AND (doc.internal_text LIKE ? OR doc.final_text LIKE ?)
			)
			OR EXISTS (
				SELECT 1
				FROM task_asset_group_revision_items ri
				JOIN task_assets ta ON ta.id=ri.task_asset_id
				WHERE ri.revision_id=g.finalized_revision_id AND ta.file_name LIKE ?
			)
		)`)
		args = append(args, like, like, like, like, like)
	}
	if params.BusinessLane.Valid() {
		where = append(where, "t.business_lane = ?")
		args = append(args, params.BusinessLane)
	}
	if normalizeAssetFormatCategoryForSQL(params.FormatCategory) != domain.AssetFormatCategoryAll {
		formatClauses, formatArgs := appendAssetFormatCategoryWhere(
			[]string{`(
				ta.id = (SELECT gr.source_task_asset_id FROM task_asset_group_revisions gr WHERE gr.id = g.finalized_revision_id)
				OR EXISTS (
					SELECT 1 FROM task_asset_group_revision_items ri
					WHERE ri.revision_id = g.finalized_revision_id AND ri.task_asset_id = ta.id
				)
			)`}, nil,
			[]string{"LOWER(ta.file_name)"}, "LOWER(COALESCE(ta.mime_type, ''))", params.FormatCategory,
		)
		where = append(where, `EXISTS (
			SELECT 1 FROM task_assets ta
			WHERE `+strings.Join(formatClauses, " AND ")+`
		)`)
		args = append(args, formatArgs...)
	}
	where, args = appendResourceGroupAccessScope(where, args, params.Access)
	clause := strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM task_asset_groups g
		JOIN tasks t ON t.id = g.task_id
		LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
		WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	queryArgs := append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT g.id, g.task_id, g.scope_kind, g.task_sku_item_id, g.retouch_requirement_id,
		       g.working_revision_id, g.finalized_revision_id, g.lock_version,
		       g.migration_incomplete, g.migration_issue, g.created_at, g.updated_at,
		       t.task_no, COALESCE(tsi.sku_code, ''),
		       COALESCE(NULLIF(tsi.product_short_name, ''), NULLIF(tsi.product_name_snapshot, ''), NULLIF(t.product_name_snapshot, ''), ''),
		       t.creator_id, COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username, ''), ''), t.business_lane
		FROM task_asset_groups g
		JOIN tasks t ON t.id = g.task_id
		LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
		LEFT JOIN users u ON u.id = t.creator_id
		WHERE `+clause+`
		ORDER BY g.updated_at DESC, g.id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.TaskAssetGroup, 0, pageSize)
	for rows.Next() {
		item, err := scanTaskResourceGroup(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := r.hydrateResourceGroupRevisions(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *TaskResourceGroupRepo) ListFlatResourceItems(ctx context.Context, params domain.ResourceGroupListParams) ([]domain.FlatResourceItem, int64, error) {
	baseWhere := []string{"g.finalized_revision_id IS NOT NULL"}
	baseArgs := make([]interface{}, 0, 12)
	if params.TaskID > 0 {
		baseWhere = append(baseWhere, "g.task_id = ?")
		baseArgs = append(baseArgs, params.TaskID)
	}
	if value := strings.TrimSpace(params.SKUCode); value != "" {
		baseWhere = append(baseWhere, "tsi.sku_code = ?")
		baseArgs = append(baseArgs, value)
	}
	if value := strings.TrimSpace(params.TaskNo); value != "" {
		baseWhere = append(baseWhere, "t.task_no LIKE ?")
		baseArgs = append(baseArgs, "%"+value+"%")
	}
	if params.CreatorID != nil && *params.CreatorID > 0 {
		baseWhere = append(baseWhere, "t.creator_id = ?")
		baseArgs = append(baseArgs, *params.CreatorID)
	}
	if params.BusinessLane.Valid() {
		baseWhere = append(baseWhere, "t.business_lane = ?")
		baseArgs = append(baseArgs, params.BusinessLane)
	}
	baseWhere, baseArgs = appendResourceGroupAccessScope(baseWhere, baseArgs, params.Access)
	baseClause := strings.Join(baseWhere, " AND ")
	if err := r.validateFlatResourceIntegrity(ctx, baseClause, baseArgs); err != nil {
		return nil, 0, err
	}

	flatCTE := `WITH flat_resources AS (
		SELECT g.id AS group_id, g.task_id, t.task_no, COALESCE(tsi.sku_code, '') AS sku_code,
		       'reference' AS resource_role,
		       COALESCE(NULLIF(rr.file_name_snapshot, ''), asr.file_name, '') AS file_name,
		       COALESCE(asr.mime_type, '') AS mime_type,
		       CASE WHEN COALESCE(asr.is_placeholder, 1) = 0 THEN COALESCE(asr.ref_key, '') ELSE '' END AS storage_key,
		       COALESCE(formal_ta.id, 0) AS task_asset_id,
		       g.updated_at AS group_updated_at, 1 AS role_sort, rr.sort_order AS item_sort, rr.id AS row_id
		FROM task_asset_groups g
		JOIN tasks t ON t.id = g.task_id
		LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
		JOIN task_asset_group_revision_references rr ON rr.revision_id = g.finalized_revision_id
		JOIN reference_file_refs f ON f.id = rr.reference_file_ref_id
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = rr.ref_id_snapshot
		LEFT JOIN task_assets formal_ta ON formal_ta.id = rr.formal_task_asset_id
		LEFT JOIN asset_storage_refs formal_asr ON formal_asr.ref_id = formal_ta.storage_ref_id
		WHERE ` + baseClause + `
		  AND f.task_id = g.task_id
		  AND rr.ref_id_snapshot = f.ref_id
		  AND rr.scope_snapshot = CASE
		    WHEN f.retouch_requirement_id IS NOT NULL THEN CONCAT('retouch_requirement:', f.retouch_requirement_id)
		    WHEN f.sku_item_id IS NOT NULL THEN CONCAT('sku:', f.sku_item_id)
		    ELSE 'task'
		  END
		  AND (
		    (f.sku_item_id IS NULL AND f.retouch_requirement_id IS NULL)
		    OR (g.scope_kind = 'sku' AND f.sku_item_id = g.task_sku_item_id AND f.retouch_requirement_id IS NULL)
		    OR (g.scope_kind = 'retouch_requirement' AND f.sku_item_id IS NULL AND f.retouch_requirement_id = g.retouch_requirement_id)
		  )
		  AND asr.ref_id IS NOT NULL
		  AND COALESCE(asr.status, '') NOT IN ('archived', 'historical_unavailable')
		  AND (
		    rr.formal_task_asset_id IS NULL
		    OR (
		      formal_ta.id IS NOT NULL AND formal_ta.task_id = g.task_id
		      AND formal_ta.asset_type = 'reference'
		      AND formal_ta.binding_state = 'bound'
		      AND (formal_ta.bound_role IS NULL OR TRIM(formal_ta.bound_role) = '')
		      AND COALESCE(formal_ta.is_archived, 0) = 0
		      AND formal_ta.deleted_at IS NULL AND formal_ta.cleaned_at IS NULL
		      AND formal_ta.access_revoked_at IS NULL AND formal_ta.object_deleted_at IS NULL
		      AND formal_ta.storage_ref_id = rr.ref_id_snapshot
		      AND formal_asr.ref_id IS NOT NULL
		      AND COALESCE(formal_asr.status, '') NOT IN ('archived', 'historical_unavailable')
		    )
		  )
		UNION ALL
		SELECT g.id AS group_id, g.task_id, t.task_no, COALESCE(tsi.sku_code, '') AS sku_code,
		       'source' AS resource_role, ta.file_name, COALESCE(ta.mime_type, '') AS mime_type,
		       COALESCE(NULLIF(ta.storage_key, ''), CASE WHEN COALESCE(asr.is_placeholder, 1) = 0 THEN NULLIF(asr.ref_key, '') END, '') AS storage_key,
		       ta.id AS task_asset_id,
		       g.updated_at AS group_updated_at, 2 AS role_sort, 0 AS item_sort, ta.id AS row_id
		FROM task_asset_groups g
		JOIN tasks t ON t.id = g.task_id
		LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
		JOIN task_asset_group_revisions rev ON rev.id = g.finalized_revision_id
		JOIN task_assets ta ON ta.id = rev.source_task_asset_id
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		WHERE ` + baseClause + `
		  AND ta.task_id = g.task_id AND ta.asset_type = 'source'
		  AND ta.binding_state = 'bound' AND ta.bound_group_id = g.id AND ta.bound_role = 'source'
		  AND COALESCE(ta.is_archived, 0) = 0
		  AND ta.deleted_at IS NULL AND ta.cleaned_at IS NULL
		  AND ta.access_revoked_at IS NULL AND ta.object_deleted_at IS NULL
		  AND ta.storage_ref_id IS NOT NULL AND asr.ref_id IS NOT NULL
		  AND COALESCE(asr.status, '') NOT IN ('archived', 'historical_unavailable')
		UNION ALL
		SELECT g.id AS group_id, g.task_id, t.task_no, COALESCE(tsi.sku_code, '') AS sku_code,
		       'final' AS resource_role, ta.file_name, COALESCE(ta.mime_type, '') AS mime_type,
		       COALESCE(NULLIF(ta.storage_key, ''), CASE WHEN COALESCE(asr.is_placeholder, 1) = 0 THEN NULLIF(asr.ref_key, '') END, '') AS storage_key,
		       ta.id AS task_asset_id,
		       g.updated_at AS group_updated_at, 3 AS role_sort, ri.sort_order AS item_sort, ri.id AS row_id
		FROM task_asset_groups g
		JOIN tasks t ON t.id = g.task_id
		LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
		JOIN task_asset_group_revision_items ri ON ri.revision_id = g.finalized_revision_id
		JOIN task_assets ta ON ta.id = ri.task_asset_id
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		WHERE ` + baseClause + `
		  AND ta.task_id = g.task_id AND ta.asset_type = 'delivery'
		  AND ta.binding_state = 'bound' AND ta.bound_group_id = g.id AND ta.bound_role = 'final'
		  AND COALESCE(ta.is_archived, 0) = 0
		  AND ta.deleted_at IS NULL AND ta.cleaned_at IS NULL
		  AND ta.access_revoked_at IS NULL AND ta.object_deleted_at IS NULL
		  AND ta.storage_ref_id IS NOT NULL AND asr.ref_id IS NOT NULL
		  AND COALESCE(asr.status, '') NOT IN ('archived', 'historical_unavailable')
	)`
	unionArgs := make([]interface{}, 0, len(baseArgs)*3)
	for range 3 {
		unionArgs = append(unionArgs, baseArgs...)
	}
	flatWhere := []string{"1 = 1"}
	flatArgs := make([]interface{}, 0, 12)
	if params.ResourceRole != "" {
		flatWhere = append(flatWhere, "flat.resource_role = ?")
		flatArgs = append(flatArgs, params.ResourceRole)
	}
	if value := strings.TrimSpace(params.Query); value != "" {
		like := "%" + value + "%"
		flatWhere = append(flatWhere, `(
			flat.task_no LIKE ? OR flat.sku_code LIKE ? OR flat.file_name LIKE ?
			OR EXISTS (
				SELECT 1 FROM task_asset_group_search_documents doc
				WHERE doc.group_id = flat.group_id AND (doc.internal_text LIKE ? OR doc.final_text LIKE ?)
			)
		)`)
		flatArgs = append(flatArgs, like, like, like, like, like)
	}
	if normalizeAssetFormatCategoryForSQL(params.FormatCategory) != domain.AssetFormatCategoryAll {
		flatWhere, flatArgs = appendAssetFormatCategoryWhere(
			flatWhere, flatArgs,
			[]string{"LOWER(flat.file_name)"}, "LOWER(COALESCE(flat.mime_type, ''))", params.FormatCategory,
		)
	}
	flatClause := strings.Join(flatWhere, " AND ")
	filterArgs := append(append([]interface{}{}, unionArgs...), flatArgs...)

	var total int64
	if err := r.db.db.QueryRowContext(ctx, flatCTE+`
		SELECT COUNT(*) FROM flat_resources flat WHERE `+flatClause, filterArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	queryArgs := append(append([]interface{}{}, filterArgs...), pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, flatCTE+`
		SELECT flat.group_id, flat.task_id, flat.task_no, flat.sku_code, flat.resource_role,
		       flat.file_name, flat.mime_type, flat.storage_key, flat.task_asset_id
		FROM flat_resources flat
		WHERE `+flatClause+`
		ORDER BY flat.group_updated_at DESC, flat.group_id DESC, flat.role_sort, flat.item_sort, flat.row_id
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.FlatResourceItem, 0, pageSize)
	for rows.Next() {
		var item domain.FlatResourceItem
		if err := rows.Scan(&item.GroupID, &item.TaskID, &item.TaskNo, &item.SKUCode, &item.ResourceRole,
			&item.FileName, &item.MimeType, &item.StorageKey, &item.TaskAssetID); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *TaskResourceGroupRepo) validateFlatResourceIntegrity(ctx context.Context, baseClause string, baseArgs []interface{}) error {
	query := `SELECT violation_code, entity_id
		FROM (
			SELECT 'source_ownership' AS violation_code, rev.id AS entity_id
			FROM task_asset_groups g
			JOIN tasks t ON t.id = g.task_id
			LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
			JOIN task_asset_group_revisions rev ON rev.id = g.finalized_revision_id
			LEFT JOIN task_assets ta ON ta.id = rev.source_task_asset_id
			LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
			WHERE ` + baseClause + `
			  AND rev.source_task_asset_id IS NOT NULL
			  AND (
			    ta.id IS NULL OR ta.task_id <> g.task_id OR ta.asset_type <> 'source'
			    OR ta.binding_state <> 'bound'
			    OR ta.bound_group_id IS NULL OR ta.bound_group_id <> g.id
			    OR ta.bound_role IS NULL OR ta.bound_role <> 'source'
			    OR COALESCE(ta.is_archived, 0) <> 0
			    OR ta.deleted_at IS NOT NULL OR ta.cleaned_at IS NOT NULL
			    OR ta.access_revoked_at IS NOT NULL OR ta.object_deleted_at IS NOT NULL
			    OR ta.storage_ref_id IS NULL OR asr.ref_id IS NULL
			    OR COALESCE(asr.status, '') IN ('archived', 'historical_unavailable')
			  )
			UNION ALL
			SELECT 'final_ownership' AS violation_code, ri.id AS entity_id
			FROM task_asset_groups g
			JOIN tasks t ON t.id = g.task_id
			LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
			JOIN task_asset_group_revision_items ri ON ri.revision_id = g.finalized_revision_id
			LEFT JOIN task_assets ta ON ta.id = ri.task_asset_id
			LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
			WHERE ` + baseClause + `
			  AND (
			    ta.id IS NULL OR ta.task_id <> g.task_id OR ta.asset_type <> 'delivery'
			    OR ta.binding_state <> 'bound'
			    OR ta.bound_group_id IS NULL OR ta.bound_group_id <> g.id
			    OR ta.bound_role IS NULL OR ta.bound_role <> 'final'
			    OR COALESCE(ta.is_archived, 0) <> 0
			    OR ta.deleted_at IS NOT NULL OR ta.cleaned_at IS NOT NULL
			    OR ta.access_revoked_at IS NOT NULL OR ta.object_deleted_at IS NOT NULL
			    OR ta.storage_ref_id IS NULL OR asr.ref_id IS NULL
			    OR COALESCE(asr.status, '') IN ('archived', 'historical_unavailable')
			  )
			UNION ALL
			SELECT 'reference_ownership' AS violation_code, rr.id AS entity_id
			FROM task_asset_groups g
			JOIN tasks t ON t.id = g.task_id
			LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
			JOIN task_asset_group_revision_references rr ON rr.revision_id = g.finalized_revision_id
			LEFT JOIN reference_file_refs f ON f.id = rr.reference_file_ref_id
			LEFT JOIN asset_storage_refs asr ON asr.ref_id = rr.ref_id_snapshot
			LEFT JOIN task_assets formal_ta ON formal_ta.id = rr.formal_task_asset_id
			LEFT JOIN asset_storage_refs formal_asr ON formal_asr.ref_id = formal_ta.storage_ref_id
			WHERE ` + baseClause + `
			  AND (
			    f.id IS NULL OR f.task_id <> g.task_id OR rr.ref_id_snapshot <> f.ref_id
			    OR rr.scope_snapshot <> CASE
			      WHEN f.retouch_requirement_id IS NOT NULL THEN CONCAT('retouch_requirement:', f.retouch_requirement_id)
			      WHEN f.sku_item_id IS NOT NULL THEN CONCAT('sku:', f.sku_item_id)
			      ELSE 'task'
			    END
			    OR NOT (
			      (f.sku_item_id IS NULL AND f.retouch_requirement_id IS NULL)
			      OR (g.scope_kind = 'sku' AND f.sku_item_id = g.task_sku_item_id AND f.retouch_requirement_id IS NULL)
			      OR (g.scope_kind = 'retouch_requirement' AND f.sku_item_id IS NULL AND f.retouch_requirement_id = g.retouch_requirement_id)
			    )
			    OR (
			      rr.formal_task_asset_id IS NOT NULL
			      AND (
			        formal_ta.id IS NULL OR formal_ta.task_id <> g.task_id
			        OR formal_ta.asset_type <> 'reference'
			        OR formal_ta.binding_state <> 'bound'
			        OR (formal_ta.bound_role IS NOT NULL AND TRIM(formal_ta.bound_role) <> '')
			        OR COALESCE(formal_ta.is_archived, 0) <> 0
			        OR formal_ta.deleted_at IS NOT NULL OR formal_ta.cleaned_at IS NOT NULL
			        OR formal_ta.access_revoked_at IS NOT NULL OR formal_ta.object_deleted_at IS NOT NULL
			        OR formal_ta.storage_ref_id IS NULL
			        OR formal_ta.storage_ref_id <> rr.ref_id_snapshot
			        OR formal_asr.ref_id IS NULL
			      )
			    )
			    OR asr.ref_id IS NULL
			    OR COALESCE(asr.status, '') IN ('archived', 'historical_unavailable')
			    OR (
			      rr.formal_task_asset_id IS NOT NULL
			      AND COALESCE(formal_asr.status, '') IN ('archived', 'historical_unavailable')
			    )
			  )
		) violations
		ORDER BY violation_code, entity_id
		LIMIT 1`
	args := make([]interface{}, 0, len(baseArgs)*3)
	for range 3 {
		args = append(args, baseArgs...)
	}
	var violationCode string
	var entityID int64
	err := r.db.db.QueryRowContext(ctx, query, args...).Scan(&violationCode, &entityID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf(
		"%w: flat resource read model %s at entity %d",
		repo.ErrDataIntegrity, violationCode, entityID,
	)
}

func appendResourceGroupAccessScope(where []string, args []interface{}, access domain.ResourceGroupAccessFilter) ([]string, []interface{}) {
	if access.Global {
		return where, args
	}
	or := make([]string, 0, 3)
	if access.Self && access.ActorID > 0 {
		or = append(or, "(t.creator_id = ? OR t.requester_id = ? OR t.designer_id = ? OR t.current_handler_id = ?)")
		args = append(args, access.ActorID, access.ActorID, access.ActorID, access.ActorID)
	}
	if len(access.DepartmentIDs) > 0 {
		or = append(or, "t.owner_department_id IN ("+resourceGroupPlaceholders(len(access.DepartmentIDs))+")")
		for _, id := range access.DepartmentIDs {
			args = append(args, id)
		}
	}
	if len(access.TeamIDs) > 0 {
		or = append(or, "t.owner_team_id IN ("+resourceGroupPlaceholders(len(access.TeamIDs))+")")
		for _, id := range access.TeamIDs {
			args = append(args, id)
		}
	}
	if len(or) == 0 {
		where = append(where, "1 = 0")
		return where, args
	}
	where = append(where, "("+strings.Join(or, " OR ")+")")
	return where, args
}

func resourceGroupPlaceholders(count int) string {
	if count <= 0 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func (r *TaskResourceGroupRepo) GetResourceGroup(ctx context.Context, groupID int64) (*domain.TaskAssetGroup, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT g.id, g.task_id, g.scope_kind, g.task_sku_item_id, g.retouch_requirement_id,
		       g.working_revision_id, g.finalized_revision_id, g.lock_version,
		       g.migration_incomplete, g.migration_issue, g.created_at, g.updated_at,
		       t.task_no, COALESCE(tsi.sku_code, ''),
		       COALESCE(NULLIF(tsi.product_short_name, ''), NULLIF(tsi.product_name_snapshot, ''), NULLIF(t.product_name_snapshot, ''), ''),
		       t.creator_id, COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username, ''), ''), t.business_lane
		FROM task_asset_groups g
		JOIN tasks t ON t.id = g.task_id
		LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
		LEFT JOIN users u ON u.id = t.creator_id
		WHERE g.id = ?`, groupID)
	item, err := scanTaskResourceGroup(row)
	if err == sql.ErrNoRows {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	groups := []domain.TaskAssetGroup{*item}
	if err := r.hydrateResourceGroupRevisions(ctx, groups); err != nil {
		return nil, err
	}
	return &groups[0], nil
}

func (r *TaskResourceGroupRepo) ListResourceGroupRevisions(ctx context.Context, groupID int64, page, pageSize int) ([]domain.TaskAssetGroupRevision, int64, error) {
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_asset_group_revisions WHERE group_id = ?`, groupID).Scan(&total); err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id
		FROM task_asset_group_revisions
		WHERE group_id = ?
		ORDER BY revision_no DESC, id DESC LIMIT ? OFFSET ?`, groupID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	revisionIDs := make([]int64, 0, pageSize)
	for rows.Next() {
		var revisionID int64
		if err := rows.Scan(&revisionID); err != nil {
			rows.Close()
			return nil, 0, err
		}
		revisionIDs = append(revisionIDs, revisionID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if len(revisionIDs) == 0 {
		return []domain.TaskAssetGroupRevision{}, total, nil
	}
	revisions, err := r.loadResourceGroupRevisions(ctx, revisionIDs)
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.TaskAssetGroupRevision, 0, len(revisionIDs))
	for _, revisionID := range revisionIDs {
		revision := revisions[revisionID]
		if revision == nil {
			return nil, 0, fmt.Errorf("%w: list resource group revisions: revision %d was not hydrated", repo.ErrDataIntegrity, revisionID)
		}
		items = append(items, *revision)
	}
	return items, total, nil
}

func (r *TaskResourceGroupRepo) GetTaskAccessSubject(ctx context.Context, taskID int64) (domain.TaskAccessSubject, error) {
	var item domain.TaskAccessSubject
	var requesterID, designerID, handlerID, departmentID, teamID sql.NullInt64
	err := r.db.db.QueryRowContext(ctx, `
		SELECT id, creator_id, requester_id, designer_id, current_handler_id, owner_department_id, owner_team_id
		FROM tasks WHERE id = ?`, taskID).
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

type resourceGroupScanner interface{ Scan(...interface{}) error }

func scanTaskResourceGroup(scanner resourceGroupScanner) (*domain.TaskAssetGroup, error) {
	var item domain.TaskAssetGroup
	var skuID, retouchID, workingID, finalizedID sql.NullInt64
	if err := scanner.Scan(&item.ID, &item.TaskID, &item.ScopeKind, &skuID, &retouchID, &workingID, &finalizedID,
		&item.LockVersion, &item.MigrationIncomplete, &item.MigrationIssue, &item.CreatedAt, &item.UpdatedAt,
		&item.TaskNo, &item.SKUCode, &item.ProductName, &item.CreatorID, &item.CreatorName, &item.BusinessLane); err != nil {
		return nil, err
	}
	item.TaskSKUItemID = fromNullInt64(skuID)
	item.RetouchRequirementID = fromNullInt64(retouchID)
	item.WorkingRevisionID = fromNullInt64(workingID)
	item.FinalizedRevisionID = fromNullInt64(finalizedID)
	return &item, nil
}

func (r *TaskResourceGroupRepo) loadResourceGroupRevisions(ctx context.Context, revisionIDs []int64) (map[int64]*domain.TaskAssetGroupRevision, error) {
	revisionIDs = uniquePositiveInt64s(revisionIDs)
	groups := make([]domain.TaskAssetGroup, len(revisionIDs))
	for index, revisionID := range revisionIDs {
		id := revisionID
		groups[index].WorkingRevisionID = &id
	}
	if err := r.hydrateHistoricalResourceGroupRevisions(ctx, groups); err != nil {
		return nil, err
	}
	revisions := make(map[int64]*domain.TaskAssetGroupRevision, len(groups))
	for index, group := range groups {
		if group.WorkingRevision == nil {
			return nil, fmt.Errorf("%w: load resource group revisions: revision %d was not hydrated", repo.ErrDataIntegrity, revisionIDs[index])
		}
		revisions[group.WorkingRevision.ID] = group.WorkingRevision
	}
	return revisions, nil
}

func (r *TaskResourceGroupRepo) hydrateResourceGroupRevisions(ctx context.Context, groups []domain.TaskAssetGroup) error {
	return r.hydrateResourceGroupRevisionsWithPolicy(ctx, groups, false)
}

func (r *TaskResourceGroupRepo) hydrateHistoricalResourceGroupRevisions(ctx context.Context, groups []domain.TaskAssetGroup) error {
	return r.hydrateResourceGroupRevisionsWithPolicy(ctx, groups, true)
}

func (r *TaskResourceGroupRepo) hydrateResourceGroupRevisionsWithPolicy(ctx context.Context, groups []domain.TaskAssetGroup, allowMissingFiles bool) error {
	type revisionOwner struct {
		taskID               int64
		scopeKind            domain.TaskAssetGroupScopeKind
		taskSKUItemID        *int64
		retouchRequirementID *int64
	}
	type assetOwnership struct {
		taskID       int64
		assetType    domain.TaskAssetType
		boundGroupID *int64
		boundRole    *string
	}
	revisionIDs := make([]int64, 0, len(groups)*2)
	for index := range groups {
		if groups[index].WorkingRevisionID != nil {
			revisionIDs = append(revisionIDs, *groups[index].WorkingRevisionID)
		}
		if groups[index].FinalizedRevisionID != nil {
			revisionIDs = append(revisionIDs, *groups[index].FinalizedRevisionID)
		}
	}
	revisionIDs = uniquePositiveInt64s(revisionIDs)
	if len(revisionIDs) == 0 {
		return nil
	}
	revisionArgs := make([]interface{}, len(revisionIDs))
	for index, id := range revisionIDs {
		revisionArgs[index] = id
	}
	revisions := make(map[int64]*domain.TaskAssetGroupRevision, len(revisionIDs))
	revisionOwners := make(map[int64]revisionOwner, len(revisionIDs))
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT r.id, r.group_id, r.revision_no, r.status, r.mode, r.source_task_asset_id, r.source_stage,
		       r.created_by, COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username, ''), ''),
		       r.reason, r.submitted_at, r.finalized_at, r.created_at,
		       g.task_id, g.scope_kind, g.task_sku_item_id, g.retouch_requirement_id
		FROM task_asset_group_revisions r
		JOIN task_asset_groups g ON g.id = r.group_id
		LEFT JOIN users u ON u.id = r.created_by
		WHERE r.id IN (`+resourceGroupPlaceholders(len(revisionIDs))+`)`, revisionArgs...)
	if err != nil {
		return err
	}
	for rows.Next() {
		item := &domain.TaskAssetGroupRevision{
			Items:      []domain.TaskAssetGroupRevisionItem{},
			References: []domain.TaskAssetGroupRevisionReference{},
		}
		var sourceID sql.NullInt64
		var submittedAt, finalizedAt sql.NullTime
		var taskSKUItemID, retouchRequirementID sql.NullInt64
		var owner revisionOwner
		if err := rows.Scan(&item.ID, &item.GroupID, &item.RevisionNo, &item.Status, &item.Mode, &sourceID, &item.SourceStage,
			&item.CreatedBy, &item.CreatedByName, &item.Reason, &submittedAt, &finalizedAt, &item.CreatedAt,
			&owner.taskID, &owner.scopeKind, &taskSKUItemID, &retouchRequirementID); err != nil {
			rows.Close()
			return err
		}
		owner.taskSKUItemID = fromNullInt64(taskSKUItemID)
		owner.retouchRequirementID = fromNullInt64(retouchRequirementID)
		item.SourceTaskAssetID = fromNullInt64(sourceID)
		item.SubmittedAt = fromNullTime(submittedAt)
		item.FinalizedAt = fromNullTime(finalizedAt)
		revisions[item.ID] = item
		revisionOwners[item.ID] = owner
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(revisions) != len(revisionIDs) {
		return fmt.Errorf("%w: hydrate resource group revisions: expected %d rows, got %d", repo.ErrDataIntegrity, len(revisionIDs), len(revisions))
	}
	assetIDs := make([]int64, 0, len(revisions)*2)
	itemRows, err := r.db.db.QueryContext(ctx, `
		SELECT id, revision_id, task_asset_id, sort_order, item_name, created_at
		FROM task_asset_group_revision_items
		WHERE revision_id IN (`+resourceGroupPlaceholders(len(revisionIDs))+`)
		ORDER BY revision_id, sort_order, id`, revisionArgs...)
	if err != nil {
		return err
	}
	for itemRows.Next() {
		var child domain.TaskAssetGroupRevisionItem
		if err := itemRows.Scan(&child.ID, &child.RevisionID, &child.TaskAssetID, &child.SortOrder, &child.ItemName, &child.CreatedAt); err != nil {
			itemRows.Close()
			return err
		}
		if revision := revisions[child.RevisionID]; revision != nil {
			revision.Items = append(revision.Items, child)
			assetIDs = append(assetIDs, child.TaskAssetID)
		}
	}
	if err := itemRows.Err(); err != nil {
		itemRows.Close()
		return err
	}
	if err := itemRows.Close(); err != nil {
		return err
	}
	refRows, err := r.db.db.QueryContext(ctx, `
		SELECT rr.id, rr.revision_id, rr.reference_file_ref_id, rr.formal_task_asset_id, rr.sort_order,
		       rr.ref_id_snapshot, rr.file_name_snapshot, rr.scope_snapshot,
		       COALESCE(asr.mime_type, ''), asr.file_size,
		       CASE WHEN COALESCE(asr.is_placeholder, 1) = 0 THEN COALESCE(asr.ref_key, '') ELSE '' END,
		       COALESCE(asr.status, ''),
		       COALESCE(formal_asr.status, ''),
		       CASE
		         WHEN rr.formal_task_asset_id IS NULL THEN 1
		         WHEN formal_ta.id IS NOT NULL
		              AND formal_ta.binding_state = 'bound'
		              AND COALESCE(formal_ta.is_archived, 0) = 0
		              AND formal_ta.deleted_at IS NULL
		              AND formal_ta.cleaned_at IS NULL
		              AND formal_ta.access_revoked_at IS NULL
		              AND formal_ta.object_deleted_at IS NULL
		              AND formal_ta.storage_ref_id IS NOT NULL
		              AND formal_asr.ref_id IS NOT NULL
		              AND COALESCE(formal_asr.status, '') NOT IN ('archived', 'historical_unavailable')
		           THEN 1
		         ELSE 0
		       END,
		       rr.created_at,
		       f.task_id, f.sku_item_id, f.retouch_requirement_id, f.ref_id,
		       formal_ta.task_id, formal_ta.binding_state, formal_ta.asset_type,
		       formal_ta.bound_role, formal_ta.storage_ref_id, asr.ref_id
		FROM task_asset_group_revision_references rr
		LEFT JOIN reference_file_refs f ON f.id = rr.reference_file_ref_id
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = rr.ref_id_snapshot
		LEFT JOIN task_assets formal_ta ON formal_ta.id = rr.formal_task_asset_id
		LEFT JOIN asset_storage_refs formal_asr ON formal_asr.ref_id = formal_ta.storage_ref_id
		WHERE rr.revision_id IN (`+resourceGroupPlaceholders(len(revisionIDs))+`)
		ORDER BY rr.revision_id, rr.sort_order, rr.id`, revisionArgs...)
	if err != nil {
		return err
	}
	unavailableReferenceCount := 0
	for refRows.Next() {
		var child domain.TaskAssetGroupRevisionReference
		var formalID sql.NullInt64
		var fileSize sql.NullInt64
		var snapshotStatus, formalStatus sql.NullString
		var formalTaskAssetActive bool
		var referenceTaskID sql.NullInt64
		var referenceSKUItemID, referenceRetouchRequirementID sql.NullInt64
		var referenceRefID sql.NullString
		var formalTaskID sql.NullInt64
		var formalBindingState, formalAssetType, formalBoundRole sql.NullString
		var formalStorageRefID, snapshotStorageRefID sql.NullString
		if err := refRows.Scan(&child.ID, &child.RevisionID, &child.ReferenceFileRefID, &formalID, &child.SortOrder,
			&child.RefIDSnapshot, &child.FileNameSnapshot, &child.ScopeSnapshot, &child.MimeType, &fileSize,
			&child.StorageKey, &snapshotStatus, &formalStatus, &formalTaskAssetActive, &child.CreatedAt,
			&referenceTaskID, &referenceSKUItemID, &referenceRetouchRequirementID, &referenceRefID,
			&formalTaskID, &formalBindingState, &formalAssetType, &formalBoundRole,
			&formalStorageRefID, &snapshotStorageRefID); err != nil {
			refRows.Close()
			return err
		}
		child.FormalTaskAssetID = fromNullInt64(formalID)
		child.FileSize = fromNullInt64(fileSize)
		owner, ownerExists := revisionOwners[child.RevisionID]
		referenceScopeMatches := !referenceSKUItemID.Valid && !referenceRetouchRequirementID.Valid
		referenceScopeSnapshot := "task"
		if !referenceScopeMatches {
			switch owner.scopeKind {
			case domain.TaskAssetGroupScopeSKU:
				referenceScopeMatches = owner.taskSKUItemID != nil &&
					referenceSKUItemID.Valid &&
					!referenceRetouchRequirementID.Valid &&
					*owner.taskSKUItemID == referenceSKUItemID.Int64
				referenceScopeSnapshot = fmt.Sprintf("sku:%d", referenceSKUItemID.Int64)
			case domain.TaskAssetGroupScopeRetouch:
				referenceScopeMatches = owner.retouchRequirementID != nil &&
					!referenceSKUItemID.Valid &&
					referenceRetouchRequirementID.Valid &&
					*owner.retouchRequirementID == referenceRetouchRequirementID.Int64
				referenceScopeSnapshot = fmt.Sprintf("retouch_requirement:%d", referenceRetouchRequirementID.Int64)
			}
		}
		if !ownerExists ||
			!referenceTaskID.Valid ||
			referenceTaskID.Int64 != owner.taskID ||
			!referenceScopeMatches ||
			!referenceRefID.Valid ||
			child.RefIDSnapshot != referenceRefID.String ||
			child.ScopeSnapshot != referenceScopeSnapshot ||
			!snapshotStorageRefID.Valid ||
			snapshotStorageRefID.String != child.RefIDSnapshot ||
			(child.FormalTaskAssetID != nil &&
				(!formalTaskID.Valid ||
					formalTaskID.Int64 != owner.taskID ||
					!formalBindingState.Valid ||
					formalBindingState.String != "bound" ||
					!formalAssetType.Valid ||
					domain.NormalizeTaskAssetType(domain.TaskAssetType(formalAssetType.String)) != domain.TaskAssetTypeReference ||
					(formalBoundRole.Valid && strings.TrimSpace(formalBoundRole.String) != "") ||
					!formalStorageRefID.Valid ||
					formalStorageRefID.String != child.RefIDSnapshot)) {
			refRows.Close()
			return fmt.Errorf(
				"%w: resource group reference %d is outside revision %d ownership",
				repo.ErrDataIntegrity, child.ID, child.RevisionID,
			)
		}
		child.Availability = domain.TaskResourceFileAvailable
		snapshotUnavailable := domain.AssetStorageRefStatus(snapshotStatus.String) == domain.AssetStorageRefStatusArchived ||
			domain.AssetStorageRefStatus(snapshotStatus.String) == domain.AssetStorageRefStatusHistoricalUnavailable
		formalUnavailable := child.FormalTaskAssetID != nil &&
			(!formalTaskAssetActive ||
				domain.AssetStorageRefStatus(formalStatus.String) == domain.AssetStorageRefStatusArchived ||
				domain.AssetStorageRefStatus(formalStatus.String) == domain.AssetStorageRefStatusHistoricalUnavailable)
		if snapshotUnavailable || formalUnavailable {
			child.Availability = domain.TaskResourceFileHistoricalUnavailable
			child.UnavailableReason = "legacy_original_object_missing"
			child.StorageKey = ""
			unavailableReferenceCount++
		}
		if revision := revisions[child.RevisionID]; revision != nil {
			revision.References = append(revision.References, child)
		}
	}
	if err := refRows.Err(); err != nil {
		refRows.Close()
		return err
	}
	if err := refRows.Close(); err != nil {
		return err
	}
	if !allowMissingFiles && unavailableReferenceCount > 0 {
		return fmt.Errorf("%w: hydrate current resource group references: expected every reference to be available, got %d historical-unavailable rows", repo.ErrDataIntegrity, unavailableReferenceCount)
	}
	for _, revision := range revisions {
		if revision.SourceTaskAssetID != nil {
			assetIDs = append(assetIDs, *revision.SourceTaskAssetID)
		}
	}
	assetIDs = uniquePositiveInt64s(assetIDs)
	files := make(map[int64]*domain.TaskResourceFile, len(assetIDs))
	assetOwnerships := make(map[int64]assetOwnership, len(assetIDs))
	unavailableFileCount := 0
	if len(assetIDs) > 0 {
		assetArgs := make([]interface{}, len(assetIDs))
		for index, id := range assetIDs {
			assetArgs[index] = id
		}
		fileRows, err := r.db.db.QueryContext(ctx, `
			SELECT ta.id, ta.file_name, ta.mime_type, ta.file_size,
			       COALESCE(NULLIF(ta.storage_key, ''), NULLIF(asr.ref_key, '')),
			       COALESCE(asr.status, ''),
			       CASE
			         WHEN COALESCE(ta.is_archived, 0) = 0
			              AND ta.deleted_at IS NULL
			              AND ta.cleaned_at IS NULL
			              AND ta.access_revoked_at IS NULL
			              AND ta.object_deleted_at IS NULL
			              AND ta.storage_ref_id IS NOT NULL
			              AND asr.ref_id IS NOT NULL
			              AND COALESCE(asr.status, '') NOT IN ('archived', 'historical_unavailable')
			           THEN 1
			         ELSE 0
			       END,
			       ta.task_id, ta.asset_type, ta.bound_group_id, ta.bound_role
			FROM task_assets ta
			LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
			WHERE ta.id IN (`+resourceGroupPlaceholders(len(assetIDs))+`)
			  AND ta.binding_state = 'bound'`, assetArgs...)
		if err != nil {
			return err
		}
		for fileRows.Next() {
			file := &domain.TaskResourceFile{}
			var mimeType, storageKey, storageRefStatus sql.NullString
			var fileSize sql.NullInt64
			var owner assetOwnership
			var boundGroupID sql.NullInt64
			var boundRole sql.NullString
			var fileActive bool
			if err := fileRows.Scan(&file.TaskAssetID, &file.FileName, &mimeType, &fileSize, &storageKey, &storageRefStatus, &fileActive,
				&owner.taskID, &owner.assetType, &boundGroupID, &boundRole); err != nil {
				fileRows.Close()
				return err
			}
			owner.boundGroupID = fromNullInt64(boundGroupID)
			if boundRole.Valid {
				role := boundRole.String
				owner.boundRole = &role
			}
			file.MimeType = mimeType.String
			file.FileSize = fromNullInt64(fileSize)
			file.StorageKey = storageKey.String
			file.Availability = domain.TaskResourceFileAvailable
			if !fileActive ||
				domain.AssetStorageRefStatus(storageRefStatus.String) == domain.AssetStorageRefStatusArchived ||
				domain.AssetStorageRefStatus(storageRefStatus.String) == domain.AssetStorageRefStatusHistoricalUnavailable {
				file.Availability = domain.TaskResourceFileHistoricalUnavailable
				file.UnavailableReason = "legacy_original_object_missing"
				file.StorageKey = ""
				unavailableFileCount++
			}
			files[file.TaskAssetID] = file
			assetOwnerships[file.TaskAssetID] = owner
		}
		if err := fileRows.Err(); err != nil {
			fileRows.Close()
			return err
		}
		if err := fileRows.Close(); err != nil {
			return err
		}
		if len(files) != len(assetIDs) {
			return fmt.Errorf("%w: hydrate resource group files: expected %d explicit active bound rows, got %d", repo.ErrDataIntegrity, len(assetIDs), len(files))
		}
		if !allowMissingFiles && unavailableFileCount > 0 {
			return fmt.Errorf("%w: hydrate current resource group files: expected all %d files to be available, got %d historical-unavailable rows", repo.ErrDataIntegrity, len(assetIDs), unavailableFileCount)
		}
		// Historical pages tolerate only explicit historical-unavailable
		// tombstones. Any absent/inactive row remains a fail-closed integrity
		// error, and current working/finalized hydration rejects tombstones.
	}
	for _, revision := range revisions {
		owner, ownerExists := revisionOwners[revision.ID]
		if revision.SourceTaskAssetID != nil {
			assetOwner := assetOwnerships[*revision.SourceTaskAssetID]
			if !ownerExists ||
				assetOwner.taskID != owner.taskID ||
				assetOwner.assetType != domain.TaskAssetTypeSource ||
				assetOwner.boundGroupID == nil ||
				*assetOwner.boundGroupID != revision.GroupID ||
				assetOwner.boundRole == nil ||
				*assetOwner.boundRole != "source" {
				return fmt.Errorf(
					"%w: resource group revision %d source asset %d has invalid ownership",
					repo.ErrDataIntegrity, revision.ID, *revision.SourceTaskAssetID,
				)
			}
			revision.SourceFile = files[*revision.SourceTaskAssetID]
		}
		for index := range revision.Items {
			assetOwner := assetOwnerships[revision.Items[index].TaskAssetID]
			if !ownerExists ||
				assetOwner.taskID != owner.taskID ||
				assetOwner.assetType != domain.TaskAssetTypeDelivery ||
				assetOwner.boundGroupID == nil ||
				*assetOwner.boundGroupID != revision.GroupID ||
				assetOwner.boundRole == nil ||
				*assetOwner.boundRole != "final" {
				return fmt.Errorf(
					"%w: resource group revision %d final asset %d has invalid ownership",
					repo.ErrDataIntegrity, revision.ID, revision.Items[index].TaskAssetID,
				)
			}
			revision.Items[index].File = files[revision.Items[index].TaskAssetID]
		}
	}
	for index := range groups {
		if groups[index].WorkingRevisionID != nil {
			groups[index].WorkingRevision = revisions[*groups[index].WorkingRevisionID]
		}
		if groups[index].FinalizedRevisionID != nil {
			groups[index].FinalizedRevision = revisions[*groups[index].FinalizedRevisionID]
		}
	}
	return nil
}

func (r *TaskResourceGroupRepo) getResourceFile(ctx context.Context, taskAssetID int64) (*domain.TaskResourceFile, error) {
	var file domain.TaskResourceFile
	var mimeType, storageKey sql.NullString
	var fileSize sql.NullInt64
	err := r.db.db.QueryRowContext(ctx, `
		SELECT ta.id, ta.file_name, ta.mime_type, ta.file_size,
		       COALESCE(NULLIF(ta.storage_key, ''), NULLIF(asr.ref_key, ''))
		FROM task_assets ta
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		WHERE ta.id = ?
		  AND ta.binding_state = 'bound'
		  AND ta.deleted_at IS NULL
		  AND ta.cleaned_at IS NULL
		  AND ta.access_revoked_at IS NULL
		  AND ta.object_deleted_at IS NULL`, taskAssetID).
		Scan(&file.TaskAssetID, &file.FileName, &mimeType, &fileSize, &storageKey)
	if err != nil {
		return nil, err
	}
	file.MimeType = mimeType.String
	file.FileSize = fromNullInt64(fileSize)
	file.StorageKey = storageKey.String
	return &file, nil
}

func (r *TaskResourceGroupRepo) LockGroup(ctx context.Context, tx repo.Tx, taskID, groupID, expectedVersion int64) (*domain.TaskAssetGroup, error) {
	var item domain.TaskAssetGroup
	var skuID, retouchID, workingID, finalizedID sql.NullInt64
	err := Unwrap(tx).QueryRowContext(ctx, `
		SELECT id, task_id, scope_kind, task_sku_item_id, retouch_requirement_id,
		       working_revision_id, finalized_revision_id, lock_version, migration_incomplete, migration_issue,
		       created_at, updated_at
		FROM task_asset_groups WHERE id = ? AND task_id = ? FOR UPDATE`, groupID, taskID).
		Scan(&item.ID, &item.TaskID, &item.ScopeKind, &skuID, &retouchID, &workingID, &finalizedID,
			&item.LockVersion, &item.MigrationIncomplete, &item.MigrationIssue, &item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if item.LockVersion != expectedVersion {
		return nil, repo.ErrConflict
	}
	item.TaskSKUItemID = fromNullInt64(skuID)
	item.RetouchRequirementID = fromNullInt64(retouchID)
	item.WorkingRevisionID = fromNullInt64(workingID)
	item.FinalizedRevisionID = fromNullInt64(finalizedID)
	return &item, nil
}

func (r *TaskResourceGroupRepo) ListGroupsForUpdate(ctx context.Context, tx repo.Tx, taskID int64) ([]domain.TaskAssetGroup, error) {
	rows, err := Unwrap(tx).QueryContext(ctx, `
		SELECT id, task_id, scope_kind, task_sku_item_id, retouch_requirement_id,
		       working_revision_id, finalized_revision_id, lock_version, migration_incomplete, migration_issue,
		       created_at, updated_at
		FROM task_asset_groups WHERE task_id = ? ORDER BY scope_kind, scope_ref_id, id FOR UPDATE`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.TaskAssetGroup, 0)
	for rows.Next() {
		var item domain.TaskAssetGroup
		var skuID, retouchID, workingID, finalizedID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.TaskID, &item.ScopeKind, &skuID, &retouchID, &workingID, &finalizedID,
			&item.LockVersion, &item.MigrationIncomplete, &item.MigrationIssue, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.TaskSKUItemID = fromNullInt64(skuID)
		item.RetouchRequirementID = fromNullInt64(retouchID)
		item.WorkingRevisionID = fromNullInt64(workingID)
		item.FinalizedRevisionID = fromNullInt64(finalizedID)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *TaskResourceGroupRepo) GetRevisionForUpdate(ctx context.Context, tx repo.Tx, revisionID int64) (*domain.TaskAssetGroupRevision, error) {
	var item domain.TaskAssetGroupRevision
	var sourceID sql.NullInt64
	var submittedAt, finalizedAt sql.NullTime
	err := Unwrap(tx).QueryRowContext(ctx, `
		SELECT id, group_id, revision_no, status, mode, source_task_asset_id, source_stage,
		       created_by, reason, submitted_at, finalized_at, created_at
		FROM task_asset_group_revisions WHERE id = ? FOR UPDATE`, revisionID).
		Scan(&item.ID, &item.GroupID, &item.RevisionNo, &item.Status, &item.Mode, &sourceID, &item.SourceStage,
			&item.CreatedBy, &item.Reason, &submittedAt, &finalizedAt, &item.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.SourceTaskAssetID = fromNullInt64(sourceID)
	item.SubmittedAt = fromNullTime(submittedAt)
	item.FinalizedAt = fromNullTime(finalizedAt)
	rows, err := Unwrap(tx).QueryContext(ctx, `
		SELECT id, revision_id, task_asset_id, sort_order, item_name, created_at
		FROM task_asset_group_revision_items WHERE revision_id = ? ORDER BY sort_order, id FOR UPDATE`, revisionID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var child domain.TaskAssetGroupRevisionItem
		if err := rows.Scan(&child.ID, &child.RevisionID, &child.TaskAssetID, &child.SortOrder, &child.ItemName, &child.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		item.Items = append(item.Items, child)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *TaskResourceGroupRepo) ListStagedAssetsForUpdate(ctx context.Context, tx repo.Tx, ids []int64) (map[int64]domain.StagedTaskAssetBinding, error) {
	ids = uniquePositiveInt64s(ids)
	if len(ids) == 0 {
		return map[int64]domain.StagedTaskAssetBinding{}, nil
	}
	args := make([]interface{}, len(ids))
	marks := make([]string, len(ids))
	for i, id := range ids {
		args[i], marks[i] = id, "?"
	}
	rows, err := Unwrap(tx).QueryContext(ctx, `
		SELECT id, task_id, binding_state, bound_group_id, bound_role, staged_task_sku_item_id, staged_retouch_requirement_id,
		       staged_role, staged_by, upload_status, COALESCE(storage_key, ''), access_revoked_at
		FROM task_assets WHERE id IN (`+strings.Join(marks, ",")+`) FOR UPDATE`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]domain.StagedTaskAssetBinding, len(ids))
	for rows.Next() {
		var item domain.StagedTaskAssetBinding
		var boundGroupID, skuID, retouchID, stagedBy sql.NullInt64
		var boundRole, role, uploadStatus sql.NullString
		var revokedAt sql.NullTime
		if err := rows.Scan(&item.TaskAssetID, &item.TaskID, &item.BindingState, &boundGroupID, &boundRole, &skuID, &retouchID,
			&role, &stagedBy, &uploadStatus, &item.StorageKey, &revokedAt); err != nil {
			return nil, err
		}
		item.BoundGroupID = fromNullInt64(boundGroupID)
		item.BoundRole = boundRole.String
		item.StagedTaskSKUItemID = fromNullInt64(skuID)
		item.StagedRetouchID = fromNullInt64(retouchID)
		item.StagedBy = fromNullInt64(stagedBy)
		item.StagedRole = role.String
		item.UploadStatus = uploadStatus.String
		item.AccessRevoked = revokedAt.Valid
		out[item.TaskAssetID] = item
	}
	return out, rows.Err()
}

func (r *TaskResourceGroupRepo) CreateRevision(ctx context.Context, tx repo.Tx, group domain.TaskAssetGroup, input domain.SubmitResourceGroupInput, status domain.TaskAssetGroupRevisionStatus, stage domain.TaskAssetSourceStage, actorID int64, reason string) (int64, error) {
	sqlTx := Unwrap(tx)
	if group.WorkingRevisionID != nil {
		if _, err := sqlTx.ExecContext(ctx, `UPDATE task_asset_group_revisions SET status = 'superseded' WHERE id = ? AND status IN ('draft','submitted')`, *group.WorkingRevisionID); err != nil {
			return 0, err
		}
	}
	var revisionNo int
	if err := sqlTx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision_no), 0) + 1 FROM task_asset_group_revisions WHERE group_id = ? FOR UPDATE`, group.ID).Scan(&revisionNo); err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	var submittedAt interface{}
	if status == domain.TaskAssetGroupRevisionSubmitted {
		submittedAt = now
	}
	result, err := sqlTx.ExecContext(ctx, `
		INSERT INTO task_asset_group_revisions
		  (group_id, revision_no, status, mode, source_task_asset_id, source_stage, created_by, reason, submitted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, group.ID, revisionNo, string(status), string(input.Mode),
		toNullInt64(input.SourceTaskAssetID), string(stage), actorID, strings.TrimSpace(reason), submittedAt)
	if err != nil {
		return 0, err
	}
	revisionID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	for order, assetID := range input.FinalTaskAssetIDs {
		if _, err := sqlTx.ExecContext(ctx, `
			INSERT INTO task_asset_group_revision_items (revision_id, task_asset_id, sort_order)
			VALUES (?, ?, ?)`, revisionID, assetID, order); err != nil {
			return 0, err
		}
	}
	for order, referenceID := range uniquePositiveInt64s(input.ReferenceFileRefIDs) {
		referenceScopeClause := `rfr.sku_item_id IS NULL AND rfr.retouch_requirement_id IS NULL`
		referenceScopeArgs := []interface{}{}
		switch group.ScopeKind {
		case domain.TaskAssetGroupScopeSKU:
			if group.TaskSKUItemID == nil {
				return 0, repo.ErrConflict
			}
			referenceScopeClause = `rfr.retouch_requirement_id IS NULL AND (rfr.sku_item_id IS NULL OR rfr.sku_item_id = ?)`
			referenceScopeArgs = append(referenceScopeArgs, *group.TaskSKUItemID)
		case domain.TaskAssetGroupScopeRetouch:
			if group.RetouchRequirementID == nil {
				return 0, repo.ErrConflict
			}
			referenceScopeClause = `rfr.sku_item_id IS NULL AND (rfr.retouch_requirement_id IS NULL OR rfr.retouch_requirement_id = ?)`
			referenceScopeArgs = append(referenceScopeArgs, *group.RetouchRequirementID)
		case domain.TaskAssetGroupScopeTask:
		default:
			return 0, repo.ErrConflict
		}
		args := []interface{}{revisionID, order, referenceID, group.TaskID}
		args = append(args, referenceScopeArgs...)
		result, err := sqlTx.ExecContext(ctx, `
			INSERT INTO task_asset_group_revision_references
			  (revision_id, reference_file_ref_id, sort_order, ref_id_snapshot, file_name_snapshot, scope_snapshot)
			SELECT ?, rfr.id, ?, rfr.ref_id, COALESCE(asr.file_name, ''),
			       CASE
			         WHEN rfr.retouch_requirement_id IS NOT NULL THEN CONCAT('retouch_requirement:', rfr.retouch_requirement_id)
			         WHEN rfr.sku_item_id IS NOT NULL THEN CONCAT('sku:', rfr.sku_item_id)
			         ELSE 'task'
			       END
			FROM reference_file_refs rfr
			LEFT JOIN asset_storage_refs asr ON asr.ref_id = rfr.ref_id
			WHERE rfr.id = ? AND rfr.task_id = ?
			  AND (`+referenceScopeClause+`)`, args...)
		if err != nil {
			return 0, err
		}
		if rows, _ := result.RowsAffected(); rows != 1 {
			return 0, repo.ErrConflict
		}
	}
	assetIDs := append([]int64{}, input.FinalTaskAssetIDs...)
	if input.SourceTaskAssetID != nil {
		assetIDs = append(assetIDs, *input.SourceTaskAssetID)
	}
	for _, assetID := range assetIDs {
		role := "final"
		if input.SourceTaskAssetID != nil && assetID == *input.SourceTaskAssetID {
			role = "source"
		}
		result, err := sqlTx.ExecContext(ctx, `
			UPDATE task_assets
			SET binding_state = 'bound', bound_group_id = ?, bound_role = ?,
			    staged_expires_at = NULL
			WHERE id = ? AND task_id = ? AND binding_state = 'staged'`, group.ID, role, assetID, group.TaskID)
		if err != nil {
			return 0, err
		}
		if rows, _ := result.RowsAffected(); rows != 1 {
			var inherited int
			if err := sqlTx.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM task_assets
				WHERE id = ? AND task_id = ? AND binding_state = 'bound' AND bound_group_id = ? AND bound_role = ?`,
				assetID, group.TaskID, group.ID, role).Scan(&inherited); err != nil || inherited != 1 {
				return 0, repo.ErrConflict
			}
		}
	}
	result, err = sqlTx.ExecContext(ctx, `
		UPDATE task_asset_groups SET working_revision_id = ?, lock_version = lock_version + 1
		WHERE id = ? AND lock_version = ?`, revisionID, group.ID, group.LockVersion)
	if err != nil {
		return 0, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return 0, repo.ErrConflict
	}
	return revisionID, nil
}

func (r *TaskResourceGroupRepo) FinalizeGroup(ctx context.Context, tx repo.Tx, groupID, revisionID, expectedLockVersion, actorID int64) error {
	sqlTx := Unwrap(tx)
	var previousRevisionID, previousSourceID, nextSourceID sql.NullInt64
	if err := sqlTx.QueryRowContext(ctx, `
		SELECT g.finalized_revision_id,
		       CASE
		         WHEN g.finalized_revision_id IS NOT NULL THEN previous.source_task_asset_id
		         ELSE (
		           SELECT predecessor.source_task_asset_id
		           FROM task_asset_group_revisions predecessor
		           WHERE predecessor.group_id = g.id
		             AND predecessor.revision_no < next_revision.revision_no
		           ORDER BY predecessor.revision_no DESC
		           LIMIT 1
		         )
		       END AS previous_source_task_asset_id,
		       next_revision.source_task_asset_id
		FROM task_asset_groups g
		JOIN task_asset_group_revisions next_revision ON next_revision.id = ? AND next_revision.group_id = g.id
		LEFT JOIN task_asset_group_revisions previous ON previous.id = g.finalized_revision_id
		WHERE g.id = ? AND g.lock_version = ? FOR UPDATE`, revisionID, groupID, expectedLockVersion).
		Scan(&previousRevisionID, &previousSourceID, &nextSourceID); err != nil {
		if err == sql.ErrNoRows {
			return repo.ErrConflict
		}
		return err
	}
	now := time.Now().UTC()
	revisionResult, err := sqlTx.ExecContext(ctx, `UPDATE task_asset_group_revisions SET status = 'finalized', finalized_at = ? WHERE id = ? AND status = 'submitted'`, now, revisionID)
	if err != nil {
		return err
	}
	if rows, _ := revisionResult.RowsAffected(); rows != 1 {
		return repo.ErrConflict
	}
	if previousRevisionID.Valid && previousRevisionID.Int64 != revisionID {
		if _, err := sqlTx.ExecContext(ctx, `UPDATE task_asset_group_revisions SET status = 'superseded' WHERE id = ?`, previousRevisionID.Int64); err != nil {
			return err
		}
	}
	result, err := sqlTx.ExecContext(ctx, `
		UPDATE task_asset_groups
		SET finalized_revision_id = ?, working_revision_id = ?, lock_version = lock_version + 1
		WHERE id = ? AND lock_version = ?`, revisionID, revisionID, groupID, expectedLockVersion)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return repo.ErrConflict
	}
	if previousSourceID.Valid && (!nextSourceID.Valid || previousSourceID.Int64 != nextSourceID.Int64) {
		if _, err := sqlTx.ExecContext(ctx, `
			UPDATE task_assets SET access_revoked_at = ?, access_revoked_reason = 'source_replaced_by_audit'
			WHERE id = ?`, now, previousSourceID.Int64); err != nil {
			return err
		}
		if err := enqueueTaskAssetObjectDeletions(ctx, sqlTx, []int64{previousSourceID.Int64}); err != nil {
			return err
		}
	}
	if err := reindexTaskAssetGroupSearchDocument(ctx, sqlTx, groupID); err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO search_reindex_outbox (entity_type, entity_id, dedupe_key)
		VALUES ('task_resource_group', ?, ?)
		ON DUPLICATE KEY UPDATE status = 'pending', next_retry_at = NULL`, groupID, fmt.Sprintf("task_resource_group:%d:%d", groupID, revisionID))
	return err
}

func reindexTaskAssetGroupSearchDocument(ctx context.Context, q taskSearchDocumentSQL, groupID int64) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO task_asset_group_search_documents (
		  group_id, task_id, finalized_revision_id, internal_text, final_text
		)
		SELECT g.id, g.task_id, g.finalized_revision_id,
		       CONCAT_WS(' ',
		         t.id, t.task_no, t.sku_code, t.primary_sku_code, t.product_name_snapshot,
		         COALESCE(tsi.sku_code, ''),
		         COALESCE(source.file_name, ''),
		         COALESCE((SELECT GROUP_CONCAT(r.file_name_snapshot ORDER BY r.sort_order SEPARATOR ' ')
		                   FROM task_asset_group_revision_references r WHERE r.revision_id = revision.id), ''),
		         COALESCE((SELECT GROUP_CONCAT(ta.file_name ORDER BY ri.sort_order SEPARATOR ' ')
		                   FROM task_asset_group_revision_items ri
		                   JOIN task_assets ta ON ta.id = ri.task_asset_id
		                   WHERE ri.revision_id = revision.id), '')
		       ),
		       CONCAT_WS(' ',
		         t.id, t.task_no, t.sku_code, t.primary_sku_code, t.product_name_snapshot,
		         COALESCE(tsi.sku_code, ''),
		         COALESCE((SELECT GROUP_CONCAT(ta.file_name ORDER BY ri.sort_order SEPARATOR ' ')
		                   FROM task_asset_group_revision_items ri
		                   JOIN task_assets ta ON ta.id = ri.task_asset_id
		                   WHERE ri.revision_id = revision.id), '')
		       )
		FROM task_asset_groups g
		JOIN tasks t ON t.id = g.task_id
		JOIN task_asset_group_revisions revision ON revision.id = g.finalized_revision_id
		LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
		LEFT JOIN task_assets source ON source.id = revision.source_task_asset_id
		WHERE g.id = ?
		ON DUPLICATE KEY UPDATE
		  task_id = VALUES(task_id),
		  finalized_revision_id = VALUES(finalized_revision_id),
		  internal_text = VALUES(internal_text),
		  final_text = VALUES(final_text)`, groupID)
	if err != nil {
		return fmt.Errorf("reindex task resource group search document: %w", err)
	}
	return nil
}

func (r *TaskResourceGroupRepo) CloneRevision(ctx context.Context, tx repo.Tx, group domain.TaskAssetGroup, sourceRevisionID int64, status domain.TaskAssetGroupRevisionStatus, stage domain.TaskAssetSourceStage, actorID int64, reason string) (int64, error) {
	sqlTx := Unwrap(tx)
	var revisionNo int
	if err := sqlTx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision_no), 0) + 1 FROM task_asset_group_revisions WHERE group_id = ? FOR UPDATE`, group.ID).Scan(&revisionNo); err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	var submittedAt interface{}
	if status == domain.TaskAssetGroupRevisionSubmitted {
		submittedAt = now
	}
	result, err := sqlTx.ExecContext(ctx, `
		INSERT INTO task_asset_group_revisions
		  (group_id, revision_no, status, mode, source_task_asset_id, source_stage, created_by, reason, submitted_at)
		SELECT group_id, ?, ?, mode, source_task_asset_id, ?, ?, ?, ?
		FROM task_asset_group_revisions WHERE id = ? AND group_id = ?`,
		revisionNo, string(status), string(stage), actorID, strings.TrimSpace(reason), submittedAt, sourceRevisionID, group.ID)
	if err != nil {
		return 0, err
	}
	newID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := sqlTx.ExecContext(ctx, `
		INSERT INTO task_asset_group_revision_items (revision_id, task_asset_id, sort_order, item_name)
		SELECT ?, task_asset_id, sort_order, item_name FROM task_asset_group_revision_items WHERE revision_id = ?`, newID, sourceRevisionID); err != nil {
		return 0, err
	}
	if _, err := sqlTx.ExecContext(ctx, `
		INSERT INTO task_asset_group_revision_references
		  (revision_id, reference_file_ref_id, formal_task_asset_id, sort_order, ref_id_snapshot, file_name_snapshot, scope_snapshot)
		SELECT ?, reference_file_ref_id, formal_task_asset_id, sort_order, ref_id_snapshot, file_name_snapshot, scope_snapshot
		FROM task_asset_group_revision_references WHERE revision_id = ?`, newID, sourceRevisionID); err != nil {
		return 0, err
	}
	result, err = sqlTx.ExecContext(ctx, `
		UPDATE task_asset_groups SET working_revision_id = ?, lock_version = lock_version + 1
		WHERE id = ? AND lock_version = ?`, newID, group.ID, group.LockVersion)
	if err != nil {
		return 0, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return 0, repo.ErrConflict
	}
	return newID, nil
}

func (r *TaskResourceGroupRepo) MarkWorkingRejected(ctx context.Context, tx repo.Tx, revisionID int64) error {
	_, err := Unwrap(tx).ExecContext(ctx, `UPDATE task_asset_group_revisions SET status = 'rejected' WHERE id = ? AND status = 'submitted'`, revisionID)
	return err
}

func (r *TaskResourceGroupRepo) CASTaskStatus(ctx context.Context, tx repo.Tx, taskID, expectedRevision int64, expectedStatus, nextStatus domain.TaskStatus, clearHandler bool) (int64, error) {
	handlerSQL := ""
	if clearHandler {
		handlerSQL = ", current_handler_id = NULL"
	}
	result, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE tasks SET task_status = ?, workflow_revision = workflow_revision + 1`+handlerSQL+`
		WHERE id = ? AND task_status = ? AND workflow_revision = ?`, string(nextStatus), taskID, string(expectedStatus), expectedRevision)
	if err != nil {
		return 0, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return 0, repo.ErrConflict
	}
	return expectedRevision + 1, nil
}

func (r *TaskResourceGroupRepo) RestoreDesignerHandler(ctx context.Context, tx repo.Tx, taskID int64) error {
	_, err := Unwrap(tx).ExecContext(ctx, `UPDATE tasks SET current_handler_id = designer_id WHERE id = ?`, taskID)
	return err
}

func (r *TaskResourceGroupRepo) RequireCustomizationReadyForSubmit(ctx context.Context, tx repo.Tx, taskID int64) error {
	var moduleState domain.ModuleState
	var jobStatus domain.CustomizationJobStatus
	err := Unwrap(tx).QueryRowContext(ctx, `
		SELECT tm.state, cj.status
		FROM task_modules tm
		JOIN customization_jobs cj ON cj.task_id = tm.task_id
		WHERE tm.task_id = ? AND tm.module_key = 'customization'
		ORDER BY cj.id DESC
		LIMIT 1
		FOR UPDATE`, taskID).Scan(&moduleState, &jobStatus)
	if err == sql.ErrNoRows {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "定制任务尚未完成设计准备", map[string]interface{}{"required_status": domain.CustomizationJobStatusReadyForSubmit})
	}
	if err != nil {
		return err
	}
	if moduleState != domain.ModuleStateSubmitted || jobStatus != domain.CustomizationJobStatusReadyForSubmit {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "定制任务尚未完成设计准备", map[string]interface{}{
			"module_state":    moduleState,
			"job_status":      jobStatus,
			"required_status": domain.CustomizationJobStatusReadyForSubmit,
		})
	}
	return nil
}

func (r *TaskResourceGroupRepo) ResetCustomizationReadyForSubmit(ctx context.Context, tx repo.Tx, taskID int64) error {
	sqlTx := Unwrap(tx)
	result, err := sqlTx.ExecContext(ctx, `
		UPDATE task_modules
		SET state = 'in_progress', terminal_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE task_id = ? AND module_key = 'customization' AND state = 'submitted'`, taskID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return repo.ErrConflict
	}
	result, err = sqlTx.ExecContext(ctx, `
		UPDATE customization_jobs
		SET status = 'in_progress', updated_at = CURRENT_TIMESTAMP
		WHERE id = (
			SELECT id FROM (
				SELECT id FROM customization_jobs WHERE task_id = ? ORDER BY id DESC LIMIT 1
			) latest
		) AND status = 'ready_for_submit'`, taskID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return repo.ErrConflict
	}
	return nil
}

func (r *TaskResourceGroupRepo) CompleteModules(ctx context.Context, tx repo.Tx, taskID int64) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE task_modules SET state = 'completed', claimed_by = NULL, claimed_team_code = NULL,
		       terminal_at = COALESCE(terminal_at, CURRENT_TIMESTAMP)
		WHERE task_id = ? AND state NOT IN ('completed','closed','forcibly_closed','closed_by_admin')`, taskID)
	return err
}

func (r *TaskResourceGroupRepo) EnqueueTaskFinalized(ctx context.Context, tx repo.Tx, taskID, workflowRevision int64, enqueueFiling, enqueueImageSync bool) error {
	sqlTx := Unwrap(tx)
	if enqueueFiling {
		if _, err := sqlTx.ExecContext(ctx, `
			INSERT INTO task_erp_outbox (task_id, job_type, dedupe_key, payload_json)
			VALUES (?, 'task_filing', ?, JSON_OBJECT('task_id', ?, 'workflow_revision', ?))
			ON DUPLICATE KEY UPDATE dedupe_key = dedupe_key`, taskID, fmt.Sprintf("task_filing:%d:%d", taskID, workflowRevision), taskID, workflowRevision); err != nil {
			return err
		}
	}
	if enqueueImageSync {
		if _, err := sqlTx.ExecContext(ctx, `
			INSERT INTO task_erp_outbox (task_id, job_type, dedupe_key, payload_json)
			VALUES (?, 'task_image_sync', ?, JSON_OBJECT('task_id', ?, 'workflow_revision', ?))
			ON DUPLICATE KEY UPDATE dedupe_key = dedupe_key`, taskID, fmt.Sprintf("task_image_sync:%d:%d", taskID, workflowRevision), taskID, workflowRevision); err != nil {
			return err
		}
	}
	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO search_reindex_outbox (entity_type, entity_id, dedupe_key)
		VALUES ('task', ?, ?)
		ON DUPLICATE KEY UPDATE status = 'pending', next_retry_at = NULL`, taskID, fmt.Sprintf("task:%d:%d", taskID, workflowRevision))
	return err
}

func (r *TaskResourceGroupRepo) StoreIdempotency(ctx context.Context, tx repo.Tx, taskID, actorID int64, action, key, requestHash string, response interface{}) (bool, json.RawMessage, error) {
	sqlTx := Unwrap(tx)
	result, err := sqlTx.ExecContext(ctx, `
		INSERT IGNORE INTO workflow_action_idempotency
		  (task_id, action_type, actor_id, idempotency_key, request_hash)
		VALUES (?, ?, ?, ?, ?)`, taskID, action, actorID, key, requestHash)
	if err != nil {
		return false, nil, err
	}
	if rows, _ := result.RowsAffected(); rows == 1 {
		return true, nil, nil
	}
	var existingHash string
	var responseJSON []byte
	if err := sqlTx.QueryRowContext(ctx, `
		SELECT request_hash, response_json FROM workflow_action_idempotency
		WHERE task_id = ? AND action_type = ? AND actor_id = ? AND idempotency_key = ? FOR UPDATE`,
		taskID, action, actorID, key).Scan(&existingHash, &responseJSON); err != nil {
		return false, nil, err
	}
	if existingHash != requestHash || len(responseJSON) == 0 {
		return false, nil, repo.ErrConflict
	}
	return false, json.RawMessage(responseJSON), nil
}

func (r *TaskResourceGroupRepo) CompleteIdempotency(ctx context.Context, tx repo.Tx, taskID, actorID int64, action, key string, response interface{}) error {
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = Unwrap(tx).ExecContext(ctx, `
		UPDATE workflow_action_idempotency SET response_json = ?, completed_at = CURRENT_TIMESTAMP
		WHERE task_id = ? AND action_type = ? AND actor_id = ? AND idempotency_key = ?`, raw, taskID, action, actorID, key)
	return err
}

func uniquePositiveInt64s(items []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(items))
	for _, item := range items {
		if item <= 0 {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
