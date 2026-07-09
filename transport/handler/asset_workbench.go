package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/repo"
	assetworkbench "workflow/service/asset_workbench"
)

type AssetWorkbenchHandler struct {
	svc               *assetworkbench.Service
	assetCookieDomain string
}

func NewAssetWorkbenchHandler(svc *assetworkbench.Service, assetCookieDomains ...string) *AssetWorkbenchHandler {
	assetCookieDomain := ""
	if len(assetCookieDomains) > 0 {
		assetCookieDomain = strings.TrimSpace(assetCookieDomains[0])
	}
	return &AssetWorkbenchHandler{svc: svc, assetCookieDomain: assetCookieDomain}
}

func (h *AssetWorkbenchHandler) Register(c *gin.Context) {
	if h == nil || h.svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench service is not configured.", nil))
		return
	}
	var req assetworkbench.RegisterParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.Register(c.Request.Context(), req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	if result != nil {
		setAssetFilesTokenCookie(c, result.Auth, h.assetCookieDomain)
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) Bootstrap(c *gin.Context) {
	if h == nil || h.svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench service is not configured.", nil))
		return
	}
	actor, ok := domain.RequestActorFromContext(c.Request.Context())
	if !ok || !domain.IsSessionBackedRequestActor(actor) {
		respondError(c, domain.NewAppError(domain.ErrCodeUnauthorized, "Authentication required.", nil))
		return
	}
	result, appErr := h.svc.Bootstrap(c.Request.Context(), actor)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) Entry(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var result *assetworkbench.EntryResponse
	result, appErr := h.svc.Entry(c.Request.Context(), actor)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) RequireActiveMembership() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.svc == nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench service is not configured.", nil))
			return
		}
		actor, ok := domain.RequestActorFromContext(c.Request.Context())
		if !ok || !domain.IsSessionBackedRequestActor(actor) {
			respondError(c, domain.NewAppError(domain.ErrCodeUnauthorized, "Authentication required.", nil))
			return
		}
		access, appErr := h.svc.ResolveAssetWorkbenchAccess(c.Request.Context(), actor)
		if appErr != nil {
			respondError(c, appErr)
			return
		}
		if access == nil || !access.IsEnabled {
			status := ""
			reason := "Asset workbench access is not active."
			if access != nil {
				status = access.MembershipStatus
				if strings.TrimSpace(access.DeniedReason) != "" {
					reason = access.DeniedReason
				}
			}
			respondError(c, domain.NewAppError(domain.ErrCodePermissionDenied, reason, gin.H{"membership_status": status}))
			return
		}
		c.Next()
	}
}

func (h *AssetWorkbenchHandler) RequestAccess(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.AccessRequestParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	var result *domain.AppMembership
	result, appErr := h.svc.RequestAccess(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) OpenAccess(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.AccessOpenParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	var result *domain.AppMembership
	result, appErr := h.svc.OpenAccess(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) DisableAccess(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.AccessDisableParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	var result *domain.AppMembership
	result, appErr := h.svc.DisableAccess(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) UpsertMyProfile(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.UpsertProfileParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.UpsertMyProfile(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListProfiles(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	userID, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	items, total, appErr := h.svc.ListProfiles(c.Request.Context(), actor, repo.AssetWorkbenchProfileFilter{
		Keyword:    c.Query("q"),
		WorkerType: c.Query("worker_type"),
		JobGrade:   c.Query("job_grade"),
		Status:     c.Query("status"),
		UserID:     userID,
		Page:       page,
		PageSize:   pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) UpsertProfile(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(c.Param("user_id")), 10, 64)
	if err != nil || userID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user_id", nil))
		return
	}
	var req assetworkbench.UpsertProfileParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.HRUpsertProfile(c.Request.Context(), actor, userID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListMembers(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	items, total, appErr := h.svc.ListMembers(c.Request.Context(), actor, repo.AssetWorkbenchMemberFilter{
		Keyword:  c.Query("q"),
		Identity: c.Query("identity"),
		Page:     page,
		PageSize: pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) SearchPeople(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	items, total, appErr := h.svc.SearchPeople(c.Request.Context(), actor, repo.AssetWorkbenchMemberFilter{
		Keyword:  c.Query("q"),
		Identity: c.Query("identity"),
		Page:     page,
		PageSize: pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) UpdateMemberIdentity(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(c.Param("user_id")), 10, 64)
	if err != nil || userID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user_id", nil))
		return
	}
	var req assetworkbench.UpdateMemberIdentityParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	var result *domain.AssetWorkbenchMember
	result, appErr := h.svc.UpdateMemberIdentity(c.Request.Context(), actor, userID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) UpdateMemberRoles(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(c.Param("user_id")), 10, 64)
	if err != nil || userID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid user_id", nil))
		return
	}
	var req assetworkbench.UpdateMemberRolesParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	var result *domain.AssetWorkbenchMember
	result, appErr := h.svc.UpdateMemberRoles(c.Request.Context(), actor, userID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) PreviewAccountMerge(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.AccountMergePreviewParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	var result *assetworkbench.AccountMergePreview
	result, appErr := h.svc.PreviewAccountMerge(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) MergeAccounts(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.AccountMergeParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	var result *assetworkbench.AccountMergePreview
	result, appErr := h.svc.MergeAccounts(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListGroups(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	items, total, appErr := h.svc.ListGroups(c.Request.Context(), actor, repo.AssetWorkbenchGroupFilter{
		Keyword:  c.Query("q"),
		Enabled:  parseOptionalBool(c.Query("enabled")),
		Page:     page,
		PageSize: pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) CreateGroup(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.UpsertGroupParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.CreateGroup(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) UpdateGroup(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	groupID, err := strconv.ParseInt(c.Param("group_id"), 10, 64)
	if err != nil || groupID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid group_id", nil))
		return
	}
	var req assetworkbench.UpsertGroupParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.UpdateGroup(c.Request.Context(), actor, groupID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) DeleteGroup(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	groupID, err := strconv.ParseInt(c.Param("group_id"), 10, 64)
	if err != nil || groupID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid group_id", nil))
		return
	}
	result, appErr := h.svc.DeleteGroup(c.Request.Context(), actor, groupID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) AddGroupMembers(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	groupID, err := strconv.ParseInt(c.Param("group_id"), 10, 64)
	if err != nil || groupID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid group_id", nil))
		return
	}
	var req assetworkbench.GroupMembersParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	if appErr := h.svc.AddGroupMembers(c.Request.Context(), actor, groupID, req); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"status": "ok"})
}

func (h *AssetWorkbenchHandler) RemoveGroupMembers(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	groupID, err := strconv.ParseInt(c.Param("group_id"), 10, 64)
	if err != nil || groupID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid group_id", nil))
		return
	}
	var req assetworkbench.GroupMembersParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	if appErr := h.svc.RemoveGroupMembers(c.Request.Context(), actor, groupID, req); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"status": "ok"})
}

func (h *AssetWorkbenchHandler) ListGroupMembers(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	groupID, err := strconv.ParseInt(c.Param("group_id"), 10, 64)
	if err != nil || groupID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid group_id", nil))
		return
	}
	result, appErr := h.svc.ListGroupMembers(c.Request.Context(), actor, groupID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListUploadDirectories(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	result, appErr := h.svc.ListUploadDirectories(c.Request.Context(), actor, false)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListUploadDirectoriesAdmin(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	result, appErr := h.svc.ListUploadDirectories(c.Request.Context(), actor, true)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) CreateUploadDirectory(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.CreateUploadDirectoryParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.CreateUploadDirectory(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) UpdateUploadDirectory(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	directoryID, err := strconv.ParseInt(c.Param("directory_id"), 10, 64)
	if err != nil || directoryID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid directory_id", nil))
		return
	}
	var req assetworkbench.UpdateUploadDirectoryParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.UpdateUploadDirectory(c.Request.Context(), actor, directoryID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListDifficultyClasses(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	result, appErr := h.svc.ListDifficultyClasses(c.Request.Context(), actor, false)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListDifficultyClassesAdmin(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	result, appErr := h.svc.ListDifficultyClasses(c.Request.Context(), actor, true)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) CreateDifficultyClass(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.CreateDifficultyClassParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.CreateDifficultyClass(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) UpdateDifficultyClass(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	code := c.Param("difficulty_code")
	var req assetworkbench.UpdateDifficultyClassParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.UpdateDifficultyClass(c.Request.Context(), actor, code, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListPriceMatrix(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	var enabled *bool
	if raw := strings.TrimSpace(c.Query("enabled")); raw != "" {
		parsed := raw == "1" || strings.EqualFold(raw, "true")
		enabled = &parsed
	}
	items, total, appErr := h.svc.ListPriceMatrix(c.Request.Context(), actor, repo.AssetWorkbenchPriceMatrixFilter{
		WorkerType:      c.Query("worker_type"),
		JobGrade:        c.Query("job_grade"),
		DifficultyClass: c.Query("difficulty_class"),
		Enabled:         enabled,
		Page:            page,
		PageSize:        pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) CreatePriceMatrix(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.CreatePriceMatrixParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.CreatePriceMatrix(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) UpdatePriceMatrix(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	ruleID, err := strconv.ParseInt(c.Param("rule_id"), 10, 64)
	if err != nil || ruleID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "Invalid price matrix rule id.", nil))
		return
	}
	var req assetworkbench.SetCostRuleEnabledParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.SetPriceMatrixEnabled(c.Request.Context(), actor, ruleID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) SupersedePriceMatrix(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	ruleID, err := strconv.ParseInt(c.Param("rule_id"), 10, 64)
	if err != nil || ruleID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "Invalid price matrix rule id.", nil))
		return
	}
	var req assetworkbench.CreatePriceMatrixParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.SupersedePriceMatrix(c.Request.Context(), actor, ruleID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) ListDeductionRules(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	items, total, appErr := h.svc.ListDeductionRules(c.Request.Context(), actor, repo.AssetWorkbenchDeductionRuleFilter{
		WorkerType:      c.Query("worker_type"),
		JobGrade:        c.Query("job_grade"),
		DifficultyClass: c.Query("difficulty_class"),
		Enabled:         parseOptionalBool(c.Query("enabled")),
		Page:            page,
		PageSize:        pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) CreateDeductionRule(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.CreateDeductionRuleParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.CreateDeductionRule(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) UpdateDeductionRule(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	ruleID, err := strconv.ParseInt(c.Param("rule_id"), 10, 64)
	if err != nil || ruleID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "Invalid deduction rule id.", nil))
		return
	}
	var req assetworkbench.SetCostRuleEnabledParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.SetDeductionRuleEnabled(c.Request.Context(), actor, ruleID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) SupersedeDeductionRule(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	ruleID, err := strconv.ParseInt(c.Param("rule_id"), 10, 64)
	if err != nil || ruleID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "Invalid deduction rule id.", nil))
		return
	}
	var req assetworkbench.CreateDeductionRuleParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.SupersedeDeductionRule(c.Request.Context(), actor, ruleID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) ListWelfareRules(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	items, total, appErr := h.svc.ListWelfareRules(c.Request.Context(), actor, repo.AssetWorkbenchWelfareRuleFilter{
		WorkerType: c.Query("worker_type"),
		JobGrade:   c.Query("job_grade"),
		RuleType:   c.Query("rule_type"),
		Enabled:    parseOptionalBool(c.Query("enabled")),
		Page:       page,
		PageSize:   pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) CreateWelfareRule(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.CreateWelfareRuleParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.CreateWelfareRule(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) UpdateWelfareRule(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	ruleID, err := strconv.ParseInt(c.Param("rule_id"), 10, 64)
	if err != nil || ruleID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "Invalid welfare rule id.", nil))
		return
	}
	var req assetworkbench.SetCostRuleEnabledParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.SetWelfareRuleEnabled(c.Request.Context(), actor, ruleID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) SupersedeWelfareRule(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	ruleID, err := strconv.ParseInt(c.Param("rule_id"), 10, 64)
	if err != nil || ruleID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "Invalid welfare rule id.", nil))
		return
	}
	var req assetworkbench.CreateWelfareRuleParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.SupersedeWelfareRule(c.Request.Context(), actor, ruleID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) ListPromoCoupons(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	items, total, appErr := h.svc.ListPromoCoupons(c.Request.Context(), actor, repo.AssetWorkbenchPromoCouponFilter{
		WorkerType:      c.Query("worker_type"),
		JobGrade:        c.Query("job_grade"),
		DifficultyClass: c.Query("difficulty_class"),
		Enabled:         parseOptionalBool(c.Query("enabled")),
		Page:            page,
		PageSize:        pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) CreatePromoCoupon(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.CreatePromoCouponParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.CreatePromoCoupon(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) UpdatePromoCoupon(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	ruleID, err := strconv.ParseInt(c.Param("rule_id"), 10, 64)
	if err != nil || ruleID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "Invalid promo coupon id.", nil))
		return
	}
	var req assetworkbench.SetCostRuleEnabledParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.SetPromoCouponEnabled(c.Request.Context(), actor, ruleID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) SupersedePromoCoupon(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	ruleID, err := strconv.ParseInt(c.Param("rule_id"), 10, 64)
	if err != nil || ruleID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "Invalid promo coupon id.", nil))
		return
	}
	var req assetworkbench.CreatePromoCouponParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.SupersedePromoCoupon(c.Request.Context(), actor, ruleID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) CreateUploadSession(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.CreateUploadSessionParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.CreateUploadSession(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) CompleteUploadSession(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.CompleteUploadSessionParams
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
			return
		}
	}
	result, appErr := h.svc.CompleteUploadSession(c.Request.Context(), actor, c.Param("session_id"), req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) CancelUploadSession(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	result, appErr := h.svc.CancelUploadSession(c.Request.Context(), actor, c.Param("session_id"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) CreateSubmission(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.CreateSubmissionParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.CreateSubmission(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) ListSubmissions(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	var submitterID *int64
	if raw := strings.TrimSpace(c.Query("submitter_user_id")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil && value > 0 {
			submitterID = &value
		}
	}
	var payeeID *int64
	if raw := strings.TrimSpace(c.Query("payee_user_id")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "payee_user_id must be a positive integer.", nil))
			return
		}
		payeeID = &value
	}
	items, total, appErr := h.svc.ListSubmissions(c.Request.Context(), actor, repo.AssetWorkbenchSubmissionFilter{
		SubmitterUserID:  submitterID,
		PayeeUserID:      payeeID,
		BusinessMonth:    c.Query("business_month"),
		Status:           c.Query("status"),
		SettlementStatus: c.Query("settlement_status"),
		OrderBy:          c.Query("order_by"),
		OrderDir:         c.Query("order_dir"),
		Page:             page,
		PageSize:         pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) OverviewSearch(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	createdFrom, appErr := parseAssetWorkbenchOverviewDate(c.Query("date_from"), false)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	createdTo, appErr := parseAssetWorkbenchOverviewDate(c.Query("date_to"), true)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.svc.OverviewSearch(c.Request.Context(), actor, assetworkbench.OverviewSearchParams{
		Query:       c.Query("q"),
		Scope:       c.Query("scope"),
		Creator:     c.Query("creator"),
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
		Page:        page,
		PageSize:    pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) GetSubmissionDetail(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	submissionID, err := strconv.ParseInt(c.Param("submission_id"), 10, 64)
	if err != nil || submissionID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid submission_id", nil))
		return
	}
	result, appErr := h.svc.GetSubmissionDetail(c.Request.Context(), actor, submissionID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) VoidSubmission(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	submissionID, err := strconv.ParseInt(c.Param("submission_id"), 10, 64)
	if err != nil || submissionID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid submission_id", nil))
		return
	}
	var req assetworkbench.VoidSubmissionParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.VoidSubmission(c.Request.Context(), actor, submissionID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) GetFilePreview(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	fileID, err := strconv.ParseInt(c.Param("file_id"), 10, 64)
	if err != nil || fileID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid file_id", nil))
		return
	}
	result, appErr := h.svc.GetFilePreview(c.Request.Context(), actor, fileID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) GetFileDownload(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	fileID, err := strconv.ParseInt(c.Param("file_id"), 10, 64)
	if err != nil || fileID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid file_id", nil))
		return
	}
	result, appErr := h.svc.GetFileDownload(c.Request.Context(), actor, fileID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) BatchDownloadFiles(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.BatchDownloadFilesParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.BuildFileBatchDownloadManifest(c.Request.Context(), actor, req.FileIDs)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) BatchMoveFiles(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.BatchMoveFilesParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.BatchMoveFiles(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) BatchDeleteFiles(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.BatchDeleteFilesParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.BatchDeleteFiles(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) DownloadSystemAsset(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	assetID, err := strconv.ParseInt(strings.TrimSpace(c.Param("asset_id")), 10, 64)
	if err != nil || assetID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset_id", nil))
		return
	}
	result, appErr := h.svc.SystemAssetDownload(c.Request.Context(), actor, assetID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) PreviewSystemAsset(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	assetID, err := strconv.ParseInt(strings.TrimSpace(c.Param("asset_id")), 10, 64)
	if err != nil || assetID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset_id", nil))
		return
	}
	result, appErr := h.svc.SystemAssetPreview(c.Request.Context(), actor, assetID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) BatchDownloadSystemAssets(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.SystemAssetBatchDownloadParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.SystemAssetBatchDownloadManifest(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) UpdateSubmissionItem(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	itemID, err := strconv.ParseInt(c.Param("item_id"), 10, 64)
	if err != nil || itemID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid item_id", nil))
		return
	}
	var req assetworkbench.UpdateSubmissionItemParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.UpdateSubmissionItem(c.Request.Context(), actor, itemID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) UpdateSubmissionItemQC(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	itemID, err := strconv.ParseInt(c.Param("item_id"), 10, 64)
	if err != nil || itemID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid item_id", nil))
		return
	}
	var req assetworkbench.UpdateSubmissionItemQCParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.UpdateSubmissionItemQC(c.Request.Context(), actor, itemID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ImportSubmissionItemQCExcel(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	businessMonth := strings.TrimSpace(c.PostForm("business_month"))
	if businessMonth == "" {
		businessMonth = strings.TrimSpace(c.Query("business_month"))
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "file is required.", err.Error()))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "failed to open uploaded file.", err.Error()))
		return
	}
	defer file.Close()
	result, appErr := h.svc.ImportSubmissionItemQCExcel(c.Request.Context(), actor, businessMonth, file)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) VoidSubmissionItem(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	itemID, err := strconv.ParseInt(c.Param("item_id"), 10, 64)
	if err != nil || itemID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid item_id", nil))
		return
	}
	var req assetworkbench.VoidSubmissionItemParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.VoidSubmissionItem(c.Request.Context(), actor, itemID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) RepriceSubmissionItem(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	itemID, err := strconv.ParseInt(c.Param("item_id"), 10, 64)
	if err != nil || itemID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid item_id", nil))
		return
	}
	var req assetworkbench.RepriceSubmissionItemParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.RepriceSubmissionItem(c.Request.Context(), actor, itemID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ImportErrorRecords(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.ImportErrorRecordsParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.ImportErrorRecords(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) ImportErrorRecordsExcel(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "file is required.", err.Error()))
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "failed to open uploaded file.", err.Error()))
		return
	}
	defer src.Close()
	result, appErr := h.svc.ImportErrorRecordsExcel(c.Request.Context(), actor, c.PostForm("business_month"), fileHeader.Filename, src)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) MySettlement(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	result, appErr := h.svc.MySettlement(c.Request.Context(), actor)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) PreviewSettlement(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	result, appErr := h.svc.PreviewSettlement(c.Request.Context(), actor, c.Query("business_month"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) SettlementReport(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var result *assetworkbench.SettlementReport
	result, appErr := h.svc.SettlementReport(c.Request.Context(), actor, c.Query("business_month"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListSettlementBatches(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	items, total, appErr := h.svc.ListSettlementBatches(c.Request.Context(), actor, repo.AssetWorkbenchSettlementBatchFilter{
		BusinessMonth: c.Query("business_month"),
		Status:        c.Query("status"),
		Page:          page,
		PageSize:      pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) GetSettlementBatchDetail(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	batchID, err := strconv.ParseInt(c.Param("batch_id"), 10, 64)
	if err != nil || batchID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid batch_id", nil))
		return
	}
	result, appErr := h.svc.GetSettlementBatchDetail(c.Request.Context(), actor, batchID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) GenerateSettlementBatch(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req struct {
		BusinessMonth string `json:"business_month"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.GenerateSettlementBatch(c.Request.Context(), actor, req.BusinessMonth)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) ConfirmSettlementBatch(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	batchID, err := strconv.ParseInt(c.Param("batch_id"), 10, 64)
	if err != nil || batchID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid batch_id", nil))
		return
	}
	if appErr := h.svc.ConfirmSettlementBatch(c.Request.Context(), actor, batchID); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"status": "ok"})
}

func (h *AssetWorkbenchHandler) CancelSettlementBatch(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	batchID, err := strconv.ParseInt(c.Param("batch_id"), 10, 64)
	if err != nil || batchID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid batch_id", nil))
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	if appErr := h.svc.CancelSettlementBatch(c.Request.Context(), actor, batchID, req.Reason); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"status": "ok"})
}

func (h *AssetWorkbenchHandler) CreateSettlementAdjustment(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	batchID, err := strconv.ParseInt(c.Param("batch_id"), 10, 64)
	if err != nil || batchID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid batch_id", nil))
		return
	}
	var req assetworkbench.CreateSettlementAdjustmentParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	req.BatchID = batchID
	result, appErr := h.svc.CreateSettlementAdjustment(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) ListSupplementPermissions(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var payeeID *int64
	if raw := strings.TrimSpace(c.Query("payee_user_id")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil && value > 0 {
			payeeID = &value
		}
	}
	var enabled *bool
	if raw := strings.TrimSpace(c.Query("enabled")); raw != "" {
		if value, err := strconv.ParseBool(raw); err == nil {
			enabled = &value
		}
	}
	items, total, appErr := h.svc.ListSupplementPermissions(c.Request.Context(), actor, repo.AssetWorkbenchSupplementPermissionFilter{
		PayeeUserID:   payeeID,
		BusinessMonth: c.Query("business_month"),
		Enabled:       enabled,
		Page:          page,
		PageSize:      pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) ListSupplementEligibleMonths(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	payeeID, err := strconv.ParseInt(strings.TrimSpace(c.Query("payee_user_id")), 10, 64)
	if err != nil || payeeID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "payee_user_id is required.", nil))
		return
	}
	months, appErr := h.svc.ListSupplementEligibleMonths(c.Request.Context(), actor, payeeID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"months": months})
}

func (h *AssetWorkbenchHandler) UpsertSupplementPermission(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.UpsertSupplementPermissionParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.UpsertSupplementPermission(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListSettlementSupplements(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	var payeeID *int64
	if raw := strings.TrimSpace(c.Query("payee_user_id")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil && value > 0 {
			payeeID = &value
		}
	}
	items, total, appErr := h.svc.ListSettlementSupplements(c.Request.Context(), actor, repo.AssetWorkbenchSettlementSupplementFilter{
		PayeeUserID:        payeeID,
		BusinessMonth:      c.Query("business_month"),
		OrderNo:            c.Query("order_no"),
		Status:             c.Query("status"),
		SupplementDate:     c.Query("supplement_date"),
		SupplementDateFrom: c.Query("supplement_date_from"),
		SupplementDateTo:   c.Query("supplement_date_to"),
		SortBy:             c.Query("sort_by"),
		SortDir:            c.Query("sort_dir"),
		Page:               page,
		PageSize:           pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) CreateSettlementSupplement(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.CreateSettlementSupplementParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.CreateSettlementSupplement(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) ImportSettlementSupplementsExcel(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	businessMonth := strings.TrimSpace(c.PostForm("business_month"))
	if businessMonth == "" {
		businessMonth = strings.TrimSpace(c.Query("business_month"))
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "file is required.", err.Error()))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "failed to open uploaded file.", err.Error()))
		return
	}
	defer file.Close()
	result, appErr := h.svc.ImportSettlementSupplementsExcel(c.Request.Context(), actor, businessMonth, file)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) DeleteSettlementSupplement(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	supplementID, err := strconv.ParseInt(c.Param("supplement_id"), 10, 64)
	if err != nil || supplementID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "Invalid supplement id.", nil))
		return
	}
	req := struct {
		Reason string `json:"reason"`
	}{Reason: strings.TrimSpace(c.Query("reason"))}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
			return
		}
	}
	result, appErr := h.svc.VoidSettlementSupplement(c.Request.Context(), actor, supplementID, req.Reason)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListEvents(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	var entityID *int64
	if raw := strings.TrimSpace(c.Query("entity_id")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil && value > 0 {
			entityID = &value
		}
	}
	var actorID *int64
	if raw := strings.TrimSpace(c.Query("actor_user_id")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil && value > 0 {
			actorID = &value
		}
	}
	items, total, appErr := h.svc.ListEvents(c.Request.Context(), actor, repo.AssetWorkbenchEventFilter{
		EventType:  c.Query("event_type"),
		EntityType: c.Query("entity_type"),
		EntityID:   entityID,
		ActorID:    actorID,
		Page:       page,
		PageSize:   pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
}

func (h *AssetWorkbenchHandler) ListSavedViews(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	items, appErr := h.svc.ListSavedViews(c.Request.Context(), actor, c.Query("view_type"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func (h *AssetWorkbenchHandler) UpsertSavedView(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.UpsertSavedViewParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.UpsertSavedView(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) DeleteSavedView(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	viewID, err := strconv.ParseInt(c.Param("view_id"), 10, 64)
	if err != nil || viewID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid view_id", nil))
		return
	}
	if appErr := h.svc.DeleteSavedView(c.Request.Context(), actor, viewID); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"status": "ok"})
}

func (h *AssetWorkbenchHandler) ListClientMaterials(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	admin := c.Query("admin") == "1" || strings.EqualFold(c.Query("admin"), "true") || strings.EqualFold(c.Query("scope"), "admin")
	result, appErr := h.svc.ListClientMaterials(c.Request.Context(), actor, admin)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) SearchClientMaterials(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	admin := c.Query("admin") == "1" || strings.EqualFold(c.Query("admin"), "true") || strings.EqualFold(c.Query("scope"), "admin")
	result, appErr := h.svc.SearchClientMaterials(c.Request.Context(), actor, assetworkbench.ClientMaterialSearchParams{
		Query:    c.Query("q"),
		SKU:      c.Query("sku"),
		Creator:  c.Query("creator"),
		Admin:    admin,
		Page:     page,
		PageSize: pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListBatchJobs(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	result, appErr := h.svc.ListBatchJobs(c.Request.Context(), actor, c.Query("status"), page, pageSize)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) GetBatchJob(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	result, appErr := h.svc.GetBatchJob(c.Request.Context(), actor, c.Param("job_id"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) CreateClientMaterial(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.CreateClientMaterialParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.CreateClientMaterial(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) BatchUpdateClientMaterials(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.BatchUpdateClientMaterialsParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.BatchUpdateClientMaterials(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) UpdateClientMaterial(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	materialID, err := strconv.ParseInt(c.Param("material_id"), 10, 64)
	if err != nil || materialID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid material_id", nil))
		return
	}
	var req assetworkbench.UpdateClientMaterialParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.UpdateClientMaterial(c.Request.Context(), actor, materialID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) DeleteClientMaterial(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	materialID, err := strconv.ParseInt(c.Param("material_id"), 10, 64)
	if err != nil || materialID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid material_id", nil))
		return
	}
	if appErr := h.svc.DeleteClientMaterial(c.Request.Context(), actor, materialID); appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, gin.H{"status": "ok"})
}

func (h *AssetWorkbenchHandler) DownloadClientMaterial(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	materialID, err := strconv.ParseInt(c.Param("material_id"), 10, 64)
	if err != nil || materialID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid material_id", nil))
		return
	}
	result, appErr := h.svc.ClientMaterialDownload(c.Request.Context(), actor, materialID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) PreviewClientMaterial(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	materialID, err := strconv.ParseInt(c.Param("material_id"), 10, 64)
	if err != nil || materialID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid material_id", nil))
		return
	}
	result, appErr := h.svc.ClientMaterialPreview(c.Request.Context(), actor, materialID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) BatchDownloadClientMaterials(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.ClientMaterialBatchDownloadParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.ClientMaterialBatchDownloadManifest(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) MaterialGroups(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	result, appErr := h.svc.MaterialGroups(c.Request.Context(), actor, assetworkbench.MaterialGroupSearchParams{
		Query:          c.Query("q"),
		Source:         c.Query("source"),
		FormatCategory: c.Query("format_category"),
		Page:           page,
		PageSize:       pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) MaterialGroupFiles(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	result, appErr := h.svc.MaterialGroupFiles(c.Request.Context(), actor, c.Query("group_key"), page, pageSize)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) BrowseMaterials(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if pageSize <= 0 {
		pageSize = limit
	}
	result, appErr := h.svc.BrowseMaterials(c.Request.Context(), actor, c.Query("path"), page, pageSize, c.Query("source"), c.Query("format_category"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) SystemSearch(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if pageSize <= 0 {
		pageSize = limit
	}
	result, appErr := h.svc.SystemSearch(c.Request.Context(), actor, c.Query("q"), page, pageSize, c.Query("source"), c.Query("format_category"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func parseAssetWorkbenchOverviewDate(raw string, endOfDay bool) (*time.Time, *domain.AppError) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		utc := parsed.UTC()
		return &utc, nil
	}
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	parsed, err := time.ParseInLocation("2006-01-02", value, loc)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "date_from/date_to must be RFC3339 or YYYY-MM-DD.", map[string]string{"value": value})
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	utc := parsed.UTC()
	return &utc, nil
}

func (h *AssetWorkbenchHandler) sessionActor(c *gin.Context) (domain.RequestActor, bool) {
	if h == nil || h.svc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "Asset workbench service is not configured.", nil))
		return domain.RequestActor{}, false
	}
	actor, ok := domain.RequestActorFromContext(c.Request.Context())
	if !ok || !domain.IsSessionBackedRequestActor(actor) {
		respondError(c, domain.NewAppError(domain.ErrCodeUnauthorized, "Authentication required.", nil))
		return domain.RequestActor{}, false
	}
	return actor, true
}

func parseOptionalBool(raw string) *bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed := raw == "1" || strings.EqualFold(raw, "true")
	return &parsed
}
