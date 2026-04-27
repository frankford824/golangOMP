package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	orgmovesvc "workflow/service/org_move_request"
)

type OrgMoveRequestHandler struct {
	svc orgmovesvc.Service
}

func NewOrgMoveRequestHandler(svc orgmovesvc.Service) *OrgMoveRequestHandler {
	return &OrgMoveRequestHandler{svc: svc}
}

type createOrgMoveReq struct {
	UserID             int64  `json:"user_id"`
	TargetDepartmentID *int64 `json:"target_department_id"`
	Reason             string `json:"reason"`
}

type rejectOrgMoveReq struct {
	Reason string `json:"reason"`
}

func (h *OrgMoveRequestHandler) Create(c *gin.Context) {
	if h == nil || h.svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "org move service is not configured", nil))
		return
	}
	sourceID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid department id", nil))
		return
	}
	var req createOrgMoveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	item, appErr := h.svc.Create(c.Request.Context(), actor, sourceID, orgmovesvc.CreateParams{
		UserID:             req.UserID,
		TargetDepartmentID: req.TargetDepartmentID,
		Reason:             req.Reason,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, item)
}

func (h *OrgMoveRequestHandler) List(c *gin.Context) {
	if h == nil || h.svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "org move service is not configured", nil))
		return
	}
	var state *domain.OrgMoveRequestState
	if raw := c.Query("state"); raw != "" {
		value := domain.OrgMoveRequestState(raw)
		state = &value
	}
	var userID *int64
	if raw := c.Query("user_id"); raw != "" {
		if id, err := parseInt64(raw); err == nil {
			userID = &id
		}
	}
	var sourceDepartmentID *int64
	if raw := c.Query("source_department_id"); raw != "" {
		if id, err := parseInt64(raw); err == nil {
			sourceDepartmentID = &id
		}
	}
	page, _ := parseInt(c.Query("page"))
	pageSize, _ := parseInt(c.Query("page_size"))
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	items, meta, appErr := h.svc.List(c.Request.Context(), actor, orgmovesvc.ListFilter{
		State:              state,
		UserID:             userID,
		SourceDepartmentID: sourceDepartmentID,
		Page:               page,
		PageSize:           pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, meta)
}

func (h *OrgMoveRequestHandler) Approve(c *gin.Context) {
	if h == nil || h.svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "org move service is not configured", nil))
		return
	}
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid request id", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	if appErr := h.svc.Approve(c.Request.Context(), actor, id); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *OrgMoveRequestHandler) Reject(c *gin.Context) {
	if h == nil || h.svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "org move service is not configured", nil))
		return
	}
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid request id", nil))
		return
	}
	var req rejectOrgMoveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	if appErr := h.svc.Reject(c.Request.Context(), actor, id, req.Reason); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(http.StatusNoContent)
}
