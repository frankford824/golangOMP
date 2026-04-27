package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type taskAssetSearchRepo struct{ db *DB }

func NewTaskAssetSearchRepo(db *DB) repo.TaskAssetSearchRepo { return &taskAssetSearchRepo{db: db} }

const taskAssetSearchSelect = `
	SELECT ta.id, ta.task_id, ta.asset_id, ta.scope_sku_code, ta.asset_type, ta.version_no, ta.asset_version_no,
	       ta.upload_mode, ta.upload_request_id, ta.storage_ref_id, ta.file_name, ta.original_filename, ta.remote_file_id,
	       ta.mime_type, ta.file_size, ta.file_path, ta.storage_key, ta.whole_hash, ta.upload_status, ta.preview_status,
	       ta.uploaded_by, ta.uploaded_at, ta.remark, ta.created_at,
	       ta.source_module_key, ta.source_task_module_id, ta.is_archived, ta.archived_at, ta.archived_by, ta.cleaned_at, ta.deleted_at,
	       t.id, t.task_no, t.source_mode, t.product_id, t.sku_code, t.product_name_snapshot,
	       t.task_type, t.operator_group_id, t.owner_team, t.owner_department, t.owner_org_team, t.creator_id, t.requester_id,
	       t.designer_id, t.current_handler_id, t.task_status, t.priority, t.deadline_at, t.need_outsource, t.is_outsource,
	       t.customization_required, t.customization_source_type, t.last_customization_operator_id, t.warehouse_reject_reason,
	       t.warehouse_reject_category, t.is_batch_task, t.batch_item_count, t.batch_mode, t.primary_sku_code,
	       t.sku_generation_status, t.created_at, t.updated_at,
	       da.asset_no, da.created_by, da.created_at, da.updated_at,
	       COALESCE(tm.claimed_team_code, tm.pool_team_code, '') AS owner_team_code`

const taskAssetSearchFrom = `
	  FROM task_assets ta
	  JOIN design_assets da ON da.id = ta.asset_id
	  JOIN tasks t ON t.id = ta.task_id
	  LEFT JOIN task_modules tm ON tm.id = ta.source_task_module_id`

func (r *taskAssetSearchRepo) Search(ctx context.Context, query domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	query = query.Normalized()
	where, args := buildTaskAssetSearchWhere(query)
	countSQL := `SELECT COUNT(*) ` + taskAssetSearchFrom + where
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset search: %w", err)
	}
	args = append(args, (query.Page-1)*query.Size, query.Size)
	rows, err := r.db.db.QueryContext(ctx, taskAssetSearchSelect+taskAssetSearchFrom+where+`
		ORDER BY ta.created_at DESC, ta.id DESC
		LIMIT ?, ?`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search task assets: %w", err)
	}
	defer rows.Close()
	items, err := scanTaskAssetSearchRows(rows)
	return items, total, err
}

func (r *taskAssetSearchRepo) GetCurrentByAssetID(ctx context.Context, assetID int64) (*repo.TaskAssetSearchRow, error) {
	row := r.db.db.QueryRowContext(ctx, taskAssetSearchSelect+taskAssetSearchFrom+`
		WHERE da.id = ?
		  AND ta.id = COALESCE(da.current_version_id, (
		      SELECT ta2.id FROM task_assets ta2 WHERE ta2.asset_id = da.id ORDER BY ta2.asset_version_no DESC, ta2.id DESC LIMIT 1
		  ))`, assetID)
	return scanTaskAssetSearchRow(row)
}

func (r *taskAssetSearchRepo) ListVersionsByAssetID(ctx context.Context, assetID int64) ([]*repo.TaskAssetSearchRow, error) {
	rows, err := r.db.db.QueryContext(ctx, taskAssetSearchSelect+taskAssetSearchFrom+`
		WHERE da.id = ?
		ORDER BY ta.asset_version_no ASC, ta.id ASC`, assetID)
	if err != nil {
		return nil, fmt.Errorf("list asset versions: %w", err)
	}
	defer rows.Close()
	return scanTaskAssetSearchRows(rows)
}

func (r *taskAssetSearchRepo) GetVersion(ctx context.Context, assetID, versionID int64) (*repo.TaskAssetSearchRow, error) {
	row := r.db.db.QueryRowContext(ctx, taskAssetSearchSelect+taskAssetSearchFrom+`
		WHERE da.id = ? AND ta.id = ?`, assetID, versionID)
	return scanTaskAssetSearchRow(row)
}

func buildTaskAssetSearchWhere(query domain.AssetSearchQuery) (string, []interface{}) {
	clauses := []string{`ta.id = COALESCE(da.current_version_id, (
		SELECT ta2.id FROM task_assets ta2 WHERE ta2.asset_id = da.id ORDER BY ta2.asset_version_no DESC, ta2.id DESC LIMIT 1
	))`, `ta.deleted_at IS NULL`}
	var args []interface{}
	if query.Keyword != "" {
		like := "%" + strings.TrimSpace(query.Keyword) + "%"
		clauses = append(clauses, `(ta.file_name LIKE ? OR t.task_no LIKE ? OR t.product_name_snapshot LIKE ?)`)
		args = append(args, like, like, like)
	}
	if query.ModuleKey != "" {
		clauses = append(clauses, `ta.source_module_key = ?`)
		args = append(args, strings.TrimSpace(query.ModuleKey))
	}
	if query.OwnerTeamCode != "" {
		clauses = append(clauses, `COALESCE(tm.claimed_team_code, tm.pool_team_code, '') = ?`)
		args = append(args, strings.TrimSpace(query.OwnerTeamCode))
	}
	if query.CreatedFrom != nil {
		clauses = append(clauses, `ta.created_at >= ?`)
		args = append(args, *query.CreatedFrom)
	}
	if query.CreatedTo != nil {
		clauses = append(clauses, `ta.created_at <= ?`)
		args = append(args, *query.CreatedTo)
	}
	switch query.IsArchived {
	case domain.AssetArchiveFilterTrue:
		clauses = append(clauses, `ta.is_archived = 1`)
	case domain.AssetArchiveFilterAll:
	default:
		clauses = append(clauses, `ta.is_archived = 0`)
	}
	switch query.TaskStatus {
	case domain.AssetTaskStatusFilterOpen:
		clauses = append(clauses, `t.task_status NOT IN (?, ?, ?)`)
		args = append(args, string(domain.TaskStatusCompleted), string(domain.TaskStatusCancelled), string(domain.TaskStatusArchived))
	case domain.AssetTaskStatusFilterClosed:
		clauses = append(clauses, `t.task_status IN (?, ?)`)
		args = append(args, string(domain.TaskStatusCompleted), string(domain.TaskStatusCancelled))
	case domain.AssetTaskStatusFilterArchived:
		clauses = append(clauses, `t.task_status = ?`)
		args = append(args, string(domain.TaskStatusArchived))
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func scanTaskAssetSearchRows(rows *sql.Rows) ([]*repo.TaskAssetSearchRow, error) {
	var out []*repo.TaskAssetSearchRow
	for rows.Next() {
		item, err := scanTaskAssetSearchScanner(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanTaskAssetSearchRow(row *sql.Row) (*repo.TaskAssetSearchRow, error) {
	item, err := scanTaskAssetSearchScanner(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

type taskAssetSearchScanner interface {
	Scan(dest ...interface{}) error
}

func scanTaskAssetSearchScanner(s taskAssetSearchScanner) (*repo.TaskAssetSearchRow, error) {
	var a domain.TaskAsset
	var t domain.Task
	var assetID, assetVersionNo, sourceTaskModuleID, archivedBy, productID, operatorGroupID, requesterID, designerID, currentHandlerID, lastCustomizationOperatorID sql.NullInt64
	var scopeSKUCode, uploadMode, uploadRequestID, storageRefID, originalFilename, remoteFileID, mimeType, filePath, storageKey, wholeHash, uploadStatus, previewStatus, customizationSourceType, warehouseRejectReason, warehouseRejectCategory sql.NullString
	var fileSize sql.NullInt64
	var uploadedAt, archivedAt, cleanedAt, deletedAt, deadlineAt sql.NullTime
	var needOutsource, isOutsource, customizationRequired, isBatchTask sql.NullBool
	var assetNo string
	var designCreatedBy int64
	var designCreatedAt, designUpdatedAt time.Time
	var ownerTeamCode string
	if err := s.Scan(
		&a.ID, &a.TaskID, &assetID, &scopeSKUCode, &a.AssetType, &a.VersionNo, &assetVersionNo,
		&uploadMode, &uploadRequestID, &storageRefID, &a.FileName, &originalFilename, &remoteFileID,
		&mimeType, &fileSize, &filePath, &storageKey, &wholeHash, &uploadStatus, &previewStatus,
		&a.UploadedBy, &uploadedAt, &a.Remark, &a.CreatedAt,
		&a.SourceModuleKey, &sourceTaskModuleID, &a.IsArchived, &archivedAt, &archivedBy, &cleanedAt, &deletedAt,
		&t.ID, &t.TaskNo, &t.SourceMode, &productID, &t.SKUCode, &t.ProductNameSnapshot,
		&t.TaskType, &operatorGroupID, &t.OwnerTeam, &t.OwnerDepartment, &t.OwnerOrgTeam, &t.CreatorID, &requesterID,
		&designerID, &currentHandlerID, &t.TaskStatus, &t.Priority, &deadlineAt, &needOutsource, &isOutsource,
		&customizationRequired, &customizationSourceType, &lastCustomizationOperatorID, &warehouseRejectReason,
		&warehouseRejectCategory, &isBatchTask, &t.BatchItemCount, &t.BatchMode, &t.PrimarySKUCode,
		&t.SKUGenerationStatus, &t.CreatedAt, &t.UpdatedAt,
		&assetNo, &designCreatedBy, &designCreatedAt, &designUpdatedAt,
		&ownerTeamCode,
	); err != nil {
		return nil, fmt.Errorf("scan task asset search row: %w", err)
	}
	a.AssetID = fromNullInt64(assetID)
	a.ScopeSKUCode = fromNullString(scopeSKUCode)
	a.AssetType = domain.NormalizeTaskAssetType(a.AssetType)
	a.AssetVersionNo = fromNullInt(assetVersionNo)
	a.UploadMode = fromNullString(uploadMode)
	a.UploadRequestID = fromNullString(uploadRequestID)
	a.StorageRefID = fromNullString(storageRefID)
	a.OriginalName = fromNullString(originalFilename)
	a.RemoteFileID = fromNullString(remoteFileID)
	a.MimeType = fromNullString(mimeType)
	a.FileSize = fromNullInt64(fileSize)
	a.FilePath = fromNullString(filePath)
	a.StorageKey = fromNullString(storageKey)
	a.WholeHash = fromNullString(wholeHash)
	a.UploadStatus = fromNullString(uploadStatus)
	a.PreviewStatus = fromNullString(previewStatus)
	a.UploadedAt = fromNullTime(uploadedAt)
	a.SourceTaskModuleID = fromNullInt64(sourceTaskModuleID)
	a.ArchivedAt = fromNullTime(archivedAt)
	a.ArchivedBy = fromNullInt64(archivedBy)
	a.CleanedAt = fromNullTime(cleanedAt)
	a.DeletedAt = fromNullTime(deletedAt)
	t.ProductID = fromNullInt64(productID)
	t.OperatorGroupID = fromNullInt64(operatorGroupID)
	t.RequesterID = fromNullInt64(requesterID)
	t.DesignerID = fromNullInt64(designerID)
	t.CurrentHandlerID = fromNullInt64(currentHandlerID)
	t.DeadlineAt = fromNullTime(deadlineAt)
	t.NeedOutsource = needOutsource.Valid && needOutsource.Bool
	t.IsOutsource = isOutsource.Valid && isOutsource.Bool
	t.CustomizationRequired = customizationRequired.Valid && customizationRequired.Bool
	if customizationSourceType.Valid {
		t.CustomizationSourceType = domain.CustomizationSourceType(customizationSourceType.String)
	}
	t.LastCustomizationOperatorID = fromNullInt64(lastCustomizationOperatorID)
	if warehouseRejectReason.Valid {
		t.WarehouseRejectReason = warehouseRejectReason.String
	}
	if warehouseRejectCategory.Valid {
		t.WarehouseRejectCategory = warehouseRejectCategory.String
	}
	t.IsBatchTask = isBatchTask.Valid && isBatchTask.Bool
	return &repo.TaskAssetSearchRow{
		Asset:           &a,
		Task:            &t,
		AssetNo:         assetNo,
		DesignCreatedBy: designCreatedBy,
		DesignCreatedAt: designCreatedAt,
		DesignUpdatedAt: designUpdatedAt,
		OwnerTeamCode:   ownerTeamCode,
	}, nil
}
