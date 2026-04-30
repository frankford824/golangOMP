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
}
