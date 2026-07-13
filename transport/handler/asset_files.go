package handler

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
	"workflow/service"
)

// AssetFilesHandler proxies GET /v1/assets/files/* to the OSS-backed upload service.
// download_url in reference_file_refs and asset versions points here.
type AssetFilesHandler struct {
	uploadServiceBaseURL string
	internalToken        string
	storageProvider      string
	ossPresigner         assetFilesOSSPresigner
	erpImageAssetRepo    repo.TaskAssetRepo
	erpImageSigner       *service.ERPImageProxySigner
	accessTaskRepo       repo.TaskRepo
	accessTaskAssetRepo  AssetFilesTaskAssetRepo
	accessStorageRefRepo AssetFilesStorageRefRepo
	accessUserRepo       repo.UserRepo
	httpClient           *http.Client
	logger               *zap.Logger
}

type assetFilesOSSPresigner interface {
	PresignPreviewURL(objectKey string) *service.OSSDirectDownloadInfo
}

type AssetFilesTaskAssetRepo interface {
	GetByID(ctx context.Context, id int64) (*domain.TaskAsset, error)
	GetByStorageKey(ctx context.Context, storageKey string) (*domain.TaskAsset, error)
}

type AssetFilesStorageRefRepo interface {
	GetByRefKey(ctx context.Context, refKey string) (*domain.AssetStorageRef, error)
	ListAttachedTaskIDsByRefID(ctx context.Context, refID string) ([]int64, error)
}

// NewAssetFilesHandler creates a handler that proxies file requests to the OSS-backed upload service.
func NewAssetFilesHandler(uploadServiceBaseURL, internalToken, storageProvider string, logger *zap.Logger, ossPresigner ...assetFilesOSSPresigner) *AssetFilesHandler {
	base := strings.TrimSuffix(strings.TrimSpace(uploadServiceBaseURL), "/")
	if base == "" {
		base = "http://127.0.0.1:8092"
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AssetFilesHandler{
		uploadServiceBaseURL: base,
		internalToken:        strings.TrimSpace(internalToken),
		storageProvider:      strings.TrimSpace(storageProvider),
		ossPresigner:         firstAssetFilesOSSPresigner(ossPresigner),
		httpClient:           &http.Client{},
		logger:               logger.Named("asset_files_proxy"),
	}
}

func (h *AssetFilesHandler) SetERPImageProxy(assetRepo repo.TaskAssetRepo, signer *service.ERPImageProxySigner) {
	if h == nil {
		return
	}
	h.erpImageAssetRepo = assetRepo
	h.erpImageSigner = signer
}

func (h *AssetFilesHandler) SetFileAccessPolicy(taskRepo repo.TaskRepo, assetRepo AssetFilesTaskAssetRepo, storageRefRepo AssetFilesStorageRefRepo, userRepo repo.UserRepo) {
	if h == nil {
		return
	}
	h.accessTaskRepo = taskRepo
	h.accessTaskAssetRepo = assetRepo
	h.accessStorageRefRepo = storageRefRepo
	h.accessUserRepo = userRepo
}

func (h *AssetFilesHandler) authorizeStorageKeyAccess(ctx context.Context, storageKey string) *domain.AppError {
	if h == nil || h.accessTaskRepo == nil || h.accessTaskAssetRepo == nil || h.accessStorageRefRepo == nil {
		return nil
	}
	storageKey = strings.TrimSpace(storageKey)
	if storageKey == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "Asset storage path is required.", nil)
	}
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || !domain.IsSessionBackedRequestActor(actor) {
		return domain.NewAppError(domain.ErrCodeUnauthorized, "Authentication required.", nil)
	}
	if asset, err := h.accessTaskAssetRepo.GetByStorageKey(ctx, storageKey); err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to verify asset file access.", nil)
	} else if asset != nil {
		return h.authorizeTaskAssetAccess(ctx, asset)
	}
	ref, err := h.accessStorageRefRepo.GetByRefKey(ctx, storageKey)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to verify asset file access.", nil)
	}
	if ref == nil {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Asset file is outside the current access scope.", gin.H{
			"deny_code": "asset_file_scope_denied",
		})
	}
	return h.authorizeStorageRefAccess(ctx, actor, ref)
}

func (h *AssetFilesHandler) authorizeTaskAssetAccess(ctx context.Context, asset *domain.TaskAsset) *domain.AppError {
	if asset == nil || asset.TaskID <= 0 {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Asset file is outside the current access scope.", gin.H{
			"deny_code": "asset_file_scope_denied",
		})
	}
	task, err := h.accessTaskRepo.GetByID(ctx, asset.TaskID)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to verify asset file access.", nil)
	}
	if task == nil {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Asset file is outside the current access scope.", gin.H{
			"deny_code": "asset_file_task_not_found",
		})
	}
	return service.AuthorizeTaskReadDetail(ctx, task, h.accessUserRepo)
}

func (h *AssetFilesHandler) authorizeStorageRefAccess(ctx context.Context, actor domain.RequestActor, ref *domain.AssetStorageRef) *domain.AppError {
	if ref == nil {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Asset file is outside the current access scope.", gin.H{
			"deny_code": "asset_file_scope_denied",
		})
	}
	switch ref.OwnerType {
	case domain.AssetOwnerTypeTask:
		return h.authorizeTaskAccess(ctx, ref.OwnerID)
	case domain.AssetOwnerTypeTaskAsset:
		assetID := ref.OwnerID
		if ref.AssetID != nil && *ref.AssetID > 0 {
			assetID = *ref.AssetID
		}
		asset, err := h.accessTaskAssetRepo.GetByID(ctx, assetID)
		if err != nil {
			return domain.NewAppError(domain.ErrCodeInternalError, "Failed to verify asset file access.", nil)
		}
		return h.authorizeTaskAssetAccess(ctx, asset)
	case domain.AssetOwnerTypeTaskCreateReference:
		if actor.ID == ref.OwnerID || assetFilePrivilegedActor(actor) {
			return nil
		}
		return h.authorizeAttachedTaskCreateReference(ctx, ref)
	case domain.AssetOwnerTypeExportJob, domain.AssetOwnerTypeOutsource, domain.AssetOwnerTypeWarehouse:
		if actor.ID == ref.OwnerID || assetFilePrivilegedActor(actor) {
			return nil
		}
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "Asset file is outside the current access scope.", gin.H{
		"deny_code":  "asset_file_scope_denied",
		"owner_type": ref.OwnerType,
	})
}

func (h *AssetFilesHandler) authorizeAttachedTaskCreateReference(ctx context.Context, ref *domain.AssetStorageRef) *domain.AppError {
	if h == nil || h.accessStorageRefRepo == nil || ref == nil {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Asset file is outside the current access scope.", gin.H{
			"deny_code": "asset_file_scope_denied",
		})
	}
	taskIDs, err := h.accessStorageRefRepo.ListAttachedTaskIDsByRefID(ctx, strings.TrimSpace(ref.RefID))
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to verify attached task reference access.", nil)
	}
	for _, taskID := range taskIDs {
		if appErr := h.authorizeTaskAccess(ctx, taskID); appErr == nil {
			return nil
		} else if appErr.Code == domain.ErrCodeInternalError {
			return appErr
		}
	}
	return domain.NewAppError(domain.ErrCodePermissionDenied, "Asset file is outside the current access scope.", gin.H{
		"deny_code":  "asset_file_task_reference_scope_denied",
		"owner_type": ref.OwnerType,
	})
}

func (h *AssetFilesHandler) authorizeTaskAccess(ctx context.Context, taskID int64) *domain.AppError {
	if taskID <= 0 {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Asset file is outside the current access scope.", gin.H{
			"deny_code": "asset_file_scope_denied",
		})
	}
	task, err := h.accessTaskRepo.GetByID(ctx, taskID)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to verify asset file access.", nil)
	}
	if task == nil {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Asset file is outside the current access scope.", gin.H{
			"deny_code": "asset_file_task_not_found",
		})
	}
	return service.AuthorizeTaskReadDetail(ctx, task, h.accessUserRepo)
}

func assetFilePrivilegedActor(actor domain.RequestActor) bool {
	return domain.ActorHasAnyRole(actor, []domain.Role{
		domain.RoleAdmin,
		domain.RoleSuperAdmin,
		domain.RoleOps,
		domain.RoleERP,
		domain.RoleWarehouse,
	})
}

// ServeFile handles GET /v1/assets/files/:path where path is the OSS object key or file id.
func (h *AssetFilesHandler) ServeFile(c *gin.Context) {
	pathParam := c.Param("path")
	if pathParam == "" || pathParam == "/" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing storage path"})
		return
	}
	storageKey := strings.TrimPrefix(pathParam, "/")
	traceID := domain.TraceIDFromContext(c.Request.Context())
	downloadFilename := strings.TrimSpace(c.Query(service.DownloadFilenameQueryParam))
	if appErr := h.authorizeStorageKeyAccess(c.Request.Context(), storageKey); appErr != nil {
		h.logger.Warn("asset_files_proxy_access_denied",
			zap.String("trace_id", traceID),
			zap.String("storage_key", storageKey),
			zap.String("code", appErr.Code),
			zap.String("message", appErr.Message),
		)
		respondError(c, appErr)
		return
	}
	if strings.EqualFold(strings.TrimSpace(h.storageProvider), "oss") && h.redirectToOSSDirect(c, storageKey, downloadFilename, traceID) {
		return
	}
	upstreamURL, err := domain.BuildAbsoluteEscapedURLPath(h.uploadServiceBaseURL, "/files", storageKey)
	if err != nil {
		h.logger.Warn("asset_files_proxy_upstream_url_invalid",
			zap.String("trace_id", traceID),
			zap.String("storage_key", storageKey),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build upstream request"})
		return
	}
	if rawQuery := strings.TrimSpace(upstreamRawQuery(c)); rawQuery != "" {
		upstreamURL += "?" + rawQuery
	}
	h.logger.Info("asset_files_proxy_downstream_request",
		zap.String("trace_id", traceID),
		zap.String("method", c.Request.Method),
		zap.String("storage_key", storageKey),
		zap.String("raw_query", c.Request.URL.RawQuery),
		zap.Bool("has_download_filename", downloadFilename != ""),
	)

	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, upstreamURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build upstream request"})
		return
	}
	copyHeaders(req.Header, c.Request.Header)
	if h.internalToken != "" {
		req.Header.Set("X-Internal-Token", h.internalToken)
	}
	if h.storageProvider != "" {
		req.Header.Set("X-Storage-Provider", h.storageProvider)
	}
	h.logger.Info("asset_files_proxy_upstream_request",
		zap.String("trace_id", traceID),
		zap.String("method", req.Method),
		zap.String("upstream_url", upstreamURL),
		zap.Bool("has_internal_token", req.Header.Get("X-Internal-Token") != ""),
		zap.String("x_storage_provider", req.Header.Get("X-Storage-Provider")),
		zap.String("range", req.Header.Get("Range")),
		zap.String("accept", req.Header.Get("Accept")),
		zap.String("if_none_match", req.Header.Get("If-None-Match")),
	)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		h.logger.Warn("asset_files_proxy_upstream_error",
			zap.String("trace_id", traceID),
			zap.String("upstream_url", upstreamURL),
			zap.Error(err),
		)
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream request failed"})
		return
	}
	defer resp.Body.Close()
	peekReader := bufio.NewReader(resp.Body)
	probe, probeErr := peekReader.Peek(64)
	if probeErr != nil && probeErr != io.EOF {
		h.logger.Warn("asset_files_proxy_probe_error",
			zap.String("trace_id", traceID),
			zap.String("upstream_url", upstreamURL),
			zap.Error(probeErr),
		)
	}
	h.logger.Info("asset_files_proxy_upstream_response",
		zap.String("trace_id", traceID),
		zap.String("upstream_url", upstreamURL),
		zap.Int("status_code", resp.StatusCode),
		zap.Int64("content_length", resp.ContentLength),
		zap.String("content_type", resp.Header.Get("Content-Type")),
		zap.Strings("transfer_encoding", resp.TransferEncoding),
		zap.Int("probe_len", len(probe)),
		zap.String("probe_prefix_b64", encodeProbe(probe)),
	)
	if resp.StatusCode == http.StatusNotFound {
		if h.redirectToOSSDirect(c, storageKey, downloadFilename, traceID) {
			return
		}
	}

	copyHeaders(c.Writer.Header(), resp.Header)
	if downloadFilename != "" && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		c.Writer.Header().Set("Content-Disposition", service.ContentDispositionAttachment(downloadFilename))
	}
	if c.Request.Method != http.MethodHead && len(probe) > 0 && resp.ContentLength == 0 {
		c.Writer.Header().Del("Content-Length")
		h.logger.Warn("asset_files_proxy_drop_zero_content_length",
			zap.String("trace_id", traceID),
			zap.String("upstream_url", upstreamURL),
		)
	}
	c.Status(resp.StatusCode)
	h.logger.Info("asset_files_proxy_downstream_headers",
		zap.String("trace_id", traceID),
		zap.Int("status_code", resp.StatusCode),
		zap.String("content_length", c.Writer.Header().Get("Content-Length")),
		zap.String("content_type", c.Writer.Header().Get("Content-Type")),
		zap.String("content_disposition", c.Writer.Header().Get("Content-Disposition")),
	)
	if c.Request.Method == http.MethodHead {
		return
	}
	written, copyErr := io.Copy(c.Writer, peekReader)
	h.logger.Info("asset_files_proxy_downstream_write",
		zap.String("trace_id", traceID),
		zap.Int64("bytes_written", written),
		zap.String("final_content_length", c.Writer.Header().Get("Content-Length")),
		zap.Int("final_status_code", c.Writer.Status()),
		zap.Error(copyErr),
	)
}

func (h *AssetFilesHandler) ServeERPProductImage(c *gin.Context) {
	if h == nil || h.erpImageAssetRepo == nil || h.erpImageSigner == nil || h.ossPresigner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "erp image proxy is not configured"})
		return
	}
	versionID, err := strconv.ParseInt(strings.TrimSpace(c.Param("version_id")), 10, 64)
	if err != nil || versionID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset version id"})
		return
	}
	asset, err := h.erpImageAssetRepo.GetByID(c.Request.Context(), versionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "asset version not found"})
			return
		}
		h.logger.Warn("erp_image_proxy_asset_lookup_failed",
			zap.Int64("version_id", versionID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load asset version"})
		return
	}
	if !isERPProductImageProxyAsset(asset) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "asset version is not a supported erp image"})
		return
	}
	if !h.erpImageSigner.Verify(asset, c.Query("exp"), c.Query("sig")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid or expired erp image signature"})
		return
	}
	storageKey := erpProductImageProxyStorageKey(asset)
	if storageKey == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset storage key is missing"})
		return
	}
	if strings.HasPrefix(storageKey, "http://") || strings.HasPrefix(storageKey, "https://") {
		c.Redirect(http.StatusFound, storageKey)
		return
	}
	info := h.ossPresigner.PresignPreviewURL(storageKey)
	if info == nil || strings.TrimSpace(info.DownloadURL) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "failed to sign erp image"})
		return
	}
	h.logger.Info("erp_image_proxy_redirect",
		zap.Int64("version_id", versionID),
		zap.String("storage_key", storageKey),
	)
	c.Header("Cache-Control", "private, max-age=300")
	c.Redirect(http.StatusFound, strings.TrimSpace(info.DownloadURL))
}

func (h *AssetFilesHandler) redirectToOSSDirect(c *gin.Context, storageKey, downloadFilename, traceID string) bool {
	if h == nil || h.ossPresigner == nil {
		return false
	}
	info := h.ossPresigner.PresignPreviewURL(storageKey)
	if filenamePresigner, ok := h.ossPresigner.(assetFilesOSSDownloadPresigner); ok && strings.TrimSpace(downloadFilename) != "" {
		info = filenamePresigner.PresignDownloadURLWithFilename(storageKey, downloadFilename)
	}
	if info == nil || strings.TrimSpace(info.DownloadURL) == "" {
		return false
	}
	h.logger.Info("asset_files_proxy_oss_direct_redirect",
		zap.String("trace_id", traceID),
		zap.String("storage_key", storageKey),
	)
	c.Redirect(http.StatusFound, strings.TrimSpace(info.DownloadURL))
	return true
}

func isERPProductImageProxyAsset(asset *domain.TaskAsset) bool {
	if asset == nil || asset.DeletedAt != nil || asset.CleanedAt != nil {
		return false
	}
	mimeType := ""
	if asset.MimeType != nil {
		mimeType = strings.ToLower(strings.TrimSpace(*asset.MimeType))
	}
	switch mimeType {
	case "image/jpeg", "image/jpg", "image/png", "image/webp", "image/gif":
		return true
	case "image/vnd.adobe.photoshop", "image/tiff", "image/heic", "image/heif", "image/avif":
		return false
	}
	filename := strings.ToLower(strings.TrimSpace(asset.FileName))
	for _, suffix := range []string{".jpg", ".jpeg", ".png", ".webp", ".gif"} {
		if strings.HasSuffix(filename, suffix) {
			return true
		}
	}
	return false
}

func erpProductImageProxyStorageKey(asset *domain.TaskAsset) string {
	if asset == nil {
		return ""
	}
	if asset.StorageKey != nil && strings.TrimSpace(*asset.StorageKey) != "" {
		return strings.TrimSpace(*asset.StorageKey)
	}
	if asset.StorageRef != nil {
		return strings.TrimSpace(asset.StorageRef.RefKey)
	}
	return ""
}

type assetFilesOSSDownloadPresigner interface {
	PresignDownloadURLWithFilename(objectKey, filename string) *service.OSSDirectDownloadInfo
}

func firstAssetFilesOSSPresigner(values []assetFilesOSSPresigner) assetFilesOSSPresigner {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func upstreamRawQuery(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return ""
	}
	query := c.Request.URL.Query()
	query.Del(service.DownloadFilenameQueryParam)
	return query.Encode()
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		if shouldSkipProxyHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func shouldSkipProxyHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "connection", "proxy-connection", "keep-alive", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

func encodeProbe(probe []byte) string {
	if len(probe) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(probe)
}
