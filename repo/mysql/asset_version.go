package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"workflow/domain"
	"workflow/repo"
)

// AssetVersionRepoImpl implements repo.AssetVersionRepo.
// Asset versions are append-only: content columns (whole_hash, chunk hashes, file_size)
// are written once on Create and never updated (spec §5.2 invariant 1).
type AssetVersionRepoImpl struct{ db *sql.DB }

func NewAssetVersionRepo(db *DB) repo.AssetVersionRepo {
	return &AssetVersionRepoImpl{db: db.db}
}

const assetVersionSelectCols = `
	id, sku_id, version_num, whole_hash,
	head_chunk_hash, tail_chunk_hash, file_size_bytes,
	is_stable, preview_url, hash_state, audit_status, exists_state, created_at`

func scanAssetVersion(s interface {
	Scan(...interface{}) error
}) (*domain.AssetVersion, error) {
	av := &domain.AssetVersion{}
	var headChunk, tailChunk, previewURL sql.NullString
	if err := s.Scan(
		&av.ID,
		&av.SKUID,
		&av.VersionNum,
		&av.WholeHash,
		&headChunk,
		&tailChunk,
		&av.FileSizeBytes,
		&av.IsStable,
		&previewURL,
		&av.HashState,
		&av.AuditStatus,
		&av.ExistsState,
		&av.CreatedAt,
	); err != nil {
		return nil, err
	}
	av.HeadChunkHash = fromNullString(headChunk)
	av.TailChunkHash = fromNullString(tailChunk)
	av.PreviewURL = fromNullString(previewURL)
	return av, nil
}

func (r *AssetVersionRepoImpl) GetByID(ctx context.Context, id int64) (*domain.AssetVersion, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+assetVersionSelectCols+` FROM asset_versions WHERE id = ?`, id)
	av, err := scanAssetVersion(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return av, err
}

// GetCurrentForSKU returns the asset version currently pointed to by skus.current_ver_id.
func (r *AssetVersionRepoImpl) GetCurrentForSKU(ctx context.Context, skuID int64) (*domain.AssetVersion, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT`+assetVersionSelectCols+`
		FROM asset_versions av
		JOIN skus s ON s.current_ver_id = av.id
		WHERE s.id = ?`,
		skuID,
	)
	av, err := scanAssetVersion(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return av, err
}

// Create inserts a new asset version inside an active transaction.
// EventRepo.Append MUST be called in the same TX immediately after (spec §8.2).
func (r *AssetVersionRepoImpl) Create(ctx context.Context, tx repo.Tx, ver *domain.AssetVersion) (int64, error) {
	sqlTx := Unwrap(tx)
	now := time.Now()
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO asset_versions
			(sku_id, version_num, whole_hash, head_chunk_hash, tail_chunk_hash,
			 file_size_bytes, is_stable, preview_url, hash_state, audit_status, exists_state, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ver.SKUID,
		ver.VersionNum,
		ver.WholeHash,
		toNullString(ver.HeadChunkHash),
		toNullString(ver.TailChunkHash),
		ver.FileSizeBytes,
		ver.IsStable,
		toNullString(ver.PreviewURL),
		ver.HashState,
		domain.AuditStatusUnreviewed,
		domain.ExistsStateExists,
		now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert asset_version: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("asset_version last insert id: %w", err)
	}
	return id, nil
}

// UpdateHashState updates the hash_state field (no TX needed; NAS Agent updates are non-critical).
func (r *AssetVersionRepoImpl) UpdateHashState(ctx context.Context, id int64, state domain.HashState) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE asset_versions SET hash_state = ? WHERE id = ?`,
		state, id,
	)
	return wrapErr(err, "update hash_state")
}

// UpdateExistsState updates exists_state; if Missing, the caller must also block the SKU workflow.
func (r *AssetVersionRepoImpl) UpdateExistsState(ctx context.Context, id int64, state domain.ExistsState) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE asset_versions SET exists_state = ? WHERE id = ?`,
		state, id,
	)
	return wrapErr(err, "update exists_state")
}

// MarkStable sets is_stable=1, unlocking the version for audit submission (spec §5.2 invariant 2).
func (r *AssetVersionRepoImpl) MarkStable(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE asset_versions SET is_stable = 1, hash_state = ? WHERE id = ?`,
		domain.HashStateReady, id,
	)
	return wrapErr(err, "mark stable")
}

// wrapErr wraps a non-nil error with context; returns nil for nil error.
func wrapErr(err error, msg string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", msg, err)
	}
	return nil
}
