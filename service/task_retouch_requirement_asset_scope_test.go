package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func TestValidateRetouchRequirementAssetScopeRejectsNonRetouchTask(t *testing.T) {
	reqID := int64(9)
	appErr := validateRetouchRequirementAssetScope(context.Background(), &domain.Task{
		ID:       1,
		TaskType: domain.TaskTypeNewProductDevelopment,
	}, &reqID, &retouchRequirementRepoStub{})
	if appErr == nil {
		t.Fatal("validateRetouchRequirementAssetScope() expected error")
	}
	if !strings.Contains(appErr.Message, "retouch_task") {
		t.Fatalf("message = %q", appErr.Message)
	}
}

func TestRejectConflictingRetouchAssetScopes(t *testing.T) {
	reqID := int64(3)
	appErr := rejectConflictingRetouchAssetScopes("SKU-A", &reqID)
	if appErr == nil {
		t.Fatal("rejectConflictingRetouchAssetScopes() expected error")
	}
}

func TestFilterTaskLevelReferenceFileRefsExcludesRequirementScopedRefs(t *testing.T) {
	reqID := int64(11)
	refs := []domain.ReferenceFileRef{
		{AssetID: "task-ref", RefID: "task-ref"},
		{AssetID: "req-ref", RefID: "req-ref"},
	}
	flatRefs := []*domain.ReferenceFileRefFlat{
		{RefID: "req-ref", RetouchRequirementID: &reqID},
	}
	filtered := FilterTaskLevelReferenceFileRefs(refs, flatRefs)
	if len(filtered) != 1 || filtered[0].AssetID != "task-ref" {
		t.Fatalf("filtered = %+v, want only task-ref", filtered)
	}
}

func TestEnrichRetouchRequirementsReadModelGroupsAssets(t *testing.T) {
	reqID := int64(7)
	now := time.Now().UTC()
	requirements := []domain.TaskRetouchRequirement{
		{ID: reqID, TaskID: 100, Description: "需求一", SortOrder: 1, CreatedAt: now, UpdatedAt: now},
	}
	flatRefs := []*domain.ReferenceFileRefFlat{
		{RefID: "ref-req", RetouchRequirementID: &reqID},
	}
	sourceID := int64(501)
	currentVersionID := int64(9001)
	designAssets := []*domain.DesignAsset{
		{
			ID:                   sourceID,
			TaskID:               100,
			RetouchRequirementID: &reqID,
			AssetType:            domain.TaskAssetTypeSource,
			CurrentVersionID:     &currentVersionID,
		},
	}
	refAssetID := int64(601)
	refVersionID := int64(9002)
	downloadURL := "/v1/assets/files/tasks/T100/assets/AST-601/v1/reference/photo.jpg"
	designAssets = append(designAssets, &domain.DesignAsset{
		ID:                   refAssetID,
		TaskID:               100,
		RetouchRequirementID: &reqID,
		AssetType:            domain.TaskAssetTypeReference,
		CurrentVersionID:     &refVersionID,
		CurrentVersion: &domain.DesignAssetVersion{
			ID:               9002,
			AssetID:          refAssetID,
			AssetType:        domain.TaskAssetTypeReference,
			OriginalFilename: "photo.jpg",
			StorageKey:       "tasks/T100/assets/AST-601/v1/reference/photo.jpg",
			DownloadURL:      &downloadURL,
		},
	})
	enriched := EnrichRetouchRequirementsReadModel(context.Background(), requirements, flatRefs, designAssets, nil)
	if len(enriched) != 1 {
		t.Fatalf("len = %d, want 1", len(enriched))
	}
	if len(enriched[0].ReferenceFileRefs) != 1 {
		t.Fatalf("reference_file_refs = %+v", enriched[0].ReferenceFileRefs)
	}
	if enriched[0].ReferenceFileRefs[0].AssetID != "601" {
		t.Fatalf("reference_file_refs asset_id = %q, want design asset id", enriched[0].ReferenceFileRefs[0].AssetID)
	}
	if enriched[0].ReferenceFileRefs[0].DownloadURL == nil || *enriched[0].ReferenceFileRefs[0].DownloadURL != downloadURL {
		t.Fatalf("reference_file_refs download_url = %+v, want %q", enriched[0].ReferenceFileRefs[0].DownloadURL, downloadURL)
	}
	if len(enriched[0].SourceAssets) != 1 || enriched[0].SourceAssets[0].ID != sourceID {
		t.Fatalf("source_assets = %+v", enriched[0].SourceAssets)
	}
}
