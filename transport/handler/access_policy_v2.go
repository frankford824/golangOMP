package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type AccessPolicyHandler struct {
	svc service.AccessPolicyService
}

func NewAccessPolicyHandler(svc service.AccessPolicyService) *AccessPolicyHandler {
	return &AccessPolicyHandler{svc: svc}
}

func (h *AccessPolicyHandler) ListPermissions(c *gin.Context) {
	items, appErr := h.svc.ListPermissions(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func (h *AccessPolicyHandler) ListRoles(c *gin.Context) {
	includeArchived, _ := strconv.ParseBool(c.Query("include_archived"))
	items, appErr := h.svc.ListRoles(c.Request.Context(), includeArchived)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func (h *AccessPolicyHandler) CreateRole(c *gin.Context) {
	var req service.AccessRoleCreateRequest
	if !bindAccessJSON(c, &req) {
		return
	}
	result, appErr := h.svc.CreateRole(c.Request.Context(), requestActor(c), req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AccessPolicyHandler) UpdateRole(c *gin.Context) {
	roleID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	var req service.AccessRoleUpdateRequest
	if !bindAccessJSON(c, &req) {
		return
	}
	result, appErr := h.svc.UpdateRole(c.Request.Context(), requestActor(c), roleID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AccessPolicyHandler) ArchiveRole(c *gin.Context) {
	roleID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	var req service.AccessRoleArchiveRequest
	if !bindAccessJSON(c, &req) {
		return
	}
	result, appErr := h.svc.ArchiveRole(c.Request.Context(), requestActor(c), roleID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AccessPolicyHandler) ReplaceRolePermissions(c *gin.Context) {
	roleID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	var req service.ReplaceRolePermissionsRequest
	if !bindAccessJSON(c, &req) {
		return
	}
	result, appErr := h.svc.ReplaceRolePermissions(c.Request.Context(), requestActor(c), roleID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AccessPolicyHandler) ReplaceUserAssignments(c *gin.Context) {
	userID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	var req domain.ReplaceAccessAssignmentsRequest
	if !bindAccessJSON(c, &req) {
		return
	}
	result, appErr := h.svc.ReplaceUserAssignments(c.Request.Context(), requestActor(c), userID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AccessPolicyHandler) EffectiveAccess(c *gin.Context) {
	userID, ok := accessPathID(c, "id")
	if !ok {
		return
	}
	result, appErr := h.svc.EffectiveAccess(c.Request.Context(), userID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AccessPolicyHandler) GetOrgPolicies(c *gin.Context) {
	subjectType, subjectID, ok := accessOrgSubject(c)
	if !ok {
		return
	}
	result, appErr := h.svc.GetOrgPolicies(c.Request.Context(), subjectType, subjectID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AccessPolicyHandler) ReplaceOrgPolicies(c *gin.Context) {
	subjectType, subjectID, ok := accessOrgSubject(c)
	if !ok {
		return
	}
	var req service.ReplaceOrgPoliciesRequest
	if !bindAccessJSON(c, &req) {
		return
	}
	result, appErr := h.svc.ReplaceOrgPolicies(c.Request.Context(), requestActor(c), subjectType, subjectID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AccessPolicyHandler) Preview(c *gin.Context) {
	var req struct {
		UserID int64 `json:"user_id" binding:"required"`
	}
	if !bindAccessJSON(c, &req) {
		return
	}
	result, appErr := h.svc.EffectiveAccess(c.Request.Context(), req.UserID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AccessPolicyHandler) ListEvents(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	result, appErr := h.svc.ListEvents(c.Request.Context(), limit)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func requestActor(c *gin.Context) domain.RequestActor {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	return actor
}

func bindAccessJSON(c *gin.Context, target interface{}) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return false
	}
	return true
}

func accessPathID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil || id <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid "+name, nil))
		return 0, false
	}
	return id, true
}

func accessOrgSubject(c *gin.Context) (domain.AccessSubjectType, int64, bool) {
	subjectType := domain.AccessSubjectType(strings.TrimSpace(c.Param("subject_type")))
	subjectID, ok := accessPathID(c, "subject_id")
	if !ok {
		return "", 0, false
	}
	if !subjectType.Valid() {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "subject_type must be department or team", nil))
		return "", 0, false
	}
	return subjectType, subjectID, true
}
