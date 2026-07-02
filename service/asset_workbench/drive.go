package assetworkbench

import (
	"context"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

// DriveFilesResult is a paginated list of leaf image rows in the netdisk view.
type DriveFilesResult struct {
	Items []*domain.AssetWorkbenchDriveFile `json:"items"`
	Total int64                             `json:"total"`
	Page  int                               `json:"page"`
	Size  int                               `json:"size"`
}

// driveOwnerFilter scopes non-admin actors to their own uploads. Managers,
// settlement roles and super admins get the full site-wide view (nil filter).
func (s *Service) driveOwnerFilter(actor domain.RequestActor) *int64 {
	if actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil
	}
	id := actor.ID
	return &id
}

func normalizeDrivePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 60
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func (s *Service) ListDriveDirectories(ctx context.Context, actor domain.RequestActor) ([]*domain.AssetWorkbenchDriveDirectory, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	items, err := s.repo.DriveListDirectories(ctx, repo.AssetWorkbenchDriveFilter{OwnerUserID: s.driveOwnerFilter(actor)})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list drive directories.", err.Error())
	}
	return items, nil
}

func (s *Service) ListDriveOrders(ctx context.Context, actor domain.RequestActor, directoryID *int64, unassigned bool) ([]*domain.AssetWorkbenchDriveOrder, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	items, err := s.repo.DriveListOrders(ctx, repo.AssetWorkbenchDriveFilter{
		OwnerUserID:       s.driveOwnerFilter(actor),
		UploadDirectoryID: directoryID,
		Unassigned:        unassigned,
	})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list drive orders.", err.Error())
	}
	return items, nil
}

func (s *Service) ListDriveFiles(ctx context.Context, actor domain.RequestActor, directoryID *int64, unassigned bool, orderNo string, page, pageSize int) (*DriveFilesResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	page, pageSize = normalizeDrivePage(page, pageSize)
	items, total, err := s.repo.DriveListFiles(ctx, repo.AssetWorkbenchDriveFilter{
		OwnerUserID:       s.driveOwnerFilter(actor),
		UploadDirectoryID: directoryID,
		Unassigned:        unassigned,
		OrderNo:           orderNo,
		Page:              page,
		PageSize:          pageSize,
	})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list drive files.", err.Error())
	}
	return &DriveFilesResult{Items: items, Total: total, Page: page, Size: pageSize}, nil
}

func (s *Service) SearchDriveFiles(ctx context.Context, actor domain.RequestActor, keyword string, page, pageSize int) (*DriveFilesResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		page, pageSize = normalizeDrivePage(page, pageSize)
		return &DriveFilesResult{Items: []*domain.AssetWorkbenchDriveFile{}, Total: 0, Page: page, Size: pageSize}, nil
	}
	page, pageSize = normalizeDrivePage(page, pageSize)
	items, total, err := s.repo.DriveSearchFiles(ctx, repo.AssetWorkbenchDriveFilter{
		OwnerUserID: s.driveOwnerFilter(actor),
		Keyword:     keyword,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to search drive files.", err.Error())
	}
	return &DriveFilesResult{Items: items, Total: total, Page: page, Size: pageSize}, nil
}

func (s *Service) LocateDriveFile(ctx context.Context, actor domain.RequestActor, fileID int64) (*domain.AssetWorkbenchDriveFile, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if fileID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_id is required.", nil)
	}
	file, err := s.repo.DriveLocateFile(ctx, repo.AssetWorkbenchDriveFilter{OwnerUserID: s.driveOwnerFilter(actor)}, fileID)
	if err != nil {
		return nil, mapRepoReadError(err, "File not found.", "Failed to locate drive file.")
	}
	return file, nil
}
