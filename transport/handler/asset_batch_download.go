package handler

import (
	"io"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	assetcenter "workflow/service/asset_center"
)

type assetBatchDownloadReq struct {
	AssetIDs    []int64  `json:"asset_ids"`
	ResourceIDs []string `json:"resource_ids,omitempty"`
	NamingMode  string   `json:"naming_mode,omitempty"`
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
	Address   string `json:"address,omitempty"`
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
			Address:   row.Address,
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

func (h *TaskAssetCenterHandler) PreviewExcelPackageFile(c *gin.Context) {
	if h.globalSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "asset center service is not configured", nil))
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel 文件不能为空", nil))
		return
	}
	if fileHeader.Size > assetcenter.MaxExcelPackageUploadBytes {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel 文件超过大小限制", map[string]interface{}{
			"limit": assetcenter.MaxExcelPackageUploadBytes,
		}))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, assetcenter.MaxExcelPackageUploadBytes+1))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	if int64(len(data)) > assetcenter.MaxExcelPackageUploadBytes {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "Excel 文件超过大小限制", map[string]interface{}{
			"limit": assetcenter.MaxExcelPackageUploadBytes,
		}))
		return
	}

	rows, appErr := assetcenter.ParseExcelPackageRows(data, fileHeader.Filename)
	if appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	manifest, appErr := h.globalSvc.BuildExcelPackageManifest(c.Request.Context(), rows)
	if appErr != nil {
		respondAssetCenterError(c, appErr)
		return
	}
	respondOK(c, manifest)
}
