package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type designAssetRepo struct{ db *DB }

func NewDesignAssetRepo(db *DB) repo.DesignAssetRepo { return &designAssetRepo{db: db} }

const designAssetSelectCols = `
	id, task_id, asset_no, source_asset_id, scope_sku_code, asset_type, current_version_id, created_by, created_at, updated_at`

func (r *designAssetRepo) Create(ctx context.Context, tx repo.Tx, asset *domain.DesignAsset) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO design_assets
		  (task_id, asset_no, source_asset_id, scope_sku_code, asset_type, current_version_id, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		asset.TaskID,
		asset.AssetNo,
		toNullInt64(asset.SourceAssetID),
		asset.ScopeSKUCode,
		string(asset.AssetType),
		toNullInt64(asset.CurrentVersionID),
		asset.CreatedBy,
	)
	if err != nil {
		return 0, fmt.Errorf("insert design_asset: %w", err)
	}
	return res.LastInsertId()
}

func (r *designAssetRepo) GetByID(ctx context.Context, id int64) (*domain.DesignAsset, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT `+designAssetSelectCols+`
		FROM design_assets
		WHERE id = ?`, id)
	return scanDesignAsset(row)
}

func (r *designAssetRepo) List(ctx context.Context, filter repo.DesignAssetListFilter) ([]*domain.DesignAsset, error) {
	whereParts := []string{"1=1"}
	args := make([]interface{}, 0, 4)
	if filter.TaskID != nil {
		whereParts = append(whereParts, "task_id = ?")
		args = append(args, *filter.TaskID)
	}
	if filter.SourceAssetID != nil {
		whereParts = append(whereParts, "source_asset_id = ?")
		args = append(args, *filter.SourceAssetID)
	}
	if filter.AssetType != nil {
		whereParts = append(whereParts, "asset_type = ?")
		args = append(args, string(domain.NormalizeTaskAssetType(*filter.AssetType)))
	}
	if scopeSKUCode := strings.TrimSpace(filter.ScopeSKUCode); scopeSKUCode != "" {
		whereParts = append(whereParts, "scope_sku_code = ?")
		args = append(args, scopeSKUCode)
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT `+designAssetSelectCols+`
		FROM design_assets
		WHERE `+strings.Join(whereParts, " AND ")+`
		ORDER BY created_at ASC, id ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list design_assets with filter: %w", err)
	}
	defer rows.Close()

	assets := []*domain.DesignAsset{}
	for rows.Next() {
		asset, err := scanDesignAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (r *designAssetRepo) ListByTaskID(ctx context.Context, taskID int64) ([]*domain.DesignAsset, error) {
	return r.List(ctx, repo.DesignAssetListFilter{TaskID: &taskID})
}

func (r *designAssetRepo) NextAssetNo(ctx context.Context, tx repo.Tx, taskID int64) (string, error) {
	sqlTx := Unwrap(tx)
	var count int64
	if err := sqlTx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM design_assets WHERE task_id = ? FOR UPDATE`,
		taskID,
	).Scan(&count); err != nil {
		return "", fmt.Errorf("design_asset next asset_no: %w", err)
	}
	return fmt.Sprintf("AST-%04d", count+1), nil
}

func (r *designAssetRepo) UpdateCurrentVersionID(ctx context.Context, tx repo.Tx, id int64, currentVersionID *int64) error {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE design_assets
		SET current_version_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		toNullInt64(currentVersionID),
		id,
	)
	if err != nil {
		return fmt.Errorf("update design_asset current version: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update design_asset current version rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanDesignAsset(scanner interface {
	Scan(...interface{}) error
}) (*domain.DesignAsset, error) {
	asset := &domain.DesignAsset{}
	var sourceAssetID sql.NullInt64
	var scopeSKUCode sql.NullString
	var currentVersionID sql.NullInt64
	if err := scanner.Scan(
		&asset.ID,
		&asset.TaskID,
		&asset.AssetNo,
		&sourceAssetID,
		&scopeSKUCode,
		&asset.AssetType,
		&currentVersionID,
		&asset.CreatedBy,
		&asset.CreatedAt,
		&asset.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan design_asset: %w", err)
	}
	if scopeSKUCode.Valid {
		asset.ScopeSKUCode = scopeSKUCode.String
	}
	asset.SourceAssetID = fromNullInt64(sourceAssetID)
	asset.AssetType = domain.NormalizeTaskAssetType(asset.AssetType)
	asset.CurrentVersionID = fromNullInt64(currentVersionID)
	return asset, nil
}
