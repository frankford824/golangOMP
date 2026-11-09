package assetworkbench

import (
	"context"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type personalLedgerFilterRepo struct {
	repo.AssetWorkbenchRepo
	filter repo.AssetWorkbenchDriveFilter
	month  time.Time
}

func (r *personalLedgerFilterRepo) DriveUploadDays(_ context.Context, filter repo.AssetWorkbenchDriveFilter, monthStart time.Time) ([]string, error) {
	r.filter, r.month = filter, monthStart
	return []string{"2026-10-02"}, nil
}

func TestUploadCalendarKeepsOwnerScopeAndShanghaiMonth(t *testing.T) {
	r := &personalLedgerFilterRepo{}
	s := NewService(Config{}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 22, Roles: []domain.Role{domain.RoleAssetSubmitter}}
	days, err := s.ListDriveUploadDays(context.Background(), actor, "2026-10", repo.AssetWorkbenchDriveFilter{
		OwnerKeyword: "another owner", OperationSource: "client_supplement",
	})
	if err != nil || len(days) != 1 || days[0] != "2026-10-02" {
		t.Fatalf("days=%v error=%v", days, err)
	}
	if r.filter.OwnerUserID == nil || *r.filter.OwnerUserID != actor.ID || r.filter.OperationSource != "client_supplement" {
		t.Fatalf("incorrect calendar scope: %+v", r.filter)
	}
	if got := r.month.UTC().Format(time.RFC3339); got != "2026-09-30T16:00:00Z" {
		t.Fatalf("incorrect Shanghai month boundary: %s", got)
	}
	if _, err := s.ListDriveUploadDays(context.Background(), actor, "2026-13", repo.AssetWorkbenchDriveFilter{}); err == nil {
		t.Fatal("invalid month should be rejected")
	}
}

func (r *personalLedgerFilterRepo) DriveListFiles(_ context.Context, filter repo.AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveFile, int64, error) {
	r.filter = filter
	return nil, 0, nil
}

func (r *personalLedgerFilterRepo) DriveSearchFiles(ctx context.Context, filter repo.AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveFile, int64, error) {
	return r.DriveListFiles(ctx, filter)
}

func TestPersonalLedgerAlwaysScopesDatesAndOwnerKeywordsToAuthenticatedSubmitter(t *testing.T) {
	for _, keyword := range []string{"", "作品"} {
		t.Run(keyword, func(t *testing.T) {
			r := &personalLedgerFilterRepo{}
			s := NewService(Config{}, WithRepository(r, assetWorkbenchTestTxRunner{}))
			from := time.Date(2026, 10, 13, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
			to := from.Add(24*time.Hour - time.Nanosecond)
			_, appErr := s.ListDriveFiles(context.Background(), domain.RequestActor{ID: 22, Roles: []domain.Role{domain.RoleAssetSubmitter}}, nil, false, "", keyword, "other-user", "normal_upload", &from, &to, "created_at", "desc", 1, 50)
			if appErr != nil {
				t.Fatal(appErr)
			}
			if r.filter.OwnerUserID == nil || *r.filter.OwnerUserID != 22 {
				t.Fatal("creator keyword must never override authenticated owner scope")
			}
			if r.filter.CreatedFrom == nil || !r.filter.CreatedFrom.Equal(from) || r.filter.CreatedTo == nil || !r.filter.CreatedTo.Equal(to) {
				t.Fatal("personal ledger date scope was lost")
			}
		})
	}
}
