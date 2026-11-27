package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	drawstd "image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func erpSetFixture(t *testing.T) (*domain.ProductManagementRecord, *domain.TaskAssetGroup, map[string][]byte, []color.RGBA) {
	t.Helper()
	const sku = "CGP000606"
	var itemID, revisionID int64 = 81, 9175
	record := &domain.ProductManagementRecord{TaskID: 5026, TaskSKUItemID: &itemID, SKUCode: sku, ProductName: "退伍条幅挂布/红光荣退伍5条装"}
	revision := &domain.TaskAssetGroupRevision{
		ID: revisionID, GroupID: 8560, Status: domain.TaskAssetGroupRevisionFinalized, Mode: domain.TaskAssetGroupModeSet,
	}
	group := &domain.TaskAssetGroup{
		ID: 8560, TaskID: record.TaskID, TaskSKUItemID: &itemID, SKUCode: sku,
		ScopeKind: domain.TaskAssetGroupScopeSKU, FinalizedRevisionID: &revisionID, FinalizedRevision: revision,
	}
	stored := map[string][]byte{}
	colors := []color.RGBA{{210, 20, 20, 255}, {25, 170, 40, 255}, {32, 70, 218, 255}, {183, 31, 186, 255}, {224, 133, 23, 255}}
	for i, c := range colors {
		version := 59744 + int64(i)
		name := fmt.Sprintf("poster-%d.png", version)
		key := "tasks/test/" + name
		object := image.NewRGBA(image.Rect(0, 0, 30, 170))
		drawstd.Draw(object, object.Bounds(), image.NewUniform(c), image.Point{}, drawstd.Src)
		var data bytes.Buffer
		if err := png.Encode(&data, object); err != nil {
			t.Fatal(err)
		}
		stored[key] = data.Bytes()
		size := int64(data.Len())
		revision.Items = append(revision.Items, domain.TaskAssetGroupRevisionItem{
			ID: int64(i + 1), RevisionID: revisionID, TaskAssetID: version, SortOrder: i,
			File: &domain.TaskResourceFile{TaskAssetID: version, StorageKey: key, FileName: name, MimeType: "image/png",
				FileSize: &size, Availability: domain.TaskResourceFileAvailable},
		})
	}
	return record, group, stored, colors
}

type erpSetGroupStub struct {
	group      *domain.TaskAssetGroup
	checks     int
	conflictAt int
}

func (s *erpSetGroupStub) ListResourceGroups(_ context.Context, p domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64, error) {
	if p.TaskID != s.group.TaskID || p.SKUCode != "CGP000606" {
		return nil, 0, fmt.Errorf("unexpected lookup")
	}
	return []domain.TaskAssetGroup{*s.group}, 1, nil
}
func (*erpSetGroupStub) ListFlatResourceItems(context.Context, domain.ResourceGroupListParams) ([]domain.FlatResourceItem, int64, error) {
	return nil, 0, errors.New("a known set must never fall through to the first-available image")
}
func (s *erpSetGroupStub) LockERPProductImageSet(_ context.Context, _ repo.Tx, group domain.TaskAssetGroup) error {
	s.checks++
	if s.checks == s.conflictAt || group.SKUCode != s.group.SKUCode {
		return repo.ErrConflict
	}
	return nil
}

type erpSetAliasStub struct {
	imageGapRepoStub
	sku string
	key string
}

func (s *erpSetAliasStub) FindLegacyObject(_ context.Context, sku, key string) (int64, error) {
	s.sku = sku
	s.key = key
	return 42, nil
}
func (s *erpSetAliasStub) RequiresERPImageSet(context.Context, string) (bool, error) {
	return true, nil
}
func (s *erpSetAliasStub) LegacyObjectKeyForSKU(_ context.Context, id int64, sku string) (string, error) {
	if id != 42 || sku != s.sku {
		return "", repo.ErrNotFound
	}
	return s.key, nil
}

func erpSetMockStore(stored map[string][]byte) *OSSDirectService {
	store := NewOSSDirectService(OSSDirectConfig{
		Enabled: true, Endpoint: "oss.test", Bucket: "bucket", AccessKeyID: "key", AccessKeySecret: "secret",
	})
	store.httpClient.Transport = erpImageRoundTripper(func(req *http.Request) (*http.Response, error) {
		key := strings.TrimPrefix(req.URL.Path, "/")
		data, exists := stored[key]
		code := http.StatusOK
		switch req.Method {
		case http.MethodPut:
			var err error
			data, err = io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			stored[key] = data
		case http.MethodHead, http.MethodGet:
			if !exists {
				code = http.StatusNotFound
			}
		default:
			return nil, fmt.Errorf("unexpected OSS verb %s", req.Method)
		}
		return &http.Response{StatusCode: code, ContentLength: int64(len(data)), Body: io.NopCloser(bytes.NewReader(data))}, nil
	})
	return store
}

func TestERPSetImageSyncUsesAllFiveMembersAndOnlyImageFields(t *testing.T) {
	record, group, stored, colors := erpSetFixture(t)
	scope := &erpSetGroupStub{group: group}
	alias := &erpSetAliasStub{}
	product := &domain.ERPProduct{SKUID: record.SKUCode, IID: "常规海报", ProductName: record.ProductName,
		ImageFields: &domain.ERPProductImageFields{Pic: "https://before.example/one.png", PicBig: "https://before.example/one.png"}}
	writes := 0
	bridge := &erpImageTestBridge{readback: product, onWrite: func(p domain.ERPProductUpsertPayload) {
		writes++
		if scope.checks != 3 || alias.sku != record.SKUCode || !strings.HasPrefix(alias.key, "erp-product-images/sets/5026/8560/9175/") {
			t.Fatal("ERP write was not guarded by the exact current SKU set")
		}
		if p.Pic != p.PicBig || !strings.Contains(p.Pic, "/erp-product-images/oss/42?") {
			t.Fatalf("ERP slots do not use a single set cover")
		}
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		biz, err := buildERPRemoteOpenWebBiz("upsert", raw)
		if err != nil {
			t.Fatal(err)
		}
		item := biz["items"].([]map[string]interface{})[0]
		if len(item) != 3 || item["sku_id"] != record.SKUCode || item["pic"] != p.Pic || item["pic_big"] != p.PicBig {
			t.Fatalf("unexpected ERP fields: %#v", item)
		}
		product.ImageFields = &domain.ERPProductImageFields{Pic: p.Pic, PicBig: p.PicBig}
	}}
	images := testImageGapService(alias, bridge)
	service := NewProductManagementService(nil, nil, nil, productManagementUnitTxRunner{},
		WithProductManagementFinalizedResources(scope), WithProductManagementERPImagePublications(images),
		WithProductManagementERPImageProxy(images.signer), WithProductManagementAssetURLServices(erpSetMockStore(stored), nil)).(*productManagementService)
	if erpErr := service.syncImageRecordToERP(context.Background(), record); erpErr != nil {
		t.Fatal(erpErr.Message)
	}
	if writes != 1 {
		t.Fatalf("writes=%d", writes)
	}
	cover, err := jpeg.Decode(bytes.NewReader(stored[alias.key]))
	if err != nil {
		t.Fatal(err)
	}
	bounds := cover.Bounds()
	const cell = 2*8 + (850-2*8)*30/170
	if bounds.Dx() != 5*cell || bounds.Dy() != 850 {
		t.Fatalf("unexpected layout: %v", bounds)
	}
	for i, want := range colors {
		actual := color.RGBAModel.Convert(cover.At(i*cell+cell/2, 425)).(color.RGBA)
		if absERPTestColor(actual.R, want.R) > 12 || absERPTestColor(actual.G, want.G) > 12 || absERPTestColor(actual.B, want.B) > 12 {
			t.Fatalf("member %d displayed out of order or absent: %v", i+1, actual)
		}
	}
	margin := color.RGBAModel.Convert(cover.At(2, 425)).(color.RGBA)
	if margin.R < 245 || margin.G < 245 || margin.B < 245 {
		t.Fatal("portrait member was stretched to the full cell width")
	}
}

func TestERPSetImageSyncKeepsExistingERPImageIfSourceOrIdentityChanges(t *testing.T) {
	for _, tc := range []struct {
		name  string
		alter func(*domain.ProductManagementRecord, *domain.TaskAssetGroup, *erpSetGroupStub, map[string][]byte)
	}{
		{"foreign SKU", func(_ *domain.ProductManagementRecord, g *domain.TaskAssetGroup, _ *erpSetGroupStub, _ map[string][]byte) {
			g.SKUCode = "CGP009999"
		}},
		{"wrong SKU item", func(p *domain.ProductManagementRecord, _ *domain.TaskAssetGroup, _ *erpSetGroupStub, _ map[string][]byte) {
			id := int64(999)
			p.TaskSKUItemID = &id
		}},
		{"declared five but only four finals", func(_ *domain.ProductManagementRecord, g *domain.TaskAssetGroup, _ *erpSetGroupStub, _ map[string][]byte) {
			g.FinalizedRevision.Items = g.FinalizedRevision.Items[:4]
		}},
		{"missing source bytes", func(_ *domain.ProductManagementRecord, g *domain.TaskAssetGroup, _ *erpSetGroupStub, data map[string][]byte) {
			delete(data, g.FinalizedRevision.Items[2].File.StorageKey)
		}},
		{"reopened during image build", func(_ *domain.ProductManagementRecord, _ *domain.TaskAssetGroup, s *erpSetGroupStub, _ map[string][]byte) {
			s.conflictAt = 3
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record, group, stored, _ := erpSetFixture(t)
			scope := &erpSetGroupStub{group: group}
			tc.alter(record, group, scope, stored)
			alias := &erpSetAliasStub{}
			product := &domain.ERPProduct{SKUID: record.SKUCode, ProductName: "红光荣退伍5条装",
				ImageFields: &domain.ERPProductImageFields{Pic: "https://before.example/existing.png", PicBig: "https://before.example/existing.png"}}
			bridge := &erpImageTestBridge{readback: product, onWrite: func(domain.ERPProductUpsertPayload) { t.Fatal("blocked set must not update ERP") }}
			images := testImageGapService(alias, bridge)
			service := NewProductManagementService(nil, nil, nil, productManagementUnitTxRunner{},
				WithProductManagementFinalizedResources(scope), WithProductManagementERPImagePublications(images),
				WithProductManagementERPImageProxy(images.signer), WithProductManagementAssetURLServices(erpSetMockStore(stored), nil)).(*productManagementService)
			if err := service.syncImageRecordToERP(context.Background(), record); err == nil {
				t.Fatal("invalid set was not held")
			}
		})
	}
}

func TestERPSetImagePeriodicSyncCannotReplaceCoverWithSingleMember(t *testing.T) {
	alias := &erpSetAliasStub{sku: "CGP000606", key: "erp-product-images/sets/5026/8560/9175/checked.jpg"}
	images := testImageGapService(alias, nil)
	member := "https://image.test/v1/public/erp-product-images/59744?exp=123&sig=member"
	if err := images.preventSingleERPSetImage(context.Background(), alias.sku, domain.ERPProductImageFields{}, domain.ERPProductImageFields{Pic: member, PicBig: member}); err == nil {
		t.Fatal("a single member could replace the complete set")
	}
	url := *images.signer.BuildLegacyImageURL(42, alias.key)
	desired := domain.ERPProductImageFields{Pic: url, PicBig: url}
	if err := images.preventSingleERPSetImage(context.Background(), alias.sku, domain.ERPProductImageFields{}, desired); err == nil {
		t.Fatal("unpublished alias should require the current revision guard")
	}
	if err := images.preventSingleERPSetImage(context.Background(), alias.sku, domain.ERPProductImageFields{Pic: url}, desired); err != nil {
		t.Fatal(err)
	}
}

func absERPTestColor(a, b uint8) int {
	return max(int(a)-int(b), int(b)-int(a))
}
