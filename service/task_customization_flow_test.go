package service

import (
	"context"
	"encoding/json"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

type customizationFlowJobRepo struct {
	nextID int64
	jobs   map[int64]*domain.CustomizationJob
}

func newCustomizationFlowJobRepo(jobs ...*domain.CustomizationJob) *customizationFlowJobRepo {
	store := make(map[int64]*domain.CustomizationJob, len(jobs))
	var maxID int64
	for _, job := range jobs {
		if job == nil {
			continue
		}
		copied := *job
		store[copied.ID] = &copied
		if copied.ID > maxID {
			maxID = copied.ID
		}
	}
	return &customizationFlowJobRepo{nextID: maxID + 1, jobs: store}
}

func (r *customizationFlowJobRepo) Create(_ context.Context, _ repo.Tx, job *domain.CustomizationJob) (int64, error) {
	if r.jobs == nil {
		r.jobs = map[int64]*domain.CustomizationJob{}
	}
	id := r.nextID
	if id == 0 {
		id = 1
	}
	r.nextID = id + 1
	copied := *job
	copied.ID = id
	r.jobs[id] = &copied
	return id, nil
}

func (r *customizationFlowJobRepo) GetByID(_ context.Context, id int64) (*domain.CustomizationJob, error) {
	if r.jobs == nil {
		return nil, nil
	}
	item := r.jobs[id]
	if item == nil {
		return nil, nil
	}
	copied := *item
	return &copied, nil
}

func (r *customizationFlowJobRepo) GetLatestByTaskID(_ context.Context, taskID int64) (*domain.CustomizationJob, error) {
	var latest *domain.CustomizationJob
	for _, item := range r.jobs {
		if item == nil || item.TaskID != taskID {
			continue
		}
		if latest == nil || item.ID > latest.ID {
			latest = item
		}
	}
	if latest == nil {
		return nil, nil
	}
	copied := *latest
	return &copied, nil
}

func (r *customizationFlowJobRepo) List(_ context.Context, filter repo.CustomizationJobListFilter) ([]*domain.CustomizationJob, int64, error) {
	items := make([]*domain.CustomizationJob, 0)
	for _, item := range r.jobs {
		if item == nil {
			continue
		}
		if filter.TaskID != nil && item.TaskID != *filter.TaskID {
			continue
		}
		if filter.Status != nil && item.Status != *filter.Status {
			continue
		}
		if filter.OperatorID != nil {
			matched := (item.AssignedOperatorID != nil && *item.AssignedOperatorID == *filter.OperatorID) ||
				(item.LastOperatorID != nil && *item.LastOperatorID == *filter.OperatorID)
			if !matched {
				continue
			}
		}
		copied := *item
		items = append(items, &copied)
	}
	return items, int64(len(items)), nil
}

func (r *customizationFlowJobRepo) Update(_ context.Context, _ repo.Tx, job *domain.CustomizationJob) error {
	if r.jobs == nil {
		r.jobs = map[int64]*domain.CustomizationJob{}
	}
	copied := *job
	r.jobs[job.ID] = &copied
	return nil
}

type customizationFlowUserRepo struct {
	users map[int64]*domain.User
}

func (r customizationFlowUserRepo) GetByID(_ context.Context, id int64) (*domain.User, error) {
	if r.users == nil {
		return nil, nil
	}
	return r.users[id], nil
}

type customizationFlowRuleRepo struct {
	rules map[string]*domain.CustomizationPricingRule
}

func (r customizationFlowRuleRepo) GetActiveByLevelAndEmploymentType(_ context.Context, levelCode string, employmentType domain.EmploymentType) (*domain.CustomizationPricingRule, error) {
	if r.rules == nil {
		return nil, nil
	}
	return r.rules[levelCode+"|"+string(employmentType)], nil
}

func customizationAdminContext() context.Context {
	return domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:         1,
		Roles:      []domain.Role{domain.RoleAdmin},
		Department: "运营部",
		Team:       "运营组",
	})
}

func TestSubmitCustomizationReviewReturnToDesignerCommitsWithoutError(t *testing.T) {
	designerID := int64(22)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:                    101,
		TaskStatus:            domain.TaskStatusPendingCustomizationReview,
		CustomizationRequired: true,
		DesignerID:            &designerID,
		OwnerDepartment:       "运营部",
		OwnerOrgTeam:          "运营组",
		CurrentHandlerID:      nil,
	})
	jobRepo := newCustomizationFlowJobRepo()
	svc := &taskService{
		taskRepo:             taskRepo,
		taskEventRepo:        &step04TaskEventRepo{},
		customizationJobRepo: jobRepo,
		txRunner:             step04TxRunner{},
	}

	item, appErr := svc.SubmitCustomizationReview(customizationAdminContext(), SubmitCustomizationReviewParams{
		TaskID:     101,
		ReviewerID: 1,
		Decision:   domain.CustomizationReviewDecisionReturnToDesigner,
	})
	if appErr != nil {
		t.Fatalf("SubmitCustomizationReview(return_to_designer) appErr = %+v", appErr)
	}
	if item == nil {
		t.Fatal("SubmitCustomizationReview(return_to_designer) item = nil")
	}
	if taskRepo.tasks[101].TaskStatus != domain.TaskStatusPendingCustomizationReview {
		t.Fatalf("task status = %s, want PendingCustomizationReview", taskRepo.tasks[101].TaskStatus)
	}
	if taskRepo.tasks[101].CurrentHandlerID == nil || *taskRepo.tasks[101].CurrentHandlerID != designerID {
		t.Fatalf("current_handler_id = %+v, want %d", taskRepo.tasks[101].CurrentHandlerID, designerID)
	}
	if len(jobRepo.jobs) != 1 {
		t.Fatalf("customization jobs = %d, want 1", len(jobRepo.jobs))
	}
	if item.Status != domain.CustomizationJobStatusPendingCustomizationReview {
		t.Fatalf("customization job status = %s, want pending_customization_review", item.Status)
	}
}

func TestCustomizationFlowAndPricingSnapshot(t *testing.T) {
	tests := []struct {
		name           string
		employmentType domain.EmploymentType
		expectPrice    float64
		expectWeight   float64
	}{
		{name: "full time pricing", employmentType: domain.EmploymentTypeFullTime, expectPrice: 21.5, expectWeight: 1.2},
		{name: "part time pricing", employmentType: domain.EmploymentTypePartTime, expectPrice: 17.6, expectWeight: 0.9},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			designerID := int64(33)
			operatorID := int64(88)
			currentAssetID := int64(9001)
			taskRepo := newStep04TaskRepo(&domain.Task{
				ID:                    102,
				TaskStatus:            domain.TaskStatusPendingCustomizationReview,
				CustomizationRequired: true,
				DesignerID:            &designerID,
				OwnerDepartment:       "运营部",
				OwnerOrgTeam:          "运营组",
				CurrentHandlerID:      nil,
			})
			jobRepo := newCustomizationFlowJobRepo(&domain.CustomizationJob{
				ID:           400,
				TaskID:       102,
				DecisionType: domain.CustomizationJobDecisionTypeFinal,
				Status:       domain.CustomizationJobStatusPendingCustomizationReview,
			})
			svc := &taskService{
				taskRepo:             taskRepo,
				taskEventRepo:        &step04TaskEventRepo{},
				customizationJobRepo: jobRepo,
				txRunner:             step04TxRunner{},
				customizationPricingUserRepo: customizationFlowUserRepo{
					users: map[int64]*domain.User{
						operatorID: {ID: operatorID, EmploymentType: tc.employmentType},
					},
				},
				customizationPricingRuleRepo: customizationFlowRuleRepo{
					rules: map[string]*domain.CustomizationPricingRule{
						"L1|" + string(tc.employmentType): {
							CustomizationLevelCode: "L1",
							EmploymentType:         tc.employmentType,
							UnitPrice:              tc.expectPrice,
							WeightFactor:           tc.expectWeight,
							IsEnabled:              true,
						},
					},
				},
			}

			job, appErr := svc.SubmitCustomizationReview(customizationAdminContext(), SubmitCustomizationReviewParams{
				TaskID:                 102,
				ReviewerID:             1,
				Decision:               domain.CustomizationReviewDecisionApproved,
				CustomizationLevelCode: "L1",
				CustomizationLevelName: "Level 1",
				CustomizationPrice:     float64Ptr(31.2),
				CustomizationWeight:    float64Ptr(1.6),
			})
			if appErr != nil {
				t.Fatalf("SubmitCustomizationReview() appErr = %+v", appErr)
			}
			if job == nil {
				t.Fatal("SubmitCustomizationReview() job = nil")
			}
			if job.ID != 400 {
				t.Fatalf("SubmitCustomizationReview() job.ID = %d, want 400", job.ID)
			}
			if job.ReviewReferenceUnitPrice == nil || *job.ReviewReferenceUnitPrice != 31.2 {
				t.Fatalf("review_reference_unit_price = %+v, want 31.2", job.ReviewReferenceUnitPrice)
			}
			if job.ReviewReferenceWeightFactor == nil || *job.ReviewReferenceWeightFactor != 1.6 {
				t.Fatalf("review_reference_weight_factor = %+v, want 1.6", job.ReviewReferenceWeightFactor)
			}

			job, appErr = svc.SubmitCustomizationEffectPreview(customizationAdminContext(), SubmitCustomizationEffectPreviewParams{
				JobID:          job.ID,
				OperatorID:     operatorID,
				OrderNo:        "ERP-1001",
				CurrentAssetID: &currentAssetID,
				Note:           "preview-1",
			})
			if appErr != nil {
				t.Fatalf("SubmitCustomizationEffectPreview() appErr = %+v", appErr)
			}
			if job.OrderNo != "ERP-1001" {
				t.Fatalf("order_no = %q, want ERP-1001", job.OrderNo)
			}
			if job.CurrentAssetID == nil || *job.CurrentAssetID != currentAssetID {
				t.Fatalf("current_asset_id = %+v, want %d", job.CurrentAssetID, currentAssetID)
			}
			if job.PricingWorkerType != tc.employmentType {
				t.Fatalf("pricing_worker_type = %s, want %s", job.PricingWorkerType, tc.employmentType)
			}
			if job.UnitPrice == nil || *job.UnitPrice != tc.expectPrice {
				t.Fatalf("unit_price = %+v, want %v", job.UnitPrice, tc.expectPrice)
			}
			if job.WeightFactor == nil || *job.WeightFactor != tc.expectWeight {
				t.Fatalf("weight_factor = %+v, want %v", job.WeightFactor, tc.expectWeight)
			}
			if job.ReviewReferenceUnitPrice == nil || *job.ReviewReferenceUnitPrice != 31.2 {
				t.Fatalf("review_reference_unit_price after freeze = %+v, want 31.2", job.ReviewReferenceUnitPrice)
			}
			if job.ReviewReferenceWeightFactor == nil || *job.ReviewReferenceWeightFactor != 1.6 {
				t.Fatalf("review_reference_weight_factor after freeze = %+v, want 1.6", job.ReviewReferenceWeightFactor)
			}
			if taskRepo.tasks[102].LastCustomizationOperatorID == nil || *taskRepo.tasks[102].LastCustomizationOperatorID != operatorID {
				t.Fatalf("last_customization_operator_id = %+v, want %d", taskRepo.tasks[102].LastCustomizationOperatorID, operatorID)
			}

			job, appErr = svc.ReviewCustomizationEffect(customizationAdminContext(), ReviewCustomizationEffectParams{
				JobID:          job.ID,
				ReviewerID:     1,
				Decision:       domain.CustomizationReviewDecisionApproved,
				CurrentAssetID: &currentAssetID,
			})
			if appErr != nil {
				t.Fatalf("ReviewCustomizationEffect(approved) appErr = %+v", appErr)
			}
			if job.Status != domain.CustomizationJobStatusPendingProductionTransfer {
				t.Fatalf("job status after effect review = %s, want pending_production_transfer", job.Status)
			}

			job, appErr = svc.TransferCustomizationProduction(customizationAdminContext(), TransferCustomizationProductionParams{
				JobID:      job.ID,
				OperatorID: operatorID,
			})
			if appErr != nil {
				t.Fatalf("TransferCustomizationProduction() appErr = %+v", appErr)
			}
			if job.Status != domain.CustomizationJobStatusPendingWarehouseQC {
				t.Fatalf("job status after transfer = %s, want pending_warehouse_qc", job.Status)
			}
			if taskRepo.tasks[102].TaskStatus != domain.TaskStatusPendingWarehouseQC {
				t.Fatalf("task status after transfer = %s, want PendingWarehouseQC", taskRepo.tasks[102].TaskStatus)
			}
		})
	}
}

func TestReviewCustomizationEffectReturnToDesigner(t *testing.T) {
	lastOperatorID := int64(66)
	currentAssetID := int64(7001)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:                    103,
		TaskStatus:            domain.TaskStatusPendingEffectReview,
		CustomizationRequired: true,
		OwnerDepartment:       "运营部",
		OwnerOrgTeam:          "运营组",
		CurrentHandlerID:      nil,
	})
	jobRepo := newCustomizationFlowJobRepo(&domain.CustomizationJob{
		ID:                     401,
		TaskID:                 103,
		CustomizationLevelCode: "L1",
		CustomizationLevelName: "Level 1",
		CurrentAssetID:         &currentAssetID,
		Status:                 domain.CustomizationJobStatusPendingEffectReview,
		LastOperatorID:         &lastOperatorID,
	})
	svc := &taskService{
		taskRepo:             taskRepo,
		taskEventRepo:        &step04TaskEventRepo{},
		customizationJobRepo: jobRepo,
		txRunner:             step04TxRunner{},
	}

	job, appErr := svc.ReviewCustomizationEffect(customizationAdminContext(), ReviewCustomizationEffectParams{
		JobID:          401,
		ReviewerID:     1,
		Decision:       domain.CustomizationReviewDecisionReturnToDesigner,
		CurrentAssetID: &currentAssetID,
	})
	if appErr != nil {
		t.Fatalf("ReviewCustomizationEffect(return_to_designer) appErr = %+v", appErr)
	}
	if job.Status != domain.CustomizationJobStatusPendingEffectRevision {
		t.Fatalf("job status = %s, want pending_effect_revision", job.Status)
	}
	if taskRepo.tasks[103].TaskStatus != domain.TaskStatusPendingEffectRevision {
		t.Fatalf("task status = %s, want PendingEffectRevision", taskRepo.tasks[103].TaskStatus)
	}
	if taskRepo.tasks[103].CurrentHandlerID == nil || *taskRepo.tasks[103].CurrentHandlerID != lastOperatorID {
		t.Fatalf("current_handler_id = %+v, want %d", taskRepo.tasks[103].CurrentHandlerID, lastOperatorID)
	}
}

func TestSubmitCustomizationEffectPreviewMissingRuleFails(t *testing.T) {
	operatorID := int64(77)
	currentAssetID := int64(7101)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:                    104,
		TaskStatus:            domain.TaskStatusPendingCustomizationProduction,
		CustomizationRequired: true,
		OwnerDepartment:       "运营部",
		OwnerOrgTeam:          "运营组",
		CurrentHandlerID:      nil,
	})
	jobRepo := newCustomizationFlowJobRepo(&domain.CustomizationJob{
		ID:                     402,
		TaskID:                 104,
		CustomizationLevelCode: "L2",
		CustomizationLevelName: "Level 2",
		Status:                 domain.CustomizationJobStatusPendingCustomizationProduction,
	})
	svc := &taskService{
		taskRepo:             taskRepo,
		taskEventRepo:        &step04TaskEventRepo{},
		customizationJobRepo: jobRepo,
		txRunner:             step04TxRunner{},
		customizationPricingUserRepo: customizationFlowUserRepo{
			users: map[int64]*domain.User{
				operatorID: {ID: operatorID, EmploymentType: domain.EmploymentTypeFullTime},
			},
		},
		customizationPricingRuleRepo: customizationFlowRuleRepo{rules: map[string]*domain.CustomizationPricingRule{}},
	}

	_, appErr := svc.SubmitCustomizationEffectPreview(customizationAdminContext(), SubmitCustomizationEffectPreviewParams{
		JobID:          402,
		OperatorID:     operatorID,
		CurrentAssetID: &currentAssetID,
	})
	if appErr == nil {
		t.Fatal("SubmitCustomizationEffectPreview() expected missing pricing rule error")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
	}
}

func TestTransferCustomizationProductionPreconditionFailure(t *testing.T) {
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:                    105,
		TaskStatus:            domain.TaskStatusPendingProductionTransfer,
		CustomizationRequired: true,
		OwnerDepartment:       "运营部",
		OwnerOrgTeam:          "运营组",
	})
	jobRepo := newCustomizationFlowJobRepo(&domain.CustomizationJob{
		ID:                     403,
		TaskID:                 105,
		CustomizationLevelCode: "L1",
		CustomizationLevelName: "Level 1",
		Status:                 domain.CustomizationJobStatusPendingEffectReview,
	})
	svc := &taskService{
		taskRepo:             taskRepo,
		taskEventRepo:        &step04TaskEventRepo{},
		customizationJobRepo: jobRepo,
		txRunner:             step04TxRunner{},
	}

	_, appErr := svc.TransferCustomizationProduction(customizationAdminContext(), TransferCustomizationProductionParams{
		JobID:      403,
		OperatorID: 1,
	})
	if appErr == nil {
		t.Fatal("TransferCustomizationProduction() expected invalid state transition")
	}
	if appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("error code = %s, want %s", appErr.Code, domain.ErrCodeInvalidStateTransition)
	}
}

func TestWarehouseRejectReturnsToLastCustomizationOperator(t *testing.T) {
	lastOperatorID := int64(501)
	receiverID := int64(900)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:                          106,
		TaskStatus:                  domain.TaskStatusPendingWarehouseQC,
		CustomizationRequired:       true,
		LastCustomizationOperatorID: &lastOperatorID,
		CurrentHandlerID:            &receiverID,
		OwnerDepartment:             "运营部",
		OwnerOrgTeam:                "运营组",
	})
	jobRepo := newCustomizationFlowJobRepo(&domain.CustomizationJob{
		ID:                     404,
		TaskID:                 106,
		CustomizationLevelCode: "L1",
		CustomizationLevelName: "Level 1",
		Status:                 domain.CustomizationJobStatusPendingWarehouseQC,
		LastOperatorID:         &lastOperatorID,
	})
	warehouseRepo := &prdWarehouseRepo{}
	eventRepo := &step04TaskEventRepo{}
	svc := NewWarehouseService(taskRepo, newStep04TaskAssetRepo(), warehouseRepo, eventRepo, step04TxRunner{}, WithWarehouseCustomizationJobRepo(jobRepo))

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{
		ID:    receiverID,
		Roles: []domain.Role{domain.RoleWarehouse},
		Team:  "运营组",
	})
	_, appErr := svc.Reject(ctx, RejectWarehouseParams{
		TaskID:         106,
		ReceiverID:     receiverID,
		RejectReason:   "color mismatch",
		RejectCategory: "color_issue",
		Remark:         "return to customization",
	})
	if appErr != nil {
		t.Fatalf("Warehouse Reject() appErr = %+v", appErr)
	}
	if taskRepo.tasks[106].TaskStatus != domain.TaskStatusRejectedByWarehouse {
		t.Fatalf("task status = %s, want RejectedByWarehouse", taskRepo.tasks[106].TaskStatus)
	}
	if taskRepo.tasks[106].CurrentHandlerID == nil || *taskRepo.tasks[106].CurrentHandlerID != lastOperatorID {
		t.Fatalf("current_handler_id = %+v, want %d", taskRepo.tasks[106].CurrentHandlerID, lastOperatorID)
	}
	latest, err := jobRepo.GetLatestByTaskID(context.Background(), 106)
	if err != nil {
		t.Fatalf("GetLatestByTaskID() err = %v", err)
	}
	if latest == nil || latest.Status != domain.CustomizationJobStatusRejectedByWarehouse {
		t.Fatalf("job status = %+v, want rejected_by_warehouse", latest)
	}
	if latest.WarehouseRejectCategory != "color_issue" {
		t.Fatalf("warehouse_reject_category = %q, want color_issue", latest.WarehouseRejectCategory)
	}
	if taskRepo.tasks[106].WarehouseRejectCategory != "color_issue" {
		t.Fatalf("task warehouse_reject_category = %q, want color_issue", taskRepo.tasks[106].WarehouseRejectCategory)
	}
}

/*
func TestSubmitCustomizationEffectPreviewFinalSkipsEffectReview(t *testing.T) {
	operatorID := int64(188)
	currentAssetID := int64(9101)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:                    107,
		TaskStatus:            domain.TaskStatusPendingCustomizationProduction,
		CustomizationRequired: true,
		OwnerDepartment:       "杩愯惀閮?,
		OwnerOrgTeam:          "杩愯惀缁?,
	})
	jobRepo := newCustomizationFlowJobRepo(&domain.CustomizationJob{
		ID:                     405,
		TaskID:                 107,
		CustomizationLevelCode: "L1",
		CustomizationLevelName: "Level 1",
		Status:                 domain.CustomizationJobStatusPendingCustomizationProduction,
	})
	svc := &taskService{
		taskRepo:             taskRepo,
		taskEventRepo:        &step04TaskEventRepo{},
		customizationJobRepo: jobRepo,
		txRunner:             step04TxRunner{},
		customizationPricingUserRepo: customizationFlowUserRepo{
			users: map[int64]*domain.User{
				operatorID: {ID: operatorID, EmploymentType: domain.EmploymentTypeFullTime},
			},
		},
		customizationPricingRuleRepo: customizationFlowRuleRepo{
			rules: map[string]*domain.CustomizationPricingRule{
				"L1|" + string(domain.EmploymentTypeFullTime): {
					CustomizationLevelCode: "L1",
					EmploymentType:         domain.EmploymentTypeFullTime,
					UnitPrice:              20,
					WeightFactor:           1,
					IsEnabled:              true,
				},
			},
		},
	}

	job, appErr := svc.SubmitCustomizationEffectPreview(customizationAdminContext(), SubmitCustomizationEffectPreviewParams{
		JobID:          405,
		OperatorID:     operatorID,
		OrderNo:        "ERP-2001",
		CurrentAssetID: &currentAssetID,
		DecisionType:   domain.CustomizationJobDecisionTypeFinal,
		Note:           "final delivery",
	})
	if appErr != nil {
		t.Fatalf("SubmitCustomizationEffectPreview(final) appErr = %+v", appErr)
	}
	if job.Status != domain.CustomizationJobStatusPendingProductionTransfer {
		t.Fatalf("job status = %s, want pending_production_transfer", job.Status)
	}
	if taskRepo.tasks[107].TaskStatus != domain.TaskStatusPendingProductionTransfer {
		t.Fatalf("task status = %s, want PendingProductionTransfer", taskRepo.tasks[107].TaskStatus)
	}
	if taskRepo.tasks[107].CurrentHandlerID == nil || *taskRepo.tasks[107].CurrentHandlerID != operatorID {
		t.Fatalf("current_handler_id = %+v, want %d", taskRepo.tasks[107].CurrentHandlerID, operatorID)
	}
}

func TestReviewCustomizationEffectReviewerFixedReplacesCurrentAsset(t *testing.T) {
	lastOperatorID := int64(266)
	oldAssetID := int64(8101)
	newAssetID := int64(8102)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:                    108,
		TaskStatus:            domain.TaskStatusPendingEffectReview,
		CustomizationRequired: true,
		OwnerDepartment:       "杩愯惀閮?,
		OwnerOrgTeam:          "杩愯惀缁?,
	})
	jobRepo := newCustomizationFlowJobRepo(&domain.CustomizationJob{
		ID:                     406,
		TaskID:                 108,
		CustomizationLevelCode: "L1",
		CustomizationLevelName: "Level 1",
		CurrentAssetID:         &oldAssetID,
		Status:                 domain.CustomizationJobStatusPendingEffectReview,
		LastOperatorID:         &lastOperatorID,
	})
	eventRepo := &step04TaskEventRepo{}
	svc := &taskService{
		taskRepo:             taskRepo,
		taskEventRepo:        eventRepo,
		customizationJobRepo: jobRepo,
		txRunner:             step04TxRunner{},
	}

	job, appErr := svc.ReviewCustomizationEffect(customizationAdminContext(), ReviewCustomizationEffectParams{
		JobID:          406,
		ReviewerID:     1,
		Decision:       domain.CustomizationReviewDecisionReviewerFixed,
		CurrentAssetID: &newAssetID,
		CustomizationPrice:  float64Ptr(29.5),
		CustomizationWeight: float64Ptr(1.3),
	})
	if appErr != nil {
		t.Fatalf("ReviewCustomizationEffect(reviewer_fixed) appErr = %+v", appErr)
	}
	if job.Status != domain.CustomizationJobStatusPendingProductionTransfer {
		t.Fatalf("job status = %s, want pending_production_transfer", job.Status)
	}
	if job.CurrentAssetID == nil || *job.CurrentAssetID != newAssetID {
		t.Fatalf("current_asset_id = %+v, want %d", job.CurrentAssetID, newAssetID)
	}
	if job.ReviewReferenceUnitPrice == nil || *job.ReviewReferenceUnitPrice != 29.5 {
		t.Fatalf("review_reference_unit_price = %+v, want 29.5", job.ReviewReferenceUnitPrice)
	}
	if job.ReviewReferenceWeightFactor == nil || *job.ReviewReferenceWeightFactor != 1.3 {
		t.Fatalf("review_reference_weight_factor = %+v, want 1.3", job.ReviewReferenceWeightFactor)
	}
}
*/

func TestSubmitCustomizationEffectPreviewFinalSkipsEffectReview(t *testing.T) {
	operatorID := int64(188)
	currentAssetID := int64(9101)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:                    107,
		TaskStatus:            domain.TaskStatusPendingCustomizationProduction,
		CustomizationRequired: true,
		OwnerDepartment:       "design-dept",
		OwnerOrgTeam:          "design-team",
	})
	jobRepo := newCustomizationFlowJobRepo(&domain.CustomizationJob{
		ID:                     405,
		TaskID:                 107,
		CustomizationLevelCode: "L1",
		CustomizationLevelName: "Level 1",
		Status:                 domain.CustomizationJobStatusPendingCustomizationProduction,
	})
	svc := &taskService{
		taskRepo:             taskRepo,
		taskEventRepo:        &step04TaskEventRepo{},
		customizationJobRepo: jobRepo,
		txRunner:             step04TxRunner{},
		customizationPricingUserRepo: customizationFlowUserRepo{
			users: map[int64]*domain.User{
				operatorID: {ID: operatorID, EmploymentType: domain.EmploymentTypeFullTime},
			},
		},
		customizationPricingRuleRepo: customizationFlowRuleRepo{
			rules: map[string]*domain.CustomizationPricingRule{
				"L1|" + string(domain.EmploymentTypeFullTime): {
					CustomizationLevelCode: "L1",
					EmploymentType:         domain.EmploymentTypeFullTime,
					UnitPrice:              20,
					WeightFactor:           1,
					IsEnabled:              true,
				},
			},
		},
	}

	job, appErr := svc.SubmitCustomizationEffectPreview(customizationAdminContext(), SubmitCustomizationEffectPreviewParams{
		JobID:          405,
		OperatorID:     operatorID,
		OrderNo:        "ERP-2001",
		CurrentAssetID: &currentAssetID,
		DecisionType:   domain.CustomizationJobDecisionTypeFinal,
		Note:           "final delivery",
	})
	if appErr != nil {
		t.Fatalf("SubmitCustomizationEffectPreview(final) appErr = %+v", appErr)
	}
	if job.Status != domain.CustomizationJobStatusPendingProductionTransfer {
		t.Fatalf("job status = %s, want pending_production_transfer", job.Status)
	}
	if job.OrderNo != "ERP-2001" {
		t.Fatalf("order_no = %q, want ERP-2001", job.OrderNo)
	}
	if job.CurrentAssetID == nil || *job.CurrentAssetID != currentAssetID {
		t.Fatalf("current_asset_id = %+v, want %d", job.CurrentAssetID, currentAssetID)
	}
	if taskRepo.tasks[107].TaskStatus != domain.TaskStatusPendingProductionTransfer {
		t.Fatalf("task status = %s, want PendingProductionTransfer", taskRepo.tasks[107].TaskStatus)
	}
	if taskRepo.tasks[107].CurrentHandlerID == nil || *taskRepo.tasks[107].CurrentHandlerID != operatorID {
		t.Fatalf("current_handler_id = %+v, want %d", taskRepo.tasks[107].CurrentHandlerID, operatorID)
	}
}

func TestReviewCustomizationEffectReviewerFixedReplacesCurrentAsset(t *testing.T) {
	lastOperatorID := int64(266)
	oldAssetID := int64(8101)
	newAssetID := int64(8102)
	taskRepo := newStep04TaskRepo(&domain.Task{
		ID:                    108,
		TaskStatus:            domain.TaskStatusPendingEffectReview,
		CustomizationRequired: true,
		OwnerDepartment:       "design-dept",
		OwnerOrgTeam:          "design-team",
	})
	jobRepo := newCustomizationFlowJobRepo(&domain.CustomizationJob{
		ID:                     406,
		TaskID:                 108,
		CustomizationLevelCode: "L1",
		CustomizationLevelName: "Level 1",
		CurrentAssetID:         &oldAssetID,
		Status:                 domain.CustomizationJobStatusPendingEffectReview,
		LastOperatorID:         &lastOperatorID,
	})
	eventRepo := &step04TaskEventRepo{}
	svc := &taskService{
		taskRepo:             taskRepo,
		taskEventRepo:        eventRepo,
		customizationJobRepo: jobRepo,
		txRunner:             step04TxRunner{},
	}

	job, appErr := svc.ReviewCustomizationEffect(customizationAdminContext(), ReviewCustomizationEffectParams{
		JobID:          406,
		ReviewerID:     1,
		Decision:       domain.CustomizationReviewDecisionReviewerFixed,
		CurrentAssetID: &newAssetID,
	})
	if appErr != nil {
		t.Fatalf("ReviewCustomizationEffect(reviewer_fixed) appErr = %+v", appErr)
	}
	if job.Status != domain.CustomizationJobStatusPendingProductionTransfer {
		t.Fatalf("job status = %s, want pending_production_transfer", job.Status)
	}
	if job.CurrentAssetID == nil || *job.CurrentAssetID != newAssetID {
		t.Fatalf("current_asset_id = %+v, want %d", job.CurrentAssetID, newAssetID)
	}
	if len(eventRepo.events) == 0 {
		t.Fatal("eventRepo.events = 0, want reviewer-fixed event")
	}
	payload := map[string]interface{}{}
	if err := json.Unmarshal(eventRepo.events[len(eventRepo.events)-1].Payload, &payload); err != nil {
		t.Fatalf("json.Unmarshal(payload) error = %v", err)
	}
	if payload["workflow_lane"] != string(domain.WorkflowLaneCustomization) {
		t.Fatalf("workflow_lane payload = %v, want %q", payload["workflow_lane"], domain.WorkflowLaneCustomization)
	}
	if payload["source_department"] != string(domain.DepartmentCustomizationArt) {
		t.Fatalf("source_department payload = %v, want %q", payload["source_department"], domain.DepartmentCustomizationArt)
	}
}
