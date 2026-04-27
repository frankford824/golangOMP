package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	r3module "workflow/service/module_action"
	"workflow/service/task_cancel"
)

type moduleClaimReq struct {
	ConfirmPoolTeamCode string `json:"confirm_pool_team_code"`
}

type moduleReassignReq struct {
	ActorID      *int64 `json:"actor_id"`
	AssigneeID   int64  `json:"assignee_id"`
	TeamCode     string `json:"team_code"`
	PoolTeamCode string `json:"pool_team_code"`
}

type taskCancelReq struct {
	Reason string `json:"reason" binding:"required"`
	Force  bool   `json:"force"`
}

func (h *TaskHandler) Pool(c *gin.Context) {
	if h.poolQuerySvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "R3 pool service is not configured", nil))
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	items, err := h.poolQuerySvc.List(c.Request.Context(), actor, c.Query("module_key"), c.Query("pool_team_code"), limit, offset)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil))
		return
	}
	respondOK(c, items)
}

func (h *TaskHandler) ModuleClaim(c *gin.Context) {
	if h.claimSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "R3 claim service is not configured", nil))
		return
	}
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req moduleClaimReq
	_ = c.ShouldBindJSON(&req)
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	dec := h.claimSvc.Claim(c.Request.Context(), actor, taskID, c.Param("module_key"), req.ConfirmPoolTeamCode)
	if !dec.OK {
		respondModuleDecision(c, dec.DenyCode, dec.Message)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"task_id": taskID, "module_key": c.Param("module_key"), "action": "claimed"}})
}

func (h *TaskHandler) ModuleAction(c *gin.Context) {
	if h.moduleSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "R3 module service is not configured", nil))
		return
	}
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var raw json.RawMessage
	_ = c.ShouldBindJSON(&raw)
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	dec := h.moduleSvc.Apply(c.Request.Context(), r3module.ActionRequest{
		Actor: actor, TaskID: taskID, ModuleKey: c.Param("module_key"), Action: c.Param("action"), Payload: raw,
	})
	if !dec.OK {
		respondModuleDecision(c, dec.DenyCode, dec.Message)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"task_id": taskID, "module_key": c.Param("module_key"), "action": c.Param("action")}})
}

func (h *TaskHandler) ModuleReassign(c *gin.Context) {
	h.moduleAdminAction(c, domain.ModuleActionReassign)
}

func (h *TaskHandler) ModulePoolReassign(c *gin.Context) {
	h.moduleAdminAction(c, domain.ModuleActionPoolReassign)
}

func (h *TaskHandler) moduleAdminAction(c *gin.Context, action string) {
	if h.moduleSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "R3 module service is not configured", nil))
		return
	}
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var raw json.RawMessage
	_ = c.ShouldBindJSON(&raw)
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	dec := h.moduleSvc.Apply(c.Request.Context(), r3module.ActionRequest{Actor: actor, TaskID: taskID, ModuleKey: c.Param("module_key"), Action: action, Payload: raw})
	if !dec.OK {
		respondModuleDecision(c, dec.DenyCode, dec.Message)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"task_id": taskID, "module_key": c.Param("module_key"), "action": action}})
}

func (h *TaskHandler) CancelR3(c *gin.Context) {
	if h.cancelSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "R3 cancel service is not configured", nil))
		return
	}
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req taskCancelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	dec := h.cancelSvc.Cancel(c.Request.Context(), task_cancel.Request{Actor: actor, TaskID: taskID, Reason: req.Reason, Force: req.Force})
	if !dec.OK {
		respondModuleDecision(c, dec.DenyCode, dec.Message)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"task_id": taskID, "cancelled": true, "force": req.Force}})
}

func respondModuleDecision(c *gin.Context, code, message string) {
	status := http.StatusForbidden
	switch code {
	case domain.DenyModuleClaimConflict, "task_already_claimed":
		status = http.StatusConflict
	case "task_not_found":
		status = http.StatusNotFound
	case domain.ErrCodeReasonRequired, domain.ErrCodeInvalidRequest:
		status = http.StatusBadRequest
	case domain.ErrCodeInternalError:
		status = http.StatusInternalServerError
	}
	if message == "" {
		message = code
	}
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message, "trace_id": c.GetString("trace_id")}})
}
