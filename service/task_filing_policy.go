package service

import (
	"strings"

	"workflow/domain"
)

type TaskFilingTriggerSource string

const (
	TaskFilingTriggerSourceCreate                  TaskFilingTriggerSource = "create"
	TaskFilingTriggerSourceBusinessInfoPatch       TaskFilingTriggerSource = "business_info_patch"
	TaskFilingTriggerSourceProcurementUpdate       TaskFilingTriggerSource = "procurement_update"
	TaskFilingTriggerSourceProcurementAdvance      TaskFilingTriggerSource = "procurement_advance"
	TaskFilingTriggerSourceAuditFinalApproved      TaskFilingTriggerSource = "audit_final_approved"
	TaskFilingTriggerSourceWarehouseCompletePrechk TaskFilingTriggerSource = "warehouse_complete_precheck"
	TaskFilingTriggerSourceManualRetry             TaskFilingTriggerSource = "manual_retry"
	TaskFilingTriggerSourceLegacyFiledAt           TaskFilingTriggerSource = "legacy_filed_at"
)

func ComputeFilingMissingFields(task *domain.Task, detail *domain.TaskDetail) ([]string, string) {
	if task == nil || detail == nil {
		return []string{"task_detail"}, "缺少任务建档明细"
	}

	fields := make([]string, 0, 8)
	labels := make([]string, 0, 8)
	seen := map[string]struct{}{}
	add := func(field, label string, missing bool) {
		if !missing || field == "" || label == "" {
			return
		}
		if _, ok := seen[field]; ok {
			return
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
		labels = append(labels, label)
	}

	switch task.TaskType {
	case domain.TaskTypeOriginalProductDevelopment:
		selection := buildTaskProductSelectionContext(task, detail)
		var erp *domain.ERPProductSelectionSnapshot
		if selection != nil {
			erp = normalizeERPProductSelectionSnapshot(selection.ERPProduct)
		}
		add("product_selection.erp_product", "ERP匹配商品", erp == nil)
		if erp != nil {
			skuID := firstNonEmptyString(strings.TrimSpace(erp.SKUID), strings.TrimSpace(erp.SKUCode), strings.TrimSpace(task.SKUCode))
			add("product_selection.erp_product.sku_id", "ERP SKU", skuID == "")
		}
		add("category_code", "品类编码", strings.TrimSpace(detail.CategoryCode) == "")
		add("spec_text", "规格", strings.TrimSpace(detail.SpecText) == "")
		add("cost_price", "成本价", detail.CostPrice == nil)

	case domain.TaskTypeNewProductDevelopment:
		add("sku_code", "SKU", strings.TrimSpace(task.SKUCode) == "")
		add("product_name", "产品名称", strings.TrimSpace(task.ProductNameSnapshot) == "")
		add("product_short_name", "产品简称", strings.TrimSpace(detail.ProductShortName) == "")
		add("category_code", "品类编码", strings.TrimSpace(detail.CategoryCode) == "")
		add("material_mode", "材质模式", strings.TrimSpace(detail.MaterialMode) == "")
		switch strings.TrimSpace(detail.MaterialMode) {
		case string(domain.MaterialModePreset):
			add("material", "材质", strings.TrimSpace(detail.Material) == "")
		case string(domain.MaterialModeOther):
			add("material_other", "自定义材质", strings.TrimSpace(detail.MaterialOther) == "")
		}
		if strings.TrimSpace(detail.CostPriceMode) == string(domain.CostPriceModeManual) {
			add("cost_price", "成本价", detail.CostPrice == nil)
		}

	case domain.TaskTypePurchaseTask:
		add("sku_code", "采购SKU", strings.TrimSpace(task.SKUCode) == "")
		add("product_name", "产品名称", strings.TrimSpace(task.ProductNameSnapshot) == "")
		add("quantity", "数量", detail.Quantity == nil || *detail.Quantity <= 0)
		add("base_sale_price", "基本售价", detail.BaseSalePrice == nil)
		if strings.TrimSpace(detail.CostPriceMode) == string(domain.CostPriceModeManual) {
			add("cost_price", "成本价", detail.CostPrice == nil)
		}
	}

	if len(labels) == 0 {
		return fields, ""
	}
	return fields, "缺少：" + strings.Join(labels, "、")
}

func shouldAutoTriggerFiling(task *domain.Task, source TaskFilingTriggerSource) bool {
	if task == nil {
		return false
	}
	switch task.TaskType {
	case domain.TaskTypeNewProductDevelopment:
		return source == TaskFilingTriggerSourceCreate || source == TaskFilingTriggerSourceBusinessInfoPatch
	case domain.TaskTypePurchaseTask:
		return source == TaskFilingTriggerSourceCreate ||
			source == TaskFilingTriggerSourceBusinessInfoPatch ||
			source == TaskFilingTriggerSourceProcurementUpdate ||
			source == TaskFilingTriggerSourceProcurementAdvance
	case domain.TaskTypeOriginalProductDevelopment:
		if source == TaskFilingTriggerSourceWarehouseCompletePrechk || source == TaskFilingTriggerSourceAuditFinalApproved {
			return true
		}
		if source == TaskFilingTriggerSourceBusinessInfoPatch {
			return originalFilingAutoEligibleStatus(task.TaskStatus)
		}
		if source == TaskFilingTriggerSourceLegacyFiledAt || source == TaskFilingTriggerSourceManualRetry {
			return true
		}
		return false
	default:
		return false
	}
}

func originalFilingAutoEligibleStatus(status domain.TaskStatus) bool {
	if strings.TrimSpace(string(status)) == "" {
		return false
	}
	switch status {
	case domain.TaskStatusPendingAssign,
		domain.TaskStatusInProgress,
		domain.TaskStatusPendingAuditA,
		domain.TaskStatusPendingAuditB,
		domain.TaskStatusRejectedByAuditA,
		domain.TaskStatusRejectedByAuditB,
		domain.TaskStatusBlocked:
		return false
	default:
		return true
	}
}

func isFinalDesignAuditApproval(nextStatus domain.TaskStatus) bool {
	switch nextStatus {
	case domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingOutsource:
		return true
	default:
		return false
	}
}
