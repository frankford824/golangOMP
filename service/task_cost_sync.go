package service

import (
	"context"
	"workflow/domain"
)

func (s *taskService) GetSKUCostSync(ctx context.Context, taskID, itemID int64, refresh bool) (*domain.TaskCostSyncView, *domain.AppError) {
	// Use the same task scope enforcement as the detail page before exposing prices.
	task, appErr := s.GetByID(ctx, taskID)
	if appErr != nil {
		return nil, appErr
	}
	for _, item := range task.SKUItems {
		if item != nil && item.ID == itemID {
			if s.costSync == nil {
				return &domain.TaskCostSyncView{Message: "成本同步服务未启用"}, nil
			}
			return s.costSync.TaskView(ctx, item.SKUCode, refresh)
		}
	}
	return nil, domain.ErrNotFound
}
