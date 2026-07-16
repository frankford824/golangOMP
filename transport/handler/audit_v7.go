package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

// AuditV7Handler handles V7 task-centric audit and task event endpoints.
type AuditV7Handler struct {
	auditSvc service.AuditV7Service
	eventSvc service.TaskEventService
}

func NewAuditV7Handler(auditSvc service.AuditV7Service, eventSvc service.TaskEventService) *AuditV7Handler {
	return &AuditV7Handler{auditSvc: auditSvc, eventSvc: eventSvc}
}

// ── GET /v1/tasks/audit/handover-candidates ─────────────────────────────────

func (h *AuditV7Handler) ListHandoverCandidates(c *gin.Context) {
	filter, appErr := parseAuditHandoverCandidateQuery(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.auditSvc.ListHandoverCandidates(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

// ── POST /v1/tasks/audit/handover-batch ─────────────────────────────────────

func (h *AuditV7Handler) BatchHandover(c *gin.Context) {
	var req service.BatchAuditHandoverParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.auditSvc.BatchHandover(c.Request.Context(), req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func parseAuditHandoverCandidateQuery(c *gin.Context) (service.AuditHandoverCandidateFilter, *domain.AppError) {
	page, appErr := parseAuditHandoverCandidateIntQuery(c, "page")
	if appErr != nil {
		return service.AuditHandoverCandidateFilter{}, appErr
	}
	pageSize, appErr := parseAuditHandoverCandidateIntQuery(c, "page_size")
	if appErr != nil {
		return service.AuditHandoverCandidateFilter{}, appErr
	}
	return service.AuditHandoverCandidateFilter{
		Keyword:      strings.TrimSpace(c.Query("keyword")),
		Status:       domain.TaskStatus(strings.TrimSpace(c.Query("status"))),
		OwnerOrgTeam: strings.TrimSpace(c.Query("owner_org_team")),
		Page:         page,
		PageSize:     pageSize,
	}, nil
}

func parseAuditHandoverCandidateIntQuery(c *gin.Context, name string) (int, *domain.AppError) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, name+" must be a positive integer", nil)
	}
	return value, nil
}

// ── POST /v1/tasks/:id/audit/handover ────────────────────────────────────────

type handoverAuditReq struct {
	FromAuditorID    *int64 `json:"from_auditor_id"`
	ToAuditorID      int64  `json:"to_auditor_id"      binding:"required"`
	Reason           string `json:"reason"             binding:"required"`
	CurrentJudgement string `json:"current_judgement"`
	RiskRemark       string `json:"risk_remark"`
}

func (h *AuditV7Handler) Handover(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req handoverAuditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	fromAuditorID, appErr := actorIDOrRequestValue(c, req.FromAuditorID, "from_auditor_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	handover, appErr := h.auditSvc.Handover(c.Request.Context(), service.HandoverAuditParams{
		TaskID:           taskID,
		FromAuditorID:    fromAuditorID,
		ToAuditorID:      req.ToAuditorID,
		Reason:           req.Reason,
		CurrentJudgement: req.CurrentJudgement,
		RiskRemark:       req.RiskRemark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, handover)
}

// ListHandovers handles GET /v1/tasks/:id/audit/handovers
func (h *AuditV7Handler) ListHandovers(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	handovers, appErr := h.auditSvc.ListHandovers(c.Request.Context(), taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, handovers)
}

// ── POST /v1/tasks/:id/audit/takeover ────────────────────────────────────────

type takeoverAuditReq struct {
	HandoverID int64  `json:"handover_id" binding:"required"`
	AuditorID  *int64 `json:"auditor_id"`
}

func (h *AuditV7Handler) Takeover(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req takeoverAuditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	auditorID, appErr := actorIDOrRequestValue(c, req.AuditorID, "auditor_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	if appErr := h.auditSvc.Takeover(c.Request.Context(), taskID, req.HandoverID, auditorID); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"task_id": taskID, "handover_id": req.HandoverID, "action": "taken_over"})
}

// ── GET /v1/tasks/:id/events ──────────────────────────────────────────────────

func (h *AuditV7Handler) ListEvents(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	events, appErr := h.eventSvc.ListByTaskID(c.Request.Context(), taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, events)
}
