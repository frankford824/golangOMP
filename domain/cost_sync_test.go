package domain

import "testing"

func TestCostObservationPolicy(t *testing.T) {
	p := func(n float64) *float64 { return &n }
	cases := []struct {
		name string
		s    CostSyncState
		erp  *float64
		want string
	}{
		{"echo", CostSyncState{LocalCost: p(12), ERPCost: p(10), Revision: 2, AckRevision: 1, ManualLock: true, Status: "pending"}, p(12), "agree"},
		{"manual conflict", CostSyncState{LocalCost: p(10), ERPCost: p(10), Revision: 1, AckRevision: 1, ManualLock: true, Status: "synced"}, p(12), "conflict"},
		{"two writers", CostSyncState{LocalCost: p(11), ERPCost: p(10), Revision: 2, AckRevision: 1, Status: "pending"}, p(12), "conflict"},
		{"ERP only", CostSyncState{LocalCost: p(10), ERPCost: p(10), Revision: 1, AckRevision: 1, Status: "synced"}, p(12.3456), "accept_erp"},
		{"historic conflict stays", CostSyncState{LocalCost: p(10), ERPCost: p(12), Revision: 1, Status: "conflict"}, p(12), "conflict"},
		{"local pending", CostSyncState{LocalCost: p(11), ERPCost: p(10), Revision: 2, AckRevision: 1, Status: "pending"}, p(10), "unchanged"},
		{"ERP removal", CostSyncState{LocalCost: p(10), ERPCost: p(10), Revision: 1, AckRevision: 1, Status: "synced"}, nil, "conflict"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.s.LocalConfirmed = true
			if got := DecideCostObservation(&tc.s, tc.erp); got != tc.want {
				t.Fatalf("%s != %s", got, tc.want)
			}
		})
	}
	s := CostSyncState{LocalCost: p(10), ERPCost: p(10), Revision: 1, AckRevision: 1, Status: "synced"}
	if DecideCostObservation(&s, p(10)) != "conflict" {
		t.Fatal("unconfirmed prices must not be acknowledged")
	}
	s.LocalConfirmed = true
	s.ManualLock = true
	s.ManualOrigin = "erp"
	if DecideCostObservation(&s, p(12)) != "accept_erp" {
		t.Fatal("consecutive ERP-only edits must continue to synchronize")
	}
	if DecideCostObservation(&s, p(0)) != "conflict" {
		t.Fatal("ERP zero cannot silently replace a nonzero cost")
	}
}
