package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
	assetcenter "workflow/service/asset_center"
	assetlifecycle "workflow/service/asset_lifecycle"
)

type TaskAssetCenterHandler struct {
	svc          service.TaskAssetCenterService
	globalSvc    *assetcenter.Service
	lifecycleSvc *assetlifecycle.Service
}

func NewTaskAssetCenterHandler(svc service.TaskAssetCenterService) *TaskAssetCenterHandler {
	return &TaskAssetCenterHandler{svc: svc}
}

func (h *TaskAssetCenterHandler) SetGlobalAssetServices(globalSvc *assetcenter.Service, lifecycleSvc *assetlifecycle.Service) {
	h.globalSvc = globalSvc
	h.lifecycleSvc = lifecycleSvc
}

type createTaskAssetUploadSessionReq struct {
	TaskID        *int64 `json:"task_id"`
	CreatedBy     *int64 `json:"created_by"`
	AssetID       *int64 `json:"asset_id"`
	SourceAssetID *int64 `json:"source_asset_id"`
	AssetType     string `json:"asset_type"`
	AssetKind     string `json:"asset_kind"`
	UploadMode    string `json:"upload_mode"`
	Filename      string `json:"filename"`
	FileName      string `json:"file_name"`
	ExpectedSize  *int64 `json:"expected_size"`
	FileSize      *int64 `json:"file_size"`
	MimeType      string `json:"mime_type"`
	FileHash      string `json:"file_hash"`
	Remark        string `json:"remark"`
	TargetSKUCode string `json:"target_sku_code"`
}

type completeTaskAssetUploadSessionReq struct {
	CompletedBy       *int64                    `json:"completed_by"`
	FileHash          string                    `json:"file_hash"`
	Remark            string                    `json:"remark"`
	UploadContentType string                    `json:"upload_content_type,omitempty"`
	OSSParts          []service.OSSCompletePart `json:"oss_parts,omitempty"`
	OSSUploadID       string                    `json:"oss_upload_id,omitempty"`
	OSSObjectKey      string                    `json:"oss_object_key,omitempty"`
}

type cancelTaskAssetUploadSessionReq struct {
	CancelledBy *int64 `json:"cancelled_by"`
	Remark      string `json:"remark"`
}

func (h *TaskAssetCenterHandler) ListAssets(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	assets, appErr := h.svc.ListAssets(c.Request.Context(), taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, assets)
}

func (h *TaskAssetCenterHandler) ListAssetResources(c *gin.Context) {
	var taskID *int64
	var sourceAssetID *int64
	if rawTaskID := strings.TrimSpace(c.Query("task_id")); rawTaskID != "" {
		parsedTaskID, err := parseInt64(rawTaskID)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task_id", nil))
			return
		}
		taskID = &parsedTaskID
	}
	if rawSourceAssetID := strings.TrimSpace(c.Query("source_asset_id")); rawSourceAssetID != "" {
		parsedSourceAssetID, err := parseInt64(rawSourceAssetID)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid source_asset_id", nil))
			return
		}
		sourceAssetID = &parsedSourceAssetID
	}
	params := service.ListAssetResourcesParams{
		TaskID:        taskID,
		SourceAssetID: sourceAssetID,
		ScopeSKUCode:  strings.TrimSpace(c.Query("scope_sku_code")),
	}
	if assetType := normalizeAssetTypeInput(c.Query("asset_kind"), c.Query("asset_type")); assetType != "" {
		params.AssetType = domain.TaskAssetType(assetType)
	}
	if archiveStatus := domain.AssetArchiveStatus(strings.TrimSpace(c.Query("archive_status"))); archiveStatus.Valid() {
		params.ArchiveStatus = archiveStatus
	}
	if uploadStatus := domain.DesignAssetUploadStatus(strings.TrimSpace(c.Query("upload_status"))); uploadStatus.Valid() {
		params.UploadStatus = uploadStatus
	}
	assets, appErr := h.svc.ListAssetResources(c.Request.Context(), params)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, assets)
}

func (h *TaskAssetCenterHandler) SearchGlobalAssets(c *gin.Context) {
	if h.globalSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "asset center service is not configured", nil))
		return
	}
	query, appErr := parseAssetSearchQuery(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.globalSvc.Search(c.Request.Context(), query)
	if appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  result.Items,
		"total": result.Total,
		"page":  result.Page,
		"size":  result.Size,
	})
}

func (h *TaskAssetCenterHandler) GetGlobalAsset(c *gin.Context) {
	if h.globalSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "asset center service is not configured", nil))
		return
	}
	assetID, err := parseInt64(strings.TrimSpace(c.Param("asset_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset id", nil))
		return
	}
	detail, appErr := h.globalSvc.GetDetail(c.Request.Context(), assetID)
	if appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	respondOK(c, detail)
}

func (h *TaskAssetCenterHandler) DownloadGlobalAsset(c *gin.Context) {
	if h.globalSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "asset center service is not configured", nil))
		return
	}
	assetID, err := parseInt64(strings.TrimSpace(c.Param("asset_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset id", nil))
		return
	}
	info, appErr := h.globalSvc.DownloadLatest(c.Request.Context(), assetID)
	if appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	respondOK(c, info)
}

func (h *TaskAssetCenterHandler) DownloadGlobalAssetVersion(c *gin.Context) {
	if h.globalSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "asset center service is not configured", nil))
		return
	}
	assetID, err := parseInt64(strings.TrimSpace(c.Param("asset_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset id", nil))
		return
	}
	versionID, err := parseInt64(strings.TrimSpace(c.Param("version_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid version id", nil))
		return
	}
	info, appErr := h.globalSvc.DownloadVersion(c.Request.Context(), assetID, versionID)
	if appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	respondOK(c, info)
}

type assetReasonReq struct {
	Reason string `json:"reason"`
}

func (h *TaskAssetCenterHandler) ArchiveGlobalAsset(c *gin.Context) {
	assetID, err := parseInt64(strings.TrimSpace(c.Param("asset_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset id", nil))
		return
	}
	var req assetReasonReq
	_ = c.ShouldBindJSON(&req)
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	if h.lifecycleSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "asset lifecycle service is not configured", nil))
		return
	}
	if appErr := h.lifecycleSvc.Archive(c.Request.Context(), actor, assetID, req.Reason); appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"asset_id": assetID, "archived": true}})
}

func (h *TaskAssetCenterHandler) RestoreGlobalAsset(c *gin.Context) {
	if h.lifecycleSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "asset lifecycle service is not configured", nil))
		return
	}
	assetID, err := parseInt64(strings.TrimSpace(c.Param("asset_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset id", nil))
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	if appErr := h.lifecycleSvc.Restore(c.Request.Context(), actor, assetID); appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"asset_id": assetID, "archived": false}})
}

func (h *TaskAssetCenterHandler) DeleteGlobalAsset(c *gin.Context) {
	if h.lifecycleSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "asset lifecycle service is not configured", nil))
		return
	}
	assetID, err := parseInt64(strings.TrimSpace(c.Param("asset_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset id", nil))
		return
	}
	var req assetReasonReq
	_ = c.ShouldBindJSON(&req)
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	if appErr := h.lifecycleSvc.Delete(c.Request.Context(), actor, assetID, req.Reason); appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TaskAssetCenterHandler) GetAssetResource(c *gin.Context) {
	assetID, err := parseInt64(strings.TrimSpace(c.Param("asset_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset id", nil))
		return
	}
	asset, appErr := h.svc.GetAsset(c.Request.Context(), assetID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, asset)
}

func (h *TaskAssetCenterHandler) ListVersions(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	assetID, err := parseInt64(strings.TrimSpace(c.Param("asset_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset id", nil))
		return
	}
	versions, appErr := h.svc.ListVersions(c.Request.Context(), taskID, assetID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, versions)
}

func (h *TaskAssetCenterHandler) DownloadAsset(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	assetID, err := parseInt64(strings.TrimSpace(c.Param("asset_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset id", nil))
		return
	}
	info, appErr := h.svc.GetAssetDownloadInfo(c.Request.Context(), taskID, assetID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, info)
}

func (h *TaskAssetCenterHandler) DownloadAssetResource(c *gin.Context) {
	assetID, err := parseInt64(strings.TrimSpace(c.Param("id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset id", nil))
		return
	}
	info, appErr := h.svc.GetAssetDownloadInfoByID(c.Request.Context(), assetID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, info)
}

func (h *TaskAssetCenterHandler) PreviewAssetResource(c *gin.Context) {
	assetID, err := parseInt64(strings.TrimSpace(c.Param("asset_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset id", nil))
		return
	}
	info, appErr := h.svc.GetAssetPreviewInfoByID(c.Request.Context(), assetID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, info)
}

func (h *TaskAssetCenterHandler) DownloadVersion(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	assetID, err := parseInt64(strings.TrimSpace(c.Param("asset_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid asset id", nil))
		return
	}
	versionID, err := parseInt64(strings.TrimSpace(c.Param("version_id")))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid version id", nil))
		return
	}
	info, appErr := h.svc.GetVersionDownloadInfo(c.Request.Context(), taskID, assetID, versionID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, info)
}

func (h *TaskAssetCenterHandler) GetUploadSession(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	session, appErr := h.svc.GetUploadSession(c.Request.Context(), taskID, strings.TrimSpace(c.Param("session_id")))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, session)
}

func (h *TaskAssetCenterHandler) GetAssetUploadSession(c *gin.Context) {
	session, appErr := h.svc.GetUploadSessionByID(c.Request.Context(), strings.TrimSpace(c.Param("session_id")))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, session)
}

func (h *TaskAssetCenterHandler) CreateSmallUploadSession(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	h.createUploadSession(c, taskID, domain.DesignAssetUploadModeSmall, false)
}

// LegacyTaskAssetsUpload serves POST /v1/tasks/{id}/assets/upload only.
// Older frontends posted multipart/form-data (file + file_role). The asset center contract is JSON
// session handoff plus OSS upload, then complete. Reject multipart early with 410 so callers do not
// hit confusing bind errors or internal failures.
func (h *TaskAssetCenterHandler) LegacyTaskAssetsUpload(c *gin.Context) {
	if legacyTaskAssetsUploadFormLikeContentType(c.Request.Header.Get("Content-Type")) {
		respondError(c, domain.NewAppError(
			domain.ErrCodeUploadEndpointDeprecated,
			"this path no longer accepts multipart or form-encoded uploads; use task asset-center upload sessions (JSON handoff, then OSS upload, then complete)",
			legacyTaskAssetsUploadReplacementDetails(),
		))
		return
	}
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	h.createUploadSession(c, taskID, domain.DesignAssetUploadModeSmall, false)
}

func legacyTaskAssetsUploadFormLikeContentType(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	base := raw
	if idx := strings.Index(raw, ";"); idx >= 0 {
		base = strings.TrimSpace(raw[:idx])
	}
	switch strings.ToLower(base) {
	case "multipart/form-data", "application/x-www-form-urlencoded":
		return true
	default:
		return false
	}
}

func legacyTaskAssetsUploadReplacementDetails() gin.H {
	return gin.H{
		"deprecated_path":          "/v1/tasks/{id}/assets/upload",
		"canonical_session_create": "/v1/assets/upload-sessions",
		"session_complete":         "/v1/assets/upload-sessions/{session_id}/complete",
		"session_cancel":           "/v1/assets/upload-sessions/{session_id}/cancel",
		"contract":                 "application/json create session, upload bytes to OSS using returned remote plan, then complete session",
	}
}

func (h *TaskAssetCenterHandler) CreateMultipartUploadSession(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	h.createUploadSession(c, taskID, domain.DesignAssetUploadModeMultipart, false)
}

func (h *TaskAssetCenterHandler) CreateUploadSession(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	h.createUploadSession(c, taskID, "", false)
}

func (h *TaskAssetCenterHandler) CreateAssetUploadSession(c *gin.Context) {
	var req createTaskAssetUploadSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	if req.TaskID == nil || *req.TaskID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_id is required", nil))
		return
	}
	h.createUploadSessionWithRequest(c, *req.TaskID, "", req, true)
}

func (h *TaskAssetCenterHandler) CompleteUploadSession(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req completeTaskAssetUploadSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	completedBy, appErr := actorIDOrRequestValue(c, req.CompletedBy, "completed_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.svc.CompleteUploadSession(c.Request.Context(), service.CompleteTaskAssetUploadSessionParams{
		TaskID:            taskID,
		SessionID:         strings.TrimSpace(c.Param("session_id")),
		CompletedBy:       completedBy,
		Remark:            strings.TrimSpace(req.Remark),
		FileHash:          strings.TrimSpace(req.FileHash),
		UploadContentType: strings.TrimSpace(req.UploadContentType),
		OSSParts:          req.OSSParts,
		OSSUploadID:       strings.TrimSpace(req.OSSUploadID),
		OSSObjectKey:      strings.TrimSpace(req.OSSObjectKey),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *TaskAssetCenterHandler) CompleteAssetUploadSession(c *gin.Context) {
	var req completeTaskAssetUploadSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	completedBy, appErr := actorIDOrRequestValue(c, req.CompletedBy, "completed_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	result, appErr := h.svc.CompleteUploadSessionByID(c.Request.Context(), service.CompleteTaskAssetUploadSessionParams{
		SessionID:         strings.TrimSpace(c.Param("session_id")),
		CompletedBy:       completedBy,
		Remark:            strings.TrimSpace(req.Remark),
		FileHash:          strings.TrimSpace(req.FileHash),
		UploadContentType: strings.TrimSpace(req.UploadContentType),
		OSSParts:          req.OSSParts,
		OSSUploadID:       strings.TrimSpace(req.OSSUploadID),
		OSSObjectKey:      strings.TrimSpace(req.OSSObjectKey),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *TaskAssetCenterHandler) CancelUploadSession(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req cancelTaskAssetUploadSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	cancelledBy, appErr := actorIDOrRequestValue(c, req.CancelledBy, "cancelled_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	session, appErr := h.svc.CancelUploadSession(c.Request.Context(), service.CancelTaskAssetUploadSessionParams{
		TaskID:      taskID,
		SessionID:   strings.TrimSpace(c.Param("session_id")),
		CancelledBy: cancelledBy,
		Remark:      strings.TrimSpace(req.Remark),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, session)
}

func (h *TaskAssetCenterHandler) CancelAssetUploadSession(c *gin.Context) {
	var req cancelTaskAssetUploadSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	cancelledBy, appErr := actorIDOrRequestValue(c, req.CancelledBy, "cancelled_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	session, appErr := h.svc.CancelUploadSessionByID(c.Request.Context(), service.CancelTaskAssetUploadSessionParams{
		SessionID:   strings.TrimSpace(c.Param("session_id")),
		CancelledBy: cancelledBy,
		Remark:      strings.TrimSpace(req.Remark),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, session)
}

func (h *TaskAssetCenterHandler) AbortUploadSession(c *gin.Context) {
	h.CancelUploadSession(c)
}

func (h *TaskAssetCenterHandler) createUploadSession(c *gin.Context, taskID int64, mode domain.DesignAssetUploadMode, topLevel bool) {
	var req createTaskAssetUploadSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	h.createUploadSessionWithRequest(c, taskID, mode, req, topLevel)
}

func (h *TaskAssetCenterHandler) createUploadSessionWithRequest(c *gin.Context, taskID int64, mode domain.DesignAssetUploadMode, req createTaskAssetUploadSessionReq, topLevel bool) {
	createdBy, appErr := actorIDOrRequestValue(c, req.CreatedBy, "created_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	assetType := normalizeAssetTypeInput(req.AssetKind, req.AssetType)
	filename := firstNonEmptyTrimmed(req.FileName, req.Filename)
	expectedSize := req.ExpectedSize
	if expectedSize == nil {
		expectedSize = req.FileSize
	}
	params := service.CreateTaskAssetUploadSessionParams{
		TaskID:        taskID,
		AssetID:       req.AssetID,
		SourceAssetID: req.SourceAssetID,
		CreatedBy:     createdBy,
		AssetType:     domain.TaskAssetType(assetType),
		Filename:      filename,
		ExpectedSize:  expectedSize,
		MimeType:      strings.TrimSpace(req.MimeType),
		FileHash:      strings.TrimSpace(req.FileHash),
		Remark:        strings.TrimSpace(req.Remark),
		TargetSKUCode: strings.TrimSpace(req.TargetSKUCode),
	}
	resolvedMode := mode
	if resolvedMode == "" {
		if rawMode := strings.TrimSpace(req.UploadMode); rawMode != "" {
			resolvedMode = domain.DesignAssetUploadMode(rawMode)
			if !resolvedMode.Valid() {
				respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_mode must be small or multipart", nil))
				return
			}
		}
	}
	var result *service.CreateTaskAssetUploadSessionResult
	if resolvedMode.Valid() {
		if resolvedMode == domain.DesignAssetUploadModeMultipart {
			result, appErr = h.svc.CreateMultipartUploadSession(c.Request.Context(), params)
		} else {
			result, appErr = h.svc.CreateSmallUploadSession(c.Request.Context(), params)
		}
	} else {
		result, appErr = h.svc.CreateUploadSession(c.Request.Context(), params)
	}
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, buildCreateUploadSessionResponse(result, topLevel))
}

func buildCreateUploadSessionResponse(result *service.CreateTaskAssetUploadSessionResult, topLevel bool) gin.H {
	if result == nil || result.Session == nil {
		return gin.H{"session": result}
	}
	sessionID := result.Session.ID
	completeEndpoint := "/v1/assets/upload-sessions/" + sessionID + "/complete"
	cancelEndpoint := "/v1/assets/upload-sessions/" + sessionID + "/cancel"
	if !topLevel {
		taskID := result.Session.TaskID
		completeEndpoint = "/v1/tasks/" + int64ToString(taskID) + "/asset-center/upload-sessions/" + sessionID + "/complete"
		cancelEndpoint = "/v1/tasks/" + int64ToString(taskID) + "/asset-center/upload-sessions/" + sessionID + "/cancel"
	}
	resp := gin.H{
		"session":                      result.Session,
		"remote":                       result.Remote,
		"upload_strategy":              uploadStrategyLabel(result.Session.UploadMode),
		"required_upload_content_type": strings.TrimSpace(result.Session.MimeType),
		"complete_endpoint":            completeEndpoint,
		"cancel_endpoint":              cancelEndpoint,
	}
	if result.OSSDirect != nil {
		resp["oss_direct"] = result.OSSDirect
	}
	return resp
}

func uploadStrategyLabel(mode domain.DesignAssetUploadMode) string {
	if mode == domain.DesignAssetUploadModeSmall {
		return "single_part"
	}
	return "multipart"
}

func normalizeAssetTypeInput(primary, fallback string) string {
	return firstNonEmptyTrimmed(primary, fallback)
}

func parseAssetSearchQuery(c *gin.Context) (domain.AssetSearchQuery, *domain.AppError) {
	query := domain.AssetSearchQuery{
		Keyword:       strings.TrimSpace(c.Query("keyword")),
		ModuleKey:     strings.TrimSpace(c.Query("module_key")),
		OwnerTeamCode: strings.TrimSpace(c.Query("owner_team_code")),
		IsArchived:    domain.AssetArchiveFilter(strings.TrimSpace(c.DefaultQuery("is_archived", string(domain.AssetArchiveFilterFalse)))),
		TaskStatus:    domain.AssetTaskStatusFilter(strings.TrimSpace(c.DefaultQuery("task_status", string(domain.AssetTaskStatusFilterAll)))),
	}
	if raw := strings.TrimSpace(c.Query("created_from")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return query, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid created_from", nil)
		}
		query.CreatedFrom = &parsed
	}
	if raw := strings.TrimSpace(c.Query("created_to")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return query, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid created_to", nil)
		}
		query.CreatedTo = &parsed
	}
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil {
			return query, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid page", nil)
		}
		query.Page = page
	}
	if raw := strings.TrimSpace(c.Query("size")); raw != "" {
		size, err := strconv.Atoi(raw)
		if err != nil {
			return query, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid size", nil)
		}
		query.Size = size
	}
	return query.Normalized(), nil
}

func respondAssetCenterError(c *gin.Context, err *domain.AppError) {
	if err == nil {
		return
	}
	err.TraceID = c.GetString("trace_id")
	status := httpStatusFromCode(err.Code)
	switch err.Code {
	case assetcenter.ErrCodeAssetGone:
		status = http.StatusGone
	case domain.DenyModuleActionRoleDenied:
		status = http.StatusForbidden
	}
	c.AbortWithStatusJSON(status, domain.APIErrorResponse{Error: err})
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func int64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}
