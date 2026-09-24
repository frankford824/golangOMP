package handler

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"workflow/domain"
	"workflow/internal/assetmedia"
	assetdelivery "workflow/service/asset_delivery"
)

type mediaAcceptedResponse struct {
	Accepted bool `json:"accepted"`
}

func mediaWorkerAppError(err error) *domain.AppError {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	code := domain.ErrCodeConflict
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "source_version_changed"), strings.Contains(message, "source length changed"):
		code = "source_version_changed"
	case strings.Contains(message, "media_lease_lost"):
		code = "media_lease_lost"
	case strings.Contains(message, "integrity"), strings.Contains(message, "checksum"):
		code = "media_integrity_failed"
	}
	return domain.NewAppError(code, "文件处理状态已变化或校验未通过", nil)
}

type mediaCapabilitiesResponse struct {
	LANAvailable   bool   `json:"lan_available"`
	GatewayID      string `json:"gateway_id"`
	GatewayURL     string `json:"gateway_url"`
	StrictPreviews bool   `json:"strict_previews"`
}

func (h *TaskAssetCenterHandler) MediaCapabilities(c *gin.Context) {
	result := mediaCapabilitiesResponse{}
	if h.media != nil {
		result.LANAvailable = h.media.Config.NASDeliveryEnabled
		result.GatewayID = h.media.Config.GatewayID
		result.GatewayURL = h.media.Config.GatewayURL
		result.StrictPreviews = h.media.Config.StrictPreviews
	}
	c.Header("Cache-Control", "private, no-store")
	respondOK(c, result)
}

type mediaScanResponse struct {
	ScanID string `json:"scan_id"`
	Part   *int   `json:"part,omitempty"`
	JobID  string `json:"job_id,omitempty"`
	State  string `json:"state,omitempty"`
}

func (h *TaskAssetCenterHandler) ResolveAssetMedia(c *gin.Context) {
	if h.media == nil {
		respondError(c, domain.NewAppError("media_temporarily_unavailable", "media delivery unavailable", nil))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var request assetdelivery.Request
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid media request", nil))
		return
	}
	var info *domain.AssetDownloadInfo
	var appErr *domain.AppError
	info, appErr = h.media.Resolve(c.Request.Context(), request)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	respondOK(c, info)
}

func (h *TaskAssetCenterHandler) CreateExternalMediaPackage(c *gin.Context) {
	if h.media == nil {
		respondError(c, domain.ErrNotFound)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	var request assetdelivery.PackageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid package selection", nil))
		return
	}
	var info *domain.AssetDownloadInfo
	var appErr *domain.AppError
	info, appErr = h.media.CreatePackage(c.Request.Context(), request)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, info)
}

func (h *IntegrationCenterHandler) ClaimMediaWork(c *gin.Context) {
	var request struct {
		WorkerID string `json:"worker_id"`
		Limit    int    `json:"limit"`
		Class    string `json:"class"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid worker claim", nil))
		return
	}
	var items []assetdelivery.WorkerClaim
	var err error
	items, err = h.media.ClaimNAS(c.Request.Context(), request.WorkerID, request.Limit, request.Class)
	if err != nil {
		respondError(c, domain.NewAppError("media_temporarily_unavailable", "media claim unavailable", nil))
		return
	}
	c.Header("Cache-Control", "no-store")
	respondOK(c, items)
}

func (h *IntegrationCenterHandler) HeartbeatMediaWork(c *gin.Context) {
	var request assetdelivery.WorkerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid worker heartbeat", nil))
		return
	}
	if err := h.media.HeartbeatNAS(c.Request.Context(), request); err != nil {
		respondError(c, domain.NewAppError("media_lease_lost", "media lease no longer valid", nil))
		return
	}
	respondOK(c, mediaAcceptedResponse{Accepted: true})
}

func (h *IntegrationCenterHandler) PrepareMediaUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	var request assetdelivery.UploadRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid media upload request", nil))
		return
	}
	var response *assetdelivery.UploadResponse
	var err error
	response, err = h.media.UploadNAS(c.Request.Context(), request)
	if err != nil {
		respondError(c, mediaWorkerAppError(err))
		return
	}
	c.Header("Cache-Control", "no-store")
	respondOK(c, response)
}

func (h *IntegrationCenterHandler) CompleteMediaWork(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	var request assetdelivery.WorkerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid media result", nil))
		return
	}
	if err := h.media.CompleteNAS(c.Request.Context(), request); err != nil {
		respondError(c, mediaWorkerAppError(err))
		return
	}
	respondOK(c, mediaAcceptedResponse{Accepted: true})
}

func (h *TaskAssetCenterHandler) ListMediaRequests(c *gin.Context) {
	actor, ok := domain.RequestActorFromContext(c.Request.Context())
	if !ok || actor.ID <= 0 {
		respondError(c, domain.ErrUnauthorized)
		return
	}
	if h.media == nil || h.media.Jobs == nil {
		respondOK(c, []domain.AssetMediaRequest{})
		return
	}
	rows, err := h.media.Jobs.ListRequests(c.Request.Context(), actor.ID, time.Now().Add(-24*time.Hour))
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "load media requests", nil))
		return
	}
	visible := []domain.AssetMediaRequest{}
	for _, r := range rows {
		if r.Job == nil {
			continue
		}
		if _, e := h.media.JobForActor(c.Request.Context(), r.Job.ID); e == nil {
			visible = append(visible, r)
		}
	}
	c.Header("Cache-Control", "private, no-store")
	respondOK(c, visible)
}

func (h *TaskAssetCenterHandler) GetMediaJob(c *gin.Context) {
	if h.media == nil {
		respondError(c, domain.ErrNotFound)
		return
	}
	var job *domain.AssetMediaJob
	var err *domain.AppError
	job, err = h.media.JobForActor(c.Request.Context(), c.Param("job_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, job)
}

func (h *TaskAssetCenterHandler) RetryMediaJob(c *gin.Context) {
	if h.media == nil {
		respondError(c, domain.ErrNotFound)
		return
	}
	var job *domain.AssetMediaJob
	var appErr *domain.AppError
	job, appErr = h.media.JobForActor(c.Request.Context(), c.Param("job_id"))
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	if err := h.media.Jobs.Retry(c.Request.Context(), job.ID, actor.ID); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeConflict, "任务不可重试或仍在重试冷却时间内", nil))
		return
	}
	job, err := h.media.Jobs.Get(c.Request.Context(), job.ID)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "load media task", nil))
		return
	}
	respondOK(c, job)
}

func (h *TaskAssetCenterHandler) CancelMediaRequest(c *gin.Context) {
	actor, ok := domain.RequestActorFromContext(c.Request.Context())
	if !ok || actor.ID <= 0 {
		respondError(c, domain.ErrUnauthorized)
		return
	}
	if h.media == nil || h.media.Jobs == nil {
		respondError(c, domain.ErrNotFound)
		return
	}
	if err := h.media.Jobs.CancelRequest(c.Request.Context(), c.Param("request_id"), actor.ID); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "cancel media request", nil))
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *IntegrationCenterHandler) AssetMediaMachineAuth(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.media == nil {
			respondError(c, domain.ErrUnauthorized)
			return
		}
		expected, header := h.media.Config.WorkerToken, "X-Asset-Media-Worker-Token"
		if role == "gateway" {
			expected = h.media.Config.GatewayToken
			header = "X-Asset-Media-Gateway-Token"
		}
		token := strings.TrimSpace(c.GetHeader(header))
		if expected == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			respondError(c, domain.ErrUnauthorized)
			return
		}
		c.Next()
	}
}

func (h *IntegrationCenterHandler) ValidateMediaTicket(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var request struct {
		Ticket string `json:"ticket"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid media ticket request", nil))
		return
	}
	var target *assetmedia.ReadTarget
	var appErr *domain.AppError
	target, appErr = h.media.ValidateTicket(c.Request.Context(), request.Ticket)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	c.Header("Cache-Control", "no-store")
	respondOK(c, target)
}

func (h *IntegrationCenterHandler) StartMediaScan(c *gin.Context) {
	if h.media == nil {
		respondError(c, domain.ErrUnauthorized)
		return
	}
	var req domain.ExternalMediaScan
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid scan", nil))
		return
	}
	if err := h.media.External.StartMediaScan(c.Request.Context(), req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeConflict, err.Error(), nil))
		return
	}
	respondOK(c, mediaScanResponse{ScanID: req.ScanID})
}

func (h *IntegrationCenterHandler) PutMediaScanPart(c *gin.Context) {
	if h.media == nil {
		respondError(c, domain.ErrUnauthorized)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	var req domain.ExternalMediaScanPart
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid scan part", nil))
		return
	}
	if err := h.media.External.PutMediaScanPart(c.Request.Context(), req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeConflict, err.Error(), nil))
		return
	}
	respondOK(c, mediaScanResponse{ScanID: req.ScanID, Part: &req.Part})
}

func (h *IntegrationCenterHandler) FinishMediaScan(c *gin.Context) {
	if h.media == nil {
		respondError(c, domain.ErrUnauthorized)
		return
	}
	var req domain.ExternalMediaScan
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid scan completion", nil))
		return
	}
	jobID, err := h.media.External.FinishMediaScan(c.Request.Context(), req)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeConflict, err.Error(), nil))
		return
	}
	respondOK(c, mediaScanResponse{ScanID: req.ScanID, JobID: jobID, State: "queued"})
}
