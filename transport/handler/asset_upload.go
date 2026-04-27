package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type AssetUploadHandler struct {
	svc service.AssetUploadService
}

func NewAssetUploadHandler(svc service.AssetUploadService) *AssetUploadHandler {
	return &AssetUploadHandler{svc: svc}
}

type createUploadRequestReq struct {
	OwnerType      string `json:"owner_type" binding:"required"`
	OwnerID        int64  `json:"owner_id" binding:"required"`
	TaskAssetType  string `json:"task_asset_type"`
	StorageAdapter string `json:"storage_adapter"`
	RefType        string `json:"ref_type"`
	FileName       string `json:"file_name" binding:"required"`
	MimeType       string `json:"mime_type"`
	FileSize       *int64 `json:"file_size"`
	ChecksumHint   string `json:"checksum_hint"`
	Remark         string `json:"remark"`
}

type advanceUploadRequestReq struct {
	Action string `json:"action" binding:"required"`
	Remark string `json:"remark"`
}

func (h *AssetUploadHandler) ListUploadRequests(c *gin.Context) {
	filter := service.UploadRequestFilter{}
	if raw := strings.TrimSpace(c.Query("owner_type")); raw != "" {
		value := domain.AssetOwnerType(raw)
		filter.OwnerType = &value
	}
	if raw := strings.TrimSpace(c.Query("owner_id")); raw != "" {
		ownerID, err := parseInt64(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "owner_id must be an integer", nil))
			return
		}
		filter.OwnerID = &ownerID
	}
	if raw := strings.TrimSpace(c.Query("task_asset_type")); raw != "" {
		value := domain.TaskAssetType(raw)
		filter.TaskAssetType = &value
	}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		value := domain.UploadRequestStatus(raw)
		filter.Status = &value
	}
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		page, err := parseInt(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be an integer", nil))
			return
		}
		filter.Page = page
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		pageSize, err := parseInt(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil))
			return
		}
		filter.PageSize = pageSize
	}
	requests, pagination, appErr := h.svc.ListUploadRequests(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, requests, pagination)
}

func (h *AssetUploadHandler) CreateUploadRequest(c *gin.Context) {
	var req createUploadRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	var taskAssetType *domain.TaskAssetType
	if raw := strings.TrimSpace(req.TaskAssetType); raw != "" {
		value := domain.TaskAssetType(raw)
		taskAssetType = &value
	}
	request, appErr := h.svc.CreateUploadRequest(c.Request.Context(), service.CreateUploadRequestParams{
		OwnerType:      domain.AssetOwnerType(strings.TrimSpace(req.OwnerType)),
		OwnerID:        req.OwnerID,
		TaskAssetType:  taskAssetType,
		StorageAdapter: domain.AssetStorageAdapter(strings.TrimSpace(req.StorageAdapter)),
		RefType:        domain.AssetStorageRefType(strings.TrimSpace(req.RefType)),
		FileName:       strings.TrimSpace(req.FileName),
		MimeType:       strings.TrimSpace(req.MimeType),
		FileSize:       req.FileSize,
		ChecksumHint:   strings.TrimSpace(req.ChecksumHint),
		Remark:         strings.TrimSpace(req.Remark),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, request)
}

func (h *AssetUploadHandler) GetUploadRequest(c *gin.Context) {
	requestID := strings.TrimSpace(c.Param("id"))
	if requestID == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid upload request id", nil))
		return
	}
	request, appErr := h.svc.GetUploadRequest(c.Request.Context(), requestID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, request)
}

func (h *AssetUploadHandler) AdvanceUploadRequest(c *gin.Context) {
	requestID := strings.TrimSpace(c.Param("id"))
	if requestID == "" {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid upload request id", nil))
		return
	}
	var req advanceUploadRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	request, appErr := h.svc.AdvanceUploadRequest(c.Request.Context(), requestID, service.AdvanceUploadRequestParams{
		Action: domain.UploadRequestAdvanceAction(strings.TrimSpace(req.Action)),
		Remark: strings.TrimSpace(req.Remark),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, request)
}
