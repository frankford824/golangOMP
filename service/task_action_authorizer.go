package service

import (
	"context"
	"log"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type TaskActionAttributes struct {
	AuditStage domain.AuditRecordStage
}

type TaskActionDecision struct {
	Allowed        bool
	DenyCode       string
	DenyReason     string
	MatchedRule    string
	ScopeSource    string
	TraceID        string
	ResolvedAction string
	TaskStatus     string
	StatusReason   string
	ActorID        int64
	ActorRoles     []domain.Role
	TaskID         int64
	OwnerDept      string
	OwnerOrgTeam   string
}

type taskActionAuthorizer struct {
	dataScopeResolver DataScopeResolver
	scopeUserRepo     repo.UserRepo
}

func newTaskActionAuthorizer(resolver DataScopeResolver, userRepo repo.UserRepo) *taskActionAuthorizer {
	return &taskActionAuthorizer{
		dataScopeResolver: resolver,
		scopeUserRepo:     userRepo,
	}
}

func (a *taskActionAuthorizer) AuthorizeTaskAction(ctx context.Context, action TaskAction, task *domain.Task) *domain.AppError {
	return a.AuthorizeTaskActionWithAttributes(ctx, action, task, TaskActionAttributes{})
}

func (a *taskActionAuthorizer) AuthorizeTaskActionWithAttributes(ctx context.Context, action TaskAction, task *domain.Task, attrs TaskActionAttributes) *domain.AppError {
	return a.authorizeTaskActionWithOwnership(ctx, action, task, "", "", attrs)
}

func (a *taskActionAuthorizer) AuthorizeTaskCreate(ctx context.Context, ownerDepartment, ownerOrgTeam string) *domain.AppError {
	return a.authorizeTaskActionWithOwnership(ctx, TaskActionCreate, nil, ownerDepartment, ownerOrgTeam, TaskActionAttributes{})
}

func (a *taskActionAuthorizer) authorizeTaskActionWithOwnership(ctx context.Context, action TaskAction, task *domain.Task, ownerDepartment, ownerOrgTeam string, attrs TaskActionAttributes) *domain.AppError {
	decision := a.EvaluateTaskActionPolicyWithAttributes(ctx, action, task, ownerDepartment, ownerOrgTeam, attrs)
	a.logDecision(action, decision)
	if decision.Allowed {
		return nil
	}
	return taskActionDecisionAppError(action, decision)
}

func taskActionDecisionAppError(action TaskAction, decision TaskActionDecision) *domain.AppError {
	return domain.NewAppError(domain.ErrCodePermissionDenied, decision.DenyReason, map[string]interface{}{
		"action":           string(action),
		"resolved_action":  decision.ResolvedAction,
		"deny_code":        decision.DenyCode,
		"deny_reason":      decision.DenyCode,
		"matched_rule":     decision.MatchedRule,
		"scope_source":     decision.ScopeSource,
		"task_id":          decision.TaskID,
		"task_status":      decision.TaskStatus,
		"status_reason":    decision.StatusReason,
		"owner_department": decision.OwnerDept,
		"owner_org_team":   decision.OwnerOrgTeam,
		"actor_id":         decision.ActorID,
		"actor_roles":      decision.ActorRoles,
	})
}

func taskActionDecisionHasElevatedScope(decision TaskActionDecision) bool {
	switch TaskActionScopeSource(decision.ScopeSource) {
	case TaskActionScopeViewAll, TaskActionScopeManagedDepartment, TaskActionScopeManagedTeam:
		return true
	case TaskActionScopeDepartment:
		return hasAnyRoleValue(decision.ActorRoles, domain.RoleDeptAdmin, domain.RoleDesignDirector)
	case TaskActionScopeTeam:
		return hasRoleValue(decision.ActorRoles, domain.RoleTeamLead)
	default:
		return false
	}
}

func (a *taskActionAuthorizer) EvaluateTaskActionPolicy(ctx context.Context, action TaskAction, task *domain.Task, ownerDepartment, ownerOrgTeam string) TaskActionDecision {
	return a.EvaluateTaskActionPolicyWithAttributes(ctx, action, task, ownerDepartment, ownerOrgTeam, TaskActionAttributes{})
}

func (a *taskActionAuthorizer) EvaluateTaskActionPolicyWithAttributes(
	ctx context.Context,
	action TaskAction,
	task *domain.Task,
	ownerDepartment, ownerOrgTeam string,
	attrs TaskActionAttributes,
) TaskActionDecision {
	actor, ok := resolveTaskActionActor(ctx)
	if !ok {
		log.Printf("WARN task_action_auth: no request actor resolved — allowing action=%s for backward compatibility", action)
		return TaskActionDecision{
			Allowed:     true,
			MatchedRule: "no_request_actor_backward_compatible",
		}
	}

	resolvedAction, stageDenyCode, stageDenyReason := resolveTaskAction(action, task, attrs)
	rule := taskActionRuleFor(resolvedAction)

	decision := TaskActionDecision{
		Allowed:        false,
		MatchedRule:    rule.MatchedRule,
		TraceID:        domain.TraceIDFromContext(ctx),
		ResolvedAction: string(resolvedAction),
		ActorID:        actor.ID,
		ActorRoles:     append([]domain.Role(nil), actor.Roles...),
	}
	if task != nil {
		applyTaskReadModelOrgOwnership(task)
		decision.TaskID = task.ID
		decision.TaskStatus = string(task.TaskStatus)
		if strings.TrimSpace(ownerDepartment) == "" {
			ownerDepartment = task.OwnerDepartment
		}
		if strings.TrimSpace(ownerOrgTeam) == "" {
			ownerOrgTeam = task.OwnerOrgTeam
		}
	}
	decision.OwnerDept = strings.TrimSpace(ownerDepartment)
	decision.OwnerOrgTeam = strings.TrimSpace(ownerOrgTeam)
	if stageDenyCode != "" {
		decision.DenyCode = stageDenyCode
		decision.DenyReason = stageDenyReason
		decision.StatusReason = stageDenyReason
		return decision
	}

	if !actorHasAllowedTaskActionRole(actor, rule.RequiredRoles) {
		decision.DenyCode = "missing_required_role"
		decision.DenyReason = authFirstNonEmpty(rule.RoleGateMessage, "task action denied because the actor role is insufficient")
		return decision
	}
	if task != nil && len(rule.AllowedStatuses) > 0 && !taskActionStatusAllowed(task.TaskStatus, rule.AllowedStatuses) {
		decision.DenyCode = authFirstNonEmpty(rule.StatusDenyCode, "task_status_not_actionable")
		decision.DenyReason = authFirstNonEmpty(rule.StatusGateMessage, "task action is not allowed in the current status")
		decision.StatusReason = decision.DenyReason
		return decision
	}

	if rule.UseReadVisibility && task != nil {
		scope, appErr := resolveDataScopeForActor(ctx, a.dataScopeResolver, a.scopeUserRepo)
		if appErr != nil {
			return TaskActionDecision{
				Allowed:      false,
				DenyCode:     "scope_resolution_failed",
				DenyReason:   appErr.Message,
				MatchedRule:  rule.MatchedRule,
				ActorID:      actor.ID,
				ActorRoles:   append([]domain.Role(nil), actor.Roles...),
				TaskID:       task.ID,
				OwnerDept:    task.OwnerDepartment,
				OwnerOrgTeam: task.OwnerOrgTeam,
			}
		}
		if canViewTaskWithScope(task, scope) {
			decision.Allowed = true
			decision.ScopeSource = inferReadScopeSource(task, scope, nil)
			return decision
		}
		if hasManagedOrgScope(scope) && !scope.ViewAll {
			actorOrg, appErr := resolveTaskActorOrgSnapshot(ctx, a.scopeUserRepo, task)
			if appErr != nil {
				return TaskActionDecision{
					Allowed:      false,
					DenyCode:     "scope_resolution_failed",
					DenyReason:   appErr.Message,
					MatchedRule:  rule.MatchedRule,
					ActorID:      actor.ID,
					ActorRoles:   append([]domain.Role(nil), actor.Roles...),
					TaskID:       task.ID,
					OwnerDept:    task.OwnerDepartment,
					OwnerOrgTeam: task.OwnerOrgTeam,
				}
			}
			if canViewTaskWithScopeAndActorOrg(task, scope, actorOrg) {
				decision.Allowed = true
				decision.ScopeSource = inferReadScopeSource(task, scope, actorOrg)
				return decision
			}
		}
		decision.DenyCode = inferReadScopeDenyCode(actor, task)
		decision.DenyReason = authFirstNonEmpty(rule.ScopeGateMessage, "task detail is outside the current data scope")
		return decision
	}

	scopeEval := evaluateTaskActionScope(actor, task, decision.OwnerDept, decision.OwnerOrgTeam)
	for _, scope := range rule.AllowedScopes {
		if scopeEval.Has(scope) && taskActionActorCanUseScope(actor, scope) {
			decision.ScopeSource = string(scope)
			break
		}
	}
	if decision.ScopeSource == "" {
		decision.DenyCode = inferWriteScopeDenyCode(actor, task, decision.OwnerDept, decision.OwnerOrgTeam, rule, scopeEval)
		decision.DenyReason = authFirstNonEmpty(rule.ScopeGateMessage, "task action denied because it is outside the actor scope")
		return decision
	}

	if task != nil && !taskActionScopeHasElevatedMatch(actor, scopeEval) {
		if handlerDenyCode, handlerDenyReason := evaluateHandlerPolicy(rule.HandlerPolicy, task, scopeEval); handlerDenyCode != "" {
			decision.DenyCode = handlerDenyCode
			decision.DenyReason = handlerDenyReason
			return decision
		}
	}
	if resolvedAction == TaskActionReassign && actor != nil && task != nil && !taskActionActorHasManagementScopeRole(actor) {
		matchedScope := TaskActionScopeSource(decision.ScopeSource)
		if matchedScope != TaskActionScopeRequester && matchedScope != TaskActionScopeCreator {
			decision.DenyCode = "task_reassign_requires_requester_or_manager"
			decision.DenyReason = "operation reassignment requires requester/initiator ownership or management scope"
			return decision
		}
	}

	decision.Allowed = true
	return decision
}

func taskActionStatusAllowed(current domain.TaskStatus, allowed []domain.TaskStatus) bool {
	for _, candidate := range allowed {
		if current == candidate {
			return true
		}
	}
	return false
}

func (a *taskActionAuthorizer) logDecision(action TaskAction, decision TaskActionDecision) {
	log.Printf(
		"task_action_auth trace_id=%s action=%s resolved_action=%s task_id=%d task_status=%s actor_id=%d actor_roles=%s owner_department=%s owner_org_team=%s scope_source=%s allowed=%t deny_reason=%s matched_rule=%s",
		decision.TraceID,
		action,
		decision.ResolvedAction,
		decision.TaskID,
		decision.TaskStatus,
		decision.ActorID,
		domain.JoinRoles(decision.ActorRoles),
		decision.OwnerDept,
		decision.OwnerOrgTeam,
		decision.ScopeSource,
		decision.Allowed,
		decision.DenyCode,
		decision.MatchedRule,
	)
}

func actorHasAllowedTaskActionRole(actor *taskActionActor, required []domain.Role) bool {
	if actor == nil {
		return false
	}
	return domain.ActorHasAnyRole(domain.RequestActor{Roles: actor.Roles}, required)
}

func inferReadScopeSource(task *domain.Task, scope *DataScope, actorOrg *taskActorOrgSnapshot) string {
	if scope == nil {
		return string(TaskActionScopeViewAll)
	}
	if scope.ViewAll {
		return string(TaskActionScopeViewAll)
	}
	if task != nil {
		if task.OwnerDepartment != "" {
			for _, department := range scope.DepartmentCodes {
				if department == task.OwnerDepartment {
					return string(TaskActionScopeDepartment)
				}
			}
		}
		if task.OwnerOrgTeam != "" {
			for _, team := range scope.TeamCodes {
				if team == task.OwnerOrgTeam {
					return string(TaskActionScopeTeam)
				}
			}
		}
		for _, uid := range scope.UserIDs {
			if uid <= 0 {
				continue
			}
			if task.CreatorID == uid {
				return "self_scope"
			}
			if task.DesignerID != nil && *task.DesignerID == uid {
				return "self_scope"
			}
			if task.CurrentHandlerID != nil && *task.CurrentHandlerID == uid {
				return "self_scope"
			}
		}
		if matchesAnyStageVisibility(task, scope.StageVisibilities) {
			return string(TaskActionScopeStage)
		}
		if actorOrg != nil {
			for _, department := range actorOrg.Departments {
				for _, managed := range scope.ManagedDepartmentCodes {
					if managed != "" && managed == department {
						return string(TaskActionScopeManagedDepartment)
					}
				}
			}
			for _, team := range actorOrg.Teams {
				for _, managed := range scope.ManagedTeamCodes {
					if managed != "" && managed == team {
						return string(TaskActionScopeManagedTeam)
					}
				}
			}
		}
	}
	return "self_scope"
}

func inferReadScopeDenyCode(actor *taskActionActor, task *domain.Task) string {
	if task == nil {
		return "task_out_of_scope"
	}
	if task.OwnerOrgTeam != "" && actor != nil && (strings.TrimSpace(actor.Team) != "" || len(actor.ManagedTeams) > 0) {
		return "task_out_of_team_scope"
	}
	if task.OwnerDepartment != "" && actor != nil && (strings.TrimSpace(actor.Department) != "" || len(actor.ManagedDepartments) > 0) {
		return "task_out_of_department_scope"
	}
	return "task_out_of_scope"
}

func inferWriteScopeDenyCode(actor *taskActionActor, task *domain.Task, ownerDepartment, ownerOrgTeam string, rule taskActionRule, scopeEval taskActionScopeEvaluation) string {
	if rule.PreferHandlerDeny && task != nil && task.CurrentHandlerID != nil && actor != nil &&
		!taskActionActorHasManagementScopeRole(actor) {
		return "task_not_assigned_to_actor"
	}
	if ruleAllowsScope(rule, TaskActionScopeStage) && !scopeEval.Has(TaskActionScopeStage) && !hasAnyMatchedAllowedScope(scopeEval, rule.AllowedScopes, TaskActionScopeStage) {
		return "task_out_of_stage_scope"
	}
	if actor != nil && taskActionActorHasTeamManagementRole(actor) && ownerOrgTeam != "" {
		return "task_out_of_team_scope"
	}
	if actor != nil && taskActionActorHasDepartmentManagementRole(actor) && ownerDepartment != "" {
		return "task_out_of_department_scope"
	}
	if ownerOrgTeam != "" && actor != nil && (strings.TrimSpace(actor.Team) != "" || len(actor.ManagedTeams) > 0) {
		return "task_out_of_team_scope"
	}
	if ownerDepartment != "" && actor != nil && (strings.TrimSpace(actor.Department) != "" || len(actor.ManagedDepartments) > 0) {
		return "task_out_of_department_scope"
	}
	if rule.PreferHandlerDeny && task != nil && task.CurrentHandlerID != nil && !scopeEval.Has(TaskActionScopeViewAll) &&
		!scopeEval.Has(TaskActionScopeManagedDepartment) && !scopeEval.Has(TaskActionScopeManagedTeam) {
		return "task_not_assigned_to_actor"
	}
	if task != nil && task.CurrentHandlerID != nil {
		return "task_not_assigned_to_actor"
	}
	return "task_out_of_scope"
}

func authFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func taskActionActorCanUseScope(actor *taskActionActor, scope TaskActionScopeSource) bool {
	if actor == nil {
		return false
	}
	switch scope {
	case TaskActionScopeDepartment:
		if taskActionActorHasTeamManagementRole(actor) && !taskActionActorHasDepartmentManagementRole(actor) {
			return false
		}
	case TaskActionScopeStage:
		return taskActionActorHasStageScopeRole(actor)
	case TaskActionScopeManagedDepartment:
		return taskActionActorHasDepartmentManagementRole(actor) || len(actor.ManagedDepartments) > 0
	case TaskActionScopeManagedTeam:
		return taskActionActorHasTeamManagementRole(actor) || len(actor.ManagedTeams) > 0
	}
	return true
}

func taskActionActorHasDepartmentManagementRole(actor *taskActionActor) bool {
	if actor == nil {
		return false
	}
	return hasAnyRoleValue(actor.Roles, domain.RoleDeptAdmin, domain.RoleDesignDirector)
}

func taskActionActorHasTeamManagementRole(actor *taskActionActor) bool {
	if actor == nil {
		return false
	}
	return hasRoleValue(actor.Roles, domain.RoleTeamLead)
}

func taskActionActorHasManagementScopeRole(actor *taskActionActor) bool {
	if actor == nil {
		return false
	}
	return hasAnyRoleValue(actor.Roles,
		domain.RoleAdmin,
		domain.RoleSuperAdmin,
		domain.RoleRoleAdmin,
		domain.RoleHRAdmin,
		domain.RoleDeptAdmin,
		domain.RoleDesignDirector,
		domain.RoleTeamLead,
	)
}

func taskActionActorHasStageScopeRole(actor *taskActionActor) bool {
	if actor == nil {
		return false
	}
	return hasAnyRoleValue(actor.Roles,
		domain.RoleAuditA,
		domain.RoleAuditB,
		domain.RoleWarehouse,
		domain.RoleOutsource,
		domain.RoleCustomizationOperator,
		domain.RoleCustomizationReviewer,
		domain.RoleDeptAdmin,
	)
}

func resolveTaskAction(action TaskAction, task *domain.Task, attrs TaskActionAttributes) (TaskAction, string, string) {
	if task == nil {
		return action, "", ""
	}
	stage, ok := activeAuditStageFromStatus(task.TaskStatus)
	switch action {
	case TaskActionAuditClaim, TaskActionAuditApprove, TaskActionAuditReject, TaskActionAuditTransfer, TaskActionAuditHandover, TaskActionAuditTakeover:
		if !ok {
			return action, "", ""
		}
		if attrs.AuditStage != "" && stage != attrs.AuditStage {
			return action, "audit_stage_mismatch", "task workflow audit stage does not match the requested audit action"
		}
		return resolveAuditStageAction(action, stage), "", ""
	default:
		return action, "", ""
	}
}

func resolveAuditStageAction(action TaskAction, stage domain.AuditRecordStage) TaskAction {
	switch stage {
	case domain.AuditRecordStageA:
		switch action {
		case TaskActionAuditClaim:
			return TaskActionAuditAClaim
		case TaskActionAuditApprove:
			return TaskActionAuditAApprove
		case TaskActionAuditReject:
			return TaskActionAuditAReject
		case TaskActionAuditTransfer:
			return TaskActionAuditATransfer
		case TaskActionAuditHandover:
			return TaskActionAuditAHandover
		case TaskActionAuditTakeover:
			return TaskActionAuditATakeover
		}
	case domain.AuditRecordStageB:
		switch action {
		case TaskActionAuditClaim:
			return TaskActionAuditBClaim
		case TaskActionAuditApprove:
			return TaskActionAuditBApprove
		case TaskActionAuditReject:
			return TaskActionAuditBReject
		case TaskActionAuditTransfer:
			return TaskActionAuditBTransfer
		case TaskActionAuditHandover:
			return TaskActionAuditBHandover
		case TaskActionAuditTakeover:
			return TaskActionAuditBTakeover
		}
	case domain.AuditRecordStageOutsourceReview:
		switch action {
		case TaskActionAuditClaim:
			return TaskActionAuditOutsourceReviewClaim
		case TaskActionAuditApprove:
			return TaskActionAuditOutsourceReviewApprove
		case TaskActionAuditReject:
			return TaskActionAuditOutsourceReviewReject
		case TaskActionAuditTransfer:
			return TaskActionAuditOutsourceReviewTransfer
		case TaskActionAuditHandover:
			return TaskActionAuditOutsourceReviewHandover
		case TaskActionAuditTakeover:
			return TaskActionAuditOutsourceReviewTakeover
		}
	}
	return action
}

func taskActionScopeHasElevatedMatch(actor *taskActionActor, scopeEval taskActionScopeEvaluation) bool {
	if actor == nil {
		return false
	}
	if scopeEval.Has(TaskActionScopeViewAll) || scopeEval.Has(TaskActionScopeManagedDepartment) || scopeEval.Has(TaskActionScopeManagedTeam) || scopeEval.Has(TaskActionScopeStage) {
		return true
	}
	if scopeEval.Has(TaskActionScopeDepartment) && taskActionActorHasDepartmentManagementRole(actor) {
		return true
	}
	if scopeEval.Has(TaskActionScopeTeam) && taskActionActorHasTeamManagementRole(actor) {
		return true
	}
	return false
}

func ruleAllowsScope(rule taskActionRule, target TaskActionScopeSource) bool {
	for _, scope := range rule.AllowedScopes {
		if scope == target {
			return true
		}
	}
	return false
}

func hasAnyMatchedAllowedScope(scopeEval taskActionScopeEvaluation, allowed []TaskActionScopeSource, skip TaskActionScopeSource) bool {
	for _, scope := range allowed {
		if scope == skip {
			continue
		}
		if scopeEval.Has(scope) {
			return true
		}
	}
	return false
}

func evaluateHandlerPolicy(policy taskActionHandlerPolicy, task *domain.Task, scopeEval taskActionScopeEvaluation) (string, string) {
	if task == nil {
		return "", ""
	}
	switch policy {
	case taskActionHandlerPolicyNone:
		return "", ""
	case taskActionHandlerPolicyUnassignedOrCurrentActor:
		if task.CurrentHandlerID != nil && !scopeEval.Has(TaskActionScopeHandler) {
			return "task_not_assigned_to_actor", "task action requires the current handler when the task is already assigned"
		}
	case taskActionHandlerPolicyRequireCurrentHandler:
		if !scopeEval.Has(TaskActionScopeHandler) {
			return "task_not_assigned_to_actor", "task action requires the current handler"
		}
	}
	return "", ""
}
