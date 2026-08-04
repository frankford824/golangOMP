package service

import (
	"context"
	"log"
	"strings"

	"workflow/domain"
)

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
	TaskID         int64
	OwnerDept      string
	OwnerOrgTeam   string
}

// taskActionAuthorizer intentionally does not retain role/data-scope
// dependencies. HTTP requests are authorized only from RequestActor's v8
// EffectiveAccess and stable organization IDs. A context without any actor is
// treated as a trusted internal call; a context with an actor but without
// effective access fails closed.
type taskActionAuthorizer struct{}

func newTaskActionAuthorizer() *taskActionAuthorizer {
	return &taskActionAuthorizer{}
}

func (a *taskActionAuthorizer) AuthorizeTaskAction(ctx context.Context, action TaskAction, task *domain.Task) *domain.AppError {
	decision := a.EvaluateTaskActionPolicy(ctx, action, task, "", "")
	a.logDecision(action, decision)
	if decision.Allowed {
		return nil
	}
	return taskActionDecisionAppError(action, decision)
}

func (a *taskActionAuthorizer) AuthorizeTaskCreate(ctx context.Context, ownerDepartment, ownerOrgTeam string) *domain.AppError {
	decision := a.EvaluateTaskActionPolicy(ctx, TaskActionCreate, nil, ownerDepartment, ownerOrgTeam)
	a.logDecision(TaskActionCreate, decision)
	if decision.Allowed {
		return nil
	}
	return taskActionDecisionAppError(TaskActionCreate, decision)
}

func (a *taskActionAuthorizer) EvaluateTaskActionPolicy(ctx context.Context, action TaskAction, task *domain.Task, ownerDepartment, ownerOrgTeam string) TaskActionDecision {
	decision := TaskActionDecision{
		ResolvedAction: string(action),
		TraceID:        domain.TraceIDFromContext(ctx),
		MatchedRule:    "explicit_capability_and_stable_scope",
		OwnerDept:      strings.TrimSpace(ownerDepartment),
		OwnerOrgTeam:   strings.TrimSpace(ownerOrgTeam),
	}
	permission := taskActionPermission(action)
	if permission == "" {
		decision.DenyCode = "unsupported_task_action"
		decision.DenyReason = "task action is not part of the active workflow"
		return decision
	}
	if task != nil {
		applyTaskReadModelOrgOwnership(task)
		decision.TaskID = task.ID
		decision.TaskStatus = string(task.TaskStatus)
		decision.OwnerDept = task.OwnerDepartment
		decision.OwnerOrgTeam = task.OwnerOrgTeam
	}
	if !taskActionStatusAllowed(action, task) {
		decision.DenyCode = "task_status_not_actionable"
		decision.DenyReason = "task action is not allowed in the current task state"
		decision.StatusReason = decision.DenyReason
		return decision
	}
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok {
		decision.Allowed = true
		decision.MatchedRule = "trusted_internal_call"
		decision.ScopeSource = "internal"
		return decision
	}
	decision.ActorID = actor.ID
	if actor.EffectiveAccess == nil {
		decision.DenyCode = "effective_access_required"
		decision.DenyReason = "explicit task permission is required"
		return decision
	}
	if task == nil {
		// Public task creation supplies stable owner IDs and performs the full
		// EffectiveAccessAllowsTask check in taskService.Create. Reaching this
		// fallback with an HTTP actor would otherwise reintroduce name-based scope.
		decision.DenyCode = "stable_scope_required"
		decision.DenyReason = "task creation requires stable organization scope"
		return decision
	}
	subject := task.AccessSubject()
	allowed := domain.EffectiveAccessAllowsTask(actor, permission, subject)
	if action == TaskActionReassign {
		allowed = domain.EffectiveAccessAllowsTaskReassign(actor, subject)
	}
	if !allowed {
		decision.DenyCode = "permission_or_scope_denied"
		decision.DenyReason = "task permission is outside the effective data scope"
		return decision
	}
	decision.Allowed = true
	decision.ScopeSource = "explicit_access"
	if action == TaskActionReassign && !domain.EffectiveAccessAllowsTask(actor, permission, subject) {
		decision.ScopeSource = "current_assignment"
	}
	return decision
}

func taskActionDecisionAppError(action TaskAction, decision TaskActionDecision) *domain.AppError {
	return domain.NewAppError(domain.ErrCodePermissionDenied, decision.DenyReason, map[string]interface{}{
		"action":           string(action),
		"resolved_action":  decision.ResolvedAction,
		"deny_code":        decision.DenyCode,
		"matched_rule":     decision.MatchedRule,
		"scope_source":     decision.ScopeSource,
		"task_id":          decision.TaskID,
		"task_status":      decision.TaskStatus,
		"status_reason":    decision.StatusReason,
		"owner_department": decision.OwnerDept,
		"owner_org_team":   decision.OwnerOrgTeam,
		"actor_id":         decision.ActorID,
	})
}

func (a *taskActionAuthorizer) logDecision(action TaskAction, decision TaskActionDecision) {
	log.Printf(
		"task_action_auth trace_id=%s action=%s task_id=%d task_status=%s actor_id=%d scope_source=%s allowed=%t deny_code=%s matched_rule=%s",
		decision.TraceID,
		action,
		decision.TaskID,
		decision.TaskStatus,
		decision.ActorID,
		decision.ScopeSource,
		decision.Allowed,
		decision.DenyCode,
		decision.MatchedRule,
	)
}
