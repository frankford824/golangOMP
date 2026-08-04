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
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
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

func TestTaskServiceUpdateSKUItemInfoRefreshesBatchTaskReferenceSummary(t *testing.T) {
	topRef := domain.ReferenceFileRef{AssetID: "task-top", RefID: "task-top", Filename: "task-top.png"}
	oldA := domain.ReferenceFileRef{AssetID: "old-a", RefID: "old-a", Filename: "old-a.png"}
	newA := domain.ReferenceFileRef{AssetID: "new-a", RefID: "new-a", Filename: "new-a.png"}
	itemB := domain.ReferenceFileRef{AssetID: "item-b", RefID: "item-b", Filename: "item-b.png"}
	detailRefsJSON, err := json.Marshal([]domain.ReferenceFileRef{topRef, oldA, itemB})
	if err != nil {
		t.Fatalf("marshal detail refs: %v", err)
	}

	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			950: {
				ID:             950,
				TaskNo:         "RW-BATCH-REF-950",
				TaskType:       domain.TaskTypeNewProductDevelopment,
				TaskStatus:     domain.TaskStatusPendingAuditA,
				IsBatchTask:    true,
				BatchItemCount: 2,
				BatchMode:      domain.TaskBatchModeMultiSKU,
			},
		},
		details: map[int64]*domain.TaskDetail{
			950: {
				TaskID:                950,
				ReferenceFileRefsJSON: string(detailRefsJSON),
			},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			950: {
				{
					ID:                  951,
					TaskID:              950,
					SequenceNo:          1,
					SKUCode:             "SKU-A",
					ProductNameSnapshot: "商品 A",
					DesignRequirement:   "旧需求",
					ReferenceFileRefs:   []domain.ReferenceFileRef{oldA},
				},
				{
					ID:                  952,
					TaskID:              950,
					SequenceNo:          2,
					SKUCode:             "SKU-B",
					ProductNameSnapshot: "商品 B",
					DesignRequirement:   "需求 B",
					ReferenceFileRefs:   []domain.ReferenceFileRef{itemB},
				},
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		prdCodeRuleService{},
		step04TxRunner{},
	)

	_, appErr := svc.UpdateSKUItemInfo(context.Background(), UpdateTaskSKUItemInfoParams{
		TaskID:               950,
		SKUItemID:            951,
		OperatorID:           9,
		ReferenceFileRefs:    []domain.ReferenceFileRef{newA},
		ReferenceFileRefsSet: true,
		Remark:               "修正子项参考图",
	})
	if appErr != nil {
		t.Fatalf("UpdateSKUItemInfo() unexpected error: %+v", appErr)
	}

	summary := domain.ParseReferenceFileRefsJSON(taskRepo.details[950].ReferenceFileRefsJSON)
	seen := map[string]bool{}
	for _, ref := range summary {
		seen[ref.CanonicalID()] = true
	}
	for _, want := range []string{"task-top", "new-a", "item-b"} {
		if !seen[want] {
			t.Fatalf("summary missing %q: %+v", want, summary)
		}
	}
	if seen["old-a"] {
		t.Fatalf("summary still contains removed old item ref: %+v", summary)
	}
	if got := taskRepo.skuItems[950][0].ReferenceFileRefs; len(got) != 1 || got[0].CanonicalID() != "new-a" {
		t.Fatalf("sku item refs = %+v, want new-a", got)
	}
}

func TestTaskReadModelReferenceFileRefsAlwaysSlice(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			902: {
				ID:          902,
				TaskNo:      "T-902",
				TaskType:    domain.TaskTypeNewProductDevelopment,
				TaskStatus:  domain.TaskStatusPendingAssign,
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
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
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
