package service

import (
	"context"
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
	Enabled *bool
}

type CreateOrgTeamParams struct {
	DepartmentID *int64
	Department   string
	Name         string
}

type UpdateOrgTeamParams struct {
	ID      int64
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
	if p.Enabled == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "enabled is required", nil)
	}
	if current.Enabled == *p.Enabled {
		return current, nil
	}
	if !*p.Enabled {
		count, err := s.userRepo.CountByDepartment(ctx, current.Name)
		if err != nil {
			return nil, infraError("count users by department", err)
		}
		if count > 0 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "department still has assigned users", map[string]interface{}{"department": current.Name, "user_count": count})
		}
		teams, err := s.orgRepo.ListTeams(ctx, false)
		if err != nil {
			return nil, infraError("list org teams before disabling department", err)
		}
		for _, team := range teams {
			if team != nil && team.DepartmentID == current.ID && team.Enabled {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "disable teams before disabling department", map[string]interface{}{"department": current.Name, "team": team.Name})
			}
		}
	}
	current.Enabled = *p.Enabled
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
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
	if p.Enabled == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "enabled is required", nil)
	}
	if current.Enabled == *p.Enabled {
		return current, nil
	}
	if !*p.Enabled {
		count, err := s.userRepo.CountByTeam(ctx, current.Name)
		if err != nil {
			return nil, infraError("count users by team", err)
		}
		if count > 0 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "team still has assigned users", map[string]interface{}{"team": current.Name, "user_count": count})
		}
	}
	current.Enabled = *p.Enabled
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.orgRepo.UpdateTeam(ctx, tx, current)
	}); err != nil {
		return nil, infraError("update org team", err)
	}
	_ = s.refreshRuntimeOrgCatalog(ctx)
	return s.getTeamByID(ctx, current.ID)
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
		return s.buildConfigBackedOrgOptions(), nil
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
		RoleCatalogSummary:    s.ListRoles(context.Background()),
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

func (s *identityService) buildConfigBackedOrgOptions() *domain.OrgOptions {
	options := &domain.OrgOptions{
		Departments:           make([]domain.DepartmentOption, 0, len(s.authSettings.Departments)),
		TeamsByDepartment:     make(map[string][]string, len(s.authSettings.DepartmentTeams)),
		RoleCatalogSummary:    s.ListRoles(context.Background()),
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
