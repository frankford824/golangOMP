package service

import (
	"encoding/json"
	"testing"

	"workflow/domain"
)

func TestNewPlanningTaskDetailUsesValidRiskFlagsJSON(t *testing.T) {
	detail := newPlanningTaskDetail([]domain.PlanningSKUItemInput{{Quantity: 2}, {Quantity: 3}})

	if !json.Valid([]byte(detail.RiskFlagsJSON)) {
		t.Fatalf("RiskFlagsJSON = %q, want valid JSON", detail.RiskFlagsJSON)
	}
	if detail.Quantity == nil || *detail.Quantity != 5 {
		t.Fatalf("Quantity = %v, want 5", detail.Quantity)
	}
}
