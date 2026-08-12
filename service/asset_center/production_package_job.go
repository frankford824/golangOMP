package asset_center

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	baseservice "workflow/service"
)

const (
	productionPackageJobMaxFiles = 20000
	productionPackageJobMaxBytes = int64(20 * 1024 * 1024 * 1024)
)

type ProductionPackageObjectStore interface {
	Enabled() bool
	CreateUploadPlan(context.Context, string, int64, string) (*baseservice.OSSDirectUploadPlan, error)
	CompleteMultipartUpload(context.Context, string, string, []baseservice.OSSCompletePart) error
	AbortMultipartUpload(context.Context, string, string) error
	PresignDownloadURLWithFilename(string, string) *baseservice.OSSDirectDownloadInfo
}

type ProductionPackageJobRequest struct {
	Rows         []ExcelPackageRow `json:"rows"`
	FormatFilter string            `json:"format_filter,omitempty"`
}

type ProductionPackageJobResult struct {
	ObjectKey string                `json:"object_key"`
	Filename  string                `json:"filename"`
	Manifest  *ExcelPackageManifest `json:"manifest"`
}

type ProductionPackageJobView struct {
	JobID          string                            `json:"job_id"`
	Status         domain.ProductionPackageJobStatus `json:"status"`
	TotalCount     int                               `json:"total_count"`
	ProcessedCount int                               `json:"processed_count"`
	FailedCount    int                               `json:"failed_count"`
	ErrorMessage   string                            `json:"error_message,omitempty"`
	DownloadURL    string                            `json:"download_url,omitempty"`
	Filename       string                            `json:"filename,omitempty"`
	Manifest       *ExcelPackageManifest             `json:"manifest,omitempty"`
	CreatedAt      time.Time                         `json:"created_at"`
	StartedAt      *time.Time                        `json:"started_at,omitempty"`
	FinishedAt     *time.Time                        `json:"finished_at,omitempty"`
	ExpiresAt      *time.Time                        `json:"expires_at,omitempty"`
}

func (s *Service) CreateProductionPackageJob(ctx context.Context, actorID int64, request ProductionPackageJobRequest) (*ProductionPackageJobView, *domain.AppError) {
	if s == nil || s.packageJobRepo == nil || s.packageStore == nil || !s.packageStore.Enabled() {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "production package worker is not configured", nil)
	}
	if actorID <= 0 {
		return nil, domain.ErrUnauthorized
	}
	if len(request.Rows) == 0 || len(request.Rows) > MaxExcelPackageRows {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rows must contain between 1 and 500 items", nil)
	}
	for index := range request.Rows {
		request.Rows[index] = normalizeExcelPackageRow(request.Rows[index], index+2)
	}
	request.FormatFilter = normalizeExcelPackageFormat(request.FormatFilter)
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	job := &domain.ProductionPackageJob{
		JobID: "pkg-" + uuid.NewString(), Status: domain.ProductionPackageJobQueued,
		RequestedBy: actorID, RequestPayload: payload, TotalCount: len(request.Rows),
	}
	if err := s.packageJobRepo.Create(ctx, job); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return &ProductionPackageJobView{JobID: job.JobID, Status: job.Status, TotalCount: job.TotalCount, CreatedAt: time.Now().UTC()}, nil
}

func (s *Service) GetProductionPackageJob(ctx context.Context, actorID int64, jobID string) (*ProductionPackageJobView, *domain.AppError) {
	if s == nil || s.packageJobRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "production package worker is not configured", nil)
	}
	job, err := s.packageJobRepo.Get(ctx, strings.TrimSpace(jobID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if job.RequestedBy != actorID {
		return nil, domain.ErrNotFound
	}
	view := &ProductionPackageJobView{
		JobID: job.JobID, Status: job.Status, TotalCount: job.TotalCount,
		ProcessedCount: job.ProcessedCount, FailedCount: job.FailedCount,
		ErrorMessage: job.ErrorMessage, CreatedAt: job.CreatedAt,
		StartedAt: job.StartedAt, FinishedAt: job.FinishedAt,
	}
	if len(job.ResultPayload) > 0 && string(job.ResultPayload) != "{}" {
		var result ProductionPackageJobResult
		if err := json.Unmarshal(job.ResultPayload, &result); err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "invalid production package result", nil)
		}
		view.Filename = result.Filename
		view.Manifest = result.Manifest
		if job.Status == domain.ProductionPackageJobSucceeded && strings.TrimSpace(result.ObjectKey) != "" {
			if signed := s.packageStore.PresignDownloadURLWithFilename(result.ObjectKey, result.Filename); signed != nil {
				view.DownloadURL = signed.DownloadURL
				view.ExpiresAt = &signed.ExpiresAt
			}
		}
	}
	return view, nil
}

func (s *Service) ProcessProductionPackageJobs(ctx context.Context, workerID string, limit int) (int, error) {
	if s == nil || s.packageJobRepo == nil || s.packageStore == nil {
		return 0, nil
	}
	jobs, err := s.packageJobRepo.Claim(ctx, workerID, limit, time.Now().UTC().Add(30*time.Minute))
	if err != nil {
		return 0, err
	}
	for _, job := range jobs {
		if err := s.processProductionPackageJob(ctx, workerID, job); err != nil {
			failCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = s.packageJobRepo.Fail(failCtx, job.JobID, workerID, err.Error(), time.Now().UTC())
			cancel()
		}
	}
	return len(jobs), nil
}

func (s *Service) processProductionPackageJob(ctx context.Context, workerID string, job *domain.ProductionPackageJob) error {
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	var processed atomic.Int64
	var failed atomic.Int64
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				_ = s.packageJobRepo.UpdateProgress(heartbeatCtx, job.JobID, workerID, int(processed.Load()), int(failed.Load()))
			}
		}
	}()
	var request ProductionPackageJobRequest
	if err := json.Unmarshal(job.RequestPayload, &request); err != nil {
		return fmt.Errorf("decode job request: %w", err)
	}
	manifest, appErr := s.buildExcelPackageManifest(ctx, request.Rows, MaxExcelPackageRows,
		productionPackageJobMaxFiles, productionPackageJobMaxBytes, request.FormatFilter)
	if appErr != nil {
		return fmt.Errorf("build package manifest: %s", appErr.Message)
	}
	if len(manifest.Items) == 0 {
		result, err := json.Marshal(ProductionPackageJobResult{Manifest: manifest})
		if err != nil {
			return err
		}
		cancelHeartbeat()
		return s.packageJobRepo.FailWithResult(
			ctx, job.JobID, workerID, productionPackageNoFilesMessage(manifest), result,
			manifest.FailureCount, time.Now().UTC(),
		)
	}
	processed.Store(int64(len(request.Rows)))
	failed.Store(int64(manifest.FailureCount))
	_ = s.packageJobRepo.UpdateProgress(ctx, job.JobID, workerID, len(request.Rows), manifest.FailureCount)

	zipFile, err := os.CreateTemp("", "production-package-*.zip")
	if err != nil {
		return err
	}
	zipPath := zipFile.Name()
	defer func() { _ = os.Remove(zipPath) }()
	if err := writeProductionPackageZIP(ctx, zipFile, manifest); err != nil {
		_ = zipFile.Close()
		return err
	}
	if err := zipFile.Close(); err != nil {
		return err
	}
	filename := "生产打包-" + time.Now().Format("20060102-150405") + ".zip"
	objectKey := path.Join("production-packages", time.Now().UTC().Format("2006/01/02"), job.JobID, filename)
	if err := uploadProductionPackageFile(ctx, s.packageStore, objectKey, zipPath); err != nil {
		return fmt.Errorf("upload package zip: %w", err)
	}
	result, err := json.Marshal(ProductionPackageJobResult{ObjectKey: objectKey, Filename: filename, Manifest: manifest})
	if err != nil {
		return err
	}
	cancelHeartbeat()
	return s.packageJobRepo.Complete(ctx, job.JobID, workerID, result, manifest.FailureCount, time.Now().UTC())
}

func productionPackageNoFilesMessage(manifest *ExcelPackageManifest) string {
	failures := 0
	if manifest != nil {
		failures = manifest.FailureCount
	}
	if failures <= 0 {
		return "未找到可打包的最终成品，请查看异常明细。"
	}
	return fmt.Sprintf("未找到可打包的最终成品：%d 行均未匹配所选格式，请查看逐行异常明细。", failures)
}

func uploadProductionPackageFile(ctx context.Context, store ProductionPackageObjectStore, objectKey, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	plan, err := store.CreateUploadPlan(ctx, objectKey, info.Size(), "application/zip")
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	if plan.Mode != "multipart" {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, plan.UploadURL, file)
		if err != nil {
			return err
		}
		req.ContentLength = info.Size()
		req.Header.Set("Content-Type", plan.RequiredContentType)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return fmt.Errorf("single-part upload status=%d body=%s", resp.StatusCode, string(raw))
		}
		return nil
	}

	uploadCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	parts := make([]baseservice.OSSCompletePart, len(plan.Parts))
	jobs := make(chan int)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				part := plan.Parts[index]
				offset := int64(index) * plan.PartSize
				length := plan.PartSize
				if remaining := info.Size() - offset; remaining < length {
					length = remaining
				}
				section := io.NewSectionReader(file, offset, length)
				req, err := http.NewRequestWithContext(uploadCtx, http.MethodPut, part.UploadURL, section)
				if err == nil {
					req.ContentLength = length
					req.Header.Set("Content-Type", plan.RequiredContentType)
					var resp *http.Response
					resp, err = client.Do(req)
					if err == nil {
						if resp.StatusCode < 200 || resp.StatusCode >= 300 {
							raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
							err = fmt.Errorf("part %d upload status=%d body=%s", part.PartNumber, resp.StatusCode, string(raw))
						} else {
							etag := strings.TrimSpace(resp.Header.Get("ETag"))
							if etag == "" {
								err = fmt.Errorf("part %d upload returned empty ETag", part.PartNumber)
							} else {
								parts[index] = baseservice.OSSCompletePart{PartNumber: part.PartNumber, ETag: etag}
							}
						}
						_ = resp.Body.Close()
					}
				}
				if err != nil {
					select {
					case errCh <- err:
						cancel()
					default:
					}
					return
				}
			}
		}()
	}
	func() {
		defer close(jobs)
		for index := range plan.Parts {
			select {
			case <-uploadCtx.Done():
				return
			case jobs <- index:
			}
		}
	}()
	wg.Wait()
	select {
	case uploadErr := <-errCh:
		abortCtx, abortCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer abortCancel()
		_ = store.AbortMultipartUpload(abortCtx, objectKey, plan.UploadID)
		return uploadErr
	default:
	}
	if err := store.CompleteMultipartUpload(ctx, objectKey, plan.UploadID, parts); err != nil {
		abortCtx, abortCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer abortCancel()
		_ = store.AbortMultipartUpload(abortCtx, objectKey, plan.UploadID)
		return err
	}
	return nil
}

func writeProductionPackageZIP(ctx context.Context, output io.Writer, manifest *ExcelPackageManifest) error {
	zw := zip.NewWriter(output)
	client := &http.Client{Timeout: 30 * time.Minute}
	addresses := map[string]string{}
	registry := map[string]int{}
	failures := append([]ExcelPackageFailure{}, manifest.Failures...)
	successfulItems := make([]ExcelPackageItem, 0, len(manifest.Items))
	for _, item := range manifest.Items {
		if err := ctx.Err(); err != nil {
			return err
		}
		orderFolder := sanitizeBatchFilename(item.OrderNo)
		if strings.Contains(item.Address, "*") {
			orderFolder += "（隐私号）"
		}
		body, err := downloadProductionPackageItem(ctx, client, item.DownloadURL)
		if err != nil {
			failures = append(failures, ExcelPackageFailure{RowNumber: item.RowNumber, OrderNo: item.OrderNo, SKUCode: item.SKUCode, SKUName: item.SKUName, Reason: "download_failed", Message: err.Error()})
			continue
		}
		cached, err := os.CreateTemp("", "production-package-item-*")
		if err != nil {
			_ = body.Close()
			failures = append(failures, ExcelPackageFailure{RowNumber: item.RowNumber, OrderNo: item.OrderNo, SKUCode: item.SKUCode, SKUName: item.SKUName, Reason: "download_failed", Message: err.Error()})
			continue
		}
		cachedPath := cached.Name()
		_, copyErr := io.Copy(cached, body)
		_ = body.Close()
		_ = cached.Close()
		if copyErr != nil {
			_ = os.Remove(cachedPath)
			failures = append(failures, ExcelPackageFailure{RowNumber: item.RowNumber, OrderNo: item.OrderNo, SKUCode: item.SKUCode, SKUName: item.SKUName, Reason: "download_failed", Message: copyErr.Error()})
			continue
		}
		for copyNo := 1; copyNo <= item.Quantity; copyNo++ {
			filename := item.Filename
			if item.Quantity > 1 {
				ext := filepath.Ext(filename)
				filename = strings.TrimSuffix(filename, ext) + fmt.Sprintf("-%d", copyNo) + ext
			}
			entryPath := path.Join(orderFolder, sanitizeBatchFilename(filename))
			if strings.TrimSpace(item.PackageFolder) != "" {
				entryPath = path.Join(orderFolder, sanitizeBatchFilename(item.PackageFolder), sanitizeBatchFilename(filename))
			}
			entryPath = ensureUniqueBatchFilename(entryPath, registry)
			entry, err := zw.Create(entryPath)
			if err != nil {
				_ = os.Remove(cachedPath)
				return err
			}
			source, err := os.Open(cachedPath)
			if err != nil {
				_ = os.Remove(cachedPath)
				return err
			}
			_, writeErr := io.Copy(entry, source)
			_ = source.Close()
			if writeErr != nil {
				_ = os.Remove(cachedPath)
				return writeErr
			}
		}
		_ = os.Remove(cachedPath)
		if _, exists := addresses[orderFolder]; !exists {
			addresses[orderFolder] = item.Address
		}
		successfulItems = append(successfulItems, item)
	}
	addressFolders := make([]string, 0, len(addresses))
	for folder := range addresses {
		addressFolders = append(addressFolders, folder)
	}
	sort.Strings(addressFolders)
	for _, folder := range addressFolders {
		address := addresses[folder]
		entry, err := zw.Create(path.Join(folder, "地址.txt"))
		if err != nil {
			return err
		}
		if _, err := io.WriteString(entry, strings.TrimSpace(address)+"\r\n"); err != nil {
			return err
		}
	}
	if len(failures) > 0 {
		entry, err := zw.Create("打包失败清单.txt")
		if err != nil {
			return err
		}
		for _, failure := range failures {
			_, _ = fmt.Fprintf(entry, "行%d\t订单%s\tSKU %s\t%s\t%s\r\n", failure.RowNumber, failure.OrderNo, failure.SKUCode, failure.Reason, failure.Message)
		}
	}
	manifest.Items = successfulItems
	manifest.Failures = failures
	manifest.FailureCount = len(failures)
	manifest.TotalFiles = 0
	manifest.TotalSize = 0
	for _, item := range successfulItems {
		manifest.TotalFiles += item.Quantity
		manifest.TotalSize += item.FileSize * int64(item.Quantity)
	}
	return zw.Close()
}

func downloadProductionPackageItem(ctx context.Context, client *http.Client, urlValue string) (io.ReadCloser, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlValue, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp.Body, nil
		}
		if resp != nil {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("download status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
	}
	return nil, lastErr
}
