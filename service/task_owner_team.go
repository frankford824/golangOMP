package service

import (
	"context"
	"log"
	"sort"
	"strings"

	"workflow/domain"
)

const (
	taskOwnerTeamMappingSourceLegacyDirect  = "legacy_direct"
	taskOwnerTeamMappingSourceOrgTeamCompat = "org_team_compat"
	taskOwnerTeamMappingSourceInvalid       = "invalid"
)

type taskOwnerTeamResolution struct {
	Normalized     string
	MappingApplied bool
	MappingSource  string
}

type TaskOwnerTeamCompatMapping struct {
	OrgTeam         string
	LegacyOwnerTeam string
}

// buildTaskCreateOwnerTeamCompatMap creates the task-create bridge between the
// runtime org-team tree and the legacy task owner_team enum.
// Only org teams whose department has exactly one configured legacy task team
// become accepted task owner_team aliases.
func buildTaskCreateOwnerTeamCompatMap(mappings []TaskOwnerTeamCompatMapping) map[string]string {
	compat := map[string]string{}
	for _, mapping := range mappings {
		orgTeam := strings.TrimSpace(mapping.OrgTeam)
		legacyOwnerTeam := strings.TrimSpace(mapping.LegacyOwnerTeam)
		if orgTeam == "" || legacyOwnerTeam == "" {
			continue
		}
		compat[orgTeam] = legacyOwnerTeam
	}
	return compat
}

func ListTaskOwnerTeamCompatMappings() []TaskOwnerTeamCompatMapping {
	catalog := getTaskOrgCatalog()
	out := make([]TaskOwnerTeamCompatMapping, 0, len(catalog.ownerTeamCompatMappings))
	for _, mapping := range catalog.ownerTeamCompatMappings {
		out = append(out, TaskOwnerTeamCompatMapping{
			OrgTeam:         strings.TrimSpace(mapping.OrgTeam),
			LegacyOwnerTeam: strings.TrimSpace(mapping.LegacyOwnerTeam),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LegacyOwnerTeam == out[j].LegacyOwnerTeam {
			return out[i].OrgTeam < out[j].OrgTeam
		}
		return out[i].LegacyOwnerTeam < out[j].LegacyOwnerTeam
	})
	return out
}

func normalizeOwnerTeamForTaskCreate(raw string) taskOwnerTeamResolution {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return taskOwnerTeamResolution{
			Normalized:    "",
			MappingSource: taskOwnerTeamMappingSourceInvalid,
		}
	}
	if validConfiguredTaskLegacyOwnerTeam(trimmed) {
		return taskOwnerTeamResolution{
			Normalized:    trimmed,
			MappingSource: taskOwnerTeamMappingSourceLegacyDirect,
		}
	}
	catalog := getTaskOrgCatalog()
	if mapped, ok := catalog.ownerTeamCompatMap[trimmed]; ok && validConfiguredTaskLegacyOwnerTeam(mapped) {
		return taskOwnerTeamResolution{
			Normalized:     mapped,
			MappingApplied: true,
			MappingSource:  taskOwnerTeamMappingSourceOrgTeamCompat,
		}
	}
	return taskOwnerTeamResolution{
		Normalized:    trimmed,
		MappingSource: taskOwnerTeamMappingSourceInvalid,
	}
}

func validConfiguredTaskLegacyOwnerTeam(team string) bool {
	trimmed := strings.TrimSpace(team)
	if trimmed == "" {
		return false
	}
	catalog := getTaskOrgCatalog()
	for _, teams := range catalog.departmentLegacyTeams {
		for _, candidate := range teams {
			if candidate == trimmed {
				return true
			}
		}
	}
	return false
}

func allConfiguredTaskLegacyOwnerTeams() []string {
	catalog := getTaskOrgCatalog()
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, teams := range catalog.departmentLegacyTeams {
		for _, candidate := range teams {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			out = append(out, candidate)
		}
	}
	sort.Strings(out)
	return out
}

func logCreateTaskOwnerTeamNormalization(ctx context.Context, p CreateTaskParams) {
	rawOwnerTeam := strings.TrimSpace(p.rawOwnerTeam)
	normalizedOwnerTeam := strings.TrimSpace(p.OwnerTeam)
	mappingSource := strings.TrimSpace(p.ownerTeamMappingSource)
	if mappingSource == "" {
		mappingSource = taskOwnerTeamMappingSourceInvalid
	}
	log.Printf(
		"create_task_owner_team_normalization trace_id=%s task_type=%s raw_owner_team=%q normalized_owner_team=%q owner_team_mapping_applied=%t mapping_source=%s",
		domain.TraceIDFromContext(ctx),
		string(p.TaskType),
		rawOwnerTeam,
		normalizedOwnerTeam,
		p.ownerTeamMappingApplied,
		mappingSource,
	)
}
