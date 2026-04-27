package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

// OutsourceHandler handles outsource order endpoints.
type OutsourceHandler struct {
	svc service.OutsourceService
}

func NewOutsourceHandler(svc service.OutsourceService) *OutsourceHandler {
	return &OutsourceHandler{svc: svc}
}

// ── POST /v1/tasks/:id/outsource ─────────────────────────────────────────────

type createOutsourceReq struct {
	OperatorID          int64  `json:"operator_id"          binding:"required"`
	VendorName          string `json:"vendor_name"          binding:"required"`
	OutsourceType       string `json:"outsource_type"       binding:"required"`
	DeliveryRequirement string `json:"delivery_requirement"`
	SettlementNote      string `json:"settlement_note"`
}

func (h *OutsourceHandler) Create(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req createOutsourceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	order, appErr := h.svc.Create(c.Request.Context(), service.CreateOutsourceParams{
		TaskID:              taskID,
		OperatorID:          req.OperatorID,
		VendorName:          req.VendorName,
		OutsourceType:       req.OutsourceType,
		DeliveryRequirement: req.DeliveryRequirement,
		SettlementNote:      req.SettlementNote,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, order)
}

// ── GET /v1/outsource-orders ──────────────────────────────────────────────────

func (h *OutsourceHandler) List(c *gin.Context) {
	filter := service.OutsourceFilter{
		Status: c.Query("status"),
		Vendor: c.Query("vendor"),
	}
	if raw := c.Query("task_id"); raw != "" {
		if id, _ := parseInt64(raw); id > 0 {
			filter.TaskID = &id
		}
	}
	filter.Page, _ = parseInt(c.Query("page"))
	filter.PageSize, _ = parseInt(c.Query("page_size"))

	orders, pagination, appErr := h.svc.List(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, orders, pagination)
}

type returnOutsourceReq struct {
	OperatorID int64  `json:"operator_id" binding:"required"`
	Remark     string `json:"remark"`
}

func (h *OutsourceHandler) Return(c *gin.Context) {
	orderID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid outsource order id", nil))
		return
	}
	var req returnOutsourceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	order, appErr := h.svc.Return(c.Request.Context(), service.ReturnOutsourceParams{
		OrderID:    orderID,
		OperatorID: req.OperatorID,
		Remark:     req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, order)
}

type reviewOutsourceReq struct {
	ReviewerID int64    `json:"reviewer_id" binding:"required"`
	Result     string   `json:"result"      binding:"required"`
	Comment    string   `json:"comment"`
	IssueTypes []string `json:"issue_types"`
}

func (h *OutsourceHandler) Review(c *gin.Context) {
	orderID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid outsource order id", nil))
		return
	}
	var req reviewOutsourceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	order, appErr := h.svc.Review(c.Request.Context(), service.ReviewOutsourceParams{
		OrderID:    orderID,
		ReviewerID: req.ReviewerID,
		Result:     req.Result,
		Comment:    req.Comment,
		IssueTypes: req.IssueTypes,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, order)
}
