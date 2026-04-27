package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type AuthHandler struct {
	svc service.IdentityService
}

func NewAuthHandler(svc service.IdentityService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

type registerReq struct {
	Username           string    `json:"username" binding:"required"`
	Account            string    `json:"account"`
	DisplayName        string    `json:"display_name"`
	Name               string    `json:"name"`
	Department         string    `json:"department" binding:"required"`
	Team               string    `json:"team"`
	Group              string    `json:"group"`
	Mobile             string    `json:"mobile" binding:"required"`
	Phone              string    `json:"phone"`
	Email              string    `json:"email"`
	Password           string    `json:"password" binding:"required"`
	AdminKey           string    `json:"admin_key"`
	SecretKey          string    `json:"secret_key"`
	ManagedDepartments *[]string `json:"managed_departments"`
}

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Account  string `json:"account"`
	Password string `json:"password" binding:"required"`
}

type changePasswordReq struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
	Confirm     string `json:"confirm"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.Register(c.Request.Context(), service.RegisterUserParams{
		Username:           firstNonEmpty(req.Account, req.Username),
		DisplayName:        firstNonEmpty(req.Name, req.DisplayName),
		Department:         domain.Department(req.Department),
		Team:               firstNonEmpty(req.Group, req.Team),
		Mobile:             firstNonEmpty(req.Phone, req.Mobile),
		Email:              req.Email,
		Password:           req.Password,
		AdminKey:           firstNonEmpty(req.SecretKey, req.AdminKey),
		ManagedDepartments: req.ManagedDepartments,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AuthHandler) RegisterOptions(c *gin.Context) {
	options, appErr := h.svc.GetRegistrationOptions(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, options)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.Login(c.Request.Context(), service.LoginParams{
		Username: firstNonEmpty(req.Account, req.Username),
		Password: req.Password,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, appErr := h.svc.GetCurrentUser(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, user)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
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
	respondOK(c, gin.H{"message": "password changed"})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
