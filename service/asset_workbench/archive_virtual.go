package assetworkbench

import (
	"archive/tar"
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bodgit/sevenzip"
	"github.com/nwaples/rardecode/v2"

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
	indexMu     sync.Mutex
	indexReady  chan struct{}
	directories map[string]*ArchiveBrowseResult
	tmpPath     string
	ready       chan struct{}
	loading     bool
	refCount    int
	expiresAt   time.Time
	lastUsed    time.Time
}

type archiveTempCacheLease struct {
	entry   *archiveTempCacheEntry
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
	lease, err := s.cachedArchiveObjectTemp(ctx, file)
	if err != nil {
		return nil, archiveErrorToAppError(err)
	}
	defer lease.Close()
	cache := lease.entry
	for {
		cache.indexMu.Lock()
		if cache.directories != nil {
			result := cache.directories[normalizeDriveFolderPath(folderPath)]
			cache.indexMu.Unlock()
			if result == nil {
				result = emptyArchiveDirectory(file.ID, archiveFormatForFile(file), normalizeDriveFolderPath(folderPath))
			}
			return result, nil
		}
		if ready := cache.indexReady; ready != nil {
			cache.indexMu.Unlock()
			select {
			case <-ready:
				continue
			case <-ctx.Done():
				return nil, archiveErrorToAppError(ctx.Err())
			}
		}
		cache.indexReady = make(chan struct{})
		cache.indexMu.Unlock()
		dirs, appErr := buildArchiveDirectoryIndex(ctx, file, lease)
		cache.indexMu.Lock()
		cache.directories = dirs
		close(cache.indexReady)
		cache.indexReady = nil
		cache.indexMu.Unlock()
		if appErr != nil {
			return nil, appErr
		}
	}
}

func emptyArchiveDirectory(id int64, format, folder string) *ArchiveBrowseResult {
	return &ArchiveBrowseResult{FileID: id, Format: format, Path: folder, Folders: []ArchiveVirtualFolder{}, Files: []ArchiveVirtualFile{}}
}

func buildArchiveDirectoryIndex(ctx context.Context, file *domain.AssetWorkbenchSubmissionFile, lease *archiveTempCacheLease) (map[string]*ArchiveBrowseResult, *domain.AppError) {
	reader, appErr := openArchiveReader(ctx, file, lease)
	if appErr != nil {
		return nil, appErr
	}
	defer reader.closeSource()
	format := archiveFormatForFile(file)
	dirs := map[string]*ArchiveBrowseResult{"": emptyArchiveDirectory(file.ID, format, "")}
	descendants := map[string]int64{}
	for {
		entry, err := reader.nextVisible(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, archiveErrorToAppError(err)
		}
		folder := entry.name
		if !entry.mode.IsDir() {
			folder = archiveParent(folder)
		}
		for ancestor := folder; ancestor != ""; ancestor = archiveParent(ancestor) {
			if dirs[ancestor] == nil {
				dirs[ancestor] = emptyArchiveDirectory(file.ID, format, ancestor)
			}
			if !entry.mode.IsDir() {
				descendants[ancestor]++
			}
		}
		if entry.mode.IsDir() {
			continue
		}
		name := path.Base(entry.name)
		mimeType := archiveEntryMimeType(entry.name)
		row := ArchiveVirtualFile{Name: name, Path: entry.name, FileSize: entry.size, MimeType: mimeType, FileType: inferFileType(name, mimeType), DownloadURL: archiveEntryURL(file.ID, entry.name, false)}
		if archiveEntryPreviewable(name, mimeType) {
			row.PreviewURL = archiveEntryURL(file.ID, entry.name, true)
		}
		dirs[folder].Files = append(dirs[folder].Files, row)
	}
	for folder := range dirs {
		if folder != "" {
			parent := dirs[archiveParent(folder)]
			parent.Folders = append(parent.Folders, ArchiveVirtualFolder{Name: path.Base(folder), Path: folder, FileCount: descendants[folder]})
		}
	}
	for _, dir := range dirs {
		sort.Slice(dir.Folders, func(i, j int) bool { return dir.Folders[i].Name < dir.Folders[j].Name })
		sort.Slice(dir.Files, func(i, j int) bool { return dir.Files[i].Name < dir.Files[j].Name })
	}
	return dirs, nil
}

func archiveParent(name string) string {
	if pos := strings.LastIndexByte(name, '/'); pos >= 0 {
		return name[:pos]
	}
	return ""
}

func (s *Service) OpenArchiveEntry(ctx context.Context, actor domain.RequestActor, fileID int64, entryPath string) (*ArchiveEntryStream, *domain.AppError) {
	file, appErr := s.loadVisibleArchiveFile(ctx, actor, fileID)
	if appErr != nil {
		return nil, appErr
	}
	target, skip, err := sanitizeArchiveEntryPath(entryPath, archiveVirtualMaxDepth)
	if err != nil || skip {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "压缩包路径无效", nil)
	}
	reader, appErr := s.openVirtualArchive(ctx, file)
	if appErr != nil {
		return nil, appErr
	}
	for {
		entry, err := reader.nextVisible(ctx)
		if err == io.EOF {
			reader.Close()
			return nil, domain.NewAppError(domain.ErrCodeNotFound, "压缩包内未找到该文件", nil)
		}
		if err != nil {
			reader.Close()
			return nil, archiveErrorToAppError(err)
		}
		if entry.name != target || entry.mode.IsDir() {
			continue
		}
		body, err := entry.open()
		if err != nil {
			reader.Close()
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "压缩包内文件无法打开，请检查是否加密或损坏", err.Error())
		}
		return &ArchiveEntryStream{Body: body, Filename: path.Base(target), MimeType: archiveEntryMimeType(target), FileSize: entry.size, ArchiveBody: reader}, nil
	}
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

type virtualMember struct {
	name string
	size int64
	mode fs.FileMode
	open func() (io.ReadCloser, error)
}

type virtualArchiveReader struct {
	releaseSlot func()
	source      io.Closer
	lease       *archiveTempCacheLease
	next        func() (virtualMember, error)
	count       int
	total       int64
}

func (r *virtualArchiveReader) Close() error {
	defer r.lease.Close()
	return r.closeSource()
}

func (r *virtualArchiveReader) closeSource() error {
	defer r.releaseSlot()
	return r.source.Close()
}

// Both directory navigation and direct member reads enforce the same path and size limits.
func (r *virtualArchiveReader) nextVisible(ctx context.Context) (virtualMember, error) {
	for {
		if err := ctx.Err(); err != nil {
			return virtualMember{}, err
		}
		m, err := r.next()
		if err != nil && !errors.Is(err, io.EOF) && !isPermanentArchiveVirtualError(err) {
			return virtualMember{}, permanentArchiveVirtualErrorf("压缩包无法读取，请检查文件是否完整或加密")
		}
		if err != nil {
			return virtualMember{}, err
		}
		r.count++
		if r.count > archiveVirtualMaxVisibleFileCount {
			return virtualMember{}, permanentArchiveVirtualErrorf("压缩包条目过多，暂不支持在线打开")
		}
		if m.size < 0 || m.size > archiveVirtualMaxUncompressedBytes-r.total {
			return virtualMember{}, permanentArchiveVirtualErrorf("压缩包内容过大，暂不支持在线打开")
		}
		r.total += m.size
		if !m.mode.IsRegular() && !m.mode.IsDir() {
			continue
		}
		if m.mode.IsDir() && path.Clean(m.name) == "." {
			continue
		}
		clean, skip, err := sanitizeArchiveEntryPath(m.name, archiveVirtualMaxDepth)
		if err != nil {
			return virtualMember{}, err
		}
		if skip {
			continue
		}
		m.name = clean
		return m, nil
	}
}

func indexedMembers[T any](files []T, member func(T) virtualMember) func() (virtualMember, error) {
	index := 0
	return func() (virtualMember, error) {
		if index == len(files) {
			return virtualMember{}, io.EOF
		}
		m := member(files[index])
		index++
		return m, nil
	}
}

func (s *Service) openVirtualArchive(ctx context.Context, file *domain.AssetWorkbenchSubmissionFile) (*virtualArchiveReader, *domain.AppError) {
	lease, err := s.cachedArchiveObjectTemp(ctx, file)
	if err != nil {
		return nil, archiveErrorToAppError(err)
	}
	reader, appErr := openArchiveReader(ctx, file, lease)
	if appErr != nil {
		lease.Close()
	}
	return reader, appErr
}

func (s *Service) acquireArchiveSlot(ctx context.Context) (func(), error) {
	select {
	case s.archiveSlots <- struct{}{}:
		return sync.OnceFunc(func() { <-s.archiveSlots }), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func openArchiveReader(ctx context.Context, file *domain.AssetWorkbenchSubmissionFile, lease *archiveTempCacheLease) (*virtualArchiveReader, *domain.AppError) {
	release, err := lease.service.acquireArchiveSlot(ctx)
	if err != nil {
		return nil, archiveErrorToAppError(err)
	}
	r := &virtualArchiveReader{lease: lease, releaseSlot: release}
	switch archiveFormatForFile(file) {
	case "zip":
		var z *zip.ReadCloser
		z, err = zip.OpenReader(lease.Path())
		if err == nil {
			r.source = z
			r.next = indexedMembers(z.File, func(f *zip.File) virtualMember {
				return virtualMember{f.Name, int64(f.UncompressedSize64), f.Mode(), f.Open}
			})
		}
	case "7z":
		var z *sevenzip.ReadCloser
		z, err = sevenzip.OpenReader(lease.Path())
		if err == nil {
			r.source = z
			r.next = indexedMembers(z.File, func(f *sevenzip.File) virtualMember {
				return virtualMember{f.Name, int64(f.UncompressedSize), f.Mode(), f.Open}
			})
		}
	case "tar":
		var f *os.File
		f, err = os.Open(lease.Path())
		if err == nil {
			r.source = f
			tr := tar.NewReader(f)
			r.next = func() (virtualMember, error) {
				h, err := tr.Next()
				if err != nil {
					return virtualMember{}, err
				}
				mode := h.FileInfo().Mode()
				if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA && h.Typeflag != tar.TypeDir {
					mode = fs.ModeIrregular
				}
				return virtualMember{h.Name, h.Size, mode, func() (io.ReadCloser, error) { return io.NopCloser(io.LimitReader(tr, h.Size)), nil }}, nil
			}
		}
	case "rar":
		var rr *rardecode.ReadCloser
		rr, err = rardecode.OpenReader(lease.Path())
		if err == nil {
			r.source = rr
			r.next = func() (virtualMember, error) {
				h, err := rr.Next()
				if err != nil {
					return virtualMember{}, err
				}
				if h.Encrypted || h.HeaderEncrypted {
					return virtualMember{}, permanentArchiveVirtualErrorf("加密压缩包暂不支持在线打开，可下载后查看")
				}
				if h.UnKnownSize {
					return virtualMember{}, permanentArchiveVirtualErrorf("压缩包内文件大小无法确认")
				}
				return virtualMember{h.Name, h.UnPackedSize, h.Mode(), func() (io.ReadCloser, error) { return io.NopCloser(io.LimitReader(rr, h.UnPackedSize)), nil }}, nil
			}
		}
	default:
		err = errors.New("unsupported archive format")
	}
	if err != nil {
		release()
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "压缩包无法打开，请检查文件是否完整或加密", err.Error())
	}
	return r, nil
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
	release, err := s.acquireArchiveSlot(ctx)
	if err != nil {
		return "", err
	}
	defer release()
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
				return &archiveTempCacheLease{entry: entry, service: s, key: key, path: path}, nil
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
			return &archiveTempCacheLease{entry: entry, service: s, key: key, path: tmpPath}, nil
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
		name := file.OriginalFilename
		if strings.TrimSpace(name) == "" {
			name = file.DisplayName
		}
		ext = strings.TrimPrefix(strings.ToLower(path.Ext(strings.ReplaceAll(name, "\\", "/"))), ".")
	}
	switch ext {
	case "zip", "rar", "tar", "7z":
		return ext
	default:
		return ""
	}
}

func sanitizeArchiveEntryPath(raw string, maxDepth int) (string, bool, error) {
	value := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	value = strings.TrimRight(value, "/")
	if value == "" {
		return "", true, nil
	}
	if strings.ContainsRune(value, 0) || path.IsAbs(value) || strings.Contains("/"+value+"/", "/../") {
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
	if isPSBFilename(filename) {
		return false
	}
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
