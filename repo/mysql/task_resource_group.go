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
			ON DUPLICATE KEY UPDATE updated_at = updated_at`, taskID)
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
			ON DUPLICATE KEY UPDATE updated_at = updated_at`, taskID)
			return err
		}
		_, err := sqlTx.ExecContext(ctx, `
			INSERT INTO task_asset_groups (task_id, scope_kind)
			VALUES (?, 'task')
			ON DUPLICATE KEY UPDATE updated_at = updated_at`, taskID)
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
		       t.task_no, COALESCE(tsi.sku_code, ''), t.business_lane
		FROM task_asset_groups g
		JOIN tasks t ON t.id = g.task_id
		LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
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
			&item.TaskNo, &item.SKUCode, &item.BusinessLane); err != nil {
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
	for i := range items {
		if items[i].WorkingRevisionID != nil {
			revision, err := r.getRevision(ctx, *items[i].WorkingRevisionID)
			if err != nil {
				return nil, err
			}
			items[i].WorkingRevision = revision
		}
		if items[i].FinalizedRevisionID != nil {
			revision, err := r.getRevision(ctx, *items[i].FinalizedRevisionID)
			if err != nil {
				return nil, err
			}
			items[i].FinalizedRevision = revision
		}
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
		       t.task_no, COALESCE(tsi.sku_code, ''), t.business_lane
		FROM task_asset_groups g
		JOIN tasks t ON t.id = g.task_id
		LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
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
		if item.FinalizedRevisionID != nil {
			item.FinalizedRevision, err = r.getRevision(ctx, *item.FinalizedRevisionID)
			if err != nil {
				return nil, 0, err
			}
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
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
		       t.task_no, COALESCE(tsi.sku_code, ''), t.business_lane
		FROM task_asset_groups g
		JOIN tasks t ON t.id = g.task_id
		LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
		WHERE g.id = ?`, groupID)
	item, err := scanTaskResourceGroup(row)
	if err == sql.ErrNoRows {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if item.WorkingRevisionID != nil {
		item.WorkingRevision, err = r.getRevision(ctx, *item.WorkingRevisionID)
		if err != nil {
			return nil, err
		}
	}
	if item.FinalizedRevisionID != nil {
		item.FinalizedRevision, err = r.getRevision(ctx, *item.FinalizedRevisionID)
		if err != nil {
			return nil, err
		}
	}
	return item, nil
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
		&item.TaskNo, &item.SKUCode, &item.BusinessLane); err != nil {
		return nil, err
	}
	item.TaskSKUItemID = fromNullInt64(skuID)
	item.RetouchRequirementID = fromNullInt64(retouchID)
	item.WorkingRevisionID = fromNullInt64(workingID)
	item.FinalizedRevisionID = fromNullInt64(finalizedID)
	return &item, nil
}

func (r *TaskResourceGroupRepo) getRevision(ctx context.Context, revisionID int64) (*domain.TaskAssetGroupRevision, error) {
	var item domain.TaskAssetGroupRevision
	var sourceID sql.NullInt64
	var submittedAt, finalizedAt sql.NullTime
	err := r.db.db.QueryRowContext(ctx, `
		SELECT id, group_id, revision_no, status, mode, source_task_asset_id, source_stage,
		       created_by, reason, submitted_at, finalized_at, created_at
		FROM task_asset_group_revisions WHERE id = ?`, revisionID).
		Scan(&item.ID, &item.GroupID, &item.RevisionNo, &item.Status, &item.Mode, &sourceID, &item.SourceStage,
			&item.CreatedBy, &item.Reason, &submittedAt, &finalizedAt, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	item.SourceTaskAssetID = fromNullInt64(sourceID)
	item.SubmittedAt = fromNullTime(submittedAt)
	item.FinalizedAt = fromNullTime(finalizedAt)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, revision_id, task_asset_id, sort_order, item_name, created_at
		FROM task_asset_group_revision_items WHERE revision_id = ? ORDER BY sort_order, id`, revisionID)
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
	refRows, err := r.db.db.QueryContext(ctx, `
		SELECT id, revision_id, reference_file_ref_id, formal_task_asset_id, sort_order,
		       ref_id_snapshot, file_name_snapshot, scope_snapshot, created_at
		FROM task_asset_group_revision_references WHERE revision_id = ? ORDER BY sort_order, id`, revisionID)
	if err != nil {
		return nil, err
	}
	for refRows.Next() {
		var child domain.TaskAssetGroupRevisionReference
		var formalID sql.NullInt64
		if err := refRows.Scan(&child.ID, &child.RevisionID, &child.ReferenceFileRefID, &formalID, &child.SortOrder,
			&child.RefIDSnapshot, &child.FileNameSnapshot, &child.ScopeSnapshot, &child.CreatedAt); err != nil {
			return nil, err
		}
		child.FormalTaskAssetID = fromNullInt64(formalID)
		item.References = append(item.References, child)
	}
	if err := refRows.Err(); err != nil {
		refRows.Close()
		return nil, err
	}
	if err := refRows.Close(); err != nil {
		return nil, err
	}
	if item.SourceTaskAssetID != nil {
		item.SourceFile, err = r.getResourceFile(ctx, *item.SourceTaskAssetID)
		if err != nil {
			return nil, err
		}
	}
	for index := range item.Items {
		item.Items[index].File, err = r.getResourceFile(ctx, item.Items[index].TaskAssetID)
		if err != nil {
			return nil, err
		}
	}
	return &item, nil
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
		SELECT g.finalized_revision_id, previous.source_task_asset_id, next_revision.source_task_asset_id
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

func (r *TaskResourceGroupRepo) CompleteModules(ctx context.Context, tx repo.Tx, taskID int64) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE task_modules SET state = 'completed', claimed_by = NULL, claimed_team_code = NULL,
		       terminal_at = COALESCE(terminal_at, CURRENT_TIMESTAMP)
		WHERE task_id = ? AND state NOT IN ('completed','closed','forcibly_closed','closed_by_admin')`, taskID)
	return err
}

func (r *TaskResourceGroupRepo) EnqueueTaskFinalized(ctx context.Context, tx repo.Tx, taskID, workflowRevision int64, enqueueFiling bool) error {
	sqlTx := Unwrap(tx)
	if enqueueFiling {
		if _, err := sqlTx.ExecContext(ctx, `
			INSERT INTO task_erp_outbox (task_id, job_type, dedupe_key, payload_json)
			VALUES (?, 'task_filing', ?, JSON_OBJECT('task_id', ?, 'workflow_revision', ?))
			ON DUPLICATE KEY UPDATE dedupe_key = dedupe_key`, taskID, fmt.Sprintf("task_filing:%d:%d", taskID, workflowRevision), taskID, workflowRevision); err != nil {
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
