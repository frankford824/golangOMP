package task_cancel

import (
	"context"
	"encoding/json"
	"strings"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/permission"
)

type Service struct {
	tasks    repo.TaskRepo
	modules  repo.TaskModuleRepo
	events   repo.TaskModuleEventRepo
	txRunner repo.TxRunner
}

func NewService(tasks repo.TaskRepo, modules repo.TaskModuleRepo, events repo.TaskModuleEventRepo, txRunner repo.TxRunner) *Service {
	return &Service{tasks: tasks, modules: modules, events: events, txRunner: txRunner}
}

type Request struct {
	Actor  domain.RequestActor
	TaskID int64
	Reason string
	Force  bool
}

func (s *Service) Cancel(ctx context.Context, req Request) permission.Decision {
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		return permission.Deny(domain.ErrCodeReasonRequired, "reason is required")
	}
	task, err := s.tasks.GetByID(ctx, req.TaskID)
	if err != nil {
		return permission.Deny(domain.ErrCodeInternalError, err.Error())
	}
	if task == nil {
		return permission.Deny("task_not_found", "task not found")
	}
	if !canCancel(req.Actor, task, req.Force) {
		return permission.Deny(domain.DenyModuleActionRoleDenied, "actor cannot cancel task")
	}
	modules, err := s.modules.ListByTask(ctx, req.TaskID)
	if err != nil {
		return permission.Deny(domain.ErrCodeInternalError, err.Error())
	}
	if !req.Force {
		for _, m := range modules {
			if m.ModuleKey != domain.ModuleKeyBasicInfo && m.State != domain.ModuleStatePendingClaim && m.State != domain.ModuleStatePending && !m.State.Terminal() {
				return permission.Deny("task_already_claimed", "task has already been claimed")
			}
		}
	}
	targetTaskStatus := domain.TaskStatusCancelled
	targetModuleState := domain.ModuleStateForciblyClosed
	eventType := domain.ModuleEventTaskCancelled
	if req.Force {
		targetTaskStatus = domain.TaskStatusCompleted
		targetModuleState = domain.ModuleStateClosedByAdmin
		eventType = domain.ModuleEventForciblyClosed
	}
	err = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.taskRepoUpdateStatus(ctx, tx, req.TaskID, targetTaskStatus); err != nil {
			return err
		}
		before, err := s.modules.CloseOpenModules(ctx, tx, req.TaskID, targetModuleState)
		if err != nil {
			return err
		}
		for _, m := range before {
			if m.State.Terminal() {
				continue
			}
			from := m.State
			to := targetModuleState
			if _, err := s.events.Insert(ctx, tx, &domain.TaskModuleEvent{
				TaskModuleID:  m.ID,
				EventType:     eventType,
				FromState:     &from,
				ToState:       &to,
				ActorID:       &req.Actor.ID,
				ActorSnapshot: mustJSON(map[string]interface{}{"user_id": req.Actor.ID, "username": req.Actor.Username, "team": req.Actor.Team}),
				Payload:       mustJSON(map[string]interface{}{"reason": req.Reason, "force": req.Force}),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return permission.Deny(domain.ErrCodeInternalError, err.Error())
	}
	return permission.Allow()
}

func (s *Service) taskRepoUpdateStatus(ctx context.Context, tx repo.Tx, taskID int64, status domain.TaskStatus) error {
	return s.tasks.UpdateStatus(ctx, tx, taskID, status)
}

func canCancel(actor domain.RequestActor, task *domain.Task, force bool) bool {
	if actor.ID > 0 && actor.ID == task.CreatorID && !force {
		return true
	}
	for _, role := range actor.Roles {
		if role == domain.RoleSuperAdmin || role == domain.RoleDeptAdmin {
			return true
		}
	}
	return actor.FrontendAccess.IsSuperAdmin || actor.FrontendAccess.IsDepartmentAdmin
}

func mustJSON(v interface{}) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}
