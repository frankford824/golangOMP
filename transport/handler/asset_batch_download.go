package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	assetcenter "workflow/service/asset_center"
)

type assetBatchDownloadReq struct {
	AssetIDs    []int64  `json:"asset_ids"`
	ResourceIDs []string `json:"resource_ids,omitempty"`
	NamingMode  string   `json:"naming_mode,omitempty"`
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

	manifest, appErr := h.globalSvc.BuildBatchDownloadManifestForResources(
		c.Request.Context(),
		assetcenter.BatchDownloadResourceRequest{AssetIDs: req.AssetIDs, ResourceIDs: req.ResourceIDs},
		assetcenter.WithBatchDownloadNamingMode(assetcenter.NormalizeBatchDownloadNamingMode(req.NamingMode)),
	)
	if appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	respondOK(c, manifest)
}
