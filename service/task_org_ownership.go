package service

import (
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type taskCanonicalOrgOwnership struct {
	OwnerDepartment string
	OwnerOrgTeam    string
	LegacyOwnerTeam string
}

func resolveTaskCanonicalOrgOwnership(p CreateTaskParams) (taskCanonicalOrgOwnership, *domain.AppError) {
	rawOwnerDepartment := normalizeTaskDepartmentCode(p.rawOwnerDepartment)
	rawOwnerOrgTeam := strings.TrimSpace(p.rawOwnerOrgTeam)
	rawOwnerTeam := strings.TrimSpace(p.rawOwnerTeam)
	legacyOwnerTeam := strings.TrimSpace(p.OwnerTeam)

	ownership := taskCanonicalOrgOwnership{
		LegacyOwnerTeam: legacyOwnerTeam,
	}

	if rawOwnerDepartment != "" && !validConfiguredTaskDepartment(rawOwnerDepartment) {
		return taskCanonicalOrgOwnership{}, taskCreateValidationError(
			"owner_department must be a valid configured department",
			p,
			taskCreateViolation("owner_department", "invalid_owner_department", "owner_department must be one of the configured departments"),
		)
	}

	if rawOwnerOrgTeam != "" {
		department, ok := canonicalDepartmentForTaskOrgTeam(rawOwnerOrgTeam)
		if !ok {
			return taskCanonicalOrgOwnership{}, taskCreateValidationError(
				"owner_org_team must be a valid configured org team",
				p,
				taskCreateViolation("owner_org_team", "invalid_owner_org_team", "owner_org_team must be one of the configured org teams"),
			)
		}
		catalog := getTaskOrgCatalog()
		legacyFromOrgTeam, ok := catalog.ownerTeamCompatMap[rawOwnerOrgTeam]
		if !ok {
			if validConfiguredTaskLegacyOwnerTeam(rawOwnerOrgTeam) {
				legacyFromOrgTeam = rawOwnerOrgTeam
			} else {
				return taskCanonicalOrgOwnership{}, taskCreateValidationError(
					"owner_org_team is not configured for task ownership compatibility",
					p,
					taskCreateViolation("owner_org_team", "unsupported_owner_org_team", "owner_org_team is not configured for task create compatibility"),
				)
			}
		}
		if rawOwnerDepartment != "" && rawOwnerDepartment != department {
			return taskCanonicalOrgOwnership{}, taskCreateValidationError(
				"owner_department does not match owner_org_team",
				p,
				taskCreateViolation("owner_department", "owner_department_owner_org_team_mismatch", "owner_department must match the department of owner_org_team"),
			)
		}
		if rawOwnerTeam != "" {
			ownerTeamResolution := normalizeOwnerTeamForTaskCreate(rawOwnerTeam)
			if ownerTeamResolution.MappingSource == taskOwnerTeamMappingSourceInvalid {
				return taskCanonicalOrgOwnership{}, taskCreateValidationError(
					"owner_team must be a valid configured team",
					p,
					taskCreateViolation("owner_team", "invalid_owner_team", "owner_team must be one of the configured teams"),
				)
			}
			if ownerTeamResolution.Normalized != legacyFromOrgTeam {
				return taskCanonicalOrgOwnership{}, taskCreateValidationError(
					"owner_team does not match owner_org_team",
					p,
					taskCreateViolation("owner_team", "owner_team_owner_org_team_mismatch", "owner_team must match owner_org_team after compatibility normalization"),
				)
			}
			if ownerTeamResolution.MappingSource == taskOwnerTeamMappingSourceOrgTeamCompat && rawOwnerTeam != rawOwnerOrgTeam {
				return taskCanonicalOrgOwnership{}, taskCreateValidationError(
					"owner_team org-team alias does not match owner_org_team",
					p,
					taskCreateViolation("owner_team", "owner_team_owner_org_team_mismatch", "owner_team org-team alias must match owner_org_team when both are provided"),
				)
			}
		}
		ownership.OwnerDepartment = department
		ownership.OwnerOrgTeam = rawOwnerOrgTeam
		ownership.LegacyOwnerTeam = legacyFromOrgTeam
		return ownership, nil
	}

	if rawOwnerTeam != "" {
		if department, ok := canonicalDepartmentForTaskOrgTeam(rawOwnerTeam); ok {
			ownership.OwnerDepartment = department
			ownership.OwnerOrgTeam = rawOwnerTeam
			return ownership, nil
		}
	}

	if rawOwnerDepartment != "" {
		if legacyOwnerTeam != "" && domain.ValidTeam(legacyOwnerTeam) && !departmentSupportsLegacyOwnerTeam(rawOwnerDepartment, legacyOwnerTeam) {
			return taskCanonicalOrgOwnership{}, taskCreateValidationError(
				"owner_department does not match owner_team",
				p,
				taskCreateViolation("owner_department", "owner_department_owner_team_mismatch", "owner_department must be compatible with owner_team"),
			)
		}
		ownership.OwnerDepartment = rawOwnerDepartment
		return ownership, nil
	}

	if department, ok := inferCanonicalDepartmentFromLegacyOwnerTeam(legacyOwnerTeam); ok {
		ownership.OwnerDepartment = department
	}
	return ownership, nil
}

func buildTaskReadModelOrgOwnership(ownerDepartment, ownerOrgTeam, legacyOwnerTeam string) taskCanonicalOrgOwnership {
	ownership := taskCanonicalOrgOwnership{
		OwnerDepartment: normalizeTaskDepartmentCode(ownerDepartment),
		OwnerOrgTeam:    strings.TrimSpace(ownerOrgTeam),
		LegacyOwnerTeam: strings.TrimSpace(legacyOwnerTeam),
	}
	if ownership.OwnerOrgTeam != "" {
		if ownership.OwnerDepartment == "" {
			if department, ok := canonicalDepartmentForTaskOrgTeam(ownership.OwnerOrgTeam); ok {
				ownership.OwnerDepartment = department
			}
		}
		return ownership
	}
	if ownership.OwnerDepartment != "" {
		return ownership
	}
	if department, ok := inferCanonicalDepartmentFromLegacyOwnerTeam(ownership.LegacyOwnerTeam); ok {
		ownership.OwnerDepartment = department
	}
	return ownership
}

func applyTaskReadModelOrgOwnership(task *domain.Task) {
	if task == nil {
		return
	}
	ownership := buildTaskReadModelOrgOwnership(task.OwnerDepartment, task.OwnerOrgTeam, task.OwnerTeam)
	task.OwnerDepartment = ownership.OwnerDepartment
	task.OwnerOrgTeam = ownership.OwnerOrgTeam
}

func applyTaskListItemReadModelOrgOwnership(item *domain.TaskListItem) {
	if item == nil {
		return
	}
	ownership := buildTaskReadModelOrgOwnership(item.OwnerDepartment, item.OwnerOrgTeam, item.OwnerTeam)
	item.OwnerDepartment = ownership.OwnerDepartment
	item.OwnerOrgTeam = ownership.OwnerOrgTeam
}

func applyTaskOrgVisibilityScope(filter repo.TaskListFilter, scope *DataScope) repo.TaskListFilter {
	if scope == nil {
		return filter
	}
	filter.ScopeViewAll = scope.ViewAll
	filter.ScopeDepartmentCodes = append([]string(nil), scope.DepartmentCodes...)
	filter.ScopeTeamCodes = append([]string(nil), scope.TeamCodes...)
	filter.ScopeManagedDepartmentCodes = append([]string(nil), scope.ManagedDepartmentCodes...)
	filter.ScopeManagedTeamCodes = append([]string(nil), scope.ManagedTeamCodes...)
	filter.ScopeUserIDs = append([]int64(nil), scope.UserIDs...)
	filter.ScopeStageVisibilities = make([]repo.ScopeStageVisibility, 0, len(scope.StageVisibilities))
	for _, visibility := range scope.StageVisibilities {
		filter.ScopeStageVisibilities = append(filter.ScopeStageVisibilities, repo.ScopeStageVisibility{
			Statuses: append([]domain.TaskStatus(nil), visibility.Statuses...),
			Lane:     cloneWorkflowLane(visibility.Lane),
		})
	}
	return filter
}

func canonicalDepartmentForTaskOrgTeam(team string) (string, bool) {
	catalog := getTaskOrgCatalog()
	department, ok := catalog.orgTeamDepartmentMap[strings.TrimSpace(team)]
	if !ok {
		return "", false
	}
	return normalizeTaskDepartmentCode(department), true
}

func validConfiguredTaskDepartment(department string) bool {
	catalog := getTaskOrgCatalog()
	_, ok := catalog.configuredDepartments[normalizeTaskDepartmentCode(department)]
	return ok
}

func inferCanonicalDepartmentFromLegacyOwnerTeam(legacyOwnerTeam string) (string, bool) {
	catalog := getTaskOrgCatalog()
	departments := catalog.legacyOwnerTeamDeptsMap[strings.TrimSpace(legacyOwnerTeam)]
	if len(departments) == 0 {
		return "", false
	}
	if len(departments) == 1 {
		return normalizeTaskDepartmentCode(departments[0]), true
	}
	canonical := ""
	for _, department := range departments {
		normalizedDepartment := normalizeTaskDepartmentCode(department)
		if _, ok := catalog.configuredDepartments[normalizedDepartment]; !ok {
			continue
		}
		if canonical != "" && canonical != normalizedDepartment {
			return "", false
		}
		canonical = normalizedDepartment
	}
	if canonical == "" {
		return "", false
	}
	return canonical, true
}

func departmentSupportsLegacyOwnerTeam(department, legacyOwnerTeam string) bool {
	catalog := getTaskOrgCatalog()
	for _, candidate := range catalog.departmentLegacyTeams[normalizeTaskDepartmentCode(department)] {
		if candidate == strings.TrimSpace(legacyOwnerTeam) {
			return true
		}
	}
	return false
}

func buildTaskOrgTeamDepartmentMap(source map[string][]string) map[string]string {
	result := make(map[string]string, len(source))
	for department, teams := range source {
		for _, team := range teams {
			trimmed := strings.TrimSpace(team)
			if trimmed == "" {
				continue
			}
			result[trimmed] = normalizeTaskDepartmentCode(department)
		}
	}
	return result
}

func buildTaskLegacyOwnerTeamDepartments(source map[string][]string) map[string][]string {
	result := map[string][]string{}
	for department, teams := range source {
		for _, team := range teams {
			trimmedTeam := strings.TrimSpace(team)
			trimmedDepartment := normalizeTaskDepartmentCode(department)
			if trimmedTeam == "" || trimmedDepartment == "" {
				continue
			}
			if containsTaskOrgOwnershipString(result[trimmedTeam], trimmedDepartment) {
				continue
			}
			result[trimmedTeam] = append(result[trimmedTeam], trimmedDepartment)
		}
	}
	return result
}

func buildTaskLegacyOwnerTeamDepartmentsWithFallback(departmentTeams, taskTeamMappings map[string][]string) map[string][]string {
	result := buildTaskLegacyOwnerTeamDepartments(taskTeamMappings)
	for department, teams := range departmentTeams {
		trimmedDepartment := normalizeTaskDepartmentCode(department)
		if trimmedDepartment == "" || len(normalizeTaskOwnershipStrings(taskTeamMappings[department])) > 0 {
			continue
		}
		for _, team := range teams {
			trimmedTeam := strings.TrimSpace(team)
			if trimmedTeam == "" || containsTaskOrgOwnershipString(result[trimmedTeam], trimmedDepartment) {
				continue
			}
			result[trimmedTeam] = append(result[trimmedTeam], trimmedDepartment)
		}
	}
	return result
}

func buildTaskDepartmentLegacyTeams(source map[string][]string) map[string][]string {
	result := map[string][]string{}
	for department, teams := range source {
		trimmedDepartment := normalizeTaskDepartmentCode(department)
		if trimmedDepartment == "" {
			continue
		}
		for _, team := range teams {
			trimmedTeam := strings.TrimSpace(team)
			if trimmedTeam == "" || containsTaskOrgOwnershipString(result[trimmedDepartment], trimmedTeam) {
				continue
			}
			result[trimmedDepartment] = append(result[trimmedDepartment], trimmedTeam)
		}
	}
	return result
}

func buildTaskDepartmentLegacyTeamsWithFallback(departmentTeams, taskTeamMappings map[string][]string) map[string][]string {
	result := buildTaskDepartmentLegacyTeams(taskTeamMappings)
	for department, teams := range departmentTeams {
		trimmedDepartment := normalizeTaskDepartmentCode(department)
		if trimmedDepartment == "" {
			continue
		}
		if len(normalizeTaskOwnershipStrings(result[trimmedDepartment])) > 0 {
			continue
		}
		for _, team := range teams {
			trimmedTeam := strings.TrimSpace(team)
			if trimmedTeam == "" || containsTaskOrgOwnershipString(result[trimmedDepartment], trimmedTeam) {
				continue
			}
			result[trimmedDepartment] = append(result[trimmedDepartment], trimmedTeam)
		}
	}
	return result
}

func containsTaskOrgOwnershipString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
