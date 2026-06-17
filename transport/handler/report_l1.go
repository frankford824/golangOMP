package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/repo"
	reportl1svc "workflow/service/report_l1"
)

type ReportL1Handler struct {
	svc      *reportl1svc.Service
	auditLog repo.PermissionLogRepo
}

func NewReportL1Handler(svc *reportl1svc.Service, auditLog repo.PermissionLogRepo) *ReportL1Handler {
	return &ReportL1Handler{svc: svc, auditLog: auditLog}
}

func (h *ReportL1Handler) Cards(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	data, appErr := h.svc.Cards(c.Request.Context(), actor)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{"data": data})
}

func (h *ReportL1Handler) Throughput(c *gin.Context) {
	from, to, deptID, taskType, appErr := parseReportRange(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	data, appErr := h.svc.Throughput(c.Request.Context(), actor, from, to, deptID, taskType)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{"data": data})
}

func (h *ReportL1Handler) ModuleDwell(c *gin.Context) {
	from, to, deptID, taskType, appErr := parseReportRange(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	data, appErr := h.svc.ModuleDwell(c.Request.Context(), actor, from, to, deptID, taskType)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{"data": data})
}

func (h *ReportL1Handler) KPIEvents(c *gin.Context) {
	from, to, _, _, appErr := parseReportRange(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	limit, _ := parseInt(c.Query("limit"))
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	data, appErr := h.svc.KPIEvents(c.Request.Context(), actor, reportl1svc.KPIEventsParams{From: from.UTC(), To: to.UTC(), Limit: limit})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{"data": data})
}

func (h *ReportL1Handler) KPIAIAnalysis(c *gin.Context) {
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "请求格式不正确", nil))
			return
		}
	}
	if strings.TrimSpace(req.From) == "" {
		req.From = c.Query("from")
	}
	if strings.TrimSpace(req.To) == "" {
		req.To = c.Query("to")
	}
	from, err := time.Parse("2006-01-02", strings.TrimSpace(req.From))
	if err != nil {
		respondError(c, domain.NewAppError(reportl1svc.CodeInvalidDateRange, "invalid from date", nil))
		return
	}
	to, err := time.Parse("2006-01-02", strings.TrimSpace(req.To))
	if err != nil {
		respondError(c, domain.NewAppError(reportl1svc.CodeInvalidDateRange, "invalid to date", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	data, appErr := h.svc.KPIAIAnalysis(c.Request.Context(), actor, reportl1svc.KPIAIAnalysisParams{From: from.UTC(), To: to.UTC()})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{"data": data})
}

func (h *ReportL1Handler) BusinessTrendPilotAnalysis(c *gin.Context) {
	var req struct {
		From    string   `json:"from"`
		To      string   `json:"to"`
		Mode    string   `json:"mode"`
		Sources []string `json:"sources"`
	}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "请求格式不正确", nil))
			return
		}
	}
	if strings.TrimSpace(req.From) == "" {
		req.From = c.Query("from")
	}
	if strings.TrimSpace(req.To) == "" {
		req.To = c.Query("to")
	}
	if strings.TrimSpace(req.Mode) == "" {
		req.Mode = c.Query("mode")
	}
	from, err := time.Parse("2006-01-02", strings.TrimSpace(req.From))
	if err != nil {
		respondError(c, domain.NewAppError(reportl1svc.CodeInvalidDateRange, "invalid from date", nil))
		return
	}
	to, err := time.Parse("2006-01-02", strings.TrimSpace(req.To))
	if err != nil {
		respondError(c, domain.NewAppError(reportl1svc.CodeInvalidDateRange, "invalid to date", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	data, appErr := h.svc.BusinessTrendPilotAnalysis(c.Request.Context(), actor, reportl1svc.BusinessTrendAnalysisParams{
		From:    from.UTC(),
		To:      to.UTC(),
		Mode:    req.Mode,
		Sources: req.Sources,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{"data": data})
}

func parseReportRange(c *gin.Context) (time.Time, time.Time, *int64, *string, *domain.AppError) {
	from, err := time.Parse("2006-01-02", strings.TrimSpace(c.Query("from")))
	if err != nil {
		return time.Time{}, time.Time{}, nil, nil, domain.NewAppError(reportl1svc.CodeInvalidDateRange, "invalid from date", nil)
	}
	to, err := time.Parse("2006-01-02", strings.TrimSpace(c.Query("to")))
	if err != nil {
		return time.Time{}, time.Time{}, nil, nil, domain.NewAppError(reportl1svc.CodeInvalidDateRange, "invalid to date", nil)
	}
	var deptID *int64
	if raw := strings.TrimSpace(c.Query("department_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return time.Time{}, time.Time{}, nil, nil, domain.NewAppError(reportl1svc.CodeInvalidDateRange, "invalid department_id", nil)
		}
		deptID = &parsed
	}
	var taskType *string
	if raw := strings.TrimSpace(c.Query("task_type")); raw != "" {
		taskType = &raw
	}
	return from.UTC(), to.UTC(), deptID, taskType, nil
}
