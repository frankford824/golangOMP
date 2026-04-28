package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type productRepo struct{ db *DB }

func NewProductRepo(db *DB) repo.ProductRepo { return &productRepo{db: db} }

func (r *productRepo) GetByID(ctx context.Context, id int64) (*domain.Product, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, erp_product_id, sku_code, product_name, category, spec_json,
		       status, source_updated_at, sync_time, created_at, updated_at
		FROM products WHERE id = ?`, id)
	return scanProduct(row)
}

func (r *productRepo) GetByERPProductID(ctx context.Context, erpProductID string) (*domain.Product, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, erp_product_id, sku_code, product_name, category, spec_json,
		       status, source_updated_at, sync_time, created_at, updated_at
		FROM products WHERE erp_product_id = ?`, erpProductID)
	return scanProduct(row)
}

func (r *productRepo) Search(ctx context.Context, filter repo.ProductSearchFilter) ([]*domain.Product, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}

	if filter.Keyword != "" {
		where = append(where, "(product_name LIKE ? OR sku_code LIKE ?)")
		like := "%" + filter.Keyword + "%"
		args = append(args, like, like)
	}
	if filter.Category != "" {
		where = append(where, "category LIKE ?")
		args = append(args, "%"+filter.Category+"%")
	}
	appendProductMappingWhere(&where, &args, filter.MappingRules)

	whereSQL := strings.Join(where, " AND ")
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM products WHERE %s`, whereSQL)
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize

	query := fmt.Sprintf(`
		SELECT id, erp_product_id, sku_code, product_name, category, spec_json,
		       status, source_updated_at, sync_time, created_at, updated_at
		FROM products WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
		whereSQL)
	args = append(args, pageSize, offset)

	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("products search: %w", err)
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		p, err := scanProductRow(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func (r *productRepo) ListIIDs(ctx context.Context, filter repo.ProductIIDListFilter) ([]*domain.ERPIIDOption, int64, error) {
	iidExpr := `TRIM(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.i_id')), ''))`
	categoryNameExpr := `TRIM(COALESCE(JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.category_name')), ''))`
	where := []string{fmt.Sprintf("%s <> ''", iidExpr)}
	args := []interface{}{}
	if q := strings.TrimSpace(filter.Q); q != "" {
		like := "%" + q + "%"
		where = append(where, fmt.Sprintf("(%s LIKE ? OR category LIKE ? OR %s LIKE ? OR product_name LIKE ? OR sku_code LIKE ?)", iidExpr, categoryNameExpr))
		args = append(args, like, like, like, like, like)
	}
	whereSQL := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM (
		SELECT %s AS i_id FROM products WHERE %s GROUP BY i_id
	) t`, iidExpr, whereSQL)
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count product i_id options: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize
	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, offset)
	query := fmt.Sprintf(`
		SELECT
			%s AS i_id,
			MIN(NULLIF(TRIM(category), '')) AS category,
			MIN(NULLIF(%s, '')) AS category_name,
			COUNT(*) AS product_count
		FROM products
		WHERE %s
		GROUP BY i_id
		ORDER BY product_count DESC, i_id ASC
		LIMIT ? OFFSET ?`, iidExpr, categoryNameExpr, whereSQL)
	rows, err := r.db.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list product i_id options: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.ERPIIDOption, 0, pageSize)
	for rows.Next() {
		var item domain.ERPIIDOption
		var category, categoryName sql.NullString
		if err := rows.Scan(&item.IID, &category, &categoryName, &item.ProductCount); err != nil {
			return nil, 0, fmt.Errorf("scan product i_id option: %w", err)
		}
		item.Category = strings.TrimSpace(category.String)
		item.CategoryName = strings.TrimSpace(categoryName.String)
		item.Label = item.IID
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func appendProductMappingWhere(where *[]string, args *[]interface{}, mappings []*domain.CategoryERPMapping) {
	if len(mappings) == 0 {
		return
	}

	clauses := make([]string, 0, len(mappings))
	clauseArgs := make([]interface{}, 0, len(mappings)*3)
	for _, mapping := range mappings {
		clause, values := buildProductMappingClause(mapping)
		if clause == "" {
			continue
		}
		clauses = append(clauses, "("+clause+")")
		clauseArgs = append(clauseArgs, values...)
	}
	if len(clauses) == 0 {
		*where = append(*where, "1 = 0")
		return
	}

	*where = append(*where, "("+strings.Join(clauses, " OR ")+")")
	*args = append(*args, clauseArgs...)
}

func buildProductMappingClause(mapping *domain.CategoryERPMapping) (string, []interface{}) {
	if mapping == nil {
		return "", nil
	}

	switch mapping.ERPMatchType {
	case domain.CategoryERPMatchTypeCategoryCode:
		value := strings.ToUpper(strings.TrimSpace(mapping.ERPMatchValue))
		if value == "" {
			return "", nil
		}
		return "UPPER(category) = ?", []interface{}{value}
	case domain.CategoryERPMatchTypeProductFamily, domain.CategoryERPMatchTypeKeyword:
		value := strings.TrimSpace(mapping.ERPMatchValue)
		if value == "" {
			return "", nil
		}
		like := "%" + value + "%"
		return "(category LIKE ? OR product_name LIKE ? OR spec_json LIKE ?)", []interface{}{like, like, like}
	case domain.CategoryERPMatchTypeSKUPrefix:
		value := strings.ToUpper(strings.TrimSpace(mapping.ERPMatchValue))
		if value == "" {
			return "", nil
		}
		return "UPPER(sku_code) LIKE ?", []interface{}{value + "%"}
	case domain.CategoryERPMatchTypeExternalID:
		value := strings.ToUpper(strings.TrimSpace(mapping.ERPMatchValue))
		if value == "" {
			return "", nil
		}
		return "UPPER(erp_product_id) = ?", []interface{}{value}
	default:
		return "", nil
	}
}

func (r *productRepo) UpsertBatch(ctx context.Context, tx repo.Tx, products []*domain.Product) (int64, error) {
	sqlTx := Unwrap(tx)
	now := time.Now()
	for _, product := range products {
		if _, err := sqlTx.ExecContext(ctx, `
			INSERT INTO products (
				erp_product_id, sku_code, product_name, category, spec_json,
				status, source_updated_at, sync_time
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				sku_code = VALUES(sku_code),
				product_name = VALUES(product_name),
				category = VALUES(category),
				spec_json = VALUES(spec_json),
				status = VALUES(status),
				source_updated_at = VALUES(source_updated_at),
				sync_time = VALUES(sync_time),
				updated_at = NOW()`,
			product.ERPProductID,
			product.SKUCode,
			product.ProductName,
			product.Category,
			product.SpecJSON,
			product.Status,
			toNullTime(product.SourceUpdatedAt),
			now,
		); err != nil {
			return 0, fmt.Errorf("upsert product %s: %w", product.ERPProductID, err)
		}
	}
	return int64(len(products)), nil
}

func scanProduct(row *sql.Row) (*domain.Product, error) {
	var p domain.Product
	var sourceUpdatedAt, syncTime sql.NullTime
	err := row.Scan(
		&p.ID, &p.ERPProductID, &p.SKUCode, &p.ProductName, &p.Category,
		&p.SpecJSON, &p.Status, &sourceUpdatedAt, &syncTime, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan product: %w", err)
	}
	p.SourceUpdatedAt = fromNullTime(sourceUpdatedAt)
	p.SyncTime = fromNullTime(syncTime)
	return &p, nil
}

func scanProductRow(rows *sql.Rows) (*domain.Product, error) {
	var p domain.Product
	var sourceUpdatedAt, syncTime sql.NullTime
	err := rows.Scan(
		&p.ID, &p.ERPProductID, &p.SKUCode, &p.ProductName, &p.Category,
		&p.SpecJSON, &p.Status, &sourceUpdatedAt, &syncTime, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan product row: %w", err)
	}
	p.SourceUpdatedAt = fromNullTime(sourceUpdatedAt)
	p.SyncTime = fromNullTime(syncTime)
	return &p, nil
}
