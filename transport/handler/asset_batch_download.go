package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	assetcenter "workflow/service/asset_center"
)

type assetBatchDownloadReq struct {
	AssetIDs   []int64 `json:"asset_ids"`
	NamingMode string  `json:"naming_mode,omitempty"`
}

type assetExcelPackagePreviewReq struct {
	Rows []assetExcelPackageRowReq `json:"rows"`
}

type assetExcelPackageRowReq struct {
	RowNumber int    `json:"row_number,omitempty"`
	OrderNo   string `json:"order_no"`
	SKUCode   string `json:"sku_code"`
	SKUName   string `json:"sku_name,omitempty"`
	Quantity  int    `json:"quantity"`
	Keyword   string `json:"keyword,omitempty"`
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

	manifest, appErr := h.globalSvc.BuildBatchDownloadManifest(
		c.Request.Context(),
		req.AssetIDs,
		assetcenter.WithBatchDownloadNamingMode(assetcenter.NormalizeBatchDownloadNamingMode(req.NamingMode)),
	)
	if appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	respondOK(c, manifest)
}

func (h *TaskAssetCenterHandler) PreviewExcelPackage(c *gin.Context) {
	if h.globalSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "asset center service is not configured", nil))
		return
	}

	var req assetExcelPackagePreviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	rows := make([]assetcenter.ExcelPackageRow, 0, len(req.Rows))
	for _, row := range req.Rows {
		rows = append(rows, assetcenter.ExcelPackageRow{
			RowNumber: row.RowNumber,
			OrderNo:   row.OrderNo,
			SKUCode:   row.SKUCode,
			SKUName:   row.SKUName,
			Quantity:  row.Quantity,
			Keyword:   row.Keyword,
		})
	}
	manifest, appErr := h.globalSvc.BuildExcelPackageManifest(c.Request.Context(), rows)
	if appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	respondOK(c, manifest)
}
