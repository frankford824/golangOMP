package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestTaskActionAuthorizerEvaluatePolicy(t *testing.T) {
	designerID := int64(11)
	otherHandlerID := int64(99)
	cases := []struct {
		name            string
		action          TaskAction
		attrs           TaskActionAttributes
		actor           domain.RequestActor
		task            *domain.Task
		wantAllowed     bool
		wantDenyCode    string
		wantScopeSource string
	}{
		{
			name:   "view_all_allow",
			action: TaskActionAssign,
			attrs:  TaskActionAttributes{},
			actor:  domain.RequestActor{ID: 1, Roles: []domain.Role{domain.RoleAdmin}},
			task: &domain.Task{
				ID:              1,
				TaskStatus:      domain.TaskStatusPendingAssign,
				OwnerDepartment: "ops",
				OwnerOrgTeam:    "ops-team-1",
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeViewAll),
		},
		{
			name:   "department_scope_allow",
			action: TaskActionAssign,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:         2,
				Roles:      []domain.Role{domain.RoleDeptAdmin},
				Department: "ops",
			},
			task: &domain.Task{
				ID:              2,
				TaskStatus:      domain.TaskStatusPendingAssign,
				OwnerDepartment: "ops",
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeDepartment),
		},
		{
			name:   "department_scope_deny",
			action: TaskActionAssign,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:         3,
				Roles:      []domain.Role{domain.RoleDeptAdmin},
				Department: "design",
			},
			task: &domain.Task{
				ID:              3,
				TaskStatus:      domain.TaskStatusPendingAssign,
				OwnerDepartment: "ops",
			},
			wantAllowed:  false,
			wantDenyCode: "task_out_of_department_scope",
		},
		{
			name:   "team_scope_allow",
			action: TaskActionAssign,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:    4,
				Roles: []domain.Role{domain.RoleTeamLead},
				Team:  "ops-team-1",
			},
			task: &domain.Task{
				ID:              4,
				TaskStatus:      domain.TaskStatusPendingAssign,
				OwnerDepartment: "ops",
				OwnerOrgTeam:    "ops-team-1",
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeTeam),
		},
		{
			name:   "team_lead_cannot_use_department_scope",
			action: TaskActionAssign,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:         40,
				Roles:      []domain.Role{domain.RoleTeamLead},
				Department: "ops",
				Team:       "ops-team-3",
			},
			task: &domain.Task{
				ID:              4,
				TaskStatus:      domain.TaskStatusPendingAssign,
				OwnerDepartment: "ops",
				OwnerOrgTeam:    "ops-team-1",
			},
			wantAllowed:  false,
			wantDenyCode: "task_out_of_team_scope",
		},
		{
			name:   "handler_allow",
			action: TaskActionSubmitDesign,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:    11,
				Roles: []domain.Role{domain.RoleDesigner},
			},
			task: &domain.Task{
				ID:               5,
				TaskStatus:       domain.TaskStatusInProgress,
				OwnerDepartment:  "ops",
				OwnerOrgTeam:     "ops-team-1",
				DesignerID:       &designerID,
				CurrentHandlerID: &designerID,
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeHandler),
		},
		{
			name:   "handler_deny",
			action: TaskActionSubmitDesign,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:         12,
				Roles:      []domain.Role{domain.RoleDesigner},
				Department: "design",
				Team:       "design-team-1",
			},
			task: &domain.Task{
				ID:               6,
				TaskStatus:       domain.TaskStatusInProgress,
				OwnerDepartment:  "ops",
				OwnerOrgTeam:     "ops-team-1",
				DesignerID:       &designerID,
				CurrentHandlerID: &otherHandlerID,
			},
			wantAllowed:  false,
			wantDenyCode: "task_not_assigned_to_actor",
		},
		{
			name:   "missing_role_deny",
			action: TaskActionAssign,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:    13,
				Roles: []domain.Role{domain.RoleMember},
			},
			task: &domain.Task{
				ID:              7,
				TaskStatus:      domain.TaskStatusPendingAssign,
				OwnerDepartment: "ops",
			},
			wantAllowed:  false,
			wantDenyCode: "missing_required_role",
		},
		{
			name:   "status_deny",
			action: TaskActionAssign,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:         14,
				Roles:      []domain.Role{domain.RoleOps},
				Department: "ops",
			},
			task: &domain.Task{
				ID:              8,
				TaskStatus:      domain.TaskStatusInProgress,
				OwnerDepartment: "ops",
			},
			wantAllowed:  false,
			wantDenyCode: "task_status_not_actionable",
		},
		{
			name:   "reassign_allow_for_team_lead_in_scope",
			action: TaskActionReassign,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:    15,
				Roles: []domain.Role{domain.RoleTeamLead},
				Team:  "ops-team-1",
			},
			task: &domain.Task{
				ID:               9,
				TaskStatus:       domain.TaskStatusInProgress,
				OwnerDepartment:  "ops",
				OwnerOrgTeam:     "ops-team-1",
				DesignerID:       &designerID,
				CurrentHandlerID: &designerID,
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeTeam),
		},
		{
			name:   "reassign_deny_for_ops_without_manager_scope",
			action: TaskActionReassign,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:         16,
				Roles:      []domain.Role{domain.RoleOps},
				Department: "ops",
			},
			task: &domain.Task{
				ID:               10,
				TaskStatus:       domain.TaskStatusInProgress,
				OwnerDepartment:  "ops",
				OwnerOrgTeam:     "ops-team-1",
				DesignerID:       &designerID,
				CurrentHandlerID: &designerID,
			},
			wantAllowed:     false,
			wantDenyCode:    "task_reassign_requires_requester_or_manager",
			wantScopeSource: string(TaskActionScopeDepartment),
		},
		{
			name:   "reassign_status_deny_for_pending_audit",
			action: TaskActionReassign,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:         17,
				Roles:      []domain.Role{domain.RoleDeptAdmin},
				Department: "ops",
			},
			task: &domain.Task{
				ID:               11,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				OwnerDepartment:  "ops",
				OwnerOrgTeam:     "ops-team-1",
				DesignerID:       &designerID,
				CurrentHandlerID: &designerID,
			},
			wantAllowed:  false,
			wantDenyCode: "task_not_reassignable",
		},
		{
			name:   "audit_a_allow_for_current_handler",
			action: TaskActionAuditApprove,
			attrs:  TaskActionAttributes{AuditStage: domain.AuditRecordStageA},
			actor: domain.RequestActor{
				ID:    21,
				Roles: []domain.Role{domain.RoleAuditA},
			},
			task: &domain.Task{
				ID:               12,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				OwnerDepartment:  "ops",
				OwnerOrgTeam:     "ops-team-1",
				CurrentHandlerID: authzInt64Ptr(21),
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeHandler),
		},
		{
			name:   "audit_b_cannot_operate_a_stage",
			action: TaskActionAuditApprove,
			attrs:  TaskActionAttributes{AuditStage: domain.AuditRecordStageA},
			actor: domain.RequestActor{
				ID:    22,
				Roles: []domain.Role{domain.RoleAuditB},
			},
			task: &domain.Task{
				ID:               13,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				OwnerDepartment:  "ops",
				OwnerOrgTeam:     "ops-team-1",
				CurrentHandlerID: authzInt64Ptr(22),
			},
			wantAllowed:  false,
			wantDenyCode: "missing_required_role",
		},
		{
			name:   "audit_stage_mismatch_denied",
			action: TaskActionAuditApprove,
			attrs:  TaskActionAttributes{AuditStage: domain.AuditRecordStageB},
			actor: domain.RequestActor{
				ID:    23,
				Roles: []domain.Role{domain.RoleAuditA},
			},
			task: &domain.Task{
				ID:               14,
				TaskStatus:       domain.TaskStatusPendingAuditA,
				OwnerDepartment:  "ops",
				OwnerOrgTeam:     "ops-team-1",
				CurrentHandlerID: authzInt64Ptr(23),
			},
			wantAllowed:  false,
			wantDenyCode: "audit_stage_mismatch",
		},
		{
			name:   "warehouse_receive_denies_takeover_without_handler_match",
			action: TaskActionWarehouseReceive,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:    24,
				Roles: []domain.Role{domain.RoleWarehouse},
				Team:  "ops-team-1",
			},
			task: &domain.Task{
				ID:               15,
				TaskStatus:       domain.TaskStatusPendingWarehouseReceive,
				OwnerDepartment:  "ops",
				OwnerOrgTeam:     "ops-team-1",
				CurrentHandlerID: authzInt64Ptr(2400),
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeTeam),
		},
		{
			name:   "warehouse_complete_requires_current_handler",
			action: TaskActionWarehouseComplete,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:    25,
				Roles: []domain.Role{domain.RoleWarehouse},
				Team:  "ops-team-1",
			},
			task: &domain.Task{
				ID:              16,
				TaskStatus:      domain.TaskStatusPendingWarehouseReceive,
				OwnerDepartment: "ops",
				OwnerOrgTeam:    "ops-team-1",
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeTeam),
		},
		{
			name:   "close_status_still_denied_for_view_all",
			action: TaskActionClose,
			attrs:  TaskActionAttributes{},
			actor: domain.RequestActor{
				ID:    26,
				Roles: []domain.Role{domain.RoleAdmin},
			},
			task: &domain.Task{
				ID:              17,
				TaskStatus:      domain.TaskStatusInProgress,
				OwnerDepartment: "ops",
				OwnerOrgTeam:    "ops-team-1",
			},
			wantAllowed:  false,
			wantDenyCode: "task_not_closable",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := domain.WithRequestActor(context.Background(), tc.actor)
			decision := newTaskActionAuthorizer(NewRoleBasedDataScopeResolver(), nil).EvaluateTaskActionPolicyWithAttributes(ctx, tc.action, tc.task, "", "", tc.attrs)
			if decision.Allowed != tc.wantAllowed {
				t.Fatalf("Allowed = %v, want %v, decision=%+v", decision.Allowed, tc.wantAllowed, decision)
			}
			if decision.DenyCode != tc.wantDenyCode {
				t.Fatalf("DenyCode = %q, want %q, decision=%+v", decision.DenyCode, tc.wantDenyCode, decision)
			}
			if decision.ScopeSource != tc.wantScopeSource {
				t.Fatalf("ScopeSource = %q, want %q, decision=%+v", decision.ScopeSource, tc.wantScopeSource, decision)
			}
		})
	}
}

func TestTaskActionAuthorizerReadVisibilityHydratesScopedUser(t *testing.T) {
	userRepo := newIdentityUserRepo()
	userRepo.users[88] = &domain.User{
		ID:         88,
		Username:   "dept_admin",
		Department: domain.DepartmentDesignRD,
		Team:       "默认组",
	}
	userRepo.roles[88] = []domain.Role{domain.RoleDeptAdmin}

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       88,
		Username: "dept_admin",
		Roles:    []domain.Role{domain.RoleDeptAdmin},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	task := &domain.Task{
		ID:              9001,
		TaskStatus:      domain.TaskStatusInProgress,
		OwnerDepartment: "运营部",
		OwnerOrgTeam:    "淘系一组",
	}

	decision := newTaskActionAuthorizer(NewRoleBasedDataScopeResolver(), userRepo).EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, task, "", "")
	if !decision.Allowed {
		t.Fatalf("EvaluateTaskActionPolicy() allowed = false, want true for globally readable task detail, decision=%+v", decision)
	}
	if decision.ScopeSource != string(TaskActionScopeMainFlowRead) {
		t.Fatalf("ScopeSource = %q, want %q", decision.ScopeSource, TaskActionScopeMainFlowRead)
	}
}

func TestTaskActionAuthorizerFrontendViewAllDoesNotGrantWriteScope(t *testing.T) {
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    77,
		Roles: []domain.Role{domain.RoleDesigner},
		FrontendAccess: domain.FrontendAccessView{
			ViewAll: true,
		},
	})
	task := &domain.Task{
		ID:               9002,
		TaskStatus:       domain.TaskStatusInProgress,
		OwnerDepartment:  "运营部",
		OwnerOrgTeam:     "淘系一组",
		DesignerID:       authzInt64Ptr(1234),
		CurrentHandlerID: authzInt64Ptr(1234),
	}

	decision := newTaskActionAuthorizer(NewRoleBasedDataScopeResolver(), nil).EvaluateTaskActionPolicy(ctx, TaskActionSubmitDesign, task, "", "")
	if decision.Allowed {
		t.Fatalf("EvaluateTaskActionPolicy() allowed = true, want false, decision=%+v", decision)
	}
	if decision.DenyCode != "task_not_assigned_to_actor" {
		t.Fatalf("EvaluateTaskActionPolicy() deny_code = %q, want task_not_assigned_to_actor", decision.DenyCode)
	}
}

func TestTaskActionAuthorizerReadDetailUsesStageVisibilityForMidLaneRoles(t *testing.T) {
	authz := newTaskActionAuthorizer(NewRoleBasedDataScopeResolver(), nil)

	t.Run("audit_a can read pending audit a through stage scope", func(t *testing.T) {
		ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
			ID:    501,
			Roles: []domain.Role{domain.RoleAuditA},
		})
		task := &domain.Task{
			ID:                    5010,
			CreatorID:             9001,
			OwnerDepartment:       string(domain.DepartmentOperations),
			TaskStatus:            domain.TaskStatusPendingAuditA,
			CustomizationRequired: false,
		}

		decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, task, "", "")
		if !decision.Allowed {
			t.Fatalf("EvaluateTaskActionPolicy() allowed = false, want true, decision=%+v", decision)
		}
		if decision.ScopeSource != string(TaskActionScopeStage) {
			t.Fatalf("ScopeSource = %q, want %q", decision.ScopeSource, TaskActionScopeStage)
		}
	})

	t.Run("customization operator cannot read normal lane in progress via stage scope", func(t *testing.T) {
		ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
			ID:    502,
			Roles: []domain.Role{domain.RoleCustomizationOperator},
		})
		task := &domain.Task{
			ID:                    5020,
			CreatorID:             9002,
			OwnerDepartment:       string(domain.DepartmentOperations),
			TaskStatus:            domain.TaskStatusInProgress,
			CustomizationRequired: false,
		}

		decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, task, "", "")
		if !decision.Allowed {
			t.Fatalf("EvaluateTaskActionPolicy() allowed = false, want true for globally readable task detail, decision=%+v", decision)
		}
		if decision.ScopeSource != string(TaskActionScopeMainFlowRead) {
			t.Fatalf("ScopeSource = %q, want %q", decision.ScopeSource, TaskActionScopeMainFlowRead)
		}
	})
}

func TestTeamLeadReadsDepartmentWritesTeamOnly(t *testing.T) {
	resolver := NewRoleBasedDataScopeResolver()

	actor := domain.RequestActor{
		ID:         50,
		Username:   "team_lead_a",
		Roles:      []domain.Role{domain.RoleTeamLead},
		Department: "设计研发部",
		Team:       "A组",
		Source:     domain.RequestActorSourceSessionToken,
		AuthMode:   domain.AuthModeSessionTokenRoleEnforced,
	}

	sameTeamTask := &domain.Task{
		ID:               101,
		TaskStatus:       domain.TaskStatusInProgress,
		OwnerDepartment:  "设计研发部",
		OwnerOrgTeam:     "A组",
		DesignerID:       authzInt64Ptr(999),
		CurrentHandlerID: authzInt64Ptr(999),
	}
	otherTeamTask := &domain.Task{
		ID:               102,
		TaskStatus:       domain.TaskStatusInProgress,
		OwnerDepartment:  "设计研发部",
		OwnerOrgTeam:     "B组",
		DesignerID:       authzInt64Ptr(999),
		CurrentHandlerID: authzInt64Ptr(999),
	}
	otherDeptTask := &domain.Task{
		ID:              103,
		TaskStatus:      domain.TaskStatusInProgress,
		OwnerDepartment: "运营部",
		OwnerOrgTeam:    "淘系一组",
	}

	authz := newTaskActionAuthorizer(resolver, nil)

	t.Run("TeamLead can read same-department other-team task", func(t *testing.T) {
		ctx := domain.WithRequestActor(context.Background(), actor)
		decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, otherTeamTask, "", "")
		if !decision.Allowed {
			t.Fatalf("expected allowed=true for department read, got decision=%+v", decision)
		}
	})

	t.Run("TeamLead can read same-team task", func(t *testing.T) {
		ctx := domain.WithRequestActor(context.Background(), actor)
		decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, sameTeamTask, "", "")
		if !decision.Allowed {
			t.Fatalf("expected allowed=true for same-team read, got decision=%+v", decision)
		}
	})

	t.Run("TeamLead can read other-department task through main flow read", func(t *testing.T) {
		ctx := domain.WithRequestActor(context.Background(), actor)
		decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, otherDeptTask, "", "")
		if !decision.Allowed {
			t.Fatalf("expected allowed=true for globally readable main task flow, got decision=%+v", decision)
		}
		if decision.ScopeSource != string(TaskActionScopeMainFlowRead) {
			t.Fatalf("ScopeSource = %q, want %q", decision.ScopeSource, TaskActionScopeMainFlowRead)
		}
	})

	t.Run("TeamLead cannot reassign cross-team task", func(t *testing.T) {
		ctx := domain.WithRequestActor(context.Background(), actor)
		decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionReassign, otherTeamTask, "", "")
		if decision.Allowed {
			t.Fatalf("expected allowed=false for cross-team reassign, got decision=%+v", decision)
		}
	})
}

func TestTaskActionAuthorizerStageScopeWriteMatrix(t *testing.T) {
	cases := []struct {
		name            string
		action          TaskAction
		attrs           TaskActionAttributes
		actor           domain.RequestActor
		task            *domain.Task
		wantAllowed     bool
		wantDenyCode    string
		wantScopeSource string
	}{
		{
			name:   "audit_dept_admin_can_claim_pending_audit_a_via_stage_scope",
			action: TaskActionAuditClaim,
			attrs:  TaskActionAttributes{AuditStage: domain.AuditRecordStageA},
			actor: domain.RequestActor{
				ID:         601,
				Roles:      []domain.Role{domain.RoleDeptAdmin},
				Department: string(domain.DepartmentAudit),
			},
			task: &domain.Task{
				ID:              6010,
				TaskStatus:      domain.TaskStatusPendingAuditA,
				OwnerDepartment: string(domain.DepartmentOperations),
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeStage),
		},
		{
			name:   "warehouse_dept_admin_can_reject_foreign_pending_qc_via_stage_scope",
			action: TaskActionWarehouseReject,
			actor: domain.RequestActor{
				ID:         602,
				Roles:      []domain.Role{domain.RoleDeptAdmin},
				Department: string(domain.DepartmentCloudWarehouse),
			},
			task: &domain.Task{
				ID:               6020,
				TaskStatus:       domain.TaskStatusPendingWarehouseQC,
				OwnerDepartment:  string(domain.DepartmentOperations),
				CurrentHandlerID: authzInt64Ptr(9999),
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeStage),
		},
		{
			name:   "audit_dept_admin_cannot_reassign_foreign_task",
			action: TaskActionReassign,
			actor: domain.RequestActor{
				ID:         603,
				Roles:      []domain.Role{domain.RoleDeptAdmin},
				Department: string(domain.DepartmentAudit),
			},
			task: &domain.Task{
				ID:               6030,
				TaskStatus:       domain.TaskStatusInProgress,
				OwnerDepartment:  string(domain.DepartmentOperations),
				CurrentHandlerID: authzInt64Ptr(1001),
			},
			wantAllowed:  false,
			wantDenyCode: "task_out_of_department_scope",
		},
		{
			name:   "audit_dept_admin_can_reject_outsource_review_via_stage_scope",
			action: TaskActionAuditReject,
			attrs:  TaskActionAttributes{AuditStage: domain.AuditRecordStageOutsourceReview},
			actor: domain.RequestActor{
				ID:         604,
				Roles:      []domain.Role{domain.RoleDeptAdmin},
				Department: string(domain.DepartmentAudit),
			},
			task: &domain.Task{
				ID:               6040,
				TaskStatus:       domain.TaskStatusPendingOutsourceReview,
				OwnerDepartment:  string(domain.DepartmentOperations),
				CurrentHandlerID: authzInt64Ptr(1002),
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeStage),
		},
		{
			name:   "customization_art_dept_admin_can_review_foreign_task_via_stage_scope",
			action: TaskActionCustomizationReview,
			actor: domain.RequestActor{
				ID:         605,
				Roles:      []domain.Role{domain.RoleDeptAdmin},
				Department: string(domain.DepartmentCustomizationArt),
			},
			task: &domain.Task{
				ID:                    6050,
				TaskStatus:            domain.TaskStatusPendingCustomizationReview,
				OwnerDepartment:       string(domain.DepartmentOperations),
				CustomizationRequired: true,
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeStage),
		},
	}

	authz := newTaskActionAuthorizer(NewRoleBasedDataScopeResolver(), nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := domain.WithRequestActor(context.Background(), tc.actor)
			decision := authz.EvaluateTaskActionPolicyWithAttributes(ctx, tc.action, tc.task, "", "", tc.attrs)
			if decision.Allowed != tc.wantAllowed {
				t.Fatalf("Allowed = %v, want %v, decision=%+v", decision.Allowed, tc.wantAllowed, decision)
			}
			if decision.DenyCode != tc.wantDenyCode {
				t.Fatalf("DenyCode = %q, want %q, decision=%+v", decision.DenyCode, tc.wantDenyCode, decision)
			}
			if decision.ScopeSource != tc.wantScopeSource {
				t.Fatalf("ScopeSource = %q, want %q, decision=%+v", decision.ScopeSource, tc.wantScopeSource, decision)
			}
		})
	}
}

func TestTaskActionAuthorizerAssetUploadStageScopeMatrix(t *testing.T) {
	cases := []struct {
		name               string
		action             TaskAction
		actor              domain.RequestActor
		task               *domain.Task
		wantAllowed        bool
		wantDenyCode       string
		wantDenyReason     string
		wantScopeSource    string
		wantScopeSourceAny []string
		wantScopeSourceNot string
	}{
		{
			name:   "case_b_f2_1_positive_stage_grant",
			action: TaskActionAssetUploadSessionCreate,
			actor: domain.RequestActor{
				ID:         213,
				Roles:      []domain.Role{domain.RoleCustomizationOperator, domain.RoleDeptAdmin, domain.RoleMember},
				Department: string(domain.DepartmentCustomizationArt),
				Team:       "默认组",
			},
			task: &domain.Task{
				ID:                    469,
				OwnerDepartment:       string(domain.DepartmentOperations),
				OwnerOrgTeam:          "淘系一组",
				TaskStatus:            domain.TaskStatusPendingCustomizationProduction,
				CustomizationRequired: true,
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeStage),
		},
		{
			name:   "case_b_f2_2_negative_wrong_lane",
			action: TaskActionAssetUploadSessionCreate,
			actor: domain.RequestActor{
				ID:         213,
				Roles:      []domain.Role{domain.RoleCustomizationOperator, domain.RoleDeptAdmin, domain.RoleMember},
				Department: string(domain.DepartmentCustomizationArt),
				Team:       "默认组",
			},
			task: &domain.Task{
				ID:                    470,
				OwnerDepartment:       string(domain.DepartmentOperations),
				OwnerOrgTeam:          "淘系一组",
				TaskStatus:            domain.TaskStatusPendingCustomizationProduction,
				CustomizationRequired: false,
			},
			wantAllowed:  false,
			wantDenyCode: "task_out_of_stage_scope",
		},
		{
			name:   "case_b_f2_3_negative_wrong_role",
			action: TaskActionAssetUploadSessionCreate,
			actor: domain.RequestActor{
				ID:         214,
				Roles:      []domain.Role{domain.RoleMember},
				Department: string(domain.DepartmentCustomizationArt),
				Team:       "默认组",
			},
			task: &domain.Task{
				ID:                    471,
				OwnerDepartment:       string(domain.DepartmentOperations),
				OwnerOrgTeam:          "淘系一组",
				TaskStatus:            domain.TaskStatusPendingCustomizationProduction,
				CustomizationRequired: true,
			},
			wantAllowed:    false,
			wantDenyCode:   "missing_required_role",
			wantDenyReason: "asset upload requires a design, audit, customization, operation, or management role",
		},
		{
			name:   "audit_a_can_upload_in_pending_audit_a_via_stage",
			action: TaskActionAssetUploadSessionCreate,
			actor: domain.RequestActor{
				ID:    301,
				Roles: []domain.Role{domain.RoleAuditA, domain.RoleMember},
			},
			task: &domain.Task{
				ID:              478,
				OwnerDepartment: string(domain.DepartmentOperations),
				OwnerOrgTeam:    "淘系一组",
				TaskStatus:      domain.TaskStatusPendingAuditA,
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeStage),
		},
		{
			name:   "audit_b_can_upload_in_pending_audit_b_via_stage",
			action: TaskActionAssetUploadSessionCreate,
			actor: domain.RequestActor{
				ID:    302,
				Roles: []domain.Role{domain.RoleAuditB, domain.RoleMember},
			},
			task: &domain.Task{
				ID:              479,
				OwnerDepartment: string(domain.DepartmentOperations),
				OwnerOrgTeam:    "淘系一组",
				TaskStatus:      domain.TaskStatusPendingAuditB,
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeStage),
		},
		{
			name:   "case_b_f2_4_negative_wrong_status",
			action: TaskActionAssetUploadSessionCreate,
			actor: domain.RequestActor{
				ID:         213,
				Roles:      []domain.Role{domain.RoleCustomizationOperator, domain.RoleDeptAdmin, domain.RoleMember},
				Department: string(domain.DepartmentCustomizationArt),
				Team:       "默认组",
			},
			task: &domain.Task{
				ID:                    472,
				OwnerDepartment:       string(domain.DepartmentOperations),
				OwnerOrgTeam:          "淘系一组",
				TaskStatus:            domain.TaskStatusPendingClose,
				CustomizationRequired: true,
			},
			wantAllowed:    false,
			wantDenyCode:   "task_status_not_actionable",
			wantDenyReason: "task action is not allowed in the current status",
		},
		{
			name:   "case_b_f2_5_mirror_complete",
			action: TaskActionAssetUploadSessionComplete,
			actor: domain.RequestActor{
				ID:         213,
				Roles:      []domain.Role{domain.RoleCustomizationOperator, domain.RoleDeptAdmin, domain.RoleMember},
				Department: string(domain.DepartmentCustomizationArt),
				Team:       "默认组",
			},
			task: &domain.Task{
				ID:                    473,
				OwnerDepartment:       string(domain.DepartmentOperations),
				OwnerOrgTeam:          "淘系一组",
				TaskStatus:            domain.TaskStatusPendingCustomizationProduction,
				CustomizationRequired: true,
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeStage),
		},
		{
			name:   "case_b_f2_6_mirror_cancel",
			action: TaskActionAssetUploadSessionCancel,
			actor: domain.RequestActor{
				ID:         213,
				Roles:      []domain.Role{domain.RoleCustomizationOperator, domain.RoleDeptAdmin, domain.RoleMember},
				Department: string(domain.DepartmentCustomizationArt),
				Team:       "默认组",
			},
			task: &domain.Task{
				ID:                    474,
				OwnerDepartment:       string(domain.DepartmentOperations),
				OwnerOrgTeam:          "淘系一组",
				TaskStatus:            domain.TaskStatusPendingCustomizationProduction,
				CustomizationRequired: true,
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeStage),
		},
		{
			name:   "case_b_f2_7_ops_owner_still_works_without_stage",
			action: TaskActionAssetUploadSessionCreate,
			actor: domain.RequestActor{
				ID:         192,
				Roles:      []domain.Role{domain.RoleOps},
				Department: string(domain.DepartmentOperations),
				Team:       "淘系一组",
			},
			task: &domain.Task{
				ID:              475,
				OwnerDepartment: string(domain.DepartmentOperations),
				OwnerOrgTeam:    "淘系一组",
				TaskStatus:      domain.TaskStatusInProgress,
			},
			wantAllowed:        true,
			wantScopeSourceAny: []string{string(TaskActionScopeDepartment), string(TaskActionScopeTeam)},
			wantScopeSourceNot: string(TaskActionScopeStage),
		},
		{
			name:   "case_b_f2_8_customization_operator_pending_review_allowed_via_stage",
			action: TaskActionAssetUploadSessionCreate,
			actor: domain.RequestActor{
				ID:         213,
				Roles:      []domain.Role{domain.RoleCustomizationOperator, domain.RoleDeptAdmin, domain.RoleMember},
				Department: string(domain.DepartmentCustomizationArt),
				Team:       "默认组",
			},
			task: &domain.Task{
				ID:                    476,
				OwnerDepartment:       string(domain.DepartmentOperations),
				OwnerOrgTeam:          "淘系一组",
				TaskStatus:            domain.TaskStatusPendingCustomizationReview,
				CustomizationRequired: true,
			},
			wantAllowed:     true,
			wantScopeSource: string(TaskActionScopeStage),
		},
		{
			name:   "case_b_f2_9_ops_owner_pending_assign_can_upload_reference",
			action: TaskActionAssetUploadSessionCreate,
			actor: domain.RequestActor{
				ID:         192,
				Roles:      []domain.Role{domain.RoleOps},
				Department: string(domain.DepartmentOperations),
				Team:       "淘系一组",
			},
			task: &domain.Task{
				ID:              477,
				OwnerDepartment: string(domain.DepartmentOperations),
				OwnerOrgTeam:    "淘系一组",
				TaskStatus:      domain.TaskStatusPendingAssign,
			},
			wantAllowed:        true,
			wantScopeSourceAny: []string{string(TaskActionScopeDepartment), string(TaskActionScopeTeam)},
			wantScopeSourceNot: string(TaskActionScopeStage),
		},
	}

	authz := newTaskActionAuthorizer(NewRoleBasedDataScopeResolver(), nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := domain.WithRequestActor(context.Background(), tc.actor)
			decision := authz.EvaluateTaskActionPolicyWithAttributes(ctx, tc.action, tc.task, "", "", TaskActionAttributes{})
			if decision.Allowed != tc.wantAllowed {
				t.Fatalf("Allowed = %v, want %v, decision=%+v", decision.Allowed, tc.wantAllowed, decision)
			}
			if decision.DenyCode != tc.wantDenyCode {
				t.Fatalf("DenyCode = %q, want %q, decision=%+v", decision.DenyCode, tc.wantDenyCode, decision)
			}
			if tc.wantDenyReason != "" && decision.DenyReason != tc.wantDenyReason {
				t.Fatalf("DenyReason = %q, want %q, decision=%+v", decision.DenyReason, tc.wantDenyReason, decision)
			}
			if tc.wantScopeSource != "" && decision.ScopeSource != tc.wantScopeSource {
				t.Fatalf("ScopeSource = %q, want %q, decision=%+v", decision.ScopeSource, tc.wantScopeSource, decision)
			}
			if len(tc.wantScopeSourceAny) > 0 {
				matched := false
				for _, candidate := range tc.wantScopeSourceAny {
					if decision.ScopeSource == candidate {
						matched = true
						break
					}
				}
				if !matched {
					t.Fatalf("ScopeSource = %q, want one of %#v, decision=%+v", decision.ScopeSource, tc.wantScopeSourceAny, decision)
				}
			}
			if tc.wantScopeSourceNot != "" && decision.ScopeSource == tc.wantScopeSourceNot {
				t.Fatalf("ScopeSource = %q, must not be %q, decision=%+v", decision.ScopeSource, tc.wantScopeSourceNot, decision)
			}
		})
	}
}

func TestTaskActionAuthorizer_ReadDetail_DeptAdmin_ManagedByDesigner(t *testing.T) {
	userRepo := newIdentityUserRepo()
	userRepo.users[228] = &domain.User{
		ID:         228,
		Username:   "designer_228",
		Department: domain.DepartmentDesignRD,
		Team:       "设计一组",
	}

	authz := newTaskActionAuthorizer(NewRoleBasedDataScopeResolver(), userRepo)
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:                 198,
		Username:           "design_super_admin",
		Roles:              []domain.Role{domain.RoleDeptAdmin, domain.RoleDesigner, domain.RoleMember, domain.RoleTeamLead},
		Department:         string(domain.DepartmentDesignRD),
		ManagedDepartments: []string{string(domain.DepartmentDesignRD)},
		Source:             domain.RequestActorSourceSessionToken,
		AuthMode:           domain.AuthModeSessionTokenRoleEnforced,
	})
	task := &domain.Task{
		ID:              484,
		CreatorID:       700,
		DesignerID:      authzInt64Ptr(228),
		TaskStatus:      domain.TaskStatusInProgress,
		OwnerDepartment: string(domain.DepartmentOperations),
		OwnerOrgTeam:    "淘系一组",
	}

	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, task, "", "")
	if !decision.Allowed {
		t.Fatalf("EvaluateTaskActionPolicy() allowed = false, want true, decision=%+v", decision)
	}
	if decision.ScopeSource != string(TaskActionScopeManagedDepartment) {
		t.Fatalf("ScopeSource = %q, want %q", decision.ScopeSource, TaskActionScopeManagedDepartment)
	}
}

func TestTaskActionAuthorizer_ReadDetail_TeamLead_PlainScope_Unchanged(t *testing.T) {
	authz := newTaskActionAuthorizer(NewRoleBasedDataScopeResolver(), nil)
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         880,
		Username:   "team_lead_plain",
		Roles:      []domain.Role{domain.RoleTeamLead},
		Department: string(domain.DepartmentDesignRD),
		Team:       "设计一组",
	})
	task := &domain.Task{
		ID:              881,
		CreatorID:       990,
		TaskStatus:      domain.TaskStatusInProgress,
		OwnerDepartment: string(domain.DepartmentOperations),
		OwnerOrgTeam:    "淘系一组",
	}

	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, task, "", "")
	if !decision.Allowed {
		t.Fatalf("EvaluateTaskActionPolicy() allowed = false, want true for globally readable task detail, decision=%+v", decision)
	}
	if decision.ScopeSource != string(TaskActionScopeMainFlowRead) {
		t.Fatalf("ScopeSource = %q, want %q", decision.ScopeSource, TaskActionScopeMainFlowRead)
	}
}

func TestTaskActionAuthorizer_ReadDetail_Admin_ViewAll_BypassUnchanged(t *testing.T) {
	userRepo := &countingIdentityUserRepoStub{identityUserRepoStub: newIdentityUserRepo()}
	authz := newTaskActionAuthorizer(NewRoleBasedDataScopeResolver(), userRepo)
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         1,
		Username:   "admin",
		Roles:      []domain.Role{domain.RoleAdmin},
		Department: string(domain.DepartmentOperations),
	})
	task := &domain.Task{
		ID:              882,
		CreatorID:       991,
		TaskStatus:      domain.TaskStatusInProgress,
		OwnerDepartment: string(domain.DepartmentOperations),
		OwnerOrgTeam:    "淘系一组",
	}

	decision := authz.EvaluateTaskActionPolicy(ctx, TaskActionReadDetail, task, "", "")
	if !decision.Allowed {
		t.Fatalf("EvaluateTaskActionPolicy() allowed = false, want true, decision=%+v", decision)
	}
	if decision.ScopeSource != string(TaskActionScopeViewAll) {
		t.Fatalf("ScopeSource = %q, want %q", decision.ScopeSource, TaskActionScopeViewAll)
	}
	if userRepo.getByIDCalls != 0 {
		t.Fatalf("GetByID() calls = %d, want 0 for view_all bypass", userRepo.getByIDCalls)
	}
}

type countingIdentityUserRepoStub struct {
	*identityUserRepoStub
	getByIDCalls int
}

func (r *countingIdentityUserRepoStub) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	r.getByIDCalls++
	return r.identityUserRepoStub.GetByID(ctx, id)
}

func authzInt64Ptr(v int64) *int64 {
	return &v
}
