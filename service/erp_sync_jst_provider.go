package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"workflow/domain"
)

// JSTOpenWebProductProvider loads products from Jushuitan OpenWeb SKU query (same path as Bridge remote search).
type JSTOpenWebProductProvider struct {
	cfg ERPRemoteClientConfig
}

func NewJSTOpenWebProductProvider(cfg ERPRemoteClientConfig) ERPProductProvider {
	return &JSTOpenWebProductProvider{cfg: cfg}
}

func (p *JSTOpenWebProductProvider) FetchProducts(ctx context.Context) ([]domain.ERPProductRecord, error) {
	if !strings.EqualFold(strings.TrimSpace(p.cfg.AuthMode), "openweb") {
		return nil, &jstSyncConfigError{msg: "ERP_SYNC_SOURCE_MODE=jst requires ERP_REMOTE_AUTH_MODE=openweb and valid JST credentials"}
	}
	client, err := NewRemoteERPBridgeClient(p.cfg)
	if err != nil {
		return nil, err
	}
	const maxPages = 200
	const pageSize = 50
	const pageThrottle = 500 * time.Millisecond
	const rateLimitRetries = 3
	seen := make(map[string]struct{})
	var records []domain.ERPProductRecord
	for page := 1; page <= maxPages; page++ {
		var res *domain.ERPProductListResponse
		for attempt := 1; attempt <= rateLimitRetries; attempt++ {
			res, err = client.SearchProducts(ctx, domain.ERPProductSearchFilter{
				Page:     page,
				PageSize: pageSize,
			})
			if err == nil {
				break
			}
			if !isJSTOpenWebRateLimited(err) || attempt >= rateLimitRetries {
				return records, err
			}
			if err := sleepWithContext(ctx, time.Duration(attempt)*3*time.Second); err != nil {
				return records, err
			}
		}
		if err != nil {
			return records, err
		}
		if res == nil || len(res.Items) == 0 {
			break
		}
		for _, it := range res.Items {
			sku := strings.TrimSpace(firstNonEmptyString(it.SKUCode, it.SKUID))
			if sku == "" {
				continue
			}
			if _, ok := seen[sku]; ok {
				continue
			}
			seen[sku] = struct{}{}
			name := firstNonEmptyString(it.ProductName, it.Name, sku)
			cat := strings.TrimSpace(it.CategoryName)
			// One local row per SKU; erp style/product ids live in spec_json for mapping (avoids collapsing variants).
			spec, _ := json.Marshal(map[string]interface{}{
				"i_id":           strings.TrimSpace(it.IID),
				"erp_product_id": strings.TrimSpace(it.ProductID),
				"sku_id":         strings.TrimSpace(it.SKUID),
				"sku_code":       sku,
				"jst_sync_page":  page,
				"jst_sync_at":    time.Now().UTC().Format(time.RFC3339),
				"sync_role":      "8080_products_replica_from_openweb",
			})
			records = append(records, domain.ERPProductRecord{
				ERPProductID: sku,
				SKUCode:      sku,
				ProductName:  name,
				Category:     cat,
				SpecJSON:     string(spec),
				Status:       "active",
			})
		}
		if len(res.Items) < pageSize {
			break
		}
		if err := sleepWithContext(ctx, pageThrottle); err != nil {
			return records, err
		}
	}
	return records, nil
}

func isJSTOpenWebRateLimited(err error) bool {
	var openWebErr *erpBridgeOpenWebError
	return errors.As(err, &openWebErr) && openWebErr.Code == 199
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

type jstSyncConfigError struct {
	msg string
}

func (e *jstSyncConfigError) Error() string { return e.msg }
