package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	coresvc "workflow/service"
	predictionsvc "workflow/service/prediction"
)

type PredictionHandler struct {
	svc           *predictionsvc.Service
	experienceSvc coresvc.ExperienceService
}

type predictionBundleResponse struct {
	Data *domain.PredictionBundle `json:"data"`
}

func NewPredictionHandler(svc *predictionsvc.Service) *PredictionHandler {
	return &PredictionHandler{svc: svc}
}

func (h *PredictionHandler) SetExperienceService(svc coresvc.ExperienceService) {
	h.experienceSvc = svc
}

func (h *PredictionHandler) Search(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	limit := queryInt(c, "limit")
	data, appErr := h.svc.SearchSuggestions(c.Request.Context(), actor, c.Query("q"), c.Query("scope"), limit)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	h.recordSuggestionDisplay(c, actor, "search", data)
	respondPredictionBundle(c, data)
}

func (h *PredictionHandler) TaskCreate(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	limit := queryInt(c, "limit")
	keyword := firstQuery(c, "keyword", "q")
	data, appErr := h.svc.TaskCreateSuggestions(c.Request.Context(), actor, keyword, c.Query("task_type"), limit)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	h.recordSuggestionDisplay(c, actor, "task_create", data)
	respondPredictionBundle(c, data)
}

func (h *PredictionHandler) TaskNextActions(c *gin.Context) {
	taskID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || taskID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	data, appErr := h.svc.TaskNextActionSuggestions(c.Request.Context(), actor, taskID, queryInt(c, "limit"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	h.recordSuggestionDisplay(c, actor, "task_next_action", data)
	respondPredictionBundle(c, data)
}

func (h *PredictionHandler) Assets(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	data, appErr := h.svc.AssetSuggestions(c.Request.Context(), actor, firstQuery(c, "q", "keyword"), queryInt(c, "limit"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	h.recordSuggestionDisplay(c, actor, "assets", data)
	respondPredictionBundle(c, data)
}

func (h *PredictionHandler) Management(c *gin.Context) {
	from, to, appErr := parsePredictionDateRange(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	data, appErr := h.svc.ManagementSuggestions(c.Request.Context(), actor, from, to, queryInt(c, "limit"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	h.recordSuggestionDisplay(c, actor, "management", data)
	respondPredictionBundle(c, data)
}

func (h *PredictionHandler) recordSuggestionDisplay(c *gin.Context, actor domain.RequestActor, surface string, bundle *domain.PredictionBundle) {
	if bundle == nil || len(bundle.Suggestions) == 0 {
		return
	}
	route := c.FullPath()
	limit := queryInt(c, "limit")
	inputSummary, _ := json.Marshal(map[string]interface{}{
		"surface": surface,
		"route":   route,
		"limit":   limit,
	})
	displayedAt := bundle.GeneratedAt
	if displayedAt.IsZero() {
		displayedAt = time.Now().UTC()
	}
	events := make([]domain.AISuggestionEvent, 0, len(bundle.Suggestions))
	for i := range bundle.Suggestions {
		bundle.Suggestions[i].SuggestionEventID = predictionSuggestionEventID(surface, bundle.Suggestions[i], displayedAt, i)
		suggestion := bundle.Suggestions[i]
		suggestionJSON, err := json.Marshal(suggestion)
		if err != nil {
			continue
		}
		confidence := predictionConfidenceScore(suggestion.Confidence)
		event := &domain.AISuggestionEvent{
			SuggestionEventID: suggestion.SuggestionEventID,
			SuggestionType:    strings.TrimSpace(suggestion.Type),
			SuggestionID:      strings.TrimSpace(suggestion.ID),
			Source:            strings.TrimSpace(suggestion.Source),
			Confidence:        confidence,
			Model:             "deterministic_prediction",
			Provider:          "internal",
			ModelVersion:      "v1",
			InputSummary:      inputSummary,
			Suggestion:        suggestionJSON,
			TargetType:        strings.TrimSpace(suggestion.TargetType),
			TargetID:          strings.TrimSpace(suggestion.TargetID),
			DisplayedAt:       displayedAt,
		}
		if actor.ID > 0 {
			event.ActorID = &actor.ID
		}
		events = append(events, *event)
	}
	if h == nil || h.experienceSvc == nil || !h.experienceSvc.RuntimeFlags().CaptureEnabled {
		return
	}
	if len(events) == 0 {
		return
	}
	baseCtx := context.WithoutCancel(c.Request.Context())
	go func(svc coresvc.ExperienceService, events []domain.AISuggestionEvent) {
		ctx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
		defer cancel()
		for i := range events {
			event := events[i]
			_ = svc.RecordAISuggestionEvent(ctx, &event)
		}
	}(h.experienceSvc, events)
}

func predictionSuggestionEventID(surface string, suggestion domain.PredictionSuggestion, displayedAt time.Time, ordinal int) string {
	displayedAt = displayedAt.UTC()
	identity := strings.Join([]string{
		strings.TrimSpace(surface),
		strings.TrimSpace(suggestion.Type),
		strings.TrimSpace(suggestion.ID),
		strings.TrimSpace(suggestion.Source),
		strings.TrimSpace(suggestion.TargetType),
		strings.TrimSpace(suggestion.TargetID),
		displayedAt.Format(time.RFC3339Nano),
		strconv.Itoa(ordinal),
	}, "|")
	sum := sha256.Sum256([]byte(identity))
	return fmt.Sprintf(
		"pred:%s:%s:%s:%02d:%s",
		predictionEventIDToken(surface, 32),
		predictionEventIDToken(suggestion.Type, 32),
		displayedAt.Format("20060102T150405.000000000Z"),
		ordinal,
		hex.EncodeToString(sum[:8]),
	)
}

func predictionEventIDToken(value string, max int) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		value = "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= max {
			break
		}
	}
	token := strings.Trim(b.String(), "_")
	if token == "" {
		return "unknown"
	}
	return token
}

func predictionConfidenceScore(confidence string) *float64 {
	var value float64
	switch strings.ToLower(strings.TrimSpace(confidence)) {
	case "high":
		value = 0.9
	case "medium":
		value = 0.6
	case "low":
		value = 0.3
	default:
		return nil
	}
	return &value
}

func respondPredictionBundle(c *gin.Context, data *domain.PredictionBundle) {
	c.JSON(http.StatusOK, predictionBundleResponse{Data: data})
}

func queryInt(c *gin.Context, key string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(c.Query(key)))
	return value
}

func firstQuery(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			return value
		}
	}
	return ""
}

func parsePredictionDateRange(c *gin.Context) (time.Time, time.Time, *domain.AppError) {
	fromRaw := strings.TrimSpace(c.Query("from"))
	toRaw := strings.TrimSpace(c.Query("to"))
	var from, to time.Time
	var err error
	if fromRaw != "" {
		from, err = time.Parse("2006-01-02", fromRaw)
		if err != nil {
			return time.Time{}, time.Time{}, domain.NewAppError("invalid_date_range", "invalid from date", nil)
		}
	}
	if toRaw != "" {
		to, err = time.Parse("2006-01-02", toRaw)
		if err != nil {
			return time.Time{}, time.Time{}, domain.NewAppError("invalid_date_range", "invalid to date", nil)
		}
	}
	return from.UTC(), to.UTC(), nil
}
