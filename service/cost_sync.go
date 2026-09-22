package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
	"workflow/domain"
	"workflow/repo"
)

type CostWriteExecutor func(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError)
type CostWriteGuard interface {
	Protect(context.Context, domain.ERPProductUpsertPayload, CostWriteExecutor) (*domain.ERPProductUpsertResult, *domain.AppError)
}
type CostSyncService struct {
	store    repo.CostSyncRepo
	observer ERPBridgeClient
	execute  CostWriteExecutor
	slots    chan struct{}
	wake     chan struct{}
}

func NewCostSyncService(store repo.CostSyncRepo, observer ERPBridgeClient) *CostSyncService {
	return &CostSyncService{store: store, observer: observer, slots: make(chan struct{}, 4), wake: make(chan struct{}, 1)}
}
func (s *CostSyncService) Wake() <-chan struct{} { return s.wake }
func (s *CostSyncService) Notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func (s *CostSyncService) Coverage(ctx context.Context) (*domain.CostSyncCoverage, *domain.AppError) {
	c, err := s.store.Coverage(ctx)
	if err != nil {
		return nil, infraError("cost sync coverage", err)
	}
	return c, nil
}
func AttachCostWriteGuard(bridge ERPBridgeService, s *CostSyncService) {
	if b, ok := bridge.(*erpBridgeService); ok {
		b.costGuard = s
		if s.execute == nil {
			s.execute = b.upsertProductUnchecked
		}
	}
}
func (s *CostSyncService) List(ctx context.Context, status, q string, page, size int) ([]*domain.CostSyncState, domain.PaginationMeta, *domain.AppError) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	rows, total, err := s.store.List(ctx, status, strings.TrimSpace(q), page, size)
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list cost sync states", err)
	}
	return rows, buildPaginationMeta(page, size, total), nil
}
func omitCost(p domain.ERPProductUpsertPayload) domain.ERPProductUpsertPayload {
	p.CostPrice = nil
	if p.BusinessInfo != nil {
		copy := *p.BusinessInfo
		copy.CostPrice = nil
		p.BusinessInfo = &copy
	}
	return p
}
func (s *CostSyncService) fresh(ctx context.Context, sku string) (*domain.ERPProduct, error) {
	if s.observer == nil {
		return nil, fmt.Errorf("未配置直连ERP成本核对，暂停自动写价")
	}
	p, err := s.observer.GetProductByID(ctx, sku)
	if err == nil && p != nil && strings.TrimSpace(firstNonEmptyString(p.SKUID, p.SKUCode, p.ProductID)) != sku {
		return nil, fmt.Errorf("ERP返回的SKU与查询编码不一致")
	}
	return p, err
}
func (s *CostSyncService) Protect(ctx context.Context, p domain.ERPProductUpsertPayload, execute CostWriteExecutor) (*domain.ERPProductUpsertResult, *domain.AppError) {
	sku := strings.TrimSpace(firstNonEmptyString(p.SKUID, p.SKUCode))
	state, err := s.store.Get(ctx, sku)
	if err != nil {
		return nil, infraError("read cost sync guard", err)
	}
	if state == nil {
		return execute(ctx, p)
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return nil, infraError("wait cost sync capacity", ctx.Err())
	}
	release, err := s.store.Lock(ctx, sku)
	if err != nil {
		return nil, infraError("lock cost sync", err)
	}
	defer release()
	block := func(reason string) (*domain.ERPProductUpsertResult, *domain.AppError) {
		if p.Operation == "cost_sync" {
			return nil, domain.NewAppError(domain.ErrCodeConflict, reason, nil)
		}
		// Identity/image delivery must remain independent from cost confirmation.
		return execute(ctx, omitCost(p))
	}
	state, err = s.store.Get(ctx, sku)
	if err != nil || state == nil {
		return nil, infraError("reload cost state", err)
	}
	if !domain.EqualCost(p.CostPrice, state.LocalCost) {
		return block("已有更新的SKU成本，已拦截旧价格")
	}
	product, readErr := s.fresh(ctx, sku)
	if readErr != nil || product == nil {
		// Only the existing full identity creation path may create a new ERP SKU.
		// A cost-only retry must never create/rename a product.
		notFound := product == nil && readErr == nil
		var remoteNotFound *erpBridgeRemoteProductNotFoundError
		var httpNotFound *erpBridgeHTTPError
		if errors.As(readErr, &remoteNotFound) || (errors.As(readErr, &httpNotFound) && httpNotFound.StatusCode == 404) {
			notFound = true
		}
		if app, ok := readErr.(*domain.AppError); ok && app.Code == domain.ErrCodeNotFound {
			notFound = true
		}
		if notFound && state.AckRevision > 0 {
			if err := s.store.Observe(ctx, sku, nil); err != nil {
				return nil, infraError("record missing ERP SKU", err)
			}
			if p.Operation == "cost_sync" && state.LocalCost == nil {
				return &domain.ERPProductUpsertResult{SKUID: sku}, nil
			}
			return block("ERP未找到该SKU，请先确认商品建档或编码映射；未自动创建或写价")
		}
		if !(notFound && p.Operation != "cost_sync" && (state.Status == "pending" || state.Status == "retry") && state.ERPCost == nil && state.AckRevision == 0) {
			_ = s.store.Fail(ctx, sku, state.Revision, "ERP最新成本读取失败，待重试")
			return block("无法核对ERP最新价格，未写入成本")
		}
	} else {
		if err = s.store.Observe(ctx, sku, product.CostPrice); err != nil {
			return nil, infraError("observe ERP cost", err)
		}
	}
	state, err = s.store.Get(ctx, sku)
	if err != nil || state == nil {
		return nil, infraError("reload observed cost", err)
	}
	if state.Status == "conflict" {
		return block(state.Reason)
	}
	if state.Status == "synced" || state.Status == "no_price" {
		if p.Operation == "cost_sync" {
			return &domain.ERPProductUpsertResult{SKUID: sku, SKUCode: sku}, nil
		}
		return execute(ctx, omitCost(p))
	}
	if !domain.EqualCost(state.LocalCost, p.CostPrice) {
		return block("核对期间本地成本发生变化，请使用最新版本")
	}
	if !state.LocalConfirmed || !domain.ValidObservedCost(p.CostPrice) {
		return block("成本尚未确认或金额无效，未写入ERP")
	}
	revision := state.Revision
	result, appErr := execute(ctx, p)
	if appErr != nil {
		_ = s.store.Fail(ctx, sku, revision, appErr.Message)
		return result, appErr
	}
	observed, readErr := s.fresh(ctx, sku)
	if readErr != nil || observed == nil || !domain.EqualCost(observed.CostPrice, p.CostPrice) {
		if readErr == nil && observed != nil {
			_ = s.store.Observe(ctx, sku, observed.CostPrice)
		}
		_ = s.store.Fail(ctx, sku, revision, "ERP写入后回读未确认，不能标记同步完成")
		return result, domain.NewAppError(domain.ErrCodeConflict, "成本回读尚未一致，请查看成本同步状态", nil)
	}
	if err = s.store.Acknowledge(ctx, sku, revision, observed.CostPrice); err != nil {
		return result, infraError("acknowledge ERP cost", err)
	}
	return result, nil
}
func (s *CostSyncService) Resolve(ctx context.Context, p domain.CostSyncResolution) *domain.AppError {
	if strings.TrimSpace(p.Reason) == "" || utf8.RuneCountInString(p.Reason) > 500 || p.Revision < 1 || p.ActorID <= 0 || (p.Choice != "local" && p.Choice != "erp") {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "请选择价格并填写确认原因", nil)
	}
	release, err := s.store.Lock(ctx, p.SKUCode)
	if err != nil {
		return infraError("lock cost resolution", err)
	}
	defer release()
	before, err := s.store.Get(ctx, p.SKUCode)
	if err != nil {
		return infraError("read resolution", err)
	}
	if before == nil {
		return domain.ErrNotFound
	}
	if before.Revision != p.Revision || before.ERPRevision != p.ERPRevision {
		return domain.NewAppError(domain.ErrCodeConflict, "价格已变化，请刷新后确认", nil)
	}
	product, err := s.fresh(ctx, p.SKUCode)
	if err != nil || product == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "无法获取ERP最新价格，未执行确认", nil)
	}
	if !domain.EqualCost(product.CostPrice, before.ERPCost) {
		_ = s.store.Observe(ctx, p.SKUCode, product.CostPrice)
		return domain.NewAppError(domain.ErrCodeConflict, "ERP价格刚刚发生变化，请刷新后重新选择", nil)
	}
	if err = s.store.Resolve(ctx, p); err != nil {
		if app, ok := err.(*domain.AppError); ok {
			return app
		}
		return infraError("resolve SKU cost conflict", err)
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return nil
}
func (s *CostSyncService) RunOnce(ctx context.Context) error {
	collectCtx, stopCollect := context.WithTimeout(ctx, 3*time.Second)
	collectErr := s.store.Collect(collectCtx)
	stopCollect()
	skus, err := s.store.Due(ctx, 4)
	if err != nil {
		return err
	}
	for _, sku := range skus {
		state, err := s.store.Get(ctx, sku)
		if err != nil {
			return err
		}
		if state == nil {
			continue
		}
		taskCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		_, appErr := s.Protect(taskCtx, domain.ERPProductUpsertPayload{ProductID: sku, SKUID: sku, SKUCode: sku, CostPrice: state.LocalCost, Operation: "cost_sync"}, s.execute)
		cancel()
		if appErr != nil {
			_ = s.store.Fail(ctx, sku, state.Revision, appErr.Message)
		}
	}
	if collectErr != nil {
		return collectErr
	}
	if reader, ok := s.observer.(interface {
		BatchCostProducts(context.Context, []string) ([]*domain.ERPProduct, error)
	}); ok {
		ids, err := s.store.BaselineDue(ctx, 50)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			products, err := reader.BatchCostProducts(ctx, ids)
			if err != nil {
				return err
			}
			bySKU := map[string]*domain.ERPProduct{}
			for _, p := range products {
				if p != nil {
					bySKU[strings.TrimSpace(firstNonEmptyString(p.SKUID, p.SKUCode, p.ProductID))] = p
				}
			}
			for _, sku := range ids {
				p := bySKU[sku]
				if p == nil {
					current, e := s.store.Get(ctx, sku)
					if e == nil && current != nil && current.Status == "baseline" {
						_ = s.store.Fail(ctx, sku, current.Revision, "ERP批量核对未返回该SKU，改为单条重试")
					}
					continue
				}
				release, err := s.store.Lock(ctx, sku)
				if err != nil {
					continue
				}
				// The row may have acquired a local edit since this read began.
				current, err := s.store.Get(ctx, sku)
				if err == nil && current != nil && current.Status == "baseline" {
					err = s.store.Observe(ctx, sku, p.CostPrice)
				}
				release()
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
