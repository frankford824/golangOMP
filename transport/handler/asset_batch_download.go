package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	baseservice "workflow/service"
)

type assetBatchDownloadReq struct {
	AssetIDs []int64 `json:"asset_ids"`
}

func (h *TaskAssetCenterHandler) BatchDownloadGlobalAssets(c *gin.Context) {
	if h.globalSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "asset center service is not configured", nil))
		return
	}

	var req assetBatchDownloadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	result, appErr := h.globalSvc.BuildBatchDownloadZip(c.Request.Context(), req.AssetIDs)
	if appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", baseservice.ContentDispositionAttachment(result.Filename))
	c.Data(http.StatusOK, "application/zip", result.ZipBytes)
}
