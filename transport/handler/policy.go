package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type PolicyHandler struct {
	svc service.PolicyService
}

func NewPolicyHandler(svc service.PolicyService) *PolicyHandler {
	return &PolicyHandler{svc: svc}
}

type updatePolicyReq struct {
	Value  string `json:"value"  binding:"required"` // JSON-encoded policy value
	Reason string `json:"reason" binding:"required"` // REQUIRED for all policy changes (dangerous action)
}

func (h *PolicyHandler) List(c *gin.Context) {
	policies, appErr := h.svc.ListAll(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, policies)
}

// Update handles PUT /v1/policies/:id (Admin only).
func (h *PolicyHandler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid id", nil))
		return
	}
	var req updatePolicyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	if appErr := h.svc.Update(c.Request.Context(), id, req.Value, req.Reason); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, nil)
}
