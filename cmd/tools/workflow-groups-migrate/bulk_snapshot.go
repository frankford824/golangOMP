package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
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
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
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
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
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

func captureTaskModulesForTasks(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]taskModuleSnapshot, error) {
	taskIDs = uniqueSortedInt64(taskIDs)
	if len(taskIDs) == 0 {
		return []taskModuleSnapshot{}, nil
	}
	items := []taskModuleSnapshot{}
	for _, chunk := range int64Chunks(taskIDs) {
		placeholders, args := int64Placeholders(chunk)
		rows, err := q.QueryContext(ctx, `
			SELECT id,task_id,module_key,state,pool_team_code,claimed_by,
			       claimed_team_code,claimed_at,actor_org_snapshot,entered_at,
			       terminal_at,data,updated_at
			FROM task_modules
			WHERE task_id IN (`+placeholders+`)
			ORDER BY task_id,module_key,id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var item taskModuleSnapshot
			var poolTeamCode, claimedTeamCode sql.NullString
			var claimedBy sql.NullInt64
			var claimedAt, terminalAt sql.NullTime
			var actorOrgSnapshot, data []byte
			if err := rows.Scan(
				&item.ID, &item.TaskID, &item.ModuleKey, &item.State,
				&poolTeamCode, &claimedBy, &claimedTeamCode, &claimedAt,
				&actorOrgSnapshot, &item.EnteredAt, &terminalAt, &data,
				&item.UpdatedAt,
			); err != nil {
				rows.Close()
				return nil, err
			}
			item.PoolTeamCode = nullStringPointer(poolTeamCode)
			item.ClaimedBy = nullInt64Pointer(claimedBy)
			item.ClaimedTeamCode = nullStringPointer(claimedTeamCode)
			item.ClaimedAt = nullTimePointer(claimedAt)
			item.TerminalAt = nullTimePointer(terminalAt)
			if actorOrgSnapshot != nil {
				item.ActorOrgSnapshot, err = compactSnapshotJSON(
					actorOrgSnapshot,
					"task_modules.actor_org_snapshot",
				)
				if err != nil {
					rows.Close()
					return nil, err
				}
			}
			item.Data, err = compactSnapshotJSON(
				data,
				"task_modules.data",
			)
			if err != nil {
				rows.Close()
				return nil, err
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func compactSnapshotJSON(raw []byte, label string) (json.RawMessage, error) {
	if raw == nil {
		return nil, nil
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return nil, fmt.Errorf("normalize %s: %w", label, err)
	}
	return append(json.RawMessage(nil), compact.Bytes()...), nil
}

func normalizeTaskModuleSnapshotJSON(
	items []taskModuleSnapshot,
) error {
	for index := range items {
		var err error
		items[index].Data, err = compactSnapshotJSON(
			items[index].Data,
			"task_modules.data",
		)
		if err != nil {
			return err
		}
		if items[index].ActorOrgSnapshot != nil {
			items[index].ActorOrgSnapshot, err = compactSnapshotJSON(
				items[index].ActorOrgSnapshot,
				"task_modules.actor_org_snapshot",
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func captureSearchDocumentsForTasks(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]searchDocumentSnapshot, error) {
	taskIDs = uniqueSortedInt64(taskIDs)
	if len(taskIDs) == 0 {
		return []searchDocumentSnapshot{}, nil
	}
	items := []searchDocumentSnapshot{}
	for _, chunk := range int64Chunks(taskIDs) {
		placeholders, args := int64Placeholders(chunk)
		rows, err := q.QueryContext(ctx, `
			SELECT group_id,task_id,finalized_revision_id,internal_text,final_text,updated_at
			FROM task_asset_group_search_documents
			WHERE task_id IN (`+placeholders+`)
			ORDER BY group_id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var item searchDocumentSnapshot
			var finalizedRevisionID sql.NullInt64
			if err := rows.Scan(
				&item.GroupID, &item.TaskID, &finalizedRevisionID,
				&item.InternalText, &item.FinalText, &item.UpdatedAt,
			); err != nil {
				rows.Close()
				return nil, err
			}
			item.FinalizedRevisionID = nullInt64Pointer(finalizedRevisionID)
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func searchDocumentsMatchSnapshot(
	ctx context.Context,
	q snapshotQueryer,
	taskIDs []int64,
	expected []searchDocumentSnapshot,
) (bool, error) {
	actual, err := captureSearchDocumentsForTasks(ctx, q, taskIDs)
	if err != nil {
		return false, err
	}
	if len(actual) == 0 {
		actual = nil
	}
	if len(expected) == 0 {
		expected = nil
	}
	return reflect.DeepEqual(actual, expected), nil
}

func captureTaskSearchDocumentsForTasks(ctx context.Context, q snapshotQueryer, taskIDs []int64) ([]taskSearchDocumentSnapshot, error) {
	taskIDs = uniqueSortedInt64(taskIDs)
	if len(taskIDs) == 0 {
		return []taskSearchDocumentSnapshot{}, nil
	}
	items := []taskSearchDocumentSnapshot{}
	for _, chunk := range int64Chunks(taskIDs) {
		placeholders, args := int64Placeholders(chunk)
		rows, err := q.QueryContext(ctx, `
			SELECT task_id,task_no,product_name_snapshot,sku_code,primary_sku_code,
			       product_i_id,task_type,task_status,priority,owner_department,
			       owner_team,owner_org_team,creator_id,creator_name,requester_id,
			       requester_name,designer_id,designer_name,current_handler_id,
			       current_handler_name,created_at,updated_at,deadline_at,asset_text,
			       search_text
			FROM task_search_documents
			WHERE task_id IN (`+placeholders+`)
			ORDER BY task_id`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var item taskSearchDocumentSnapshot
			var creatorID, requesterID, designerID, currentHandlerID sql.NullInt64
			var createdAt, updatedAt, deadlineAt sql.NullTime
			var assetText sql.NullString
			if err := rows.Scan(
				&item.TaskID, &item.TaskNo, &item.ProductNameSnapshot,
				&item.SKUCode, &item.PrimarySKUCode, &item.ProductIID,
				&item.TaskType, &item.TaskStatus, &item.Priority,
				&item.OwnerDepartment, &item.OwnerTeam, &item.OwnerOrgTeam,
				&creatorID, &item.CreatorName, &requesterID,
				&item.RequesterName, &designerID, &item.DesignerName,
				&currentHandlerID, &item.CurrentHandlerName, &createdAt,
				&updatedAt, &deadlineAt, &assetText, &item.SearchText,
			); err != nil {
				rows.Close()
				return nil, err
			}
			item.CreatorID = nullInt64Pointer(creatorID)
			item.RequesterID = nullInt64Pointer(requesterID)
			item.DesignerID = nullInt64Pointer(designerID)
			item.CurrentHandlerID = nullInt64Pointer(currentHandlerID)
			item.CreatedAt = nullTimePointer(createdAt)
			item.UpdatedAt = nullTimePointer(updatedAt)
			item.DeadlineAt = nullTimePointer(deadlineAt)
			item.AssetText = nullStringPointer(assetText)
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func taskSearchDocumentsMatchSnapshot(
	ctx context.Context,
	q snapshotQueryer,
	taskIDs []int64,
	expected []taskSearchDocumentSnapshot,
) (bool, error) {
	actual, err := captureTaskSearchDocumentsForTasks(ctx, q, taskIDs)
	if err != nil {
		return false, err
	}
	if len(actual) == 0 {
		actual = nil
	}
	if len(expected) == 0 {
		expected = nil
	}
	return reflect.DeepEqual(actual, expected), nil
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
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
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
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
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
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
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
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
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
			       asset_type,scope_sku_code,retouch_requirement_id,flow_review_status,approved_at,approved_by,
			       COALESCE(mime_type,''),COALESCE(whole_hash,''),deleted_at,cleaned_at
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
			var stagedExpiresAt, revokedAt, objectDeletedAt, approvedAt, deletedAt, cleanedAt sql.NullTime
			var approvedBy sql.NullInt64
			if err := rows.Scan(&item.ID, &item.TaskID, &item.BindingState, &boundGroupID, &boundRole,
				&stagedSKUItemID, &stagedRetouchID, &stagedRole, &stagedBy, &uploadSessionID, &stagedExpiresAt,
				&revokedAt, &item.AccessRevokedReason, &objectDeletedAt,
				&item.AssetType, &scopeSKUCode, &retouchRequirementID, &item.FlowReviewStatus, &approvedAt, &approvedBy,
				&item.MimeType, &item.WholeHash, &deletedAt, &cleanedAt); err != nil {
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
			item.ApprovedAt = nullTimePointer(approvedAt)
			item.ApprovedBy = nullInt64Pointer(approvedBy)
			item.DeletedAt = nullTimePointer(deletedAt)
			item.CleanedAt = nullTimePointer(cleanedAt)
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
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

func validatePlanningImages(
	ctx context.Context,
	q snapshotQueryer,
	planning []planningMapping,
	forUpdate bool,
) error {
	lockClause := ""
	if forUpdate {
		lockClause = " FOR UPDATE"
	}
	for _, mapping := range planning {
		for _, item := range mapping.Items {
			rows, err := q.QueryContext(ctx, `
				SELECT ta.id,ta.storage_ref_id
				FROM task_assets ta
				JOIN task_sku_items si ON si.id=? AND si.task_id=?
				JOIN asset_storage_refs sr ON sr.ref_id=ta.storage_ref_id
				WHERE ta.task_id=?
				  AND BINARY TRIM(ta.scope_sku_code)=BINARY TRIM(si.sku_code)
				  AND BINARY ta.asset_type=BINARY 'erp_product_image'
				  AND BINARY ta.upload_status=BINARY 'uploaded'
				  AND ta.is_archived=0
				  AND ta.superseded_by_version_id IS NULL
				  AND ta.deleted_at IS NULL
				  AND ta.cleaned_at IS NULL
				  AND ta.access_revoked_at IS NULL
				  AND ta.object_deleted_at IS NULL
				  AND BINARY sr.owner_type=BINARY 'task_asset'
				  AND sr.owner_id=ta.id
				  AND BINARY sr.status IN (BINARY 'active',BINARY 'recorded')
				  AND sr.is_placeholder=0
				  AND BINARY COALESCE(TRIM(sr.ref_key),'')<>BINARY ''
				ORDER BY ta.id`+lockClause,
				item.TaskSKUItemID, mapping.TaskID, mapping.TaskID)
			if err != nil {
				return fmt.Errorf("lock planning image candidates for task %d SKU item %d: %w", mapping.TaskID, item.TaskSKUItemID, err)
			}
			type candidate struct {
				assetID int64
				refID   string
			}
			candidates := []candidate{}
			for rows.Next() {
				var value candidate
				if err := rows.Scan(&value.assetID, &value.refID); err != nil {
					rows.Close()
					return fmt.Errorf("scan planning image candidate for task %d SKU item %d: %w", mapping.TaskID, item.TaskSKUItemID, err)
				}
				candidates = append(candidates, value)
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return fmt.Errorf("iterate planning image candidates for task %d SKU item %d: %w", mapping.TaskID, item.TaskSKUItemID, err)
			}
			if err := rows.Close(); err != nil {
				return err
			}
			valid := len(candidates) == 0 && item.ImageStorageRef == ""
			if len(candidates) == 1 && candidates[0].refID == item.ImageStorageRef {
				valid = true
			}
			if !valid {
				return fmt.Errorf(
					"planning task %d SKU item %d image selection drifted: mapping ref=%q eligible candidates=%v",
					mapping.TaskID,
					item.TaskSKUItemID,
					item.ImageStorageRef,
					candidates,
				)
			}
		}
	}
	return nil
}

func validateAndLockPlanningImages(ctx context.Context, tx *sql.Tx, planning []planningMapping) error {
	return validatePlanningImages(ctx, tx, planning, true)
}

func lockCutoverTargets(ctx context.Context, tx *sql.Tx, m mappingFile) error {
	taskIDs, err := lockInt64Rows(ctx, tx, `
		SELECT id FROM tasks
		WHERE task_status IN ('PendingAuditA','PendingAuditB','RejectedByAuditA','RejectedByAuditB','PendingCustomizationReview','PendingCustomizationProduction','PendingEffectReview','PendingEffectRevision','PendingProductionTransfer','PendingWarehouseQC','PendingWarehouseReceive','PendingClose','PendingOutsource','Outsourcing','PendingOutsourceReview')
		   OR task_type='purchase_task'
		   OR (task_status='Completed' AND EXISTS (
		       SELECT 1 FROM task_modules tm
		       WHERE tm.task_id=tasks.id
		         AND tm.state NOT IN ('completed','closed','forcibly_closed','closed_by_admin')
		   ))
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
	if _, err := lockInt64RowsByIDs(ctx, tx, "task_modules", "task_id", taskIDs); err != nil {
		return err
	}
	for _, chunk := range int64Chunks(taskIDs) {
		placeholders, args := int64Placeholders(chunk)
		if _, err := lockInt64Rows(ctx, tx,
			`SELECT group_id FROM task_asset_group_search_documents WHERE task_id IN (`+placeholders+`) ORDER BY group_id FOR UPDATE`,
			args...); err != nil {
			return err
		}
		if _, err := lockInt64Rows(ctx, tx,
			`SELECT task_id FROM task_search_documents WHERE task_id IN (`+placeholders+`) ORDER BY task_id FOR UPDATE`,
			args...); err != nil {
			return err
		}
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
		if recovery.Strategy == "verified_oss_recovery_v1" {
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
	if err := validateAndLockPlanningImages(ctx, tx, m.Planning); err != nil {
		return err
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
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
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
