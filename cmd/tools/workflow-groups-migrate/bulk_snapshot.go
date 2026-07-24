package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const workflowGroupsBulkChunkSize = 500

var errBulkSnapshotMissingRows = errors.New("bulk snapshot missing expected rows")

func int64Chunks(values []int64) [][]int64 {
	values = uniqueSortedInt64(values)
	if len(values) == 0 {
		return nil
	}
	chunks := make([][]int64, 0, (len(values)+workflowGroupsBulkChunkSize-1)/workflowGroupsBulkChunkSize)
	for start := 0; start < len(values); start += workflowGroupsBulkChunkSize {
		end := start + workflowGroupsBulkChunkSize
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
	}
	return chunks
}

func int64Placeholders(values []int64) (string, []interface{}) {
	placeholders := make([]string, len(values))
	args := make([]interface{}, len(values))
	for index, value := range values {
		placeholders[index] = "?"
		args[index] = value
	}
	return strings.Join(placeholders, ","), args
}

func captureTaskSnapshotsBulk(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]taskSnapshot, error) {
	taskIDs = uniqueSortedInt64(taskIDs)
	if len(taskIDs) == 0 {
		return []taskSnapshot{}, nil
	}
	byID := make(map[int64]*taskSnapshot, len(taskIDs))
	for _, chunk := range int64Chunks(taskIDs) {
		placeholders, args := int64Placeholders(chunk)
		rows, err := q.QueryContext(ctx, `
			SELECT id,task_type,task_status,workflow_revision,current_handler_id,updated_at
			FROM tasks
			WHERE id IN (`+placeholders+`)
			ORDER BY id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var item taskSnapshot
			var handlerID sql.NullInt64
			if err := rows.Scan(&item.ID, &item.TaskType, &item.TaskStatus, &item.WorkflowRevision, &handlerID, &item.UpdatedAt); err != nil {
				rows.Close()
				return nil, err
			}
			item.CurrentHandlerID = nullInt64Pointer(handlerID)
			item.EventIDs = []string{}
			item.ModuleEventIDs = []int64{}
			byID[item.ID] = &item
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	if len(byID) != len(taskIDs) {
		return nil, fmt.Errorf("%w: loaded %d tasks for %d ids", errBulkSnapshotMissingRows, len(byID), len(taskIDs))
	}
	for _, chunk := range int64Chunks(taskIDs) {
		placeholders, args := int64Placeholders(chunk)
		rows, err := q.QueryContext(ctx, `
			SELECT task_id,id
			FROM task_event_logs
			WHERE task_id IN (`+placeholders+`)
			ORDER BY task_id,sequence,id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var taskID int64
			var eventID string
			if err := rows.Scan(&taskID, &eventID); err != nil {
				rows.Close()
				return nil, err
			}
			byID[taskID].EventIDs = append(byID[taskID].EventIDs, eventID)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		rows, err = q.QueryContext(ctx, `
			SELECT tm.task_id,e.id
			FROM task_module_events e
			JOIN task_modules tm ON tm.id=e.task_module_id
			WHERE tm.task_id IN (`+placeholders+`)
			ORDER BY tm.task_id,e.id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var taskID, eventID int64
			if err := rows.Scan(&taskID, &eventID); err != nil {
				rows.Close()
				return nil, err
			}
			byID[taskID].ModuleEventIDs = append(byID[taskID].ModuleEventIDs, eventID)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	result := make([]taskSnapshot, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		result = append(result, *byID[taskID])
	}
	return result, nil
}

func captureResourceGroupsForTasks(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]resourceGroupSnapshot, error) {
	taskIDs = uniqueSortedInt64(taskIDs)
	if len(taskIDs) == 0 {
		return []resourceGroupSnapshot{}, nil
	}
	groups := []resourceGroupSnapshot{}
	groupIndex := map[int64]int{}
	for _, chunk := range int64Chunks(taskIDs) {
		placeholders, args := int64Placeholders(chunk)
		rows, err := q.QueryContext(ctx, `
			SELECT id,task_id,working_revision_id,finalized_revision_id,lock_version,migration_incomplete,migration_issue,updated_at
			FROM task_asset_groups
			WHERE task_id IN (`+placeholders+`)
			ORDER BY id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var item resourceGroupSnapshot
			var workingID, finalizedID sql.NullInt64
			if err := rows.Scan(&item.ID, &item.TaskID, &workingID, &finalizedID, &item.LockVersion, &item.MigrationIncomplete, &item.MigrationIssue, &item.UpdatedAt); err != nil {
				rows.Close()
				return nil, err
			}
			item.WorkingRevisionID = nullInt64Pointer(workingID)
			item.FinalizedRevisionID = nullInt64Pointer(finalizedID)
			item.RevisionIDs = []int64{}
			item.Revisions = []resourceRevisionSnapshot{}
			groupIndex[item.ID] = len(groups)
			groups = append(groups, item)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].ID < groups[j].ID })
	groupIndex = make(map[int64]int, len(groups))
	groupIDs := make([]int64, 0, len(groups))
	for index := range groups {
		groupIndex[groups[index].ID] = index
		groupIDs = append(groupIDs, groups[index].ID)
	}
	revisionIndex := map[int64][2]int{}
	revisionIDs := []int64{}
	for _, chunk := range int64Chunks(groupIDs) {
		placeholders, args := int64Placeholders(chunk)
		rows, err := q.QueryContext(ctx, `
			SELECT id,group_id,revision_no,status,mode,source_task_asset_id,source_stage,created_by,reason,submitted_at,finalized_at,created_at
			FROM task_asset_group_revisions
			WHERE group_id IN (`+placeholders+`)
			ORDER BY group_id,revision_no,id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var item resourceRevisionSnapshot
			var sourceID sql.NullInt64
			var submittedAt, finalizedAt sql.NullTime
			if err := rows.Scan(&item.ID, &item.GroupID, &item.RevisionNo, &item.Status, &item.Mode, &sourceID, &item.SourceStage,
				&item.CreatedBy, &item.Reason, &submittedAt, &finalizedAt, &item.CreatedAt); err != nil {
				rows.Close()
				return nil, err
			}
			item.SourceAssetID = nullInt64Pointer(sourceID)
			item.SubmittedAt = nullTimePointer(submittedAt)
			item.FinalizedAt = nullTimePointer(finalizedAt)
			item.Items = []resourceRevisionItemSnapshot{}
			item.References = []resourceRevisionReferenceSnapshot{}
			groupPosition := groupIndex[item.GroupID]
			groups[groupPosition].RevisionIDs = append(groups[groupPosition].RevisionIDs, item.ID)
			groups[groupPosition].Revisions = append(groups[groupPosition].Revisions, item)
			revisionPosition := len(groups[groupPosition].Revisions) - 1
			revisionIndex[item.ID] = [2]int{groupPosition, revisionPosition}
			revisionIDs = append(revisionIDs, item.ID)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	sortResourceGroupRevisionIDs(groups)
	for _, chunk := range int64Chunks(revisionIDs) {
		placeholders, args := int64Placeholders(chunk)
		rows, err := q.QueryContext(ctx, `
			SELECT id,revision_id,task_asset_id,sort_order,item_name,created_at
			FROM task_asset_group_revision_items
			WHERE revision_id IN (`+placeholders+`)
			ORDER BY revision_id,sort_order,id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var child resourceRevisionItemSnapshot
			if err := rows.Scan(&child.ID, &child.RevisionID, &child.TaskAssetID, &child.SortOrder, &child.ItemName, &child.CreatedAt); err != nil {
				rows.Close()
				return nil, err
			}
			position := revisionIndex[child.RevisionID]
			groups[position[0]].Revisions[position[1]].Items = append(groups[position[0]].Revisions[position[1]].Items, child)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		rows, err = q.QueryContext(ctx, `
			SELECT id,revision_id,reference_file_ref_id,formal_task_asset_id,sort_order,
			       ref_id_snapshot,file_name_snapshot,scope_snapshot,created_at
			FROM task_asset_group_revision_references
			WHERE revision_id IN (`+placeholders+`)
			ORDER BY revision_id,sort_order,id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var child resourceRevisionReferenceSnapshot
			var formalID sql.NullInt64
			if err := rows.Scan(&child.ID, &child.RevisionID, &child.ReferenceFileRefID, &formalID, &child.SortOrder,
				&child.RefIDSnapshot, &child.FileNameSnapshot, &child.ScopeSnapshot, &child.CreatedAt); err != nil {
				rows.Close()
				return nil, err
			}
			child.FormalTaskAssetID = nullInt64Pointer(formalID)
			position := revisionIndex[child.RevisionID]
			groups[position[0]].Revisions[position[1]].References = append(groups[position[0]].Revisions[position[1]].References, child)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return groups, nil
}

func sortResourceGroupRevisionIDs(groups []resourceGroupSnapshot) {
	for index := range groups {
		sort.Slice(groups[index].RevisionIDs, func(i, j int) bool {
			return groups[index].RevisionIDs[i] < groups[index].RevisionIDs[j]
		})
	}
}

func captureAssetBindingsForTasks(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]assetBindingSnapshot, error) {
	taskIDs = uniqueSortedInt64(taskIDs)
	if len(taskIDs) == 0 {
		return []assetBindingSnapshot{}, nil
	}
	items := []assetBindingSnapshot{}
	for _, chunk := range int64Chunks(taskIDs) {
		placeholders, args := int64Placeholders(chunk)
		rows, err := q.QueryContext(ctx, `
			SELECT id,task_id,binding_state,bound_group_id,bound_role,
			       staged_task_sku_item_id,staged_retouch_requirement_id,staged_role,staged_by,upload_session_id,staged_expires_at,
			       access_revoked_at,access_revoked_reason,object_deleted_at,
			       asset_type,scope_sku_code,retouch_requirement_id,COALESCE(mime_type,''),COALESCE(whole_hash,''),deleted_at,cleaned_at
			FROM task_assets
			WHERE task_id IN (`+placeholders+`)
			ORDER BY id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var item assetBindingSnapshot
			var boundGroupID, stagedSKUItemID, stagedRetouchID, stagedBy sql.NullInt64
			var boundRole, stagedRole, uploadSessionID, scopeSKUCode sql.NullString
			var retouchRequirementID sql.NullInt64
			var stagedExpiresAt, revokedAt, objectDeletedAt, deletedAt, cleanedAt sql.NullTime
			if err := rows.Scan(&item.ID, &item.TaskID, &item.BindingState, &boundGroupID, &boundRole,
				&stagedSKUItemID, &stagedRetouchID, &stagedRole, &stagedBy, &uploadSessionID, &stagedExpiresAt,
				&revokedAt, &item.AccessRevokedReason, &objectDeletedAt,
				&item.AssetType, &scopeSKUCode, &retouchRequirementID, &item.MimeType, &item.WholeHash, &deletedAt, &cleanedAt); err != nil {
				rows.Close()
				return nil, err
			}
			item.BoundGroupID = nullInt64Pointer(boundGroupID)
			item.BoundRole = nullStringPointer(boundRole)
			item.StagedTaskSKUItemID = nullInt64Pointer(stagedSKUItemID)
			item.StagedRetouchRequirementID = nullInt64Pointer(stagedRetouchID)
			item.StagedRole = nullStringPointer(stagedRole)
			item.StagedBy = nullInt64Pointer(stagedBy)
			item.UploadSessionID = nullStringPointer(uploadSessionID)
			item.StagedExpiresAt = nullTimePointer(stagedExpiresAt)
			item.AccessRevokedAt = nullTimePointer(revokedAt)
			item.ObjectDeletedAt = nullTimePointer(objectDeletedAt)
			item.ScopeSKUCode = nullStringPointer(scopeSKUCode)
			item.RetouchRequirementID = nullInt64Pointer(retouchRequirementID)
			item.DeletedAt = nullTimePointer(deletedAt)
			item.CleanedAt = nullTimePointer(cleanedAt)
			items = append(items, item)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func lockInt64RowsByIDs(ctx context.Context, tx *sql.Tx, table, column string, ids []int64) ([]int64, error) {
	locked := []int64{}
	for _, chunk := range int64Chunks(ids) {
		placeholders, args := int64Placeholders(chunk)
		rows, err := lockInt64Rows(ctx, tx,
			`SELECT id FROM `+table+` WHERE `+column+` IN (`+placeholders+`) ORDER BY id FOR UPDATE`,
			args...)
		if err != nil {
			return nil, err
		}
		locked = append(locked, rows...)
	}
	return locked, nil
}

func lockCutoverTargets(ctx context.Context, tx *sql.Tx, m mappingFile) error {
	taskIDs, err := lockInt64Rows(ctx, tx, `
		SELECT id FROM tasks
		WHERE task_status IN ('PendingAuditA','PendingAuditB','RejectedByAuditA','RejectedByAuditB','PendingCustomizationReview','PendingCustomizationProduction','PendingEffectReview','PendingEffectRevision','PendingProductionTransfer','PendingWarehouseQC','PendingWarehouseReceive','PendingClose','PendingOutsource','Outsourcing','PendingOutsourceReview')
		   OR task_type='purchase_task'
		ORDER BY id FOR UPDATE`)
	if err != nil {
		return err
	}
	for _, item := range m.Resources {
		taskIDs = append(taskIDs, item.TaskID)
	}
	for _, item := range m.Planning {
		taskIDs = append(taskIDs, item.TaskID)
	}
	for _, item := range m.TaskDecisions {
		taskIDs = append(taskIDs, item.TaskID)
	}
	for _, item := range m.AssetRecoveries {
		taskIDs = append(taskIDs, item.TaskID)
	}
	taskIDs = uniqueSortedInt64(taskIDs)
	if _, err := lockInt64RowsByIDs(ctx, tx, "tasks", "id", taskIDs); err != nil {
		return err
	}
	if _, err := lockInt64RowsByIDs(ctx, tx, "task_sku_items", "task_id", taskIDs); err != nil {
		return err
	}
	groupIDs, err := lockInt64Rows(ctx, tx, `SELECT id FROM task_asset_groups WHERE migration_incomplete=1 ORDER BY id FOR UPDATE`)
	if err != nil {
		return err
	}
	lockedGroups, err := lockInt64RowsByIDs(ctx, tx, "task_asset_groups", "task_id", taskIDs)
	if err != nil {
		return err
	}
	groupIDs = uniqueSortedInt64(append(groupIDs, lockedGroups...))
	resources := append([]resourceMapping(nil), m.Resources...)
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].TaskID != resources[j].TaskID {
			return resources[i].TaskID < resources[j].TaskID
		}
		if resources[i].ScopeKind != resources[j].ScopeKind {
			return resources[i].ScopeKind < resources[j].ScopeKind
		}
		return resources[i].ScopeRefID < resources[j].ScopeRefID
	})
	for _, item := range resources {
		ids, err := lockInt64Rows(ctx, tx,
			`SELECT id FROM task_asset_groups WHERE task_id=? AND scope_kind=? AND scope_ref_id=? FOR UPDATE`,
			item.TaskID, item.ScopeKind, item.ScopeRefID)
		if err != nil {
			return err
		}
		groupIDs = append(groupIDs, ids...)
	}
	groupIDs = uniqueSortedInt64(groupIDs)
	if _, err := lockInt64RowsByIDs(ctx, tx, "task_asset_group_revisions", "group_id", groupIDs); err != nil {
		return err
	}
	if _, err := lockInt64RowsByIDs(ctx, tx, "task_assets", "task_id", taskIDs); err != nil {
		return err
	}
	assetIDs := []int64{}
	referenceIDs := []int64{}
	for _, item := range m.Resources {
		assetIDs = append(assetIDs, item.mappedAssetIDs()...)
		referenceIDs = append(referenceIDs, item.mappedReferenceIDs()...)
	}
	if _, err := lockInt64RowsByIDs(ctx, tx, "task_assets", "id", assetIDs); err != nil {
		return err
	}
	if _, err := lockInt64RowsByIDs(ctx, tx, "reference_file_refs", "id", referenceIDs); err != nil {
		return err
	}
	for _, recovery := range m.AssetRecoveries {
		if recovery.Strategy == "clone_b_prematerialized_storage_ref_v1" {
			continue
		}
		if _, err := queryStringIDs(ctx, tx, `SELECT ref_id FROM asset_storage_refs WHERE ref_id=? FOR UPDATE`, recovery.OriginalStorageRefID); err != nil {
			return err
		}
	}
	planningIDs := planningTaskIDs(m)
	planningQueries := []string{
		`SELECT d.task_sku_item_id FROM task_planning_sku_details d JOIN task_sku_items si ON si.id=d.task_sku_item_id WHERE si.task_id IN (%s) ORDER BY d.task_sku_item_id FOR UPDATE`,
		`SELECT r.id FROM task_planning_sku_revisions r JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id IN (%s) ORDER BY r.id FOR UPDATE`,
		`SELECT i.revision_id FROM task_planning_sku_revision_images i JOIN task_planning_sku_revisions r ON r.id=i.revision_id JOIN task_sku_items si ON si.id=r.task_sku_item_id WHERE si.task_id IN (%s) ORDER BY i.revision_id FOR UPDATE`,
	}
	for _, chunk := range int64Chunks(planningIDs) {
		placeholders, args := int64Placeholders(chunk)
		if _, err := lockInt64Rows(ctx, tx,
			`SELECT task_id FROM task_planning_settings WHERE task_id IN (`+placeholders+`) ORDER BY task_id FOR UPDATE`,
			args...); err != nil {
			return err
		}
	}
	for _, chunk := range int64Chunks(planningIDs) {
		placeholders, args := int64Placeholders(chunk)
		for _, query := range planningQueries {
			if _, err := lockInt64Rows(ctx, tx, fmt.Sprintf(query, placeholders), args...); err != nil {
				return err
			}
		}
	}
	for _, chunk := range int64Chunks(taskIDs) {
		placeholders, args := int64Placeholders(chunk)
		rows, err := tx.QueryContext(ctx, `SELECT id FROM task_event_logs WHERE task_id IN (`+placeholders+`) ORDER BY task_id,sequence,id FOR UPDATE`, args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if _, err := lockInt64Rows(ctx, tx, `
			SELECT e.id FROM task_module_events e
			JOIN task_modules tm ON tm.id=e.task_module_id
			WHERE tm.task_id IN (`+placeholders+`)
			ORDER BY tm.task_id,e.id FOR UPDATE`, args...); err != nil {
			return err
		}
	}
	return nil
}
