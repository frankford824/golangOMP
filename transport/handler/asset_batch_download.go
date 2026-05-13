package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
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

	manifest, appErr := h.globalSvc.BuildBatchDownloadManifest(c.Request.Context(), req.AssetIDs)
	if appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	respondOK(c, manifest)
}
