package service

import (
	"context"
	"strings"

	"workflow/domain"
)

func validateTaskAssetType(assetType domain.TaskAssetType) *domain.AppError {
	switch domain.NormalizeTaskAssetType(assetType) {
	case domain.TaskAssetTypeReference,
		domain.TaskAssetTypeSource,
		domain.TaskAssetTypeDelivery,
		domain.TaskAssetTypePreview,
		domain.TaskAssetTypeERPProduct:
		return nil
	default:
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset_type", nil)
	}
}

func (s *taskAssetCenterService) requireScopedBatchAsset(ctx context.Context, taskID int64, assetType domain.TaskAssetType, targetSKUCode string) *domain.AppError {
	if assetType.IsReference() {
		return nil
	}
	items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, taskID)
	if err != nil {
		return infraError("list task sku items for batch asset scope", err)
	}
	if countScopedSKUItems(items) <= 1 {
		return nil
	}
	if strings.TrimSpace(targetSKUCode) == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "target_sku_code is required for batch non-reference asset uploads", map[string]interface{}{
			"task_id":     taskID,
			"asset_type":  string(assetType),
			"batch_items": countScopedSKUItems(items),
		})
	}
	return nil
}

func countScopedSKUItems(items []*domain.TaskSKUItem) int {
	count := 0
	for _, item := range items {
		if item != nil && strings.TrimSpace(item.SKUCode) != "" {
			count++
		}
	}
	return count
}
