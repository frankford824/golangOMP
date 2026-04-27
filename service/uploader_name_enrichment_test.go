package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestDesignAssetVersion_UploaderName_Populated(t *testing.T) {
	resolver := &countingUploaderNameResolver{
		names: map[int64]string{
			101: "Designer A",
			202: "Designer B",
		},
	}
	versions := []*domain.DesignAssetVersion{
		{ID: 1, UploadedBy: 101},
		{ID: 2, UploadedBy: 202},
	}

	enrichDesignAssetVersionUploaderNames(context.Background(), resolver, versions)

	if versions[0].UploadedByName != "Designer A" {
		t.Fatalf("version 1 uploader_name = %q, want Designer A", versions[0].UploadedByName)
	}
	if versions[1].UploadedByName != "Designer B" {
		t.Fatalf("version 2 uploader_name = %q, want Designer B", versions[1].UploadedByName)
	}
}

func TestDesignAssetVersion_UploaderName_UnknownEmpty(t *testing.T) {
	resolver := &countingUploaderNameResolver{names: map[int64]string{}}
	versions := []*domain.DesignAssetVersion{
		{ID: 1, UploadedBy: 303},
	}

	enrichDesignAssetVersionUploaderNames(context.Background(), resolver, versions)

	if versions[0].UploadedByName != "" {
		t.Fatalf("unknown uploader_name = %q, want empty string", versions[0].UploadedByName)
	}
}

func TestUploaderName_ResolvedInBatch(t *testing.T) {
	resolver := &countingUploaderNameResolver{
		names: map[int64]string{404: "Batch Designer"},
	}
	versions := []*domain.DesignAssetVersion{
		{ID: 1, UploadedBy: 404},
		{ID: 2, UploadedBy: 404},
		{ID: 3, UploadedBy: 404},
		{ID: 4, UploadedBy: 0},
	}

	enrichDesignAssetVersionUploaderNames(context.Background(), resolver, versions)

	if resolver.callsByID[404] > 1 {
		t.Fatalf("resolver calls for uploader 404 = %d, want at most 1", resolver.callsByID[404])
	}
	for _, version := range versions[:3] {
		if version.UploadedByName != "Batch Designer" {
			t.Fatalf("version %d uploader_name = %q, want Batch Designer", version.ID, version.UploadedByName)
		}
	}
	if _, ok := resolver.callsByID[0]; ok {
		t.Fatalf("resolver was called for zero uploader id")
	}
}

type countingUploaderNameResolver struct {
	names     map[int64]string
	callsByID map[int64]int
}

func (r *countingUploaderNameResolver) GetDisplayName(_ context.Context, userID int64) string {
	if r.callsByID == nil {
		r.callsByID = map[int64]int{}
	}
	r.callsByID[userID]++
	return r.names[userID]
}
