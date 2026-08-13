package asset_center

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
)

const (
	FinalizedSyncSchemaVersion  = 1
	MaxFinalizedDownloadTickets = 50
)

type FinalizedAssetSyncObjectStore interface {
	Enabled() bool
	StatObject(context.Context, string) (*baseservice.OSSObjectInfo, bool, error)
	PresignDownloadURLWithFilename(string, string) *baseservice.OSSDirectDownloadInfo
}

type FinalizedSyncManifest struct {
	SchemaVersion    int                  `json:"schema_version"`
	ManifestID       string               `json:"manifest_id"`
	GeneratedAt      time.Time            `json:"generated_at"`
	GroupCount       int                  `json:"group_count"`
	ItemCount        int                  `json:"item_count"`
	ObjectCount      int                  `json:"object_count"`
	TotalObjectBytes int64                `json:"total_object_bytes"`
	Groups           []FinalizedSyncGroup `json:"groups"`
}

type FinalizedSyncGroup struct {
	GroupID      int64                          `json:"group_id"`
	RevisionID   int64                          `json:"revision_id"`
	RevisionMode domain.TaskAssetGroupMode      `json:"revision_mode"`
	FinalizedAt  time.Time                      `json:"finalized_at"`
	TaskID       int64                          `json:"task_id"`
	TaskNo       string                         `json:"task_no"`
	ScopeKind    domain.TaskAssetGroupScopeKind `json:"scope_kind"`
	SKUCode      string                         `json:"sku_code"`
	ProductName  string                         `json:"product_name"`
	Items        []FinalizedSyncItem            `json:"items"`
}

type FinalizedSyncItem struct {
	RevisionItemID   int64     `json:"revision_item_id"`
	SortOrder        int       `json:"sort_order"`
	ItemName         string    `json:"item_name"`
	TaskAssetID      int64     `json:"task_asset_id"`
	FileName         string    `json:"file_name"`
	OriginalFilename string    `json:"original_filename"`
	Format           string    `json:"format"`
	MimeType         string    `json:"mime_type"`
	FileSize         int64     `json:"file_size"`
	StorageKey       string    `json:"storage_key"`
	WholeHash        *string   `json:"whole_hash"`
	AssetUpdatedAt   time.Time `json:"asset_updated_at"`
}

type FinalizedDownloadTicketStatus string

const (
	FinalizedDownloadReady        FinalizedDownloadTicketStatus = "ready"
	FinalizedDownloadMissing      FinalizedDownloadTicketStatus = "missing"
	FinalizedDownloadSizeMismatch FinalizedDownloadTicketStatus = "size_mismatch"
	FinalizedDownloadNotCurrent   FinalizedDownloadTicketStatus = "not_current"
	FinalizedDownloadError        FinalizedDownloadTicketStatus = "error"
)

type FinalizedDownloadTicketResult struct {
	TaskAssetID  int64                         `json:"task_asset_id"`
	Status       FinalizedDownloadTicketStatus `json:"status"`
	StorageKey   string                        `json:"storage_key,omitempty"`
	FileName     string                        `json:"file_name,omitempty"`
	ExpectedSize *int64                        `json:"expected_size,omitempty"`
	ActualSize   *int64                        `json:"actual_size,omitempty"`
	ETag         string                        `json:"etag,omitempty"`
	CRC64ECMA    string                        `json:"crc64_ecma,omitempty"`
	WholeHash    *string                       `json:"whole_hash,omitempty"`
	DownloadURL  string                        `json:"download_url,omitempty"`
	ExpiresAt    *time.Time                    `json:"expires_at,omitempty"`
	Retryable    bool                          `json:"retryable"`
	ErrorMessage string                        `json:"error_message,omitempty"`
}

type FinalizedDownloadTicketResponse struct {
	Results []FinalizedDownloadTicketResult `json:"results"`
}

func (s *Service) FinalizedSyncManifest(ctx context.Context) (*FinalizedSyncManifest, *domain.AppError) {
	if s == nil || s.finalizedSyncRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "finalized asset sync repository is not configured", nil)
	}
	rows, err := s.finalizedSyncRepo.ListAllFinalizedAssets(ctx)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("list finalized sync manifest: %v", err), nil)
	}
	manifest, err := buildFinalizedSyncManifest(rows, time.Now().UTC())
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("build finalized sync manifest: %v", err), nil)
	}
	return manifest, nil
}

func buildFinalizedSyncManifest(rows []repo.ProductionPackageAsset, generatedAt time.Time) (*FinalizedSyncManifest, error) {
	groups := make([]FinalizedSyncGroup, 0)
	groupIndex := map[int64]int{}
	objectSizes := map[int64]int64{}
	for _, row := range rows {
		format, eligible := finalizedSyncFormat(row)
		if !eligible {
			continue
		}
		index, exists := groupIndex[row.GroupID]
		if !exists {
			index = len(groups)
			groupIndex[row.GroupID] = index
			groups = append(groups, FinalizedSyncGroup{
				GroupID: row.GroupID, RevisionID: row.RevisionID, RevisionMode: row.RevisionMode,
				FinalizedAt: row.RevisionFinalizedAt.UTC(), TaskID: row.TaskID, TaskNo: row.TaskNo,
				ScopeKind: row.ScopeKind, SKUCode: row.SKUCode, ProductName: row.SKUName,
				Items: []FinalizedSyncItem{},
			})
		}
		groups[index].Items = append(groups[index].Items, finalizedSyncItem(row, format))
		objectSizes[row.TaskAssetID] = row.FileSize
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if !groups[i].FinalizedAt.Equal(groups[j].FinalizedAt) {
			return groups[i].FinalizedAt.After(groups[j].FinalizedAt)
		}
		return groups[i].GroupID > groups[j].GroupID
	})
	itemCount := 0
	for index := range groups {
		sort.SliceStable(groups[index].Items, func(i, j int) bool {
			if groups[index].Items[i].SortOrder != groups[index].Items[j].SortOrder {
				return groups[index].Items[i].SortOrder < groups[index].Items[j].SortOrder
			}
			return groups[index].Items[i].RevisionItemID < groups[index].Items[j].RevisionItemID
		})
		itemCount += len(groups[index].Items)
	}
	var totalBytes int64
	for _, size := range objectSizes {
		totalBytes += size
	}
	digestInput := struct {
		SchemaVersion int                  `json:"schema_version"`
		Groups        []FinalizedSyncGroup `json:"groups"`
	}{SchemaVersion: FinalizedSyncSchemaVersion, Groups: groups}
	raw, err := json.Marshal(digestInput)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	return &FinalizedSyncManifest{
		SchemaVersion: FinalizedSyncSchemaVersion,
		ManifestID:    hex.EncodeToString(digest[:]),
		GeneratedAt:   generatedAt,
		GroupCount:    len(groups), ItemCount: itemCount, ObjectCount: len(objectSizes),
		TotalObjectBytes: totalBytes, Groups: groups,
	}, nil
}

func finalizedSyncFormat(row repo.ProductionPackageAsset) (string, bool) {
	format := normalizeExcelPackageExtension(firstNonEmptyExcelPackage(row.OriginalFilename, row.FileName, row.ItemName), row.MimeType)
	if !excelPackageExtensionMatches(format, "image") || hasProductionPackageExcludedMarker(row.FileName, row.OriginalFilename, row.ItemName) {
		return "", false
	}
	return format, true
}

func finalizedSyncItem(row repo.ProductionPackageAsset, format string) FinalizedSyncItem {
	var wholeHash *string
	if value := strings.TrimSpace(row.WholeHash); value != "" {
		wholeHash = &value
	}
	return FinalizedSyncItem{
		RevisionItemID: row.RevisionItemID, SortOrder: row.SortOrder, ItemName: row.ItemName,
		TaskAssetID: row.TaskAssetID, FileName: row.FileName, OriginalFilename: row.OriginalFilename,
		Format: format, MimeType: row.MimeType, FileSize: row.FileSize, StorageKey: row.StorageKey,
		WholeHash: wholeHash, AssetUpdatedAt: row.CreatedAt.UTC(),
	}
}

func (s *Service) FinalizedDownloadTickets(ctx context.Context, taskAssetIDs []int64) (*FinalizedDownloadTicketResponse, *domain.AppError) {
	ids, appErr := normalizeFinalizedTicketIDs(taskAssetIDs)
	if appErr != nil {
		return nil, appErr
	}
	if s == nil || s.finalizedSyncRepo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "finalized asset sync repository is not configured", nil)
	}
	rows, err := s.finalizedSyncRepo.ListFinalizedAssetsByIDs(ctx, ids)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, fmt.Sprintf("list finalized download tickets: %v", err), nil)
	}
	current := make(map[int64]repo.ProductionPackageAsset, len(rows))
	for _, row := range rows {
		if _, eligible := finalizedSyncFormat(row); eligible {
			if _, exists := current[row.TaskAssetID]; !exists {
				current[row.TaskAssetID] = row
			}
		}
	}
	results := make([]FinalizedDownloadTicketResult, len(ids))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for index, id := range ids {
		index, id := index, id
		row, exists := current[id]
		if !exists {
			results[index] = FinalizedDownloadTicketResult{TaskAssetID: id, Status: FinalizedDownloadNotCurrent, Retryable: false}
			continue
		}
		wg.Add(1)
		go func(row repo.ProductionPackageAsset) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[index] = finalizedTicketError(row, ctx.Err(), true)
				return
			}
			results[index] = s.resolveFinalizedDownloadTicket(ctx, row)
		}(row)
	}
	wg.Wait()
	return &FinalizedDownloadTicketResponse{Results: results}, nil
}

func normalizeFinalizedTicketIDs(values []int64) ([]int64, *domain.AppError) {
	if len(values) == 0 || len(values) > MaxFinalizedDownloadTickets {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_asset_ids must contain between 1 and 50 items", nil)
	}
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_asset_ids must contain positive integers", nil)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func (s *Service) resolveFinalizedDownloadTicket(ctx context.Context, row repo.ProductionPackageAsset) FinalizedDownloadTicketResult {
	base := finalizedTicketBase(row)
	if s.finalizedSyncStore == nil || !s.finalizedSyncStore.Enabled() {
		base.Status = FinalizedDownloadError
		base.ErrorMessage = "OSS object store is not configured"
		return base
	}
	info, exists, err := s.finalizedSyncStore.StatObject(ctx, row.StorageKey)
	if err != nil {
		return finalizedTicketError(row, err, true)
	}
	if !exists || info == nil {
		base.Status = FinalizedDownloadMissing
		return base
	}
	actualSize := info.ContentLength
	base.ActualSize = &actualSize
	base.ETag = info.ETag
	base.CRC64ECMA = info.CRC64ECMA
	if info.ContentLength != row.FileSize {
		base.Status = FinalizedDownloadSizeMismatch
		return base
	}
	filename := firstNonEmptyExcelPackage(row.OriginalFilename, row.FileName, row.ItemName)
	signed := s.finalizedSyncStore.PresignDownloadURLWithFilename(row.StorageKey, filename)
	if signed == nil || strings.TrimSpace(signed.DownloadURL) == "" {
		base.Status = FinalizedDownloadError
		base.ErrorMessage = "OSS download URL is unavailable"
		return base
	}
	base.Status = FinalizedDownloadReady
	base.DownloadURL = strings.TrimSpace(signed.DownloadURL)
	expiresAt := signed.ExpiresAt.UTC()
	base.ExpiresAt = &expiresAt
	return base
}

func finalizedTicketBase(row repo.ProductionPackageAsset) FinalizedDownloadTicketResult {
	expectedSize := row.FileSize
	var wholeHash *string
	if value := strings.TrimSpace(row.WholeHash); value != "" {
		wholeHash = &value
	}
	return FinalizedDownloadTicketResult{
		TaskAssetID: row.TaskAssetID, StorageKey: row.StorageKey,
		FileName:     firstNonEmptyExcelPackage(row.OriginalFilename, row.FileName, row.ItemName),
		ExpectedSize: &expectedSize, WholeHash: wholeHash, Retryable: false,
	}
}

func finalizedTicketError(row repo.ProductionPackageAsset, err error, retryable bool) FinalizedDownloadTicketResult {
	result := finalizedTicketBase(row)
	result.Status = FinalizedDownloadError
	result.Retryable = retryable
	if err != nil {
		result.ErrorMessage = err.Error()
	}
	return result
}
