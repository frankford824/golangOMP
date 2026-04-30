package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type taskAssetRepo struct{ db *DB }

func NewTaskAssetRepo(db *DB) repo.TaskAssetRepo { return &taskAssetRepo{db: db} }

const taskAssetSelectCols = `
	ta.id, ta.task_id, ta.asset_id, ta.scope_sku_code, ta.asset_type, ta.version_no, ta.asset_version_no, ta.upload_mode, ta.upload_request_id, ta.storage_ref_id,
	ta.file_name, ta.original_filename, ta.remote_file_id, ta.mime_type, ta.file_size, ta.file_path, ta.storage_key, ta.whole_hash, ta.upload_status, ta.preview_status, ta.uploaded_by, ta.uploaded_at, ta.remark, ta.created_at,
	asr.ref_id, asr.asset_id, asr.owner_type, asr.owner_id, asr.upload_request_id, asr.storage_adapter,
	asr.ref_type, asr.ref_key, asr.file_name, asr.mime_type, asr.file_size, asr.is_placeholder, asr.checksum_hint,
	asr.status, asr.created_at`

func (r *taskAssetRepo) Create(ctx context.Context, tx repo.Tx, asset *domain.TaskAsset) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO task_assets
		  (task_id, asset_id, scope_sku_code, asset_type, version_no, asset_version_no, upload_mode, upload_request_id, storage_ref_id, file_name, original_filename, remote_file_id, mime_type, file_size, file_path, storage_key, whole_hash, upload_status, preview_status, uploaded_by, uploaded_at, remark, source_module_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		asset.TaskID,
		toNullInt64(asset.AssetID),
		toNullString(asset.ScopeSKUCode),
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
		taskAssetSourceModuleKey(asset),
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
	var assetVersionNo sql.NullInt64
	var uploadMode, uploadRequestID, storageRefID sql.NullString
	var originalFilename, remoteFileID, mimeType, filePath, storageKey, wholeHash, uploadStatus, previewStatus sql.NullString
	var fileSize sql.NullInt64
	var uploadedAt sql.NullTime
	var refID, refOwnerType, refUploadRequestID, refStorageAdapter sql.NullString
	var refType, refKey, refFileName, refMimeType, refChecksumHint, refStatus sql.NullString
	var refAssetID, refOwnerID, refFileSize sql.NullInt64
	var refIsPlaceholder sql.NullBool
	var refCreatedAt sql.NullTime
	err := row.Scan(
		&asset.ID, &asset.TaskID, &assetID, &scopeSKUCode, &asset.AssetType, &asset.VersionNo, &assetVersionNo, &uploadMode, &uploadRequestID, &storageRefID,
		&asset.FileName, &originalFilename, &remoteFileID, &mimeType, &fileSize, &filePath, &storageKey, &wholeHash, &uploadStatus, &previewStatus, &asset.UploadedBy, &uploadedAt, &asset.Remark, &asset.CreatedAt,
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
	asset.AssetID = fromNullInt64(assetID)
	asset.ScopeSKUCode = fromNullString(scopeSKUCode)
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
	var assetVersionNo sql.NullInt64
	var uploadMode, uploadRequestID, storageRefID sql.NullString
	var originalFilename, remoteFileID, mimeType, filePath, storageKey, wholeHash, uploadStatus, previewStatus sql.NullString
	var fileSize sql.NullInt64
	var uploadedAt sql.NullTime
	var refID, refOwnerType, refUploadRequestID, refStorageAdapter sql.NullString
	var refType, refKey, refFileName, refMimeType, refChecksumHint, refStatus sql.NullString
	var refAssetID, refOwnerID, refFileSize sql.NullInt64
	var refIsPlaceholder sql.NullBool
	var refCreatedAt sql.NullTime
	if err := rows.Scan(
		&asset.ID, &asset.TaskID, &assetID, &scopeSKUCode, &asset.AssetType, &asset.VersionNo, &assetVersionNo, &uploadMode, &uploadRequestID, &storageRefID,
		&asset.FileName, &originalFilename, &remoteFileID, &mimeType, &fileSize, &filePath, &storageKey, &wholeHash, &uploadStatus, &previewStatus, &asset.UploadedBy, &uploadedAt, &asset.Remark, &asset.CreatedAt,
		&refID, &refAssetID, &refOwnerType, &refOwnerID, &refUploadRequestID, &refStorageAdapter,
		&refType, &refKey, &refFileName, &refMimeType, &refFileSize, &refIsPlaceholder, &refChecksumHint,
		&refStatus, &refCreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan task_asset row: %w", err)
	}
	asset.AssetID = fromNullInt64(assetID)
	asset.ScopeSKUCode = fromNullString(scopeSKUCode)
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
