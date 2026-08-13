package repo

import (
	"context"
	"time"

	"workflow/domain"
)

// ProductionPackageQuery deliberately resolves only finalized resource-group
// revisions. It is separate from the broad asset-center search contract so a
// draft/current design version can never become a production-delivery input.
type ProductionPackageQuery struct {
	SKUCodes []string
	SKUNames []string
}

type ProductionPackageAsset struct {
	GroupID             int64
	RevisionID          int64
	RevisionMode        domain.TaskAssetGroupMode
	RevisionFinalizedAt time.Time
	RevisionItemID      int64
	SortOrder           int
	ItemName            string
	TaskAssetID         int64
	TaskID              int64
	TaskNo              string
	SKUCode             string
	SKUName             string
	ScopeKind           domain.TaskAssetGroupScopeKind
	FileName            string
	OriginalFilename    string
	MimeType            string
	FileSize            int64
	StorageKey          string
	WholeHash           string
	CreatedAt           time.Time
}

type ProductionPackageRepo interface {
	ListFinalizedAssets(ctx context.Context, query ProductionPackageQuery) ([]ProductionPackageAsset, error)
}

// FinalizedAssetSyncRepo exposes the same current-finalized authority used by
// production packaging without adding machine-sync concerns to the ordinary
// SKU/name lookup contract.
type FinalizedAssetSyncRepo interface {
	ListAllFinalizedAssets(ctx context.Context) ([]ProductionPackageAsset, error)
	ListFinalizedAssetsByIDs(ctx context.Context, taskAssetIDs []int64) ([]ProductionPackageAsset, error)
}

type ProductionPackageStore interface {
	ProductionPackageRepo
	FinalizedAssetSyncRepo
}
