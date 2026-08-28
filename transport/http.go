package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"workflow/domain"
	"workflow/transport/handler"
	transportws "workflow/transport/ws"
)

const traceIDKey = "trace_id"

// NewRouter builds the gin router with all routes registered per spec §7.2.
func NewRouter(
	authH *handler.AuthHandler,
	accessPolicyH *handler.AccessPolicyHandler,
	userAdminH *handler.UserAdminHandler,
	erpBridgeH *handler.ERPBridgeHandler,
	productManagementH *handler.ProductManagementHandler,
	categoryH *handler.CategoryHandler,
	categoryMappingH *handler.CategoryERPMappingHandler,
	costRuleH *handler.CostRuleHandler,
	costRuleBindingH *handler.CostRuleBindingHandler,
	taskH *handler.TaskHandler,
	taskAssignmentH *handler.TaskAssignmentHandler,
	taskAssetCenterH *handler.TaskAssetCenterHandler,
	taskCreateReferenceUploadH *handler.TaskCreateReferenceUploadHandler,
	assetFilesH *handler.AssetFilesHandler,
	taskResourceWorkflowH *handler.TaskResourceWorkflowHandler,
	planningSKUH *handler.PlanningSKUHandler,
	taskDetailH *handler.TaskDetailHandler,
	taskCostOverrideH *handler.TaskCostOverrideHandler,
	taskBoardH *handler.TaskBoardHandler,
	taskBatchExcelH *handler.TaskBatchExcelHandler,
	taskSingleExcelH *handler.TaskSingleExcelHandler,
	assetWorkbenchH *handler.AssetWorkbenchHandler,
	integrationCenterH *handler.IntegrationCenterHandler,
	codeRuleH *handler.CodeRuleHandler,
	auditV7H *handler.AuditV7Handler,
	serverLogH *handler.ServerLogHandler,
	taskDraftH *handler.TaskDraftHandler,
	notificationH *handler.NotificationHandler,
	erpProductH *handler.ERPProductHandler,
	designSourceH *handler.DesignSourceHandler,
	searchH *handler.SearchHandler,
	aiChatH *handler.AIChatHandler,
	wsH *transportws.Handler,
	routeAccessCatalog *RouteAccessCatalog,
	actorResolver RequestActorResolver,
	permissionLogger PermissionLogWriter,
	effectiveAccessResolver EffectiveAccessResolver,
	logger *zap.Logger,
	traceRecorders ...workflowTraceRecorder,
) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	var traceRecorder workflowTraceRecorder
	if len(traceRecorders) > 0 {
		traceRecorder = traceRecorders[0]
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(injectTraceID())
	r.Use(injectRequestActor(actorResolver))
	r.Use(requestLogger(logger, serverLogH, traceRecorder))
	registerOperationalRoutes(r)

	v1 := r.Group("/v1")
	if routeAccessCatalog == nil {
		routeAccessCatalog = NewRouteAccessCatalog()
	}
	routeAccessCatalog.Reset()

	capabilityAccess := func(group *gin.RouterGroup, method, path string, readiness domain.APIReadiness, permissions ...domain.PermissionCode) gin.HandlerFunc {
		fullPath := joinRoutePath(group.BasePath(), path)
		routeAccessCatalog.AddRule(domain.NewCapabilityRouteAccessRule(method, fullPath, readiness, permissions...))
		return withCapabilityAccess(permissionLogger, effectiveAccessResolver, readiness, permissions...)
	}
	erpInternalOrCapabilityAccess := func(group *gin.RouterGroup, method, path string, readiness domain.APIReadiness, permissions ...domain.PermissionCode) gin.HandlerFunc {
		fullPath := joinRoutePath(group.BasePath(), path)
		routeAccessCatalog.AddRule(domain.NewCapabilityRouteAccessRule(method, fullPath, readiness, permissions...))
		return withERPBridgeInternalOrCapabilityAccess(permissionLogger, effectiveAccessResolver, readiness, permissions...)
	}
	access := func(group *gin.RouterGroup, method, path string, readiness domain.APIReadiness, roles ...domain.Role) gin.HandlerFunc {
		fullPath := joinRoutePath(group.BasePath(), path)
		if readiness == domain.APIReadinessReadyForFrontend {
			if permissions, governed := v8BusinessRoutePermissions(method, fullPath); governed {
				if len(permissions) == 0 {
					permissions = []domain.PermissionCode{unmappedV8RoutePermission}
				}
				routeAccessCatalog.AddRule(domain.NewCapabilityRouteAccessRule(method, fullPath, readiness, permissions...))
				return withCapabilityAccess(permissionLogger, effectiveAccessResolver, readiness, permissions...)
			}
		}
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(method, fullPath, readiness, roles...))
		return withAccessMetaAndLogger(permissionLogger, readiness, roles...)
	}
	registerV1IdentityRoutes(r, v1, routeAccessCatalog, permissionLogger, capabilityAccess, authH, taskDraftH, designSourceH, searchH, notificationH, wsH)

	if aiChatH != nil {
		aiChatGroup := v1.Group("/ai/chat")
		aiChatGroup.GET("/config", capabilityAccess(aiChatGroup, http.MethodGet, "/config", domain.APIReadinessReadyForFrontend, domain.PermissionReportView), aiChatH.Config)
		aiChatGroup.GET("/conversations", capabilityAccess(aiChatGroup, http.MethodGet, "/conversations", domain.APIReadinessReadyForFrontend, domain.PermissionReportView), aiChatH.ListConversations)
		aiChatGroup.POST("/conversations", capabilityAccess(aiChatGroup, http.MethodPost, "/conversations", domain.APIReadinessReadyForFrontend, domain.PermissionReportView), aiChatH.CreateConversation)
		aiChatGroup.GET("/conversations/:conversation_id", capabilityAccess(aiChatGroup, http.MethodGet, "/conversations/:conversation_id", domain.APIReadinessReadyForFrontend, domain.PermissionReportView), aiChatH.GetConversation)
		aiChatGroup.DELETE("/conversations/:conversation_id", capabilityAccess(aiChatGroup, http.MethodDelete, "/conversations/:conversation_id", domain.APIReadinessReadyForFrontend, domain.PermissionReportView), aiChatH.DeleteConversation)
		aiChatGroup.POST("/conversations/:conversation_id/messages:stream", capabilityAccess(aiChatGroup, http.MethodPost, "/conversations/:conversation_id/messages:stream", domain.APIReadinessReadyForFrontend, domain.PermissionReportView), aiChatH.StreamMessage)
		aiChatGroup.GET("/admin/conversations", capabilityAccess(aiChatGroup, http.MethodGet, "/admin/conversations", domain.APIReadinessReadyForFrontend, domain.PermissionReportView), aiChatH.AdminListConversations)
		aiChatGroup.GET("/admin/conversations/:conversation_id", capabilityAccess(aiChatGroup, http.MethodGet, "/admin/conversations/:conversation_id", domain.APIReadinessReadyForFrontend, domain.PermissionReportView), aiChatH.AdminGetConversation)

		analyticsGroup := v1.Group("/analytics")
		analyticsGroup.GET("/mcp", capabilityAccess(analyticsGroup, http.MethodGet, "/mcp", domain.APIReadinessReadyForFrontend, domain.PermissionReportView), aiChatH.AnalyticsMCPGet)
		analyticsGroup.POST("/mcp", capabilityAccess(analyticsGroup, http.MethodPost, "/mcp", domain.APIReadinessReadyForFrontend, domain.PermissionReportView), aiChatH.AnalyticsMCPPost)
	}

	if accessPolicyH != nil {
		accessGroup := v1.Group("/access")
		accessGroup.GET("/permissions", capabilityAccess(accessGroup, http.MethodGet, "/permissions", domain.APIReadinessReadyForFrontend, domain.PermissionAccessView, domain.PermissionAccessManage), accessPolicyH.ListPermissions)
		accessGroup.GET("/roles", capabilityAccess(accessGroup, http.MethodGet, "/roles", domain.APIReadinessReadyForFrontend, domain.PermissionAccessView, domain.PermissionAccessManage), accessPolicyH.ListRoles)
		accessGroup.POST("/roles", capabilityAccess(accessGroup, http.MethodPost, "/roles", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), accessPolicyH.CreateRole)
		accessGroup.PATCH("/roles/:id", capabilityAccess(accessGroup, http.MethodPatch, "/roles/:id", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), accessPolicyH.UpdateRole)
		accessGroup.POST("/roles/:id/archive", capabilityAccess(accessGroup, http.MethodPost, "/roles/:id/archive", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), accessPolicyH.ArchiveRole)
		accessGroup.PUT("/roles/:id/permissions", capabilityAccess(accessGroup, http.MethodPut, "/roles/:id/permissions", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), accessPolicyH.ReplaceRolePermissions)
		accessGroup.GET("/users", capabilityAccess(accessGroup, http.MethodGet, "/users", domain.APIReadinessReadyForFrontend, domain.PermissionAccessView, domain.PermissionAccessManage), userAdminH.ListAccessPolicyUsers)
		accessGroup.PUT("/users/:id/assignments", capabilityAccess(accessGroup, http.MethodPut, "/users/:id/assignments", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), accessPolicyH.ReplaceUserAssignments)
		accessGroup.GET("/users/:id/effective", capabilityAccess(accessGroup, http.MethodGet, "/users/:id/effective", domain.APIReadinessReadyForFrontend, domain.PermissionAccessView, domain.PermissionAccessManage), accessPolicyH.EffectiveAccess)
		accessGroup.GET("/org-policies/:subject_type/:subject_id", capabilityAccess(accessGroup, http.MethodGet, "/org-policies/:subject_type/:subject_id", domain.APIReadinessReadyForFrontend, domain.PermissionAccessView, domain.PermissionAccessManage), accessPolicyH.GetOrgPolicies)
		accessGroup.PUT("/org-policies/:subject_type/:subject_id", capabilityAccess(accessGroup, http.MethodPut, "/org-policies/:subject_type/:subject_id", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), accessPolicyH.ReplaceOrgPolicies)
		accessGroup.POST("/preview", capabilityAccess(accessGroup, http.MethodPost, "/preview", domain.APIReadinessReadyForFrontend, domain.PermissionAccessView, domain.PermissionAccessManage), accessPolicyH.Preview)
		accessGroup.GET("/events", capabilityAccess(accessGroup, http.MethodGet, "/events", domain.APIReadinessReadyForFrontend, domain.PermissionAccessView, domain.PermissionAccessManage), accessPolicyH.ListEvents)
	}

	if assetFilesH != nil {
		v1.GET("/public/erp-product-images/:version_id", assetFilesH.ServeERPProductImage)
	}

	registerV1AdminRoutes(v1, capabilityAccess, userAdminH, notificationH)
	registerAssetWorkbenchRoutes(v1, access, capabilityAccess, assetWorkbenchH, notificationH)

	v1.POST("/trace-events", capabilityAccess(v1, http.MethodPost, "/trace-events", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), userAdminH.RecordWorkflowTraceEvent)

	// Audit (idempotent via action_id)

	if productManagementH != nil {
		costManagementGroup := v1.Group("/cost-management")
		{
			costManagementGroup.GET("/dashboard", capabilityAccess(costManagementGroup, http.MethodGet, "/dashboard", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), productManagementH.CostDashboard)
			costManagementGroup.POST("/recalculation-runs", capabilityAccess(costManagementGroup, http.MethodPost, "/recalculation-runs", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), productManagementH.CreateCostRecalculationRun)
			costManagementGroup.GET("/recalculation-runs", capabilityAccess(costManagementGroup, http.MethodGet, "/recalculation-runs", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), productManagementH.ListCostRecalculationRuns)
			costManagementGroup.GET("/recalculation-runs/:run_id", capabilityAccess(costManagementGroup, http.MethodGet, "/recalculation-runs/{run_id}", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), productManagementH.GetCostRecalculationRun)
			costManagementGroup.POST("/recalculation-runs/:run_id/apply", capabilityAccess(costManagementGroup, http.MethodPost, "/recalculation-runs/{run_id}/apply", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), productManagementH.ApplyCostRecalculationRun)
			costManagementGroup.POST("/recalculation-runs/:run_id/sync-erp", capabilityAccess(costManagementGroup, http.MethodPost, "/recalculation-runs/{run_id}/sync-erp", domain.APIReadinessReadyForFrontend, domain.PermissionERPManage), productManagementH.SyncERPCostRecalculationRun)
			costManagementGroup.POST("/recalculation-runs/:run_id/cancel", capabilityAccess(costManagementGroup, http.MethodPost, "/recalculation-runs/{run_id}/cancel", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), productManagementH.CancelCostRecalculationRun)
		}
	}

	erpGroup := v1.Group("/erp")
	{
		erpGroup.GET("/products", capabilityAccess(erpGroup, http.MethodGet, "/products", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), erpBridgeH.SearchProducts)
		erpGroup.GET("/iids", capabilityAccess(erpGroup, http.MethodGet, "/iids", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView, domain.PermissionTaskCreate, domain.PermissionPlanningSKUCreate), erpBridgeH.ListIIDs)
		erpGroup.GET("/products/*id", erpInternalOrCapabilityAccess(erpGroup, http.MethodGet, "/products/{id}", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), func(c *gin.Context) {
			if erpProductH != nil && strings.Trim(strings.TrimSpace(c.Param("id")), "/") == "by-code" {
				erpProductH.ByCode(c)
				return
			}
			erpBridgeH.GetProductByID(c)
		})
		erpGroup.GET("/categories", erpInternalOrCapabilityAccess(erpGroup, http.MethodGet, "/categories", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), erpBridgeH.ListCategories)
		erpGroup.GET("/warehouses", capabilityAccess(erpGroup, http.MethodGet, "/warehouses", domain.APIReadinessReadyForFrontend, domain.PermissionERPManage), erpBridgeH.ListWarehouses)
		erpGroup.GET("/sync-logs", capabilityAccess(erpGroup, http.MethodGet, "/sync-logs", domain.APIReadinessReadyForFrontend, domain.PermissionERPManage), erpBridgeH.ListSyncLogs)
		erpGroup.GET("/sync-logs/*id", capabilityAccess(erpGroup, http.MethodGet, "/sync-logs/{id}", domain.APIReadinessReadyForFrontend, domain.PermissionERPManage), erpBridgeH.GetSyncLogByID)
		erpGroup.POST("/products/upsert", erpInternalOrCapabilityAccess(erpGroup, http.MethodPost, "/products/upsert", domain.APIReadinessReadyForFrontend, domain.PermissionERPManage), erpBridgeH.UpsertProduct)
		erpGroup.POST("/products/style/update", erpInternalOrCapabilityAccess(erpGroup, http.MethodPost, "/products/style/update", domain.APIReadinessReadyForFrontend, domain.PermissionERPManage), erpBridgeH.UpdateItemStyle)
		erpGroup.POST("/products/shelve/batch", erpInternalOrCapabilityAccess(erpGroup, http.MethodPost, "/products/shelve/batch", domain.APIReadinessReadyForFrontend, domain.PermissionERPManage), erpBridgeH.ShelveProductsBatch)
		erpGroup.POST("/products/unshelve/batch", erpInternalOrCapabilityAccess(erpGroup, http.MethodPost, "/products/unshelve/batch", domain.APIReadinessReadyForFrontend, domain.PermissionERPManage), erpBridgeH.UnshelveProductsBatch)
		erpGroup.POST("/inventory/virtual-qty", erpInternalOrCapabilityAccess(erpGroup, http.MethodPost, "/inventory/virtual-qty", domain.APIReadinessReadyForFrontend, domain.PermissionERPManage), erpBridgeH.UpdateVirtualInventory)
	}

	categoryGroup := v1.Group("/categories")
	{
		categoryGroup.GET("", capabilityAccess(categoryGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), categoryH.List)
		categoryGroup.GET("/search", capabilityAccess(categoryGroup, http.MethodGet, "/search", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), categoryH.Search)
		categoryGroup.GET("/:id", capabilityAccess(categoryGroup, http.MethodGet, "/:id", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), categoryH.GetByID)
		categoryGroup.POST("", capabilityAccess(categoryGroup, http.MethodPost, "", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), categoryH.Create)
		categoryGroup.PATCH("/:id", capabilityAccess(categoryGroup, http.MethodPatch, "/:id", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), categoryH.Patch)
	}

	categoryMappingGroup := v1.Group("/category-mappings")
	{
		categoryMappingGroup.GET("", capabilityAccess(categoryMappingGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), categoryMappingH.List)
		categoryMappingGroup.GET("/search", capabilityAccess(categoryMappingGroup, http.MethodGet, "/search", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), categoryMappingH.Search)
		categoryMappingGroup.GET("/:id", capabilityAccess(categoryMappingGroup, http.MethodGet, "/:id", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), categoryMappingH.GetByID)
		categoryMappingGroup.POST("", capabilityAccess(categoryMappingGroup, http.MethodPost, "", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), categoryMappingH.Create)
		categoryMappingGroup.PATCH("/:id", capabilityAccess(categoryMappingGroup, http.MethodPatch, "/:id", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), categoryMappingH.Patch)
	}

	costRuleGroup := v1.Group("/cost-rules")
	{
		costRuleGroup.GET("", capabilityAccess(costRuleGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), costRuleH.List)
		costRuleGroup.GET("/:id", capabilityAccess(costRuleGroup, http.MethodGet, "/:id", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), costRuleH.GetByID)
		costRuleGroup.GET("/:id/history", capabilityAccess(costRuleGroup, http.MethodGet, "/:id/history", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), costRuleH.GetHistory)
		costRuleGroup.POST("", capabilityAccess(costRuleGroup, http.MethodPost, "", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), costRuleH.Create)
		costRuleGroup.PATCH("/:id", capabilityAccess(costRuleGroup, http.MethodPatch, "/:id", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), costRuleH.Patch)
		costRuleGroup.POST("/preview", capabilityAccess(costRuleGroup, http.MethodPost, "/preview", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), costRuleH.Preview)
	}

	if costRuleBindingH != nil {
		costRuleBindingGroup := v1.Group("/cost-rule-bindings")
		{
			costRuleBindingGroup.GET("", capabilityAccess(costRuleBindingGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), costRuleBindingH.List)
			costRuleBindingGroup.POST("", capabilityAccess(costRuleBindingGroup, http.MethodPost, "", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), costRuleBindingH.Create)
			costRuleBindingGroup.GET("/unbound-candidates", capabilityAccess(costRuleBindingGroup, http.MethodGet, "/unbound-candidates", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), costRuleBindingH.ListUnboundCandidates)
			costRuleBindingGroup.PATCH("/:id", capabilityAccess(costRuleBindingGroup, http.MethodPatch, "/:id", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), costRuleBindingH.Patch)
		}
	}

	// V8: task aggregate root. Organization names are display-only; capability and
	// stable organization-ID scope are resolved by capabilityAccess/requestActor.
	taskGroup := v1.Group("/tasks")
	{
		taskGroup.POST("/reference-upload-sessions", capabilityAccess(taskGroup, http.MethodPost, "/reference-upload-sessions", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskCreateReferenceUploadH.CreateUploadSession)
		taskGroup.GET("/reference-upload-sessions/:session_id", capabilityAccess(taskGroup, http.MethodGet, "/reference-upload-sessions/:session_id", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskCreateReferenceUploadH.GetUploadSession)
		taskGroup.POST("/reference-upload-sessions/:session_id/complete", capabilityAccess(taskGroup, http.MethodPost, "/reference-upload-sessions/:session_id/complete", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskCreateReferenceUploadH.CompleteUploadSession)
		taskGroup.POST("/reference-upload-sessions/:session_id/abort", capabilityAccess(taskGroup, http.MethodPost, "/reference-upload-sessions/:session_id/abort", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskCreateReferenceUploadH.AbortUploadSession)
		taskGroup.POST("/reference-upload", capabilityAccess(taskGroup, http.MethodPost, "/reference-upload", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskCreateReferenceUploadH.UploadFile)
		taskGroup.POST("/prepare-product-codes", capabilityAccess(taskGroup, http.MethodPost, "/prepare-product-codes", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskH.PrepareProductCodes)
		taskGroup.GET("/excel-assist/template.xlsx", capabilityAccess(taskGroup, http.MethodGet, "/excel-assist/template.xlsx", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskSingleExcelH.DownloadTemplate)
		taskGroup.POST("/excel-assist/parse-excel", capabilityAccess(taskGroup, http.MethodPost, "/excel-assist/parse-excel", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskSingleExcelH.ParseUpload)
		taskGroup.GET("/batch-create/template.xlsx", capabilityAccess(taskGroup, http.MethodGet, "/batch-create/template.xlsx", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskBatchExcelH.DownloadTemplate)
		taskGroup.POST("/batch-create/parse-excel", capabilityAccess(taskGroup, http.MethodPost, "/batch-create/parse-excel", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskBatchExcelH.ParseUpload)
		taskGroup.POST("/sku-planning/image-upload-sessions", capabilityAccess(taskGroup, http.MethodPost, "/sku-planning/image-upload-sessions", domain.APIReadinessReadyForFrontend, domain.PermissionPlanningSKUCreate), taskCreateReferenceUploadH.CreatePlanningSKUImageUploadSession)
		taskGroup.GET("/sku-planning/image-upload-sessions/:session_id", capabilityAccess(taskGroup, http.MethodGet, "/sku-planning/image-upload-sessions/:session_id", domain.APIReadinessReadyForFrontend, domain.PermissionPlanningSKUCreate), taskCreateReferenceUploadH.GetUploadSession)
		taskGroup.POST("/sku-planning/image-upload-sessions/:session_id/complete", capabilityAccess(taskGroup, http.MethodPost, "/sku-planning/image-upload-sessions/:session_id/complete", domain.APIReadinessReadyForFrontend, domain.PermissionPlanningSKUCreate), taskCreateReferenceUploadH.CompletePlanningSKUImageUploadSession)
		taskGroup.POST("/sku-planning/image-upload-sessions/:session_id/abort", capabilityAccess(taskGroup, http.MethodPost, "/sku-planning/image-upload-sessions/:session_id/abort", domain.APIReadinessReadyForFrontend, domain.PermissionPlanningSKUCreate), taskCreateReferenceUploadH.AbortPlanningSKUImageUploadSession)
		taskGroup.POST("", capabilityAccess(taskGroup, http.MethodPost, "", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate, domain.PermissionPlanningSKUCreate), taskH.Create)
		taskGroup.GET("", capabilityAccess(taskGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView), taskH.List)
		taskGroup.GET("/filter-options", capabilityAccess(taskGroup, http.MethodGet, "/filter-options", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView), taskH.FilterOptions)
		if auditV7H != nil {
			taskGroup.GET("/audit/handover-candidates", capabilityAccess(taskGroup, http.MethodGet, "/audit/handover-candidates", domain.APIReadinessReadyForFrontend, domain.PermissionTaskAuditHandover), auditV7H.ListHandoverCandidates)
			taskGroup.POST("/audit/handover-batch", capabilityAccess(taskGroup, http.MethodPost, "/audit/handover-batch", domain.APIReadinessReadyForFrontend, domain.PermissionTaskAuditHandover), auditV7H.BatchHandover)
		}
		taskGroup.GET("/:id", capabilityAccess(taskGroup, http.MethodGet, "/:id", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView), taskH.GetByID)
		taskGroup.GET("/:id/product-info", capabilityAccess(taskGroup, http.MethodGet, "/:id/product-info", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView), taskH.GetProductInfo)
		taskGroup.PATCH("/:id/product-info", capabilityAccess(taskGroup, http.MethodPatch, "/:id/product-info", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), taskH.PatchProductInfo)
		taskGroup.GET("/:id/cost-info", capabilityAccess(taskGroup, http.MethodGet, "/:id/cost-info", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView), taskH.GetCostInfo)
		taskGroup.PATCH("/:id/cost-info", capabilityAccess(taskGroup, http.MethodPatch, "/:id/cost-info", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage), taskH.PatchCostInfo)
		taskGroup.PATCH("/:id/sku-items/:sku_item_id", capabilityAccess(taskGroup, http.MethodPatch, "/:id/sku-items/:sku_item_id", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage, domain.PermissionTaskCreate), taskH.PatchSKUItemInfo)
		taskGroup.PATCH("/:id/sku-items/:sku_item_id/cost-info", capabilityAccess(taskGroup, http.MethodPatch, "/:id/sku-items/:sku_item_id/cost-info", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage, domain.PermissionTaskCreate), taskH.PatchSKUItemCostInfo)
		taskGroup.POST("/:id/cost-quote/preview", capabilityAccess(taskGroup, http.MethodPost, "/:id/cost-quote/preview", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), taskH.PreviewCostQuote)
		// Business information remains editable, while filing is asynchronous and
		// never acts as a task-completion gate in the v8 workflow.
		taskGroup.PATCH("/:id/business-info", capabilityAccess(taskGroup, http.MethodPatch, "/:id/business-info", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogManage, domain.PermissionTaskCreate), taskH.UpdateBusinessInfo)
		taskGroup.GET("/:id/filing-status", capabilityAccess(taskGroup, http.MethodGet, "/:id/filing-status", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView), taskH.GetFilingStatus)
		taskGroup.POST("/:id/filing/retry", capabilityAccess(taskGroup, http.MethodPost, "/:id/filing/retry", domain.APIReadinessReadyForFrontend, domain.PermissionERPManage), taskH.RetryFiling)
		taskGroup.GET("/:id/detail", capabilityAccess(taskGroup, http.MethodGet, "/:id/detail", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView), taskDetailH.GetByTaskID)
		taskGroup.POST("/:id/references/replace", capabilityAccess(taskGroup, http.MethodPost, "/:id/references/replace", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate, domain.PermissionAssetManage), taskAssetCenterH.ReplaceTaskReference)
		taskGroup.POST("/:id/modules/:module_key/claim", capabilityAccess(taskGroup, http.MethodPost, "/:id/modules/:module_key/claim", domain.APIReadinessReadyForFrontend, domain.PermissionTaskDesignSubmit, domain.PermissionTaskAuditDecision), taskH.ModuleClaim)
		taskGroup.POST("/:id/modules/:module_key/actions/:action", capabilityAccess(taskGroup, http.MethodPost, "/:id/modules/:module_key/actions/:action", domain.APIReadinessReadyForFrontend, domain.PermissionTaskDesignSubmit), taskH.ModuleAction)
		taskGroup.POST("/:id/cancel", capabilityAccess(taskGroup, http.MethodPost, "/:id/cancel", domain.APIReadinessReadyForFrontend, domain.PermissionTaskTerminate), taskH.CancelR3)
		taskGroup.GET("/:id/cost-overrides", capabilityAccess(taskGroup, http.MethodGet, "/:id/cost-overrides", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), taskCostOverrideH.ListByTaskID)
		taskGroup.POST("/batch/assign", capabilityAccess(taskGroup, http.MethodPost, "/batch/assign", domain.APIReadinessReadyForFrontend, domain.PermissionTaskAssign, domain.PermissionTaskReassign), taskAssignmentH.BatchAssign)
		taskGroup.POST("/batch/remind", capabilityAccess(taskGroup, http.MethodPost, "/batch/remind", domain.APIReadinessReadyForFrontend, domain.PermissionTaskAssign, domain.PermissionTaskAuditDecision), taskAssignmentH.BatchRemind)
		taskGroup.POST("/:id/assign", capabilityAccess(taskGroup, http.MethodPost, "/:id/assign", domain.APIReadinessReadyForFrontend, domain.PermissionTaskAssign, domain.PermissionTaskReassign), taskAssignmentH.Assign)
		if taskResourceWorkflowH != nil {
			taskGroup.POST("/:id/submit-design", capabilityAccess(taskGroup, http.MethodPost, "/:id/submit-design", domain.APIReadinessReadyForFrontend, domain.PermissionTaskDesignSubmit), taskResourceWorkflowH.SubmitDesign)
			taskGroup.POST("/:id/audit/decision", capabilityAccess(taskGroup, http.MethodPost, "/:id/audit/decision", domain.APIReadinessReadyForFrontend, domain.PermissionTaskAuditDecision), taskResourceWorkflowH.AuditDecision)
			taskGroup.POST("/:id/reopen", capabilityAccess(taskGroup, http.MethodPost, "/:id/reopen", domain.APIReadinessReadyForFrontend, domain.PermissionTaskReopen), taskResourceWorkflowH.Reopen)
			taskGroup.GET("/:id/resource-bundle", capabilityAccess(taskGroup, http.MethodGet, "/:id/resource-bundle", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView, domain.PermissionAssetView), taskResourceWorkflowH.ResourceBundle)
		}
		if planningSKUH != nil {
			taskGroup.GET("/sku-planning/template.xlsx", capabilityAccess(taskGroup, http.MethodGet, "/sku-planning/template.xlsx", domain.APIReadinessReadyForFrontend, domain.PermissionPlanningSKUCreate), planningSKUH.Template)
			taskGroup.POST("/sku-planning/parse-excel", capabilityAccess(taskGroup, http.MethodPost, "/sku-planning/parse-excel", domain.APIReadinessReadyForFrontend, domain.PermissionPlanningSKUCreate), planningSKUH.ParseExcel)
			taskGroup.GET("/:id/planning-skus", capabilityAccess(taskGroup, http.MethodGet, "/:id/planning-skus", domain.APIReadinessReadyForFrontend, domain.PermissionPlanningSKUView), planningSKUH.GetResult)
			taskGroup.PATCH("/:id/planning-skus/:item_id", capabilityAccess(taskGroup, http.MethodPatch, "/:id/planning-skus/:item_id", domain.APIReadinessReadyForFrontend, domain.PermissionPlanningSKUEdit), planningSKUH.Update)
			taskGroup.GET("/:id/planning-skus/export.xlsx", capabilityAccess(taskGroup, http.MethodGet, "/:id/planning-skus/export.xlsx", domain.APIReadinessReadyForFrontend, domain.PermissionPlanningSKUExport), planningSKUH.ExportTask)
			taskGroup.POST("/:id/planning-skus/erp-retry", capabilityAccess(taskGroup, http.MethodPost, "/:id/planning-skus/erp-retry", domain.APIReadinessReadyForFrontend, domain.PermissionPlanningSKURetry), planningSKUH.ERPRetry)
			taskGroup.POST("/:id/planning-skus/erp-resync", capabilityAccess(taskGroup, http.MethodPost, "/:id/planning-skus/erp-resync", domain.APIReadinessReadyForFrontend, domain.PermissionPlanningSKUSync), planningSKUH.ERPResync)
		}
		taskGroup.GET("/:id/assets", capabilityAccess(taskGroup, http.MethodGet, "/:id/assets", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView, domain.PermissionAssetView), taskAssetCenterH.ListAssets)
		taskGroup.POST("/:id/reference-assets/batch-download", capabilityAccess(taskGroup, http.MethodPost, "/:id/reference-assets/batch-download", domain.APIReadinessReadyForFrontend, domain.PermissionAssetDownload), taskAssetCenterH.BatchDownloadTaskReferenceAssets)

		if auditV7H != nil {
			// V8 audit collaboration is capability governed. Handover is only a
			// current-handler action; management reassignment is a separate flow.
			taskGroup.POST("/:id/audit/handover", capabilityAccess(taskGroup, http.MethodPost, "/:id/audit/handover", domain.APIReadinessReadyForFrontend, domain.PermissionTaskAuditHandover), auditV7H.Handover)
			taskGroup.GET("/:id/audit/handovers", capabilityAccess(taskGroup, http.MethodGet, "/:id/audit/handovers", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView, domain.PermissionTaskAuditHandover), auditV7H.ListHandovers)
			taskGroup.POST("/:id/audit/takeover", capabilityAccess(taskGroup, http.MethodPost, "/:id/audit/takeover", domain.APIReadinessReadyForFrontend, domain.PermissionTaskAuditHandover), auditV7H.Takeover)
			// V8 task event log
			taskGroup.GET("/:id/events", capabilityAccess(taskGroup, http.MethodGet, "/:id/events", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView), auditV7H.ListEvents)
		}
	}
	if planningSKUH != nil {
		v1.POST("/planning-skus/export.xlsx", capabilityAccess(v1, http.MethodPost, "/planning-skus/export.xlsx", domain.APIReadinessReadyForFrontend, domain.PermissionPlanningSKUExport), planningSKUH.ExportSelection)
	}
	if taskResourceWorkflowH != nil {
		resourceGroups := v1.Group("/resource-groups")
		resourceGroups.GET("", capabilityAccess(resourceGroups, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskResourceWorkflowH.ListResourceGroups)
		resourceGroups.GET("/:id", capabilityAccess(resourceGroups, http.MethodGet, "/:id", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskResourceWorkflowH.ResourceGroup)
		resourceGroups.GET("/:id/cost-reconciliation", capabilityAccess(resourceGroups, http.MethodGet, "/:id/cost-reconciliation", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskResourceWorkflowH.ResourceGroupCostReconciliation)
		resourceGroups.GET("/:id/revisions", capabilityAccess(resourceGroups, http.MethodGet, "/:id/revisions", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskResourceWorkflowH.ResourceGroupRevisions)
		resourceGroups.POST("/batch-download", capabilityAccess(resourceGroups, http.MethodPost, "/batch-download", domain.APIReadinessReadyForFrontend, domain.PermissionAssetDownload), taskResourceWorkflowH.BatchDownloadResourceGroups)
	}

	assetGroup := v1.Group("/assets")
	{
		assetGroup.POST("/search/batch", capabilityAccess(assetGroup, http.MethodPost, "/search/batch", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskAssetCenterH.BatchSearchGlobalAssets)
		assetGroup.POST("/batch-download", capabilityAccess(assetGroup, http.MethodPost, "/batch-download", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskAssetCenterH.BatchDownloadGlobalAssets)
		assetGroup.POST("/excel-package/preview", capabilityAccess(assetGroup, http.MethodPost, "/excel-package/preview", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskAssetCenterH.PreviewExcelPackage)
		assetGroup.POST("/excel-package/preview-file", capabilityAccess(assetGroup, http.MethodPost, "/excel-package/preview-file", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskAssetCenterH.PreviewExcelPackageFile)
		assetGroup.POST("/excel-package/jobs", capabilityAccess(assetGroup, http.MethodPost, "/excel-package/jobs", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskAssetCenterH.CreateExcelPackageJob)
		assetGroup.POST("/excel-package/jobs/file", capabilityAccess(assetGroup, http.MethodPost, "/excel-package/jobs/file", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskAssetCenterH.CreateExcelPackageFileJob)
		assetGroup.GET("/excel-package/jobs/:job_id", capabilityAccess(assetGroup, http.MethodGet, "/excel-package/jobs/:job_id", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskAssetCenterH.GetExcelPackageJob)
		assetGroup.GET("/:asset_id", capabilityAccess(assetGroup, http.MethodGet, "/:asset_id", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskAssetCenterH.GetGlobalAsset)
		assetGroup.DELETE("/:asset_id", capabilityAccess(assetGroup, http.MethodDelete, "/:asset_id", domain.APIReadinessReadyForFrontend, domain.PermissionAssetManage, domain.PermissionTaskCreate, domain.PermissionTaskDesignSubmit, domain.PermissionTaskAuditDecision), taskAssetCenterH.DeleteGlobalAsset)
		assetGroup.GET("/:asset_id/download", capabilityAccess(assetGroup, http.MethodGet, "/:asset_id/download", domain.APIReadinessReadyForFrontend, domain.PermissionAssetDownload), taskAssetCenterH.DownloadGlobalAsset)
		assetGroup.GET("/:asset_id/content", withAssetFileTokenFallback(actorResolver), capabilityAccess(assetGroup, http.MethodGet, "/:asset_id/content", domain.APIReadinessReadyForFrontend, domain.PermissionAssetDownload), taskAssetCenterH.StreamGlobalExternalAsset)
		assetGroup.GET("/:asset_id/preview", capabilityAccess(assetGroup, http.MethodGet, "/:asset_id/preview", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView, domain.PermissionTaskAuditDecision), taskAssetCenterH.PreviewAssetResource)
		assetGroup.POST("/upload-sessions", capabilityAccess(assetGroup, http.MethodPost, "/upload-sessions", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate, domain.PermissionTaskDesignSubmit, domain.PermissionTaskAuditDecision, domain.PermissionAssetManage), taskAssetCenterH.CreateAssetUploadSession)
		assetGroup.GET("/upload-sessions/:session_id", capabilityAccess(assetGroup, http.MethodGet, "/upload-sessions/:session_id", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView, domain.PermissionAssetView, domain.PermissionTaskCreate, domain.PermissionTaskDesignSubmit, domain.PermissionTaskAuditDecision, domain.PermissionAssetManage), taskAssetCenterH.GetAssetUploadSession)
		assetGroup.POST("/upload-sessions/:session_id/complete", capabilityAccess(assetGroup, http.MethodPost, "/upload-sessions/:session_id/complete", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate, domain.PermissionTaskDesignSubmit, domain.PermissionTaskAuditDecision, domain.PermissionAssetManage), taskAssetCenterH.CompleteAssetUploadSession)
		assetGroup.POST("/upload-sessions/:session_id/cancel", capabilityAccess(assetGroup, http.MethodPost, "/upload-sessions/:session_id/cancel", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate, domain.PermissionTaskDesignSubmit, domain.PermissionTaskAuditDecision, domain.PermissionAssetManage), taskAssetCenterH.CancelAssetUploadSession)
		// GET /v1/assets/files/* — compatibility proxy fallback for OSS-backed business file bytes.
		// Browser-native loads (<img>) authenticate via login-issued HttpOnly
		// cookie; header-based sessions pass through.
		assetGroup.GET("/files/*path", withAssetFileTokenFallback(actorResolver), capabilityAccess(assetGroup, http.MethodGet, "/files/*path", domain.APIReadinessReadyForFrontend, domain.PermissionAssetDownload), assetFilesH.ServeFile)
	}
	taskAssetGroup := v1.Group("/task-assets")
	{
		// task-assets identifies one immutable task_assets row. It is distinct
		// from /v1/assets, whose numeric id is a design_assets resource id.
		taskAssetGroup.GET("/:task_asset_id/download", capabilityAccess(taskAssetGroup, http.MethodGet, "/:task_asset_id/download", domain.APIReadinessReadyForFrontend, domain.PermissionAssetDownload), taskAssetCenterH.DownloadTaskAssetResource)
		taskAssetGroup.GET("/:task_asset_id/preview", capabilityAccess(taskAssetGroup, http.MethodGet, "/:task_asset_id/preview", domain.APIReadinessReadyForFrontend, domain.PermissionAssetView), taskAssetCenterH.PreviewTaskAssetResource)
	}

	// Current operations dashboard snapshot.
	taskBoardGroup := v1.Group("/task-board")
	{
		taskBoardGroup.GET("/overview", capabilityAccess(taskBoardGroup, http.MethodGet, "/overview", domain.APIReadinessReadyForFrontend, domain.PermissionTaskView), taskBoardH.OperationalOverview)
	}

	integrationGroup := v1.Group("/integration")
	{
		assetSyncGroup := integrationGroup.Group("/asset-sync")
		assetSyncGroup.Use(withAssetSyncTokenAuth())
		assetSyncGroup.GET("/finalized/manifest", integrationCenterH.FinalizedSyncManifest)
		assetSyncGroup.POST("/finalized/download-tickets", integrationCenterH.FinalizedDownloadTickets)
		assetSyncGroup.GET("/external-current/manifest", integrationCenterH.ExternalCurrentSyncManifest)
		assetSyncGroup.GET("/external-current/head", integrationCenterH.ExternalCurrentSyncHead)
		assetSyncGroup.GET("/external-current/changes", integrationCenterH.ExternalCurrentSyncChanges)
		assetSyncGroup.POST("/external-current/download-tickets", integrationCenterH.ExternalCurrentDownloadTickets)

		externalAssetIntegrationGroup := integrationGroup.Group("/external-assets")
		externalAssetIntegrationGroup.Use(withExternalAssetEventTokenAuth())
		externalAssetIntegrationGroup.POST("/events", integrationCenterH.IngestExternalAssetEvents)
	}

	// V7: CodeRule (numbering engine)
	codeRuleGroup := v1.Group("/code-rules")
	{
		codeRuleGroup.GET("", capabilityAccess(codeRuleGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), codeRuleH.List)
		codeRuleGroup.GET("/:id/preview", capabilityAccess(codeRuleGroup, http.MethodGet, "/:id/preview", domain.APIReadinessReadyForFrontend, domain.PermissionCatalogView), codeRuleH.Preview)
	}

	return r
}

type routeAccessRegistrar func(group *gin.RouterGroup, method, path string, readiness domain.APIReadiness, roles ...domain.Role) gin.HandlerFunc

type capabilityRouteAccessRegistrar func(group *gin.RouterGroup, method, path string, readiness domain.APIReadiness, permissions ...domain.PermissionCode) gin.HandlerFunc

func registerOperationalRoutes(r *gin.Engine) {
	healthHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	r.GET("/health", healthHandler)
	r.GET("/healthz", healthHandler)
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
}

func injectTraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := c.GetHeader("X-Trace-ID")
		if tid == "" {
			tid = uuid.New().String()
		}
		c.Set(traceIDKey, tid)
		c.Header("X-Trace-ID", tid)
		c.Request = c.Request.WithContext(domain.ContextWithTraceID(c.Request.Context(), tid))
		c.Next()
	}
}

type serverLogRecorder interface {
	RecordHTTPError(c *gin.Context, status int, path, method, traceID, clientIP string)
}

type workflowTraceRecorder interface {
	RecordTraceEvent(ctx context.Context, event *domain.WorkflowTraceEvent) (*domain.WorkflowTraceEvent, *domain.AppError)
}

func requestLogger(logger *zap.Logger, recorder serverLogRecorder, traceRecorder workflowTraceRecorder) gin.HandlerFunc {
	if logger == nil {
		logger = zap.NewNop()
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		latency := time.Since(start)
		logger.Info("http_request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("trace_id", c.GetString(traceIDKey)),
			zap.String("client_ip", c.ClientIP()),
		)
		if recorder != nil && status >= 500 {
			recorder.RecordHTTPError(c, status, c.Request.URL.Path, c.Request.Method, c.GetString(traceIDKey), c.ClientIP())
		}
		recordHTTPWorkflowTraceEvent(c, traceRecorder, logger, status, start, latency)
	}
}

func recordHTTPWorkflowTraceEvent(c *gin.Context, recorder workflowTraceRecorder, logger *zap.Logger, status int, start time.Time, latency time.Duration) {
	if recorder == nil || shouldSkipWorkflowTracePath(c.Request.URL.Path) {
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	routePath := routePathForLog(c)
	latencyMS := latency.Milliseconds()
	outcome := domain.WorkflowTraceOutcomeSucceeded
	if status >= 500 {
		outcome = domain.WorkflowTraceOutcomeFailed
	}
	httpStatus := status
	event := &domain.WorkflowTraceEvent{
		TraceID:         c.GetString(traceIDKey),
		EventSource:     domain.WorkflowTraceSourceAPI,
		EventType:       domain.WorkflowTraceEventAPIRequest,
		Action:          c.Request.Method + " " + routePath,
		ActorID:         actorIDPtrFromTransportActor(actor),
		ActorUsername:   actor.Username,
		ActorSource:     actor.Source,
		ActorAuthMode:   actor.AuthMode,
		ActorRoles:      actor.Roles,
		ActorDepartment: actor.Department,
		ActorTeam:       actor.Team,
		RouteMethod:     c.Request.Method,
		RoutePath:       routePath,
		RouteFullPath:   c.Request.URL.Path,
		HTTPStatus:      &httpStatus,
		LatencyMS:       &latencyMS,
		ClientIP:        c.ClientIP(),
		UserAgent:       c.GetHeader("User-Agent"),
		TaskID:          taskIDFromRoute(c),
		TaskModuleID:    positiveInt64Param(c, "task_module_id"),
		ModuleKey:       strings.TrimSpace(c.Param("module_key")),
		TaskSKUItemID:   positiveInt64Param(c, "sku_item_id"),
		AssetID:         assetIDFromRoute(c),
		Outcome:         outcome,
		Payload:         httpTracePayload(c),
		OccurredAt:      start.UTC(),
	}
	if strings.Contains(routePath, "/integration/call-logs/:id") {
		event.IntegrationCallLogID = positiveInt64Param(c, "id")
	}
	ctx := context.WithoutCancel(c.Request.Context())
	if _, appErr := recorder.RecordTraceEvent(ctx, event); appErr != nil {
		logger.Warn("workflow_trace_http_record_failed",
			zap.String("code", appErr.Code),
			zap.String("message", appErr.Message),
			zap.String("trace_id", event.TraceID),
			zap.String("route_path", routePath),
		)
	}
}

func shouldSkipWorkflowTracePath(path string) bool {
	switch strings.TrimSpace(path) {
	case "/health", "/healthz", "/ping", "/favicon.ico", "/v1/trace-events", "/v1/search", "/v1/assets/search":
		return true
	default:
		return false
	}
}

func actorIDPtrFromTransportActor(actor domain.RequestActor) *int64 {
	if actor.ID <= 0 {
		return nil
	}
	return &actor.ID
}

func taskIDFromRoute(c *gin.Context) *int64 {
	routePath := routePathForLog(c)
	if strings.Contains(routePath, "/tasks/:id") || strings.Contains(routePath, "/tasks/{id}") {
		return positiveInt64Param(c, "id")
	}
	return positiveInt64Param(c, "task_id")
}

func assetIDFromRoute(c *gin.Context) *int64 {
	routePath := routePathForLog(c)
	if strings.Contains(routePath, "/assets/:asset_id") {
		return positiveInt64Param(c, "asset_id")
	}
	return positiveInt64Param(c, "asset_id")
}

func positiveInt64Param(c *gin.Context, name string) *int64 {
	raw := strings.TrimSpace(c.Param(name))
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return nil
	}
	return &value
}

func httpTracePayload(c *gin.Context) json.RawMessage {
	payload := map[string]string{}
	if rawQuery := strings.TrimSpace(c.Request.URL.RawQuery); rawQuery != "" {
		payload["query"] = rawQuery
	}
	if referer := strings.TrimSpace(c.GetHeader("Referer")); referer != "" {
		payload["referer"] = referer
	}
	if len(payload) == 0 {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return raw
}
