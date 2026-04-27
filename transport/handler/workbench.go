package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type WorkbenchHandler struct {
	svc service.WorkbenchService
}

func NewWorkbenchHandler(svc service.WorkbenchService) *WorkbenchHandler {
	return &WorkbenchHandler{svc: svc}
}

func (h *WorkbenchHandler) GetPreferences(c *gin.Context) {
	result, appErr := h.svc.GetPreferences(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *WorkbenchHandler) PatchPreferences(c *gin.Context) {
	var patch domain.WorkbenchPreferencesPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	result, appErr := h.svc.PatchPreferences(c.Request.Context(), patch)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}
