package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	assetcenter "workflow/service/asset_center"
)

// IntegrationCenterHandler owns the narrow production machine boundaries:
// filesystem-event ingestion and read-only finalized asset synchronization.
type IntegrationCenterHandler struct {
	externalAssetEvents externalAssetEventService
	assetSync           finalizedAssetSyncService
}

type finalizedAssetSyncService interface {
	FinalizedSyncManifest(context.Context) (*assetcenter.FinalizedSyncManifest, *domain.AppError)
	FinalizedDownloadTickets(context.Context, []int64) (*assetcenter.FinalizedDownloadTicketResponse, *domain.AppError)
}

func (h *IntegrationCenterHandler) SetFinalizedAssetSyncService(svc finalizedAssetSyncService) {
	h.assetSync = svc
}

type externalAssetEventService interface {
	ApplyFilesystemEvents(context.Context, domain.ExternalAssetFilesystemEventBatch) (*domain.ExternalAssetFilesystemEventResult, *domain.AppError)
}

func (h *IntegrationCenterHandler) FinalizedSyncManifest(c *gin.Context) {
	if h.assetSync == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "finalized asset sync service is not configured", nil))
		return
	}
	manifest, appErr := h.assetSync.FinalizedSyncManifest(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	// generated_at is observational metadata and is intentionally excluded from
	// manifest_id, so this validator represents semantic rather than byte-for-byte
	// response equality.
	etag := `W/"` + manifest.ManifestID + `"`
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, no-cache")
	if requestETagMatches(c.GetHeader("If-None-Match"), manifest.ManifestID) {
		c.Status(http.StatusNotModified)
		return
	}
	respondOK(c, manifest)
}

type finalizedDownloadTicketsRequest struct {
	TaskAssetIDs []int64 `json:"task_asset_ids"`
}

func (h *IntegrationCenterHandler) FinalizedDownloadTickets(c *gin.Context) {
	if h.assetSync == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "finalized asset sync service is not configured", nil))
		return
	}
	var request finalizedDownloadTicketsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.assetSync.FinalizedDownloadTickets(c.Request.Context(), request.TaskAssetIDs)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func requestETagMatches(value, manifestID string) bool {
	manifestID = strings.TrimSpace(manifestID)
	for _, candidate := range strings.Split(value, ",") {
		candidate = strings.TrimSpace(candidate)
		candidate = strings.TrimPrefix(candidate, "W/")
		candidate = strings.Trim(candidate, `"`)
		if candidate == "*" || (manifestID != "" && candidate == manifestID) {
			return true
		}
	}
	return false
}

func NewIntegrationCenterHandler(svc externalAssetEventService) *IntegrationCenterHandler {
	return &IntegrationCenterHandler{externalAssetEvents: svc}
}

func (h *IntegrationCenterHandler) IngestExternalAssetEvents(c *gin.Context) {
	if h.externalAssetEvents == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "external asset event service is not configured", nil))
		return
	}
	var batch domain.ExternalAssetFilesystemEventBatch
	if err := c.ShouldBindJSON(&batch); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.externalAssetEvents.ApplyFilesystemEvents(c.Request.Context(), batch)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}
