package asset_center

import (
	"context"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
)

type Service struct {
	searchRepo repo.TaskAssetSearchRepo
	presigner  DownloadPresigner
	urlBuilder BrowserURLBuilder
}

type DownloadPresigner interface {
	Enabled() bool
	PresignDownloadURL(objectKey string) *baseservice.OSSDirectDownloadInfo
}

type BrowserURLBuilder interface {
	BuildBrowserFileURL(storageKey string) *string
}

func NewService(searchRepo repo.TaskAssetSearchRepo, presigner DownloadPresigner, urlBuilder BrowserURLBuilder) *Service {
	return &Service{searchRepo: searchRepo, presigner: presigner, urlBuilder: urlBuilder}
}

func (s *Service) Search(ctx context.Context, query domain.AssetSearchQuery) (*SearchResult, *domain.AppError) {
	query = query.Normalized()
	rows, total, err := s.searchRepo.Search(ctx, query)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	items := make([]*AssetDetail, 0, len(rows))
	for _, row := range rows {
		items = append(items, buildAssetDetail(row, nil))
	}
	return &SearchResult{Items: items, Total: total, Page: query.Page, Size: query.Size}, nil
}
