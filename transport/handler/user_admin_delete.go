package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type deleteUserReq struct {
	Reason string `json:"reason"`
}

func (h *UserAdminHandler) Delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user id", nil))
		return
	}
	var req deleteUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	if appErr := h.svc.DeleteUser(c.Request.Context(), service.DeleteUserParams{UserID: id, Reason: req.Reason}); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(http.StatusNoContent)
}
