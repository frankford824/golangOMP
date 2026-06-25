package service

import (
	"context"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

func validateRetouchRequirementAssetScope(
	ctx context.Context,
	task *domain.Task,
	retouchRequirementID *int64,
	retouchRepo repo.TaskRetouchRequirementRepo,
) *domain.AppError {
	if retouchRequirementID == nil || *retouchRequirementID <= 0 {
		return nil
	}
	if task == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "task is required for retouch_requirement_id", nil)
	}
	if task.TaskType != domain.TaskTypeRetouchTask {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "retouch_requirement_id is only supported for retouch_task", map[string]interface{}{
			"task_id":                task.ID,
			"task_type":              task.TaskType,
			"retouch_requirement_id": *retouchRequirementID,
		})
	}
	if retouchRepo == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "retouch requirement repository is not configured", nil)
	}
	row, err := retouchRepo.GetByID(ctx, *retouchRequirementID)
	if err != nil {
		return infraError("get retouch requirement for asset scope", err)
	}
	if row == nil || row.TaskID != task.ID {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "retouch_requirement_id must belong to the task", map[string]interface{}{
			"task_id":                task.ID,
			"retouch_requirement_id": *retouchRequirementID,
		})
	}
	return nil
}

func rejectConflictingRetouchAssetScopes(targetSKUCode string, retouchRequirementID *int64) *domain.AppError {
	if strings.TrimSpace(targetSKUCode) != "" && retouchRequirementID != nil && *retouchRequirementID > 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "target_sku_code and retouch_requirement_id cannot both be set", map[string]interface{}{
			"target_sku_code":        strings.TrimSpace(targetSKUCode),
			"retouch_requirement_id": *retouchRequirementID,
		})
	}
	return nil
}

func retouchRequirementIDsEqual(left, right *int64) bool {
	if left == nil || *left <= 0 {
		return right == nil || *right <= 0
	}
	if right == nil || *right <= 0 {
		return false
	}
	return *left == *right
}
