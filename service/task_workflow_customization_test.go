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

func TestBuildTaskWorkflowSnapshotRetouchUsesDesignButSkipsAudit(t *testing.T) {
	item := &domain.TaskListItem{
		ID:         502,
		TaskNo:     "T-RET-1",
		TaskType:   domain.TaskTypeRetouchTask,
		SourceMode: domain.TaskSourceModeNewProduct,
		TaskStatus: domain.TaskStatusInProgress,
		UpdatedAt:  time.Now().UTC(),
	}

	workflow := buildTaskWorkflowSnapshotFromListItem(item)

	if workflow.SubStatus.Design.Code != domain.TaskSubStatusDesigning {
		t.Fatalf("design sub status = %s, want designing for retouch", workflow.SubStatus.Design.Code)
	}
	if workflow.SubStatus.Audit.Code != domain.TaskSubStatusNotTriggered {
		t.Fatalf("audit sub status = %s, want not_triggered for retouch", workflow.SubStatus.Audit.Code)
	}
	if hasReason(workflow.WarehouseBlockingReasons, domain.WorkflowReasonAuditNotApproved) {
		t.Fatalf("retouch warehouse blocking reasons include audit_not_approved: %+v", workflow.WarehouseBlockingReasons)
	}
}

func hasReason(reasons []domain.WorkflowReason, code domain.WorkflowReasonCode) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}
