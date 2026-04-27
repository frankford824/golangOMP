package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type AuditHandler struct {
	svc service.AuditService
}

func NewAuditHandler(svc service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

type submitAuditReq struct {
	// ActionID is a client-generated UUID used as idempotency key (spec §7.3).
	ActionID       string               `json:"action_id"        binding:"required"`
	AssetVersionID int64                `json:"asset_version_id" binding:"required"` // CAS guard
	WholeHash      string               `json:"whole_hash"       binding:"required"` // CAS guard
	Stage          domain.AuditStage    `json:"stage"            binding:"required"`
	Decision       domain.AuditDecision `json:"decision"         binding:"required"`
	Reason         *string              `json:"reason"`
}

// Submit handles POST /v1/audit (idempotent via action_id).
func (h *AuditHandler) Submit(c *gin.Context) {
	var req submitAuditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.Submit(c.Request.Context(), service.AuditSubmitParams{
		ActionID:       req.ActionID,
		AssetVersionID: req.AssetVersionID,
		WholeHash:      req.WholeHash,
		Stage:          req.Stage,
		Decision:       req.Decision,
		Reason:         req.Reason,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}
