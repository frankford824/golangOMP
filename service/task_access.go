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
