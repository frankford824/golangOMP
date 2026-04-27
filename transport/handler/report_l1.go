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
