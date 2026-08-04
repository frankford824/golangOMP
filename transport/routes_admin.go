package transport

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/transport/handler"
)

func registerV1AdminRoutes(
	v1 *gin.RouterGroup,
	capabilityAccess capabilityRouteAccessRegistrar,
	userAdminH *handler.UserAdminHandler,
	notificationH *handler.NotificationHandler,
) {
	v1.GET("/users", capabilityAccess(v1, http.MethodGet, "/users", domain.APIReadinessReadyForFrontend, domain.PermissionAccessView, domain.PermissionAccessManage), userAdminH.ListUsers)
	v1.POST("/users", capabilityAccess(v1, http.MethodPost, "/users", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.CreateUser)
	v1.GET("/users/designers", capabilityAccess(v1, http.MethodGet, "/users/designers", domain.APIReadinessReadyForFrontend, domain.PermissionTaskCreate, domain.PermissionTaskAssign, domain.PermissionTaskReassign, domain.PermissionTaskAuditHandover), userAdminH.ListDesigners)
	v1.GET("/users/:id", capabilityAccess(v1, http.MethodGet, "/users/:id", domain.APIReadinessReadyForFrontend, domain.PermissionAccessView, domain.PermissionAccessManage), userAdminH.GetUser)
	v1.PATCH("/users/:id", capabilityAccess(v1, http.MethodPatch, "/users/:id", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.PatchUser)
	v1.POST("/users/:id/activate", capabilityAccess(v1, http.MethodPost, "/users/:id/activate", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.Activate)
	v1.POST("/users/:id/deactivate", capabilityAccess(v1, http.MethodPost, "/users/:id/deactivate", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.Deactivate)
	v1.PUT("/users/:id/password", capabilityAccess(v1, http.MethodPut, "/users/:id/password", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.ResetPassword)
	if notificationH != nil {
		v1.POST("/notifications/broadcast", capabilityAccess(v1, http.MethodPost, "/notifications/broadcast", domain.APIReadinessReadyForFrontend, domain.PermissionSystemManage), notificationH.Broadcast)
	}
	v1.GET("/org/options", capabilityAccess(v1, http.MethodGet, "/org/options", domain.APIReadinessReadyForFrontend, domain.PermissionAccessView, domain.PermissionAccessManage), userAdminH.GetOrgOptions)
	v1.POST("/org/departments", capabilityAccess(v1, http.MethodPost, "/org/departments", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.CreateDepartment)
	v1.PUT("/org/departments/:id", capabilityAccess(v1, http.MethodPut, "/org/departments/:id", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.UpdateDepartment)
	v1.POST("/org/departments/:id/merge", capabilityAccess(v1, http.MethodPost, "/org/departments/:id/merge", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.MergeDepartment)
	v1.DELETE("/org/departments/:id", capabilityAccess(v1, http.MethodDelete, "/org/departments/:id", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.DeleteDepartment)
	v1.POST("/org/teams", capabilityAccess(v1, http.MethodPost, "/org/teams", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.CreateTeam)
	v1.PUT("/org/teams/:id", capabilityAccess(v1, http.MethodPut, "/org/teams/:id", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.UpdateTeam)
	v1.POST("/org/teams/:id/merge", capabilityAccess(v1, http.MethodPost, "/org/teams/:id/merge", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.MergeTeam)
	v1.DELETE("/org/teams/:id", capabilityAccess(v1, http.MethodDelete, "/org/teams/:id", domain.APIReadinessReadyForFrontend, domain.PermissionAccessManage), userAdminH.DeleteTeam)
}
