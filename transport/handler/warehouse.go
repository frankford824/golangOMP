package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type WarehouseHandler struct {
	svc service.WarehouseService
}

func NewWarehouseHandler(svc service.WarehouseService) *WarehouseHandler {
	return &WarehouseHandler{svc: svc}
}

type warehouseActionReq struct {
	ReceiverID     *int64 `json:"receiver_id"`
	RejectReason   string `json:"reject_reason"`
	RejectCategory string `json:"reject_category"`
	Remark         string `json:"remark"`
}

// List handles GET /v1/warehouse/receipts
func (h *WarehouseHandler) List(c *gin.Context) {
	filter := service.WarehouseFilter{
		Status:       c.Query("status"),
		WorkflowLane: c.Query("workflow_lane"),
	}
	if raw := c.Query("task_id"); raw != "" {
		if id, _ := parseInt64(raw); id > 0 {
			filter.TaskID = &id
		}
	}
	if raw := c.Query("receiver_id"); raw != "" {
		if id, _ := parseInt64(raw); id > 0 {
			filter.ReceiverID = &id
		}
	}
	filter.Page, _ = parseInt(c.Query("page"))
	filter.PageSize, _ = parseInt(c.Query("page_size"))

	receipts, pagination, appErr := h.svc.List(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, receipts, pagination)
}

// Receive handles POST /v1/tasks/:id/warehouse/receive
func (h *WarehouseHandler) Receive(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req warehouseActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	receiverID, appErr := actorIDOrRequestValue(c, req.ReceiverID, "receiver_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	receipt, appErr := h.svc.Receive(c.Request.Context(), service.ReceiveWarehouseParams{
		TaskID:     taskID,
		ReceiverID: receiverID,
		Remark:     req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, receipt)
}

// Reject handles POST /v1/tasks/:id/warehouse/reject
func (h *WarehouseHandler) Reject(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req warehouseActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	receiverID, appErr := actorIDOrRequestValue(c, req.ReceiverID, "receiver_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	receipt, appErr := h.svc.Reject(c.Request.Context(), service.RejectWarehouseParams{
		TaskID:         taskID,
		ReceiverID:     receiverID,
		RejectReason:   req.RejectReason,
		RejectCategory: req.RejectCategory,
		Remark:         req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, receipt)
}

// Complete handles POST /v1/tasks/:id/warehouse/complete
func (h *WarehouseHandler) Complete(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req warehouseActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	receiverID, appErr := actorIDOrRequestValue(c, req.ReceiverID, "receiver_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	receipt, appErr := h.svc.Complete(c.Request.Context(), service.CompleteWarehouseParams{
		TaskID:     taskID,
		ReceiverID: receiverID,
		Remark:     req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, receipt)
}
