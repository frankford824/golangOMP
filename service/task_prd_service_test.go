package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"workflow/domain"
	"workflow/repo"
)

// Case2 (service layer): new_product_development with ProductSelection must reject.
func TestTaskServiceCreateRejectsProductSelectionForNewProductDevelopment(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		CategoryCode:        "LIGHTBOX",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "铝型材",
		ProductNameSnapshot: "New Lightbox",
		ProductShortName:    "Lightbox",
		DesignRequirement:   "need design",
		ProductSelection: &domain.TaskProductSelectionContext{
			SelectedProductID:      int64Ptr(88),
			SelectedProductName:    "Existing Product",
			SelectedProductSKUCode: "SKU-088",
		},
	})
	if appErr == nil {
		t.Fatal("Create() expected error for product_selection with new_product_development")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("Create() error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
	if !strings.Contains(appErr.Message, "product_selection is only supported when source_mode is existing_product") {
		t.Fatalf("Create() error message = %q", appErr.Message)
	}
}

func TestTaskServiceCreateRejectsOriginalOnlyFieldForNewProductDevelopment(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		CategoryCode:        "LIGHTBOX",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "铝型材",
		ProductNameSnapshot: "New Lightbox",
		ProductShortName:    "Lightbox",
		DesignRequirement:   "need design",
		ChangeRequest:       "should-not-be-sent-for-new-product",
	})
	if appErr == nil {
		t.Fatal("Create() expected whitelist error for change_request with new_product_development")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("Create() error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
	if !strings.Contains(appErr.Message, "task_type field whitelist validation failed") {
		t.Fatalf("Create() error message = %q", appErr.Message)
	}
}

func TestTaskServiceCreateAllowsCategoryCodeForPurchaseTask(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypePurchaseTask,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		PurchaseSKU:         "PUR-001",
		ProductNameSnapshot: "Accessory Pack",
		CostPriceMode:       string(domain.CostPriceModeTemplate),
		Quantity:            int64Ptr(100),
		BaseSalePrice:       float64Ptr(12.5),
		CategoryCode:        "LIGHTBOX",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task == nil {
		t.Fatal("Create() returned nil task")
	}
}

// Case A: original_product_development + is_outsource (frontend alias for need_outsource).
// Handler normalizes is_outsource/need_outsource -> IsOutsource; whitelist must not reject.
func TestTaskServiceCreateOriginalProductWithIsOutsourceAliasPasses(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	jobRepo := newCustomizationFlowJobRepo()
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskCustomizationJobRepo(jobRepo),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:      domain.TaskTypeOriginalProductDevelopment,
		SourceMode:    domain.TaskSourceModeExistingProduct,
		CreatorID:     9,
		OwnerTeam:     domain.AllValidTeams()[0],
		DeadlineAt:    timePtr(),
		ChangeRequest: "update design",
		ProductID:     int64Ptr(88),
		SKUCode:       "SKU-088",
		IsOutsource:   true,
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v (is_outsource alias should pass whitelist)", appErr)
	}
	if task.NeedOutsource != true {
		t.Fatalf("Create() need_outsource = %v, want true", task.NeedOutsource)
	}
	if !task.CustomizationRequired {
		t.Fatal("Create() customization_required = false, want true for outsource compatibility input")
	}
	if task.TaskStatus != domain.TaskStatusPendingCustomizationReview {
		t.Fatalf("Create() task_status = %s, want PendingCustomizationReview", task.TaskStatus)
	}
	if task.CustomizationSourceType != domain.CustomizationSourceTypeExistingProduct {
		t.Fatalf("Create() customization_source_type = %s, want existing_product", task.CustomizationSourceType)
	}
	if len(jobRepo.jobs) != 1 {
		t.Fatalf("Create() customization jobs = %d, want 1", len(jobRepo.jobs))
	}
}

func TestTaskServiceCreateCustomizationLaneCreatesImmediateJobForNewAndExistingSource(t *testing.T) {
	tests := []struct {
		name                    string
		taskType                domain.TaskType
		sourceMode              domain.TaskSourceMode
		customizationSourceType domain.CustomizationSourceType
		productID               *int64
		skuCode                 string
		productName             string
		changeRequest           string
		categoryCode            string
		materialMode            string
		material                string
		productShortName        string
		designRequirement       string
	}{
		{
			name:                    "existing product customization",
			taskType:                domain.TaskTypeOriginalProductDevelopment,
			sourceMode:              domain.TaskSourceModeExistingProduct,
			customizationSourceType: domain.CustomizationSourceTypeExistingProduct,
			productID:               int64Ptr(88),
			skuCode:                 "SKU-088",
			productName:             "Existing Product",
			changeRequest:           "adjust customization source",
		},
		{
			name:                    "new product customization",
			taskType:                domain.TaskTypeNewProductDevelopment,
			sourceMode:              domain.TaskSourceModeNewProduct,
			customizationSourceType: domain.CustomizationSourceTypeNewProduct,
			productName:             "New Product",
			categoryCode:            "LIGHTBOX",
			materialMode:            string(domain.MaterialModePreset),
			material:                "Aluminum",
			productShortName:        "New Box",
			designRequirement:       "create customization source",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			taskRepo := &prdTaskRepo{}
			jobRepo := newCustomizationFlowJobRepo()
			svc := NewTaskService(
				taskRepo,
				&prdProcurementRepo{},
				&prdTaskAssetRepo{},
				&prdTaskEventRepo{},
				nil,
				&prdWarehouseRepo{},
				prdCodeRuleService{},
				step04TxRunner{},
				WithTaskCustomizationJobRepo(jobRepo),
			)

			task, appErr := svc.Create(context.Background(), CreateTaskParams{
				TaskType:                tc.taskType,
				SourceMode:              tc.sourceMode,
				CreatorID:               9,
				OwnerTeam:               domain.AllValidTeams()[0],
				DeadlineAt:              timePtr(),
				ProductID:               tc.productID,
				SKUCode:                 tc.skuCode,
				ProductNameSnapshot:     tc.productName,
				ChangeRequest:           tc.changeRequest,
				CategoryCode:            tc.categoryCode,
				MaterialMode:            tc.materialMode,
				Material:                tc.material,
				ProductShortName:        tc.productShortName,
				DesignRequirement:       tc.designRequirement,
				CustomizationRequired:   true,
				CustomizationSourceType: tc.customizationSourceType,
			})
			if appErr != nil {
				t.Fatalf("Create() unexpected error: %+v", appErr)
			}
			if task.TaskStatus != domain.TaskStatusPendingCustomizationReview {
				t.Fatalf("task_status = %s, want PendingCustomizationReview", task.TaskStatus)
			}
			if !task.CustomizationRequired {
				t.Fatal("customization_required = false, want true")
			}
			if !task.NeedOutsource {
				t.Fatal("need_outsource = false, want true as derived compatibility marker")
			}
			if task.CustomizationSourceType != tc.customizationSourceType {
				t.Fatalf("customization_source_type = %s, want %s", task.CustomizationSourceType, tc.customizationSourceType)
			}
			if len(jobRepo.jobs) != 1 {
				t.Fatalf("customization jobs = %d, want 1", len(jobRepo.jobs))
			}
			var job *domain.CustomizationJob
			for _, item := range jobRepo.jobs {
				job = item
			}
			if job == nil {
				t.Fatal("customization job = nil")
			}
			if job.TaskID != task.ID {
				t.Fatalf("job.task_id = %d, want %d", job.TaskID, task.ID)
			}
			if job.Status != domain.CustomizationJobStatusPendingCustomizationReview {
				t.Fatalf("job.status = %s, want pending_customization_review", job.Status)
			}
		})
	}
}

func TestTaskServiceCreateNormalLaneDoesNotCreateCustomizationJob(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	jobRepo := newCustomizationFlowJobRepo()
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskCustomizationJobRepo(jobRepo),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:      domain.TaskTypeOriginalProductDevelopment,
		SourceMode:    domain.TaskSourceModeExistingProduct,
		CreatorID:     9,
		OwnerTeam:     domain.AllValidTeams()[0],
		DeadlineAt:    timePtr(),
		ChangeRequest: "normal design lane",
		ProductID:     int64Ptr(88),
		SKUCode:       "SKU-088",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.TaskStatus != domain.TaskStatusPendingAssign {
		t.Fatalf("task_status = %s, want PendingAssign", task.TaskStatus)
	}
	if len(jobRepo.jobs) != 0 {
		t.Fatalf("customization jobs = %d, want 0", len(jobRepo.jobs))
	}
}

func TestTaskServiceCreateCustomizationLaneHasImmediateCustomizationListVisibility(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	jobRepo := newCustomizationFlowJobRepo()
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskCustomizationJobRepo(jobRepo),
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:                domain.TaskTypeOriginalProductDevelopment,
		SourceMode:              domain.TaskSourceModeExistingProduct,
		CreatorID:               9,
		OwnerTeam:               domain.AllValidTeams()[0],
		DeadlineAt:              timePtr(),
		ChangeRequest:           "customization lane immediate list visibility",
		ProductID:               int64Ptr(88),
		SKUCode:                 "SKU-088",
		CustomizationRequired:   true,
		CustomizationSourceType: domain.CustomizationSourceTypeExistingProduct,
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}

	items, pagination, appErr := svc.ListCustomizationJobs(context.Background(), CustomizationJobFilter{
		TaskID:   &task.ID,
		Page:     1,
		PageSize: 20,
	})
	if appErr != nil {
		t.Fatalf("ListCustomizationJobs() unexpected error: %+v", appErr)
	}
	if pagination.Total != 1 {
		t.Fatalf("ListCustomizationJobs() total = %d, want 1", pagination.Total)
	}
	if len(items) != 1 {
		t.Fatalf("ListCustomizationJobs() items = %d, want 1", len(items))
	}
	if items[0].TaskID != task.ID {
		t.Fatalf("ListCustomizationJobs() task_id = %d, want %d", items[0].TaskID, task.ID)
	}
	if items[0].Status != domain.CustomizationJobStatusPendingCustomizationReview {
		t.Fatalf("ListCustomizationJobs() status = %s, want pending_customization_review", items[0].Status)
	}
}

// Case B: original_product_development + product_selection + defer_local_product_binding.
// ERP snapshot path must pass whitelist.
func TestTaskServiceCreateOriginalProductWithProductSelectionDeferBindingPasses(t *testing.T) {
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
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:      domain.TaskTypeOriginalProductDevelopment,
		SourceMode:    domain.TaskSourceModeExistingProduct,
		CreatorID:     9,
		OwnerTeam:     domain.AllValidTeams()[0],
		DeadlineAt:    timePtr(),
		ChangeRequest: "defer binding design change",
		ProductSelection: &domain.TaskProductSelectionContext{
			DeferLocalProductBinding: true,
			ERPProduct: &domain.ERPProductSelectionSnapshot{
				ProductID:   "ERP-001",
				SKUCode:     "HQT02872",
				SKUID:       "SKU-ERP",
				ProductName: "Test Product",
			},
		},
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v (product_selection+defer_local_product_binding should pass)", appErr)
	}
	if task.SKUCode != "HQT02872" {
		t.Fatalf("Create() sku_code = %s, want HQT02872", task.SKUCode)
	}
}

func TestTaskServiceCreateOriginalProductWithOrgTeamCompatOwnerTeamPasses(t *testing.T) {
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
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:      domain.TaskTypeOriginalProductDevelopment,
		SourceMode:    domain.TaskSourceModeExistingProduct,
		CreatorID:     9,
		OwnerTeam:     "运营三组",
		DeadlineAt:    timePtr(),
		ChangeRequest: "defer binding design change",
		ProductSelection: &domain.TaskProductSelectionContext{
			DeferLocalProductBinding: true,
			ERPProduct: &domain.ERPProductSelectionSnapshot{
				ProductID:   "ERP-001",
				SKUCode:     "HQT02872",
				SKUID:       "SKU-ERP",
				ProductName: "Test Product",
			},
		},
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.OwnerTeam != "内贸运营组" {
		t.Fatalf("Create() owner_team = %q, want 内贸运营组", task.OwnerTeam)
	}
}

func TestTaskServiceCreateNewProductWithOrgTeamCompatOwnerTeamPasses(t *testing.T) {
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
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           "运营三组",
		DeadlineAt:          timePtr(),
		CategoryCode:        "LIGHTBOX",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "铝型材",
		ProductNameSnapshot: "New Lightbox",
		ProductShortName:    "Lightbox",
		DesignRequirement:   "need design",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.OwnerTeam != "内贸运营组" {
		t.Fatalf("Create() owner_team = %q, want 内贸运营组", task.OwnerTeam)
	}
}

func TestTaskServiceCreatePurchaseTaskWithOrgTeamCompatOwnerTeamPasses(t *testing.T) {
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
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypePurchaseTask,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           "运营三组",
		DeadlineAt:          timePtr(),
		PurchaseSKU:         "PUR-001",
		ProductNameSnapshot: "Accessory Pack",
		CostPriceMode:       string(domain.CostPriceModeTemplate),
		Quantity:            int64Ptr(100),
		BaseSalePrice:       float64Ptr(12.5),
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.OwnerTeam != "内贸运营组" {
		t.Fatalf("Create() owner_team = %q, want 内贸运营组", task.OwnerTeam)
	}
}

func TestTaskServiceCreateLegacyOwnerTeamStillPasses(t *testing.T) {
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
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           "内贸运营组",
		DeadlineAt:          timePtr(),
		CategoryCode:        "LIGHTBOX",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "铝型材",
		ProductNameSnapshot: "New Lightbox",
		ProductShortName:    "Lightbox",
		DesignRequirement:   "need design",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.OwnerTeam != "内贸运营组" {
		t.Fatalf("Create() owner_team = %q, want 内贸运营组", task.OwnerTeam)
	}
}

func TestTaskServiceCreateRejectsInvalidOwnerTeamInputs(t *testing.T) {
	cases := []struct {
		name        string
		ownerTeam   string
		wantCode    string
		wantMessage string
	}{
		{name: "empty", ownerTeam: "", wantCode: "missing_owner_team", wantMessage: "owner_team is required"},
		{name: "unknown_group", ownerTeam: "不存在的组", wantCode: "invalid_owner_team", wantMessage: "owner_team must be a valid configured team"},
		{name: "random_string", ownerTeam: "random-invalid-team", wantCode: "invalid_owner_team", wantMessage: "owner_team must be a valid configured team"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewTaskService(
				&prdTaskRepo{},
				&prdProcurementRepo{},
				&prdTaskAssetRepo{},
				&prdTaskEventRepo{},
				nil,
				&prdWarehouseRepo{},
				prdCodeRuleService{},
				step04TxRunner{},
			)

			_, appErr := svc.Create(context.Background(), CreateTaskParams{
				TaskType:            domain.TaskTypeNewProductDevelopment,
				SourceMode:          domain.TaskSourceModeNewProduct,
				CreatorID:           9,
				OwnerTeam:           tc.ownerTeam,
				DeadlineAt:          timePtr(),
				CategoryCode:        "LIGHTBOX",
				MaterialMode:        string(domain.MaterialModePreset),
				Material:            "铝型材",
				ProductNameSnapshot: "New Lightbox",
				ProductShortName:    "Lightbox",
				DesignRequirement:   "need design",
			})
			if appErr == nil {
				t.Fatal("Create() expected owner_team validation error")
			}
			if appErr.Message != tc.wantMessage {
				t.Fatalf("Create() error message = %q, want %q", appErr.Message, tc.wantMessage)
			}
			details, ok := appErr.Details.(map[string]interface{})
			if !ok {
				t.Fatalf("Create() details type = %#v", appErr.Details)
			}
			violations, ok := details["violations"].([]map[string]interface{})
			if !ok {
				rawViolations, ok := details["violations"].([]interface{})
				if !ok || len(rawViolations) == 0 {
					t.Fatalf("Create() violations missing: %#v", details["violations"])
				}
				first, ok := rawViolations[0].(map[string]interface{})
				if !ok {
					t.Fatalf("Create() first violation type = %#v", rawViolations[0])
				}
				violations = []map[string]interface{}{first}
			}
			if violations[0]["field"] != "owner_team" {
				t.Fatalf("Create() violation field = %v, want owner_team", violations[0]["field"])
			}
			if violations[0]["code"] != tc.wantCode {
				t.Fatalf("Create() violation code = %v, want %s", violations[0]["code"], tc.wantCode)
			}
		})
	}
}

// design_requirement alias: when frontend sends design_requirement but not change_request,
// normalize copies it to change_request so whitelist does not reject.
func TestTaskServiceCreateOriginalProductDesignRequirementAliasPasses(t *testing.T) {
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
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:          domain.TaskTypeOriginalProductDevelopment,
		SourceMode:        domain.TaskSourceModeExistingProduct,
		CreatorID:         9,
		OwnerTeam:         domain.AllValidTeams()[0],
		DeadlineAt:        timePtr(),
		ChangeRequest:     "",
		DesignRequirement: "modify layout (alias for change_request)",
		ProductID:         int64Ptr(88),
		SKUCode:           "SKU-088",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v (design_requirement alias should normalize to change_request)", appErr)
	}
	if task == nil {
		t.Fatal("Create() task = nil")
	}
	detail := taskRepo.details[task.ID]
	if detail == nil {
		t.Fatal("Create() detail = nil")
	}
	if detail.ChangeRequest != "modify layout (alias for change_request)" {
		t.Fatalf("detail.change_request = %q", detail.ChangeRequest)
	}
	if detail.DesignRequirement != "" {
		t.Fatalf("detail.design_requirement = %q, want empty after alias normalization", detail.DesignRequirement)
	}
}

func TestTaskServiceCreateOriginalProductChangeRequestAndDesignRequirementAliasPasses(t *testing.T) {
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
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:          domain.TaskTypeOriginalProductDevelopment,
		SourceMode:        domain.TaskSourceModeExistingProduct,
		CreatorID:         9,
		OwnerTeam:         domain.AllValidTeams()[0],
		DeadlineAt:        timePtr(),
		ChangeRequest:     "keep this explicit change request",
		DesignRequirement: "frontend alias should be ignored then cleared",
		ProductID:         int64Ptr(88),
		SKUCode:           "SKU-088",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v (design_requirement alias should not trip whitelist when change_request already exists)", appErr)
	}
	if task == nil {
		t.Fatal("Create() task = nil")
	}
	detail := taskRepo.details[task.ID]
	if detail == nil {
		t.Fatal("Create() detail = nil")
	}
	if detail.ChangeRequest != "keep this explicit change request" {
		t.Fatalf("detail.change_request = %q", detail.ChangeRequest)
	}
	if detail.DesignRequirement != "" {
		t.Fatalf("detail.design_requirement = %q, want empty after alias normalization", detail.DesignRequirement)
	}
}

func TestTaskServiceCreateTaskOriginalProductDevelopmentResponseEchoesChangeRequest(t *testing.T) {
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
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeOriginalProductDevelopment,
		SourceMode:          domain.TaskSourceModeExistingProduct,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		ChangeRequest:       "ABC",
		ProductID:           int64Ptr(88),
		SKUCode:             "SKU-ABC",
		ProductNameSnapshot: "Original Product",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}

	readModel, appErr := svc.GetByID(context.Background(), task.ID)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.ChangeRequest != "ABC" {
		t.Fatalf("readModel.change_request = %q, want ABC", readModel.ChangeRequest)
	}
	if readModel.DesignRequirement != "ABC" {
		t.Fatalf("readModel.design_requirement = %q, want ABC", readModel.DesignRequirement)
	}
	if len(readModel.SKUItems) != 1 {
		t.Fatalf("sku_items len = %d, want 1", len(readModel.SKUItems))
	}
	if readModel.SKUItems[0].ChangeRequest != "ABC" || readModel.SKUItems[0].DesignRequirement != "ABC" {
		t.Fatalf("sku item demand fields = change_request:%q design_requirement:%q, want ABC/ABC", readModel.SKUItems[0].ChangeRequest, readModel.SKUItems[0].DesignRequirement)
	}
}

func TestTaskServiceCreateTaskNewProductDevelopmentResponseEchoesDesignRequirement(t *testing.T) {
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
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		SKUCode:             "NEW-XYZ",
		ProductNameSnapshot: "New Product",
		ProductShortName:    "New",
		CategoryCode:        "LIGHTBOX",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "Aluminum",
		DesignRequirement:   "XYZ",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}

	readModel, appErr := svc.GetByID(context.Background(), task.ID)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.ChangeRequest != "" {
		t.Fatalf("readModel.change_request = %q, want empty", readModel.ChangeRequest)
	}
	if readModel.DesignRequirement != "XYZ" {
		t.Fatalf("readModel.design_requirement = %q, want XYZ", readModel.DesignRequirement)
	}
}

// Case C: original_product_development + truly illegal fields.
// Must return 400 with invalid_fields listing the illegal fields.
func TestTaskServiceCreateOriginalProductWithIllegalFieldsReturnsInvalidFields(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:       domain.TaskTypeOriginalProductDevelopment,
		SourceMode:     domain.TaskSourceModeExistingProduct,
		CreatorID:      9,
		OwnerTeam:      domain.AllValidTeams()[0],
		DeadlineAt:     timePtr(),
		ChangeRequest:  "valid change",
		ProductID:      int64Ptr(88),
		SKUCode:        "SKU-088",
		MaterialMode:   "preset",
		Material:       "铝型材",
		ProductChannel: "channel-x",
	})
	if appErr == nil {
		t.Fatal("Create() expected whitelist error for illegal fields")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("Create() error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
	if !strings.Contains(appErr.Message, "task_type field whitelist validation failed") {
		t.Fatalf("Create() error message = %q", appErr.Message)
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("details type = %#v", appErr.Details)
	}
	invalidFields, ok := details["invalid_fields"].([]interface{})
	if !ok {
		invalidFieldsSlice, ok2 := details["invalid_fields"].([]string)
		if !ok2 || len(invalidFieldsSlice) == 0 {
			t.Fatalf("invalid_fields missing or empty: %#v", details["invalid_fields"])
		}
		invalidFields = make([]interface{}, len(invalidFieldsSlice))
		for i, s := range invalidFieldsSlice {
			invalidFields[i] = s
		}
	}
	if len(invalidFields) == 0 {
		t.Fatal("invalid_fields must list the illegal fields")
	}
	fieldSet := make(map[string]bool)
	for _, f := range invalidFields {
		if s, ok := f.(string); ok {
			fieldSet[s] = true
		}
	}
	if !fieldSet["material_mode"] || !fieldSet["material"] || !fieldSet["product_channel"] {
		t.Fatalf("invalid_fields = %v, want material_mode, material, product_channel", fieldSet)
	}
}

func TestTaskServiceCreateRejectsMisalignedTaskTypeAndSourceMode(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		SourceMode: domain.TaskSourceModeExistingProduct,
		ProductID:  int64Ptr(8),
		SKUCode:    "SKU-001",
		TaskType:   domain.TaskTypeNewProductDevelopment,
		CreatorID:  99,
		OwnerTeam:  domain.AllValidTeams()[0],
		DeadlineAt: timePtr(),
	})
	if appErr == nil {
		t.Fatal("Create() expected error, got nil")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("Create() error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
}

func TestTaskServiceCreatePersistsProductSelectionContext(t *testing.T) {
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
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:      domain.TaskTypeOriginalProductDevelopment,
		CreatorID:     9,
		OwnerTeam:     domain.AllValidTeams()[0],
		DeadlineAt:    timePtr(),
		ChangeRequest: "update banner stand layout",
		ProductSelection: &domain.TaskProductSelectionContext{
			SelectedProductID:      int64Ptr(88),
			SelectedProductName:    "KT Banner Stand",
			SelectedProductSKUCode: "SKU-088",
			MatchedCategoryCode:    "HBJ",
			MatchedSearchEntryCode: "HBJ",
			MatchedMappingRule: &domain.ProductSearchMatchedMapping{
				MappingID:       12,
				CategoryCode:    "HBJ",
				SearchEntryCode: "HBJ",
				ERPMatchType:    domain.CategoryERPMatchTypeKeyword,
				ERPMatchValue:   "banner stand",
				IsPrimary:       true,
				Priority:        10,
			},
		},
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.ProductID == nil || *task.ProductID != 88 {
		t.Fatalf("Create() product_id = %+v, want 88", task.ProductID)
	}
	if task.SKUCode != "SKU-088" || task.ProductNameSnapshot != "KT Banner Stand" {
		t.Fatalf("Create() product binding = %s / %s", task.SKUCode, task.ProductNameSnapshot)
	}

	detail := taskRepo.details[task.ID]
	if detail == nil {
		t.Fatal("task detail not persisted")
	}
	if detail.SourceProductID == nil || *detail.SourceProductID != 88 {
		t.Fatalf("detail.source_product_id = %+v, want 88", detail.SourceProductID)
	}
	if detail.SourceMatchType != string(domain.CategoryERPMatchTypeKeyword) {
		t.Fatalf("detail.source_match_type = %s, want keyword", detail.SourceMatchType)
	}
	if detail.MatchedSearchEntryCode != "HBJ" || detail.SourceSearchEntryCode != "HBJ" {
		t.Fatalf("detail search entry = %s / %s", detail.MatchedSearchEntryCode, detail.SourceSearchEntryCode)
	}
	readModel, appErr := svc.GetByID(context.Background(), task.ID)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.ProductSelection == nil || readModel.ProductSelection.SelectedProductID == nil || *readModel.ProductSelection.SelectedProductID != 88 {
		t.Fatalf("read_model.product_selection = %+v", readModel.ProductSelection)
	}
}

func TestTaskServiceCreateAutoGeneratesSKUForNewProductDevelopment(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		CreatorID:           9,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		CategoryCode:        "LIGHTBOX",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "铝型材",
		ProductNameSnapshot: "New Lightbox",
		ProductShortName:    "Lightbox",
		DesignRequirement:   "need a clean lightbox design",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.SourceMode != domain.TaskSourceModeNewProduct {
		t.Fatalf("Create() source_mode = %s, want %s", task.SourceMode, domain.TaskSourceModeNewProduct)
	}
	if task.SKUCode != "SKU-TEST" {
		t.Fatalf("Create() sku_code = %s, want SKU-TEST", task.SKUCode)
	}
	if task.ProductID != nil {
		t.Fatalf("Create() product_id = %+v, want nil", task.ProductID)
	}

	readModel, appErr := svc.GetByID(context.Background(), task.ID)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.Workflow.MainStatus != domain.TaskMainStatusCreated {
		t.Fatalf("workflow.main_status = %s, want %s", readModel.Workflow.MainStatus, domain.TaskMainStatusCreated)
	}
	if readModel.Workflow.SubStatus.Design.Code != domain.TaskSubStatusPendingDesign {
		t.Fatalf("workflow.sub_status.design = %+v", readModel.Workflow.SubStatus.Design)
	}
	if readModel.Workflow.SubStatus.Audit.Code != domain.TaskSubStatusNotTriggered {
		t.Fatalf("workflow.sub_status.audit = %+v", readModel.Workflow.SubStatus.Audit)
	}
	if readModel.ProductSelection != nil {
		t.Fatalf("product_selection = %+v, want nil", readModel.ProductSelection)
	}
	if len(eventRepo.events) < 1 || eventRepo.events[0].EventType != domain.TaskEventCreated {
		t.Fatalf("Create() events = %+v", eventRepo.events)
	}
	detail := taskRepo.details[task.ID]
	if detail == nil {
		t.Fatal("task detail not found")
	}
	if detail.FilingStatus != domain.FilingStatusPending {
		t.Fatalf("filing_status = %s, want pending_filing until i_id is supplied", detail.FilingStatus)
	}
}

func TestTaskServiceCreateInitializesPurchaseTaskDraftProcurementReadModel(t *testing.T) {
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
		step04TxRunner{},
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:            domain.TaskTypePurchaseTask,
		CreatorID:           12,
		OwnerTeam:           domain.AllValidTeams()[0],
		DeadlineAt:          timePtr(),
		PurchaseSKU:         "PUR-001",
		ProductNameSnapshot: "Accessory Pack",
		CostPriceMode:       string(domain.CostPriceModeTemplate),
		Quantity:            int64Ptr(100),
		BaseSalePrice:       float64Ptr(12.5),
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.SourceMode != domain.TaskSourceModeNewProduct {
		t.Fatalf("Create() source_mode = %s, want %s", task.SourceMode, domain.TaskSourceModeNewProduct)
	}
	if task.SKUCode != "PUR-001" {
		t.Fatalf("Create() sku_code = %s, want PUR-001", task.SKUCode)
	}
	record := procurementRepo.records[task.ID]
	if record == nil || record.Status != domain.ProcurementStatusDraft {
		t.Fatalf("procurement record = %+v, want draft", record)
	}

	readModel, appErr := svc.GetByID(context.Background(), task.ID)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.Procurement == nil || readModel.Procurement.Status != domain.ProcurementStatusDraft {
		t.Fatalf("read_model.procurement = %+v", readModel.Procurement)
	}
	if readModel.ProcurementSummary == nil || readModel.ProcurementSummary.CoordinationStatus != domain.ProcurementCoordinationStatusPreparing {
		t.Fatalf("read_model.procurement_summary = %+v", readModel.ProcurementSummary)
	}
	if readModel.Workflow.SubStatus.Design.Code != domain.TaskSubStatusNotRequired {
		t.Fatalf("workflow.sub_status.design = %+v", readModel.Workflow.SubStatus.Design)
	}
	if readModel.Workflow.SubStatus.Audit.Code != domain.TaskSubStatusNotTriggered {
		t.Fatalf("workflow.sub_status.audit = %+v", readModel.Workflow.SubStatus.Audit)
	}
	if readModel.Workflow.SubStatus.Procurement.Code != domain.TaskSubStatusPreparing {
		t.Fatalf("workflow.sub_status.procurement = %+v", readModel.Workflow.SubStatus.Procurement)
	}
}

func TestTaskServiceCreateAllowsNewProductWithoutFilingRequiredFieldsAndMarksPending(t *testing.T) {
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
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:          domain.TaskTypeNewProductDevelopment,
		CreatorID:         3,
		OwnerTeam:         domain.AllValidTeams()[0],
		DeadlineAt:        timePtr(),
		CategoryCode:      "KT_STANDARD",
		MaterialMode:      string(domain.MaterialModePreset),
		Material:          "KT板",
		ProductShortName:  "KT",
		DesignRequirement: "new product without name should fail",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	detail := taskRepo.details[task.ID]
	if detail == nil {
		t.Fatal("task detail not found")
	}
	if detail.FilingStatus != domain.FilingStatusPending {
		t.Fatalf("filing_status = %s, want %s", detail.FilingStatus, domain.FilingStatusPending)
	}
	if len(detail.MissingFields) == 0 {
		t.Fatalf("missing_fields = %+v, want non-empty", detail.MissingFields)
	}
	if detail.MissingFieldsSummaryCN == "" {
		t.Fatalf("missing_fields_summary_cn should not be empty")
	}
}

func TestTaskServicePrepareWarehouseAllowsPurchaseTaskAfterArrivalCompleted(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			1: {
				ID:         1,
				TaskNo:     "RW-001",
				SKUCode:    "SKU-001",
				TaskType:   domain.TaskTypePurchaseTask,
				TaskStatus: domain.TaskStatusPendingAssign,
			},
		},
		details: map[int64]*domain.TaskDetail{
			1: {
				TaskID:    1,
				Category:  "Accessory",
				SpecText:  "Spec-A",
				CostPrice: float64Ptr(12.5),
				FiledAt:   timePtr(),
			},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	procurementRepo := &prdProcurementRepo{
		records: map[int64]*domain.ProcurementRecord{
			1: {
				TaskID:           1,
				Status:           domain.ProcurementStatusCompleted,
				ProcurementPrice: float64Ptr(8.8),
				Quantity:         int64Ptr(50),
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		procurementRepo,
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	task, appErr := svc.PrepareWarehouse(context.Background(), PrepareTaskForWarehouseParams{
		TaskID:     1,
		OperatorID: 7,
		Remark:     "purchase task handoff after arrival",
	})
	if appErr != nil {
		t.Fatalf("PrepareWarehouse() unexpected error: %+v", appErr)
	}
	if task.TaskStatus != domain.TaskStatusPendingWarehouseReceive {
		t.Fatalf("PrepareWarehouse() status = %s, want %s", task.TaskStatus, domain.TaskStatusPendingWarehouseReceive)
	}
	if len(eventRepo.events) != 1 || eventRepo.events[0].EventType != domain.TaskEventWarehousePrepared {
		t.Fatalf("PrepareWarehouse() events = %+v", eventRepo.events)
	}
}

func TestTaskServicePrepareWarehouseBlocksPurchaseTaskAwaitingArrival(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			11: {
				ID:         11,
				TaskNo:     "RW-011",
				SKUCode:    "SKU-011",
				TaskType:   domain.TaskTypePurchaseTask,
				TaskStatus: domain.TaskStatusPendingAssign,
			},
		},
		details: map[int64]*domain.TaskDetail{
			11: {
				TaskID:    11,
				Category:  "Accessory",
				SpecText:  "Spec-A",
				CostPrice: float64Ptr(12.5),
				FiledAt:   timePtr(),
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{
			records: map[int64]*domain.ProcurementRecord{
				11: {
					TaskID:           11,
					Status:           domain.ProcurementStatusInProgress,
					ProcurementPrice: float64Ptr(8.8),
					Quantity:         int64Ptr(50),
				},
			},
		},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	_, appErr := svc.PrepareWarehouse(context.Background(), PrepareTaskForWarehouseParams{
		TaskID:     11,
		OperatorID: 7,
	})
	if appErr == nil {
		t.Fatal("PrepareWarehouse() expected error, got nil")
	}
	reasons, ok := appErr.Details.(map[string]interface{})["warehouse_blocking_reasons"].([]domain.WorkflowReason)
	if !ok {
		t.Fatalf("PrepareWarehouse() reasons type = %#v", appErr.Details)
	}
	want := []domain.WorkflowReason{
		{Code: domain.WorkflowReasonProcurementNotReady, Message: "Procurement arrival is not completed yet."},
	}
	if !reflect.DeepEqual(reasons, want) {
		t.Fatalf("PrepareWarehouse() reasons = %#v, want %#v", reasons, want)
	}
}

func TestTaskServicePrepareWarehouseBlocksDesignTaskWithoutFinalAsset(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			2: {
				ID:         2,
				TaskNo:     "RW-002",
				SKUCode:    "SKU-002",
				TaskType:   domain.TaskTypeOriginalProductDevelopment,
				TaskStatus: domain.TaskStatusInProgress,
			},
		},
		details: map[int64]*domain.TaskDetail{
			2: {
				TaskID:    2,
				Category:  "Poster",
				SpecText:  "A4",
				CostPrice: float64Ptr(20),
				FiledAt:   timePtr(),
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

	_, appErr := svc.PrepareWarehouse(context.Background(), PrepareTaskForWarehouseParams{
		TaskID:     2,
		OperatorID: 7,
	})
	if appErr == nil {
		t.Fatal("PrepareWarehouse() expected error, got nil")
	}
	reasons, ok := appErr.Details.(map[string]interface{})["warehouse_blocking_reasons"].([]domain.WorkflowReason)
	if !ok {
		t.Fatalf("PrepareWarehouse() reasons type = %#v", appErr.Details)
	}
	want := []domain.WorkflowReason{
		{Code: domain.WorkflowReasonMissingFinalAsset, Message: "Final design asset is missing."},
		{Code: domain.WorkflowReasonAuditNotApproved, Message: "Audit has not been approved yet."},
	}
	if !reflect.DeepEqual(reasons, want) {
		t.Fatalf("PrepareWarehouse() reasons = %#v, want %#v", reasons, want)
	}
}

func TestTaskServiceCloseRequiresPendingCloseReadiness(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			3: {
				ID:         3,
				TaskNo:     "RW-003",
				SKUCode:    "SKU-003",
				TaskType:   domain.TaskTypePurchaseTask,
				TaskStatus: domain.TaskStatusPendingAssign,
			},
		},
		details: map[int64]*domain.TaskDetail{
			3: {
				TaskID:    3,
				Category:  "Sticker",
				SpecText:  "Spec-A",
				CostPrice: float64Ptr(12.5),
				FiledAt:   timePtr(),
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

	_, appErr := svc.Close(context.Background(), CloseTaskParams{
		TaskID:     3,
		OperatorID: 9,
	})
	if appErr == nil {
		t.Fatal("Close() expected error, got nil")
	}

	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("Close() details type = %#v", appErr.Details)
	}
	reasons, ok := details["cannot_close_reasons"].([]domain.WorkflowReason)
	if !ok {
		t.Fatalf("Close() reasons type = %#v", details)
	}
	want := []domain.WorkflowReason{
		{Code: domain.WorkflowReasonProcurementMissing, Message: "Procurement record is missing."},
		{Code: domain.WorkflowReasonNotPendingClose, Message: "Task is not in pending-close state."},
		{Code: domain.WorkflowReasonWarehouseNotReceived, Message: "Warehouse has not received the task."},
	}
	if !reflect.DeepEqual(reasons, want) {
		t.Fatalf("Close() reasons = %#v, want %#v", reasons, want)
	}
}

func TestTaskServiceCloseTransitionsPendingCloseToCompleted(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			4: {
				ID:         4,
				TaskNo:     "RW-004",
				SKUCode:    "SKU-004",
				TaskType:   domain.TaskTypePurchaseTask,
				TaskStatus: domain.TaskStatusPendingClose,
			},
		},
		details: map[int64]*domain.TaskDetail{
			4: {
				TaskID:    4,
				Category:  "Sticker",
				SpecText:  "Spec-B",
				CostPrice: float64Ptr(10.4),
				FiledAt:   timePtr(),
			},
		},
	}
	warehouseRepo := &prdWarehouseRepo{
		receipts: map[int64]*domain.WarehouseReceipt{
			4: {
				TaskID: 4,
				Status: domain.WarehouseReceiptStatusCompleted,
			},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	procurementRepo := &prdProcurementRepo{
		records: map[int64]*domain.ProcurementRecord{
			4: {
				TaskID:           4,
				Status:           domain.ProcurementStatusCompleted,
				ProcurementPrice: float64Ptr(6.2),
				Quantity:         int64Ptr(20),
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		procurementRepo,
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		warehouseRepo,
		prdCodeRuleService{},
		step04TxRunner{},
	)

	task, appErr := svc.Close(context.Background(), CloseTaskParams{
		TaskID:     4,
		OperatorID: 9,
		Remark:     "close",
	})
	if appErr != nil {
		t.Fatalf("Close() unexpected error: %+v", appErr)
	}
	if task.TaskStatus != domain.TaskStatusCompleted {
		t.Fatalf("Close() status = %s, want %s", task.TaskStatus, domain.TaskStatusCompleted)
	}
	if task.Workflow.MainStatus != domain.TaskMainStatusClosed {
		t.Fatalf("Close() main_status = %s, want %s", task.Workflow.MainStatus, domain.TaskMainStatusClosed)
	}
	if len(eventRepo.events) != 1 || eventRepo.events[0].EventType != domain.TaskEventClosed {
		t.Fatalf("Close() events = %+v", eventRepo.events)
	}
}

func TestWarehouseServiceCompleteMovesTaskToPendingClose(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			5: {
				ID:         5,
				TaskNo:     "RW-005",
				SKUCode:    "SKU-005",
				TaskType:   domain.TaskTypePurchaseTask,
				TaskStatus: domain.TaskStatusPendingWarehouseReceive,
			},
		},
	}
	warehouseRepo := &prdWarehouseRepo{
		receipts: map[int64]*domain.WarehouseReceipt{
			5: {
				TaskID:     5,
				ReceiptNo:  "WR-5",
				Status:     domain.WarehouseReceiptStatusReceived,
				ReceiverID: int64Ptr(11),
			},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	svc := NewWarehouseService(taskRepo, &prdTaskAssetRepo{}, warehouseRepo, eventRepo, step04TxRunner{})

	receipt, appErr := svc.Complete(context.Background(), CompleteWarehouseParams{
		TaskID:     5,
		ReceiverID: 11,
		Remark:     "warehouse done",
	})
	if appErr != nil {
		t.Fatalf("Complete() unexpected error: %+v", appErr)
	}
	if receipt.Status != domain.WarehouseReceiptStatusCompleted {
		t.Fatalf("Complete() receipt status = %s, want %s", receipt.Status, domain.WarehouseReceiptStatusCompleted)
	}
	if taskRepo.tasks[5].TaskStatus != domain.TaskStatusPendingClose {
		t.Fatalf("Complete() task status = %s, want %s", taskRepo.tasks[5].TaskStatus, domain.TaskStatusPendingClose)
	}
	if taskRepo.tasks[5].CurrentHandlerID != nil {
		t.Fatalf("Complete() current_handler_id = %+v, want nil", taskRepo.tasks[5].CurrentHandlerID)
	}
	if len(eventRepo.events) != 1 || eventRepo.events[0].EventType != domain.TaskEventWarehouseCompleted {
		t.Fatalf("Complete() events = %+v", eventRepo.events)
	}
}

func TestWarehouseServiceRejectRoutesDesignTaskBackToRejectedAuditB(t *testing.T) {
	designerID := int64(21)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			31: {
				ID:         31,
				TaskNo:     "RW-031",
				SKUCode:    "SKU-031",
				TaskType:   domain.TaskTypeOriginalProductDevelopment,
				DesignerID: &designerID,
				TaskStatus: domain.TaskStatusPendingWarehouseReceive,
			},
		},
	}
	warehouseRepo := &prdWarehouseRepo{}
	eventRepo := &prdTaskEventRepo{}
	svc := NewWarehouseService(taskRepo, &prdTaskAssetRepo{}, warehouseRepo, eventRepo, step04TxRunner{})

	receipt, appErr := svc.Reject(context.Background(), RejectWarehouseParams{
		TaskID:       31,
		ReceiverID:   88,
		RejectReason: "packaging issue",
		Remark:       "send back",
	})
	if appErr != nil {
		t.Fatalf("Reject() unexpected error: %+v", appErr)
	}
	if receipt.Status != domain.WarehouseReceiptStatusRejected {
		t.Fatalf("Reject() receipt status = %s, want rejected", receipt.Status)
	}
	if taskRepo.tasks[31].TaskStatus != domain.TaskStatusRejectedByWarehouse {
		t.Fatalf("Reject() task status = %s, want %s", taskRepo.tasks[31].TaskStatus, domain.TaskStatusRejectedByWarehouse)
	}
	if taskRepo.tasks[31].CurrentHandlerID == nil || *taskRepo.tasks[31].CurrentHandlerID != designerID {
		t.Fatalf("Reject() current_handler_id = %+v, want %d", taskRepo.tasks[31].CurrentHandlerID, designerID)
	}
	if taskRepo.tasks[31].WarehouseRejectReason != "packaging issue" {
		t.Fatalf("Reject() warehouse_reject_reason = %q, want packaging issue", taskRepo.tasks[31].WarehouseRejectReason)
	}
}

func TestWarehouseServiceReceiveReusesRejectedReceipt(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			32: {
				ID:         32,
				TaskNo:     "RW-032",
				SKUCode:    "SKU-032",
				TaskType:   domain.TaskTypePurchaseTask,
				TaskStatus: domain.TaskStatusPendingWarehouseReceive,
			},
		},
	}
	warehouseRepo := &prdWarehouseRepo{
		receipts: map[int64]*domain.WarehouseReceipt{
			32: {
				ID:           1,
				TaskID:       32,
				ReceiptNo:    "WR-32",
				Status:       domain.WarehouseReceiptStatusRejected,
				RejectReason: "missing label",
			},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	svc := NewWarehouseService(taskRepo, &prdTaskAssetRepo{}, warehouseRepo, eventRepo, step04TxRunner{})

	receipt, appErr := svc.Receive(context.Background(), ReceiveWarehouseParams{
		TaskID:     32,
		ReceiverID: 77,
		Remark:     "received after relabel",
	})
	if appErr != nil {
		t.Fatalf("Receive() unexpected error: %+v", appErr)
	}
	if receipt.Status != domain.WarehouseReceiptStatusReceived {
		t.Fatalf("Receive() receipt status = %s, want received", receipt.Status)
	}
	if taskRepo.tasks[32].CurrentHandlerID == nil || *taskRepo.tasks[32].CurrentHandlerID != 77 {
		t.Fatalf("Receive() current_handler_id = %+v, want 77", taskRepo.tasks[32].CurrentHandlerID)
	}
}

func TestWarehouseServiceInjectedAuthorizerDeniesHydratedDepartmentManagerOutsideScope(t *testing.T) {
	userRepo := newIdentityUserRepo()
	userRepo.users[88] = &domain.User{
		ID:         88,
		Username:   "dept_admin",
		Department: domain.DepartmentDesign,
		Team:       "default-team",
	}
	userRepo.roles[88] = []domain.Role{domain.RoleDeptAdmin}

	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			33: {
				ID:              33,
				TaskNo:          "RW-033",
				SKUCode:         "SKU-033",
				TaskType:        domain.TaskTypePurchaseTask,
				TaskStatus:      domain.TaskStatusPendingWarehouseReceive,
				OwnerDepartment: "ops",
				OwnerOrgTeam:    "ops-team-1",
			},
		},
	}
	warehouseRepo := &prdWarehouseRepo{}
	eventRepo := &prdTaskEventRepo{}
	svc := NewWarehouseService(
		taskRepo,
		&prdTaskAssetRepo{},
		warehouseRepo,
		eventRepo,
		step04TxRunner{},
		WithWarehouseDataScopeResolver(NewRoleBasedDataScopeResolver()),
		WithWarehouseScopeUserRepo(userRepo),
	)

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    88,
		Roles: []domain.Role{domain.RoleDeptAdmin},
	})
	_, appErr := svc.Receive(ctx, ReceiveWarehouseParams{
		TaskID:     33,
		ReceiverID: 88,
		Remark:     "should be denied by hydrated scope",
	})
	if appErr == nil {
		t.Fatal("Receive() expected permission error")
	}
	if appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("Receive() code = %s, want %s", appErr.Code, domain.ErrCodePermissionDenied)
	}
	if len(eventRepo.events) != 0 {
		t.Fatalf("Receive() events = %+v, want none", eventRepo.events)
	}
}

func TestTaskServicePurchaseRejectKeepsCoordinationTruthfulAfterWarehouseReject(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			33: {
				ID:         33,
				TaskNo:     "RW-033",
				SKUCode:    "SKU-033",
				TaskType:   domain.TaskTypePurchaseTask,
				TaskStatus: domain.TaskStatusPendingAssign,
			},
		},
		details: map[int64]*domain.TaskDetail{
			33: {
				TaskID:    33,
				Category:  "Accessory",
				SpecText:  "Spec-X",
				CostPrice: float64Ptr(5.5),
				FiledAt:   timePtr(),
			},
		},
	}
	procurementRepo := &prdProcurementRepo{
		records: map[int64]*domain.ProcurementRecord{
			33: {
				TaskID:           33,
				Status:           domain.ProcurementStatusCompleted,
				ProcurementPrice: float64Ptr(3.3),
				Quantity:         int64Ptr(10),
			},
		},
	}
	warehouseRepo := &prdWarehouseRepo{
		receipts: map[int64]*domain.WarehouseReceipt{
			33: {
				TaskID:       33,
				ReceiptNo:    "WR-33",
				Status:       domain.WarehouseReceiptStatusRejected,
				RejectReason: "quality issue",
			},
		},
	}
	svc := NewTaskService(
		taskRepo,
		procurementRepo,
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		warehouseRepo,
		prdCodeRuleService{},
		step04TxRunner{},
	)

	readModel, appErr := svc.GetByID(context.Background(), 33)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.ProcurementSummary == nil || readModel.ProcurementSummary.CoordinationStatus != domain.ProcurementCoordinationStatusReadyForWarehouse {
		t.Fatalf("GetByID() procurement_summary = %+v", readModel.ProcurementSummary)
	}
	if !readModel.ProcurementSummary.WarehousePrepareReady {
		t.Fatalf("GetByID() warehouse_prepare_ready = false, want true")
	}
	if readModel.ProcurementSummary.WarehouseReceiveReady {
		t.Fatalf("GetByID() warehouse_receive_ready = true, want false")
	}
	if readModel.Workflow.SubStatus.Warehouse.Code != domain.TaskSubStatusRejected {
		t.Fatalf("GetByID() warehouse sub_status = %+v", readModel.Workflow.SubStatus.Warehouse)
	}
}

func TestTaskServiceUpdateProcurementPersistsRecord(t *testing.T) {
	procurementRepo := &prdProcurementRepo{}
	svc := NewTaskService(
		&prdTaskRepo{
			tasks: map[int64]*domain.Task{
				6: {
					ID:       6,
					TaskType: domain.TaskTypePurchaseTask,
				},
			},
		},
		procurementRepo,
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	record, appErr := svc.UpdateProcurement(context.Background(), UpdateTaskProcurementParams{
		TaskID:           6,
		OperatorID:       12,
		Status:           domain.ProcurementStatusDraft,
		ProcurementPrice: float64Ptr(18.5),
		Quantity:         int64Ptr(30),
		SupplierName:     "Vendor A",
		PurchaseRemark:   "ready",
	})
	if appErr != nil {
		t.Fatalf("UpdateProcurement() unexpected error: %+v", appErr)
	}
	if record.Status != domain.ProcurementStatusDraft {
		t.Fatalf("UpdateProcurement() status = %s, want %s", record.Status, domain.ProcurementStatusDraft)
	}
	if got := procurementRepo.records[6]; got == nil || got.SupplierName != "Vendor A" {
		t.Fatalf("UpdateProcurement() repo state = %+v", procurementRepo.records[6])
	}
}

func TestTaskServiceUpdateProcurementStatusOnlyPreservesProcurementValues(t *testing.T) {
	procurementRepo := &prdProcurementRepo{
		records: map[int64]*domain.ProcurementRecord{
			61: {
				TaskID:           61,
				Status:           domain.ProcurementStatusDraft,
				ProcurementPrice: float64Ptr(222),
				Quantity:         int64Ptr(22),
				SupplierName:     "Vendor A",
			},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskService(
		&prdTaskRepo{
			tasks: map[int64]*domain.Task{
				61: {
					ID:       61,
					TaskType: domain.TaskTypePurchaseTask,
				},
			},
		},
		procurementRepo,
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	record, appErr := svc.UpdateProcurement(context.Background(), UpdateTaskProcurementParams{
		TaskID:         61,
		OperatorID:     12,
		Status:         domain.ProcurementStatusCompleted,
		PurchaseRemark: "arrived",
	})
	if appErr != nil {
		t.Fatalf("UpdateProcurement() unexpected error: %+v", appErr)
	}
	if record.ProcurementPrice == nil || *record.ProcurementPrice != 222 {
		t.Fatalf("UpdateProcurement() procurement_price = %+v, want 222", record.ProcurementPrice)
	}
	if record.Quantity == nil || *record.Quantity != 22 {
		t.Fatalf("UpdateProcurement() quantity = %+v, want 22", record.Quantity)
	}
	if record.Status != domain.ProcurementStatusCompleted {
		t.Fatalf("UpdateProcurement() status = %s, want %s", record.Status, domain.ProcurementStatusCompleted)
	}
	if len(eventRepo.events) != 1 {
		t.Fatalf("UpdateProcurement() events = %+v", eventRepo.events)
	}
}

func TestTaskServiceUpdateBusinessInfoAutoPrefillsCost(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   1,
		CategoryCode: "KT_STANDARD",
		CategoryName: "KT Standard",
		DisplayName:  "KT Standard",
		CategoryType: domain.CategoryTypeBoard,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:       1,
			RuleVersion:  1,
			RuleName:     "KT Standard Base",
			CategoryCode: "KT_STANDARD",
			RuleType:     domain.CostRuleTypeFixedUnitPrice,
			BasePrice:    float64Ptr(10),
			Priority:     10,
			IsActive:     true,
			Source:       "phase_021_test",
		},
	}
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			101: {ID: 101, TaskType: domain.TaskTypeNewProductDevelopment},
		},
		details: map[int64]*domain.TaskDetail{
			101: {TaskID: 101},
		},
	}

	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		categoryRepo,
		costRuleRepo,
		prdCodeRuleService{},
		step04TxRunner{},
	)

	detail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:       101,
		OperatorID:   9,
		CategoryCode: "KT_STANDARD",
		Area:         float64Ptr(2),
		SpecText:     "2sqm board",
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if detail.EstimatedCost == nil || *detail.EstimatedCost != 20 {
		t.Fatalf("estimated_cost = %+v, want 20", detail.EstimatedCost)
	}
	if detail.CostPrice == nil || *detail.CostPrice != 20 {
		t.Fatalf("cost_price = %+v, want 20", detail.CostPrice)
	}
	if detail.CostRuleSource != "phase_021_test" || detail.CostRuleName != "KT Standard Base" {
		t.Fatalf("cost rule provenance = %s / %s", detail.CostRuleSource, detail.CostRuleName)
	}
	if detail.MatchedRuleVersion == nil || *detail.MatchedRuleVersion != 1 {
		t.Fatalf("matched_rule_version = %+v, want 1", detail.MatchedRuleVersion)
	}
	if detail.PrefillSource != taskCostPrefillSourcePreview || detail.PrefillAt == nil {
		t.Fatalf("prefill trace = %s / %+v", detail.PrefillSource, detail.PrefillAt)
	}
	if detail.ManualCostOverride {
		t.Fatalf("manual_cost_override = true, want false")
	}
	if detail.RequiresManualReview {
		t.Fatalf("requires_manual_review = true, want false")
	}
}

func TestTaskServiceUpdateBusinessInfoRebindsExistingProductSelection(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			150: {
				ID:                  150,
				SourceMode:          domain.TaskSourceModeExistingProduct,
				ProductID:           int64Ptr(10),
				SKUCode:             "SKU-010",
				ProductNameSnapshot: "Old Product",
				TaskType:            domain.TaskTypeOriginalProductDevelopment,
			},
		},
		details: map[int64]*domain.TaskDetail{
			150: {TaskID: 150},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	detail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:     150,
		OperatorID: 7,
		ProductSelection: &domain.TaskProductSelectionContext{
			SelectedProductID:      int64Ptr(11),
			SelectedProductName:    "New Product",
			SelectedProductSKUCode: "SKU-011",
			MatchedCategoryCode:    "KT_STANDARD",
			MatchedSearchEntryCode: "KT_STANDARD",
			MatchedMappingRule: &domain.ProductSearchMatchedMapping{
				MappingID:       7,
				CategoryCode:    "KT_STANDARD",
				SearchEntryCode: "KT_STANDARD",
				ERPMatchType:    domain.CategoryERPMatchTypeProductFamily,
				ERPMatchValue:   "KT",
				IsPrimary:       true,
				Priority:        10,
			},
		},
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[150].ProductID == nil || *taskRepo.tasks[150].ProductID != 11 {
		t.Fatalf("task product_id = %+v, want 11", taskRepo.tasks[150].ProductID)
	}
	if taskRepo.tasks[150].SKUCode != "SKU-011" || taskRepo.tasks[150].ProductNameSnapshot != "New Product" {
		t.Fatalf("task binding = %s / %s", taskRepo.tasks[150].SKUCode, taskRepo.tasks[150].ProductNameSnapshot)
	}
	if detail.ProductSelection == nil {
		t.Fatal("detail.product_selection = nil")
	}
	if detail.ProductSelection.SourceMatchRule != "KT" || detail.ProductSelection.MatchedSearchEntryCode != "KT_STANDARD" {
		t.Fatalf("detail.product_selection = %+v", detail.ProductSelection)
	}
	if len(eventRepo.events) != 1 {
		t.Fatalf("event count = %d, want 1", len(eventRepo.events))
	}
}

func TestTaskServiceGetByIDBuildsLegacyExistingProductSelectionFallback(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			160: {
				ID:                  160,
				SourceMode:          domain.TaskSourceModeExistingProduct,
				ProductID:           int64Ptr(66),
				SKUCode:             "SKU-066",
				ProductNameSnapshot: "Legacy Product",
				TaskType:            domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:          domain.TaskStatusPendingAssign,
			},
		},
		details: map[int64]*domain.TaskDetail{
			160: {TaskID: 160},
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

	readModel, appErr := svc.GetByID(context.Background(), 160)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.ProductSelection == nil {
		t.Fatal("product_selection = nil")
	}
	if readModel.ProductSelection.SourceMatchType != taskProductSelectionMatchLegacy {
		t.Fatalf("source_match_type = %s, want %s", readModel.ProductSelection.SourceMatchType, taskProductSelectionMatchLegacy)
	}
	if readModel.ProductSelection.SelectedProductSKUCode != "SKU-066" {
		t.Fatalf("selected_product_sku_code = %s, want SKU-066", readModel.ProductSelection.SelectedProductSKUCode)
	}
}

func TestTaskServiceGetByIDDoesNotExposeProductSelectionForNewProductTask(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			161: {
				ID:                  161,
				SourceMode:          domain.TaskSourceModeNewProduct,
				SKUCode:             "SKU-NEW-161",
				ProductNameSnapshot: "New Product Draft",
				TaskType:            domain.TaskTypeNewProductDevelopment,
				TaskStatus:          domain.TaskStatusPendingAssign,
			},
		},
		details: map[int64]*domain.TaskDetail{
			161: {TaskID: 161},
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

	readModel, appErr := svc.GetByID(context.Background(), 161)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.ProductSelection != nil {
		t.Fatalf("product_selection = %+v, want nil", readModel.ProductSelection)
	}
}

func TestTaskServiceGetByIDOriginalProductDevelopmentChangeRequestEchoedBoth(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			162: {
				ID:                  162,
				SourceMode:          domain.TaskSourceModeExistingProduct,
				ProductID:           int64Ptr(66),
				SKUCode:             "SKU-ORIG-162",
				ProductNameSnapshot: "Original Product",
				TaskType:            domain.TaskTypeOriginalProductDevelopment,
				TaskStatus:          domain.TaskStatusPendingAssign,
			},
		},
		details: map[int64]*domain.TaskDetail{
			162: {TaskID: 162, ChangeRequest: "1234567", DesignRequirement: ""},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			162: {{TaskID: 162, SequenceNo: 1, SKUCode: "SKU-ORIG-162", SKUStatus: domain.TaskSKUStatusGenerated, DesignRequirement: ""}},
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

	readModel, appErr := svc.GetByID(context.Background(), 162)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.ChangeRequest != "1234567" {
		t.Fatalf("readModel.change_request = %q, want 1234567", readModel.ChangeRequest)
	}
	if readModel.DesignRequirement != "1234567" {
		t.Fatalf("readModel.design_requirement = %q, want 1234567", readModel.DesignRequirement)
	}
	if len(readModel.SKUItems) != 1 {
		t.Fatalf("sku_items len = %d, want 1", len(readModel.SKUItems))
	}
	if readModel.SKUItems[0].ChangeRequest != "1234567" || readModel.SKUItems[0].DesignRequirement != "1234567" {
		t.Fatalf("sku item demand fields = change_request:%q design_requirement:%q, want 1234567/1234567", readModel.SKUItems[0].ChangeRequest, readModel.SKUItems[0].DesignRequirement)
	}
}

func TestTaskServiceGetByIDNewProductDevelopmentChangeRequestOmitted(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			163: {
				ID:                  163,
				SourceMode:          domain.TaskSourceModeNewProduct,
				SKUCode:             "SKU-NEW-163",
				ProductNameSnapshot: "New Product",
				TaskType:            domain.TaskTypeNewProductDevelopment,
				TaskStatus:          domain.TaskStatusPendingAssign,
			},
		},
		details: map[int64]*domain.TaskDetail{
			163: {TaskID: 163, DesignRequirement: "11111", ChangeRequest: ""},
		},
		skuItems: map[int64][]*domain.TaskSKUItem{
			163: {{TaskID: 163, SequenceNo: 1, SKUCode: "SKU-NEW-163", SKUStatus: domain.TaskSKUStatusGenerated, DesignRequirement: "11111"}},
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

	readModel, appErr := svc.GetByID(context.Background(), 163)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.ChangeRequest != "" {
		t.Fatalf("readModel.change_request = %q, want empty", readModel.ChangeRequest)
	}
	if readModel.DesignRequirement != "11111" {
		t.Fatalf("readModel.design_requirement = %q, want 11111", readModel.DesignRequirement)
	}
	if len(readModel.SKUItems) != 1 {
		t.Fatalf("sku_items len = %d, want 1", len(readModel.SKUItems))
	}
	if readModel.SKUItems[0].ChangeRequest != "" || readModel.SKUItems[0].DesignRequirement != "11111" {
		t.Fatalf("sku item demand fields = change_request:%q design_requirement:%q, want empty/11111", readModel.SKUItems[0].ChangeRequest, readModel.SKUItems[0].DesignRequirement)
	}
}

func TestTaskServiceUpdateBusinessInfoSeparatesManualOverrideFromEstimatedCost(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   2,
		CategoryCode: "KT_CUSTOM",
		CategoryName: "KT Custom",
		DisplayName:  "KT Custom",
		CategoryType: domain.CategoryTypeBoard,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:       2,
			RuleVersion:  3,
			RuleName:     "KT Custom Base",
			CategoryCode: "KT_CUSTOM",
			RuleType:     domain.CostRuleTypeFixedUnitPrice,
			BasePrice:    float64Ptr(8),
			Priority:     10,
			IsActive:     true,
			Source:       "phase_021_test",
		},
	}
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			102: {ID: 102, TaskType: domain.TaskTypePurchaseTask},
		},
		details: map[int64]*domain.TaskDetail{
			102: {TaskID: 102},
		},
	}
	overrideAuditRepo := &prdTaskCostOverrideEventRepo{}

	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		overrideAuditRepo,
		&prdWarehouseRepo{},
		categoryRepo,
		costRuleRepo,
		prdCodeRuleService{},
		step04TxRunner{},
	)

	detail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:                   102,
		OperatorID:               9,
		CategoryCode:             "KT_CUSTOM",
		Area:                     float64Ptr(2),
		CostPrice:                float64Ptr(25),
		ManualCostOverride:       true,
		ManualCostOverrideReason: "supplier special case",
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if detail.EstimatedCost == nil || *detail.EstimatedCost != 16 {
		t.Fatalf("estimated_cost = %+v, want 16", detail.EstimatedCost)
	}
	if detail.CostPrice == nil || *detail.CostPrice != 25 {
		t.Fatalf("cost_price = %+v, want 25", detail.CostPrice)
	}
	if !detail.ManualCostOverride || detail.ManualCostOverrideReason != "supplier special case" {
		t.Fatalf("manual override state = %+v", detail)
	}
	if detail.OverrideActor != "operator:9" || detail.OverrideAt == nil {
		t.Fatalf("override trace = %s / %+v", detail.OverrideActor, detail.OverrideAt)
	}
	if len(overrideAuditRepo.events[102]) != 1 {
		t.Fatalf("override audit events = %+v", overrideAuditRepo.events)
	}
	event := overrideAuditRepo.events[102][0]
	if event.EventType != domain.TaskCostOverrideAuditEventApplied || event.OverrideCost == nil || *event.OverrideCost != 25 {
		t.Fatalf("override audit event = %+v", event)
	}
	if event.PreviousEstimatedCost == nil || *event.PreviousEstimatedCost != 16 {
		t.Fatalf("override audit previous_estimated_cost = %+v", event.PreviousEstimatedCost)
	}
	if event.MatchedRuleVersion == nil || *event.MatchedRuleVersion != 3 {
		t.Fatalf("override audit matched_rule_version = %+v", event.MatchedRuleVersion)
	}
}

func TestTaskServiceUpdateBusinessInfoAcceptsExternalCategoryDisplayValue(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			606: {ID: 606, TaskType: domain.TaskTypeNewProductDevelopment, SKUCode: "SKU-606", ProductNameSnapshot: "Product 606"},
		},
		details: map[int64]*domain.TaskDetail{
			606: {TaskID: 606},
		},
	}

	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		categoryRepo,
		newCostRuleRepoStub(),
		prdCodeRuleService{},
		step04TxRunner{},
	)

	detail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:       606,
		OperatorID:   1,
		CategoryCode: "激光打印",
		SpecText:     "20*20",
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if detail.Category != "激光打印" || detail.CategoryName != "激光打印" || detail.CategoryCode != "" {
		t.Fatalf("category fields = category:%q category_name:%q category_code:%q", detail.Category, detail.CategoryName, detail.CategoryCode)
	}
	if detail.SpecText != "20*20" {
		t.Fatalf("spec_text = %q, want 20*20", detail.SpecText)
	}
}

func TestTaskServiceUpdateBusinessInfoPatchesProductNameAndIID(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			607: {
				ID:                  607,
				TaskType:            domain.TaskTypeNewProductDevelopment,
				SKUCode:             "SKU-607",
				ProductNameSnapshot: "Old Product",
				TaskStatus:          domain.TaskStatusPendingAssign,
			},
		},
		details: map[int64]*domain.TaskDetail{
			607: {TaskID: 607, Category: "OLD-IID", CategoryName: "OLD-IID"},
		},
	}

	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		newCategoryRepoStub(),
		newCostRuleRepoStub(),
		prdCodeRuleService{},
		step04TxRunner{},
	)

	detail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:      607,
		OperatorID:  1,
		ProductName: "New Product",
		ProductIID:  "NEW-IID",
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if taskRepo.tasks[607].ProductNameSnapshot != "New Product" {
		t.Fatalf("product_name_snapshot = %q, want New Product", taskRepo.tasks[607].ProductNameSnapshot)
	}
	if detail.Category != "NEW-IID" || detail.CategoryName != "NEW-IID" || detail.CategoryID != nil {
		t.Fatalf("i_id fields = category:%q category_name:%q category_id:%+v", detail.Category, detail.CategoryName, detail.CategoryID)
	}
}

func TestTaskServiceUpdateBusinessInfoPatchesDemandTextByTaskType(t *testing.T) {
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			608: {
				ID:       608,
				TaskType: domain.TaskTypeNewProductDevelopment,
			},
			609: {
				ID:       609,
				TaskType: domain.TaskTypeOriginalProductDevelopment,
			},
		},
		details: map[int64]*domain.TaskDetail{
			608: {TaskID: 608, DesignRequirement: "old new-product demand"},
			609: {TaskID: 609, ChangeRequest: "old original-product change"},
		},
	}
	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		newCategoryRepoStub(),
		newCostRuleRepoStub(),
		prdCodeRuleService{},
		step04TxRunner{},
	)

	newDetail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:            608,
		OperatorID:        1,
		DesignRequirement: "updated new-product demand",
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo(new product demand) unexpected error: %+v", appErr)
	}
	if newDetail.DesignRequirement != "updated new-product demand" || newDetail.ChangeRequest != "" {
		t.Fatalf("new product demand fields = design_requirement:%q change_request:%q", newDetail.DesignRequirement, newDetail.ChangeRequest)
	}

	originalDetail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:            609,
		OperatorID:        1,
		DesignRequirement: "updated original-product change via alias",
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo(original demand) unexpected error: %+v", appErr)
	}
	if originalDetail.ChangeRequest != "updated original-product change via alias" || originalDetail.DesignRequirement != "updated original-product change via alias" {
		t.Fatalf("original product demand fields = change_request:%q design_requirement:%q", originalDetail.ChangeRequest, originalDetail.DesignRequirement)
	}
}

func TestTaskServiceUpdateBusinessInfoMarksManualReviewForManualQuote(t *testing.T) {
	categoryRepo := newCategoryRepoStub()
	costRuleRepo := newCostRuleRepoStub()
	categoryRepo.mustCreate(&domain.Category{
		CategoryID:   3,
		CategoryCode: "ACRYLIC",
		CategoryName: "Acrylic",
		DisplayName:  "Acrylic",
		CategoryType: domain.CategoryTypeMaterial,
		IsActive:     true,
		Level:        1,
	})
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:       3,
			RuleVersion:  5,
			RuleName:     "Acrylic Manual Quote",
			CategoryCode: "ACRYLIC",
			RuleType:     domain.CostRuleTypeManualQuote,
			Priority:     10,
			IsActive:     true,
			Source:       "phase_021_test",
		},
	}
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			103: {ID: 103, TaskType: domain.TaskTypePurchaseTask},
		},
		details: map[int64]*domain.TaskDetail{
			103: {TaskID: 103},
		},
	}

	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		categoryRepo,
		costRuleRepo,
		prdCodeRuleService{},
		step04TxRunner{},
	)

	detail, appErr := svc.UpdateBusinessInfo(context.Background(), UpdateTaskBusinessInfoParams{
		TaskID:       103,
		OperatorID:   9,
		CategoryCode: "ACRYLIC",
	})
	if appErr != nil {
		t.Fatalf("UpdateBusinessInfo() unexpected error: %+v", appErr)
	}
	if !detail.RequiresManualReview {
		t.Fatalf("requires_manual_review = false, want true")
	}
	if detail.EstimatedCost != nil || detail.CostPrice != nil {
		t.Fatalf("prefill cost state = estimated %+v cost %+v, want nil/nil", detail.EstimatedCost, detail.CostPrice)
	}
	if detail.CostRuleSource != "phase_021_test" {
		t.Fatalf("cost_rule_source = %s, want phase_021_test", detail.CostRuleSource)
	}
	if detail.MatchedRuleVersion == nil || *detail.MatchedRuleVersion != 5 {
		t.Fatalf("matched_rule_version = %+v, want 5", detail.MatchedRuleVersion)
	}
	if detail.PrefillSource != taskCostPrefillSourcePreview || detail.PrefillAt == nil {
		t.Fatalf("prefill trace = %s / %+v", detail.PrefillSource, detail.PrefillAt)
	}
}

func TestTaskServiceGetByIDReturnsProcurementSummaryCostSignals(t *testing.T) {
	overrideAt := time.Now().UTC()
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			104: {
				ID:                  104,
				SourceMode:          domain.TaskSourceModeExistingProduct,
				ProductID:           int64Ptr(904),
				SKUCode:             "SKU-904",
				ProductNameSnapshot: "KT Custom Product",
				TaskType:            domain.TaskTypePurchaseTask,
				TaskStatus:          domain.TaskStatusPendingAssign,
			},
		},
		details: map[int64]*domain.TaskDetail{
			104: {
				TaskID:                   104,
				Category:                 "KT Custom",
				CategoryCode:             "KT_CUSTOM",
				CategoryName:             "KT Custom",
				SpecText:                 "board",
				CostPrice:                float64Ptr(25),
				EstimatedCost:            float64Ptr(16),
				CostRuleID:               int64Ptr(2),
				CostRuleName:             "KT Custom Base",
				CostRuleSource:           "phase_021_test",
				MatchedRuleVersion:       intPtr(3),
				PrefillSource:            taskCostPrefillSourcePreview,
				PrefillAt:                timePtr(),
				RequiresManualReview:     true,
				ManualCostOverride:       true,
				ManualCostOverrideReason: "supplier special case",
				OverrideActor:            "operator:9",
				OverrideAt:               &overrideAt,
				SourceProductID:          int64Ptr(904),
				SourceProductName:        "KT Custom Product",
				SourceSearchEntryCode:    "KT_STANDARD",
				SourceMatchType:          taskProductSelectionMatchMapped,
				SourceMatchRule:          "KT",
				MatchedCategoryCode:      "KT_CUSTOM",
				MatchedSearchEntryCode:   "KT_STANDARD",
				FiledAt:                  timePtr(),
			},
		},
	}
	costRuleRepo := newCostRuleRepoStub()
	costRuleRepo.rules = []*domain.CostRule{
		{
			RuleID:       1,
			RuleVersion:  2,
			RuleName:     "KT Custom Base V2",
			CategoryCode: "KT_CUSTOM",
			RuleType:     domain.CostRuleTypeFixedUnitPrice,
			BasePrice:    float64Ptr(14),
			Priority:     10,
			IsActive:     true,
			Source:       "phase_021_test",
		},
		{
			RuleID:           2,
			RuleVersion:      3,
			RuleName:         "KT Custom Base V3",
			CategoryCode:     "KT_CUSTOM",
			RuleType:         domain.CostRuleTypeFixedUnitPrice,
			BasePrice:        float64Ptr(16),
			Priority:         10,
			IsActive:         true,
			SupersedesRuleID: int64Ptr(1),
			Source:           "phase_021_test",
		},
		{
			RuleID:           3,
			RuleVersion:      4,
			RuleName:         "KT Custom Base V4",
			CategoryCode:     "KT_CUSTOM",
			RuleType:         domain.CostRuleTypeFixedUnitPrice,
			BasePrice:        float64Ptr(18),
			Priority:         10,
			IsActive:         true,
			SupersedesRuleID: int64Ptr(2),
			Source:           "phase_041_test",
		},
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"cost_price":                  25.0,
		"manual_cost_override":        true,
		"manual_cost_override_reason": "supplier special case",
		"override_actor":              "operator:9",
		"override_at":                 overrideAt,
	})
	eventRepo := &prdTaskEventRepo{
		events: []*domain.TaskEvent{
			{
				ID:        "evt-104-1",
				TaskID:    104,
				Sequence:  1,
				EventType: domain.TaskEventBusinessInfoUpdated,
				Payload:   payload,
				CreatedAt: overrideAt,
			},
		},
	}
	overrideAuditRepo := &prdTaskCostOverrideEventRepo{
		events: map[int64][]*domain.TaskCostOverrideAuditEvent{
			104: {
				{
					EventID:               "cov-104-1",
					TaskID:                104,
					Sequence:              1,
					EventType:             domain.TaskCostOverrideAuditEventApplied,
					CategoryCode:          "KT_CUSTOM",
					MatchedRuleID:         int64Ptr(2),
					MatchedRuleVersion:    intPtr(3),
					MatchedRuleSource:     "phase_021_test",
					GovernanceStatus:      domain.CostRuleGovernanceStatusEffective,
					PreviousEstimatedCost: float64Ptr(16),
					PreviousCostPrice:     float64Ptr(16),
					OverrideCost:          float64Ptr(25),
					ResultCostPrice:       float64Ptr(25),
					OverrideReason:        "supplier special case",
					OverrideActor:         "operator:9",
					OverrideAt:            overrideAt,
					Source:                taskCostOverrideAuditSourceBusinessInfo,
					Note:                  "manual special case",
					CreatedAt:             overrideAt,
				},
			},
		},
	}
	procurementRepo := &prdProcurementRepo{
		records: map[int64]*domain.ProcurementRecord{
			104: {
				TaskID:           104,
				Status:           domain.ProcurementStatusCompleted,
				ProcurementPrice: float64Ptr(18),
				Quantity:         int64Ptr(3),
				SupplierName:     "Vendor Z",
			},
		},
	}

	svc := NewTaskServiceWithCatalog(
		taskRepo,
		procurementRepo,
		&prdTaskAssetRepo{},
		eventRepo,
		overrideAuditRepo,
		&prdWarehouseRepo{},
		nil,
		costRuleRepo,
		prdCodeRuleService{},
		step04TxRunner{},
	)

	readModel, appErr := svc.GetByID(context.Background(), 104)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.ProcurementSummary == nil {
		t.Fatal("procurement_summary = nil")
	}
	if readModel.ProcurementSummary.CostRuleSource != "phase_021_test" || !readModel.ProcurementSummary.ManualCostOverride {
		t.Fatalf("procurement_summary = %+v", readModel.ProcurementSummary)
	}
	if readModel.ProcurementSummary.MatchedRuleVersion == nil || *readModel.ProcurementSummary.MatchedRuleVersion != 3 {
		t.Fatalf("procurement_summary matched_rule_version = %+v, want 3", readModel.ProcurementSummary.MatchedRuleVersion)
	}
	if readModel.ProcurementSummary.PrefillSource != taskCostPrefillSourcePreview || readModel.ProcurementSummary.PrefillAt == nil {
		t.Fatalf("procurement_summary prefill trace = %s / %+v", readModel.ProcurementSummary.PrefillSource, readModel.ProcurementSummary.PrefillAt)
	}
	if readModel.ProcurementSummary.OverrideActor != "operator:9" || readModel.ProcurementSummary.OverrideAt == nil {
		t.Fatalf("procurement_summary override trace = %s / %+v", readModel.ProcurementSummary.OverrideActor, readModel.ProcurementSummary.OverrideAt)
	}
	if readModel.ProcurementSummary.CategoryCode != "KT_CUSTOM" || readModel.ProcurementSummary.EstimatedCost == nil || *readModel.ProcurementSummary.EstimatedCost != 16 {
		t.Fatalf("procurement_summary cost signals = %+v", readModel.ProcurementSummary)
	}
	if readModel.ProcurementSummary.ProductSelection == nil {
		t.Fatal("procurement_summary.product_selection = nil")
	}
	if readModel.ProcurementSummary.ProductSelection.SelectedProductSKUCode != "SKU-904" || readModel.ProcurementSummary.ProductSelection.SourceMatchRule != "KT" {
		t.Fatalf("procurement_summary.product_selection = %+v", readModel.ProcurementSummary.ProductSelection)
	}
	if readModel.MatchedRuleGovernance == nil || readModel.MatchedRuleGovernance.MatchedRule == nil {
		t.Fatalf("matched_rule_governance = %+v", readModel.MatchedRuleGovernance)
	}
	if !readModel.MatchedRuleGovernance.IsRuleOutdated || readModel.MatchedRuleGovernance.CurrentRule == nil || readModel.MatchedRuleGovernance.CurrentRule.RuleID != 3 {
		t.Fatalf("matched_rule_governance current/outdated = %+v", readModel.MatchedRuleGovernance)
	}
	if readModel.MatchedRuleGovernance.VersionChainSummary == nil || readModel.MatchedRuleGovernance.VersionChainSummary.TotalVersions != 3 {
		t.Fatalf("matched_rule_governance summary = %+v", readModel.MatchedRuleGovernance.VersionChainSummary)
	}
	if readModel.MatchedRuleGovernance.CurrentRuleVersionHint == nil || *readModel.MatchedRuleGovernance.CurrentRuleVersionHint != 4 {
		t.Fatalf("matched_rule_governance current_rule_version_hint = %+v", readModel.MatchedRuleGovernance.CurrentRuleVersionHint)
	}
	if readModel.OverrideSummary == nil || !readModel.OverrideSummary.CurrentOverrideActive || readModel.OverrideSummary.OverrideEventCount != 1 {
		t.Fatalf("override_summary = %+v", readModel.OverrideSummary)
	}
	if readModel.OverrideSummary.HistorySource != taskCostOverrideAuditHistorySource {
		t.Fatalf("override_summary.history_source = %s, want %s", readModel.OverrideSummary.HistorySource, taskCostOverrideAuditHistorySource)
	}
	if readModel.OverrideSummary.LatestOverrideEvent == nil || readModel.OverrideSummary.LatestOverrideEvent.Actor != "operator:9" {
		t.Fatalf("latest_override_event = %+v", readModel.OverrideSummary.LatestOverrideEvent)
	}
	if readModel.OverrideSummary.LatestOverrideEvent.MatchedRuleVersion == nil || *readModel.OverrideSummary.LatestOverrideEvent.MatchedRuleVersion != 3 {
		t.Fatalf("latest_override_event matched_rule_version = %+v", readModel.OverrideSummary.LatestOverrideEvent)
	}
	if readModel.GovernanceAuditSummary == nil || readModel.GovernanceAuditSummary.EventCount != 1 {
		t.Fatalf("governance_audit_summary = %+v", readModel.GovernanceAuditSummary)
	}
	if readModel.ProcurementSummary.MatchedRuleGovernance == nil || !readModel.ProcurementSummary.MatchedRuleGovernance.IsRuleOutdated {
		t.Fatalf("procurement_summary.matched_rule_governance = %+v", readModel.ProcurementSummary.MatchedRuleGovernance)
	}
	if readModel.ProcurementSummary.OverrideSummary == nil || readModel.ProcurementSummary.OverrideSummary.OverrideEventCount != 1 {
		t.Fatalf("procurement_summary.override_summary = %+v", readModel.ProcurementSummary.OverrideSummary)
	}
	if readModel.ProcurementSummary.GovernanceAuditSummary == nil || readModel.ProcurementSummary.GovernanceAuditSummary.LatestEventID != "cov-104-1" {
		t.Fatalf("procurement_summary.governance_audit_summary = %+v", readModel.ProcurementSummary.GovernanceAuditSummary)
	}
}

func TestTaskServiceAdvanceProcurementTransitionsLifecycle(t *testing.T) {
	procurementRepo := &prdProcurementRepo{
		records: map[int64]*domain.ProcurementRecord{
			7: {
				TaskID:           7,
				Status:           domain.ProcurementStatusDraft,
				ProcurementPrice: float64Ptr(20.5),
				Quantity:         int64Ptr(100),
				SupplierName:     "Vendor B",
			},
		},
	}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskService(
		&prdTaskRepo{
			tasks: map[int64]*domain.Task{
				7: {
					ID:       7,
					TaskType: domain.TaskTypePurchaseTask,
				},
			},
		},
		procurementRepo,
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	record, appErr := svc.AdvanceProcurement(context.Background(), AdvanceTaskProcurementParams{
		TaskID:     7,
		OperatorID: 15,
		Action:     domain.ProcurementActionPrepare,
	})
	if appErr != nil {
		t.Fatalf("AdvanceProcurement(prepare) unexpected error: %+v", appErr)
	}
	if record.Status != domain.ProcurementStatusPrepared {
		t.Fatalf("AdvanceProcurement(prepare) status = %s, want %s", record.Status, domain.ProcurementStatusPrepared)
	}

	record, appErr = svc.AdvanceProcurement(context.Background(), AdvanceTaskProcurementParams{
		TaskID:     7,
		OperatorID: 15,
		Action:     domain.ProcurementActionStart,
	})
	if appErr != nil {
		t.Fatalf("AdvanceProcurement(start) unexpected error: %+v", appErr)
	}
	if record.Status != domain.ProcurementStatusInProgress {
		t.Fatalf("AdvanceProcurement(start) status = %s, want %s", record.Status, domain.ProcurementStatusInProgress)
	}

	record, appErr = svc.AdvanceProcurement(context.Background(), AdvanceTaskProcurementParams{
		TaskID:     7,
		OperatorID: 15,
		Action:     domain.ProcurementActionComplete,
	})
	if appErr != nil {
		t.Fatalf("AdvanceProcurement(complete) unexpected error: %+v", appErr)
	}
	if record.Status != domain.ProcurementStatusCompleted {
		t.Fatalf("AdvanceProcurement(complete) status = %s, want %s", record.Status, domain.ProcurementStatusCompleted)
	}
	if len(eventRepo.events) != 3 || eventRepo.events[2].EventType != domain.TaskEventProcurementAdvanced {
		t.Fatalf("AdvanceProcurement() events = %+v", eventRepo.events)
	}
}

func TestTaskServiceListBuildsWorkflowFilterAndProcurementSummary(t *testing.T) {
	taskRepo := &prdTaskRepo{
		listItems: []*domain.TaskListItem{
			{
				ID:                     8,
				TaskNo:                 "RW-008",
				ProductID:              int64Ptr(508),
				SKUCode:                "SKU-008",
				ProductNameSnapshot:    "Accessory Product",
				TaskType:               domain.TaskTypePurchaseTask,
				SourceMode:             domain.TaskSourceModeExistingProduct,
				CreatorID:              1,
				TaskStatus:             domain.TaskStatusPendingAssign,
				Category:               "Accessory",
				CategoryCode:           "ACC",
				CategoryName:           "Accessory",
				SourceProductID:        int64Ptr(508),
				SourceProductName:      "Accessory Product",
				SourceSearchEntryCode:  "ACC",
				SourceMatchType:        taskProductSelectionMatchMapped,
				SourceMatchRule:        "accessory",
				MatchedCategoryCode:    "ACC",
				MatchedSearchEntryCode: "ACC",
				SpecText:               "Spec-C",
				CostPrice:              float64Ptr(5.5),
				FiledAt:                timePtr(),
				ProcurementStatus:      procurementStatusPtr(domain.ProcurementStatusCompleted),
				ProcurementPrice:       float64Ptr(11.2),
				ProcurementQuantity:    int64Ptr(12),
				SupplierName:           "Vendor C",
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

	items, _, appErr := svc.List(context.Background(), TaskFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			MainStatuses: []domain.TaskMainStatus{domain.TaskMainStatusFiled},
			SubStatusCodes: []domain.TaskSubStatusCode{
				domain.TaskSubStatusReady,
			},
			SubStatusScope: func() *domain.TaskSubStatusScope {
				scope := domain.TaskSubStatusScopeProcurement
				return &scope
			}(),
		},
	})
	if appErr != nil {
		t.Fatalf("List() unexpected error: %+v", appErr)
	}
	if len(taskRepo.lastListFilter.MainStatuses) != 1 || taskRepo.lastListFilter.MainStatuses[0] != domain.TaskMainStatusFiled {
		t.Fatalf("List() main_statuses filter = %+v", taskRepo.lastListFilter.MainStatuses)
	}
	if len(taskRepo.lastListFilter.SubStatusCodes) != 1 || taskRepo.lastListFilter.SubStatusCodes[0] != domain.TaskSubStatusReady {
		t.Fatalf("List() sub_status_codes filter = %+v", taskRepo.lastListFilter.SubStatusCodes)
	}
	if taskRepo.lastListFilter.SubStatusScope == nil || *taskRepo.lastListFilter.SubStatusScope != domain.TaskSubStatusScopeProcurement {
		t.Fatalf("List() sub_status_scope = %+v", taskRepo.lastListFilter.SubStatusScope)
	}
	if len(items) != 1 || items[0].ProcurementSummary == nil || items[0].ProcurementSummary.SupplierName != "Vendor C" {
		t.Fatalf("List() items = %+v", items)
	}
	if items[0].ProductSelection == nil {
		t.Fatal("List() product_selection = nil")
	}
	if items[0].ProductSelection.SelectedProductID == nil || *items[0].ProductSelection.SelectedProductID != 508 {
		t.Fatalf("List() product_selection.selected_product_id = %+v", items[0].ProductSelection)
	}
	if items[0].ProcurementSummary.ProductSelection == nil || items[0].ProcurementSummary.ProductSelection.SourceSearchEntryCode != "ACC" {
		t.Fatalf("List() procurement_summary.product_selection = %+v", items[0].ProcurementSummary.ProductSelection)
	}
	if items[0].ProcurementSummary.CoordinationStatus != domain.ProcurementCoordinationStatusReadyForWarehouse {
		t.Fatalf("List() coordination_status = %s, want %s", items[0].ProcurementSummary.CoordinationStatus, domain.ProcurementCoordinationStatusReadyForWarehouse)
	}
	if !items[0].ProcurementSummary.WarehousePrepareReady {
		t.Fatalf("List() warehouse_prepare_ready = false, want true")
	}
}

func TestTaskServiceListSupportsDerivedBoardFilters(t *testing.T) {
	taskRepo := &prdTaskRepo{
		listItems: []*domain.TaskListItem{
			{
				ID:                  21,
				TaskNo:              "RW-021",
				SKUCode:             "SKU-021",
				TaskType:            domain.TaskTypePurchaseTask,
				SourceMode:          domain.TaskSourceModeExistingProduct,
				CreatorID:           1,
				TaskStatus:          domain.TaskStatusPendingAssign,
				UpdatedAt:           *timePtr(),
				Category:            "Accessory",
				SpecText:            "Spec-A",
				CostPrice:           float64Ptr(8.8),
				FiledAt:             timePtr(),
				ProcurementStatus:   procurementStatusPtr(domain.ProcurementStatusCompleted),
				ProcurementPrice:    float64Ptr(5.5),
				ProcurementQuantity: int64Ptr(10),
				SupplierName:        "Vendor A",
			},
			{
				ID:                  22,
				TaskNo:              "RW-022",
				SKUCode:             "SKU-022",
				TaskType:            domain.TaskTypePurchaseTask,
				SourceMode:          domain.TaskSourceModeExistingProduct,
				CreatorID:           1,
				TaskStatus:          domain.TaskStatusPendingAssign,
				UpdatedAt:           *timePtr(),
				Category:            "Accessory",
				SpecText:            "Spec-B",
				CostPrice:           float64Ptr(7.7),
				FiledAt:             timePtr(),
				ProcurementStatus:   procurementStatusPtr(domain.ProcurementStatusInProgress),
				ProcurementPrice:    float64Ptr(5.5),
				ProcurementQuantity: int64Ptr(10),
				SupplierName:        "Vendor B",
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

	trueValue := true
	items, pagination, appErr := svc.List(context.Background(), TaskFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			TaskTypes: []domain.TaskType{
				domain.TaskTypePurchaseTask,
			},
			CoordinationStatuses: []domain.ProcurementCoordinationStatus{
				domain.ProcurementCoordinationStatusReadyForWarehouse,
			},
			WarehousePrepareReady: &trueValue,
		},
	})
	if appErr != nil {
		t.Fatalf("List() unexpected error: %+v", appErr)
	}
	if pagination.Total != 1 {
		t.Fatalf("List() pagination.total = %d, want 1", pagination.Total)
	}
	if len(items) != 1 || items[0].ID != 21 {
		t.Fatalf("List() derived filter items = %+v", items)
	}
	if taskRepo.listCalls != 1 {
		t.Fatalf("List() repo calls = %d, want 1", taskRepo.listCalls)
	}
}

func TestTaskServiceCreateBatchNewProductDevelopmentSucceeds(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	eventRepo := &prdTaskEventRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		eventRepo,
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:     domain.TaskTypeNewProductDevelopment,
		SourceMode:   domain.TaskSourceModeNewProduct,
		CreatorID:    9,
		OwnerTeam:    domain.AllValidTeams()[0],
		DeadlineAt:   timePtr(),
		BatchSKUMode: "multiple",
		BatchItems: []CreateTaskBatchSKUItemParams{
			{
				ProductName:       "New Lightbox A",
				ProductShortName:  "Lightbox-A",
				CategoryCode:      "LIGHTBOX",
				MaterialMode:      string(domain.MaterialModePreset),
				DesignRequirement: "need design A",
				NewSKU:            "BATCH-NEW-001",
			},
			{
				ProductName:       "New Lightbox B",
				ProductShortName:  "Lightbox-B",
				CategoryCode:      "LIGHTBOX",
				MaterialMode:      string(domain.MaterialModeOther),
				DesignRequirement: "need design B",
				NewSKU:            "BATCH-NEW-002",
				VariantJSON:       json.RawMessage(`{"color":"red"}`),
			},
		},
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if !task.IsBatchTask {
		t.Fatal("Create() is_batch_task = false, want true")
	}
	if task.BatchItemCount != 2 {
		t.Fatalf("Create() batch_item_count = %d, want 2", task.BatchItemCount)
	}
	if task.BatchMode != domain.TaskBatchModeMultiSKU {
		t.Fatalf("Create() batch_mode = %s, want %s", task.BatchMode, domain.TaskBatchModeMultiSKU)
	}
	if task.PrimarySKUCode == "" {
		t.Fatal("Create() primary_sku_code is empty")
	}
	if task.SKUGenerationStatus != domain.TaskSKUGenerationStatusCompleted {
		t.Fatalf("Create() sku_generation_status = %s, want completed", task.SKUGenerationStatus)
	}

	items := taskRepo.skuItems[task.ID]
	if len(items) != 2 {
		t.Fatalf("task_sku_items len = %d, want 2", len(items))
	}
	if items[0].SKUCode == items[1].SKUCode {
		t.Fatalf("task_sku_items sku duplicated: %s", items[0].SKUCode)
	}
	if task.PrimarySKUCode != items[0].SKUCode {
		t.Fatalf("primary_sku_code = %s, want %s", task.PrimarySKUCode, items[0].SKUCode)
	}
	hasCreated := false
	hasBatchCreated := false
	for _, event := range eventRepo.events {
		if event == nil {
			continue
		}
		if event.EventType == domain.TaskEventCreated {
			hasCreated = true
		}
		if event.EventType == domain.TaskEventBatchItemsCreated {
			hasBatchCreated = true
		}
	}
	if !hasCreated || !hasBatchCreated {
		t.Fatalf("event types = %+v", eventRepo.events)
	}

	readModel, appErr := svc.GetByID(context.Background(), task.ID)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if len(readModel.SKUItems) != 2 {
		t.Fatalf("GetByID() sku_items len = %d, want 2", len(readModel.SKUItems))
	}
}

func TestTaskServiceCreateBatchPurchaseTaskSucceeds(t *testing.T) {
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
		step04TxRunner{},
	)

	task, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:     domain.TaskTypePurchaseTask,
		SourceMode:   domain.TaskSourceModeNewProduct,
		CreatorID:    9,
		OwnerTeam:    domain.AllValidTeams()[0],
		DeadlineAt:   timePtr(),
		BatchSKUMode: "multiple",
		BatchItems: []CreateTaskBatchSKUItemParams{
			{
				ProductName:   "Purchase Pack A",
				CategoryCode:  "KT",
				PurchaseSKU:   "BATCH-PUR-001",
				CostPriceMode: string(domain.CostPriceModeTemplate),
				Quantity:      int64Ptr(5),
				BaseSalePrice: float64Ptr(19.5),
			},
			{
				ProductName:   "Purchase Pack B",
				CategoryCode:  "KT",
				PurchaseSKU:   "BATCH-PUR-002",
				CostPriceMode: string(domain.CostPriceModeManual),
				Quantity:      int64Ptr(6),
				BaseSalePrice: float64Ptr(29.5),
				VariantJSON:   json.RawMessage(`{"size":"L"}`),
			},
		},
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if !task.IsBatchTask || task.BatchItemCount != 2 {
		t.Fatalf("task batch meta = %+v", task)
	}
	if procurementRepo.records == nil || procurementRepo.records[task.ID] == nil {
		t.Fatal("procurement record not initialized")
	}
	if len(procurementRepo.items[task.ID]) != 2 {
		t.Fatalf("procurement items len = %d, want 2", len(procurementRepo.items[task.ID]))
	}
}

func TestTaskServiceCreateBatchRejectsOriginalProductDevelopment(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:     domain.TaskTypeOriginalProductDevelopment,
		SourceMode:   domain.TaskSourceModeExistingProduct,
		CreatorID:    9,
		OwnerTeam:    domain.AllValidTeams()[0],
		DeadlineAt:   timePtr(),
		BatchSKUMode: "multiple",
		BatchItems: []CreateTaskBatchSKUItemParams{
			{ProductName: "Original-1"},
			{ProductName: "Original-2"},
		},
	})
	if appErr == nil {
		t.Fatal("Create() expected error, got nil")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
}

func TestTaskServiceCreateBatchRejectsEmptyBatchItems(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:     domain.TaskTypePurchaseTask,
		SourceMode:   domain.TaskSourceModeNewProduct,
		CreatorID:    9,
		OwnerTeam:    domain.AllValidTeams()[0],
		DeadlineAt:   timePtr(),
		BatchSKUMode: "multiple",
	})
	if appErr == nil {
		t.Fatal("Create() expected error for empty batch_items")
	}
}

func TestTaskServiceCreateBatchRejectsDuplicateDedupeKey(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:     domain.TaskTypeNewProductDevelopment,
		SourceMode:   domain.TaskSourceModeNewProduct,
		CreatorID:    9,
		OwnerTeam:    domain.AllValidTeams()[0],
		DeadlineAt:   timePtr(),
		BatchSKUMode: "multiple",
		BatchItems: []CreateTaskBatchSKUItemParams{
			{
				ProductName:       "Same Name",
				ProductShortName:  "Same Short",
				CategoryCode:      "LIGHTBOX",
				MaterialMode:      string(domain.MaterialModePreset),
				DesignRequirement: "same",
				VariantJSON:       json.RawMessage(`{"color":"blue"}`),
			},
			{
				ProductName:       "Same Name",
				ProductShortName:  "Same Short",
				CategoryCode:      "LIGHTBOX",
				MaterialMode:      string(domain.MaterialModePreset),
				DesignRequirement: "different text ignored in dedupe",
				VariantJSON:       json.RawMessage(`{"color":"blue"}`),
			},
		},
	})
	if appErr == nil {
		t.Fatal("Create() expected duplicate dedupe_key error")
	}
}

func TestTaskServiceCreateBatchRejectsMixedTopLevelSingleSKUField(t *testing.T) {
	svc := NewTaskService(
		&prdTaskRepo{},
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
	)

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:       domain.TaskTypeNewProductDevelopment,
		SourceMode:     domain.TaskSourceModeNewProduct,
		CreatorID:      9,
		OwnerTeam:      domain.AllValidTeams()[0],
		DeadlineAt:     timePtr(),
		BatchSKUMode:   "multiple",
		TopLevelNewSKU: "TOP-ONLY",
		BatchItems: []CreateTaskBatchSKUItemParams{
			{ProductName: "A", ProductShortName: "A", CategoryCode: "LIGHTBOX", MaterialMode: string(domain.MaterialModePreset), DesignRequirement: "req-a", NewSKU: "CONFLICT-001"},
			{ProductName: "B", ProductShortName: "B", CategoryCode: "LIGHTBOX", MaterialMode: string(domain.MaterialModePreset), DesignRequirement: "req-b", NewSKU: "CONFLICT-002"},
		},
	})
	if appErr == nil {
		t.Fatal("Create() expected mixed top-level single sku error")
	}
}

func TestTaskServiceCreateBatchRejectsExistingManualSKU(t *testing.T) {
	taskRepo := &prdTaskRepo{
		skuByCode: map[string]*domain.TaskSKUItem{
			"DUP-SKU": {SKUCode: "DUP-SKU"},
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

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:     domain.TaskTypePurchaseTask,
		SourceMode:   domain.TaskSourceModeNewProduct,
		CreatorID:    9,
		OwnerTeam:    domain.AllValidTeams()[0],
		DeadlineAt:   timePtr(),
		BatchSKUMode: "multiple",
		BatchItems: []CreateTaskBatchSKUItemParams{
			{ProductName: "A", PurchaseSKU: "DUP-SKU", CostPriceMode: string(domain.CostPriceModeTemplate), Quantity: int64Ptr(1), BaseSalePrice: float64Ptr(1)},
			{ProductName: "B", CostPriceMode: string(domain.CostPriceModeTemplate), Quantity: int64Ptr(1), BaseSalePrice: float64Ptr(2)},
		},
	})
	if appErr == nil {
		t.Fatal("Create() expected duplicate manual sku error")
	}
}

func TestTaskServiceCreateBatchUniqueConflictReturnsInvalidRequest(t *testing.T) {
	taskRepo := &batchConflictTaskRepo{prdTaskRepo: prdTaskRepo{}}
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

	_, appErr := svc.Create(context.Background(), CreateTaskParams{
		TaskType:     domain.TaskTypeNewProductDevelopment,
		SourceMode:   domain.TaskSourceModeNewProduct,
		CreatorID:    9,
		OwnerTeam:    domain.AllValidTeams()[0],
		DeadlineAt:   timePtr(),
		BatchSKUMode: "multiple",
		BatchItems: []CreateTaskBatchSKUItemParams{
			{ProductName: "A", ProductShortName: "A", CategoryCode: "LIGHTBOX", MaterialMode: string(domain.MaterialModePreset), DesignRequirement: "req-a"},
			{ProductName: "B", ProductShortName: "B", CategoryCode: "LIGHTBOX", MaterialMode: string(domain.MaterialModePreset), DesignRequirement: "req-b"},
		},
	})
	if appErr == nil {
		t.Fatal("Create() expected duplicate key error")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
}

type prdTaskRepo struct {
	tasks          map[int64]*domain.Task
	details        map[int64]*domain.TaskDetail
	skuItems       map[int64][]*domain.TaskSKUItem
	skuByCode      map[string]*domain.TaskSKUItem
	listItems      []*domain.TaskListItem
	lastListFilter repo.TaskListFilter
	listCalls      int
}

type prdProcurementRepo struct {
	records map[int64]*domain.ProcurementRecord
	items   map[int64][]*domain.ProcurementRecordItem
}

func (r *prdProcurementRepo) GetByTaskID(_ context.Context, taskID int64) (*domain.ProcurementRecord, error) {
	if r.records == nil {
		return nil, nil
	}
	return r.records[taskID], nil
}

func (r *prdProcurementRepo) Upsert(_ context.Context, _ repo.Tx, record *domain.ProcurementRecord) error {
	if r.records == nil {
		r.records = map[int64]*domain.ProcurementRecord{}
	}
	copied := *record
	if copied.ID == 0 {
		copied.ID = int64(len(r.records) + 1)
	}
	record.ID = copied.ID
	r.records[record.TaskID] = &copied
	return nil
}

func (r *prdProcurementRepo) ListItemsByTaskID(_ context.Context, taskID int64) ([]*domain.ProcurementRecordItem, error) {
	if r.items == nil {
		return []*domain.ProcurementRecordItem{}, nil
	}
	return r.items[taskID], nil
}

func (r *prdProcurementRepo) CreateItems(_ context.Context, _ repo.Tx, items []*domain.ProcurementRecordItem) error {
	if r.items == nil {
		r.items = map[int64][]*domain.ProcurementRecordItem{}
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		copied := *item
		if copied.ID == 0 {
			copied.ID = int64(len(r.items[item.TaskID]) + 1)
		}
		item.ID = copied.ID
		r.items[item.TaskID] = append(r.items[item.TaskID], &copied)
	}
	return nil
}

func (r *prdTaskRepo) Create(_ context.Context, _ repo.Tx, task *domain.Task, detail *domain.TaskDetail) (int64, error) {
	if r.tasks == nil {
		r.tasks = map[int64]*domain.Task{}
	}
	if r.details == nil {
		r.details = map[int64]*domain.TaskDetail{}
	}
	if task.ID == 0 {
		task.ID = int64(len(r.tasks) + 1)
	}
	r.tasks[task.ID] = task
	detail.TaskID = task.ID
	r.details[task.ID] = detail
	return task.ID, nil
}

func (r *prdTaskRepo) CreateSKUItems(_ context.Context, _ repo.Tx, items []*domain.TaskSKUItem) error {
	if r.skuItems == nil {
		r.skuItems = map[int64][]*domain.TaskSKUItem{}
	}
	if r.skuByCode == nil {
		r.skuByCode = map[string]*domain.TaskSKUItem{}
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		copied := *item
		if copied.ID == 0 {
			copied.ID = int64(len(r.skuItems[item.TaskID]) + 1)
		}
		item.ID = copied.ID
		r.skuItems[item.TaskID] = append(r.skuItems[item.TaskID], &copied)
		r.skuByCode[item.SKUCode] = &copied
	}
	return nil
}

func (r *prdTaskRepo) GetByID(_ context.Context, id int64) (*domain.Task, error) {
	return r.tasks[id], nil
}

func (r *prdTaskRepo) GetDetailByTaskID(_ context.Context, taskID int64) (*domain.TaskDetail, error) {
	return r.details[taskID], nil
}

func (r *prdTaskRepo) GetSKUItemBySKUCode(_ context.Context, skuCode string) (*domain.TaskSKUItem, error) {
	if r.skuByCode == nil {
		return nil, nil
	}
	return r.skuByCode[skuCode], nil
}

func (r *prdTaskRepo) ListSKUItemsByTaskID(_ context.Context, taskID int64) ([]*domain.TaskSKUItem, error) {
	if r.skuItems == nil {
		return []*domain.TaskSKUItem{}, nil
	}
	return r.skuItems[taskID], nil
}

func (r *prdTaskRepo) List(_ context.Context, filter repo.TaskListFilter) ([]*domain.TaskListItem, int64, error) {
	r.lastListFilter = filter
	r.listCalls++
	if r.listItems != nil {
		taskFilter := TaskFilter{
			TaskQueryFilterDefinition: filter.TaskQueryFilterDefinition,
			CreatorID:                 filter.CreatorID,
			DesignerID:                filter.DesignerID,
			NeedOutsource:             filter.NeedOutsource,
			Overdue:                   filter.Overdue,
			Keyword:                   filter.Keyword,
			Page:                      filter.Page,
			PageSize:                  filter.PageSize,
		}
		filtered := make([]*domain.TaskListItem, 0, len(r.listItems))
		for _, item := range r.listItems {
			if item == nil {
				continue
			}
			copied := *item
			copied.Workflow = buildTaskWorkflowSnapshotFromListItem(&copied)
			copied.ProcurementSummary = buildProcurementSummaryFromListItem(&copied)
			if matchesTaskFilter(&copied, taskFilter) {
				filtered = append(filtered, &copied)
			}
		}
		pagination := buildPaginationMeta(filter.Page, filter.PageSize, int64(len(filtered)))
		start := (pagination.Page - 1) * pagination.PageSize
		if start > len(filtered) {
			start = len(filtered)
		}
		end := start + pagination.PageSize
		if end > len(filtered) {
			end = len(filtered)
		}
		return filtered[start:end], int64(len(filtered)), nil
	}
	return []*domain.TaskListItem{}, int64(len(r.tasks)), nil
}

func (r *prdTaskRepo) ListBoardCandidates(_ context.Context, filter repo.TaskBoardCandidateFilter) ([]*domain.TaskListItem, error) {
	r.lastListFilter = filter.TaskListFilter
	r.listCalls++
	if r.listItems == nil {
		return []*domain.TaskListItem{}, nil
	}

	taskFilter := TaskFilter{
		TaskQueryFilterDefinition: filter.TaskListFilter.TaskQueryFilterDefinition,
		CreatorID:                 filter.CreatorID,
		DesignerID:                filter.DesignerID,
		NeedOutsource:             filter.NeedOutsource,
		Overdue:                   filter.Overdue,
		Keyword:                   filter.Keyword,
	}

	filtered := make([]*domain.TaskListItem, 0, len(r.listItems))
	for _, item := range r.listItems {
		if item == nil {
			continue
		}
		copied := *item
		copied.Workflow = buildTaskWorkflowSnapshotFromListItem(&copied)
		copied.ProcurementSummary = buildProcurementSummaryFromListItem(&copied)
		if !matchesTaskFilter(&copied, taskFilter) {
			continue
		}
		for _, preset := range filter.CandidateFilters {
			effective, ok := mergeTaskBoardFilter(taskFilter, preset)
			if !ok {
				continue
			}
			if matchesTaskFilter(&copied, effective) {
				filtered = append(filtered, &copied)
				break
			}
		}
	}
	return filtered, nil
}

func (r *prdTaskRepo) UpdateDetailBusinessInfo(_ context.Context, _ repo.Tx, detail *domain.TaskDetail) error {
	r.details[detail.TaskID] = detail
	return nil
}

func (r *prdTaskRepo) UpdateProductBinding(_ context.Context, _ repo.Tx, task *domain.Task) error {
	r.tasks[task.ID] = task
	return nil
}

func (r *prdTaskRepo) UpdateStatus(_ context.Context, _ repo.Tx, id int64, status domain.TaskStatus) error {
	r.tasks[id].TaskStatus = status
	return nil
}

func (r *prdTaskRepo) UpdateDesigner(_ context.Context, _ repo.Tx, id int64, designerID *int64) error {
	r.tasks[id].DesignerID = designerID
	return nil
}

func (r *prdTaskRepo) UpdateHandler(_ context.Context, _ repo.Tx, id int64, handlerID *int64) error {
	r.tasks[id].CurrentHandlerID = handlerID
	return nil
}

func (r *prdTaskRepo) UpdateNeedOutsource(_ context.Context, _ repo.Tx, id int64, needOutsource bool) error {
	r.tasks[id].NeedOutsource = needOutsource
	return nil
}

func (r *prdTaskRepo) UpdateCustomizationState(_ context.Context, _ repo.Tx, id int64, lastOperatorID *int64, rejectReason, rejectCategory string) error {
	task := r.tasks[id]
	if task == nil {
		return nil
	}
	task.LastCustomizationOperatorID = lastOperatorID
	task.WarehouseRejectReason = rejectReason
	task.WarehouseRejectCategory = rejectCategory
	return nil
}

type batchConflictTaskRepo struct {
	prdTaskRepo
}

func (r *batchConflictTaskRepo) CreateSKUItems(_ context.Context, _ repo.Tx, _ []*domain.TaskSKUItem) error {
	return &mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'SKU-TEST' for key 'uq_task_sku_items_sku_code'"}
}

type prdTaskAssetRepo struct {
	assets []*domain.TaskAsset
}

func (r *prdTaskAssetRepo) Create(_ context.Context, _ repo.Tx, asset *domain.TaskAsset) (int64, error) {
	asset.ID = int64(len(r.assets) + 1)
	r.assets = append(r.assets, asset)
	return asset.ID, nil
}

func (r *prdTaskAssetRepo) GetByID(_ context.Context, id int64) (*domain.TaskAsset, error) {
	if id < 1 || int(id) > len(r.assets) {
		return nil, nil
	}
	return r.assets[id-1], nil
}

func (r *prdTaskAssetRepo) ListByTaskID(_ context.Context, taskID int64) ([]*domain.TaskAsset, error) {
	out := []*domain.TaskAsset{}
	for _, asset := range r.assets {
		if asset.TaskID == taskID {
			out = append(out, asset)
		}
	}
	return out, nil
}

func (r *prdTaskAssetRepo) ListByAssetID(_ context.Context, assetID int64) ([]*domain.TaskAsset, error) {
	out := []*domain.TaskAsset{}
	for _, asset := range r.assets {
		if asset.AssetID != nil && *asset.AssetID == assetID {
			out = append(out, asset)
		}
	}
	return out, nil
}

func (r *prdTaskAssetRepo) NextVersionNo(_ context.Context, _ repo.Tx, _ int64) (int, error) {
	return len(r.assets) + 1, nil
}

func (r *prdTaskAssetRepo) NextAssetVersionNo(_ context.Context, _ repo.Tx, assetID int64) (int, error) {
	maxVersion := 0
	for _, asset := range r.assets {
		if asset.AssetID != nil && *asset.AssetID == assetID && asset.AssetVersionNo != nil && *asset.AssetVersionNo > maxVersion {
			maxVersion = *asset.AssetVersionNo
		}
	}
	return maxVersion + 1, nil
}

type prdTaskEventRepo struct {
	events []*domain.TaskEvent
}

func (r *prdTaskEventRepo) Append(_ context.Context, _ repo.Tx, taskID int64, eventType string, operatorID *int64, payload interface{}) (*domain.TaskEvent, error) {
	raw, _ := json.Marshal(payload)
	event := &domain.TaskEvent{
		ID:         "evt",
		TaskID:     taskID,
		Sequence:   int64(len(r.events) + 1),
		EventType:  eventType,
		OperatorID: operatorID,
		Payload:    raw,
		CreatedAt:  time.Now().UTC(),
	}
	r.events = append(r.events, event)
	return event, nil
}

func (r *prdTaskEventRepo) ListByTaskID(_ context.Context, _ int64) ([]*domain.TaskEvent, error) {
	return r.events, nil
}

func (r *prdTaskEventRepo) ListRecent(_ context.Context, _ repo.TaskEventListFilter) ([]*domain.TaskEvent, int64, error) {
	return r.events, int64(len(r.events)), nil
}

type prdTaskCostOverrideEventRepo struct {
	events map[int64][]*domain.TaskCostOverrideAuditEvent
}

func (r *prdTaskCostOverrideEventRepo) Append(_ context.Context, _ repo.Tx, event *domain.TaskCostOverrideAuditEvent) (*domain.TaskCostOverrideAuditEvent, error) {
	if r.events == nil {
		r.events = map[int64][]*domain.TaskCostOverrideAuditEvent{}
	}
	copyEvent := *event
	if copyEvent.EventID == "" {
		copyEvent.EventID = "cov"
	}
	if copyEvent.Sequence == 0 {
		copyEvent.Sequence = int64(len(r.events[event.TaskID]) + 1)
	}
	r.events[event.TaskID] = append(r.events[event.TaskID], &copyEvent)
	return &copyEvent, nil
}

func (r *prdTaskCostOverrideEventRepo) ListByTaskID(_ context.Context, taskID int64) ([]*domain.TaskCostOverrideAuditEvent, error) {
	if r.events == nil {
		return []*domain.TaskCostOverrideAuditEvent{}, nil
	}
	return r.events[taskID], nil
}

func (r *prdTaskCostOverrideEventRepo) GetByEventID(_ context.Context, eventID string) (*domain.TaskCostOverrideAuditEvent, error) {
	for _, events := range r.events {
		for _, event := range events {
			if event != nil && event.EventID == eventID {
				return event, nil
			}
		}
	}
	return nil, nil
}

type prdTaskCostOverrideReviewRepo struct {
	records map[string]*domain.TaskCostOverrideReviewRecord
}

func (r *prdTaskCostOverrideReviewRepo) Upsert(_ context.Context, _ repo.Tx, record *domain.TaskCostOverrideReviewRecord) (*domain.TaskCostOverrideReviewRecord, error) {
	if r.records == nil {
		r.records = map[string]*domain.TaskCostOverrideReviewRecord{}
	}
	copyRecord := *record
	if copyRecord.RecordID == 0 {
		copyRecord.RecordID = int64(len(r.records) + 1)
	}
	if copyRecord.CreatedAt.IsZero() {
		copyRecord.CreatedAt = time.Now().UTC()
	}
	copyRecord.UpdatedAt = time.Now().UTC()
	r.records[copyRecord.OverrideEventID] = &copyRecord
	return &copyRecord, nil
}

func (r *prdTaskCostOverrideReviewRepo) GetByEventID(_ context.Context, eventID string) (*domain.TaskCostOverrideReviewRecord, error) {
	if r.records == nil {
		return nil, nil
	}
	return r.records[eventID], nil
}

func (r *prdTaskCostOverrideReviewRepo) ListByTaskID(_ context.Context, taskID int64) ([]*domain.TaskCostOverrideReviewRecord, error) {
	items := []*domain.TaskCostOverrideReviewRecord{}
	for _, record := range r.records {
		if record != nil && record.TaskID == taskID {
			items = append(items, record)
		}
	}
	return items, nil
}

type prdTaskCostFinanceFlagRepo struct {
	flags map[string]*domain.TaskCostFinanceFlag
}

func (r *prdTaskCostFinanceFlagRepo) Upsert(_ context.Context, _ repo.Tx, flag *domain.TaskCostFinanceFlag) (*domain.TaskCostFinanceFlag, error) {
	if r.flags == nil {
		r.flags = map[string]*domain.TaskCostFinanceFlag{}
	}
	copyFlag := *flag
	if copyFlag.RecordID == 0 {
		copyFlag.RecordID = int64(len(r.flags) + 1)
	}
	if copyFlag.CreatedAt.IsZero() {
		copyFlag.CreatedAt = time.Now().UTC()
	}
	copyFlag.UpdatedAt = time.Now().UTC()
	r.flags[copyFlag.OverrideEventID] = &copyFlag
	return &copyFlag, nil
}

func (r *prdTaskCostFinanceFlagRepo) GetByEventID(_ context.Context, eventID string) (*domain.TaskCostFinanceFlag, error) {
	if r.flags == nil {
		return nil, nil
	}
	return r.flags[eventID], nil
}

func (r *prdTaskCostFinanceFlagRepo) ListByTaskID(_ context.Context, taskID int64) ([]*domain.TaskCostFinanceFlag, error) {
	items := []*domain.TaskCostFinanceFlag{}
	for _, flag := range r.flags {
		if flag != nil && flag.TaskID == taskID {
			items = append(items, flag)
		}
	}
	return items, nil
}

type prdWarehouseRepo struct {
	receipts map[int64]*domain.WarehouseReceipt
}

func (r *prdWarehouseRepo) Create(_ context.Context, _ repo.Tx, receipt *domain.WarehouseReceipt) (int64, error) {
	if r.receipts == nil {
		r.receipts = map[int64]*domain.WarehouseReceipt{}
	}
	r.receipts[receipt.TaskID] = receipt
	return int64(len(r.receipts)), nil
}

func (r *prdWarehouseRepo) GetByID(_ context.Context, _ int64) (*domain.WarehouseReceipt, error) {
	return nil, nil
}

func (r *prdWarehouseRepo) GetByTaskID(_ context.Context, taskID int64) (*domain.WarehouseReceipt, error) {
	if r.receipts == nil {
		return nil, nil
	}
	return r.receipts[taskID], nil
}

func (r *prdWarehouseRepo) List(_ context.Context, _ repo.WarehouseListFilter) ([]*domain.WarehouseReceipt, int64, error) {
	return []*domain.WarehouseReceipt{}, 0, nil
}

func (r *prdWarehouseRepo) Update(_ context.Context, _ repo.Tx, receipt *domain.WarehouseReceipt) error {
	if r.receipts == nil {
		r.receipts = map[int64]*domain.WarehouseReceipt{}
	}
	r.receipts[receipt.TaskID] = receipt
	return nil
}

type prdCodeRuleService struct{}

func (prdCodeRuleService) List(context.Context) ([]*domain.CodeRule, *domain.AppError) {
	return nil, nil
}

func (prdCodeRuleService) Preview(context.Context, int64) (*domain.CodePreview, *domain.AppError) {
	return nil, nil
}

func (prdCodeRuleService) GenerateCode(_ context.Context, ruleType domain.CodeRuleType) (string, *domain.AppError) {
	if ruleType == domain.CodeRuleTypeNewSKU {
		return "SKU-TEST", nil
	}
	return "RW-TEST", nil
}

func (prdCodeRuleService) GenerateSKU(context.Context, int64) (string, *domain.AppError) {
	return "SKU-TEST", nil
}

func int64Ptr(v int64) *int64 {
	return &v
}

func intPtr(v int) *int {
	return &v
}

func procurementStatusPtr(v domain.ProcurementStatus) *domain.ProcurementStatus {
	return &v
}
