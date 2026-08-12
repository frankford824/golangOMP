package asset_center

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
)

type finalizedSyncRepoStub struct {
	rows  []repo.ProductionPackageAsset
	byIDs []repo.ProductionPackageAsset
}

func (s *finalizedSyncRepoStub) ListAllFinalizedAssets(context.Context) ([]repo.ProductionPackageAsset, error) {
	return s.rows, nil
}

func (s *finalizedSyncRepoStub) ListFinalizedAssetsByIDs(_ context.Context, ids []int64) ([]repo.ProductionPackageAsset, error) {
	allowed := map[int64]struct{}{}
	for _, id := range ids {
		allowed[id] = struct{}{}
	}
	out := make([]repo.ProductionPackageAsset, 0)
	for _, row := range s.byIDs {
		if _, exists := allowed[row.TaskAssetID]; exists {
			out = append(out, row)
		}
	}
	return out, nil
}

type finalizedSyncStoreStub struct {
	infos map[string]*baseservice.OSSObjectInfo
	errs  map[string]error
}

func (s *finalizedSyncStoreStub) Enabled() bool { return true }

func (s *finalizedSyncStoreStub) StatObject(_ context.Context, key string) (*baseservice.OSSObjectInfo, bool, error) {
	if err := s.errs[key]; err != nil {
		return nil, false, err
	}
	info, exists := s.infos[key]
	return info, exists, nil
}

func (s *finalizedSyncStoreStub) PresignDownloadURLWithFilename(key, _ string) *baseservice.OSSDirectDownloadInfo {
	return &baseservice.OSSDirectDownloadInfo{DownloadURL: "https://oss.example/" + key, ExpiresAt: time.Unix(2000, 0).UTC()}
}

func finalizedSyncRow(groupID, revisionItemID, assetID int64, sortOrder int, filename string, size int64) repo.ProductionPackageAsset {
	return repo.ProductionPackageAsset{
		GroupID: groupID, RevisionID: groupID * 10, RevisionMode: domain.TaskAssetGroupModeSet,
		RevisionFinalizedAt: time.Unix(groupID*100, 0).UTC(), RevisionItemID: revisionItemID,
		SortOrder: sortOrder, ItemName: filename, TaskAssetID: assetID, TaskID: groupID * 1000,
		TaskNo: "RW-TEST", SKUCode: "SKU-1", SKUName: "Product", ScopeKind: domain.TaskAssetGroupScopeSKU,
		FileName: filename, MimeType: "image/jpeg", FileSize: size, StorageKey: "tasks/" + filename,
		UpdatedAt: time.Unix(1500+assetID, 0).UTC(),
	}
}

func TestBuildFinalizedSyncManifestIsDeterministicAndProductionFiltered(t *testing.T) {
	rows := []repo.ProductionPackageAsset{
		finalizedSyncRow(1, 12, 102, 2, "B.jpg", 20),
		finalizedSyncRow(2, 21, 201, 0, "new.png", 30),
		finalizedSyncRow(1, 11, 101, 1, "A.jpeg", 10),
		finalizedSyncRow(3, 31, 301, 0, "preview.jpg", 40),
		finalizedSyncRow(4, 41, 401, 0, "source.psd", 50),
	}
	rows[4].MimeType = "application/octet-stream"
	first, err := buildFinalizedSyncManifest(rows, time.Unix(3000, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildFinalizedSyncManifest(rows, time.Unix(4000, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if first.ManifestID != second.ManifestID {
		t.Fatalf("manifest id changed with generated_at: %s != %s", first.ManifestID, second.ManifestID)
	}
	if first.GroupCount != 2 || first.ItemCount != 3 || first.ObjectCount != 3 || first.TotalObjectBytes != 60 {
		t.Fatalf("unexpected manifest counts: %+v", first)
	}
	if got := []int64{first.Groups[0].GroupID, first.Groups[1].GroupID}; !reflect.DeepEqual(got, []int64{2, 1}) {
		t.Fatalf("group order = %v", got)
	}
	if got := []int64{first.Groups[1].Items[0].TaskAssetID, first.Groups[1].Items[1].TaskAssetID}; !reflect.DeepEqual(got, []int64{101, 102}) {
		t.Fatalf("item order = %v", got)
	}
	if first.Groups[1].Items[0].Format != "jpg" {
		t.Fatalf("jpeg format = %q", first.Groups[1].Items[0].Format)
	}
}

func TestFinalizedDownloadTicketsReturnsPerItemStatuses(t *testing.T) {
	ready := finalizedSyncRow(1, 1, 1, 0, "ready.jpg", 10)
	missing := finalizedSyncRow(2, 2, 2, 0, "missing.jpg", 20)
	mismatch := finalizedSyncRow(3, 3, 3, 0, "mismatch.jpg", 30)
	failing := finalizedSyncRow(4, 4, 4, 0, "error.jpg", 40)
	repository := &finalizedSyncRepoStub{byIDs: []repo.ProductionPackageAsset{ready, missing, mismatch, failing}}
	store := &finalizedSyncStoreStub{
		infos: map[string]*baseservice.OSSObjectInfo{
			ready.StorageKey:    {ContentLength: 10, ETag: "etag-ready", CRC64ECMA: "123"},
			mismatch.StorageKey: {ContentLength: 31, ETag: "etag-mismatch"},
		},
		errs: map[string]error{failing.StorageKey: errors.New("temporary OSS failure")},
	}
	svc := NewService(nil, nil, nil, WithFinalizedAssetSync(repository, store))
	result, appErr := svc.FinalizedDownloadTickets(context.Background(), []int64{1, 2, 3, 4, 5, 1})
	if appErr != nil {
		t.Fatalf("unexpected app error: %v", appErr)
	}
	want := []FinalizedDownloadTicketStatus{
		FinalizedDownloadReady, FinalizedDownloadMissing, FinalizedDownloadSizeMismatch,
		FinalizedDownloadError, FinalizedDownloadNotCurrent,
	}
	got := make([]FinalizedDownloadTicketStatus, len(result.Results))
	for index := range result.Results {
		got[index] = result.Results[index].Status
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	if result.Results[0].ETag != "etag-ready" || result.Results[0].CRC64ECMA != "123" || result.Results[0].DownloadURL == "" {
		t.Fatalf("ready ticket = %+v", result.Results[0])
	}
	if result.Results[2].ActualSize == nil || *result.Results[2].ActualSize != 31 || result.Results[2].DownloadURL != "" {
		t.Fatalf("mismatch ticket = %+v", result.Results[2])
	}
	if !result.Results[3].Retryable {
		t.Fatalf("error ticket should be retryable: %+v", result.Results[3])
	}
	if result.Results[4].StorageKey != "" {
		t.Fatalf("not_current leaked storage key: %+v", result.Results[4])
	}
}

func TestFinalizedDownloadTicketsValidatesIDs(t *testing.T) {
	svc := NewService(nil, nil, nil)
	for _, ids := range [][]int64{nil, {0}, make([]int64, 51)} {
		if _, appErr := svc.FinalizedDownloadTickets(context.Background(), ids); appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
			t.Fatalf("ids=%v error=%v", ids, appErr)
		}
	}
}
