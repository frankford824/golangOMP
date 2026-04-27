package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

func (h *UserAdminHandler) Activate(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user id", nil))
		return
	}
	if appErr := h.svc.ActivateUser(c.Request.Context(), id); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *UserAdminHandler) Deactivate(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user id", nil))
		return
	}
	if appErr := h.svc.DeactivateUser(c.Request.Context(), id); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(http.StatusNoContent)
}
