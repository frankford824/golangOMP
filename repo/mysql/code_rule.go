package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type codeRuleRepo struct{ db *DB }

func NewCodeRuleRepo(db *DB) repo.CodeRuleRepo { return &codeRuleRepo{db: db} }

func (r *codeRuleRepo) GetByID(ctx context.Context, id int64) (*domain.CodeRule, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, rule_type, rule_name, prefix, date_format, site_code, biz_code,
		       seq_length, reset_cycle, is_enabled, config_json, created_at, updated_at
		FROM code_rules WHERE id = ?`, id)
	return scanCodeRule(row)
}

func (r *codeRuleRepo) GetEnabledByType(ctx context.Context, ruleType domain.CodeRuleType) (*domain.CodeRule, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, rule_type, rule_name, prefix, date_format, site_code, biz_code,
		       seq_length, reset_cycle, is_enabled, config_json, created_at, updated_at
		FROM code_rules WHERE rule_type = ? AND is_enabled = 1 LIMIT 1`,
		string(ruleType))
	return scanCodeRule(row)
}

func (r *codeRuleRepo) ListAll(ctx context.Context) ([]*domain.CodeRule, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, rule_type, rule_name, prefix, date_format, site_code, biz_code,
		       seq_length, reset_cycle, is_enabled, config_json, created_at, updated_at
		FROM code_rules ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list code_rules: %w", err)
	}
	defer rows.Close()

	var rules []*domain.CodeRule
	for rows.Next() {
		r2, err := scanCodeRuleRow(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r2)
	}
	return rules, rows.Err()
}

// NextSeq atomically returns the next sequence number for the given rule.
// Uses a code_rule_sequences counter table with SELECT FOR UPDATE to prevent races.
// MUST be called inside an active transaction.
func (r *codeRuleRepo) NextSeq(ctx context.Context, tx repo.Tx, ruleID int64) (int64, error) {
	sqlTx := Unwrap(tx)

	var current int64
	err := sqlTx.QueryRowContext(ctx,
		`SELECT last_seq FROM code_rule_sequences WHERE rule_id = ? FOR UPDATE`,
		ruleID,
	).Scan(&current)

	if err == sql.ErrNoRows {
		if _, err = sqlTx.ExecContext(ctx,
			`INSERT INTO code_rule_sequences (rule_id, last_seq) VALUES (?, 1)`,
			ruleID,
		); err != nil {
			return 0, fmt.Errorf("code_rule NextSeq insert: %w", err)
		}
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("code_rule NextSeq select: %w", err)
	}

	next := current + 1
	if _, err = sqlTx.ExecContext(ctx,
		`UPDATE code_rule_sequences SET last_seq = ? WHERE rule_id = ?`,
		next, ruleID,
	); err != nil {
		return 0, fmt.Errorf("code_rule NextSeq update: %w", err)
	}
	return next, nil
}

func scanCodeRule(row *sql.Row) (*domain.CodeRule, error) {
	var cr domain.CodeRule
	err := row.Scan(
		&cr.ID, &cr.RuleType, &cr.RuleName, &cr.Prefix, &cr.DateFormat,
		&cr.SiteCode, &cr.BizCode, &cr.SeqLength, &cr.ResetCycle,
		&cr.IsEnabled, &cr.ConfigJSON, &cr.CreatedAt, &cr.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan code_rule: %w", err)
	}
	return &cr, nil
}

func scanCodeRuleRow(rows *sql.Rows) (*domain.CodeRule, error) {
	var cr domain.CodeRule
	err := rows.Scan(
		&cr.ID, &cr.RuleType, &cr.RuleName, &cr.Prefix, &cr.DateFormat,
		&cr.SiteCode, &cr.BizCode, &cr.SeqLength, &cr.ResetCycle,
		&cr.IsEnabled, &cr.ConfigJSON, &cr.CreatedAt, &cr.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan code_rule row: %w", err)
	}
	return &cr, nil
}
