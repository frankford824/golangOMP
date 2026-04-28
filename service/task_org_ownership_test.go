package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestTaskServiceCreateOriginalProductWithOrgTeamCompatWritesCanonicalOwnership(t *testing.T) {
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

func TestTaskServiceCreateNewAndPurchaseTaskWithOrgTeamCompatWriteCanonicalOwnership(t *testing.T) {
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
		{
			name: "purchase_task",
			params: CreateTaskParams{
				TaskType:            domain.TaskTypePurchaseTask,
				SourceMode:          domain.TaskSourceModeNewProduct,
				CreatorID:           9,
				OwnerTeam:           "运营三组",
				DeadlineAt:          timePtr(),
				PurchaseSKU:         "PUR-001",
				ProductNameSnapshot: "Purchase Product",
				CostPriceMode:       string(domain.CostPriceModeManual),
				CostPrice:           float64Ptr(12.5),
				Quantity:            int64Ptr(10),
				BaseSalePrice:       float64Ptr(25),
				ProductChannel:      "1688",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
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

func TestTaskServiceListAppliesCanonicalOrgVisibilityScope(t *testing.T) {
	now := time.Now().UTC()
	taskRepo := &taskOrgVisibilityRepo{
		items: []*domain.TaskListItem{
			{
				ID:                  1,
				TaskNo:              "T-OPS-TEAM",
				SKUCode:             "SKU-OPS-1",
				ProductNameSnapshot: "Ops Team",
				TaskType:            domain.TaskTypeNewProductDevelopment,
				SourceMode:          domain.TaskSourceModeNewProduct,
				OwnerTeam:           "内贸运营组",
				OwnerDepartment:     "运营部",
				OwnerOrgTeam:        "运营三组",
				TaskStatus:          domain.TaskStatusPendingAssign,
				Priority:            domain.TaskPriorityLow,
				CreatorID:           11,
				CreatedAt:           now,
				UpdatedAt:           now,
				BatchMode:           domain.TaskBatchModeSingle,
				BatchItemCount:      1,
			},
			{
				ID:                  2,
				TaskNo:              "T-OPS-DEPT",
				SKUCode:             "SKU-OPS-2",
				ProductNameSnapshot: "Ops Department",
				TaskType:            domain.TaskTypeNewProductDevelopment,
				SourceMode:          domain.TaskSourceModeNewProduct,
				OwnerTeam:           "内贸运营组",
				OwnerDepartment:     "运营部",
				TaskStatus:          domain.TaskStatusPendingAssign,
				Priority:            domain.TaskPriorityLow,
				CreatorID:           12,
				CreatedAt:           now,
				UpdatedAt:           now,
				BatchMode:           domain.TaskBatchModeSingle,
				BatchItemCount:      1,
			},
			{
				ID:                  3,
				TaskNo:              "T-DESIGN",
				SKUCode:             "SKU-DESIGN-1",
				ProductNameSnapshot: "Design",
				TaskType:            domain.TaskTypeNewProductDevelopment,
				SourceMode:          domain.TaskSourceModeNewProduct,
				OwnerTeam:           "设计组",
				OwnerDepartment:     "设计部",
				OwnerOrgTeam:        "设计审核组",
				TaskStatus:          domain.TaskStatusPendingAssign,
				Priority:            domain.TaskPriorityLow,
				CreatorID:           13,
				CreatedAt:           now,
				UpdatedAt:           now,
				BatchMode:           domain.TaskBatchModeSingle,
				BatchItemCount:      1,
			},
		},
	}
	userRepo := newIdentityUserRepo()
	userRepo.users[100] = &domain.User{ID: 100, Username: "super", Department: domain.DepartmentUnassigned, Team: "未分配池"}
	userRepo.users[200] = &domain.User{ID: 200, Username: "dept-admin", Department: domain.DepartmentOperations, Team: "运营三组"}
	userRepo.users[300] = &domain.User{ID: 300, Username: "team-lead", Department: domain.DepartmentOperations, Team: "运营三组"}

	svc := NewTaskService(
		taskRepo,
		&prdProcurementRepo{},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		nil,
		&prdWarehouseRepo{},
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskScopeUserRepo(userRepo),
	)

	viewAllCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       100,
		Roles:    []domain.Role{domain.RoleSuperAdmin},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	viewAllItems, _, appErr := svc.List(viewAllCtx, TaskFilter{Page: 1, PageSize: 20})
	if appErr != nil {
		t.Fatalf("view-all List() unexpected error: %+v", appErr)
	}
	if len(viewAllItems) != 3 {
		t.Fatalf("view-all List() len = %d, want 3", len(viewAllItems))
	}

	departmentCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       200,
		Roles:    []domain.Role{domain.RoleDeptAdmin},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	departmentItems, _, appErr := svc.List(departmentCtx, TaskFilter{Page: 1, PageSize: 20})
	if appErr != nil {
		t.Fatalf("department List() unexpected error: %+v", appErr)
	}
	if len(departmentItems) != 3 {
		t.Fatalf("department List() len = %d, want 3 (main task flow is globally readable)", len(departmentItems))
	}

	teamCtx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:       300,
		Roles:    []domain.Role{domain.RoleTeamLead},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	teamItems, _, appErr := svc.List(teamCtx, TaskFilter{Page: 1, PageSize: 20})
	if appErr != nil {
		t.Fatalf("team List() unexpected error: %+v", appErr)
	}
	if len(teamItems) != 3 {
		t.Fatalf("team List() len = %d, want 3 (main task flow is globally readable)", len(teamItems))
	}
}

func TestTaskServiceListSupportsWorkflowLaneFilter(t *testing.T) {
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
				CustomizationRequired: false,
				WorkflowLane:          domain.WorkflowLaneNormal,
			},
			{
				ID:                    2,
				TaskNo:                "T-CUSTOM",
				SKUCode:               "SKU-CUSTOM",
				ProductNameSnapshot:   "Customization",
				TaskType:              domain.TaskTypeOriginalProductDevelopment,
				SourceMode:            domain.TaskSourceModeExistingProduct,
				OwnerDepartment:       string(domain.DepartmentCustomizationArt),
				TaskStatus:            domain.TaskStatusPendingCustomizationReview,
				Priority:              domain.TaskPriorityLow,
				CreatorID:             9,
				CreatedAt:             now,
				UpdatedAt:             now,
				BatchMode:             domain.TaskBatchModeSingle,
				BatchItemCount:        1,
				CustomizationRequired: true,
				WorkflowLane:          domain.WorkflowLaneCustomization,
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
			WorkflowLanes: []domain.WorkflowLane{domain.WorkflowLaneCustomization},
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

func TestApplyTaskOrgVisibilityScopeCopiesStageVisibilities(t *testing.T) {
	scope := &DataScope{
		DepartmentCodes: []string{string(domain.DepartmentAudit)},
		StageVisibilities: []StageVisibility{
			{
				Statuses: []domain.TaskStatus{
					domain.TaskStatusPendingAuditA,
					domain.TaskStatusRejectedByAuditA,
				},
				Lane: workflowLanePtr(domain.WorkflowLaneNormal),
			},
			{
				Statuses: []domain.TaskStatus{
					domain.TaskStatusPendingCustomizationReview,
				},
				Lane: workflowLanePtr(domain.WorkflowLaneCustomization),
			},
		},
	}

	filter := applyTaskOrgVisibilityScope(repo.TaskListFilter{}, scope)
	if len(filter.ScopeStageVisibilities) != 2 {
		t.Fatalf("ScopeStageVisibilities len = %d, want 2", len(filter.ScopeStageVisibilities))
	}
	if !reflect.DeepEqual(filter.ScopeStageVisibilities[0].Statuses, scope.StageVisibilities[0].Statuses) {
		t.Fatalf("ScopeStageVisibilities[0].Statuses = %#v, want %#v", filter.ScopeStageVisibilities[0].Statuses, scope.StageVisibilities[0].Statuses)
	}
	if filter.ScopeStageVisibilities[0].Lane == nil || *filter.ScopeStageVisibilities[0].Lane != domain.WorkflowLaneNormal {
		t.Fatalf("ScopeStageVisibilities[0].Lane = %+v, want normal", filter.ScopeStageVisibilities[0].Lane)
	}
	if filter.ScopeStageVisibilities[1].Lane == nil || *filter.ScopeStageVisibilities[1].Lane != domain.WorkflowLaneCustomization {
		t.Fatalf("ScopeStageVisibilities[1].Lane = %+v, want customization", filter.ScopeStageVisibilities[1].Lane)
	}

	filter.ScopeStageVisibilities[0].Statuses[0] = domain.TaskStatusCompleted
	if scope.StageVisibilities[0].Statuses[0] != domain.TaskStatusPendingAuditA {
		t.Fatal("applyTaskOrgVisibilityScope() should deep-copy stage statuses")
	}
}

func workflowLanePtr(lane domain.WorkflowLane) *domain.WorkflowLane {
	return &lane
}

type taskOrgVisibilityRepo struct {
	prdTaskRepo
	items []*domain.TaskListItem
}

func (r *taskOrgVisibilityRepo) List(_ context.Context, filter repo.TaskListFilter) ([]*domain.TaskListItem, int64, error) {
	filtered := make([]*domain.TaskListItem, 0, len(r.items))
	for _, item := range r.items {
		if item == nil {
			continue
		}
		copied := *item
		applyTaskListItemReadModelOrgOwnership(&copied)
		if !filter.ScopeViewAll {
			visible := false
			for _, uid := range filter.ScopeUserIDs {
				if copied.CreatorID == uid {
					visible = true
					break
				}
			}
			if !visible {
				for _, department := range filter.ScopeDepartmentCodes {
					if department != "" && department == copied.OwnerDepartment {
						visible = true
						break
					}
				}
			}
			if !visible {
				for _, team := range filter.ScopeTeamCodes {
					if team != "" && team == copied.OwnerOrgTeam {
						visible = true
						break
					}
				}
			}
			if !visible {
				continue
			}
		}
		filtered = append(filtered, &copied)
	}
	return filtered, int64(len(filtered)), nil
}
