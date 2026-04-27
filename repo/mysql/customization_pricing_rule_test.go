package mysqlrepo

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
)

func TestCustomizationPricingRuleRepoGetActiveByLevelAndEmploymentType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "customization_level_code", "employment_type", "unit_price", "weight_factor", "is_enabled", "created_at", "updated_at",
	}).AddRow(
		int64(1), "L1", "part_time", 18.5, 0.95, 1, now, now,
	)
	mock.ExpectQuery("FROM customization_pricing_rules").
		WithArgs("L1", "part_time").
		WillReturnRows(rows)

	repo := NewCustomizationPricingRuleRepo(New(db))
	item, err := repo.GetActiveByLevelAndEmploymentType(context.Background(), "L1", domain.EmploymentTypePartTime)
	if err != nil {
		t.Fatalf("GetActiveByLevelAndEmploymentType() error = %v", err)
	}
	if item == nil {
		t.Fatal("GetActiveByLevelAndEmploymentType() = nil")
	}
	if item.EmploymentType != domain.EmploymentTypePartTime {
		t.Fatalf("item.EmploymentType = %q, want %q", item.EmploymentType, domain.EmploymentTypePartTime)
	}
	if item.UnitPrice != 18.5 || item.WeightFactor != 0.95 {
		t.Fatalf("item price snapshot = (%v, %v), want (18.5, 0.95)", item.UnitPrice, item.WeightFactor)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock.ExpectationsWereMet() = %v", err)
	}
}
