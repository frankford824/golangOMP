package externalassets

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const (
	ExternalCurrentSyncSchemaVersion = 1
	MaxExternalCurrentTickets        = 50
)

type externalCurrentSyncRepo interface {
	ListExternalAssetSyncSnapshot(context.Context, []repo.ExternalAssetOriginPrefix) ([]repo.ExternalAssetSyncRow, error)
	ListCurrentExternalAssetsForSyncByIDs(context.Context, []int64, []repo.ExternalAssetOriginPrefix) ([]repo.ExternalAssetSyncRow, error)
	GetExternalAssetSyncHead(context.Context, []repo.ExternalAssetOriginPrefix) (repo.ExternalAssetSyncCursor, error)
	ListExternalAssetSyncChanges(context.Context, []repo.ExternalAssetOriginPrefix, repo.ExternalAssetSyncCursor, int) ([]repo.ExternalAssetSyncRow, bool, error)
}

type ExternalCurrentManifest struct {
	SchemaVersion    int                           `json:"schema_version"`
	ManifestID       string                        `json:"manifest_id"`
	GeneratedAt      time.Time                     `json:"generated_at"`
	ItemCount        int                           `json:"item_count"`
	ActiveCount      int                           `json:"active_count"`
	DeletedCount     int                           `json:"deleted_count"`
	TotalObjectBytes int64                         `json:"total_object_bytes"`
	Items            []ExternalCurrentManifestItem `json:"items"`
}

type ExternalCurrentManifestItem struct {
	ExternalAssetID  int64      `json:"external_asset_id"`
	OriginPathHash   string     `json:"origin_path_hash"`
	RelativePath     string     `json:"relative_path"`
	FileName         string     `json:"file_name"`
	MimeType         string     `json:"mime_type"`
	FileSize         int64      `json:"file_size"`
	StorageKey       string     `json:"storage_key,omitempty"`
	SourceModifiedAt *time.Time `json:"source_modified_at"`
	RecordUpdatedAt  time.Time  `json:"record_updated_at"`
	Deleted          bool       `json:"deleted"`
}

type ExternalCurrentTicketStatus string

const (
	ExternalCurrentTicketReady        ExternalCurrentTicketStatus = "ready"
	ExternalCurrentTicketMissing      ExternalCurrentTicketStatus = "missing"
	ExternalCurrentTicketSizeMismatch ExternalCurrentTicketStatus = "size_mismatch"
	ExternalCurrentTicketNotCurrent   ExternalCurrentTicketStatus = "not_current"
	ExternalCurrentTicketError        ExternalCurrentTicketStatus = "error"
)

type ExternalCurrentTicketResult struct {
	ExternalAssetID int64                       `json:"external_asset_id"`
	Status          ExternalCurrentTicketStatus `json:"status"`
	OriginPathHash  string                      `json:"origin_path_hash,omitempty"`
	RelativePath    string                      `json:"relative_path,omitempty"`
	FileName        string                      `json:"file_name,omitempty"`
	StorageKey      string                      `json:"storage_key,omitempty"`
	ExpectedSize    *int64                      `json:"expected_size,omitempty"`
	ActualSize      *int64                      `json:"actual_size,omitempty"`
	ETag            string                      `json:"etag,omitempty"`
	CRC64ECMA       string                      `json:"crc64_ecma,omitempty"`
	DownloadURL     string                      `json:"download_url,omitempty"`
	ExpiresAt       *time.Time                  `json:"expires_at,omitempty"`
	Retryable       bool                        `json:"retryable"`
	ErrorMessage    string                      `json:"error_message,omitempty"`
}

type ExternalCurrentTicketResponse struct {
	Results []ExternalCurrentTicketResult `json:"results"`
}

type ExternalCurrentSyncHead struct {
	SchemaVersion int       `json:"schema_version"`
	Cursor        string    `json:"cursor"`
	ObservedAt    time.Time `json:"observed_at"`
}

type ExternalCurrentSyncChanges struct {
	SchemaVersion int                           `json:"schema_version"`
	Cursor        string                        `json:"cursor"`
	NextCursor    string                        `json:"next_cursor"`
	HasMore       bool                          `json:"has_more"`
	GeneratedAt   time.Time                     `json:"generated_at"`
	Items         []ExternalCurrentManifestItem `json:"items"`
}

func (s *Service) ExternalCurrentSyncManifest(ctx context.Context) (*ExternalCurrentManifest, *domain.AppError) {
	repository, prefixes, appErr := s.externalCurrentSyncDependencies()
	if appErr != nil {
		return nil, appErr
	}
	rows, err := repository.ListExternalAssetSyncSnapshot(ctx, prefixes)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("list external current manifest: %v", err), nil)
	}
	manifest, err := buildExternalCurrentManifest(rows, prefixes, time.Now().UTC())
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("build external current manifest: %v", err), nil)
	}
	return manifest, nil
}

func buildExternalCurrentManifest(rows []repo.ExternalAssetSyncRow, prefixes []repo.ExternalAssetOriginPrefix, generatedAt time.Time) (*ExternalCurrentManifest, error) {
	items := make([]ExternalCurrentManifestItem, 0, len(rows))
	var totalBytes int64
	activeCount := 0
	deletedCount := 0
	for _, row := range rows {
		item, ok := externalCurrentManifestItemFromRow(row, prefixes)
		if !ok {
			continue
		}
		if item.Deleted {
			deletedCount++
		} else {
			activeCount++
			totalBytes += row.FileSize
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].RelativePath != items[j].RelativePath {
			return items[i].RelativePath < items[j].RelativePath
		}
		return items[i].ExternalAssetID < items[j].ExternalAssetID
	})
	semanticItems := append([]ExternalCurrentManifestItem(nil), items...)
	for index := range semanticItems {
		// Database bookkeeping (preview/direct-link preparation) may update the
		// record timestamp without changing source bytes. Keep it observable in
		// the response but out of the semantic ETag.
		semanticItems[index].RecordUpdatedAt = time.Time{}
	}
	digestInput := struct {
		SchemaVersion int                           `json:"schema_version"`
		Items         []ExternalCurrentManifestItem `json:"items"`
	}{SchemaVersion: ExternalCurrentSyncSchemaVersion, Items: semanticItems}
	raw, err := json.Marshal(digestInput)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	return &ExternalCurrentManifest{
		SchemaVersion: ExternalCurrentSyncSchemaVersion, ManifestID: hex.EncodeToString(digest[:]), GeneratedAt: generatedAt,
		ItemCount: len(items), ActiveCount: activeCount, DeletedCount: deletedCount, TotalObjectBytes: totalBytes, Items: items,
	}, nil
}

func (s *Service) ExternalCurrentSyncHead(ctx context.Context) (*ExternalCurrentSyncHead, *domain.AppError) {
	repository, prefixes, appErr := s.externalCurrentSyncDependencies()
	if appErr != nil {
		return nil, appErr
	}
	cursor, err := repository.GetExternalAssetSyncHead(ctx, prefixes)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("get external current head: %v", err), nil)
	}
	return &ExternalCurrentSyncHead{SchemaVersion: ExternalCurrentSyncSchemaVersion, Cursor: encodeExternalCurrentCursor(cursor), ObservedAt: time.Now().UTC()}, nil
}

func (s *Service) ExternalCurrentSyncChanges(ctx context.Context, rawCursor string, limit int, wait time.Duration) (*ExternalCurrentSyncChanges, *domain.AppError) {
	after, err := decodeExternalCurrentCursor(rawCursor)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid external current cursor", nil)
	}
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	if wait < 0 || wait > 30*time.Second {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "wait_seconds must be between 0 and 30", nil)
	}
	repository, prefixes, appErr := s.externalCurrentSyncDependencies()
	if appErr != nil {
		return nil, appErr
	}
	deadline := time.Now().Add(wait)
	for {
		rows, hasMore, queryErr := repository.ListExternalAssetSyncChanges(ctx, prefixes, after, limit)
		if queryErr != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("list external current changes: %v", queryErr), nil)
		}
		items := make([]ExternalCurrentManifestItem, 0, len(rows))
		for _, row := range rows {
			if item, ok := externalCurrentManifestItemFromRow(row, prefixes); ok {
				items = append(items, item)
			}
		}
		next := after
		if len(rows) > 0 {
			last := rows[len(rows)-1]
			next = repo.ExternalAssetSyncCursor{UpdatedAt: last.UpdatedAt, ID: last.ID}
		}
		if len(rows) > 0 || wait == 0 || !time.Now().Before(deadline) {
			return &ExternalCurrentSyncChanges{
				SchemaVersion: ExternalCurrentSyncSchemaVersion,
				Cursor:        encodeExternalCurrentCursor(after), NextCursor: encodeExternalCurrentCursor(next),
				HasMore: hasMore, GeneratedAt: time.Now().UTC(), Items: items,
			}, nil
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, domain.NewAppError(domain.ErrCodeInternalError, ctx.Err().Error(), nil)
		case <-timer.C:
		}
	}
}

func externalCurrentManifestItemFromRow(row repo.ExternalAssetSyncRow, prefixes []repo.ExternalAssetOriginPrefix) (ExternalCurrentManifestItem, bool) {
	relativePath, ok := externalCurrentRelativePath(row.MountPath, row.OriginPath, prefixes)
	if !ok || ignoredExternalCurrentName(row.FileName) {
		return ExternalCurrentManifestItem{}, false
	}
	deleted := row.Status == domain.ExternalAssetStatusMissing
	item := ExternalCurrentManifestItem{
		ExternalAssetID: row.ID, OriginPathHash: strings.TrimSpace(row.OriginPathHash), RelativePath: relativePath,
		FileName: strings.TrimSpace(row.FileName), MimeType: strings.TrimSpace(row.MimeType), FileSize: row.FileSize,
		StorageKey: strings.TrimSpace(row.OSSOriginalKey), SourceModifiedAt: normalizedExternalCurrentTime(row.SourceModifiedAt),
		RecordUpdatedAt: row.UpdatedAt.UTC(), Deleted: deleted,
	}
	if deleted {
		item.StorageKey = ""
	}
	return item, true
}

func encodeExternalCurrentCursor(cursor repo.ExternalAssetSyncCursor) string {
	payload := "v1:" + strconv.FormatInt(cursor.UpdatedAt.UTC().UnixMicro(), 10) + ":" + strconv.FormatInt(cursor.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func decodeExternalCurrentCursor(value string) (repo.ExternalAssetSyncCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return repo.ExternalAssetSyncCursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return repo.ExternalAssetSyncCursor{}, err
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) != 3 || parts[0] != "v1" {
		return repo.ExternalAssetSyncCursor{}, fmt.Errorf("unsupported cursor")
	}
	micros, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || micros < 0 {
		return repo.ExternalAssetSyncCursor{}, fmt.Errorf("invalid cursor time")
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || id < 0 {
		return repo.ExternalAssetSyncCursor{}, fmt.Errorf("invalid cursor id")
	}
	return repo.ExternalAssetSyncCursor{UpdatedAt: time.UnixMicro(micros).UTC(), ID: id}, nil
}

func (s *Service) ExternalCurrentDownloadTickets(ctx context.Context, values []int64) (*ExternalCurrentTicketResponse, *domain.AppError) {
	ids, appErr := normalizeExternalCurrentTicketIDs(values)
	if appErr != nil {
		return nil, appErr
	}
	repository, prefixes, appErr := s.externalCurrentSyncDependencies()
	if appErr != nil {
		return nil, appErr
	}
	rows, err := repository.ListCurrentExternalAssetsForSyncByIDs(ctx, ids, prefixes)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("list external current tickets: %v", err), nil)
	}
	current := make(map[int64]repo.ExternalAssetSyncRow, len(rows))
	for _, row := range rows {
		current[row.ID] = row
	}
	results := make([]ExternalCurrentTicketResult, len(ids))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for index, id := range ids {
		index, id := index, id
		row, exists := current[id]
		if !exists {
			results[index] = ExternalCurrentTicketResult{ExternalAssetID: id, Status: ExternalCurrentTicketNotCurrent, Retryable: false}
			continue
		}
		relativePath, ok := externalCurrentRelativePath(row.MountPath, row.OriginPath, prefixes)
		if !ok {
			results[index] = ExternalCurrentTicketResult{ExternalAssetID: id, Status: ExternalCurrentTicketNotCurrent, Retryable: false}
			continue
		}
		wg.Add(1)
		go func(row repo.ExternalAssetSyncRow, relativePath string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[index] = externalCurrentTicketError(row, relativePath, ctx.Err(), true)
				return
			}
			results[index] = s.resolveExternalCurrentTicket(ctx, row, relativePath)
		}(row, relativePath)
	}
	wg.Wait()
	return &ExternalCurrentTicketResponse{Results: results}, nil
}

func (s *Service) resolveExternalCurrentTicket(ctx context.Context, row repo.ExternalAssetSyncRow, relativePath string) ExternalCurrentTicketResult {
	result := externalCurrentTicketBase(row, relativePath)
	if s.ossDirect == nil || !s.ossDirect.Enabled() {
		result.Status = ExternalCurrentTicketError
		result.ErrorMessage = "OSS object store is not configured"
		return result
	}
	info, exists, err := s.ossDirect.StatObject(ctx, row.OSSOriginalKey)
	if err != nil {
		return externalCurrentTicketError(row, relativePath, err, true)
	}
	if !exists || info == nil {
		result.Status = ExternalCurrentTicketMissing
		return result
	}
	actualSize := info.ContentLength
	result.ActualSize = &actualSize
	result.ETag = info.ETag
	result.CRC64ECMA = info.CRC64ECMA
	if actualSize != row.FileSize {
		result.Status = ExternalCurrentTicketSizeMismatch
		return result
	}
	signed := s.ossDirect.PresignDownloadURLWithFilename(row.OSSOriginalKey, row.FileName)
	if signed == nil || strings.TrimSpace(signed.DownloadURL) == "" {
		result.Status = ExternalCurrentTicketError
		result.ErrorMessage = "OSS download URL is unavailable"
		return result
	}
	result.Status = ExternalCurrentTicketReady
	result.DownloadURL = strings.TrimSpace(signed.DownloadURL)
	expiresAt := signed.ExpiresAt.UTC()
	result.ExpiresAt = &expiresAt
	return result
}

func (s *Service) externalCurrentSyncDependencies() (externalCurrentSyncRepo, []repo.ExternalAssetOriginPrefix, *domain.AppError) {
	if s == nil || !s.Enabled() {
		return nil, nil, domain.NewAppError(domain.ErrCodeInternalError, "external asset sync service is not configured", nil)
	}
	repository, ok := s.repo.(externalCurrentSyncRepo)
	if !ok {
		return nil, nil, domain.NewAppError(domain.ErrCodeInternalError, "external current sync repository is not configured", nil)
	}
	prefixes := s.externalCurrentSyncPrefixes()
	if len(prefixes) == 0 {
		return nil, nil, domain.NewAppError(domain.ErrCodeInternalError, "external current sync roots are not configured", nil)
	}
	return repository, prefixes, nil
}

func (s *Service) externalCurrentSyncPrefixes() []repo.ExternalAssetOriginPrefix {
	values := make([]repo.ExternalAssetOriginPrefix, 0, len(s.cfg.SyncExportRoots))
	for _, raw := range s.cfg.SyncExportRoots {
		root := cleanExternalMaterialBrowsePath(raw)
		for _, mount := range s.cfg.Mounts {
			mountPath := cleanExternalMaterialBrowsePath(mount.Path)
			if root == mountPath || strings.HasPrefix(root, mountPath+"/") {
				values = append(values, repo.ExternalAssetOriginPrefix{MountPath: mountPath, OriginPath: root})
				break
			}
		}
	}
	return values
}

func externalCurrentRelativePath(mountPath, originPath string, prefixes []repo.ExternalAssetOriginPrefix) (string, bool) {
	originPath = cleanExternalMaterialBrowsePath(originPath)
	best := ""
	for _, prefix := range prefixes {
		if cleanExternalMaterialBrowsePath(prefix.MountPath) != cleanExternalMaterialBrowsePath(mountPath) {
			continue
		}
		root := cleanExternalMaterialBrowsePath(prefix.OriginPath)
		if originPath == root || strings.HasPrefix(originPath, root+"/") {
			if len(root) > len(best) {
				best = root
			}
		}
	}
	if best == "" || originPath == best {
		return "", false
	}
	relativePath := strings.TrimPrefix(originPath, best+"/")
	cleaned := strings.TrimLeft(path.Clean("/"+relativePath), "/")
	if cleaned == "" || cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return cleaned, true
}

func ignoredExternalCurrentName(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "thumbs.db" || value == "desktop.ini" || strings.HasPrefix(value, "._")
}

func normalizedExternalCurrentTime(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	normalized := value.UTC().Truncate(time.Second)
	return &normalized
}

func normalizeExternalCurrentTicketIDs(values []int64) ([]int64, *domain.AppError) {
	if len(values) == 0 || len(values) > MaxExternalCurrentTickets {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "external_asset_ids must contain between 1 and 50 items", nil)
	}
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "external_asset_ids must contain positive integers", nil)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func externalCurrentTicketBase(row repo.ExternalAssetSyncRow, relativePath string) ExternalCurrentTicketResult {
	expectedSize := row.FileSize
	return ExternalCurrentTicketResult{
		ExternalAssetID: row.ID, OriginPathHash: row.OriginPathHash, RelativePath: relativePath,
		FileName: row.FileName, StorageKey: row.OSSOriginalKey, ExpectedSize: &expectedSize, Retryable: false,
	}
}

func externalCurrentTicketError(row repo.ExternalAssetSyncRow, relativePath string, err error, retryable bool) ExternalCurrentTicketResult {
	result := externalCurrentTicketBase(row, relativePath)
	result.Status = ExternalCurrentTicketError
	result.Retryable = retryable
	if err != nil {
		result.ErrorMessage = err.Error()
	}
	return result
}
