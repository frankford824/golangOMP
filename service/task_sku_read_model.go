package service

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

func loadTaskSKUItems(ctx context.Context, taskRepo repo.TaskRepo, task *domain.Task, detail *domain.TaskDetail) ([]*domain.TaskSKUItem, *domain.AppError) {
	if task == nil || taskRepo == nil {
		return []*domain.TaskSKUItem{}, nil
	}
	items, err := taskRepo.ListSKUItemsByTaskID(ctx, task.ID)
	if err != nil {
		return nil, infraError("list task sku items", err)
	}
	if len(items) == 0 {
		items = synthesizeTaskSKUItems(task, detail)
	}
	applyTaskSKUItemChangeRequestAlias(task, detail, items)
	normalizeTaskBatchProjection(task, items)
	return items, nil
}

func synthesizeTaskSKUItems(task *domain.Task, detail *domain.TaskDetail) []*domain.TaskSKUItem {
	if task == nil || task.SKUCode == "" {
		return []*domain.TaskSKUItem{}
	}
	item := &domain.TaskSKUItem{
		TaskID:              task.ID,
		SequenceNo:          1,
		SKUCode:             task.SKUCode,
		SKUStatus:           taskSKUStatusFromDetail(detail),
		ProductID:           cloneInt64Ptr(task.ProductID),
		ProductNameSnapshot: task.ProductNameSnapshot,
	}
	if detail != nil {
		item.ProductShortName = detail.ProductShortName
		item.CategoryCode = detail.CategoryCode
		item.MaterialMode = detail.MaterialMode
		item.CostPriceMode = detail.CostPriceMode
		item.Quantity = cloneInt64Ptr(detail.Quantity)
		item.BaseSalePrice = cloneFloat64Ptr(detail.BaseSalePrice)
		if task.TaskType != domain.TaskTypeOriginalProductDevelopment {
			item.DesignRequirement = detail.DesignRequirement
		}
		item.ReferenceFileRefs = domain.ParseReferenceFileRefsJSON(detail.ReferenceFileRefsJSON)
		if len(item.ReferenceFileRefs) == 0 {
			item.ReferenceFileRefs = domain.ParseReferenceFileRefsJSON(detail.ReferenceImagesJSON)
		}
	}
	if item.ReferenceFileRefs == nil {
		item.ReferenceFileRefs = []domain.ReferenceFileRef{}
	}
	return []*domain.TaskSKUItem{item}
}

func enrichTaskSKUItemReferenceFileRefs(items []*domain.TaskSKUItem, enricher *ReferenceFileRefsEnricher) {
	if len(items) == 0 || enricher == nil {
		return
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		item.ReferenceFileRefs = enricher.EnrichAll(item.ReferenceFileRefs)
		if item.ReferenceFileRefs == nil {
			item.ReferenceFileRefs = []domain.ReferenceFileRef{}
		}
	}
}

func applyTaskSKUItemChangeRequestAlias(task *domain.Task, detail *domain.TaskDetail, items []*domain.TaskSKUItem) {
	if detail == nil || task == nil || task.TaskType != domain.TaskTypeOriginalProductDevelopment {
		return
	}
	changeRequest := detail.ChangeRequest
	for _, item := range items {
		if item == nil {
			continue
		}
		item.ChangeRequest = changeRequest
		if item.DesignRequirement == "" {
			item.DesignRequirement = changeRequest
		}
	}
}

func normalizeTaskBatchProjection(task *domain.Task, items []*domain.TaskSKUItem) {
	if task == nil {
		return
	}
	if !task.BatchMode.Valid() {
		task.BatchMode = domain.TaskBatchModeSingle
	}
	if task.PrimarySKUCode == "" {
		task.PrimarySKUCode = task.SKUCode
	}
	if task.BatchItemCount == 0 && len(items) > 0 {
		task.BatchItemCount = len(items)
	}
	if !task.SKUGenerationStatus.Valid() {
		if task.TaskType == domain.TaskTypeNewProductDevelopment || task.TaskType == domain.TaskTypePurchaseTask {
			task.SKUGenerationStatus = domain.TaskSKUGenerationStatusCompleted
		} else {
			task.SKUGenerationStatus = domain.TaskSKUGenerationStatusNotApplicable
		}
	}
}

func taskSKUStatusFromDetail(detail *domain.TaskDetail) domain.TaskSKUStatus {
	if detail == nil {
		return domain.TaskSKUStatusGenerated
	}
	switch detail.FilingStatus {
	case domain.FilingStatusFiled:
		return domain.TaskSKUStatusFiled
	case domain.FilingStatusFilingFailed:
		return domain.TaskSKUStatusFilingFailed
	default:
		return domain.TaskSKUStatusGenerated
	}
}
