package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"workflow/domain"
)

func (c *remoteERPBridgeClient) QueryHistoryCosts(ctx context.Context, query domain.JSTHistoryCostQuery) (*domain.JSTHistoryCostResponse, error) {
	if !strings.EqualFold(strings.TrimSpace(c.authMode), "openweb") {
		return nil, fmt.Errorf("%w: JST history cost query requires ERP_REMOTE_AUTH_MODE=openweb", ErrERPRemoteOpenWebAuthRequired)
	}
	raw, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal JST history cost query: %w", err)
	}
	responseBody, err := c.doRequestWithRetry(ctx, http.MethodPost, c.historyCostPath, nil, raw, "jst_history_cost_query")
	if err != nil {
		return nil, err
	}
	return decodeJSTHistoryCostResponse(responseBody)
}

func (c *hybridERPBridgeClient) QueryHistoryCosts(ctx context.Context, query domain.JSTHistoryCostQuery) (*domain.JSTHistoryCostResponse, error) {
	provider, ok := c.remote.(JSTHistoryCostProvider)
	if !ok || provider == nil {
		return nil, fmt.Errorf("JST history cost query requires the remote OpenWeb client")
	}
	// Historical cost is uniquely upstream-owned. Never substitute the local
	// products cache when OpenWeb is unavailable.
	return provider.QueryHistoryCosts(ctx, query)
}

func decodeJSTHistoryCostResponse(payload []byte) (*domain.JSTHistoryCostResponse, error) {
	root, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil, err
	}
	container := unwrapERPBridgePayload(root)
	mapped, ok := container.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("JST history cost response data is not an object")
	}
	var rawMaps interface{}
	for _, key := range []string{"sku_history_cost_price_maps", "skuHistoryCostPriceMaps", "SkuHistoryCostPriceMaps"} {
		if value, exists := mapped[key]; exists {
			rawMaps = value
			break
		}
	}
	if rawMaps == nil {
		return &domain.JSTHistoryCostResponse{Periods: []domain.JSTHistoryCostPeriod{}}, nil
	}
	wmsMaps, ok := rawMaps.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("JST history cost maps field is not an object")
	}
	periods := make([]domain.JSTHistoryCostPeriod, 0)
	for wmsCoID, rawRows := range wmsMaps {
		rows, ok := rawRows.([]interface{})
		if !ok {
			continue
		}
		for _, rawRow := range rows {
			row, ok := rawRow.(map[string]interface{})
			if !ok {
				continue
			}
			period := domain.JSTHistoryCostPeriod{
				WMSCoID:   strings.TrimSpace(wmsCoID),
				SKUID:     firstString(row, "sku_id", "skuId", "SkuId"),
				BeginDate: firstString(row, "begin_date", "beginDate", "BeginDate"),
				EndDate:   firstString(row, "end_date", "endDate", "EndDate"),
				Remark:    firstString(row, "remark", "Remark"),
			}
			if costPrice := firstString(row, "cost_price", "costPrice", "CostPrice"); costPrice != "" {
				period.CostPrice = &costPrice
			}
			if period.SKUID != "" {
				periods = append(periods, period)
			}
		}
	}
	return &domain.JSTHistoryCostResponse{Periods: periods}, nil
}
