package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"workflow/domain"
)

func TestDecodeJSTHistoryCostResponseSupportsWarehouseMap(t *testing.T) {
	payload := []byte(`{
		"code": 0,
		"data": {
			"sku_history_cost_price_maps": {
				"1001": [
					{"sku_id":"SKU-A","cost_price":"5.3619","begin_date":"2026-01-01","end_date":"2026-12-31","remark":"annual"}
				]
			}
		}
	}`)
	result, err := decodeJSTHistoryCostResponse(payload)
	if err != nil {
		t.Fatalf("decodeJSTHistoryCostResponse() error = %v", err)
	}
	if len(result.Periods) != 1 {
		t.Fatalf("periods = %+v", result.Periods)
	}
	period := result.Periods[0]
	if period.WMSCoID != "1001" || period.SKUID != "SKU-A" || period.CostPrice == nil || *period.CostPrice != "5.3619" {
		t.Fatalf("period = %+v", period)
	}
}

func TestRemoteERPBridgeHistoryCostUsesConfiguredOpenWebPathAndBiz(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/history-cost" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		var biz map[string]interface{}
		if err := json.Unmarshal([]byte(r.Form.Get("biz")), &biz); err != nil {
			t.Fatalf("decode biz: %v; raw=%q", err, r.Form.Get("biz"))
		}
		skuIDs, _ := biz["sku_ids"].([]interface{})
		if len(skuIDs) != 2 || skuIDs[0] != "SKU-A" || skuIDs[1] != "SKU-B" {
			t.Fatalf("sku_ids = %#v", biz["sku_ids"])
		}
		if biz["get_way"] != "all" || biz["is_use_item_sku_cost_price"] != true {
			t.Fatalf("biz = %#v", biz)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"skuHistoryCostPriceMaps":{"1001":[{"skuId":"SKU-A","costPrice":"5.3619","beginDate":"2026-01-01"}]}}}`))
	}))
	defer server.Close()

	client, err := NewRemoteERPBridgeClient(ERPRemoteClientConfig{
		BaseURL: server.URL, HistoryCostPath: "/history-cost", AuthMode: "openweb",
		AppKey: "app", AppSecret: "secret", AccessToken: "access", RetryMax: 0,
	})
	if err != nil {
		t.Fatalf("NewRemoteERPBridgeClient() error = %v", err)
	}
	provider, ok := client.(JSTHistoryCostProvider)
	if !ok {
		t.Fatal("remote client does not implement JSTHistoryCostProvider")
	}
	result, err := provider.QueryHistoryCosts(context.Background(), domain.JSTHistoryCostQuery{
		SKUIDs: []string{"SKU-A", "SKU-B"}, GetWay: "all", IsUseItemSKUCostPrice: true,
	})
	if err != nil {
		t.Fatalf("QueryHistoryCosts() error = %v", err)
	}
	if len(result.Periods) != 1 || result.Periods[0].SKUID != "SKU-A" {
		t.Fatalf("result = %+v", result)
	}
}
