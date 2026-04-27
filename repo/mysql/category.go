package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type categoryRepo struct{ db *DB }

func NewCategoryRepo(db *DB) repo.CategoryRepo { return &categoryRepo{db: db} }

func (r *categoryRepo) GetByID(ctx context.Context, id int64) (*domain.Category, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, category_code, category_name, display_name, parent_id, level_no,
		       search_entry_code, is_search_entry, category_type, is_active, sort_order, source, remark, created_at, updated_at
		FROM categories
		WHERE id = ?`, id)
	return scanCategory(row)
}

func (r *categoryRepo) GetByCode(ctx context.Context, code string) (*domain.Category, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, category_code, category_name, display_name, parent_id, level_no,
		       search_entry_code, is_search_entry, category_type, is_active, sort_order, source, remark, created_at, updated_at
		FROM categories
		WHERE category_code = ?`, strings.TrimSpace(code))
	return scanCategory(row)
}

func (r *categoryRepo) List(ctx context.Context, filter repo.CategoryListFilter) ([]*domain.Category, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	appendCategoryFilterWhere(&where, &args, filter.Keyword, filter.CategoryType, filter.ParentID, filter.Level, filter.IsActive, filter.Source)

	countQuery := `SELECT COUNT(*) FROM categories WHERE ` + strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count categories: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize

	query := `
		SELECT id, category_code, category_name, display_name, parent_id, level_no,
		       search_entry_code, is_search_entry, category_type, is_active, sort_order, source, remark, created_at, updated_at
		FROM categories
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY sort_order ASC, id ASC
		LIMIT ? OFFSET ?`
	queryArgs := append(append([]interface{}{}, args...), pageSize, offset)

	rows, err := r.db.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	items, err := scanCategoryRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *categoryRepo) Search(ctx context.Context, filter repo.CategorySearchFilter) ([]*domain.Category, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	appendCategoryFilterWhere(&where, &args, filter.Keyword, filter.CategoryType, nil, nil, filter.IsActive, "")

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	query := `
		SELECT id, category_code, category_name, display_name, parent_id, level_no,
		       search_entry_code, is_search_entry, category_type, is_active, sort_order, source, remark, created_at, updated_at
		FROM categories
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY sort_order ASC, id ASC
		LIMIT ?`
	queryArgs := append(append([]interface{}{}, args...), limit)

	rows, err := r.db.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("search categories: %w", err)
	}
	defer rows.Close()

	return scanCategoryRows(rows)
}

func (r *categoryRepo) Create(ctx context.Context, tx repo.Tx, category *domain.Category) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO categories (
			category_code, category_name, display_name, parent_id, level_no,
			search_entry_code, is_search_entry, category_type, is_active, sort_order, source, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		category.CategoryCode,
		category.CategoryName,
		category.DisplayName,
		toNullInt64(category.ParentID),
		category.Level,
		category.SearchEntryCode,
		category.IsSearchEntry,
		string(category.CategoryType),
		category.IsActive,
		category.SortOrder,
		category.Source,
		category.Remark,
	)
	if err != nil {
		return 0, fmt.Errorf("insert category: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id (category): %w", err)
	}
	return id, nil
}

func (r *categoryRepo) Update(ctx context.Context, tx repo.Tx, category *domain.Category) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE categories
		SET category_code = ?,
		    category_name = ?,
		    display_name = ?,
		    parent_id = ?,
		    level_no = ?,
		    search_entry_code = ?,
		    is_search_entry = ?,
		    category_type = ?,
		    is_active = ?,
		    sort_order = ?,
		    source = ?,
		    remark = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		category.CategoryCode,
		category.CategoryName,
		category.DisplayName,
		toNullInt64(category.ParentID),
		category.Level,
		category.SearchEntryCode,
		category.IsSearchEntry,
		string(category.CategoryType),
		category.IsActive,
		category.SortOrder,
		category.Source,
		category.Remark,
		category.CategoryID,
	)
	if err != nil {
		return fmt.Errorf("update category: %w", err)
	}
	return nil
}

func appendCategoryFilterWhere(where *[]string, args *[]interface{}, keyword string, categoryType *domain.CategoryType, parentID *int64, level *int, isActive *bool, source string) {
	if trimmed := strings.TrimSpace(keyword); trimmed != "" {
		like := "%" + trimmed + "%"
		*where = append(*where, "(category_code LIKE ? OR category_name LIKE ? OR display_name LIKE ?)")
		*args = append(*args, like, like, like)
	}
	if categoryType != nil {
		*where = append(*where, "category_type = ?")
		*args = append(*args, string(*categoryType))
	}
	if parentID != nil {
		*where = append(*where, "parent_id = ?")
		*args = append(*args, *parentID)
	}
	if level != nil {
		*where = append(*where, "level_no = ?")
		*args = append(*args, *level)
	}
	if isActive != nil {
		*where = append(*where, "is_active = ?")
		*args = append(*args, *isActive)
	}
	if trimmed := strings.TrimSpace(source); trimmed != "" {
		*where = append(*where, "source = ?")
		*args = append(*args, trimmed)
	}
}

func scanCategory(row *sql.Row) (*domain.Category, error) {
	var item domain.Category
	var parentID sql.NullInt64
	if err := row.Scan(
		&item.CategoryID,
		&item.CategoryCode,
		&item.CategoryName,
		&item.DisplayName,
		&parentID,
		&item.Level,
		&item.SearchEntryCode,
		&item.IsSearchEntry,
		&item.CategoryType,
		&item.IsActive,
		&item.SortOrder,
		&item.Source,
		&item.Remark,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan category: %w", err)
	}
	item.ParentID = fromNullInt64(parentID)
	return &item, nil
}

func scanCategoryRows(rows *sql.Rows) ([]*domain.Category, error) {
	var items []*domain.Category
	for rows.Next() {
		var item domain.Category
		var parentID sql.NullInt64
		if err := rows.Scan(
			&item.CategoryID,
			&item.CategoryCode,
			&item.CategoryName,
			&item.DisplayName,
			&parentID,
			&item.Level,
			&item.SearchEntryCode,
			&item.IsSearchEntry,
			&item.CategoryType,
			&item.IsActive,
			&item.SortOrder,
			&item.Source,
			&item.Remark,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan category row: %w", err)
		}
		item.ParentID = fromNullInt64(parentID)
		items = append(items, &item)
	}
	return items, rows.Err()
}
