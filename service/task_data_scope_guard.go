package service

import (
	"context"
	"log"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

// taskActorOrgSnapshot captures the departments & teams of the users who touched the task.
type taskActorOrgSnapshot struct {
	Departments []string
	Teams       []string
}

func (s *taskService) resolveDataScope(ctx context.Context) (*DataScope, *domain.AppError) {
	return resolveDataScopeForActor(ctx, s.dataScopeResolver, s.scopeUserRepo)
}

func resolveDataScopeForActor(ctx context.Context, resolver DataScopeResolver, userRepo repo.UserRepo) (*DataScope, *domain.AppError) {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok {
		log.Printf("WARN resolve_data_scope: no request actor in context — granting ViewAll for backward compatibility")
		return &DataScope{ViewAll: true}, nil
	}
	if actor.ID <= 0 {
		log.Printf("WARN resolve_data_scope: request actor has non-positive ID=%d — granting ViewAll for backward compatibility", actor.ID)
		return &DataScope{ViewAll: true}, nil
	}
	if resolver == nil {
		log.Printf("WARN resolve_data_scope: data scope resolver not injected for actor_id=%d — denying scope", actor.ID)
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "data scope resolver not configured", nil)
	}
	user := &domain.User{
		ID:                 actor.ID,
		Department:         domain.Department(normalizeTaskDepartmentCode(actor.Department)),
		Team:               strings.TrimSpace(actor.Team),
		ManagedDepartments: normalizeTaskDepartmentCodes(actor.ManagedDepartments),
		ManagedTeams:       append([]string(nil), actor.ManagedTeams...),
	}
	needsHydration := user.Department == "" && user.Team == "" &&
		len(user.ManagedDepartments) == 0 && len(user.ManagedTeams) == 0
	if needsHydration && userRepo != nil {
		loaded, err := userRepo.GetByID(ctx, actor.ID)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "load scoped user failed", map[string]interface{}{"error": err.Error()})
		}
		if loaded != nil {
			user = loaded
		}
	}
	user.Roles = append([]domain.Role(nil), actor.Roles...)
	scope, err := resolver.Resolve(ctx, user)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "resolve data scope failed", map[string]interface{}{"error": err.Error()})
	}
	if scope == nil {
		return &DataScope{ViewAll: true}, nil
	}
	return scope, nil
}

func canViewTaskWithScope(task *domain.Task, scope *DataScope) bool {
	return canViewTaskWithScopeAndActorOrg(task, scope, nil)
}

func canViewTaskWithScopeAndActorOrg(task *domain.Task, scope *DataScope, actorOrg *taskActorOrgSnapshot) bool {
	if task == nil {
		return false
	}
	if scope == nil || scope.ViewAll {
		return true
	}
	applyTaskReadModelOrgOwnership(task)
	if canViewTaskWithPrimaryScope(task, scope) {
		return true
	}
	if actorOrg == nil || !hasManagedOrgScope(scope) {
		return false
	}
	for _, department := range actorOrg.Departments {
		if department == "" {
			continue
		}
		for _, managed := range scope.ManagedDepartmentCodes {
			if managed != "" && managed == department {
				return true
			}
		}
	}
	for _, team := range actorOrg.Teams {
		if team == "" {
			continue
		}
		for _, managed := range scope.ManagedTeamCodes {
			if managed != "" && managed == team {
				return true
			}
		}
	}
	return false
}

func canViewTaskWithPrimaryScope(task *domain.Task, scope *DataScope) bool {
	if task == nil {
		return false
	}
	if scope == nil || scope.ViewAll {
		return true
	}
	if len(scope.UserIDs) > 0 {
		for _, uid := range scope.UserIDs {
			if uid <= 0 {
				continue
			}
			if task.CreatorID == uid {
				return true
			}
			if task.DesignerID != nil && *task.DesignerID == uid {
				return true
			}
			if task.CurrentHandlerID != nil && *task.CurrentHandlerID == uid {
				return true
			}
		}
	}
	if len(scope.DepartmentCodes) > 0 {
		for _, department := range scope.DepartmentCodes {
			if department != "" && department == task.OwnerDepartment {
				return true
			}
		}
	}
	if len(scope.TeamCodes) > 0 {
		for _, team := range scope.TeamCodes {
			if team != "" && team == task.OwnerOrgTeam {
				return true
			}
		}
	}
	return matchesAnyStageVisibility(task, scope.StageVisibilities)
}

func hasManagedOrgScope(scope *DataScope) bool {
	if scope == nil {
		return false
	}
	return len(scope.ManagedDepartmentCodes) > 0 || len(scope.ManagedTeamCodes) > 0
}

func resolveTaskActorOrgSnapshot(ctx context.Context, userRepo repo.UserRepo, task *domain.Task) (*taskActorOrgSnapshot, *domain.AppError) {
	if task == nil || userRepo == nil {
		return nil, nil
	}
	actorIDs := taskActorSnapshotUserIDs(task)
	if len(actorIDs) == 0 {
		return nil, nil
	}

	departmentSet := make(map[string]struct{}, len(actorIDs))
	teamSet := make(map[string]struct{}, len(actorIDs))
	snapshot := &taskActorOrgSnapshot{
		Departments: make([]string, 0, len(actorIDs)),
		Teams:       make([]string, 0, len(actorIDs)),
	}

	for _, actorID := range actorIDs {
		user, err := userRepo.GetByID(ctx, actorID)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "load task actor organization failed", map[string]interface{}{
				"user_id": actorID,
				"error":   err.Error(),
			})
		}
		if user == nil {
			continue
		}
		department := normalizeTaskDepartmentCode(string(user.Department))
		if department != "" {
			if _, exists := departmentSet[department]; !exists {
				departmentSet[department] = struct{}{}
				snapshot.Departments = append(snapshot.Departments, department)
			}
		}
		team := strings.TrimSpace(user.Team)
		if team != "" {
			if _, exists := teamSet[team]; !exists {
				teamSet[team] = struct{}{}
				snapshot.Teams = append(snapshot.Teams, team)
			}
		}
	}

	if len(snapshot.Departments) == 0 && len(snapshot.Teams) == 0 {
		return nil, nil
	}
	return snapshot, nil
}

func taskActorSnapshotUserIDs(task *domain.Task) []int64 {
	if task == nil {
		return nil
	}
	seen := map[int64]struct{}{}
	ids := make([]int64, 0, 3)
	appendID := func(id int64) {
		if id <= 0 {
			return
		}
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	appendID(task.CreatorID)
	if task.DesignerID != nil {
		appendID(*task.DesignerID)
	}
	if task.CurrentHandlerID != nil {
		appendID(*task.CurrentHandlerID)
	}
	return ids
}

func matchesAnyStageVisibility(task *domain.Task, visibilities []StageVisibility) bool {
	for _, visibility := range visibilities {
		if taskMatchesStageVisibility(task, visibility) {
			return true
		}
	}
	return false
}

func taskMatchesStageVisibility(task *domain.Task, visibility StageVisibility) bool {
	if task == nil {
		return false
	}
	matchesStatus := false
	for _, status := range visibility.Statuses {
		if task.TaskStatus == status {
			matchesStatus = true
			break
		}
	}
	if !matchesStatus {
		return false
	}
	if visibility.Lane == nil {
		return true
	}
	return task.WorkflowLane() == *visibility.Lane
}
