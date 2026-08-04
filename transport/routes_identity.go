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
	capabilityAccess capabilityRouteAccessRegistrar,
	authH *handler.AuthHandler,
	taskDraftH *handler.TaskDraftHandler,
	designSourceH *handler.DesignSourceHandler,
	searchH *handler.SearchHandler,
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

	v1.GET("/me", capabilityAccess(v1, http.MethodGet, "/me", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), authH.GetMe)
	v1.PATCH("/me", capabilityAccess(v1, http.MethodPatch, "/me", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), authH.PatchMe)
	v1.POST("/me/avatar", capabilityAccess(v1, http.MethodPost, "/me/avatar", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), authH.UploadMyAvatar)
	v1.DELETE("/me/avatar", capabilityAccess(v1, http.MethodDelete, "/me/avatar", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), authH.DeleteMyAvatar)
	v1.GET("/me/avatar-files/:filename", authH.ServeMyAvatar)
	v1.POST("/me/change-password", capabilityAccess(v1, http.MethodPost, "/me/change-password", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), authH.ChangeMyPassword)
	v1.GET("/me/org", capabilityAccess(v1, http.MethodGet, "/me/org", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), authH.GetMyOrg)

	if taskDraftH != nil {
		v1.POST("/task-drafts", capabilityAccess(v1, http.MethodPost, "/task-drafts", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskDraftH.CreateOrUpdate)
		v1.GET("/me/task-drafts", capabilityAccess(v1, http.MethodGet, "/me/task-drafts", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskDraftH.MyList)
		v1.GET("/task-drafts/:draft_id", capabilityAccess(v1, http.MethodGet, "/task-drafts/:draft_id", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskDraftH.Get)
		v1.DELETE("/task-drafts/:draft_id", capabilityAccess(v1, http.MethodDelete, "/task-drafts/:draft_id", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), taskDraftH.Delete)
	}
	if designSourceH != nil {
		v1.GET("/design-sources/search", capabilityAccess(v1, http.MethodGet, "/design-sources/search", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate), designSourceH.Search)
	}
	if searchH != nil {
		v1.GET("/search", capabilityAccess(v1, http.MethodGet, "/search", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), searchH.Search)
	}
	if notificationH != nil {
		v1.GET("/me/notifications", capabilityAccess(v1, http.MethodGet, "/me/notifications", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), notificationH.MyList)
		v1.POST("/me/notifications/:id/read", capabilityAccess(v1, http.MethodPost, "/me/notifications/:id/read", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), notificationH.MarkRead)
		v1.POST("/me/notifications/read-all", capabilityAccess(v1, http.MethodPost, "/me/notifications/read-all", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), notificationH.MarkAllRead)
		v1.GET("/me/notifications/unread-count", capabilityAccess(v1, http.MethodGet, "/me/notifications/unread-count", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), notificationH.UnreadCount)
		v1.GET("/me/notifications/web-push/config", capabilityAccess(v1, http.MethodGet, "/me/notifications/web-push/config", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), notificationH.WebPushConfig)
		v1.POST("/me/notifications/web-push/subscriptions", capabilityAccess(v1, http.MethodPost, "/me/notifications/web-push/subscriptions", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), notificationH.RegisterWebPushSubscription)
		v1.DELETE("/me/notifications/web-push/subscriptions/current", capabilityAccess(v1, http.MethodDelete, "/me/notifications/web-push/subscriptions/current", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), notificationH.DeleteCurrentWebPushSubscription)
		v1.POST("/me/notifications/web-push/test", capabilityAccess(v1, http.MethodPost, "/me/notifications/web-push/test", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), notificationH.SendWebPushTest)
		v1.GET("/me/notifications/preferences", capabilityAccess(v1, http.MethodGet, "/me/notifications/preferences", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), notificationH.GetPreferences)
		v1.PATCH("/me/notifications/preferences", capabilityAccess(v1, http.MethodPatch, "/me/notifications/preferences", domain.APIReadinessReadyForFrontend, domain.PermissionAccountUse), notificationH.PatchPreferences)
	}
	if wsH != nil {
		r.GET("/ws/v1", wsH.Upgrade)
	}
}
