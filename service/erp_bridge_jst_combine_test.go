package service

import (
	"testing"
)

func TestDecodeJSTCombineSKUList(t *testing.T) {
	raw := []byte(`{
	  "code": 0,
	  "data": {
	    "data_count": 1,
	    "datas": [
	      {
	        "sku_id": "COMBO001",
	        "name": "组合装一",
	        "i_id": "IID001",
	        "enabled": true,
	        "cost_price": "12.50",
	        "sale_price": "29.90",
	        "modified": "2026-06-18 10:00:00",
	        "items": [
	          {"src_sku_id": "SKU001", "qty": 2},
	          {"src_sku_id": "SKU002", "qty": "3"}
	        ]
	      }
	    ]
	  }
	}`)
	got, err := decodeJSTCombineSKUList(raw, 1, 50)
	if err != nil {
		t.Fatalf("decodeJSTCombineSKUList() error = %v", err)
	}
	if got.Pagination.Total != 1 || len(got.Items) != 1 {
		t.Fatalf("result = %#v", got)
	}
	item := got.Items[0]
	if item.ComboSKUCode != "COMBO001" || item.IID != "IID001" {
		t.Fatalf("item identity = %#v", item)
	}
	if item.CostPrice == nil || *item.CostPrice != 12.5 {
		t.Fatalf("cost = %#v", item.CostPrice)
	}
	if len(item.Children) != 2 || item.Children[0].SKUCode != "SKU001" || item.Children[0].Quantity != 2 || item.Children[1].Quantity != 3 {
		t.Fatalf("children = %#v", item.Children)
	}
}

func TestDecodeJSTCombineSKUListAcceptsLocalBridgeEnvelope(t *testing.T) {
	raw := []byte(`{
	  "data": [
	    {
	      "sku_id": "COMBO002",
	      "name": "组合装二",
	      "items": [{"src_sku_id": "SKU003", "qty": 1}]
	    }
	  ],
	  "pagination": {"page": 2, "page_size": 50, "total": 88}
	}`)
	got, err := decodeJSTCombineSKUList(raw, 2, 50)
	if err != nil {
		t.Fatalf("decodeJSTCombineSKUList() error = %v", err)
	}
	if got.Pagination.Total != 88 || len(got.Items) != 1 {
		t.Fatalf("result = %#v", got)
	}
	if got.Items[0].ComboSKUCode != "COMBO002" || len(got.Items[0].Children) != 1 {
		t.Fatalf("item = %#v", got.Items[0])
	}
}
