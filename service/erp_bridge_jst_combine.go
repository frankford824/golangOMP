package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
)

func decodeJSTCombineSKUList(payload []byte, pageIndex, pageSize int) (*domain.JSTCombineSKUListResponse, error) {
	rows, total, err := jstExtractSkuRows(payload)
	if err != nil {
		return nil, fmt.Errorf("decode jst combine sku response: %w", err)
	}
	items := make([]domain.JSTCombineSKUItem, 0, len(rows))
	for _, row := range rows {
		item := jstMapToCombineSKUItem(row)
		if strings.TrimSpace(item.ComboSKUCode) == "" {
			continue
		}
		items = append(items, item)
	}
	if pageIndex <= 0 {
		pageIndex = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	return &domain.JSTCombineSKUListResponse{
		Items: items,
		Pagination: domain.PaginationMeta{
			Page:     pageIndex,
			PageSize: pageSize,
			Total:    total,
		},
	}, nil
}

func jstMapToCombineSKUItem(row map[string]interface{}) domain.JSTCombineSKUItem {
	raw, _ := json.Marshal(row)
	item := domain.JSTCombineSKUItem{
		ComboSKUCode: firstString(row, "sku_id", "sku_code"),
		Name:         firstString(row, "name", "sku_name", "title"),
		ShortName:    firstString(row, "short_name"),
		IID:          firstString(row, "i_id", "iId"),
		Enabled:      jstBoolPtr(row, "enabled"),
		CostPrice:    firstFloatPtr(row, "cost_price", "c_price"),
		SalePrice:    firstFloatPtr(row, "sale_price", "s_price"),
		ModifiedAt:   jstTimePtr(firstString(row, "modified", "modified_at", "update_time")),
		Children:     jstCombineChildren(row),
		RawPayload:   raw,
	}
	return item
}

func jstCombineChildren(row map[string]interface{}) []domain.JSTCombineSKUChild {
	rawItems, ok := row["items"].([]interface{})
	if !ok {
		rawItems, _ = row["item_list"].([]interface{})
	}
	children := make([]domain.JSTCombineSKUChild, 0, len(rawItems))
	for _, raw := range rawItems {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		sku := firstString(m, "src_sku_id", "sku_id", "sku_code")
		if sku == "" {
			continue
		}
		qty := float64(1)
		if parsed := firstFloatPtr(m, "qty", "quantity", "sku_qty"); parsed != nil && *parsed > 0 {
			qty = *parsed
		}
		children = append(children, domain.JSTCombineSKUChild{SKUCode: sku, Quantity: qty})
	}
	return children
}

func buildJSTCombineSKUQueryBizFilter(filter domain.JSTCombineSKUFilter) map[string]interface{} {
	page := filter.PageIndex
	if page < 1 {
		page = 1
	}
	ps := filter.PageSize
	if ps < 1 {
		ps = 50
	}
	if ps > 50 {
		ps = 50
	}
	biz := map[string]interface{}{
		"page_index": strconv.Itoa(page),
		"page_size":  strconv.Itoa(ps),
	}
	if skuIDs := strings.TrimSpace(filter.SKUIDs); skuIDs != "" {
		biz["sku_ids"] = skuIDs
	}
	if begin := strings.TrimSpace(filter.ModifiedBegin); begin != "" {
		biz["modified_begin"] = begin
	}
	if end := strings.TrimSpace(filter.ModifiedEnd); end != "" {
		biz["modified_end"] = end
	}
	return biz
}

func jstBoolPtr(row map[string]interface{}, key string) *bool {
	raw, ok := row[key]
	if !ok || raw == nil {
		return nil
	}
	switch value := raw.(type) {
	case bool:
		return &value
	case float64:
		parsed := value != 0
		return &parsed
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			return nil
		}
		parsed := trimmed == "1" || trimmed == "true" || trimmed == "yes" || trimmed == "启用"
		return &parsed
	}
	return nil
}

func jstTimePtr(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return &parsed
		}
	}
	return nil
}
