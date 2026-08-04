package service

import (
	"context"
	"slices"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestAuditV7ServiceListHandoverCandidatesScopesToCurrentActor(t *testing.T) {
	actorID := int64(331)
	otherHandlerID := int64(332)
	now := time.Now().UTC()
	taskRepo := &prdTaskRepo{
		listItems: []*domain.TaskListItem{
			auditHandoverCandidateListItem(41, "RW-41", actorID, domain.TaskStatusPendingAudit, domain.TaskBusinessLaneNormal, now),
			auditHandoverCandidateListItem(42, "RW-42", actorID, domain.TaskStatusPendingAudit, domain.TaskBusinessLaneNormal, now),
			auditHandoverCandidateListItem(43, "RW-43", otherHandlerID, domain.TaskStatusPendingAudit, domain.TaskBusinessLaneNormal, now),
			auditHandoverCandidateListItem(44, "RW-44", actorID, domain.TaskStatusPendingAudit, domain.TaskBusinessLaneCustomization, now),
		},
	}
	svc := NewAuditV7Service(taskRepo, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{})

	resp, appErr := svc.ListHandoverCandidates(auditHandoverTestActorContext(actorID, domain.RoleAuditA), AuditHandoverCandidateFilter{})
	if appErr != nil {
		t.Fatalf("ListHandoverCandidates() unexpected error: %+v", appErr)
	}
	if resp.EligibleCount != 3 || len(resp.Items) != 3 {
		t.Fatalf("candidate count = eligible %d len %d, want 3 including customization", resp.EligibleCount, len(resp.Items))
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

func TestAuditV7ServiceListHandoverCandidatesRejectsLegacyAuditRoleWithoutEffectiveCapability(t *testing.T) {
	svc := NewAuditV7Service(&prdTaskRepo{}, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{})
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 331, Roles: []domain.Role{domain.RoleAuditA}})

	_, appErr := svc.ListHandoverCandidates(ctx, AuditHandoverCandidateFilter{})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("ListHandoverCandidates() appErr = %+v, want explicit capability denial", appErr)
	}
}

func TestAuditV7ServiceListHandoverCandidatesExcludesLegacyAuditStates(t *testing.T) {
	actorID := int64(331)
	now := time.Now().UTC()
	taskRepo := &prdTaskRepo{listItems: []*domain.TaskListItem{
		auditHandoverCandidateListItem(41, "RW-41", actorID, domain.TaskStatusPendingAuditA, domain.TaskBusinessLaneNormal, now),
		auditHandoverCandidateListItem(42, "RW-42", actorID, domain.TaskStatusPendingAuditB, domain.TaskBusinessLaneCustomization, now),
	}}
	svc := NewAuditV7Service(taskRepo, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{})

	resp, appErr := svc.ListHandoverCandidates(auditHandoverTestActorContext(actorID, domain.RoleAuditA), AuditHandoverCandidateFilter{})
	if appErr != nil {
		t.Fatalf("ListHandoverCandidates() unexpected error: %+v", appErr)
	}
	if resp.EligibleCount != 0 || len(resp.Items) != 0 {
		t.Fatalf("legacy candidate result = %+v, want empty", resp)
	}
}

func TestAuditV7ServiceListHandoverCandidatesPushesStableDepartmentScope(t *testing.T) {
	actorID := int64(331)
	departmentID := int64(88)
	access := auditV8EffectiveAccess(actorID, 9003, domain.AccessScopeOwnDepartment)
	actor := domain.RequestActor{ID: actorID, DepartmentID: &departmentID, EffectiveAccess: access, Permissions: access.Permissions}
	taskRepo := &prdTaskRepo{}
	svc := NewAuditV7Service(taskRepo, &auditV7RepoStub{}, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{})

	if _, appErr := svc.ListHandoverCandidates(domain.WithRequestActor(context.Background(), actor), AuditHandoverCandidateFilter{}); appErr != nil {
		t.Fatalf("ListHandoverCandidates() unexpected error: %+v", appErr)
	}
	if len(taskRepo.lastListFilter.ScopeDepartmentIDs) != 1 || taskRepo.lastListFilter.ScopeDepartmentIDs[0] != departmentID {
		t.Fatalf("ScopeDepartmentIDs = %v, want [%d]", taskRepo.lastListFilter.ScopeDepartmentIDs, departmentID)
	}
}

func TestAuditV7ServiceV8HandoverAndTakeoverSupportSelfScopedTarget(t *testing.T) {
	const (
		taskID   = int64(510)
		fromID   = int64(511)
		targetID = int64(512)
	)
	departmentID := int64(77)
	taskRepo := &prdTaskRepo{tasks: map[int64]*domain.Task{
		taskID: {
			ID: taskID, TaskNo: "RW-510", TaskType: domain.TaskTypeOriginalProductDevelopment,
			TaskStatus: domain.TaskStatusPendingAudit, WorkflowRevision: 9,
			CurrentHandlerID: int64Ptr(fromID), OwnerDepartmentID: &departmentID,
		},
	}}
	auditRepo := &auditV7RepoStub{}
	eventRepo := &prdTaskEventRepo{}
	userRepo := newIdentityUserRepo()
	userRepo.users[targetID] = &domain.User{ID: targetID, Status: domain.UserStatusActive}
	targetAccess := auditV8EffectiveAccess(targetID, 9102, domain.AccessScopeSelf)
	svc := NewAuditV7Service(taskRepo, auditRepo, eventRepo, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7ScopeUserRepo(userRepo),
		WithAuditV8EffectiveAccessResolver(auditV8AccessResolverStub{byUser: map[int64]*domain.EffectiveAccess{targetID: targetAccess}}))
	fromAccess := auditV8EffectiveAccess(fromID, 9101, domain.AccessScopeOwnDepartment)
	fromActor := domain.RequestActor{ID: fromID, DepartmentID: &departmentID, EffectiveAccess: fromAccess, Permissions: fromAccess.Permissions}

	handover, appErr := svc.Handover(domain.WithRequestActor(context.Background(), fromActor), HandoverAuditParams{
		TaskID: taskID, FromAuditorID: fromID, ToAuditorID: targetID, Reason: "轮班交接",
	})
	if appErr != nil {
		t.Fatalf("Handover() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[taskID].CurrentHandlerID != nil || len(auditRepo.records) != 1 || auditRepo.records[0].Stage != domain.AuditRecordStageUnified {
		t.Fatalf("handover state task=%+v records=%+v", taskRepo.tasks[taskID], auditRepo.records)
	}

	targetActor := domain.RequestActor{ID: targetID, EffectiveAccess: targetAccess, Permissions: targetAccess.Permissions}
	targetCtx := domain.WithRequestActor(context.Background(), targetActor)
	listed, appErr := svc.ListHandovers(targetCtx, taskID)
	if appErr != nil || len(listed) != 1 || !slices.Contains(listed[0].AllowedActions, "task.audit.takeover") {
		t.Fatalf("target ListHandovers() = %+v err=%+v, want item takeover action", listed, appErr)
	}

	otherAccess := auditV8EffectiveAccess(513, 9103, domain.AccessScopeGlobal)
	otherCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 513, EffectiveAccess: otherAccess, Permissions: otherAccess.Permissions})
	listed, appErr = svc.ListHandovers(otherCtx, taskID)
	if appErr != nil || len(listed) != 1 || len(listed[0].AllowedActions) != 0 {
		t.Fatalf("non-target ListHandovers() = %+v err=%+v, want no item action", listed, appErr)
	}

	if appErr := svc.Takeover(targetCtx, taskID, handover.ID, targetID); appErr != nil {
		t.Fatalf("Takeover() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[taskID].CurrentHandlerID == nil || *taskRepo.tasks[taskID].CurrentHandlerID != targetID || len(auditRepo.records) != 2 || len(eventRepo.events) != 2 {
		t.Fatalf("takeover state task=%+v records=%d events=%d", taskRepo.tasks[taskID], len(auditRepo.records), len(eventRepo.events))
	}
	listed, appErr = svc.ListHandovers(targetCtx, taskID)
	if appErr != nil || len(listed) != 1 || len(listed[0].AllowedActions) != 0 {
		t.Fatalf("completed handover actions = %+v err=%+v, want none", listed, appErr)
	}
}

func TestAuditV7ServiceV8HandoverRequiresCurrentHandler(t *testing.T) {
	const (
		taskID    = int64(520)
		actorID   = int64(521)
		handlerID = int64(522)
	)
	access := auditV8EffectiveAccess(actorID, 9201, domain.AccessScopeGlobal)
	taskRepo := &prdTaskRepo{tasks: map[int64]*domain.Task{taskID: {ID: taskID, TaskStatus: domain.TaskStatusPendingAudit, CurrentHandlerID: int64Ptr(handlerID)}}}
	auditRepo := &auditV7RepoStub{}
	eventRepo := &prdTaskEventRepo{}
	svc := NewAuditV7Service(taskRepo, auditRepo, eventRepo, prdCodeRuleService{}, step04TxRunner{})
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: actorID, EffectiveAccess: access, Permissions: access.Permissions})

	_, appErr := svc.Handover(ctx, HandoverAuditParams{TaskID: taskID, FromAuditorID: actorID, ToAuditorID: 523, Reason: "越权交班"})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied || len(auditRepo.handovers) != 0 || len(auditRepo.records) != 0 || len(eventRepo.events) != 0 || taskRepo.tasks[taskID].CurrentHandlerID == nil || *taskRepo.tasks[taskID].CurrentHandlerID != handlerID {
		t.Fatalf("Handover() err=%+v task=%+v handovers=%d records=%d events=%d", appErr, taskRepo.tasks[taskID], len(auditRepo.handovers), len(auditRepo.records), len(eventRepo.events))
	}
}

func TestAuditV7ServiceV8HandoverRejectsConcurrentHandlerChange(t *testing.T) {
	const (
		taskID       = int64(530)
		actorID      = int64(531)
		targetID     = int64(532)
		newHandlerID = int64(533)
	)
	access := auditV8EffectiveAccess(actorID, 9301, domain.AccessScopeGlobal)
	taskRepo := &prdTaskRepo{tasks: map[int64]*domain.Task{taskID: {ID: taskID, TaskStatus: domain.TaskStatusPendingAudit, WorkflowRevision: 4, CurrentHandlerID: int64Ptr(actorID)}}}
	taskRepo.getForUpdateHook = func(task *domain.Task) { task.CurrentHandlerID = int64Ptr(newHandlerID); task.WorkflowRevision++ }
	auditRepo := &auditV7RepoStub{}
	eventRepo := &prdTaskEventRepo{}
	userRepo := newIdentityUserRepo()
	userRepo.users[targetID] = &domain.User{ID: targetID, Status: domain.UserStatusActive}
	targetAccess := auditV8EffectiveAccess(targetID, 9302, domain.AccessScopeSelf)
	svc := NewAuditV7Service(taskRepo, auditRepo, eventRepo, prdCodeRuleService{}, step04TxRunner{}, WithAuditV7ScopeUserRepo(userRepo), WithAuditV8EffectiveAccessResolver(auditV8AccessResolverStub{byUser: map[int64]*domain.EffectiveAccess{targetID: targetAccess}}))
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: actorID, EffectiveAccess: access, Permissions: access.Permissions})

	_, appErr := svc.Handover(ctx, HandoverAuditParams{TaskID: taskID, FromAuditorID: actorID, ToAuditorID: targetID, Reason: "并发交班"})
	if appErr == nil || appErr.Code != domain.ErrCodeConflict || len(auditRepo.handovers) != 0 || len(auditRepo.records) != 0 || len(eventRepo.events) != 0 || taskRepo.tasks[taskID].CurrentHandlerID == nil || *taskRepo.tasks[taskID].CurrentHandlerID != newHandlerID {
		t.Fatalf("Handover() err=%+v task=%+v handovers=%d records=%d events=%d", appErr, taskRepo.tasks[taskID], len(auditRepo.handovers), len(auditRepo.records), len(eventRepo.events))
	}
}

func TestAuditV7ServiceV8TakeoverRejectsConcurrentTaskOrHandoverChange(t *testing.T) {
	const (
		taskID     = int64(540)
		targetID   = int64(542)
		handoverID = int64(5401)
	)
	access := auditV8EffectiveAccess(targetID, 9401, domain.AccessScopeSelf)
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: targetID, EffectiveAccess: access, Permissions: access.Permissions})
	for _, tc := range []struct {
		name       string
		mutateTask bool
		forceCAS   bool
	}{
		{name: "task revision changed", mutateTask: true},
		{name: "handover CAS lost", forceCAS: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			taskRepo := &prdTaskRepo{tasks: map[int64]*domain.Task{taskID: {ID: taskID, TaskStatus: domain.TaskStatusPendingAudit, WorkflowRevision: 7}}}
			if tc.mutateTask {
				taskRepo.getForUpdateHook = func(task *domain.Task) { task.TaskStatus = domain.TaskStatusInProgress; task.WorkflowRevision++ }
			}
			auditRepo := &auditV7RepoStub{forceTakeoverCASMiss: tc.forceCAS, handovers: []*domain.AuditHandover{{ID: handoverID, TaskID: taskID, ToAuditorID: targetID, Status: domain.HandoverStatusPendingTakeover}}}
			eventRepo := &prdTaskEventRepo{}
			svc := NewAuditV7Service(taskRepo, auditRepo, eventRepo, prdCodeRuleService{}, step04TxRunner{})

			appErr := svc.Takeover(ctx, taskID, handoverID, targetID)
			if appErr == nil || appErr.Code != domain.ErrCodeConflict || len(auditRepo.records) != 0 || len(eventRepo.events) != 0 || taskRepo.tasks[taskID].CurrentHandlerID != nil || auditRepo.handovers[0].Status != domain.HandoverStatusPendingTakeover {
				t.Fatalf("Takeover() err=%+v task=%+v handover=%+v records=%d events=%d", appErr, taskRepo.tasks[taskID], auditRepo.handovers[0], len(auditRepo.records), len(eventRepo.events))
			}
		})
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
			41: auditHandoverTask(41, "RW-41", actorID, domain.TaskStatusPendingAudit),
			42: auditHandoverTask(42, "RW-42", actorID, domain.TaskStatusInProgress),
		},
		listItems: []*domain.TaskListItem{
			auditHandoverCandidateListItem(41, "RW-41", actorID, domain.TaskStatusPendingAudit, domain.TaskBusinessLaneNormal, now),
			auditHandoverCandidateListItem(42, "RW-42", actorID, domain.TaskStatusPendingAudit, domain.TaskBusinessLaneNormal, now),
		},
	}
	auditRepo := &auditV7RepoStub{}
	userRepo := newIdentityUserRepo()
	userRepo.users[toAuditorID] = &domain.User{ID: toAuditorID, Status: domain.UserStatusActive}
	targetAccess := auditV8EffectiveAccess(toAuditorID, 9002, domain.AccessScopeSelf)
	svc := NewAuditV7Service(taskRepo, auditRepo, &prdTaskEventRepo{}, prdCodeRuleService{}, step04TxRunner{},
		WithAuditV7ScopeUserRepo(userRepo),
		WithAuditV8EffectiveAccessResolver(auditV8AccessResolverStub{byUser: map[int64]*domain.EffectiveAccess{toAuditorID: targetAccess}}))

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

func auditHandoverTestActorContext(actorID int64, role domain.Role) context.Context {
	actor := domain.RequestActor{
		ID:       actorID,
		Roles:    []domain.Role{role},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}
	if role == domain.RoleAuditA || role == domain.RoleAuditB {
		actor.EffectiveAccess = auditV8EffectiveAccess(actorID, 9001, domain.AccessScopeGlobal)
		actor.Permissions = actor.EffectiveAccess.Permissions
	}
	return domain.WithRequestActor(context.Background(), actor)
}

func auditV8EffectiveAccess(userID, roleID int64, scope domain.AccessScopeMode) *domain.EffectiveAccess {
	return &domain.EffectiveAccess{
		UserID:      userID,
		Permissions: []domain.PermissionCode{domain.PermissionTaskAuditDecision, domain.PermissionTaskView},
		Assignments: []domain.AccessAssignment{{UserID: userID, RoleID: roleID, ScopeMode: scope, Subjects: []domain.AccessScopeSubject{}}},
		Sources: []domain.EffectiveAccessNote{
			{Permission: domain.PermissionTaskAuditDecision, RoleID: roleID, ScopeMode: scope, SourceType: "direct"},
			{Permission: domain.PermissionTaskView, RoleID: roleID, ScopeMode: scope, SourceType: "direct"},
		},
	}
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
	records              []*domain.AuditRecord
	handovers            []*domain.AuditHandover
	forceTakeoverCASMiss bool
}

type auditV8AccessResolverStub struct {
	byUser map[int64]*domain.EffectiveAccess
}

func (r auditV8AccessResolverStub) EffectiveAccess(_ context.Context, userID int64) (*domain.EffectiveAccess, *domain.AppError) {
	return r.byUser[userID], nil
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

func (r *auditV7RepoStub) GetHandoverByIDForUpdate(ctx context.Context, _ repo.Tx, id int64) (*domain.AuditHandover, error) {
	return r.GetHandoverByID(ctx, id)
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

func (r *auditV7RepoStub) CASUpdateHandoverStatus(_ context.Context, _ repo.Tx, id int64, expected, next domain.HandoverStatus) (bool, error) {
	if r.forceTakeoverCASMiss {
		return false, nil
	}
	for _, handover := range r.handovers {
		if handover != nil && handover.ID == id && handover.Status == expected {
			handover.Status = next
			return true, nil
		}
	}
	return false, nil
}
