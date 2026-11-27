package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestERPProductImageRotationKeepsOldLinksOnlyDuringGrace(t *testing.T) {
	now := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	asset := &domain.TaskAsset{ID: 42, StorageKey: new(string)}
	*asset.StorageKey = "tasks/RW-1/assets/sample.png"
	old := NewERPImageProxySigner(ERPImageProxyConfig{PublicBaseURL: "https://image.test", SigningSecret: "previous"})
	old.now = func() time.Time { return now }
	next := NewERPImageProxySigner(ERPImageProxyConfig{
		PublicBaseURL: "https://image.test", SigningSecret: "current",
		PreviousSigningSecret: "previous", PreviousValidUntil: now.Add(time.Hour),
	})
	next.now = func() time.Time { return now }
	link, err := url.Parse(*old.BuildImageURL(asset))
	if err != nil {
		t.Fatal(err)
	}
	q := link.Query()
	if !next.Verify(asset, q.Get("exp"), q.Get("sig")) || next.verifiesCurrentKey(asset, q.Get("exp"), q.Get("sig")) {
		t.Fatal("old link must be accepted temporarily and selected for renewal")
	}
	legacy, _ := url.Parse(*next.BuildLegacyImageURL(asset.ID, *asset.StorageKey))
	legacyQuery := legacy.Query()
	if !next.VerifyLegacyImage(asset.ID, *asset.StorageKey, legacyQuery.Get("exp"), legacyQuery.Get("sig")) ||
		next.Verify(asset, legacyQuery.Get("exp"), legacyQuery.Get("sig")) {
		t.Fatal("legacy-object signature must not authorize a normal asset ID")
	}
	now = now.Add(time.Hour)
	if next.Verify(asset, q.Get("exp"), q.Get("sig")) {
		t.Fatal("old signing key remained valid past rotation deadline")
	}
}

func TestERPProductImageReadbackDoesNotCollapseSlots(t *testing.T) {
	raw := map[string]interface{}{"sku_id": "CG1", "pic": "", "pic_big": "https://host.test/big.png"}
	product := jstMapsToERPProducts([]map[string]interface{}{raw}, "")[0]
	if product.ImageURL != raw["pic_big"] || product.ImageFields == nil || product.ImageFields.Pic != "" ||
		product.ImageFields.PicBig != raw["pic_big"] {
		t.Fatalf("independent slots were lost: %+v", product)
	}
	expected := domain.ERPProductImageFields{Pic: product.ImageURL, PicBig: product.ImageURL}
	if erpImagePairMatches(expected, product.ImageFields) {
		t.Fatal("a healthy big image must not count as successful small-image readback")
	}
	if erpImageSlotMatches("https://oss.test/a.png?Expires=2", "https://oss.test/a.png?Expires=1") {
		t.Fatal("readback must not ignore a stale direct-link signature")
	}
}

func TestERPProductImageLegacyKeyRequiresSingleOwningSKU(t *testing.T) {
	target := repo.ERPProductImageTarget{TaskNo: "RW-42", SingleSKU: true}
	for _, key := range []string{
		"tasks/RW-43/assets/one.png", "tasks/RW-42/assets/../one.png",
		"tasks/RW-42/assets/one.psd", "tasks/RW-42/assets/one\\x.png",
	} {
		if erpImageLegacyKeyBelongs(key, target) {
			t.Fatalf("accepted legacy key %q", key)
		}
	}
	key := "tasks/RW-42/assets/a/v1/delivery/one.png"
	if !erpImageLegacyKeyBelongs(key, target) {
		t.Fatal("single SKU's image key rejected")
	}
	target.SingleSKU = false
	if erpImageLegacyKeyBelongs(key, target) {
		t.Fatal("ambiguous multi-SKU task cannot claim a legacy object")
	}
}

type erpImageTestRepo struct {
	repo.ERPProductImageRepo
	staged, confirmed []int64
}

func (r *erpImageTestRepo) ActiveAsset(_ context.Context, id int64) (*domain.TaskAsset, error) {
	return &domain.TaskAsset{ID: id}, nil
}

func (r *erpImageTestRepo) RecordReferences(_ context.Context, _, _ string, refs []repo.ERPProductImageReference, replace bool) error {
	for _, ref := range refs {
		if replace {
			r.confirmed = append(r.confirmed, ref.VersionID)
		} else {
			r.staged = append(r.staged, ref.VersionID)
		}
	}
	return nil
}

type erpImageTestBridge struct {
	ERPBridgeService
	readback *domain.ERPProduct
	onWrite  func(domain.ERPProductUpsertPayload)
}

func (b *erpImageTestBridge) UpsertProduct(_ context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
	b.onWrite(payload)
	return nil, nil
}

func (b *erpImageTestBridge) GetProductByID(context.Context, string) (*domain.ERPProduct, *domain.AppError) {
	return b.readback, nil
}

type erpImageRoundTripper func(*http.Request) (*http.Response, error)

func (r erpImageRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return r(req)
}

func TestERPProductImagePublishPinsOldUntilBothSlotsConfirmed(t *testing.T) {
	for _, partial := range []bool{true, false} {
		t.Run(fmt.Sprintf("partial=%v", partial), func(t *testing.T) {
			signer := NewERPImageProxySigner(ERPImageProxyConfig{PublicBaseURL: "https://image.test", SigningSecret: "secret"})
			oldKey, newKey := "old.png", "new.png"
			old := *signer.BuildImageURL(&domain.TaskAsset{ID: 41, StorageKey: &oldKey})
			next := *signer.BuildImageURL(&domain.TaskAsset{ID: 42, StorageKey: &newKey})
			repository := &erpImageTestRepo{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			current := &domain.ERPProduct{SKUID: "CG1", IID: "style", ProductName: "name",
				ImageFields: &domain.ERPProductImageFields{Pic: old, PicBig: old}}
			bridge := &erpImageTestBridge{onWrite: func(p domain.ERPProductUpsertPayload) {
				if !reflect.DeepEqual(repository.staged, []int64{41, 42}) || len(repository.confirmed) != 0 {
					t.Fatalf("pin state before write: staged=%v confirmed=%v", repository.staged, repository.confirmed)
				}
				if p.Pic != next || p.PicBig != next || p.CostPrice != nil {
					t.Fatal("wrong write")
				}
			}}
			result := *current
			result.ImageFields = &domain.ERPProductImageFields{Pic: next, PicBig: next}
			if partial {
				result.ImageFields.PicBig = old
				onWrite := bridge.onWrite
				bridge.onWrite = func(p domain.ERPProductUpsertPayload) { onWrite(p); cancel() }
			}
			bridge.readback = &result
			service := NewERPProductImages(repository, nil, signer, bridge, nil)
			service.client.Transport = erpImageRoundTripper(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 206, Body: io.NopCloser(strings.NewReader("\x89PNG\r\n\x1a\n"))}, nil
			})
			err := service.publish(ctx, "CG1", "token", current, domain.ERPProductImageFields{Pic: next, PicBig: next})
			if partial {
				if err == nil || len(repository.confirmed) != 0 {
					t.Fatalf("partial readback released old file: %v, %v", err, repository.confirmed)
				}
			} else if err != nil || !reflect.DeepEqual(repository.confirmed, []int64{42}) {
				t.Fatalf("successful readback did not release old file: %v, %v", err, repository.confirmed)
			}
		})
	}
}

func TestERPProductImageCheckPreservesSignedQueryEscapes(t *testing.T) {
	const query = "name=原图 1.jpg&Expires=3&Signature=A%2BB"
	const expected = "name=%E5%8E%9F%E5%9B%BE%201.jpg&Expires=3&Signature=A%2BB"
	if got := escapeERPImageQuery(query); got != expected {
		t.Fatalf("escaped ERP image query = %q", got)
	}
}

type erpImageLegacySKURepo struct {
	repo.ERPProductImageRepo
}

func (erpImageLegacySKURepo) LegacyObjectKeyForSKU(_ context.Context, id int64, sku string) (string, error) {
	if id != 71 || sku != "CGG001" {
		return "", fmt.Errorf("legacy object not bound to SKU")
	}
	return "tasks/archived-task/assets/old.png", nil
}

func TestERPProductImagesRenewBoundLegacyObjectWithoutTaskPrefixGuess(t *testing.T) {
	signer := NewERPImageProxySigner(ERPImageProxyConfig{PublicBaseURL: "https://image.test", SigningSecret: "primary"})
	now := time.Now()
	signer.now = func() time.Time { return now.Add(-350 * 24 * time.Hour) }
	key := "tasks/archived-task/assets/old.png"
	old := *signer.BuildLegacyImageURL(71, key)
	signer.now = func() time.Time { return now }
	engine := NewERPProductImages(erpImageLegacySKURepo{}, nil, signer, nil, nil)
	engine.oss = &OSSDirectService{}
	target := repo.ERPProductImageTarget{TaskNo: "RW-new", SingleSKU: false}
	newURL, err := engine.renewLegacyURL(context.Background(), "CGG001", old, target)
	if err != nil || newURL == "" || newURL == old {
		t.Fatalf("explicit legacy binding did not renew: %v", err)
	}
	if _, err := engine.renewLegacyURL(context.Background(), "CGG002", old, target); err == nil {
		t.Fatal("another SKU was allowed to renew the legacy object")
	}
}
