package service

import (
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

func (s *taskAssetCenterService) resolveSourceDerivedPreviewInfo(ctx context.Context, sourceAsset *domain.DesignAsset) (*domain.AssetDownloadInfo, *domain.AppError) {
	if sourceAsset == nil || !sourceAsset.AssetType.IsSource() {
		return nil, nil
	}
	for _, assetType := range []domain.TaskAssetType{domain.TaskAssetTypePreview, domain.TaskAssetTypeDesignThumb} {
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
			if appErr := validateAssetVersionObjectAvailable(hydrated.CurrentVersion); appErr != nil {
				continue
			}
			return buildAssetPreviewInfoWithOSS(hydrated.CurrentVersion, s.uploadClient, s.ossDirectService), nil
		}
	}
	return nil, nil
}

func (s *taskAssetCenterService) scheduleDerivedPreviewGeneration(taskID, sourceAssetID, completedBy int64, sourceVersion *domain.DesignAssetVersion) {
	if sourceVersion == nil || !sourceVersion.IsSourceFile {
		return
	}
	if isOSSIMGDirectPreviewSupportedSourceVersion(sourceVersion) {
		return
	}
	if s.ossDirectService == nil || !s.ossDirectService.Enabled() {
		log.Printf("source_preview_derive_skipped task_id=%d source_asset_id=%d reason=oss_direct_disabled", taskID, sourceAssetID)
		return
	}
	runAsync := s.runAsyncFn
	if runAsync == nil {
		runAsync = func(fn func()) { go fn() }
	}
	runAsync(func() {
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
		if err := s.ensureDerivedPreviewAssets(ctx, taskID, sourceAssetID, completedBy); err != nil {
			log.Printf("source_preview_derive_failed task_id=%d source_asset_id=%d error=%v", taskID, sourceAssetID, err)
		}
	})
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
	task, appErr := s.requireTask(ctx, taskID)
	if appErr != nil {
		return appErr
	}
	sourceAsset, appErr := s.requireDesignAsset(ctx, taskID, sourceAssetID)
	if appErr != nil {
		return appErr
	}
	sourceAsset, appErr = s.loadAssetResource(ctx, sourceAsset)
	if appErr != nil {
		return appErr
	}
	if !sourceAsset.AssetType.IsSource() {
		return nil
	}
	if sourceAsset.CurrentVersion == nil {
		return nil
	}
	if shouldSkipDerivedPreviewGeneration(sourceAsset.CurrentVersion) {
		return nil
	}
	for _, spec := range sourceDerivedPreviewSpecs {
		if err := s.ensureSingleDerivedPreviewAsset(ctx, task, sourceAsset, spec, completedBy); err != nil {
			return err
		}
	}
	log.Printf("source_preview_derive_done task_id=%d source_asset_id=%d", taskID, sourceAssetID)
	return nil
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
	filter := repo.DesignAssetListFilter{
		TaskID:        &task.ID,
		SourceAssetID: &sourceAssetID,
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
		if targetAsset == nil {
			targetAsset = hydrated
		}
		if hydrated != nil && hydrated.CurrentVersion != nil && strings.TrimSpace(hydrated.CurrentVersion.StorageKey) != "" {
			if validateAssetVersionObjectAvailable(hydrated.CurrentVersion) == nil && !isLegacyPlaceholderDerivedPreview(hydrated.CurrentVersion) {
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

	return s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		asset := targetAsset
		if asset == nil {
			assetNo, err := s.designAssetRepo.NextAssetNo(ctx, tx, task.ID)
			if err != nil {
				return err
			}
			asset = &domain.DesignAsset{
				TaskID:        task.ID,
				AssetNo:       assetNo,
				SourceAssetID: &sourceAssetID,
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
		timelineVersionNo, err := s.taskAssetRepo.NextVersionNo(ctx, tx, task.ID)
		if err != nil {
			return err
		}
		assetVersionNo, err := s.taskAssetRepo.NextAssetVersionNo(ctx, tx, asset.ID)
		if err != nil {
			return err
		}
		taskRef := strings.TrimSpace(task.TaskNo)
		if taskRef == "" {
			taskRef = fmt.Sprintf("TASK-%d", task.ID)
		}
		objectKey := s.ossDirectService.BuildObjectKey(taskRef, asset.AssetNo, assetVersionNo, spec.AssetType, spec.Filename)
		if err := s.ossDirectService.UploadObject(ctx, objectKey, spec.MimeType, content); err != nil {
			return err
		}
		storageRefID := uuid.NewString()
		uploadStatus := string(domain.DesignAssetUploadStatusUploaded)
		previewStatus := string(domain.DesignAssetPreviewStatusNotApplicable)
		taskAsset := &domain.TaskAsset{
			TaskID:          task.ID,
			AssetID:         &asset.ID,
			ScopeSKUCode:    optionalStringPtr(strings.TrimSpace(sourceAsset.ScopeSKUCode)),
			AssetType:       spec.AssetType,
			VersionNo:       timelineVersionNo,
			AssetVersionNo:  &assetVersionNo,
			UploadMode:      optionalStringPtr(string(domain.DesignAssetUploadModeSmall)),
			UploadRequestID: nil,
			StorageRefID:    &storageRefID,
			FileName:        spec.Filename,
			OriginalName:    &spec.Filename,
			RemoteFileID:    nil,
			MimeType:        &spec.MimeType,
			FileSize:        &contentSize,
			StorageKey:      &objectKey,
			WholeHash:       &contentHashHex,
			UploadStatus:    &uploadStatus,
			PreviewStatus:   &previewStatus,
			UploadedBy:      completedBy,
			UploadedAt:      &now,
			Remark:          "async-derived-preview:webp",
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
	if _, err := io.Copy(file, reader); err != nil {
		_ = file.Close()
		cleanup()
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
