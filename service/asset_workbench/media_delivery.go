package assetworkbench

import (
	"context"
	"strconv"
	"workflow/domain"
)

type publicationMediaPreviewer interface {
	GetAuthorizedPublicationPreview(context.Context, domain.ResourceGroupPublicationSnapshot, int64, bool) (*domain.AssetDownloadInfo, *domain.AppError)
}

func withClientMaterialMediaReference(ctx context.Context, material *domain.AssetWorkbenchClientMaterial, purpose, rendition string) context.Context {
	if material == nil {
		return ctx
	}
	options := domain.AssetMediaOptionsFromContext(ctx)
	if options.AccessReference == nil {
		options.AccessReference = &domain.AssetMediaAccessReference{ResourceKind: "client_material", ResourceID: strconv.FormatInt(material.ID, 10), Purpose: purpose, Rendition: rendition, ExpectedSourceVersion: options.ExpectedVersion}
	}
	return domain.WithAssetMediaOptions(ctx, options)
}

func (s *Service) ClientMaterialMediaInfo(ctx context.Context, actor domain.RequestActor, id int64, purpose, rendition string, itemIDs ...int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	if appErr := s.requireRepo(); appErr != nil {
		return nil, appErr
	}
	permission := domain.PermissionAssetDownload
	if purpose == "preview" {
		permission = domain.PermissionAssetView
	}
	if !domain.ActorHasPermission(actor, permission) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "client material access denied", nil)
	}
	material, appErr := s.resolveDownloadableClientMaterial(ctx, actor, id)
	if appErr != nil {
		return nil, appErr
	}
	return s.clientMaterialMediaInfo(ctx, material, purpose, rendition, itemIDs...)
}

func (s *Service) clientMaterialMediaInfo(ctx context.Context, material *domain.AssetWorkbenchClientMaterial, purpose, rendition string, itemIDs ...int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	ctx = withClientMaterialMediaReference(ctx, material, purpose, rendition)
	normalizeClientMaterialRow(material)
	if material.SourceType == "task_resource_group" {
		resolver, ok := s.repo.(resourceGroupPublicationResolver)
		if !ok || material.ResourceGroupID == nil || material.FinalizedRevisionID == nil || material.CoverRevisionItemID == nil {
			return nil, domain.ErrNotFound
		}
		publication, err := resolver.ResolveResourceGroupPublication(ctx, *material.ResourceGroupID, *material.FinalizedRevisionID, *material.CoverRevisionItemID)
		if err != nil || publication == nil || publication.GroupID != *material.ResourceGroupID || publication.FinalizedRevisionID != *material.FinalizedRevisionID {
			return nil, domain.ErrNotFound
		}
		var requested int64
		if len(itemIDs) > 0 {
			requested = itemIDs[0]
		}
		for _, file := range publication.Files {
			if (requested > 0 && file.TaskAssetID != requested) || (requested == 0 && file.RevisionItemID != publication.CoverRevisionItemID) {
				continue
			}
			if purpose == "preview" {
				p, ok := s.systemPreviews.(publicationMediaPreviewer)
				if !ok {
					return &domain.AssetDownloadInfo{State: "unsupported", AccessHint: "preview_unsupported", Rendition: rendition}, nil
				}
				return p.GetAuthorizedPublicationPreview(ctx, *publication, file.TaskAssetID, rendition == "thumbnail")
			}
			if s.oss == nil || !s.oss.Enabled() {
				return nil, domain.ErrNotFound
			}
			signed := s.oss.PresignDownloadURLWithFilename(file.StorageKey, file.FileName)
			if signed == nil {
				return nil, domain.ErrNotFound
			}
			size := int64(0)
			if file.FileSize != nil {
				size = *file.FileSize
			}
			version := domain.TaskAssetSourceVersion(file.TaskAssetID)
			return &domain.AssetDownloadInfo{DownloadMode: domain.AssetDownloadModeDirect, DownloadURL: &signed.DownloadURL, ExpiresAt: &signed.ExpiresAt,
				ObjectKey: file.StorageKey, SourceVersion: version, ContentID: domain.AssetMediaIdentity(version, "original", domain.AssetMediaRecipe),
				Rendition: "original", State: "ready", Filename: file.FileName, MimeType: file.MimeType, FileSize: size}, nil
		}
		return nil, domain.ErrNotFound
	}
	if domain.NormalizeAssetResourceSource(material.SourceType) == domain.AssetResourceSourceExternal {
		provider, ok := s.systemAssets.(ExternalAssetDownloader)
		if !ok {
			return nil, domain.ErrNotFound
		}
		if purpose == "preview" {
			return provider.PreviewExternal(ctx, material.AssetID, rendition)
		}
		return provider.DownloadExternal(ctx, material.AssetID)
	}
	if purpose == "preview" {
		if rendition == "thumbnail" {
			if p, ok := s.systemPreviews.(SystemAssetThumbnailer); ok {
				return p.GetAssetThumbnailInfoByID(ctx, material.AssetID)
			}
		}
		return s.systemPreviews.GetAssetPreviewInfoByID(ctx, material.AssetID)
	}
	info, appErr := s.systemDownloadSnapshot(ctx, material.AssetID)
	if info != nil && info.SourceVersion == "" {
		info.SourceVersion = "legacy-asset:" + strconv.FormatInt(material.AssetID, 10)
	}
	return info, appErr
}
