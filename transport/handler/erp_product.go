package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	erpproductsvc "workflow/service/erp_product"
)

type ERPProductHandler struct {
	svc *erpproductsvc.Service
}

func NewERPProductHandler(svc *erpproductsvc.Service) *ERPProductHandler {
	return &ERPProductHandler{svc: svc}
}

func (h *ERPProductHandler) ByCode(c *gin.Context) {
	if h == nil || h.svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "erp product service is not configured", nil))
		return
	}
	snapshot, appErr := h.svc.LookupByCode(c.Request.Context(), c.Query("code"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, snapshot)
}
