package assetworkbench

import (
	"context"
	"reflect"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

type personnelSuggestionsRepo struct {
	repo.AssetWorkbenchRepo
	candidates []domain.AssetWorkbenchPersonnelSuggestion
	scope      string
	ownerID    *int64
}

func (r *personnelSuggestionsRepo) ListPersonnelSuggestionCandidates(_ context.Context, scope, _ string) ([]domain.AssetWorkbenchPersonnelSuggestion, error) {
	r.scope = scope
	return r.candidates, nil
}

func (r *personnelSuggestionsRepo) DriveListFiles(_ context.Context, filter repo.AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveFile, int64, error) {
	r.ownerID = filter.OwnerUserID
	return nil, 0, nil
}

func TestRankPersonnelSuggestions(t *testing.T) {
	people := []domain.AssetWorkbenchPersonnelSuggestion{
		{UserID: 7, Name: "张敏", Username: "example-one", PersonnelNo: "QA-07"},
		{UserID: 8, Name: "李明", Username: "example-two"},
		{UserID: 9, Name: "章明", Username: "example-three", PersonnelNo: "QA-09"},
	}
	for _, test := range []struct {
		query string
		want  []int64
	}{
		{query: "章", want: []int64{9, 7}},
		{query: "zhang", want: []int64{7, 9}},
		{query: "zm", want: []int64{7, 9}},
		{query: "明", want: []int64{8, 9}},
		{query: "qa-09", want: []int64{9}},
		{query: "8", want: []int64{8}},
		{query: "example-two", want: []int64{8}},
		{query: "%", want: []int64{}},
	} {
		t.Run(test.query, func(t *testing.T) {
			got := rankPersonnelSuggestions(people, test.query)
			ids := make([]int64, 0, len(got.Items))
			for _, person := range got.Items {
				ids = append(ids, person.UserID)
			}
			if !reflect.DeepEqual(ids, test.want) || got.Total != len(test.want) {
				t.Fatalf("got IDs %v (total %d), want %v", ids, got.Total, test.want)
			}
		})
	}
}

func TestPersonnelSearchReturnsAllMatchesBeyondInitialRoster(t *testing.T) {
	people := make([]domain.AssetWorkbenchPersonnelSuggestion, 60)
	for i := range people {
		people[i] = domain.AssetWorkbenchPersonnelSuggestion{UserID: int64(i + 1), Name: "张敏"}
	}
	if got := rankPersonnelSuggestions(people, "张"); len(got.Items) != len(people) || got.Total != len(people) {
		t.Fatalf("search showed %d of %d matches, want %d", len(got.Items), got.Total, len(people))
	}
	if got := rankPersonnelSuggestions(people, ""); len(got.Items) != 50 || got.Total != len(people) {
		t.Fatalf("initial roster showed %d of %d profiles", len(got.Items), got.Total)
	}
}

func TestPersonnelSuggestionsAndExactUploaderUseExistingReadScopes(t *testing.T) {
	ctx := context.Background()
	r := &personnelSuggestionsRepo{candidates: []domain.AssetWorkbenchPersonnelSuggestion{{UserID: 7, Name: "张敏"}}}
	s := NewService(Config{}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	manager := domain.RequestActor{ID: 22, EffectiveAccess: &domain.EffectiveAccess{
		Permissions: []domain.PermissionCode{domain.PermissionAssetManage, domain.PermissionAssetWorkbenchMembers, domain.PermissionAssetWorkbenchAuditView},
	}}
	worker := domain.RequestActor{ID: 23, EffectiveAccess: &domain.EffectiveAccess{
		Permissions: []domain.PermissionCode{domain.PermissionAssetWorkbenchSubmit},
	}}
	settler := domain.RequestActor{ID: 24, EffectiveAccess: &domain.EffectiveAccess{
		Permissions: []domain.PermissionCode{domain.PermissionAssetWorkbenchSettlement},
	}}

	for _, test := range []struct {
		actor       domain.RequestActor
		scope       string
		wantAllowed bool
	}{
		{actor: manager, scope: "upload_owner", wantAllowed: true},
		{actor: manager, scope: "member", wantAllowed: true},
		{actor: manager, scope: "audit_actor", wantAllowed: true},
		{actor: manager, scope: "main_account"},
		{actor: manager, scope: "payee"},
		{actor: worker, scope: "upload_owner"},
		{actor: worker, scope: "payee"},
		{actor: settler, scope: "upload_owner", wantAllowed: true},
		{actor: settler, scope: "payee", wantAllowed: true},
	} {
		got, appErr := s.SuggestPersonnel(ctx, test.actor, test.scope, "张", "")
		if test.wantAllowed {
			if appErr != nil || got.Total != 1 || r.scope != test.scope {
				t.Fatalf("actor %d searching %s: got=%+v err=%+v", test.actor.ID, test.scope, got, appErr)
			}
		} else if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
			t.Fatalf("actor %d should be denied for %s: %v", test.actor.ID, test.scope, appErr)
		}
	}

	ownerID := int64(7)
	filter := repo.AssetWorkbenchDriveFilter{OwnerUserID: &ownerID}
	if _, appErr := s.ListDriveFiles(ctx, manager, filter); appErr != nil || r.ownerID == nil || *r.ownerID != ownerID {
		t.Fatalf("site-wide exact owner filter: owner=%v err=%v", r.ownerID, appErr)
	}
	if _, appErr := s.ListDriveFiles(ctx, worker, filter); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("another employee's ledger filter must be denied, got %v", appErr)
	}
	if _, appErr := s.ListDriveUploadDays(ctx, worker, "2026-11", filter); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("another employee's calendar filter must be denied, got %v", appErr)
	}
}
