package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestAuditV7ServiceApproveClearsHandlerForNextStage(t *testing.T) {
	currentHandlerID := int64(41)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			1: {
				ID:               1,
				TaskNo:           "RW-001",
				SKUCode:          "SKU-001",
				TaskType:         domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				CurrentHandlerID: &currentHandlerID,
			},
		},
	}
	auditRepo := &auditV7RepoStub{}
	eventRepo := &prdTaskEventRepo{}
	svc := NewAuditV7Service(taskRepo, auditRepo, eventRepo, prdCodeRuleService{}, step04TxRunner{})

	appErr := svc.Approve(context.Background(), ApproveAuditParams{
		TaskID:     1,
		AuditorID:  41,
		Stage:      domain.AuditRecordStageA,
		NextStatus: domain.TaskStatusPendingAuditB,
		Comment:    "pass to B",
	})
	if appErr != nil {
		t.Fatalf("Approve() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[1].TaskStatus != domain.TaskStatusPendingAuditB {
		t.Fatalf("Approve() task status = %s, want %s", taskRepo.tasks[1].TaskStatus, domain.TaskStatusPendingAuditB)
	}
	if taskRepo.tasks[1].CurrentHandlerID != nil {
		t.Fatalf("Approve() current_handler_id = %+v, want nil", taskRepo.tasks[1].CurrentHandlerID)
	}
}

func TestAuditV7ServiceApproveToPendingOutsourcePersistsNeedOutsource(t *testing.T) {
	currentHandlerID := int64(81)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			4: {
				ID:               4,
				TaskNo:           "RW-004",
				SKUCode:          "SKU-004",
				TaskType:         domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				CurrentHandlerID: &currentHandlerID,
				NeedOutsource:    false,
			},
		},
	}
	auditRepo := &auditV7RepoStub{}
	eventRepo := &prdTaskEventRepo{}
	svc := NewAuditV7Service(taskRepo, auditRepo, eventRepo, prdCodeRuleService{}, step04TxRunner{})

	appErr := svc.Approve(context.Background(), ApproveAuditParams{
		TaskID:     4,
		AuditorID:  81,
		Stage:      domain.AuditRecordStageA,
		NextStatus: domain.TaskStatusPendingOutsource,
		Comment:    "need outsource",
	})
	if appErr != nil {
		t.Fatalf("Approve() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[4].TaskStatus != domain.TaskStatusPendingOutsource {
		t.Fatalf("Approve() task status = %s, want %s", taskRepo.tasks[4].TaskStatus, domain.TaskStatusPendingOutsource)
	}
	if !taskRepo.tasks[4].NeedOutsource {
		t.Fatal("Approve() should persist need_outsource=true when routing to PendingOutsource")
	}
}

func TestAuditV7ServiceRejectRoutesBackToDesigner(t *testing.T) {
	designerID := int64(51)
	auditorID := int64(61)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			2: {
				ID:               2,
				TaskNo:           "RW-002",
				SKUCode:          "SKU-002",
				TaskType:         domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:       domain.TaskStatusPendingAuditB,
				DesignerID:       &designerID,
				CurrentHandlerID: &auditorID,
			},
		},
	}
	auditRepo := &auditV7RepoStub{}
	eventRepo := &prdTaskEventRepo{}
	svc := NewAuditV7Service(taskRepo, auditRepo, eventRepo, prdCodeRuleService{}, step04TxRunner{})

	appErr := svc.Reject(context.Background(), RejectAuditParams{
		TaskID:     2,
		AuditorID:  auditorID,
		Stage:      domain.AuditRecordStageB,
		Comment:    "fix layout",
		IssueTypes: []string{"layout_error"},
	})
	if appErr != nil {
		t.Fatalf("Reject() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[2].TaskStatus != domain.TaskStatusRejectedByAuditB {
		t.Fatalf("Reject() task status = %s, want %s", taskRepo.tasks[2].TaskStatus, domain.TaskStatusRejectedByAuditB)
	}
	if taskRepo.tasks[2].CurrentHandlerID == nil || *taskRepo.tasks[2].CurrentHandlerID != designerID {
		t.Fatalf("Reject() current_handler_id = %+v, want %d", taskRepo.tasks[2].CurrentHandlerID, designerID)
	}
}

func TestAuditV7ServiceApproveEnqueuesExperienceSample(t *testing.T) {
	currentHandlerID := int64(71)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			25: {
				ID:                  25,
				TaskNo:              "RW-025",
				SKUCode:             "SKU-025",
				TaskType:            domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:          domain.TaskStatusPendingAuditA,
				BusinessLane:        domain.TaskBusinessLaneNormal,
				OwnerDepartment:     "运营部",
				OwnerOrgTeam:        "淘系一组",
				Priority:            domain.TaskPriorityHigh,
				CurrentHandlerID:    &currentHandlerID,
				ProductNameSnapshot: "测试商品",
			},
		},
	}
	experienceSvc := &auditExperienceServiceStub{}
	svc := NewAuditV7Service(taskRepo, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7ExperienceService(experienceSvc))

	appErr := svc.Approve(context.Background(), ApproveAuditParams{
		TaskID:     25,
		AuditorID:  71,
		Stage:      domain.AuditRecordStageA,
		NextStatus: domain.TaskStatusPendingAuditB,
		Comment:    "pass",
	})
	if appErr != nil {
		t.Fatalf("Approve() unexpected error: %+v", appErr)
	}
	if len(experienceSvc.events) != 1 {
		t.Fatalf("experience events = %d, want 1", len(experienceSvc.events))
	}
	event := experienceSvc.events[0]
	if event.SourceType != "task_audit" || event.SourceID != "RW-025" || event.Action != "audit_approved" || event.Outcome != "approved" {
		t.Fatalf("experience event identity = %#v", event)
	}
	if event.TaskID == nil || *event.TaskID != 25 {
		t.Fatalf("experience task_id = %+v, want 25", event.TaskID)
	}
	var business map[string]interface{}
	if err := json.Unmarshal(event.BusinessSnapshot, &business); err != nil {
		t.Fatalf("business snapshot json: %v", err)
	}
	if business["from_task_status"] != string(domain.TaskStatusPendingAuditA) || business["to_task_status"] != string(domain.TaskStatusPendingAuditB) {
		t.Fatalf("business status snapshot = %#v", business)
	}
}

func TestAuditV7ServiceRejectExperienceFailureDoesNotBlockAudit(t *testing.T) {
	designerID := int64(81)
	auditorID := int64(82)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			26: {
				ID:               26,
				TaskNo:           "RW-026",
				SKUCode:          "SKU-026",
				TaskType:         domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:       domain.TaskStatusPendingAuditB,
				DesignerID:       &designerID,
				CurrentHandlerID: &auditorID,
			},
		},
	}
	experienceSvc := &auditExperienceServiceStub{failEnqueue: true}
	svc := NewAuditV7Service(taskRepo, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7ExperienceService(experienceSvc))

	appErr := svc.Reject(context.Background(), RejectAuditParams{
		TaskID:     26,
		AuditorID:  auditorID,
		Stage:      domain.AuditRecordStageB,
		Comment:    "layout needs fix",
		IssueTypes: []string{"layout_error"},
	})
	if appErr != nil {
		t.Fatalf("Reject() unexpected error when experience enqueue fails: %+v", appErr)
	}
	if taskRepo.tasks[26].TaskStatus != domain.TaskStatusRejectedByAuditB {
		t.Fatalf("task status = %s, want %s", taskRepo.tasks[26].TaskStatus, domain.TaskStatusRejectedByAuditB)
	}
	if experienceSvc.enqueueCalls != 1 {
		t.Fatalf("experience enqueue calls = %d, want 1", experienceSvc.enqueueCalls)
	}
}

func TestAuditV7ServiceApproveAllowsNonHandlerStageScope(t *testing.T) {
	currentHandlerID := int64(231)
	nonHandlerID := int64(235)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			28: {
				ID:               28,
				TaskNo:           "RW-028",
				TaskType:         domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				BusinessLane:     domain.TaskBusinessLaneNormal,
				OwnerDepartment:  "设计研发部",
				OwnerOrgTeam:     "设计审核组",
				CurrentHandlerID: &currentHandlerID,
			},
		},
	}
	userRepo := newIdentityUserRepo()
	userRepo.users[currentHandlerID] = &domain.User{ID: currentHandlerID, DisplayName: "马雨琪", Team: "设计审核组", Status: domain.UserStatusActive}
	userRepo.roles[currentHandlerID] = []domain.Role{domain.RoleAuditA}
	userRepo.users[nonHandlerID] = &domain.User{ID: nonHandlerID, DisplayName: "同组普通审核", Team: "设计审核组", Status: domain.UserStatusActive}
	userRepo.roles[nonHandlerID] = []domain.Role{domain.RoleAuditA}
	svc := NewAuditV7Service(taskRepo, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7ScopeUserRepo(userRepo))
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       nonHandlerID,
		Username: "same_team_auditor",
		Roles:    []domain.Role{domain.RoleAuditA},
		Team:     "设计审核组",
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})

	appErr := svc.Approve(ctx, ApproveAuditParams{
		TaskID:     28,
		AuditorID:  nonHandlerID,
		Stage:      domain.AuditRecordStageA,
		NextStatus: domain.TaskStatusPendingWarehouseReceive,
		Comment:    "direct audit without claim",
	})
	if appErr != nil {
		t.Fatalf("Approve() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[28].TaskStatus != domain.TaskStatusPendingWarehouseReceive {
		t.Fatalf("Approve() task status = %s, want PendingWarehouseReceive", taskRepo.tasks[28].TaskStatus)
	}
	if taskRepo.tasks[28].CurrentHandlerID != nil {
		t.Fatalf("Approve() current_handler_id = %+v, want cleared", taskRepo.tasks[28].CurrentHandlerID)
	}
}

func TestAuditV7ServiceHandoverRequiresTakeoverBeforeFurtherAuditActions(t *testing.T) {
	currentHandlerID := int64(71)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			3: {
				ID:               3,
				TaskNo:           "RW-003",
				SKUCode:          "SKU-003",
				TaskType:         domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				CurrentHandlerID: &currentHandlerID,
			},
		},
	}
	auditRepo := &auditV7RepoStub{}
	eventRepo := &prdTaskEventRepo{}
	svc := NewAuditV7Service(taskRepo, auditRepo, eventRepo, prdCodeRuleService{}, step04TxRunner{})

	handover, appErr := svc.Handover(context.Background(), HandoverAuditParams{
		TaskID:        3,
		FromAuditorID: 71,
		ToAuditorID:   72,
		Reason:        "shift change",
	})
	if appErr != nil {
		t.Fatalf("Handover() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[3].CurrentHandlerID != nil {
		t.Fatalf("Handover() current_handler_id = %+v, want nil", taskRepo.tasks[3].CurrentHandlerID)
	}

	appErr = svc.Claim(context.Background(), ClaimAuditParams{
		TaskID:    3,
		AuditorID: 99,
		Stage:     domain.AuditRecordStageA,
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("Claim() appErr = %+v, want invalid_state_transition", appErr)
	}

	appErr = svc.Takeover(context.Background(), 3, handover.ID, 72)
	if appErr != nil {
		t.Fatalf("Takeover() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[3].CurrentHandlerID == nil || *taskRepo.tasks[3].CurrentHandlerID != 72 {
		t.Fatalf("Takeover() current_handler_id = %+v, want 72", taskRepo.tasks[3].CurrentHandlerID)
	}
}

func TestAuditV7ServiceListHandoverCandidatesScopesToCurrentActor(t *testing.T) {
	actorID := int64(331)
	otherHandlerID := int64(332)
	now := time.Now().UTC()
	taskRepo := &prdTaskRepo{
		listItems: []*domain.TaskListItem{
			auditHandoverCandidateListItem(41, "RW-41", actorID, domain.TaskStatusPendingAuditA, domain.TaskBusinessLaneNormal, now),
			auditHandoverCandidateListItem(42, "RW-42", actorID, domain.TaskStatusPendingAuditB, domain.TaskBusinessLaneNormal, now),
			auditHandoverCandidateListItem(43, "RW-43", otherHandlerID, domain.TaskStatusPendingAuditA, domain.TaskBusinessLaneNormal, now),
			auditHandoverCandidateListItem(44, "RW-44", actorID, domain.TaskStatusPendingCustomizationReview, domain.TaskBusinessLaneCustomization, now),
		},
	}
	svc := NewAuditV7Service(taskRepo, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{})

	resp, appErr := svc.ListHandoverCandidates(auditHandoverTestActorContext(actorID, domain.RoleAuditA), AuditHandoverCandidateFilter{})
	if appErr != nil {
		t.Fatalf("ListHandoverCandidates() unexpected error: %+v", appErr)
	}
	if resp.EligibleCount != 2 || len(resp.Items) != 2 {
		t.Fatalf("candidate count = eligible %d len %d, want 2", resp.EligibleCount, len(resp.Items))
	}
	if taskRepo.lastListFilter.CurrentHandlerID == nil || *taskRepo.lastListFilter.CurrentHandlerID != actorID {
		t.Fatalf("CurrentHandlerID filter = %+v, want %d", taskRepo.lastListFilter.CurrentHandlerID, actorID)
	}
	if !taskRepo.lastListFilter.ExcludePendingAuditHandover {
		t.Fatalf("ExcludePendingAuditHandover = false, want true")
	}
	if resp.SelectedLimit != AuditHandoverBatchDefaultLimit {
		t.Fatalf("SelectedLimit = %d, want %d", resp.SelectedLimit, AuditHandoverBatchDefaultLimit)
	}
}

func TestAuditV7ServiceListHandoverCandidatesRejectsNonAuditRole(t *testing.T) {
	svc := NewAuditV7Service(&prdTaskRepo{}, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{})

	_, appErr := svc.ListHandoverCandidates(auditHandoverTestActorContext(331, domain.RoleMember), AuditHandoverCandidateFilter{})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("ListHandoverCandidates() appErr = %+v, want permission denied", appErr)
	}
}

func TestAuditV7ServiceBatchHandoverRejectsOverLimit(t *testing.T) {
	svc := NewAuditV7Service(&prdTaskRepo{}, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{})
	taskIDs := make([]int64, AuditHandoverBatchDefaultLimit+1)
	for i := range taskIDs {
		taskIDs[i] = int64(i + 1)
	}

	_, appErr := svc.BatchHandover(auditHandoverTestActorContext(331, domain.RoleAuditA), BatchAuditHandoverParams{
		Mode:        BatchAuditHandoverModeExplicit,
		TaskIDs:     taskIDs,
		ToAuditorID: 442,
		Reason:      "调班",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("BatchHandover() appErr = %+v, want invalid request", appErr)
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok || details["deny_code"] != "BATCH_LIMIT_EXCEEDED" {
		t.Fatalf("BatchHandover() details = %#v, want BATCH_LIMIT_EXCEEDED", appErr.Details)
	}
}

func TestAuditV7ServiceBatchHandoverAllMatchingAllowsPartialFailures(t *testing.T) {
	actorID := int64(331)
	toAuditorID := int64(442)
	now := time.Now().UTC()
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			41: auditHandoverTask(41, "RW-41", actorID, domain.TaskStatusPendingAuditA),
			42: auditHandoverTask(42, "RW-42", actorID, domain.TaskStatusInProgress),
		},
		listItems: []*domain.TaskListItem{
			auditHandoverCandidateListItem(41, "RW-41", actorID, domain.TaskStatusPendingAuditA, domain.TaskBusinessLaneNormal, now),
			auditHandoverCandidateListItem(42, "RW-42", actorID, domain.TaskStatusPendingAuditA, domain.TaskBusinessLaneNormal, now),
		},
	}
	auditRepo := &auditV7RepoStub{}
	svc := NewAuditV7Service(taskRepo, auditRepo, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{})

	resp, appErr := svc.BatchHandover(auditHandoverTestActorContext(actorID, domain.RoleAuditA), BatchAuditHandoverParams{
		Mode:        BatchAuditHandoverModeAllMatching,
		ToAuditorID: toAuditorID,
		Reason:      "调班",
	})
	if appErr != nil {
		t.Fatalf("BatchHandover() unexpected error: %+v", appErr)
	}
	if resp.SuccessCount != 1 || resp.FailureCount != 1 || len(resp.Results) != 2 {
		t.Fatalf("BatchHandover() result = %+v, want 1 success and 1 failure", resp)
	}
	if len(auditRepo.handovers) != 1 || auditRepo.handovers[0].TaskID != 41 {
		t.Fatalf("handovers = %+v, want only task 41 handover", auditRepo.handovers)
	}
	if taskRepo.tasks[41].CurrentHandlerID != nil {
		t.Fatalf("task 41 handler = %+v, want nil after handover", taskRepo.tasks[41].CurrentHandlerID)
	}
}

func TestAuditV7ServiceClaimDeniesDepartmentManagerOutsideScope(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			9: {
				ID:              9,
				TaskNo:          "RW-009",
				TaskType:        domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:      domain.TaskStatusPendingAuditA,
				OwnerDepartment: "运营部",
				OwnerOrgTeam:    "淘系一组",
			},
		},
	}
	userRepo := newIdentityUserRepo()
	auditRepo := &auditV7RepoStub{}
	eventRepo := &prdTaskEventRepo{}
	svc := NewAuditV7Service(taskRepo, auditRepo, eventRepo, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7DataScopeResolver(NewRoleBasedDataScopeResolver()),
		WithAuditV7ScopeUserRepo(userRepo))

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         91,
		Username:   "design_admin",
		Roles:      []domain.Role{domain.RoleDeptAdmin},
		Department: "设计研发部",
		Source:     domain.RequestActorSourceSessionToken,
		AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
	})
	appErr := svc.Claim(ctx, ClaimAuditParams{
		TaskID:    9,
		AuditorID: 91,
		Stage:     domain.AuditRecordStageA,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Claim() appErr = %+v, want permission denied", appErr)
	}
}

func TestAuditV7ServiceClaimRejectsAuditorOutsideBusinessLane(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			21: {
				ID:               21,
				TaskNo:           "RW-021",
				TaskType:         domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				BusinessLane:     domain.TaskBusinessLaneCustomization,
				OwnerDepartment:  "运营部",
				OwnerOrgTeam:     "淘系一组",
				CurrentHandlerID: authzInt64Ptr(211),
			},
		},
	}
	userRepo := newIdentityUserRepo()
	userRepo.users[211] = &domain.User{
		ID:          211,
		Username:    "mqy",
		DisplayName: "马雨琪",
		Status:      domain.UserStatusActive,
	}
	userRepo.roles[211] = []domain.Role{domain.RoleAuditA}
	svc := NewAuditV7Service(taskRepo, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7ScopeUserRepo(userRepo))

	appErr := svc.Claim(context.Background(), ClaimAuditParams{
		TaskID:    21,
		AuditorID: 211,
		Stage:     domain.AuditRecordStageA,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Claim() appErr = %+v, want permission denied", appErr)
	}
}

func TestAuditV7ServiceClaimDeniesAdminWithoutLaneBinding(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			22: {
				ID:              22,
				TaskNo:          "RW-022",
				TaskType:        domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:      domain.TaskStatusPendingAuditA,
				BusinessLane:    domain.TaskBusinessLaneNormal,
				OwnerDepartment: "运营部",
				OwnerOrgTeam:    "淘系一组",
			},
		},
	}
	userRepo := newIdentityUserRepo()
	userRepo.users[221] = &domain.User{
		ID:          221,
		Username:    "admin221",
		DisplayName: "跨域管理员",
		Status:      domain.UserStatusActive,
	}
	userRepo.roles[221] = []domain.Role{domain.RoleAdmin}
	svc := NewAuditV7Service(taskRepo, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7ScopeUserRepo(userRepo))

	appErr := svc.Claim(context.Background(), ClaimAuditParams{
		TaskID:    22,
		AuditorID: 221,
		Stage:     domain.AuditRecordStageA,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Claim() appErr = %+v, want permission denied", appErr)
	}
}

func TestAuditV7ServiceTransferRejectsCrossLaneAuditor(t *testing.T) {
	fromAuditorID := int64(231)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			23: {
				ID:               23,
				TaskNo:           "RW-023",
				TaskType:         domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				BusinessLane:     domain.TaskBusinessLaneNormal,
				CurrentHandlerID: &fromAuditorID,
			},
		},
	}
	userRepo := newIdentityUserRepo()
	userRepo.users[231] = &domain.User{ID: 231, DisplayName: "马雨琪", Status: domain.UserStatusActive}
	userRepo.roles[231] = []domain.Role{domain.RoleAuditA}
	userRepo.users[232] = &domain.User{ID: 232, DisplayName: "章鹏鹏", Status: domain.UserStatusActive}
	userRepo.roles[232] = []domain.Role{domain.RoleAuditA}
	svc := NewAuditV7Service(taskRepo, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7ScopeUserRepo(userRepo))

	appErr := svc.Transfer(context.Background(), TransferAuditParams{
		TaskID:        23,
		FromAuditorID: 231,
		ToAuditorID:   232,
		Stage:         domain.AuditRecordStageA,
		Comment:       "cross lane transfer",
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Transfer() appErr = %+v, want permission denied", appErr)
	}
}

func TestAuditV7ServiceTransferAllowsScopedManagerReassignCurrentHandler(t *testing.T) {
	fromAuditorID := int64(231)
	toAuditorID := int64(233)
	managerID := int64(999)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			25: {
				ID:               25,
				TaskNo:           "RW-025",
				TaskType:         domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				BusinessLane:     domain.TaskBusinessLaneNormal,
				OwnerDepartment:  "设计研发部",
				OwnerOrgTeam:     "设计审核组",
				CurrentHandlerID: &fromAuditorID,
			},
		},
	}
	userRepo := newIdentityUserRepo()
	userRepo.users[fromAuditorID] = &domain.User{ID: fromAuditorID, DisplayName: "马雨琪", Team: "设计审核组", Status: domain.UserStatusActive}
	userRepo.roles[fromAuditorID] = []domain.Role{domain.RoleAuditA}
	userRepo.users[toAuditorID] = &domain.User{ID: toAuditorID, DisplayName: "常规审核B", Team: "设计审核组", Status: domain.UserStatusActive}
	userRepo.roles[toAuditorID] = []domain.Role{domain.RoleAuditA}
	userRepo.users[managerID] = &domain.User{ID: managerID, DisplayName: "审核主管", Team: "设计审核组", Status: domain.UserStatusActive}
	userRepo.roles[managerID] = []domain.Role{domain.RoleSuperAdmin}
	auditRepo := &auditV7RepoStub{}
	eventRepo := &prdTaskEventRepo{}
	svc := NewAuditV7Service(taskRepo, auditRepo, eventRepo, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7ScopeUserRepo(userRepo))
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       managerID,
		Username: "audit_manager",
		Roles:    []domain.Role{domain.RoleSuperAdmin},
		Team:     "设计审核组",
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})

	appErr := svc.Transfer(ctx, TransferAuditParams{
		TaskID:        25,
		ActorID:       managerID,
		FromAuditorID: fromAuditorID,
		ToAuditorID:   toAuditorID,
		Stage:         domain.AuditRecordStageA,
		Comment:       "A 休息，主管改派",
	})
	if appErr != nil {
		t.Fatalf("Transfer() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[25].CurrentHandlerID == nil || *taskRepo.tasks[25].CurrentHandlerID != toAuditorID {
		t.Fatalf("Transfer() current_handler_id = %+v, want %d", taskRepo.tasks[25].CurrentHandlerID, toAuditorID)
	}
	if len(auditRepo.records) != 1 || auditRepo.records[0].AuditorID != managerID {
		t.Fatalf("Transfer() audit record actor = %+v, want manager %d", auditRepo.records, managerID)
	}
	if len(eventRepo.events) != 1 || eventRepo.events[0].OperatorID == nil || *eventRepo.events[0].OperatorID != managerID {
		t.Fatalf("Transfer() event operator = %+v, want manager %d", eventRepo.events, managerID)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(eventRepo.events[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal transfer payload: %v", err)
	}
	if payload["transfer_actor_id"] == nil || payload["from_auditor_id"] == nil || payload["to_auditor_id"] == nil {
		t.Fatalf("Transfer() payload missing actor/from/to fields: %v", payload)
	}
}

func TestAuditV7ServiceTransferRejectsFromAuditorMismatch(t *testing.T) {
	currentHandlerID := int64(231)
	incorrectFromID := int64(999)
	toAuditorID := int64(233)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			26: {
				ID:               26,
				TaskNo:           "RW-026",
				TaskType:         domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				BusinessLane:     domain.TaskBusinessLaneNormal,
				OwnerDepartment:  "设计研发部",
				OwnerOrgTeam:     "设计审核组",
				CurrentHandlerID: &currentHandlerID,
			},
		},
	}
	userRepo := newIdentityUserRepo()
	userRepo.users[currentHandlerID] = &domain.User{ID: currentHandlerID, DisplayName: "马雨琪", Team: "设计审核组", Status: domain.UserStatusActive}
	userRepo.roles[currentHandlerID] = []domain.Role{domain.RoleAuditA}
	userRepo.users[toAuditorID] = &domain.User{ID: toAuditorID, DisplayName: "常规审核B", Team: "设计审核组", Status: domain.UserStatusActive}
	userRepo.roles[toAuditorID] = []domain.Role{domain.RoleAuditA}
	userRepo.users[incorrectFromID] = &domain.User{ID: incorrectFromID, DisplayName: "审核主管", Team: "设计审核组", Status: domain.UserStatusActive}
	userRepo.roles[incorrectFromID] = []domain.Role{domain.RoleSuperAdmin}
	svc := NewAuditV7Service(taskRepo, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7ScopeUserRepo(userRepo))
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       incorrectFromID,
		Username: "audit_manager",
		Roles:    []domain.Role{domain.RoleSuperAdmin},
		Team:     "设计审核组",
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})

	appErr := svc.Transfer(ctx, TransferAuditParams{
		TaskID:        26,
		ActorID:       incorrectFromID,
		FromAuditorID: incorrectFromID,
		ToAuditorID:   toAuditorID,
		Stage:         domain.AuditRecordStageA,
		Comment:       "bad from",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("Transfer() appErr = %+v, want invalid request", appErr)
	}
	details, _ := appErr.Details.(map[string]interface{})
	if got := details["deny_code"]; got != "audit_transfer_from_mismatch" {
		t.Fatalf("Transfer() deny_code = %v, want audit_transfer_from_mismatch", got)
	}
}

func TestAuditV7ServiceTransferRejectsNonHandlerSpoofingCurrentHandler(t *testing.T) {
	currentHandlerID := int64(231)
	toAuditorID := int64(233)
	nonHandlerID := int64(235)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			27: {
				ID:               27,
				TaskNo:           "RW-027",
				TaskType:         domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				BusinessLane:     domain.TaskBusinessLaneNormal,
				OwnerDepartment:  "设计研发部",
				OwnerOrgTeam:     "设计审核组",
				CurrentHandlerID: &currentHandlerID,
			},
		},
	}
	userRepo := newIdentityUserRepo()
	userRepo.users[currentHandlerID] = &domain.User{ID: currentHandlerID, DisplayName: "马雨琪", Team: "设计审核组", Status: domain.UserStatusActive}
	userRepo.roles[currentHandlerID] = []domain.Role{domain.RoleAuditA}
	userRepo.users[toAuditorID] = &domain.User{ID: toAuditorID, DisplayName: "常规审核B", Team: "设计审核组", Status: domain.UserStatusActive}
	userRepo.roles[toAuditorID] = []domain.Role{domain.RoleAuditA}
	userRepo.users[nonHandlerID] = &domain.User{ID: nonHandlerID, DisplayName: "同组普通审核", Team: "设计审核组", Status: domain.UserStatusActive}
	userRepo.roles[nonHandlerID] = []domain.Role{domain.RoleAuditA}
	svc := NewAuditV7Service(taskRepo, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7ScopeUserRepo(userRepo))
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       nonHandlerID,
		Username: "same_team_auditor",
		Roles:    []domain.Role{domain.RoleAuditA},
		Team:     "设计审核组",
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})

	appErr := svc.Transfer(ctx, TransferAuditParams{
		TaskID:        27,
		ActorID:       nonHandlerID,
		FromAuditorID: currentHandlerID,
		ToAuditorID:   toAuditorID,
		Stage:         domain.AuditRecordStageA,
		Comment:       "spoof current handler",
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Transfer() appErr = %+v, want permission denied", appErr)
	}
	details, _ := appErr.Details.(map[string]interface{})
	if got := details["deny_code"]; got != "task_not_assigned_to_actor" {
		t.Fatalf("Transfer() deny_code = %v, want task_not_assigned_to_actor", got)
	}
	if taskRepo.tasks[27].CurrentHandlerID == nil || *taskRepo.tasks[27].CurrentHandlerID != currentHandlerID {
		t.Fatalf("Transfer() current_handler_id = %+v, want unchanged %d", taskRepo.tasks[27].CurrentHandlerID, currentHandlerID)
	}
}

func TestAuditV7ServiceListHandoversAllowsRegularAuditorByStage(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			29: {
				ID:              29,
				TaskNo:          "RW-029",
				TaskType:        domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:      domain.TaskStatusPendingAuditA,
				BusinessLane:    domain.TaskBusinessLaneNormal,
				OwnerDepartment: string(domain.DepartmentOperations),
				OwnerOrgTeam:    "淘系一组",
			},
		},
	}
	auditRepo := &auditV7RepoStub{
		handovers: []*domain.AuditHandover{
			{ID: 2901, TaskID: 29, FromAuditorID: 231, ToAuditorID: 232, Status: domain.HandoverStatusPendingTakeover},
		},
	}
	svc := NewAuditV7Service(taskRepo, auditRepo, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7DataScopeResolver(NewRoleBasedDataScopeResolver()))
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       231,
		Username: "audit_a",
		Roles:    []domain.Role{domain.RoleAuditA},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})

	handovers, appErr := svc.ListHandovers(ctx, 29)
	if appErr != nil {
		t.Fatalf("ListHandovers() unexpected error: %+v", appErr)
	}
	if len(handovers) != 1 || handovers[0].ID != 2901 {
		t.Fatalf("ListHandovers() = %+v, want handover 2901", handovers)
	}
}

func TestAuditV7ServiceListHandoversAllowsScopedDepartmentManager(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			30: {
				ID:              30,
				TaskNo:          "RW-030",
				TaskType:        domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:      domain.TaskStatusPendingAuditA,
				BusinessLane:    domain.TaskBusinessLaneNormal,
				OwnerDepartment: string(domain.DepartmentDesignRD),
				OwnerOrgTeam:    "设计审核组",
			},
		},
	}
	auditRepo := &auditV7RepoStub{
		handovers: []*domain.AuditHandover{
			{ID: 3001, TaskID: 30, FromAuditorID: 231, ToAuditorID: 232, Status: domain.HandoverStatusPendingTakeover},
		},
	}
	userRepo := newIdentityUserRepo()
	userRepo.users[301] = &domain.User{
		ID:                 301,
		Username:           "design_dept_admin",
		DisplayName:        "设计部门管理员",
		Department:         domain.DepartmentDesignRD,
		ManagedDepartments: []string{string(domain.DepartmentDesignRD)},
		Status:             domain.UserStatusActive,
	}
	userRepo.roles[301] = []domain.Role{domain.RoleDeptAdmin}
	svc := NewAuditV7Service(taskRepo, auditRepo, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7DataScopeResolver(NewRoleBasedDataScopeResolver()),
		WithAuditV7ScopeUserRepo(userRepo))
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       301,
		Username: "design_dept_admin",
		Roles:    []domain.Role{domain.RoleDeptAdmin},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})

	handovers, appErr := svc.ListHandovers(ctx, 30)
	if appErr != nil {
		t.Fatalf("ListHandovers() unexpected error: %+v", appErr)
	}
	if len(handovers) != 1 || handovers[0].ID != 3001 {
		t.Fatalf("ListHandovers() = %+v, want handover 3001", handovers)
	}
}

func TestAuditV7ServiceListHandoversRejectsScopedDepartmentManagerOutsideScope(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			31: {
				ID:              31,
				TaskNo:          "RW-031",
				TaskType:        domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:      domain.TaskStatusPendingAuditA,
				BusinessLane:    domain.TaskBusinessLaneNormal,
				OwnerDepartment: string(domain.DepartmentOperations),
				OwnerOrgTeam:    "淘系一组",
			},
		},
	}
	auditRepo := &auditV7RepoStub{
		handovers: []*domain.AuditHandover{
			{ID: 3101, TaskID: 31, FromAuditorID: 231, ToAuditorID: 232, Status: domain.HandoverStatusPendingTakeover},
		},
	}
	userRepo := newIdentityUserRepo()
	userRepo.users[311] = &domain.User{
		ID:                 311,
		Username:           "design_dept_admin",
		DisplayName:        "设计部门管理员",
		Department:         domain.DepartmentDesignRD,
		ManagedDepartments: []string{string(domain.DepartmentDesignRD)},
		Status:             domain.UserStatusActive,
	}
	userRepo.roles[311] = []domain.Role{domain.RoleDeptAdmin}
	svc := NewAuditV7Service(taskRepo, auditRepo, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7DataScopeResolver(NewRoleBasedDataScopeResolver()),
		WithAuditV7ScopeUserRepo(userRepo))
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       311,
		Username: "design_dept_admin",
		Roles:    []domain.Role{domain.RoleDeptAdmin},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})

	_, appErr := svc.ListHandovers(ctx, 31)
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("ListHandovers() appErr = %+v, want permission denied", appErr)
	}
	details, _ := appErr.Details.(map[string]interface{})
	if got := details["deny_code"]; got != "audit_handover_list_out_of_scope" {
		t.Fatalf("ListHandovers() deny_code = %v, want audit_handover_list_out_of_scope", got)
	}
}

func TestAuditV7ServiceTakeoverRejectsCrossLaneHandover(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			24: {
				ID:           24,
				TaskNo:       "RW-024",
				TaskType:     domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:   domain.TaskStatusPendingAuditA,
				BusinessLane: domain.TaskBusinessLaneNormal,
			},
		},
	}
	auditRepo := &auditV7RepoStub{
		handovers: []*domain.AuditHandover{
			{
				ID:            2401,
				TaskID:        24,
				FromAuditorID: 241,
				ToAuditorID:   242,
				Status:        domain.HandoverStatusPendingTakeover,
			},
		},
	}
	userRepo := newIdentityUserRepo()
	userRepo.users[241] = &domain.User{ID: 241, DisplayName: "马雨琪", Status: domain.UserStatusActive}
	userRepo.roles[241] = []domain.Role{domain.RoleAuditA}
	userRepo.users[242] = &domain.User{ID: 242, DisplayName: "章鹏鹏", Status: domain.UserStatusActive}
	userRepo.roles[242] = []domain.Role{domain.RoleAuditA}
	svc := NewAuditV7Service(taskRepo, auditRepo, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7ScopeUserRepo(userRepo))

	appErr := svc.Takeover(context.Background(), 24, 2401, 242)
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Takeover() appErr = %+v, want permission denied", appErr)
	}
}

func auditHandoverTestActorContext(actorID int64, role domain.Role) context.Context {
	return domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       actorID,
		Roles:    []domain.Role{role},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
}

func auditHandoverTask(id int64, taskNo string, handlerID int64, status domain.TaskStatus) *domain.Task {
	return &domain.Task{
		ID:                  id,
		TaskNo:              taskNo,
		SKUCode:             "SKU",
		TaskType:            domain.TaskTypeOriginalProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		ProductNameSnapshot: "候选任务",
		TaskStatus:          status,
		CurrentHandlerID:    int64Ptr(handlerID),
		BusinessLane:        domain.TaskBusinessLaneNormal,
		Priority:            domain.TaskPriorityNormal,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}
}

func auditHandoverCandidateListItem(id int64, taskNo string, handlerID int64, status domain.TaskStatus, lane domain.TaskBusinessLane, updatedAt time.Time) *domain.TaskListItem {
	return &domain.TaskListItem{
		ID:                  id,
		TaskNo:              taskNo,
		SKUCode:             "SKU",
		PrimarySKUCode:      "SKU",
		ProductNameSnapshot: "候选任务",
		TaskType:            domain.TaskTypeOriginalProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		TaskStatus:          status,
		CurrentHandlerID:    int64Ptr(handlerID),
		CurrentHandlerName:  "当前审核",
		BusinessLane:        lane,
		WorkflowLane:        lane.WorkflowLane(),
		Priority:            domain.TaskPriorityNormal,
		UpdatedAt:           updatedAt,
	}
}

type auditV7RepoStub struct {
	records   []*domain.AuditRecord
	handovers []*domain.AuditHandover
}

type auditExperienceServiceStub struct {
	events       []*domain.ExperienceOutboxEvent
	enqueueCalls int
	failEnqueue  bool
}

func (s *auditExperienceServiceStub) RuntimeFlags() domain.ExperienceRuntimeFlags {
	return domain.ExperienceRuntimeFlags{
		UIEnabled:              true,
		CaptureEnabled:         true,
		WorkerEnabled:          true,
		BehaviorCaptureEnabled: true,
		BehaviorSampleRate:     1,
	}
}

func (s *auditExperienceServiceStub) ClientConfig() domain.ExperienceClientConfig {
	return domain.ExperienceClientConfig{}
}

func (s *auditExperienceServiceStub) ListReasonTags(context.Context, string) ([]*domain.ExperienceReasonTag, *domain.AppError) {
	return nil, nil
}

func (s *auditExperienceServiceStub) ListClientReasonTags(context.Context, string) ([]*domain.ExperienceClientReasonTag, *domain.AppError) {
	return nil, nil
}

func (s *auditExperienceServiceStub) ListSamples(context.Context, ExperienceEventFilter) ([]*domain.ExperienceEvent, domain.PaginationMeta, *domain.AppError) {
	return nil, domain.PaginationMeta{}, nil
}

func (s *auditExperienceServiceStub) ListReviewItems(context.Context, ExperienceReviewItemFilter) ([]*domain.ExperienceReviewItem, domain.PaginationMeta, *domain.AppError) {
	return nil, domain.PaginationMeta{}, nil
}

func (s *auditExperienceServiceStub) Stats(context.Context) (*domain.ExperienceStats, *domain.AppError) {
	return &domain.ExperienceStats{}, nil
}

func (s *auditExperienceServiceStub) EnqueueEvent(_ context.Context, event *domain.ExperienceOutboxEvent) *domain.AppError {
	s.enqueueCalls++
	if event != nil {
		copied := *event
		s.events = append(s.events, &copied)
	}
	if s.failEnqueue {
		return domain.NewAppError(domain.ErrCodeInternalError, "stub enqueue failed", nil)
	}
	return nil
}

func (s *auditExperienceServiceStub) RecordAISuggestionEvent(context.Context, *domain.AISuggestionEvent) *domain.AppError {
	return nil
}

func (s *auditExperienceServiceStub) RecordBehaviorEvents(context.Context, domain.RequestActor, ExperienceBehaviorBatchRequest) (ExperienceBehaviorBatchResult, *domain.AppError) {
	return ExperienceBehaviorBatchResult{}, nil
}

func (s *auditExperienceServiceStub) RecordAISuggestionFeedback(context.Context, domain.RequestActor, AISuggestionFeedbackRequest) (*domain.AISuggestionFeedback, *domain.AppError) {
	return nil, nil
}

func (s *auditExperienceServiceStub) MicroQuestionEligibility(context.Context, domain.RequestActor, ExperienceMicroQuestionEligibilityRequest) (*domain.ExperienceMicroQuestionEligibility, *domain.AppError) {
	return &domain.ExperienceMicroQuestionEligibility{}, nil
}

func (s *auditExperienceServiceStub) RecordMicroQuestionAnswer(context.Context, domain.RequestActor, ExperienceMicroQuestionAnswerRequest) (*domain.ExperienceMicroQuestionAnswer, *domain.AppError) {
	return &domain.ExperienceMicroQuestionAnswer{}, nil
}

func (s *auditExperienceServiceStub) RecordReviewDecision(context.Context, domain.RequestActor, string, ExperienceReviewDecisionRequest) (*domain.ExperienceReviewDecision, *domain.AppError) {
	return &domain.ExperienceReviewDecision{}, nil
}

func (s *auditExperienceServiceStub) ProcessOutcomeObservers(context.Context, int) (domain.ExperienceObserverRun, *domain.AppError) {
	return domain.ExperienceObserverRun{}, nil
}

func (s *auditExperienceServiceStub) ProcessOutbox(context.Context, int) (domain.ExperienceWorkerRun, *domain.AppError) {
	return domain.ExperienceWorkerRun{}, nil
}

func (s *auditExperienceServiceStub) ProcessAttributions(context.Context, int) (domain.ExperienceAttributionRun, *domain.AppError) {
	return domain.ExperienceAttributionRun{}, nil
}

func (s *auditExperienceServiceStub) ProcessRetention(context.Context, time.Time, int) (domain.ExperienceRetentionRun, *domain.AppError) {
	return domain.ExperienceRetentionRun{}, nil
}

func (s *auditExperienceServiceStub) ReserveRateLimit(context.Context, domain.RequestActor, string, time.Time, time.Time, int) (*domain.ExperienceRateLimitReservation, *domain.AppError) {
	return nil, nil
}

func (r *auditV7RepoStub) CreateRecord(_ context.Context, _ repo.Tx, record *domain.AuditRecord) (int64, error) {
	copyRecord := *record
	copyRecord.ID = int64(len(r.records) + 1)
	r.records = append(r.records, &copyRecord)
	return copyRecord.ID, nil
}

func (r *auditV7RepoStub) ListRecordsByTaskID(_ context.Context, taskID int64) ([]*domain.AuditRecord, error) {
	items := []*domain.AuditRecord{}
	for _, record := range r.records {
		if record != nil && record.TaskID == taskID {
			items = append(items, record)
		}
	}
	return items, nil
}

func (r *auditV7RepoStub) ListRecords(_ context.Context, _ repo.AuditRecordListFilter) ([]*domain.AuditRecord, error) {
	items := make([]*domain.AuditRecord, 0, len(r.records))
	for _, record := range r.records {
		if record != nil {
			items = append(items, record)
		}
	}
	return items, nil
}

func (r *auditV7RepoStub) CreateHandover(_ context.Context, _ repo.Tx, handover *domain.AuditHandover) (int64, error) {
	copyHandover := *handover
	copyHandover.ID = int64(len(r.handovers) + 1)
	r.handovers = append(r.handovers, &copyHandover)
	return copyHandover.ID, nil
}

func (r *auditV7RepoStub) GetHandoverByID(_ context.Context, id int64) (*domain.AuditHandover, error) {
	for _, handover := range r.handovers {
		if handover != nil && handover.ID == id {
			return handover, nil
		}
	}
	return nil, nil
}

func (r *auditV7RepoStub) ListHandoversByTaskID(_ context.Context, taskID int64) ([]*domain.AuditHandover, error) {
	items := []*domain.AuditHandover{}
	for _, handover := range r.handovers {
		if handover != nil && handover.TaskID == taskID {
			items = append(items, handover)
		}
	}
	return items, nil
}

func (r *auditV7RepoStub) UpdateHandoverStatus(_ context.Context, _ repo.Tx, id int64, status domain.HandoverStatus) error {
	for _, handover := range r.handovers {
		if handover != nil && handover.ID == id {
			handover.Status = status
			return nil
		}
	}
	return nil
}
