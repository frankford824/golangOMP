package service

import (
	"testing"

	"workflow/domain"
)

func TestBuildTaskERPBridgeProductUpsertPayloadUsesProductNameAsShortName(t *testing.T) {
	name := "菲瑶/常规kt板/端午父亲立牌/粽情父爱/47*100cm"
	payload, appErr := buildTaskERPBridgeProductUpsertPayload(
		&domain.Task{
			ID:                  1120,
			TaskNo:              "RW-20260605-A-001117",
			SKUCode:             "CGK000184",
			ProductNameSnapshot: name,
			TaskType:            domain.TaskTypeNewProductDevelopment,
			SourceMode:          domain.TaskSourceModeNewProduct,
		},
		&domain.TaskDetail{
			Category:     "常规kt板",
			CategoryName: "常规kt板",
		},
		318,
		"",
		"create",
	)
	if appErr != nil {
		t.Fatalf("buildTaskERPBridgeProductUpsertPayload() error = %+v", appErr)
	}
	if payload.Name != name || payload.ProductName != name {
		t.Fatalf("product name was changed: name=%q product_name=%q", payload.Name, payload.ProductName)
	}
	if payload.ShortName == "" {
		t.Fatal("ShortName is empty")
	}
	if payload.ShortName != name {
		t.Fatalf("ShortName = %q, want full product name %q", payload.ShortName, name)
	}
	if payload.ProductShortName != payload.ShortName {
		t.Fatalf("ProductShortName = %q, want ShortName %q", payload.ProductShortName, payload.ShortName)
	}
	if validateERPProductUpsertNameLength(payload) != nil {
		t.Fatalf("payload should pass ERP name validation: %+v", payload)
	}
}

func TestBuildTaskERPBridgeProductUpsertPayloadIgnoresExplicitShortNameForFiling(t *testing.T) {
	name := "菲瑶/常规KT板/毕业手举牌/最好的我们/6个装"
	payload, appErr := buildTaskERPBridgeProductUpsertPayload(
		&domain.Task{
			ID:                  1124,
			TaskNo:              "RW-20260605-A-001121",
			SKUCode:             "CGK000186",
			ProductNameSnapshot: name,
			TaskType:            domain.TaskTypeNewProductDevelopment,
			SourceMode:          domain.TaskSourceModeNewProduct,
		},
		&domain.TaskDetail{
			ProductShortName: "旧简称不再用于ERP同步",
			Category:         "常规kt板",
			CategoryName:     "常规kt板",
		},
		318,
		"",
		"manual_retry",
	)
	if appErr != nil {
		t.Fatalf("buildTaskERPBridgeProductUpsertPayload() error = %+v", appErr)
	}
	if payload.ShortName != name {
		t.Fatalf("ShortName = %q, want full product name %q", payload.ShortName, name)
	}
	if validateERPProductUpsertNameLength(payload) != nil {
		t.Fatalf("payload should pass ERP name validation: %+v", payload)
	}
}

func TestBuildBatchSKUItemERPBridgeProductUpsertPayloadUsesProductNameAsShortName(t *testing.T) {
	name := "谷常规KT板/开槽/端午射五毒投壶筒/壁虎款/20*20*40cm"
	payload, appErr := buildBatchSKUItemERPBridgeProductUpsertPayload(
		&domain.Task{
			ID:                  1001,
			TaskNo:              "RW-BATCH",
			SKUCode:             "CGG000000",
			ProductNameSnapshot: "批量任务",
			TaskType:            domain.TaskTypeNewProductDevelopment,
			SourceMode:          domain.TaskSourceModeNewProduct,
		},
		&domain.TaskDetail{CategoryName: "常规kt板"},
		&domain.TaskSKUItem{
			ID:                  1,
			TaskID:              1001,
			SequenceNo:          1,
			SKUCode:             "CGG000001",
			ProductNameSnapshot: name,
			ProductIID:          "常规kt板",
		},
		318,
		"",
		"create",
	)
	if appErr != nil {
		t.Fatalf("buildBatchSKUItemERPBridgeProductUpsertPayload() error = %+v", appErr)
	}
	if payload.Name != name {
		t.Fatalf("Name = %q, want %q", payload.Name, name)
	}
	if payload.ShortName != name {
		t.Fatalf("ShortName = %q, want full product name %q", payload.ShortName, name)
	}
	if validateERPProductUpsertNameLength(payload) != nil {
		t.Fatalf("payload should pass ERP name validation: %+v", payload)
	}
}
