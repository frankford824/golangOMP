package transport

import (
	"net/http"
	"strings"

	"workflow/domain"
)

// v8BusinessRoutePermissions is the code-owned authorization catalog for the
// task, asset, catalogue and ERP surfaces that remain active after the v8
// cutover. Legacy role arguments on those registrations are documentation-only
// inputs during the source migration and never participate in authorization.
func v8BusinessRoutePermissions(method, path string) ([]domain.PermissionCode, bool) {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}

	switch {
	case strings.HasPrefix(path, "/v1/tasks"):
		return v8TaskRoutePermissions(method, path), true
	case strings.HasPrefix(path, "/v1/assets"):
		return v8AssetRoutePermissions(method, path), true
	case strings.HasPrefix(path, "/v1/asset-workbench"):
		return v8AssetWorkbenchRoutePermissions(method, path)
	case strings.HasPrefix(path, "/v1/task-board"):
		return []domain.PermissionCode{domain.PermissionTaskView}, true
	case strings.HasPrefix(path, "/v1/workbench"):
		return []domain.PermissionCode{domain.PermissionAccountUse}, true
	case strings.HasPrefix(path, "/v1/export-templates"), strings.HasPrefix(path, "/v1/export-jobs"):
		return []domain.PermissionCode{domain.PermissionAssetExport}, true
	case strings.HasPrefix(path, "/v1/predictions"):
		return []domain.PermissionCode{domain.PermissionTaskView}, true
	case strings.HasPrefix(path, "/v1/erp"), strings.HasPrefix(path, "/v1/products"), strings.HasPrefix(path, "/v1/product-management"):
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionCatalogView}, true
		}
		return []domain.PermissionCode{domain.PermissionERPManage}, true
	case strings.HasPrefix(path, "/v1/categories"), strings.HasPrefix(path, "/v1/category-mappings"), strings.HasPrefix(path, "/v1/cost-rules"), strings.HasPrefix(path, "/v1/cost-rule-bindings"), strings.HasPrefix(path, "/v1/code-rules"), strings.HasPrefix(path, "/v1/rule-templates"), strings.HasPrefix(path, "/v1/sku"):
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionCatalogView}, true
		}
		return []domain.PermissionCode{domain.PermissionCatalogManage}, true
	default:
		return nil, false
	}
}

func v8AssetWorkbenchRoutePermissions(method, path string) ([]domain.PermissionCode, bool) {
	relative := strings.TrimPrefix(path, "/v1/asset-workbench")
	switch {
	case relative == "/entry", relative == "/access/request":
		return []domain.PermissionCode{domain.PermissionAccountUse}, true
	case relative == "/access/open":
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchMembers}, true
	case relative == "/access/disable", strings.HasPrefix(relative, "/members/") && (strings.HasSuffix(relative, "/identity") || strings.HasSuffix(relative, "/roles")), strings.HasPrefix(relative, "/accounts/merge"):
		return []domain.PermissionCode{domain.PermissionSystemManage}, true
	case (relative == "/profiles" || strings.HasPrefix(relative, "/profiles/")) && method == http.MethodGet:
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchProfiles, domain.PermissionAssetWorkbenchSettlement}, true
	case relative == "/profiles" || strings.HasPrefix(relative, "/profiles/"):
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchProfiles}, true
	case relative == "/members":
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchMembers}, true
	case relative == "/people-lookup", relative == "/groups", strings.HasPrefix(relative, "/groups/"):
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchGroups}, true
	case method == http.MethodGet && (strings.HasPrefix(relative, "/difficulty-classes/admin") || relative == "/price-matrix" || strings.HasPrefix(relative, "/price-matrix/") || relative == "/deduction-rules" || strings.HasPrefix(relative, "/deduction-rules/") || relative == "/welfare-rules" || strings.HasPrefix(relative, "/welfare-rules/") || relative == "/promo-coupons" || strings.HasPrefix(relative, "/promo-coupons/")):
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchTemplates, domain.PermissionAssetWorkbenchSettlement, domain.PermissionAssetManage}, true
	case strings.HasPrefix(relative, "/difficulty-classes/admin"), method != http.MethodGet && (relative == "/difficulty-classes" || strings.HasPrefix(relative, "/difficulty-classes/")), relative == "/price-matrix", strings.HasPrefix(relative, "/price-matrix/"), relative == "/deduction-rules", strings.HasPrefix(relative, "/deduction-rules/"), relative == "/welfare-rules", strings.HasPrefix(relative, "/welfare-rules/"), relative == "/promo-coupons", strings.HasPrefix(relative, "/promo-coupons/"):
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchTemplates}, true
	case relative == "/upload-directories/admin", method != http.MethodGet && strings.HasPrefix(relative, "/upload-directories"):
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchDrive}, true
	case relative == "/batch-jobs" || strings.HasPrefix(relative, "/batch-jobs/"):
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchBatch}, true
	case relative == "/files/batch-move", strings.HasPrefix(relative, "/items/"), strings.HasPrefix(relative, "/error-imports"), strings.HasSuffix(relative, "/void") && strings.HasPrefix(relative, "/submissions/"):
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchQC}, true
	case strings.HasPrefix(relative, "/files/") && method != http.MethodGet && !strings.HasPrefix(relative, "/files/batch-"):
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchQC}, true
	case (relative == "/settlement/supplements" && method == http.MethodPost), relative == "/settlement/supplements/batch-delete", method == http.MethodDelete && strings.HasPrefix(relative, "/settlement/supplements/"):
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchSubmit, domain.PermissionAssetWorkbenchSettlement}, true
	case strings.HasPrefix(relative, "/settlement/") && relative != "/settlement/my":
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchSettlement}, true
	case relative == "/events":
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchAuditView}, true
	case strings.Contains(relative, "/download"):
		return []domain.PermissionCode{domain.PermissionAssetDownload}, true
	case strings.HasPrefix(relative, "/upload-sessions"):
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionAssetWorkbenchUse}, true
		}
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchSubmit, domain.PermissionAssetWorkbenchSettlement}, true
	case relative == "/submissions", relative == "/files/batch-delete":
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionAssetWorkbenchUse}, true
		}
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchSubmit}, true
	case relative == "/profile", relative == "/bootstrap", strings.HasPrefix(relative, "/notifications"), relative == "/upload-directories", relative == "/difficulty-classes", strings.HasPrefix(relative, "/overview-search"), strings.HasPrefix(relative, "/drive/"), strings.HasPrefix(relative, "/submissions"), strings.HasPrefix(relative, "/files/"), relative == "/settlement/my", strings.HasPrefix(relative, "/saved-views"):
		return []domain.PermissionCode{domain.PermissionAssetWorkbenchUse}, true
	default:
		// Resource-group/client-material routes are registered with explicit
		// capabilityAccess and never reach this compatibility registrar.
		return nil, false
	}
}

func v8TaskRoutePermissions(method, path string) []domain.PermissionCode {
	if strings.Contains(path, "/upload-sessions") {
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionTaskView, domain.PermissionAssetView, domain.PermissionTaskDesignSubmit, domain.PermissionTaskAuditDecision, domain.PermissionAssetManage}
		}
		return []domain.PermissionCode{domain.PermissionTaskDesignSubmit, domain.PermissionTaskAuditDecision, domain.PermissionAssetManage}
	}
	if strings.Contains(path, "/audit/handover") || strings.Contains(path, "/audit/takeover") {
		if method == http.MethodGet && strings.HasSuffix(path, "/audit/handovers") {
			return []domain.PermissionCode{domain.PermissionTaskView, domain.PermissionTaskAuditDecision}
		}
		return []domain.PermissionCode{domain.PermissionTaskAuditDecision}
	}
	if strings.Contains(path, "/planning-skus") || strings.Contains(path, "/sku-planning/") {
		switch {
		case strings.Contains(path, "/export"):
			return []domain.PermissionCode{domain.PermissionPlanningSKUExport}
		case strings.Contains(path, "/erp-resync"):
			return []domain.PermissionCode{domain.PermissionPlanningSKUSync}
		case strings.Contains(path, "/erp-retry"):
			return []domain.PermissionCode{domain.PermissionPlanningSKURetry}
		case method == http.MethodGet:
			return []domain.PermissionCode{domain.PermissionPlanningSKUView}
		case method == http.MethodPatch:
			return []domain.PermissionCode{domain.PermissionPlanningSKUEdit}
		default:
			return []domain.PermissionCode{domain.PermissionPlanningSKUCreate}
		}
	}
	if strings.Contains(path, "/filing/") || strings.Contains(path, "/product-management") {
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionTaskView, domain.PermissionCatalogView}
		}
		return []domain.PermissionCode{domain.PermissionERPManage}
	}
	if strings.Contains(path, "/assets") || strings.Contains(path, "/reference-assets") || strings.Contains(path, "/asset-center") {
		if strings.Contains(path, "/download") || strings.Contains(path, "/batch-download") {
			return []domain.PermissionCode{domain.PermissionAssetDownload}
		}
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionAssetView}
		}
		return []domain.PermissionCode{domain.PermissionTaskDesignSubmit, domain.PermissionAssetManage}
	}
	if method == http.MethodGet {
		if strings.Contains(path, ".xlsx") {
			return []domain.PermissionCode{domain.PermissionTaskCreate}
		}
		return []domain.PermissionCode{domain.PermissionTaskView}
	}
	if path == "/v1/tasks" {
		return []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionPlanningSKUCreate}
	}
	if strings.Contains(path, "/reference-upload") || strings.Contains(path, "/prepare-product-codes") || strings.Contains(path, "/excel-assist/") || strings.Contains(path, "/batch-create/") {
		return []domain.PermissionCode{domain.PermissionTaskCreate}
	}
	return []domain.PermissionCode{domain.PermissionTaskManage}
}

func v8AssetRoutePermissions(method, path string) []domain.PermissionCode {
	if strings.Contains(path, "/upload-sessions") {
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionTaskView, domain.PermissionAssetView, domain.PermissionTaskDesignSubmit, domain.PermissionTaskAuditDecision, domain.PermissionAssetManage}
		}
		return []domain.PermissionCode{domain.PermissionTaskDesignSubmit, domain.PermissionTaskAuditDecision, domain.PermissionAssetManage}
	}
	if strings.Contains(path, "/download") || strings.Contains(path, "/content") || strings.HasSuffix(path, "/files/*path") {
		return []domain.PermissionCode{domain.PermissionAssetDownload}
	}
	if strings.Contains(path, "/excel-package/") {
		return []domain.PermissionCode{domain.PermissionAssetExport}
	}
	if strings.Contains(path, "/search") {
		return []domain.PermissionCode{domain.PermissionAssetView}
	}
	if method == http.MethodGet {
		return []domain.PermissionCode{domain.PermissionAssetView}
	}
	if strings.Contains(path, "/batch-download") {
		return []domain.PermissionCode{domain.PermissionAssetDownload}
	}
	return []domain.PermissionCode{domain.PermissionAssetManage}
}
