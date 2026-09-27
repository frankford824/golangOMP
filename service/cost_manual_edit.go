package service

import (
	"context"
	"strings"
	"workflow/domain"
	"workflow/repo"
)

type ManualCostEdit struct {
	store    repo.ManualCostSyncRepo
	sku      string
	revision int64
	erp      *float64
	release  func()
}

func (g *ManualCostEdit) Close() {
	if g != nil {
		g.release()
	}
}
func (g *ManualCostEdit) Lock(ctx context.Context, tx repo.Tx) error {
	if g == nil {
		return nil
	}
	return g.store.LockManualRevision(ctx, tx, g.sku, g.revision)
}
func (g *ManualCostEdit) Stage(ctx context.Context, tx repo.Tx, actor int64, reason string) error {
	if g == nil {
		return nil
	}
	return g.store.StageManualCost(ctx, tx, g.sku, g.erp, actor, reason)
}

func (s *CostSyncService) TaskView(ctx context.Context, sku string, refresh bool) (*domain.TaskCostSyncView, *domain.AppError) {
	state, err := s.store.Get(ctx, sku)
	if err != nil {
		return nil, infraError("read task cost sync", err)
	}
	v := &domain.TaskCostSyncView{State: state}
	if !refresh {
		return v, nil
	}
	p, err := s.fresh(ctx, sku)
	if err != nil || p == nil {
		v.Message = "暂未取得ERP最新成本，可保存到任务；核对恢复后再同步"
		return v, nil
	}
	if state == nil {
		v.Message = "该SKU尚未建立成本同步记录"
		return v, nil
	}
	v.ERPAvailable = true
	v.Baseline = &domain.CostEditBaseline{Revision: state.Revision, ERPCost: p.CostPrice}
	return v, nil
}

func (s *CostSyncService) PrepareManualEdit(ctx context.Context, sku string, baseline *domain.CostEditBaseline) (*ManualCostEdit, *domain.AppError) {
	if baseline == nil {
		return nil, nil
	} // Older callers retain conservative asynchronous handling.
	if baseline.Revision < 1 || (baseline.ERPCost != nil && !domain.ValidObservedCost(baseline.ERPCost)) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "成本核对信息无效，请重新核对", nil)
	}
	store, ok := s.store.(repo.ManualCostSyncRepo)
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "人工成本同步未配置", nil)
	}
	release, err := s.store.Lock(ctx, sku)
	if err != nil {
		return nil, infraError("lock manual cost edit", err)
	}
	accepted := false
	defer func() {
		if !accepted {
			release()
		}
	}()
	state, err := s.store.Get(ctx, sku)
	if err != nil {
		return nil, infraError("read manual cost baseline", err)
	}
	if state == nil || state.Revision != baseline.Revision {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "系统成本已变化，请重新核对后保存；本次未改价", nil)
	}
	p, err := s.fresh(ctx, sku)
	if err != nil || p == nil {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "暂时无法核对ERP最新价格，请稍后重试；本次未改价", nil)
	}
	if !domain.EqualCost(p.CostPrice, baseline.ERPCost) {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "ERP价格在编辑期间发生变化，请重新核对后保存；本次未改价", nil)
	}
	accepted = true
	return &ManualCostEdit{store: store, sku: strings.TrimSpace(sku), revision: baseline.Revision, erp: p.CostPrice, release: release}, nil
}
