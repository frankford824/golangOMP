package mysqlrepo

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
	"workflow/repo"
)

func TestCustomizationJobRepoUpdatePersistsPricingWorkerType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	sqlDB := New(db)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE customization_jobs").
		WithArgs(
			sqlmock.AnyArg(), // source_asset_id
			sqlmock.AnyArg(), // current_asset_id
			"",               // order_no
			"L1",
			"Level 1",
			sqlmock.AnyArg(), // review_reference_unit_price
			sqlmock.AnyArg(), // review_reference_weight_factor
			sqlmock.AnyArg(), // unit_price
			sqlmock.AnyArg(), // weight_factor
			"",
			"approved",
			"final",
			sqlmock.AnyArg(), // assigned_operator_id
			sqlmock.AnyArg(), // last_operator_id
			"part_time",
			"pending_effect_review",
			"",
			"",
			int64(9),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	appErr := sqlDB.RunInTx(context.Background(), func(tx repo.Tx) error {
		r := NewCustomizationJobRepo(sqlDB)
		unitPrice := 18.8
		weightFactor := 0.88
		reviewReferenceUnitPrice := 22.4
		reviewReferenceWeightFactor := 1.15
		return r.Update(context.Background(), tx, &domain.CustomizationJob{
			ID:                          9,
			TaskID:                      2,
			CustomizationLevelCode:      "L1",
			CustomizationLevelName:      "Level 1",
			ReviewReferenceUnitPrice:    &reviewReferenceUnitPrice,
			ReviewReferenceWeightFactor: &reviewReferenceWeightFactor,
			UnitPrice:                   &unitPrice,
			WeightFactor:                &weightFactor,
			ReviewDecision:              domain.CustomizationReviewDecisionApproved,
			DecisionType:                domain.CustomizationJobDecisionTypeFinal,
			PricingWorkerType:           domain.EmploymentTypePartTime,
			Status:                      domain.CustomizationJobStatusPendingEffectReview,
		})
	})
	if appErr != nil {
		t.Fatalf("RunInTx(Update customization job) error = %v", appErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock.ExpectationsWereMet() = %v", err)
	}
}
