package asset_lifecycle

import (
	"testing"

	"workflow/domain"
)

func TestStateMachineGuards(t *testing.T) {
	tests := []struct {
		state     domain.AssetLifecycleState
		canDelete bool
	}{
		{domain.AssetLifecycleStateActive, true},
		{domain.AssetLifecycleStateClosedRetained, true},
		{domain.AssetLifecycleStateArchived, true},
		{domain.AssetLifecycleStateAutoCleaned, false},
		{domain.AssetLifecycleStateDeleted, false},
	}
	for _, tt := range tests {
		if got := CanDelete(tt.state); got != tt.canDelete {
			t.Fatalf("CanDelete(%s) = %t, want %t", tt.state, got, tt.canDelete)
		}
	}
}
