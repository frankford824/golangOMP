package service

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
)

func TestResolveTaskCanonicalOrgOwnershipUsesConfiguredDepartmentAlias(t *testing.T) {
	ConfigureTaskOrgCatalog(domain.AuthSettings{
		DepartmentTeams: map[string][]string{
			"视觉研创部": {"默认组"},
		},
	})
	defer ConfigureTaskOrgCatalog(domain.AuthSettings{})

	ownership, appErr := resolveTaskCanonicalOrgOwnership(CreateTaskParams{
		rawOwnerDepartment: string(domain.DepartmentDesignRD),
	})
	if appErr != nil {
		t.Fatalf("resolveTaskCanonicalOrgOwnership() unexpected error: %+v", appErr)
	}
	if ownership.OwnerDepartment != "视觉研创部" {
		t.Fatalf("OwnerDepartment = %q, want 视觉研创部", ownership.OwnerDepartment)
	}
}

func TestTaskServiceCreateOriginalProductWithOrgTeamCompatWritesCanonicalOwnership(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
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
	if task.OwnerDepartment != "运营部" {
		t.Fatalf("Create() owner_department = %q, want 运营部", task.OwnerDepartment)
	}
	if task.OwnerOrgTeam != "运营三组" {
		t.Fatalf("Create() owner_org_team = %q, want 运营三组", task.OwnerOrgTeam)
	}

	readModel, appErr := svc.GetByID(context.Background(), task.ID)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.OwnerDepartment != "运营部" || readModel.OwnerOrgTeam != "运营三组" {
		t.Fatalf("GetByID() canonical ownership = (%q, %q), want (运营部, 运营三组)", readModel.OwnerDepartment, readModel.OwnerOrgTeam)
	}
}

func TestTaskServiceCreateNewTaskWithOrgTeamCompatWritesCanonicalOwnership(t *testing.T) {
	cases := []struct {
		name   string
		params CreateTaskParams
	}{
		{
			name: "new_product_development",
			params: CreateTaskParams{
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
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			taskRepo := &prdTaskRepo{}
			svc := NewTaskService(
				taskRepo,
				&prdTaskAssetRepo{},
				&prdTaskEventRepo{},
				nil,
				prdCodeRuleService{},
				step04TxRunner{},
			)

			task, appErr := svc.Create(context.Background(), tc.params)
			if appErr != nil {
				t.Fatalf("Create() unexpected error: %+v", appErr)
			}
			if task.OwnerTeam != "内贸运营组" {
				t.Fatalf("Create() owner_team = %q, want 内贸运营组", task.OwnerTeam)
			}
			if task.OwnerDepartment != "运营部" {
				t.Fatalf("Create() owner_department = %q, want 运营部", task.OwnerDepartment)
			}
			if task.OwnerOrgTeam != "运营三组" {
				t.Fatalf("Create() owner_org_team = %q, want 运营三组", task.OwnerOrgTeam)
			}
		})
	}
}

func TestTaskServiceCreateLegacyOwnerTeamBackfillsCanonicalDepartmentWhenUnique(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
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
		ProductNameSnapshot: "Legacy Lightbox",
		ProductShortName:    "Legacy",
		DesignRequirement:   "need design",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.OwnerDepartment != "运营部" {
		t.Fatalf("Create() owner_department = %q, want 运营部", task.OwnerDepartment)
	}
	if task.OwnerOrgTeam != "" {
		t.Fatalf("Create() owner_org_team = %q, want empty because legacy mapping is ambiguous", task.OwnerOrgTeam)
	}
}

func TestTaskServiceCreateDerivesOwnershipFromSessionActorWhenOwnerFieldsOmitted(t *testing.T) {
	taskRepo := &prdTaskRepo{}
	svc := NewTaskService(
		taskRepo,
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		prdCodeRuleService{},
		step04TxRunner{},
	)
	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:              291,
		Username:        "ops-user",
		Department:      string(domain.DepartmentOperations),
		Team:            "淘系一组",
		Source:          domain.RequestActorSourceSessionToken,
		AuthMode:        domain.AuthModeSessionTokenRoleEnforced,
		EffectiveAccess: globalCapabilityActor(291, domain.PermissionTaskCreate).EffectiveAccess,
	})

	task, appErr := svc.Create(ctx, CreateTaskParams{
		TaskType:            domain.TaskTypeNewProductDevelopment,
		SourceMode:          domain.TaskSourceModeNewProduct,
		CreatorID:           291,
		DeadlineAt:          timePtr(),
		CategoryCode:        "LIGHTBOX",
		MaterialMode:        string(domain.MaterialModePreset),
		Material:            "铝型材",
		ProductNameSnapshot: "Actor Owned Lightbox",
		ProductShortName:    "ActorOwned",
		DesignRequirement:   "need design",
	})
	if appErr != nil {
		t.Fatalf("Create() unexpected error: %+v", appErr)
	}
	if task.OwnerTeam != "内贸运营组" {
		t.Fatalf("Create() owner_team = %q, want 内贸运营组", task.OwnerTeam)
	}
	if task.OwnerDepartment != string(domain.DepartmentOperations) {
		t.Fatalf("Create() owner_department = %q, want %q", task.OwnerDepartment, domain.DepartmentOperations)
	}
	if task.OwnerOrgTeam != "淘系一组" {
		t.Fatalf("Create() owner_org_team = %q, want 淘系一组", task.OwnerOrgTeam)
	}
}

func TestTaskServiceListHydratesCanonicalOwnershipFields(t *testing.T) {
	taskRepo := &prdTaskRepo{
		listItems: []*domain.TaskListItem{
			{
				ID:                  1,
				TaskNo:              "T-001",
				SKUCode:             "SKU-001",
				ProductNameSnapshot: "Ops Product",
				TaskType:            domain.TaskTypeNewProductDevelopment,
				SourceMode:          domain.TaskSourceModeNewProduct,
				OwnerTeam:           "内贸运营组",
				TaskStatus:          domain.TaskStatusPendingAssign,
				Priority:            domain.TaskPriorityLow,
				CreatorID:           9,
				CreatedAt:           time.Now().UTC(),
				UpdatedAt:           time.Now().UTC(),
				BatchMode:           domain.TaskBatchModeSingle,
				BatchItemCount:      1,
			},
			{
				ID:                  2,
				TaskNo:              "T-002",
				SKUCode:             "SKU-002",
				ProductNameSnapshot: "Ops Team Product",
				TaskType:            domain.TaskTypeNewProductDevelopment,
				SourceMode:          domain.TaskSourceModeNewProduct,
				OwnerTeam:           "内贸运营组",
				OwnerDepartment:     "运营部",
				OwnerOrgTeam:        "运营三组",
				TaskStatus:          domain.TaskStatusPendingAssign,
				Priority:            domain.TaskPriorityLow,
				CreatorID:           9,
				CreatedAt:           time.Now().UTC(),
				UpdatedAt:           time.Now().UTC(),
				BatchMode:           domain.TaskBatchModeSingle,
				BatchItemCount:      1,
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

	items, _, appErr := svc.List(context.Background(), TaskFilter{Page: 1, PageSize: 20})
	if appErr != nil {
		t.Fatalf("List() unexpected error: %+v", appErr)
	}
	if len(items) != 2 {
		t.Fatalf("List() len = %d, want 2", len(items))
	}
	if items[0].OwnerDepartment != "运营部" {
		t.Fatalf("List()[0].owner_department = %q, want 运营部", items[0].OwnerDepartment)
	}
	if items[0].OwnerOrgTeam != "" {
		t.Fatalf("List()[0].owner_org_team = %q, want empty for legacy-only inference", items[0].OwnerOrgTeam)
	}
	if items[1].OwnerDepartment != "运营部" || items[1].OwnerOrgTeam != "运营三组" {
		t.Fatalf("List()[1] canonical ownership = (%q, %q), want (运营部, 运营三组)", items[1].OwnerDepartment, items[1].OwnerOrgTeam)
	}
}

func TestTaskReadModelOwnershipNormalizesLegacyDepartmentsToCanonical(t *testing.T) {
	ownership := buildTaskReadModelOrgOwnership(
		string(domain.DepartmentWarehouse),
		"",
		"",
	)
	if ownership.OwnerDepartment != string(domain.DepartmentCloudWarehouse) {
		t.Fatalf("owner_department = %q, want %q", ownership.OwnerDepartment, domain.DepartmentCloudWarehouse)
	}

	createOwnership, appErr := resolveTaskCanonicalOrgOwnership(CreateTaskParams{
		rawOwnerDepartment: string(domain.DepartmentDesign),
	})
	if appErr != nil {
		t.Fatalf("resolveTaskCanonicalOrgOwnership() appErr = %+v", appErr)
	}
	if createOwnership.OwnerDepartment != string(domain.DepartmentDesignRD) {
		t.Fatalf("create owner_department = %q, want %q", createOwnership.OwnerDepartment, domain.DepartmentDesignRD)
	}
}

func TestTaskServiceListSupportsBusinessLaneFilter(t *testing.T) {
	now := time.Now().UTC()
	taskRepo := &prdTaskRepo{
		listItems: []*domain.TaskListItem{
			{
				ID:                    1,
				TaskNo:                "T-NORMAL",
				SKUCode:               "SKU-NORMAL",
				ProductNameSnapshot:   "Normal",
				TaskType:              domain.TaskTypeNewProductDevelopment,
				SourceMode:            domain.TaskSourceModeNewProduct,
				OwnerDepartment:       string(domain.DepartmentOperations),
				TaskStatus:            domain.TaskStatusPendingAssign,
				Priority:              domain.TaskPriorityLow,
				CreatorID:             9,
				CreatedAt:             now,
				UpdatedAt:             now,
				BatchMode:             domain.TaskBatchModeSingle,
				BatchItemCount:        1,
				BusinessLane:          domain.TaskBusinessLaneNormal,
				CustomizationRequired: false,
			},
			{
				ID:                    2,
				TaskNo:                "T-CUSTOM",
				SKUCode:               "SKU-CUSTOM",
				ProductNameSnapshot:   "Customization",
				TaskType:              domain.TaskTypeOriginalProductDevelopment,
				SourceMode:            domain.TaskSourceModeExistingProduct,
				OwnerDepartment:       string(domain.DepartmentCustomizationArt),
				TaskStatus:            domain.TaskStatusPendingAudit,
				Priority:              domain.TaskPriorityLow,
				CreatorID:             9,
				CreatedAt:             now,
				UpdatedAt:             now,
				BatchMode:             domain.TaskBatchModeSingle,
				BatchItemCount:        1,
				BusinessLane:          domain.TaskBusinessLaneCustomization,
				CustomizationRequired: true,
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

	items, _, appErr := svc.List(context.Background(), TaskFilter{
		TaskQueryFilterDefinition: domain.TaskQueryFilterDefinition{
			BusinessLanes: []domain.TaskBusinessLane{domain.TaskBusinessLaneCustomization},
		},
		Page:     1,
		PageSize: 20,
	})
	if appErr != nil {
		t.Fatalf("List() unexpected error: %+v", appErr)
	}
	if len(items) != 1 || items[0].TaskNo != "T-CUSTOM" {
		t.Fatalf("List() items = %+v, want only customization lane item", items)
	}
}
