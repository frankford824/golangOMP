package transport

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/transport/handler"
	transportws "workflow/transport/ws"
)

func registerV1IdentityRoutes(
	r *gin.Engine,
	v1 *gin.RouterGroup,
	routeAccessCatalog *RouteAccessCatalog,
	permissionLogger PermissionLogWriter,
	authH *handler.AuthHandler,
	taskDraftH *handler.TaskDraftHandler,
	designSourceH *handler.DesignSourceHandler,
	searchH *handler.SearchHandler,
	reportL1H *handler.ReportL1Handler,
	notificationH *handler.NotificationHandler,
	wsH *transportws.Handler,
) {
	authGroup := v1.Group("/auth")
	{
		authGroup.GET("/register-options", authH.RegisterOptions)
		authGroup.POST("/register", authH.Register)
		authGroup.POST("/login", authH.Login)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, joinRoutePath(authGroup.BasePath(), "/me"), domain.APIReadinessReadyForFrontend))
		authGroup.GET("/me", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.Me)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPost, joinRoutePath(authGroup.BasePath(), "/asset-cookie"), domain.APIReadinessReadyForFrontend))
		authGroup.POST("/asset-cookie", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.RefreshAssetCookie)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPost, joinRoutePath(authGroup.BasePath(), "/logout"), domain.APIReadinessReadyForFrontend))
		authGroup.POST("/logout", authH.Logout)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPut, joinRoutePath(authGroup.BasePath(), "/password"), domain.APIReadinessReadyForFrontend))
		authGroup.PUT("/password", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.ChangePassword)
	}

	routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, "/v1/me", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
	v1.GET("/me", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.GetMe)
	routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPatch, "/v1/me", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
	v1.PATCH("/me", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.PatchMe)
	routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPost, "/v1/me/avatar", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
	v1.POST("/me/avatar", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.UploadMyAvatar)
	routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodDelete, "/v1/me/avatar", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
	v1.DELETE("/me/avatar", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), authH.DeleteMyAvatar)
	v1.GET("/me/avatar-files/:filename", authH.ServeMyAvatar)
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
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, "/v1/reports/l1/kpi-events", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin))
		v1.GET("/reports/l1/kpi-events", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), reportL1H.KPIEvents)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPost, "/v1/reports/l1/kpi-ai-analysis", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin))
		v1.POST("/reports/l1/kpi-ai-analysis", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), reportL1H.KPIAIAnalysis)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPost, "/v1/reports/business-trends/pilot-analysis", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin))
		v1.POST("/reports/business-trends/pilot-analysis", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), reportL1H.BusinessTrendPilotAnalysis)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPost, "/v1/reports/business-trends/deep-analysis-jobs", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin))
		v1.POST("/reports/business-trends/deep-analysis-jobs", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), reportL1H.StartBusinessTrendDeepAnalysis)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, "/v1/reports/business-trends/deep-analysis-jobs/:job_id", domain.APIReadinessReadyForFrontend, domain.RoleSuperAdmin))
		v1.GET("/reports/business-trends/deep-analysis-jobs/:job_id", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), reportL1H.GetBusinessTrendDeepAnalysisJob)
	}
	if notificationH != nil {
		v1.GET("/me/notifications", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.MyList)
		v1.POST("/me/notifications/:id/read", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.MarkRead)
		v1.POST("/me/notifications/read-all", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.MarkAllRead)
		v1.GET("/me/notifications/unread-count", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.UnreadCount)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, "/v1/me/notifications/web-push/config", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
		v1.GET("/me/notifications/web-push/config", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.WebPushConfig)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPost, "/v1/me/notifications/web-push/subscriptions", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
		v1.POST("/me/notifications/web-push/subscriptions", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.RegisterWebPushSubscription)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodDelete, "/v1/me/notifications/web-push/subscriptions/current", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
		v1.DELETE("/me/notifications/web-push/subscriptions/current", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.DeleteCurrentWebPushSubscription)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPost, "/v1/me/notifications/web-push/test", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
		v1.POST("/me/notifications/web-push/test", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.SendWebPushTest)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodGet, "/v1/me/notifications/preferences", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
		v1.GET("/me/notifications/preferences", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.GetPreferences)
		routeAccessCatalog.AddRule(domain.NewRouteAccessRule(http.MethodPatch, "/v1/me/notifications/preferences", domain.APIReadinessReadyForFrontend, v1R1AllLoggedInRoles()...))
		v1.PATCH("/me/notifications/preferences", withAuthenticated(domain.APIReadinessReadyForFrontend, permissionLogger), notificationH.PatchPreferences)
	}
	if wsH != nil {
		r.GET("/ws/v1", wsH.Upgrade)
	}
}
