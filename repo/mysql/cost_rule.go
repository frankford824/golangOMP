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

type costRuleRepo struct{ db *DB }

func NewCostRuleRepo(db *DB) repo.CostRuleRepo { return &costRuleRepo{db: db} }

func (r *costRuleRepo) GetByID(ctx context.Context, id int64) (*domain.CostRule, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, rule_name, rule_version, category_id, category_code, product_family, rule_type,
		       base_price, tax_multiplier, min_area, area_threshold, surcharge_amount,
		       special_process_keyword, special_process_price, formula_expression,
		       priority, is_active, effective_from, effective_to, supersedes_rule_id,
		       (SELECT cr2.id FROM cost_rules cr2 WHERE cr2.supersedes_rule_id = cost_rules.id ORDER BY cr2.rule_version DESC, cr2.id DESC LIMIT 1) AS superseded_by_rule_id,
		       governance_note, source, remark, created_at, updated_at
		FROM cost_rules
		WHERE id = ?`, id)
	return scanCostRule(row)
}

func (r *costRuleRepo) List(ctx context.Context, filter repo.CostRuleListFilter) ([]*domain.CostRule, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	appendCostRuleFilterWhere(&where, &args, filter)

	countQuery := `SELECT COUNT(*) FROM cost_rules WHERE ` + strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count cost rules: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize

	query := `
		SELECT id, rule_name, rule_version, category_id, category_code, product_family, rule_type,
		       base_price, tax_multiplier, min_area, area_threshold, surcharge_amount,
		       special_process_keyword, special_process_price, formula_expression,
		       priority, is_active, effective_from, effective_to, supersedes_rule_id,
		       (SELECT cr2.id FROM cost_rules cr2 WHERE cr2.supersedes_rule_id = cost_rules.id ORDER BY cr2.rule_version DESC, cr2.id DESC LIMIT 1) AS superseded_by_rule_id,
		       governance_note, source, remark, created_at, updated_at
		FROM cost_rules
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY priority ASC, rule_version DESC, id ASC
		LIMIT ? OFFSET ?`
	queryArgs := append(append([]interface{}{}, args...), pageSize, offset)

	rows, err := r.db.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list cost rules: %w", err)
	}
	defer rows.Close()

	items, err := scanCostRuleRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *costRuleRepo) ListActiveByCategory(ctx context.Context, categoryID *int64, categoryCode string, asOf time.Time) ([]*domain.CostRule, error) {
	where := []string{"is_active = 1"}
	args := []interface{}{}
	if categoryID != nil {
		where = append(where, "(category_id = ? OR category_code = ?)")
		args = append(args, *categoryID, strings.TrimSpace(categoryCode))
	} else {
		where = append(where, "category_code = ?")
		args = append(args, strings.TrimSpace(categoryCode))
	}
	where = append(where, "(effective_from IS NULL OR effective_from <= ?)")
	args = append(args, asOf)
	where = append(where, "(effective_to IS NULL OR effective_to >= ?)")
	args = append(args, asOf)

	query := `
		SELECT id, rule_name, rule_version, category_id, category_code, product_family, rule_type,
		       base_price, tax_multiplier, min_area, area_threshold, surcharge_amount,
		       special_process_keyword, special_process_price, formula_expression,
		       priority, is_active, effective_from, effective_to, supersedes_rule_id,
		       (SELECT cr2.id FROM cost_rules cr2 WHERE cr2.supersedes_rule_id = cost_rules.id ORDER BY cr2.rule_version DESC, cr2.id DESC LIMIT 1) AS superseded_by_rule_id,
		       governance_note, source, remark, created_at, updated_at
		FROM cost_rules
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY priority ASC, rule_version DESC, id ASC`

	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list active cost rules: %w", err)
	}
	defer rows.Close()

	return scanCostRuleRows(rows)
}

func (r *costRuleRepo) Create(ctx context.Context, tx repo.Tx, rule *domain.CostRule) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO cost_rules (
			rule_name, rule_version, category_id, category_code, product_family, rule_type,
			base_price, tax_multiplier, min_area, area_threshold, surcharge_amount,
			special_process_keyword, special_process_price, formula_expression,
			priority, is_active, effective_from, effective_to, supersedes_rule_id, governance_note, source, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.RuleName,
		rule.RuleVersion,
		toNullInt64(rule.CategoryID),
		rule.CategoryCode,
		rule.ProductFamily,
		string(rule.RuleType),
		toNullFloat64(rule.BasePrice),
		toNullFloat64(rule.TaxMultiplier),
		toNullFloat64(rule.MinArea),
		toNullFloat64(rule.AreaThreshold),
		toNullFloat64(rule.SurchargeAmount),
		rule.SpecialProcessKeyword,
		toNullFloat64(rule.SpecialProcessPrice),
		rule.FormulaExpression,
		rule.Priority,
		rule.IsActive,
		toNullTime(rule.EffectiveFrom),
		toNullTime(rule.EffectiveTo),
		toNullInt64(rule.SupersedesRuleID),
		rule.GovernanceNote,
		rule.Source,
		rule.Remark,
	)
	if err != nil {
		return 0, fmt.Errorf("insert cost rule: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id (cost rule): %w", err)
	}
	return id, nil
}

func (r *costRuleRepo) Update(ctx context.Context, tx repo.Tx, rule *domain.CostRule) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE cost_rules
		SET rule_name = ?,
		    rule_version = ?,
		    category_id = ?,
		    category_code = ?,
		    product_family = ?,
		    rule_type = ?,
		    base_price = ?,
		    tax_multiplier = ?,
		    min_area = ?,
		    area_threshold = ?,
		    surcharge_amount = ?,
		    special_process_keyword = ?,
		    special_process_price = ?,
		    formula_expression = ?,
		    priority = ?,
		    is_active = ?,
		    effective_from = ?,
		    effective_to = ?,
		    supersedes_rule_id = ?,
		    governance_note = ?,
		    source = ?,
		    remark = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		rule.RuleName,
		rule.RuleVersion,
		toNullInt64(rule.CategoryID),
		rule.CategoryCode,
		rule.ProductFamily,
		string(rule.RuleType),
		toNullFloat64(rule.BasePrice),
		toNullFloat64(rule.TaxMultiplier),
		toNullFloat64(rule.MinArea),
		toNullFloat64(rule.AreaThreshold),
		toNullFloat64(rule.SurchargeAmount),
		rule.SpecialProcessKeyword,
		toNullFloat64(rule.SpecialProcessPrice),
		rule.FormulaExpression,
		rule.Priority,
		rule.IsActive,
		toNullTime(rule.EffectiveFrom),
		toNullTime(rule.EffectiveTo),
		toNullInt64(rule.SupersedesRuleID),
		rule.GovernanceNote,
		rule.Source,
		rule.Remark,
		rule.RuleID,
	)
	if err != nil {
		return fmt.Errorf("update cost rule: %w", err)
	}
	return nil
}

func appendCostRuleFilterWhere(where *[]string, args *[]interface{}, filter repo.CostRuleListFilter) {
	if filter.CategoryID != nil {
		*where = append(*where, "category_id = ?")
		*args = append(*args, *filter.CategoryID)
	}
	if trimmed := strings.TrimSpace(filter.CategoryCode); trimmed != "" {
		*where = append(*where, "category_code = ?")
		*args = append(*args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.ProductFamily); trimmed != "" {
		*where = append(*where, "product_family = ?")
		*args = append(*args, trimmed)
	}
	if filter.RuleType != nil {
		*where = append(*where, "rule_type = ?")
		*args = append(*args, string(*filter.RuleType))
	}
	if filter.IsActive != nil {
		*where = append(*where, "is_active = ?")
		*args = append(*args, *filter.IsActive)
	}
}

func scanCostRule(row *sql.Row) (*domain.CostRule, error) {
	var item domain.CostRule
	var categoryID, supersedesRuleID, supersededByRuleID sql.NullInt64
	var basePrice, taxMultiplier, minArea, areaThreshold, surchargeAmount, specialProcessPrice sql.NullFloat64
	var effectiveFrom, effectiveTo sql.NullTime
	if err := row.Scan(
		&item.RuleID,
		&item.RuleName,
		&item.RuleVersion,
		&categoryID,
		&item.CategoryCode,
		&item.ProductFamily,
		&item.RuleType,
		&basePrice,
		&taxMultiplier,
		&minArea,
		&areaThreshold,
		&surchargeAmount,
		&item.SpecialProcessKeyword,
		&specialProcessPrice,
		&item.FormulaExpression,
		&item.Priority,
		&item.IsActive,
		&effectiveFrom,
		&effectiveTo,
		&supersedesRuleID,
		&supersededByRuleID,
		&item.GovernanceNote,
		&item.Source,
		&item.Remark,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan cost rule: %w", err)
	}
	item.CategoryID = fromNullInt64(categoryID)
	item.BasePrice = fromNullFloat64(basePrice)
	item.TaxMultiplier = fromNullFloat64(taxMultiplier)
	item.MinArea = fromNullFloat64(minArea)
	item.AreaThreshold = fromNullFloat64(areaThreshold)
	item.SurchargeAmount = fromNullFloat64(surchargeAmount)
	item.SpecialProcessPrice = fromNullFloat64(specialProcessPrice)
	item.EffectiveFrom = fromNullTime(effectiveFrom)
	item.EffectiveTo = fromNullTime(effectiveTo)
	item.SupersedesRuleID = fromNullInt64(supersedesRuleID)
	item.SupersededByRuleID = fromNullInt64(supersededByRuleID)
	return &item, nil
}

func scanCostRuleRows(rows *sql.Rows) ([]*domain.CostRule, error) {
	var items []*domain.CostRule
	for rows.Next() {
		var item domain.CostRule
		var categoryID, supersedesRuleID, supersededByRuleID sql.NullInt64
		var basePrice, taxMultiplier, minArea, areaThreshold, surchargeAmount, specialProcessPrice sql.NullFloat64
		var effectiveFrom, effectiveTo sql.NullTime
		if err := rows.Scan(
			&item.RuleID,
			&item.RuleName,
			&item.RuleVersion,
			&categoryID,
			&item.CategoryCode,
			&item.ProductFamily,
			&item.RuleType,
			&basePrice,
			&taxMultiplier,
			&minArea,
			&areaThreshold,
			&surchargeAmount,
			&item.SpecialProcessKeyword,
			&specialProcessPrice,
			&item.FormulaExpression,
			&item.Priority,
			&item.IsActive,
			&effectiveFrom,
			&effectiveTo,
			&supersedesRuleID,
			&supersededByRuleID,
			&item.GovernanceNote,
			&item.Source,
			&item.Remark,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cost rule row: %w", err)
		}
		item.CategoryID = fromNullInt64(categoryID)
		item.BasePrice = fromNullFloat64(basePrice)
		item.TaxMultiplier = fromNullFloat64(taxMultiplier)
		item.MinArea = fromNullFloat64(minArea)
		item.AreaThreshold = fromNullFloat64(areaThreshold)
		item.SurchargeAmount = fromNullFloat64(surchargeAmount)
		item.SpecialProcessPrice = fromNullFloat64(specialProcessPrice)
		item.EffectiveFrom = fromNullTime(effectiveFrom)
		item.EffectiveTo = fromNullTime(effectiveTo)
		item.SupersedesRuleID = fromNullInt64(supersedesRuleID)
		item.SupersededByRuleID = fromNullInt64(supersededByRuleID)
		items = append(items, &item)
	}
	return items, rows.Err()
}
