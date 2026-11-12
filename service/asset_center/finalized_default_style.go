package asset_center

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type currentDefaultStyleSyncRepository interface {
	ListCurrentDefaultStyleAssetsByIDs(context.Context, []int64) ([]repo.ProductionPackageAsset, error)
}

type finalizedSyncV2Repository interface {
	ListFinalizedSyncV2(context.Context) (finals, waived, defaults []repo.ProductionPackageAsset, err error)
}

func (s *Service) FinalizedSyncManifestV2(ctx context.Context) (*FinalizedSyncManifest, *domain.AppError) {
	if s == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "finalized sync v2 service is not configured", nil)
	}
	repository, ok := s.finalizedSyncRepo.(finalizedSyncV2Repository)
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "finalized sync v2 repository is not configured", nil)
	}
	finals, waived, defaults, err := repository.ListFinalizedSyncV2(ctx)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("list finalized sync v2 manifest: %v", err), nil)
	}
	manifest, err := buildFinalizedSyncManifestV2(finals, waived, defaults, time.Now().UTC())
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("build finalized sync v2 manifest: %v", err), nil)
	}
	return manifest, nil
}

func buildFinalizedSyncManifestV2(finals, waived, defaults []repo.ProductionPackageAsset, now time.Time) (*FinalizedSyncManifest, error) {
	manifest, err := buildFinalizedSyncManifest(finals, now)
	if err != nil {
		return nil, err
	}
	manifest.SchemaVersion = 2
	for _, row := range waived {
		manifest.Groups = append(manifest.Groups, FinalizedSyncGroup{
			GroupID: row.GroupID, RevisionID: row.RevisionID, RevisionMode: row.RevisionMode, FinalizedAt: row.RevisionFinalizedAt.UTC(),
			TaskID: row.TaskID, TaskNo: row.TaskNo, ScopeKind: row.ScopeKind, SKUCode: row.SKUCode, ProductName: row.SKUName,
			Items: []FinalizedSyncItem{}, FinalOutputWaived: true,
		})
	}
	sort.Slice(manifest.Groups, func(i, j int) bool {
		a, b := manifest.Groups[i], manifest.Groups[j]
		if !a.FinalizedAt.Equal(b.FinalizedAt) {
			return a.FinalizedAt.After(b.FinalizedAt)
		}
		return a.GroupID > b.GroupID
	})
	manifest.GroupCount = len(manifest.Groups)
	styles := make([]FinalizedSyncDefaultStyle, 0, len(defaults))
	for _, row := range defaults {
		item := finalizedSyncItem(row, strings.TrimPrefix(strings.ToLower(filepath.Ext(row.FileName)), "."))
		styles = append(styles, FinalizedSyncDefaultStyle{
			GroupID: row.GroupID, RevisionID: row.RevisionID, TaskID: row.TaskID, TaskNo: row.TaskNo,
			ScopeKind: row.ScopeKind, SKUCode: row.SKUCode, ProductName: row.SKUName, TaskAssetID: item.TaskAssetID,
			FileName: item.FileName, OriginalFilename: item.OriginalFilename, Format: item.Format, MimeType: item.MimeType,
			FileSize: item.FileSize, StorageKey: item.StorageKey, WholeHash: item.WholeHash, CreatedAt: item.AssetUpdatedAt,
		})
		manifest.ObjectCount++
		manifest.TotalObjectBytes += row.FileSize
	}
	sort.Slice(styles, func(i, j int) bool { return styles[i].GroupID < styles[j].GroupID })
	manifest.DefaultStyles = &styles
	content := struct {
		SchemaVersion int                         `json:"schema_version"`
		Groups        []FinalizedSyncGroup        `json:"groups"`
		DefaultStyles []FinalizedSyncDefaultStyle `json:"default_styles"`
	}{manifest.SchemaVersion, manifest.Groups, styles}
	raw, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	manifest.ManifestID = hex.EncodeToString(digest[:])
	return manifest, nil
}
