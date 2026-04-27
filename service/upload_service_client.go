package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
)

type UploadServiceClient interface {
	CreateUploadSession(ctx context.Context, req RemoteCreateUploadSessionRequest) (*RemoteUploadSessionPlan, error)
	GetUploadSession(ctx context.Context, req RemoteGetUploadSessionRequest) (*RemoteUploadSessionPlan, error)
	UploadFileToSession(ctx context.Context, req RemoteSessionFileUploadRequest) (*RemoteFileMeta, error)
	UploadSmallFile(ctx context.Context, req RemoteSmallFileUploadRequest) (*RemoteFileMeta, error)
	PreparePartUpload(ctx context.Context, req RemotePreparePartUploadRequest) (*RemotePartUploadPlan, error)
	CompleteUploadSession(ctx context.Context, req RemoteCompleteUploadRequest) (*RemoteFileMeta, error)
	AbortUploadSession(ctx context.Context, req RemoteAbortUploadRequest) error
	GetFileMeta(ctx context.Context, req RemoteGetFileMetaRequest) (*RemoteFileMeta, error)
	ProbeStoredFile(ctx context.Context, req RemoteProbeStoredFileRequest) (*RemoteStoredFileProbe, error)
	BuildBrowserFileURL(storageKey string) *string
}

type UploadServiceClientConfig struct {
	Enabled                 bool
	BaseURL                 string
	BrowserMultipartBaseURL string
	BrowserDownloadBaseURL  string
	Timeout                 time.Duration
	InternalToken           string
	StorageProvider         string
}

type RemoteCreateUploadSessionRequest struct {
	TaskID       int64
	TaskRef      string
	AssetID      *int64
	AssetNo      string
	AssetType    domain.TaskAssetType
	VersionNo    int
	UploadMode   domain.DesignAssetUploadMode
	Filename     string
	ExpectedSize *int64
	MimeType     string
	CreatedBy    int64
}

type RemoteGetUploadSessionRequest struct {
	RemoteUploadID string
}

type RemoteSmallFileUploadRequest struct {
	TaskID        int64
	AssetID       *int64
	AssetType     domain.TaskAssetType
	Filename      string
	MimeType      string
	ExpectedSize  *int64
	CreatedBy     int64
	File          io.Reader
	FileFieldName string
}

type RemoteSessionFileUploadRequest struct {
	UploadURL      string
	Method         string
	Headers        map[string]string
	RemoteUploadID string
	TaskRef        string
	AssetNo        string
	AssetType      domain.TaskAssetType
	VersionNo      int
	Filename       string
	MimeType       string
	ExpectedSize   *int64
	CreatedBy      int64
	File           io.Reader
	FileFieldName  string
}

type RemotePreparePartUploadRequest struct {
	RemoteUploadID string
	PartNumber     int
	ContentLength  *int64
	ChecksumHint   string
}

type RemoteCompleteUploadRequest struct {
	RemoteUploadID string
	Filename       string
	ExpectedSize   *int64
	MimeType       string
	ChecksumHint   string
}

type RemoteAbortUploadRequest struct {
	RemoteUploadID string
}

type RemoteGetFileMetaRequest struct {
	RemoteUploadID string
	RemoteFileID   string
	Filename       string
	ExpectedSize   *int64
	MimeType       string
	ChecksumHint   string
}

type RemoteProbeStoredFileRequest struct {
	StorageKey string
}

type RemoteUploadSessionPlan struct {
	UploadID              string                          `json:"upload_id"`
	FileID                *string                         `json:"file_id,omitempty"`
	BaseURL               string                          `json:"base_url"`
	UploadURL             string                          `json:"upload_url,omitempty"`
	PartUploadURLTemplate string                          `json:"part_upload_url_template,omitempty"`
	CompleteURL           string                          `json:"complete_url,omitempty"`
	AbortURL              string                          `json:"abort_url,omitempty"`
	Method                string                          `json:"method,omitempty"`
	Headers               map[string]string               `json:"headers,omitempty"`
	PartSizeHint          int64                           `json:"part_size_hint,omitempty"`
	PartsTotal            int                             `json:"parts_total,omitempty"`
	UploadMode            domain.DesignAssetUploadMode    `json:"upload_mode,omitempty"`
	SessionStatus         domain.DesignAssetSessionStatus `json:"session_status,omitempty"`
	Filename              string                          `json:"filename,omitempty"`
	ExpectedSize          *int64                          `json:"expected_size,omitempty"`
	MimeType              string                          `json:"mime_type,omitempty"`
	TaskRef               string                          `json:"task_ref,omitempty"`
	AssetNo               string                          `json:"asset_no,omitempty"`
	VersionNo             int                             `json:"version_no,omitempty"`
	FileRole              string                          `json:"file_role,omitempty"`
	StorageKey            string                          `json:"storage_key,omitempty"`
	ExpiresAt             *time.Time                      `json:"expires_at,omitempty"`
	LastSyncedAt          *time.Time                      `json:"last_synced_at,omitempty"`
	IsStub                bool                            `json:"is_stub"`
}

type RemotePartUploadPlan struct {
	UploadID     string            `json:"upload_id"`
	PartNumber   int               `json:"part_number"`
	Method       string            `json:"method,omitempty"`
	UploadURL    string            `json:"upload_url,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	ExpiresAt    *time.Time        `json:"expires_at,omitempty"`
	PartSizeHint int64             `json:"part_size_hint,omitempty"`
}

type RemoteFileMeta struct {
	FileID     *string   `json:"file_id,omitempty"`
	StorageKey string    `json:"storage_key"`
	FileSize   *int64    `json:"file_size,omitempty"`
	FileHash   *string   `json:"file_hash,omitempty"`
	MimeType   string    `json:"mime_type,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
	IsStub     bool      `json:"is_stub"`
}

type RemoteStoredFileProbe struct {
	StatusCode          int    `json:"status_code"`
	ContentType         string `json:"content_type,omitempty"`
	ContentLengthHeader int64  `json:"content_length_header"`
	BytesRead           int64  `json:"bytes_read"`
	SHA256              string `json:"sha256,omitempty"`
}

type multipartFormField struct {
	Key   string
	Value string
}

type UploadServiceHTTPError struct {
	Operation  string
	StatusCode int
	Message    string
	Body       string
}

func (e *UploadServiceHTTPError) Error() string {
	if e == nil {
		return ""
	}
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = strings.TrimSpace(e.Body)
	}
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	return fmt.Sprintf("upload service %s failed: status=%d message=%s", e.Operation, e.StatusCode, msg)
}

type httpUploadServiceClient struct {
	cfg        UploadServiceClientConfig
	httpClient *http.Client
}

func (c *httpUploadServiceClient) UploadServiceBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
}

func NewUploadServiceClient(cfg UploadServiceClientConfig) UploadServiceClient {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "http://127.0.0.1:8092"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if strings.TrimSpace(cfg.StorageProvider) == "" {
		cfg.StorageProvider = string(domain.DesignAssetStorageProviderOSS)
	}
	cfg.BrowserMultipartBaseURL = normalizeBrowserMultipartBaseURL(cfg.BrowserMultipartBaseURL)
	cfg.BrowserDownloadBaseURL = normalizeBrowserDownloadBaseURL(cfg.BrowserDownloadBaseURL)
	return &httpUploadServiceClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (c *httpUploadServiceClient) CreateUploadSession(ctx context.Context, req RemoteCreateUploadSessionRequest) (*RemoteUploadSessionPlan, error) {
	if err := c.ensureEnabled(); err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"task_id":       req.TaskID,
		"task_ref":      strings.TrimSpace(req.TaskRef),
		"asset_id":      req.AssetID,
		"asset_no":      strings.TrimSpace(req.AssetNo),
		"asset_type":    req.AssetType,
		"file_role":     string(req.AssetType),
		"version_no":    req.VersionNo,
		"upload_mode":   req.UploadMode,
		"filename":      strings.TrimSpace(req.Filename),
		"expected_size": req.ExpectedSize,
		"file_size":     req.ExpectedSize,
		"mime_type":     strings.TrimSpace(req.MimeType),
		"created_by":    req.CreatedBy,
	}
	body, requestURL, err := c.doJSON(ctx, http.MethodPost, createUploadSessionCandidates(), payload, "create_upload_session")
	if err != nil {
		return nil, err
	}
	plan := buildRemoteUploadSessionPlan(body)
	if plan.UploadID == "" {
		return nil, fmt.Errorf("upload service create_upload_session missing upload_id")
	}
	if !plan.UploadMode.Valid() {
		plan.UploadMode = req.UploadMode
	}
	if !plan.SessionStatus.Valid() {
		plan.SessionStatus = domain.DesignAssetSessionStatusCreated
	}
	if plan.BaseURL == "" {
		plan.BaseURL = strings.TrimRight(c.cfg.BaseURL, "/")
	}
	if plan.TaskRef == "" {
		plan.TaskRef = strings.TrimSpace(req.TaskRef)
	}
	if plan.AssetNo == "" {
		plan.AssetNo = strings.TrimSpace(req.AssetNo)
	}
	if plan.VersionNo <= 0 {
		plan.VersionNo = req.VersionNo
	}
	if plan.FileRole == "" {
		plan.FileRole = string(req.AssetType)
	}
	if plan.ExpectedSize == nil {
		plan.ExpectedSize = req.ExpectedSize
	}
	normalizeUploadSessionPlanMethod(plan)
	if plan.UploadURL == "" && req.UploadMode == domain.DesignAssetUploadModeSmall {
		if uploadURL, resolveErr := c.resolveURL("/upload/files"); resolveErr == nil {
			plan.UploadURL = uploadURL
		}
	}
	if plan.PartUploadURLTemplate != "" && !strings.HasPrefix(plan.PartUploadURLTemplate, "http://") && !strings.HasPrefix(plan.PartUploadURLTemplate, "https://") {
		if resolvedTemplate, resolveErr := c.resolveURL(plan.PartUploadURLTemplate); resolveErr == nil {
			plan.PartUploadURLTemplate = resolvedTemplate
		}
	}
	c.applyBrowserUploadTarget(plan)
	c.applyBrowserHeaders(plan)
	logUploadProbe("upload_service_create_session", map[string]interface{}{
		"trace_id":         domain.TraceIDFromContext(ctx),
		"request_url":      requestURL,
		"task_ref":         strings.TrimSpace(req.TaskRef),
		"asset_no":         strings.TrimSpace(req.AssetNo),
		"asset_type":       string(req.AssetType),
		"upload_mode":      string(req.UploadMode),
		"filename":         strings.TrimSpace(req.Filename),
		"expected_size":    int64ValueFromPtr(req.ExpectedSize),
		"mime_type":        strings.TrimSpace(req.MimeType),
		"remote_upload_id": plan.UploadID,
		"remote_file_id":   valueOrEmpty(plan.FileID),
		"upload_url":       plan.UploadURL,
		"method":           plan.Method,
		"response":         truncateLogString(mustMarshalJSON(body), 512),
	})
	return plan, nil
}

func (c *httpUploadServiceClient) GetUploadSession(ctx context.Context, req RemoteGetUploadSessionRequest) (*RemoteUploadSessionPlan, error) {
	if err := c.ensureEnabled(); err != nil {
		return nil, err
	}
	uploadID := strings.TrimSpace(req.RemoteUploadID)
	if uploadID == "" {
		return nil, fmt.Errorf("upload service get_upload_session requires remote_upload_id")
	}
	body, _, err := c.doJSON(ctx, http.MethodGet, getUploadSessionCandidates(uploadID), nil, "get_upload_session")
	if err != nil {
		return nil, err
	}
	plan := buildRemoteUploadSessionPlan(body)
	if plan.UploadID == "" {
		plan.UploadID = uploadID
	}
	if plan.BaseURL == "" {
		plan.BaseURL = strings.TrimRight(c.cfg.BaseURL, "/")
	}
	normalizeUploadSessionPlanMethod(plan)
	c.applyBrowserUploadTarget(plan)
	c.applyBrowserHeaders(plan)
	if plan.LastSyncedAt == nil {
		now := time.Now().UTC()
		plan.LastSyncedAt = &now
	}
	return plan, nil
}

func (c *httpUploadServiceClient) UploadSmallFile(ctx context.Context, req RemoteSmallFileUploadRequest) (*RemoteFileMeta, error) {
	if err := c.ensureEnabled(); err != nil {
		return nil, err
	}
	if req.File == nil {
		return nil, fmt.Errorf("upload service upload_small_file requires file reader")
	}
	fileFieldName := strings.TrimSpace(req.FileFieldName)
	if fileFieldName == "" {
		fileFieldName = "file"
	}
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	fields := []multipartFormField{
		{Key: "task_id", Value: strconv.FormatInt(req.TaskID, 10)},
		{Key: "asset_type", Value: string(req.AssetType)},
		{Key: "filename", Value: strings.TrimSpace(req.Filename)},
		{Key: "mime_type", Value: strings.TrimSpace(req.MimeType)},
		{Key: "created_by", Value: strconv.FormatInt(req.CreatedBy, 10)},
	}
	if req.AssetID != nil {
		fields = append(fields, multipartFormField{Key: "asset_id", Value: strconv.FormatInt(*req.AssetID, 10)})
	}
	if req.ExpectedSize != nil {
		fields = append(fields, multipartFormField{Key: "expected_size", Value: strconv.FormatInt(*req.ExpectedSize, 10)})
	}
	multipartBody, contentType, fileBytes, fileSHA256, err := buildMultipartBody(fileFieldName, strings.TrimSpace(req.Filename), strings.TrimSpace(req.MimeType), req.File, fields)
	if err != nil {
		return nil, err
	}

	requestURL, err := c.resolveURL(uploadSmallFileCandidates()[0])
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(multipartBody))
	if err != nil {
		return nil, err
	}
	httpReq.ContentLength = int64(len(multipartBody))
	httpReq.Header.Set("Content-Type", contentType)
	c.applyCommonHeaders(httpReq, false)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upload service upload_small_file request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logUploadProbe("upload_service_small_upload", map[string]interface{}{
			"trace_id":        domain.TraceIDFromContext(ctx),
			"method":          http.MethodPost,
			"request_url":     requestURL,
			"content_type":    contentType,
			"content_length":  len(multipartBody),
			"form_fields":     multipartFieldNames(fields),
			"file_field_name": fileFieldName,
			"file_bytes":      len(fileBytes),
			"file_sha256":     fileSHA256,
			"status_code":     resp.StatusCode,
			"response_body":   truncateLogString(string(raw), 512),
		})
		return nil, &UploadServiceHTTPError{
			Operation:  "upload_small_file",
			StatusCode: resp.StatusCode,
			Body:       string(raw),
		}
	}
	decoded, err := decodeJSONBytes(raw)
	if err != nil {
		return nil, err
	}
	meta := buildRemoteFileMeta(decoded)
	if meta.UploadedAt.IsZero() {
		meta.UploadedAt = time.Now().UTC()
	}
	logUploadProbe("upload_service_small_upload", map[string]interface{}{
		"trace_id":        domain.TraceIDFromContext(ctx),
		"method":          http.MethodPost,
		"request_url":     requestURL,
		"content_type":    contentType,
		"content_length":  len(multipartBody),
		"form_fields":     multipartFieldNames(fields),
		"file_field_name": fileFieldName,
		"file_bytes":      len(fileBytes),
		"file_sha256":     fileSHA256,
		"status_code":     resp.StatusCode,
		"response_body":   truncateLogString(string(raw), 512),
		"remote_file_id":  valueOrEmpty(meta.FileID),
		"storage_key":     meta.StorageKey,
	})
	return meta, nil
}

func (c *httpUploadServiceClient) UploadFileToSession(ctx context.Context, req RemoteSessionFileUploadRequest) (*RemoteFileMeta, error) {
	if err := c.ensureEnabled(); err != nil {
		return nil, err
	}
	if req.File == nil {
		return nil, fmt.Errorf("upload service upload_file_to_session requires file reader")
	}
	uploadURL := strings.TrimSpace(req.UploadURL)
	if uploadURL == "" {
		return nil, fmt.Errorf("upload service upload_file_to_session requires upload_url")
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodPost
	}
	fileFieldName := strings.TrimSpace(req.FileFieldName)
	if fileFieldName == "" {
		fileFieldName = "file"
	}
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	fields := make([]multipartFormField, 0, 12)
	if value := strings.TrimSpace(req.RemoteUploadID); value != "" {
		fields = append(fields,
			multipartFormField{Key: "upload_id", Value: value},
			multipartFormField{Key: "remote_upload_id", Value: value},
		)
	}
	if value := strings.TrimSpace(req.TaskRef); value != "" {
		fields = append(fields, multipartFormField{Key: "task_ref", Value: value})
	}
	if value := strings.TrimSpace(req.AssetNo); value != "" {
		fields = append(fields, multipartFormField{Key: "asset_no", Value: value})
	}
	if req.AssetType != "" {
		fields = append(fields, multipartFormField{Key: "asset_type", Value: string(req.AssetType)})
	}
	if req.VersionNo > 0 {
		fields = append(fields, multipartFormField{Key: "version_no", Value: strconv.Itoa(req.VersionNo)})
	}
	if value := strings.TrimSpace(req.Filename); value != "" {
		fields = append(fields, multipartFormField{Key: "filename", Value: value})
	}
	if value := strings.TrimSpace(req.MimeType); value != "" {
		fields = append(fields, multipartFormField{Key: "mime_type", Value: value})
	}
	if req.ExpectedSize != nil {
		fields = append(fields,
			multipartFormField{Key: "expected_size", Value: strconv.FormatInt(*req.ExpectedSize, 10)},
			multipartFormField{Key: "file_size", Value: strconv.FormatInt(*req.ExpectedSize, 10)},
		)
	}
	if req.CreatedBy > 0 {
		fields = append(fields, multipartFormField{Key: "created_by", Value: strconv.FormatInt(req.CreatedBy, 10)})
	}
	multipartBody, contentType, fileBytes, fileSHA256, err := buildMultipartBody(fileFieldName, strings.TrimSpace(req.Filename), strings.TrimSpace(req.MimeType), req.File, fields)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": contentType,
	}
	if token := strings.TrimSpace(c.cfg.InternalToken); token != "" {
		headers["X-Internal-Token"] = token
	}
	if provider := strings.TrimSpace(c.cfg.StorageProvider); provider != "" {
		headers["X-Storage-Provider"] = provider
	}
	for key, value := range req.Headers {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			headers[key] = value
		}
	}
	statusCode, raw, err := c.executeCurlMultipartUpload(ctx, method, uploadURL, headers, fields, fileFieldName, strings.TrimSpace(req.Filename), strings.TrimSpace(req.MimeType), fileBytes)
	if err != nil {
		return nil, fmt.Errorf("upload service upload_file_to_session request failed: %w", err)
	}
	var meta *RemoteFileMeta
	if len(bytes.TrimSpace(raw)) > 0 {
		decoded, decodeErr := decodeJSONBytes(raw)
		if decodeErr != nil {
			return nil, decodeErr
		}
		meta = buildRemoteFileMeta(decoded)
	}
	logUploadProbe("upload_service_upload_file_to_session", map[string]interface{}{
		"trace_id":         domain.TraceIDFromContext(ctx),
		"method":           method,
		"request_url":      uploadURL,
		"content_type":     contentType,
		"content_length":   len(multipartBody),
		"form_fields":      multipartFieldNames(fields),
		"file_field_name":  fileFieldName,
		"file_field_found": true,
		"file_bytes":       len(fileBytes),
		"file_sha256":      fileSHA256,
		"remote_file_id":   valueOrEmpty(metaFileID(meta)),
		"storage_key":      metaStorageKey(meta),
		"status_code":      statusCode,
		"response_body":    truncateLogString(string(raw), 512),
	})
	if statusCode < 200 || statusCode >= 300 {
		message := ""
		if decoded, decodeErr := decodeJSONBytes(raw); decodeErr == nil {
			message = stringValue(decoded, "message", "error", "detail")
		}
		return nil, &UploadServiceHTTPError{
			Operation:  "upload_file_to_session",
			StatusCode: statusCode,
			Message:    message,
			Body:       string(raw),
		}
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	return meta, nil
}

func (c *httpUploadServiceClient) PreparePartUpload(ctx context.Context, req RemotePreparePartUploadRequest) (*RemotePartUploadPlan, error) {
	if err := c.ensureEnabled(); err != nil {
		return nil, err
	}
	uploadID := strings.TrimSpace(req.RemoteUploadID)
	if uploadID == "" {
		return nil, fmt.Errorf("upload service prepare_part_upload requires remote_upload_id")
	}
	payload := map[string]interface{}{
		"part_number":    req.PartNumber,
		"content_length": req.ContentLength,
		"checksum_hint":  strings.TrimSpace(req.ChecksumHint),
	}
	body, _, err := c.doJSON(ctx, http.MethodPost, preparePartUploadCandidates(uploadID), payload, "prepare_part_upload")
	if err != nil {
		return nil, err
	}
	plan := buildRemotePartUploadPlan(body)
	if plan.UploadID == "" {
		plan.UploadID = uploadID
	}
	if plan.PartNumber == 0 {
		plan.PartNumber = req.PartNumber
	}
	return plan, nil
}

func (c *httpUploadServiceClient) CompleteUploadSession(ctx context.Context, req RemoteCompleteUploadRequest) (*RemoteFileMeta, error) {
	if err := c.ensureEnabled(); err != nil {
		return nil, err
	}
	uploadID := strings.TrimSpace(req.RemoteUploadID)
	if uploadID == "" {
		return nil, fmt.Errorf("upload service complete_upload_session requires remote_upload_id")
	}
	payload := map[string]interface{}{
		"filename":      strings.TrimSpace(req.Filename),
		"expected_size": req.ExpectedSize,
		"file_size":     req.ExpectedSize,
		"mime_type":     strings.TrimSpace(req.MimeType),
		"checksum_hint": strings.TrimSpace(req.ChecksumHint),
	}
	body, requestURL, err := c.doJSON(ctx, http.MethodPost, completeUploadSessionCandidates(uploadID), payload, "complete_upload_session")
	if err != nil {
		return nil, err
	}
	meta := buildRemoteFileMeta(body)
	if meta.UploadedAt.IsZero() {
		meta.UploadedAt = time.Now().UTC()
	}
	logUploadProbe("upload_service_complete_session", map[string]interface{}{
		"trace_id":         domain.TraceIDFromContext(ctx),
		"request_url":      requestURL,
		"remote_upload_id": uploadID,
		"filename":         strings.TrimSpace(req.Filename),
		"expected_size":    int64ValueFromPtr(req.ExpectedSize),
		"mime_type":        strings.TrimSpace(req.MimeType),
		"checksum_hint":    strings.TrimSpace(req.ChecksumHint),
		"remote_file_id":   valueOrEmpty(meta.FileID),
		"storage_key":      meta.StorageKey,
		"file_size":        int64ValueFromPtr(meta.FileSize),
		"response":         truncateLogString(mustMarshalJSON(body), 512),
	})
	return meta, nil
}

func (c *httpUploadServiceClient) AbortUploadSession(ctx context.Context, req RemoteAbortUploadRequest) error {
	if err := c.ensureEnabled(); err != nil {
		return err
	}
	uploadID := strings.TrimSpace(req.RemoteUploadID)
	if uploadID == "" {
		return fmt.Errorf("upload service abort_upload_session requires remote_upload_id")
	}
	if _, _, err := c.doJSON(ctx, http.MethodPost, abortUploadSessionCandidates(uploadID), map[string]interface{}{}, "abort_upload_session"); err != nil {
		return err
	}
	return nil
}

func (c *httpUploadServiceClient) GetFileMeta(ctx context.Context, req RemoteGetFileMetaRequest) (*RemoteFileMeta, error) {
	if err := c.ensureEnabled(); err != nil {
		return nil, err
	}
	candidates := getFileMetaCandidates(strings.TrimSpace(req.RemoteFileID), strings.TrimSpace(req.RemoteUploadID))
	query := map[string]string{
		"filename":      strings.TrimSpace(req.Filename),
		"mime_type":     strings.TrimSpace(req.MimeType),
		"checksum_hint": strings.TrimSpace(req.ChecksumHint),
	}
	if req.ExpectedSize != nil {
		query["expected_size"] = strconv.FormatInt(*req.ExpectedSize, 10)
	}
	body, _, err := c.doJSON(ctx, http.MethodGet, appendQuery(candidates, query), nil, "get_file_meta")
	if err != nil {
		return nil, err
	}
	meta := buildRemoteFileMeta(body)
	if meta.UploadedAt.IsZero() {
		meta.UploadedAt = time.Now().UTC()
	}
	return meta, nil
}

func (c *httpUploadServiceClient) ProbeStoredFile(ctx context.Context, req RemoteProbeStoredFileRequest) (*RemoteStoredFileProbe, error) {
	if err := c.ensureEnabled(); err != nil {
		return nil, err
	}
	storageKey := strings.TrimSpace(req.StorageKey)
	if storageKey == "" {
		return nil, fmt.Errorf("upload service probe_stored_file requires storage_key")
	}
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	requestURL, err := domain.BuildAbsoluteEscapedURLPath(c.cfg.BaseURL, "/files", storageKey)
	if err != nil {
		return nil, err
	}
	selectedProbeHost := ""
	if parsedURL, parseErr := url.Parse(requestURL); parseErr == nil {
		selectedProbeHost = strings.TrimSpace(parsedURL.Host)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	c.applyCommonHeaders(httpReq, false)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upload service probe_stored_file request failed: %w", err)
	}
	defer resp.Body.Close()

	probe := &RemoteStoredFileProbe{
		StatusCode:          resp.StatusCode,
		ContentType:         resp.Header.Get("Content-Type"),
		ContentLengthHeader: resp.ContentLength,
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
		logUploadProbe("upload_service_probe_stored_file", map[string]interface{}{
			"trace_id":                 domain.TraceIDFromContext(ctx),
			"base_url":                 c.UploadServiceBaseURL(),
			"selected_probe_host":      selectedProbeHost,
			"request_url":              requestURL,
			"storage_key":              truncateLogString(storageKey, 192),
			"status_code":              resp.StatusCode,
			"content_length_header":    resp.ContentLength,
			"response_body":            truncateLogString(string(raw), 512),
			"probe_request_succeeded":  true,
			"probe_response_validated": false,
		})
		return nil, &UploadServiceHTTPError{
			Operation:  "probe_stored_file",
			StatusCode: resp.StatusCode,
			Body:       string(raw),
		}
	}
	hasher := sha256.New()
	written, err := io.Copy(hasher, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("upload service probe_stored_file read body: %w", err)
	}
	probe.BytesRead = written
	probe.SHA256 = hex.EncodeToString(hasher.Sum(nil))
	logUploadProbe("upload_service_probe_stored_file", map[string]interface{}{
		"trace_id":                 domain.TraceIDFromContext(ctx),
		"base_url":                 c.UploadServiceBaseURL(),
		"selected_probe_host":      selectedProbeHost,
		"request_url":              requestURL,
		"storage_key":              truncateLogString(storageKey, 192),
		"status_code":              resp.StatusCode,
		"content_type":             probe.ContentType,
		"content_length_header":    probe.ContentLengthHeader,
		"bytes_read":               probe.BytesRead,
		"sha256":                   probe.SHA256,
		"probe_request_succeeded":  true,
		"probe_response_validated": true,
	})
	return probe, nil
}

func (c *httpUploadServiceClient) ensureEnabled() error {
	if !c.cfg.Enabled {
		return fmt.Errorf("upload service client is disabled")
	}
	return nil
}

func (c *httpUploadServiceClient) doJSON(ctx context.Context, method string, candidates []string, payload interface{}, operation string) (map[string]interface{}, string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	var requestBody []byte
	var err error
	if payload != nil {
		requestBody, err = json.Marshal(payload)
		if err != nil {
			return nil, "", fmt.Errorf("upload service %s marshal payload: %w", operation, err)
		}
	}
	var lastErr error
	var lastURL string
	for _, candidate := range candidates {
		var body io.Reader
		if requestBody != nil {
			body = bytes.NewReader(requestBody)
		}
		requestURL, err := c.resolveURL(candidate)
		if err != nil {
			return nil, "", err
		}
		lastURL = requestURL
		httpReq, err := http.NewRequestWithContext(ctx, method, requestURL, body)
		if err != nil {
			return nil, "", fmt.Errorf("upload service %s build request: %w", operation, err)
		}
		c.applyCommonHeaders(httpReq, requestBody != nil)
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("upload service %s request failed: %w", operation, err)
			continue
		}
		decoded, decodeErr := decodeJSONResponse(resp)
		resp.Body.Close()
		if decodeErr != nil {
			lastErr = decodeErr
			continue
		}
		return decoded, requestURL, nil
	}
	if lastErr != nil {
		return nil, lastURL, lastErr
	}
	return nil, lastURL, fmt.Errorf("upload service %s has no request candidates", operation)
}

func (c *httpUploadServiceClient) resolveURL(candidate string) (string, error) {
	base, err := url.Parse(strings.TrimRight(c.cfg.BaseURL, "/"))
	if err != nil {
		return "", fmt.Errorf("invalid upload service base url %q: %w", c.cfg.BaseURL, err)
	}
	rel, err := url.Parse(candidate)
	if err != nil {
		return "", fmt.Errorf("invalid upload service path %q: %w", candidate, err)
	}
	return base.ResolveReference(rel).String(), nil
}

func (c *httpUploadServiceClient) applyCommonHeaders(req *http.Request, includeJSONContentType bool) {
	req.Header.Set("Accept", "application/json")
	if includeJSONContentType {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := strings.TrimSpace(c.cfg.InternalToken); token != "" {
		req.Header.Set("X-Internal-Token", token)
	}
	if provider := strings.TrimSpace(c.cfg.StorageProvider); provider != "" {
		req.Header.Set("X-Storage-Provider", provider)
	}
}

func decodeJSONResponse(resp *http.Response) (map[string]interface{}, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
		message := ""
		if decoded, err := decodeJSONBytes(raw); err == nil {
			message = stringValue(decoded, "message", "error", "detail")
		}
		return nil, &UploadServiceHTTPError{
			StatusCode: resp.StatusCode,
			Message:    message,
			Body:       string(raw),
		}
	}
	return decodeJSONBody(resp.Body)
}

func decodeJSONBody(body io.Reader) (map[string]interface{}, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("read upload service response: %w", err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]interface{}{}, nil
	}
	return decodeJSONBytes(raw)
}

func decodeJSONBytes(raw []byte) (map[string]interface{}, error) {
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode upload service response: %w", err)
	}
	return unwrapData(decoded), nil
}

func unwrapData(decoded map[string]interface{}) map[string]interface{} {
	if decoded == nil {
		return map[string]interface{}{}
	}
	if nested, ok := decoded["data"].(map[string]interface{}); ok {
		return nested
	}
	return decoded
}

func buildRemoteUploadSessionPlan(decoded map[string]interface{}) *RemoteUploadSessionPlan {
	uploadURL := firstNonEmpty(
		stringValue(decoded, "browser_upload_url"),
		stringValue(decoded, "direct_upload_url"),
		stringValue(decoded, "signed_upload_url"),
		stringValue(decoded, "presigned_upload_url"),
		stringValue(decoded, "oss_upload_url"),
		stringValue(decoded, "upload_url", "upload_link", "url"),
	)
	partUploadURLTemplate := firstNonEmpty(
		stringValue(decoded, "browser_part_upload_url_template"),
		stringValue(decoded, "direct_part_upload_url_template"),
		stringValue(decoded, "signed_part_upload_url_template"),
		stringValue(decoded, "presigned_part_upload_url_template"),
		stringValue(decoded, "oss_part_upload_url_template"),
		stringValue(decoded, "part_upload_url_template"),
	)
	completeURL := firstNonEmpty(
		stringValue(decoded, "browser_complete_url"),
		stringValue(decoded, "direct_complete_url"),
		stringValue(decoded, "signed_complete_url"),
		stringValue(decoded, "presigned_complete_url"),
		stringValue(decoded, "oss_complete_url"),
		stringValue(decoded, "complete_url"),
	)
	abortURL := firstNonEmpty(
		stringValue(decoded, "browser_abort_url"),
		stringValue(decoded, "direct_abort_url"),
		stringValue(decoded, "signed_abort_url"),
		stringValue(decoded, "presigned_abort_url"),
		stringValue(decoded, "oss_abort_url"),
		stringValue(decoded, "abort_url"),
	)
	plan := &RemoteUploadSessionPlan{
		UploadID:              stringValue(decoded, "upload_id", "session_id", "id"),
		BaseURL:               stringValue(decoded, "base_url"),
		UploadURL:             uploadURL,
		PartUploadURLTemplate: partUploadURLTemplate,
		CompleteURL:           completeURL,
		AbortURL:              abortURL,
		Method:                strings.ToUpper(stringValue(decoded, "method")),
		Headers:               stringMapValue(decoded, "headers"),
		PartSizeHint:          int64Value(decoded, "part_size_hint"),
		PartsTotal:            intValue(decoded, "parts_total"),
		UploadMode:            domain.DesignAssetUploadMode(stringValue(decoded, "upload_mode", "mode")),
		SessionStatus:         normalizeRemoteSessionStatus(stringValue(decoded, "session_status", "status", "upload_status")),
		Filename:              stringValue(decoded, "filename", "file_name"),
		ExpectedSize:          int64PtrValue(decoded, "expected_size", "file_size"),
		MimeType:              stringValue(decoded, "mime_type"),
		TaskRef:               stringValue(decoded, "task_ref"),
		AssetNo:               stringValue(decoded, "asset_no"),
		VersionNo:             intValue(decoded, "version_no"),
		FileRole:              stringValue(decoded, "file_role", "asset_type"),
		StorageKey:            stringValue(decoded, "storage_key"),
		ExpiresAt:             timePtrValue(decoded, "expires_at"),
		LastSyncedAt:          timePtrValue(decoded, "last_synced_at", "updated_at"),
	}
	if fileID := stringValue(decoded, "file_id", "remote_file_id"); fileID != "" {
		plan.FileID = &fileID
	}
	return plan
}

func normalizeUploadSessionPlanMethod(plan *RemoteUploadSessionPlan) {
	if plan == nil {
		return
	}
	if method := strings.ToUpper(strings.TrimSpace(plan.Method)); method != "" {
		plan.Method = method
		return
	}
	switch plan.UploadMode {
	case domain.DesignAssetUploadModeSmall:
		plan.Method = http.MethodPost
	case domain.DesignAssetUploadModeMultipart:
		plan.Method = http.MethodPut
	default:
		if strings.TrimSpace(plan.PartUploadURLTemplate) != "" {
			plan.Method = http.MethodPut
			return
		}
		plan.Method = http.MethodPost
	}
}

func (c *httpUploadServiceClient) applyBrowserUploadTarget(plan *RemoteUploadSessionPlan) {
	if plan == nil {
		return
	}
	base := strings.TrimSpace(c.cfg.BrowserMultipartBaseURL)
	if base == "" {
		return
	}
	plan.BaseURL = rebaseBrowserURL(base, plan.BaseURL)
	plan.UploadURL = rebaseBrowserURL(base, plan.UploadURL)
	plan.PartUploadURLTemplate = rebaseBrowserURL(base, plan.PartUploadURLTemplate)
	plan.CompleteURL = rebaseBrowserURL(base, plan.CompleteURL)
	plan.AbortURL = rebaseBrowserURL(base, plan.AbortURL)
}

func normalizeBrowserMultipartBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if trimmed == "/" {
		return "/"
	}
	return strings.TrimRight(trimmed, "/")
}

func normalizeBrowserDownloadBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if trimmed == "/" {
		return "/"
	}
	return strings.TrimRight(trimmed, "/")
}

func (c *httpUploadServiceClient) applyBrowserHeaders(plan *RemoteUploadSessionPlan) {
	if plan == nil {
		return
	}
	headers := make(map[string]string)
	for key, value := range plan.Headers {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			headers[key] = value
		}
	}
	delete(headers, "X-Internal-Token")
	delete(headers, "Authorization")
	delete(headers, "authorization")
	delete(headers, "x-internal-token")
	if len(headers) == 0 {
		plan.Headers = nil
		return
	}
	plan.Headers = headers
}

func rebaseRelativeURL(baseURL, rawTarget string) string {
	target := strings.TrimSpace(rawTarget)
	if target == "" {
		return ""
	}
	parsedTarget, parseErr := url.Parse(target)
	if parseErr == nil && (parsedTarget.Scheme != "" || parsedTarget.Host != "") {
		return target
	}
	baseRaw := strings.TrimSpace(baseURL)
	base, err := url.Parse(baseRaw)
	if err != nil {
		return target
	}
	if base.Scheme == "" && base.Host == "" {
		basePath := strings.TrimRight(strings.TrimSpace(base.Path), "/")
		targetPath := target
		rest := ""
		targetPath, rest = splitURLPathSuffix(target)
		rebasedPath := rebaseTargetPath(basePath, targetPath)
		if rebasedPath == "" {
			rebasedPath = "/"
		}
		return rebasedPath + rest
	}
	if base.Scheme == "" || base.Host == "" {
		return target
	}
	if parsedTarget.Scheme == "" || parsedTarget.Host == "" {
		basePrefix := base.Scheme + "://"
		if base.User != nil {
			basePrefix += base.User.String() + "@"
		}
		basePrefix += base.Host
		basePath := strings.TrimRight(base.Path, "/")
		if strings.HasPrefix(target, "/") {
			return basePrefix + rebaseTargetPath(basePath, target)
		}
		if basePath != "" {
			return basePrefix + basePath + "/" + strings.TrimLeft(target, "/")
		}
		return basePrefix + "/" + strings.TrimLeft(target, "/")
	}
	targetPrefix := parsedTarget.Scheme + "://"
	if parsedTarget.User != nil {
		targetPrefix += parsedTarget.User.String() + "@"
	}
	targetPrefix += parsedTarget.Host
	suffix := strings.TrimPrefix(target, targetPrefix)

	basePrefix := base.Scheme + "://"
	if base.User != nil {
		basePrefix += base.User.String() + "@"
	}
	basePrefix += base.Host

	if suffix == "" {
		return basePrefix + rebaseTargetPath(strings.TrimRight(base.Path, "/"), parsedTarget.Path)
	}
	pathPart, rest := splitURLPathSuffix(suffix)
	return basePrefix + rebaseTargetPath(strings.TrimRight(base.Path, "/"), pathPart) + rest
}

func (c *httpUploadServiceClient) BuildBrowserFileURL(storageKey string) *string {
	storageKey = strings.TrimSpace(storageKey)
	if storageKey == "" {
		return nil
	}
	if strings.HasPrefix(storageKey, "http://") || strings.HasPrefix(storageKey, "https://") {
		direct := storageKey
		return &direct
	}
	base := strings.TrimSpace(c.cfg.BrowserDownloadBaseURL)
	if base == "" {
		return nil
	}
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		absolute, err := domain.BuildAbsoluteEscapedURLPath(base, "/", storageKey)
		if err != nil {
			return nil
		}
		return &absolute
	}
	relative := domain.BuildRelativeEscapedURLPath(base, storageKey)
	return &relative
}

func rebaseBrowserURL(baseURL, rawTarget string) string {
	target := strings.TrimSpace(rawTarget)
	if target == "" {
		return ""
	}
	parsedTarget, parseErr := url.Parse(target)
	if parseErr == nil && parsedTarget.Scheme != "" && parsedTarget.Host != "" {
		if !isPrivateNetworkHost(parsedTarget.Hostname()) {
			return target
		}
		pathPart := parsedTarget.EscapedPath()
		if pathPart == "" {
			pathPart = "/"
		}
		if parsedTarget.RawQuery != "" {
			pathPart += "?" + parsedTarget.RawQuery
		}
		if parsedTarget.Fragment != "" {
			pathPart += "#" + parsedTarget.Fragment
		}
		return rebaseRelativeURL(baseURL, pathPart)
	}
	return rebaseRelativeURL(baseURL, target)
}

func isPrivateNetworkHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return false
	}
	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
	}
	return false
}

func rebaseTargetPath(basePath, targetPath string) string {
	basePath = strings.TrimRight(strings.TrimSpace(basePath), "/")
	targetPath = strings.TrimSpace(targetPath)
	if basePath == "" {
		if targetPath == "" {
			return ""
		}
		if strings.HasPrefix(targetPath, "/") {
			return targetPath
		}
		return "/" + strings.TrimLeft(targetPath, "/")
	}
	if targetPath == "" || targetPath == "/" {
		return basePath
	}
	if strings.HasPrefix(targetPath, basePath+"/") || targetPath == basePath {
		return targetPath
	}
	return basePath + "/" + strings.TrimLeft(targetPath, "/")
}

func splitURLPathSuffix(raw string) (string, string) {
	if raw == "" {
		return "", ""
	}
	for i, r := range raw {
		if r == '?' || r == '#' {
			return raw[:i], raw[i:]
		}
	}
	return raw, ""
}

func buildRemotePartUploadPlan(decoded map[string]interface{}) *RemotePartUploadPlan {
	return &RemotePartUploadPlan{
		UploadID:     stringValue(decoded, "upload_id", "session_id", "id"),
		PartNumber:   intValue(decoded, "part_number"),
		Method:       strings.ToUpper(firstNonEmpty(stringValue(decoded, "method"), "PUT")),
		UploadURL:    stringValue(decoded, "upload_url", "url"),
		Headers:      stringMapValue(decoded, "headers"),
		ExpiresAt:    timePtrValue(decoded, "expires_at"),
		PartSizeHint: int64Value(decoded, "part_size_hint"),
	}
}

func buildRemoteFileMeta(decoded map[string]interface{}) *RemoteFileMeta {
	meta := &RemoteFileMeta{
		StorageKey: stringValue(decoded, "storage_key", "key"),
		FileSize:   int64PtrValue(decoded, "file_size", "size"),
		MimeType:   stringValue(decoded, "mime_type"),
		UploadedAt: timeValue(decoded, "uploaded_at", "created_at"),
	}
	if fileID := stringValue(decoded, "file_id", "remote_file_id"); fileID != "" {
		meta.FileID = &fileID
	}
	if fileHash := stringValue(decoded, "file_hash", "hash", "checksum"); fileHash != "" {
		meta.FileHash = &fileHash
	}
	return meta
}

func createUploadSessionCandidates() []string {
	return []string{
		"/upload/sessions",
		"/v1/upload-sessions",
		"/upload-sessions",
		"/api/v1/upload-sessions",
		"/v1/uploads/sessions",
		"/uploads/sessions",
	}
}

func getUploadSessionCandidates(uploadID string) []string {
	return []string{
		path.Join("/upload/sessions", uploadID),
		path.Join("/v1/upload-sessions", uploadID),
		path.Join("/upload-sessions", uploadID),
		path.Join("/api/v1/upload-sessions", uploadID),
		path.Join("/v1/uploads/sessions", uploadID),
		path.Join("/uploads/sessions", uploadID),
	}
}

func uploadSmallFileCandidates() []string {
	return []string{
		"/upload/files",
		"/v1/upload-files",
		"/upload-files",
		"/api/v1/upload-files",
		"/v1/files/upload",
	}
}

func preparePartUploadCandidates(uploadID string) []string {
	return []string{
		path.Join("/upload/sessions", uploadID, "parts"),
		path.Join("/v1/upload-sessions", uploadID, "parts"),
		path.Join("/upload-sessions", uploadID, "parts"),
		path.Join("/api/v1/upload-sessions", uploadID, "parts"),
		path.Join("/v1/uploads/sessions", uploadID, "parts"),
	}
}

func completeUploadSessionCandidates(uploadID string) []string {
	return []string{
		path.Join("/upload/sessions", uploadID, "complete"),
		path.Join("/v1/upload-sessions", uploadID, "complete"),
		path.Join("/upload-sessions", uploadID, "complete"),
		path.Join("/api/v1/upload-sessions", uploadID, "complete"),
		path.Join("/v1/uploads/sessions", uploadID, "complete"),
	}
}

func abortUploadSessionCandidates(uploadID string) []string {
	return []string{
		path.Join("/upload/sessions", uploadID, "abort"),
		path.Join("/v1/upload-sessions", uploadID, "abort"),
		path.Join("/upload-sessions", uploadID, "abort"),
		path.Join("/api/v1/upload-sessions", uploadID, "abort"),
		path.Join("/v1/uploads/sessions", uploadID, "abort"),
	}
}

func getFileMetaCandidates(fileID, uploadID string) []string {
	candidates := []string{}
	if fileID != "" {
		candidates = append(candidates,
			path.Join("/v1/files", fileID, "meta"),
			path.Join("/v1/files", fileID),
			path.Join("/files", fileID, "meta"),
			path.Join("/files", fileID),
		)
	}
	if uploadID != "" {
		candidates = append(candidates,
			path.Join("/upload/sessions", uploadID, "file-meta"),
			path.Join("/upload/sessions", uploadID, "file"),
			path.Join("/v1/upload-sessions", uploadID, "file-meta"),
			path.Join("/v1/upload-sessions", uploadID, "file"),
			path.Join("/upload-sessions", uploadID, "file-meta"),
			path.Join("/upload-sessions", uploadID, "file"),
			path.Join("/v1/uploads/sessions", uploadID, "file-meta"),
		)
	}
	return candidates
}

func appendQuery(candidates []string, query map[string]string) []string {
	appended := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		parsed, err := url.Parse(candidate)
		if err != nil {
			appended = append(appended, candidate)
			continue
		}
		values := parsed.Query()
		for key, value := range query {
			if strings.TrimSpace(value) == "" {
				continue
			}
			values.Set(key, value)
		}
		parsed.RawQuery = values.Encode()
		appended = append(appended, parsed.String())
	}
	return appended
}

func stringValue(decoded map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := decoded[key]; ok {
			switch typed := value.(type) {
			case string:
				return strings.TrimSpace(typed)
			case json.Number:
				return typed.String()
			case float64:
				return strconv.FormatInt(int64(typed), 10)
			}
		}
	}
	return ""
}

func stringMapValue(decoded map[string]interface{}, key string) map[string]string {
	raw, ok := decoded[key]
	if !ok {
		return nil
	}
	source, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	out := map[string]string{}
	for k, v := range source {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func intValue(decoded map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if value, ok := decoded[key]; ok {
			switch typed := value.(type) {
			case float64:
				return int(typed)
			case int:
				return typed
			case json.Number:
				if parsed, err := typed.Int64(); err == nil {
					return int(parsed)
				}
			case string:
				if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
					return parsed
				}
			}
		}
	}
	return 0
}

func int64Value(decoded map[string]interface{}, keys ...string) int64 {
	if value := int64PtrValue(decoded, keys...); value != nil {
		return *value
	}
	return 0
}

func int64PtrValue(decoded map[string]interface{}, keys ...string) *int64 {
	for _, key := range keys {
		if value, ok := decoded[key]; ok {
			switch typed := value.(type) {
			case float64:
				v := int64(typed)
				return &v
			case int64:
				v := typed
				return &v
			case int:
				v := int64(typed)
				return &v
			case json.Number:
				if parsed, err := typed.Int64(); err == nil {
					return &parsed
				}
			case string:
				if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
					return &parsed
				}
			}
		}
	}
	return nil
}

func timePtrValue(decoded map[string]interface{}, keys ...string) *time.Time {
	for _, key := range keys {
		if value, ok := decoded[key]; ok {
			switch typed := value.(type) {
			case string:
				if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(typed)); err == nil {
					return &parsed
				}
			}
		}
	}
	return nil
}

func timeValue(decoded map[string]interface{}, keys ...string) time.Time {
	if value := timePtrValue(decoded, keys...); value != nil {
		return *value
	}
	return time.Time{}
}

func normalizeRemoteSessionStatus(raw string) domain.DesignAssetSessionStatus {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "created", "pending", "uploading":
		return domain.DesignAssetSessionStatusCreated
	case "completed", "uploaded":
		return domain.DesignAssetSessionStatusCompleted
	case "cancelled", "canceled", "aborted":
		return domain.DesignAssetSessionStatusCancelled
	case "expired":
		return domain.DesignAssetSessionStatusExpired
	default:
		return ""
	}
}

func (c *httpUploadServiceClient) executeFullBodyRequest(ctx context.Context, method, requestURL string, headers map[string]string, body []byte) (int, []byte, error) {
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return 0, nil, fmt.Errorf("parse upload request url %q: %w", requestURL, err)
	}
	if strings.EqualFold(parsed.Scheme, "http") {
		return c.executeRawHTTPUpload(ctx, method, parsed, headers, body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	httpReq.ContentLength = int64(len(body))
	for key, value := range headers {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			httpReq.Header.Set(key, value)
		}
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	return resp.StatusCode, raw, nil
}

func (c *httpUploadServiceClient) executeRawHTTPUpload(ctx context.Context, method string, parsed *url.URL, headers map[string]string, body []byte) (int, []byte, error) {
	hostPort := parsed.Host
	if !strings.Contains(hostPort, ":") {
		hostPort = net.JoinHostPort(hostPort, "80")
	}
	dialer := &net.Dialer{Timeout: c.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", hostPort)
	if err != nil {
		return 0, nil, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	writer := bufio.NewWriter(conn)
	requestURI := parsed.RequestURI()
	if requestURI == "" {
		requestURI = "/"
	}
	if _, err := fmt.Fprintf(writer, "%s %s HTTP/1.1\r\n", method, requestURI); err != nil {
		return 0, nil, err
	}
	if _, err := fmt.Fprintf(writer, "Host: %s\r\n", parsed.Host); err != nil {
		return 0, nil, err
	}
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n", len(body)); err != nil {
		return 0, nil, err
	}
	if _, err := fmt.Fprintf(writer, "Connection: close\r\n"); err != nil {
		return 0, nil, err
	}
	for key, value := range headers {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		if _, err := fmt.Fprintf(writer, "%s: %s\r\n", key, value); err != nil {
			return 0, nil, err
		}
	}
	if _, err := writer.WriteString("\r\n"); err != nil {
		return 0, nil, err
	}
	if _, err := writer.Write(body); err != nil {
		return 0, nil, err
	}
	if err := writer.Flush(); err != nil {
		return 0, nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	return resp.StatusCode, raw, nil
}

func (c *httpUploadServiceClient) executeCurlMultipartUpload(ctx context.Context, method, requestURL string, headers map[string]string, fields []multipartFormField, fileFieldName, filename, mimeType string, fileBytes []byte) (int, []byte, error) {
	if strings.ToUpper(strings.TrimSpace(method)) != http.MethodPost {
		return c.executeFullBodyRequest(ctx, method, requestURL, headers, fileBytes)
	}
	fileHandle, err := os.CreateTemp("", "upload-service-file-*")
	if err != nil {
		return 0, nil, fmt.Errorf("create curl upload temp file: %w", err)
	}
	filePath := fileHandle.Name()
	defer os.Remove(filePath)
	if _, err := fileHandle.Write(fileBytes); err != nil {
		fileHandle.Close()
		return 0, nil, fmt.Errorf("write curl upload temp file: %w", err)
	}
	if err := fileHandle.Close(); err != nil {
		return 0, nil, fmt.Errorf("close curl upload temp file: %w", err)
	}

	headerFile, err := os.CreateTemp("", "upload-service-headers-*")
	if err != nil {
		return 0, nil, fmt.Errorf("create curl header temp file: %w", err)
	}
	headerPath := headerFile.Name()
	headerFile.Close()
	defer os.Remove(headerPath)

	bodyFile, err := os.CreateTemp("", "upload-service-body-*")
	if err != nil {
		return 0, nil, fmt.Errorf("create curl body temp file: %w", err)
	}
	bodyPath := bodyFile.Name()
	bodyFile.Close()
	defer os.Remove(bodyPath)

	args := []string{
		"--http1.1",
		"-sS",
		"-D", headerPath,
		"-o", bodyPath,
		"-w", "%{http_code}",
		"-X", http.MethodPost,
		requestURL,
	}
	for key, value := range headers {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" || strings.EqualFold(key, "Content-Type") {
			continue
		}
		args = append(args, "-H", fmt.Sprintf("%s: %s", key, value))
	}
	for _, field := range fields {
		if strings.TrimSpace(field.Key) == "" || field.Value == "" {
			continue
		}
		args = append(args, "-F", fmt.Sprintf("%s=%s", field.Key, field.Value))
	}
	args = append(args, "-F", fmt.Sprintf("%s=@%s;filename=%s;type=%s", fileFieldName, filePath, filename, firstNonEmpty(mimeType, "application/octet-stream")))

	cmd := exec.CommandContext(ctx, "curl", args...)
	statusOut, err := cmd.CombinedOutput()
	if err != nil {
		return 0, nil, fmt.Errorf("curl multipart upload command failed: %w output=%s", err, strings.TrimSpace(string(statusOut)))
	}
	statusCode, err := strconv.Atoi(strings.TrimSpace(string(statusOut)))
	if err != nil {
		return 0, nil, fmt.Errorf("parse curl status code %q: %w", string(statusOut), err)
	}
	rawBody, err := os.ReadFile(bodyPath)
	if err != nil {
		return 0, nil, fmt.Errorf("read curl body temp file: %w", err)
	}
	return statusCode, rawBody, nil
}

func buildMultipartBody(fileFieldName, filename, mimeType string, file io.Reader, fields []multipartFormField) ([]byte, string, []byte, string, error) {
	fileBytesRaw, err := io.ReadAll(file)
	if err != nil {
		return nil, "", nil, "", fmt.Errorf("read upload file bytes: %w", err)
	}
	if len(fileBytesRaw) == 0 {
		return nil, "", nil, "", fmt.Errorf("upload file bytes are empty")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range fields {
		if strings.TrimSpace(field.Key) == "" || field.Value == "" {
			continue
		}
		if err := writer.WriteField(field.Key, field.Value); err != nil {
			return nil, "", nil, "", fmt.Errorf("write multipart field %s: %w", field.Key, err)
		}
	}

	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeMultipartValue(fileFieldName), escapeMultipartValue(filename)))
	partHeader.Set("Content-Type", firstNonEmpty(strings.TrimSpace(mimeType), "application/octet-stream"))
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return nil, "", nil, "", fmt.Errorf("create multipart file part: %w", err)
	}
	if _, err := part.Write(fileBytesRaw); err != nil {
		return nil, "", nil, "", fmt.Errorf("write multipart file part: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, "", nil, "", fmt.Errorf("close multipart writer: %w", err)
	}

	sum := sha256.Sum256(fileBytesRaw)
	return body.Bytes(), writer.FormDataContentType(), fileBytesRaw, hex.EncodeToString(sum[:]), nil
}

func multipartFieldNames(fields []multipartFormField) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field.Key) == "" || field.Value == "" {
			continue
		}
		names = append(names, field.Key)
	}
	return names
}

func escapeMultipartValue(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", `"`, "\\\"")
	return replacer.Replace(value)
}

func mustMarshalJSON(value interface{}) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf(`{"marshal_error":%q}`, err.Error())
	}
	return string(raw)
}

func int64ValueFromPtr(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func logUploadProbe(event string, fields map[string]interface{}) {
	if strings.TrimSpace(event) == "" {
		event = "upload_service_probe"
	}
	if fields == nil {
		fields = map[string]interface{}{}
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		log.Printf("%s marshal_error=%q", event, err.Error())
		return
	}
	log.Printf("%s %s", event, string(raw))
}

func metaFileID(meta *RemoteFileMeta) *string {
	if meta == nil {
		return nil
	}
	return meta.FileID
}

func metaFileSize(meta *RemoteFileMeta) *int64 {
	if meta == nil {
		return nil
	}
	return meta.FileSize
}

func metaStorageKey(meta *RemoteFileMeta) string {
	if meta == nil {
		return ""
	}
	return strings.TrimSpace(meta.StorageKey)
}

func metaMimeType(meta *RemoteFileMeta) string {
	if meta == nil {
		return ""
	}
	return strings.TrimSpace(meta.MimeType)
}

func metaIsStub(meta *RemoteFileMeta) bool {
	return meta != nil && meta.IsStub
}
