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
	versions := make([]*domain.DesignAssetVersion, 0, len(records))
	for _, record := range records {
		if version := domain.BuildDesignAssetVersion(record); version != nil {
			s.applyDesignAssetVersionDerivedFields(task, asset, version)
			versions = append(versions, version)
		}
	}
	enrichDesignAssetVersionUploaderNames(ctx, s.userDisplayNameResolver, versions)
	s.applyDesignAssetVersionRoles(task, asset, versions)
	return nil
}

func (s *taskAssetCenterService) applyDesignAssetVersionDerivedFields(task *domain.Task, asset *domain.DesignAsset, version *domain.DesignAssetVersion) {
	if version == nil {
		return
	}
	if task != nil {
		version.TaskNo = task.TaskNo
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
	warehouseReady := findWarehouseReadyDesignAssetVersion(task, versions)

	asset.CurrentVersion = current
	asset.ApprovedVersion = approved
	asset.WarehouseReadyVersion = warehouseReady
	asset.AssetType = domain.NormalizeTaskAssetType(asset.AssetType)
	asset.ApprovedVersionID = designAssetVersionIDPtr(approved)
	asset.WarehouseReadyVersionID = designAssetVersionIDPtr(warehouseReady)

	for _, version := range versions {
		if version == nil {
			continue
		}
		version.ApprovedForFlow = approved != nil && version.ID == approved.ID
		version.WarehouseReady = warehouseReady != nil && version.ID == warehouseReady.ID
		version.CurrentVersionRole = buildCurrentVersionRole(current, approved, warehouseReady, version)
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
	if !taskHasApprovedDelivery(task) {
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

func findWarehouseReadyDesignAssetVersion(task *domain.Task, versions []*domain.DesignAssetVersion) *domain.DesignAssetVersion {
	if !taskHasApprovedDelivery(task) {
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

func taskHasApprovedDelivery(task *domain.Task) bool {
	if task == nil {
		return false
	}
	switch task.TaskStatus {
	case domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingClose, domain.TaskStatusCompleted:
		return true
	default:
		return false
	}
}

func designAssetVersionIDPtr(version *domain.DesignAssetVersion) *int64 {
	if version == nil {
		return nil
	}
	id := version.ID
	return &id
}

func buildCurrentVersionRole(current, approved, warehouseReady, version *domain.DesignAssetVersion) string {
	if version == nil {
		return ""
	}
	isCurrent := current != nil && current.ID == version.ID
	isApproved := approved != nil && approved.ID == version.ID
	isWarehouseReady := warehouseReady != nil && warehouseReady.ID == version.ID

	switch {
	case isCurrent && isWarehouseReady:
		return "current_warehouse_ready_version"
	case isCurrent && isApproved:
		return "current_approved_version"
	case isWarehouseReady:
		return "warehouse_ready_version"
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
		return fmt.Sprintf("Use task_no=%s asset_no=%s version_no=%d object_key=%s to fetch the OSS-backed source file.", version.TaskNo, version.AssetNo, version.VersionNo, version.StorageKey)
	}
	if version.IsDeliveryFile {
		return "Delivery assets are the business flow truth for audit and warehouse after approval."
	}
	if version.IsPreviewFile {
		return "Preview assets are auxiliary only and must not replace delivery assets in business flow."
	}
	if version.IsDesignThumb {
		return "Design thumb assets are lightweight preview derivatives for list/detail rendering."
	}
	return "Reference assets are task-scoped files for task creation, design reference, and business understanding only."
}

func buildDesignAssetNotes(version *domain.DesignAssetVersion) string {
	if version == nil {
		return ""
	}
	switch {
	case version.IsSourceFile:
		return "Source files remain OSS-backed business assets; no NAS-only path is required."
	case version.IsDeliveryFile:
		return "Warehouse and audit should consume the warehouse_ready_version or approved_version based on current task status."
	case version.IsPreviewFile:
		return "Preview artifacts are not the formal source of truth."
	case version.IsDesignThumb:
		return "Design thumb artifacts are backend-owned derivatives for preview rendering only."
	default:
		return "Reference assets never enter the warehouse_ready_version path."
	}
}
