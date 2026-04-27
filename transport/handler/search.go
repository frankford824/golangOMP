package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	searchsvc "workflow/service/search"
)

type SearchHandler struct {
	svc *searchsvc.Service
}

func NewSearchHandler(svc *searchsvc.Service) *SearchHandler {
	return &SearchHandler{svc: svc}
}

func (h *SearchHandler) Search(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	result, appErr := h.svc.Search(c.Request.Context(), actor, c.Query("q"), strings.TrimSpace(c.Query("scope")), limit)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{
		"query":   strings.TrimSpace(c.Query("q")),
		"results": result,
	})
}
