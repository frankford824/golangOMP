package service

import (
	"context"

	"workflow/domain"
)

// authorizeTaskSKUItemBusinessInfoUpdate keeps catalog maintenance as the
// broad administrative capability while allowing a task creator to maintain
// only the non-cost fields of their own batch SKU rows.
func authorizeTaskSKUItemBusinessInfoUpdate(ctx context.Context, task *domain.Task) *domain.AppError {
	authorizer := newTaskActionAuthorizer()
	decision := authorizer.EvaluateTaskActionPolicy(ctx, TaskActionUpdateBusinessInfo, task, "", "")
	if decision.Allowed {
		authorizer.logDecision(TaskActionUpdateBusinessInfo, decision)
		return nil
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
		decision.MatchedRule = "task_creator_own_sku_business_info"
		decision.ScopeSource = "explicit_access"
		authorizer.logDecision(TaskActionUpdateBusinessInfo, decision)
		return nil
	}

	authorizer.logDecision(TaskActionUpdateBusinessInfo, decision)
	return taskActionDecisionAppError(TaskActionUpdateBusinessInfo, decision)
}
