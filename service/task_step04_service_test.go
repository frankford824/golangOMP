package service

import (
	"context"
	"encoding/json"
	"testing"

	"workflow/domain"
)

func TestTaskAssignmentServiceAssign(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID: 1, TaskNo: "T-0001", TaskStatus: domain.TaskStatusPendingAssign,
		TaskType: domain.TaskTypeNewProductDevelopment,
	})
	eventRepo := &step04TaskEventRepo{}
	notifications := &step04AssignmentNotificationService{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{}, WithTaskAssignmentNotificationService(notifications))

	task, appErr := svc.Assign(context.Background(), AssignTaskParams{
		TaskID: 1, DesignerID: authzInt64Ptr(101), AssignedBy: 7, Remark: "first assignment",
	})
	if appErr != nil {
		t.Fatalf("Assign() unexpected error: %+v", appErr)
	}
	if task.TaskStatus != domain.TaskStatusInProgress || task.DesignerID == nil || *task.DesignerID != 101 || task.CurrentHandlerID == nil || *task.CurrentHandlerID != 101 {
		t.Fatalf("assigned task = %+v, want designer and handler 101 in progress", task)
	}
	if len(eventRepo.events) != 1 || eventRepo.events[0].EventType != domain.TaskEventAssigned {
		t.Fatalf("events = %+v, want one task.assigned", eventRepo.events)
	}
	if len(notifications.created) != 1 || notifications.created[0].userID != 101 || notifications.created[0].ntype != domain.NotificationTypeTaskAssignedToMe {
		t.Fatalf("notifications = %+v, want one task_assigned_to_me", notifications.created)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(notifications.created[0].payload, &payload); err != nil {
		t.Fatalf("notification payload: %v", err)
	}
	if payload["task_no"] != "T-0001" || payload["action"] != "assign" {
		t.Fatalf("notification payload = %+v", payload)
	}
}

func TestTaskAssignmentServiceAssignSyncsDesignModule(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID: 1012, TaskNo: "T-1012", TaskStatus: domain.TaskStatusPendingAssign,
		TaskType: domain.TaskTypeNewProductDevelopment,
	})
	eventRepo := &step04TaskEventRepo{}
	moduleRepo := newStep04TaskModuleRepo(&domain.TaskModule{
		ID: 55, TaskID: 1012, ModuleKey: domain.ModuleKeyDesign,
		State: domain.ModuleStatePendingClaim, PoolTeamCode: strPtr(domain.TeamDesignStandard),
	})
	moduleEventRepo := &step04TaskModuleEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{}, WithTaskAssignmentModuleSync(moduleRepo, moduleEventRepo))

	task, appErr := svc.Assign(context.Background(), AssignTaskParams{TaskID: 1012, DesignerID: authzInt64Ptr(203), AssignedBy: 1})
	if appErr != nil {
		t.Fatalf("Assign() unexpected error: %+v", appErr)
	}
	module := moduleRepo.modules[domain.ModuleKeyDesign]
	if task.TaskStatus != domain.TaskStatusInProgress || module.State != domain.ModuleStateInProgress || module.ClaimedBy == nil || *module.ClaimedBy != 203 {
		t.Fatalf("task/module = %+v/%+v", task, module)
	}
	if len(moduleEventRepo.events) != 1 || moduleEventRepo.events[0].EventType != domain.ModuleEventClaimed {
		t.Fatalf("module events = %+v", moduleEventRepo.events)
	}
}

func TestTaskAssignmentServiceAssignSyncsRetouchModule(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID: 1014, TaskNo: "T-1014", TaskStatus: domain.TaskStatusPendingAssign,
		TaskType: domain.TaskTypeRetouchTask,
	})
	eventRepo := &step04TaskEventRepo{}
	moduleRepo := newStep04TaskModuleRepo(&domain.TaskModule{
		ID: 57, TaskID: 1014, ModuleKey: domain.ModuleKeyRetouch,
		State: domain.ModuleStatePendingClaim, PoolTeamCode: strPtr(domain.TeamDesignRetouch),
	})
	moduleEventRepo := &step04TaskModuleEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{}, WithTaskAssignmentModuleSync(moduleRepo, moduleEventRepo))

	task, appErr := svc.Assign(context.Background(), AssignTaskParams{TaskID: 1014, DesignerID: authzInt64Ptr(204), AssignedBy: 1})
	if appErr != nil {
		t.Fatalf("Assign() unexpected error: %+v", appErr)
	}
	module := moduleRepo.modules[domain.ModuleKeyRetouch]
	if task.TaskStatus != domain.TaskStatusInProgress || module.State != domain.ModuleStateInProgress || module.ClaimedBy == nil || *module.ClaimedBy != 204 {
		t.Fatalf("task/module = %+v/%+v", task, module)
	}
	if _, exists := moduleRepo.modules[domain.ModuleKeyDesign]; exists {
		t.Fatal("retouch assignment touched design module")
	}
}

func TestTaskAssignmentServiceReassignSyncsDesignModule(t *testing.T) {
	previous := int64(101)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID: 1013, TaskNo: "T-1013", TaskStatus: domain.TaskStatusInProgress,
		TaskType: domain.TaskTypeOriginalProductDevelopment, DesignerID: &previous, CurrentHandlerID: &previous,
	})
	eventRepo := &step04TaskEventRepo{}
	moduleRepo := newStep04TaskModuleRepo(&domain.TaskModule{
		ID: 56, TaskID: 1013, ModuleKey: domain.ModuleKeyDesign,
		State: domain.ModuleStateInProgress, ClaimedBy: &previous,
	})
	moduleEventRepo := &step04TaskModuleEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{}, WithTaskAssignmentModuleSync(moduleRepo, moduleEventRepo))

	task, appErr := svc.Assign(context.Background(), AssignTaskParams{TaskID: 1013, DesignerID: authzInt64Ptr(202), AssignedBy: 8})
	if appErr != nil {
		t.Fatalf("Assign() unexpected error: %+v", appErr)
	}
	if task.DesignerID == nil || *task.DesignerID != 202 || task.CurrentHandlerID == nil || *task.CurrentHandlerID != 202 {
		t.Fatalf("reassigned task = %+v", task)
	}
	if len(eventRepo.events) != 1 || eventRepo.events[0].EventType != domain.TaskEventReassigned {
		t.Fatalf("task events = %+v", eventRepo.events)
	}
	if len(moduleEventRepo.events) != 1 || moduleEventRepo.events[0].EventType != domain.ModuleEventReassigned {
		t.Fatalf("module events = %+v", moduleEventRepo.events)
	}
}
