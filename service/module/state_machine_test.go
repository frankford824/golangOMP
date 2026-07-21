package module

import (
	"testing"

	"workflow/domain"
)

func TestNextState_Table(t *testing.T) {
	tests := []struct {
		module string
		state  domain.ModuleState
		action string
		want   domain.ModuleState
		ok     bool
	}{
		{domain.ModuleKeyCustomization, domain.ModuleStateInProgress, domain.ModuleActionSubmit, domain.ModuleStateSubmitted, true},
		{domain.ModuleKeyRetouch, domain.ModuleStateInProgress, domain.ModuleActionSubmit, domain.ModuleStateSubmitted, true},
		{domain.ModuleKeyDesign, domain.ModuleStateInProgress, domain.ModuleActionSubmit, "", false},
		{domain.ModuleKeyAudit, domain.ModuleStateInProgress, domain.ModuleActionApprove, "", false},
		{domain.ModuleKeyCustomization, domain.ModuleStateSubmitted, domain.ModuleActionSubmit, "", false},
	}
	for _, tt := range tests {
		got, _, ok := NextState(tt.module, tt.state, tt.action)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("NextState(%s,%s,%s)=(%s,%v), want (%s,%v)", tt.module, tt.state, tt.action, got, ok, tt.want, tt.ok)
		}
	}
}
