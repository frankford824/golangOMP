package handler

import (
	"bufio"
	"encoding/base64"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"workflow/domain"
)

// AssetFilesHandler proxies GET /v1/assets/files/* to the OSS-backed upload service.
// download_url in reference_file_refs and asset versions points here.
type AssetFilesHandler struct {
	uploadServiceBaseURL string
	internalToken        string
	storageProvider      string
	httpClient           *http.Client
	logger               *zap.Logger
}

// NewAssetFilesHandler creates a handler that proxies file requests to the OSS-backed upload service.
func NewAssetFilesHandler(uploadServiceBaseURL, internalToken, storageProvider string, logger *zap.Logger) *AssetFilesHandler {
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
		httpClient:           &http.Client{},
		logger:               logger.Named("asset_files_proxy"),
	}
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
	if rawQuery := strings.TrimSpace(c.Request.URL.RawQuery); rawQuery != "" {
		upstreamURL += "?" + rawQuery
	}
	h.logger.Info("asset_files_proxy_downstream_request",
		zap.String("trace_id", traceID),
		zap.String("method", c.Request.Method),
		zap.String("storage_key", storageKey),
		zap.String("raw_query", c.Request.URL.RawQuery),
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

	copyHeaders(c.Writer.Header(), resp.Header)
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
