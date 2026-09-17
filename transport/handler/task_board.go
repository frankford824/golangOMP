package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type TaskBoardHandler struct {
	svc service.TaskBoardService
}

func (h *TaskBoardHandler) DesignDepartmentDashboard(c *gin.Context) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	now := time.Now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	startAt := today.AddDate(0, 0, -29)
	endAt := today.AddDate(0, 0, 1)
	if raw := strings.TrimSpace(c.Query("start_date")); raw != "" {
		parsed, parseErr := time.ParseInLocation("2006-01-02", raw, location)
		if parseErr != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "start_date must be YYYY-MM-DD", nil))
			return
		}
		startAt = parsed
	}
	if raw := strings.TrimSpace(c.Query("end_date")); raw != "" {
		parsed, parseErr := time.ParseInLocation("2006-01-02", raw, location)
		if parseErr != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "end_date must be YYYY-MM-DD", nil))
			return
		}
		endAt = parsed.AddDate(0, 0, 1)
	}
	dateBasis := domain.DesignDashboardDateBasis(strings.TrimSpace(c.DefaultQuery("date_basis", string(domain.DesignDashboardDateCreated))))
	granularity := domain.DesignDashboardGranularity(strings.TrimSpace(c.DefaultQuery("granularity", string(domain.DesignDashboardGranularityDay))))
	filter := domain.DesignDepartmentDashboardFilter{
		StartAt: startAt.UTC(), EndAt: endAt.UTC(), DateBasis: dateBasis, Granularity: granularity,
		DesignerIDs: parseDashboardInt64List(c.Query("designer_ids")), TeamIDs: parseDashboardInt64List(c.Query("team_ids")),
	}
	for _, value := range parseDashboardStringList(c.Query("task_types")) {
		taskType := domain.TaskType(value)
		if !taskType.Valid() {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task_types", nil))
			return
		}
		filter.TaskTypes = append(filter.TaskTypes, taskType)
	}
	for _, value := range parseDashboardStringList(c.Query("statuses")) {
		status := domain.TaskStatus(value)
		if !validDashboardTaskStatus(status) {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid statuses", nil))
			return
		}
		filter.Statuses = append(filter.Statuses, status)
	}
	for _, value := range parseDashboardStringList(c.Query("priorities")) {
		priority := domain.TaskPriority(value)
		if !validDashboardTaskPriority(priority) {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid priorities", nil))
			return
		}
		filter.Priorities = append(filter.Priorities, priority)
	}
	for _, value := range parseDashboardStringList(c.Query("business_lanes")) {
		lane := domain.TaskBusinessLane(value)
		if !lane.Valid() {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid business_lanes", nil))
			return
		}
		filter.BusinessLanes = append(filter.BusinessLanes, lane)
	}
	result, appErr := h.svc.GetDesignDepartmentDashboard(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func validDashboardTaskStatus(value domain.TaskStatus) bool {
	switch value {
	case domain.TaskStatusDraft, domain.TaskStatusPendingAssign, domain.TaskStatusAssigned, domain.TaskStatusInProgress,
		domain.TaskStatusPendingAudit, domain.TaskStatusCompleted, domain.TaskStatusArchived, domain.TaskStatusCancelled, domain.TaskStatusBlocked:
		return true
	default:
		return false
	}
}

func validDashboardTaskPriority(value domain.TaskPriority) bool {
	switch value {
	case domain.TaskPriorityNormal, domain.TaskPriorityHigh, domain.TaskPriorityDrawing, domain.TaskPriorityLow, domain.TaskPriorityCritical:
		return true
	default:
		return false
	}
}

func parseDashboardStringList(raw string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func parseDashboardInt64List(raw string) []int64 {
	out := []int64{}
	for _, item := range parseDashboardStringList(raw) {
		value, err := strconv.ParseInt(item, 10, 64)
		if err == nil && value > 0 {
			out = append(out, value)
		}
	}
	return out
}

func NewTaskBoardHandler(svc service.TaskBoardService) *TaskBoardHandler {
	return &TaskBoardHandler{svc: svc}
}

func (h *TaskBoardHandler) OperationalOverview(c *gin.Context) {
	result, appErr := h.svc.GetOperationalOverview(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}
