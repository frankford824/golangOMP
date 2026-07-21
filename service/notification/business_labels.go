package notification

import (
	"strings"

	"workflow/domain"
)

func businessTeamLabel(code string) string {
	switch strings.TrimSpace(code) {
	case domain.TeamDesignStandard:
		return "设计组"
	case domain.TeamDesignRetouch:
		return "精修组"
	case domain.TeamAuditStandard, domain.TeamAuditCustomization:
		return "审核组"
	case domain.TeamCustomizationArt:
		return "定制美工组"
	default:
		return ""
	}
}

func businessModuleLabel(key string) string {
	switch strings.TrimSpace(key) {
	case domain.ModuleKeyBasicInfo:
		return "任务信息"
	case domain.ModuleKeyDesign:
		return "设计"
	case domain.ModuleKeyRetouch:
		return "精修"
	case domain.ModuleKeyAudit:
		return "审核"
	case domain.ModuleKeyCustomization:
		return "定制美工"
	default:
		return ""
	}
}

func displayTeamLabel(code, fallback string) string {
	if label := strings.TrimSpace(fallback); label != "" && !looksTechnical(label) {
		return label
	}
	if label := businessTeamLabel(code); label != "" {
		return label
	}
	if label := businessTeamLabel(fallback); label != "" {
		return label
	}
	return ""
}

func looksTechnical(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || strings.Contains(value, "_") || strings.Contains(value, ".")
}
