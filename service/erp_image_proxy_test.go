package service

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestERPImageProxySignerBuildsShortSignedURLAndVerifies(t *testing.T) {
	asset := erpImageProxyTestAsset(4395, "tasks/RW-20260603-A-001080/assets/AST-0005/v1/delivery/image.jpg")
	signer := NewERPImageProxySigner(ERPImageProxyConfig{
		PublicBaseURL: "https://yongbo.cloud",
		SigningSecret: "proxy-secret",
		TokenTTL:      time.Hour,
	})
	now := time.Date(2026, 6, 4, 8, 0, 0, 0, time.UTC)
	signer.now = func() time.Time { return now }

	imageURL := signer.BuildImageURL(asset)
	if imageURL == nil {
		t.Fatal("BuildImageURL() = nil")
	}
	if len(*imageURL) >= 300 {
		t.Fatalf("short image url length = %d, want < 300: %s", len(*imageURL), *imageURL)
	}
	if !strings.HasPrefix(*imageURL, "https://yongbo.cloud/v1/public/erp-product-images/4395?") {
		t.Fatalf("image url = %q", *imageURL)
	}
	exp := queryValue(t, *imageURL, "exp")
	sig := queryValue(t, *imageURL, "sig")
	if !signer.Verify(asset, exp, sig) {
		t.Fatal("Verify() = false, want true")
	}

	otherStorageKey := "tasks/RW-20260603-A-001080/assets/AST-0005/v1/delivery/changed.jpg"
	asset.StorageKey = &otherStorageKey
	if signer.Verify(asset, exp, sig) {
		t.Fatal("Verify() = true after storage key changed, want false")
	}
}

func TestProductManagementResolveERPImageURLUsesProxyBeforeLongOSSSignedURL(t *testing.T) {
	storageKey := "tasks/RW-20260603-A-001080/assets/AST-0005/v1/delivery/" + strings.Repeat("long-name-", 16) + "image.jpg"
	assetID := int64(7301)
	asset := erpImageProxyTestAsset(4395, storageKey)
	asset.AssetID = &assetID
	signer := NewERPImageProxySigner(ERPImageProxyConfig{
		PublicBaseURL: "https://yongbo.cloud",
		SigningSecret: "proxy-secret",
		TokenTTL:      time.Hour,
	})
	svc := &productManagementService{
		assetSearch: productManagementAssetSearchStub{row: &repo.TaskAssetSearchRow{Asset: asset}},
		imageProxy:  signer,
	}

	got, appErr := svc.resolveERPImageURL(context.Background(), &domain.ProductManagementRecord{ImageAssetID: &assetID})
	if appErr != nil {
		t.Fatalf("resolveERPImageURL() appErr = %+v", appErr)
	}
	if len(got) >= 300 {
		t.Fatalf("resolveERPImageURL() length = %d, want < 300: %s", len(got), got)
	}
	if !strings.HasPrefix(got, "https://yongbo.cloud/v1/public/erp-product-images/4395?") {
		t.Fatalf("resolveERPImageURL() = %q", got)
	}
}

func TestProductManagementResolveERPImageURLUsesSelectedAssetVersion(t *testing.T) {
	assetID := int64(7301)
	selectedVersionID := int64(4395)
	current := erpImageProxyTestAsset(9900, "tasks/current/image.jpg")
	current.AssetID = &assetID
	selected := erpImageProxyTestAsset(selectedVersionID, "tasks/selected/image.jpg")
	selected.AssetID = &assetID
	signer := NewERPImageProxySigner(ERPImageProxyConfig{
		PublicBaseURL: "https://yongbo.cloud",
		SigningSecret: "proxy-secret",
		TokenTTL:      time.Hour,
	})
	search := productManagementAssetSearchStub{
		row:        &repo.TaskAssetSearchRow{Asset: current},
		versionRow: &repo.TaskAssetSearchRow{Asset: selected},
	}
	svc := &productManagementService{
		assetSearch: search,
		imageProxy:  signer,
	}

	got, appErr := svc.resolveERPImageURL(context.Background(), &domain.ProductManagementRecord{
		ImageAssetID:        &assetID,
		ImageAssetVersionID: &selectedVersionID,
	})
	if appErr != nil {
		t.Fatalf("resolveERPImageURL() appErr = %+v", appErr)
	}
	if !strings.HasPrefix(got, "https://yongbo.cloud/v1/public/erp-product-images/4395?") {
		t.Fatalf("resolveERPImageURL() = %q, want selected version path", got)
	}
}

type productManagementAssetSearchStub struct {
	row        *repo.TaskAssetSearchRow
	versionRow *repo.TaskAssetSearchRow
	err        error
}

func (s productManagementAssetSearchStub) Search(context.Context, domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	return nil, 0, nil
}

func (s productManagementAssetSearchStub) GetCurrentByAssetID(context.Context, int64) (*repo.TaskAssetSearchRow, error) {
	return s.row, s.err
}

func (s productManagementAssetSearchStub) ListCurrentByAssetIDs(context.Context, []int64) ([]*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

func (s productManagementAssetSearchStub) ListVersionsByAssetID(context.Context, int64) ([]*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

func (s productManagementAssetSearchStub) GetVersion(context.Context, int64, int64) (*repo.TaskAssetSearchRow, error) {
	return s.versionRow, s.err
}

func erpImageProxyTestAsset(versionID int64, storageKey string) *domain.TaskAsset {
	mimeType := "image/jpeg"
	fileName := "image.jpg"
	return &domain.TaskAsset{
		ID:         versionID,
		TaskID:     1080,
		FileName:   fileName,
		MimeType:   &mimeType,
		StorageKey: &storageKey,
	}
}

func queryValue(t *testing.T, rawURL, key string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url %q: %v", rawURL, err)
	}
	value := parsed.Query().Get(key)
	if value == "" {
		t.Fatalf("query %q missing in %q", key, rawURL)
	}
	return value
}
