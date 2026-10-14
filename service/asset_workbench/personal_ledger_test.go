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
