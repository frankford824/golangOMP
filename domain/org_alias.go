package domain

import "strings"

var orgDepartmentAliasGroups = map[string][]string{
	string(DepartmentDesignRD): {
		string(DepartmentDesignRD),
		string(DepartmentDesign),
		"视觉研创部",
	},
	string(DepartmentCloudWarehouse): {
		string(DepartmentCloudWarehouse),
		string(DepartmentProcurement),
		string(DepartmentWarehouse),
		string(DepartmentBakeryWH),
	},
}

var orgDepartmentAliasIndex = buildOrgAliasIndex(orgDepartmentAliasGroups)

var orgTeamAliasGroups = map[string][]string{
	"ops:taoxi:1": {
		"淘系一组",
		"淘系运营一部",
		"运营一组",
	},
	"ops:taoxi:2": {
		"淘系二组",
		"淘系运营二部",
		"运营二组",
	},
	"ops:taoxi:3": {
		"淘系三组",
		"淘系运营三部",
	},
	"ops:taoxi:4": {
		"运营四组",
		"淘系运营四部",
	},
	"ops:pdd:nanjing": {
		"拼多多南京组",
		"拼多多运营部（南京）",
		"拼多多运营部(南京)",
	},
	"ops:pdd:chizhou": {
		"拼多多池州组",
		"天猫运营一部（池州)",
		"天猫运营一部（池州）",
		"天猫运营一部(池州)",
	},
}

var orgTeamAliasIndex = buildOrgAliasIndex(orgTeamAliasGroups)

func buildOrgAliasIndex(groups map[string][]string) map[string]string {
	index := make(map[string]string)
	for key, aliases := range groups {
		for _, alias := range aliases {
			trimmed := strings.TrimSpace(alias)
			if trimmed == "" {
				continue
			}
			index[aliasLookupKey(trimmed)] = key
		}
	}
	return index
}

func aliasLookupKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func NormalizeOrgDepartmentAlias(department string) string {
	trimmed := strings.TrimSpace(department)
	if trimmed == "" {
		return ""
	}
	if canonical, ok := orgDepartmentAliasIndex[aliasLookupKey(trimmed)]; ok {
		return canonical
	}
	return trimmed
}

func NormalizeOrgTeamAlias(team string) string {
	trimmed := strings.TrimSpace(team)
	if trimmed == "" {
		return ""
	}
	if canonical, ok := orgTeamAliasIndex[aliasLookupKey(trimmed)]; ok {
		return canonical
	}
	return trimmed
}

func OrgDepartmentsEquivalent(left, right string) bool {
	left = NormalizeOrgDepartmentAlias(left)
	right = NormalizeOrgDepartmentAlias(right)
	return left != "" && right != "" && strings.EqualFold(left, right)
}

func OrgTeamsEquivalent(left, right string) bool {
	left = NormalizeOrgTeamAlias(left)
	right = NormalizeOrgTeamAlias(right)
	return left != "" && right != "" && strings.EqualFold(left, right)
}

func OrgDepartmentAliases(department string) []string {
	return orgAliasesForValue(department, orgDepartmentAliasIndex, orgDepartmentAliasGroups)
}

func OrgTeamAliases(team string) []string {
	return orgAliasesForValue(team, orgTeamAliasIndex, orgTeamAliasGroups)
}

func ExpandOrgDepartmentAliases(values []string) []string {
	return expandOrgAliases(values, OrgDepartmentAliases)
}

func ExpandOrgTeamAliases(values []string) []string {
	return expandOrgAliases(values, OrgTeamAliases)
}

func expandOrgAliases(values []string, aliasesFor func(string) []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, alias := range aliasesFor(value) {
			lookup := aliasLookupKey(alias)
			if lookup == "" {
				continue
			}
			if _, exists := seen[lookup]; exists {
				continue
			}
			seen[lookup] = struct{}{}
			out = append(out, strings.TrimSpace(alias))
		}
	}
	return out
}

func orgAliasesForValue(value string, index map[string]string, groups map[string][]string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	key, ok := index[aliasLookupKey(trimmed)]
	if !ok {
		return []string{trimmed}
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(groups[key])+1)
	add := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		lookup := aliasLookupKey(candidate)
		if _, exists := seen[lookup]; exists {
			return
		}
		seen[lookup] = struct{}{}
		out = append(out, candidate)
	}
	add(trimmed)
	for _, alias := range groups[key] {
		add(alias)
	}
	return out
}
