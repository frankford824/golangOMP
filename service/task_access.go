package service

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

// AuthorizeTaskReadDetail applies the same read-detail policy used by task
// detail APIs. Thin HTTP helpers such as file proxies can call this without
// duplicating task data-scope logic.
func AuthorizeTaskReadDetail(ctx context.Context, task *domain.Task, _ repo.UserRepo) *domain.AppError {
	return newTaskActionAuthorizer().
		AuthorizeTaskAction(ctx, TaskActionReadDetail, task)
}

// authorizeTaskAssetList applies the task scope carried by either capability
// accepted by GET /v1/tasks/{id}/assets. A capability without a matching task
// scope must not expose staged asset metadata or controlled file URLs.
func authorizeTaskAssetList(ctx context.Context, task *domain.Task) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok {
		return nil
	}
	if actor.EffectiveAccess != nil &&
		(domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskView, task.AccessSubject()) ||
			domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetView, task.AccessSubject())) {
		return nil
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "task assets are outside the actor's explicit capability or data scope", map[string]interface{}{
		"deny_code": "task_asset_permission_or_scope_denied",
		"task_id":   task.ID,
	})
}
