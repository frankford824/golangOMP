package externalassets

import (
	"context"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestBuildExternalCurrentManifestIsDeterministicAndCarriesTombstones(t *testing.T) {
	prefixes := []repo.ExternalAssetOriginPrefix{{MountPath: "/p3", OriginPath: "/p3/仓库素材区/徐凯/1"}}
	modified := time.Date(2026, 8, 19, 10, 0, 0, 0, time.Local)
	rows := []repo.ExternalAssetSyncRow{
		{ID: 2, MountPath: "/p3", OriginPathHash: "hash-2", OriginPath: "/p3/仓库素材区/徐凯/1/KT/SKU/old.jpg", FileName: "old.jpg", FileSize: 20, Status: domain.ExternalAssetStatusMissing, UpdatedAt: modified},
		{ID: 1, MountPath: "/p3", OriginPathHash: "hash-1", OriginPath: "/p3/仓库素材区/徐凯/1/KT/SKU/new.jpg", FileName: "new.jpg", MimeType: "image/jpeg", FileSize: 10, Status: domain.ExternalAssetStatusIndexed, OSSSyncStatus: domain.ExternalAssetOSSStatusReady, OSSOriginalKey: "external/new.jpg", SourceModifiedAt: &modified, UpdatedAt: modified},
		{ID: 3, MountPath: "/p3", OriginPathHash: "hash-3", OriginPath: "/p3/仓库素材区/徐凯/1/KT/SKU/Thumbs.db", FileName: "Thumbs.db", Status: domain.ExternalAssetStatusIndexed, OSSSyncStatus: domain.ExternalAssetOSSStatusReady, OSSOriginalKey: "external/thumbs"},
		{ID: 4, MountPath: "/p3", OriginPathHash: "hash-4", OriginPath: "/p3/other/outside.jpg", FileName: "outside.jpg", Status: domain.ExternalAssetStatusIndexed, OSSSyncStatus: domain.ExternalAssetOSSStatusReady, OSSOriginalKey: "external/outside"},
	}
	first, err := buildExternalCurrentManifest(rows, prefixes, time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildExternalCurrentManifest(rows, prefixes, time.Unix(2, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if first.ManifestID != second.ManifestID {
		t.Fatalf("manifest IDs differ: %s != %s", first.ManifestID, second.ManifestID)
	}
	rows[1].UpdatedAt = modified.Add(time.Hour)
	bookkeepingOnly, err := buildExternalCurrentManifest(rows, prefixes, time.Unix(3, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if first.ManifestID != bookkeepingOnly.ManifestID {
		t.Fatal("record bookkeeping timestamp changed semantic manifest ID")
	}
	if first.ItemCount != 2 || first.ActiveCount != 1 || first.DeletedCount != 1 || first.TotalObjectBytes != 10 {
		t.Fatalf("manifest counts = %+v", first)
	}
	if first.Items[0].RelativePath != "KT/SKU/new.jpg" || first.Items[0].Deleted || first.Items[0].StorageKey != "external/new.jpg" {
		t.Fatalf("active item = %+v", first.Items[0])
	}
	if first.Items[1].RelativePath != "KT/SKU/old.jpg" || !first.Items[1].Deleted || first.Items[1].StorageKey != "" {
		t.Fatalf("deleted item = %+v", first.Items[1])
	}
}

func TestNormalizeExternalCurrentTicketIDs(t *testing.T) {
	ids, appErr := normalizeExternalCurrentTicketIDs([]int64{3, 3, 2})
	if appErr != nil || len(ids) != 2 || ids[0] != 3 || ids[1] != 2 {
		t.Fatalf("ids=%v appErr=%+v", ids, appErr)
	}
	if _, appErr := normalizeExternalCurrentTicketIDs(nil); appErr == nil {
		t.Fatal("expected empty ticket request to fail")
	}
}

func TestExternalCurrentHeadAndChangesCursor(t *testing.T) {
	updated := time.Date(2026, 8, 19, 10, 11, 12, 0, time.UTC)
	repository := &externalAssetRepoStub{
		syncHead: repo.ExternalAssetSyncCursor{UpdatedAt: updated, ID: 99},
		syncChanges: []repo.ExternalAssetSyncRow{{
			ID: 100, MountPath: "/p3", OriginPathHash: strings.Repeat("a", 64),
			OriginPath: "/p3/仓库素材区/徐凯/1/KT/HSC100.jpg", FileName: "HSC100.jpg",
			MimeType: "image/jpeg", FileSize: 10, Status: domain.ExternalAssetStatusIndexed,
			OSSSyncStatus: domain.ExternalAssetOSSStatusReady, OSSOriginalKey: "external/HSC100.jpg", UpdatedAt: updated.Add(time.Second),
		}},
		syncHasMore: true,
	}
	service := NewService(repository, Config{
		Enabled:         true,
		Mounts:          []MountConfig{{Path: "/p3", Kind: domain.ExternalAssetKindNASLocal}},
		SyncExportRoots: []string{"/p3/仓库素材区/徐凯"},
	}, nil)
	head, appErr := service.ExternalCurrentSyncHead(context.Background())
	if appErr != nil || head.Cursor == "" {
		t.Fatalf("head=%+v appErr=%+v", head, appErr)
	}
	decoded, err := decodeExternalCurrentCursor(head.Cursor)
	if err != nil || decoded.ID != 99 || !decoded.UpdatedAt.Equal(updated) {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
	changes, appErr := service.ExternalCurrentSyncChanges(context.Background(), head.Cursor, 100, 0)
	if appErr != nil || len(changes.Items) != 1 || !changes.HasMore {
		t.Fatalf("changes=%+v appErr=%+v", changes, appErr)
	}
	next, err := decodeExternalCurrentCursor(changes.NextCursor)
	if err != nil || next.ID != 100 || !next.UpdatedAt.Equal(updated.Add(time.Second)) {
		t.Fatalf("next=%+v err=%v", next, err)
	}
}
