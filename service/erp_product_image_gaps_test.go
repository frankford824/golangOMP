package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

const (
	testGapSKU = "CGP001639"
	testGapPic = "https://images-erp.sursung.com/prod/erp/ItemSku/1/photo.png?name=CGP001639 商品图.png"
)

type imageGapRepoStub struct {
	erpImageTestRepo
	batches       map[string][]string
	finishedIssue string
}

func (r *imageGapRepoStub) ImageGapScanSKUs(_ context.Context, after string, _ time.Time, limit int) ([]string, error) {
	if limit != 50 {
		return nil, errors.New("incorrect bulk size")
	}
	return r.batches[after], nil
}

func (r *imageGapRepoStub) Acquire(context.Context, string, string, time.Time, time.Time) (bool, error) {
	return true, nil
}

func (r *imageGapRepoStub) Finish(_ context.Context, _, _ string, _ time.Time, issue string) error {
	r.finishedIssue = issue
	return nil
}

func (r *imageGapRepoStub) Target(context.Context, string) (repo.ERPProductImageTarget, error) {
	return repo.ERPProductImageTarget{}, nil
}

func (r *imageGapRepoStub) PendingReference(context.Context, string) (repo.ERPProductImageRequest, error) {
	return repo.ERPProductImageRequest{}, nil
}

type imageGapBatchStub func(context.Context, []string) ([]*domain.ERPProduct, error)

func (b imageGapBatchStub) BatchProductsBySKU(ctx context.Context, skus []string) ([]*domain.ERPProduct, error) {
	return b(ctx, skus)
}

func testImageGapService(r repo.ERPProductImageRepo, bridge ERPBridgeService) *ERPProductImages {
	signer := NewERPImageProxySigner(ERPImageProxyConfig{PublicBaseURL: "https://image.test", SigningSecret: "secret"})
	s := NewERPProductImages(r, nil, signer, bridge, nil)
	s.client.Transport = erpImageRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 206, Body: io.NopCloser(strings.NewReader("\x89PNG\r\n\x1a\n"))}, nil
	})
	return s
}

func TestERPImageGapScanQueuesSingleSideBeforeOrdinaryAndWraps(t *testing.T) {
	skus := []string{"CGA000001", "CGA000002", "CGA000003", "CGA000004", "CGP001639"}
	repository := &imageGapRepoStub{batches: map[string][]string{"": skus}}
	s := testImageGapService(repository, nil)
	sweep := erpImageGapSweep{}
	reader := imageGapBatchStub(func(_ context.Context, actual []string) ([]*domain.ERPProduct, error) {
		if !reflect.DeepEqual(actual, skus) {
			t.Fatalf("batch=%v", actual)
		}
		return []*domain.ERPProduct{
			{SKUID: skus[0], ImageFields: &domain.ERPProductImageFields{Pic: "a", PicBig: "b"}},
			{SKUID: skus[1], ImageFields: &domain.ERPProductImageFields{Pic: "https://images-erp.sursung.com/prod/erp/ItemSku/file.png"}},
			{SKUID: skus[2], ImageFields: &domain.ERPProductImageFields{}},
			{SKUID: skus[3], ImageFields: &domain.ERPProductImageFields{PicBig: "https://image.test/big.png"}},
			{SKUID: skus[4], ImageFields: &domain.ERPProductImageFields{Pic: testGapPic}},
			{SKUID: "CGA999999", ImageFields: &domain.ERPProductImageFields{PicBig: "irrelevant"}},
		}, nil
	})
	if err := s.discoverImageGaps(context.Background(), reader, &sweep, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sweep.pending, []string{skus[3], skus[4]}) || sweep.after != skus[4] {
		t.Fatalf("wrong single-side priority queue: %+v", sweep)
	}
	if sku, ok := sweep.takePriority(); !ok || sku != skus[3] {
		t.Fatalf("first priority = %q %v", sku, ok)
	}
	sweep.burst = erpImageGapPriorityBurst
	if _, ok := sweep.takePriority(); ok {
		t.Fatal("ordinary queue was starved by the priority burst")
	}
	if sku, ok := sweep.takePriority(); !ok || sku != skus[4] {
		t.Fatalf("second priority = %q %v", sku, ok)
	}
	if err := s.discoverImageGaps(context.Background(), reader, &sweep, zap.NewNop()); err != nil || sweep.after != "" {
		t.Fatalf("did not wrap at end of catalogue: %+v %v", sweep, err)
	}
}

func TestERPImageGapOnlyWritesBigForProvenCurrentSKUSmall(t *testing.T) {
	for _, tc := range []struct {
		name, pic, big string
		writes         int
		wantHeld       bool
	}{
		{"confirmed", testGapPic, "", 1, false},
		{"other SKU in name", testGapPic + "&filename=CGP001640.png", "", 0, true},
		{"anonymous", "https://images-erp.sursung.com/prod/erp/ItemSku/a.png", "", 0, true},
		{"filled during discovery", testGapPic, testGapPic, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &imageGapRepoStub{}
			cost := 6.45
			readback := &domain.ERPProduct{SKUID: testGapSKU, IID: "style", ProductName: "name", ShortName: "short",
				CostPrice: &cost, ImageFields: &domain.ERPProductImageFields{Pic: tc.pic, PicBig: tc.big}}
			writes := 0
			b := &erpImageTestBridge{readback: readback, onWrite: func(p domain.ERPProductUpsertPayload) {
				writes++
				raw, err := json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				biz, err := buildERPRemoteOpenWebBiz("upsert", raw)
				if err != nil {
					t.Fatal(err)
				}
				items := biz["items"].([]map[string]interface{})
				if len(items) != 1 || !reflect.DeepEqual(items[0], map[string]interface{}{"sku_id": testGapSKU, "pic_big": tc.pic}) {
					t.Fatalf("unexpected OpenWeb update: %#v", items)
				}
				readback.ImageFields.PicBig = p.PicBig
			}}
			s := testImageGapService(r, b)
			err := s.checkSKU(context.Background(), testGapSKU, true)
			if writes != tc.writes || errors.Is(err, errERPImageGapUnclear) != tc.wantHeld || (err != nil && !tc.wantHeld) {
				t.Fatalf("writes=%d error=%v issue=%q", writes, err, r.finishedIssue)
			}
			if readback.ImageFields.Pic != tc.pic {
				t.Fatal("small image was changed")
			}
		})
	}
}

func TestERPNativeImageNeedsExactFilenameEvidence(t *testing.T) {
	for _, raw := range []string{
		"https://arbitrary.example/prod/erp/ItemSku/a?name=CGP001639.png",
		"https://images-erp.sursung.com/wrong-route/a?name=CGP001639.png",
		"https://images-erp.sursung.com/prod/erp/ItemSku/a?name=CGP0016390.png",
		"https://images-erp.sursung.com/prod/erp/ItemSku/a?anything=CGP001639.png",
	} {
		u, _ := url.Parse(raw)
		if erpNativeImageNamesExactSKU(u, testGapSKU) {
			t.Fatalf("accepted weak source proof: %s", raw)
		}
	}
}

type imageGapOwnerStub struct {
	repo.ERPProductImageRepo
	asset *domain.TaskAsset
}

func (r imageGapOwnerStub) ActiveAsset(context.Context, int64) (*domain.TaskAsset, error) {
	return r.asset, nil
}

func TestERPImageGapProxyOwnershipRejectsConflictingScope(t *testing.T) {
	const version = 42
	other := "CGP007000"
	asset := &domain.TaskAsset{ID: version, TaskID: 99, ScopeSKUCode: &other}
	target := repo.ERPProductImageTarget{TaskID: 99, VersionID: version}
	r := imageGapOwnerStub{asset: asset}
	s := testImageGapService(r, nil)
	raw := "https://image.test/v1/public/erp-product-images/42?exp=123&sig=old"
	ok, err := s.confirmSmallImage(context.Background(), testGapSKU, raw, target, nil)
	if err != nil || ok {
		t.Fatalf("another SKU's active asset was accepted: %v %v", ok, err)
	}
	asset.ScopeSKUCode = nil
	ok, err = s.confirmSmallImage(context.Background(), testGapSKU, raw, target, nil)
	if err != nil || !ok {
		t.Fatalf("the authoritative SKU version was rejected: %v %v", ok, err)
	}
}
