package service

import (
	"testing"
	"time"

	"workflow/domain"
)

func TestBuildTaskWorkflowSnapshotFromListItemCustomizationBypassesNormalDesignAndAuditQueues(t *testing.T) {
	item := &domain.TaskListItem{
		ID:                    501,
		TaskNo:                "T-CUST-1",
		TaskType:              domain.TaskTypeOriginalProductDevelopment,
		SourceMode:            domain.TaskSourceModeExistingProduct,
		TaskStatus:            domain.TaskStatusPendingCustomizationReview,
		NeedOutsource:         true,
		CustomizationRequired: true,
		UpdatedAt:             time.Now().UTC(),
	}

	workflow := buildTaskWorkflowSnapshotFromListItem(item)

	if workflow.SubStatus.Design.Code != domain.TaskSubStatusNotRequired {
		t.Fatalf("design sub status = %s, want not_required for customization lane", workflow.SubStatus.Design.Code)
	}
	if workflow.SubStatus.Audit.Code != domain.TaskSubStatusNotTriggered {
		t.Fatalf("audit sub status = %s, want not_triggered", workflow.SubStatus.Audit.Code)
	}
	if workflow.SubStatus.Outsource.Code != domain.TaskSubStatusPendingReview {
		t.Fatalf("outsource/customization sub status = %s, want pending_review", workflow.SubStatus.Outsource.Code)
	}
}
