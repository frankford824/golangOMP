package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

// AssetFilesTokenCookie carries the session token for browser-native asset
// loads (<img> src, direct download links) that cannot attach an Authorization
// header. The companion stream cookie covers authenticated external downloads.
const (
	AssetFilesTokenCookie      = "wf_asset_token"
	assetFilesTokenCookiePath  = "/v1/assets/files"
	AssetStreamTokenCookie     = "wf_asset_stream_token"
	assetStreamTokenCookiePath = "/v1/assets"
	WSTokenCookie              = "wf_ws_token"
	wsTokenCookiePath          = "/ws"
)

func setAssetFilesTokenCookie(c *gin.Context, result *domain.AuthResult, cookieDomain string) {
	if result == nil || result.Session == nil || strings.TrimSpace(result.Session.Token) == "" {
		return
	}
	maxAge := int(time.Until(result.Session.ExpiresAt).Seconds())
	if maxAge <= 0 {
		return
	}
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(AssetFilesTokenCookie, result.Session.Token, maxAge, assetFilesTokenCookiePath, cookieDomain, secure, true)
	c.SetCookie(AssetStreamTokenCookie, result.Session.Token, maxAge, assetStreamTokenCookiePath, cookieDomain, secure, true)
	c.SetCookie(WSTokenCookie, result.Session.Token, maxAge, wsTokenCookiePath, cookieDomain, secure, true)
}

func setAssetFilesRawTokenCookie(c *gin.Context, token string, maxAge int, cookieDomain string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(AssetFilesTokenCookie, token, maxAge, assetFilesTokenCookiePath, cookieDomain, secure, true)
	c.SetCookie(AssetStreamTokenCookie, token, maxAge, assetStreamTokenCookiePath, cookieDomain, secure, true)
	c.SetCookie(WSTokenCookie, token, maxAge, wsTokenCookiePath, cookieDomain, secure, true)
}

func clearAssetFilesTokenCookie(c *gin.Context, cookieDomain string) {
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(AssetFilesTokenCookie, "", -1, assetFilesTokenCookiePath, cookieDomain, secure, true)
	c.SetCookie(AssetStreamTokenCookie, "", -1, assetStreamTokenCookiePath, cookieDomain, secure, true)
	c.SetCookie(WSTokenCookie, "", -1, wsTokenCookiePath, cookieDomain, secure, true)
}

func bearerTokenFromHeader(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	fields := strings.Fields(header)
	if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
		return strings.TrimSpace(fields[1])
	}
	return ""
}

type AuthHandler struct {
	svc               service.IdentityService
	assetCookieDomain string
}

func NewAuthHandler(svc service.IdentityService, assetCookieDomains ...string) *AuthHandler {
	assetCookieDomain := ""
	if len(assetCookieDomains) > 0 {
		assetCookieDomain = strings.TrimSpace(assetCookieDomains[0])
	}
	return &AuthHandler{svc: svc, assetCookieDomain: assetCookieDomain}
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
	OldPassword          string `json:"old_password" binding:"required"`
	NewPassword          string `json:"new_password" binding:"required"`
	Confirm              string `json:"confirm"`
	PasswordConfirmation string `json:"password_confirmation"`
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
	setAssetFilesTokenCookie(c, result, h.assetCookieDomain)
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
	setAssetFilesTokenCookie(c, result, h.assetCookieDomain)
	respondOK(c, result)
}

func (h *AuthHandler) RefreshAssetCookie(c *gin.Context) {
	if actor, ok := domain.RequestActorFromContext(c.Request.Context()); !ok || !domain.IsSessionBackedRequestActor(actor) {
		respondError(c, domain.NewAppError(domain.ErrCodeUnauthorized, "Authentication required.", nil))
		return
	}
	token, ok := domain.RequestBearerTokenFromContext(c.Request.Context())
	if !ok {
		token = bearerTokenFromHeader(c.GetHeader("Authorization"))
	}
	if token == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeUnauthorized, "Authentication required.", nil))
		return
	}
	setAssetFilesRawTokenCookie(c, token, 0, h.assetCookieDomain)
	respondOK(c, gin.H{"status": "ok"})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	clearAssetFilesTokenCookie(c, h.assetCookieDomain)
	respondOK(c, gin.H{"status": "ok"})
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
		Confirm:     firstNonEmpty(req.Confirm, req.PasswordConfirmation),
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
