package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

func parseTaskFilterQuery(c *gin.Context) (service.TaskFilter, *domain.AppError) {
	filter := service.TaskFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			Statuses:                     parseTaskStatuses(c, "status"),
			TaskTypes:                    parseTaskTypes(c, "task_type"),
			SourceModes:                  parseTaskSourceModes(c, "source_mode"),
			WorkflowLanes:                parseWorkflowLanes(c, "workflow_lane"),
			MainStatuses:                 parseTaskMainStatuses(c, "main_status"),
			SubStatusCodes:               parseTaskSubStatusCodes(c, "sub_status_code"),
			CoordinationStatuses:         parseCoordinationStatuses(c, "coordination_status"),
			OwnerDepartments:             readQueryList(c, "owner_department"),
			OwnerOrgTeams:                readQueryList(c, "owner_org_team"),
			WarehouseBlockingReasonCodes: parseWorkflowReasonCodes(c, "warehouse_blocking_reason_code"),
		},
		Keyword: c.Query("keyword"),
	}

	if raw := c.Query("sub_status_scope"); raw != "" {
		scope := domain.TaskSubStatusScope(raw)
		filter.SubStatusScope = &scope
	}
	if raw := c.Query("creator_id"); raw != "" {
		id, err := parseInt64(raw)
		if err != nil {
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "creator_id must be an integer", nil)
		}
		filter.CreatorID = &id
	}
	if raw := c.Query("designer_id"); raw != "" {
		id, err := parseInt64(raw)
		if err != nil {
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "designer_id must be an integer", nil)
		}
		filter.DesignerID = &id
	}
	if raw := c.Query("need_outsource"); raw != "" {
		value, err := parseBool(raw)
		if err != nil {
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "need_outsource must be true/false/1/0", nil)
		}
		filter.NeedOutsource = &value
	}
	if raw := c.Query("overdue"); raw != "" {
		value, err := parseBool(raw)
		if err != nil {
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "overdue must be true/false/1/0", nil)
		}
		filter.Overdue = &value
	}
	if raw := c.Query("warehouse_prepare_ready"); raw != "" {
		value, err := parseBool(raw)
		if err != nil {
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "warehouse_prepare_ready must be true/false/1/0", nil)
		}
		filter.WarehousePrepareReady = &value
	}
	if raw := c.Query("warehouse_receive_ready"); raw != "" {
		value, err := parseBool(raw)
		if err != nil {
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "warehouse_receive_ready must be true/false/1/0", nil)
		}
		filter.WarehouseReceiveReady = &value
	}
	if raw := c.Query("page"); raw != "" {
		page, err := parseInt(raw)
		if err != nil {
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be an integer", nil)
		}
		filter.Page = page
	}
	if raw := c.Query("page_size"); raw != "" {
		pageSize, err := parseInt(raw)
		if err != nil {
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil)
		}
		filter.PageSize = pageSize
	}

	return filter, nil
}

func parseTaskStatuses(c *gin.Context, key string) []domain.TaskStatus {
	values := readQueryList(c, key)
	out := make([]domain.TaskStatus, 0, len(values))
	for _, value := range values {
		out = append(out, domain.TaskStatus(value))
	}
	return out
}

func parseTaskTypes(c *gin.Context, key string) []domain.TaskType {
	values := readQueryList(c, key)
	out := make([]domain.TaskType, 0, len(values))
	for _, value := range values {
		out = append(out, domain.TaskType(value))
	}
	return out
}

func parseTaskSourceModes(c *gin.Context, key string) []domain.TaskSourceMode {
	values := readQueryList(c, key)
	out := make([]domain.TaskSourceMode, 0, len(values))
	for _, value := range values {
		out = append(out, domain.TaskSourceMode(value))
	}
	return out
}

func parseTaskMainStatuses(c *gin.Context, key string) []domain.TaskMainStatus {
	values := readQueryList(c, key)
	out := make([]domain.TaskMainStatus, 0, len(values))
	for _, value := range values {
		out = append(out, domain.TaskMainStatus(value))
	}
	return out
}

func parseWorkflowLanes(c *gin.Context, key string) []domain.WorkflowLane {
	values := readQueryList(c, key)
	out := make([]domain.WorkflowLane, 0, len(values))
	for _, value := range values {
		out = append(out, domain.WorkflowLane(value))
	}
	return out
}

func parseTaskSubStatusCodes(c *gin.Context, key string) []domain.TaskSubStatusCode {
	values := readQueryList(c, key)
	out := make([]domain.TaskSubStatusCode, 0, len(values))
	for _, value := range values {
		out = append(out, domain.TaskSubStatusCode(value))
	}
	return out
}

func parseCoordinationStatuses(c *gin.Context, key string) []domain.ProcurementCoordinationStatus {
	values := readQueryList(c, key)
	out := make([]domain.ProcurementCoordinationStatus, 0, len(values))
	for _, value := range values {
		out = append(out, domain.ProcurementCoordinationStatus(value))
	}
	return out
}

func parseWorkflowReasonCodes(c *gin.Context, key string) []domain.WorkflowReasonCode {
	values := readQueryList(c, key)
	out := make([]domain.WorkflowReasonCode, 0, len(values))
	for _, value := range values {
		out = append(out, domain.WorkflowReasonCode(value))
	}
	return out
}

func readQueryList(c *gin.Context, key string) []string {
	rawValues := c.Request.URL.Query()[key]
	if len(rawValues) == 0 {
		if raw := c.Query(key); raw != "" {
			rawValues = []string{raw}
		}
	}
	out := make([]string, 0, len(rawValues))
	for _, raw := range rawValues {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			out = append(out, part)
		}
	}
	return out
}
