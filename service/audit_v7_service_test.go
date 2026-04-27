package service

import (
	"context"
	"testing"

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

type auditV7RepoStub struct {
	records   []*domain.AuditRecord
	handovers []*domain.AuditHandover
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
