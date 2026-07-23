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

type taskAssetRepo struct{ db *DB }

func NewTaskAssetRepo(db *DB) repo.TaskAssetRepo { return &taskAssetRepo{db: db} }

const taskAssetSelectCols = `
	ta.id, ta.task_id, ta.asset_id, ta.scope_sku_code, ta.retouch_requirement_id, ta.asset_type, ta.version_no, ta.asset_version_no, ta.upload_mode, ta.upload_request_id, ta.storage_ref_id,
	ta.file_name, ta.original_filename, ta.remote_file_id, ta.mime_type, ta.file_size, ta.file_path, ta.storage_key, ta.whole_hash, ta.upload_status, ta.preview_status, ta.uploaded_by, ta.uploaded_at, ta.remark, ta.created_at,
	ta.source_module_key, ta.source_task_module_id, COALESCE(ta.is_archived, 0), ta.archived_at, ta.archived_by, ta.cleaned_at, ta.deleted_at,
	ta.flow_review_status, ta.approved_at, ta.approved_by, ta.rejected_at, ta.rejected_by, ta.superseded_by_version_id, ta.superseded_at, ta.cleanup_after_at, ta.source_asset_version_id,
	asr.ref_id, asr.asset_id, asr.owner_type, asr.owner_id, asr.upload_request_id, asr.storage_adapter,
	asr.ref_type, asr.ref_key, asr.file_name, asr.mime_type, asr.file_size, asr.is_placeholder, asr.checksum_hint,
	asr.status, asr.created_at`

const taskAssetInsertSQL = `
			INSERT INTO task_assets
			  (task_id, asset_id, scope_sku_code, retouch_requirement_id, asset_type, version_no, asset_version_no, upload_mode, upload_request_id, storage_ref_id, file_name, original_filename, remote_file_id, mime_type, file_size, file_path, storage_key, whole_hash, upload_status, preview_status, uploaded_by, uploaded_at, remark, source_module_key, source_task_module_id, flow_review_status, approved_at, approved_by, source_asset_version_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, (
			  SELECT tm.id
			    FROM task_modules tm
			   WHERE tm.task_id = ?
			     AND tm.module_key = ?
			   LIMIT 1
			)), ?, ?, ?, ?)`

func (r *taskAssetRepo) Create(ctx context.Context, tx repo.Tx, asset *domain.TaskAsset) (int64, error) {
	sqlTx := Unwrap(tx)
	moduleKey := taskAssetSourceModuleKey(asset)
	res, err := sqlTx.ExecContext(ctx, taskAssetInsertSQL,
		asset.TaskID,
		toNullInt64(asset.AssetID),
		toNullString(asset.ScopeSKUCode),
		toNullInt64(asset.RetouchRequirementID),
		string(domain.NormalizeTaskAssetType(asset.AssetType)),
		asset.VersionNo,
		toNullInt(asset.AssetVersionNo),
		toNullString(asset.UploadMode),
		toNullString(asset.UploadRequestID),
		toNullString(asset.StorageRefID),
		asset.FileName,
		toNullString(asset.OriginalName),
		toNullString(asset.RemoteFileID),
		toNullString(asset.MimeType),
		toNullInt64(asset.FileSize),
		toNullString(asset.FilePath),
		toNullString(asset.StorageKey),
		toNullString(asset.WholeHash),
		toNullString(asset.UploadStatus),
		toNullString(asset.PreviewStatus),
		asset.UploadedBy,
		toNullTime(asset.UploadedAt),
		asset.Remark,
		moduleKey,
		toNullInt64(asset.SourceTaskModuleID),
		asset.TaskID,
		moduleKey,
		string(domain.NormalizeTaskAssetFlowReviewStatus(asset.FlowReviewStatus, asset.AssetType)),
		toNullTime(asset.ApprovedAt),
		toNullInt64(asset.ApprovedBy),
		toNullInt64(asset.SourceAssetVersionID),
	)
	if err != nil {
		return 0, fmt.Errorf("insert task_asset: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := reindexTaskSearchDocument(ctx, sqlTx, asset.TaskID); err != nil {
		return 0, err
	}
	return id, nil
}

// MarkBindingStaged makes a completed source/final upload available only to the
// resource-group binding workflow. It intentionally does not touch the legacy
// design_assets.current_version_id pointer.
func (r *taskAssetRepo) MarkBindingStaged(ctx context.Context, tx repo.Tx, taskAssetID, taskID, actorID int64, scopeSKUCode string, retouchRequirementID *int64, role, uploadSessionID string, expiresAt time.Time) error {
	result, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE task_assets
		SET binding_state = 'staged',
		    bound_group_id = NULL,
		    bound_role = NULL,
		    staged_task_sku_item_id = (
		      SELECT tsi.id FROM task_sku_items tsi
		      WHERE tsi.task_id = ? AND tsi.sku_code = NULLIF(?, '')
		      LIMIT 1
		    ),
		    staged_retouch_requirement_id = ?,
		    staged_role = ?,
		    staged_by = ?,
		    upload_session_id = ?,
		    staged_expires_at = ?,
		    access_revoked_at = NULL,
		    access_revoked_reason = ''
		WHERE id = ? AND task_id = ?`,
		taskID, strings.TrimSpace(scopeSKUCode), toNullInt64(retouchRequirementID), role, actorID,
		strings.TrimSpace(uploadSessionID), expiresAt, taskAssetID, taskID)
	if err != nil {
		return fmt.Errorf("mark task asset staged: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return repo.ErrConflict
	}
	return nil
}

func taskAssetSourceModuleKey(asset *domain.TaskAsset) string {
	if asset == nil || asset.SourceModuleKey == "" {
		return domain.ModuleKeyDesign
	}
	return asset.SourceModuleKey
}

func (r *taskAssetRepo) GetByID(ctx context.Context, id int64) (*domain.TaskAsset, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT `+taskAssetSelectCols+`
		FROM task_assets ta
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		WHERE ta.id = ?`, id)
	return scanTaskAsset(row)
}

// GetBoundRevisionTaskAssetByID resolves a task_assets identity only when it is
// still an active, bound member of at least one immutable resource-group
// revision. It is intentionally separate from design_assets identity and is
// used by the controlled historical preview/download routes.
func (r *taskAssetRepo) GetBoundRevisionTaskAssetByID(ctx context.Context, id int64) (*domain.TaskAsset, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT `+taskAssetSelectCols+`
		FROM task_assets ta
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		WHERE ta.id = ?
		  AND ta.binding_state = 'bound'
		  AND ta.deleted_at IS NULL
		  AND ta.cleaned_at IS NULL
		  AND ta.access_revoked_at IS NULL
		  AND ta.object_deleted_at IS NULL
		  AND (
		    EXISTS (
		      SELECT 1
		      FROM task_asset_group_revisions revision
		      WHERE revision.source_task_asset_id = ta.id
		    )
		    OR EXISTS (
		      SELECT 1
		      FROM task_asset_group_revision_items item
		      WHERE item.task_asset_id = ta.id
		    )
		    OR EXISTS (
		      SELECT 1
		      FROM task_asset_group_revision_references reference
		      WHERE reference.formal_task_asset_id = ta.id
		    )
		  )`, id)
	return scanTaskAsset(row)
}

// GetByIDForUpdate returns one version while holding its task_assets row lock.
// State-sensitive services discover this capability through a local optional
// interface so the broad TaskAssetRepo contract does not need to grow.
func (r *taskAssetRepo) GetByIDForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.TaskAsset, error) {
	row := Unwrap(tx).QueryRowContext(ctx, `
		SELECT `+taskAssetSelectCols+`
		FROM task_assets ta
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		WHERE ta.id = ?
		FOR UPDATE`, id)
	return scanTaskAsset(row)
}

func (r *taskAssetRepo) GetByStorageKey(ctx context.Context, storageKey string) (*domain.TaskAsset, error) {
	key := strings.TrimSpace(storageKey)
	if key == "" {
		return nil, nil
	}
	row := r.db.db.QueryRowContext(ctx, `
		SELECT `+taskAssetSelectCols+`
		FROM task_assets ta
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		WHERE ta.storage_key = ? OR asr.ref_key = ?
		ORDER BY ta.created_at DESC
		LIMIT 1`, key, key)
	return scanTaskAsset(row)
}

func (r *taskAssetRepo) ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskAsset, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT `+taskAssetSelectCols+`
		FROM task_assets ta
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		WHERE ta.task_id = ?
		ORDER BY ta.version_no ASC, ta.created_at ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task_assets: %w", err)
	}
	defer rows.Close()

	var assets []*domain.TaskAsset
	for rows.Next() {
		asset, err := scanTaskAssetRow(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (r *taskAssetRepo) ListByAssetID(ctx context.Context, assetID int64) ([]*domain.TaskAsset, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT `+taskAssetSelectCols+`
		FROM task_assets ta
		LEFT JOIN asset_storage_refs asr ON asr.ref_id = ta.storage_ref_id
		WHERE ta.asset_id = ?
		ORDER BY ta.asset_version_no ASC, ta.created_at ASC`, assetID)
	if err != nil {
		return nil, fmt.Errorf("list task_assets by asset_id: %w", err)
	}
	defer rows.Close()

	var assets []*domain.TaskAsset
	for rows.Next() {
		asset, err := scanTaskAssetRow(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

// GetStagedPreviewAccessByDesignAssetID returns only an active staged version.
// Bound/finalized resources must be read through the resource-group authority;
// this projection exists solely for the short-lived uploader/auditor preview.
func (r *taskAssetRepo) GetStagedPreviewAccessByDesignAssetID(ctx context.Context, assetID int64) (*domain.StagedTaskAssetPreviewAccess, error) {
	var item domain.StagedTaskAssetPreviewAccess
	err := r.db.db.QueryRowContext(ctx, `
		SELECT ta.id, ta.task_id, COALESCE(ta.staged_by, ta.uploaded_by)
		FROM task_assets ta
		WHERE ta.asset_id = ?
		  AND ta.binding_state = 'staged'
		  AND ta.deleted_at IS NULL
		  AND ta.cleaned_at IS NULL
		  AND ta.access_revoked_at IS NULL
		  AND ta.object_deleted_at IS NULL
		  AND (ta.staged_expires_at IS NULL OR ta.staged_expires_at > CURRENT_TIMESTAMP)
		ORDER BY ta.asset_version_no DESC, ta.id DESC
		LIMIT 1`, assetID).Scan(&item.TaskAssetID, &item.TaskID, &item.StagedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get staged preview access by design asset: %w", err)
	}
	return &item, nil
}

func (r *taskAssetRepo) NextVersionNo(ctx context.Context, tx repo.Tx, taskID int64) (int, error) {
	sqlTx := Unwrap(tx)

	var current int
	if err := sqlTx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version_no), 0) FROM task_assets WHERE task_id = ? FOR UPDATE`,
		taskID,
	).Scan(&current); err != nil {
		return 0, fmt.Errorf("task_asset next version: %w", err)
	}
	return current + 1, nil
}

func (r *taskAssetRepo) NextAssetVersionNo(ctx context.Context, tx repo.Tx, assetID int64) (int, error) {
	sqlTx := Unwrap(tx)

	var current int
	if err := sqlTx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(asset_version_no), 0) FROM task_assets WHERE asset_id = ? FOR UPDATE`,
		assetID,
	).Scan(&current); err != nil {
		return 0, fmt.Errorf("task_asset next asset version: %w", err)
	}
	return current + 1, nil
}

func scanTaskAsset(row *sql.Row) (*domain.TaskAsset, error) {
	var asset domain.TaskAsset
	var assetID sql.NullInt64
	var scopeSKUCode sql.NullString
	var retouchRequirementID sql.NullInt64
	var assetVersionNo sql.NullInt64
	var uploadMode, uploadRequestID, storageRefID sql.NullString
	var originalFilename, remoteFileID, mimeType, filePath, storageKey, wholeHash, uploadStatus, previewStatus sql.NullString
	var fileSize sql.NullInt64
	var uploadedAt sql.NullTime
	var flowReviewStatus sql.NullString
	var archivedAt, cleanedAt, deletedAt sql.NullTime
	var approvedAt, rejectedAt, supersededAt, cleanupAfterAt sql.NullTime
	var sourceTaskModuleID, archivedBy, approvedBy, rejectedBy, supersededByVersionID, sourceAssetVersionID sql.NullInt64
	var isArchived bool
	var refID, refOwnerType, refUploadRequestID, refStorageAdapter sql.NullString
	var refType, refKey, refFileName, refMimeType, refChecksumHint, refStatus sql.NullString
	var refAssetID, refOwnerID, refFileSize sql.NullInt64
	var refIsPlaceholder sql.NullBool
	var refCreatedAt sql.NullTime
	err := row.Scan(
		&asset.ID, &asset.TaskID, &assetID, &scopeSKUCode, &retouchRequirementID, &asset.AssetType, &asset.VersionNo, &assetVersionNo, &uploadMode, &uploadRequestID, &storageRefID,
		&asset.FileName, &originalFilename, &remoteFileID, &mimeType, &fileSize, &filePath, &storageKey, &wholeHash, &uploadStatus, &previewStatus, &asset.UploadedBy, &uploadedAt, &asset.Remark, &asset.CreatedAt,
		&asset.SourceModuleKey, &sourceTaskModuleID, &isArchived, &archivedAt, &archivedBy, &cleanedAt, &deletedAt,
		&flowReviewStatus, &approvedAt, &approvedBy, &rejectedAt, &rejectedBy, &supersededByVersionID, &supersededAt, &cleanupAfterAt, &sourceAssetVersionID,
		&refID, &refAssetID, &refOwnerType, &refOwnerID, &refUploadRequestID, &refStorageAdapter,
		&refType, &refKey, &refFileName, &refMimeType, &refFileSize, &refIsPlaceholder, &refChecksumHint,
		&refStatus, &refCreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan task_asset: %w", err)
	}
	asset.SourceModuleKey = strings.TrimSpace(asset.SourceModuleKey)
	asset.AssetID = fromNullInt64(assetID)
	asset.ScopeSKUCode = fromNullString(scopeSKUCode)
	asset.RetouchRequirementID = fromNullInt64(retouchRequirementID)
	asset.AssetType = domain.NormalizeTaskAssetType(asset.AssetType)
	asset.AssetVersionNo = fromNullInt(assetVersionNo)
	asset.UploadMode = fromNullString(uploadMode)
	asset.UploadRequestID = fromNullString(uploadRequestID)
	asset.StorageRefID = fromNullString(storageRefID)
	asset.OriginalName = fromNullString(originalFilename)
	asset.RemoteFileID = fromNullString(remoteFileID)
	asset.MimeType = fromNullString(mimeType)
	asset.FileSize = fromNullInt64(fileSize)
	asset.FilePath = fromNullString(filePath)
	asset.StorageKey = fromNullString(storageKey)
	asset.WholeHash = fromNullString(wholeHash)
	asset.UploadStatus = fromNullString(uploadStatus)
	asset.PreviewStatus = fromNullString(previewStatus)
	asset.UploadedAt = fromNullTime(uploadedAt)
	asset.SourceTaskModuleID = fromNullInt64(sourceTaskModuleID)
	asset.IsArchived = isArchived
	asset.ArchivedAt = fromNullTime(archivedAt)
	asset.ArchivedBy = fromNullInt64(archivedBy)
	asset.CleanedAt = fromNullTime(cleanedAt)
	asset.DeletedAt = fromNullTime(deletedAt)
	if flowReviewStatus.Valid {
		asset.FlowReviewStatus = domain.NormalizeTaskAssetFlowReviewStatus(domain.TaskAssetFlowReviewStatus(flowReviewStatus.String), asset.AssetType)
	} else {
		asset.FlowReviewStatus = domain.NormalizeTaskAssetFlowReviewStatus("", asset.AssetType)
	}
	asset.ApprovedAt = fromNullTime(approvedAt)
	asset.ApprovedBy = fromNullInt64(approvedBy)
	asset.RejectedAt = fromNullTime(rejectedAt)
	asset.RejectedBy = fromNullInt64(rejectedBy)
	asset.SupersededByVersionID = fromNullInt64(supersededByVersionID)
	asset.SupersededAt = fromNullTime(supersededAt)
	asset.CleanupAfterAt = fromNullTime(cleanupAfterAt)
	asset.SourceAssetVersionID = fromNullInt64(sourceAssetVersionID)
	asset.StorageRef = buildAssetStorageRef(
		refID,
		refAssetID,
		refOwnerType,
		refOwnerID,
		refUploadRequestID,
		refStorageAdapter,
		refType,
		refKey,
		refFileName,
		refMimeType,
		refFileSize,
		refIsPlaceholder,
		refChecksumHint,
		refStatus,
		refCreatedAt,
	)
	return &asset, nil
}

func scanTaskAssetRow(rows *sql.Rows) (*domain.TaskAsset, error) {
	var asset domain.TaskAsset
	var assetID sql.NullInt64
	var scopeSKUCode sql.NullString
	var retouchRequirementID sql.NullInt64
	var assetVersionNo sql.NullInt64
	var uploadMode, uploadRequestID, storageRefID sql.NullString
	var originalFilename, remoteFileID, mimeType, filePath, storageKey, wholeHash, uploadStatus, previewStatus sql.NullString
	var fileSize sql.NullInt64
	var uploadedAt sql.NullTime
	var flowReviewStatus sql.NullString
	var archivedAt, cleanedAt, deletedAt sql.NullTime
	var approvedAt, rejectedAt, supersededAt, cleanupAfterAt sql.NullTime
	var sourceTaskModuleID, archivedBy, approvedBy, rejectedBy, supersededByVersionID, sourceAssetVersionID sql.NullInt64
	var isArchived bool
	var refID, refOwnerType, refUploadRequestID, refStorageAdapter sql.NullString
	var refType, refKey, refFileName, refMimeType, refChecksumHint, refStatus sql.NullString
	var refAssetID, refOwnerID, refFileSize sql.NullInt64
	var refIsPlaceholder sql.NullBool
	var refCreatedAt sql.NullTime
	if err := rows.Scan(
		&asset.ID, &asset.TaskID, &assetID, &scopeSKUCode, &retouchRequirementID, &asset.AssetType, &asset.VersionNo, &assetVersionNo, &uploadMode, &uploadRequestID, &storageRefID,
		&asset.FileName, &originalFilename, &remoteFileID, &mimeType, &fileSize, &filePath, &storageKey, &wholeHash, &uploadStatus, &previewStatus, &asset.UploadedBy, &uploadedAt, &asset.Remark, &asset.CreatedAt,
		&asset.SourceModuleKey, &sourceTaskModuleID, &isArchived, &archivedAt, &archivedBy, &cleanedAt, &deletedAt,
		&flowReviewStatus, &approvedAt, &approvedBy, &rejectedAt, &rejectedBy, &supersededByVersionID, &supersededAt, &cleanupAfterAt, &sourceAssetVersionID,
		&refID, &refAssetID, &refOwnerType, &refOwnerID, &refUploadRequestID, &refStorageAdapter,
		&refType, &refKey, &refFileName, &refMimeType, &refFileSize, &refIsPlaceholder, &refChecksumHint,
		&refStatus, &refCreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan task_asset row: %w", err)
	}
	asset.SourceModuleKey = strings.TrimSpace(asset.SourceModuleKey)
	asset.AssetID = fromNullInt64(assetID)
	asset.ScopeSKUCode = fromNullString(scopeSKUCode)
	asset.RetouchRequirementID = fromNullInt64(retouchRequirementID)
	asset.AssetType = domain.NormalizeTaskAssetType(asset.AssetType)
	asset.AssetVersionNo = fromNullInt(assetVersionNo)
	asset.UploadMode = fromNullString(uploadMode)
	asset.UploadRequestID = fromNullString(uploadRequestID)
	asset.StorageRefID = fromNullString(storageRefID)
	asset.OriginalName = fromNullString(originalFilename)
	asset.RemoteFileID = fromNullString(remoteFileID)
	asset.MimeType = fromNullString(mimeType)
	asset.FileSize = fromNullInt64(fileSize)
	asset.FilePath = fromNullString(filePath)
	asset.StorageKey = fromNullString(storageKey)
	asset.WholeHash = fromNullString(wholeHash)
	asset.UploadStatus = fromNullString(uploadStatus)
	asset.PreviewStatus = fromNullString(previewStatus)
	asset.UploadedAt = fromNullTime(uploadedAt)
	asset.SourceTaskModuleID = fromNullInt64(sourceTaskModuleID)
	asset.IsArchived = isArchived
	asset.ArchivedAt = fromNullTime(archivedAt)
	asset.ArchivedBy = fromNullInt64(archivedBy)
	asset.CleanedAt = fromNullTime(cleanedAt)
	asset.DeletedAt = fromNullTime(deletedAt)
	if flowReviewStatus.Valid {
		asset.FlowReviewStatus = domain.NormalizeTaskAssetFlowReviewStatus(domain.TaskAssetFlowReviewStatus(flowReviewStatus.String), asset.AssetType)
	} else {
		asset.FlowReviewStatus = domain.NormalizeTaskAssetFlowReviewStatus("", asset.AssetType)
	}
	asset.ApprovedAt = fromNullTime(approvedAt)
	asset.ApprovedBy = fromNullInt64(approvedBy)
	asset.RejectedAt = fromNullTime(rejectedAt)
	asset.RejectedBy = fromNullInt64(rejectedBy)
	asset.SupersededByVersionID = fromNullInt64(supersededByVersionID)
	asset.SupersededAt = fromNullTime(supersededAt)
	asset.CleanupAfterAt = fromNullTime(cleanupAfterAt)
	asset.SourceAssetVersionID = fromNullInt64(sourceAssetVersionID)
	asset.StorageRef = buildAssetStorageRef(
		refID,
		refAssetID,
		refOwnerType,
		refOwnerID,
		refUploadRequestID,
		refStorageAdapter,
		refType,
		refKey,
		refFileName,
		refMimeType,
		refFileSize,
		refIsPlaceholder,
		refChecksumHint,
		refStatus,
		refCreatedAt,
	)
	return &asset, nil
}

func buildAssetStorageRef(
	refID sql.NullString,
	refAssetID sql.NullInt64,
	refOwnerType sql.NullString,
	refOwnerID sql.NullInt64,
	refUploadRequestID sql.NullString,
	refStorageAdapter sql.NullString,
	refType sql.NullString,
	refKey sql.NullString,
	refFileName sql.NullString,
	refMimeType sql.NullString,
	refFileSize sql.NullInt64,
	refIsPlaceholder sql.NullBool,
	refChecksumHint sql.NullString,
	refStatus sql.NullString,
	refCreatedAt sql.NullTime,
) *domain.AssetStorageRef {
	if !refID.Valid || refID.String == "" {
		return nil
	}
	ref := &domain.AssetStorageRef{
		RefID:           refID.String,
		AssetID:         fromNullInt64(refAssetID),
		OwnerType:       domain.AssetOwnerType(refOwnerType.String),
		OwnerID:         refOwnerID.Int64,
		UploadRequestID: "",
		StorageAdapter:  domain.AssetStorageAdapter(refStorageAdapter.String),
		RefType:         domain.AssetStorageRefType(refType.String),
		RefKey:          refKey.String,
		FileName:        refFileName.String,
		MimeType:        refMimeType.String,
		FileSize:        fromNullInt64(refFileSize),
		IsPlaceholder:   refIsPlaceholder.Valid && refIsPlaceholder.Bool,
		ChecksumHint:    refChecksumHint.String,
		Status:          domain.AssetStorageRefStatus(refStatus.String),
	}
	if refUploadRequestID.Valid {
		ref.UploadRequestID = refUploadRequestID.String
	}
	if refCreatedAt.Valid {
		ref.CreatedAt = refCreatedAt.Time
	}
	domain.HydrateAssetStorageRefDerived(ref)
	return ref
}
