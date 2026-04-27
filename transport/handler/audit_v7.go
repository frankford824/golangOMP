package handler

import (
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

// ── POST /v1/tasks/:id/audit/claim ───────────────────────────────────────────

type claimAuditReq struct {
	AuditorID *int64 `json:"auditor_id"`
	Stage     string `json:"stage"      binding:"required"`
}

func (h *AuditV7Handler) Claim(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req claimAuditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	auditorID, appErr := actorIDOrRequestValue(c, req.AuditorID, "auditor_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	if appErr := h.auditSvc.Claim(c.Request.Context(), service.ClaimAuditParams{
		TaskID:    taskID,
		AuditorID: auditorID,
		Stage:     domain.AuditRecordStage(req.Stage),
	}); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"task_id": taskID, "action": "claimed"})
}

// ── POST /v1/tasks/:id/audit/approve ─────────────────────────────────────────

type approveAuditReq struct {
	AuditorID          *int64   `json:"auditor_id"`
	Stage              string   `json:"stage"       binding:"required"`
	NextStatus         string   `json:"next_status" binding:"required"`
	Comment            string   `json:"comment"`
	IssueTypes         []string `json:"issue_types"`
	ReplacementAssetID *int64   `json:"replacement_asset_id,omitempty"`
	PreviousAssetID    *int64   `json:"previous_asset_id,omitempty"`
	ReplacementNote    string   `json:"replacement_note,omitempty"`
}

func (h *AuditV7Handler) Approve(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req approveAuditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	auditorID, appErr := actorIDOrRequestValue(c, req.AuditorID, "auditor_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	if appErr := h.auditSvc.Approve(c.Request.Context(), service.ApproveAuditParams{
		TaskID:             taskID,
		AuditorID:          auditorID,
		Stage:              domain.AuditRecordStage(req.Stage),
		NextStatus:         domain.TaskStatus(req.NextStatus),
		Comment:            req.Comment,
		IssueTypes:         req.IssueTypes,
		ReplacementAssetID: req.ReplacementAssetID,
		PreviousAssetID:    req.PreviousAssetID,
		ReplacementNote:    req.ReplacementNote,
	}); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"task_id": taskID, "action": "approved"})
}

// ── POST /v1/tasks/:id/audit/reject ──────────────────────────────────────────

type rejectAuditReq struct {
	AuditorID          *int64   `json:"auditor_id"`
	Stage              string   `json:"stage"           binding:"required"`
	Comment            string   `json:"comment"         binding:"required"`
	IssueTypes         []string `json:"issue_types"`
	AffectsLaunch      bool     `json:"affects_launch"`
	ReplacementAssetID *int64   `json:"replacement_asset_id,omitempty"`
	PreviousAssetID    *int64   `json:"previous_asset_id,omitempty"`
	ReplacementNote    string   `json:"replacement_note,omitempty"`
}

func (h *AuditV7Handler) Reject(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req rejectAuditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	auditorID, appErr := actorIDOrRequestValue(c, req.AuditorID, "auditor_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	if appErr := h.auditSvc.Reject(c.Request.Context(), service.RejectAuditParams{
		TaskID:             taskID,
		AuditorID:          auditorID,
		Stage:              domain.AuditRecordStage(req.Stage),
		Comment:            req.Comment,
		IssueTypes:         req.IssueTypes,
		AffectsLaunch:      req.AffectsLaunch,
		ReplacementAssetID: req.ReplacementAssetID,
		PreviousAssetID:    req.PreviousAssetID,
		ReplacementNote:    req.ReplacementNote,
	}); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"task_id": taskID, "action": "rejected"})
}

// ── POST /v1/tasks/:id/audit/transfer ────────────────────────────────────────

type transferAuditReq struct {
	FromAuditorID *int64 `json:"from_auditor_id"`
	ToAuditorID   int64  `json:"to_auditor_id"   binding:"required"`
	Stage         string `json:"stage"           binding:"required"`
	Comment       string `json:"comment"`
}

func (h *AuditV7Handler) Transfer(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req transferAuditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	fromAuditorID, appErr := actorIDOrRequestValue(c, req.FromAuditorID, "from_auditor_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	if appErr := h.auditSvc.Transfer(c.Request.Context(), service.TransferAuditParams{
		TaskID:        taskID,
		FromAuditorID: fromAuditorID,
		ToAuditorID:   req.ToAuditorID,
		Stage:         domain.AuditRecordStage(req.Stage),
		Comment:       req.Comment,
	}); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"task_id": taskID, "action": "transferred"})
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
