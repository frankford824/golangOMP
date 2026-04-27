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
	if task.SKUCode != "NSKT000000" {
		t.Fatalf("Create() sku_code=%s, want NSKT000000", task.SKUCode)
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
	if task2.SKUCode != "NSKT000001" {
		t.Fatalf("Create() second sku_code=%s, want NSKT000001", task2.SKUCode)
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
	if task.SKUCode != "NSKT000000" {
		t.Fatalf("Create() sku_code=%s, want NSKT000000", task.SKUCode)
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
	if batchResult.Codes[0].SKUCode != "NSKT000000" || batchResult.Codes[1].SKUCode != "NSKT000001" || batchResult.Codes[2].SKUCode != "NSAB000000" {
		t.Fatalf("PrepareProductCodes(batch) codes=%+v", batchResult.Codes)
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
		if code != "KT" {
			t.Fatalf("short code=%s, want KT", code)
		}
	})

	t.Run("extract_first_two_letters", func(t *testing.T) {
		cases := map[string]string{
			"kt_standard":  "KT",
			"K-T-standard": "KT",
			"A1B2":         "AB",
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
		if !regexp.MustCompile(`^[A-Z]{2}$`).MatchString(first) {
			t.Fatalf("fallback short code=%s, want ^[A-Z]{2}$", first)
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
	pattern := regexp.MustCompile(`^NS[A-Z]{2}[0-9]{6}$`)
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
	if len(prepared.Codes) != 1 || prepared.Codes[0].SKUCode != "NSKT000000" {
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
	if created.SKUCode != "NSKT000001" {
		t.Fatalf("Create() sku_code=%s, want NSKT000001", created.SKUCode)
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
