package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

// IntegrationCenterHandler owns only the production filesystem-event ingress.
// The former connector/call-log execution playground was never a real executor
// and has been removed from the public runtime contract.
type IntegrationCenterHandler struct {
	externalAssetEvents externalAssetEventService
}

type externalAssetEventService interface {
	ApplyFilesystemEvents(context.Context, domain.ExternalAssetFilesystemEventBatch) (*domain.ExternalAssetFilesystemEventResult, *domain.AppError)
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
