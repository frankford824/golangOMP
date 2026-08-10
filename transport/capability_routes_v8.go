package transport

import (
	"net/http"
	"strings"

	"workflow/domain"
)

const unmappedV8RoutePermission domain.PermissionCode = "__v8_unmapped_route__"

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
		permissions, _ := v8TaskRoutePermissions(method, path)
		// The tasks route family is always capability governed. An empty result
		// means the route is unmapped and must be denied by the registrar rather
		// than falling back to retired role middleware.
		return permissions, true
	case strings.HasPrefix(path, "/v1/assets"):
		return v8AssetRoutePermissions(method, path), true
	case strings.HasPrefix(path, "/v1/asset-workbench"):
		return v8AssetWorkbenchRoutePermissions(method, path)
	case strings.HasPrefix(path, "/v1/task-board"):
		return []domain.PermissionCode{domain.PermissionTaskView}, true
	case strings.HasPrefix(path, "/v1/erp"), strings.HasPrefix(path, "/v1/products"):
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionCatalogView}, true
		}
		return []domain.PermissionCode{domain.PermissionERPManage}, true
	case strings.HasPrefix(path, "/v1/categories"), strings.HasPrefix(path, "/v1/category-mappings"), strings.HasPrefix(path, "/v1/cost-rules"), strings.HasPrefix(path, "/v1/cost-rule-bindings"), strings.HasPrefix(path, "/v1/code-rules"), strings.HasPrefix(path, "/v1/sku"):
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

func v8TaskRoutePermissions(method, path string) ([]domain.PermissionCode, bool) {
	if strings.Contains(path, "/upload-sessions") {
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionTaskView, domain.PermissionAssetView, domain.PermissionTaskCreate, domain.PermissionTaskUploadSource, domain.PermissionTaskAudit, domain.PermissionAssetManage}, true
		}
		return []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionTaskUploadSource, domain.PermissionTaskAudit, domain.PermissionAssetManage}, true
	}
	if strings.Contains(path, "/audit/handover") || strings.Contains(path, "/audit/takeover") {
		if method == http.MethodGet && strings.HasSuffix(path, "/audit/handovers") {
			return []domain.PermissionCode{domain.PermissionTaskView, domain.PermissionTaskAuditHandover}, true
		}
		return []domain.PermissionCode{domain.PermissionTaskAuditHandover}, true
	}
	if strings.Contains(path, "/audit/decision") {
		return []domain.PermissionCode{domain.PermissionTaskAudit}, true
	}
	if strings.Contains(path, "/submit-design") {
		return []domain.PermissionCode{domain.PermissionTaskUploadSource}, true
	}
	if strings.Contains(path, "/reopen") {
		return []domain.PermissionCode{domain.PermissionTaskReopen}, true
	}
	if strings.Contains(path, "/planning-skus") || strings.Contains(path, "/sku-planning/") {
		switch {
		case strings.Contains(path, "/export"):
			return []domain.PermissionCode{domain.PermissionPlanningSKUExport}, true
		case strings.Contains(path, "/erp-resync"):
			return []domain.PermissionCode{domain.PermissionPlanningSKUSync}, true
		case strings.Contains(path, "/erp-retry"):
			return []domain.PermissionCode{domain.PermissionPlanningSKURetry}, true
		case strings.Contains(path, "/image-upload-sessions"), strings.Contains(path, "/parse-excel"):
			return []domain.PermissionCode{domain.PermissionPlanningSKUCreate}, true
		case method == http.MethodGet:
			return []domain.PermissionCode{domain.PermissionPlanningSKUView}, true
		case method == http.MethodPatch:
			return []domain.PermissionCode{domain.PermissionPlanningSKUEdit}, true
		default:
			return []domain.PermissionCode{domain.PermissionPlanningSKUCreate}, true
		}
	}
	if strings.Contains(path, "/filing/") {
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionTaskView, domain.PermissionCatalogView}, true
		}
		return []domain.PermissionCode{domain.PermissionERPManage}, true
	}
	if strings.Contains(path, "/assets") || strings.Contains(path, "/reference-assets") || strings.Contains(path, "/asset-center") {
		if strings.Contains(path, "/download") || strings.Contains(path, "/batch-download") {
			return []domain.PermissionCode{domain.PermissionAssetDownload}, true
		}
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionAssetView}, true
		}
		return []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionTaskUploadSource, domain.PermissionTaskAudit, domain.PermissionAssetManage}, true
	}
	if method == http.MethodGet {
		if strings.Contains(path, ".xlsx") {
			return []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionPlanningSKUExport}, true
		}
		return []domain.PermissionCode{domain.PermissionTaskView}, true
	}
	if path == "/v1/tasks" {
		return []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionPlanningSKUCreate}, true
	}
	if strings.Contains(path, "/reference-upload") || strings.Contains(path, "/prepare-product-codes") || strings.Contains(path, "/excel-assist/") || strings.Contains(path, "/batch-create/") {
		return []domain.PermissionCode{domain.PermissionTaskCreate}, true
	}
	if strings.Contains(path, "/sku-items/") {
		return []domain.PermissionCode{domain.PermissionCatalogManage, domain.PermissionTaskCreate}, true
	}
	if strings.Contains(path, "/product-info") || strings.Contains(path, "/cost-info") || strings.Contains(path, "/business-info") {
		return []domain.PermissionCode{domain.PermissionCatalogManage}, true
	}
	if strings.Contains(path, "/cost-quote/preview") {
		return []domain.PermissionCode{domain.PermissionCatalogView}, true
	}
	if strings.Contains(path, "/filing/retry") {
		return []domain.PermissionCode{domain.PermissionERPManage}, true
	}
	if strings.Contains(path, "/batch/remind") {
		return []domain.PermissionCode{domain.PermissionTaskView}, true
	}
	if strings.Contains(path, "/modules/") {
		switch {
		case strings.Contains(path, "/claim"):
			return []domain.PermissionCode{domain.PermissionTaskUploadSource, domain.PermissionTaskAudit}, true
		case strings.Contains(path, "/actions/"):
			return []domain.PermissionCode{domain.PermissionTaskUploadSource}, true
		}
	}
	if strings.Contains(path, "/cancel") {
		return []domain.PermissionCode{domain.PermissionTaskTerminate}, true
	}
	if strings.Contains(path, "/batch/assign") || strings.HasSuffix(path, "/assign") {
		return []domain.PermissionCode{domain.PermissionTaskAssign}, true
	}
	return nil, false
}

func v8AssetRoutePermissions(method, path string) []domain.PermissionCode {
	if method == http.MethodPost && (path == "/v1/assets/batch-download" || path == "/v1/assets/excel-package/preview" || path == "/v1/assets/excel-package/preview-file") {
		// Production packaging is available to every authenticated asset-center
		// viewer. Keep the wider asset.download capability on single-file,
		// task-attachment and asset-workbench download surfaces.
		return []domain.PermissionCode{domain.PermissionAssetView}
	}
	if strings.Contains(path, "/upload-sessions") {
		if method == http.MethodGet {
			return []domain.PermissionCode{domain.PermissionTaskView, domain.PermissionAssetView, domain.PermissionTaskCreate, domain.PermissionTaskUploadSource, domain.PermissionTaskAudit, domain.PermissionAssetManage}
		}
		return []domain.PermissionCode{domain.PermissionTaskCreate, domain.PermissionTaskUploadSource, domain.PermissionTaskAudit, domain.PermissionAssetManage}
	}
	if strings.Contains(path, "/download") || strings.Contains(path, "/content") || strings.HasSuffix(path, "/files/*path") {
		return []domain.PermissionCode{domain.PermissionAssetDownload}
	}
	if method == http.MethodGet {
		return []domain.PermissionCode{domain.PermissionAssetView}
	}
	if strings.Contains(path, "/batch-download") {
		return []domain.PermissionCode{domain.PermissionAssetDownload}
	}
	return []domain.PermissionCode{domain.PermissionAssetManage}
}
