package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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

func TestRemoteERPBridgeHistoryCostRetriesJSTRateLimit(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if attempts.Add(1) == 1 {
			_, _ = w.Write([]byte(`{"code":199,"msg":"调用太频繁，请稍后再试!"}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"skuHistoryCostPriceMaps":{"1001":[{"skuId":"SKU-A","costPrice":"5.3619","beginDate":"2026-01-01"}]}}}`))
	}))
	defer server.Close()

	client, err := NewRemoteERPBridgeClient(ERPRemoteClientConfig{
		BaseURL: server.URL, HistoryCostPath: "/history-cost", AuthMode: "openweb",
		AppKey: "app", AppSecret: "secret", AccessToken: "access",
		RetryMax: 1, RetryBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewRemoteERPBridgeClient() error = %v", err)
	}
	result, err := client.(JSTHistoryCostProvider).QueryHistoryCosts(context.Background(), domain.JSTHistoryCostQuery{
		SKUIDs: []string{"SKU-A"}, GetWay: "all", IsUseItemSKUCostPrice: true,
	})
	if err != nil {
		t.Fatalf("QueryHistoryCosts() error = %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
	if len(result.Periods) != 1 || result.Periods[0].SKUID != "SKU-A" {
		t.Fatalf("result = %+v", result)
	}
}

func TestERPRemoteOpenWebRateLimitClassification(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		message string
		want    bool
	}{
		{name: "jst code 199", code: 199, message: "调用太频繁，请稍后再试!", want: true},
		{name: "jst code 200", code: 200, message: "调用频次超过限制!", want: true},
		{name: "english", code: 429, message: "rate limit exceeded", want: true},
		{name: "business validation", code: 200, message: "参数错误", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isERPRemoteOpenWebRateLimit(tt.code, tt.message); got != tt.want {
				t.Fatalf("isERPRemoteOpenWebRateLimit(%d, %q) = %v, want %v", tt.code, tt.message, got, tt.want)
			}
		})
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
