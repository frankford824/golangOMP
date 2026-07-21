package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type UserAdminHandler struct {
	svc           service.IdentityService
	traceEventSvc service.WorkflowTraceEventService
}

func NewUserAdminHandler(svc service.IdentityService, traceEventSvcs ...service.WorkflowTraceEventService) *UserAdminHandler {
	h := &UserAdminHandler{svc: svc}
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
	DepartmentID       *int64          `json:"department_id"`
	Team               *string         `json:"team"`
	TeamID             *int64          `json:"team_id"`
	Email              *string         `json:"email"`
	Mobile             *string         `json:"mobile"`
	ManagedDepartments *[]string       `json:"managed_departments"`
	ManagedTeams       *[]string       `json:"managed_teams"`
}

type createUserReq struct {
	Username           string          `json:"username"`
	EmployeeNo         json.RawMessage `json:"employee_no"`
	DisplayName        string          `json:"display_name"`
	Department         string          `json:"department"`
	DepartmentID       *int64          `json:"department_id"`
	Team               string          `json:"team"`
	TeamID             *int64          `json:"team_id"`
	Mobile             string          `json:"mobile"`
	Email              string          `json:"email"`
	Password           string          `json:"password"`
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

type mergeDepartmentReq struct {
	TargetDepartmentID int64 `json:"target_department_id"`
}

type mergeTeamReq struct {
	TargetTeamID int64 `json:"target_team_id"`
}

type resetUserPasswordReq struct {
	Password string `json:"password"`
}

func (h *UserAdminHandler) ListUsers(c *gin.Context) {
	var status *domain.UserStatus
	if raw := c.Query("status"); raw != "" {
		value := domain.UserStatus(raw)
		status = &value
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

// ListAccessPolicyUsers returns the minimal personnel selector used by the
// explicit access-policy console. Authorization is enforced by the
// capability guard on /v1/access/users; no legacy role inference happens in
// this handler or its service path.
func (h *UserAdminHandler) ListAccessPolicyUsers(c *gin.Context) {
	page, _ := parseInt(c.Query("page"))
	pageSize, _ := parseInt(c.Query("page_size"))
	users, pagination, appErr := h.svc.ListAccessPolicyUsers(c.Request.Context(), service.UserFilter{
		Keyword:  c.Query("q"),
		Page:     page,
		PageSize: pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	type option struct {
		ID           int64             `json:"id"`
		Username     string            `json:"username"`
		DisplayName  string            `json:"display_name"`
		Department   domain.Department `json:"department"`
		DepartmentID *int64            `json:"department_id,omitempty"`
		Team         string            `json:"team,omitempty"`
		TeamID       *int64            `json:"team_id,omitempty"`
	}
	items := make([]option, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		items = append(items, option{ID: user.ID, Username: user.Username, DisplayName: user.DisplayName, Department: user.Department, DepartmentID: user.DepartmentID, Team: user.Team, TeamID: user.TeamID})
	}
	respondOKWithPagination(c, items, pagination)
}

// ListDesigners returns the active candidate pool selected from explicit
// auth_* assignments. It intentionally exposes only identity labels needed by
// task assignment and audit handover controls.
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
		Username:           req.Username,
		EmployeeNo:         employeeNo,
		DisplayName:        req.DisplayName,
		Department:         domain.Department(req.Department),
		DepartmentID:       req.DepartmentID,
		Team:               req.Team,
		TeamID:             req.TeamID,
		Mobile:             req.Mobile,
		Email:              req.Email,
		Password:           req.Password,
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
		DepartmentID:       req.DepartmentID,
		Team:               req.Team,
		TeamID:             req.TeamID,
		Email:              req.Email,
		Mobile:             req.Mobile,
		ManagedDepartments: req.ManagedDepartments,
		ManagedTeams:       req.ManagedTeams,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
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

func (h *UserAdminHandler) GetOrgOptions(c *gin.Context) {
	includeDisabled, appErr := parseIncludeDisabledOrgOptions(c.Query("include_disabled"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	if includeDisabled {
		actor, ok := domain.RequestActorFromContext(c.Request.Context())
		if !ok || !domain.ActorHasPermission(actor, domain.PermissionAccessManage) || !domain.ResourceGroupAccessFilterForActor(actor, domain.PermissionAccessManage).Global {
			respondError(c, domain.NewAppError(domain.ErrCodePermissionDenied, "查看已停用组织需要全局组织管理权限。", nil))
			return
		}
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

func (h *UserAdminHandler) MergeDepartment(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid department id", nil))
		return
	}
	var req mergeDepartmentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	item, appErr := h.svc.MergeDepartment(c.Request.Context(), service.MergeOrgDepartmentParams{
		SourceID: id,
		TargetID: req.TargetDepartmentID,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *UserAdminHandler) MergeTeam(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid team id", nil))
		return
	}
	var req mergeTeamReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	item, appErr := h.svc.MergeTeam(c.Request.Context(), service.MergeOrgTeamParams{
		SourceID: id,
		TargetID: req.TargetTeamID,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *UserAdminHandler) DeleteDepartment(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid department id", nil))
		return
	}
	if appErr := h.svc.DeleteDepartment(c.Request.Context(), id); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *UserAdminHandler) DeleteTeam(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid team id", nil))
		return
	}
	if appErr := h.svc.DeleteTeam(c.Request.Context(), id); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(http.StatusNoContent)
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

func actorIDPtrFromRequestActor(actor domain.RequestActor) *int64 {
	if actor.ID <= 0 {
		return nil
	}
	return &actor.ID
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
