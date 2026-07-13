package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestTaskAssetCenterServiceAuditSupplementReadAllowsPeerAuditors(t *testing.T) {
	tests := []struct {
		name         string
		task         *domain.Task
		actor        domain.RequestActor
		otherAuditor int64
	}{
		{
			name: "left_reads_ma_yuqi_task",
			task: &domain.Task{
				ID:              2293,
				TaskNo:          "RW-20260712-A-002290",
				TaskStatus:      domain.TaskStatusCompleted,
				BusinessLane:    domain.TaskBusinessLaneNormal,
				OwnerDepartment: string(domain.DepartmentOperations),
				OwnerOrgTeam:    "淘系运营二部",
			},
			actor: domain.RequestActor{
				ID:         331,
				Username:   "左取名",
				Roles:      []domain.Role{domain.RoleAuditA, domain.RoleCustomizationReviewer},
				Department: string(domain.DepartmentCustomizationArt),
				Team:       "全职组",
				Source:     domain.RequestActorSourceSessionToken,
				AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
			},
			otherAuditor: 242,
		},
		{
			name: "ma_yuqi_reads_left_task",
			task: &domain.Task{
				ID:              2317,
				TaskNo:          "RW-20260713-A-002314",
				TaskStatus:      domain.TaskStatusCompleted,
				BusinessLane:    domain.TaskBusinessLaneNormal,
				OwnerDepartment: string(domain.DepartmentOperations),
				OwnerOrgTeam:    "天猫运营一部（池州）",
			},
			actor: domain.RequestActor{
				ID:         242,
				Username:   "马雨琪",
				Roles:      []domain.Role{domain.RoleAuditA, domain.RoleAuditB, domain.RoleDeptAdmin},
				Department: string(domain.DepartmentCustomizationArt),
				Source:     domain.RequestActorSourceSessionToken,
				AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
			},
			otherAuditor: 331,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskRepo := newStep04TaskRepo(tt.task)
			eventRepo := &step04TaskEventRepo{}
			auditRepo := &auditV7RepoStub{records: []*domain.AuditRecord{{
				TaskID:    tt.task.ID,
				AuditorID: tt.otherAuditor,
				Stage:     domain.AuditRecordStageA,
				Action:    domain.AuditActionTypeApprove,
			}}}
			svc := NewTaskAssetCenterService(taskRepo, nil, nil, nil, nil, eventRepo, step04TxRunner{}, nil,
				WithTaskAssetCenterDataScopeResolver(NewRoleBasedDataScopeResolver()),
				WithTaskAssetCenterAuditRepo(auditRepo),
			).(*taskAssetCenterService)
			ctx := domain.WithRequestActor(context.Background(), tt.actor)

			items, appErr := svc.ListAuditSupplements(ctx, tt.task.ID)
			if appErr != nil {
				t.Fatalf("ListAuditSupplements() unexpected error: %+v", appErr)
			}
			if len(items) != 0 {
				t.Fatalf("ListAuditSupplements() items = %+v, want empty list", items)
			}
		})
	}
}

func TestTaskAssetCenterServiceAuditSupplementWriteStillRequiresHistoryOrOrgScope(t *testing.T) {
	task := &domain.Task{
		ID:              2317,
		TaskNo:          "RW-20260713-A-002314",
		TaskStatus:      domain.TaskStatusCompleted,
		BusinessLane:    domain.TaskBusinessLaneNormal,
		OwnerDepartment: string(domain.DepartmentOperations),
		OwnerOrgTeam:    "天猫运营一部（池州）",
	}
	auditRepo := &auditV7RepoStub{records: []*domain.AuditRecord{{
		TaskID:    task.ID,
		AuditorID: 331,
		Stage:     domain.AuditRecordStageA,
		Action:    domain.AuditActionTypeApprove,
	}}}
	svc := NewTaskAssetCenterService(newStep04TaskRepo(task), nil, nil, nil, nil, &step04TaskEventRepo{}, step04TxRunner{}, nil,
		WithTaskAssetCenterAuditRepo(auditRepo),
	).(*taskAssetCenterService)
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         242,
		Username:   "马雨琪",
		Roles:      []domain.Role{domain.RoleAuditA, domain.RoleAuditB, domain.RoleDeptAdmin},
		Department: string(domain.DepartmentCustomizationArt),
		Source:     domain.RequestActorSourceSessionToken,
		AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
	})

	appErr := svc.authorizeAuditSupplementWrite(ctx, task)
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("authorizeAuditSupplementWrite() error = %+v, want permission denied", appErr)
	}
	details, _ := appErr.Details.(map[string]interface{})
	if got := details["deny_code"]; got != "audit_supplement_scope_denied" {
		t.Fatalf("deny_code = %v, want audit_supplement_scope_denied", got)
	}
}
