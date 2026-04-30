package permission

import (
	"strings"

	"workflow/domain"
	"workflow/service/module"
)

func ScopeAllows(actor domain.RequestActor, task *domain.Task, tm *domain.TaskModule) bool {
	if task == nil || tm == nil {
		return false
	}
	if hasRole(actor, domain.RoleSuperAdmin) || actor.FrontendAccess.IsSuperAdmin {
		return true
	}
	if tm.ModuleKey == domain.ModuleKeyBasicInfo {
		if actor.ID > 0 && actor.ID == task.CreatorID {
			return true
		}
		return basicInfoLeadScope(actor, task)
	}
	desc, ok := module.DescriptorFor(tm.ModuleKey)
	if ok && hasRole(actor, domain.RoleDeptAdmin) && stringIn(strings.TrimSpace(string(desc.Department)), actor.ManagedDepartments, actor.FrontendAccess.ManagedDepartments, actor.FrontendAccess.DepartmentCodes) {
		return true
	}
	if tm.ClaimedBy != nil && actor.ID > 0 && *tm.ClaimedBy == actor.ID {
		return true
	}
	if tm.State == domain.ModuleStatePendingClaim {
		if tm.PoolTeamCode == nil {
			return false
		}
		return teamInActor(*tm.PoolTeamCode, actor)
	}
	if tm.ClaimedTeamCode != nil {
		if hasRole(actor, domain.RoleTeamLead) && teamManagedByActor(*tm.ClaimedTeamCode, actor) {
			return true
		}
	}
	return false
}

func basicInfoLeadScope(actor domain.RequestActor, task *domain.Task) bool {
	if !hasRole(actor, domain.RoleTeamLead) {
		return false
	}
	creatorTeam := strings.TrimSpace(task.OwnerOrgTeam)
	if creatorTeam == "" {
		creatorTeam = strings.TrimSpace(task.OwnerTeam)
	}
	return creatorTeam != "" && teamManagedByActor(creatorTeam, actor)
}

func teamInActor(team string, actor domain.RequestActor) bool {
	team = strings.TrimSpace(team)
	if team == "" {
		return false
	}
	if strings.EqualFold(team, strings.TrimSpace(actor.Team)) {
		return true
	}
	if stringIn(team, actor.ManagedTeams, actor.FrontendAccess.ManagedTeams, actor.FrontendAccess.TeamCodes) {
		return true
	}
	for _, target := range domain.PoolTeamTargets(team) {
		if !actorDepartmentMatches(actor, target.Department) {
			continue
		}
		if strings.EqualFold(target.Team, strings.TrimSpace(actor.Team)) ||
			stringIn(target.Team, actor.ManagedTeams, actor.FrontendAccess.ManagedTeams, actor.FrontendAccess.TeamCodes) {
			return true
		}
	}
	return false
}

func teamManagedByActor(team string, actor domain.RequestActor) bool {
	team = strings.TrimSpace(team)
	if team == "" {
		return false
	}
	if strings.EqualFold(team, strings.TrimSpace(actor.Team)) {
		return true
	}
	if stringIn(team, actor.ManagedTeams, actor.FrontendAccess.ManagedTeams, actor.FrontendAccess.TeamCodes) {
		return true
	}
	for _, target := range domain.PoolTeamTargets(team) {
		if actorDepartmentMatches(actor, target.Department) &&
			stringIn(target.Team, actor.ManagedTeams, actor.FrontendAccess.ManagedTeams, actor.FrontendAccess.TeamCodes) {
			return true
		}
	}
	return false
}

func actorDepartmentMatches(actor domain.RequestActor, department string) bool {
	return strings.EqualFold(strings.TrimSpace(actor.Department), department) ||
		stringIn(department, actor.ManagedDepartments, actor.FrontendAccess.ManagedDepartments, actor.FrontendAccess.DepartmentCodes)
}

func hasRole(actor domain.RequestActor, role domain.Role) bool {
	for _, candidate := range actor.Roles {
		if candidate == role {
			return true
		}
	}
	for _, candidate := range actor.FrontendAccess.Roles {
		if strings.EqualFold(candidate, string(role)) {
			return true
		}
	}
	return false
}

func stringIn(value string, groups ...[]string) bool {
	value = strings.TrimSpace(value)
	for _, group := range groups {
		for _, item := range group {
			if strings.EqualFold(value, strings.TrimSpace(item)) {
				return true
			}
		}
	}
	return false
}
