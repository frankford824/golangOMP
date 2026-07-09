package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
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
	       ta.flow_review_status, ta.approved_at, ta.approved_by, ta.rejected_at, ta.rejected_by, ta.superseded_by_version_id, ta.superseded_at, ta.cleanup_after_at, ta.source_asset_version_id,
	       t.id, t.task_no, t.source_mode, t.product_id, t.sku_code, t.product_name_snapshot,
	       t.task_type, t.operator_group_id, t.owner_team, t.owner_department, t.owner_org_team, t.creator_id, t.requester_id,
	       t.designer_id, t.current_handler_id, t.task_status, t.priority, t.deadline_at, t.need_outsource, t.is_outsource,
	       COALESCE(t.business_lane, ''), t.customization_required, t.customization_source_type, t.last_customization_operator_id, t.warehouse_reject_reason,
	       t.warehouse_reject_category, t.is_batch_task, t.batch_item_count, t.batch_mode, t.primary_sku_code,
	       t.sku_generation_status, t.created_at, t.updated_at,
	       da.asset_no, da.created_by, da.created_at, da.updated_at,
	       COALESCE(tm.claimed_team_code, tm.pool_team_code, '') AS owner_team_code,
	       COALESCE(NULLIF(task_creator.username, ''), '') AS task_creator_username,
	       COALESCE(NULLIF(task_creator.display_name, ''), '') AS task_creator_name,
	       COALESCE(NULLIF(asset_creator.username, ''), '') AS asset_creator_username,
	       COALESCE(NULLIF(asset_creator.display_name, ''), '') AS asset_creator_name,
	       COALESCE(NULLIF(uploaded_user.username, ''), '') AS uploaded_by_username,
	       COALESCE(NULLIF(uploaded_user.display_name, ''), '') AS uploaded_by_name`

const taskAssetSearchFrom = `
	  FROM task_assets ta
	  JOIN design_assets da ON da.id = ta.asset_id
	  JOIN tasks t ON t.id = ta.task_id
	  LEFT JOIN task_modules tm ON tm.id = ta.source_task_module_id
	  LEFT JOIN users task_creator ON task_creator.id = t.creator_id
	  LEFT JOIN users asset_creator ON asset_creator.id = da.created_by
	  LEFT JOIN users uploaded_user ON uploaded_user.id = ta.uploaded_by`

const taskAssetSearchCountFrom = `
	  FROM task_assets ta
	  JOIN design_assets da ON da.id = ta.asset_id
	  JOIN tasks t ON t.id = ta.task_id
	  LEFT JOIN task_modules tm ON tm.id = ta.source_task_module_id`

func (r *taskAssetSearchRepo) Search(ctx context.Context, query domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	query = query.Normalized()
	where, args := buildTaskAssetSearchWhere(query)
	countSQL := `SELECT COUNT(*) ` + taskAssetSearchCountFrom + where
	var total int64
	countCtx, cancelCount := mysqlReadQueryContext(ctx)
	err := r.db.db.QueryRowContext(countCtx, countSQL, args...).Scan(&total)
	cancelCount()
	if err != nil {
		return nil, 0, fmt.Errorf("count asset search: %w", err)
	}
	args = append(args, (query.Page-1)*query.Size, query.Size)
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, taskAssetSearchSelect+taskAssetSearchFrom+where+`
		ORDER BY `+taskAssetSearchOrderBy(query)+`
		LIMIT ?, ?`, args...)
	if err != nil {
		cancelQuery()
		return nil, 0, fmt.Errorf("search task assets: %w", err)
	}
	defer cancelQuery()
	defer rows.Close()
	items, err := scanTaskAssetSearchRows(rows)
	return items, total, err
}

func (r *taskAssetSearchRepo) GetCurrentByAssetID(ctx context.Context, assetID int64) (*repo.TaskAssetSearchRow, error) {
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	row := r.db.db.QueryRowContext(queryCtx, taskAssetSearchSelect+taskAssetSearchFrom+`
		WHERE da.id = ?
		  AND `+taskAssetCurrentVersionPredicate(), assetID)
	item, err := scanTaskAssetSearchRow(row)
	cancelQuery()
	return item, err
}

func (r *taskAssetSearchRepo) ListCurrentByAssetIDs(ctx context.Context, assetIDs []int64) ([]*repo.TaskAssetSearchRow, error) {
	query, args := buildListCurrentByAssetIDsQuery(assetIDs)
	if query == "" {
		return []*repo.TaskAssetSearchRow{}, nil
	}
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, query, args...)
	if err != nil {
		cancelQuery()
		return nil, fmt.Errorf("list current assets by ids: %w", err)
	}
	defer cancelQuery()
	defer rows.Close()
	return scanTaskAssetSearchRows(rows)
}

func (r *taskAssetSearchRepo) ListVersionsByAssetID(ctx context.Context, assetID int64) ([]*repo.TaskAssetSearchRow, error) {
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	rows, err := r.db.db.QueryContext(queryCtx, taskAssetSearchSelect+taskAssetSearchFrom+`
		WHERE da.id = ?
		ORDER BY ta.asset_version_no ASC, ta.id ASC`, assetID)
	if err != nil {
		cancelQuery()
		return nil, fmt.Errorf("list asset versions: %w", err)
	}
	defer cancelQuery()
	defer rows.Close()
	return scanTaskAssetSearchRows(rows)
}

func (r *taskAssetSearchRepo) GetVersion(ctx context.Context, assetID, versionID int64) (*repo.TaskAssetSearchRow, error) {
	queryCtx, cancelQuery := mysqlReadQueryContext(ctx)
	row := r.db.db.QueryRowContext(queryCtx, taskAssetSearchSelect+taskAssetSearchFrom+`
		WHERE da.id = ? AND ta.id = ?`, assetID, versionID)
	item, err := scanTaskAssetSearchRow(row)
	cancelQuery()
	return item, err
}

func buildTaskAssetSearchWhere(query domain.AssetSearchQuery) (string, []interface{}) {
	clauses := []string{taskAssetCurrentVersionPredicate(), `ta.deleted_at IS NULL`, `NOT (
		da.source_asset_id IS NOT NULL
		AND da.asset_type IN ('preview', 'design_thumb')
		AND COALESCE(ta.remark, '') IN ('async-derived-preview', 'async-derived-preview:webp')
	)`}
	var args []interface{}
	if query.Keyword != "" {
		kw := normalizeSearchKeyword(query.Keyword)
		keywordClauses := []string{
			"ta.file_name LIKE ?",
			"ta.original_filename LIKE ?",
			"t.product_name_snapshot LIKE ?",
		}
		keywordArgs := []interface{}{kw.Like, kw.Like, kw.Like}
		if kw.HasInt64 {
			keywordClauses = append(keywordClauses, "ta.asset_id = ?", "ta.id = ?", "ta.task_id = ?")
			keywordArgs = append(keywordArgs, kw.Int64, kw.Int64, kw.Int64)
		}
		if kw.IsCode {
			keywordClauses = append(keywordClauses,
				"t.sku_code = ?",
				"t.primary_sku_code = ?",
				"ta.scope_sku_code = ?",
				"t.task_no = ?",
				"t.sku_code LIKE ?",
				"t.primary_sku_code LIKE ?",
				"ta.scope_sku_code LIKE ?",
				"t.task_no LIKE ?",
			)
			keywordArgs = append(keywordArgs, kw.Upper, kw.Upper, kw.Upper, kw.Upper, kw.Upper+"%", kw.Upper+"%", kw.Upper+"%", kw.Upper+"%")
		} else {
			keywordClauses = append(keywordClauses,
				"t.sku_code LIKE ?",
				"t.primary_sku_code LIKE ?",
				"ta.scope_sku_code LIKE ?",
				"t.task_no LIKE ?",
			)
			keywordArgs = append(keywordArgs, kw.Like, kw.Like, kw.Like, kw.Like)
		}
		clauses = append(clauses, "("+strings.Join(keywordClauses, " OR ")+")")
		args = append(args, keywordArgs...)
	}
	if query.ModuleKey != "" {
		clauses = append(clauses, `ta.source_module_key = ?`)
		args = append(args, strings.TrimSpace(query.ModuleKey))
	}
	if query.OwnerTeamCode != "" {
		clauses = append(clauses, `COALESCE(tm.claimed_team_code, tm.pool_team_code, '') = ?`)
		args = append(args, strings.TrimSpace(query.OwnerTeamCode))
	}
	switch query.BusinessLane {
	case domain.TaskBusinessLaneCustomization:
		clauses = append(clauses, `COALESCE(t.business_lane, '') = ?`)
		args = append(args, string(domain.TaskBusinessLaneCustomization))
	case domain.TaskBusinessLaneNormal:
		clauses = append(clauses, `(COALESCE(t.business_lane, '') = '' OR t.business_lane = ?)`)
		args = append(args, string(domain.TaskBusinessLaneNormal))
	}
	if assetTypes := assetSearchSQLAssetTypes(query.AssetType); len(assetTypes) > 0 {
		placeholders := make([]string, 0, len(assetTypes))
		for _, assetType := range assetTypes {
			placeholders = append(placeholders, "?")
			args = append(args, string(assetType))
		}
		clauses = append(clauses, fmt.Sprintf(`ta.asset_type IN (%s)`, strings.Join(placeholders, ", ")))
	}
	if query.CreatedFrom != nil {
		clauses = append(clauses, taskAssetSearchTimeColumn(query)+` >= ?`)
		args = append(args, *query.CreatedFrom)
	}
	if query.CreatedTo != nil {
		clauses = append(clauses, taskAssetSearchTimeColumn(query)+` <= ?`)
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
	switch query.UsableState {
	case domain.AssetUsableStateFilterEditable:
		clauses = append(clauses, `ta.asset_type IN (?, ?, ?)`)
		args = append(args, string(domain.TaskAssetTypeDelivery), string(domain.TaskAssetTypeSource), string(domain.TaskAssetTypeReference))
		clauses = append(clauses, `COALESCE(ta.flow_review_status, '') NOT IN (?, ?)`)
		args = append(args, string(domain.TaskAssetFlowReviewStatusSuperseded), string(domain.TaskAssetFlowReviewStatusCleaned))
	case domain.AssetUsableStateFilterReadyForUse:
		clauses = append(clauses, `ta.flow_review_status = ?`)
		args = append(args, string(domain.TaskAssetFlowReviewStatusApproved))
	case domain.AssetUsableStateFilterPendingReview:
		clauses = append(clauses, `ta.flow_review_status = ?`)
		args = append(args, string(domain.TaskAssetFlowReviewStatusPendingReview))
	case domain.AssetUsableStateFilterRejected:
		clauses = append(clauses, `ta.flow_review_status = ?`)
		args = append(args, string(domain.TaskAssetFlowReviewStatusRejected))
	case domain.AssetUsableStateFilterHistory:
		clauses = append(clauses, `ta.flow_review_status = ?`)
		args = append(args, string(domain.TaskAssetFlowReviewStatusSuperseded))
	case domain.AssetUsableStateFilterCleaned:
		clauses = append(clauses, `ta.flow_review_status = ?`)
		args = append(args, string(domain.TaskAssetFlowReviewStatusCleaned))
	case domain.AssetUsableStateFilterOther:
		clauses = append(clauses, `(ta.flow_review_status IS NULL OR ta.flow_review_status = ? OR ta.flow_review_status = '')`)
		args = append(args, string(domain.TaskAssetFlowReviewStatusNotApplicable))
	}
	clauses, args = appendAssetFormatCategoryWhere(
		clauses,
		args,
		[]string{`LOWER(ta.file_name)`, `LOWER(COALESCE(ta.original_filename, ''))`},
		`LOWER(COALESCE(ta.mime_type, ''))`,
		query.FormatCategory,
	)
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func taskAssetSearchTimeColumn(query domain.AssetSearchQuery) string {
	switch query.Normalized().TimeBasis {
	case domain.AssetSearchTimeBasisTaskCreatedAt:
		return `t.created_at`
	default:
		return `ta.sort_time`
	}
}

func taskAssetSearchOrderBy(query domain.AssetSearchQuery) string {
	return taskAssetSearchTimeColumn(query) + ` DESC, ta.id DESC`
}

func assetSearchSQLAssetTypes(assetType domain.TaskAssetType) []domain.TaskAssetType {
	switch assetType.Canonical() {
	case domain.TaskAssetTypeDelivery:
		return []domain.TaskAssetType{
			domain.TaskAssetTypeDelivery,
			domain.TaskAssetTypeDraft,
			domain.TaskAssetTypeRevised,
			domain.TaskAssetTypeFinal,
			domain.TaskAssetTypeOutsourceReturn,
		}
	case domain.TaskAssetTypeReference:
		return []domain.TaskAssetType{domain.TaskAssetTypeReference}
	case domain.TaskAssetTypeSource:
		return []domain.TaskAssetType{domain.TaskAssetTypeSource, domain.TaskAssetTypeOriginal}
	case domain.TaskAssetTypePreview:
		return []domain.TaskAssetType{domain.TaskAssetTypePreview}
	case domain.TaskAssetTypeDesignThumb:
		return []domain.TaskAssetType{domain.TaskAssetTypeDesignThumb}
	case domain.TaskAssetTypeERPProduct:
		return []domain.TaskAssetType{domain.TaskAssetTypeERPProduct}
	default:
		return nil
	}
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
	var assetID, assetVersionNo, sourceTaskModuleID, archivedBy, approvedBy, rejectedBy, supersededByVersionID, sourceAssetVersionID, productID, operatorGroupID, requesterID, designerID, currentHandlerID, lastCustomizationOperatorID sql.NullInt64
	var scopeSKUCode, uploadMode, uploadRequestID, storageRefID, originalFilename, remoteFileID, mimeType, filePath, storageKey, wholeHash, uploadStatus, previewStatus, businessLane, customizationSourceType, warehouseRejectReason, warehouseRejectCategory sql.NullString
	var flowReviewStatus sql.NullString
	var fileSize sql.NullInt64
	var uploadedAt, archivedAt, cleanedAt, deletedAt, approvedAt, rejectedAt, supersededAt, cleanupAfterAt, deadlineAt sql.NullTime
	var needOutsource, isOutsource, customizationRequired, isBatchTask sql.NullBool
	var assetNo string
	var designCreatedBy int64
	var designCreatedAt, designUpdatedAt time.Time
	var ownerTeamCode string
	var taskCreatorUsername, taskCreatorName, assetCreatorUsername, assetCreatorName, uploadedByUsername, uploadedByName string
	if err := s.Scan(
		&a.ID, &a.TaskID, &assetID, &scopeSKUCode, &a.AssetType, &a.VersionNo, &assetVersionNo,
		&uploadMode, &uploadRequestID, &storageRefID, &a.FileName, &originalFilename, &remoteFileID,
		&mimeType, &fileSize, &filePath, &storageKey, &wholeHash, &uploadStatus, &previewStatus,
		&a.UploadedBy, &uploadedAt, &a.Remark, &a.CreatedAt,
		&a.SourceModuleKey, &sourceTaskModuleID, &a.IsArchived, &archivedAt, &archivedBy, &cleanedAt, &deletedAt,
		&flowReviewStatus, &approvedAt, &approvedBy, &rejectedAt, &rejectedBy, &supersededByVersionID, &supersededAt, &cleanupAfterAt, &sourceAssetVersionID,
		&t.ID, &t.TaskNo, &t.SourceMode, &productID, &t.SKUCode, &t.ProductNameSnapshot,
		&t.TaskType, &operatorGroupID, &t.OwnerTeam, &t.OwnerDepartment, &t.OwnerOrgTeam, &t.CreatorID, &requesterID,
		&designerID, &currentHandlerID, &t.TaskStatus, &t.Priority, &deadlineAt, &needOutsource, &isOutsource,
		&businessLane, &customizationRequired, &customizationSourceType, &lastCustomizationOperatorID, &warehouseRejectReason,
		&warehouseRejectCategory, &isBatchTask, &t.BatchItemCount, &t.BatchMode, &t.PrimarySKUCode,
		&t.SKUGenerationStatus, &t.CreatedAt, &t.UpdatedAt,
		&assetNo, &designCreatedBy, &designCreatedAt, &designUpdatedAt,
		&ownerTeamCode,
		&taskCreatorUsername, &taskCreatorName,
		&assetCreatorUsername, &assetCreatorName,
		&uploadedByUsername, &uploadedByName,
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
	if flowReviewStatus.Valid {
		a.FlowReviewStatus = domain.NormalizeTaskAssetFlowReviewStatus(domain.TaskAssetFlowReviewStatus(flowReviewStatus.String), a.AssetType)
	} else {
		a.FlowReviewStatus = domain.NormalizeTaskAssetFlowReviewStatus("", a.AssetType)
	}
	a.ApprovedAt = fromNullTime(approvedAt)
	a.ApprovedBy = fromNullInt64(approvedBy)
	a.RejectedAt = fromNullTime(rejectedAt)
	a.RejectedBy = fromNullInt64(rejectedBy)
	a.SupersededByVersionID = fromNullInt64(supersededByVersionID)
	a.SupersededAt = fromNullTime(supersededAt)
	a.CleanupAfterAt = fromNullTime(cleanupAfterAt)
	a.SourceAssetVersionID = fromNullInt64(sourceAssetVersionID)
	t.ProductID = fromNullInt64(productID)
	t.OperatorGroupID = fromNullInt64(operatorGroupID)
	t.RequesterID = fromNullInt64(requesterID)
	t.DesignerID = fromNullInt64(designerID)
	t.CurrentHandlerID = fromNullInt64(currentHandlerID)
	t.DeadlineAt = fromNullTime(deadlineAt)
	t.NeedOutsource = needOutsource.Valid && needOutsource.Bool
	t.IsOutsource = isOutsource.Valid && isOutsource.Bool
	t.CustomizationRequired = customizationRequired.Valid && customizationRequired.Bool
	if businessLane.Valid {
		t.BusinessLane = domain.NormalizeTaskBusinessLane(domain.TaskBusinessLane(businessLane.String), t.CustomizationRequired)
	} else {
		t.BusinessLane = domain.TaskBusinessLaneFromLegacy(t.CustomizationRequired)
	}
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
		Asset:                &a,
		Task:                 &t,
		AssetNo:              assetNo,
		DesignCreatedBy:      designCreatedBy,
		DesignCreatedAt:      designCreatedAt,
		DesignUpdatedAt:      designUpdatedAt,
		OwnerTeamCode:        ownerTeamCode,
		TaskCreatorUsername:  taskCreatorUsername,
		TaskCreatorName:      taskCreatorName,
		AssetCreatorUsername: assetCreatorUsername,
		AssetCreatorName:     assetCreatorName,
		UploadedByUsername:   uploadedByUsername,
		UploadedByName:       uploadedByName,
	}, nil
}

func buildListCurrentByAssetIDsQuery(assetIDs []int64) (string, []interface{}) {
	if len(assetIDs) == 0 {
		return "", nil
	}
	placeholders := make([]string, 0, len(assetIDs))
	args := make([]interface{}, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		if assetID <= 0 {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, assetID)
	}
	if len(placeholders) == 0 {
		return "", nil
	}
	query := taskAssetSearchSelect + taskAssetSearchFrom + `
		WHERE da.id IN (` + strings.Join(placeholders, ", ") + `)
		  AND ` + taskAssetCurrentVersionPredicate() + `
		ORDER BY ta.sort_time DESC, ta.id DESC`
	return query, args
}

func taskAssetCurrentVersionPredicate() string {
	if taskAssetLegacyCurrentVersionEnabled() {
		return `ta.id = COALESCE(da.current_version_id, (
		      SELECT ta2.id FROM task_assets ta2 WHERE ta2.asset_id = da.id ORDER BY ta2.asset_version_no DESC, ta2.id DESC LIMIT 1
		  ))`
	}
	return `ta.id = da.current_version_id`
}

func taskAssetLegacyCurrentVersionEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("ASSET_SEARCH_LEGACY_CURRENT_VERSION")))
	return value == "1" || value == "true" || value == "yes"
}
