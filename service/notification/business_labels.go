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
	case domain.TeamAuditStandard, "audit_a":
		return "常规审核组"
	case domain.TeamAuditCustomization, "audit_b":
		return "定制审核组"
	case domain.TeamCustomizationArt:
		return "定制美工组"
	case domain.TeamWarehouseMain:
		return "云仓组"
	case domain.TeamProcurementMain:
		return "采购组"
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
	case domain.ModuleKeyWarehouse:
		return "仓库"
	case domain.ModuleKeyProcurement:
		return "采购"
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
