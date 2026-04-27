package handler

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type designSubmissionUploadSessionService interface {
	GetUploadSessionByID(ctx context.Context, sessionID string) (*domain.UploadSession, *domain.AppError)
	CompleteUploadSessionByID(ctx context.Context, params service.CompleteTaskAssetUploadSessionParams) (*service.CompleteTaskAssetUploadSessionResult, *domain.AppError)
}

type designSubmissionTaskReadService interface {
	GetByID(ctx context.Context, id int64) (*domain.TaskReadModel, *domain.AppError)
}

type DesignSubmissionHandler struct {
	svc              service.TaskAssetService
	uploadSessionSvc designSubmissionUploadSessionService
	taskReadSvc      designSubmissionTaskReadService
}

func NewDesignSubmissionHandler(
	svc service.TaskAssetService,
	uploadSessionSvc designSubmissionUploadSessionService,
	taskReadSvc designSubmissionTaskReadService,
) *DesignSubmissionHandler {
	return &DesignSubmissionHandler{
		svc:              svc,
		uploadSessionSvc: uploadSessionSvc,
		taskReadSvc:      taskReadSvc,
	}
}

type submitDesignReq struct {
	UploadedBy      *int64                     `json:"uploaded_by"`
	AssetType       string                     `json:"asset_type"`
	UploadRequestID string                     `json:"upload_request_id"`
	FileName        string                     `json:"file_name"`
	MimeType        string                     `json:"mime_type"`
	FileSize        *int64                     `json:"file_size"`
	FilePath        *string                    `json:"file_path"`
	WholeHash       *string                    `json:"whole_hash"`
	Remark          string                     `json:"remark"`
	Assets          []submitDesignAssetItemReq `json:"assets"`
}

type submitDesignAssetItemReq struct {
	UploadSessionID   string                    `json:"upload_session_id"`
	AssetType         string                    `json:"asset_type"`
	AssetKind         string                    `json:"asset_kind"`
	TargetSKUCode     string                    `json:"target_sku_code"`
	FileHash          string                    `json:"file_hash"`
	UploadContentType string                    `json:"upload_content_type,omitempty"`
	Remark            string                    `json:"remark"`
	OSSParts          []service.OSSCompletePart `json:"oss_parts,omitempty"`
	OSSUploadID       string                    `json:"oss_upload_id,omitempty"`
	OSSObjectKey      string                    `json:"oss_object_key,omitempty"`
}

func (h *DesignSubmissionHandler) Submit(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}

	var req submitDesignReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	if len(req.Assets) > 0 {
		completedBy, appErr := actorIDOrRequestValue(c, req.UploadedBy, "uploaded_by")
		if appErr != nil {
			respondError(c, appErr)
			return
		}
		submitted, appErr := h.submitBatch(c, taskID, completedBy, req)
		if appErr != nil {
			respondError(c, appErr)
			return
		}
		respondCreated(c, gin.H{"submitted_assets": submitted})
		return
	}

	if strings.TrimSpace(req.AssetType) == "" || strings.TrimSpace(req.FileName) == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_type and file_name are required", nil))
		return
	}
	uploadedBy, appErr := actorIDOrRequestValue(c, req.UploadedBy, "uploaded_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	asset, appErr := h.svc.SubmitDesign(c.Request.Context(), service.SubmitDesignParams{
		TaskID:          taskID,
		UploadedBy:      uploadedBy,
		AssetType:       domain.TaskAssetType(req.AssetType),
		UploadRequestID: req.UploadRequestID,
		FileName:        req.FileName,
		MimeType:        req.MimeType,
		FileSize:        req.FileSize,
		FilePath:        req.FilePath,
		WholeHash:       req.WholeHash,
		Remark:          req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, asset)
}

func (h *DesignSubmissionHandler) submitBatch(
	c *gin.Context,
	taskID int64,
	completedBy int64,
	req submitDesignReq,
) ([]*service.CompleteTaskAssetUploadSessionResult, *domain.AppError) {
	if h.uploadSessionSvc == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "submit-design batch mode is not configured", nil)
	}
	if h.taskReadSvc == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task read service is not configured for submit-design batch mode", nil)
	}
	task, appErr := h.taskReadSvc.GetByID(c.Request.Context(), taskID)
	if appErr != nil {
		return nil, appErr
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	taskSKUSet := map[string]struct{}{}
	for _, skuItem := range task.SKUItems {
		if skuItem == nil {
			continue
		}
		if skuCode := strings.TrimSpace(skuItem.SKUCode); skuCode != "" {
			taskSKUSet[skuCode] = struct{}{}
		}
	}
	isBatchTask := len(taskSKUSet) > 1

	seenSessions := map[string]struct{}{}
	results := make([]*service.CompleteTaskAssetUploadSessionResult, 0, len(req.Assets))
	for idx, item := range req.Assets {
		sessionID := strings.TrimSpace(item.UploadSessionID)
		if sessionID == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "assets[].upload_session_id is required", map[string]interface{}{"index": idx})
		}
		if _, exists := seenSessions[sessionID]; exists {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "duplicate upload_session_id in submit payload", map[string]interface{}{
				"upload_session_id": sessionID,
				"index":             idx,
			})
		}
		seenSessions[sessionID] = struct{}{}

		session, sessionErr := h.uploadSessionSvc.GetUploadSessionByID(c.Request.Context(), sessionID)
		if sessionErr != nil {
			return nil, sessionErr
		}
		if session == nil || session.TaskID != taskID {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "upload_session_id does not belong to task", map[string]interface{}{
				"upload_session_id": sessionID,
				"task_id":           taskID,
				"index":             idx,
			})
		}

		sessionAssetType := domain.TaskAssetType("")
		if session.AssetType != nil {
			sessionAssetType = domain.NormalizeTaskAssetType(*session.AssetType)
		}
		requestAssetType := domain.NormalizeTaskAssetType(domain.TaskAssetType(firstNonEmptyTrimmedDesign(item.AssetKind, item.AssetType)))
		if requestAssetType != "" && !requestAssetType.IsSource() && !requestAssetType.IsDelivery() {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "submit-design only allows source or delivery assets", map[string]interface{}{
				"index":      idx,
				"asset_type": requestAssetType,
			})
		}
		resolvedAssetType := requestAssetType
		if resolvedAssetType == "" {
			resolvedAssetType = sessionAssetType
		}
		if !resolvedAssetType.IsSource() && !resolvedAssetType.IsDelivery() {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "submit-design only allows source or delivery assets", map[string]interface{}{
				"index":      idx,
				"asset_type": resolvedAssetType,
			})
		}
		if requestAssetType != "" && sessionAssetType != "" && requestAssetType != sessionAssetType {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "assets[].asset_type does not match upload session asset_type", map[string]interface{}{
				"index":                   idx,
				"upload_session_id":       sessionID,
				"asset_type":              requestAssetType,
				"session_captured_type":   sessionAssetType,
				"session_target_sku_code": strings.TrimSpace(session.TargetSKUCode),
			})
		}

		payloadSKUCode := strings.TrimSpace(item.TargetSKUCode)
		sessionSKUCode := strings.TrimSpace(session.TargetSKUCode)
		resolvedSKUCode := payloadSKUCode
		if resolvedSKUCode == "" {
			resolvedSKUCode = sessionSKUCode
		}
		if payloadSKUCode != "" && sessionSKUCode != "" && payloadSKUCode != sessionSKUCode {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "assets[].target_sku_code does not match upload session target_sku_code", map[string]interface{}{
				"index":                   idx,
				"upload_session_id":       sessionID,
				"target_sku_code":         payloadSKUCode,
				"session_target_sku_code": sessionSKUCode,
			})
		}
		if isBatchTask && resolvedAssetType.IsDelivery() && payloadSKUCode == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "assets[].target_sku_code is required for batch delivery submissions", map[string]interface{}{
				"index":             idx,
				"upload_session_id": sessionID,
			})
		}
		if resolvedSKUCode != "" {
			if _, ok := taskSKUSet[resolvedSKUCode]; !ok {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "assets[].target_sku_code must belong to the task", map[string]interface{}{
					"index":             idx,
					"upload_session_id": sessionID,
					"target_sku_code":   resolvedSKUCode,
					"task_id":           taskID,
				})
			}
		}

		result, completeErr := h.uploadSessionSvc.CompleteUploadSessionByID(c.Request.Context(), service.CompleteTaskAssetUploadSessionParams{
			SessionID:         sessionID,
			CompletedBy:       completedBy,
			Remark:            firstNonEmptyTrimmedDesign(item.Remark, req.Remark),
			FileHash:          strings.TrimSpace(item.FileHash),
			UploadContentType: strings.TrimSpace(item.UploadContentType),
			OSSParts:          item.OSSParts,
			OSSUploadID:       strings.TrimSpace(item.OSSUploadID),
			OSSObjectKey:      strings.TrimSpace(item.OSSObjectKey),
		})
		if completeErr != nil {
			return nil, completeErr
		}
		results = append(results, result)
	}

	return results, nil
}

func firstNonEmptyTrimmedDesign(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
