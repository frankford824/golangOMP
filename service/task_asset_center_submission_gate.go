package service

import (
	"context"
	"strings"

	"workflow/domain"
)

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

func (s *taskAssetCenterService) shouldAdvanceTaskToPendingAuditA(ctx context.Context, task *domain.Task, completedScopeSKU string) (bool, *domain.AppError) {
	if task == nil {
		return false, domain.NewAppError(domain.ErrCodeInvalidRequest, "task is required", nil)
	}
	items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, task.ID)
	if err != nil {
		return false, infraError("list task sku items for delivery gate", err)
	}
	if countScopedSKUItems(items) <= 1 {
		return true, nil
	}

	required := map[string]struct{}{}
	for _, item := range items {
		if item == nil {
			continue
		}
		skuCode := strings.TrimSpace(item.SKUCode)
		if skuCode != "" {
			required[skuCode] = struct{}{}
		}
	}
	if len(required) <= 1 {
		return true, nil
	}

	completed := map[string]struct{}{}
	assets, err := s.designAssetRepo.ListByTaskID(ctx, task.ID)
	if err != nil {
		return false, infraError("list design assets for delivery gate", err)
	}
	for _, asset := range assets {
		if asset == nil || !domain.NormalizeTaskAssetType(asset.AssetType).IsDelivery() || asset.CurrentVersionID == nil {
			continue
		}
		scope := strings.TrimSpace(asset.ScopeSKUCode)
		if scope != "" {
			completed[scope] = struct{}{}
		}
	}
	if scope := strings.TrimSpace(completedScopeSKU); scope != "" {
		completed[scope] = struct{}{}
	}
	for skuCode := range required {
		if _, ok := completed[skuCode]; !ok {
			return false, nil
		}
	}
	return true, nil
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
