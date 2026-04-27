package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type customizationPricingRuleRepo struct{ db *DB }

func NewCustomizationPricingRuleRepo(db *DB) repo.CustomizationPricingRuleRepo {
	return &customizationPricingRuleRepo{db: db}
}

func (r *customizationPricingRuleRepo) GetActiveByLevelAndEmploymentType(ctx context.Context, levelCode string, employmentType domain.EmploymentType) (*domain.CustomizationPricingRule, error) {
	code := strings.TrimSpace(levelCode)
	if code == "" || !employmentType.Valid() {
		return nil, nil
	}
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, customization_level_code, employment_type, unit_price, weight_factor, is_enabled, created_at, updated_at
		FROM customization_pricing_rules
		WHERE customization_level_code = ? AND employment_type = ? AND is_enabled = 1
		ORDER BY id DESC
		LIMIT 1`,
		code,
		string(employmentType),
	)
	var item domain.CustomizationPricingRule
	var enabled int
	if err := row.Scan(
		&item.ID,
		&item.CustomizationLevelCode,
		&item.EmploymentType,
		&item.UnitPrice,
		&item.WeightFactor,
		&enabled,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get customization pricing rule: %w", err)
	}
	item.IsEnabled = enabled == 1
	return &item, nil
}
