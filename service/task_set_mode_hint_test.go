package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"workflow/domain"
)

func TestBuildSingleTaskSKUItemsPreservesOperationsSetModeHint(t *testing.T) {
	task := &domain.Task{ID: 42, TaskType: domain.TaskTypeNewProductDevelopment, SKUCode: "SKU-42"}
	detail := &domain.TaskDetail{TaskID: 42, DesignRequirement: "白底主图", SetModeHint: true}

	items := buildSingleTaskSKUItems(task, detail)
	if len(items) != 1 || items[0] == nil || items[0].Item == nil {
		t.Fatalf("buildSingleTaskSKUItems() = %+v, want one SKU item", items)
	}
	if !items[0].Item.SetModeHint {
		t.Fatal("SetModeHint = false, want operations suggestion preserved")
	}
}

func TestTaskServiceCreatePersistsSingleTaskDimensionsAndHint(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
	)
	width, height, area := 1.2, 0.8, 0.96
	deadline := time.Now().Add(24 * time.Hour)
	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		BusinessLane:        domain.TaskBusinessLaneNormal,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          &deadline,
		CategoryCode:        "KT_STANDARD",
		ProductNameSnapshot: "尺寸款",
		DesignRequirement:   "按尺寸排版",
		SetModeHint:         true,
		Width:               &width,
		Height:              &height,
		Area:                &area,
	})
	if appErr != nil {
		t.Fatalf("Create() error = %+v", appErr)
	}
	detail := taskRepo.details[task.ID]
	if detail == nil || detail.Width == nil || *detail.Width != width || detail.Height == nil || *detail.Height != height || detail.Area == nil || *detail.Area != area {
		t.Fatalf("persisted detail dimensions = %+v", detail)
	}
	items := taskRepo.skuItems[task.ID]
	if len(items) != 1 || items[0] == nil || !items[0].SetModeHint {
		t.Fatalf("persisted SKU item = %+v", items)
	}
	var variant map[string]interface{}
	if err := json.Unmarshal(items[0].VariantJSON, &variant); err != nil || variant["width"] != width || variant["height"] != height || variant["area"] != area {
		t.Fatalf("persisted SKU variant = %s err=%v", string(items[0].VariantJSON), err)
	}
}

func TestCreateSingleTaskUsesExplicitCostShape(t *testing.T) {
	for _, tc := range []struct {
		shape string
		cost  float64
	}{{"normal", 3.5}, {"irregular", 4.9}} {
		t.Run(tc.shape, func(t *testing.T) {
			taskRepo := &prdTaskRepo{}
			rules := newCostRuleRepoStub()
			rules.rules = []*domain.CostRule{{
				RuleID: 70, RuleVersion: 2, CategoryCode: "KT_STANDARD",
				RuleType: domain.CostRuleTypeModel, IsActive: true, Source: "admin_manual",
				FormulaExpression: `{"version":1,"material":"PVC","basis":"area","unit_price":10,"multiplier":1,"variant_prices":[{"thickness_mm":0,"normal_price":10,"irregular_price":14}]}`,
			}}
			svc := NewTaskServiceWithCatalog(
				taskRepo, &prdTaskAssetRepo{}, &prdTaskEventRepo{}, nil, nil, rules,
				prdCodeRuleService{}, productCodeTestTxRunner{},
				WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
				WithTaskCostRuleBindingRepo(repairBindingStub{enabled: true}),
				WithERPBridgeSelectionBinding(&erpBridgeSelectionBinderStub{
					iidOptions: []*domain.ERPIIDOption{{IID: "超透PVC"}},
				}),
			)
			area := 0.35
			deadline := time.Now().Add(24 * time.Hour)
			task, err := svc.Create(context.Background(), CreateTaskParams{
				TaskType:     domain.TaskTypeNewProductDevelopment,
				SourceMode:   domain.TaskSourceModeNewProduct,
				BusinessLane: domain.TaskBusinessLaneNormal,
				CreatorID:    9, OwnerTeam: domain.AllValidTeams()[0], DeadlineAt: &deadline,
				ProductIID: "超透PVC", CategoryCode: "KT_STANDARD", ProductNameSnapshot: "测试款", DesignRequirement: "画图",
				Area:      &area,
				CostInput: &domain.CostInput{AreaMode: "total", AreaM2: area, Pieces: 1, Shape: tc.shape},
			})
			if err != nil {
				t.Fatalf("Create failed: %v", err)
			}
			item := taskRepo.skuItems[task.ID][0]
			if input := skuCostInput(item); input == nil || input.Shape != tc.shape {
				t.Fatalf("explicit shape missing from persisted SKU: %s", item.VariantJSON)
			}
			if item.CostPrice == nil || *item.CostPrice != tc.cost || item.RequiresManualReview {
				t.Fatalf("unexpected cost for %s: %s", tc.shape, item.VariantJSON)
			}
		})
	}
}
