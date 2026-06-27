package handler

import (
	"strconv"
	"strings"

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
	items, total, appErr := h.svc.ListProfiles(c.Request.Context(), actor, repo.AssetWorkbenchProfileFilter{
		Keyword:    c.Query("q"),
		WorkerType: c.Query("worker_type"),
		JobGrade:   c.Query("job_grade"),
		Status:     c.Query("status"),
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
	result, appErr := h.svc.UpdateMemberIdentity(c.Request.Context(), actor, userID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListMyTemplates(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	result, appErr := h.svc.ListMyTemplates(c.Request.Context(), actor)
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

func (h *AssetWorkbenchHandler) ListTemplates(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	items, total, appErr := h.svc.ListTemplates(c.Request.Context(), actor, repo.AssetWorkbenchTemplateFilter{
		Keyword:         c.Query("q"),
		Category:        c.Query("category"),
		DifficultyClass: c.Query("difficulty_class"),
		WorkerType:      c.Query("worker_type"),
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

func (h *AssetWorkbenchHandler) CreateTemplate(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.UpsertTemplateParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.CreateTemplate(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) UpdateTemplate(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	templateID, err := strconv.ParseInt(c.Param("template_id"), 10, 64)
	if err != nil || templateID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid template_id", nil))
		return
	}
	var req assetworkbench.UpsertTemplateParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.UpdateTemplate(c.Request.Context(), actor, templateID, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) DeleteTemplate(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	templateID, err := strconv.ParseInt(c.Param("template_id"), 10, 64)
	if err != nil || templateID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid template_id", nil))
		return
	}
	result, appErr := h.svc.DeleteTemplate(c.Request.Context(), actor, templateID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *AssetWorkbenchHandler) ListTemplateAssignments(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	var templateID *int64
	if raw := strings.TrimSpace(c.Query("template_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && parsed > 0 {
			templateID = &parsed
		}
	}
	var targetID *int64
	if raw := strings.TrimSpace(c.Query("target_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && parsed > 0 {
			targetID = &parsed
		}
	}
	items, total, appErr := h.svc.ListTemplateAssignments(c.Request.Context(), actor, repo.AssetWorkbenchTemplateAssignmentFilter{
		TemplateID: templateID,
		TargetType: c.Query("target_type"),
		TargetID:   targetID,
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

func (h *AssetWorkbenchHandler) AssignTemplate(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	var req assetworkbench.AssignTemplateParams
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.svc.AssignTemplate(c.Request.Context(), actor, req)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, result)
}

func (h *AssetWorkbenchHandler) DeleteTemplateAssignment(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	assignmentID, err := strconv.ParseInt(c.Param("assignment_id"), 10, 64)
	if err != nil || assignmentID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid assignment_id", nil))
		return
	}
	result, appErr := h.svc.DeleteTemplateAssignment(c.Request.Context(), actor, assignmentID)
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
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil && value > 0 {
			payeeID = &value
		}
	}
	items, total, appErr := h.svc.ListSubmissions(c.Request.Context(), actor, repo.AssetWorkbenchSubmissionFilter{
		SubmitterUserID:  submitterID,
		PayeeUserID:      payeeID,
		BusinessMonth:    c.Query("business_month"),
		Status:           c.Query("status"),
		SettlementStatus: c.Query("settlement_status"),
		Page:             page,
		PageSize:         pageSize,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, gin.H{"total": total, "page": page, "page_size": pageSize})
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
		PayeeUserID:   payeeID,
		BusinessMonth: c.Query("business_month"),
		OrderNo:       c.Query("order_no"),
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

func (h *AssetWorkbenchHandler) SystemSearch(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	result, appErr := h.svc.SystemSearch(c.Request.Context(), actor, c.Query("q"), limit)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
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
