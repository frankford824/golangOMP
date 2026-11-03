package assetworkbench

import (
	"context"
	"testing"
	"time"
	"workflow/domain"
	"workflow/repo"
)

func (r *supplementRepo) ListSettlementMonthStates(context.Context) (map[string]bool, error) {
	return r.monthStates, nil
}
func (r *supplementRepo) RunSettlementMonthTx(_ context.Context, _ string, fn func(repo.Tx) error) error {
	if r.monthTxHook != nil {
		r.monthTxHook()
	}
	return fn(assetWorkbenchTestTx{})
}
func (r *settlementBatchRepo) RunSettlementMonthTx(_ context.Context, _ string, fn func(repo.Tx) error) error {
	return fn(assetWorkbenchTestTx{})
}
func (r *submissionDirectoryDifficultyRepo) ListSettlementMonthStates(context.Context) (map[string]bool, error) {
	return nil, nil
}
func (r *submissionDirectoryDifficultyRepo) RunSettlementMonthTx(_ context.Context, _ string, fn func(repo.Tx) error) error {
	return fn(assetWorkbenchTestTx{})
}

func TestHistoricalSupplementMonthLifecycle(t *testing.T) {
	for _, test := range []struct {
		name         string
		locked, race bool
		wantCode     string
	}{
		{"unsettled", false, false, ""}, {"generated or confirmed", true, false, domain.ErrCodeConflict}, {"batch generated after precheck", false, true, domain.ErrCodeConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := &supplementRepo{monthStates: map[string]bool{"2026-10": test.locked}}
			s := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(r, assetWorkbenchTestTxRunner{}))
			s.nowFn = func() time.Time { return time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC) }
			actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}
			permission, err := s.UpsertSupplementPermission(context.Background(), actor, UpsertSupplementPermissionParams{PayeeUserID: 1001, BusinessMonth: "2026-10", Enabled: true})
			if test.locked {
				if err == nil || err.Code != test.wantCode || len(r.permissions) != 0 {
					t.Fatalf("locked permission=%+v err=%+v", permission, err)
				}
				// Closing old permissions must remain possible even in a locked month.
				_, err = s.UpsertSupplementPermission(context.Background(), actor, UpsertSupplementPermissionParams{PayeeUserID: 1001, BusinessMonth: "2026-10", Enabled: false})
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.race {
				r.monthTxHook = func() { r.monthStates["2026-10"] = true }
			}
			created, err := s.CreateSettlementSupplement(context.Background(), actor, CreateSettlementSupplementParams{PayeeUserID: 1001, BusinessMonth: "2026-10", SupplementDate: "2026-10-20", OrderNo: "missing.png", DifficultyClass: "A", PageCount: 1, GrossAmount: 12, Status: domain.AssetWorkbenchSupplementStatusApproved})
			if test.wantCode != "" {
				if err == nil || err.Code != test.wantCode || r.created != nil {
					t.Fatalf("race created=%+v err=%+v", created, err)
				}
			} else if err != nil || created.BusinessMonth != "2026-10" || created.SupplementDate != "2026-10-20" {
				t.Fatalf("historical created=%+v err=%+v", created, err)
			}
		})
	}
}

func TestEligibleSupplementMonthsExcludeLockedAndIncludeEmptyGaps(t *testing.T) {
	r := &supplementRepo{monthStates: map[string]bool{"2026-07": false, "2026-09": true, "2026-10": true}}
	s := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	s.nowFn = func() time.Time { return time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC) }
	months, err := s.ListSupplementEligibleMonths(context.Background(), domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}, 1001)
	if err != nil || len(months) != 3 || months[0] != "2026-11" || months[1] != "2026-08" || months[2] != "2026-07" {
		t.Fatalf("months=%v err=%+v", months, err)
	}
}
