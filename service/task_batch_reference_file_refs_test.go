package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func TestMergeBatchItemReferenceFileRefsIntoTask(t *testing.T) {
	t.Run("multiple merges and dedupes", func(t *testing.T) {
		p := CreateTaskParams{
			BatchSKUMode:      "multiple",
			ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: "top"}},
			BatchItems: []CreateTaskBatchSKUItemParams{
				{ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: "top"}, {AssetID: "item-a"}}},
				{ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: "item-b"}}},
			},
		}
		mergeBatchItemReferenceFileRefsIntoTask(&p)
		if len(p.ReferenceFileRefs) != 3 {
			t.Fatalf("merged len = %d, want 3: %+v", len(p.ReferenceFileRefs), p.ReferenceFileRefs)
		}
		seen := map[string]bool{}
		for _, r := range p.ReferenceFileRefs {
			seen[r.AssetID] = true
		}
		for _, id := range []string{"top", "item-a", "item-b"} {
			if !seen[id] {
				t.Fatalf("missing asset_id %q in %+v", id, p.ReferenceFileRefs)
			}
		}
	})

	t.Run("single mode does not merge item refs", func(t *testing.T) {
		p := CreateTaskParams{
			BatchSKUMode:      "single",
			ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: "only-top"}},
			BatchItems: []CreateTaskBatchSKUItemParams{
				{ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: "ignored"}}},
			},
		}
		mergeBatchItemReferenceFileRefsIntoTask(&p)
		if len(p.ReferenceFileRefs) != 1 || p.ReferenceFileRefs[0].AssetID != "only-top" {
			t.Fatalf("refs = %+v, want single only-top", p.ReferenceFileRefs)
		}
	})
}

func TestTaskServiceCreateBatchMergesItemLevelReferenceFileRefsWithValidation(t *testing.T) {
	uploadRequestRepo := newStep37UploadRequestRepo()
	assetStorageRefRepo := newStep37AssetStorageRefRepo()
	refUpload := NewTaskCreateReferenceUploadService(uploadRequestRepo, assetStorageRefRepo, step04TxRunner{}, newStubUploadServiceClient()).(*taskCreateReferenceUploadService)
	refUpload.nowFn = func() time.Time { return time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC) }

	completeRef := func(filename string) string {
		t.Helper()
		createResult, appErr := refUpload.CreateUploadSession(context.Background(), CreateTaskReferenceUploadSessionParams{
			CreatedBy:    9,
			Filename:     filename,
			ExpectedSize: uploadRequestInt64Ptr(1024),
			MimeType:     "image/png",
			FileHash:     "hash-" + filename,
		})
		if appErr != nil {
			t.Fatalf("CreateUploadSession: %+v", appErr)
		}
		refUpload.nowFn = func() time.Time { return time.Date(2026, 4, 2, 12, 1, 0, 0, time.UTC) }
		completeResult, appErr := refUpload.CompleteUploadSession(context.Background(), CompleteTaskReferenceUploadSessionParams{
			SessionID:   createResult.Session.ID,
			CompletedBy: 9,
			FileHash:    "hash-" + filename,
		})
		if appErr != nil {
			t.Fatalf("CompleteUploadSession: %+v", appErr)
		}
		return completeResult.ReferenceFileRef
	}

	refA := completeRef("batch-line-a.png")
	refB := completeRef("batch-line-b.png")

	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskReferenceFileRefValidation(uploadRequestRepo, assetStorageRefRepo),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:     domain.TaskTypeNewProductDevelopment,
		SourceMode:   domain.TaskSourceModeNewProduct,
		CreatorID:    9,
		OwnerTeam:    domain.AllValidTeams()[0],
		DeadlineAt:   referenceImageTestTimePtr(),
		BatchSKUMode: "multiple",
		BatchItems: []CreateTaskBatchSKUItemParams{
			{
				ProductName:       "Batch Ref A",
				ProductShortName:  "BRA",
				CategoryCode:      "CAT",
				MaterialMode:      string(domain.MaterialModePreset),
				DesignRequirement: "d1",
				NewSKU:            "BATCH-REF-A",
				ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: refA}},
			},
			{
				ProductName:       "Batch Ref B",
				ProductShortName:  "BRB",
				CategoryCode:      "CAT",
				MaterialMode:      string(domain.MaterialModePreset),
				DesignRequirement: "d2",
				NewSKU:            "BATCH-REF-B",
				ReferenceFileRefs: []domain.ReferenceFileRef{{AssetID: refB}},
			},
		},
	})
	if appErr != nil {
		t.Fatalf("Create: %+v", appErr)
	}
	detail := taskRepo.details[task.ID]
	if detail == nil {
		t.Fatal("detail missing")
	}
	var persisted []domain.ReferenceFileRef
	if err := json.Unmarshal([]byte(detail.ReferenceFileRefsJSON), &persisted); err != nil {
		t.Fatalf("unmarshal reference_file_refs_json: %v", err)
	}
	if len(persisted) != 2 {
		t.Fatalf("persisted refs len = %d, want 2: %+v", len(persisted), persisted)
	}

	readModel, appErr := svc.GetByID(context.Background(), task.ID)
	if appErr != nil {
		t.Fatalf("GetByID: %+v", appErr)
	}
	if len(readModel.ReferenceFileRefs) != 2 {
		t.Fatalf("read model reference_file_refs len = %d, want 2: %+v", len(readModel.ReferenceFileRefs), readModel.ReferenceFileRefs)
	}
	if readModel.ReferenceFileRefs == nil {
		t.Fatal("readModel.ReferenceFileRefs is nil, want non-nil slice")
	}
	if len(readModel.SKUItems) != 2 {
		t.Fatalf("readModel sku_items len = %d, want 2", len(readModel.SKUItems))
	}
	if len(readModel.SKUItems[0].ReferenceFileRefs) != 1 || readModel.SKUItems[0].ReferenceFileRefs[0].AssetID != refA {
		t.Fatalf("sku_items[0].reference_file_refs = %+v, want %q", readModel.SKUItems[0].ReferenceFileRefs, refA)
	}
	if len(readModel.SKUItems[1].ReferenceFileRefs) != 1 || readModel.SKUItems[1].ReferenceFileRefs[0].AssetID != refB {
		t.Fatalf("sku_items[1].reference_file_refs = %+v, want %q", readModel.SKUItems[1].ReferenceFileRefs, refB)
	}
}

func TestTaskReadModelReferenceFileRefsAlwaysSlice(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			902: {
				ID:         902,
				TaskNo:     "T-902",
				TaskType:   domain.TaskTypeNewProductDevelopment,
				TaskStatus: domain.TaskStatusPendingAssign,
				IsBatchTask: true,
				BatchMode:   domain.TaskBatchModeMultiSKU,
			},
		},
		details: map[int64]*domain.TaskDetail{
			902: {
				TaskID:                902,
				ReferenceFileRefsJSON: `[]`,
				ReferenceImagesJSON:   `[]`,
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			902: {
				{TaskID: 902, SequenceNo: 1, SKUCode: "S1", ReferenceFileRefs: []domain.ReferenceFileRef{}},
				{TaskID: 902, SequenceNo: 2, SKUCode: "S2", ReferenceFileRefs: []domain.ReferenceFileRef{}},
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)
	rm, appErr := svc.GetByID(context.Background(), 902)
	if appErr != nil {
		t.Fatalf("GetByID: %+v", appErr)
	}
	if rm.ReferenceFileRefs == nil {
		t.Fatal("ReferenceFileRefs = nil, want empty slice")
	}
	if len(rm.ReferenceFileRefs) != 0 {
		t.Fatalf("len = %d, want 0", len(rm.ReferenceFileRefs))
	}
	raw, err := json.Marshal(rm)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"reference_file_refs"`) {
		t.Fatalf("JSON missing reference_file_refs key: %s", string(raw))
	}
}
