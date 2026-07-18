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
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
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
