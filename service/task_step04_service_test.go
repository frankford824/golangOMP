package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestTaskAssignmentServiceAssign(t *testing.T) {
	ctx := context.Background()
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:         1,
		TaskStatus: domain.TaskStatusPendingAssign,
	})
	eventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{})

	task, appErr := svc.Assign(ctx, AssignTaskParams{
		TaskID:     1,
		DesignerID: authzInt64Ptr(101),
		AssignedBy: 7,
		Remark:     "first assignment",
	})
	if appErr != nil {
		t.Fatalf("Assign() unexpected error: %+v", appErr)
	}
	if task.TaskStatus != domain.TaskStatusInProgress {
		t.Fatalf("Assign() task status = %s, want InProgress", task.TaskStatus)
	}
	if task.DesignerID == nil || *task.DesignerID != 101 {
		t.Fatalf("Assign() designer_id = %+v, want 101", task.DesignerID)
	}
	if task.CurrentHandlerID == nil || *task.CurrentHandlerID != 101 {
		t.Fatalf("Assign() current_handler_id = %+v, want 101", task.CurrentHandlerID)
	}
	if len(eventRepo.events) != 1 || eventRepo.events[0].EventType != domain.TaskEventAssigned {
		t.Fatalf("Assign() expected one task.assigned event, got %+v", eventRepo.events)
	}
}

func TestTaskAssignmentServiceSelfClaimPendingAssign(t *testing.T) {
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    101,
		Roles: []domain.Role{domain.RoleDesigner},
		Team:  domain.TeamDesignStandard,
	})
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:         1010,
		TaskStatus: domain.TaskStatusPendingAssign,
	})
	eventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{})

	task, appErr := svc.Assign(ctx, AssignTaskParams{
		TaskID:     1010,
		DesignerID: authzInt64Ptr(101),
		AssignedBy: 101,
		Remark:     "self claim",
	})
	if appErr != nil {
		t.Fatalf("Assign(self claim) unexpected error: %+v", appErr)
	}
	if task.TaskStatus != domain.TaskStatusInProgress {
		t.Fatalf("self claim status = %s, want InProgress", task.TaskStatus)
	}
	if task.DesignerID == nil || *task.DesignerID != 101 || task.CurrentHandlerID == nil || *task.CurrentHandlerID != 101 {
		t.Fatalf("self claim assignment = designer:%+v handler:%+v, want 101", task.DesignerID, task.CurrentHandlerID)
	}
}

func TestTaskAssignmentServiceSelfClaimDeniedAfterReassignToOther(t *testing.T) {
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    101,
		Roles: []domain.Role{domain.RoleDesigner},
		Team:  domain.TeamDesignStandard,
	})
	assignedID := int64(202)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:               1011,
		TaskStatus:       domain.TaskStatusInProgress,
		DesignerID:       &assignedID,
		CurrentHandlerID: &assignedID,
	})
	eventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{})

	_, appErr := svc.Assign(ctx, AssignTaskParams{
		TaskID:     1011,
		DesignerID: authzInt64Ptr(101),
		AssignedBy: 101,
		Remark:     "old assignee tries claim",
	})
	if appErr == nil {
		t.Fatal("Assign(self claim after reassigned) expected error")
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok || details["deny_code"] != domain.DenyTaskAlreadyClaimed {
		t.Fatalf("deny details = %+v, want task_already_claimed", appErr.Details)
	}
}

func TestTaskAssignmentServiceReassign(t *testing.T) {
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    8,
		Roles: []domain.Role{domain.RoleTeamLead},
		Team:  "ops-team-1",
	})
	currentDesignerID := int64(101)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:               11,
		TaskStatus:       domain.TaskStatusInProgress,
		OwnerDepartment:  "ops",
		OwnerOrgTeam:     "ops-team-1",
		DesignerID:       &currentDesignerID,
		CurrentHandlerID: &currentDesignerID,
	})
	eventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{})

	task, appErr := svc.Assign(ctx, AssignTaskParams{
		TaskID:     11,
		DesignerID: authzInt64Ptr(202),
		AssignedBy: 8,
		Remark:     "manager reassign",
	})
	if appErr != nil {
		t.Fatalf("Assign(reassign) unexpected error: %+v", appErr)
	}
	if task.TaskStatus != domain.TaskStatusInProgress {
		t.Fatalf("Assign(reassign) task status = %s, want InProgress", task.TaskStatus)
	}
	if task.DesignerID == nil || *task.DesignerID != 202 {
		t.Fatalf("Assign(reassign) designer_id = %+v, want 202", task.DesignerID)
	}
	if task.CurrentHandlerID == nil || *task.CurrentHandlerID != 202 {
		t.Fatalf("Assign(reassign) current_handler_id = %+v, want 202", task.CurrentHandlerID)
	}
	if len(eventRepo.events) != 1 || eventRepo.events[0].EventType != domain.TaskEventReassigned {
		t.Fatalf("Assign(reassign) expected one task.reassigned event, got %+v", eventRepo.events)
	}
}

func TestTaskAssignmentServiceReassignDeniedForOps(t *testing.T) {
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         9,
		Roles:      []domain.Role{domain.RoleOps},
		Department: "ops",
	})
	currentDesignerID := int64(101)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:               12,
		TaskStatus:       domain.TaskStatusInProgress,
		OwnerDepartment:  "ops",
		OwnerOrgTeam:     "ops-team-1",
		DesignerID:       &currentDesignerID,
		CurrentHandlerID: &currentDesignerID,
	})
	eventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssignmentService(taskRepo, eventRepo, step04TxRunner{})

	_, appErr := svc.Assign(ctx, AssignTaskParams{
		TaskID:     12,
		DesignerID: authzInt64Ptr(202),
		AssignedBy: 9,
		Remark:     "ops cannot reassign",
	})
	if appErr == nil {
		t.Fatal("Assign(reassign) expected permission error")
	}
	if appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Assign(reassign) code = %s, want %s", appErr.Code, domain.ErrCodePermissionDenied)
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("Assign(reassign) details = %#v, want map", appErr.Details)
	}
	if got, _ := details["deny_code"].(string); got != "task_reassign_requires_requester_or_manager" {
		t.Fatalf("Assign(reassign) deny_code = %v, want task_reassign_requires_requester_or_manager", details["deny_code"])
	}
	if len(eventRepo.events) != 0 {
		t.Fatalf("Assign(reassign) events = %+v, want none", eventRepo.events)
	}
}

func TestTaskAssignmentServiceDeptAdminAssignAndReassignRoundV(t *testing.T) {
	userRepo := newIdentityUserRepo()
	userRepo.users[202] = &domain.User{ID: 202, Department: domain.DepartmentOperations}
	userRepo.users[303] = &domain.User{ID: 303, Department: domain.DepartmentDesignRD}

	t.Run("assign pending in managed department", func(t *testing.T) {
		ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
			ID:                 18,
			Roles:              []domain.Role{domain.RoleDeptAdmin},
			ManagedDepartments: []string{string(domain.DepartmentOperations)},
		})
		taskRepo := newStep04TaskRepo(&domain.Task{
			ID:              21,
			TaskStatus:      domain.TaskStatusPendingAssign,
			OwnerDepartment: string(domain.DepartmentOperations),
		})
		svc := NewTaskAssignmentService(taskRepo, &step04TaskEventRepo{}, step04TxRunner{}, WithTaskAssignmentScopeUserRepo(userRepo))

		task, appErr := svc.Assign(ctx, AssignTaskParams{TaskID: 21, DesignerID: authzInt64Ptr(202), AssignedBy: 18})
		if appErr != nil {
			t.Fatalf("Assign() unexpected error: %+v", appErr)
		}
		if task.TaskStatus != domain.TaskStatusInProgress || task.DesignerID == nil || *task.DesignerID != 202 {
			t.Fatalf("Assign() task = %+v, want InProgress assigned to 202", task)
		}
	})

	t.Run("clear in progress returns pending assign", func(t *testing.T) {
		ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
			ID:                 18,
			Roles:              []domain.Role{domain.RoleDeptAdmin},
			ManagedDepartments: []string{string(domain.DepartmentOperations)},
		})
		currentDesignerID := int64(202)
		taskRepo := newStep04TaskRepo(&domain.Task{
			ID:               22,
			TaskStatus:       domain.TaskStatusInProgress,
			OwnerDepartment:  string(domain.DepartmentOperations),
			DesignerID:       &currentDesignerID,
			CurrentHandlerID: &currentDesignerID,
		})
		svc := NewTaskAssignmentService(taskRepo, &step04TaskEventRepo{}, step04TxRunner{}, WithTaskAssignmentScopeUserRepo(userRepo))

		task, appErr := svc.Assign(ctx, AssignTaskParams{TaskID: 22, AssignedBy: 18})
		if appErr != nil {
			t.Fatalf("Assign(clear) unexpected error: %+v", appErr)
		}
		if task.TaskStatus != domain.TaskStatusPendingAssign || task.DesignerID != nil || task.CurrentHandlerID != nil {
			t.Fatalf("Assign(clear) task = %+v, want PendingAssign with nil handlers", task)
		}
	})

	t.Run("rejects target outside managed department", func(t *testing.T) {
		ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
			ID:                 18,
			Roles:              []domain.Role{domain.RoleDeptAdmin},
			ManagedDepartments: []string{string(domain.DepartmentOperations)},
		})
		currentDesignerID := int64(202)
		taskRepo := newStep04TaskRepo(&domain.Task{
			ID:               23,
			TaskStatus:       domain.TaskStatusInProgress,
			OwnerDepartment:  string(domain.DepartmentOperations),
			DesignerID:       &currentDesignerID,
			CurrentHandlerID: &currentDesignerID,
		})
		svc := NewTaskAssignmentService(taskRepo, &step04TaskEventRepo{}, step04TxRunner{}, WithTaskAssignmentScopeUserRepo(userRepo))

		_, appErr := svc.Assign(ctx, AssignTaskParams{TaskID: 23, DesignerID: authzInt64Ptr(303), AssignedBy: 18})
		if appErr == nil {
			t.Fatal("Assign(out-of-dept target) expected permission error")
		}
		details, _ := appErr.Details.(map[string]interface{})
		if got, _ := details["deny_code"].(string); got != "reassign_target_out_of_managed_department" {
			t.Fatalf("deny_code = %v, want reassign_target_out_of_managed_department", details["deny_code"])
		}
	})
}

func TestTaskAssignmentServiceDeptAdminReassignRegressionsRoundV(t *testing.T) {
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:                 18,
		Roles:              []domain.Role{domain.RoleDeptAdmin},
		ManagedDepartments: []string{string(domain.DepartmentOperations)},
	})
	userRepo := newIdentityUserRepo()
	userRepo.users[202] = &domain.User{ID: 202, Department: domain.DepartmentOperations}
	currentDesignerID := int64(101)

	t.Run("purchase task remains blocked", func(t *testing.T) {
		taskRepo := newStep04TaskRepo(&domain.Task{
			ID:               31,
			TaskType:         domain.TaskTypePurchaseTask,
			TaskStatus:       domain.TaskStatusInProgress,
			OwnerDepartment:  string(domain.DepartmentOperations),
			DesignerID:       &currentDesignerID,
			CurrentHandlerID: &currentDesignerID,
		})
		svc := NewTaskAssignmentService(taskRepo, &step04TaskEventRepo{}, step04TxRunner{}, WithTaskAssignmentScopeUserRepo(userRepo))
		_, appErr := svc.Assign(ctx, AssignTaskParams{TaskID: 31, DesignerID: authzInt64Ptr(202), AssignedBy: 18})
		if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
			t.Fatalf("Assign(purchase) appErr = %+v, want invalid state", appErr)
		}
	})

	t.Run("completed task remains blocked", func(t *testing.T) {
		taskRepo := newStep04TaskRepo(&domain.Task{
			ID:               32,
			TaskStatus:       domain.TaskStatusCompleted,
			OwnerDepartment:  string(domain.DepartmentOperations),
			DesignerID:       &currentDesignerID,
			CurrentHandlerID: &currentDesignerID,
		})
		svc := NewTaskAssignmentService(taskRepo, &step04TaskEventRepo{}, step04TxRunner{}, WithTaskAssignmentScopeUserRepo(userRepo))
		_, appErr := svc.Assign(ctx, AssignTaskParams{TaskID: 32, DesignerID: authzInt64Ptr(202), AssignedBy: 18})
		if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
			t.Fatalf("Assign(completed) appErr = %+v, want permission denied", appErr)
		}
	})
}

func TestTaskAssignmentServiceInjectedAuthorizerDeniesHydratedDepartmentManagerOutsideScope(t *testing.T) {
	userRepo := newIdentityUserRepo()
	userRepo.users[18] = &domain.User{
		ID:         18,
		Username:   "dept_admin",
		Department: domain.DepartmentDesign,
		Team:       "default-team",
	}
	userRepo.roles[18] = []domain.Role{domain.RoleDeptAdmin}

	currentDesignerID := int64(101)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:               18,
		TaskStatus:       domain.TaskStatusInProgress,
		OwnerDepartment:  "ops",
		OwnerOrgTeam:     "ops-team-1",
		DesignerID:       &currentDesignerID,
		CurrentHandlerID: &currentDesignerID,
	})
	eventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssignmentService(
		taskRepo,
		eventRepo,
		step04TxRunner{},
		WithTaskAssignmentDataScopeResolver(NewRoleBasedDataScopeResolver()),
		WithTaskAssignmentScopeUserRepo(userRepo),
	)

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    18,
		Roles: []domain.Role{domain.RoleDeptAdmin},
	})
	_, appErr := svc.Assign(ctx, AssignTaskParams{
		TaskID:     18,
		DesignerID: authzInt64Ptr(202),
		AssignedBy: 18,
		Remark:     "should be denied by hydrated scope",
	})
	if appErr == nil {
		t.Fatal("Assign() expected permission error")
	}
	if appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Assign() code = %s, want %s", appErr.Code, domain.ErrCodePermissionDenied)
	}
	if len(eventRepo.events) != 0 {
		t.Fatalf("Assign() events = %+v, want none", eventRepo.events)
	}
}

func TestTaskAssetServiceSubmitDesignFromInProgress(t *testing.T) {
	ctx := context.Background()
	designerID := int64(101)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:               2,
		DesignerID:       &designerID,
		CurrentHandlerID: &designerID,
		TaskStatus:       domain.TaskStatusInProgress,
	})
	assetRepo := newStep04TaskAssetRepo()
	eventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssetService(taskRepo, assetRepo, eventRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), step04TxRunner{})

	asset, appErr := svc.SubmitDesign(ctx, SubmitDesignParams{
		TaskID:     2,
		UploadedBy: 101,
		AssetType:  domain.TaskAssetTypeDraft,
		FileName:   "draft-v1.psd",
		FilePath:   strPtr("mock/path/draft-v1.psd"),
	})
	if appErr != nil {
		t.Fatalf("SubmitDesign() unexpected error: %+v", appErr)
	}
	if asset.VersionNo != 1 {
		t.Fatalf("SubmitDesign() version_no = %d, want 1", asset.VersionNo)
	}
	if taskRepo.tasks[2].TaskStatus != domain.TaskStatusPendingAuditA {
		t.Fatalf("SubmitDesign() task status = %s, want PendingAuditA", taskRepo.tasks[2].TaskStatus)
	}
	if taskRepo.tasks[2].CurrentHandlerID != nil {
		t.Fatalf("SubmitDesign() current_handler_id = %+v, want nil", taskRepo.tasks[2].CurrentHandlerID)
	}
	if len(eventRepo.events) != 1 || eventRepo.events[0].EventType != domain.TaskEventDesignSubmitted {
		t.Fatalf("SubmitDesign() expected one task.design.submitted event, got %+v", eventRepo.events)
	}
}

func TestTaskAssetServiceSubmitDesignFromRejectedByAuditA(t *testing.T) {
	ctx := context.Background()
	designerID := int64(102)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:         3,
		DesignerID: &designerID,
		TaskStatus: domain.TaskStatusRejectedByAuditA,
	})
	assetRepo := newStep04TaskAssetRepo()
	eventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssetService(taskRepo, assetRepo, eventRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), step04TxRunner{})

	asset, appErr := svc.SubmitDesign(ctx, SubmitDesignParams{
		TaskID:     3,
		UploadedBy: 102,
		AssetType:  domain.TaskAssetTypeRevised,
		FileName:   "revise-v2.psd",
		Remark:     "fix audit issues",
	})
	if appErr != nil {
		t.Fatalf("SubmitDesign(rejected) unexpected error: %+v", appErr)
	}
	if asset.AssetType != domain.TaskAssetTypeDelivery {
		t.Fatalf("SubmitDesign(rejected) asset_type = %s, want delivery", asset.AssetType)
	}
	if taskRepo.tasks[3].TaskStatus != domain.TaskStatusPendingAuditA {
		t.Fatalf("SubmitDesign(rejected) task status = %s, want PendingAuditA", taskRepo.tasks[3].TaskStatus)
	}
}

func TestTaskAssetServiceSubmitDesignFromRejectedByAuditB(t *testing.T) {
	ctx := context.Background()
	designerID := int64(103)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:               6,
		DesignerID:       &designerID,
		CurrentHandlerID: &designerID,
		TaskStatus:       domain.TaskStatusRejectedByAuditB,
	})
	assetRepo := newStep04TaskAssetRepo()
	eventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssetService(taskRepo, assetRepo, eventRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), step04TxRunner{})

	asset, appErr := svc.SubmitDesign(ctx, SubmitDesignParams{
		TaskID:     6,
		UploadedBy: 103,
		AssetType:  domain.TaskAssetTypeRevised,
		FileName:   "revise-b.psd",
		Remark:     "fix warehouse/audit B issues",
	})
	if appErr != nil {
		t.Fatalf("SubmitDesign(rejected B) unexpected error: %+v", appErr)
	}
	if asset.AssetType != domain.TaskAssetTypeDelivery {
		t.Fatalf("SubmitDesign(rejected B) asset_type = %s, want delivery", asset.AssetType)
	}
	if taskRepo.tasks[6].TaskStatus != domain.TaskStatusPendingAuditA {
		t.Fatalf("SubmitDesign(rejected B) task status = %s, want PendingAuditA", taskRepo.tasks[6].TaskStatus)
	}
	if taskRepo.tasks[6].CurrentHandlerID != nil {
		t.Fatalf("SubmitDesign(rejected B) current_handler_id = %+v, want nil", taskRepo.tasks[6].CurrentHandlerID)
	}
}

func TestTaskAssetServiceMockUploadKeepsStatusAndIncrementsVersion(t *testing.T) {
	ctx := context.Background()
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:         4,
		TaskStatus: domain.TaskStatusPendingAssign,
	})
	assetRepo := newStep04TaskAssetRepo()
	eventRepo := &step04TaskEventRepo{}
	svc := NewTaskAssetService(taskRepo, assetRepo, eventRepo, newStep37UploadRequestRepo(), newStep37AssetStorageRefRepo(), step04TxRunner{})

	first, appErr := svc.MockUpload(ctx, MockUploadTaskAssetParams{
		TaskID:     4,
		UploadedBy: 201,
		AssetType:  domain.TaskAssetTypeReference,
		FileName:   "ref-1.png",
	})
	if appErr != nil {
		t.Fatalf("MockUpload(first) unexpected error: %+v", appErr)
	}
	second, appErr := svc.MockUpload(ctx, MockUploadTaskAssetParams{
		TaskID:     4,
		UploadedBy: 201,
		AssetType:  domain.TaskAssetTypeReference,
		FileName:   "ref-2.png",
	})
	if appErr != nil {
		t.Fatalf("MockUpload(second) unexpected error: %+v", appErr)
	}

	if first.VersionNo != 1 || second.VersionNo != 2 {
		t.Fatalf("MockUpload() versions = %d,%d, want 1,2", first.VersionNo, second.VersionNo)
	}
	if taskRepo.tasks[4].TaskStatus != domain.TaskStatusPendingAssign {
		t.Fatalf("MockUpload() task status changed to %s", taskRepo.tasks[4].TaskStatus)
	}
	if len(eventRepo.events) != 2 || eventRepo.events[0].EventType != domain.TaskEventAssetMockUploaded {
		t.Fatalf("MockUpload() expected mock upload events, got %+v", eventRepo.events)
	}
	if first.StorageRef == nil || first.StorageRef.RefID == "" {
		t.Fatalf("MockUpload() storage_ref = %+v", first.StorageRef)
	}
	if first.StorageRef.AdapterRefSummary == nil || first.StorageRef.ResourceRefSummary == nil {
		t.Fatalf("MockUpload() storage_ref summaries = %+v", first.StorageRef)
	}
}

func TestTaskAssetServiceSubmitDesignConsumesUploadRequest(t *testing.T) {
	ctx := context.Background()
	designerID := int64(301)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:         5,
		DesignerID: &designerID,
		TaskStatus: domain.TaskStatusInProgress,
	})
	assetRepo := newStep04TaskAssetRepo()
	eventRepo := &step04TaskEventRepo{}
	uploadRequestRepo := newStep37UploadRequestRepo()
	uploadAssetType := domain.TaskAssetTypeFinal
	uploadRequestRepo.requests["upload-1"] = &domain.UploadRequest{
		RequestID:      "upload-1",
		OwnerType:      domain.AssetOwnerTypeTask,
		OwnerID:        5,
		TaskAssetType:  &uploadAssetType,
		StorageAdapter: domain.AssetStorageAdapterPlaceholderStorage,
		RefType:        domain.AssetStorageRefTypeTaskAssetObject,
		FileName:       "final.psd",
		MimeType:       "image/vnd.adobe.photoshop",
		Status:         domain.UploadRequestStatusRequested,
		IsPlaceholder:  true,
	}
	svc := NewTaskAssetService(taskRepo, assetRepo, eventRepo, uploadRequestRepo, newStep37AssetStorageRefRepo(), step04TxRunner{})

	asset, appErr := svc.SubmitDesign(ctx, SubmitDesignParams{
		TaskID:          5,
		UploadedBy:      301,
		AssetType:       domain.TaskAssetTypeFinal,
		UploadRequestID: "upload-1",
		FileName:        "final.psd",
	})
	if appErr != nil {
		t.Fatalf("SubmitDesign(upload_request) unexpected error: %+v", appErr)
	}
	if asset.UploadRequestID == nil || *asset.UploadRequestID != "upload-1" {
		t.Fatalf("asset upload_request_id = %+v", asset.UploadRequestID)
	}
	if asset.StorageRef == nil || asset.StorageRef.UploadRequestID != "upload-1" {
		t.Fatalf("asset storage_ref = %+v", asset.StorageRef)
	}
	if asset.StorageRef.ResourceRefSummary == nil || asset.StorageRef.ResourceRefSummary.RefKey != asset.StorageRef.RefKey {
		t.Fatalf("asset storage_ref resource_ref_summary = %+v", asset.StorageRef.ResourceRefSummary)
	}
	request := uploadRequestRepo.requests["upload-1"]
	if request.Status != domain.UploadRequestStatusBound {
		t.Fatalf("upload request status = %s, want bound", request.Status)
	}
	if request.BoundAssetID == nil || *request.BoundAssetID != asset.ID {
		t.Fatalf("upload request bound_asset_id = %+v, want %d", request.BoundAssetID, asset.ID)
	}
}

type step04Tx struct{}

func (step04Tx) IsTx() {}

type step04TxRunner struct{}

func (step04TxRunner) RunInTx(_ context.Context, fn func(tx repo.Tx) error) error {
	return fn(step04Tx{})
}

type step04TaskRepo struct {
	tasks     map[int64]*domain.Task
	skuItems  map[int64][]*domain.TaskSKUItem
	skuByCode map[string]*domain.TaskSKUItem
}

func newStep04TaskRepo(tasks ...*domain.Task) *step04TaskRepo {
	store := make(map[int64]*domain.Task, len(tasks))
	for _, task := range tasks {
		store[task.ID] = task
	}
	return &step04TaskRepo{tasks: store}
}

func (r *step04TaskRepo) Create(_ context.Context, _ repo.Tx, task *domain.Task, _ *domain.TaskDetail) (int64, error) {
	r.tasks[task.ID] = task
	return task.ID, nil
}

func (r *step04TaskRepo) CreateSKUItems(_ context.Context, _ repo.Tx, items []*domain.TaskSKUItem) error {
	if r.skuItems == nil {
		r.skuItems = map[int64][]*domain.TaskSKUItem{}
	}
	if r.skuByCode == nil {
		r.skuByCode = map[string]*domain.TaskSKUItem{}
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		copied := *item
		if copied.ID == 0 {
			copied.ID = int64(len(r.skuItems[item.TaskID]) + 1)
		}
		item.ID = copied.ID
		r.skuItems[item.TaskID] = append(r.skuItems[item.TaskID], &copied)
		r.skuByCode[item.SKUCode] = &copied
	}
	return nil
}

func (r *step04TaskRepo) GetByID(_ context.Context, id int64) (*domain.Task, error) {
	return r.tasks[id], nil
}

func (r *step04TaskRepo) GetDetailByTaskID(_ context.Context, _ int64) (*domain.TaskDetail, error) {
	return nil, nil
}

func (r *step04TaskRepo) GetSKUItemBySKUCode(_ context.Context, skuCode string) (*domain.TaskSKUItem, error) {
	if r.skuByCode == nil {
		return nil, nil
	}
	return r.skuByCode[skuCode], nil
}

func (r *step04TaskRepo) ListSKUItemsByTaskID(_ context.Context, taskID int64) ([]*domain.TaskSKUItem, error) {
	if r.skuItems == nil {
		return []*domain.TaskSKUItem{}, nil
	}
	return r.skuItems[taskID], nil
}

func (r *step04TaskRepo) List(_ context.Context, _ repo.TaskListFilter) ([]*domain.TaskListItem, int64, error) {
	return []*domain.TaskListItem{}, int64(len(r.tasks)), nil
}

func (r *step04TaskRepo) ListBoardCandidates(_ context.Context, _ repo.TaskBoardCandidateFilter) ([]*domain.TaskListItem, error) {
	return []*domain.TaskListItem{}, nil
}

func (r *step04TaskRepo) UpdateDetailBusinessInfo(_ context.Context, _ repo.Tx, _ *domain.TaskDetail) error {
	return nil
}

func (r *step04TaskRepo) UpdateProductBinding(_ context.Context, _ repo.Tx, task *domain.Task) error {
	r.tasks[task.ID] = task
	return nil
}

func (r *step04TaskRepo) UpdateStatus(_ context.Context, _ repo.Tx, id int64, status domain.TaskStatus) error {
	r.tasks[id].TaskStatus = status
	return nil
}

func (r *step04TaskRepo) UpdateDesigner(_ context.Context, _ repo.Tx, id int64, designerID *int64) error {
	r.tasks[id].DesignerID = designerID
	return nil
}

func (r *step04TaskRepo) UpdateHandler(_ context.Context, _ repo.Tx, id int64, handlerID *int64) error {
	r.tasks[id].CurrentHandlerID = handlerID
	return nil
}

func (r *step04TaskRepo) UpdateCustomizationState(_ context.Context, _ repo.Tx, id int64, lastOperatorID *int64, rejectReason, rejectCategory string) error {
	task := r.tasks[id]
	if task == nil {
		return nil
	}
	task.LastCustomizationOperatorID = lastOperatorID
	task.WarehouseRejectReason = rejectReason
	task.WarehouseRejectCategory = rejectCategory
	return nil
}

type step04TaskAssetRepo struct {
	nextID int64
	assets map[int64]*domain.TaskAsset
}

func newStep04TaskAssetRepo() *step04TaskAssetRepo {
	return &step04TaskAssetRepo{
		nextID: 1,
		assets: map[int64]*domain.TaskAsset{},
	}
}

func (r *step04TaskAssetRepo) Create(_ context.Context, _ repo.Tx, asset *domain.TaskAsset) (int64, error) {
	asset.ID = r.nextID
	r.assets[asset.ID] = asset
	r.nextID++
	return asset.ID, nil
}

func (r *step04TaskAssetRepo) GetByID(_ context.Context, id int64) (*domain.TaskAsset, error) {
	return r.assets[id], nil
}

func (r *step04TaskAssetRepo) ListByTaskID(_ context.Context, taskID int64) ([]*domain.TaskAsset, error) {
	var out []*domain.TaskAsset
	for _, asset := range r.assets {
		if asset.TaskID == taskID {
			out = append(out, asset)
		}
	}
	return out, nil
}

func (r *step04TaskAssetRepo) ListByAssetID(_ context.Context, assetID int64) ([]*domain.TaskAsset, error) {
	var out []*domain.TaskAsset
	for _, asset := range r.assets {
		if asset.AssetID != nil && *asset.AssetID == assetID {
			out = append(out, asset)
		}
	}
	return out, nil
}

func (r *step04TaskAssetRepo) NextVersionNo(_ context.Context, _ repo.Tx, taskID int64) (int, error) {
	maxVersion := 0
	for _, asset := range r.assets {
		if asset.TaskID == taskID && asset.VersionNo > maxVersion {
			maxVersion = asset.VersionNo
		}
	}
	return maxVersion + 1, nil
}

func (r *step04TaskAssetRepo) NextAssetVersionNo(_ context.Context, _ repo.Tx, assetID int64) (int, error) {
	maxVersion := 0
	for _, asset := range r.assets {
		if asset.AssetID != nil && *asset.AssetID == assetID && asset.AssetVersionNo != nil && *asset.AssetVersionNo > maxVersion {
			maxVersion = *asset.AssetVersionNo
		}
	}
	return maxVersion + 1, nil
}

type step04TaskEventRepo struct {
	events []*domain.TaskEvent
}

func (r *step04TaskEventRepo) Append(_ context.Context, _ repo.Tx, taskID int64, eventType string, operatorID *int64, payload interface{}) (*domain.TaskEvent, error) {
	raw, _ := json.Marshal(payload)
	event := &domain.TaskEvent{
		TaskID:     taskID,
		Sequence:   int64(len(r.events) + 1),
		EventType:  eventType,
		OperatorID: operatorID,
		Payload:    raw,
	}
	r.events = append(r.events, event)
	return event, nil
}

func (r *step04TaskEventRepo) ListByTaskID(_ context.Context, _ int64) ([]*domain.TaskEvent, error) {
	return r.events, nil
}

func (r *step04TaskEventRepo) ListRecent(_ context.Context, _ repo.TaskEventListFilter) ([]*domain.TaskEvent, int64, error) {
	return r.events, int64(len(r.events)), nil
}

func strPtr(s string) *string {
	return &s
}

type step37UploadRequestRepo struct {
	requests map[string]*domain.UploadRequest
}

func newStep37UploadRequestRepo() *step37UploadRequestRepo {
	return &step37UploadRequestRepo{requests: map[string]*domain.UploadRequest{}}
}

func (r *step37UploadRequestRepo) Create(_ context.Context, _ repo.Tx, request *domain.UploadRequest) (*domain.UploadRequest, error) {
	if request.RequestID == "" {
		// Unique per row so multiple reference sessions in one test repo do not overwrite each other.
		request.RequestID = fmt.Sprintf("upload-test-%d", len(r.requests)+1)
	}
	if request.LastSyncedAt == nil && !request.CreatedAt.IsZero() {
		request.LastSyncedAt = &request.CreatedAt
	}
	r.requests[request.RequestID] = request
	return request, nil
}

func (r *step37UploadRequestRepo) GetByRequestID(_ context.Context, requestID string) (*domain.UploadRequest, error) {
	return r.requests[requestID], nil
}

func (r *step37UploadRequestRepo) List(_ context.Context, filter repo.UploadRequestListFilter) ([]*domain.UploadRequest, int64, error) {
	out := make([]*domain.UploadRequest, 0, len(r.requests))
	for _, request := range r.requests {
		if filter.OwnerType != nil && request.OwnerType != *filter.OwnerType {
			continue
		}
		if filter.OwnerID != nil && request.OwnerID != *filter.OwnerID {
			continue
		}
		if filter.TaskAssetType != nil {
			if request.TaskAssetType == nil || *request.TaskAssetType != *filter.TaskAssetType {
				continue
			}
		}
		if filter.Status != nil && request.Status != *filter.Status {
			continue
		}
		copyRequest := *request
		domain.HydrateUploadRequestDerived(&copyRequest)
		out = append(out, &copyRequest)
	}
	total := int64(len(out))
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(out) {
		return []*domain.UploadRequest{}, total, nil
	}
	end := start + pageSize
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], total, nil
}

func (r *step37UploadRequestRepo) UpdateLifecycle(_ context.Context, _ repo.Tx, update repo.UploadRequestLifecycleUpdate) error {
	request := r.requests[update.RequestID]
	request.Status = update.Status
	request.Remark = update.Remark
	domain.HydrateUploadRequestDerived(request)
	return nil
}

func (r *step37UploadRequestRepo) UpdateBinding(_ context.Context, _ repo.Tx, requestID string, boundAssetID *int64, boundRefID string, status domain.UploadRequestStatus, remark string) error {
	request := r.requests[requestID]
	request.BoundAssetID = boundAssetID
	request.BoundRefID = boundRefID
	request.Status = status
	request.Remark = remark
	domain.HydrateUploadRequestDerived(request)
	return nil
}

func (r *step37UploadRequestRepo) UpdateSession(_ context.Context, _ repo.Tx, update repo.UploadRequestSessionUpdate) error {
	request := r.requests[update.RequestID]
	if update.AssetID != nil {
		request.AssetID = update.AssetID
	}
	request.SessionStatus = update.SessionStatus
	request.RemoteUploadID = update.RemoteUploadID
	if update.RemoteFileID != nil {
		request.RemoteFileID = *update.RemoteFileID
	}
	if update.CreatedBy != nil {
		request.CreatedBy = *update.CreatedBy
	}
	request.ExpiresAt = update.ExpiresAt
	request.LastSyncedAt = update.LastSyncedAt
	request.Remark = update.Remark
	domain.HydrateUploadRequestDerived(request)
	return nil
}

type step37AssetStorageRefRepo struct {
	refs map[string]*domain.AssetStorageRef
}

func newStep37AssetStorageRefRepo() *step37AssetStorageRefRepo {
	return &step37AssetStorageRefRepo{refs: map[string]*domain.AssetStorageRef{}}
}

func (r *step37AssetStorageRefRepo) Create(_ context.Context, _ repo.Tx, ref *domain.AssetStorageRef) (*domain.AssetStorageRef, error) {
	r.refs[ref.RefID] = ref
	return ref, nil
}

func (r *step37AssetStorageRefRepo) GetByRefID(_ context.Context, refID string) (*domain.AssetStorageRef, error) {
	return r.refs[refID], nil
}

func (r *step37AssetStorageRefRepo) UpdateStatus(_ context.Context, _ repo.Tx, refID string, status domain.AssetStorageRefStatus) error {
	ref := r.refs[refID]
	if ref == nil {
		return nil
	}
	ref.Status = status
	domain.HydrateAssetStorageRefDerived(ref)
	return nil
}

type step67DesignAssetRepo struct {
	nextID int64
	assets map[int64]*domain.DesignAsset
}

func newStep67DesignAssetRepo() *step67DesignAssetRepo {
	return &step67DesignAssetRepo{
		nextID: 1,
		assets: map[int64]*domain.DesignAsset{},
	}
}

func (r *step67DesignAssetRepo) Create(_ context.Context, _ repo.Tx, asset *domain.DesignAsset) (int64, error) {
	asset.ID = r.nextID
	if asset.CreatedAt.IsZero() {
		asset.CreatedAt = time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	}
	if asset.UpdatedAt.IsZero() {
		asset.UpdatedAt = asset.CreatedAt
	}
	r.assets[asset.ID] = asset
	r.nextID++
	return asset.ID, nil
}

func (r *step67DesignAssetRepo) GetByID(_ context.Context, id int64) (*domain.DesignAsset, error) {
	return r.assets[id], nil
}

func (r *step67DesignAssetRepo) List(_ context.Context, filter repo.DesignAssetListFilter) ([]*domain.DesignAsset, error) {
	var out []*domain.DesignAsset
	for _, asset := range r.assets {
		if filter.TaskID != nil && asset.TaskID != *filter.TaskID {
			continue
		}
		if filter.SourceAssetID != nil {
			if asset.SourceAssetID == nil || *asset.SourceAssetID != *filter.SourceAssetID {
				continue
			}
		}
		if filter.AssetType != nil && asset.AssetType != domain.NormalizeTaskAssetType(*filter.AssetType) {
			continue
		}
		if scopeSKUCode := strings.TrimSpace(filter.ScopeSKUCode); scopeSKUCode != "" && strings.TrimSpace(asset.ScopeSKUCode) != scopeSKUCode {
			continue
		}
		out = append(out, asset)
	}
	return out, nil
}

func (r *step67DesignAssetRepo) ListByTaskID(_ context.Context, taskID int64) ([]*domain.DesignAsset, error) {
	var out []*domain.DesignAsset
	for _, asset := range r.assets {
		if asset.TaskID == taskID {
			out = append(out, asset)
		}
	}
	return out, nil
}

func (r *step67DesignAssetRepo) NextAssetNo(_ context.Context, _ repo.Tx, taskID int64) (string, error) {
	count := 0
	for _, asset := range r.assets {
		if asset.TaskID == taskID {
			count++
		}
	}
	return fmt.Sprintf("AST-%04d", count+1), nil
}

func (r *step67DesignAssetRepo) UpdateCurrentVersionID(_ context.Context, _ repo.Tx, id int64, currentVersionID *int64) error {
	asset := r.assets[id]
	if asset == nil {
		return nil
	}
	asset.CurrentVersionID = currentVersionID
	asset.UpdatedAt = time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	return nil
}
