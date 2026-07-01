package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type ExperienceHandler struct {
	svc service.ExperienceService
}

func NewExperienceHandler(svc service.ExperienceService) *ExperienceHandler {
	return &ExperienceHandler{svc: svc}
}

func (h *ExperienceHandler) Config(c *gin.Context) {
	respondOK(c, h.svc.RuntimeFlags())
}

func (h *ExperienceHandler) ReasonTags(c *gin.Context) {
	items, appErr := h.svc.ListReasonTags(c.Request.Context(), c.Query("scene"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func (h *ExperienceHandler) Stats(c *gin.Context) {
	data, appErr := h.svc.Stats(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, data)
}

func (h *ExperienceHandler) Samples(c *gin.Context) {
	filter, appErr := parseExperienceSamplesFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	items, pagination, appErr := h.svc.ListSamples(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, pagination)
}

func (h *ExperienceHandler) AISuggestionFeedback(c *gin.Context) {
	var req service.AISuggestionFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid request body", nil))
		return
	}
	if strings.TrimSpace(req.SuggestionEventID) == "" {
		req.SuggestionEventID = c.Param("suggestion_event_id")
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	data, appErr := h.svc.RecordAISuggestionFeedback(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, data)
}

func parseExperienceSamplesFilter(c *gin.Context) (service.ExperienceEventFilter, *domain.AppError) {
	var filter service.ExperienceEventFilter
	filter.SourceType = c.Query("source_type")
	filter.SourceID = c.Query("source_id")
	filter.Action = c.Query("action")
	filter.Outcome = c.Query("outcome")
	filter.MinEvidenceLevel = c.Query("min_evidence_level")
	filter.Page = queryInt(c, "page")
	filter.PageSize = queryInt(c, "page_size")
	if raw := strings.TrimSpace(c.Query("task_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return filter, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task_id", nil)
		}
		filter.TaskID = &id
	}
	from, appErr := parseExperienceTimeQuery(c.Query("from"), false)
	if appErr != nil {
		return filter, appErr
	}
	to, appErr := parseExperienceTimeQuery(c.Query("to"), true)
	if appErr != nil {
		return filter, appErr
	}
	filter.From = from
	filter.To = to
	return filter, nil
}

func parseExperienceTimeQuery(raw string, endOfDay bool) (*time.Time, *domain.AppError) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		utc := parsed.UTC()
		return &utc, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, domain.NewAppError("invalid_date_range", "invalid date range", nil)
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	utc := parsed.UTC()
	return &utc, nil
}
