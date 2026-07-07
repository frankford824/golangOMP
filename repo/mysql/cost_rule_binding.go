package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type costRuleBindingRepo struct{ db *DB }

func NewCostRuleBindingRepo(db *DB) repo.CostRuleBindingRepo {
	return &costRuleBindingRepo{db: db}
}

func (r *costRuleBindingRepo) GetByID(ctx context.Context, id int64) (*domain.CostRuleBinding, error) {
	if id <= 0 {
		return nil, nil
	}
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, i_id_raw, normalized_i_id, rule_group, display_name, source,
		       is_active, created_by, updated_by, created_at, updated_at
		  FROM cost_rule_bindings
		 WHERE id = ?`, id)
	item, err := scanCostRuleBinding(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (r *costRuleBindingRepo) GetActiveByNormalizedIID(ctx context.Context, normalizedIID string) (*domain.CostRuleBinding, error) {
	normalizedIID = domain.NormalizeIID(normalizedIID)
	if normalizedIID == "" {
		return nil, nil
	}
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, i_id_raw, normalized_i_id, rule_group, display_name, source, is_active,
		       created_by, updated_by, created_at, updated_at
		  FROM cost_rule_bindings
		 WHERE normalized_i_id = ? AND is_active = 1
		 LIMIT 1`, normalizedIID)
	item, err := scanCostRuleBinding(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (r *costRuleBindingRepo) List(ctx context.Context, filter repo.CostRuleBindingListFilter) ([]*domain.CostRuleBinding, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		normalizedLike := "%" + domain.NormalizeIID(keyword) + "%"
		where = append(where, "(i_id_raw LIKE ? OR normalized_i_id LIKE ? OR display_name LIKE ? OR rule_group LIKE ?)")
		args = append(args, like, normalizedLike, like, like)
	}
	if ruleGroup := strings.TrimSpace(filter.RuleGroup); ruleGroup != "" {
		where = append(where, "rule_group = ?")
		args = append(args, ruleGroup)
	}
	if filter.IsActive != nil {
		where = append(where, "is_active = ?")
		args = append(args, *filter.IsActive)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cost_rule_bindings WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count cost rule bindings: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	queryArgs := append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, i_id_raw, normalized_i_id, rule_group, display_name, source, is_active,
		       created_by, updated_by, created_at, updated_at
		  FROM cost_rule_bindings
		 WHERE `+whereSQL+`
		 ORDER BY is_active DESC, updated_at DESC, id DESC
		 LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list cost rule bindings: %w", err)
	}
	defer rows.Close()
	items := make([]*domain.CostRuleBinding, 0)
	for rows.Next() {
		item, err := scanCostRuleBinding(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate cost rule bindings: %w", err)
	}
	return items, total, nil
}

func (r *costRuleBindingRepo) Create(ctx context.Context, tx repo.Tx, binding *domain.CostRuleBinding) (int64, error) {
	if binding == nil {
		return 0, fmt.Errorf("cost rule binding is required")
	}
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO cost_rule_bindings (
		  i_id_raw, normalized_i_id, rule_group, display_name, source, is_active, created_by, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(binding.IIDRaw),
		strings.TrimSpace(binding.NormalizedIID),
		strings.TrimSpace(binding.RuleGroup),
		strings.TrimSpace(binding.DisplayName),
		strings.TrimSpace(binding.Source),
		binding.IsActive,
		toNullInt64(binding.CreatedBy),
		toNullInt64(binding.UpdatedBy),
	)
	if err != nil {
		return 0, fmt.Errorf("insert cost rule binding: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("cost rule binding id: %w", err)
	}
	return id, nil
}

func (r *costRuleBindingRepo) Update(ctx context.Context, tx repo.Tx, binding *domain.CostRuleBinding) error {
	if binding == nil || binding.ID <= 0 {
		return fmt.Errorf("cost rule binding id is required")
	}
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE cost_rule_bindings
		   SET i_id_raw = ?, normalized_i_id = ?, rule_group = ?, display_name = ?,
		       source = ?, is_active = ?, updated_by = ?
		 WHERE id = ?`,
		strings.TrimSpace(binding.IIDRaw),
		strings.TrimSpace(binding.NormalizedIID),
		strings.TrimSpace(binding.RuleGroup),
		strings.TrimSpace(binding.DisplayName),
		strings.TrimSpace(binding.Source),
		binding.IsActive,
		toNullInt64(binding.UpdatedBy),
		binding.ID,
	)
	if err != nil {
		return fmt.Errorf("update cost rule binding: %w", err)
	}
	return nil
}

func (r *costRuleBindingRepo) Patch(ctx context.Context, tx repo.Tx, patch domain.CostRuleBindingPatch) error {
	if patch.ID <= 0 {
		return fmt.Errorf("cost rule binding id is required")
	}
	assignments := []string{}
	args := []interface{}{}
	if patch.IIDRaw != nil {
		raw := strings.TrimSpace(*patch.IIDRaw)
		assignments = append(assignments, "i_id_raw = ?", "normalized_i_id = ?")
		args = append(args, raw, domain.NormalizeIID(raw))
	}
	if patch.RuleGroup != nil {
		assignments = append(assignments, "rule_group = ?")
		args = append(args, strings.TrimSpace(*patch.RuleGroup))
	}
	if patch.DisplayName != nil {
		assignments = append(assignments, "display_name = ?")
		args = append(args, strings.TrimSpace(*patch.DisplayName))
	}
	if patch.Source != nil {
		assignments = append(assignments, "source = ?")
		args = append(args, strings.TrimSpace(*patch.Source))
	}
	if patch.IsActive != nil {
		assignments = append(assignments, "is_active = ?")
		args = append(args, *patch.IsActive)
	}
	if patch.UpdatedBy != nil {
		assignments = append(assignments, "updated_by = ?")
		args = append(args, *patch.UpdatedBy)
	}
	if len(assignments) == 0 {
		return nil
	}
	args = append(args, patch.ID)
	sqlTx := Unwrap(tx)
	if _, err := sqlTx.ExecContext(ctx, `UPDATE cost_rule_bindings SET `+strings.Join(assignments, ", ")+` WHERE id = ?`, args...); err != nil {
		return fmt.Errorf("patch cost rule binding: %w", err)
	}
	return nil
}

func (r *costRuleBindingRepo) RuleGroupExists(ctx context.Context, ruleGroup string) (bool, error) {
	ruleGroup = strings.TrimSpace(ruleGroup)
	if ruleGroup == "" {
		return false, nil
	}
	var exists int
	if err := r.db.db.QueryRowContext(ctx, `
		SELECT 1 FROM cost_rules
		 WHERE category_code = ? AND is_active = 1
		 LIMIT 1`, ruleGroup).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check cost rule group: %w", err)
	}
	return true, nil
}

func (r *costRuleBindingRepo) ListUnboundCandidates(ctx context.Context, filter repo.UnboundCostRuleCandidateFilter) ([]*domain.UnboundCostRuleCandidate, int64, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = filter.PageSize
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	keyword := strings.TrimSpace(filter.Keyword)
	normalizedExpr := "UPPER(REPLACE(REPLACE(TRIM(COALESCE(NULLIF(pm.erp_i_id, ''), pm.product_i_id)), ' ', ''), '　', ''))"
	groupByNormalizedExpr := normalizedExpr
	where := []string{
		"COALESCE(NULLIF(pm.erp_i_id, ''), pm.product_i_id, '') <> ''",
		"b.id IS NULL",
		"JSON_VALID(cost_snapshot.calculation_snapshot_json)",
		"JSON_UNQUOTE(JSON_EXTRACT(cost_snapshot.calculation_snapshot_json, '$.legacy_alias_fallback')) = 'true'",
	}
	args := []interface{}{}
	if keyword != "" {
		like := "%" + keyword + "%"
		normalizedLike := "%" + domain.NormalizeIID(keyword) + "%"
		where = append(where, "(pm.erp_i_id LIKE ? OR pm.product_i_id LIKE ? OR "+normalizedExpr+" LIKE ? OR pm.sku_code LIKE ? OR pm.task_no LIKE ?)")
		args = append(args, like, like, normalizedLike, like, like)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
		  SELECT `+normalizedExpr+` normalized_i_id
		    FROM erp_product_sync_records pm
		    `+productManagementCostTraceJoin+`
		    LEFT JOIN cost_rule_bindings b
		      ON b.normalized_i_id = `+normalizedExpr+`
		     AND b.is_active = 1
		   WHERE `+whereSQL+`
		   GROUP BY `+groupByNormalizedExpr+`
		) x`, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count unbound cost rule candidates: %w", err)
	}
	queryArgs := append(append([]interface{}{}, args...), limit)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT
		  COALESCE(MIN(NULLIF(pm.erp_i_id, '')), '') AS erp_i_id,
		  COALESCE(MIN(NULLIF(pm.product_i_id, '')), '') AS product_i_id,
		  `+normalizedExpr+` AS normalized_i_id,
		  COALESCE(MIN(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(cost_snapshot.calculation_snapshot_json, '$.rule_group')), '')), '') AS suggested_rule_group,
		  COUNT(*) AS match_count,
		  MIN(pm.sku_code) AS example_sku_code,
		  MIN(pm.task_no) AS example_task_no,
		  AVG(pm.cost_price) AS average_cost_price
		FROM erp_product_sync_records pm
		`+productManagementCostTraceJoin+`
		LEFT JOIN cost_rule_bindings b
		  ON b.normalized_i_id = `+normalizedExpr+`
		 AND b.is_active = 1
		WHERE `+whereSQL+`
		GROUP BY `+groupByNormalizedExpr+`
		ORDER BY match_count DESC, normalized_i_id ASC
		LIMIT ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list unbound cost rule candidates: %w", err)
	}
	defer rows.Close()
	items := make([]*domain.UnboundCostRuleCandidate, 0)
	for rows.Next() {
		var item domain.UnboundCostRuleCandidate
		if err := rows.Scan(
			&item.ERPProductIID,
			&item.ProductIID,
			&item.NormalizedIID,
			&item.SuggestedRuleGroup,
			&item.MatchCount,
			&item.ExampleSKUCode,
			&item.ExampleTaskNo,
			&item.AverageCostPrice,
		); err != nil {
			return nil, 0, fmt.Errorf("scan unbound cost rule candidate: %w", err)
		}
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate unbound cost rule candidates: %w", err)
	}
	return items, total, nil
}

func scanCostRuleBinding(scanner interface{ Scan(...interface{}) error }) (*domain.CostRuleBinding, error) {
	var item domain.CostRuleBinding
	if err := scanner.Scan(
		&item.ID,
		&item.IIDRaw,
		&item.NormalizedIID,
		&item.RuleGroup,
		&item.DisplayName,
		&item.Source,
		&item.IsActive,
		&item.CreatedBy,
		&item.UpdatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}
