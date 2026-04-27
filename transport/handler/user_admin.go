package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type UserAdminHandler struct {
	svc             service.IdentityService
	operationLogSvc service.OperationLogService
	routeRules      routeAccessRuleReader
}

type routeAccessRuleReader interface {
	ListRouteAccessRules() []domain.RouteAccessRule
}

func NewUserAdminHandler(svc service.IdentityService, routeRules routeAccessRuleReader, operationLogSvc service.OperationLogService) *UserAdminHandler {
	return &UserAdminHandler{svc: svc, routeRules: routeRules, operationLogSvc: operationLogSvc}
}

type patchUserReq struct {
	DisplayName        *string        `json:"display_name"`
	Status             *string        `json:"status"`
	EmploymentType     *string        `json:"employment_type"`
	Department         *string        `json:"department"`
	Team               *string        `json:"team"`
	Group              *string        `json:"group"`
	Email              *string        `json:"email"`
	Mobile             *string        `json:"mobile"`
	ManagedDepartments *[]string      `json:"managed_departments"`
	ManagedTeams       *[]string      `json:"managed_teams"`
	Roles              *[]domain.Role `json:"roles"`
	Avatar             *string        `json:"avatar"`
	TeamCodes          *[]string      `json:"team_codes"`
	PrimaryTeamCode    *string        `json:"primary_team_code"`
}

type createUserReq struct {
	Username           string        `json:"username"`
	Account            string        `json:"account"`
	DisplayName        string        `json:"display_name"`
	Name               string        `json:"name"`
	Department         string        `json:"department"`
	Team               string        `json:"team"`
	Group              string        `json:"group"`
	Mobile             string        `json:"mobile"`
	Phone              string        `json:"phone"`
	Email              string        `json:"email"`
	Password           string        `json:"password"`
	Roles              []domain.Role `json:"roles"`
	Status             *string       `json:"status"`
	EmploymentType     *string       `json:"employment_type"`
	ManagedDepartments *[]string     `json:"managed_departments"`
}

type createDepartmentReq struct {
	Name string `json:"name"`
}

type updateDepartmentReq struct {
	Enabled *bool `json:"enabled"`
}

type createTeamReq struct {
	DepartmentID *int64 `json:"department_id"`
	Department   string `json:"department"`
	Name         string `json:"name"`
}

type updateTeamReq struct {
	Enabled *bool `json:"enabled"`
}

type setUserRolesReq struct {
	Roles []domain.Role `json:"roles"`
}

type resetUserPasswordReq struct {
	Password string `json:"password"`
}

func (h *UserAdminHandler) ListUsers(c *gin.Context) {
	if !h.ensureManagementReadAccess(c) {
		return
	}
	var status *domain.UserStatus
	if raw := c.Query("status"); raw != "" {
		value := domain.UserStatus(raw)
		status = &value
	}
	var role *domain.Role
	if raw := c.Query("role"); raw != "" {
		value := domain.Role(raw)
		role = &value
	}
	var department *domain.Department
	if raw := c.Query("department"); raw != "" {
		value := domain.Department(raw)
		department = &value
	}
	page, _ := parseInt(c.Query("page"))
	pageSize, _ := parseInt(c.Query("page_size"))
	users, pagination, appErr := h.svc.ListUsers(c.Request.Context(), service.UserFilter{
		Keyword:    c.Query("keyword"),
		Status:     status,
		Role:       role,
		Department: department,
		Team:       c.Query("team"),
		Page:       page,
		PageSize:   pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, users, pagination)
}

// ListDesigners returns designers for task assignment (Ops/Designer/Admin/
// HRAdmin/SuperAdmin). Minimal fields: id, username, display_name.
//
// Round D (v1.6): this handler now routes to the dedicated
// `ListAssignableDesigners` service method, which bypasses the standard
// `authorizeUserListFilter` management-scope filter and returns every active
// candidate for the requested workflow_lane regardless of the actor's
// department/team. The default lane is normal, preserving Round D Designer
// behavior; customization selects CustomizationOperator; all returns the
// deduped union. Access control for this route is enforced exclusively by
// the route guard registered in transport/http.go (`/v1/users/designers`).
//
// This endpoint intentionally accepts no keyword/department/team/pagination
// parameters — it is a narrowly scoped assignment-candidate-pool lookup.
func (h *UserAdminHandler) ListDesigners(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	lane, appErr := parseAssignableLane(c.Query("workflow_lane"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	users, appErr := h.svc.ListAssignableDesigners(c.Request.Context(), &actor, lane)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	type designerItem struct {
		ID          int64  `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	}
	items := make([]designerItem, 0, len(users))
	for _, u := range users {
		items = append(items, designerItem{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName})
	}
	total := int64(len(items))
	respondOKWithPagination(c, items, domain.PaginationMeta{
		Page:     1,
		PageSize: len(items),
		Total:    total,
	})
}

func parseAssignableLane(raw string) (service.AssignableLane, *domain.AppError) {
	switch strings.TrimSpace(raw) {
	case "", string(service.AssignableLaneNormal):
		return service.AssignableLaneNormal, nil
	case string(service.AssignableLaneCustomization):
		return service.AssignableLaneCustomization, nil
	case string(service.AssignableLaneAll):
		return service.AssignableLaneAll, nil
	default:
		return "", domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			"workflow_lane is not supported",
			map[string]string{
				"field":     "workflow_lane",
				"deny_code": "workflow_lane_unsupported",
			},
		)
	}
}

func (h *UserAdminHandler) GetUser(c *gin.Context) {
	if !h.ensureManagementReadAccess(c) {
		return
	}
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user id", nil))
		return
	}
	user, appErr := h.svc.GetUser(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, user)
}

func (h *UserAdminHandler) CreateUser(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	var status *domain.UserStatus
	if req.Status != nil {
		value := domain.UserStatus(*req.Status)
		status = &value
	}
	var employmentType *domain.EmploymentType
	if req.EmploymentType != nil {
		value := domain.EmploymentType(*req.EmploymentType)
		employmentType = &value
	}
	user, appErr := h.svc.CreateManagedUser(c.Request.Context(), service.CreateManagedUserParams{
		Username:           firstNonEmpty(req.Account, req.Username),
		DisplayName:        firstNonEmpty(req.Name, req.DisplayName),
		Department:         domain.Department(req.Department),
		Team:               firstNonEmpty(req.Group, req.Team),
		Mobile:             firstNonEmpty(req.Phone, req.Mobile),
		Email:              req.Email,
		Password:           req.Password,
		Roles:              req.Roles,
		Status:             status,
		EmploymentType:     employmentType,
		ManagedDepartments: req.ManagedDepartments,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, user)
}

func (h *UserAdminHandler) PatchUser(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user id", nil))
		return
	}
	var req patchUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	var status *domain.UserStatus
	if req.Status != nil {
		value := domain.UserStatus(*req.Status)
		status = &value
	}
	var employmentType *domain.EmploymentType
	if req.EmploymentType != nil {
		value := domain.EmploymentType(*req.EmploymentType)
		employmentType = &value
	}
	var department *domain.Department
	if req.Department != nil {
		value := domain.Department(*req.Department)
		department = &value
	}
	user, appErr := h.svc.UpdateUser(c.Request.Context(), service.UpdateUserParams{
		UserID:             id,
		DisplayName:        req.DisplayName,
		Status:             status,
		EmploymentType:     employmentType,
		Department:         department,
		Team:               req.Team,
		Group:              req.Group,
		Email:              req.Email,
		Mobile:             req.Mobile,
		ManagedDepartments: req.ManagedDepartments,
		ManagedTeams:       req.ManagedTeams,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	if req.Roles != nil {
		user, appErr = h.svc.SetUserRoles(c.Request.Context(), service.SetUserRolesParams{
			UserID: id,
			Roles:  *req.Roles,
		})
		if appErr != nil {
			respondError(c, appErr)
			return
		}
	}
	respondOK(c, user)
}

func (h *UserAdminHandler) ResetPassword(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user id", nil))
		return
	}
	var req resetUserPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	user, appErr := h.svc.ResetUserPassword(c.Request.Context(), service.ResetUserPasswordParams{
		UserID:      id,
		NewPassword: req.Password,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, user)
}

func (h *UserAdminHandler) SetRoles(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user id", nil))
		return
	}
	var req setUserRolesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	user, appErr := h.svc.SetUserRoles(c.Request.Context(), service.SetUserRolesParams{
		UserID: id,
		Roles:  req.Roles,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, user)
}

func (h *UserAdminHandler) AddRoles(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user id", nil))
		return
	}
	var req setUserRolesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	user, appErr := h.svc.AddUserRoles(c.Request.Context(), service.AddUserRolesParams{
		UserID: id,
		Roles:  req.Roles,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, user)
}

func (h *UserAdminHandler) RemoveRole(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user id", nil))
		return
	}
	user, appErr := h.svc.RemoveUserRole(c.Request.Context(), service.RemoveUserRoleParams{
		UserID: id,
		Role:   domain.Role(c.Param("role")),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, user)
}

func (h *UserAdminHandler) ListRoles(c *gin.Context) {
	if !h.ensureRoleCatalogAccess(c) {
		return
	}
	respondOK(c, h.svc.ListRoles(c.Request.Context()))
}

func (h *UserAdminHandler) GetOrgOptions(c *gin.Context) {
	if !h.ensureOrgOptionsAccess(c) {
		return
	}
	options, appErr := h.svc.GetOrgOptions(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	if options != nil && len(options.TeamsByDepartment) > 0 {
		c.Header("Deprecation", `version="v1.8"`)
	}
	respondOK(c, options)
}

func (h *UserAdminHandler) CreateDepartment(c *gin.Context) {
	var req createDepartmentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	item, appErr := h.svc.CreateDepartment(c.Request.Context(), service.CreateOrgDepartmentParams{Name: req.Name})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, item)
}

func (h *UserAdminHandler) UpdateDepartment(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid department id", nil))
		return
	}
	var req updateDepartmentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	item, appErr := h.svc.UpdateDepartment(c.Request.Context(), service.UpdateOrgDepartmentParams{
		ID:      id,
		Enabled: req.Enabled,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *UserAdminHandler) CreateTeam(c *gin.Context) {
	var req createTeamReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	item, appErr := h.svc.CreateTeam(c.Request.Context(), service.CreateOrgTeamParams{
		DepartmentID: req.DepartmentID,
		Department:   req.Department,
		Name:         req.Name,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, item)
}

func (h *UserAdminHandler) UpdateTeam(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid team id", nil))
		return
	}
	var req updateTeamReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	item, appErr := h.svc.UpdateTeam(c.Request.Context(), service.UpdateOrgTeamParams{
		ID:      id,
		Enabled: req.Enabled,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *UserAdminHandler) ListPermissionLogs(c *gin.Context) {
	if !h.ensureManagementReadAccess(c) {
		return
	}
	var actorID *int64
	if raw := c.Query("actor_id"); raw != "" {
		if value, err := parseInt64(raw); err == nil && value > 0 {
			actorID = &value
		}
	}
	var targetUserID *int64
	if raw := c.Query("target_user_id"); raw != "" {
		if value, err := parseInt64(raw); err == nil && value > 0 {
			targetUserID = &value
		}
	}
	var granted *bool
	if raw := c.Query("granted"); raw != "" {
		value := raw == "true" || raw == "1"
		granted = &value
	}
	page, _ := parseInt(c.Query("page"))
	pageSize, _ := parseInt(c.Query("page_size"))
	logs, pagination, appErr := h.svc.ListPermissionLogs(c.Request.Context(), service.PermissionLogFilter{
		ActorID:        actorID,
		ActorUsername:  c.Query("actor_username"),
		ActionType:     c.Query("action_type"),
		TargetUserID:   targetUserID,
		TargetUsername: c.Query("target_username"),
		Granted:        granted,
		Method:         c.Query("method"),
		RoutePath:      c.Query("route_path"),
		Page:           page,
		PageSize:       pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, logs, pagination)
}

func (h *UserAdminHandler) ListRouteAccessRules(c *gin.Context) {
	if !h.ensureManagementReadAccess(c) {
		return
	}
	if h.routeRules == nil {
		respondOK(c, []domain.RouteAccessRule{})
		return
	}
	respondOK(c, h.routeRules.ListRouteAccessRules())
}

func (h *UserAdminHandler) ListOperationLogs(c *gin.Context) {
	if !h.ensureOperationLogAccess(c) {
		return
	}
	if h.operationLogSvc == nil {
		respondOKWithPagination(c, []*domain.OperationLogEntry{}, domain.PaginationMeta{
			Page:     1,
			PageSize: 20,
			Total:    0,
		})
		return
	}
	page, _ := parseInt(c.Query("page"))
	pageSize, _ := parseInt(c.Query("page_size"))
	logs, pagination, appErr := h.operationLogSvc.List(c.Request.Context(), service.OperationLogFilter{
		Source:    c.Query("source"),
		EventType: c.Query("event_type"),
		Page:      page,
		PageSize:  pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, logs, pagination)
}

func (h *UserAdminHandler) ensureManagementReadAccess(c *gin.Context) bool {
	user, appErr := h.svc.GetCurrentUser(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return false
	}
	if user == nil {
		respondError(c, domain.ErrUnauthorized)
		return false
	}
	if hasManagementReadRole(user.Roles) || user.FrontendAccess.IsSuperAdmin {
		return true
	}
	respondError(c, domain.NewAppError(domain.ErrCodePermissionDenied, "management access is required", nil))
	return false
}

func (h *UserAdminHandler) ensureRoleCatalogAccess(c *gin.Context) bool {
	user, appErr := h.svc.GetCurrentUser(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return false
	}
	if user == nil {
		respondError(c, domain.ErrUnauthorized)
		return false
	}
	if hasRoleCatalogAccess(user.Roles) || user.FrontendAccess.IsSuperAdmin {
		return true
	}
	respondError(c, domain.NewAppError(domain.ErrCodePermissionDenied, "role catalog access requires department management or higher", nil))
	return false
}

func (h *UserAdminHandler) ensureOrgOptionsAccess(c *gin.Context) bool {
	user, appErr := h.svc.GetCurrentUser(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return false
	}
	if user == nil {
		respondError(c, domain.ErrUnauthorized)
		return false
	}
	if hasOrgOptionsAccess(user.Roles) || user.FrontendAccess.IsSuperAdmin {
		return true
	}
	respondError(c, domain.NewAppError(domain.ErrCodePermissionDenied, "organization access requires department management or higher", nil))
	return false
}

func (h *UserAdminHandler) ensureOperationLogAccess(c *gin.Context) bool {
	user, appErr := h.svc.GetCurrentUser(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return false
	}
	if user == nil {
		respondError(c, domain.ErrUnauthorized)
		return false
	}
	if hasOperationLogAccess(user.Roles) || user.FrontendAccess.IsSuperAdmin {
		return true
	}
	respondError(c, domain.NewAppError(domain.ErrCodePermissionDenied, "operation logs require HRAdmin or SuperAdmin access", nil))
	return false
}

func hasManagementReadRole(roles []domain.Role) bool {
	for _, role := range domain.NormalizeRoleValues(roles) {
		switch role {
		case domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleOrgAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin, domain.RoleTeamLead:
			return true
		}
	}
	return false
}

func hasRoleCatalogAccess(roles []domain.Role) bool {
	for _, role := range domain.NormalizeRoleValues(roles) {
		switch role {
		case domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleOrgAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin:
			return true
		}
	}
	return false
}

func hasOrgOptionsAccess(roles []domain.Role) bool {
	for _, role := range domain.NormalizeRoleValues(roles) {
		switch role {
		case domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin, domain.RoleOrgAdmin, domain.RoleRoleAdmin, domain.RoleDeptAdmin:
			return true
		}
	}
	return false
}

func hasOperationLogAccess(roles []domain.Role) bool {
	for _, role := range domain.NormalizeRoleValues(roles) {
		switch role {
		case domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleHRAdmin:
			return true
		}
	}
	return false
}
