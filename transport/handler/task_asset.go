package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type TaskAssetHandler struct {
	svc service.TaskAssetService
}

func NewTaskAssetHandler(svc service.TaskAssetService) *TaskAssetHandler {
	return &TaskAssetHandler{svc: svc}
}

type mockUploadTaskAssetReq struct {
	UploadedBy      *int64  `json:"uploaded_by"`
	AssetType       string  `json:"asset_type"  binding:"required"`
	UploadRequestID string  `json:"upload_request_id"`
	FileName        string  `json:"file_name"   binding:"required"`
	MimeType        string  `json:"mime_type"`
	FileSize        *int64  `json:"file_size"`
	FilePath        *string `json:"file_path"`
	WholeHash       *string `json:"whole_hash"`
	Remark          string  `json:"remark"`
}

func (h *TaskAssetHandler) List(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	assets, appErr := h.svc.ListByTaskID(c.Request.Context(), taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, assets)
}

func (h *TaskAssetHandler) MockUpload(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}

	var req mockUploadTaskAssetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	uploadedBy, appErr := actorIDOrRequestValue(c, req.UploadedBy, "uploaded_by")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	asset, appErr := h.svc.MockUpload(c.Request.Context(), service.MockUploadTaskAssetParams{
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
