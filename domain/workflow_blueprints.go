package domain

const (
	TaskTypeRetouchTask           TaskType = "retouch_task"
	TaskTypeCustomerCustomization TaskType = "customer_customization"
	TaskTypeRegularCustomization  TaskType = "regular_customization"
)

const (
	TeamDesignStandard     = "design_standard"
	TeamDesignRetouch      = "design_retouch"
	TeamAuditStandard      = "audit_standard"
	TeamAuditCustomization = "audit_customization"
	TeamCustomizationArt   = "customization_art"
	TeamWarehouseMain      = "warehouse_main"
	TeamProcurementMain    = "procurement_main"
)

type PoolTeamTarget struct {
	Department string
	Team       string
}

func PoolTeamTargets(poolTeamCode string) []PoolTeamTarget {
	switch poolTeamCode {
	case TeamDesignStandard, TeamDesignRetouch:
		return []PoolTeamTarget{{Department: string(DepartmentDesignRD), Team: "默认组"}}
	case TeamAuditStandard:
		return []PoolTeamTarget{
			{Department: string(DepartmentAudit), Team: "普通审核组"},
			{Department: string(DepartmentAudit), Team: "常规审核组"},
		}
	case TeamAuditCustomization:
		return []PoolTeamTarget{
			{Department: string(DepartmentAudit), Team: "定制审核组"},
			{Department: string(DepartmentAudit), Team: "定制美工审核组"},
		}
	case TeamCustomizationArt:
		return []PoolTeamTarget{{Department: string(DepartmentCustomizationArt), Team: "默认组"}}
	case TeamWarehouseMain:
		return []PoolTeamTarget{{Department: string(DepartmentCloudWarehouse), Team: "默认组"}}
	default:
		return nil
	}
}

func V1TaskTypes() []TaskType {
	return []TaskType{
		TaskTypeOriginalProductDevelopment,
		TaskTypeNewProductDevelopment,
		TaskTypePurchaseTask,
		TaskTypeRetouchTask,
		TaskTypeCustomerCustomization,
		TaskTypeRegularCustomization,
	}
}
