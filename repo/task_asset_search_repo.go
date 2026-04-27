package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type TaskAssetSearchRow struct {
	Asset           *domain.TaskAsset
	Task            *domain.Task
	AssetNo         string
	DesignCreatedBy int64
	DesignCreatedAt time.Time
	DesignUpdatedAt time.Time
	OwnerTeamCode   string
}

type TaskAssetSearchRepo interface {
	Search(ctx context.Context, query domain.AssetSearchQuery) ([]*TaskAssetSearchRow, int64, error)
	GetCurrentByAssetID(ctx context.Context, assetID int64) (*TaskAssetSearchRow, error)
	ListVersionsByAssetID(ctx context.Context, assetID int64) ([]*TaskAssetSearchRow, error)
	GetVersion(ctx context.Context, assetID, versionID int64) (*TaskAssetSearchRow, error)
}
