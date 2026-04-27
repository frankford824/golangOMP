package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/service"
)

type ERPSyncHandler struct {
	svc service.ERPSyncService
}

func NewERPSyncHandler(svc service.ERPSyncService) *ERPSyncHandler {
	return &ERPSyncHandler{svc: svc}
}

// Status handles GET /v1/products/sync/status.
func (h *ERPSyncHandler) Status(c *gin.Context) {
	status, appErr := h.svc.GetStatus(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, status)
}

// Run handles POST /v1/products/sync/run.
func (h *ERPSyncHandler) Run(c *gin.Context) {
	result, appErr := h.svc.RunManual(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}
