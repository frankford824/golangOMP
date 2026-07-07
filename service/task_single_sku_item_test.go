package service

import (
	"encoding/json"
	"testing"

	"workflow/domain"
)

func TestBuildSingleTaskSKUItemsPersistsProductIIDInVariantJSON(t *testing.T) {
	items := buildSingleTaskSKUItems(
		&domain.Task{
			TaskType:            domain.TaskTypeNewProductDevelopment,
			SKUCode:             "CGK000613",
			ProductNameSnapshot: "露岩常规kt板/乐考迎宾牌/趣味古诗60*80cm",
		},
		&domain.TaskDetail{
			CategoryName: "常规kt板",
			Category:     "备用分类",
			SKUCodeType:  domain.TaskSKUCodeTypeRegular,
		},
	)

	if len(items) != 1 || items[0] == nil || items[0].Item == nil {
		t.Fatalf("items = %+v, want one SKU item", items)
	}
	item := items[0].Item
	if item.ProductIID != "常规kt板" {
		t.Fatalf("ProductIID = %q, want task product_i_id", item.ProductIID)
	}
	var variant map[string]string
	if err := json.Unmarshal(item.VariantJSON, &variant); err != nil {
		t.Fatalf("VariantJSON unmarshal error = %v, raw=%s", err, string(item.VariantJSON))
	}
	if variant["i_id"] != "常规kt板" || variant["product_i_id"] != "常规kt板" {
		t.Fatalf("variant = %+v, want i_id/product_i_id from task product_i_id", variant)
	}
}
