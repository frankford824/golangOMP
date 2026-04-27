package service

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
)

func TestTaskReadModelIncludesOverrideGovernanceBoundary(t *testing.T) {
	overrideAt := time.Now().UTC()
	financeAt := overrideAt.Add(2 * time.Minute)
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			104: {
				ID:       104,
				TaskType: domain.TaskTypePurchaseTask,
			},
		},
		details: map[int64]*domain.TaskDetail{
			104: {
				ID:                       904,
				TaskID:                   104,
				CategoryCode:             "HBJ",
				CostRuleSource:           "governed_rule",
				ManualCostOverride:       true,
				ManualCostOverrideReason: "supplier override",
				OverrideActor:            "operator:9",
				OverrideAt:               &overrideAt,
				CostPrice:                float64Ptr(88.8),
			},
		},
	}
	overrideRepo := &prdTaskCostOverrideEventRepo{
		events: map[int64][]*domain.TaskCostOverrideAuditEvent{
			104: {
				{
					EventID:         "cov-104-1",
					TaskID:          104,
					TaskDetailID:    int64Ptr(904),
					Sequence:        1,
					EventType:       domain.TaskCostOverrideAuditEventApplied,
					CategoryCode:    "HBJ",
					OverrideCost:    float64Ptr(88.8),
					ResultCostPrice: float64Ptr(88.8),
					OverrideReason:  "supplier override",
					OverrideActor:   "operator:9",
					OverrideAt:      overrideAt,
					Source:          "task_business_info_patch",
					CreatedAt:       overrideAt,
				},
			},
		},
	}
	reviewRepo := &prdTaskCostOverrideReviewRepo{
		records: map[string]*domain.TaskCostOverrideReviewRecord{
			"cov-104-1": {
				RecordID:        11,
				OverrideEventID: "cov-104-1",
				TaskID:          104,
				ReviewRequired:  true,
				ReviewStatus:    domain.TaskCostOverrideReviewStatusApproved,
				ReviewNote:      "placeholder approved",
				ReviewActor:     "actor:7",
				ReviewedAt:      &overrideAt,
			},
		},
	}
	financeRepo := &prdTaskCostFinanceFlagRepo{
		flags: map[string]*domain.TaskCostFinanceFlag{
			"cov-104-1": {
				RecordID:        21,
				OverrideEventID: "cov-104-1",
				TaskID:          104,
				FinanceRequired: true,
				FinanceStatus:   domain.TaskCostOverrideFinanceStatusReadyForView,
				FinanceNote:     "ready for finance placeholder",
				FinanceMarkedBy: "actor:8",
				FinanceMarkedAt: &financeAt,
			},
		},
	}

	svc := NewTaskServiceWithCatalog(
		taskRepo,
		&prdProcurementRepo{
			records: map[int64]*domain.ProcurementRecord{
				104: {TaskID: 104, Status: domain.ProcurementStatusPrepared},
			},
		},
		&prdTaskAssetRepo{},
		&prdTaskEventRepo{},
		overrideRepo,
		&prdWarehouseRepo{},
		nil,
		nil,
		prdCodeRuleService{},
		step04TxRunner{},
		WithTaskCostOverridePlaceholderRepos(reviewRepo, financeRepo),
	)

	readModel, appErr := svc.GetByID(context.Background(), 104)
	if appErr != nil {
		t.Fatalf("GetByID() unexpected error: %+v", appErr)
	}
	if readModel.OverrideBoundary == nil {
		t.Fatalf("OverrideBoundary = nil")
	}
	if readModel.PlatformEntryBoundary == nil || readModel.PlatformEntryBoundary.KPIEntrySummary == nil {
		t.Fatalf("task platform_entry_boundary = %+v", readModel.PlatformEntryBoundary)
	}
	if readModel.OverrideBoundary.ApprovalPlaceholderStatus != domain.TaskCostOverrideReviewStatusApproved {
		t.Fatalf("approval_placeholder_status = %s", readModel.OverrideBoundary.ApprovalPlaceholderStatus)
	}
	if readModel.OverrideBoundary.FinancePlaceholderStatus != domain.TaskCostOverrideFinanceStatusReadyForView {
		t.Fatalf("finance_placeholder_status = %s", readModel.OverrideBoundary.FinancePlaceholderStatus)
	}
	if !readModel.OverrideBoundary.FinanceViewReady {
		t.Fatalf("finance_view_ready = false, want true")
	}
	if readModel.OverrideBoundary.GovernanceBoundarySummary == nil {
		t.Fatal("governance_boundary_summary = nil")
	}
	if readModel.OverrideBoundary.GovernanceBoundarySummary.LatestBoundaryActor != "actor:8" {
		t.Fatalf("latest_boundary_actor = %s, want actor:8", readModel.OverrideBoundary.GovernanceBoundarySummary.LatestBoundaryActor)
	}
	if readModel.OverrideBoundary.GovernanceBoundarySummary.LatestBoundaryAt == nil || !readModel.OverrideBoundary.GovernanceBoundarySummary.LatestBoundaryAt.Equal(financeAt) {
		t.Fatalf("latest_boundary_at = %+v, want %s", readModel.OverrideBoundary.GovernanceBoundarySummary.LatestBoundaryAt, financeAt.Format(time.RFC3339))
	}
	if readModel.OverrideBoundary.ApprovalPlaceholderSummary == nil || readModel.OverrideBoundary.ApprovalPlaceholderSummary.LatestReviewAction == nil {
		t.Fatalf("approval_placeholder_summary = %+v", readModel.OverrideBoundary.ApprovalPlaceholderSummary)
	}
	if readModel.OverrideBoundary.ApprovalPlaceholderSummary.LatestReviewAction.ActionType != "review_approved" {
		t.Fatalf("latest_review_action = %+v", readModel.OverrideBoundary.ApprovalPlaceholderSummary.LatestReviewAction)
	}
	if readModel.OverrideBoundary.FinancePlaceholderSummary == nil || readModel.OverrideBoundary.FinancePlaceholderSummary.LatestFinanceAction == nil {
		t.Fatalf("finance_placeholder_summary = %+v", readModel.OverrideBoundary.FinancePlaceholderSummary)
	}
	if readModel.OverrideBoundary.FinancePlaceholderSummary.LatestFinanceAction.ActionType != "finance_ready_for_view" {
		t.Fatalf("latest_finance_action = %+v", readModel.OverrideBoundary.FinancePlaceholderSummary.LatestFinanceAction)
	}
	if readModel.ProcurementSummary == nil || readModel.ProcurementSummary.OverrideBoundary == nil {
		t.Fatalf("procurement_summary.override_boundary = %+v", readModel.ProcurementSummary)
	}
	if readModel.ProcurementSummary.PlatformEntryBoundary == nil || readModel.ProcurementSummary.PlatformEntryBoundary.FinanceEntrySummary == nil {
		t.Fatalf("procurement_summary.platform_entry_boundary = %+v", readModel.ProcurementSummary.PlatformEntryBoundary)
	}
	if readModel.OverrideBoundary.PlatformEntryBoundary == nil || readModel.OverrideBoundary.PlatformEntryBoundary.FinanceEntrySummary == nil {
		t.Fatalf("override_boundary.platform_entry_boundary = %+v", readModel.OverrideBoundary.PlatformEntryBoundary)
	}
}

func TestTaskCostOverrideAuditServiceUpsertsPlaceholderBoundary(t *testing.T) {
	overrideAt := time.Now().UTC()
	taskRepo := &prdTaskRepo{
		tasks: map[int64]*domain.Task{
			5: {ID: 5, TaskType: domain.TaskTypeNewProductDevelopment},
		},
		details: map[int64]*domain.TaskDetail{
			5: {ID: 15, TaskID: 5, ManualCostOverride: true, ManualCostOverrideReason: "manual", OverrideAt: &overrideAt},
		},
	}
	overrideRepo := &prdTaskCostOverrideEventRepo{
		events: map[int64][]*domain.TaskCostOverrideAuditEvent{
			5: {
				{
					EventID:         "cov-5-1",
					TaskID:          5,
					TaskDetailID:    int64Ptr(15),
					Sequence:        1,
					EventType:       domain.TaskCostOverrideAuditEventApplied,
					CategoryCode:    "HBJ",
					OverrideCost:    float64Ptr(18.5),
					ResultCostPrice: float64Ptr(18.5),
					OverrideReason:  "manual",
					OverrideActor:   "operator:3",
					OverrideAt:      overrideAt,
					Source:          "task_business_info_patch",
					CreatedAt:       overrideAt,
				},
			},
		},
	}
	reviewRepo := &prdTaskCostOverrideReviewRepo{}
	financeRepo := &prdTaskCostFinanceFlagRepo{}
	svc := NewTaskCostOverrideAuditService(taskRepo, overrideRepo, &prdTaskEventRepo{}, reviewRepo, financeRepo)

	ctx := domain.WithRequestActor(context.Background(), domain.RequestActor{ID: 51})
	reviewBoundary, appErr := svc.UpsertReview(ctx, UpsertTaskCostOverrideReviewParams{
		TaskID:          5,
		OverrideEventID: "cov-5-1",
		ReviewRequired:  testBoolPtr(true),
		ReviewStatus:    domain.TaskCostOverrideReviewStatusApproved,
		ReviewNote:      "placeholder review ok",
	})
	if appErr != nil {
		t.Fatalf("UpsertReview() unexpected error: %+v", appErr)
	}
	if reviewBoundary.ReviewActor != "actor:51" {
		t.Fatalf("review_actor = %s, want actor:51", reviewBoundary.ReviewActor)
	}

	financeBoundary, appErr := svc.UpsertFinanceFlag(ctx, UpsertTaskCostFinanceFlagParams{
		TaskID:          5,
		OverrideEventID: "cov-5-1",
	})
	if appErr != nil {
		t.Fatalf("UpsertFinanceFlag() unexpected error: %+v", appErr)
	}
	if financeBoundary.FinanceStatus != domain.TaskCostOverrideFinanceStatusReadyForView {
		t.Fatalf("finance_status = %s, want %s", financeBoundary.FinanceStatus, domain.TaskCostOverrideFinanceStatusReadyForView)
	}
	if financeBoundary.FinanceMarkedBy != "actor:51" {
		t.Fatalf("finance_marked_by = %s, want actor:51", financeBoundary.FinanceMarkedBy)
	}
	if financeBoundary.GovernanceBoundarySummary == nil || financeBoundary.GovernanceBoundarySummary.LatestBoundaryActor != "actor:51" {
		t.Fatalf("governance_boundary_summary = %+v", financeBoundary.GovernanceBoundarySummary)
	}
	if financeBoundary.FinancePlaceholderSummary == nil || financeBoundary.FinancePlaceholderSummary.LatestFinanceAction == nil {
		t.Fatalf("finance_placeholder_summary = %+v", financeBoundary.FinancePlaceholderSummary)
	}

	timeline, appErr := svc.ListByTaskID(ctx, 5)
	if appErr != nil {
		t.Fatalf("ListByTaskID() unexpected error: %+v", appErr)
	}
	if timeline.OverrideBoundary == nil || !timeline.OverrideBoundary.FinanceViewReady {
		t.Fatalf("timeline.override_boundary = %+v", timeline.OverrideBoundary)
	}
	if timeline.OverrideBoundary.PlatformEntryBoundary == nil || timeline.OverrideBoundary.PlatformEntryBoundary.ReportEntrySummary == nil {
		t.Fatalf("timeline.override_boundary.platform_entry_boundary = %+v", timeline.OverrideBoundary.PlatformEntryBoundary)
	}
	if len(timeline.Events) != 1 || timeline.Events[0].OverrideBoundary == nil {
		t.Fatalf("timeline.events = %+v", timeline.Events)
	}
	if timeline.Events[0].OverrideBoundary.ReviewRecordID == nil || *timeline.Events[0].OverrideBoundary.ReviewRecordID != 1 {
		t.Fatalf("review_record_id = %+v", timeline.Events[0].OverrideBoundary)
	}
	if timeline.Events[0].OverrideBoundary.FinanceRecordID == nil || *timeline.Events[0].OverrideBoundary.FinanceRecordID != 1 {
		t.Fatalf("finance_record_id = %+v", timeline.Events[0].OverrideBoundary)
	}
	if timeline.OverrideBoundary.GovernanceBoundarySummary == nil || timeline.OverrideBoundary.GovernanceBoundarySummary.LatestFinanceAction == nil {
		t.Fatalf("timeline.governance_boundary_summary = %+v", timeline.OverrideBoundary)
	}
}

func testBoolPtr(value bool) *bool {
	return &value
}
