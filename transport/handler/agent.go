package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type AgentHandler struct {
	svc service.AgentService
}

func NewAgentHandler(svc service.AgentService) *AgentHandler {
	return &AgentHandler{svc: svc}
}

type agentSyncReq struct {
	AgentID       string  `json:"agent_id"        binding:"required"`
	SKUCode       string  `json:"sku_code"        binding:"required"`
	FilePath      string  `json:"file_path"       binding:"required"`
	WholeHash     string  `json:"whole_hash"      binding:"required"`
	HeadChunkHash *string `json:"head_chunk_hash"` // required when file_size_bytes > 100 MB
	TailChunkHash *string `json:"tail_chunk_hash"` // required when file_size_bytes > 100 MB
	FileSizeBytes int64   `json:"file_size_bytes" binding:"required"`
	IsStable      bool    `json:"is_stable"`
	PreviewURL    *string `json:"preview_url"`
}

type pullJobReq struct {
	AgentID string `json:"agent_id" binding:"required"`
}

type heartbeatReq struct {
	AttemptID string `json:"attempt_id" binding:"required"`
}

type ackJobReq struct {
	AttemptID  string           `json:"attempt_id" binding:"required"`
	Success    bool             `json:"success"`
	Evidence   *domain.Evidence `json:"evidence"`
	FailReason *string          `json:"fail_reason"`
}

// Sync handles POST /v1/agent/sync — NAS Agent reports a stable file with hash.
func (h *AgentHandler) Sync(c *gin.Context) {
	var req agentSyncReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.Sync(c.Request.Context(), service.AgentSyncParams{
		AgentID:       req.AgentID,
		SKUCode:       req.SKUCode,
		FilePath:      req.FilePath,
		WholeHash:     req.WholeHash,
		HeadChunkHash: req.HeadChunkHash,
		TailChunkHash: req.TailChunkHash,
		FileSizeBytes: req.FileSizeBytes,
		IsStable:      req.IsStable,
		PreviewURL:    req.PreviewURL,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

// PullJob handles POST /v1/agent/pull_job — Agent claims a pending job and receives a lease.
func (h *AgentHandler) PullJob(c *gin.Context) {
	var req pullJobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.PullJob(c.Request.Context(), req.AgentID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

// Heartbeat handles POST /v1/agent/heartbeat — renews the lease for an active attempt.
func (h *AgentHandler) Heartbeat(c *gin.Context) {
	var req heartbeatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.Heartbeat(c.Request.Context(), req.AttemptID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

// AckJob handles POST /v1/agent/ack_job — Agent reports job completion with evidence.
func (h *AgentHandler) AckJob(c *gin.Context) {
	var req ackJobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	appErr := h.svc.AckJob(c.Request.Context(), service.AckJobParams{
		AttemptID:  req.AttemptID,
		Success:    req.Success,
		Evidence:   req.Evidence,
		FailReason: req.FailReason,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, nil)
}
