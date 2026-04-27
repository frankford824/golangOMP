package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type CodeRuleHandler struct {
	svc service.CodeRuleService
}

func NewCodeRuleHandler(svc service.CodeRuleService) *CodeRuleHandler {
	return &CodeRuleHandler{svc: svc}
}

// List handles GET /v1/code-rules
func (h *CodeRuleHandler) List(c *gin.Context) {
	rules, appErr := h.svc.List(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, rules)
}

// Preview handles GET /v1/code-rules/:id/preview
func (h *CodeRuleHandler) Preview(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid id", nil))
		return
	}
	preview, appErr := h.svc.Preview(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, preview)
}

type generateSKUReq struct {
	RuleID int64 `json:"rule_id" binding:"required"`
}

// GenerateSKU handles POST /v1/code-rules/generate-sku
func (h *CodeRuleHandler) GenerateSKU(c *gin.Context) {
	var req generateSKUReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	sku, appErr := h.svc.GenerateSKU(c.Request.Context(), req.RuleID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"sku_code": sku})
}
