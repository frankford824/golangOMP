package handler

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type upsertTaskCostOverrideReviewReq struct {
	ReviewRequired *bool  `json:"review_required"`
	ReviewStatus   string `json:"review_status"`
	ReviewNote     string `json:"review_note"`
	ReviewActor    string `json:"review_actor"`
	ReviewedAt     string `json:"reviewed_at"`
}

type upsertTaskCostFinanceFlagReq struct {
	FinanceRequired *bool  `json:"finance_required"`
	FinanceStatus   string `json:"finance_status"`
	FinanceNote     string `json:"finance_note"`
	FinanceMarkedBy string `json:"finance_marked_by"`
	FinanceMarkedAt string `json:"finance_marked_at"`
}

type TaskCostOverrideHandler struct {
	svc service.TaskCostOverrideAuditService
}

func NewTaskCostOverrideHandler(svc service.TaskCostOverrideAuditService) *TaskCostOverrideHandler {
	return &TaskCostOverrideHandler{svc: svc}
}

func (h *TaskCostOverrideHandler) ListByTaskID(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}

	timeline, appErr := h.svc.ListByTaskID(c.Request.Context(), taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, timeline)
}

func (h *TaskCostOverrideHandler) UpsertReview(c *gin.Context) {
	taskID, eventID, ok := parseTaskCostOverridePath(c)
	if !ok {
		return
	}

	var req upsertTaskCostOverrideReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	reviewedAt, appErr := parseOptionalRFC3339("reviewed_at", req.ReviewedAt)
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	result, appErr := h.svc.UpsertReview(c.Request.Context(), service.UpsertTaskCostOverrideReviewParams{
		TaskID:          taskID,
		OverrideEventID: eventID,
		ReviewRequired:  req.ReviewRequired,
		ReviewStatus:    domain.TaskCostOverrideReviewStatus(strings.TrimSpace(req.ReviewStatus)),
		ReviewNote:      req.ReviewNote,
		ReviewActor:     req.ReviewActor,
		ReviewedAt:      reviewedAt,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *TaskCostOverrideHandler) UpsertFinanceFlag(c *gin.Context) {
	taskID, eventID, ok := parseTaskCostOverridePath(c)
	if !ok {
		return
	}

	var req upsertTaskCostFinanceFlagReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	financeMarkedAt, appErr := parseOptionalRFC3339("finance_marked_at", req.FinanceMarkedAt)
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	result, appErr := h.svc.UpsertFinanceFlag(c.Request.Context(), service.UpsertTaskCostFinanceFlagParams{
		TaskID:          taskID,
		OverrideEventID: eventID,
		FinanceRequired: req.FinanceRequired,
		FinanceStatus:   domain.TaskCostOverrideFinanceStatus(strings.TrimSpace(req.FinanceStatus)),
		FinanceNote:     req.FinanceNote,
		FinanceMarkedBy: req.FinanceMarkedBy,
		FinanceMarkedAt: financeMarkedAt,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func parseTaskCostOverridePath(c *gin.Context) (int64, string, bool) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return 0, "", false
	}
	eventID := strings.TrimSpace(c.Param("event_id"))
	if eventID == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid override event id", nil))
		return 0, "", false
	}
	return taskID, eventID, true
}

func parseOptionalRFC3339(field string, raw string) (*time.Time, *domain.AppError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, field+" must be RFC3339", nil)
	}
	return &value, nil
}
