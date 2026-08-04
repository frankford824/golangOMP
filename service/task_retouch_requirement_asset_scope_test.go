package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
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

func TestEnrichRetouchRequirementsReadModelHydratesFlatStorageRef(t *testing.T) {
	reqID := int64(7)
	size := int64(192104)
	now := time.Now().UTC()
	requirements := []domain.TaskRetouchRequirement{
		{
			ID: reqID, TaskID: 100, Description: "需求一",
			SortOrder: 1, CreatedAt: now, UpdatedAt: now,
		},
	}
	flatRefs := []*domain.ReferenceFileRefFlat{
		{
			RefID:                "formal-ref",
			RetouchRequirementID: &reqID,
			StorageKey:           "tasks/T100/reference/photo.jpg",
			FileName:             "photo.jpg",
			MimeType:             "image/jpeg",
			FileSize:             &size,
			StorageStatus:        "recorded",
		},
	}

	enriched := EnrichRetouchRequirementsReadModel(
		context.Background(), requirements, flatRefs, nil, nil,
	)
	if len(enriched) != 1 || len(enriched[0].ReferenceFileRefs) != 1 {
		t.Fatalf("enriched = %+v", enriched)
	}
	ref := enriched[0].ReferenceFileRefs[0]
	if ref.RefID != "formal-ref" || ref.AssetID != "formal-ref" {
		t.Fatalf("identity = %+v", ref)
	}
	if ref.Filename != "photo.jpg" || ref.MimeType != "image/jpeg" {
		t.Fatalf("metadata = %+v", ref)
	}
	if ref.FileSize == nil || *ref.FileSize != size {
		t.Fatalf("file_size = %+v", ref.FileSize)
	}
	wantURL := "/v1/assets/files/tasks/T100/reference/photo.jpg"
	if ref.DownloadURL == nil || *ref.DownloadURL != wantURL {
		t.Fatalf("download_url = %+v, want %q", ref.DownloadURL, wantURL)
	}
	if ref.Source != domain.ReferenceFileRefSourceTaskCreateAssetCenter ||
		ref.Status != domain.ReferenceFileRefStatusUploaded {
		t.Fatalf("source/status = %+v", ref)
	}
}

func TestEnrichRetouchRequirementsReadModelDoesNotPublishUnavailableFlatStorageRef(t *testing.T) {
	reqID := int64(8)
	requirements := []domain.TaskRetouchRequirement{
		{ID: reqID, TaskID: 100, Description: "historical", SortOrder: 1},
	}
	flatRefs := []*domain.ReferenceFileRefFlat{
		{
			RefID:                "historical-ref",
			RetouchRequirementID: &reqID,
			StorageKey:           "tasks/T100/reference/historical.jpg",
			FileName:             "historical.jpg",
			MimeType:             "image/jpeg",
			StorageStatus:        string(domain.AssetStorageRefStatusHistoricalUnavailable),
		},
	}

	enriched := EnrichRetouchRequirementsReadModel(
		context.Background(), requirements, flatRefs, nil, nil,
	)
	ref := enriched[0].ReferenceFileRefs[0]
	if ref.DownloadURL != nil || ref.URL != nil || ref.StorageKey != "" {
		t.Fatalf("unavailable ref exposes object access: %+v", ref)
	}
	if ref.Status == domain.ReferenceFileRefStatusUploaded {
		t.Fatalf("unavailable ref is marked uploaded: %+v", ref)
	}
}

type retouchReadModelFlatRepoStub struct {
	refs []*domain.ReferenceFileRefFlat
}

func (r *retouchReadModelFlatRepoStub) InsertFlat(context.Context, repo.Tx, *domain.ReferenceFileRefFlat) (int64, error) {
	return 0, nil
}

func (r *retouchReadModelFlatRepoStub) ListByTask(context.Context, int64) ([]*domain.ReferenceFileRefFlat, error) {
	return append([]*domain.ReferenceFileRefFlat{}, r.refs...), nil
}

func (r *retouchReadModelFlatRepoStub) DeleteByTaskAndRef(context.Context, repo.Tx, int64, string) error {
	return nil
}

func TestTaskReadModelUsesFormalRetouchAssetsForRequirementProjection(t *testing.T) {
	taskID := int64(2807)
	requirementID := int64(223)
	referenceRootID := int64(22096)
	sourceRootID := int64(22097)
	referenceVersionID := int64(23639)
	sourceVersionID := int64(23640)
	assetVersionNo := 1
	now := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	referenceStorageKey := "tasks/RW-2807/assets/AST-0001/v1/reference.png"
	sourceStorageKey := "tasks/RW-2807/assets/AST-0002/v1/source.psd"

	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			taskID: {
				ID:         taskID,
				TaskType:   domain.TaskTypeRetouchTask,
				TaskStatus: domain.TaskStatusCompleted,
				CreatorID:  1,
			},
		},
		details: map[int64]*domain.TaskDetail{
			taskID: {TaskID: taskID},
		},
	}
	designRepo := &designAssetReadModelStub{
		assetsByTask: map[int64][]*domain.DesignAsset{
			taskID: {
				{
					ID:                   referenceRootID,
					TaskID:               taskID,
					AssetNo:              "AST-0001",
					RetouchRequirementID: &requirementID,
					AssetType:            domain.TaskAssetTypeReference,
					CurrentVersionID:     &referenceVersionID,
				},
				{
					ID:                   sourceRootID,
					TaskID:               taskID,
					AssetNo:              "AST-0002",
					RetouchRequirementID: &requirementID,
					AssetType:            domain.TaskAssetTypeSource,
					CurrentVersionID:     &sourceVersionID,
				},
			},
		},
	}
	taskAssetRepo := &taskAssetReadModelStub{
		recordsByTask: map[int64][]*domain.TaskAsset{
			taskID: {
				{
					ID:                   referenceVersionID,
					TaskID:               taskID,
					AssetID:              &referenceRootID,
					RetouchRequirementID: &requirementID,
					AssetType:            domain.TaskAssetTypeReference,
					VersionNo:            1,
					AssetVersionNo:       &assetVersionNo,
					FileName:             "reference.png",
					StorageKey:           &referenceStorageKey,
					UploadedBy:           1,
					CreatedAt:            now,
				},
				{
					ID:                   sourceVersionID,
					TaskID:               taskID,
					AssetID:              &sourceRootID,
					RetouchRequirementID: &requirementID,
					AssetType:            domain.TaskAssetTypeSource,
					VersionNo:            2,
					AssetVersionNo:       &assetVersionNo,
					FileName:             "source.psd",
					StorageKey:           &sourceStorageKey,
					UploadedBy:           1,
					CreatedAt:            now,
				},
			},
		},
	}
	requirementRepo := &retouchRequirementRepoStub{
		byTask: map[int64][]*domain.TaskRetouchRequirement{
			taskID: {
				{
					ID:          requirementID,
					TaskID:      taskID,
					Description: "retouch",
					SortOrder:   1,
				},
			},
		},
	}
	flatRepo := &retouchReadModelFlatRepoStub{
		refs: []*domain.ReferenceFileRefFlat{
			{
				TaskID:               taskID,
				RetouchRequirementID: &requirementID,
				RefID:                "legacy-storage-uuid",
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		taskAssetRepo,
		&prdTaskEventRepo{},
		nil,
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskRetouchRequirementRepo(requirementRepo),
		WithTaskReferenceFileRefFlatRepo(flatRepo),
		WithTaskDesignAssetReadModel(designRepo),
	)

	readModel, appErr := svc.GetByID(context.Background(), taskID)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if len(readModel.RetouchRequirements) != 1 {
		t.Fatalf("retouch_requirements len = %d, want 1", len(readModel.RetouchRequirements))
	}
	requirement := readModel.RetouchRequirements[0]
	if len(requirement.ReferenceFileRefs) != 1 || requirement.ReferenceFileRefs[0].RefID != "22096" {
		t.Fatalf("reference_file_refs = %#v, want formal root 22096", requirement.ReferenceFileRefs)
	}
	if len(requirement.SourceAssets) != 1 || requirement.SourceAssets[0].ID != sourceRootID {
		t.Fatalf("source_assets = %#v, want formal source root %d", requirement.SourceAssets, sourceRootID)
	}
}
