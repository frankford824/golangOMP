package mysqlrepo

import (
	"context"
	"workflow/domain"
	"workflow/repo"
)

// Manual price edits must not replay product identity, quantities or filing
// snapshots loaded before the ERP baseline lookup.
func (r *taskRepo) UpdateSKUCostOnly(ctx context.Context, tx repo.Tx, item *domain.TaskSKUItem, syncDetail bool) error {
	q := Unwrap(tx)
	_, err := q.ExecContext(ctx, `UPDATE task_sku_items SET cost_price=?,estimated_cost=?,cost_rule_id=?,cost_rule_name=?,cost_rule_source=?,matched_rule_version=?,prefill_source=?,prefill_at=?,requires_manual_review=?,manual_cost_override=?,manual_cost_override_reason=?,override_actor=?,override_at=?,updated_at=CURRENT_TIMESTAMP WHERE id=? AND task_id=?`,
		item.CostPrice, item.EstimatedCost, item.CostRuleID, item.CostRuleName, item.CostRuleSource, item.MatchedRuleVersion, item.PrefillSource, item.PrefillAt, item.RequiresManualReview, item.ManualCostOverride, item.ManualCostOverrideReason, item.OverrideActor, item.OverrideAt, item.ID, item.TaskID)
	if err != nil || !syncDetail {
		return err
	}
	_, err = q.ExecContext(ctx, `UPDATE task_details d JOIN task_sku_items s ON s.task_id=d.task_id SET d.cost_price=s.cost_price,d.estimated_cost=s.estimated_cost,d.cost_rule_id=s.cost_rule_id,d.cost_rule_name=s.cost_rule_name,d.cost_rule_source=s.cost_rule_source,d.matched_rule_version=s.matched_rule_version,d.prefill_source=s.prefill_source,d.prefill_at=s.prefill_at,d.requires_manual_review=s.requires_manual_review,d.manual_cost_override=s.manual_cost_override,d.manual_cost_override_reason=s.manual_cost_override_reason,d.override_actor=s.override_actor,d.override_at=s.override_at,d.updated_at=CURRENT_TIMESTAMP WHERE s.id=? AND s.task_id=?`, item.ID, item.TaskID)
	return err
}
