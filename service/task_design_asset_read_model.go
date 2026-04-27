package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func (s *taskService) loadTaskDesignAssetReadModel(ctx context.Context, task *domain.Task) ([]*domain.DesignAsset, []*domain.DesignAssetVersion, *domain.AppError) {
	return loadTaskDesignAssetReadModel(ctx, s.taskRepo, s.designAssetRepo, s.taskAssetRepo, task)
}

func loadTaskDesignAssetReadModel(
	ctx context.Context,
	taskRepo repo.TaskRepo,
	designAssetRepo repo.DesignAssetRepo,
	taskAssetRepo repo.TaskAssetRepo,
	task *domain.Task,
) ([]*domain.DesignAsset, []*domain.DesignAssetVersion, *domain.AppError) {
	if task == nil || designAssetRepo == nil || taskAssetRepo == nil {
		return []*domain.DesignAsset{}, []*domain.DesignAssetVersion{}, nil
	}

	designAssets, err := designAssetRepo.ListByTaskID(ctx, task.ID)
	if err != nil {
		return nil, nil, infraError("list design assets for task read model", err)
	}
	records, err := taskAssetRepo.ListByTaskID(ctx, task.ID)
	if err != nil {
		return nil, nil, infraError("list task assets for task design read model", err)
	}

	assetCenterView := &taskAssetCenterService{
		taskRepo:        taskRepo,
		designAssetRepo: designAssetRepo,
		taskAssetRepo:   taskAssetRepo,
	}
	if designAssets == nil || len(designAssets) == 0 {
		fallbackAssets, fallbackVersions := buildTaskLevelFallbackDesignAssetReadModel(task, records, assetCenterView)
		return fallbackAssets, fallbackVersions, nil
	}
	designAssets = filterDesignAssetsWithCurrentVersion(designAssets)
	if len(designAssets) == 0 {
		return []*domain.DesignAsset{}, []*domain.DesignAssetVersion{}, nil
	}
	versionsByAssetID := make(map[int64][]*domain.DesignAssetVersion, len(designAssets))
	for _, record := range records {
		version := buildTaskAssetVersionForReadModel(record)
		if version == nil {
			continue
		}
		versionsByAssetID[version.AssetID] = append(versionsByAssetID[version.AssetID], version)
	}

	assetVersions := make([]*domain.DesignAssetVersion, 0, len(records))
	for _, asset := range designAssets {
		if asset == nil {
			continue
		}
		versions := versionsByAssetID[asset.ID]
		sort.SliceStable(versions, func(i, j int) bool {
			li := versions[i]
			lj := versions[j]
			switch {
			case li == nil:
				return false
			case lj == nil:
				return true
			}
			if li.VersionNo == lj.VersionNo {
				return li.ID < lj.ID
			}
			return li.VersionNo < lj.VersionNo
		})
		for _, version := range versions {
			if version == nil {
				continue
			}
			assetCenterView.applyDesignAssetVersionDerivedFields(task, asset, version)
		}
		assetCenterView.applyDesignAssetVersionRoles(task, asset, versions)
		assetVersions = append(assetVersions, versions...)
	}
	if designAssets == nil {
		designAssets = []*domain.DesignAsset{}
	}
	return designAssets, assetVersions, nil
}

func filterDesignAssetsWithCurrentVersion(assets []*domain.DesignAsset) []*domain.DesignAsset {
	if len(assets) == 0 {
		return assets
	}
	out := assets[:0]
	for _, asset := range assets {
		if asset == nil || asset.CurrentVersionID == nil || *asset.CurrentVersionID == 0 {
			continue
		}
		out = append(out, asset)
	}
	return out
}

func buildTaskLevelFallbackDesignAssetReadModel(task *domain.Task, records []*domain.TaskAsset, view *taskAssetCenterService) ([]*domain.DesignAsset, []*domain.DesignAssetVersion) {
	if len(records) == 0 {
		return []*domain.DesignAsset{}, []*domain.DesignAssetVersion{}
	}

	orderedAssetIDs := make([]int64, 0)
	assetsByID := make(map[int64]*domain.DesignAsset)
	versionsByAssetID := make(map[int64][]*domain.DesignAssetVersion)

	for _, record := range records {
		version := buildTaskAssetVersionForReadModel(record)
		if version == nil {
			continue
		}
		assetID := version.AssetID
		if _, exists := assetsByID[assetID]; !exists {
			orderedAssetIDs = append(orderedAssetIDs, assetID)
			asset := &domain.DesignAsset{
				ID:           assetID,
				TaskID:       version.TaskID,
				AssetNo:      buildFallbackAssetNo(assetID, len(orderedAssetIDs)),
				ScopeSKUCode: version.ScopeSKUCode,
				AssetType:    version.AssetType,
				CreatedBy:    version.UploadedBy,
				CreatedAt:    fallbackTaskAssetTimestamp(record),
				UpdatedAt:    fallbackTaskAssetTimestamp(record),
			}
			assetsByID[assetID] = asset
		}
		versionsByAssetID[assetID] = append(versionsByAssetID[assetID], version)
	}

	if len(orderedAssetIDs) == 0 {
		return []*domain.DesignAsset{}, []*domain.DesignAssetVersion{}
	}

	designAssets := make([]*domain.DesignAsset, 0, len(orderedAssetIDs))
	assetVersions := make([]*domain.DesignAssetVersion, 0)
	for _, assetID := range orderedAssetIDs {
		asset := assetsByID[assetID]
		if asset == nil {
			continue
		}
		versions := versionsByAssetID[assetID]
		sort.SliceStable(versions, func(i, j int) bool {
			li := versions[i]
			lj := versions[j]
			switch {
			case li == nil:
				return false
			case lj == nil:
				return true
			}
			if li.VersionNo == lj.VersionNo {
				return li.ID < lj.ID
			}
			return li.VersionNo < lj.VersionNo
		})
		for _, version := range versions {
			if version == nil {
				continue
			}
			view.applyDesignAssetVersionDerivedFields(task, asset, version)
		}
		view.applyDesignAssetVersionRoles(task, asset, versions)
		designAssets = append(designAssets, asset)
		assetVersions = append(assetVersions, versions...)
	}
	return designAssets, assetVersions
}

func buildTaskAssetVersionForReadModel(record *domain.TaskAsset) *domain.DesignAssetVersion {
	version := domain.BuildDesignAssetVersion(record)
	if version != nil {
		return version
	}
	if record == nil || record.AssetID == nil || *record.AssetID <= 0 {
		return nil
	}

	assetVersionNo := 0
	if record.AssetVersionNo != nil {
		assetVersionNo = *record.AssetVersionNo
	}
	if assetVersionNo <= 0 {
		assetVersionNo = record.VersionNo
	}
	if assetVersionNo <= 0 {
		return nil
	}

	recordCopy := *record
	recordCopy.AssetVersionNo = &assetVersionNo
	return domain.BuildDesignAssetVersion(&recordCopy)
}

func buildFallbackAssetNo(assetID int64, sequence int) string {
	if assetID > 0 {
		return fmt.Sprintf("AST-%04d", assetID)
	}
	return fmt.Sprintf("AST-FALLBACK-%04d", sequence)
}

func fallbackTaskAssetTimestamp(record *domain.TaskAsset) time.Time {
	if record != nil {
		if record.UploadedAt != nil && !record.UploadedAt.IsZero() {
			return record.UploadedAt.UTC()
		}
		if !record.CreatedAt.IsZero() {
			return record.CreatedAt.UTC()
		}
	}
	return time.Now().UTC()
}
