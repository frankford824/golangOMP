package transport

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/transport/handler"
)

func registerAssetWorkbenchRoutes(
	v1 *gin.RouterGroup,
	access routeAccessRegistrar,
	assetWorkbenchH *handler.AssetWorkbenchHandler,
) {
	if assetWorkbenchH == nil {
		return
	}
	group := v1.Group("/asset-workbench")
	{
		group.POST("/register", assetWorkbenchH.Register)
		group.GET("/bootstrap", access(group, http.MethodGet, "/bootstrap", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.Bootstrap)
		group.PATCH("/profile", access(group, http.MethodPatch, "/profile", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.UpsertMyProfile)
		group.GET("/profiles", access(group, http.MethodGet, "/profiles", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListProfiles)
		group.PATCH("/profiles/:user_id", access(group, http.MethodPatch, "/profiles/:user_id", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.UpsertProfile)
		group.GET("/price-matrix", access(group, http.MethodGet, "/price-matrix", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListPriceMatrix)
		group.POST("/price-matrix", access(group, http.MethodPost, "/price-matrix", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.CreatePriceMatrix)
		group.GET("/deduction-rules", access(group, http.MethodGet, "/deduction-rules", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListDeductionRules)
		group.POST("/deduction-rules", access(group, http.MethodPost, "/deduction-rules", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.CreateDeductionRule)
		group.GET("/welfare-rules", access(group, http.MethodGet, "/welfare-rules", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListWelfareRules)
		group.POST("/welfare-rules", access(group, http.MethodPost, "/welfare-rules", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.CreateWelfareRule)
		group.GET("/promo-coupons", access(group, http.MethodGet, "/promo-coupons", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListPromoCoupons)
		group.POST("/promo-coupons", access(group, http.MethodPost, "/promo-coupons", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.CreatePromoCoupon)
		group.POST("/upload-sessions", access(group, http.MethodPost, "/upload-sessions", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.CreateUploadSession)
		group.POST("/upload-sessions/:session_id/complete", access(group, http.MethodPost, "/upload-sessions/:session_id/complete", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.CompleteUploadSession)
		group.POST("/upload-sessions/:session_id/cancel", access(group, http.MethodPost, "/upload-sessions/:session_id/cancel", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.CancelUploadSession)
		group.GET("/submissions", access(group, http.MethodGet, "/submissions", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.ListSubmissions)
		group.GET("/submissions/:submission_id", access(group, http.MethodGet, "/submissions/:submission_id", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.GetSubmissionDetail)
		group.POST("/submissions", access(group, http.MethodPost, "/submissions", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.CreateSubmission)
		group.GET("/files/:file_id/preview", access(group, http.MethodGet, "/files/:file_id/preview", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.GetFilePreview)
		group.GET("/files/:file_id/download", access(group, http.MethodGet, "/files/:file_id/download", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.GetFileDownload)
		group.POST("/files/batch-download", access(group, http.MethodPost, "/files/batch-download", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.BatchDownloadFiles)
		group.PATCH("/items/:item_id/qc", access(group, http.MethodPatch, "/items/:item_id/qc", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.UpdateSubmissionItemQC)
		group.POST("/items/:item_id/void", access(group, http.MethodPost, "/items/:item_id/void", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.VoidSubmissionItem)
		group.POST("/items/:item_id/reprice", access(group, http.MethodPost, "/items/:item_id/reprice", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.RepriceSubmissionItem)
		group.POST("/error-imports", access(group, http.MethodPost, "/error-imports", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ImportErrorRecords)
		group.POST("/error-imports/excel", access(group, http.MethodPost, "/error-imports/excel", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ImportErrorRecordsExcel)
		group.GET("/settlement/preview", access(group, http.MethodGet, "/settlement/preview", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.PreviewSettlement)
		group.GET("/settlement/batches", access(group, http.MethodGet, "/settlement/batches", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListSettlementBatches)
		group.POST("/settlement/batches", access(group, http.MethodPost, "/settlement/batches", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.GenerateSettlementBatch)
		group.GET("/settlement/batches/:batch_id", access(group, http.MethodGet, "/settlement/batches/:batch_id", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.GetSettlementBatchDetail)
		group.POST("/settlement/batches/:batch_id/confirm", access(group, http.MethodPost, "/settlement/batches/:batch_id/confirm", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ConfirmSettlementBatch)
		group.POST("/settlement/batches/:batch_id/cancel", access(group, http.MethodPost, "/settlement/batches/:batch_id/cancel", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.CancelSettlementBatch)
		group.POST("/settlement/batches/:batch_id/adjustments", access(group, http.MethodPost, "/settlement/batches/:batch_id/adjustments", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.CreateSettlementAdjustment)
		group.GET("/settlement/supplement-permissions", access(group, http.MethodGet, "/settlement/supplement-permissions", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListSupplementPermissions)
		group.GET("/settlement/supplement-eligible-months", access(group, http.MethodGet, "/settlement/supplement-eligible-months", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListSupplementEligibleMonths)
		group.PUT("/settlement/supplement-permissions", access(group, http.MethodPut, "/settlement/supplement-permissions", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.UpsertSupplementPermission)
		group.GET("/settlement/supplements", access(group, http.MethodGet, "/settlement/supplements", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListSettlementSupplements)
		group.POST("/settlement/supplements", access(group, http.MethodPost, "/settlement/supplements", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.CreateSettlementSupplement)
		group.GET("/events", access(group, http.MethodGet, "/events", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListEvents)
		group.GET("/saved-views", access(group, http.MethodGet, "/saved-views", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.ListSavedViews)
		group.PUT("/saved-views", access(group, http.MethodPut, "/saved-views", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.UpsertSavedView)
		group.DELETE("/saved-views/:view_id", access(group, http.MethodDelete, "/saved-views/:view_id", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.DeleteSavedView)
		group.GET("/system-assets/:asset_id/download", access(group, http.MethodGet, "/system-assets/:asset_id/download", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.DownloadSystemAsset)
		group.POST("/system-assets/batch-download", access(group, http.MethodPost, "/system-assets/batch-download", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.BatchDownloadSystemAssets)
		group.GET("/system-search", access(group, http.MethodGet, "/system-search", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.SystemSearch)
	}
}

func assetWorkbenchRoles() []domain.Role {
	return []domain.Role{
		domain.RoleAssetSubmitter,
		domain.RoleAssetManager,
		domain.RoleAssetTemplateAdmin,
		domain.RoleAssetSettlement,
		domain.RoleHRAdmin,
		domain.RoleSuperAdmin,
	}
}
