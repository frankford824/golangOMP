package assetworkbench

import (
	"context"
	"sort"
	"strings"
	"time"

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

type DriveFolderBrowseResult struct {
	Path      string                              `json:"path"`
	Folders   []*domain.AssetWorkbenchDriveFolder `json:"folders"`
	Files     []*domain.AssetWorkbenchDriveFile   `json:"files"`
	Total     int64                               `json:"total"`
	Page      int                                 `json:"page"`
	Size      int                                 `json:"size"`
	Truncated bool                                `json:"truncated"`
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

func (s *Service) ListDriveFiles(ctx context.Context, actor domain.RequestActor, directoryID *int64, unassigned bool, orderNo string, keyword string, ownerKeyword string, createdFrom *time.Time, createdTo *time.Time, sortBy string, sortDir string, page, pageSize int) (*DriveFilesResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	page, pageSize = normalizeDrivePage(page, pageSize)
	filter := repo.AssetWorkbenchDriveFilter{
		OwnerUserID:       s.driveOwnerFilter(actor),
		UploadDirectoryID: directoryID,
		Unassigned:        unassigned,
		OrderNo:           orderNo,
		Keyword:           strings.TrimSpace(keyword),
		OwnerKeyword:      strings.TrimSpace(ownerKeyword),
		CreatedFrom:       createdFrom,
		CreatedTo:         createdTo,
		SortBy:            strings.TrimSpace(sortBy),
		SortDir:           strings.TrimSpace(sortDir),
		Page:              page,
		PageSize:          pageSize,
	}
	var items []*domain.AssetWorkbenchDriveFile
	var total int64
	var err error
	if filter.Keyword != "" {
		items, total, err = s.repo.DriveSearchFiles(ctx, filter)
	} else {
		items, total, err = s.repo.DriveListFiles(ctx, filter)
	}
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to list drive files.", err.Error())
	}
	return &DriveFilesResult{Items: items, Total: total, Page: page, Size: pageSize}, nil
}

func (s *Service) BrowseDriveFolder(ctx context.Context, actor domain.RequestActor, directoryID *int64, unassigned bool, folderPath string, page, pageSize int) (*DriveFolderBrowseResult, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	page, pageSize = normalizeDrivePage(page, pageSize)
	path := normalizeDriveFolderPath(folderPath)
	filter := repo.AssetWorkbenchDriveFilter{
		OwnerUserID:       s.driveOwnerFilter(actor),
		UploadDirectoryID: directoryID,
		Unassigned:        unassigned,
		PageSize:          200,
	}
	const maxScannedFiles = 10000
	all := []*domain.AssetWorkbenchDriveFile{}
	total := int64(0)
	truncated := false
	for scanPage := 1; ; scanPage++ {
		filter.Page = scanPage
		items, count, err := s.repo.DriveListFiles(ctx, filter)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to browse drive folder.", err.Error())
		}
		total = count
		all = append(all, items...)
		if len(all) >= int(total) || len(items) == 0 {
			break
		}
		if len(all) >= maxScannedFiles {
			truncated = true
			break
		}
	}
	folderByPath := map[string]*domain.AssetWorkbenchDriveFolder{}
	directFiles := []*domain.AssetWorkbenchDriveFile{}
	for _, file := range all {
		relativePath := driveFileVirtualPath(file)
		remainder, ok := driveFolderRemainder(relativePath, path)
		if !ok || remainder == "" {
			continue
		}
		if idx := strings.Index(remainder, "/"); idx >= 0 {
			childName := strings.TrimSpace(remainder[:idx])
			if childName == "" {
				continue
			}
			childPath := childName
			if path != "" {
				childPath = path + "/" + childName
			}
			folder := folderByPath[childPath]
			if folder == nil {
				folder = &domain.AssetWorkbenchDriveFolder{Name: childName, Path: childPath}
				folderByPath[childPath] = folder
			}
			folder.FileCount++
			childRemainder := strings.Trim(remainder[idx+1:], "/")
			if childRemainder != "" && !strings.Contains(childRemainder, "/") {
				folder.DirectFileCount++
			}
			if file.CreatedAt.After(folder.LatestAt) {
				folder.LatestAt = file.CreatedAt
			}
			continue
		}
		directFiles = append(directFiles, file)
	}
	folders := make([]*domain.AssetWorkbenchDriveFolder, 0, len(folderByPath))
	for _, folder := range folderByPath {
		folders = append(folders, folder)
	}
	sort.Slice(folders, func(i, j int) bool {
		if folders[i].Name == folders[j].Name {
			return folders[i].Path < folders[j].Path
		}
		return folders[i].Name < folders[j].Name
	})
	start := (page - 1) * pageSize
	if start > len(directFiles) {
		start = len(directFiles)
	}
	end := start + pageSize
	if end > len(directFiles) {
		end = len(directFiles)
	}
	return &DriveFolderBrowseResult{
		Path:      path,
		Folders:   folders,
		Files:     directFiles[start:end],
		Total:     int64(len(directFiles)),
		Page:      page,
		Size:      pageSize,
		Truncated: truncated || total > int64(len(all)),
	}, nil
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

func normalizeDriveFolderPath(raw string) string {
	parts := []string{}
	for _, part := range strings.Split(strings.ReplaceAll(raw, "\\", "/"), "/") {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "/")
}

func driveFileVirtualPath(file *domain.AssetWorkbenchDriveFile) string {
	if file == nil {
		return ""
	}
	return normalizeDriveFolderPath(firstNonEmpty(file.RelativePath, file.DisplayName, file.OriginalFilename, "file"))
}

func driveFolderRemainder(relativePath string, folderPath string) (string, bool) {
	relativePath = normalizeDriveFolderPath(relativePath)
	folderPath = normalizeDriveFolderPath(folderPath)
	if relativePath == "" {
		return "", false
	}
	if folderPath == "" {
		return relativePath, true
	}
	if relativePath == folderPath {
		return "", false
	}
	prefix := folderPath + "/"
	if !strings.HasPrefix(relativePath, prefix) {
		return "", false
	}
	return strings.TrimPrefix(relativePath, prefix), true
}
