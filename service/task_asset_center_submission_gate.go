package service

import (
	"context"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const deliveryUploadPreparedBatchWindow = 2 * time.Hour

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

func (s *taskAssetCenterService) shouldAdvanceTaskToPendingAuditA(ctx context.Context, task *domain.Task, completedScopeSKU string, currentRequest *domain.UploadRequest) (bool, *domain.AppError) {
	if task == nil {
		return false, domain.NewAppError(domain.ErrCodeInvalidRequest, "task is required", nil)
	}
	hasOpenPeerSession, appErr := s.hasOtherPreparedDeliveryUploadSession(ctx, task.ID, currentRequest)
	if appErr != nil {
		return false, appErr
	}
	if hasOpenPeerSession {
		return false, nil
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

func (s *taskAssetCenterService) hasOtherPreparedDeliveryUploadSession(ctx context.Context, taskID int64, currentRequest *domain.UploadRequest) (bool, *domain.AppError) {
	if currentRequest == nil || !uploadRequestAssetTypeIsDelivery(currentRequest) {
		return false, nil
	}
	ownerType := domain.AssetOwnerTypeTask
	assetType := domain.TaskAssetTypeDelivery
	status := domain.UploadRequestStatusRequested
	ownerID := taskID
	for page := 1; ; page++ {
		requests, total, err := s.uploadRequestRepo.List(ctx, repo.UploadRequestListFilter{
			OwnerType:     &ownerType,
			OwnerID:       &ownerID,
			TaskAssetType: &assetType,
			Status:        &status,
			Page:          page,
			PageSize:      100,
		})
		if err != nil {
			return false, infraError("list pending delivery upload sessions", err)
		}
		for _, candidate := range requests {
			if isSamePreparedDeliveryBatchPeer(currentRequest, candidate) {
				return true, nil
			}
		}
		if len(requests) == 0 || int64(page*100) >= total {
			return false, nil
		}
	}
}

func uploadRequestAssetTypeIsDelivery(request *domain.UploadRequest) bool {
	if request == nil || request.TaskAssetType == nil {
		return false
	}
	return domain.NormalizeTaskAssetType(*request.TaskAssetType).IsDelivery()
}

func isSamePreparedDeliveryBatchPeer(current, candidate *domain.UploadRequest) bool {
	if current == nil || candidate == nil {
		return false
	}
	if strings.TrimSpace(candidate.RequestID) == "" || strings.TrimSpace(candidate.RequestID) == strings.TrimSpace(current.RequestID) {
		return false
	}
	if !uploadRequestAssetTypeIsDelivery(candidate) {
		return false
	}
	if candidate.Status != domain.UploadRequestStatusRequested {
		return false
	}
	if candidate.SessionStatus == domain.DesignAssetSessionStatusCancelled ||
		candidate.SessionStatus == domain.DesignAssetSessionStatusExpired ||
		candidate.SessionStatus == domain.DesignAssetSessionStatusCompleted {
		return false
	}
	if current.CreatedBy > 0 && candidate.CreatedBy > 0 && current.CreatedBy != candidate.CreatedBy {
		return false
	}
	if current.CreatedAt.IsZero() || candidate.CreatedAt.IsZero() {
		return true
	}
	diff := current.CreatedAt.Sub(candidate.CreatedAt)
	if diff < 0 {
		diff = -diff
	}
	return diff <= deliveryUploadPreparedBatchWindow
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
