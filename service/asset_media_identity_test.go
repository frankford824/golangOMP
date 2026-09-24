package service

import (
	"context"
	"testing"
	"workflow/domain"
)

func TestMediaContentIdentitySurvivesPreparationAndObjectRelocation(t *testing.T) {
	s := &taskAssetCenterService{}
	source := &domain.DesignAssetVersion{ID: 123, OriginalFilename: "source.jpg"}
	ctx := domain.WithAssetMediaOptions(context.Background(), domain.AssetMediaOptions{AuthorizeOnly: true})
	pending, err := s.pendingMediaPreview(ctx, 1, 2, source, true)
	if err != nil {
		t.Fatal(err)
	}
	derived := &domain.DesignAssetVersion{ID: 999, SourceAssetVersionID: &source.ID, StorageKey: "first-derived-key"}
	ready := &domain.AssetDownloadInfo{}
	decorateVersionMediaInfo(ready, derived, "thumbnail")
	if pending.ContentID != ready.ContentID {
		t.Fatal("pending and ready representation IDs differ")
	}
	derived.StorageKey = "regenerated-same-version-key"
	second := &domain.AssetDownloadInfo{}
	decorateVersionMediaInfo(second, derived, "thumbnail")
	if ready.ContentID != second.ContentID {
		t.Fatal("delivery location changed logical content identity")
	}
	if ready.ContentID == domain.AssetMediaIdentity(ready.SourceVersion, "preview", domain.AssetMediaRecipe) {
		t.Fatal("thumbnail and preview identities collided")
	}
}
