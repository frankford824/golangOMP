package service

import (
	"context"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

func validateRetouchRequirements(p CreateTaskParams) *domain.AppError {
	if len(p.RetouchRequirements) == 0 {
		return nil
	}
	if p.TaskType != domain.TaskTypeRetouchTask {
		return taskCreateValidationError(
			"retouch_requirements is only supported for retouch_task",
			p,
			taskCreateViolation("retouch_requirements", "field_not_allowed_for_task_type", "retouch_requirements is not allowed for task_type="+string(p.TaskType)),
		)
	}
	for i, item := range p.RetouchRequirements {
		if strings.TrimSpace(item.Description) == "" {
			return taskCreateValidationError(
				"retouch_requirements description is required",
				p,
				taskCreateViolation(
					fmt.Sprintf("retouch_requirements[%d].description", i),
					"missing_retouch_requirement_description",
					"retouch_requirements[].description is required",
				),
			)
		}
	}
	return nil
}

func retouchRequirementsForbiddenForTaskType(p CreateTaskParams) bool {
	return len(p.RetouchRequirements) > 0 && p.TaskType != domain.TaskTypeRetouchTask
}

func normalizeRetouchRequirementItems(items []domain.CreateRetouchRequirementItem) []domain.CreateRetouchRequirementItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]domain.CreateRetouchRequirementItem, 0, len(items))
	for i, item := range items {
		description := strings.TrimSpace(item.Description)
		if description == "" {
			continue
		}
		sortOrder := item.SortOrder
		if sortOrder <= 0 {
			sortOrder = i + 1
		}
		out = append(out, domain.CreateRetouchRequirementItem{
			Description: description,
			SKUCode:     strings.TrimSpace(item.SKUCode),
			Spec:        strings.TrimSpace(item.Spec),
			Remark:      strings.TrimSpace(item.Remark),
			SortOrder:   sortOrder,
		})
	}
	return out
}

func (s *taskService) insertTaskRetouchRequirements(ctx context.Context, tx repo.Tx, taskID int64, p CreateTaskParams) error {
	if s.retouchRequirementRepo == nil || taskID <= 0 || p.TaskType != domain.TaskTypeRetouchTask {
		return nil
	}
	items := normalizeRetouchRequirementItems(p.RetouchRequirements)
	if len(items) == 0 {
		return nil
	}
	if err := s.retouchRequirementRepo.CreateBatch(ctx, tx, taskID, p.CreatorID, items); err != nil {
		return fmt.Errorf("create task retouch requirements: %w", err)
	}
	return nil
}

func (s *taskService) listTaskRetouchRequirements(ctx context.Context, task *domain.Task) []domain.TaskRetouchRequirement {
	if s == nil || s.retouchRequirementRepo == nil || task == nil || task.TaskType != domain.TaskTypeRetouchTask {
		return []domain.TaskRetouchRequirement{}
	}
	rows, err := s.retouchRequirementRepo.ListByTaskID(ctx, task.ID)
	if err != nil {
		return []domain.TaskRetouchRequirement{}
	}
	if len(rows) == 0 {
		return []domain.TaskRetouchRequirement{}
	}
	out := make([]domain.TaskRetouchRequirement, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		out = append(out, *row)
	}
	return out
}
