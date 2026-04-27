package handler

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type RuleTemplateHandler struct {
	svc service.RuleTemplateService
}

func NewRuleTemplateHandler(svc service.RuleTemplateService) *RuleTemplateHandler {
	return &RuleTemplateHandler{svc: svc}
}

// List handles GET /v1/rule-templates
func (h *RuleTemplateHandler) List(c *gin.Context) {
	list, appErr := h.svc.List(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, list)
}

// GetByType handles GET /v1/rule-templates/:type
func (h *RuleTemplateHandler) GetByType(c *gin.Context) {
	rawType := c.Param("type")
	if rawType == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "template type is required", nil))
		return
	}
	templateType := domain.RuleTemplateType(rawType)
	rt, appErr := h.svc.GetByType(c.Request.Context(), templateType)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, rt)
}

// Put handles PUT /v1/rule-templates/:type
func (h *RuleTemplateHandler) Put(c *gin.Context) {
	rawType := c.Param("type")
	if rawType == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "template type is required", nil))
		return
	}
	templateType := domain.RuleTemplateType(rawType)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "failed to read body", nil))
		return
	}
	configJSON := string(bytes.TrimSpace(body))
	rt, appErr := h.svc.Put(c.Request.Context(), templateType, configJSON)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, rt)
}
