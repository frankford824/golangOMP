package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type updateMeReq struct {
	DisplayName *string `json:"display_name"`
	Mobile      *string `json:"mobile"`
	Email       *string `json:"email"`
	Avatar      *string `json:"avatar"`
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	user, appErr := h.svc.GetMe(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, user)
}

func (h *AuthHandler) PatchMe(c *gin.Context) {
	var req updateMeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	user, appErr := h.svc.UpdateMe(c.Request.Context(), service.UpdateMeParams{
		DisplayName: req.DisplayName,
		Mobile:      req.Mobile,
		Email:       req.Email,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, user)
}

func (h *AuthHandler) ChangeMyPassword(c *gin.Context) {
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	if req.Confirm == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "confirm is required", map[string]string{"deny_code": "password_confirmation_required"}))
		return
	}
	if appErr := h.svc.ChangePassword(c.Request.Context(), service.ChangePasswordParams{
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
		Confirm:     req.Confirm,
	}); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) GetMyOrg(c *gin.Context) {
	profile, appErr := h.svc.GetMyOrg(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, profile)
}
