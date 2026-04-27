package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
)

// jstExtractSkuRows pulls SKU rows from common Jushuitan OpenWeb response envelopes.
func jstExtractSkuRows(payload []byte) ([]map[string]interface{}, int64, error) {
	rootIface, err := decodeERPBridgePayload(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("decode jst sku response: %w", err)
	}
	root, ok := rootIface.(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("jst sku response root is not an object")
	}
	if code := firstInt(root, "code", "errcode", "error_code"); code != 0 {
		msg := firstString(root, "msg", "message", "error_msg")
		return nil, 0, fmt.Errorf("jst sku query business code %d: %s", code, strings.TrimSpace(msg))
	}
	dataIface := unwrapERPBridgePayload(root)
	var data map[string]interface{}
	if dm, ok := dataIface.(map[string]interface{}); ok {
		data = dm
	}
	rows := jstCollectSkuRows(data)
	if len(rows) == 0 {
		rows = jstCollectSkuRows(root)
	}
	total := int64(len(rows))
	if data != nil {
		if v := firstInt64ish(data, "data_count", "total", "count"); v > 0 {
			total = v
		}
	}
	if v := firstInt64ish(root, "data_count", "total"); v > 0 && int64(len(rows)) < v {
		total = v
	}
	return rows, total, nil
}

func jstCollectSkuRows(node map[string]interface{}) []map[string]interface{} {
	if node == nil {
		return nil
	}
	for _, key := range []string{"datas", "items", "skus", "list", "data_list"} {
		if raw, ok := node[key]; ok {
			if arr, ok := raw.([]interface{}); ok {
				out := make([]map[string]interface{}, 0, len(arr))
				for _, it := range arr {
					if m, ok := it.(map[string]interface{}); ok {
						out = append(out, m)
					}
				}
				if len(out) > 0 {
					return out
				}
			}
		}
	}
	return nil
}

func firstInt64ish(m map[string]interface{}, keys ...string) int64 {
	if m == nil {
		return 0
	}
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case float64:
			return int64(x)
		case int64:
			return x
		case int:
			return int64(x)
		case string:
			if n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64); err == nil {
				return n
			}
		}
	}
	return 0
}

func jstRowSKUCode(m map[string]interface{}) string {
	return firstNonEmptyString(
		firstString(m, "sku_id", "sku_code", "sku_code_id"),
		firstString(m, "item_code", "outer_id"),
	)
}

func jstRowName(m map[string]interface{}) string {
	return firstNonEmptyString(firstString(m, "name", "sku_name", "title"), "")
}

func jstMapsToERPProducts(rows []map[string]interface{}, keyword string) []*domain.ERPProduct {
	kw := strings.ToLower(strings.TrimSpace(keyword))
	var out []*domain.ERPProduct
	for _, m := range rows {
		sku := jstRowSKUCode(m)
		if sku == "" {
			continue
		}
		name := jstRowName(m)
		if kw != "" {
			if !strings.Contains(strings.ToLower(sku), kw) &&
				!strings.Contains(strings.ToLower(name), kw) &&
				!strings.Contains(strings.ToLower(firstString(m, "i_id", "iId")), kw) {
				continue
			}
		}
		var price *float64
		if sp := firstString(m, "sale_price", "s_price"); sp != "" {
			if f, err := strconv.ParseFloat(sp, 64); err == nil {
				price = &f
			}
		}
		out = append(out, &domain.ERPProduct{
			ProductID:   sku,
			SKUID:       sku,
			IID:         firstString(m, "i_id", "iId"),
			SKUCode:     sku,
			Name:        name,
			ProductName: firstNonEmptyString(name, sku),
			CategoryID:  firstString(m, "c_id", "category_id"),
			CategoryName: firstNonEmptyString(
				firstString(m, "category", "category_name", "vc_name"),
				"",
			),
			ImageURL: firstNonEmptyString(firstString(m, "pic_big", "pic"), ""),
			SPrice:   price,
		})
	}
	return out
}

func jstMapsToERPProductRecords(rows []map[string]interface{}) []domain.ERPProductRecord {
	var out []domain.ERPProductRecord
	now := time.Now().UTC()
	for _, m := range rows {
		sku := jstRowSKUCode(m)
		if sku == "" {
			continue
		}
		name := firstNonEmptyString(jstRowName(m), sku)
		cat := firstNonEmptyString(firstString(m, "category", "category_name", "vc_name"), "")
		spec := map[string]interface{}{
			"i_id":              firstString(m, "i_id", "iId"),
			"properties_value":  firstString(m, "properties_value"),
			"jst_sale_price":    firstString(m, "sale_price"),
			"jst_cost_price":    firstString(m, "cost_price"),
			"jst_source":        "jst_openweb_sku_query",
		}
		b, _ := json.Marshal(spec)
		out = append(out, domain.ERPProductRecord{
			ERPProductID:    sku,
			SKUCode:         sku,
			ProductName:     name,
			Category:        cat,
			SpecJSON:        string(b),
			Status:          "active",
			SourceUpdatedAt: &now,
		})
	}
	return out
}

func buildJSTSkuQueryBizFilter(filter domain.ERPProductSearchFilter) map[string]interface{} {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	ps := filter.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 50 {
		ps = 50
	}
	biz := map[string]interface{}{
		"page_index": strconv.Itoa(page),
		"page_size":  strconv.Itoa(ps),
	}
	skuKey := strings.TrimSpace(filter.SKUCode)
	if skuKey == "" {
		skuKey = strings.TrimSpace(filter.Q)
	}
	if skuKey == "" {
		skuKey = strings.TrimSpace(filter.Keyword)
	}
	if skuKey != "" {
		biz["sku_ids"] = skuKey
	} else {
		end := time.Now()
		begin := end.AddDate(0, 0, -6)
		biz["modified_begin"] = begin.Format("2006-01-02 15:04:05")
		biz["modified_end"] = end.Format("2006-01-02 15:04:05")
	}
	return biz
}
