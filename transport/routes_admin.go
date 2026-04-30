package transport

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/transport/handler"
)

func registerV1AdminRoutes(
	v1 *gin.RouterGroup,
	access routeAccessRegistrar,
	legacyRoleConvergedAccess legacyRouteAccessRegistrar,
	userAdminH *handler.UserAdminHandler,
	orgMoveH *handler.OrgMoveRequestHandler,
	auditLogH *handler.AuditLogHandler,
	serverLogH *handler.ServerLogHandler,
) {
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
}
