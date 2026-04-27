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
