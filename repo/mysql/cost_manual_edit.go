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
	s.Reason = "人工成本已保存，等待ERP回读"
	if err = persistCostState(ctx, q, s); err != nil {
		return err
	}
	return auditCost(ctx, q, s, "manual_saved", reason, actor)
}
