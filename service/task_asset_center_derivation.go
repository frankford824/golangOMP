package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

type derivedPreviewSpec struct {
	AssetType domain.TaskAssetType
	Filename  string
	MimeType  string
	Width     int
	Height    int
	Quality   int
}

var sourceDerivedPreviewSpecs = []derivedPreviewSpec{
	{
		AssetType: domain.TaskAssetTypePreview,
		Filename:  "preview.webp",
		MimeType:  "image/webp",
		Width:     1600,
		Height:    1600,
		Quality:   82,
	},
	{
		AssetType: domain.TaskAssetTypeDesignThumb,
		Filename:  "design-thumb.webp",
		MimeType:  "image/webp",
		Width:     480,
		Height:    480,
		Quality:   78,
	},
}

func (s *taskAssetCenterService) resolveDerivedPreviewInfo(ctx context.Context, sourceAsset *domain.DesignAsset) (*domain.AssetDownloadInfo, *domain.AppError) {
	if s.strictMediaPreviews {
		return s.resolveDerivedRenditionInfo(ctx, sourceAsset, []domain.TaskAssetType{domain.TaskAssetTypePreview}, buildAssetPreviewInfoWithOSS)
	}
	return s.resolveDerivedRenditionInfo(ctx, sourceAsset,
		[]domain.TaskAssetType{domain.TaskAssetTypePreview, domain.TaskAssetTypeDesignThumb},
		buildAssetPreviewInfoWithOSS,
	)
}

func (s *taskAssetCenterService) resolveDerivedThumbnailInfo(ctx context.Context, sourceAsset *domain.DesignAsset) (*domain.AssetDownloadInfo, *domain.AppError) {
	if s.strictMediaPreviews {
		return s.resolveDerivedRenditionInfo(ctx, sourceAsset, []domain.TaskAssetType{domain.TaskAssetTypeDesignThumb}, buildAssetThumbnailInfoWithOSS)
	}
	return s.resolveDerivedRenditionInfo(ctx, sourceAsset,
		[]domain.TaskAssetType{domain.TaskAssetTypeDesignThumb, domain.TaskAssetTypePreview},
		buildAssetThumbnailInfoWithOSS,
	)
}

func (s *taskAssetCenterService) resolveDerivedRenditionInfo(
	ctx context.Context,
	sourceAsset *domain.DesignAsset,
	assetTypes []domain.TaskAssetType,
	build func(*domain.DesignAssetVersion, UploadServiceClient, *OSSDirectService) *domain.AssetDownloadInfo,
) (*domain.AssetDownloadInfo, *domain.AppError) {
	if sourceAsset == nil || sourceAsset.AssetType.IsPreview() || sourceAsset.AssetType.IsDesignThumb() {
		return nil, nil
	}
	for _, assetType := range assetTypes {
		sourceAssetID := sourceAsset.ID
		filter := repo.DesignAssetListFilter{
			TaskID:        &sourceAsset.TaskID,
			SourceAssetID: &sourceAssetID,
			AssetType:     &assetType,
		}
		derivedAssets, err := s.designAssetRepo.List(ctx, filter)
		if err != nil {
			return nil, infraError("list source-derived preview assets", err)
		}
		for _, candidate := range derivedAssets {
			hydrated, appErr := s.loadAssetResource(ctx, candidate)
			if appErr != nil {
				return nil, appErr
			}
			if hydrated == nil || hydrated.CurrentVersion == nil || !hydrated.CurrentVersion.PreviewAvailable {
				continue
			}
			if !derivedPreviewMatchesSourceVersion(hydrated.CurrentVersion, sourceAsset.CurrentVersion) {
				continue
			}
			if s.strictMediaPreviews && !isBoundedMediaRendition(hydrated.CurrentVersion) {
				continue
			}
			if appErr := validateAssetVersionObjectAvailable(hydrated.CurrentVersion); appErr != nil {
				continue
			}
			if s.strictMediaPreviews && !domain.AssetMediaOptionsFromContext(ctx).AuthorizeOnly && s.ossDirectService != nil && s.ossDirectService.Enabled() {
				stat, exists, err := s.ossDirectService.StatObject(ctx, hydrated.CurrentVersion.StorageKey)
				if err != nil {
					return nil, infraError("verify bounded preview object", err)
				}
				if !exists || stat == nil || hydrated.CurrentVersion.FileSize == nil || stat.ContentLength != *hydrated.CurrentVersion.FileSize {
					continue
				}
			}
			return build(hydrated.CurrentVersion, s.uploadClient, s.ossDirectService), nil
		}
	}
	return nil, nil
}

func derivedPreviewMatchesSourceVersion(derivedVersion, sourceVersion *domain.DesignAssetVersion) bool {
	return derivedVersion != nil &&
		sourceVersion != nil &&
		derivedVersion.SourceAssetVersionID != nil &&
		*derivedVersion.SourceAssetVersionID == sourceVersion.ID
}

func isBoundedMediaRendition(version *domain.DesignAssetVersion) bool {
	if version == nil || version.Remark != "async-derived-preview:webp:"+domain.AssetMediaRecipe || version.FileSize == nil || *version.FileSize <= 0 {
		return false
	}
	limit := int64(2 << 20)
	if version.IsDesignThumb {
		limit = 200 << 10
	}
	return *version.FileSize <= limit
}

func (s *taskAssetCenterService) scheduleDerivedPreviewGeneration(taskID, sourceAssetID, completedBy int64, sourceVersion *domain.DesignAssetVersion) {
	if sourceVersion == nil || !isDerivedPreviewGenerationCandidate(sourceVersion) {
		return
	}
	if s.mediaJobs != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, _, err := s.enqueueMediaPreview(ctx, taskID, sourceAssetID, sourceVersion, 0); err != nil {
			log.Printf("source_preview_enqueue_failed task_id=%d source_asset_id=%d error=%v", taskID, sourceAssetID, err)
		}
		return
	}
	if s.ossDirectService == nil || !s.ossDirectService.Enabled() {
		log.Printf("source_preview_derive_skipped task_id=%d source_asset_id=%d reason=oss_direct_disabled", taskID, sourceAssetID)
		return
	}
	inflightKey := fmt.Sprintf("%d:%d:%d:%s", taskID, sourceAssetID, sourceVersion.ID, strings.TrimSpace(sourceVersion.StorageKey))
	if !s.claimDerivedPreviewGeneration(inflightKey) {
		return
	}
	runAsync := s.runAsyncFn
	if runAsync == nil {
		runAsync = func(fn func()) { go fn() }
	}
	runAsync(func() {
		defer s.releaseDerivedPreviewGeneration(inflightKey)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if s.derivedPreviewGracePeriod > 0 {
			timer := time.NewTimer(s.derivedPreviewGracePeriod)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
		}
		if s.derivedPreviewSlots != nil {
			select {
			case s.derivedPreviewSlots <- struct{}{}:
				defer func() { <-s.derivedPreviewSlots }()
			case <-ctx.Done():
				return
			}
		}
		if err := s.ensureDerivedPreviewAssets(ctx, taskID, sourceAssetID, completedBy); err != nil {
			log.Printf("source_preview_derive_failed task_id=%d source_asset_id=%d error=%v", taskID, sourceAssetID, err)
		}
	})
}

func (s *taskAssetCenterService) claimDerivedPreviewGeneration(key string) bool {
	if s == nil || strings.TrimSpace(key) == "" {
		return false
	}
	s.derivedPreviewMu.Lock()
	defer s.derivedPreviewMu.Unlock()
	if s.derivedPreviewInflight == nil {
		s.derivedPreviewInflight = make(map[string]struct{})
	}
	if _, exists := s.derivedPreviewInflight[key]; exists {
		return false
	}
	s.derivedPreviewInflight[key] = struct{}{}
	return true
}

func (s *taskAssetCenterService) releaseDerivedPreviewGeneration(key string) {
	if s == nil {
		return
	}
	s.derivedPreviewMu.Lock()
	delete(s.derivedPreviewInflight, key)
	s.derivedPreviewMu.Unlock()
}

func (s *taskAssetCenterService) EnsureDerivedPreviewAssets(ctx context.Context, taskID, sourceAssetID, actorID int64) *domain.AppError {
	if err := s.ensureDerivedPreviewAssets(ctx, taskID, sourceAssetID, actorID); err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			return appErr
		}
		return infraError("ensure derived preview assets", err)
	}
	return nil
}

func (s *taskAssetCenterService) ensureDerivedPreviewAssets(ctx context.Context, taskID, sourceAssetID, completedBy int64) error {
	return s.ensureDerivedPreviewVersion(ctx, taskID, sourceAssetID, 0, completedBy)
}

type previewSourcePathContextKey struct{}
type pinnedDerivationContextKey struct{}
type previewIntermediateContextKey struct{}

func (s *taskAssetCenterService) ensureDerivedPreviewVersion(ctx context.Context, taskID, sourceAssetID, sourceVersionID, completedBy int64) error {
	ctx = context.WithValue(ctx, pinnedDerivationContextKey{}, sourceVersionID > 0)
	task, appErr := s.requireTask(ctx, taskID)
	if appErr != nil {
		return appErr
	}
	var sourceAsset *domain.DesignAsset
	if sourceVersionID > 0 {
		record, err := s.taskAssetRepo.GetByID(ctx, sourceVersionID)
		if err != nil {
			return err
		}
		if record == nil || record.TaskID != taskID || record.DeletedAt != nil || record.CleanedAt != nil || (record.AssetID != nil && *record.AssetID != sourceAssetID) || (record.AssetID == nil && sourceAssetID != 0) {
			return fmt.Errorf("source_version_changed")
		}
		sourceAsset = &domain.DesignAsset{ID: sourceAssetID, TaskID: taskID, AssetType: record.AssetType, CurrentVersionID: &sourceVersionID,
			CurrentVersion: s.buildBoundTaskAssetVersion(record, task), CreatedBy: record.UploadedBy}
	} else {
		sourceAsset, appErr = s.requireDesignAsset(ctx, taskID, sourceAssetID)
		if appErr != nil {
			return appErr
		}
		sourceAsset, appErr = s.loadAssetResource(ctx, sourceAsset)
		if appErr != nil {
			return appErr
		}
	}
	if !sourceAsset.AssetType.IsSource() {
		if sourceAsset.AssetType.IsPreview() || sourceAsset.AssetType.IsDesignThumb() {
			return nil
		}
	}
	if sourceAsset.CurrentVersion == nil {
		return nil
	}
	if shouldSkipDerivedPreviewGeneration(sourceAsset.CurrentVersion) {
		if pinned, _ := ctx.Value(pinnedDerivationContextKey{}).(bool); pinned {
			return fmt.Errorf("corrupt source file: truncated design source")
		}
		return nil
	}
	if appErr := validateAssetVersionObjectAvailable(sourceAsset.CurrentVersion); appErr != nil {
		return appErr
	}
	if s.ossDirectService == nil || !s.ossDirectService.Enabled() {
		return fmt.Errorf("OSS preview reader unavailable")
	}
	reader, err := s.ossDirectService.OpenObject(ctx, sourceAsset.CurrentVersion.StorageKey)
	if err != nil {
		return err
	}
	sourcePath, cleanup, err := writePreviewRendererSourceTempFile(reader, sourceAsset.CurrentVersion.OriginalFilename, sourceAsset.CurrentVersion.MimeType)
	_ = reader.Close()
	if err != nil {
		return err
	}
	defer cleanup()
	ctx = context.WithValue(ctx, previewSourcePathContextKey{}, sourcePath)
	var intermediate []byte
	ctx = context.WithValue(ctx, previewIntermediateContextKey{}, &intermediate)
	for _, spec := range sourceDerivedPreviewSpecs {
		if err := s.ensureSingleDerivedPreviewAsset(ctx, task, sourceAsset, spec, completedBy); err != nil {
			return err
		}
	}
	log.Printf("source_preview_derive_done task_id=%d source_asset_id=%d", taskID, sourceAssetID)
	return nil
}

func isDerivedPreviewGenerationCandidate(version *domain.DesignAssetVersion) bool {
	if version == nil {
		return false
	}
	if version.IsPreviewFile || version.IsDesignThumb {
		return false
	}
	if strings.TrimSpace(version.StorageKey) == "" {
		return false
	}
	if !isExternalRendererPreviewSupported(version.OriginalFilename, version.MimeType) {
		return false
	}
	return version.IsSourceFile || version.IsDeliveryFile || version.AssetType.IsReference()
}

func isExternalRendererPreviewSupported(filename, mimeType string) bool {
	ext := sourceAssetFormatExtension(filename, mimeType)
	switch ext {
	case ".jpg", ".png", ".bmp", ".gif", ".webp", ".tiff", ".heic", ".avif":
		return true
	case ".psd", ".psb", ".pdf", ".ai", ".eps", ".ps":
		return true
	default:
		return false
	}
}

func shouldSkipDerivedPreviewGeneration(sourceVersion *domain.DesignAssetVersion) bool {
	if sourceVersion == nil || sourceVersion.FileSize == nil {
		return false
	}
	if *sourceVersion.FileSize >= 1024 {
		return false
	}
	ext := normalizePreviewFileExtension(sourceVersion.OriginalFilename)
	switch ext {
	case ".psd", ".psb":
		return true
	default:
		return false
	}
}

func (s *taskAssetCenterService) ensureSingleDerivedPreviewAsset(
	ctx context.Context,
	task *domain.Task,
	sourceAsset *domain.DesignAsset,
	spec derivedPreviewSpec,
	completedBy int64,
) error {
	if task == nil || sourceAsset == nil || sourceAsset.CurrentVersion == nil {
		return nil
	}
	sourceAssetID := sourceAsset.ID
	var sourceAssetRef *int64
	if sourceAssetID > 0 {
		sourceAssetRef = &sourceAssetID
	}
	filter := repo.DesignAssetListFilter{
		TaskID:        &task.ID,
		SourceAssetID: sourceAssetRef,
		AssetType:     &spec.AssetType,
	}
	derivedAssets, err := s.designAssetRepo.List(ctx, filter)
	if err != nil {
		return fmt.Errorf("list derived assets: %w", err)
	}

	var targetAsset *domain.DesignAsset
	for _, asset := range derivedAssets {
		hydrated, appErr := s.loadAssetResource(ctx, asset)
		if appErr != nil {
			return fmt.Errorf("hydrate derived asset: %w", appErr)
		}
		// A historical rendition must never replace a different source version's
		// derived current pointer or schedule that newer rendition for cleanup.
		pinned, _ := ctx.Value(pinnedDerivationContextKey{}).(bool)
		if hydrated != nil && (!pinned || derivedPreviewMatchesSourceVersion(hydrated.CurrentVersion, sourceAsset.CurrentVersion)) && targetAsset == nil {
			targetAsset = hydrated
		}
		if hydrated != nil && hydrated.CurrentVersion != nil &&
			derivedPreviewMatchesSourceVersion(hydrated.CurrentVersion, sourceAsset.CurrentVersion) &&
			strings.TrimSpace(hydrated.CurrentVersion.StorageKey) != "" {
			pinned, _ := ctx.Value(pinnedDerivationContextKey{}).(bool)
			ready := validateAssetVersionObjectAvailable(hydrated.CurrentVersion) == nil && !isLegacyPlaceholderDerivedPreview(hydrated.CurrentVersion) && (!pinned || isBoundedMediaRendition(hydrated.CurrentVersion))
			if ready && pinned && s.ossDirectService != nil {
				stat, exists, e := s.ossDirectService.StatObject(ctx, hydrated.CurrentVersion.StorageKey)
				if e != nil {
					return fmt.Errorf("verify existing rendition: %w", e)
				}
				ready = exists && stat != nil && hydrated.CurrentVersion.FileSize != nil && stat.ContentLength == *hydrated.CurrentVersion.FileSize
			}
			if ready {
				return nil
			}
		}
	}

	content, err := s.renderDerivedPreviewContent(ctx, sourceAsset.CurrentVersion, spec)
	if err != nil {
		return fmt.Errorf("render derived preview: %w", err)
	}
	contentSize := int64(len(content))
	contentHash := sha256.Sum256(content)
	contentHashHex := hex.EncodeToString(contentHash[:])
	now := s.nowFn().UTC()

	// Network I/O must not hold task/asset locks. A unique immutable object key
	// also prevents a rolled-back or concurrent derivation overwriting a preview
	// that another transaction has already published.
	taskRef := sanitizeOSSObjectKeySegment(task.TaskNo, fmt.Sprintf("TASK-%d", task.ID))
	objectKey := fmt.Sprintf("tasks/%s/derived-previews/%d/%s/%s.webp", taskRef, sourceAsset.CurrentVersion.ID, spec.AssetType, uuid.NewString())
	if err := s.ossDirectService.UploadDerivedPreviewObject(ctx, objectKey, spec.MimeType, content); err != nil {
		return err
	}

	return s.runAssetTransaction(ctx, task.ID, "derived_preview", func(tx repo.Tx) error {
		// The foreground upload locks tasks -> design_assets -> task_assets.
		// NextAssetNo locks the task's asset range; taking it before the task
		// caused a cycle with the task_assets foreign-key check in production.
		lockedTask, err := s.getTaskForUpdate(ctx, tx, task.ID)
		if err != nil {
			return fmt.Errorf("lock task before derived preview: %w", err)
		}
		if lockedTask == nil {
			return domain.ErrNotFound
		}
		if guard, ok := ctx.Value(mediaLeaseGuardContextKey{}).(func(context.Context, repo.Tx) error); ok {
			if err := guard(ctx, tx); err != nil {
				return err
			}
		}
		frozenSource, err := s.getTaskAssetForUpdate(ctx, tx, sourceAsset.CurrentVersion.ID)
		if err != nil {
			return err
		}
		if frozenSource == nil || frozenSource.DeletedAt != nil || frozenSource.CleanedAt != nil || valueOrEmpty(frozenSource.StorageKey) != sourceAsset.CurrentVersion.StorageKey {
			return fmt.Errorf("source_version_changed")
		}
		asset := targetAsset
		if asset != nil {
			asset, err = s.getDesignAssetForUpdate(ctx, tx, asset.ID)
			if err != nil {
				return err
			}
			if asset == nil {
				return domain.ErrNotFound
			}
		}
		if asset == nil {
			assetNo, err := s.designAssetRepo.NextAssetNo(ctx, tx, task.ID)
			if err != nil {
				return err
			}
			asset = &domain.DesignAsset{
				TaskID:        task.ID,
				AssetNo:       assetNo,
				SourceAssetID: sourceAssetRef,
				ScopeSKUCode:  strings.TrimSpace(sourceAsset.ScopeSKUCode),
				AssetType:     spec.AssetType,
				CreatedBy:     completedBy,
			}
			assetID, err := s.designAssetRepo.Create(ctx, tx, asset)
			if err != nil {
				return err
			}
			asset.ID = assetID
		}
		previousCurrentVersionID := cloneInt64Ptr(asset.CurrentVersionID)
		timelineVersionNo, err := s.taskAssetRepo.NextVersionNo(ctx, tx, task.ID)
		if err != nil {
			return err
		}
		assetVersionNo, err := s.taskAssetRepo.NextAssetVersionNo(ctx, tx, asset.ID)
		if err != nil {
			return err
		}
		storageRefID := uuid.NewString()
		uploadStatus := string(domain.DesignAssetUploadStatusUploaded)
		previewStatus := string(domain.DesignAssetPreviewStatusNotApplicable)
		taskAsset := &domain.TaskAsset{
			TaskID:               task.ID,
			AssetID:              &asset.ID,
			ScopeSKUCode:         optionalStringPtr(strings.TrimSpace(sourceAsset.ScopeSKUCode)),
			AssetType:            spec.AssetType,
			VersionNo:            timelineVersionNo,
			AssetVersionNo:       &assetVersionNo,
			UploadMode:           optionalStringPtr(string(domain.DesignAssetUploadModeSmall)),
			UploadRequestID:      nil,
			StorageRefID:         &storageRefID,
			FileName:             spec.Filename,
			OriginalName:         &spec.Filename,
			RemoteFileID:         nil,
			MimeType:             &spec.MimeType,
			FileSize:             &contentSize,
			StorageKey:           &objectKey,
			WholeHash:            &contentHashHex,
			UploadStatus:         &uploadStatus,
			PreviewStatus:        &previewStatus,
			UploadedBy:           completedBy,
			UploadedAt:           &now,
			Remark:               "async-derived-preview:webp:" + domain.AssetMediaRecipe,
			FlowReviewStatus:     domain.TaskAssetFlowReviewStatusNotApplicable,
			SourceAssetVersionID: &sourceAsset.CurrentVersion.ID,
		}
		versionID, err := s.taskAssetRepo.Create(ctx, tx, taskAsset)
		if err != nil {
			return err
		}
		ref := &domain.AssetStorageRef{
			RefID:           storageRefID,
			AssetID:         &versionID,
			OwnerType:       domain.AssetOwnerTypeTaskAsset,
			OwnerID:         versionID,
			UploadRequestID: "",
			StorageAdapter:  domain.AssetStorageAdapterOSSUploadService,
			RefType:         domain.AssetStorageRefTypeTaskAssetObject,
			RefKey:          objectKey,
			FileName:        spec.Filename,
			MimeType:        spec.MimeType,
			FileSize:        &contentSize,
			IsPlaceholder:   false,
			ChecksumHint:    contentHashHex,
			Status:          domain.AssetStorageRefStatusRecorded,
			CreatedAt:       now,
		}
		if _, err := s.assetStorageRefRepo.Create(ctx, tx, ref); err != nil {
			return err
		}
		if err := s.designAssetRepo.UpdateCurrentVersionID(ctx, tx, asset.ID, &versionID); err != nil {
			return err
		}
		if previousCurrentVersionID != nil && *previousCurrentVersionID > 0 && *previousCurrentVersionID != versionID {
			if supersedeRepo, ok := s.taskAssetRepo.(taskAssetVersionSupersedeRepo); ok {
				cleanupAfter := now.Add(assetVersionReplacementRetention)
				if err := supersedeRepo.MarkAssetVersionSuperseded(ctx, tx, *previousCurrentVersionID, versionID, now, cleanupAfter); err != nil {
					return err
				}
			}
		}
		_, err = s.taskEventRepo.Append(ctx, tx, task.ID, domain.TaskEventAssetVersionCreated, &completedBy, map[string]interface{}{
			"asset_id":          asset.ID,
			"asset_type":        string(spec.AssetType),
			"source_asset_id":   sourceAssetID,
			"asset_version_id":  versionID,
			"asset_version_no":  assetVersionNo,
			"timeline_version":  timelineVersionNo,
			"storage_key":       objectKey,
			"upload_mode":       string(domain.DesignAssetUploadModeSmall),
			"mime_type":         spec.MimeType,
			"derived_async":     true,
			"derived_format":    "webp",
			"derivation_reason": "source_non_direct_preview",
		})
		return err
	})
}

func (s *taskAssetCenterService) renderDerivedPreviewContent(ctx context.Context, sourceVersion *domain.DesignAssetVersion, spec derivedPreviewSpec) ([]byte, error) {
	if sourceVersion == nil {
		return nil, fmt.Errorf("source version is required")
	}
	if s.ossDirectService == nil || !s.ossDirectService.Enabled() {
		return nil, fmt.Errorf("oss direct service is not enabled")
	}
	if s.previewRenderer == nil {
		return nil, fmt.Errorf("asset preview renderer is not configured")
	}
	sourceKey := strings.TrimSpace(sourceVersion.StorageKey)
	if sourceKey == "" {
		return nil, fmt.Errorf("source storage key is empty")
	}
	if inputPath, ok := ctx.Value(previewSourcePathContextKey{}).(string); ok && inputPath != "" {
		meta := AssetPreviewSourceMeta{Filename: sourceVersion.OriginalFilename, MimeType: sourceVersion.MimeType}
		intermediate, _ := ctx.Value(previewIntermediateContextKey{}).(*[]byte)
		if spec.Width <= 480 && intermediate != nil && len(*intermediate) > 0 {
			smallPath, cleanup, e := writePreviewRendererSourceTempFile(bytes.NewReader(*intermediate), "preview.webp", "image/webp")
			if e != nil {
				return nil, e
			}
			defer cleanup()
			inputPath = smallPath
			meta = AssetPreviewSourceMeta{Filename: "preview.webp", MimeType: "image/webp"}
		}
		body, err := s.previewRenderer.Render(ctx, inputPath, meta, AssetPreviewRenderSpec{MaxWidth: spec.Width, MaxHeight: spec.Height, Quality: spec.Quality})
		if err == nil && spec.Width == 1600 && intermediate != nil {
			*intermediate = body
		}
		return body, err
	}
	reader, err := s.ossDirectService.OpenObject(ctx, sourceKey)
	if err != nil {
		return nil, fmt.Errorf("open source object: %w", err)
	}
	defer reader.Close()

	inputPath, cleanup, err := writePreviewRendererSourceTempFile(reader, sourceVersion.OriginalFilename, sourceVersion.MimeType)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	return s.previewRenderer.Render(ctx, inputPath, AssetPreviewSourceMeta{
		Filename: sourceVersion.OriginalFilename,
		MimeType: sourceVersion.MimeType,
	}, AssetPreviewRenderSpec{
		MaxWidth:  spec.Width,
		MaxHeight: spec.Height,
		Quality:   spec.Quality,
	})
}

func writePreviewRendererSourceTempFile(reader io.Reader, filename, mimeType string) (string, func(), error) {
	ext := sourceAssetFormatExtension(filename, mimeType)
	if ext == "" {
		ext = strings.ToLower(strings.TrimSpace(filepath.Ext(filename)))
	}
	if ext == "" || strings.ContainsAny(ext, `/\`) {
		ext = ".bin"
	}
	file, err := os.CreateTemp("", "asset-preview-source-*"+ext)
	if err != nil {
		return "", func() {}, fmt.Errorf("create source temp file: %w", err)
	}
	path := file.Name()
	cleanup := func() {
		_ = os.Remove(path)
	}
	n, err := io.Copy(file, io.LimitReader(reader, AssetPreviewSourceMaxBytes+1))
	if err != nil || n > AssetPreviewSourceMaxBytes {
		_ = file.Close()
		cleanup()
		if n > AssetPreviewSourceMaxBytes {
			return "", func() {}, fmt.Errorf("preview_source_exceeds_limit")
		}
		return "", func() {}, fmt.Errorf("write source temp file: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close source temp file: %w", err)
	}
	return path, cleanup, nil
}

func isLegacyPlaceholderDerivedPreview(version *domain.DesignAssetVersion) bool {
	if version == nil {
		return false
	}
	if strings.TrimSpace(version.Remark) != "async-derived-preview" {
		return false
	}
	ext := normalizePreviewFileExtension(version.OriginalFilename)
	return ext == ".png"
}
