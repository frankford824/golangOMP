package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type designSourceRepo struct{ db *DB }

func NewDesignSourceRepo(db *DB) repo.DesignSourceRepo { return &designSourceRepo{db: db} }

func (r *designSourceRepo) Search(ctx context.Context, filter repo.DesignSourceSearchFilter) ([]domain.DesignSourceEntry, int, string, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Size <= 0 || filter.Size > 100 {
		filter.Size = 20
	}
	if exists, err := r.tableExists(ctx, "design_sources"); err != nil {
		return nil, 0, "", err
	} else if exists {
		items, total, err := r.searchDesignSources(ctx, filter)
		return items, total, "design_sources", err
	}
	items, total, err := r.searchTaskAssetsFallback(ctx, filter)
	return items, total, "task_assets", err
}

func (r *designSourceRepo) tableExists(ctx context.Context, table string) (bool, error) {
	var name string
	err := r.db.db.QueryRowContext(ctx, `SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check table exists %s: %w", table, err)
	}
	return true, nil
}

func (r *designSourceRepo) searchDesignSources(ctx context.Context, filter repo.DesignSourceSearchFilter) ([]domain.DesignSourceEntry, int, error) {
	where, args := designSourceKeywordWhere(filter.Keyword, "file_name", "origin_task_id")
	if where != "" {
		where = " WHERE" + strings.TrimPrefix(where, " AND")
	}
	var total int
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM design_sources`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count design_sources: %w", err)
	}
	args = append(args, (filter.Page-1)*filter.Size, filter.Size)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, file_name, owner_team_code, preview_url_key, version_no, origin_task_id, created_at
		  FROM design_sources`+where+`
		 ORDER BY created_at DESC, id DESC
		 LIMIT ?, ?`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search design_sources: %w", err)
	}
	defer rows.Close()
	return scanDesignSourceEntries(rows), total, rows.Err()
}

func (r *designSourceRepo) searchTaskAssetsFallback(ctx context.Context, filter repo.DesignSourceSearchFilter) ([]domain.DesignSourceEntry, int, error) {
	where, args := designSourceKeywordWhere(filter.Keyword, "ta.file_name", "ta.task_id")
	prefix := `
	  FROM task_assets ta
	  LEFT JOIN task_modules tm ON tm.id = ta.source_task_module_id
	 WHERE ta.source_module_key = 'design'
	   AND ta.deleted_at IS NULL
	   AND ta.cleaned_at IS NULL`
	var total int
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*)`+prefix+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count design task_assets: %w", err)
	}
	args = append(args, (filter.Page-1)*filter.Size, filter.Size)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT ta.id, ta.file_name, COALESCE(tm.claimed_team_code, tm.pool_team_code, '') AS owner_team_code,
		       ta.storage_key, ta.asset_version_no, ta.task_id, ta.created_at`+prefix+where+`
		 ORDER BY ta.created_at DESC, ta.id DESC
		 LIMIT ?, ?`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search design task_assets: %w", err)
	}
	defer rows.Close()
	return scanDesignSourceEntries(rows), total, rows.Err()
}

func designSourceKeywordWhere(keyword, fileColumn, taskColumn string) (string, []interface{}) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return "", nil
	}
	like := "%" + keyword + "%"
	return fmt.Sprintf(" AND (%s LIKE ? OR CAST(%s AS CHAR) LIKE ?)", fileColumn, taskColumn), []interface{}{like, like}
}

func scanDesignSourceEntries(rows *sql.Rows) []domain.DesignSourceEntry {
	var out []domain.DesignSourceEntry
	for rows.Next() {
		var item domain.DesignSourceEntry
		var preview sql.NullString
		var version sql.NullInt64
		var origin sql.NullInt64
		if err := rows.Scan(&item.ID, &item.FileName, &item.OwnerTeamCode, &preview, &version, &origin, &item.CreatedAt); err != nil {
			continue
		}
		if preview.Valid && strings.TrimSpace(preview.String) != "" {
			value := preview.String
			item.PreviewURL = &value
		}
		if version.Valid {
			value := int(version.Int64)
			item.VersionNo = &value
		}
		if origin.Valid {
			value := origin.Int64
			item.OriginTaskID = &value
		}
		out = append(out, item)
	}
	return out
}
