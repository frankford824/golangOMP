package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	designsourcesvc "workflow/service/design_source"
)

type DesignSourceHandler struct {
	svc *designsourcesvc.Service
}

func NewDesignSourceHandler(svc *designsourcesvc.Service) *DesignSourceHandler {
	return &DesignSourceHandler{svc: svc}
}

func (h *DesignSourceHandler) Search(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	items, total, appErr := h.svc.Search(c.Request.Context(), actor, c.Query("keyword"), page, size)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.JSON(200, gin.H{"data": items, "total": total, "page": page, "size": size})
}
