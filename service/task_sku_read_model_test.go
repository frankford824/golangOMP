package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestTaskSKUReadModelLoadTaskSKUItemsOriginalAliasPassRepoPath(t *testing.T) {
	task := &domain.Task{
		ID:                  201,
		TaskType:            domain.TaskTypeOriginalProductDevelopment,
		SKUCode:             "SKU-ORIG-201",
		ProductNameSnapshot: "Original Product",
	}
	detail := &domain.TaskDetail{TaskID: 201, ChangeRequest: "foo", DesignRequirement: ""}
	taskRepo := &prdTaskRepo{
		skuItems: map[int64][]*domain.TaskSKUItem{
			201: {{TaskID: 201, SequenceNo: 1, SKUCode: "SKU-ORIG-201", SKUStatus: domain.TaskSKUStatusGenerated, DesignRequirement: ""}},
		},
	}

	items, appErr := loadTaskSKUItems(context.Background(), taskRepo, task, detail)
	if appErr != nil {
		t.Fatalf("loadTaskSKUItems() unexpected error: %+v", appErr)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if items[0].ChangeRequest != "foo" || items[0].DesignRequirement != "foo" {
		t.Fatalf("item demand fields = change_request:%q design_requirement:%q, want foo/foo", items[0].ChangeRequest, items[0].DesignRequirement)
	}
}

func TestTaskSKUReadModelLoadTaskSKUItemsOriginalAliasPassSynthesizePath(t *testing.T) {
	task := &domain.Task{
		ID:                  202,
		TaskType:            domain.TaskTypeOriginalProductDevelopment,
		SKUCode:             "SKU-ORIG-202",
		ProductNameSnapshot: "Original Product",
	}
	detail := &domain.TaskDetail{TaskID: 202, ChangeRequest: "foo", DesignRequirement: ""}
	taskRepo := &prdTaskRepo{}

	items, appErr := loadTaskSKUItems(context.Background(), taskRepo, task, detail)
	if appErr != nil {
		t.Fatalf("loadTaskSKUItems() unexpected error: %+v", appErr)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if items[0].ChangeRequest != "foo" || items[0].DesignRequirement != "foo" {
		t.Fatalf("item demand fields = change_request:%q design_requirement:%q, want foo/foo", items[0].ChangeRequest, items[0].DesignRequirement)
	}
}

func TestTaskSKUReadModelLoadTaskSKUItemsNewProductNoAlias(t *testing.T) {
	task := &domain.Task{
		ID:                  203,
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SKUCode:             "SKU-NEW-203",
		ProductNameSnapshot: "New Product",
	}
	detail := &domain.TaskDetail{TaskID: 203, DesignRequirement: "bar", ChangeRequest: ""}
	taskRepo := &prdTaskRepo{
		skuItems: map[int64][]*domain.TaskSKUItem{
			203: {{TaskID: 203, SequenceNo: 1, SKUCode: "SKU-NEW-203", SKUStatus: domain.TaskSKUStatusGenerated, DesignRequirement: "bar"}},
		},
	}

	items, appErr := loadTaskSKUItems(context.Background(), taskRepo, task, detail)
	if appErr != nil {
		t.Fatalf("loadTaskSKUItems() unexpected error: %+v", appErr)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if items[0].ChangeRequest != "" || items[0].DesignRequirement != "bar" {
		t.Fatalf("item demand fields = change_request:%q design_requirement:%q, want empty/bar", items[0].ChangeRequest, items[0].DesignRequirement)
	}
}
