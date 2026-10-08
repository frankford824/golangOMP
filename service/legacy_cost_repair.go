package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"workflow/domain"
	"workflow/repo"
)

type LegacyCostRepairPreview struct {
	SKU                 string                          `json:"sku"`
	TaskID              int64                           `json:"task_id"`
	OldCost             *float64                        `json:"old_cost"`
	NewCost             *float64                        `json:"new_cost"`
	Revision            int64                           `json:"revision"`
	InputDigest         string                          `json:"input_digest"`
	RuleDigest          string                          `json:"rule_digest"`
	SkipReason          string                          `json:"skip_reason,omitempty"`
	UnreviewedERPImport bool                            `json:"unreviewed_erp_import"`
	Response            *domain.CostRulePreviewResponse `json:"calculation,omitempty"`
}

type legacyCostRecoveryStore interface {
	repo.CostSyncRepo
	repo.ManualCostSyncRepo
	StageRestoredCost(context.Context, repo.Tx, string, *float64, int64, string) error
	QuarantineRestoredCost(context.Context, repo.Tx, string, int64, string) error
}

type LegacyCostRepairService struct {
	tasks    repo.TaskRepo
	rules    repo.CostRuleRepo
	bindings repo.CostRuleBindingRepo
	trace    repo.SKUTraceRepo
	tx       repo.TxRunner
	store    legacyCostRecoveryStore
}

func NewLegacyCostRepairService(tasks repo.TaskRepo, rules repo.CostRuleRepo, bindings repo.CostRuleBindingRepo, trace repo.SKUTraceRepo, tx repo.TxRunner, store repo.CostSyncRepo) *LegacyCostRepairService {
	st, _ := store.(legacyCostRecoveryStore)
	return &LegacyCostRepairService{tasks: tasks, rules: rules, bindings: bindings, trace: trace, tx: tx, store: st}
}

func (s *LegacyCostRepairService) Preview(ctx context.Context, sku string) (*LegacyCostRepairPreview, error) {
	item, err := s.tasks.GetSKUItemBySKUCode(ctx, sku)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("SKU missing")
	}
	p := &LegacyCostRepairPreview{SKU: sku, TaskID: item.TaskID, OldCost: cloneFloat64Ptr(item.CostPrice), InputDigest: LegacyCostPlanDigest(item)}
	if s.store == nil {
		return nil, fmt.Errorf("cost recovery store unavailable")
	}
	state, err := s.store.Get(ctx, sku)
	if err != nil {
		return nil, err
	}
	if state == nil {
		p.SkipReason = "missing_sync_state"
		return p, nil
	}
	p.Revision = state.Revision
	if item.ManualCostOverride && item.OverrideActor != "erp_cost_sync" || state.ManualLock && state.ManualOrigin != "erp" || domain.CostPriceMode(item.CostPriceMode) == domain.CostPriceModeManual {
		p.SkipReason = "preserved_manual_price"
		return p, nil
	}
	p.UnreviewedERPImport = item.ManualCostOverride && item.OverrideActor == "erp_cost_sync" && state.ManualOrigin == "erp"
	// No parent-task dimension fallback in a bulk repair. Each row must carry
	// sufficient own specifications or its own unambiguous product dimensions.
	svc := &taskService{taskRepo: s.tasks, costRuleRepo: s.rules, costRuleBindingRepo: s.bindings, costLegacyAliasFallbackEnabled: false}
	r, appErr := svc.previewTaskSKUItemCost(ctx, &domain.TaskDetail{TaskID: item.TaskID}, item)
	if appErr != nil {
		return nil, fmt.Errorf("%s", appErr.Message)
	}
	p.Response = r.Response
	if r.MatchTrace == nil || (r.MatchTrace.MatchMode != domain.CostRuleMatchModeBindingERPIID && r.MatchTrace.MatchMode != domain.CostRuleMatchModeBindingProductIID) {
		p.SkipReason = "missing_exact_binding"
		return p, nil
	}
	if r.Response == nil || r.Response.RequiresManualReview || r.Response.EstimatedCost == nil {
		p.SkipReason = "incomplete_tariff_or_specification"
		return p, nil
	}
	p.NewCost = cloneFloat64Ptr(r.Response.EstimatedCost)
	boundRules, err := s.rules.ListActiveByCategory(ctx, nil, r.MatchTrace.RuleGroup, time.Now())
	if err != nil {
		return nil, err
	}
	p.RuleDigest = LegacyCostPlanDigest(boundRules)
	if domain.EqualCost(p.OldCost, p.NewCost) && item.OverrideActor != "erp_cost_sync" && !item.RequiresManualReview {
		p.SkipReason = "already_correct"
	}
	return p, nil
}

func (p *LegacyCostRepairPreview) NeedsQuarantine() bool {
	return p != nil && p.UnreviewedERPImport && (p.SkipReason == "missing_exact_binding" || p.SkipReason == "incomplete_tariff_or_specification")
}

func (s *LegacyCostRepairService) Quarantine(ctx context.Context, p *LegacyCostRepairPreview, actor int64, reportID string) error {
	if !p.NeedsQuarantine() || actor <= 0 || reportID == "" {
		return fmt.Errorf("invalid quarantine item")
	}
	release, err := s.store.Lock(ctx, p.SKU)
	if err != nil {
		return err
	}
	defer release()
	return s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.store.LockManualRevision(ctx, tx, p.SKU, p.Revision); err != nil {
			return err
		}
		current, err := s.Preview(ctx, p.SKU)
		if err != nil {
			return err
		}
		if !current.NeedsQuarantine() || current.InputDigest != p.InputDigest {
			return fmt.Errorf("price/specification changed since quarantine preview")
		}
		item, err := s.tasks.GetSKUItemBySKUCode(ctx, p.SKU)
		if err != nil {
			return err
		}
		task, err := s.tasks.GetByID(ctx, item.TaskID)
		if err != nil {
			return err
		}
		detail, err := s.tasks.GetDetailByTaskID(ctx, item.TaskID)
		if err != nil {
			return err
		}
		if task == nil || detail == nil {
			return fmt.Errorf("task context missing")
		}
		item.ManualCostOverride = false
		item.RequiresManualReview = true
		item.OverrideActor = ""
		item.OverrideAt = nil
		item.ManualCostOverrideReason = "ERP观察价未经人工核定；既有规则恢复待补规格或明确方案"
		writer, ok := s.tasks.(interface {
			UpdateSKUCostOnly(context.Context, repo.Tx, *domain.TaskSKUItem, bool) error
		})
		if !ok {
			return fmt.Errorf("cost-only writer unavailable")
		}
		if err = writer.UpdateSKUCostOnly(ctx, tx, item, shouldSyncRunSKUCostToDetail(task, item)); err != nil {
			return err
		}
		snapshot := buildOMPSKUCostSnapshotFromTask(task, detail, item, actor, "legacy_cost_quarantine", reportID, time.Now().UTC())
		if _, err = s.trace.AppendCostSnapshot(ctx, tx, snapshot); err != nil {
			return err
		}
		return s.store.QuarantineRestoredCost(ctx, tx, p.SKU, actor, reportID)
	})
}

// Apply changes only canonical price fields, appends an immutable calculation
// snapshot, and stages the normal cost-only worker using a fresh ERP baseline.
// The caller owns an exclusive, durable recovery report and observed-price check.
func (s *LegacyCostRepairService) Apply(ctx context.Context, p *LegacyCostRepairPreview, erp *float64, actor int64, reportID string) error {
	if p == nil || p.SkipReason != "" || p.NewCost == nil || actor <= 0 || strings.TrimSpace(reportID) == "" {
		return fmt.Errorf("invalid reviewed recovery item")
	}
	release, err := s.store.Lock(ctx, p.SKU)
	if err != nil {
		return err
	}
	defer release()
	return s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.store.LockManualRevision(ctx, tx, p.SKU, p.Revision); err != nil {
			return err
		}
		current, err := s.Preview(ctx, p.SKU)
		if err != nil {
			return err
		}
		if current.SkipReason != "" || current.InputDigest != p.InputDigest || current.RuleDigest != p.RuleDigest || !domain.EqualCost(current.NewCost, p.NewCost) {
			return fmt.Errorf("SKU inputs, rules or manual protection changed since preview")
		}
		item, err := s.tasks.GetSKUItemBySKUCode(ctx, p.SKU)
		if err != nil {
			return err
		}
		task, err := s.tasks.GetByID(ctx, item.TaskID)
		if err != nil {
			return err
		}
		if task == nil {
			return fmt.Errorf("task missing")
		}
		detail, err := s.tasks.GetDetailByTaskID(ctx, item.TaskID)
		if err != nil {
			return err
		}
		if detail == nil {
			return fmt.Errorf("task details missing")
		}
		now := time.Now().UTC()
		item.CostPrice = cloneFloat64Ptr(p.NewCost)
		item.EstimatedCost = cloneFloat64Ptr(p.NewCost)
		item.RequiresManualReview = false
		item.ManualCostOverride = false
		item.ManualCostOverrideReason = ""
		item.OverrideActor = ""
		item.OverrideAt = nil
		item.PrefillSource = "legacy_cost_restore"
		item.PrefillAt = &now
		item.CostRuleID = cloneInt64Ptr(p.Response.MatchedRuleID)
		item.MatchedRuleVersion = cloneIntPtr(p.Response.MatchedRuleVersion)
		item.CostRuleSource = p.Response.RuleSource
		if p.Response.MatchedRule != nil {
			item.CostRuleName = p.Response.MatchedRule.RuleName
		}
		writer, ok := s.tasks.(interface {
			UpdateSKUCostOnly(context.Context, repo.Tx, *domain.TaskSKUItem, bool) error
		})
		if !ok {
			return fmt.Errorf("cost-only writer unavailable")
		}
		// The acknowledgement projection updates parent/mirror prices using the
		// current primary-SKU relationship; never replay stale task business data.
		if err = writer.UpdateSKUCostOnly(ctx, tx, item, false); err != nil {
			return err
		}
		snapshot := buildOMPSKUCostSnapshotFromTask(task, detail, item, actor, "legacy_cost_restore", reportID, now)
		snapshot.CalculationSnapshotJSON = marshalJSONBestEffort(map[string]interface{}{"report": reportID, "old_cost": p.OldCost, "observed_erp": erp, "new_cost": p.NewCost, "response": p.Response, "input_digest": p.InputDigest, "rule_digest": p.RuleDigest})
		if _, err = s.trace.AppendCostSnapshot(ctx, tx, snapshot); err != nil {
			return err
		}
		return s.store.StageRestoredCost(ctx, tx, p.SKU, erp, actor, "历史计价规则恢复；报告 "+reportID)
	})
}
