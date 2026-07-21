package handler

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

func parseTaskFilterQuery(c *gin.Context) (service.TaskFilter, *domain.AppError) {
	priorities, appErr := parseTaskPriorities(c, "priority")
	if appErr != nil {
		return service.TaskFilter{}, appErr
	}

	mineFilterEnabled := strings.EqualFold(strings.TrimSpace(c.Query("filter")), "mine")
	filter := service.TaskFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			Statuses:         parseTaskStatuses(c, "status"),
			Priorities:       priorities,
			TaskTypes:        parseTaskTypes(c, "task_type"),
			SourceModes:      parseTaskSourceModes(c, "source_mode"),
			BusinessLanes:    parseTaskBusinessLanes(c, "business_lane"),
			OwnerDepartments: readQueryList(c, "owner_department"),
			OwnerOrgTeams:    readQueryList(c, "owner_org_team"),
		},
		Keyword: c.Query("keyword"),
	}

	if raw := c.Query("creator_id"); raw != "" {
		id, err := parseInt64(raw)
		if err != nil {
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "creator_id must be an integer", nil)
		}
		filter.CreatorID = &id
	}
	if mineFilterEnabled {
		actorID, appErr := actorIDOrRequestValue(c, nil, "creator_id")
		if appErr != nil {
			return service.TaskFilter{}, appErr
		}
		// "mine" includes tasks where the actor is creator, assigned designer, or current handler.
		filter.MineActorID = &actorID
	}
	if raw := c.Query("designer_id"); raw != "" {
		id, err := parseInt64(raw)
		if err != nil {
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "designer_id must be an integer", nil)
		}
		filter.DesignerID = &id
	}
	if raw := c.Query("designer_empty"); raw != "" {
		value, err := parseBool(raw)
		if err != nil {
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "designer_empty must be true/false/1/0", nil)
		}
		filter.DesignerEmpty = &value
	}
	if raw := c.Query("overdue"); raw != "" {
		value, err := parseBool(raw)
		if err != nil {
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "overdue must be true/false/1/0", nil)
		}
		filter.Overdue = &value
	}
	createdFrom, appErr := parseTaskCreatedDateBoundary(c.Query("date_from"), false)
	if appErr != nil {
		return service.TaskFilter{}, appErr
	}
	createdTo, appErr := parseTaskCreatedDateBoundary(c.Query("date_to"), true)
	if appErr != nil {
		return service.TaskFilter{}, appErr
	}
	if createdFrom != nil && createdTo != nil && createdFrom.After(*createdTo) {
		return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "date_from must not be after date_to", map[string]interface{}{
			"date_from": strings.TrimSpace(c.Query("date_from")),
			"date_to":   strings.TrimSpace(c.Query("date_to")),
		})
	}
	filter.CreatedFrom = createdFrom
	filter.CreatedTo = createdTo
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
	if raw := strings.TrimSpace(c.Query("sort")); raw != "" {
		field := strings.TrimPrefix(raw, "-")
		switch field {
		case "updated_at", "task_no", "due_at", "created_at":
			filter.Sort = raw
		default:
			return service.TaskFilter{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "unsupported sort field", map[string]interface{}{"sort": raw})
		}
	}

	return filter, nil
}

func parseTaskCreatedDateBoundary(raw string, endOfDay bool) (*time.Time, *domain.AppError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return &parsed, nil
	}
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	parsed, err := time.ParseInLocation("2006-01-02", raw, location)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "date filter must be YYYY-MM-DD or RFC3339", map[string]interface{}{
			"value": raw,
		})
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	return &parsed, nil
}

func parseTaskPriorities(c *gin.Context, key string) ([]domain.TaskPriority, *domain.AppError) {
	values := readQueryList(c, key)
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]domain.TaskPriority, 0, len(values))
	seen := make(map[domain.TaskPriority]struct{}, len(values))
	for _, value := range values {
		priority := domain.TaskPriority(strings.ToLower(strings.TrimSpace(value)))
		if !validListTaskPriority(priority) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_priority_invalid", map[string]interface{}{
				"field": "priority",
				"value": value,
			})
		}
		if _, exists := seen[priority]; exists {
			continue
		}
		seen[priority] = struct{}{}
		out = append(out, priority)
	}
	return out, nil
}

func validListTaskPriority(priority domain.TaskPriority) bool {
	switch priority {
	case domain.TaskPriorityLow, domain.TaskPriorityNormal, domain.TaskPriorityHigh, domain.TaskPriorityCritical:
		return true
	default:
		return false
	}
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

func parseTaskBusinessLanes(c *gin.Context, key string) []domain.TaskBusinessLane {
	values := readQueryList(c, key)
	out := make([]domain.TaskBusinessLane, 0, len(values))
	for _, value := range values {
		out = append(out, domain.TaskBusinessLane(value))
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
