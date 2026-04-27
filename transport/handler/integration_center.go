package handler

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type IntegrationCenterHandler struct {
	svc service.IntegrationCenterService
}

func NewIntegrationCenterHandler(svc service.IntegrationCenterService) *IntegrationCenterHandler {
	return &IntegrationCenterHandler{svc: svc}
}

type createIntegrationCallLogReq struct {
	ConnectorKey   string          `json:"connector_key" binding:"required"`
	OperationKey   string          `json:"operation_key" binding:"required"`
	Direction      string          `json:"direction" binding:"required"`
	ResourceType   string          `json:"resource_type"`
	ResourceID     *int64          `json:"resource_id"`
	RequestPayload json.RawMessage `json:"request_payload"`
	Remark         string          `json:"remark"`
}

type advanceIntegrationCallLogReq struct {
	Status          string          `json:"status" binding:"required"`
	ResponsePayload json.RawMessage `json:"response_payload"`
	ErrorMessage    string          `json:"error_message"`
	Remark          string          `json:"remark"`
}

type createIntegrationExecutionReq struct {
	ExecutionMode string `json:"execution_mode"`
	TriggerSource string `json:"trigger_source"`
	AdapterNote   string `json:"adapter_note"`
}

type integrationExecutionActionReq struct {
	ExecutionMode string `json:"execution_mode"`
	AdapterNote   string `json:"adapter_note"`
}

type advanceIntegrationExecutionReq struct {
	Status          string          `json:"status" binding:"required"`
	ResponsePayload json.RawMessage `json:"response_payload"`
	ErrorMessage    string          `json:"error_message"`
	AdapterNote     string          `json:"adapter_note"`
	Retryable       *bool           `json:"retryable"`
}

func (h *IntegrationCenterHandler) ListConnectors(c *gin.Context) {
	connectors, appErr := h.svc.ListConnectors(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, connectors)
}

func (h *IntegrationCenterHandler) CreateCallLog(c *gin.Context) {
	var req createIntegrationCallLogReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	log, appErr := h.svc.CreateCallLog(c.Request.Context(), service.CreateIntegrationCallLogParams{
		ConnectorKey:   domain.IntegrationConnectorKey(strings.TrimSpace(req.ConnectorKey)),
		OperationKey:   strings.TrimSpace(req.OperationKey),
		Direction:      domain.IntegrationCallDirection(strings.TrimSpace(req.Direction)),
		ResourceType:   strings.TrimSpace(req.ResourceType),
		ResourceID:     req.ResourceID,
		RequestPayload: req.RequestPayload,
		Remark:         strings.TrimSpace(req.Remark),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, log)
}

func (h *IntegrationCenterHandler) AdvanceCallLog(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid integration call log id", nil))
		return
	}
	var req advanceIntegrationCallLogReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	log, appErr := h.svc.AdvanceCallLog(c.Request.Context(), id, service.AdvanceIntegrationCallLogParams{
		Status:          domain.IntegrationCallStatus(strings.TrimSpace(req.Status)),
		ResponsePayload: req.ResponsePayload,
		ErrorMessage:    strings.TrimSpace(req.ErrorMessage),
		Remark:          strings.TrimSpace(req.Remark),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, log)
}

func (h *IntegrationCenterHandler) ListCallLogs(c *gin.Context) {
	filter := service.IntegrationCallLogFilter{}
	if raw := strings.TrimSpace(c.Query("connector_key")); raw != "" {
		key := domain.IntegrationConnectorKey(raw)
		filter.ConnectorKey = &key
	}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		status := domain.IntegrationCallStatus(raw)
		filter.Status = &status
	}
	filter.ResourceType = strings.TrimSpace(c.Query("resource_type"))
	if raw := strings.TrimSpace(c.Query("resource_id")); raw != "" {
		resourceID, err := parseInt64(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "resource_id must be an integer", nil))
			return
		}
		filter.ResourceID = &resourceID
	}
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		page, err := parseInt(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be an integer", nil))
			return
		}
		filter.Page = page
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		pageSize, err := parseInt(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil))
			return
		}
		filter.PageSize = pageSize
	}
	logs, pagination, appErr := h.svc.ListCallLogs(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, logs, pagination)
}

func (h *IntegrationCenterHandler) GetCallLog(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid integration call log id", nil))
		return
	}
	log, appErr := h.svc.GetCallLog(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, log)
}

func (h *IntegrationCenterHandler) CreateExecution(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid integration call log id", nil))
		return
	}
	var req createIntegrationExecutionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	execution, appErr := h.svc.CreateExecution(c.Request.Context(), id, service.CreateIntegrationExecutionParams{
		ExecutionMode: domain.IntegrationExecutionMode(strings.TrimSpace(req.ExecutionMode)),
		TriggerSource: strings.TrimSpace(req.TriggerSource),
		AdapterNote:   strings.TrimSpace(req.AdapterNote),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, execution)
}

func (h *IntegrationCenterHandler) ListExecutions(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid integration call log id", nil))
		return
	}
	executions, appErr := h.svc.ListExecutions(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, executions)
}

func (h *IntegrationCenterHandler) RetryCallLog(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid integration call log id", nil))
		return
	}
	var req integrationExecutionActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	log, appErr := h.svc.RetryCallLog(c.Request.Context(), id, service.RetryIntegrationCallLogParams{
		ExecutionMode: domain.IntegrationExecutionMode(strings.TrimSpace(req.ExecutionMode)),
		AdapterNote:   strings.TrimSpace(req.AdapterNote),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, log)
}

func (h *IntegrationCenterHandler) ReplayCallLog(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid integration call log id", nil))
		return
	}
	var req integrationExecutionActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	log, appErr := h.svc.ReplayCallLog(c.Request.Context(), id, service.ReplayIntegrationCallLogParams{
		ExecutionMode: domain.IntegrationExecutionMode(strings.TrimSpace(req.ExecutionMode)),
		AdapterNote:   strings.TrimSpace(req.AdapterNote),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, log)
}

func (h *IntegrationCenterHandler) AdvanceExecution(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid integration call log id", nil))
		return
	}
	executionID := strings.TrimSpace(c.Param("execution_id"))
	if executionID == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid integration execution id", nil))
		return
	}
	var req advanceIntegrationExecutionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	execution, appErr := h.svc.AdvanceExecution(c.Request.Context(), id, executionID, service.AdvanceIntegrationExecutionParams{
		Status:          domain.IntegrationExecutionStatus(strings.TrimSpace(req.Status)),
		ResponsePayload: req.ResponsePayload,
		ErrorMessage:    strings.TrimSpace(req.ErrorMessage),
		AdapterNote:     strings.TrimSpace(req.AdapterNote),
		Retryable:       req.Retryable,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, execution)
}
