package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type categoryERPMappingRepo struct{ db *DB }

func NewCategoryERPMappingRepo(db *DB) repo.CategoryERPMappingRepo {
	return &categoryERPMappingRepo{db: db}
}

func (r *categoryERPMappingRepo) GetByID(ctx context.Context, id int64) (*domain.CategoryERPMapping, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, category_id, category_code, search_entry_code, erp_match_type, erp_match_value,
		       secondary_condition_key, secondary_condition_value, tertiary_condition_key, tertiary_condition_value,
		       is_primary, is_active, priority, source, remark, created_at, updated_at
		FROM category_erp_mappings
		WHERE id = ?`, id)
	return scanCategoryERPMapping(row)
}

func (r *categoryERPMappingRepo) List(ctx context.Context, filter repo.CategoryERPMappingListFilter) ([]*domain.CategoryERPMapping, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	appendCategoryERPMappingWhere(&where, &args, filter.Keyword, filter.CategoryID, filter.CategoryCode, filter.SearchEntryCode, filter.ERPMatchType, filter.IsActive, filter.IsPrimary, filter.Source)

	countQuery := `SELECT COUNT(*) FROM category_erp_mappings WHERE ` + strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count category_erp_mappings: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize

	query := `
		SELECT id, category_id, category_code, search_entry_code, erp_match_type, erp_match_value,
		       secondary_condition_key, secondary_condition_value, tertiary_condition_key, tertiary_condition_value,
		       is_primary, is_active, priority, source, remark, created_at, updated_at
		FROM category_erp_mappings
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY is_primary DESC, priority ASC, id ASC
		LIMIT ? OFFSET ?`
	queryArgs := append(append([]interface{}{}, args...), pageSize, offset)

	rows, err := r.db.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list category_erp_mappings: %w", err)
	}
	defer rows.Close()

	items, err := scanCategoryERPMappingRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *categoryERPMappingRepo) Search(ctx context.Context, filter repo.CategoryERPMappingSearchFilter) ([]*domain.CategoryERPMapping, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	appendCategoryERPMappingWhere(&where, &args, filter.Keyword, nil, filter.CategoryCode, filter.SearchEntryCode, filter.ERPMatchType, filter.IsActive, nil, "")

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	query := `
		SELECT id, category_id, category_code, search_entry_code, erp_match_type, erp_match_value,
		       secondary_condition_key, secondary_condition_value, tertiary_condition_key, tertiary_condition_value,
		       is_primary, is_active, priority, source, remark, created_at, updated_at
		FROM category_erp_mappings
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY is_primary DESC, priority ASC, id ASC
		LIMIT ?`
	queryArgs := append(append([]interface{}{}, args...), limit)

	rows, err := r.db.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("search category_erp_mappings: %w", err)
	}
	defer rows.Close()

	return scanCategoryERPMappingRows(rows)
}

func (r *categoryERPMappingRepo) ListActiveByCategory(ctx context.Context, categoryID *int64, categoryCode string) ([]*domain.CategoryERPMapping, error) {
	where := []string{"is_active = 1"}
	args := []interface{}{}
	categoryCode = strings.TrimSpace(categoryCode)
	if categoryID != nil {
		where = append(where, "(category_id = ? OR category_code = ?)")
		args = append(args, *categoryID, categoryCode)
	} else if categoryCode != "" {
		where = append(where, "category_code = ?")
		args = append(args, categoryCode)
	}

	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, category_id, category_code, search_entry_code, erp_match_type, erp_match_value,
		       secondary_condition_key, secondary_condition_value, tertiary_condition_key, tertiary_condition_value,
		       is_primary, is_active, priority, source, remark, created_at, updated_at
		FROM category_erp_mappings
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY is_primary DESC, priority ASC, id ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list active category_erp_mappings: %w", err)
	}
	defer rows.Close()

	return scanCategoryERPMappingRows(rows)
}

func (r *categoryERPMappingRepo) ListActiveBySearchEntry(ctx context.Context, searchEntryCode string) ([]*domain.CategoryERPMapping, error) {
	searchEntryCode = strings.TrimSpace(searchEntryCode)
	if searchEntryCode == "" {
		return []*domain.CategoryERPMapping{}, nil
	}

	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, category_id, category_code, search_entry_code, erp_match_type, erp_match_value,
		       secondary_condition_key, secondary_condition_value, tertiary_condition_key, tertiary_condition_value,
		       is_primary, is_active, priority, source, remark, created_at, updated_at
		FROM category_erp_mappings
		WHERE is_active = 1 AND search_entry_code = ?
		ORDER BY is_primary DESC, priority ASC, id ASC`, searchEntryCode)
	if err != nil {
		return nil, fmt.Errorf("list active category_erp_mappings by search entry: %w", err)
	}
	defer rows.Close()

	return scanCategoryERPMappingRows(rows)
}

func (r *categoryERPMappingRepo) Create(ctx context.Context, tx repo.Tx, mapping *domain.CategoryERPMapping) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO category_erp_mappings (
			category_id, category_code, search_entry_code, erp_match_type, erp_match_value,
			secondary_condition_key, secondary_condition_value, tertiary_condition_key, tertiary_condition_value,
			is_primary, is_active, priority, source, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		toNullInt64(mapping.CategoryID),
		mapping.CategoryCode,
		mapping.SearchEntryCode,
		string(mapping.ERPMatchType),
		mapping.ERPMatchValue,
		mapping.SecondaryConditionKey,
		mapping.SecondaryConditionValue,
		mapping.TertiaryConditionKey,
		mapping.TertiaryConditionValue,
		mapping.IsPrimary,
		mapping.IsActive,
		mapping.Priority,
		mapping.Source,
		mapping.Remark,
	)
	if err != nil {
		return 0, fmt.Errorf("insert category_erp_mapping: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id (category_erp_mapping): %w", err)
	}
	return id, nil
}

func (r *categoryERPMappingRepo) Update(ctx context.Context, tx repo.Tx, mapping *domain.CategoryERPMapping) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE category_erp_mappings
		SET category_id = ?,
		    category_code = ?,
		    search_entry_code = ?,
		    erp_match_type = ?,
		    erp_match_value = ?,
		    secondary_condition_key = ?,
		    secondary_condition_value = ?,
		    tertiary_condition_key = ?,
		    tertiary_condition_value = ?,
		    is_primary = ?,
		    is_active = ?,
		    priority = ?,
		    source = ?,
		    remark = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		toNullInt64(mapping.CategoryID),
		mapping.CategoryCode,
		mapping.SearchEntryCode,
		string(mapping.ERPMatchType),
		mapping.ERPMatchValue,
		mapping.SecondaryConditionKey,
		mapping.SecondaryConditionValue,
		mapping.TertiaryConditionKey,
		mapping.TertiaryConditionValue,
		mapping.IsPrimary,
		mapping.IsActive,
		mapping.Priority,
		mapping.Source,
		mapping.Remark,
		mapping.MappingID,
	)
	if err != nil {
		return fmt.Errorf("update category_erp_mapping: %w", err)
	}
	return nil
}

func appendCategoryERPMappingWhere(where *[]string, args *[]interface{}, keyword string, categoryID *int64, categoryCode, searchEntryCode string, matchType *domain.CategoryERPMatchType, isActive, isPrimary *bool, source string) {
	if trimmed := strings.TrimSpace(keyword); trimmed != "" {
		like := "%" + trimmed + "%"
		*where = append(*where, "(category_code LIKE ? OR search_entry_code LIKE ? OR erp_match_value LIKE ? OR secondary_condition_value LIKE ? OR tertiary_condition_value LIKE ? OR remark LIKE ?)")
		*args = append(*args, like, like, like, like, like, like)
	}
	if categoryID != nil {
		*where = append(*where, "category_id = ?")
		*args = append(*args, *categoryID)
	}
	if trimmed := strings.TrimSpace(categoryCode); trimmed != "" {
		*where = append(*where, "category_code = ?")
		*args = append(*args, trimmed)
	}
	if trimmed := strings.TrimSpace(searchEntryCode); trimmed != "" {
		*where = append(*where, "search_entry_code = ?")
		*args = append(*args, trimmed)
	}
	if matchType != nil {
		*where = append(*where, "erp_match_type = ?")
		*args = append(*args, string(*matchType))
	}
	if isActive != nil {
		*where = append(*where, "is_active = ?")
		*args = append(*args, *isActive)
	}
	if isPrimary != nil {
		*where = append(*where, "is_primary = ?")
		*args = append(*args, *isPrimary)
	}
	if trimmed := strings.TrimSpace(source); trimmed != "" {
		*where = append(*where, "source = ?")
		*args = append(*args, trimmed)
	}
}

func scanCategoryERPMapping(row *sql.Row) (*domain.CategoryERPMapping, error) {
	var item domain.CategoryERPMapping
	var categoryID sql.NullInt64
	if err := row.Scan(
		&item.MappingID,
		&categoryID,
		&item.CategoryCode,
		&item.SearchEntryCode,
		&item.ERPMatchType,
		&item.ERPMatchValue,
		&item.SecondaryConditionKey,
		&item.SecondaryConditionValue,
		&item.TertiaryConditionKey,
		&item.TertiaryConditionValue,
		&item.IsPrimary,
		&item.IsActive,
		&item.Priority,
		&item.Source,
		&item.Remark,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan category_erp_mapping: %w", err)
	}
	item.CategoryID = fromNullInt64(categoryID)
	return &item, nil
}

func scanCategoryERPMappingRows(rows *sql.Rows) ([]*domain.CategoryERPMapping, error) {
	var items []*domain.CategoryERPMapping
	for rows.Next() {
		var item domain.CategoryERPMapping
		var categoryID sql.NullInt64
		if err := rows.Scan(
			&item.MappingID,
			&categoryID,
			&item.CategoryCode,
			&item.SearchEntryCode,
			&item.ERPMatchType,
			&item.ERPMatchValue,
			&item.SecondaryConditionKey,
			&item.SecondaryConditionValue,
			&item.TertiaryConditionKey,
			&item.TertiaryConditionValue,
			&item.IsPrimary,
			&item.IsActive,
			&item.Priority,
			&item.Source,
			&item.Remark,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan category_erp_mapping row: %w", err)
		}
		item.CategoryID = fromNullInt64(categoryID)
		items = append(items, &item)
	}
	return items, rows.Err()
}
