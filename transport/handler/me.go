package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

const (
	maxMeAvatarBytes          int64 = 2 * 1024 * 1024
	maxMeAvatarMultipartBytes int64 = maxMeAvatarBytes + 64*1024
)

var allowedMeAvatarExt = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".webp": {},
}

var allowedMeAvatarMime = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

type updateMeReq struct {
	DisplayName *string `json:"display_name"`
	Mobile      *string `json:"mobile"`
	Email       *string `json:"email"`
	Avatar      *string `json:"avatar"`
	AvatarURL   *string `json:"avatar_url"`
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
	if req.Avatar != nil || req.AvatarURL != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "avatar changes must use the profile avatar upload or delete action", map[string]string{"deny_code": "avatar_update_requires_avatar_api"}))
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
	confirm := firstNonEmpty(req.Confirm, req.PasswordConfirmation)
	if confirm == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "confirm is required", map[string]string{"deny_code": "password_confirmation_required"}))
		return
	}
	if appErr := h.svc.ChangePassword(c.Request.Context(), service.ChangePasswordParams{
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
		Confirm:     confirm,
	}); appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) UploadMyAvatar(c *gin.Context) {
	actor, ok := domain.RequestActorFromContext(c.Request.Context())
	if !ok || !domain.IsSessionBackedRequestActor(actor) {
		respondError(c, domain.ErrUnauthorized)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMeAvatarMultipartBytes)
	file, err := c.FormFile("file")
	if err != nil {
		if isMeAvatarRequestTooLarge(err) {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "avatar file must be 2MB or smaller", map[string]string{"deny_code": "avatar_file_too_large"}))
			return
		}
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "avatar file is required", map[string]string{"deny_code": "avatar_file_required"}))
		return
	}
	if appErr := validateMeAvatarFile(file); appErr != nil {
		respondError(c, appErr)
		return
	}
	if err := os.MkdirAll(meAvatarStorageDir(), 0o755); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "avatar storage is not available", nil))
		return
	}
	filename, err := newMeAvatarFilename(filepath.Ext(file.Filename))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "avatar filename generation failed", nil))
		return
	}
	target := filepath.Join(meAvatarStorageDir(), filename)
	if err := c.SaveUploadedFile(file, target); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "avatar upload failed", nil))
		return
	}

	oldUser, _ := h.svc.GetMe(c.Request.Context())
	avatarURL := "/v1/me/avatar-files/" + filename
	user, appErr := h.svc.UpdateMyAvatar(c.Request.Context(), service.UpdateMyAvatarParams{
		AvatarURL: avatarURL,
		Method:    http.MethodPost,
	})
	if appErr != nil {
		_ = os.Remove(target)
		respondError(c, appErr)
		return
	}
	if oldUser != nil && strings.TrimSpace(oldUser.AvatarURL) != "" && oldUser.AvatarURL != avatarURL {
		removeStoredMeAvatar(oldUser.AvatarURL)
	}
	respondOK(c, user)
}

func (h *AuthHandler) DeleteMyAvatar(c *gin.Context) {
	oldUser, _ := h.svc.GetMe(c.Request.Context())
	user, appErr := h.svc.UpdateMyAvatar(c.Request.Context(), service.UpdateMyAvatarParams{
		AvatarURL: "",
		Method:    http.MethodDelete,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	if oldUser != nil && strings.TrimSpace(oldUser.AvatarURL) != "" {
		removeStoredMeAvatar(oldUser.AvatarURL)
	}
	respondOK(c, user)
}

func (h *AuthHandler) ServeMyAvatar(c *gin.Context) {
	filename := strings.TrimSpace(c.Param("filename"))
	if !isManagedMeAvatarFilename(filename) {
		respondError(c, domain.ErrNotFound)
		return
	}
	path := filepath.Join(meAvatarStorageDir(), filename)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			respondError(c, domain.ErrNotFound)
			return
		}
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "avatar read failed", nil))
		return
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.File(path)
}

func (h *AuthHandler) GetMyOrg(c *gin.Context) {
	profile, appErr := h.svc.GetMyOrg(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, profile)
}

func validateMeAvatarFile(file *multipart.FileHeader) *domain.AppError {
	if file == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "avatar file is required", map[string]string{"deny_code": "avatar_file_required"})
	}
	if file.Size <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "avatar file is empty", map[string]string{"deny_code": "avatar_file_empty"})
	}
	if file.Size > maxMeAvatarBytes {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "avatar file must be 2MB or smaller", map[string]string{"deny_code": "avatar_file_too_large"})
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if _, ok := allowedMeAvatarExt[ext]; !ok {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "avatar only supports JPG, PNG, or WebP", map[string]string{"deny_code": "avatar_file_type_unsupported"})
	}
	src, err := file.Open()
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "avatar file cannot be read", map[string]string{"deny_code": "avatar_file_unreadable"})
	}
	defer src.Close()
	buf := make([]byte, 512)
	n, readErr := src.Read(buf)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "avatar file cannot be read", map[string]string{"deny_code": "avatar_file_unreadable"})
	}
	mimeType := http.DetectContentType(buf[:n])
	if _, ok := allowedMeAvatarMime[mimeType]; !ok {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "avatar only supports JPG, PNG, or WebP", map[string]string{"deny_code": "avatar_file_type_unsupported"})
	}
	return nil
}

func isMeAvatarRequestTooLarge(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "http: request body too large")
}

func meAvatarStorageDir() string {
	if dir := strings.TrimSpace(os.Getenv("USER_AVATAR_DIR")); dir != "" {
		return dir
	}
	return filepath.Join("data", "avatars")
}

func newMeAvatarFilename(ext string) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "avatar-" + hex.EncodeToString(raw) + strings.ToLower(ext), nil
}

func removeStoredMeAvatar(avatarURL string) {
	const prefix = "/v1/me/avatar-files/"
	if !strings.HasPrefix(avatarURL, prefix) {
		return
	}
	filename := strings.TrimPrefix(avatarURL, prefix)
	if filename == "." || filename == "/" || filename == "" || filename != filepath.Base(filename) {
		return
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if _, ok := allowedMeAvatarExt[ext]; !ok {
		return
	}
	_ = os.Remove(filepath.Join(meAvatarStorageDir(), filename))
}

func isManagedMeAvatarFilename(filename string) bool {
	filename = strings.TrimSpace(filename)
	if filename == "" || filename != filepath.Base(filename) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if _, ok := allowedMeAvatarExt[ext]; !ok {
		return false
	}
	stem := strings.TrimSuffix(filename, filepath.Ext(filename))
	const prefix = "avatar-"
	if !strings.HasPrefix(stem, prefix) {
		return false
	}
	token := strings.TrimPrefix(stem, prefix)
	if len(token) != 32 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}
