package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

type CreateOrgDepartmentParams struct {
	Name string
}

type UpdateOrgDepartmentParams struct {
	ID      int64
	Name    *string
	Enabled *bool
}

type CreateOrgTeamParams struct {
	DepartmentID *int64
	Department   string
	Name         string
}

type UpdateOrgTeamParams struct {
	ID      int64
	Name    *string
	Enabled *bool
}

type MergeOrgDepartmentParams struct {
	SourceID int64
	TargetID int64
}

type MergeOrgTeamParams struct {
	SourceID int64
	TargetID int64
}

func WithOrgRepo(orgRepo repo.OrgRepo) IdentityServiceOption {
	return func(s *identityService) {
		s.orgRepo = orgRepo
	}
}

func (s *identityService) CreateDepartment(ctx context.Context, p CreateOrgDepartmentParams) (*domain.OrgDepartment, *domain.AppError) {
	if s.orgRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "org master backend is not configured", nil)
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "department name is required", nil)
	}
	if existing, err := s.orgRepo.GetDepartmentByName(ctx, name); err != nil {
		return nil, infraError("get org department by name", err)
	} else if existing != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "department already exists", map[string]interface{}{"department": name})
	}

	item := &domain.OrgDepartment{Name: name, Enabled: true}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		id, err := s.orgRepo.CreateDepartment(ctx, tx, item)
		if err != nil {
			return err
		}
		item.ID = id
		return nil
	}); err != nil {
		return nil, infraError("create org department", err)
	}
	s.refreshRuntimeOrgCatalogLogged(ctx, "create_department")
	return s.getDepartmentByID(ctx, item.ID)
}

func (s *identityService) UpdateDepartment(ctx context.Context, p UpdateOrgDepartmentParams) (*domain.OrgDepartment, *domain.AppError) {
	if s.orgRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "org master backend is not configured", nil)
	}
	if p.ID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "department id is required", nil)
	}
	current, appErr := s.getDepartmentByID(ctx, p.ID)
	if appErr != nil {
		return nil, appErr
	}
	if p.Name == nil && p.Enabled == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "部门名称或启用状态至少需要填写一项。", nil)
	}
	originalName := current.Name
	disableRequested := p.Enabled != nil && !*p.Enabled
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if name == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "部门名称不能为空。", map[string]interface{}{"deny_code": "department_name_required"})
		}
		if disableRequested && name != current.Name {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "停用部门时不能同时修改名称。", map[string]interface{}{"deny_code": "org_delete_rename_conflict"})
		}
		if !strings.EqualFold(name, current.Name) {
			if existing, err := s.orgRepo.GetDepartmentByName(ctx, name); err != nil {
				return nil, infraError("get org department by name", err)
			} else if existing != nil && existing.ID != current.ID {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "部门名称已存在，请换一个名称。", map[string]interface{}{"deny_code": "department_name_conflict", "department": name})
			}
			current.Name = name
		}
	}
	nextEnabled := current.Enabled
	if p.Enabled != nil {
		nextEnabled = *p.Enabled
	}
	if current.Name == originalName && current.Enabled == nextEnabled {
		return current, nil
	}
	if !nextEnabled && current.Name == string(domain.DepartmentUnassigned) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "未分配是系统归属，不能停用。", map[string]interface{}{"deny_code": "system_org_delete_denied"})
	}
	var unassignedTeam string
	if !nextEnabled {
		unassigned, appErr := s.defaultUnassignedPoolTeam()
		if appErr != nil {
			return nil, appErr
		}
		unassignedTeam = unassigned
	}
	current.Enabled = nextEnabled
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if current.Name != originalName {
			if err := s.rewriteUsersForDepartmentRename(ctx, tx, originalName, current.Name); err != nil {
				return err
			}
		}
		if !current.Enabled {
			if err := s.moveDepartmentUsersToUnassigned(ctx, tx, current.ID, originalName, unassignedTeam); err != nil {
				return err
			}
			if err := s.disableTeamsForDepartment(ctx, tx, current.ID); err != nil {
				return err
			}
		}
		return s.orgRepo.UpdateDepartment(ctx, tx, current)
	}); err != nil {
		return nil, infraError("update org department", err)
	}
	s.refreshRuntimeOrgCatalogLogged(ctx, "update_department")
	return s.getDepartmentByID(ctx, current.ID)
}

func (s *identityService) CreateTeam(ctx context.Context, p CreateOrgTeamParams) (*domain.OrgTeam, *domain.AppError) {
	if s.orgRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "org master backend is not configured", nil)
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "team name is required", nil)
	}
	department, appErr := s.resolveDepartmentForTeamWrite(ctx, p.DepartmentID, p.Department)
	if appErr != nil {
		return nil, appErr
	}
	// Department-scoped lookup backed by uq_org_teams_department_name; the DB
	// unique key remains the concurrency backstop.
	if existing, err := s.orgRepo.GetTeamByDepartmentAndName(ctx, department.ID, name); err != nil {
		return nil, infraError("get org team by department and name", err)
	} else if existing != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "team already exists", map[string]interface{}{
			"department": department.Name,
			"team":       name,
		})
	}
	item := &domain.OrgTeam{
		DepartmentID: department.ID,
		Name:         name,
		Enabled:      true,
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		id, err := s.orgRepo.CreateTeam(ctx, tx, item)
		if err != nil {
			return err
		}
		item.ID = id
		return nil
	}); err != nil {
		return nil, infraError("create org team", err)
	}
	s.refreshRuntimeOrgCatalogLogged(ctx, "create_team")
	return s.getTeamByID(ctx, item.ID)
}

func (s *identityService) UpdateTeam(ctx context.Context, p UpdateOrgTeamParams) (*domain.OrgTeam, *domain.AppError) {
	if s.orgRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "org master backend is not configured", nil)
	}
	if p.ID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "team id is required", nil)
	}
	current, appErr := s.getTeamByID(ctx, p.ID)
	if appErr != nil {
		return nil, appErr
	}
	if p.Name == nil && p.Enabled == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "小组名称或启用状态至少需要填写一项。", nil)
	}
	originalName := current.Name
	disableRequested := p.Enabled != nil && !*p.Enabled
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if name == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "小组名称不能为空。", map[string]interface{}{"deny_code": "team_name_required"})
		}
		if disableRequested && name != current.Name {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "停用小组时不能同时修改名称。", map[string]interface{}{"deny_code": "org_delete_rename_conflict"})
		}
		if name != current.Name {
			if appErr := s.ensureTeamNameAvailable(ctx, current.DepartmentID, name, current.ID); appErr != nil {
				return nil, appErr
			}
			current.Name = name
		}
	}
	nextEnabled := current.Enabled
	if p.Enabled != nil {
		nextEnabled = *p.Enabled
	}
	if current.Name == originalName && current.Enabled == nextEnabled {
		return current, nil
	}
	if nextEnabled && !current.Enabled {
		// Restoring a team must not produce an enabled team under a disabled
		// department; the same rule the frontend enforces is validated here.
		department, appErr := s.getDepartmentByID(ctx, current.DepartmentID)
		if appErr != nil {
			return nil, appErr
		}
		if !department.Enabled {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "所属部门已停用，请先恢复部门再恢复小组。", map[string]interface{}{
				"deny_code":  "team_restore_department_disabled",
				"department": department.Name,
			})
		}
	}
	var unassignedTeam string
	if !nextEnabled {
		unassigned, appErr := s.defaultUnassignedPoolTeam()
		if appErr != nil {
			return nil, appErr
		}
		unassignedTeam = unassigned
		if current.Department == string(domain.DepartmentUnassigned) && current.Name == unassignedTeam {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "未分配池是系统归属，不能停用。", map[string]interface{}{"deny_code": "system_org_delete_denied"})
		}
	}
	current.Enabled = nextEnabled
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if current.Name != originalName {
			if err := s.rewriteUsersForTeamRename(ctx, tx, current.Department, originalName, current.Name); err != nil {
				return err
			}
		}
		if !current.Enabled {
			if err := s.moveTeamUsersToUnassigned(ctx, tx, current.Department, originalName, unassignedTeam); err != nil {
				return err
			}
		}
		return s.orgRepo.UpdateTeam(ctx, tx, current)
	}); err != nil {
		return nil, infraError("update org team", err)
	}
	s.refreshRuntimeOrgCatalogLogged(ctx, "update_team")
	return s.getTeamByID(ctx, current.ID)
}

// MergeDepartment moves every member and managed-scope reference of the
// source department onto the target department, then disables the source.
// Members whose team has no enabled same-name team under the target become
// ungrouped inside the target department.
func (s *identityService) MergeDepartment(ctx context.Context, p MergeOrgDepartmentParams) (*domain.OrgDepartment, *domain.AppError) {
	if s.orgRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "org master backend is not configured", nil)
	}
	if p.SourceID <= 0 || p.TargetID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "请选择要合并的部门与目标部门。", map[string]interface{}{"deny_code": "org_merge_target_required"})
	}
	if p.SourceID == p.TargetID {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "合并的源部门与目标部门不能相同。", map[string]interface{}{"deny_code": "org_merge_same_target"})
	}
	source, appErr := s.getDepartmentByID(ctx, p.SourceID)
	if appErr != nil {
		return nil, appErr
	}
	target, appErr := s.getDepartmentByID(ctx, p.TargetID)
	if appErr != nil {
		return nil, appErr
	}
	if source.Name == string(domain.DepartmentUnassigned) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "未分配是系统归属，不能合并。", map[string]interface{}{"deny_code": "system_org_merge_denied"})
	}
	if !target.Enabled {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "目标部门已停用，请先恢复目标部门再合并。", map[string]interface{}{"deny_code": "org_merge_target_disabled", "department": target.Name})
	}
	sourceTeams, err := s.teamNamesForDepartment(ctx, source.ID)
	if err != nil {
		return nil, infraError("list source department teams for merge", err)
	}
	enabledTeams, err := s.orgRepo.ListTeams(ctx, false)
	if err != nil {
		return nil, infraError("list org teams for merge", err)
	}
	targetEnabledTeams := map[string]struct{}{}
	for _, team := range enabledTeams {
		if team == nil || team.DepartmentID != target.ID {
			continue
		}
		if name := strings.TrimSpace(team.Name); name != "" {
			targetEnabledTeams[name] = struct{}{}
		}
	}
	// Source-only teams disappear with the merge; drop them from managed scopes.
	removedTeams := map[string]struct{}{}
	for name := range sourceTeams {
		if _, ok := targetEnabledTeams[name]; !ok {
			removedTeams[name] = struct{}{}
		}
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.rewriteAllUsers(ctx, tx, func(user *domain.User) bool {
			changed := false
			sourceDepartmentScoped := userReferencesDepartmentName(user, source.Name)
			if strings.TrimSpace(string(user.Department)) == source.Name {
				user.Department = domain.Department(target.Name)
				if _, ok := targetEnabledTeams[strings.TrimSpace(user.Team)]; !ok {
					user.Team = ""
				}
				changed = true
			}
			if next := replaceStringValue(user.ManagedDepartments, source.Name, target.Name); !stringSlicesEqual(next, user.ManagedDepartments) {
				user.ManagedDepartments = next
				changed = true
			}
			if sourceDepartmentScoped {
				next := removeStringValues(user.ManagedTeams, removedTeams)
				if stringSlicesEqual(next, user.ManagedTeams) {
					return changed
				}
				user.ManagedTeams = next
				changed = true
			}
			return changed
		}); err != nil {
			return err
		}
		if err := s.disableTeamsForDepartment(ctx, tx, source.ID); err != nil {
			return err
		}
		source.Enabled = false
		return s.orgRepo.UpdateDepartment(ctx, tx, source)
	}); err != nil {
		return nil, infraError("merge org department", err)
	}
	s.refreshRuntimeOrgCatalogLogged(ctx, "merge_department")
	return s.getDepartmentByID(ctx, target.ID)
}

// MergeTeam moves every member of the source team into the target team, then
// disables the source team. Managed-scope team references follow the rename.
func (s *identityService) MergeTeam(ctx context.Context, p MergeOrgTeamParams) (*domain.OrgTeam, *domain.AppError) {
	if s.orgRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "org master backend is not configured", nil)
	}
	if p.SourceID <= 0 || p.TargetID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "请选择要合并的小组与目标小组。", map[string]interface{}{"deny_code": "org_merge_target_required"})
	}
	if p.SourceID == p.TargetID {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "合并的源小组与目标小组不能相同。", map[string]interface{}{"deny_code": "org_merge_same_target"})
	}
	source, appErr := s.getTeamByID(ctx, p.SourceID)
	if appErr != nil {
		return nil, appErr
	}
	target, appErr := s.getTeamByID(ctx, p.TargetID)
	if appErr != nil {
		return nil, appErr
	}
	if source.Department == string(domain.DepartmentUnassigned) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "未分配池是系统归属，不能合并。", map[string]interface{}{"deny_code": "system_org_merge_denied"})
	}
	if !target.Enabled {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "目标小组已停用，请先恢复目标小组再合并。", map[string]interface{}{"deny_code": "org_merge_target_disabled", "team": target.Name})
	}
	targetDepartment, appErr := s.getDepartmentByID(ctx, target.DepartmentID)
	if appErr != nil {
		return nil, appErr
	}
	if !targetDepartment.Enabled {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "目标小组所属部门已停用，请先恢复部门再合并。", map[string]interface{}{"deny_code": "org_merge_target_disabled", "department": targetDepartment.Name})
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.rewriteAllUsers(ctx, tx, func(user *domain.User) bool {
			changed := false
			sourceDepartmentScoped := userReferencesDepartmentName(user, source.Department)
			if strings.TrimSpace(string(user.Department)) == source.Department && strings.TrimSpace(user.Team) == source.Name {
				user.Department = domain.Department(target.Department)
				user.Team = target.Name
				changed = true
			}
			if sourceDepartmentScoped {
				next := replaceStringValue(user.ManagedTeams, source.Name, target.Name)
				if stringSlicesEqual(next, user.ManagedTeams) {
					return changed
				}
				user.ManagedTeams = next
				changed = true
			}
			return changed
		}); err != nil {
			return err
		}
		source.Enabled = false
		return s.orgRepo.UpdateTeam(ctx, tx, source)
	}); err != nil {
		return nil, infraError("merge org team", err)
	}
	s.refreshRuntimeOrgCatalogLogged(ctx, "merge_team")
	return s.getTeamByID(ctx, target.ID)
}

// DeleteDepartment hard-deletes one non-system department together with its
// child teams. Assigned users are moved to the system unassigned pool first, so
// organization cleanup never depends on manual member migration.
func (s *identityService) DeleteDepartment(ctx context.Context, id int64) *domain.AppError {
	if s.orgRepo == nil {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "org master backend is not configured", nil)
	}
	if id <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "department id is required", nil)
	}
	current, appErr := s.getDepartmentByID(ctx, id)
	if appErr != nil {
		return appErr
	}
	if current.Name == string(domain.DepartmentUnassigned) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "未分配是系统归属，不能删除。", map[string]interface{}{"deny_code": "system_org_delete_denied"})
	}
	unassignedTeam, appErr := s.defaultUnassignedPoolTeam()
	if appErr != nil {
		return appErr
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.moveDepartmentUsersToUnassigned(ctx, tx, current.ID, current.Name, unassignedTeam); err != nil {
			return err
		}
		if err := s.orgRepo.DeleteTeamsByDepartment(ctx, tx, current.ID); err != nil {
			return err
		}
		return s.orgRepo.DeleteDepartment(ctx, tx, current.ID)
	}); err != nil {
		return infraError("delete org department", err)
	}
	s.refreshRuntimeOrgCatalogLogged(ctx, "delete_department")
	return nil
}

// DeleteTeam hard-deletes one non-system team. Assigned users are moved to the
// system unassigned pool first, so organization cleanup never depends on manual
// member migration.
func (s *identityService) DeleteTeam(ctx context.Context, id int64) *domain.AppError {
	if s.orgRepo == nil {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "org master backend is not configured", nil)
	}
	if id <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "team id is required", nil)
	}
	current, appErr := s.getTeamByID(ctx, id)
	if appErr != nil {
		return appErr
	}
	if current.Department == string(domain.DepartmentUnassigned) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "未分配池是系统归属，不能删除。", map[string]interface{}{"deny_code": "system_org_delete_denied"})
	}
	unassignedTeam, appErr := s.defaultUnassignedPoolTeam()
	if appErr != nil {
		return appErr
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.moveTeamUsersToUnassigned(ctx, tx, current.Department, current.Name, unassignedTeam); err != nil {
			return err
		}
		return s.orgRepo.DeleteTeam(ctx, tx, current.ID)
	}); err != nil {
		return infraError("delete org team", err)
	}
	s.refreshRuntimeOrgCatalogLogged(ctx, "delete_team")
	return nil
}

func (s *identityService) ensureTeamNameAvailable(ctx context.Context, departmentID int64, name string, excludeTeamID int64) *domain.AppError {
	existing, err := s.orgRepo.GetTeamByDepartmentAndName(ctx, departmentID, strings.TrimSpace(name))
	if err != nil {
		return infraError("get org team by department and name", err)
	}
	if existing != nil && existing.ID != excludeTeamID {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "该部门下已存在同名小组，请换一个名称。", map[string]interface{}{"deny_code": "team_name_conflict", "team": name})
	}
	return nil
}

func (s *identityService) rewriteUsersForDepartmentRename(ctx context.Context, tx repo.Tx, oldName, newName string) error {
	return s.rewriteAllUsers(ctx, tx, func(user *domain.User) bool {
		changed := false
		if strings.TrimSpace(string(user.Department)) == oldName {
			user.Department = domain.Department(newName)
			changed = true
		}
		if next := replaceStringValue(user.ManagedDepartments, oldName, newName); !stringSlicesEqual(next, user.ManagedDepartments) {
			user.ManagedDepartments = next
			changed = true
		}
		return changed
	})
}

func (s *identityService) rewriteUsersForTeamRename(ctx context.Context, tx repo.Tx, department, oldName, newName string) error {
	return s.rewriteAllUsers(ctx, tx, func(user *domain.User) bool {
		changed := false
		sourceDepartmentScoped := userReferencesDepartmentName(user, department)
		if strings.TrimSpace(string(user.Department)) == department && strings.TrimSpace(user.Team) == oldName {
			user.Team = newName
			changed = true
		}
		if sourceDepartmentScoped {
			next := replaceStringValue(user.ManagedTeams, oldName, newName)
			if stringSlicesEqual(next, user.ManagedTeams) {
				return changed
			}
			user.ManagedTeams = next
			changed = true
		}
		return changed
	})
}

func (s *identityService) moveDepartmentUsersToUnassigned(ctx context.Context, tx repo.Tx, departmentID int64, departmentName, unassignedTeam string) error {
	teamNames, err := s.teamNamesForDepartment(ctx, departmentID)
	if err != nil {
		return err
	}
	return s.rewriteAllUsers(ctx, tx, func(user *domain.User) bool {
		changed := false
		sourceDepartmentScoped := userReferencesDepartmentName(user, departmentName)
		if strings.TrimSpace(string(user.Department)) == departmentName {
			user.Department = domain.DepartmentUnassigned
			user.Team = unassignedTeam
			changed = true
		}
		if next := removeStringValue(user.ManagedDepartments, departmentName); !stringSlicesEqual(next, user.ManagedDepartments) {
			user.ManagedDepartments = next
			changed = true
		}
		if sourceDepartmentScoped {
			next := removeStringValues(user.ManagedTeams, teamNames)
			if stringSlicesEqual(next, user.ManagedTeams) {
				return changed
			}
			user.ManagedTeams = next
			changed = true
		}
		return changed
	})
}

func (s *identityService) moveTeamUsersToUnassigned(ctx context.Context, tx repo.Tx, department, team, unassignedTeam string) error {
	return s.rewriteAllUsers(ctx, tx, func(user *domain.User) bool {
		changed := false
		sourceDepartmentScoped := userReferencesDepartmentName(user, department)
		if strings.TrimSpace(string(user.Department)) == department && strings.TrimSpace(user.Team) == team {
			user.Department = domain.DepartmentUnassigned
			user.Team = unassignedTeam
			changed = true
		}
		if sourceDepartmentScoped {
			next := removeStringValue(user.ManagedTeams, team)
			if stringSlicesEqual(next, user.ManagedTeams) {
				return changed
			}
			user.ManagedTeams = next
			changed = true
		}
		return changed
	})
}

func (s *identityService) disableTeamsForDepartment(ctx context.Context, tx repo.Tx, departmentID int64) error {
	teams, err := s.orgRepo.ListTeams(ctx, true)
	if err != nil {
		return fmt.Errorf("list org teams for department disable: %w", err)
	}
	for _, team := range teams {
		if team == nil || team.DepartmentID != departmentID || !team.Enabled {
			continue
		}
		team.Enabled = false
		if err := s.orgRepo.UpdateTeam(ctx, tx, team); err != nil {
			return err
		}
	}
	return nil
}

func (s *identityService) teamNamesForDepartment(ctx context.Context, departmentID int64) (map[string]struct{}, error) {
	teams, err := s.orgRepo.ListTeams(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("list org teams for department users: %w", err)
	}
	out := make(map[string]struct{})
	for _, team := range teams {
		if team == nil || team.DepartmentID != departmentID {
			continue
		}
		if name := strings.TrimSpace(team.Name); name != "" {
			out[name] = struct{}{}
		}
	}
	return out, nil
}

func userReferencesDepartmentName(user *domain.User, department string) bool {
	if user == nil {
		return false
	}
	department = strings.TrimSpace(department)
	if department == "" {
		return false
	}
	if strings.TrimSpace(string(user.Department)) == department {
		return true
	}
	for _, managed := range user.ManagedDepartments {
		if strings.TrimSpace(managed) == department {
			return true
		}
	}
	return false
}

// rewriteAllUsers snapshots the full user table once and rewrites only the
// rows the mutate callback reports as changed. Org writes previously ran two
// passes (org members, then managed scopes) which updated the same row twice
// inside a single transaction; one combined pass keeps every row's update
// atomic and halves the transaction's write volume.
func (s *identityService) rewriteAllUsers(ctx context.Context, tx repo.Tx, mutate func(*domain.User) bool) error {
	const pageSize = 500
	allUsers := make([]*domain.User, 0)
	for page := 1; ; page++ {
		users, _, err := s.userRepo.List(ctx, repo.UserListFilter{
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			return err
		}
		if len(users) == 0 {
			break
		}
		allUsers = append(allUsers, users...)
		if len(users) < pageSize {
			break
		}
	}
	for _, user := range allUsers {
		if user == nil || !mutate(user) {
			continue
		}
		user.UpdatedAt = time.Now().UTC()
		if err := s.userRepo.Update(ctx, tx, user); err != nil {
			return err
		}
	}
	return nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func replaceStringValue(values []string, oldValue, newValue string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		next := strings.TrimSpace(value)
		if next == "" {
			continue
		}
		if next == oldValue {
			next = newValue
		}
		if _, ok := seen[next]; ok {
			continue
		}
		seen[next] = struct{}{}
		out = append(out, next)
	}
	return out
}

func removeStringValue(values []string, remove string) []string {
	removeSet := map[string]struct{}{remove: {}}
	return removeStringValues(values, removeSet)
}

func removeStringValues(values []string, remove map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		next := strings.TrimSpace(value)
		if next == "" {
			continue
		}
		if _, shouldRemove := remove[next]; shouldRemove {
			continue
		}
		if _, ok := seen[next]; ok {
			continue
		}
		seen[next] = struct{}{}
		out = append(out, next)
	}
	return out
}

func (s *identityService) syncOrgMasterData(ctx context.Context) *domain.AppError {
	if s.orgRepo == nil {
		return nil
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		departmentsByName := map[string]*domain.OrgDepartment{}
		existingDepartments, err := s.orgRepo.ListDepartments(ctx, true)
		if err != nil {
			return err
		}
		for _, department := range existingDepartments {
			if department != nil {
				departmentsByName[strings.TrimSpace(department.Name)] = department
			}
		}

		teamsByDepartmentKey := map[string]*domain.OrgTeam{}
		existingTeams, err := s.orgRepo.ListTeams(ctx, true)
		if err != nil {
			return err
		}
		for _, team := range existingTeams {
			if team != nil {
				teamsByDepartmentKey[departmentScopedTeamKey(team.DepartmentID, team.Name)] = team
			}
		}

		// First-time initialization seeds the full configured baseline. Once
		// org master holds any department, startup sync stops re-creating
		// rows from static seeds (which used to resurrect renamed or retired
		// legacy departments/teams on every restart) and only guarantees the
		// system unassigned bucket plus org references required by configured
		// accounts.
		var seedDepartmentNames []string
		var seedDepartmentTeams map[string][]string
		if len(departmentsByName) > 0 {
			seedDepartmentNames, seedDepartmentTeams = s.requiredOrgSeedReferences()
		} else {
			// First-time initialization: seed the configured layout minus
			// compatibility entries, then union with the references required
			// by configured accounts so a config that intentionally places an
			// account in a retired department still boots.
			seedDepartmentNames, seedDepartmentTeams = filterCompatibilityOrgSeeds(s.seedOrgDepartmentNames(), s.authSettings.DepartmentTeams)
			requiredNames, requiredTeams := s.requiredOrgSeedReferences()
			for _, name := range requiredNames {
				seedDepartmentNames = appendUniqueString(seedDepartmentNames, name)
			}
			for department, teams := range requiredTeams {
				for _, team := range teams {
					seedDepartmentTeams[department] = appendUniqueString(seedDepartmentTeams[department], team)
				}
			}
		}

		for _, name := range seedDepartmentNames {
			if _, ok := departmentsByName[name]; ok {
				continue
			}
			item := &domain.OrgDepartment{Name: name, Enabled: true}
			id, err := s.orgRepo.CreateDepartment(ctx, tx, item)
			if err != nil {
				return err
			}
			item.ID = id
			departmentsByName[name] = item
		}

		for departmentName, teams := range seedDepartmentTeams {
			departmentName = strings.TrimSpace(departmentName)
			if departmentName == "" {
				continue
			}
			department := departmentsByName[departmentName]
			if department == nil {
				continue
			}
			for _, teamName := range teams {
				teamName = strings.TrimSpace(teamName)
				if teamName == "" {
					continue
				}
				teamKey := departmentScopedTeamKey(department.ID, teamName)
				if _, ok := teamsByDepartmentKey[teamKey]; ok {
					continue
				}
				item := &domain.OrgTeam{DepartmentID: department.ID, Name: teamName, Enabled: true}
				id, err := s.orgRepo.CreateTeam(ctx, tx, item)
				if err != nil {
					return err
				}
				item.ID = id
				item.Department = departmentName
				teamsByDepartmentKey[teamKey] = item
			}
		}
		return nil
	}); err != nil {
		return infraError("sync org master data", err)
	}
	return s.refreshRuntimeOrgCatalog(ctx)
}

// requiredOrgSeedReferences returns the minimum org rows startup sync must
// guarantee on an already-initialized installation: the system unassigned
// bucket and any department/team referenced by configured super admins or
// configured assignments (so config-driven account upsert cannot fail).
func (s *identityService) requiredOrgSeedReferences() ([]string, map[string][]string) {
	departments := []string{string(domain.DepartmentUnassigned)}
	seen := map[string]struct{}{string(domain.DepartmentUnassigned): {}}
	poolTeams := make([]string, 0, 1)
	for _, team := range s.authSettings.DepartmentTeams[string(domain.DepartmentUnassigned)] {
		if trimmed := strings.TrimSpace(team); trimmed != "" {
			poolTeams = appendUniqueString(poolTeams, trimmed)
		}
	}
	if len(poolTeams) == 0 {
		poolTeams = []string{"未分配池"}
	}
	teams := map[string][]string{string(domain.DepartmentUnassigned): poolTeams}
	addReference := func(department domain.Department, team string) {
		name := strings.TrimSpace(string(department))
		if name == "" {
			return
		}
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			departments = append(departments, name)
		}
		if trimmed := strings.TrimSpace(team); trimmed != "" {
			teams[name] = appendUniqueString(teams[name], trimmed)
		}
	}
	for _, entry := range s.authSettings.SuperAdmins {
		addReference(entry.Department, entry.Team)
	}
	for _, entry := range s.authSettings.ConfiguredAssignments {
		addReference(entry.Department, entry.Team)
	}
	return departments, teams
}

// filterCompatibilityOrgSeeds strips retired departments and retired team
// names from a first-init seeding plan so a fresh installation never starts
// out with legacy dirty data, even when the runtime auth settings still list
// those entries for validation compatibility.
func filterCompatibilityOrgSeeds(departmentNames []string, departmentTeams map[string][]string) ([]string, map[string][]string) {
	retiredDepartments := map[string]struct{}{}
	for _, department := range domain.CompatibilityDepartments() {
		retiredDepartments[strings.TrimSpace(string(department))] = struct{}{}
	}
	retiredTeams := map[string]map[string]struct{}{}
	for department, teams := range domain.CompatibilityOrgDepartmentTeams() {
		set := make(map[string]struct{}, len(teams))
		for _, team := range teams {
			set[strings.TrimSpace(team)] = struct{}{}
		}
		retiredTeams[strings.TrimSpace(department)] = set
	}

	filteredNames := make([]string, 0, len(departmentNames))
	for _, name := range departmentNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, retired := retiredDepartments[name]; retired {
			continue
		}
		filteredNames = append(filteredNames, name)
	}

	filteredTeams := make(map[string][]string, len(departmentTeams))
	for department, teams := range departmentTeams {
		department = strings.TrimSpace(department)
		if _, retired := retiredDepartments[department]; retired {
			continue
		}
		retiredSet := retiredTeams[department]
		kept := make([]string, 0, len(teams))
		for _, team := range teams {
			team = strings.TrimSpace(team)
			if team == "" {
				continue
			}
			if retiredSet != nil {
				if _, retired := retiredSet[team]; retired {
					continue
				}
			}
			kept = appendUniqueString(kept, team)
		}
		if len(kept) > 0 {
			filteredTeams[department] = kept
		}
	}
	return filteredNames, filteredTeams
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// refreshRuntimeOrgCatalogLogged refreshes the runtime org catalog after an
// org-master write and logs (instead of silently discarding) any failure so a
// stale runtime catalog is observable.
func (s *identityService) refreshRuntimeOrgCatalogLogged(ctx context.Context, op string) {
	if appErr := s.refreshRuntimeOrgCatalog(ctx); appErr != nil {
		s.logger.Warn("refresh runtime org catalog failed; runtime catalog may be stale until the next org write",
			zap.String("op", op),
			zap.Error(appErr),
		)
	}
}

func (s *identityService) refreshRuntimeOrgCatalog(ctx context.Context) *domain.AppError {
	s.orgOptionsOnce = sync.Once{}
	s.orgOptionsCache = nil
	if s.orgRepo == nil {
		ConfigureTaskOrgCatalog(s.authSettings)
		return nil
	}
	options, appErr := s.buildOrgOptions(ctx, false)
	if appErr != nil {
		return appErr
	}
	settings := s.authSettings
	settings.Departments = make([]domain.Department, 0, len(options.Departments))
	settings.DepartmentTeams = make(map[string][]string, len(options.TeamsByDepartment))
	for _, department := range options.Departments {
		settings.Departments = append(settings.Departments, domain.Department(department.Name))
		settings.DepartmentTeams[department.Name] = append([]string{}, options.TeamsByDepartment[department.Name]...)
	}
	ConfigureTaskOrgCatalog(settings)
	return nil
}

func (s *identityService) buildOrgOptions(ctx context.Context, includeDisabled bool) (*domain.OrgOptions, *domain.AppError) {
	if s.orgRepo == nil {
		return s.buildConfigBackedOrgOptions(ctx), nil
	}
	departments, err := s.orgRepo.ListDepartments(ctx, includeDisabled)
	if err != nil {
		return nil, infraError("list org departments", err)
	}
	teams, err := s.orgRepo.ListTeams(ctx, includeDisabled)
	if err != nil {
		return nil, infraError("list org teams", err)
	}
	memberCounts, appErr := s.collectOrgMemberCounts(ctx)
	if appErr != nil {
		return nil, appErr
	}

	teamsByDepartmentID := map[int64][]domain.OrgTeamOption{}
	teamsByDepartmentName := map[string][]string{}
	for _, team := range teams {
		if team == nil {
			continue
		}
		teamsByDepartmentID[team.DepartmentID] = append(teamsByDepartmentID[team.DepartmentID], domain.OrgTeamOption{
			ID:          team.ID,
			Name:        team.Name,
			Enabled:     team.Enabled,
			MemberCount: memberCounts.teams[orgMemberCountKey(team.Department, team.Name)],
		})
		teamsByDepartmentName[team.Department] = append(teamsByDepartmentName[team.Department], team.Name)
	}

	options := &domain.OrgOptions{
		Departments:           make([]domain.DepartmentOption, 0, len(departments)),
		TeamsByDepartment:     make(map[string][]string, len(departments)),
		RoleCatalogSummary:    s.ListRoles(ctx),
		UnassignedPoolEnabled: s.authSettings.UnassignedPoolEnabled,
		ConfiguredAssignments: append([]domain.ConfiguredUserAssignment{}, s.authSettings.ConfiguredAssignments...),
	}
	for _, department := range departments {
		if department == nil {
			continue
		}
		teamItems := append([]domain.OrgTeamOption{}, teamsByDepartmentID[department.ID]...)
		sort.Slice(teamItems, func(i, j int) bool { return teamItems[i].ID < teamItems[j].ID })
		teamNames := append([]string{}, teamsByDepartmentName[department.Name]...)
		options.Departments = append(options.Departments, domain.DepartmentOption{
			ID:          department.ID,
			Name:        department.Name,
			Teams:       teamNames,
			TeamItems:   teamItems,
			Enabled:     department.Enabled,
			MemberCount: memberCounts.departments[strings.TrimSpace(department.Name)],
		})
		options.TeamsByDepartment[department.Name] = teamNames
	}
	return options, nil
}

type orgMemberCounts struct {
	departments map[string]int
	teams       map[string]int
}

func orgMemberCountKey(department, team string) string {
	return strings.TrimSpace(department) + "\x00" + strings.TrimSpace(team)
}

// collectOrgMemberCounts aggregates user membership per department and per
// department-scoped team in one pass over the user table, so org options can
// expose member_count badges (and make zero-member legacy orgs obvious).
func (s *identityService) collectOrgMemberCounts(ctx context.Context) (orgMemberCounts, *domain.AppError) {
	counts := orgMemberCounts{
		departments: map[string]int{},
		teams:       map[string]int{},
	}
	// 注意:MySQL 仓储层 normalizePage 会把 >100 的 page_size 钳到 20,
	// 因此这里必须用仓储允许的最大页宽,并以返回的 total 作为终止条件,
	// 否则只会统计到第一页,人数徽标恒为 0/偏小。
	const pageSize = 100
	seen := 0
	for page := 1; ; page++ {
		users, total, err := s.userRepo.List(ctx, repo.UserListFilter{
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			return counts, infraError("list users for org member counts", err)
		}
		for _, user := range users {
			if user == nil {
				continue
			}
			department := strings.TrimSpace(string(user.Department))
			if department == "" {
				continue
			}
			counts.departments[department]++
			if team := strings.TrimSpace(user.Team); team != "" {
				counts.teams[orgMemberCountKey(department, team)]++
			}
		}
		seen += len(users)
		if len(users) == 0 || seen >= int(total) {
			break
		}
	}
	return counts, nil
}

func (s *identityService) buildConfigBackedOrgOptions(ctx context.Context) *domain.OrgOptions {
	options := &domain.OrgOptions{
		Departments:           make([]domain.DepartmentOption, 0, len(s.authSettings.Departments)),
		TeamsByDepartment:     make(map[string][]string, len(s.authSettings.DepartmentTeams)),
		RoleCatalogSummary:    s.ListRoles(ctx),
		UnassignedPoolEnabled: s.authSettings.UnassignedPoolEnabled,
		ConfiguredAssignments: append([]domain.ConfiguredUserAssignment{}, s.authSettings.ConfiguredAssignments...),
	}
	for _, department := range s.authSettings.Departments {
		teams := append([]string{}, s.authSettings.DepartmentTeams[string(department)]...)
		options.Departments = append(options.Departments, domain.DepartmentOption{
			Name:    string(department),
			Teams:   teams,
			Enabled: true,
		})
		options.TeamsByDepartment[string(department)] = teams
	}
	return options
}

func (s *identityService) getDepartmentByID(ctx context.Context, id int64) (*domain.OrgDepartment, *domain.AppError) {
	item, err := s.orgRepo.GetDepartmentByID(ctx, id)
	if err != nil {
		return nil, infraError("get org department", err)
	}
	if item == nil {
		return nil, domain.ErrNotFound
	}
	return item, nil
}

func (s *identityService) getTeamByID(ctx context.Context, id int64) (*domain.OrgTeam, *domain.AppError) {
	item, err := s.orgRepo.GetTeamByID(ctx, id)
	if err != nil {
		return nil, infraError("get org team", err)
	}
	if item == nil {
		return nil, domain.ErrNotFound
	}
	return item, nil
}

func (s *identityService) resolveDepartmentForTeamWrite(ctx context.Context, departmentID *int64, departmentName string) (*domain.OrgDepartment, *domain.AppError) {
	if departmentID != nil && *departmentID > 0 {
		department, appErr := s.getDepartmentByID(ctx, *departmentID)
		if appErr != nil {
			return nil, appErr
		}
		if !department.Enabled {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "department is disabled", map[string]interface{}{"department_id": department.ID, "department": department.Name})
		}
		if trimmed := strings.TrimSpace(departmentName); trimmed != "" && trimmed != department.Name {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "department_id and department do not match", map[string]interface{}{"department_id": department.ID, "department": department.Name, "provided_department": trimmed})
		}
		return department, nil
	}
	trimmedName := strings.TrimSpace(departmentName)
	if trimmedName == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "department is required", nil)
	}
	department, err := s.orgRepo.GetDepartmentByName(ctx, trimmedName)
	if err != nil {
		return nil, infraError("get org department by name", err)
	}
	if department == nil || !department.Enabled {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "department is invalid", map[string]interface{}{"department": trimmedName})
	}
	return department, nil
}

func (s *identityService) seedOrgDepartmentNames() []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(s.authSettings.Departments)+len(s.authSettings.DepartmentTeams))
	for _, department := range s.authSettings.Departments {
		name := strings.TrimSpace(string(department))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for department := range s.authSettings.DepartmentTeams {
		name := strings.TrimSpace(department)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func newOrgRecord(id int64, name string, enabled bool) *domain.OrgDepartment {
	now := time.Now().UTC()
	return &domain.OrgDepartment{
		ID:        id,
		Name:      name,
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func departmentScopedTeamKey(departmentID int64, teamName string) string {
	return strconv.FormatInt(departmentID, 10) + "\x00" + strings.TrimSpace(teamName)
}
