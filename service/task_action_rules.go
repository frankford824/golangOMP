package service

import "workflow/domain"

// TaskAction is the small internal vocabulary used by services that still
// perform task-level authorization. Public workflow actions are returned by
// the v8 allowed_actions contract and are not inferred from legacy states or
// roles here.
type TaskAction string

const (
	TaskActionCreate             TaskAction = "create"
	TaskActionReadDetail         TaskAction = "read_detail"
	TaskActionUpdateBusinessInfo TaskAction = "update_business_info"
	TaskActionAssign             TaskAction = "assign"
	TaskActionReassign           TaskAction = "reassign"
	TaskActionAuditHandover      TaskAction = "audit_handover"
)

func taskActionPermission(action TaskAction) domain.PermissionCode {
	switch action {
	case TaskActionCreate:
		return domain.PermissionTaskCreate
	case TaskActionReadDetail:
		return domain.PermissionTaskView
	case TaskActionUpdateBusinessInfo:
		return domain.PermissionCatalogManage
	case TaskActionAssign:
		return domain.PermissionTaskAssign
	case TaskActionReassign:
		return domain.PermissionTaskReassign
	case TaskActionAuditHandover:
		return domain.PermissionTaskAuditHandover
	default:
		return ""
	}
}

func taskActionStatusAllowed(action TaskAction, task *domain.Task) bool {
	if task == nil {
		return action == TaskActionCreate
	}
	switch action {
	case TaskActionAssign:
		return task.TaskStatus == domain.TaskStatusPendingAssign
	case TaskActionReassign:
		return task.TaskStatus == domain.TaskStatusInProgress
	default:
		return true
	}
}
