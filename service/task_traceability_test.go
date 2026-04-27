package service

import (
	"testing"

	"workflow/domain"
)

func TestTaskSourceDepartmentUsesCanonicalUpstreamDepartment(t *testing.T) {
	tests := []struct {
		name string
		task *domain.Task
		want string
	}{
		{
			name: "normal design lane maps to design rd",
			task: &domain.Task{
				TaskType:              domain.TaskTypeOriginalProductDevelopment,
				CustomizationRequired: false,
			},
			want: string(domain.DepartmentDesignRD),
		},
		{
			name: "customization lane maps to customization art",
			task: &domain.Task{
				TaskType:              domain.TaskTypeOriginalProductDevelopment,
				CustomizationRequired: true,
			},
			want: string(domain.DepartmentCustomizationArt),
		},
		{
			name: "purchase task maps to cloud warehouse",
			task: &domain.Task{
				TaskType: domain.TaskTypePurchaseTask,
			},
			want: string(domain.DepartmentCloudWarehouse),
		},
	}

	for _, tc := range tests {
		if got := taskSourceDepartment(tc.task); got != tc.want {
			t.Fatalf("%s: taskSourceDepartment() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestTaskEventBasePayloadIncludesLaneAndSourceDepartment(t *testing.T) {
	payload := taskEventBasePayload(&domain.Task{
		TaskNo:                "T-TRACE",
		TaskType:              domain.TaskTypeOriginalProductDevelopment,
		SourceMode:            domain.TaskSourceModeExistingProduct,
		SKUCode:               "SKU-TRACE",
		ProductNameSnapshot:   "Trace",
		CustomizationRequired: true,
	})
	if payload["workflow_lane"] != string(domain.WorkflowLaneCustomization) {
		t.Fatalf("workflow_lane = %v, want %q", payload["workflow_lane"], domain.WorkflowLaneCustomization)
	}
	if payload["source_department"] != string(domain.DepartmentCustomizationArt) {
		t.Fatalf("source_department = %v, want %q", payload["source_department"], domain.DepartmentCustomizationArt)
	}
}
