package service

import (
	"context"

	"workflow/domain"
)

type referenceFileRefEnricher interface {
	EnrichAll([]domain.ReferenceFileRef) []domain.ReferenceFileRef
}

func FilterTaskLevelReferenceFileRefs(refs []domain.ReferenceFileRef, flatRefs []*domain.ReferenceFileRefFlat) []domain.ReferenceFileRef {
	scopedRefIDs := requirementScopedReferenceRefIDs(flatRefs)
	if len(scopedRefIDs) == 0 {
		if refs == nil {
			return []domain.ReferenceFileRef{}
		}
		return refs
	}
	out := make([]domain.ReferenceFileRef, 0, len(refs))
	for _, ref := range refs {
		if _, scoped := scopedRefIDs[ref.CanonicalID()]; scoped {
			continue
		}
		out = append(out, ref)
	}
	if out == nil {
		return []domain.ReferenceFileRef{}
	}
	return out
}

func requirementScopedReferenceRefIDs(flatRefs []*domain.ReferenceFileRefFlat) map[string]struct{} {
	out := make(map[string]struct{})
	for _, flat := range flatRefs {
		if flat == nil || flat.RetouchRequirementID == nil || *flat.RetouchRequirementID <= 0 {
			continue
		}
		if flat.RefID == "" {
			continue
		}
		out[flat.RefID] = struct{}{}
	}
	return out
}

func FilterTaskLevelDesignAssetReadModel(
	designAssets []*domain.DesignAsset,
	assetVersions []*domain.DesignAssetVersion,
) ([]*domain.DesignAsset, []*domain.DesignAssetVersion) {
	if len(assetVersions) == 0 {
		if designAssets == nil {
			return []*domain.DesignAsset{}, []*domain.DesignAssetVersion{}
		}
		return designAssets, assetVersions
	}
	filteredVersions := make([]*domain.DesignAssetVersion, 0, len(assetVersions))
	keptAssetIDs := make(map[int64]struct{})
	for _, version := range assetVersions {
		if version == nil || isRequirementScopedReadModelAsset(version.RetouchRequirementID, version.AssetType) {
			continue
		}
		filteredVersions = append(filteredVersions, version)
		if version.AssetID > 0 {
			keptAssetIDs[version.AssetID] = struct{}{}
		}
	}
	filteredAssets := make([]*domain.DesignAsset, 0, len(designAssets))
	for _, asset := range designAssets {
		if asset == nil {
			continue
		}
		if isRequirementScopedReadModelAsset(asset.RetouchRequirementID, asset.AssetType) {
			continue
		}
		if len(keptAssetIDs) > 0 {
			if _, ok := keptAssetIDs[asset.ID]; !ok {
				continue
			}
		}
		filteredAssets = append(filteredAssets, asset)
	}
	if filteredAssets == nil {
		filteredAssets = []*domain.DesignAsset{}
	}
	if filteredVersions == nil {
		filteredVersions = []*domain.DesignAssetVersion{}
	}
	return filteredAssets, filteredVersions
}

func isRequirementScopedReadModelAsset(retouchRequirementID *int64, assetType domain.TaskAssetType) bool {
	if retouchRequirementID == nil || *retouchRequirementID <= 0 {
		return false
	}
	normalized := domain.NormalizeTaskAssetType(assetType)
	return normalized.IsReference() || normalized.IsSource()
}

func EnrichRetouchRequirementsReadModel(
	_ context.Context,
	requirements []domain.TaskRetouchRequirement,
	flatRefs []*domain.ReferenceFileRefFlat,
	designAssets []*domain.DesignAsset,
	enricher referenceFileRefEnricher,
) []domain.TaskRetouchRequirement {
	if len(requirements) == 0 {
		return []domain.TaskRetouchRequirement{}
	}
	refsByRequirement := groupRequirementReferenceFileRefs(flatRefs)
	sourcesByRequirement := groupRequirementSourceAssets(designAssets)
	out := make([]domain.TaskRetouchRequirement, 0, len(requirements))
	for _, item := range requirements {
		row := item
		refs := refsByRequirement[row.ID]
		if enricher != nil {
			refs = enricher.EnrichAll(refs)
		}
		if refs == nil {
			refs = []domain.ReferenceFileRef{}
		}
		sources := sourcesByRequirement[row.ID]
		if sources == nil {
			sources = []*domain.DesignAsset{}
		}
		row.ReferenceFileRefs = refs
		row.SourceAssets = sources
		out = append(out, row)
	}
	return out
}

func groupRequirementReferenceFileRefs(flatRefs []*domain.ReferenceFileRefFlat) map[int64][]domain.ReferenceFileRef {
	out := make(map[int64][]domain.ReferenceFileRef)
	for _, flat := range flatRefs {
		if flat == nil || flat.RetouchRequirementID == nil || *flat.RetouchRequirementID <= 0 || flat.RefID == "" {
			continue
		}
		reqID := *flat.RetouchRequirementID
		out[reqID] = append(out[reqID], domain.ReferenceFileRef{
			AssetID: flat.RefID,
			RefID:   flat.RefID,
		})
	}
	for reqID, refs := range out {
		out[reqID] = domain.NormalizeReferenceFileRefs(refs)
	}
	return out
}

func groupRequirementSourceAssets(designAssets []*domain.DesignAsset) map[int64][]*domain.DesignAsset {
	out := make(map[int64][]*domain.DesignAsset)
	for _, asset := range designAssets {
		if asset == nil || asset.RetouchRequirementID == nil || *asset.RetouchRequirementID <= 0 {
			continue
		}
		if !asset.AssetType.IsSource() {
			continue
		}
		if asset.CurrentVersionID == nil || *asset.CurrentVersionID == 0 {
			continue
		}
		reqID := *asset.RetouchRequirementID
		out[reqID] = append(out[reqID], asset)
	}
	return out
}

func BuildTaskLevelDetailReferenceFileRefs(detail *domain.TaskDetail, flatRefs []*domain.ReferenceFileRefFlat) []domain.ReferenceFileRef {
	refs := buildDetailReferenceFileRefsFromDetail(detail, flatRefs)
	return FilterTaskLevelReferenceFileRefs(refs, flatRefs)
}

func buildDetailReferenceFileRefsFromDetail(detail *domain.TaskDetail, flatRefs []*domain.ReferenceFileRefFlat) []domain.ReferenceFileRef {
	if detail != nil {
		if refs := domain.ParseReferenceFileRefsJSON(detail.ReferenceFileRefsJSON); len(refs) > 0 {
			return refs
		}
		if refs := domain.ParseReferenceFileRefsJSON(detail.ReferenceImagesJSON); len(refs) > 0 {
			return refs
		}
	}
	if len(flatRefs) == 0 {
		return nil
	}
	refs := make([]domain.ReferenceFileRef, 0, len(flatRefs))
	for _, flat := range flatRefs {
		if flat == nil || flat.RefID == "" {
			continue
		}
		refs = append(refs, domain.ReferenceFileRef{
			AssetID: flat.RefID,
			RefID:   flat.RefID,
		})
	}
	return domain.NormalizeReferenceFileRefs(refs)
}
