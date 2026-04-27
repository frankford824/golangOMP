package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type SKUHandler struct {
	svc service.SKUService
}

func NewSKUHandler(svc service.SKUService) *SKUHandler {
	return &SKUHandler{svc: svc}
}

type createSKUReq struct {
	SKUCode string `json:"sku_code" binding:"required"`
	Name    string `json:"name"     binding:"required"`
}

func (h *SKUHandler) List(c *gin.Context) {
	skus, appErr := h.svc.List(c.Request.Context(), service.SKUFilter{
		WorkflowStatus: c.Query("workflow_status"),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, skus)
}

func (h *SKUHandler) Create(c *gin.Context) {
	var req createSKUReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	sku, appErr := h.svc.Create(c.Request.Context(), req.SKUCode, req.Name)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, sku)
}

func (h *SKUHandler) GetByID(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid id", nil))
		return
	}
	sku, appErr := h.svc.GetByID(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, sku)
}

// SyncStatus handles GET /v1/sku/:id/sync_status?since_sequence=N
// Called by the frontend when a sequence gap is detected (spec §5.2 invariant 9).
func (h *SKUHandler) SyncStatus(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid id", nil))
		return
	}
	sinceSeq, _ := strconv.ParseInt(c.Query("since_sequence"), 10, 64)
	result, appErr := h.svc.SyncStatus(c.Request.Context(), id, sinceSeq)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *SKUHandler) PreviewCode(c *gin.Context) {
	// TODO: implement preview_code generation
	respondOK(c, nil)
}

func parseID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}

func parseInt(s string) (int, error) {
	v, err := strconv.Atoi(s)
	return v, err
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func parseBool(s string) (bool, error) {
	switch s {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool: %s", s)
	}
}
