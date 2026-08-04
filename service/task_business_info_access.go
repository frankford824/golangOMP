package service

import (
	"context"

	"workflow/domain"
)

type taskBusinessInfoUpdateAccess struct {
	CanManageGovernedFields bool
}

// authorizeTaskBusinessInfoUpdate keeps catalog maintenance as the broad
// administrative capability while allowing task creators to maintain the
// non-governed business fields of their own active tasks.
func authorizeTaskBusinessInfoUpdate(ctx context.Context, task *domain.Task) (taskBusinessInfoUpdateAccess, *domain.AppError) {
	authorizer := newTaskActionAuthorizer()
	decision := authorizer.EvaluateTaskActionPolicy(ctx, TaskActionUpdateBusinessInfo, task, "", "")
	if decision.Allowed {
		authorizer.logDecision(TaskActionUpdateBusinessInfo, decision)
		return taskBusinessInfoUpdateAccess{CanManageGovernedFields: true}, nil
	}
	if decision.DenyCode == "task_status_not_actionable" {
		authorizer.logDecision(TaskActionUpdateBusinessInfo, decision)
		return taskBusinessInfoUpdateAccess{}, taskActionDecisionAppError(TaskActionUpdateBusinessInfo, decision)
	}

	actor, ok := domain.RequestActorFromContext(ctx)
	if ok &&
		task != nil &&
		actor.ID > 0 &&
		actor.ID == task.CreatorID &&
		domain.ActorHasPermission(actor, domain.PermissionTaskCreate) &&
		domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskCreate, task.AccessSubject()) {
		decision.Allowed = true
		decision.DenyCode = ""
		decision.DenyReason = ""
		decision.MatchedRule = "task_creator_own_business_info"
		decision.ScopeSource = "explicit_access"
		authorizer.logDecision(TaskActionUpdateBusinessInfo, decision)
		return taskBusinessInfoUpdateAccess{}, nil
	}

	authorizer.logDecision(TaskActionUpdateBusinessInfo, decision)
	return taskBusinessInfoUpdateAccess{}, taskActionDecisionAppError(TaskActionUpdateBusinessInfo, decision)
}
