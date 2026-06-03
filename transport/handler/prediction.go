package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	predictionsvc "workflow/service/prediction"
)

type PredictionHandler struct {
	svc *predictionsvc.Service
}

type predictionBundleResponse struct {
	Data *domain.PredictionBundle `json:"data"`
}

func NewPredictionHandler(svc *predictionsvc.Service) *PredictionHandler {
	return &PredictionHandler{svc: svc}
}

func (h *PredictionHandler) Search(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	limit := queryInt(c, "limit")
	data, appErr := h.svc.SearchSuggestions(c.Request.Context(), actor, c.Query("q"), c.Query("scope"), limit)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
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
	respondPredictionBundle(c, data)
}

func (h *PredictionHandler) Assets(c *gin.Context) {
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	data, appErr := h.svc.AssetSuggestions(c.Request.Context(), actor, firstQuery(c, "q", "keyword"), queryInt(c, "limit"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
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
	respondPredictionBundle(c, data)
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
