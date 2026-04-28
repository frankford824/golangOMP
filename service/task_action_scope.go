package service

import (
	"context"
	"log"
	"strings"

	"workflow/domain"
)

type TaskActionScopeSource string

const (
	TaskActionScopeViewAll           TaskActionScopeSource = "view_all"
	TaskActionScopeDepartment        TaskActionScopeSource = "department_scope"
	TaskActionScopeTeam              TaskActionScopeSource = "team_scope"
	TaskActionScopeManagedDepartment TaskActionScopeSource = "managed_department_scope"
	TaskActionScopeManagedTeam       TaskActionScopeSource = "managed_team_scope"
	TaskActionScopeHandler           TaskActionScopeSource = "handler_match"
	TaskActionScopeDesigner          TaskActionScopeSource = "designer_match"
	TaskActionScopeCreator           TaskActionScopeSource = "creator_match"
	TaskActionScopeRequester         TaskActionScopeSource = "requester_match"
	TaskActionScopeStage             TaskActionScopeSource = "stage_scope"
	TaskActionScopeMainFlowRead      TaskActionScopeSource = "main_flow_read"
)

type taskActionActor struct {
	ID                 int64
	Username           string
	Roles              []domain.Role
	Department         string
	Team               string
	ManagedDepartments []string
	ManagedTeams       []string
	FrontendAccess     domain.FrontendAccessView
}

type taskActionScopeEvaluation struct {
	MatchedSources []TaskActionScopeSource
}

func (e taskActionScopeEvaluation) Has(source TaskActionScopeSource) bool {
	for _, candidate := range e.MatchedSources {
		if candidate == source {
			return true
		}
	}
	return false
}

func resolveTaskActionActor(ctx context.Context) (*taskActionActor, bool) {
	requestActor, ok := domain.RequestActorFromContext(ctx)
	if !ok || requestActor.ID <= 0 {
		return nil, false
	}
	actor := &taskActionActor{
		ID:                 requestActor.ID,
		Username:           strings.TrimSpace(requestActor.Username),
		Roles:              append([]domain.Role(nil), requestActor.Roles...),
		Department:         normalizeTaskDepartmentCode(requestActor.Department),
		Team:               strings.TrimSpace(requestActor.Team),
		ManagedDepartments: normalizeTaskDepartmentCodes(requestActor.ManagedDepartments),
		ManagedTeams:       append([]string(nil), requestActor.ManagedTeams...),
		FrontendAccess:     requestActor.FrontendAccess,
	}
	if actor.Department == "" {
		fallback := normalizeTaskDepartmentCode(actor.FrontendAccess.Department)
		if fallback != "" {
			log.Printf("WARN resolve_task_action_actor: actor_id=%d department empty, falling back to FrontendAccess.Department=%q", actor.ID, fallback)
			actor.Department = fallback
		}
	}
	if actor.Team == "" {
		fallback := strings.TrimSpace(actor.FrontendAccess.Team)
		if fallback != "" {
			log.Printf("WARN resolve_task_action_actor: actor_id=%d team empty, falling back to FrontendAccess.Team=%q", actor.ID, fallback)
			actor.Team = fallback
		}
	}
	if len(actor.ManagedDepartments) == 0 && len(actor.FrontendAccess.ManagedDepartments) > 0 {
		log.Printf("WARN resolve_task_action_actor: actor_id=%d managed_departments empty, falling back to FrontendAccess", actor.ID)
		actor.ManagedDepartments = normalizeTaskDepartmentCodes(actor.FrontendAccess.ManagedDepartments)
	}
	if len(actor.ManagedTeams) == 0 && len(actor.FrontendAccess.ManagedTeams) > 0 {
		log.Printf("WARN resolve_task_action_actor: actor_id=%d managed_teams empty, falling back to FrontendAccess", actor.ID)
		actor.ManagedTeams = append([]string(nil), actor.FrontendAccess.ManagedTeams...)
	}
	return actor, true
}

func evaluateTaskActionScope(actor *taskActionActor, task *domain.Task, ownerDepartment, ownerOrgTeam string) taskActionScopeEvaluation {
	out := taskActionScopeEvaluation{}
	if actor == nil {
		return out
	}
	ownerDepartment = normalizeTaskDepartmentCode(ownerDepartment)
	ownerOrgTeam = strings.TrimSpace(ownerOrgTeam)

	if hasAnyRoleValue(actor.Roles, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleRoleAdmin, domain.RoleHRAdmin) {
		out.MatchedSources = append(out.MatchedSources, TaskActionScopeViewAll)
	}
	if ownerDepartment != "" {
		if strings.EqualFold(strings.TrimSpace(actor.Department), ownerDepartment) {
			out.MatchedSources = appendIfMissingScope(out.MatchedSources, TaskActionScopeDepartment)
		}
		for _, department := range actor.ManagedDepartments {
			if strings.EqualFold(strings.TrimSpace(department), ownerDepartment) {
				out.MatchedSources = appendIfMissingScope(out.MatchedSources, TaskActionScopeManagedDepartment)
				out.MatchedSources = appendIfMissingScope(out.MatchedSources, TaskActionScopeDepartment)
				break
			}
		}
	}
	if ownerOrgTeam != "" {
		if strings.EqualFold(strings.TrimSpace(actor.Team), ownerOrgTeam) {
			out.MatchedSources = appendIfMissingScope(out.MatchedSources, TaskActionScopeTeam)
		}
		for _, team := range actor.ManagedTeams {
			if strings.EqualFold(strings.TrimSpace(team), ownerOrgTeam) {
				out.MatchedSources = appendIfMissingScope(out.MatchedSources, TaskActionScopeManagedTeam)
				out.MatchedSources = appendIfMissingScope(out.MatchedSources, TaskActionScopeTeam)
				break
			}
		}
	}
	if task != nil && actor.ID > 0 {
		if task.CurrentHandlerID != nil && *task.CurrentHandlerID == actor.ID {
			out.MatchedSources = appendIfMissingScope(out.MatchedSources, TaskActionScopeHandler)
		}
		if task.DesignerID != nil && *task.DesignerID == actor.ID {
			out.MatchedSources = appendIfMissingScope(out.MatchedSources, TaskActionScopeDesigner)
		}
		if task.CreatorID == actor.ID {
			out.MatchedSources = appendIfMissingScope(out.MatchedSources, TaskActionScopeCreator)
		}
		if task.RequesterID != nil && *task.RequesterID == actor.ID {
			out.MatchedSources = appendIfMissingScope(out.MatchedSources, TaskActionScopeRequester)
		}
		if matchesAnyStageVisibility(task, buildTaskActionStageVisibilities(actor)) {
			out.MatchedSources = appendIfMissingScope(out.MatchedSources, TaskActionScopeStage)
		}
	}
	return out
}

func buildTaskActionStageVisibilities(actor *taskActionActor) []StageVisibility {
	if actor == nil {
		return nil
	}
	stageVisibilities := append([]StageVisibility(nil), buildRoleBasedStageVisibilities(actor.Roles, actor.Department)...)
	if hasAnyRoleValue(actor.Roles, domain.RoleAuditA, domain.RoleAuditB) ||
		(hasRoleValue(actor.Roles, domain.RoleDeptAdmin) && domain.Department(actor.Department) == domain.DepartmentAudit) {
		stageBuilder := newStageVisibilityBuilder()
		for _, visibility := range stageVisibilities {
			if visibility.Lane == nil {
				stageBuilder.GrantNoLane(visibility.Statuses...)
				continue
			}
			stageBuilder.Grant(*visibility.Lane, visibility.Statuses...)
		}
		stageBuilder.GrantNoLane(domain.TaskStatusPendingOutsourceReview)
		return stageBuilder.Build()
	}
	return stageVisibilities
}

func appendIfMissingScope(scopes []TaskActionScopeSource, source TaskActionScopeSource) []TaskActionScopeSource {
	for _, candidate := range scopes {
		if candidate == source {
			return scopes
		}
	}
	return append(scopes, source)
}
