package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type IncidentHandler struct {
	svc service.IncidentService
}

func NewIncidentHandler(svc service.IncidentService) *IncidentHandler {
	return &IncidentHandler{svc: svc}
}

type assignIncidentReq struct {
	AssigneeID int64  `json:"assignee_id" binding:"required"`
	Reason     string `json:"reason"      binding:"required"` // required for all incident actions
}

type resolveIncidentReq struct {
	Reason string `json:"reason" binding:"required"`
}

func (h *IncidentHandler) List(c *gin.Context) {
	incidents, appErr := h.svc.List(c.Request.Context(), service.IncidentFilter{
		Status: c.Query("status"),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, incidents)
}

func (h *IncidentHandler) Assign(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid id", nil))
		return
	}
	var req assignIncidentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	if appErr := h.svc.Assign(c.Request.Context(), id, req.AssigneeID, req.Reason); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, nil)
}

func (h *IncidentHandler) Resolve(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid id", nil))
		return
	}
	var req resolveIncidentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	if appErr := h.svc.Resolve(c.Request.Context(), id, req.Reason); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, nil)
}
