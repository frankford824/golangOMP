package service

import (
	"context"
	"fmt"
	"strings"

	"workflow/domain"
)

func normalizeRequestedUploadAssetType(assetType domain.TaskAssetType, mode domain.DesignAssetUploadMode) (domain.TaskAssetType, *domain.AppError) {
	normalized := domain.NormalizeTaskAssetType(assetType)
	if normalized != "" {
		return normalized, nil
	}
	if mode == domain.DesignAssetUploadModeSmall {
		return domain.TaskAssetTypeReference, nil
	}
	return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "asset_type is required", nil)
}

func (s *taskAssetCenterService) hydrateDesignAssetReadModel(ctx context.Context, task *domain.Task, asset *domain.DesignAsset) error {
	if asset == nil {
		return nil
	}
	records, err := s.taskAssetRepo.ListByAssetID(ctx, asset.ID)
	if err != nil {
		return err
	}
	versions, err := s.buildLegacyDesignAssetVersions(task, asset, records)
	if err != nil {
		return err
	}
	enrichDesignAssetVersionUploaderNames(ctx, s.userDisplayNameResolver, versions)
	s.applyDesignAssetVersionRoles(task, asset, versions)
	return nil
}

func (s *taskAssetCenterService) buildLegacyDesignAssetVersions(
	task *domain.Task,
	asset *domain.DesignAsset,
	records []*domain.TaskAsset,
) ([]*domain.DesignAssetVersion, error) {
	versions := make([]*domain.DesignAssetVersion, 0, len(records))
	for _, record := range records {
		if isWorkflowMigrationSourceAlias(record) {
			if asset.CurrentVersionID != nil && *asset.CurrentVersionID == record.ID {
				return nil, fmt.Errorf(
					"workflow migration source alias %d cannot be design asset %d current version",
					record.ID, asset.ID,
				)
			}
			continue
		}
		if version := domain.BuildDesignAssetVersion(record); version != nil {
			s.applyDesignAssetVersionDerivedFields(task, asset, version)
			versions = append(versions, version)
		}
	}
	return versions, nil
}

// isWorkflowMigrationSourceAlias identifies the synthetic source identity used
// only by immutable V8 resource-group revisions when a legacy delivery had to
// stand in for a missing source file. The row deliberately shares the legacy
// design_assets root so object lineage remains intact, but it must not enter
// that root's current/approved version projection.
func isWorkflowMigrationSourceAlias(record *domain.TaskAsset) bool {
	if record == nil {
		return false
	}
	return domain.NormalizeTaskAssetType(record.AssetType).IsSource() &&
		strings.TrimSpace(record.SourceModuleKey) == "migration" &&
		strings.HasPrefix(strings.TrimSpace(record.Remark), "v8-source-alias:")
}

func (s *taskAssetCenterService) applyDesignAssetVersionDerivedFields(task *domain.Task, asset *domain.DesignAsset, version *domain.DesignAssetVersion) {
	if version == nil {
		return
	}
	if task != nil {
		version.TaskNo = task.TaskNo
		version.ProductNameSnapshot = strings.TrimSpace(task.ProductNameSnapshot)
	}
	if asset != nil {
		version.AssetNo = asset.AssetNo
		version.SourceAssetID = asset.SourceAssetID
		if version.ScopeSKUCode == "" {
			version.ScopeSKUCode = strings.TrimSpace(asset.ScopeSKUCode)
		}
		version.AssetType = domain.NormalizeTaskAssetType(asset.AssetType)
	}

	version.IsSourceFile = version.AssetType.IsSource()
	version.IsDeliveryFile = version.AssetType.IsDelivery()
	version.IsPreviewFile = version.AssetType.IsPreview()
	version.IsDesignThumb = version.AssetType.IsDesignThumb()
	version.PreviewAvailable = designAssetPreviewAvailable(version)
	version.SourceAccessMode = domain.DesignAssetSourceAccessModeStandard
	version.AccessPolicy = domain.DesignAssetAccessPolicyReferenceDirect
	version.PreviewPublicAllowed = version.PreviewAvailable

	switch {
	case version.IsSourceFile:
		version.AccessPolicy = domain.DesignAssetAccessPolicySourceControlled
		version.PreviewAvailable = isOSSIMGDirectPreviewSupportedSourceVersion(version)
		version.PreviewPublicAllowed = version.PreviewAvailable
	case version.IsDeliveryFile:
		version.AccessPolicy = domain.DesignAssetAccessPolicyDeliveryFlow
	case version.IsPreviewFile:
		version.AccessPolicy = domain.DesignAssetAccessPolicyPreviewAssist
	case version.IsDesignThumb:
		version.AccessPolicy = domain.DesignAssetAccessPolicyPreviewAssist
	default:
		version.AccessPolicy = domain.DesignAssetAccessPolicyReferenceDirect
	}

	s.applyAccessURLs(version)
	version.AccessHint = buildDesignAssetAccessHint(version)
	version.Notes = buildDesignAssetNotes(version)
}

func (s *taskAssetCenterService) applyAccessURLs(version *domain.DesignAssetVersion) {
	if version == nil || version.StorageKey == "" {
		return
	}
	downloadURL := domain.BuildRelativeEscapedURLPath("/v1/assets/files", version.StorageKey)
	version.DownloadURL = &downloadURL
	version.PublicDownloadAllowed = true
}

func (s *taskAssetCenterService) applyDesignAssetVersionRoles(task *domain.Task, asset *domain.DesignAsset, versions []*domain.DesignAssetVersion) {
	if asset == nil {
		return
	}
	current := findCurrentDesignAssetVersion(asset.CurrentVersionID, versions)
	if current == nil && len(versions) > 0 {
		current = versions[len(versions)-1]
		asset.CurrentVersionID = &current.ID
	}
	approved := findApprovedDesignAssetVersion(task, versions)

	asset.CurrentVersion = current
	asset.ApprovedVersion = approved
	asset.AssetType = domain.NormalizeTaskAssetType(asset.AssetType)
	asset.ApprovedVersionID = designAssetVersionIDPtr(approved)

	for _, version := range versions {
		if version == nil {
			continue
		}
		version.ApprovedForFlow = approved != nil && version.ID == approved.ID
		version.CurrentVersionRole = buildCurrentVersionRole(current, approved, version)
	}
}

func findCurrentDesignAssetVersion(currentVersionID *int64, versions []*domain.DesignAssetVersion) *domain.DesignAssetVersion {
	if currentVersionID != nil {
		for _, version := range versions {
			if version != nil && version.ID == *currentVersionID {
				return version
			}
		}
	}
	return nil
}

func findApprovedDesignAssetVersion(task *domain.Task, versions []*domain.DesignAssetVersion) *domain.DesignAssetVersion {
	for i := len(versions) - 1; i >= 0; i-- {
		version := versions[i]
		if version != nil && version.IsDeliveryFile && version.FlowReviewStatus == domain.TaskAssetFlowReviewStatusApproved {
			return version
		}
	}
	if task == nil || task.TaskStatus != domain.TaskStatusCompleted {
		return nil
	}
	for i := len(versions) - 1; i >= 0; i-- {
		version := versions[i]
		if version != nil && version.IsDeliveryFile {
			return version
		}
	}
	return nil
}

func designAssetVersionIDPtr(version *domain.DesignAssetVersion) *int64 {
	if version == nil {
		return nil
	}
	id := version.ID
	return &id
}

func buildCurrentVersionRole(current, approved, version *domain.DesignAssetVersion) string {
	if version == nil {
		return ""
	}
	isCurrent := current != nil && current.ID == version.ID
	isApproved := approved != nil && approved.ID == version.ID

	switch {
	case isCurrent && isApproved:
		return "current_approved_version"
	case isApproved:
		return "approved_version"
	case isCurrent:
		return "current_version"
	default:
		return ""
	}
}

func designAssetPreviewAvailable(version *domain.DesignAssetVersion) bool {
	if version == nil {
		return false
	}
	if version.IsSourceFile {
		return isOSSIMGDirectPreviewSupportedSourceVersion(version)
	}
	if isPSDLikeAsset(version) {
		return false
	}
	if version.IsPreviewFile {
		return true
	}
	if version.IsDesignThumb {
		return true
	}
	mimeType := strings.ToLower(strings.TrimSpace(version.MimeType))
	if strings.HasPrefix(mimeType, "image/") {
		return true
	}
	ext := normalizePreviewFileExtension(version.OriginalFilename)
	switch ext {
	case ".jpg", ".png", ".webp", ".gif", ".bmp", ".tiff", ".heic", ".avif":
		return true
	default:
		return false
	}
}

func isPSDLikeAsset(version *domain.DesignAssetVersion) bool {
	if version == nil {
		return false
	}
	return isPSDLikeAssetFile(version.OriginalFilename, version.MimeType)
}

func buildDesignAssetAccessHint(version *domain.DesignAssetVersion) string {
	if version == nil {
		return ""
	}
	if version.IsSourceFile {
		return fmt.Sprintf("源文件属于任务 %s，文件编号 %s，第 %d 版。", version.TaskNo, version.AssetNo, version.VersionNo)
	}
	if version.IsDeliveryFile {
		return "成品图通过审核后，作为任务当前有效成品。"
	}
	if version.IsPreviewFile {
		return "该文件仅用于预览，不替代正式成品图。"
	}
	if version.IsDesignThumb {
		return "该缩略图仅用于页面预览。"
	}
	return "参考图用于说明任务需求。"
}

func buildDesignAssetNotes(version *domain.DesignAssetVersion) string {
	if version == nil {
		return ""
	}
	switch {
	case version.IsSourceFile:
		return "当前源文件由任务资源组统一管理。"
	case version.IsDeliveryFile:
		return "当前成品图由任务资源组统一管理。"
	case version.IsPreviewFile:
		return "预览文件不是正式业务文件。"
	case version.IsDesignThumb:
		return "缩略图只用于页面预览。"
	default:
		return "参考图用于说明任务需求。"
	}
}
