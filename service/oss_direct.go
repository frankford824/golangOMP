package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
)

type OSSDirectConfig struct {
	Enabled         bool
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	PresignExpiry   time.Duration
	HTTPTimeout     time.Duration
	PublicEndpoint  string
	PartSize        int64
}

type OSSDirectService struct {
	cfg        OSSDirectConfig
	httpClient *http.Client
	nowFn      func() time.Time
}

var ossObjectExtensionPattern = regexp.MustCompile(`^[A-Za-z0-9]{1,10}$`)

func NewOSSDirectService(cfg OSSDirectConfig) *OSSDirectService {
	if cfg.PresignExpiry <= 0 {
		cfg.PresignExpiry = 15 * time.Minute
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 5 * time.Minute
	}
	if cfg.PublicEndpoint == "" {
		cfg.PublicEndpoint = cfg.Endpoint
	}
	if cfg.PartSize <= 0 {
		cfg.PartSize = 10 * 1024 * 1024
	}
	return &OSSDirectService{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.HTTPTimeout},
		nowFn:      time.Now,
	}
}

func (s *OSSDirectService) Enabled() bool {
	return s != nil && s.cfg.Enabled &&
		strings.TrimSpace(s.cfg.Endpoint) != "" &&
		strings.TrimSpace(s.cfg.Bucket) != "" &&
		strings.TrimSpace(s.cfg.AccessKeyID) != "" &&
		strings.TrimSpace(s.cfg.AccessKeySecret) != ""
}

func (s *OSSDirectService) Config() OSSDirectConfig {
	if s == nil {
		return OSSDirectConfig{}
	}
	return s.cfg
}

type OSSMultipartInit struct {
	UploadID  string
	ObjectKey string
	Bucket    string
}

type OSSPresignedPart struct {
	PartNumber int       `json:"part_number"`
	UploadURL  string    `json:"upload_url"`
	Method     string    `json:"method"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type OSSDirectUploadPlan struct {
	Mode                string             `json:"mode"`
	ObjectKey           string             `json:"object_key"`
	UploadID            string             `json:"upload_id,omitempty"`
	UploadURL           string             `json:"upload_url,omitempty"`
	Parts               []OSSPresignedPart `json:"parts,omitempty"`
	PartSize            int64              `json:"part_size,omitempty"`
	ExpiresAt           time.Time          `json:"expires_at"`
	Method              string             `json:"method,omitempty"`
	Bucket              string             `json:"bucket,omitempty"`
	Endpoint            string             `json:"endpoint,omitempty"`
	RequiredContentType string             `json:"required_upload_content_type,omitempty"`
}

type OSSDirectDownloadInfo struct {
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type OSSObjectInfo struct {
	ContentLength int64
	ContentType   string
	ETag          string
}

type OSSCompletePart struct {
	PartNumber int    `xml:"PartNumber" json:"part_number"`
	ETag       string `xml:"ETag" json:"etag"`
}

const DownloadFilenameQueryParam = "download_filename"

func (s *OSSDirectService) BuildObjectKey(taskRef, assetNo string, versionNo int, assetType domain.TaskAssetType, filename string) string {
	roleSubdir := assetTypeToSubdir(assetType)
	storageFilename := asciiStorageFilename(filename)
	return fmt.Sprintf("tasks/%s/assets/%s/v%d/%s/%s", taskRef, assetNo, versionNo, roleSubdir, storageFilename)
}

func (s *OSSDirectService) BuildUploadSessionObjectKey(taskRef, sessionID, filename string) string {
	taskRef = sanitizeOSSObjectKeySegment(taskRef, "task")
	sessionID = sanitizeOSSObjectKeySegment(sessionID, "session")
	storageFilename := sessionID
	if ext := validObjectKeyExtension(filename); ext != "" {
		storageFilename += "." + strings.ToLower(ext)
	}
	return fmt.Sprintf("tasks/%s/upload-sessions/%s/%s", taskRef, sessionID, storageFilename)
}

func (s *OSSDirectService) CreateUploadPlan(ctx context.Context, objectKey string, fileSize int64, contentType string) (*OSSDirectUploadPlan, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("oss direct service is not enabled")
	}
	if fileSize <= s.cfg.PartSize {
		return s.CreateSingleUploadPlan(objectKey, contentType)
	}
	return s.CreateMultipartUploadPlan(ctx, objectKey, fileSize, contentType)
}

func (s *OSSDirectService) UsesMultipartUpload(fileSize int64) bool {
	return s != nil && fileSize > s.cfg.PartSize
}

func normalizeRequiredUploadContentType(contentType string) string {
	if trimmed := strings.TrimSpace(contentType); trimmed != "" {
		return trimmed
	}
	return "application/octet-stream"
}

func (s *OSSDirectService) CreateMultipartUploadPlan(ctx context.Context, objectKey string, fileSize int64, contentType string) (*OSSDirectUploadPlan, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("oss direct service is not enabled")
	}
	contentType = normalizeRequiredUploadContentType(contentType)
	partSize := s.cfg.PartSize
	partsTotal := int((fileSize + partSize - 1) / partSize)
	if partsTotal <= 0 {
		partsTotal = 1
	}
	if fileSize <= 0 {
		partsTotal = 1
	}
	if partsTotal > 10000 {
		return nil, fmt.Errorf("file too large: %d parts exceeds OSS limit of 10000", partsTotal)
	}

	init, err := s.initiateMultipartUpload(ctx, objectKey)
	if err != nil {
		return nil, err
	}

	parts := make([]OSSPresignedPart, partsTotal)
	for i := 0; i < partsTotal; i++ {
		parts[i] = s.presignPartUploadURL(objectKey, init.UploadID, i+1, contentType)
	}

	expires := s.now().Add(s.cfg.PresignExpiry)

	log.Printf("oss_direct_create_multipart_plan object_key=%s upload_id=%s parts_total=%d part_size=%d",
		objectKey, init.UploadID, partsTotal, partSize)

	return &OSSDirectUploadPlan{
		Mode:                "multipart",
		ObjectKey:           objectKey,
		UploadID:            init.UploadID,
		Parts:               parts,
		PartSize:            partSize,
		ExpiresAt:           expires,
		Method:              http.MethodPut,
		Bucket:              s.cfg.Bucket,
		Endpoint:            s.cfg.PublicEndpoint,
		RequiredContentType: contentType,
	}, nil
}

func (s *OSSDirectService) CreateSingleUploadPlan(objectKey, contentType string) (*OSSDirectUploadPlan, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("oss direct service is not enabled")
	}
	contentType = normalizeRequiredUploadContentType(contentType)

	expires := s.now().Add(s.cfg.PresignExpiry)
	expiresStr := strconv.FormatInt(expires.Unix(), 10)

	canonResource := "/" + s.cfg.Bucket + "/" + objectKey
	sig := s.signV1(http.MethodPut, "", contentType, expiresStr, "", canonResource)

	presignURL := s.publicBucketURL() + "/" + ossEscapePath(objectKey) +
		"?OSSAccessKeyId=" + url.QueryEscape(s.cfg.AccessKeyID) +
		"&Expires=" + expiresStr +
		"&Signature=" + url.QueryEscape(sig)

	log.Printf("oss_direct_create_single_plan object_key=%s", objectKey)

	return &OSSDirectUploadPlan{
		Mode:                "single_part",
		ObjectKey:           objectKey,
		UploadURL:           presignURL,
		ExpiresAt:           expires,
		Method:              http.MethodPut,
		Bucket:              s.cfg.Bucket,
		Endpoint:            s.cfg.PublicEndpoint,
		RequiredContentType: contentType,
	}, nil
}

func (s *OSSDirectService) CompleteMultipartUpload(ctx context.Context, objectKey, uploadID string, parts []OSSCompletePart) error {
	if !s.Enabled() {
		return fmt.Errorf("oss direct service is not enabled")
	}

	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	type completeBody struct {
		XMLName xml.Name          `xml:"CompleteMultipartUpload"`
		Parts   []OSSCompletePart `xml:"Part"`
	}
	xmlBody, err := xml.Marshal(completeBody{Parts: parts})
	if err != nil {
		return fmt.Errorf("oss complete multipart marshal: %w", err)
	}

	reqURL := s.bucketURL() + "/" + ossEscapePath(objectKey) + "?uploadId=" + url.QueryEscape(uploadID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(xmlBody))
	if err != nil {
		return fmt.Errorf("oss complete multipart upload: %w", err)
	}

	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)
	req.Header.Set("Content-Type", "application/xml")

	canonResource := "/" + s.cfg.Bucket + "/" + objectKey + "?uploadId=" + uploadID
	sig := s.signV1(http.MethodPost, "", "application/xml", date, "", canonResource)
	req.Header.Set("Authorization", "OSS "+s.cfg.AccessKeyID+":"+sig)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("oss complete multipart upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("oss complete multipart upload: status=%d body=%s", resp.StatusCode, string(body))
	}

	log.Printf("oss_direct_complete_multipart object_key=%s upload_id=%s parts=%d", objectKey, uploadID, len(parts))
	return nil
}

func (s *OSSDirectService) AbortMultipartUpload(ctx context.Context, objectKey, uploadID string) error {
	if !s.Enabled() {
		return fmt.Errorf("oss direct service is not enabled")
	}

	reqURL := s.bucketURL() + "/" + ossEscapePath(objectKey) + "?uploadId=" + url.QueryEscape(uploadID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("oss abort multipart upload: %w", err)
	}

	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)

	canonResource := "/" + s.cfg.Bucket + "/" + objectKey + "?uploadId=" + uploadID
	sig := s.signV1(http.MethodDelete, "", "", date, "", canonResource)
	req.Header.Set("Authorization", "OSS "+s.cfg.AccessKeyID+":"+sig)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("oss abort multipart upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("oss abort multipart upload: status=%d body=%s", resp.StatusCode, string(body))
	}

	log.Printf("oss_direct_abort_multipart object_key=%s upload_id=%s", objectKey, uploadID)
	return nil
}

func (s *OSSDirectService) PresignDownloadURL(objectKey string) *OSSDirectDownloadInfo {
	return s.presignGetURL(objectKey, "attachment")
}

func (s *OSSDirectService) PresignDownloadURLWithFilename(objectKey, filename string) *OSSDirectDownloadInfo {
	return s.presignGetURL(objectKey, attachmentContentDisposition(filename))
}

func (s *OSSDirectService) PresignPreviewURL(objectKey string) *OSSDirectDownloadInfo {
	return s.presignGetURL(objectKey, "inline")
}

func (s *OSSDirectService) PresignPreviewURLWithProcess(objectKey, process string) *OSSDirectDownloadInfo {
	process = strings.TrimSpace(process)
	if process == "" {
		return s.PresignPreviewURL(objectKey)
	}
	return s.presignGetURLWithQuery(objectKey, map[string]string{
		"response-content-disposition": "inline",
		"x-oss-process":                process,
	})
}

func (s *OSSDirectService) UploadObject(ctx context.Context, objectKey, contentType string, body []byte) error {
	if !s.Enabled() {
		return fmt.Errorf("oss direct service is not enabled")
	}
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return fmt.Errorf("oss direct upload object_key is required")
	}
	if len(body) == 0 {
		return fmt.Errorf("oss direct upload body is empty")
	}
	contentType = normalizeRequiredUploadContentType(contentType)

	reqURL := s.bucketURL() + "/" + ossEscapePath(objectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("oss direct upload build request: %w", err)
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)
	req.Header.Set("Content-Type", contentType)

	canonResource := "/" + s.cfg.Bucket + "/" + objectKey
	sig := s.signV1(http.MethodPut, "", contentType, date, "", canonResource)
	req.Header.Set("Authorization", "OSS "+s.cfg.AccessKeyID+":"+sig)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("oss direct upload request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("oss direct upload failed: status=%d body=%s", resp.StatusCode, string(raw))
	}
	return nil
}

func (s *OSSDirectService) UploadObjectFromReader(ctx context.Context, objectKey, contentType string, body io.Reader) error {
	if !s.Enabled() {
		return fmt.Errorf("oss direct service is not enabled")
	}
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return fmt.Errorf("oss direct upload object_key is required")
	}
	if body == nil {
		return fmt.Errorf("oss direct upload body is nil")
	}
	contentType = normalizeRequiredUploadContentType(contentType)

	reqURL := s.bucketURL() + "/" + ossEscapePath(objectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, body)
	if err != nil {
		return fmt.Errorf("oss direct upload reader build request: %w", err)
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)
	req.Header.Set("Content-Type", contentType)

	canonResource := "/" + s.cfg.Bucket + "/" + objectKey
	sig := s.signV1(http.MethodPut, "", contentType, date, "", canonResource)
	req.Header.Set("Authorization", "OSS "+s.cfg.AccessKeyID+":"+sig)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("oss direct upload reader request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("oss direct upload reader failed: status=%d body=%s", resp.StatusCode, string(raw))
	}
	return nil
}

func (s *OSSDirectService) HeadObject(ctx context.Context, objectKey string) (bool, error) {
	_, exists, err := s.StatObject(ctx, objectKey)
	return exists, err
}

// PresignERPImageURL omits content-disposition so the URL stays within ERP
// image-field limits. It remains a time-limited, signed private-object URL.
func (s *OSSDirectService) PresignERPImageURL(objectKey string) *OSSDirectDownloadInfo {
	return s.presignGetURLWithQuery(objectKey, nil)
}

func (s *OSSDirectService) StatObject(ctx context.Context, objectKey string) (*OSSObjectInfo, bool, error) {
	if !s.Enabled() {
		return nil, false, fmt.Errorf("oss direct service is not enabled")
	}
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil, false, fmt.Errorf("oss direct head object_key is required")
	}
	reqURL := s.bucketURL() + "/" + ossEscapePath(objectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, reqURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("oss direct head build request: %w", err)
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)

	canonResource := "/" + s.cfg.Bucket + "/" + objectKey
	sig := s.signV1(http.MethodHead, "", "", date, "", canonResource)
	req.Header.Set("Authorization", "OSS "+s.cfg.AccessKeyID+":"+sig)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("oss direct head request: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return &OSSObjectInfo{
			ContentLength: resp.ContentLength,
			ContentType:   strings.TrimSpace(resp.Header.Get("Content-Type")),
			ETag:          strings.Trim(strings.TrimSpace(resp.Header.Get("ETag")), `"`),
		}, true, nil
	case http.StatusNotFound:
		return nil, false, nil
	default:
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, false, fmt.Errorf("oss direct head failed: status=%d body=%s", resp.StatusCode, string(raw))
	}
}

func (s *OSSDirectService) OpenObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("oss direct service is not enabled")
	}
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil, fmt.Errorf("oss direct open object_key is required")
	}
	reqURL := s.bucketURL() + "/" + ossEscapePath(objectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("oss direct open build request: %w", err)
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)

	canonResource := "/" + s.cfg.Bucket + "/" + objectKey
	sig := s.signV1(http.MethodGet, "", "", date, "", canonResource)
	req.Header.Set("Authorization", "OSS "+s.cfg.AccessKeyID+":"+sig)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oss direct open request: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("oss direct open failed: status=%d body=%s", resp.StatusCode, string(raw))
	}
	return resp.Body, nil
}

func (s *OSSDirectService) CopyObject(ctx context.Context, srcKey, dstKey string) error {
	if !s.Enabled() {
		return fmt.Errorf("oss direct service is not enabled")
	}
	srcKey = strings.TrimSpace(srcKey)
	dstKey = strings.TrimSpace(dstKey)
	if srcKey == "" || dstKey == "" {
		return fmt.Errorf("oss direct copy src_key and dst_key are required")
	}
	reqURL := s.bucketURL() + "/" + ossEscapePath(dstKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, nil)
	if err != nil {
		return fmt.Errorf("oss direct copy build request: %w", err)
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	copySource := "/" + s.cfg.Bucket + "/" + ossEscapePath(srcKey)
	req.Header.Set("Date", date)
	req.Header.Set("x-oss-copy-source", copySource)

	canonHeaders := canonicalOSSHeaders(map[string]string{"x-oss-copy-source": copySource})
	canonResource := "/" + s.cfg.Bucket + "/" + dstKey
	sig := s.signV1(http.MethodPut, "", "", date, canonHeaders, canonResource)
	req.Header.Set("Authorization", "OSS "+s.cfg.AccessKeyID+":"+sig)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("oss direct copy request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("oss direct copy failed: status=%d body=%s", resp.StatusCode, string(raw))
	}
	return nil
}

func (s *OSSDirectService) DeleteObject(ctx context.Context, objectKey string) error {
	if !s.Enabled() {
		return fmt.Errorf("oss direct service is not enabled")
	}
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return fmt.Errorf("oss direct delete object_key is required")
	}
	reqURL := s.bucketURL() + "/" + ossEscapePath(objectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("oss direct delete build request: %w", err)
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)

	canonResource := "/" + s.cfg.Bucket + "/" + objectKey
	sig := s.signV1(http.MethodDelete, "", "", date, "", canonResource)
	req.Header.Set("Authorization", "OSS "+s.cfg.AccessKeyID+":"+sig)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("oss direct delete request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("oss direct delete failed: status=%d body=%s", resp.StatusCode, string(raw))
	}
	return nil
}

func (s *OSSDirectService) presignGetURL(objectKey, disposition string) *OSSDirectDownloadInfo {
	return s.presignGetURLWithQuery(objectKey, map[string]string{
		"response-content-disposition": strings.TrimSpace(disposition),
	})
}

func (s *OSSDirectService) presignGetURLWithQuery(objectKey string, subresources map[string]string) *OSSDirectDownloadInfo {
	if !s.Enabled() || strings.TrimSpace(objectKey) == "" {
		return nil
	}

	expires := s.now().Add(s.cfg.PresignExpiry)
	expiresStr := strconv.FormatInt(expires.Unix(), 10)

	keys := make([]string, 0, len(subresources))
	normalized := map[string]string{}
	for key, value := range subresources {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		normalized[key] = value
		keys = append(keys, key)
	}
	sort.Strings(keys)

	canonResource := "/" + s.cfg.Bucket + "/" + objectKey
	if len(keys) > 0 {
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, key+"="+normalized[key])
		}
		canonResource += "?" + strings.Join(parts, "&")
	}
	sig := s.signV1(http.MethodGet, "", "", expiresStr, "", canonResource)

	queryParts := []string{
		"OSSAccessKeyId=" + url.QueryEscape(s.cfg.AccessKeyID),
		"Expires=" + expiresStr,
		"Signature=" + url.QueryEscape(sig),
	}
	for _, key := range keys {
		queryParts = append(queryParts, key+"="+url.QueryEscape(normalized[key]))
	}
	presignURL := s.publicBucketURL() + "/" + ossEscapePath(objectKey) + "?" + strings.Join(queryParts, "&")

	return &OSSDirectDownloadInfo{
		DownloadURL: presignURL,
		ExpiresAt:   expires,
	}
}

func (s *OSSDirectService) initiateMultipartUpload(ctx context.Context, objectKey string) (*OSSMultipartInit, error) {
	reqURL := s.bucketURL() + "/" + ossEscapePath(objectKey) + "?uploads"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("oss initiate multipart upload build request: %w", err)
	}

	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)
	req.Header.Set("Content-Type", "application/octet-stream")

	canonResource := "/" + s.cfg.Bucket + "/" + objectKey + "?uploads"
	sig := s.signV1(http.MethodPost, "", "application/octet-stream", date, "", canonResource)
	req.Header.Set("Authorization", "OSS "+s.cfg.AccessKeyID+":"+sig)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oss initiate multipart upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("oss initiate multipart upload: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
		Bucket   string   `xml:"Bucket"`
		Key      string   `xml:"Key"`
		UploadID string   `xml:"UploadId"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("oss initiate multipart upload decode: %w", err)
	}

	return &OSSMultipartInit{
		UploadID:  result.UploadID,
		ObjectKey: objectKey,
		Bucket:    s.cfg.Bucket,
	}, nil
}

func (s *OSSDirectService) presignPartUploadURL(objectKey, uploadID string, partNumber int, contentType string) OSSPresignedPart {
	contentType = normalizeRequiredUploadContentType(contentType)
	expires := s.now().Add(s.cfg.PresignExpiry)
	expiresStr := strconv.FormatInt(expires.Unix(), 10)

	canonResource := "/" + s.cfg.Bucket + "/" + objectKey + "?partNumber=" + strconv.Itoa(partNumber) + "&uploadId=" + uploadID

	sig := s.signV1(http.MethodPut, "", contentType, expiresStr, "", canonResource)

	presignURL := s.publicBucketURL() + "/" + ossEscapePath(objectKey) +
		"?partNumber=" + strconv.Itoa(partNumber) +
		"&uploadId=" + url.QueryEscape(uploadID) +
		"&OSSAccessKeyId=" + url.QueryEscape(s.cfg.AccessKeyID) +
		"&Expires=" + expiresStr +
		"&Signature=" + url.QueryEscape(sig)

	return OSSPresignedPart{
		PartNumber: partNumber,
		UploadURL:  presignURL,
		Method:     http.MethodPut,
		ExpiresAt:  expires,
	}
}

func (s *OSSDirectService) signV1(verb, contentMD5, contentType, dateOrExpires, ossHeaders, canonResource string) string {
	stringToSign := verb + "\n" + contentMD5 + "\n" + contentType + "\n" + dateOrExpires + "\n"
	if ossHeaders != "" {
		stringToSign += ossHeaders
	}
	stringToSign += canonResource

	mac := hmac.New(sha1.New, []byte(s.cfg.AccessKeySecret))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (s *OSSDirectService) now() time.Time {
	if s == nil || s.nowFn == nil {
		return time.Now()
	}
	return s.nowFn()
}

func canonicalOSSHeaders(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}
	keys := make([]string, 0, len(headers))
	normalized := map[string]string{}
	for key, value := range headers {
		key = strings.ToLower(strings.TrimSpace(key))
		if !strings.HasPrefix(key, "x-oss-") {
			continue
		}
		keys = append(keys, key)
		normalized[key] = strings.TrimSpace(value)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte(':')
		builder.WriteString(normalized[key])
		builder.WriteByte('\n')
	}
	return builder.String()
}

func (s *OSSDirectService) bucketURL() string {
	return "https://" + s.cfg.Bucket + "." + strings.TrimSpace(s.cfg.Endpoint)
}

func (s *OSSDirectService) publicBucketURL() string {
	endpoint := strings.TrimSpace(s.cfg.PublicEndpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(s.cfg.Endpoint)
	}
	return "https://" + s.cfg.Bucket + "." + endpoint
}

func ossEscapePath(objectKey string) string {
	parts := strings.Split(objectKey, "/")
	escaped := make([]string, len(parts))
	for i, part := range parts {
		escaped[i] = strings.ReplaceAll(url.PathEscape(part), "+", "%2B")
	}
	return strings.Join(escaped, "/")
}

func ResolveAssetDownloadFilename(originalFilename, fileName string, assetID int64) string {
	if filename := strings.TrimSpace(originalFilename); filename != "" {
		return filename
	}
	if filename := strings.TrimSpace(fileName); filename != "" {
		return filename
	}
	if assetID > 0 {
		return "asset-" + strconv.FormatInt(assetID, 10)
	}
	return "asset"
}

func ResolveAssetDownloadFilenameForSingle(originalFilename, fileName string, assetID int64, skuCode string) string {
	if filename := strings.TrimSpace(originalFilename); filename != "" {
		return filename
	}
	if filename := composeBusinessAssetFilename(skuCode, fileName, fileName); filename != "" {
		return filename
	}
	return ResolveAssetDownloadFilename("", fileName, assetID)
}

func ResolveAssetDownloadFilenameForBusiness(originalFilename, fileName string, assetID int64, skuCode, businessName string) string {
	if filename := composeBusinessAssetFilename(skuCode, businessName, firstNonEmptyTrimmed(originalFilename, fileName)); filename != "" {
		return filename
	}
	if filename := composeBusinessAssetFilename(skuCode, fileName, fileName); filename != "" {
		return filename
	}
	return ResolveAssetDownloadFilename(originalFilename, fileName, assetID)
}

func composeBusinessAssetFilename(skuCode, label, extensionSource string) string {
	sku := sanitizeDownloadFilenamePart(skuCode)
	base := sanitizeDownloadFilenamePart(stripFilenameExtension(label))
	if sku == "" || base == "" {
		return ""
	}
	name := sku + "-" + base
	ext := safeDownloadExtension(extensionSource)
	if ext != "" {
		name += ext
	}
	return name
}

func stripFilenameExtension(value string) string {
	value = strings.TrimSpace(value)
	ext := safeDownloadExtension(value)
	if ext == "" {
		return value
	}
	return strings.TrimSpace(value[:len(value)-len(ext)])
}

func safeDownloadExtension(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lastSlash := strings.LastIndexAny(value, `/\`)
	lastDot := strings.LastIndex(value, ".")
	if lastDot <= lastSlash || lastDot < 0 || lastDot == len(value)-1 {
		return ""
	}
	ext := value[lastDot:]
	if len(ext) > 12 {
		return ""
	}
	if !isKnownDownloadExtension(strings.ToLower(ext)) {
		return ""
	}
	for _, r := range ext[1:] {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		return ""
	}
	return ext
}

func isKnownDownloadExtension(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".svg", ".heic", ".heif":
		return true
	case ".psd", ".psb", ".tif", ".tiff", ".ai", ".cdr", ".pdf":
		return true
	case ".zip", ".rar", ".7z", ".xlsx", ".xls":
		return true
	default:
		return false
	}
}

func sanitizeDownloadFilenamePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		`"`, "_",
		"<", "_",
		">", "_",
		"|", "_",
		"\x00", "",
		"\r", "_",
		"\n", "_",
	)
	value = replacer.Replace(value)
	value = strings.ReplaceAll(value, "..", "_")
	return strings.TrimSpace(value)
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func ContentDispositionAttachment(filename string) string {
	return attachmentContentDisposition(filename)
}

func attachmentContentDisposition(filename string) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "attachment"
	}
	if value := mime.FormatMediaType("attachment", map[string]string{"filename": filename}); value != "" {
		return value
	}
	return "attachment"
}

func AppendProxyDownloadFilenameQuery(rawURL, filename string) string {
	rawURL = strings.TrimSpace(rawURL)
	filename = strings.TrimSpace(filename)
	if rawURL == "" || filename == "" {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = parsed.Path
	}
	if !strings.HasPrefix(path, "/v1/assets/files/") && !strings.HasPrefix(path, "/files/") {
		return rawURL
	}
	query := parsed.Query()
	query.Set(DownloadFilenameQueryParam, filename)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func assetTypeToSubdir(assetType domain.TaskAssetType) string {
	switch {
	case assetType.IsSource():
		return "source"
	case assetType.IsDelivery():
		return "delivery"
	case assetType.IsPreview():
		return "preview"
	case assetType.IsDesignThumb():
		return "design_thumb"
	case assetType.IsReference():
		return "derived"
	default:
		return "derived"
	}
}

func asciiStorageFilename(filename string) string {
	base := strconv.FormatInt(time.Now().UnixNano(), 10) + "_" + randomHex8()
	if ext := validObjectKeyExtension(filename); ext != "" {
		return base + "." + ext
	}
	return base
}

func validObjectKeyExtension(filename string) string {
	s := strings.TrimSpace(filename)
	idx := strings.LastIndexAny(s, "/\\")
	if idx >= 0 {
		s = s[idx+1:]
	}
	dot := strings.LastIndex(s, ".")
	if dot < 0 || dot == len(s)-1 {
		return ""
	}
	ext := s[dot+1:]
	if ossObjectExtensionPattern.MatchString(ext) {
		return ext
	}
	return ""
}

func sanitizeOSSObjectKeySegment(value, fallback string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	result := strings.Trim(builder.String(), "._-")
	if result == "" {
		return fallback
	}
	return result
}

func randomHex8() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%08x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}
