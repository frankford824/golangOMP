package service

import (
	"strings"

	"workflow/domain"
)

func normalizeTaskDepartmentCode(department string) string {
	switch strings.TrimSpace(department) {
	case "":
		return ""
	case string(domain.DepartmentDesign):
		return string(domain.DepartmentDesignRD)
	case string(domain.DepartmentProcurement),
		string(domain.DepartmentWarehouse),
		string(domain.DepartmentBakeryWH):
		return string(domain.DepartmentCloudWarehouse)
	default:
		return strings.TrimSpace(department)
	}
}

func normalizeTaskDepartmentCodes(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := normalizeTaskDepartmentCode(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func taskSourceDepartment(task *domain.Task) string {
	if task == nil {
		return ""
	}
	switch {
	case task.TaskType == domain.TaskTypePurchaseTask:
		return string(domain.DepartmentCloudWarehouse)
	case task.WorkflowLane() == domain.WorkflowLaneCustomization:
		return string(domain.DepartmentCustomizationArt)
	default:
		return string(domain.DepartmentDesignRD)
	}
}
