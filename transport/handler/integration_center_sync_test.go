package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/repo"
	assetcenter "workflow/service/asset_center"
)

type integrationFinalizedSyncRepoStub struct {
	rows []repo.ProductionPackageAsset
}

func (s integrationFinalizedSyncRepoStub) ListAllFinalizedAssets(context.Context) ([]repo.ProductionPackageAsset, error) {
	return s.rows, nil
}

func (s integrationFinalizedSyncRepoStub) ListFinalizedAssetsByIDs(context.Context, []int64) ([]repo.ProductionPackageAsset, error) {
	return nil, nil
}

func TestRequestETagMatchesFinalizedManifest(t *testing.T) {
	for _, value := range []string{`"abc"`, `W/"abc"`, `"other", "abc"`, `*`} {
		if !requestETagMatches(value, "abc") {
			t.Fatalf("expected %q to match", value)
		}
	}
	for _, value := range []string{"", `"other"`, `W/"other"`} {
		if requestETagMatches(value, "abc") {
			t.Fatalf("expected %q not to match", value)
		}
	}
}

func TestFinalizedSyncManifestReturnsETagEnvelopeAnd304(t *testing.T) {
	gin.SetMode(gin.TestMode)
	row := repo.ProductionPackageAsset{
		GroupID: 1, RevisionID: 2, RevisionMode: domain.TaskAssetGroupModeSingle,
		RevisionFinalizedAt: time.Unix(1000, 0).UTC(), RevisionItemID: 3,
		TaskAssetID: 4, TaskID: 5, TaskNo: "RW-1", ScopeKind: domain.TaskAssetGroupScopeTask,
		FileName: "final.jpg", MimeType: "image/jpeg", FileSize: 10, StorageKey: "tasks/final.jpg",
		CreatedAt: time.Unix(900, 0).UTC(),
	}
	svc := assetcenter.NewService(nil, nil, nil, assetcenter.WithFinalizedAssetSync(integrationFinalizedSyncRepoStub{rows: []repo.ProductionPackageAsset{row}}, nil))
	handler := NewIntegrationCenterHandler(nil)
	handler.SetFinalizedAssetSyncService(svc)
	router := gin.New()
	router.GET("/manifest", handler.FinalizedSyncManifest)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/manifest", nil))
	if first.Code != http.StatusOK || first.Header().Get("ETag") == "" {
		t.Fatalf("first response status=%d etag=%q body=%s", first.Code, first.Header().Get("ETag"), first.Body.String())
	}
	if got := first.Header().Get("ETag"); len(got) < 4 || got[:3] != `W/"` {
		t.Fatalf("expected weak manifest ETag, got %q", got)
	}
	var envelope struct {
		Data struct {
			ManifestID string `json:"manifest_id"`
			ItemCount  int    `json:"item_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.ManifestID == "" || envelope.Data.ItemCount != 1 {
		t.Fatalf("manifest envelope = %+v", envelope)
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/manifest", nil)
	secondRequest.Header.Set("If-None-Match", first.Header().Get("ETag"))
	second := httptest.NewRecorder()
	router.ServeHTTP(second, secondRequest)
	if second.Code != http.StatusNotModified || second.Body.Len() != 0 {
		t.Fatalf("conditional response status=%d body=%q", second.Code, second.Body.String())
	}
}
