package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	assetcenter "workflow/service/asset_center"
	externalassets "workflow/service/external_assets"
)

// IntegrationCenterHandler owns the narrow production machine boundaries:
// filesystem-event ingestion and read-only finalized asset synchronization.
type IntegrationCenterHandler struct {
	externalAssetEvents externalAssetEventService
	assetSync           finalizedAssetSyncService
	externalAssetSync   externalAssetSyncService
}

type finalizedAssetSyncService interface {
	FinalizedSyncManifest(context.Context) (*assetcenter.FinalizedSyncManifest, *domain.AppError)
	FinalizedDownloadTickets(context.Context, []int64) (*assetcenter.FinalizedDownloadTicketResponse, *domain.AppError)
}

func (h *IntegrationCenterHandler) SetFinalizedAssetSyncService(svc finalizedAssetSyncService) {
	h.assetSync = svc
}

type externalAssetSyncService interface {
	ExternalCurrentSyncManifest(context.Context) (*externalassets.ExternalCurrentManifest, *domain.AppError)
	ExternalCurrentSyncHead(context.Context) (*externalassets.ExternalCurrentSyncHead, *domain.AppError)
	ExternalCurrentSyncChanges(context.Context, string, int, time.Duration) (*externalassets.ExternalCurrentSyncChanges, *domain.AppError)
	ExternalCurrentDownloadTickets(context.Context, []int64) (*externalassets.ExternalCurrentTicketResponse, *domain.AppError)
}

func (h *IntegrationCenterHandler) SetExternalAssetSyncService(svc externalAssetSyncService) {
	h.externalAssetSync = svc
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

func (h *IntegrationCenterHandler) ExternalCurrentSyncManifest(c *gin.Context) {
	if h.externalAssetSync == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "external current sync service is not configured", nil))
		return
	}
	manifest, appErr := h.externalAssetSync.ExternalCurrentSyncManifest(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	etag := `W/"` + manifest.ManifestID + `"`
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, no-cache")
	if requestETagMatches(c.GetHeader("If-None-Match"), manifest.ManifestID) {
		c.Status(http.StatusNotModified)
		return
	}
	respondOK(c, manifest)
}

func (h *IntegrationCenterHandler) ExternalCurrentSyncHead(c *gin.Context) {
	if h.externalAssetSync == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "external current sync service is not configured", nil))
		return
	}
	result, appErr := h.externalAssetSync.ExternalCurrentSyncHead(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *IntegrationCenterHandler) ExternalCurrentSyncChanges(c *gin.Context) {
	if h.externalAssetSync == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "external current sync service is not configured", nil))
		return
	}
	limit := 500
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 500 {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "limit must be between 1 and 500", nil))
			return
		}
		limit = value
	}
	waitSeconds := 20
	if raw := strings.TrimSpace(c.Query("wait_seconds")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 || value > 30 {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "wait_seconds must be between 0 and 30", nil))
			return
		}
		waitSeconds = value
	}
	result, appErr := h.externalAssetSync.ExternalCurrentSyncChanges(
		c.Request.Context(), c.Query("cursor"), limit, time.Duration(waitSeconds)*time.Second,
	)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

type externalCurrentDownloadTicketsRequest struct {
	ExternalAssetIDs []int64 `json:"external_asset_ids"`
}

func (h *IntegrationCenterHandler) ExternalCurrentDownloadTickets(c *gin.Context) {
	if h.externalAssetSync == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "external current sync service is not configured", nil))
		return
	}
	var request externalCurrentDownloadTicketsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	result, appErr := h.externalAssetSync.ExternalCurrentDownloadTickets(c.Request.Context(), request.ExternalAssetIDs)
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
