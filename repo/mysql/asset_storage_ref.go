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

type assetStorageRefRepo struct{ db *DB }

func NewAssetStorageRefRepo(db *DB) repo.AssetStorageRefRepo {
	return &assetStorageRefRepo{db: db}
}

const assetStorageRefSelectCols = `
	ref_id, asset_id, owner_type, owner_id, upload_request_id, storage_adapter, ref_type,
	ref_key, file_name, mime_type, file_size, is_placeholder, checksum_hint, status, created_at`

func (r *assetStorageRefRepo) Create(ctx context.Context, tx repo.Tx, ref *domain.AssetStorageRef) (*domain.AssetStorageRef, error) {
	if ref == nil {
		return nil, fmt.Errorf("create asset storage ref: ref is nil")
	}
	sqlTx := Unwrap(tx)
	refID := strings.TrimSpace(ref.RefID)
	if refID == "" {
		refID = uuid.NewString()
	}
	createdAt := ref.CreatedAt.UTC()
	if ref.CreatedAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	var uploadRequestID *string
	if trimmed := strings.TrimSpace(ref.UploadRequestID); trimmed != "" {
		uploadRequestID = &trimmed
	}
	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO asset_storage_refs (
			ref_id, asset_id, owner_type, owner_id, upload_request_id, storage_adapter, ref_type,
			ref_key, file_name, mime_type, file_size, is_placeholder, checksum_hint, status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		refID,
		toNullInt64(ref.AssetID),
		string(ref.OwnerType),
		ref.OwnerID,
		toNullString(uploadRequestID),
		string(ref.StorageAdapter),
		string(ref.RefType),
		strings.TrimSpace(ref.RefKey),
		strings.TrimSpace(ref.FileName),
		strings.TrimSpace(ref.MimeType),
		toNullInt64(ref.FileSize),
		ref.IsPlaceholder,
		strings.TrimSpace(ref.ChecksumHint),
		string(ref.Status),
		createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset storage ref: %w", err)
	}
	copyRef := *ref
	copyRef.RefID = refID
	copyRef.CreatedAt = createdAt
	if uploadRequestID != nil {
		copyRef.UploadRequestID = *uploadRequestID
	}
	domain.HydrateAssetStorageRefDerived(&copyRef)
	return &copyRef, nil
}

func (r *assetStorageRefRepo) GetByRefID(ctx context.Context, refID string) (*domain.AssetStorageRef, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT `+assetStorageRefSelectCols+`
		FROM asset_storage_refs
		WHERE ref_id = ?`, strings.TrimSpace(refID))
	ref, err := scanAssetStorageRef(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get asset storage ref: %w", err)
	}
	return ref, nil
}

func (r *assetStorageRefRepo) UpdateStatus(ctx context.Context, tx repo.Tx, refID string, status domain.AssetStorageRefStatus) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE asset_storage_refs
		SET status = ?
		WHERE ref_id = ?`,
		string(status),
		strings.TrimSpace(refID),
	)
	if err != nil {
		return fmt.Errorf("update asset storage ref status: %w", err)
	}
	return nil
}

func scanAssetStorageRef(scanner interface {
	Scan(...interface{}) error
}) (*domain.AssetStorageRef, error) {
	ref := &domain.AssetStorageRef{}
	var assetID, ownerID, fileSize sql.NullInt64
	var uploadRequestID sql.NullString
	if err := scanner.Scan(
		&ref.RefID,
		&assetID,
		&ref.OwnerType,
		&ownerID,
		&uploadRequestID,
		&ref.StorageAdapter,
		&ref.RefType,
		&ref.RefKey,
		&ref.FileName,
		&ref.MimeType,
		&fileSize,
		&ref.IsPlaceholder,
		&ref.ChecksumHint,
		&ref.Status,
		&ref.CreatedAt,
	); err != nil {
		return nil, err
	}
	ref.AssetID = fromNullInt64(assetID)
	ref.OwnerID = ownerID.Int64
	ref.FileSize = fromNullInt64(fileSize)
	if uploadRequestID.Valid {
		ref.UploadRequestID = uploadRequestID.String
	}
	domain.HydrateAssetStorageRefDerived(ref)
	return ref, nil
}
