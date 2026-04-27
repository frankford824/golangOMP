package service

import (
	"log"
	"sort"
	"strings"
	"sync"

	"workflow/domain"
)

type taskOrgCatalog struct {
	configuredDepartments   map[string]struct{}
	orgTeamDepartmentMap    map[string]string
	legacyOwnerTeamDeptsMap map[string][]string
	departmentLegacyTeams   map[string][]string
	ownerTeamCompatMappings []TaskOwnerTeamCompatMapping
	ownerTeamCompatMap      map[string]string
}

var (
	taskOrgCatalogMu      sync.RWMutex
	currentTaskOrgCatalog = buildTaskOrgCatalog(domain.AuthSettings{})
)

// ConfigureTaskOrgCatalog keeps task ownership compatibility aligned with the
// runtime org configuration. Zero-value settings reset the default catalog.
func ConfigureTaskOrgCatalog(settings domain.AuthSettings) {
	taskOrgCatalogMu.Lock()
	defer taskOrgCatalogMu.Unlock()
	log.Printf("task_org_catalog_configure department_team_count=%d department_keys=%q", len(settings.DepartmentTeams), sortedTaskOrgCatalogKeys(settings.DepartmentTeams))
	currentTaskOrgCatalog = buildTaskOrgCatalog(settings)
}

func buildTaskOrgCatalog(settings domain.AuthSettings) taskOrgCatalog {
	departmentTeams := cloneStringSliceMap(settings.DepartmentTeams)
	if len(departmentTeams) == 0 {
		departmentTeams = cloneStringSliceMap(domain.DefaultOrgDepartmentTeams())
	}
	taskTeamMappings := cloneStringSliceMap(domain.DefaultDepartmentTeams())
	configuredTaskMappings := cloneStringSliceMap(settings.TaskTeamMappings)
	if len(configuredTaskMappings) == 0 {
		configuredTaskMappings = cloneStringSliceMap(domain.DefaultTaskTeamMappings())
	}
	for department, teams := range configuredTaskMappings {
		taskTeamMappings[strings.TrimSpace(department)] = append([]string{}, teams...)
	}

	compatMappings := buildTaskOwnerTeamCompatMappings(departmentTeams, taskTeamMappings)
	compatMap := buildTaskCreateOwnerTeamCompatMap(compatMappings)

	return taskOrgCatalog{
		configuredDepartments:   buildTaskConfiguredDepartments(departmentTeams),
		orgTeamDepartmentMap:    buildTaskOrgTeamDepartmentMap(departmentTeams),
		legacyOwnerTeamDeptsMap: buildTaskLegacyOwnerTeamDepartmentsWithFallback(departmentTeams, taskTeamMappings),
		departmentLegacyTeams:   buildTaskDepartmentLegacyTeamsWithFallback(departmentTeams, taskTeamMappings),
		ownerTeamCompatMappings: compatMappings,
		ownerTeamCompatMap:      compatMap,
	}
}

func buildTaskOwnerTeamCompatMappings(departmentTeams, taskTeamMappings map[string][]string) []TaskOwnerTeamCompatMapping {
	out := make([]TaskOwnerTeamCompatMapping, 0)
	seen := map[string]struct{}{}
	for department, orgTeams := range departmentTeams {
		legacyTeams := normalizeTaskOwnershipStrings(taskTeamMappings[department])
		if len(legacyTeams) != 1 {
			continue
		}
		legacyOwnerTeam := legacyTeams[0]
		if !domain.ValidTeam(legacyOwnerTeam) {
			continue
		}
		for _, orgTeam := range normalizeTaskOwnershipStrings(orgTeams) {
			if orgTeam == "" || orgTeam == legacyOwnerTeam {
				continue
			}
			key := orgTeam + "\x00" + legacyOwnerTeam
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, TaskOwnerTeamCompatMapping{
				OrgTeam:         orgTeam,
				LegacyOwnerTeam: legacyOwnerTeam,
			})
		}
	}
	return out
}

func getTaskOrgCatalog() taskOrgCatalog {
	taskOrgCatalogMu.RLock()
	defer taskOrgCatalogMu.RUnlock()
	return cloneTaskOrgCatalog(currentTaskOrgCatalog)
}

func cloneTaskOrgCatalog(catalog taskOrgCatalog) taskOrgCatalog {
	return taskOrgCatalog{
		configuredDepartments:   cloneStringSet(catalog.configuredDepartments),
		orgTeamDepartmentMap:    cloneStringMap(catalog.orgTeamDepartmentMap),
		legacyOwnerTeamDeptsMap: cloneStringSliceMap(catalog.legacyOwnerTeamDeptsMap),
		departmentLegacyTeams:   cloneStringSliceMap(catalog.departmentLegacyTeams),
		ownerTeamCompatMappings: append([]TaskOwnerTeamCompatMapping{}, catalog.ownerTeamCompatMappings...),
		ownerTeamCompatMap:      cloneStringMap(catalog.ownerTeamCompatMap),
	}
}

func cloneStringSliceMap(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(in))
	for key, values := range in {
		out[strings.TrimSpace(key)] = append([]string{}, values...)
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneStringSet(in map[string]struct{}) map[string]struct{} {
	if len(in) == 0 {
		return map[string]struct{}{}
	}
	out := make(map[string]struct{}, len(in))
	for key := range in {
		out[key] = struct{}{}
	}
	return out
}

func normalizeTaskOwnershipStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func buildTaskConfiguredDepartments(departmentTeams map[string][]string) map[string]struct{} {
	out := map[string]struct{}{}
	for department := range departmentTeams {
		trimmed := normalizeTaskDepartmentCode(department)
		if trimmed != "" {
			out[trimmed] = struct{}{}
		}
	}
	return out
}

func sortedTaskOrgCatalogKeys(departmentTeams map[string][]string) []string {
	keys := make([]string, 0, len(departmentTeams))
	for department := range departmentTeams {
		keys = append(keys, strings.TrimSpace(department))
	}
	sort.Strings(keys)
	return keys
}
