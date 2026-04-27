package handler

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type ExportCenterHandler struct {
	svc service.ExportCenterService
}

func NewExportCenterHandler(svc service.ExportCenterService) *ExportCenterHandler {
	return &ExportCenterHandler{svc: svc}
}

type createExportJobReq struct {
	ExportType        string                            `json:"export_type" binding:"required"`
	TemplateKey       string                            `json:"template_key"`
	SourceQueryType   string                            `json:"source_query_type" binding:"required"`
	SourceFilters     createExportJobSourceFiltersReq   `json:"source_filters"`
	QueryTemplate     *domain.TaskQueryTemplate         `json:"query_template"`
	NormalizedFilters *domain.TaskQueryFilterDefinition `json:"normalized_filters"`
	Remark            string                            `json:"remark"`
}

type advanceExportJobReq struct {
	Action         string  `json:"action" binding:"required"`
	ResultFileName string  `json:"result_file_name"`
	ResultMimeType string  `json:"result_mime_type"`
	ExpiresAt      *string `json:"expires_at"`
	FailureReason  string  `json:"failure_reason"`
	Remark         string  `json:"remark"`
}

type createExportDispatchReq struct {
	TriggerSource string  `json:"trigger_source"`
	ExpiresAt     *string `json:"expires_at"`
	Remark        string  `json:"remark"`
}

type advanceExportDispatchReq struct {
	Action string `json:"action" binding:"required"`
	Reason string `json:"reason"`
	Remark string `json:"remark"`
}

type createExportJobSourceFiltersReq struct {
	QueueKey   string `json:"queue_key"`
	BoardView  string `json:"board_view"`
	TaskID     *int64 `json:"task_id"`
	ReceiverID *int64 `json:"receiver_id"`
	Status     string `json:"status"`
}

func (r createExportJobSourceFiltersReq) toDomain() domain.ExportSourceFilters {
	return domain.ExportSourceFilters{
		QueueKey:   strings.TrimSpace(r.QueueKey),
		BoardView:  domain.TaskBoardView(strings.TrimSpace(r.BoardView)),
		TaskID:     r.TaskID,
		ReceiverID: r.ReceiverID,
		Status:     strings.TrimSpace(r.Status),
	}
}

func (h *ExportCenterHandler) ListTemplates(c *gin.Context) {
	templates, appErr := h.svc.ListTemplates(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, templates)
}

func (h *ExportCenterHandler) CreateJob(c *gin.Context) {
	var req createExportJobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	job, appErr := h.svc.CreateJob(c.Request.Context(), service.CreateExportJobParams{
		ExportType:        domain.ExportType(req.ExportType),
		TemplateKey:       req.TemplateKey,
		SourceQueryType:   domain.ExportSourceQueryType(req.SourceQueryType),
		SourceFilters:     req.SourceFilters.toDomain(),
		QueryTemplate:     req.QueryTemplate,
		NormalizedFilters: req.NormalizedFilters,
		Remark:            req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, job)
}

func (h *ExportCenterHandler) CreateJobDispatch(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid export job id", nil))
		return
	}
	var req createExportDispatchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil && strings.TrimSpace(*req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "expires_at must be RFC3339", nil))
			return
		}
		expiresAt = &parsed
	}
	dispatch, appErr := h.svc.CreateJobDispatch(c.Request.Context(), id, service.CreateExportDispatchParams{
		TriggerSource: strings.TrimSpace(req.TriggerSource),
		ExpiresAt:     expiresAt,
		Remark:        strings.TrimSpace(req.Remark),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, dispatch)
}

func (h *ExportCenterHandler) AdvanceJobDispatch(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid export job id", nil))
		return
	}
	dispatchID := strings.TrimSpace(c.Param("dispatch_id"))
	if dispatchID == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid dispatch id", nil))
		return
	}
	var req advanceExportDispatchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	dispatch, appErr := h.svc.AdvanceJobDispatch(c.Request.Context(), id, dispatchID, service.AdvanceExportDispatchParams{
		Action: strings.TrimSpace(req.Action),
		Reason: strings.TrimSpace(req.Reason),
		Remark: strings.TrimSpace(req.Remark),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, dispatch)
}

func (h *ExportCenterHandler) AdvanceJob(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid export job id", nil))
		return
	}

	var req advanceExportJobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && strings.TrimSpace(*req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "expires_at must be RFC3339", nil))
			return
		}
		expiresAt = &parsed
	}

	job, appErr := h.svc.AdvanceJob(c.Request.Context(), id, service.AdvanceExportJobParams{
		Action:         domain.ExportJobAdvanceAction(strings.TrimSpace(req.Action)),
		ResultFileName: strings.TrimSpace(req.ResultFileName),
		ResultMimeType: strings.TrimSpace(req.ResultMimeType),
		ExpiresAt:      expiresAt,
		FailureReason:  strings.TrimSpace(req.FailureReason),
		Remark:         strings.TrimSpace(req.Remark),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, job)
}

func (h *ExportCenterHandler) StartJob(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid export job id", nil))
		return
	}
	job, appErr := h.svc.StartJob(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, job)
}

func (h *ExportCenterHandler) ListJobs(c *gin.Context) {
	filter := service.ExportJobFilter{}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		status := domain.ExportJobStatus(raw)
		filter.Status = &status
	}
	if raw := strings.TrimSpace(c.Query("source_query_type")); raw != "" {
		sourceType := domain.ExportSourceQueryType(raw)
		filter.SourceQueryType = &sourceType
	}
	if raw := strings.TrimSpace(c.Query("requested_by_id")); raw != "" {
		id, err := parseInt64(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "requested_by_id must be an integer", nil))
			return
		}
		filter.RequestedByID = &id
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

	jobs, pagination, appErr := h.svc.ListJobs(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, jobs, pagination)
}

func (h *ExportCenterHandler) GetJob(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid export job id", nil))
		return
	}
	job, appErr := h.svc.GetJob(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, job)
}

func (h *ExportCenterHandler) ListJobAttempts(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid export job id", nil))
		return
	}
	attempts, appErr := h.svc.ListJobAttempts(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, attempts)
}

func (h *ExportCenterHandler) ListJobDispatches(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid export job id", nil))
		return
	}
	dispatches, appErr := h.svc.ListJobDispatches(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, dispatches)
}

func (h *ExportCenterHandler) ListJobEvents(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid export job id", nil))
		return
	}
	events, appErr := h.svc.ListJobEvents(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, events)
}

func (h *ExportCenterHandler) ClaimDownload(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid export job id", nil))
		return
	}
	handoff, appErr := h.svc.ClaimDownload(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, handoff)
}

func (h *ExportCenterHandler) ReadDownload(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid export job id", nil))
		return
	}
	handoff, appErr := h.svc.ReadDownload(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, handoff)
}

func (h *ExportCenterHandler) RefreshDownload(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid export job id", nil))
		return
	}
	handoff, appErr := h.svc.RefreshDownload(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, handoff)
}
