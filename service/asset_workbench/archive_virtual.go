package assetworkbench

import (
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"workflow/domain"
)

const (
	archiveVirtualMaxArchiveBytes      int64 = 2 << 30
	archiveVirtualMaxUncompressedBytes int64 = 5 << 30
	archiveVirtualMaxVisibleFileCount        = 20000
	archiveVirtualMaxDepth                   = 16
	archiveVirtualTempCacheTTL               = 10 * time.Minute
	archiveVirtualMaxCachedArchives          = 8
)

type ArchiveBrowseResult struct {
	FileID  int64                  `json:"file_id"`
	Path    string                 `json:"path"`
	Format  string                 `json:"format"`
	Folders []ArchiveVirtualFolder `json:"folders"`
	Files   []ArchiveVirtualFile   `json:"files"`
}

type ArchiveVirtualFolder struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	FileCount int64  `json:"file_count"`
}

type ArchiveVirtualFile struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	MimeType    string `json:"mime_type"`
	FileType    string `json:"file_type"`
	FileSize    int64  `json:"file_size"`
	PreviewURL  string `json:"preview_url,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
}

type ArchiveEntryStream struct {
	Body        io.ReadCloser
	Filename    string
	MimeType    string
	FileSize    int64
	TempPath    string
	ArchiveBody io.Closer
	CacheLease  io.Closer
}

type archiveTempCacheEntry struct {
	tmpPath   string
	ready     chan struct{}
	loading   bool
	refCount  int
	expiresAt time.Time
	lastUsed  time.Time
}

type archiveTempCacheLease struct {
	service *Service
	key     string
	path    string
	once    sync.Once
}

func (s *Service) BrowseArchiveFile(ctx context.Context, actor domain.RequestActor, fileID int64, folderPath string) (*ArchiveBrowseResult, *domain.AppError) {
	file, appErr := s.loadVisibleArchiveFile(ctx, actor, fileID)
	if appErr != nil {
		return nil, appErr
	}
	format := archiveFormatForFile(file)
	if format != "zip" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "该压缩包暂不支持在线打开，可下载后查看", map[string]interface{}{"archive_format": format})
	}
	lease, err := s.cachedArchiveObjectTemp(ctx, file)
	if err != nil {
		return nil, archiveErrorToAppError(err)
	}
	defer lease.Close()
	reader, err := zip.OpenReader(lease.Path())
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "压缩包无法打开，请下载后检查文件是否完整", err.Error())
	}
	defer reader.Close()
	return s.buildArchiveBrowseResult(file.ID, "zip", normalizeDriveFolderPath(folderPath), &reader.Reader)
}

func (s *Service) OpenArchiveEntry(ctx context.Context, actor domain.RequestActor, fileID int64, entryPath string) (*ArchiveEntryStream, *domain.AppError) {
	file, appErr := s.loadVisibleArchiveFile(ctx, actor, fileID)
	if appErr != nil {
		return nil, appErr
	}
	if archiveFormatForFile(file) != "zip" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "该压缩包暂不支持在线打开，可下载后查看", nil)
	}
	cleanEntryPath, skip, err := sanitizeArchiveEntryPath(entryPath, archiveVirtualMaxDepth)
	if err != nil || skip {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "压缩包路径无效", nil)
	}
	lease, err := s.cachedArchiveObjectTemp(ctx, file)
	if err != nil {
		return nil, archiveErrorToAppError(err)
	}
	reader, err := zip.OpenReader(lease.Path())
	if err != nil {
		_ = lease.Close()
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "压缩包无法打开，请下载后检查文件是否完整", err.Error())
	}
	for _, entry := range reader.File {
		clean, skip, err := sanitizeArchiveEntryPath(entry.Name, archiveVirtualMaxDepth)
		if err != nil || skip || clean != cleanEntryPath || entry.FileInfo().IsDir() {
			continue
		}
		if entry.UncompressedSize64 > uint64(archiveVirtualMaxUncompressedBytes) {
			_ = reader.Close()
			_ = lease.Close()
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "压缩包内文件过大，暂不支持在线打开", nil)
		}
		body, err := entry.Open()
		if err != nil {
			_ = reader.Close()
			_ = lease.Close()
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "压缩包内文件打开失败", err.Error())
		}
		return &ArchiveEntryStream{
			Body:        body,
			Filename:    path.Base(clean),
			MimeType:    archiveEntryMimeType(clean),
			FileSize:    int64(entry.UncompressedSize64),
			ArchiveBody: reader,
			CacheLease:  lease,
		}, nil
	}
	_ = reader.Close()
	_ = lease.Close()
	return nil, domain.NewAppError(domain.ErrCodeNotFound, "压缩包内未找到该文件", nil)
}

func (s *Service) loadVisibleArchiveFile(ctx context.Context, actor domain.RequestActor, fileID int64) (*domain.AssetWorkbenchSubmissionFile, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSubmitter, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "没有权限查看该文件", nil)
	}
	if s.oss == nil || !s.oss.Enabled() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "文件存储服务暂不可用", nil)
	}
	if fileID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "file_id is required.", nil)
	}
	file, err := s.repo.GetSubmissionFile(ctx, fileID)
	if err != nil {
		return nil, mapRepoReadError(err, "文件不存在", "文件读取失败")
	}
	if file.OwnerUserID != actor.ID && !actorHasAny(actor, domain.RoleAssetManager, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "没有权限查看该文件", nil)
	}
	if archiveFormatForFile(file) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "该文件不是压缩包", map[string]interface{}{"file_id": file.ID})
	}
	return file, nil
}

func (s *Service) buildArchiveBrowseResult(fileID int64, format string, folderPath string, reader *zip.Reader) (*ArchiveBrowseResult, *domain.AppError) {
	result := &ArchiveBrowseResult{FileID: fileID, Format: format, Path: normalizeDriveFolderPath(folderPath), Folders: []ArchiveVirtualFolder{}, Files: []ArchiveVirtualFile{}}
	folderByPath := map[string]*ArchiveVirtualFolder{}
	var visibleFileCount int
	var totalUncompressed int64
	for _, entry := range reader.File {
		if entry == nil || entry.FileInfo().IsDir() {
			continue
		}
		visibleFileCount++
		if visibleFileCount > archiveVirtualMaxVisibleFileCount {
			return nil, archiveErrorToAppError(permanentArchiveVirtualErrorf("压缩包内文件过多，暂不支持在线打开"))
		}
		if entry.UncompressedSize64 > uint64(archiveVirtualMaxUncompressedBytes) {
			return nil, archiveErrorToAppError(permanentArchiveVirtualErrorf("压缩包内容过大，暂不支持在线打开"))
		}
		entrySize := int64(entry.UncompressedSize64)
		if entrySize > 0 {
			if archiveVirtualMaxUncompressedBytes-totalUncompressed < entrySize {
				return nil, archiveErrorToAppError(permanentArchiveVirtualErrorf("压缩包内容过大，暂不支持在线打开"))
			}
			totalUncompressed += entrySize
		}
		clean, skip, err := sanitizeArchiveEntryPath(entry.Name, archiveVirtualMaxDepth)
		if err != nil {
			return nil, archiveErrorToAppError(err)
		}
		if skip {
			continue
		}
		remainder, ok := driveFolderRemainder(clean, result.Path)
		if !ok || remainder == "" {
			continue
		}
		if idx := strings.Index(remainder, "/"); idx >= 0 {
			childName := strings.TrimSpace(remainder[:idx])
			if childName == "" {
				continue
			}
			childPath := childName
			if result.Path != "" {
				childPath = result.Path + "/" + childName
			}
			folder := folderByPath[childPath]
			if folder == nil {
				folder = &ArchiveVirtualFolder{Name: childName, Path: childPath}
				folderByPath[childPath] = folder
			}
			folder.FileCount++
			continue
		}
		mimeType := archiveEntryMimeType(clean)
		previewURL := ""
		if archiveEntryPreviewable(clean, mimeType) {
			previewURL = archiveEntryURL(fileID, clean, true)
		}
		result.Files = append(result.Files, ArchiveVirtualFile{
			Name:        path.Base(clean),
			Path:        clean,
			MimeType:    mimeType,
			FileType:    inferFileType(clean, mimeType),
			FileSize:    entrySize,
			PreviewURL:  previewURL,
			DownloadURL: archiveEntryURL(fileID, clean, false),
		})
	}
	for _, folder := range folderByPath {
		result.Folders = append(result.Folders, *folder)
	}
	sort.Slice(result.Folders, func(i, j int) bool { return result.Folders[i].Name < result.Folders[j].Name })
	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].Name < result.Files[j].Name })
	return result, nil
}

func archiveEntryURL(fileID int64, entryPath string, preview bool) string {
	query := url.Values{}
	query.Set("path", entryPath)
	if preview {
		query.Set("disposition", "inline")
	}
	return fmt.Sprintf("/v1/asset-workbench/files/%d/archive/entry?%s", fileID, query.Encode())
}

func (s *Service) copyArchiveObjectToTemp(ctx context.Context, file *domain.AssetWorkbenchSubmissionFile) (string, error) {
	if file == nil || strings.TrimSpace(file.ObjectKey) == "" {
		return "", permanentArchiveVirtualErrorf("文件存储地址缺失")
	}
	if file.FileSize > archiveVirtualMaxArchiveBytes {
		return "", permanentArchiveVirtualErrorf("压缩包过大，暂不支持在线打开")
	}
	body, err := s.oss.OpenObject(ctx, file.ObjectKey)
	if err != nil {
		return "", err
	}
	defer body.Close()
	tmp, err := os.CreateTemp("", "asset-workbench-archive-*.zip")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	written, copyErr := io.Copy(tmp, io.LimitReader(body, archiveVirtualMaxArchiveBytes+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", closeErr
	}
	if written > archiveVirtualMaxArchiveBytes {
		_ = os.Remove(tmpPath)
		return "", permanentArchiveVirtualErrorf("压缩包过大，暂不支持在线打开")
	}
	return tmpPath, nil
}

func archiveTempCacheKey(file *domain.AssetWorkbenchSubmissionFile) string {
	if file == nil {
		return ""
	}
	return fmt.Sprintf("%d:%s:%d", file.ID, strings.TrimSpace(file.ObjectKey), file.FileSize)
}

func (s *Service) cachedArchiveObjectTemp(ctx context.Context, file *domain.AssetWorkbenchSubmissionFile) (*archiveTempCacheLease, error) {
	key := archiveTempCacheKey(file)
	if key == "" {
		return nil, permanentArchiveVirtualErrorf("文件存储地址缺失")
	}
	for {
		now := s.nowFn().UTC()
		s.archiveCacheMu.Lock()
		if s.archiveCache == nil {
			s.archiveCache = map[string]*archiveTempCacheEntry{}
		}
		s.cleanupArchiveTempCacheLocked(now)
		if entry := s.archiveCache[key]; entry != nil {
			if entry.loading {
				ready := entry.ready
				s.archiveCacheMu.Unlock()
				select {
				case <-ready:
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			if strings.TrimSpace(entry.tmpPath) != "" && (now.Before(entry.expiresAt) || entry.refCount > 0) {
				entry.refCount++
				entry.lastUsed = now
				path := entry.tmpPath
				s.archiveCacheMu.Unlock()
				return &archiveTempCacheLease{service: s, key: key, path: path}, nil
			}
			if entry.refCount == 0 {
				s.removeArchiveTempEntryLocked(key, entry)
			}
		}
		entry := &archiveTempCacheEntry{loading: true, ready: make(chan struct{}), lastUsed: now}
		s.archiveCache[key] = entry
		s.archiveCacheMu.Unlock()

		tmpPath, err := s.copyArchiveObjectToTemp(ctx, file)
		s.archiveCacheMu.Lock()
		current := s.archiveCache[key]
		if current == entry {
			entry.loading = false
			entry.lastUsed = s.nowFn().UTC()
			if err != nil {
				delete(s.archiveCache, key)
				close(entry.ready)
				s.archiveCacheMu.Unlock()
				return nil, err
			}
			entry.tmpPath = tmpPath
			entry.refCount = 1
			entry.expiresAt = entry.lastUsed.Add(archiveVirtualTempCacheTTL)
			close(entry.ready)
			s.cleanupArchiveTempCacheLocked(entry.lastUsed)
			s.archiveCacheMu.Unlock()
			return &archiveTempCacheLease{service: s, key: key, path: tmpPath}, nil
		}
		s.archiveCacheMu.Unlock()
		if err != nil {
			return nil, err
		}
		_ = os.Remove(tmpPath)
	}
}

func (s *Service) cleanupArchiveTempCacheLocked(now time.Time) {
	if len(s.archiveCache) == 0 {
		return
	}
	for key, entry := range s.archiveCache {
		if entry == nil {
			delete(s.archiveCache, key)
			continue
		}
		if entry.loading || entry.refCount > 0 {
			continue
		}
		if !entry.expiresAt.IsZero() && !now.Before(entry.expiresAt) {
			s.removeArchiveTempEntryLocked(key, entry)
		}
	}
	if len(s.archiveCache) <= archiveVirtualMaxCachedArchives {
		return
	}
	keys := make([]string, 0, len(s.archiveCache))
	for key, entry := range s.archiveCache {
		if entry != nil && !entry.loading && entry.refCount == 0 {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return s.archiveCache[keys[i]].lastUsed.Before(s.archiveCache[keys[j]].lastUsed)
	})
	for len(s.archiveCache) > archiveVirtualMaxCachedArchives && len(keys) > 0 {
		key := keys[0]
		keys = keys[1:]
		if entry := s.archiveCache[key]; entry != nil && !entry.loading && entry.refCount == 0 {
			s.removeArchiveTempEntryLocked(key, entry)
		}
	}
}

func (s *Service) removeArchiveTempEntryLocked(key string, entry *archiveTempCacheEntry) {
	if entry != nil && strings.TrimSpace(entry.tmpPath) != "" {
		_ = os.Remove(entry.tmpPath)
	}
	delete(s.archiveCache, key)
}

func (l *archiveTempCacheLease) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *archiveTempCacheLease) Close() error {
	if l == nil || l.service == nil {
		return nil
	}
	l.once.Do(func() {
		now := l.service.nowFn().UTC()
		l.service.archiveCacheMu.Lock()
		if entry := l.service.archiveCache[l.key]; entry != nil {
			if entry.refCount > 0 {
				entry.refCount--
			}
			entry.lastUsed = now
			entry.expiresAt = now.Add(archiveVirtualTempCacheTTL)
		}
		l.service.cleanupArchiveTempCacheLocked(now)
		l.service.archiveCacheMu.Unlock()
	})
	return nil
}

func archiveFormatForFile(file *domain.AssetWorkbenchSubmissionFile) string {
	if file == nil {
		return ""
	}
	ext := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(file.FileExt)), ".")
	if ext == "" {
		name := file.DisplayName
		if strings.TrimSpace(name) == "" {
			name = file.OriginalFilename
		}
		ext = strings.TrimPrefix(strings.ToLower(path.Ext(strings.ReplaceAll(name, "\\", "/"))), ".")
	}
	switch ext {
	case "zip", "rar", "7z":
		return ext
	default:
		return ""
	}
}

func sanitizeArchiveEntryPath(raw string, maxDepth int) (string, bool, error) {
	value := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	value = strings.Trim(value, "/")
	if value == "" {
		return "", true, nil
	}
	if strings.ContainsRune(value, 0) || path.IsAbs(value) {
		return "", false, permanentArchiveVirtualErrorf("压缩包路径无效")
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false, permanentArchiveVirtualErrorf("压缩包路径无效")
	}
	segments := strings.Split(clean, "/")
	if maxDepth > 0 && len(segments) > maxDepth {
		return "", false, permanentArchiveVirtualErrorf("压缩包层级过深，暂不支持在线打开")
	}
	for _, segment := range segments {
		name := strings.TrimSpace(segment)
		if name == "" || name == "." || name == ".." {
			return "", false, permanentArchiveVirtualErrorf("压缩包路径无效")
		}
		if shouldSkipArchiveSystemEntry(name) {
			return "", true, nil
		}
	}
	return clean, false, nil
}

func shouldSkipArchiveSystemEntry(segment string) bool {
	lower := strings.ToLower(strings.TrimSpace(segment))
	switch lower {
	case "__macosx", ".ds_store", "thumbs.db", "desktop.ini", "@eadir", "#recycle":
		return true
	default:
		return strings.HasPrefix(lower, "._")
	}
}

func archiveEntryMimeType(filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	if detected := mime.TypeByExtension(ext); detected != "" {
		return detected
	}
	switch strings.TrimPrefix(ext, ".") {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "bmp":
		return "image/bmp"
	case "svg":
		return "image/svg+xml"
	case "tif", "tiff":
		return "image/tiff"
	case "pdf":
		return "application/pdf"
	case "mp4":
		return "video/mp4"
	case "webm":
		return "video/webm"
	case "mov":
		return "video/quicktime"
	case "m4v":
		return "video/x-m4v"
	default:
		return "application/octet-stream"
	}
}

func archiveEntryPreviewable(filename, mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(mimeType, "image/") || strings.HasPrefix(mimeType, "video/") || mimeType == "application/pdf" {
		return true
	}
	switch strings.TrimPrefix(strings.ToLower(path.Ext(filename)), ".") {
	case "jpg", "jpeg", "png", "webp", "gif", "bmp", "svg", "tif", "tiff", "pdf", "mp4", "webm", "mov", "m4v":
		return true
	default:
		return false
	}
}

type archiveVirtualPermanentError struct {
	message string
}

func (e *archiveVirtualPermanentError) Error() string {
	return e.message
}

func permanentArchiveVirtualErrorf(format string, args ...interface{}) error {
	return &archiveVirtualPermanentError{message: fmt.Sprintf(format, args...)}
}

func isPermanentArchiveVirtualError(err error) bool {
	var permanent *archiveVirtualPermanentError
	return errors.As(err, &permanent)
}

func archiveErrorToAppError(err error) *domain.AppError {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewAppError(domain.ErrCodeNotFound, "文件不存在", nil)
	}
	if isPermanentArchiveVirtualError(err) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil)
	}
	return domain.NewAppError(domain.ErrCodeInternalError, "压缩包打开失败", err.Error())
}

func (s *ArchiveEntryStream) Close() error {
	if s == nil {
		return nil
	}
	if s.Body != nil {
		_ = s.Body.Close()
	}
	if s.ArchiveBody != nil {
		_ = s.ArchiveBody.Close()
	}
	if s.CacheLease != nil {
		_ = s.CacheLease.Close()
	}
	if strings.TrimSpace(s.TempPath) != "" {
		_ = os.Remove(s.TempPath)
	}
	return nil
}
