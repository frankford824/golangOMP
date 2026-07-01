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
		group.GET("/entry", access(group, http.MethodGet, "/entry", domain.APIReadinessReadyForFrontend), assetWorkbenchH.Entry)
		group.POST("/access/request", access(group, http.MethodPost, "/access/request", domain.APIReadinessReadyForFrontend), assetWorkbenchH.RequestAccess)

		protected := group.Group("")
		protected.Use(assetWorkbenchH.RequireActiveMembership())
		group = protected

		group.POST("/access/open", access(group, http.MethodPost, "/access/open", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.OpenAccess)
		group.POST("/access/disable", access(group, http.MethodPost, "/access/disable", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin), assetWorkbenchH.DisableAccess)
		group.GET("/bootstrap", access(group, http.MethodGet, "/bootstrap", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.Bootstrap)
		group.PATCH("/profile", access(group, http.MethodPatch, "/profile", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.UpsertMyProfile)
		group.GET("/profiles", access(group, http.MethodGet, "/profiles", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListProfiles)
		group.PATCH("/profiles/:user_id", access(group, http.MethodPatch, "/profiles/:user_id", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.UpsertProfile)
		group.GET("/members", access(group, http.MethodGet, "/members", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.ListMembers)
		group.PATCH("/members/:user_id/identity", access(group, http.MethodPatch, "/members/:user_id/identity", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin), assetWorkbenchH.UpdateMemberIdentity)
		group.PATCH("/members/:user_id/roles", access(group, http.MethodPatch, "/members/:user_id/roles", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin), assetWorkbenchH.UpdateMemberRoles)
		group.POST("/accounts/merge/preview", access(group, http.MethodPost, "/accounts/merge/preview", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin), assetWorkbenchH.PreviewAccountMerge)
		group.POST("/accounts/merge", access(group, http.MethodPost, "/accounts/merge", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin), assetWorkbenchH.MergeAccounts)
		group.GET("/people-lookup", access(group, http.MethodGet, "/people-lookup", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.SearchPeople)
		group.GET("/groups", access(group, http.MethodGet, "/groups", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.ListGroups)
		group.POST("/groups", access(group, http.MethodPost, "/groups", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.CreateGroup)
		group.PATCH("/groups/:group_id", access(group, http.MethodPatch, "/groups/:group_id", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.UpdateGroup)
		group.DELETE("/groups/:group_id", access(group, http.MethodDelete, "/groups/:group_id", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.DeleteGroup)
		group.GET("/groups/:group_id/members", access(group, http.MethodGet, "/groups/:group_id/members", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.ListGroupMembers)
		group.PUT("/groups/:group_id/members", access(group, http.MethodPut, "/groups/:group_id/members", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.AddGroupMembers)
		group.DELETE("/groups/:group_id/members", access(group, http.MethodDelete, "/groups/:group_id/members", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.RemoveGroupMembers)
		group.GET("/upload-directories", access(group, http.MethodGet, "/upload-directories", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.ListUploadDirectories)
		group.GET("/upload-directories/admin", access(group, http.MethodGet, "/upload-directories/admin", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.ListUploadDirectoriesAdmin)
		group.POST("/upload-directories", access(group, http.MethodPost, "/upload-directories", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.CreateUploadDirectory)
		group.PATCH("/upload-directories/:directory_id", access(group, http.MethodPatch, "/upload-directories/:directory_id", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.UpdateUploadDirectory)
		group.GET("/difficulty-classes", access(group, http.MethodGet, "/difficulty-classes", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.ListDifficultyClasses)
		group.GET("/difficulty-classes/admin", access(group, http.MethodGet, "/difficulty-classes/admin", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListDifficultyClassesAdmin)
		group.POST("/difficulty-classes", access(group, http.MethodPost, "/difficulty-classes", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.CreateDifficultyClass)
		group.PATCH("/difficulty-classes/:difficulty_code", access(group, http.MethodPatch, "/difficulty-classes/:difficulty_code", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.UpdateDifficultyClass)
		group.GET("/price-matrix", access(group, http.MethodGet, "/price-matrix", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListPriceMatrix)
		group.POST("/price-matrix", access(group, http.MethodPost, "/price-matrix", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.CreatePriceMatrix)
		group.PATCH("/price-matrix/:rule_id", access(group, http.MethodPatch, "/price-matrix/:rule_id", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.UpdatePriceMatrix)
		group.POST("/price-matrix/:rule_id/supersede", access(group, http.MethodPost, "/price-matrix/:rule_id/supersede", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.SupersedePriceMatrix)
		group.GET("/deduction-rules", access(group, http.MethodGet, "/deduction-rules", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListDeductionRules)
		group.POST("/deduction-rules", access(group, http.MethodPost, "/deduction-rules", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.CreateDeductionRule)
		group.PATCH("/deduction-rules/:rule_id", access(group, http.MethodPatch, "/deduction-rules/:rule_id", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.UpdateDeductionRule)
		group.POST("/deduction-rules/:rule_id/supersede", access(group, http.MethodPost, "/deduction-rules/:rule_id/supersede", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.SupersedeDeductionRule)
		group.GET("/welfare-rules", access(group, http.MethodGet, "/welfare-rules", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListWelfareRules)
		group.POST("/welfare-rules", access(group, http.MethodPost, "/welfare-rules", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.CreateWelfareRule)
		group.PATCH("/welfare-rules/:rule_id", access(group, http.MethodPatch, "/welfare-rules/:rule_id", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.UpdateWelfareRule)
		group.POST("/welfare-rules/:rule_id/supersede", access(group, http.MethodPost, "/welfare-rules/:rule_id/supersede", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.SupersedeWelfareRule)
		group.GET("/promo-coupons", access(group, http.MethodGet, "/promo-coupons", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListPromoCoupons)
		group.POST("/promo-coupons", access(group, http.MethodPost, "/promo-coupons", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.CreatePromoCoupon)
		group.PATCH("/promo-coupons/:rule_id", access(group, http.MethodPatch, "/promo-coupons/:rule_id", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.UpdatePromoCoupon)
		group.POST("/promo-coupons/:rule_id/supersede", access(group, http.MethodPost, "/promo-coupons/:rule_id/supersede", domain.APIReadinessReadyForFrontend, domain.RoleAssetTemplateAdmin, domain.RoleSuperAdmin), assetWorkbenchH.SupersedePromoCoupon)
		group.POST("/upload-sessions", access(group, http.MethodPost, "/upload-sessions", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.CreateUploadSession)
		group.POST("/upload-sessions/:session_id/complete", access(group, http.MethodPost, "/upload-sessions/:session_id/complete", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.CompleteUploadSession)
		group.POST("/upload-sessions/:session_id/cancel", access(group, http.MethodPost, "/upload-sessions/:session_id/cancel", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.CancelUploadSession)
		group.GET("/overview-search", access(group, http.MethodGet, "/overview-search", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.OverviewSearch)
		group.GET("/submissions", access(group, http.MethodGet, "/submissions", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.ListSubmissions)
		group.GET("/submissions/:submission_id", access(group, http.MethodGet, "/submissions/:submission_id", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.GetSubmissionDetail)
		group.POST("/submissions", access(group, http.MethodPost, "/submissions", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.CreateSubmission)
		group.POST("/submissions/:submission_id/void", access(group, http.MethodPost, "/submissions/:submission_id/void", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.VoidSubmission)
		group.GET("/files/:file_id/preview", access(group, http.MethodGet, "/files/:file_id/preview", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.GetFilePreview)
		group.GET("/files/:file_id/download", access(group, http.MethodGet, "/files/:file_id/download", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.GetFileDownload)
		group.POST("/files/batch-download", access(group, http.MethodPost, "/files/batch-download", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.BatchDownloadFiles)
		group.POST("/files/batch-move", access(group, http.MethodPost, "/files/batch-move", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.BatchMoveFiles)
		group.POST("/files/batch-delete", access(group, http.MethodPost, "/files/batch-delete", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.BatchDeleteFiles)
		group.PATCH("/items/:item_id", access(group, http.MethodPatch, "/items/:item_id", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.UpdateSubmissionItem)
		group.POST("/items/qc/excel", access(group, http.MethodPost, "/items/qc/excel", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ImportSubmissionItemQCExcel)
		group.PATCH("/items/:item_id/qc", access(group, http.MethodPatch, "/items/:item_id/qc", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.UpdateSubmissionItemQC)
		group.POST("/items/:item_id/void", access(group, http.MethodPost, "/items/:item_id/void", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.VoidSubmissionItem)
		group.POST("/items/:item_id/reprice", access(group, http.MethodPost, "/items/:item_id/reprice", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.RepriceSubmissionItem)
		group.POST("/error-imports", access(group, http.MethodPost, "/error-imports", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ImportErrorRecords)
		group.POST("/error-imports/excel", access(group, http.MethodPost, "/error-imports/excel", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ImportErrorRecordsExcel)
		group.GET("/settlement/my", access(group, http.MethodGet, "/settlement/my", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.MySettlement)
		group.GET("/settlement/preview", access(group, http.MethodGet, "/settlement/preview", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.PreviewSettlement)
		group.GET("/settlement/report", access(group, http.MethodGet, "/settlement/report", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.SettlementReport)
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
		group.POST("/settlement/supplements/excel", access(group, http.MethodPost, "/settlement/supplements/excel", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ImportSettlementSupplementsExcel)
		group.DELETE("/settlement/supplements/:supplement_id", access(group, http.MethodDelete, "/settlement/supplements/:supplement_id", domain.APIReadinessReadyForFrontend, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.DeleteSettlementSupplement)
		group.GET("/events", access(group, http.MethodGet, "/events", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin), assetWorkbenchH.ListEvents)
		group.GET("/saved-views", access(group, http.MethodGet, "/saved-views", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.ListSavedViews)
		group.PUT("/saved-views", access(group, http.MethodPut, "/saved-views", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.UpsertSavedView)
		group.DELETE("/saved-views/:view_id", access(group, http.MethodDelete, "/saved-views/:view_id", domain.APIReadinessReadyForFrontend, assetWorkbenchRoles()...), assetWorkbenchH.DeleteSavedView)
		group.GET("/client-materials", access(group, http.MethodGet, "/client-materials", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.ListClientMaterials)
		group.POST("/client-materials", access(group, http.MethodPost, "/client-materials", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.CreateClientMaterial)
		group.PATCH("/client-materials/:material_id", access(group, http.MethodPatch, "/client-materials/:material_id", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.UpdateClientMaterial)
		group.DELETE("/client-materials/:material_id", access(group, http.MethodDelete, "/client-materials/:material_id", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.DeleteClientMaterial)
		group.GET("/client-materials/:material_id/download", access(group, http.MethodGet, "/client-materials/:material_id/download", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.DownloadClientMaterial)
		group.GET("/client-materials/:material_id/preview", access(group, http.MethodGet, "/client-materials/:material_id/preview", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.PreviewClientMaterial)
		group.POST("/client-materials/batch-download", access(group, http.MethodPost, "/client-materials/batch-download", domain.APIReadinessReadyForFrontend, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.BatchDownloadClientMaterials)
		group.GET("/system-assets/:asset_id/download", access(group, http.MethodGet, "/system-assets/:asset_id/download", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.DownloadSystemAsset)
		group.GET("/system-assets/:asset_id/preview", access(group, http.MethodGet, "/system-assets/:asset_id/preview", domain.APIReadinessReadyForFrontend, domain.RoleAssetManager, domain.RoleSuperAdmin), assetWorkbenchH.PreviewSystemAsset)
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
