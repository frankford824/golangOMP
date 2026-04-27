package transport

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"workflow/domain"
	taskbatchexcel "workflow/service/task_batch_excel"
	"workflow/transport/handler"
	transportws "workflow/transport/ws"
)

const traceIDKey = "trace_id"

// NewRouter builds the gin router with all routes registered per spec §7.2.
func NewRouter(
	skuH *handler.SKUHandler,
	auditH *handler.AuditHandler,
	agentH *handler.AgentHandler,
	incidentH *handler.IncidentHandler,
	policyH *handler.PolicyHandler,
	authH *handler.AuthHandler,
	userAdminH *handler.UserAdminHandler,
	erpBridgeH *handler.ERPBridgeHandler,
	productH *handler.ProductHandler,
	categoryH *handler.CategoryHandler,
	categoryMappingH *handler.CategoryERPMappingHandler,
	costRuleH *handler.CostRuleHandler,
	erpSyncH *handler.ERPSyncHandler,
	taskH *handler.TaskHandler,
	taskAssignmentH *handler.TaskAssignmentHandler,
	taskAssetH *handler.TaskAssetHandler,
	taskAssetCenterH *handler.TaskAssetCenterHandler,
	taskCreateReferenceUploadH *handler.TaskCreateReferenceUploadHandler,
	assetUploadH *handler.AssetUploadHandler,
	assetFilesH *handler.AssetFilesHandler,
	designSubmissionH *handler.DesignSubmissionHandler,
	taskDetailH *handler.TaskDetailHandler,
	taskCostOverrideH *handler.TaskCostOverrideHandler,
	taskBoardH *handler.TaskBoardHandler,
	workbenchH *handler.WorkbenchHandler,
	exportCenterH *handler.ExportCenterHandler,
	integrationCenterH *handler.IntegrationCenterHandler,
	codeRuleH *handler.CodeRuleHandler,
	ruleTemplateH *handler.RuleTemplateHandler,
	auditV7H *handler.AuditV7Handler,
	auditLogH *handler.AuditLogHandler,
	outsourceH *handler.OutsourceHandler,
	warehouseH *handler.WarehouseHandler,
	jstUserAdminH *handler.JSTUserAdminHandler,
	serverLogH *handler.ServerLogHandler,
	orgMoveH *handler.OrgMoveRequestHandler,
	taskDraftH *handler.TaskDraftHandler,
	notificationH *handler.NotificationHandler,
	erpProductH *handler.ERPProductHandler,
	designSourceH *handler.DesignSourceHandler,
	searchH *handler.SearchHandler,
	reportL1H *handler.ReportL1Handler,
	wsH *transportws.Handler,
	routeAccessCatalog *RouteAccessCatalog,
	actorResolver RequestActorResolver,
	permissionLogger PermissionLogWriter,
	logger *zap.Logger,
) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(injectTraceID())
	r.Use(injectRequestActor(actorResolver))
	r.Use(requestLogger(logger, serverLogH))
	registerOperationalRoutes(r)

	v1 := r.Group("/v1")
	ws := r.Group("/ws")
	if routeAccessCatalog == nil {
		routeAccessCatalog = NewRouteAccessCatalog()
	}
	routeAccessCatalog.Reset()

	access := func(group *gin.RouterGroup, method, path string, readiness domain.APIReadiness, roles ...domain.Role) gin.HandlerFunc {
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(method, joinRoutePath(group.BasePath(), path), readiness, roles...))
		return withAccessMetaAndLogger(permissionLogger, readiness, roles...)
	}
	taskBatchTemplateSvc, taskBatchParseSvc := taskbatchexcel.New()
	taskBatchExcelH := handler.NewTaskBatchExcelHandler(taskBatchTemplateSvc, taskBatchParseSvc)
	legacyRoleConvergedAccess := func(group *gin.RouterGroup, method, path string, readiness domain.APIReadiness, roles ...domain.Role) []gin.HandlerFunc {
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(method, joinRoutePath(group.BasePath(), path), readiness, roles...))
		return []gin.HandlerFunc{
			withLegacyCompatibilityOnlyRoleRejection(permissionLogger, readiness, roles, domain.RoleAdmin, domain.RoleOrgAdmin, domain.RoleRoleAdmin),
			withAccessMetaAndLogger(permissionLogger, readiness, roles...),
		}
	}

	authGroup := v1.Group("/auth")
	{
		authGroup.GET("/register-options", authH.RegisterOptions)
		authGroup.POST("/register", authH.Register)
		authGroup.POST("/login", authH.Login)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, joinRoutePath(authGroup.BasePath(), "/me"), domain.APIReadinessReadyForFrontend))
		authGroup.GET("/me", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.Me)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPut, joinRoutePath(authGroup.BasePath(), "/password"), domain.APIReadinessReadyForFrontend))
		authGroup.PUT("/password", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.ChangePassword)
	}

	routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, "/v1/me", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
	v1.GET("/me", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.GetMe)
	routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPatch, "/v1/me", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
	v1.PATCH("/me", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.PatchMe)
	routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPost, "/v1/me/change-password", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
	v1.POST("/me/change-password", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.ChangeMyPassword)
	routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, "/v1/me/org", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
	v1.GET("/me/org", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.GetMyOrg)

	if taskDraftH != nil {
		v1.POST("/task-drafts", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), taskDraftH.CreateOrUpdate)
		v1.GET("/me/task-drafts", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), taskDraftH.MyList)
		v1.GET("/task-drafts/:draft_id", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), taskDraftH.Get)
		v1.DELETE("/task-drafts/:draft_id", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), taskDraftH.Delete)
	}
	if designSourceH != nil {
		v1.GET("/design-sources/search", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), designSourceH.Search)
	}
	if searchH != nil {
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, "/v1/search", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
		v1.GET("/search", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), searchH.Search)
	}
	if reportL1H != nil {
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, "/v1/reports/l1/cards", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin))
		v1.GET("/reports/l1/cards", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), reportL1H.Cards)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, "/v1/reports/l1/throughput", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin))
		v1.GET("/reports/l1/throughput", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), reportL1H.Throughput)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, "/v1/reports/l1/module-dwell", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin))
		v1.GET("/reports/l1/module-dwell", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), reportL1H.ModuleDwell)
	}
	if notificationH != nil {
		v1.GET("/me/notifications", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.MyList)
		v1.POST("/me/notifications/:id/read", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.MarkRead)
		v1.POST("/me/notifications/read-all", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.MarkAllRead)
		v1.GET("/me/notifications/unread-count", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.UnreadCount)
	}
	if wsH != nil {
		r.GET("/ws/v1", wsH.Upgrade)
	}

	v1.GET("/roles", access(v1, http.MethodGet, "/roles", domain.APIReadinessReadyForFrontend, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleDeptAdmin, domain.RoleOrgAdmin, domain.RoleRoleAdmin), userAdminH.ListRoles)
	v1.GET("/access-rules", access(v1, http.MethodGet, "/access-rules", domain.APIReadinessReadyForFrontend), userAdminH.ListRouteAccessRules)
	v1.GET("/users", access(v1, http.MethodGet, "/users", domain.APIReadinessReadyForFrontend, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleOrgAdmin, domain.RoleRoleAdmin), userAdminH.ListUsers)
	v1.POST("/users", append(legacyRoleConvergedAccess(v1, http.MethodPost, "/users", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleSuperAdmin, domain.RoleDeptAdmin), userAdminH.CreateUser)...)
	v1.GET("/users/designers", access(v1, http.MethodGet, "/users/designers", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleAdmin, domain.RoleHRAdmin, domain.RoleSuperAdmin), userAdminH.ListDesigners)
	v1.GET("/users/:id", access(v1, http.MethodGet, "/users/:id", domain.APIReadinessReadyForFrontend, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleOrgAdmin, domain.RoleRoleAdmin), userAdminH.GetUser)
	v1.PATCH("/users/:id", append(legacyRoleConvergedAccess(v1, http.MethodPatch, "/users/:id", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleSuperAdmin, domain.RoleDeptAdmin), userAdminH.PatchUser)...)
	v1.DELETE("/users/:id", access(v1, http.MethodDelete, "/users/:id", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin, domain.RoleAdmin), userAdminH.Delete)
	v1.POST("/users/:id/activate", access(v1, http.MethodPost, "/users/:id/activate", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleAdmin), userAdminH.Activate)
	v1.POST("/users/:id/deactivate", access(v1, http.MethodPost, "/users/:id/deactivate", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleAdmin), userAdminH.Deactivate)
	v1.PUT("/users/:id/password", append(legacyRoleConvergedAccess(v1, http.MethodPut, "/users/:id/password", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleSuperAdmin, domain.RoleDeptAdmin), userAdminH.ResetPassword)...)
	v1.POST("/users/:id/roles", append(legacyRoleConvergedAccess(v1, http.MethodPost, "/users/:id/roles", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleSuperAdmin), userAdminH.AddRoles)...)
	v1.PUT("/users/:id/roles", append(legacyRoleConvergedAccess(v1, http.MethodPut, "/users/:id/roles", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleSuperAdmin), userAdminH.SetRoles)...)
	v1.DELETE("/users/:id/roles/:role", append(legacyRoleConvergedAccess(v1, http.MethodDelete, "/users/:id/roles/:role", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleSuperAdmin), userAdminH.RemoveRole)...)
	v1.GET("/permission-logs", access(v1, http.MethodGet, "/permission-logs", domain.APIReadinessReadyForFrontend, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleOrgAdmin, domain.RoleRoleAdmin), userAdminH.ListPermissionLogs)
	v1.GET("/operation-logs", access(v1, http.MethodGet, "/operation-logs", domain.APIReadinessReadyForFrontend, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin), userAdminH.ListOperationLogs)
	v1.GET("/org/options", access(v1, http.MethodGet, "/org/options", domain.APIReadinessReadyForFrontend, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleDeptAdmin, domain.RoleOrgAdmin, domain.RoleRoleAdmin), userAdminH.GetOrgOptions)
	v1.POST("/org/departments", append(legacyRoleConvergedAccess(v1, http.MethodPost, "/org/departments", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleSuperAdmin), userAdminH.CreateDepartment)...)
	v1.PUT("/org/departments/:id", append(legacyRoleConvergedAccess(v1, http.MethodPut, "/org/departments/:id", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleSuperAdmin), userAdminH.UpdateDepartment)...)
	v1.POST("/org/teams", append(legacyRoleConvergedAccess(v1, http.MethodPost, "/org/teams", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleSuperAdmin), userAdminH.CreateTeam)...)
	v1.PUT("/org/teams/:id", append(legacyRoleConvergedAccess(v1, http.MethodPut, "/org/teams/:id", domain.APIReadinessReadyForFrontend, domain.RoleHRAdmin, domain.RoleSuperAdmin), userAdminH.UpdateTeam)...)
	if orgMoveH != nil {
		v1.POST("/departments/:id/org-move-requests", access(v1, http.MethodPost, "/departments/:id/org-move-requests", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleDeptAdmin, domain.RoleAdmin), orgMoveH.Create)
		v1.GET("/org-move-requests", access(v1, http.MethodGet, "/org-move-requests", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleDeptAdmin, domain.RoleAdmin), orgMoveH.List)
		v1.POST("/org-move-requests/:id/approve", access(v1, http.MethodPost, "/org-move-requests/:id/approve", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin, domain.RoleAdmin), orgMoveH.Approve)
		v1.POST("/org-move-requests/:id/reject", access(v1, http.MethodPost, "/org-move-requests/:id/reject", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin, domain.RoleAdmin), orgMoveH.Reject)
	}
	v1.GET("/audit-logs", access(v1, http.MethodGet, "/audit-logs", domain.APIReadinessReadyForFrontend, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleAuditA, domain.RoleAuditB, domain.RoleCustomizationReviewer), auditLogH.List)
	v1.GET("/server-logs", access(v1, http.MethodGet, "/server-logs", domain.APIReadinessReadyForFrontend, domain.RoleAdmin), serverLogH.List)
	v1.POST("/server-logs/clean", access(v1, http.MethodPost, "/server-logs/clean", domain.APIReadinessReadyForFrontend, domain.RoleAdmin), serverLogH.Clean)

	// SKU endpoints
	skuGroup := v1.Group("/sku")
	{
		skuGroup.GET("/list", skuH.List)
		skuGroup.POST("", skuH.Create)
		skuGroup.GET("/:id", skuH.GetByID)
		skuGroup.GET("/:id/sync_status", skuH.SyncStatus) // sequence-gap recovery
		skuGroup.POST("/preview_code", skuH.PreviewCode)
	}

	// Audit (idempotent via action_id)
	v1.POST("/audit", withDeprecatedRoute("/v1/tasks/{id}/audit/*", "candidate_for_v1_0_removal"), auditH.Submit)

	// NAS Agent endpoints
	agentGroup := v1.Group("/agent")
	{
		agentGroup.POST("/sync", agentH.Sync)
		agentGroup.POST("/pull_job", agentH.PullJob)
		agentGroup.POST("/heartbeat", agentH.Heartbeat)
		agentGroup.POST("/ack_job", agentH.AckJob)
	}

	// Incident endpoints
	incidentGroup := v1.Group("/incidents")
	{
		incidentGroup.GET("", incidentH.List)
		incidentGroup.POST("/:id/assign", incidentH.Assign)
		incidentGroup.POST("/:id/resolve", incidentH.Resolve)
	}

	// Policy endpoints (Admin-protected on Update)
	policyGroup := v1.Group("/policies")
	{
		policyGroup.GET("", policyH.List)
		policyGroup.PUT("/:id", policyH.Update)
	}

	// V7: Product (ERP master data)
	productGroup := v1.Group("/products")
	{
		productGroup.GET("/search", access(productGroup, http.MethodGet, "/search", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse), withCompatibilityRoute("/v1/erp/products", "candidate_for_v1_0_removal"), productH.Search)
		productGroup.GET("/sync/status", access(productGroup, http.MethodGet, "/sync/status", domain.APIReadinessInternalPlaceholder, domain.RoleERP, domain.RoleAdmin), erpSyncH.Status)
		productGroup.POST("/sync/run", access(productGroup, http.MethodPost, "/sync/run", domain.APIReadinessInternalPlaceholder, domain.RoleERP, domain.RoleAdmin), erpSyncH.Run)
		productGroup.GET("/:id", access(productGroup, http.MethodGet, "/:id", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse), withCompatibilityRoute("/v1/erp/products/{id}", "candidate_for_v1_0_removal"), productH.GetByID)
	}

	erpGroup := v1.Group("/erp")
	{
		erpGroup.GET("/products", access(erpGroup, http.MethodGet, "/products", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleERP, domain.RoleAdmin), erpBridgeH.SearchProducts)
		erpGroup.GET("/products/*id", access(erpGroup, http.MethodGet, "/products/{id}", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleERP, domain.RoleAdmin), func(c *gin.Context) {
			if erpProductH != nil && strings.Trim(strings.TrimSpace(c.Param("id")), "/") == "by-code" {
				erpProductH.ByCode(c)
				return
			}
			erpBridgeH.GetProductByID(c)
		})
		erpGroup.GET("/categories", access(erpGroup, http.MethodGet, "/categories", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleERP, domain.RoleAdmin), erpBridgeH.ListCategories)
		erpGroup.GET("/warehouses", access(erpGroup, http.MethodGet, "/warehouses", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleERP, domain.RoleAdmin), erpBridgeH.ListWarehouses)
		erpGroup.GET("/sync-logs", access(erpGroup, http.MethodGet, "/sync-logs", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleERP, domain.RoleAdmin), erpBridgeH.ListSyncLogs)
		erpGroup.GET("/sync-logs/*id", access(erpGroup, http.MethodGet, "/sync-logs/{id}", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleERP, domain.RoleAdmin), erpBridgeH.GetSyncLogByID)
		erpGroup.GET("/users", access(erpGroup, http.MethodGet, "/users", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin, domain.RoleERP), erpBridgeH.ListJSTUsers)
		erpGroup.POST("/products/upsert", access(erpGroup, http.MethodPost, "/products/upsert", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleERP, domain.RoleAdmin), erpBridgeH.UpsertProduct)
		erpGroup.POST("/products/style/update", access(erpGroup, http.MethodPost, "/products/style/update", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleERP, domain.RoleAdmin), erpBridgeH.UpdateItemStyle)
		erpGroup.POST("/products/shelve/batch", access(erpGroup, http.MethodPost, "/products/shelve/batch", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleERP, domain.RoleAdmin), erpBridgeH.ShelveProductsBatch)
		erpGroup.POST("/products/unshelve/batch", access(erpGroup, http.MethodPost, "/products/unshelve/batch", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleERP, domain.RoleAdmin), erpBridgeH.UnshelveProductsBatch)
		erpGroup.POST("/inventory/virtual-qty", access(erpGroup, http.MethodPost, "/inventory/virtual-qty", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleERP, domain.RoleAdmin), erpBridgeH.UpdateVirtualInventory)
	}

	categoryGroup := v1.Group("/categories")
	{
		categoryGroup.GET("", access(categoryGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), categoryH.List)
		categoryGroup.GET("/search", access(categoryGroup, http.MethodGet, "/search", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), categoryH.Search)
		categoryGroup.GET("/:id", access(categoryGroup, http.MethodGet, "/:id", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), categoryH.GetByID)
		categoryGroup.POST("", access(categoryGroup, http.MethodPost, "", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), categoryH.Create)
		categoryGroup.PATCH("/:id", access(categoryGroup, http.MethodPatch, "/:id", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), categoryH.Patch)
	}

	categoryMappingGroup := v1.Group("/category-mappings")
	{
		categoryMappingGroup.GET("", access(categoryMappingGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), categoryMappingH.List)
		categoryMappingGroup.GET("/search", access(categoryMappingGroup, http.MethodGet, "/search", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), categoryMappingH.Search)
		categoryMappingGroup.GET("/:id", access(categoryMappingGroup, http.MethodGet, "/:id", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), categoryMappingH.GetByID)
		categoryMappingGroup.POST("", access(categoryMappingGroup, http.MethodPost, "", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), categoryMappingH.Create)
		categoryMappingGroup.PATCH("/:id", access(categoryMappingGroup, http.MethodPatch, "/:id", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), categoryMappingH.Patch)
	}

	costRuleGroup := v1.Group("/cost-rules")
	{
		costRuleGroup.GET("", access(costRuleGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), costRuleH.List)
		costRuleGroup.GET("/:id", access(costRuleGroup, http.MethodGet, "/:id", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), costRuleH.GetByID)
		costRuleGroup.GET("/:id/history", access(costRuleGroup, http.MethodGet, "/:id/history", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), costRuleH.GetHistory)
		costRuleGroup.POST("", access(costRuleGroup, http.MethodPost, "", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), costRuleH.Create)
		costRuleGroup.PATCH("/:id", access(costRuleGroup, http.MethodPatch, "/:id", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), costRuleH.Patch)
		costRuleGroup.POST("/preview", access(costRuleGroup, http.MethodPost, "/preview", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), costRuleH.Preview)
	}

	taskCreateAssetCenterGroup := v1.Group("/task-create/asset-center")
	{
		taskCreateAssetCenterGroup.POST("/upload-sessions", access(taskCreateAssetCenterGroup, http.MethodPost, "/upload-sessions", domain.APIReadinessReadyForFrontend, domain.RoleOps), withCompatibilityRoute("/v1/tasks/reference-upload", "remove_after_frontend_migration"), taskCreateReferenceUploadH.CreateUploadSession)
		taskCreateAssetCenterGroup.GET("/upload-sessions/:session_id", access(taskCreateAssetCenterGroup, http.MethodGet, "/upload-sessions/:session_id", domain.APIReadinessReadyForFrontend, domain.RoleOps), withCompatibilityRoute("/v1/tasks/reference-upload", "remove_after_frontend_migration"), taskCreateReferenceUploadH.GetUploadSession)
		taskCreateAssetCenterGroup.POST("/upload-sessions/:session_id/complete", access(taskCreateAssetCenterGroup, http.MethodPost, "/upload-sessions/:session_id/complete", domain.APIReadinessReadyForFrontend, domain.RoleOps), withCompatibilityRoute("/v1/tasks/reference-upload", "remove_after_frontend_migration"), taskCreateReferenceUploadH.CompleteUploadSession)
		taskCreateAssetCenterGroup.POST("/upload-sessions/:session_id/abort", access(taskCreateAssetCenterGroup, http.MethodPost, "/upload-sessions/:session_id/abort", domain.APIReadinessReadyForFrontend, domain.RoleOps), withCompatibilityRoute("/v1/tasks/reference-upload", "remove_after_frontend_migration"), taskCreateReferenceUploadH.AbortUploadSession)
	}

	// V7: Task (business aggregate root)
	taskGroup := v1.Group("/tasks")
	{
		taskGroup.POST("/reference-upload", access(taskGroup, http.MethodPost, "/reference-upload", domain.APIReadinessReadyForFrontend, domain.RoleOps), taskCreateReferenceUploadH.UploadFile)
		taskGroup.POST("/prepare-product-codes", access(taskGroup, http.MethodPost, "/prepare-product-codes", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleAdmin), taskH.PrepareProductCodes)
		taskGroup.GET("/batch-create/template.xlsx", access(taskGroup, http.MethodGet, "/batch-create/template.xlsx", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...), taskBatchExcelH.DownloadTemplate)
		taskGroup.POST("/batch-create/parse-excel", access(taskGroup, http.MethodPost, "/batch-create/parse-excel", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...), taskBatchExcelH.ParseUpload)
		taskGroup.POST("", access(taskGroup, http.MethodPost, "", domain.APIReadinessReadyForFrontend, domain.RoleOps), taskH.Create)
		taskGroup.GET("", access(taskGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleOrgAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.List)
		taskGroup.GET("/pool", access(taskGroup, http.MethodGet, "/pool", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...), taskH.Pool)
		taskGroup.GET("/:id", access(taskGroup, http.MethodGet, "/:id", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleOrgAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.GetByID)
		taskGroup.GET("/:id/product-info", access(taskGroup, http.MethodGet, "/:id/product-info", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleAdmin), taskH.GetProductInfo)
		taskGroup.PATCH("/:id/product-info", access(taskGroup, http.MethodPatch, "/:id/product-info", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.PatchProductInfo)
		taskGroup.GET("/:id/cost-info", access(taskGroup, http.MethodGet, "/:id/cost-info", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleAdmin), taskH.GetCostInfo)
		taskGroup.PATCH("/:id/cost-info", access(taskGroup, http.MethodPatch, "/:id/cost-info", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.PatchCostInfo)
		taskGroup.POST("/:id/cost-quote/preview", access(taskGroup, http.MethodPost, "/:id/cost-quote/preview", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), taskH.PreviewCostQuote)
		// business-info remains a compatibility filing entry, but Step 87 filing policy
		// also auto-triggers from create/audit/procurement/warehouse checkpoints.
		taskGroup.PATCH("/:id/business-info", access(taskGroup, http.MethodPatch, "/:id/business-info", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.UpdateBusinessInfo)
		taskGroup.GET("/:id/filing-status", access(taskGroup, http.MethodGet, "/:id/filing-status", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleAuditA, domain.RoleAuditB), taskH.GetFilingStatus)
		taskGroup.POST("/:id/filing/retry", access(taskGroup, http.MethodPost, "/:id/filing/retry", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.RetryFiling)
		taskGroup.PATCH("/:id/procurement", access(taskGroup, http.MethodPatch, "/:id/procurement", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.UpdateProcurement)
		taskGroup.POST("/:id/procurement/advance", access(taskGroup, http.MethodPost, "/:id/procurement/advance", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.AdvanceProcurement)
		taskGroup.GET("/:id/detail", access(taskGroup, http.MethodGet, "/:id/detail", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleOrgAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskDetailH.GetByTaskID)
		taskGroup.POST("/:id/modules/:module_key/claim", access(taskGroup, http.MethodPost, "/:id/modules/:module_key/claim", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...), taskH.ModuleClaim)
		taskGroup.POST("/:id/modules/:module_key/actions/:action", access(taskGroup, http.MethodPost, "/:id/modules/:module_key/actions/:action", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...), taskH.ModuleAction)
		taskGroup.POST("/:id/modules/:module_key/reassign", access(taskGroup, http.MethodPost, "/:id/modules/:module_key/reassign", domain.APIReadinessReadyForFrontend, v1R1ManagementRoles()...), taskH.ModuleReassign)
		taskGroup.POST("/:id/modules/:module_key/pool-reassign", access(taskGroup, http.MethodPost, "/:id/modules/:module_key/pool-reassign", domain.APIReadinessReadyForFrontend, v1R1DepartmentAdminRoles()...), taskH.ModulePoolReassign)
		taskGroup.POST("/:id/cancel", access(taskGroup, http.MethodPost, "/:id/cancel", domain.APIReadinessReadyForFrontend, v1R1TaskCancelRoles()...), taskH.CancelR3)
		taskGroup.GET("/:id/cost-overrides", access(taskGroup, http.MethodGet, "/:id/cost-overrides", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), taskCostOverrideH.ListByTaskID)
		taskGroup.POST("/:id/cost-overrides/:event_id/review", access(taskGroup, http.MethodPost, "/:id/cost-overrides/:event_id/review", domain.APIReadinessInternalPlaceholder, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin), taskCostOverrideH.UpsertReview)
		taskGroup.POST("/:id/cost-overrides/:event_id/finance-mark", access(taskGroup, http.MethodPost, "/:id/cost-overrides/:event_id/finance-mark", domain.APIReadinessInternalPlaceholder, domain.RoleERP, domain.RoleAdmin), taskCostOverrideH.UpsertFinanceFlag)
		taskGroup.POST("/:id/close", access(taskGroup, http.MethodPost, "/:id/close", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.Close)
		taskGroup.POST("/batch/assign", access(taskGroup, http.MethodPost, "/batch/assign", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskAssignmentH.BatchAssign)
		taskGroup.POST("/batch/remind", access(taskGroup, http.MethodPost, "/batch/remind", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), taskAssignmentH.BatchRemind)
		taskGroup.POST("/:id/assign", access(taskGroup, http.MethodPost, "/:id/assign", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskAssignmentH.Assign)
		taskGroup.POST("/:id/submit-design", access(taskGroup, http.MethodPost, "/:id/submit-design", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), designSubmissionH.Submit)
		taskGroup.GET("/:id/assets", access(taskGroup, http.MethodGet, "/:id/assets", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), taskAssetCenterH.ListAssets)
		taskGroup.GET("/:id/assets/timeline", access(taskGroup, http.MethodGet, "/:id/assets/timeline", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), withCompatibilityRoute("/v1/tasks/{id}/asset-center/assets", "candidate_for_v1_0_removal"), taskAssetH.List)
		taskGroup.GET("/:id/assets/:asset_id/versions", access(taskGroup, http.MethodGet, "/:id/assets/:asset_id/versions", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), withCompatibilityRoute("/v1/tasks/{id}/asset-center/assets/{asset_id}/versions", "remove_after_frontend_migration"), taskAssetCenterH.ListVersions)
		taskGroup.GET("/:id/assets/:asset_id/download", access(taskGroup, http.MethodGet, "/:id/assets/:asset_id/download", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), withCompatibilityRoute("/v1/tasks/{id}/asset-center/assets/{asset_id}/download", "remove_after_frontend_migration"), taskAssetCenterH.DownloadAsset)
		taskGroup.GET("/:id/assets/:asset_id/versions/:version_id/download", access(taskGroup, http.MethodGet, "/:id/assets/:asset_id/versions/:version_id/download", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), withCompatibilityRoute("/v1/tasks/{id}/asset-center/assets/{asset_id}/versions/{version_id}/download", "remove_after_frontend_migration"), taskAssetCenterH.DownloadVersion)
		taskGroup.POST("/:id/assets/upload-sessions", access(taskGroup, http.MethodPost, "/:id/assets/upload-sessions", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), withCompatibilityRoute("/v1/assets/upload-sessions", "remove_after_frontend_migration"), taskAssetCenterH.CreateUploadSession)
		taskGroup.GET("/:id/assets/upload-sessions/:session_id", access(taskGroup, http.MethodGet, "/:id/assets/upload-sessions/:session_id", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), withCompatibilityRoute("/v1/assets/upload-sessions/{session_id}", "remove_after_frontend_migration"), taskAssetCenterH.GetUploadSession)
		taskGroup.POST("/:id/assets/upload-sessions/:session_id/complete", access(taskGroup, http.MethodPost, "/:id/assets/upload-sessions/:session_id/complete", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), withCompatibilityRoute("/v1/assets/upload-sessions/{session_id}/complete", "remove_after_frontend_migration"), taskAssetCenterH.CompleteUploadSession)
		taskGroup.POST("/:id/assets/upload-sessions/:session_id/abort", access(taskGroup, http.MethodPost, "/:id/assets/upload-sessions/:session_id/abort", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), withCompatibilityRoute("/v1/assets/upload-sessions/{session_id}/cancel", "remove_after_frontend_migration"), taskAssetCenterH.AbortUploadSession)
		taskGroup.POST("/:id/assets/upload", access(taskGroup, http.MethodPost, "/:id/assets/upload", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), withDeprecatedRoute("/v1/assets/upload-sessions", "remove_after_frontend_migration"), taskAssetCenterH.LegacyTaskAssetsUpload)
		taskGroup.POST("/:id/assets/mock-upload", access(taskGroup, http.MethodPost, "/:id/assets/mock-upload", domain.APIReadinessMockPlaceholderOnly, domain.RoleDesigner, domain.RoleOps), taskAssetH.MockUpload)
		taskGroup.GET("/:id/asset-center/assets", access(taskGroup, http.MethodGet, "/:id/asset-center/assets", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), taskAssetCenterH.ListAssets)
		taskGroup.GET("/:id/asset-center/assets/:asset_id/versions", access(taskGroup, http.MethodGet, "/:id/asset-center/assets/:asset_id/versions", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), taskAssetCenterH.ListVersions)
		taskGroup.GET("/:id/asset-center/assets/:asset_id/download", access(taskGroup, http.MethodGet, "/:id/asset-center/assets/:asset_id/download", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), taskAssetCenterH.DownloadAsset)
		taskGroup.GET("/:id/asset-center/assets/:asset_id/versions/:version_id/download", access(taskGroup, http.MethodGet, "/:id/asset-center/assets/:asset_id/versions/:version_id/download", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), taskAssetCenterH.DownloadVersion)
		taskGroup.POST("/:id/asset-center/upload-sessions", access(taskGroup, http.MethodPost, "/:id/asset-center/upload-sessions", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), withCompatibilityRoute("/v1/assets/upload-sessions", "remove_after_frontend_migration"), taskAssetCenterH.CreateUploadSession)
		taskGroup.POST("/:id/asset-center/upload-sessions/small", access(taskGroup, http.MethodPost, "/:id/asset-center/upload-sessions/small", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), withCompatibilityRoute("/v1/assets/upload-sessions", "remove_after_frontend_migration"), taskAssetCenterH.CreateSmallUploadSession)
		taskGroup.POST("/:id/asset-center/upload-sessions/multipart", access(taskGroup, http.MethodPost, "/:id/asset-center/upload-sessions/multipart", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), withCompatibilityRoute("/v1/assets/upload-sessions", "remove_after_frontend_migration"), taskAssetCenterH.CreateMultipartUploadSession)
		taskGroup.GET("/:id/asset-center/upload-sessions/:session_id", access(taskGroup, http.MethodGet, "/:id/asset-center/upload-sessions/:session_id", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), withCompatibilityRoute("/v1/assets/upload-sessions/{session_id}", "remove_after_frontend_migration"), taskAssetCenterH.GetUploadSession)
		taskGroup.POST("/:id/asset-center/upload-sessions/:session_id/complete", access(taskGroup, http.MethodPost, "/:id/asset-center/upload-sessions/:session_id/complete", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), withCompatibilityRoute("/v1/assets/upload-sessions/{session_id}/complete", "remove_after_frontend_migration"), taskAssetCenterH.CompleteUploadSession)
		taskGroup.POST("/:id/asset-center/upload-sessions/:session_id/cancel", access(taskGroup, http.MethodPost, "/:id/asset-center/upload-sessions/:session_id/cancel", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), withCompatibilityRoute("/v1/assets/upload-sessions/{session_id}/cancel", "remove_after_frontend_migration"), taskAssetCenterH.CancelUploadSession)
		taskGroup.POST("/:id/asset-center/upload-sessions/:session_id/abort", access(taskGroup, http.MethodPost, "/:id/asset-center/upload-sessions/:session_id/abort", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), withCompatibilityRoute("/v1/assets/upload-sessions/{session_id}/cancel", "candidate_for_v1_0_removal"), taskAssetCenterH.AbortUploadSession)

		// V7 audit actions (task-centric)
		taskGroup.POST("/:id/audit/claim", access(taskGroup, http.MethodPost, "/:id/audit/claim", domain.APIReadinessReadyForFrontend, domain.RoleAuditA, domain.RoleAuditB, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), auditV7H.Claim)
		taskGroup.POST("/:id/audit/approve", access(taskGroup, http.MethodPost, "/:id/audit/approve", domain.APIReadinessReadyForFrontend, domain.RoleAuditA, domain.RoleAuditB, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), auditV7H.Approve)
		taskGroup.POST("/:id/audit/reject", access(taskGroup, http.MethodPost, "/:id/audit/reject", domain.APIReadinessReadyForFrontend, domain.RoleAuditA, domain.RoleAuditB, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), auditV7H.Reject)
		taskGroup.POST("/:id/audit/transfer", access(taskGroup, http.MethodPost, "/:id/audit/transfer", domain.APIReadinessReadyForFrontend, domain.RoleAuditA, domain.RoleAuditB, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), auditV7H.Transfer)
		taskGroup.POST("/:id/audit/handover", access(taskGroup, http.MethodPost, "/:id/audit/handover", domain.APIReadinessReadyForFrontend, domain.RoleAuditA, domain.RoleAuditB, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), auditV7H.Handover)
		taskGroup.GET("/:id/audit/handovers", access(taskGroup, http.MethodGet, "/:id/audit/handovers", domain.APIReadinessReadyForFrontend, domain.RoleAuditA, domain.RoleAuditB, domain.RoleAdmin), auditV7H.ListHandovers)
		taskGroup.POST("/:id/audit/takeover", access(taskGroup, http.MethodPost, "/:id/audit/takeover", domain.APIReadinessReadyForFrontend, domain.RoleAuditA, domain.RoleAuditB, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), auditV7H.Takeover)

		// V7 outsource creation under task
		taskGroup.POST("/:id/outsource", access(taskGroup, http.MethodPost, "/:id/outsource", domain.APIReadinessReadyForFrontend, domain.RoleOutsource, domain.RoleOps, domain.RoleAdmin), withCompatibilityRoute("/v1/tasks", "candidate_for_v1_0_removal"), outsourceH.Create)

		// V7 warehouse actions under task
		taskGroup.POST("/:id/warehouse/prepare", access(taskGroup, http.MethodPost, "/:id/warehouse/prepare", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleWarehouse, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.PrepareWarehouse)
		taskGroup.POST("/:id/warehouse/receive", access(taskGroup, http.MethodPost, "/:id/warehouse/receive", domain.APIReadinessReadyForFrontend, domain.RoleWarehouse, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), warehouseH.Receive)
		taskGroup.POST("/:id/warehouse/reject", access(taskGroup, http.MethodPost, "/:id/warehouse/reject", domain.APIReadinessReadyForFrontend, domain.RoleWarehouse, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), warehouseH.Reject)
		taskGroup.POST("/:id/warehouse/complete", access(taskGroup, http.MethodPost, "/:id/warehouse/complete", domain.APIReadinessReadyForFrontend, domain.RoleWarehouse, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), warehouseH.Complete)
		taskGroup.POST("/:id/customization/review", access(taskGroup, http.MethodPost, "/:id/customization/review", domain.APIReadinessReadyForFrontend, domain.RoleCustomizationReviewer, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.SubmitCustomizationReview)

		// V7 task event log
		taskGroup.GET("/:id/events", access(taskGroup, http.MethodGet, "/:id/events", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), auditV7H.ListEvents)
	}

	customizationGroup := v1.Group("/customization-jobs")
	{
		customizationGroup.GET("", access(customizationGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.RoleCustomizationReviewer, domain.RoleCustomizationOperator, domain.RoleOps, domain.RoleDesigner, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.ListCustomizationJobs)
		customizationGroup.GET("/:id", access(customizationGroup, http.MethodGet, "/:id", domain.APIReadinessReadyForFrontend, domain.RoleCustomizationReviewer, domain.RoleCustomizationOperator, domain.RoleOps, domain.RoleDesigner, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.GetCustomizationJob)
		customizationGroup.POST("/:id/effect-preview", access(customizationGroup, http.MethodPost, "/:id/effect-preview", domain.APIReadinessReadyForFrontend, domain.RoleCustomizationOperator, domain.RoleDesigner, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.SubmitCustomizationEffectPreview)
		customizationGroup.POST("/:id/effect-review", access(customizationGroup, http.MethodPost, "/:id/effect-review", domain.APIReadinessReadyForFrontend, domain.RoleCustomizationReviewer, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.ReviewCustomizationEffect)
		customizationGroup.POST("/:id/production-transfer", access(customizationGroup, http.MethodPost, "/:id/production-transfer", domain.APIReadinessReadyForFrontend, domain.RoleCustomizationOperator, domain.RoleDesigner, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskH.TransferCustomizationProduction)
	}

	assetGroup := v1.Group("/assets")
	{
		assetGroup.GET("", access(assetGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), taskAssetCenterH.ListAssetResources)
		assetGroup.GET("/search", access(assetGroup, http.MethodGet, "/search", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...), taskAssetCenterH.SearchGlobalAssets)
		assetGroup.GET("/:asset_id", access(assetGroup, http.MethodGet, "/:asset_id", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...), taskAssetCenterH.GetGlobalAsset)
		assetGroup.DELETE("/:asset_id", access(assetGroup, http.MethodDelete, "/:asset_id", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...), taskAssetCenterH.DeleteGlobalAsset)
		assetGroup.GET("/:asset_id/download", access(assetGroup, http.MethodGet, "/:asset_id/download", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...), taskAssetCenterH.DownloadGlobalAsset)
		assetGroup.GET("/:asset_id/versions/:version_id/download", access(assetGroup, http.MethodGet, "/:asset_id/versions/:version_id/download", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...), taskAssetCenterH.DownloadGlobalAssetVersion)
		assetGroup.POST("/:asset_id/archive", access(assetGroup, http.MethodPost, "/:asset_id/archive", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...), taskAssetCenterH.ArchiveGlobalAsset)
		assetGroup.POST("/:asset_id/restore", access(assetGroup, http.MethodPost, "/:asset_id/restore", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...), taskAssetCenterH.RestoreGlobalAsset)
		assetGroup.GET("/:asset_id/preview", access(assetGroup, http.MethodGet, "/:asset_id/preview", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), taskAssetCenterH.PreviewAssetResource)
		assetGroup.POST("/upload-sessions", access(assetGroup, http.MethodPost, "/upload-sessions", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskAssetCenterH.CreateAssetUploadSession)
		assetGroup.GET("/upload-sessions/:session_id", access(assetGroup, http.MethodGet, "/upload-sessions/:session_id", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), taskAssetCenterH.GetAssetUploadSession)
		assetGroup.POST("/upload-sessions/:session_id/complete", access(assetGroup, http.MethodPost, "/upload-sessions/:session_id/complete", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskAssetCenterH.CompleteAssetUploadSession)
		assetGroup.POST("/upload-sessions/:session_id/cancel", access(assetGroup, http.MethodPost, "/upload-sessions/:session_id/cancel", domain.APIReadinessReadyForFrontend, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleOps, domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead, domain.RoleDesignDirector), taskAssetCenterH.CancelAssetUploadSession)
		// GET /v1/assets/files/* — compatibility proxy fallback for OSS-backed business file bytes
		assetGroup.GET("/files/*path", assetFilesH.ServeFile)
		assetGroup.GET("/upload-requests", access(assetGroup, http.MethodGet, "/upload-requests", domain.APIReadinessInternalPlaceholder, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleAdmin), assetUploadH.ListUploadRequests)
		assetGroup.POST("/upload-requests", access(assetGroup, http.MethodPost, "/upload-requests", domain.APIReadinessInternalPlaceholder, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleAdmin), assetUploadH.CreateUploadRequest)
		assetGroup.GET("/upload-requests/:id", access(assetGroup, http.MethodGet, "/upload-requests/:id", domain.APIReadinessInternalPlaceholder, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleAdmin), assetUploadH.GetUploadRequest)
		assetGroup.POST("/upload-requests/:id/advance", access(assetGroup, http.MethodPost, "/upload-requests/:id/advance", domain.APIReadinessInternalPlaceholder, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleOutsource, domain.RoleAdmin), assetUploadH.AdvanceUploadRequest)
	}

	// V7: Outsource orders list (cross-task)
	v1.GET("/outsource-orders", access(v1, http.MethodGet, "/outsource-orders", domain.APIReadinessReadyForFrontend, domain.RoleOutsource, domain.RoleOps, domain.RoleAdmin), withCompatibilityRoute("/v1/customization-jobs", "candidate_for_v1_0_removal"), outsourceH.List)

	// V7: Warehouse receipts list (cross-task)
	v1.GET("/warehouse/receipts", access(v1, http.MethodGet, "/warehouse/receipts", domain.APIReadinessReadyForFrontend, domain.RoleWarehouse, domain.RoleOps, domain.RoleAdmin), warehouseH.List)

	// V7: Task board / inbox aggregation
	taskBoardGroup := v1.Group("/task-board")
	{
		taskBoardGroup.GET("/summary", access(taskBoardGroup, http.MethodGet, "/summary", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), taskBoardH.Summary)
		taskBoardGroup.GET("/queues", access(taskBoardGroup, http.MethodGet, "/queues", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), taskBoardH.Queues)
	}

	workbenchGroup := v1.Group("/workbench")
	{
		workbenchGroup.GET("/preferences", withUserScopedActor(domain.APIReadinessReadyForFrontend, permissionLogger, false), access(workbenchGroup, http.MethodGet, "/preferences", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), workbenchH.GetPreferences)
		workbenchGroup.PATCH("/preferences", withUserScopedActor(domain.APIReadinessReadyForFrontend, permissionLogger, false), access(workbenchGroup, http.MethodPatch, "/preferences", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleCustomizationOperator, domain.RoleCustomizationReviewer, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), workbenchH.PatchPreferences)
	}

	exportTemplateGroup := v1.Group("/export-templates")
	{
		exportTemplateGroup.GET("", access(exportTemplateGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), exportCenterH.ListTemplates)
	}

	exportJobGroup := v1.Group("/export-jobs")
	{
		exportJobGroup.POST("", access(exportJobGroup, http.MethodPost, "", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), exportCenterH.CreateJob)
		exportJobGroup.GET("", access(exportJobGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), exportCenterH.ListJobs)
		exportJobGroup.GET("/:id", access(exportJobGroup, http.MethodGet, "/:id", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), exportCenterH.GetJob)
		exportJobGroup.GET("/:id/dispatches", access(exportJobGroup, http.MethodGet, "/:id/dispatches", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin), exportCenterH.ListJobDispatches)
		exportJobGroup.POST("/:id/dispatches", access(exportJobGroup, http.MethodPost, "/:id/dispatches", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin), exportCenterH.CreateJobDispatch)
		exportJobGroup.POST("/:id/dispatches/:dispatch_id/advance", access(exportJobGroup, http.MethodPost, "/:id/dispatches/:dispatch_id/advance", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin), exportCenterH.AdvanceJobDispatch)
		exportJobGroup.GET("/:id/attempts", access(exportJobGroup, http.MethodGet, "/:id/attempts", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin), exportCenterH.ListJobAttempts)
		exportJobGroup.GET("/:id/events", access(exportJobGroup, http.MethodGet, "/:id/events", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), exportCenterH.ListJobEvents)
		exportJobGroup.POST("/:id/claim-download", access(exportJobGroup, http.MethodPost, "/:id/claim-download", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), exportCenterH.ClaimDownload)
		exportJobGroup.GET("/:id/download", access(exportJobGroup, http.MethodGet, "/:id/download", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), exportCenterH.ReadDownload)
		exportJobGroup.POST("/:id/refresh-download", access(exportJobGroup, http.MethodPost, "/:id/refresh-download", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleDesigner, domain.RoleAuditA, domain.RoleAuditB, domain.RoleWarehouse, domain.RoleAdmin), exportCenterH.RefreshDownload)
		exportJobGroup.POST("/:id/start", access(exportJobGroup, http.MethodPost, "/:id/start", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin), exportCenterH.StartJob)
		exportJobGroup.POST("/:id/advance", access(exportJobGroup, http.MethodPost, "/:id/advance", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin), exportCenterH.AdvanceJob)
	}

	integrationGroup := v1.Group("/integration")
	{
		integrationGroup.GET("/connectors", access(integrationGroup, http.MethodGet, "/connectors", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin, domain.RoleERP), integrationCenterH.ListConnectors)
		integrationGroup.POST("/call-logs", access(integrationGroup, http.MethodPost, "/call-logs", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin, domain.RoleERP), integrationCenterH.CreateCallLog)
		integrationGroup.GET("/call-logs", access(integrationGroup, http.MethodGet, "/call-logs", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin, domain.RoleERP), integrationCenterH.ListCallLogs)
		integrationGroup.GET("/call-logs/:id", access(integrationGroup, http.MethodGet, "/call-logs/:id", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin, domain.RoleERP), integrationCenterH.GetCallLog)
		integrationGroup.GET("/call-logs/:id/executions", access(integrationGroup, http.MethodGet, "/call-logs/:id/executions", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin, domain.RoleERP), integrationCenterH.ListExecutions)
		integrationGroup.POST("/call-logs/:id/executions", access(integrationGroup, http.MethodPost, "/call-logs/:id/executions", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin, domain.RoleERP), integrationCenterH.CreateExecution)
		integrationGroup.POST("/call-logs/:id/retry", access(integrationGroup, http.MethodPost, "/call-logs/:id/retry", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin, domain.RoleERP), integrationCenterH.RetryCallLog)
		integrationGroup.POST("/call-logs/:id/replay", access(integrationGroup, http.MethodPost, "/call-logs/:id/replay", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin, domain.RoleERP), integrationCenterH.ReplayCallLog)
		integrationGroup.POST("/call-logs/:id/executions/:execution_id/advance", access(integrationGroup, http.MethodPost, "/call-logs/:id/executions/:execution_id/advance", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin, domain.RoleERP), integrationCenterH.AdvanceExecution)
		integrationGroup.POST("/call-logs/:id/advance", access(integrationGroup, http.MethodPost, "/call-logs/:id/advance", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin, domain.RoleERP), integrationCenterH.AdvanceCallLog)
	}

	// V7: JST user sync pre-wiring (Admin only; does NOT change auth/permission logic)
	adminGroup := v1.Group("/admin")
	{
		adminGroup.GET("/jst-users", access(adminGroup, http.MethodGet, "/jst-users", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin), jstUserAdminH.ListJSTUsers)
		adminGroup.POST("/jst-users/import-preview", access(adminGroup, http.MethodPost, "/jst-users/import-preview", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin), jstUserAdminH.ImportPreview)
		adminGroup.POST("/jst-users/import", access(adminGroup, http.MethodPost, "/jst-users/import", domain.APIReadinessInternalPlaceholder, domain.RoleAdmin), jstUserAdminH.Import)
	}

	// V7: CodeRule (numbering engine)
	codeRuleGroup := v1.Group("/code-rules")
	{
		codeRuleGroup.GET("", access(codeRuleGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleAdmin), codeRuleH.List)
		codeRuleGroup.GET("/:id/preview", access(codeRuleGroup, http.MethodGet, "/:id/preview", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleAdmin), codeRuleH.Preview)
		codeRuleGroup.POST("/generate-sku", access(codeRuleGroup, http.MethodPost, "/generate-sku", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleAdmin), codeRuleH.GenerateSKU)
	}

	// V0.5: Rule templates (cost-pricing, product-code, short-name)
	ruleTemplateGroup := v1.Group("/rule-templates")
	{
		ruleTemplateGroup.GET("", access(ruleTemplateGroup, http.MethodGet, "", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleAdmin), ruleTemplateH.List)
		ruleTemplateGroup.GET("/:type", access(ruleTemplateGroup, http.MethodGet, "/:type", domain.APIReadinessReadyForFrontend, domain.RoleOps, domain.RoleAdmin), ruleTemplateH.GetByType)
		ruleTemplateGroup.PUT("/:type", access(ruleTemplateGroup, http.MethodPut, "/:type", domain.APIReadinessReadyForFrontend, domain.RoleAdmin), ruleTemplateH.Put)
	}

	registerV1R1ReservedRoutes(r, v1, ws, access, false)

	return r
}

type routeAccessRegistrar func(group *gin.RouterGroup, method, path string, readiness domain.APIReadiness, roles ...domain.Role) gin.HandlerFunc

type v1R1RouteSpec struct {
	GroupBase         string
	Method            string
	RelativePath      string
	OwnerRound        string
	RequiredRoles     []domain.Role
	OverlapsLiveRoute bool
	SamplePath        string
}

func registerV1R1ReservedRoutes(
	r *gin.Engine,
	v1 *gin.RouterGroup,
	ws *gin.RouterGroup,
	access routeAccessRegistrar,
	includeLiveRouteOverlaps bool,
) {
	for _, spec := range v1R1ContractRouteSpecs() {
		if spec.OwnerRound == "R4-SA-A" || spec.OwnerRound == "R4-SA-B" || spec.OwnerRound == "R4-SA-C" {
			continue
		}
		if spec.OverlapsLiveRoute && !includeLiveRouteOverlaps {
			continue
		}

		var group *gin.RouterGroup
		switch spec.GroupBase {
		case "/v1":
			group = v1
		case "/ws":
			group = ws
		default:
			continue
		}

		handlers := []gin.HandlerFunc{
			access(group, spec.Method, spec.RelativePath, domain.APIReadinessReadyForFrontend, spec.RequiredRoles...),
			v1R1ReservedHandler(spec.OwnerRound),
		}

		switch spec.Method {
		case http.MethodGet:
			group.GET(spec.RelativePath, handlers...)
		case http.MethodPost:
			group.POST(spec.RelativePath, handlers...)
		case http.MethodPatch:
			group.PATCH(spec.RelativePath, handlers...)
		case http.MethodDelete:
			group.DELETE(spec.RelativePath, handlers...)
		default:
			panic("unsupported v1 R1 contract route method: " + spec.Method)
		}
	}

	_ = r
}

func v1R1ReservedHandler(ownerRound string) gin.HandlerFunc {
	ownerRound = strings.TrimSpace(ownerRound)
	return func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{
			"error": gin.H{
				"code":    "not_implemented",
				"message": "reserved for " + ownerRound,
			},
		})
	}
}

func v1R1ContractRouteSpecs() []v1R1RouteSpec {
	return []v1R1RouteSpec{
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/tasks", OwnerRound: "R3", RequiredRoles: v1R1AllLoggedInRoles(), OverlapsLiveRoute: true, SamplePath: "/v1/tasks"},
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/tasks/:id/detail", OwnerRound: "R3", RequiredRoles: v1R1AllLoggedInRoles(), OverlapsLiveRoute: true, SamplePath: "/v1/tasks/101/detail"},
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/tasks/pool", OwnerRound: "R3", RequiredRoles: v1R1AllLoggedInRoles(), OverlapsLiveRoute: true, SamplePath: "/v1/tasks/pool"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/tasks/:id/modules/:module_key/claim", OwnerRound: "R3", RequiredRoles: v1R1AllLoggedInRoles(), OverlapsLiveRoute: true, SamplePath: "/v1/tasks/101/modules/design/claim"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/tasks/:id/modules/:module_key/actions/:action", OwnerRound: "R3", RequiredRoles: v1R1AllLoggedInRoles(), OverlapsLiveRoute: true, SamplePath: "/v1/tasks/101/modules/design/actions/submit"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/tasks/:id/modules/:module_key/reassign", OwnerRound: "R3", RequiredRoles: v1R1ManagementRoles(), OverlapsLiveRoute: true, SamplePath: "/v1/tasks/101/modules/design/reassign"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/tasks/:id/modules/:module_key/pool-reassign", OwnerRound: "R3", RequiredRoles: v1R1DepartmentAdminRoles(), OverlapsLiveRoute: true, SamplePath: "/v1/tasks/101/modules/design/pool-reassign"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/tasks/:id/cancel", OwnerRound: "R3", RequiredRoles: v1R1TaskCancelRoles(), OverlapsLiveRoute: true, SamplePath: "/v1/tasks/101/cancel"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/tasks", OwnerRound: "R3", RequiredRoles: v1R1TaskCreateRoles(), OverlapsLiveRoute: true, SamplePath: "/v1/tasks"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/task-drafts", OwnerRound: "R4-SA-C", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/task-drafts"},
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/me/task-drafts", OwnerRound: "R4-SA-C", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/me/task-drafts"},
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/task-drafts/:draft_id", OwnerRound: "R4-SA-C", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/task-drafts/501"},
		{GroupBase: "/v1", Method: http.MethodDelete, RelativePath: "/task-drafts/:draft_id", OwnerRound: "R4-SA-C", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/task-drafts/501"},
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/erp/products/by-code", OwnerRound: "R4-SA-C", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/erp/products/by-code?code=SKU-001"},
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/design-sources/search", OwnerRound: "R4-SA-C", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/design-sources/search?keyword=poster"},
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/me/notifications", OwnerRound: "R4-SA-C", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/me/notifications"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/me/notifications/:id/read", OwnerRound: "R4-SA-C", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/me/notifications/7/read"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/me/notifications/read-all", OwnerRound: "R4-SA-C", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/me/notifications/read-all"},
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/me/notifications/unread-count", OwnerRound: "R4-SA-C", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/me/notifications/unread-count"},
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/me", OwnerRound: "R4-SA-B", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/me"},
		{GroupBase: "/v1", Method: http.MethodPatch, RelativePath: "/me", OwnerRound: "R4-SA-B", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/me"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/me/change-password", OwnerRound: "R4-SA-B", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/me/change-password"},
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/me/org", OwnerRound: "R4-SA-B", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/v1/me/org"},
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/users", OwnerRound: "R4-SA-B", RequiredRoles: v1R1ManagementRoles(), OverlapsLiveRoute: true, SamplePath: "/v1/users"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/users", OwnerRound: "R4-SA-B", RequiredRoles: v1R1UserWriteRoles(), OverlapsLiveRoute: true, SamplePath: "/v1/users"},
		{GroupBase: "/v1", Method: http.MethodPatch, RelativePath: "/users/:id", OwnerRound: "R4-SA-B", RequiredRoles: v1R1UserWriteRoles(), OverlapsLiveRoute: true, SamplePath: "/v1/users/88"},
		{GroupBase: "/v1", Method: http.MethodDelete, RelativePath: "/users/:id", OwnerRound: "R4-SA-B", RequiredRoles: v1R1SuperAdminRoles(), SamplePath: "/v1/users/88"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/users/:id/activate", OwnerRound: "R4-SA-B", RequiredRoles: v1R1UserActivationRoles(), SamplePath: "/v1/users/88/activate"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/users/:id/deactivate", OwnerRound: "R4-SA-B", RequiredRoles: v1R1UserActivationRoles(), SamplePath: "/v1/users/88/deactivate"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/departments/:id/org-move-requests", OwnerRound: "R4-SA-B", RequiredRoles: v1R1DepartmentAdminRoles(), SamplePath: "/v1/departments/9/org-move-requests"},
		{GroupBase: "/v1", Method: http.MethodGet, RelativePath: "/org-move-requests", OwnerRound: "R4-SA-B", RequiredRoles: v1R1OrgMoveReviewRoles(), SamplePath: "/v1/org-move-requests"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/org-move-requests/:id/approve", OwnerRound: "R4-SA-B", RequiredRoles: v1R1SuperAdminRoles(), SamplePath: "/v1/org-move-requests/77/approve"},
		{GroupBase: "/v1", Method: http.MethodPost, RelativePath: "/org-move-requests/:id/reject", OwnerRound: "R4-SA-B", RequiredRoles: v1R1SuperAdminRoles(), SamplePath: "/v1/org-move-requests/77/reject"},
		{GroupBase: "/ws", Method: http.MethodGet, RelativePath: "/v1", OwnerRound: "R4-SA-C", RequiredRoles: v1R1AllLoggedInRoles(), SamplePath: "/ws/v1"},
	}
}

func v1R1AllLoggedInRoles() []domain.Role {
	return []domain.Role{
		domain.RoleMember,
		domain.RoleOps,
		domain.RoleDesigner,
		domain.RoleCustomizationOperator,
		domain.RoleCustomizationReviewer,
		domain.RoleAuditA,
		domain.RoleAuditB,
		domain.RoleWarehouse,
		domain.RoleDeptAdmin,
		domain.RoleTeamLead,
		domain.RoleHRAdmin,
		domain.RoleSuperAdmin,
		domain.RoleAdmin,
		domain.RoleOrgAdmin,
		domain.RoleRoleAdmin,
		domain.RoleDesignDirector,
		domain.RoleDesignReviewer,
		domain.RoleOutsource,
		domain.RoleERP,
	}
}

func v1R1TaskCreateRoles() []domain.Role {
	return []domain.Role{domain.RoleOps, domain.RoleDeptAdmin, domain.RoleSuperAdmin, domain.RoleAdmin}
}

func v1R1TaskCancelRoles() []domain.Role {
	return []domain.Role{domain.RoleOps, domain.RoleDeptAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleAdmin}
}

func v1R1ManagementRoles() []domain.Role {
	return []domain.Role{
		domain.RoleTeamLead,
		domain.RoleDeptAdmin,
		domain.RoleHRAdmin,
		domain.RoleSuperAdmin,
		domain.RoleAdmin,
		domain.RoleOrgAdmin,
		domain.RoleRoleAdmin,
	}
}

func v1R1DepartmentAdminRoles() []domain.Role {
	return []domain.Role{domain.RoleDeptAdmin, domain.RoleHRAdmin, domain.RoleSuperAdmin, domain.RoleAdmin}
}

func v1R1UserWriteRoles() []domain.Role {
	return []domain.Role{domain.RoleDeptAdmin, domain.RoleHRAdmin, domain.RoleSuperAdmin, domain.RoleAdmin}
}

func v1R1UserActivationRoles() []domain.Role {
	return []domain.Role{domain.RoleTeamLead, domain.RoleDeptAdmin, domain.RoleHRAdmin, domain.RoleSuperAdmin, domain.RoleAdmin}
}

func v1R1OrgMoveReviewRoles() []domain.Role {
	return []domain.Role{domain.RoleHRAdmin, domain.RoleSuperAdmin, domain.RoleAdmin}
}

func v1R1SuperAdminRoles() []domain.Role {
	return []domain.Role{domain.RoleSuperAdmin, domain.RoleAdmin}
}

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

func requestLogger(logger *zap.Logger, recorder serverLogRecorder) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		logger.Info("http_request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", status),
			zap.Duration("latency", time.Since(start)),
			zap.String("trace_id", c.GetString(traceIDKey)),
			zap.String("client_ip", c.ClientIP()),
		)
		if recorder != nil && status >= 500 {
			recorder.RecordHTTPError(c, status, c.Request.URL.Path, c.Request.Method, c.GetString(traceIDKey), c.ClientIP())
		}
	}
}

func withCompatibilityRoute(successorPath, targetRemovalPhase string) gin.HandlerFunc {
	return withDeprecationRoute("compatibility", successorPath, false, targetRemovalPhase)
}

func withDeprecatedRoute(successorPath, targetRemovalPhase string) gin.HandlerFunc {
	return withDeprecationRoute("deprecated", successorPath, false, targetRemovalPhase)
}

func withDeprecationRoute(apiStatus, successorPath string, newUsageAllowed bool, targetRemovalPhase string) gin.HandlerFunc {
	apiStatus = strings.TrimSpace(apiStatus)
	if apiStatus == "" {
		apiStatus = "deprecated"
	}
	successorPath = strings.TrimSpace(successorPath)
	targetRemovalPhase = strings.TrimSpace(targetRemovalPhase)
	return func(c *gin.Context) {
		c.Header("Deprecation", "true")
		c.Header("X-Workflow-API-Status", apiStatus)
		c.Header("X-Workflow-New-Usage-Allowed", strconv.FormatBool(newUsageAllowed))
		if targetRemovalPhase != "" {
			c.Header("X-Workflow-Target-Removal-Phase", targetRemovalPhase)
		}
		warningLabel := "Deprecated API"
		if apiStatus == "compatibility" {
			warningLabel = "Compatibility-only API"
		}
		if successorPath != "" {
			c.Header("X-Workflow-Successor-Path", successorPath)
			c.Header("Link", "<"+successorPath+">; rel=\"successor-version\"")
			c.Header("Warning", "299 - \""+warningLabel+"; use "+successorPath+"\"")
			c.Next()
			return
		}
		c.Header("Warning", "299 - \""+warningLabel+"\"")
		c.Next()
	}
}
