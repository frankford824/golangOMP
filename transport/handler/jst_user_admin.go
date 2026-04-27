package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

// JSTUserAdminHandler JST 用户管理（预埋能力，仅 Admin 可调用）。
type JSTUserAdminHandler struct {
	erpBridgeSvc service.ERPBridgeService
	importSvc    service.JSTUserImportService
}

// NewJSTUserAdminHandler 创建 JST 用户管理 handler。
func NewJSTUserAdminHandler(erpBridgeSvc service.ERPBridgeService, importSvc service.JSTUserImportService) *JSTUserAdminHandler {
	return &JSTUserAdminHandler{
		erpBridgeSvc: erpBridgeSvc,
		importSvc:    importSvc,
	}
}

// ListJSTUsers 查询 JST 用户列表（通过 Bridge）。
func (h *JSTUserAdminHandler) ListJSTUsers(c *gin.Context) {
	filter, appErr := parseJSTUserListFilter(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.erpBridgeSvc.ListJSTUsers(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

// ImportPreview 预览导入结果。
func (h *JSTUserAdminHandler) ImportPreview(c *gin.Context) {
	var body struct {
		CurrentPage  int    `json:"current_page"`
		PageSize     int    `json:"page_size"`
		PageAction   int    `json:"page_action"`
		Enabled      *bool  `json:"enabled"`
		Version      int    `json:"version"`
		LoginID      string `json:"loginId"`
		CreatedBegin string `json:"creatd_begin"`
		CreatedEnd   string `json:"creatd_end"`
		WriteRoles   bool   `json:"write_roles"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid request body", nil))
		return
	}
	filter := domain.JSTUserListFilter{
		CurrentPage:  body.CurrentPage,
		PageSize:     body.PageSize,
		PageAction:   body.PageAction,
		Enabled:      body.Enabled,
		Version:      body.Version,
		LoginID:      strings.TrimSpace(body.LoginID),
		CreatedBegin: strings.TrimSpace(body.CreatedBegin),
		CreatedEnd:   strings.TrimSpace(body.CreatedEnd),
	}
	if filter.CurrentPage <= 0 {
		filter.CurrentPage = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 50
	}
	if filter.Version <= 0 {
		filter.Version = 2
	}
	opts := service.JSTUserImportOptions{WriteRoles: body.WriteRoles}
	result, appErr := h.importSvc.ImportPreview(c.Request.Context(), filter, opts)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

// Import 执行导入。
func (h *JSTUserAdminHandler) Import(c *gin.Context) {
	var body struct {
		CurrentPage  int    `json:"current_page"`
		PageSize     int    `json:"page_size"`
		PageAction   int    `json:"page_action"`
		Enabled      *bool  `json:"enabled"`
		Version      int    `json:"version"`
		LoginID      string `json:"loginId"`
		CreatedBegin string `json:"creatd_begin"`
		CreatedEnd   string `json:"creatd_end"`
		WriteRoles   bool   `json:"write_roles"`
		DryRun       bool   `json:"dry_run"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid request body", nil))
		return
	}
	filter := domain.JSTUserListFilter{
		CurrentPage:  body.CurrentPage,
		PageSize:     body.PageSize,
		PageAction:   body.PageAction,
		Enabled:      body.Enabled,
		Version:      body.Version,
		LoginID:      strings.TrimSpace(body.LoginID),
		CreatedBegin: strings.TrimSpace(body.CreatedBegin),
		CreatedEnd:   strings.TrimSpace(body.CreatedEnd),
	}
	if filter.CurrentPage <= 0 {
		filter.CurrentPage = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 50
	}
	if filter.Version <= 0 {
		filter.Version = 2
	}
	opts := service.JSTUserImportOptions{WriteRoles: body.WriteRoles}
	result, appErr := h.importSvc.Import(c.Request.Context(), filter, opts, body.DryRun)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}
