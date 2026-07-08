package handler

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type UserAdminHandler struct {
	svc             service.IdentityService
	operationLogSvc service.OperationLogService
	traceEventSvc   service.WorkflowTraceEventService
	routeRules      routeAccessRuleReader
}

type routeAccessRuleReader interface {
	ListRouteAccessRules() []domain.RouteAccessRule
}

func NewUserAdminHandler(svc service.IdentityService, routeRules routeAccessRuleReader, operationLogSvc service.OperationLogService, traceEventSvcs ...service.WorkflowTraceEventService) *UserAdminHandler {
	h := &UserAdminHandler{svc: svc, routeRules: routeRules, operationLogSvc: operationLogSvc}
	if len(traceEventSvcs) > 0 {
		h.traceEventSvc = traceEventSvcs[0]
	}
	return h
}

type patchUserReq struct {
	EmployeeNo         json.RawMessage `json:"employee_no"`
	DisplayName        *string         `json:"display_name"`
	Status             *string         `json:"status"`
	EmploymentType     *string         `json:"employment_type"`
	Department         *string         `json:"department"`
	Team               *string         `json:"team"`
	Group              *string         `json:"group"`
	Email              *string         `json:"email"`
	Mobile             *string         `json:"mobile"`
	ManagedDepartments *[]string       `json:"managed_departments"`
	ManagedTeams       *[]string       `json:"managed_teams"`
	Roles              *[]domain.Role  `json:"roles"`
	Avatar             *string         `json:"avatar"`
	TeamCodes          *[]string       `json:"team_codes"`
	PrimaryTeamCode    *string         `json:"primary_team_code"`
}

type createUserReq struct {
	Username           string          `json:"username"`
	EmployeeNo         json.RawMessage `json:"employee_no"`
	Account            string          `json:"account"`
	DisplayName        string          `json:"display_name"`
	Name               string          `json:"name"`
	Department         string          `json:"department"`
	Team               string          `json:"team"`
	Group              string          `json:"group"`
	Mobile             string          `json:"mobile"`
	Phone              string          `json:"phone"`
	Email              string          `json:"email"`
	Password           string          `json:"password"`
	Roles              []domain.Role   `json:"roles"`
	Status             *string         `json:"status"`
	EmploymentType     *string         `json:"employment_type"`
	ManagedDepartments *[]string       `json:"managed_departments"`
}

type createDepartmentReq struct {
	Name string `json:"name"`
}

type updateDepartmentReq struct {
	Name    *string `json:"name"`
	Enabled *bool   `json:"enabled"`
}

type createTeamReq struct {
	DepartmentID *int64 `json:"department_id"`
	Department   string `json:"department"`
	Name         string `json:"name"`
}

type updateTeamReq struct {
	Name    *string `json:"name"`
	Enabled *bool   `json:"enabled"`
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
// behavior; customization selects CustomizationOperator; audit selects regular
// auditors; all returns the deduped design/customization union. Access control
// for this route is enforced exclusively by
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
		Name        string `json:"name,omitempty"`
		RealName    string `json:"real_name,omitempty"`
	}
	items := make([]designerItem, 0, len(users))
	for _, u := range users {
		items = append(items, designerItem{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Name: u.Name, RealName: u.RealName})
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
	case string(service.AssignableLaneAudit):
		return service.AssignableLaneAudit, nil
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
	employeeNo, appErr := parseEmployeeNoRequestField(req.EmployeeNo, true)
	if appErr != nil {
		respondError(c, appErr)
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
		EmployeeNo:         employeeNo,
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
	employeeNo, appErr := parseEmployeeNoRequestField(req.EmployeeNo, false)
	if appErr != nil {
		respondError(c, appErr)
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
		EmployeeNo:         employeeNo,
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
	user, ok := h.loadOrgOptionsUser(c)
	if !ok {
		return
	}
	includeDisabled, appErr := parseIncludeDisabledOrgOptions(c.Query("include_disabled"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	if includeDisabled && !hasOrgMasterWriteRole(user.Roles) && !user.FrontendAccess.IsSuperAdmin {
		respondError(c, domain.NewAppError(domain.ErrCodePermissionDenied, "组织维护权限不足，不能查看已停用组织。", nil))
		return
	}
	var options *domain.OrgOptions
	if includeDisabled {
		options, appErr = h.svc.GetOrgOptionsIncludingDisabled(c.Request.Context())
	} else {
		options, appErr = h.svc.GetOrgOptions(c.Request.Context())
	}
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
		Name:    req.Name,
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
		Name:    req.Name,
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

type recordWorkflowTraceEventReq struct {
	EventType            string          `json:"event_type"`
	Action               string          `json:"action"`
	PageURL              string          `json:"page_url"`
	PageName             string          `json:"page_name"`
	ComponentID          string          `json:"component_id"`
	TaskID               *int64          `json:"task_id"`
	TaskModuleID         *int64          `json:"task_module_id"`
	ModuleKey            string          `json:"module_key"`
	SKUCode              string          `json:"sku_code"`
	TaskSKUItemID        *int64          `json:"task_sku_item_id"`
	AssetID              *int64          `json:"asset_id"`
	DesignAssetID        *int64          `json:"design_asset_id"`
	TaskAssetID          *int64          `json:"task_asset_id"`
	IntegrationCallLogID *int64          `json:"integration_call_log_id"`
	ResourceType         string          `json:"resource_type"`
	ResourceID           string          `json:"resource_id"`
	Outcome              string          `json:"outcome"`
	Payload              json.RawMessage `json:"payload"`
	OccurredAt           *time.Time      `json:"occurred_at"`
}

func (h *UserAdminHandler) RecordWorkflowTraceEvent(c *gin.Context) {
	if h.traceEventSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "workflow trace service is not configured", nil))
		return
	}
	var req recordWorkflowTraceEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	event := &domain.WorkflowTraceEvent{
		TraceID:              c.GetString("trace_id"),
		EventSource:          domain.WorkflowTraceSourceFrontend,
		EventType:            req.EventType,
		Action:               req.Action,
		ActorID:              actorIDPtrFromRequestActor(actor),
		ActorUsername:        actor.Username,
		ActorSource:          actor.Source,
		ActorAuthMode:        actor.AuthMode,
		ActorRoles:           actor.Roles,
		ActorDepartment:      actor.Department,
		ActorTeam:            actor.Team,
		ClientIP:             c.ClientIP(),
		UserAgent:            c.GetHeader("User-Agent"),
		PageURL:              req.PageURL,
		PageName:             req.PageName,
		ComponentID:          req.ComponentID,
		TaskID:               req.TaskID,
		TaskModuleID:         req.TaskModuleID,
		ModuleKey:            req.ModuleKey,
		SKUCode:              req.SKUCode,
		TaskSKUItemID:        req.TaskSKUItemID,
		AssetID:              req.AssetID,
		DesignAssetID:        req.DesignAssetID,
		TaskAssetID:          req.TaskAssetID,
		IntegrationCallLogID: req.IntegrationCallLogID,
		ResourceType:         req.ResourceType,
		ResourceID:           req.ResourceID,
		Outcome:              req.Outcome,
		Payload:              req.Payload,
	}
	if req.OccurredAt != nil {
		event.OccurredAt = req.OccurredAt.UTC()
	}
	created, appErr := h.traceEventSvc.RecordTraceEvent(c.Request.Context(), event)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, created)
}

func (h *UserAdminHandler) ListWorkflowTraceEvents(c *gin.Context) {
	if !h.ensureOperationLogAccess(c) {
		return
	}
	if h.traceEventSvc == nil {
		respondOKWithPagination(c, []*domain.WorkflowTraceEvent{}, domain.PaginationMeta{Page: 1, PageSize: 20, Total: 0})
		return
	}
	page, _ := parseInt(c.Query("page"))
	pageSize, _ := parseInt(c.Query("page_size"))
	filter := service.WorkflowTraceEventFilter{
		TraceID:         c.Query("trace_id"),
		EventSource:     c.Query("event_source"),
		EventType:       c.Query("event_type"),
		Action:          c.Query("action"),
		ActorUsername:   c.Query("actor_username"),
		ActorSource:     c.Query("actor_source"),
		ActorDepartment: c.Query("actor_department"),
		ActorTeam:       c.Query("actor_team"),
		RoutePath:       c.Query("route_path"),
		ModuleKey:       c.Query("module_key"),
		SKUCode:         c.Query("sku_code"),
		ResourceType:    c.Query("resource_type"),
		ResourceID:      c.Query("resource_id"),
		Outcome:         c.Query("outcome"),
		BusinessOnly:    parseTraceBool(c.Query("business_only")),
		Page:            page,
		PageSize:        pageSize,
	}
	if raw := strings.TrimSpace(c.Query("actor_id")); raw != "" {
		if value, err := parseInt64(raw); err == nil && value > 0 {
			filter.ActorID = &value
		}
	}
	if raw := strings.TrimSpace(c.Query("task_id")); raw != "" {
		if value, err := parseInt64(raw); err == nil && value > 0 {
			filter.TaskID = &value
		}
	}
	if raw := strings.TrimSpace(c.Query("asset_id")); raw != "" {
		if value, err := parseInt64(raw); err == nil && value > 0 {
			filter.AssetID = &value
		}
	}
	if raw := strings.TrimSpace(c.Query("design_asset_id")); raw != "" {
		if value, err := parseInt64(raw); err == nil && value > 0 {
			filter.DesignAssetID = &value
		}
	}
	if raw := strings.TrimSpace(c.Query("task_asset_id")); raw != "" {
		if value, err := parseInt64(raw); err == nil && value > 0 {
			filter.TaskAssetID = &value
		}
	}
	if raw := strings.TrimSpace(c.Query("integration_call_log_id")); raw != "" {
		if value, err := parseInt64(raw); err == nil && value > 0 {
			filter.IntegrationCallLogID = &value
		}
	}
	if raw := firstNonEmptyTraceQuery(c.Query("from"), c.Query("since")); raw != "" {
		if value, err := time.Parse(time.RFC3339, raw); err == nil {
			filter.From = &value
		}
	}
	if raw := firstNonEmptyTraceQuery(c.Query("to"), c.Query("until")); raw != "" {
		if value, err := time.Parse(time.RFC3339, raw); err == nil {
			filter.To = &value
		}
	}
	events, pagination, appErr := h.traceEventSvc.ListTraceEvents(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, events, pagination)
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

func (h *UserAdminHandler) loadOrgOptionsUser(c *gin.Context) (*domain.User, bool) {
	user, appErr := h.svc.GetCurrentUser(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return nil, false
	}
	if user == nil {
		respondError(c, domain.ErrUnauthorized)
		return nil, false
	}
	if hasOrgOptionsAccess(user.Roles) || user.FrontendAccess.IsSuperAdmin {
		return user, true
	}
	respondError(c, domain.NewAppError(domain.ErrCodePermissionDenied, "organization access requires department management or higher", nil))
	return nil, false
}

func (h *UserAdminHandler) ensureOrgOptionsAccess(c *gin.Context) bool {
	_, ok := h.loadOrgOptionsUser(c)
	return ok
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

func hasOrgMasterWriteRole(roles []domain.Role) bool {
	for _, role := range domain.NormalizeRoleValues(roles) {
		switch role {
		case domain.RoleSuperAdmin, domain.RoleHRAdmin:
			return true
		}
	}
	return false
}

func parseIncludeDisabledOrgOptions(raw string) (bool, *domain.AppError) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(trimmed)
	if err != nil {
		return false, domain.NewAppError(domain.ErrCodeInvalidRequest, "include_disabled 参数必须为 true 或 false。", map[string]interface{}{
			"field": "include_disabled",
		})
	}
	return value, nil
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

func actorIDPtrFromRequestActor(actor domain.RequestActor) *int64 {
	if actor.ID <= 0 {
		return nil
	}
	return &actor.ID
}

func firstNonEmptyTraceQuery(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseEmployeeNoRequestField(raw json.RawMessage, required bool) (*int, *domain.AppError) {
	if len(raw) == 0 {
		if required {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "工号必填。", map[string]interface{}{"deny_code": "employee_no_required"})
		}
		return nil, nil
	}
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "工号不能为空。", map[string]interface{}{"deny_code": "employee_no_required"})
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "工号必须是 0 到 9999 之间的纯数字。", map[string]interface{}{"deny_code": "employee_no_invalid"})
		}
	}
	value, err := strconv.Atoi(text)
	if err != nil || value < 0 || value > 9999 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "工号必须是 0 到 9999 之间的纯数字。", map[string]interface{}{"deny_code": "employee_no_invalid"})
	}
	return &value, nil
}

func parseTraceBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
