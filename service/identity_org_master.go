package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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
	_ = s.refreshRuntimeOrgCatalog(ctx)
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
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "删除部门时不能同时修改名称。", map[string]interface{}{"deny_code": "org_delete_rename_conflict"})
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
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "未分配是系统归属，不能删除。", map[string]interface{}{"deny_code": "system_org_delete_denied"})
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
	_ = s.refreshRuntimeOrgCatalog(ctx)
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
	existingTeams, err := s.orgRepo.ListTeams(ctx, true)
	if err != nil {
		return nil, infraError("list org teams for create", err)
	}
	for _, existing := range existingTeams {
		if existing == nil {
			continue
		}
		if existing.DepartmentID == department.ID && strings.TrimSpace(existing.Name) == name {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "team already exists", map[string]interface{}{
				"department": department.Name,
				"team":       name,
			})
		}
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
	_ = s.refreshRuntimeOrgCatalog(ctx)
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
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "删除小组时不能同时修改名称。", map[string]interface{}{"deny_code": "org_delete_rename_conflict"})
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
	var unassignedTeam string
	if !nextEnabled {
		unassigned, appErr := s.defaultUnassignedPoolTeam()
		if appErr != nil {
			return nil, appErr
		}
		unassignedTeam = unassigned
		if current.Department == string(domain.DepartmentUnassigned) && current.Name == unassignedTeam {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "未分配池是系统归属，不能删除。", map[string]interface{}{"deny_code": "system_org_delete_denied"})
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
	_ = s.refreshRuntimeOrgCatalog(ctx)
	return s.getTeamByID(ctx, current.ID)
}

func (s *identityService) ensureTeamNameAvailable(ctx context.Context, departmentID int64, name string, excludeTeamID int64) *domain.AppError {
	teams, err := s.orgRepo.ListTeams(ctx, true)
	if err != nil {
		return infraError("list org teams for name check", err)
	}
	for _, team := range teams {
		if team == nil || team.ID == excludeTeamID || team.DepartmentID != departmentID {
			continue
		}
		if strings.TrimSpace(team.Name) == strings.TrimSpace(name) {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "该部门下已存在同名小组，请换一个名称。", map[string]interface{}{"deny_code": "team_name_conflict", "team": name})
		}
	}
	return nil
}

func (s *identityService) rewriteUsersForDepartmentRename(ctx context.Context, tx repo.Tx, oldName, newName string) error {
	if err := s.rewriteUsersByOrg(ctx, tx, oldName, "", func(user *domain.User) {
		user.Department = domain.Department(newName)
		user.ManagedDepartments = replaceStringValue(user.ManagedDepartments, oldName, newName)
	}); err != nil {
		return err
	}
	return s.rewriteAllUserManagedScopes(ctx, tx, func(user *domain.User) bool {
		next := replaceStringValue(user.ManagedDepartments, oldName, newName)
		if stringSlicesEqual(next, user.ManagedDepartments) {
			return false
		}
		user.ManagedDepartments = next
		return true
	})
}

func (s *identityService) rewriteUsersForTeamRename(ctx context.Context, tx repo.Tx, department, oldName, newName string) error {
	if err := s.rewriteUsersByOrg(ctx, tx, department, oldName, func(user *domain.User) {
		user.Team = newName
		user.ManagedTeams = replaceStringValue(user.ManagedTeams, oldName, newName)
	}); err != nil {
		return err
	}
	return s.rewriteAllUserManagedScopes(ctx, tx, func(user *domain.User) bool {
		next := replaceStringValue(user.ManagedTeams, oldName, newName)
		if stringSlicesEqual(next, user.ManagedTeams) {
			return false
		}
		user.ManagedTeams = next
		return true
	})
}

func (s *identityService) moveDepartmentUsersToUnassigned(ctx context.Context, tx repo.Tx, departmentID int64, departmentName, unassignedTeam string) error {
	teamNames, err := s.teamNamesForDepartment(ctx, departmentID)
	if err != nil {
		return err
	}
	if err := s.rewriteUsersByOrg(ctx, tx, departmentName, "", func(user *domain.User) {
		user.Department = domain.DepartmentUnassigned
		user.Team = unassignedTeam
		user.ManagedDepartments = removeStringValue(user.ManagedDepartments, departmentName)
		user.ManagedTeams = removeStringValues(user.ManagedTeams, teamNames)
	}); err != nil {
		return err
	}
	return s.rewriteAllUserManagedScopes(ctx, tx, func(user *domain.User) bool {
		nextDepartments := removeStringValue(user.ManagedDepartments, departmentName)
		nextTeams := removeStringValues(user.ManagedTeams, teamNames)
		if stringSlicesEqual(nextDepartments, user.ManagedDepartments) && stringSlicesEqual(nextTeams, user.ManagedTeams) {
			return false
		}
		user.ManagedDepartments = nextDepartments
		user.ManagedTeams = nextTeams
		return true
	})
}

func (s *identityService) moveTeamUsersToUnassigned(ctx context.Context, tx repo.Tx, department, team, unassignedTeam string) error {
	if err := s.rewriteUsersByOrg(ctx, tx, department, team, func(user *domain.User) {
		user.Department = domain.DepartmentUnassigned
		user.Team = unassignedTeam
		user.ManagedTeams = removeStringValue(user.ManagedTeams, team)
	}); err != nil {
		return err
	}
	return s.rewriteAllUserManagedScopes(ctx, tx, func(user *domain.User) bool {
		next := removeStringValue(user.ManagedTeams, team)
		if stringSlicesEqual(next, user.ManagedTeams) {
			return false
		}
		user.ManagedTeams = next
		return true
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

func (s *identityService) rewriteUsersByOrg(ctx context.Context, tx repo.Tx, department, team string, mutate func(*domain.User)) error {
	users, err := s.listUsersByOrgSnapshot(ctx, department, team)
	if err != nil {
		return err
	}
	for _, user := range users {
		if user == nil {
			continue
		}
		mutate(user)
		user.UpdatedAt = time.Now().UTC()
		if err := s.userRepo.Update(ctx, tx, user); err != nil {
			return err
		}
	}
	return nil
}

func (s *identityService) listUsersByOrgSnapshot(ctx context.Context, department, team string) ([]*domain.User, error) {
	dept := domain.Department(department)
	const pageSize = 500
	out := make([]*domain.User, 0)
	for page := 1; ; page++ {
		users, _, err := s.userRepo.List(ctx, repo.UserListFilter{
			Department: &dept,
			Team:       team,
			Page:       page,
			PageSize:   pageSize,
		})
		if err != nil {
			return nil, err
		}
		if len(users) == 0 {
			return out, nil
		}
		out = append(out, users...)
		if len(users) < pageSize {
			return out, nil
		}
	}
}

func (s *identityService) rewriteAllUserManagedScopes(ctx context.Context, tx repo.Tx, mutate func(*domain.User) bool) error {
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

		for _, name := range s.seedOrgDepartmentNames() {
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

		for departmentName, teams := range s.authSettings.DepartmentTeams {
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

	teamsByDepartmentID := map[int64][]domain.OrgTeamOption{}
	teamsByDepartmentName := map[string][]string{}
	for _, team := range teams {
		if team == nil {
			continue
		}
		teamsByDepartmentID[team.DepartmentID] = append(teamsByDepartmentID[team.DepartmentID], domain.OrgTeamOption{
			ID:      team.ID,
			Name:    team.Name,
			Enabled: team.Enabled,
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
			ID:        department.ID,
			Name:      department.Name,
			Teams:     teamNames,
			TeamItems: teamItems,
			Enabled:   department.Enabled,
		})
		options.TeamsByDepartment[department.Name] = teamNames
	}
	return options, nil
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
