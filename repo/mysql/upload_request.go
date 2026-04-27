package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

type uploadRequestRepo struct{ db *DB }

func NewUploadRequestRepo(db *DB) repo.UploadRequestRepo {
	return &uploadRequestRepo{db: db}
}

const uploadRequestSelectCols = `
	request_id, owner_type, owner_id, task_id, asset_id, source_asset_id, target_sku_code, task_asset_type, storage_adapter, upload_mode, ref_type,
	file_name, mime_type, file_size, expected_size, checksum_hint, storage_provider, status, session_status, remote_upload_id, remote_file_id,
	is_placeholder, bound_asset_id, bound_ref_id, created_by, expires_at, last_synced_at, remark, created_at, updated_at`

func (r *uploadRequestRepo) Create(ctx context.Context, tx repo.Tx, request *domain.UploadRequest) (*domain.UploadRequest, error) {
	if request == nil {
		return nil, fmt.Errorf("create upload request: request is nil")
	}
	sqlTx := Unwrap(tx)
	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	now := request.CreatedAt.UTC()
	if request.CreatedAt.IsZero() {
		now = time.Now().UTC()
	}
	updatedAt := request.UpdatedAt.UTC()
	if request.UpdatedAt.IsZero() {
		updatedAt = now
	}
	var taskAssetType string
	if request.TaskAssetType != nil {
		taskAssetType = string(*request.TaskAssetType)
	}
	taskID := request.TaskID
	if taskID == 0 && request.OwnerType == domain.AssetOwnerTypeTask {
		taskID = request.OwnerID
	}
	if !request.UploadMode.Valid() {
		request.UploadMode = domain.DesignAssetUploadModeSmall
	}
	if !request.StorageProvider.Valid() {
		request.StorageProvider = domain.DesignAssetStorageProviderOSS
	}
	if !request.SessionStatus.Valid() {
		request.SessionStatus = domain.DesignAssetSessionStatusCreated
	}
	var taskIDPtr *int64
	if taskID > 0 {
		taskIDPtr = &taskID
	}
	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO upload_requests (
			request_id, owner_type, owner_id, task_id, asset_id, source_asset_id, target_sku_code, task_asset_type, storage_adapter, upload_mode, ref_type,
			file_name, mime_type, file_size, expected_size, checksum_hint, storage_provider, status, session_status, remote_upload_id, remote_file_id,
			is_placeholder, bound_asset_id, bound_ref_id, created_by, expires_at, last_synced_at, remark, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		requestID,
		string(request.OwnerType),
		request.OwnerID,
		toNullInt64(taskIDPtr),
		toNullInt64(request.AssetID),
		toNullInt64(request.SourceAssetID),
		strings.TrimSpace(request.TargetSKUCode),
		sql.NullString{String: taskAssetType, Valid: taskAssetType != ""},
		string(request.StorageAdapter),
		string(request.UploadMode),
		string(request.RefType),
		strings.TrimSpace(request.FileName),
		strings.TrimSpace(request.MimeType),
		toNullInt64(request.FileSize),
		toNullInt64(request.ExpectedSize),
		strings.TrimSpace(request.ChecksumHint),
		string(request.StorageProvider),
		string(request.Status),
		string(request.SessionStatus),
		strings.TrimSpace(request.RemoteUploadID),
		strings.TrimSpace(request.RemoteFileID),
		request.IsPlaceholder,
		toNullInt64(request.BoundAssetID),
		strings.TrimSpace(request.BoundRefID),
		sql.NullInt64{Int64: request.CreatedBy, Valid: request.CreatedBy > 0},
		toNullTime(request.ExpiresAt),
		toNullTime(request.LastSyncedAt),
		strings.TrimSpace(request.Remark),
		now,
		updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert upload request: %w", err)
	}
	copyRequest := *request
	copyRequest.RequestID = requestID
	copyRequest.TaskID = taskID
	copyRequest.CreatedAt = now
	copyRequest.UpdatedAt = updatedAt
	if copyRequest.LastSyncedAt == nil {
		copyRequest.LastSyncedAt = &updatedAt
	}
	domain.HydrateUploadRequestDerived(&copyRequest)
	return &copyRequest, nil
}

func (r *uploadRequestRepo) GetByRequestID(ctx context.Context, requestID string) (*domain.UploadRequest, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT `+uploadRequestSelectCols+`
		FROM upload_requests
		WHERE request_id = ?`, strings.TrimSpace(requestID))
	request, err := scanUploadRequest(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get upload request: %w", err)
	}
	return request, nil
}

func (r *uploadRequestRepo) List(ctx context.Context, filter repo.UploadRequestListFilter) ([]*domain.UploadRequest, int64, error) {
	page, pageSize := normalizeUploadRequestPage(filter.Page, filter.PageSize)
	whereParts := []string{"1=1"}
	args := make([]interface{}, 0, 8)

	if filter.OwnerType != nil {
		whereParts = append(whereParts, "owner_type = ?")
		args = append(args, string(*filter.OwnerType))
	}
	if filter.OwnerID != nil {
		whereParts = append(whereParts, "owner_id = ?")
		args = append(args, *filter.OwnerID)
	}
	if filter.TaskAssetType != nil {
		whereParts = append(whereParts, "task_asset_type = ?")
		args = append(args, string(*filter.TaskAssetType))
	}
	if filter.Status != nil {
		whereParts = append(whereParts, "status = ?")
		args = append(args, string(*filter.Status))
	}
	whereSQL := strings.Join(whereParts, " AND ")

	var total int64
	if err := r.db.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM upload_requests
		WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count upload requests: %w", err)
	}

	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT `+uploadRequestSelectCols+`
		FROM upload_requests
		WHERE `+whereSQL+`
		ORDER BY created_at DESC, request_id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list upload requests: %w", err)
	}
	defer rows.Close()

	requests := make([]*domain.UploadRequest, 0)
	for rows.Next() {
		request, err := scanUploadRequest(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan upload request: %w", err)
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate upload requests: %w", err)
	}
	return requests, total, nil
}

func (r *uploadRequestRepo) UpdateLifecycle(ctx context.Context, tx repo.Tx, update repo.UploadRequestLifecycleUpdate) error {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE upload_requests
		SET status = ?, remark = ?, updated_at = ?
		WHERE request_id = ?`,
		string(update.Status),
		strings.TrimSpace(update.Remark),
		time.Now().UTC(),
		strings.TrimSpace(update.RequestID),
	)
	if err != nil {
		return fmt.Errorf("update upload request lifecycle: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update upload request lifecycle rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *uploadRequestRepo) UpdateBinding(ctx context.Context, tx repo.Tx, requestID string, boundAssetID *int64, boundRefID string, status domain.UploadRequestStatus, remark string) error {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE upload_requests
		SET bound_asset_id = ?, bound_ref_id = ?, status = ?, remark = ?, updated_at = ?
		WHERE request_id = ?`,
		toNullInt64(boundAssetID),
		strings.TrimSpace(boundRefID),
		string(status),
		strings.TrimSpace(remark),
		time.Now().UTC(),
		strings.TrimSpace(requestID),
	)
	if err != nil {
		return fmt.Errorf("update upload request binding: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update upload request binding rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *uploadRequestRepo) UpdateSession(ctx context.Context, tx repo.Tx, update repo.UploadRequestSessionUpdate) error {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE upload_requests
		SET asset_id = COALESCE(?, asset_id),
			session_status = ?,
			remote_upload_id = ?,
			remote_file_id = COALESCE(?, remote_file_id),
			created_by = COALESCE(?, created_by),
			expires_at = ?,
			last_synced_at = COALESCE(?, last_synced_at),
			remark = ?,
			updated_at = ?
		WHERE request_id = ?`,
		toNullInt64(update.AssetID),
		string(update.SessionStatus),
		strings.TrimSpace(update.RemoteUploadID),
		toNullString(update.RemoteFileID),
		toNullInt64(update.CreatedBy),
		toNullTime(update.ExpiresAt),
		toNullTime(update.LastSyncedAt),
		strings.TrimSpace(update.Remark),
		time.Now().UTC(),
		strings.TrimSpace(update.RequestID),
	)
	if err != nil {
		return fmt.Errorf("update upload request session: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update upload request session rows affected: %w", err)
	}
	if rowsAffected == 0 {
		var existing int
		if err := sqlTx.QueryRowContext(ctx,
			`SELECT 1 FROM upload_requests WHERE request_id = ? LIMIT 1`,
			strings.TrimSpace(update.RequestID),
		).Scan(&existing); err != nil {
			if err == sql.ErrNoRows {
				return sql.ErrNoRows
			}
			return fmt.Errorf("verify upload request session existence: %w", err)
		}
		return nil
	}
	return nil
}

func scanUploadRequest(scanner interface {
	Scan(...interface{}) error
}) (*domain.UploadRequest, error) {
	request := &domain.UploadRequest{}
	var taskAssetType sql.NullString
	var taskID, assetID, sourceAssetID, fileSize, expectedSize, boundAssetID, createdBy sql.NullInt64
	var targetSKUCode, uploadMode, storageProvider, sessionStatus, remoteUploadID, remoteFileID sql.NullString
	var expiresAt, lastSyncedAt sql.NullTime
	if err := scanner.Scan(
		&request.RequestID,
		&request.OwnerType,
		&request.OwnerID,
		&taskID,
		&assetID,
		&sourceAssetID,
		&targetSKUCode,
		&taskAssetType,
		&request.StorageAdapter,
		&uploadMode,
		&request.RefType,
		&request.FileName,
		&request.MimeType,
		&fileSize,
		&expectedSize,
		&request.ChecksumHint,
		&storageProvider,
		&request.Status,
		&sessionStatus,
		&remoteUploadID,
		&remoteFileID,
		&request.IsPlaceholder,
		&boundAssetID,
		&request.BoundRefID,
		&createdBy,
		&expiresAt,
		&lastSyncedAt,
		&request.Remark,
		&request.CreatedAt,
		&request.UpdatedAt,
	); err != nil {
		return nil, err
	}
	request.TaskID = taskID.Int64
	request.AssetID = fromNullInt64(assetID)
	request.SourceAssetID = fromNullInt64(sourceAssetID)
	if targetSKUCode.Valid {
		request.TargetSKUCode = targetSKUCode.String
	}
	request.FileSize = fromNullInt64(fileSize)
	request.ExpectedSize = fromNullInt64(expectedSize)
	request.BoundAssetID = fromNullInt64(boundAssetID)
	request.CreatedBy = createdBy.Int64
	request.ExpiresAt = fromNullTime(expiresAt)
	if taskAssetType.Valid {
		assetType := domain.TaskAssetType(taskAssetType.String)
		request.TaskAssetType = &assetType
	}
	if uploadMode.Valid {
		request.UploadMode = domain.DesignAssetUploadMode(uploadMode.String)
	}
	if storageProvider.Valid {
		request.StorageProvider = domain.DesignAssetStorageProvider(storageProvider.String)
	}
	if sessionStatus.Valid {
		request.SessionStatus = domain.DesignAssetSessionStatus(sessionStatus.String)
	}
	if remoteUploadID.Valid {
		request.RemoteUploadID = remoteUploadID.String
	}
	if remoteFileID.Valid {
		request.RemoteFileID = remoteFileID.String
	}
	request.LastSyncedAt = fromNullTime(lastSyncedAt)
	domain.HydrateUploadRequestDerived(request)
	return request, nil
}

func normalizeUploadRequestPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
