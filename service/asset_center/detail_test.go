package asset_center

import (
	"context"
	"testing"

	"workflow/domain"
	externalassets "workflow/service/external_assets"
)

func TestGetDetail_NotFound_ReturnsErrNotFound(t *testing.T) {
	svc := NewService(&fakeSearchRepo{}, nil, nil)

	detail, appErr := svc.GetDetail(context.Background(), 999999999)
	if detail != nil {
		t.Fatalf("GetDetail detail = %#v, want nil", detail)
	}
	if appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("GetDetail error = %#v, want %s", appErr, domain.ErrCodeNotFound)
	}
}

func TestExternalDetailDisabledReturnsNotFound(t *testing.T) {
	svc := NewService(&fakeSearchRepo{}, nil, nil)
	svc.SetExternalAssetService(externalassets.NewService(nil, externalassets.Config{Enabled: false}, nil))

	detail, appErr := svc.GetExternalDetail(context.Background(), 42)
	if detail != nil {
		t.Fatalf("GetExternalDetail detail = %#v, want nil", detail)
	}
	if appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("GetExternalDetail error = %#v, want %s", appErr, domain.ErrCodeNotFound)
	}
}

func TestExternalSearchDisabledReturnsEmpty(t *testing.T) {
	svc := NewService(&fakeSearchRepo{}, nil, nil)
	svc.SetExternalAssetService(externalassets.NewService(nil, externalassets.Config{Enabled: false}, nil))

	result, appErr := svc.Search(context.Background(), domain.AssetSearchQuery{
		Source: domain.AssetResourceSourceExternal,
		Page:   1,
		Size:   20,
	})
	if appErr != nil {
		t.Fatalf("Search external appErr = %#v", appErr)
	}
	if result == nil || result.Total != 0 || len(result.Items) != 0 {
		t.Fatalf("Search external result = %#v, want empty", result)
	}
}

func TestAssetSearchHasSystemOnlyFiltersIncludesTaskCreatedTimeBasisAndModule(t *testing.T) {
	if !assetSearchHasSystemOnlyFilters(domain.AssetSearchQuery{TimeBasis: domain.AssetSearchTimeBasisTaskCreatedAt}) {
		t.Fatal("task_created_at time basis should be system-only")
	}
	if !assetSearchHasSystemOnlyFilters(domain.AssetSearchQuery{ModuleKey: domain.ModuleKeyAudit}) {
		t.Fatal("module_key should be system-only")
	}
	if assetSearchHasSystemOnlyFilters(domain.AssetSearchQuery{TimeBasis: domain.AssetSearchTimeBasisUploadedAt}.Normalized()) {
		t.Fatal("asset_uploaded_at alone should not be system-only")
	}
}
