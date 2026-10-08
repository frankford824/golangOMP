package mysqlrepo

import (
	"context"
	"workflow/domain"
	"workflow/repo"
)

func (r *costSyncRepo) LockManualRevision(ctx context.Context, tx repo.Tx, sku string, revision int64) error {
	q := Unwrap(tx)
	var id, current int64
	// Match the canonical row -> sync row lock order used by the triggers.
	if err := q.QueryRowContext(ctx, `SELECT id FROM task_sku_items WHERE sku_code=? FOR UPDATE`, sku).Scan(&id); err != nil {
		return err
	}
	if err := q.QueryRowContext(ctx, `SELECT revision FROM sku_cost_sync_states WHERE sku_code=? FOR UPDATE`, sku).Scan(&current); err != nil {
		return err
	}
	if current != revision {
		return domain.NewAppError(domain.ErrCodeConflict, "系统成本已变化，请重新核对后保存；本次未改价", nil)
	}
	return nil
}

func (r *costSyncRepo) StageManualCost(ctx context.Context, tx repo.Tx, sku string, observedERP *float64, actor int64, reason string) error {
	return r.stageReviewedCost(ctx, tx, sku, observedERP, actor, reason, "manual_saved", "人工成本已保存，等待ERP回读")
}

// Operator recovery retains automatic-rule provenance instead of labelling an
// audited deterministic recalculation as a human-entered price.
func (r *costSyncRepo) StageRestoredCost(ctx context.Context, tx repo.Tx, sku string, observedERP *float64, actor int64, reason string) error {
	return r.stageReviewedCost(ctx, tx, sku, observedERP, actor, reason, "rule_restored", "既有计价规则已恢复，等待ERP回读")
}

func (r *costSyncRepo) QuarantineRestoredCost(ctx context.Context, tx repo.Tx, sku string, actor int64, report string) error {
	q := Unwrap(tx)
	s, err := scanCostState(q.QueryRowContext(ctx, `SELECT `+syncColumns+` FROM sku_cost_sync_states WHERE sku_code=? FOR UPDATE`, sku))
	if err != nil {
		return err
	}
	if s == nil || s.LocalConfirmed || s.ManualLock {
		return domain.NewAppError(domain.ErrCodeConflict, "价格确认状态已变化", nil)
	}
	s.Status = "conflict"
	s.NeedsCheck = false
	s.Reason = "ERP观察价未经核定；请补齐规格或明确计价方案"
	if err = persistCostState(ctx, q, s); err != nil {
		return err
	}
	return auditCost(ctx, q, s, "rule_quarantined", report, actor)
}

func (r *costSyncRepo) stageReviewedCost(ctx context.Context, tx repo.Tx, sku string, observedERP *float64, actor int64, reason, action, pendingReason string) error {
	q := Unwrap(tx)
	s, err := scanCostState(q.QueryRowContext(ctx, `SELECT `+syncColumns+` FROM sku_cost_sync_states WHERE sku_code=? FOR UPDATE`, sku))
	if err != nil {
		return err
	}
	if s == nil || !s.LocalConfirmed || !domain.ValidObservedCost(s.LocalCost) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "请填写有效人工成本", nil)
	}
	// Even an explicit retry of the same amount is a new intent. Older network
	// acknowledgements must not acknowledge this edit.
	if _, err = q.ExecContext(ctx, `UPDATE sku_cost_sync_states SET revision=revision+1 WHERE sku_code=?`, sku); err != nil {
		return err
	}
	s.Revision++
	if !domain.EqualCost(s.ERPCost, observedERP) {
		s.ERPRevision++
	}
	s.ERPCost = observedERP
	s.Status = "pending"
	s.NeedsCheck = true
	s.Reason = pendingReason
	if err = persistCostState(ctx, q, s); err != nil {
		return err
	}
	return auditCost(ctx, q, s, action, reason, actor)
}
