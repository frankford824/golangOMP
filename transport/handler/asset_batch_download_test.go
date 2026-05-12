package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/repo"
	assetcenter "workflow/service/asset_center"
)

func TestBatchDownloadGlobalAssetsReturnsZip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uploaded := string(domain.DesignAssetUploadStatusUploaded)
	globalSvc := assetcenter.NewService(&batchDownloadRepoStub{
		rows: []*repo.TaskAssetSearchRow{
			{
				Asset: &domain.TaskAsset{
					ID:           1,
					AssetID:      int64PtrBatch(1001),
					TaskID:       7001,
					FileName:     "design.psd",
					OriginalName: strPtrBatch("原始稿.psd"),
					StorageKey:   strPtrBatch("k-1001"),
					FileSize:     int64PtrBatch(8),
					UploadStatus: &uploaded,
				},
				Task: &domain.Task{ID: 7001},
			},
		},
	}, nil, nil)
	globalSvc.SetStorageStreamOpener(&batchDownloadStreamStub{
		contentByKey: map[string]string{"k-1001": "zip-content"},
	})

	h := NewTaskAssetCenterHandler(&taskAssetCenterServiceStub{})
	h.SetGlobalAssetServices(globalSvc, nil)

	router := gin.New()
	group := router.Group("/v1/assets")
	group.POST("/batch-download", h.BatchDownloadGlobalAssets)

	req := httptest.NewRequest(http.MethodPost, "/v1/assets/batch-download", bytes.NewBufferString(`{"asset_ids":[1001]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/zip") {
		t.Fatalf("Content-Type=%q", got)
	}
	disposition := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, "attachment") || !strings.Contains(disposition, ".zip") {
		t.Fatalf("Content-Disposition=%q", disposition)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("zip body is empty")
	}

	reader, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("zip parse error: %v", err)
	}
	found := false
	for _, f := range reader.File {
		if f.Name == "task-7001/原始稿.psd" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected zip entry task-7001/原始稿.psd")
	}
}

func TestBatchDownloadGlobalAssetsInvalidBodyAndRoutePriority(t *testing.T) {
	gin.SetMode(gin.TestMode)
	globalSvc := assetcenter.NewService(&batchDownloadRepoStub{}, nil, nil)
	globalSvc.SetStorageStreamOpener(&batchDownloadStreamStub{})
	h := NewTaskAssetCenterHandler(&taskAssetCenterServiceStub{})
	h.SetGlobalAssetServices(globalSvc, nil)

	router := gin.New()
	group := router.Group("/v1/assets")
	group.POST("/batch-download", h.BatchDownloadGlobalAssets)
	group.GET("/:asset_id", func(c *gin.Context) { c.String(http.StatusTeapot, "dynamic") })

	req := httptest.NewRequest(http.MethodPost, "/v1/assets/batch-download", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Code == http.StatusTeapot {
		t.Fatal("batch-download was incorrectly matched to /:asset_id")
	}
	var payload map[string]map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid error json: %v", err)
	}
	if payload["error"]["code"] != domain.ErrCodeInvalidRequest {
		t.Fatalf("error.code=%v want=%s", payload["error"]["code"], domain.ErrCodeInvalidRequest)
	}
}

func TestBatchDownloadGlobalAssetsEmptyAssetIDsReturnsErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	globalSvc := assetcenter.NewService(&batchDownloadRepoStub{}, nil, nil)
	globalSvc.SetStorageStreamOpener(&batchDownloadStreamStub{})

	h := NewTaskAssetCenterHandler(&taskAssetCenterServiceStub{})
	h.SetGlobalAssetServices(globalSvc, nil)

	router := gin.New()
	group := router.Group("/v1/assets")
	group.POST("/batch-download", h.BatchDownloadGlobalAssets)

	req := httptest.NewRequest(http.MethodPost, "/v1/assets/batch-download", bytes.NewBufferString(`{"asset_ids":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid error json: %v", err)
	}
	if payload["error"]["code"] != domain.ErrCodeInvalidRequest {
		t.Fatalf("error.code=%v want=%s", payload["error"]["code"], domain.ErrCodeInvalidRequest)
	}
}

type batchDownloadRepoStub struct {
	rows []*repo.TaskAssetSearchRow
}

func (b *batchDownloadRepoStub) Search(context.Context, domain.AssetSearchQuery) ([]*repo.TaskAssetSearchRow, int64, error) {
	return nil, 0, nil
}
func (b *batchDownloadRepoStub) GetCurrentByAssetID(context.Context, int64) (*repo.TaskAssetSearchRow, error) {
	return nil, nil
}
func (b *batchDownloadRepoStub) ListCurrentByAssetIDs(context.Context, []int64) ([]*repo.TaskAssetSearchRow, error) {
	return b.rows, nil
}
func (b *batchDownloadRepoStub) ListVersionsByAssetID(context.Context, int64) ([]*repo.TaskAssetSearchRow, error) {
	return nil, nil
}
func (b *batchDownloadRepoStub) GetVersion(context.Context, int64, int64) (*repo.TaskAssetSearchRow, error) {
	return nil, nil
}

type batchDownloadStreamStub struct {
	contentByKey map[string]string
}

func (b *batchDownloadStreamStub) Open(_ context.Context, storageKey string) (io.ReadCloser, error) {
	content := ""
	if b.contentByKey != nil {
		content = b.contentByKey[storageKey]
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func strPtrBatch(v string) *string { return &v }
func int64PtrBatch(v int64) *int64 { return &v }
