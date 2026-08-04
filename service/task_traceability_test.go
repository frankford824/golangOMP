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
	if payload["business_lane"] != string(domain.TaskBusinessLaneCustomization) {
		t.Fatalf("business_lane = %v, want %q", payload["business_lane"], domain.TaskBusinessLaneCustomization)
	}
	if _, ok := payload["workflow_lane"]; ok {
		t.Fatal("retired workflow_lane must not be emitted into new task events")
	}
	if payload["source_department"] != string(domain.DepartmentCustomizationArt) {
		t.Fatalf("source_department = %v, want %q", payload["source_department"], domain.DepartmentCustomizationArt)
	}
}
