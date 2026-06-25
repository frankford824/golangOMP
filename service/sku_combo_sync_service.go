package service

import (
	"context"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type SKUComboSyncService interface {
	ProcessNextPage(ctx context.Context) (int, *domain.AppError)
}

type skuComboSyncService struct {
	erpBridge  ERPBridgeService
	skuCombos  repo.SKUComboRepo
	txRunner   repo.TxRunner
	now        func() time.Time
	windowSize time.Duration
}

var jstOpenWebLocation = time.FixedZone("Asia/Shanghai", 8*3600)

func NewSKUComboSyncService(erpBridge ERPBridgeService, skuCombos repo.SKUComboRepo, txRunner repo.TxRunner) SKUComboSyncService {
	return &skuComboSyncService{
		erpBridge:  erpBridge,
		skuCombos:  skuCombos,
		txRunner:   txRunner,
		now:        time.Now,
		windowSize: 7 * 24 * time.Hour,
	}
}

func (s *skuComboSyncService) ProcessNextPage(ctx context.Context) (int, *domain.AppError) {
	if s == nil || s.erpBridge == nil || s.skuCombos == nil || s.txRunner == nil {
		return 0, nil
	}
	now := s.now().In(jstOpenWebLocation)
	state, err := s.skuCombos.EnsureNextSyncWindow(ctx, now, s.windowSize)
	if err != nil {
		return 0, infraAppError("ensure sku combo sync window", err)
	}
	if state == nil {
		return 0, nil
	}
	if strings.TrimSpace(state.Status) == "failed" && state.NextRetryAt != nil && state.NextRetryAt.After(now) {
		return 0, nil
	}
	claimed := false
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		var claimErr error
		claimed, claimErr = s.skuCombos.ClaimSyncState(ctx, tx, state.ID, now)
		return claimErr
	}); err != nil {
		return 0, infraAppError("claim sku combo sync window", err)
	}
	if !claimed {
		return 0, nil
	}
	pageSize := state.PageSize
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 50
	}
	pageIndex := state.PageIndex
	if pageIndex <= 0 {
		pageIndex = 1
	}
	result, appErr := s.erpBridge.QueryCombineSKUs(ctx, domain.JSTCombineSKUFilter{
		ModifiedBegin: state.WindowBegin.Format("2006-01-02 15:04:05"),
		ModifiedEnd:   state.WindowEnd.Format("2006-01-02 15:04:05"),
		PageIndex:     pageIndex,
		PageSize:      pageSize,
	})
	if appErr != nil {
		retryAt := now.Add(10 * time.Minute)
		if strings.Contains(strings.ToLower(appErr.Message), "频次") || strings.Contains(strings.ToLower(appErr.Message), "rate") {
			retryAt = now.Add(30 * time.Minute)
		}
		if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			return s.skuCombos.MarkSyncStateFailed(ctx, tx, state.ID, appErr.Message, retryAt)
		}); err != nil {
			return 0, infraAppError("mark sku combo sync failed", err)
		}
		return 0, appErr
	}
	if result == nil {
		result = &domain.JSTCombineSKUListResponse{}
	}
	processed := len(result.Items)
	finished := processed == 0
	if result.Pagination.Total > 0 {
		finished = int64(pageIndex*pageSize) >= result.Pagination.Total
	} else if processed < pageSize {
		finished = true
	}
	nextPage := pageIndex + 1
	if finished {
		nextPage = 1
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		for _, item := range result.Items {
			if err := s.upsertComboItem(ctx, tx, item, now); err != nil {
				return err
			}
		}
		return s.skuCombos.MarkSyncStateSuccess(ctx, tx, state.ID, nextPage, processed, finished, now)
	}); err != nil {
		return 0, infraAppError("persist sku combo sync page", err)
	}
	return processed, nil
}

func (s *skuComboSyncService) upsertComboItem(ctx context.Context, tx repo.Tx, item domain.JSTCombineSKUItem, now time.Time) error {
	comboCode := strings.TrimSpace(item.ComboSKUCode)
	if comboCode == "" {
		return nil
	}
	if err := s.skuCombos.UpsertComboRecord(ctx, tx, &domain.OMPSKUComboRecord{
		ComboSKUCode:   comboCode,
		Name:           item.Name,
		ShortName:      item.ShortName,
		ERPIID:         item.IID,
		EntitySKUID:    item.EntitySKUID,
		PicURL:         item.PicURL,
		Brand:          item.Brand,
		VCName:         item.VCName,
		Properties:     item.Properties,
		Enabled:        item.Enabled,
		CostPrice:      item.CostPrice,
		SalePrice:      item.SalePrice,
		Weight:         item.Weight,
		SKUQty:         item.SKUQty,
		ERPCreatedAt:   item.ERPCreatedAt,
		ModifiedAt:     item.ModifiedAt,
		Source:         "jst_openweb_combine_sku_query",
		RawPayloadJSON: item.RawPayload,
		LastSyncedAt:   now,
	}); err != nil {
		return err
	}
	for _, child := range item.Children {
		childSKU := strings.TrimSpace(child.SKUCode)
		if childSKU == "" {
			continue
		}
		qty := child.Quantity
		if qty <= 0 {
			qty = 1
		}
		if err := s.skuCombos.UpsertComboRelation(ctx, tx, &domain.OMPSKUComboRelation{
			ComboSKUCode:   comboCode,
			ChildSKUCode:   childSKU,
			Quantity:       qty,
			Source:         "jst_openweb_combine_sku_query",
			RawPayloadJSON: item.RawPayload,
			FirstSeenAt:    now,
			LastSeenAt:     now,
		}); err != nil {
			return err
		}
	}
	currentChildSKUs := make([]string, 0, len(item.Children))
	for _, child := range item.Children {
		if sku := strings.TrimSpace(child.SKUCode); sku != "" {
			currentChildSKUs = append(currentChildSKUs, sku)
		}
	}
	if err := s.skuCombos.DeleteStaleComboRelations(ctx, tx, comboCode, "jst_openweb_combine_sku_query", currentChildSKUs); err != nil {
		return err
	}
	return nil
}
