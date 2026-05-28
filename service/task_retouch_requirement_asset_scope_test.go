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
	enriched := EnrichRetouchRequirementsReadModel(context.Background(), requirements, flatRefs, designAssets, nil)
	if len(enriched) != 1 {
		t.Fatalf("len = %d, want 1", len(enriched))
	}
	if len(enriched[0].ReferenceFileRefs) != 1 || enriched[0].ReferenceFileRefs[0].AssetID != "ref-req" {
		t.Fatalf("reference_file_refs = %+v", enriched[0].ReferenceFileRefs)
	}
	if len(enriched[0].SourceAssets) != 1 || enriched[0].SourceAssets[0].ID != sourceID {
		t.Fatalf("source_assets = %+v", enriched[0].SourceAssets)
	}
}
