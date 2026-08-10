package service

import (
	"context"

	"workflow/domain"
)

// authorizeTaskSKUItemBusinessInfoUpdate keeps catalog maintenance as the
// broad administrative capability while allowing a task creator to maintain
// the business fields of their own active batch SKU rows.
func authorizeTaskSKUItemBusinessInfoUpdate(ctx context.Context, task *domain.Task) *domain.AppError {
	return authorizeTaskSKUItemUpdate(ctx, task, "task_creator_own_sku_business_info")
}

// authorizeTaskSKUItemCostInfoUpdate restores row-scoped manual cost
// correction for the creator of an active task without granting global cost
// rule maintenance. The cost update itself remains audited by the task service.
func authorizeTaskSKUItemCostInfoUpdate(ctx context.Context, task *domain.Task) *domain.AppError {
	return authorizeTaskSKUItemUpdate(ctx, task, "task_creator_own_sku_cost_info")
}

func authorizeTaskSKUItemUpdate(ctx context.Context, task *domain.Task, creatorMatchedRule string) *domain.AppError {
	authorizer := newTaskActionAuthorizer()
	decision := authorizer.EvaluateTaskActionPolicy(ctx, TaskActionUpdateBusinessInfo, task, "", "")
	if decision.Allowed {
		authorizer.logDecision(TaskActionUpdateBusinessInfo, decision)
		return nil
	}
	if decision.DenyCode == "task_status_not_actionable" {
		authorizer.logDecision(TaskActionUpdateBusinessInfo, decision)
		return taskActionDecisionAppError(TaskActionUpdateBusinessInfo, decision)
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
		decision.MatchedRule = creatorMatchedRule
		decision.ScopeSource = "explicit_access"
		authorizer.logDecision(TaskActionUpdateBusinessInfo, decision)
		return nil
	}

	authorizer.logDecision(TaskActionUpdateBusinessInfo, decision)
	return taskActionDecisionAppError(TaskActionUpdateBusinessInfo, decision)
}
