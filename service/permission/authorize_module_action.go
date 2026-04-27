package permission

import (
	"context"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/module"
)

type Authorizer struct {
	tasks   taskGetter
	modules repo.TaskModuleRepo
}

type taskGetter interface {
	GetByID(ctx context.Context, id int64) (*domain.Task, error)
}

func NewAuthorizer(tasks taskGetter, modules repo.TaskModuleRepo) *Authorizer {
	return &Authorizer{tasks: tasks, modules: modules}
}

func (a *Authorizer) AuthorizeModuleAction(ctx context.Context, actor domain.RequestActor, taskID int64, moduleKey, action string) Decision {
	task, err := a.tasks.GetByID(ctx, taskID)
	if err != nil {
		return Deny(domain.ErrCodeInternalError, err.Error())
	}
	if task == nil {
		return Deny("task_not_found", "task not found")
	}
	return a.AuthorizeKnownTaskModuleAction(ctx, actor, task, moduleKey, action)
}

func (a *Authorizer) AuthorizeKnownTaskModuleAction(ctx context.Context, actor domain.RequestActor, task *domain.Task, moduleKey, action string) Decision {
	tm, err := a.modules.GetByTaskAndKey(ctx, task.ID, moduleKey)
	if err != nil {
		return Deny(domain.ErrCodeInternalError, err.Error())
	}
	if tm == nil {
		return Deny(DenyModuleNotInstantiated, "module is not instantiated")
	}
	if !ScopeAllows(actor, task, tm) {
		return Deny(DenyModuleOutOfScope, "module is out of actor scope")
	}
	if action == domain.ModuleActionClaim {
		return Allow()
	}
	spec, ok := module.ActionSpecFor(moduleKey, action)
	if !ok {
		return Deny(DenyModuleActionRoleDenied, "module action is not allowed")
	}
	if _, denyCode, ok := module.NextState(moduleKey, tm.State, action); !ok {
		return Deny(denyCode, "module state does not allow action")
	}
	if !roleFilterAllows(spec.RoleFilter, actor, task, tm) {
		return Deny(DenyModuleActionRoleDenied, "actor role does not allow action")
	}
	return Allow()
}

func roleFilterAllows(filter module.RoleFilter, actor domain.RequestActor, task *domain.Task, tm *domain.TaskModule) bool {
	if hasRole(actor, domain.RoleSuperAdmin) {
		return true
	}
	switch filter {
	case module.RoleScopedWorker:
		return true
	case module.RoleSelfOnly:
		return tm.ClaimedBy != nil && actor.ID > 0 && *tm.ClaimedBy == actor.ID
	case module.RoleSelfOrLead:
		if tm.ClaimedBy != nil && actor.ID > 0 && *tm.ClaimedBy == actor.ID {
			return true
		}
		return hasRole(actor, domain.RoleTeamLead) || hasRole(actor, domain.RoleDeptAdmin)
	case module.RoleLeadOrDepartmentAdmin:
		return hasRole(actor, domain.RoleTeamLead) || hasRole(actor, domain.RoleDeptAdmin)
	case module.RoleDepartmentAdmin:
		return hasRole(actor, domain.RoleDeptAdmin)
	case module.RoleCreatorOrOpsLead:
		return actor.ID > 0 && actor.ID == task.CreatorID || hasRole(actor, domain.RoleTeamLead) || hasRole(actor, domain.RoleDeptAdmin)
	case module.RoleCreatorOrAdmin:
		return actor.ID > 0 && actor.ID == task.CreatorID || hasRole(actor, domain.RoleDeptAdmin)
	default:
		return false
	}
}
