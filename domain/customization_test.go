package domain

import "testing"

func TestCustomizationJobStatusValidOnlyAcceptsCurrentInternalStates(t *testing.T) {
	for _, status := range []CustomizationJobStatus{
		CustomizationJobStatusInProgress,
		CustomizationJobStatusReadyForSubmit,
		CustomizationJobStatusCompleted,
	} {
		if !status.Valid() {
			t.Fatalf("status %q must be valid", status)
		}
	}

	for _, historical := range []CustomizationJobStatus{
		"pending_customization_review",
		CustomizationJobStatusLegacyPendingProduction,
		"pending_effect_review",
		"pending_effect_revision",
		"pending_production_transfer",
		"pending_warehouse_qc",
		"rejected_by_warehouse",
	} {
		if historical.Valid() {
			t.Fatalf("historical status %q must not be accepted by current writes", historical)
		}
	}
}
