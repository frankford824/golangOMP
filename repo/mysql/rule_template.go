package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type ruleTemplateRepo struct{ db *DB }

func NewRuleTemplateRepo(db *DB) repo.RuleTemplateRepo {
	return &ruleTemplateRepo{db: db}
}

func (r *ruleTemplateRepo) GetByType(ctx context.Context, templateType domain.RuleTemplateType) (*domain.RuleTemplate, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, template_type, config_json, created_at, updated_at
		FROM rule_templates WHERE template_type = ?`, templateType)
	var rt domain.RuleTemplate
	err := row.Scan(&rt.ID, &rt.TemplateType, &rt.ConfigJSON, &rt.CreatedAt, &rt.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get rule template: %w", err)
	}
	return &rt, nil
}

func (r *ruleTemplateRepo) ListAll(ctx context.Context) ([]*domain.RuleTemplate, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, template_type, config_json, created_at, updated_at
		FROM rule_templates ORDER BY template_type`)
	if err != nil {
		return nil, fmt.Errorf("list rule templates: %w", err)
	}
	defer rows.Close()
	var list []*domain.RuleTemplate
	for rows.Next() {
		var rt domain.RuleTemplate
		if err := rows.Scan(&rt.ID, &rt.TemplateType, &rt.ConfigJSON, &rt.CreatedAt, &rt.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan rule template: %w", err)
		}
		list = append(list, &rt)
	}
	return list, rows.Err()
}

func (r *ruleTemplateRepo) Upsert(ctx context.Context, templateType domain.RuleTemplateType, configJSON string) (*domain.RuleTemplate, error) {
	res, err := r.db.db.ExecContext(ctx, `
		INSERT INTO rule_templates (template_type, config_json)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE config_json = VALUES(config_json), updated_at = CURRENT_TIMESTAMP`,
		templateType, configJSON)
	if err != nil {
		return nil, fmt.Errorf("upsert rule template: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		id, _ := res.LastInsertId()
		return r.getByID(ctx, id)
	}
	return r.GetByType(ctx, templateType)
}

func (r *ruleTemplateRepo) getByID(ctx context.Context, id int64) (*domain.RuleTemplate, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, template_type, config_json, created_at, updated_at
		FROM rule_templates WHERE id = ?`, id)
	var rt domain.RuleTemplate
	if err := row.Scan(&rt.ID, &rt.TemplateType, &rt.ConfigJSON, &rt.CreatedAt, &rt.UpdatedAt); err != nil {
		return nil, err
	}
	return &rt, nil
}
