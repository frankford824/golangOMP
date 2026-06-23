package service

import (
	"context"
	"regexp"
	"sync"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestTaskServiceCreateNewProductUsesDefaultProductCodeRule(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		BusinessLane:        domain.TaskBusinessLaneNormal,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		CategoryCode:        "KT_STANDARD",
		ProductNameSnapshot: "KT Item",
		ProductShortName:    "KT",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "Al",
		DesignRequirement:   "new design",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.SKUCode != "CGK000000" {
		t.Fatalf("Create() sku_code=%s, want CGK000000", task.SKUCode)
	}

	task2, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		CategoryCode:        "K-T-standard",
		ProductNameSnapshot: "KT Item 2",
		ProductShortName:    "KT2",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "Al",
		DesignRequirement:   "new design 2",
	})
	if appErr != nil {
		t.Fatalf("Create() second unexpected error: %+v", appErr)
	}
	if task2.SKUCode != "CGK000001" {
		t.Fatalf("Create() second sku_code=%s, want CGK000001", task2.SKUCode)
	}
}

func TestTaskServiceCreatePurchaseTaskUsesDefaultProductCodeRule(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypePurchaseTask,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		CategoryCode:        "KT_STANDARD",
		ProductNameSnapshot: "Purchase KT",
		CostPriceMode:       string(domain.CostPriceModeTemplate),
		Quantity:            int64Ptr(100),
		BaseSalePrice:       float64Ptr(12.5),
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.SKUCode != "CGK000000" {
		t.Fatalf("Create() sku_code=%s, want CGK000000", task.SKUCode)
	}
}

func TestTaskServiceCreateCustomizationPurchaseKeepsProcurementFlowAndDZ(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	procurementRepo := &prdProcurementRepo{}
	svc := NewTaskService(
		taskRepo,
		procurementRepo,
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:              domain.TaskTypePurchaseTask,
		SourceMode:            domain.TaskSourceModeNewProduct,
		BusinessLane:          domain.TaskBusinessLaneCustomization,
		CreatorID:             9,
		OwnerTeam:             domain.AllValidTeams()[0],
		DeadlineAt:            timePtr(),
		CategoryCode:          "KT_STANDARD",
		ProductNameSnapshot:   "Custom Purchase KT",
		CostPriceMode:         string(domain.CostPriceModeTemplate),
		Quantity:              int64Ptr(100),
		BaseSalePrice:         float64Ptr(12.5),
		CustomizationRequired: true,
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.SKUCode != "DZK000000" {
		t.Fatalf("Create() sku_code=%s, want DZK000000", task.SKUCode)
	}
	if task.TaskStatus != domain.TaskStatusPendingAssign {
		t.Fatalf("Create() task_status=%s, want %s", task.TaskStatus, domain.TaskStatusPendingAssign)
	}
	if task.BusinessLane != domain.TaskBusinessLaneCustomization {
		t.Fatalf("Create() business_lane=%s, want customization", task.BusinessLane)
	}
	if got := procurementRepo.records[task.ID]; got == nil {
		t.Fatal("Create() did not initialize procurement record")
	}
}

func TestTaskServiceCreateCustomizationSKUUsesDZRule(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		BusinessLane:        domain.TaskBusinessLaneCustomization,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		CategoryCode:        "KT_STANDARD",
		ProductNameSnapshot: "Custom KT",
		ProductShortName:    "KT",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "Al",
		DesignRequirement:   "custom design",
		SKUCodeType:         domain.TaskSKUCodeTypeCustomization,
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.SKUCode != "DZK000000" {
		t.Fatalf("Create() sku_code=%s, want DZK000000", task.SKUCode)
	}
}

func TestTaskServiceCustomizationLaneDefaultsSKUToDZ(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithTaskCustomizationJobRepo(newCustomizationFlowJobRepo()),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:              domain.TaskTypeNewProductDevelopment,
		SourceMode:            domain.TaskSourceModeNewProduct,
		CreatorID:             9,
		OwnerTeam:             domain.AllValidTeams()[0],
		DeadlineAt:            timePtr(),
		CategoryCode:          "KT_STANDARD",
		ProductNameSnapshot:   "Lane Custom KT",
		ProductShortName:      "KT",
		MaterialMode:          string(domain.MaterialModePreset),
		Material:              "Al",
		DesignRequirement:     "custom design",
		CustomizationRequired: true,
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.SKUCode != "DZK000000" {
		t.Fatalf("Create() sku_code=%s, want DZK000000", task.SKUCode)
	}
}

func TestTaskServiceCreateCustomizationBatchNewProductUsesDZForAllItems(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
		WithTaskCustomizationJobRepo(newCustomizationFlowJobRepo()),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:              domain.TaskTypeNewProductDevelopment,
		SourceMode:            domain.TaskSourceModeNewProduct,
		BusinessLane:          domain.TaskBusinessLaneCustomization,
		CreatorID:             9,
		OwnerTeam:             domain.AllValidTeams()[0],
		DeadlineAt:            timePtr(),
		BatchSKUMode:          "multiple",
		CustomizationRequired: true,
		BatchItems: []CreateTaskBatchSKUItemParams{
			{
				ProductName:       "Custom Batch A",
				CategoryCode:      "KT_STANDARD",
				DesignRequirement: "custom design A",
			},
			{
				ProductName:       "Custom Batch B",
				CategoryCode:      "KT_STANDARD",
				DesignRequirement: "custom design B",
			},
		},
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	items, err := taskRepo.ListSKUItemsByTaskID(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ListSKUItemsByTaskID() err = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("task_sku_items len=%d, want 2", len(items))
	}
	for idx, item := range items {
		if matched, _ := regexp.MatchString(`^DZK00000[0-1]$`, item.SKUCode); !matched {
			t.Fatalf("item[%d].sku_code=%s, want DZK000000/DZK000001", idx, item.SKUCode)
		}
		if item.SKUCodeType != domain.TaskSKUCodeTypeCustomization {
			t.Fatalf("item[%d].sku_code_type=%s, want customization", idx, item.SKUCodeType)
		}
	}
}

func TestTaskServiceCreateRetouchTaskDoesNotAllocateSKU(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeRetouchTask,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		ProductNameSnapshot: "Retouch only",
		DesignRequirement:   "retouch image",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.SKUCode != "" {
		t.Fatalf("Create() sku_code=%q, want empty for retouch_task", task.SKUCode)
	}
	if task.PrimarySKUCode != "" {
		t.Fatalf("Create() primary_sku_code=%q, want empty for retouch_task", task.PrimarySKUCode)
	}
	if task.SKUGenerationStatus != domain.TaskSKUGenerationStatusNotApplicable {
		t.Fatalf("Create() sku_generation_status=%s, want not_applicable", task.SKUGenerationStatus)
	}
}

func TestTaskServicePrepareProductCodesBatchAndConcurrentUnique(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
	)
	prepareSvc, ok := svc.(TaskProductCodePrepareService)
	if !ok {
		t.Fatal("task service does not implement TaskProductCodePrepareService")
	}

	batchResult, appErr := prepareSvc.PrepareProductCodes(context.Background(), PrepareTaskProductCodesParams{
		TaskType: domain.TaskTypeNewProductDevelopment,
		BatchItems: []PrepareTaskProductCodeBatchItemParams{
			{CategoryCode: "KT_STANDARD"},
			{CategoryCode: "KT_STANDARD"},
			{CategoryCode: "AB"},
		},
	})
	if appErr != nil {
		t.Fatalf("PrepareProductCodes(batch) unexpected error: %+v", appErr)
	}
	if len(batchResult.Codes) != 3 {
		t.Fatalf("PrepareProductCodes(batch) len=%d, want 3", len(batchResult.Codes))
	}
	if batchResult.Codes[0].SKUCode != "CGK000000" || batchResult.Codes[1].SKUCode != "CGK000001" || batchResult.Codes[2].SKUCode != "CGA000000" {
		t.Fatalf("PrepareProductCodes(batch) codes=%+v", batchResult.Codes)
	}

	customPurchase, appErr := prepareSvc.PrepareProductCodes(context.Background(), PrepareTaskProductCodesParams{
		TaskType:     domain.TaskTypePurchaseTask,
		BusinessLane: domain.TaskBusinessLaneCustomization,
		CategoryCode: "KT_STANDARD",
		Count:        1,
	})
	if appErr != nil {
		t.Fatalf("PrepareProductCodes(custom purchase) unexpected error: %+v", appErr)
	}
	if len(customPurchase.Codes) != 1 || customPurchase.Codes[0].SKUCode != "DZK000000" {
		t.Fatalf("PrepareProductCodes(custom purchase) codes=%+v, want DZK000000", customPurchase.Codes)
	}

	const goroutines = 30
	const perRequest = 4
	var wg sync.WaitGroup
	codesCh := make(chan string, goroutines*perRequest)
	errCh := make(chan *domain.AppError, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, appErr := prepareSvc.PrepareProductCodes(context.Background(), PrepareTaskProductCodesParams{
				TaskType:     domain.TaskTypePurchaseTask,
				CategoryCode: "KT_STANDARD",
				Count:        perRequest,
			})
			if appErr != nil {
				errCh <- appErr
				return
			}
			for _, item := range result.Codes {
				codesCh <- item.SKUCode
			}
		}()
	}
	wg.Wait()
	close(codesCh)
	close(errCh)
	for appErr := range errCh {
		t.Fatalf("PrepareProductCodes(concurrent) unexpected error: %+v", appErr)
	}

	seen := make(map[string]struct{}, goroutines*perRequest)
	for code := range codesCh {
		if _, exists := seen[code]; exists {
			t.Fatalf("duplicate prepared product code: %s", code)
		}
		seen[code] = struct{}{}
	}
	if len(seen) != goroutines*perRequest {
		t.Fatalf("prepared code count=%d, want %d", len(seen), goroutines*perRequest)
	}
}

func TestDefaultTaskProductCategoryShortCodeRules(t *testing.T) {
	t.Run("explicit_map", func(t *testing.T) {
		code, appErr := deriveDefaultTaskProductCategoryShortCode("KT_STANDARD")
		if appErr != nil {
			t.Fatalf("derive short code error: %+v", appErr)
		}
		if code != "K" {
			t.Fatalf("short code=%s, want K", code)
		}
	})

	t.Run("extract_first_two_letters", func(t *testing.T) {
		cases := map[string]string{
			"kt_standard":  "K",
			"K-T-standard": "K",
			"A1B2":         "A",
		}
		for input, want := range cases {
			got, appErr := deriveDefaultTaskProductCategoryShortCode(input)
			if appErr != nil {
				t.Fatalf("%s derive short code error: %+v", input, appErr)
			}
			if got != want {
				t.Fatalf("%s short code=%s, want %s", input, got, want)
			}
		}
	})

	t.Run("stable_fallback", func(t *testing.T) {
		first, appErr := deriveDefaultTaskProductCategoryShortCode("1")
		if appErr != nil {
			t.Fatalf("derive first fallback error: %+v", appErr)
		}
		second, appErr := deriveDefaultTaskProductCategoryShortCode("1")
		if appErr != nil {
			t.Fatalf("derive second fallback error: %+v", appErr)
		}
		if first != second {
			t.Fatalf("fallback not stable: first=%s second=%s", first, second)
		}
		if !regexp.MustCompile(`^[A-Z]$`).MatchString(first) {
			t.Fatalf("fallback short code=%s, want ^[A-Z]$", first)
		}
	})
}

func TestTaskServiceDefaultProductCodesFollowRegex(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
	)
	prepareSvc, ok := svc.(TaskProductCodePrepareService)
	if !ok {
		t.Fatal("task service does not implement TaskProductCodePrepareService")
	}
	result, appErr := prepareSvc.PrepareProductCodes(context.Background(), PrepareTaskProductCodesParams{
		TaskType:     domain.TaskTypeNewProductDevelopment,
		CategoryCode: "KT_STANDARD",
		Count:        3,
	})
	if appErr != nil {
		t.Fatalf("PrepareProductCodes unexpected error: %+v", appErr)
	}
	pattern := regexp.MustCompile(`^CG[A-Z][0-9]{6}$`)
	for _, item := range result.Codes {
		if !pattern.MatchString(item.SKUCode) {
			t.Fatalf("sku_code=%s, want %s", item.SKUCode, pattern.String())
		}
	}
}

func TestTaskServicePrepareAndCreateUseSameShortCodeSequence(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		productCodeTestTxRunner{},
		WithTaskProductCodeSequenceRepo(newProductCodeSequenceRepoStub()),
	)
	prepareSvc, ok := svc.(TaskProductCodePrepareService)
	if !ok {
		t.Fatal("task service does not implement TaskProductCodePrepareService")
	}

	prepared, appErr := prepareSvc.PrepareProductCodes(context.Background(), PrepareTaskProductCodesParams{
		TaskType:     domain.TaskTypeNewProductDevelopment,
		CategoryCode: "KT_STANDARD",
		Count:        1,
	})
	if appErr != nil {
		t.Fatalf("PrepareProductCodes unexpected error: %+v", appErr)
	}
	if len(prepared.Codes) != 1 || prepared.Codes[0].SKUCode != "CGK000000" {
		t.Fatalf("prepared codes=%+v", prepared.Codes)
	}

	created, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		CategoryCode:        "K-T-standard",
		ProductNameSnapshot: "KT Item",
		ProductShortName:    "KT",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "Al",
		DesignRequirement:   "new design",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if created.SKUCode != "CGK000001" {
		t.Fatalf("Create() sku_code=%s, want CGK000001", created.SKUCode)
	}
}

type productCodeTestTxRunner struct{}

func (productCodeTestTxRunner) RunInTx(_ context.Context, fn func(tx repo.Tx) error) error {
	return fn(productCodeTestTx{})
}

type productCodeTestTx struct{}

func (productCodeTestTx) IsTx() {}

type productCodeSequenceRepoStub struct {
	mu   sync.Mutex
	next map[string]int64
}

func newProductCodeSequenceRepoStub() *productCodeSequenceRepoStub {
	return &productCodeSequenceRepoStub{next: map[string]int64{}}
}

func (s *productCodeSequenceRepoStub) AllocateRange(_ context.Context, _ repo.Tx, prefix, categoryCode string, count int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := prefix + "|" + categoryCode
	start := s.next[key]
	s.next[key] += int64(count)
	return start, nil
}
