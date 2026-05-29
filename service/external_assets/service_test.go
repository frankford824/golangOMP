package externalassets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func TestAListClientList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/fs/list" {
			t.Fatalf("path=%s, want /api/fs/list", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "token-1" {
			t.Fatalf("Authorization=%q, want token-1", got)
		}
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req["path"] != "/p3" {
			t.Fatalf("path payload=%v, want /p3", req["path"])
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data": map[string]interface{}{
				"total": 2,
				"content": []map[string]interface{}{
					{"name": "folder", "is_dir": true, "size": 0, "type": 1},
					{"name": "image.jpg", "is_dir": false, "size": 123, "type": 5},
				},
			},
		})
	}))
	defer server.Close()

	client := NewAListClient(server.URL, "token-1", time.Second)
	got, err := client.List(context.Background(), "/p3", 1, 50)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got.Total != 2 || len(got.Content) != 2 {
		t.Fatalf("List() = total %d len %d, want total 2 len 2", got.Total, len(got.Content))
	}
	if got.Content[0].Parent != "/p3" || !got.Content[0].IsDir {
		t.Fatalf("first item = %+v, want p3 dir", got.Content[0])
	}
}

func TestSyncFullIndexWalksMountAndFiltersSystemFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		content := []map[string]interface{}{}
		switch req["path"] {
		case "/p3":
			content = []map[string]interface{}{
				{"name": "keep", "is_dir": true, "size": 0},
				{"name": "#recycle", "is_dir": true, "size": 0},
				{"name": "root.jpg", "is_dir": false, "size": 11},
			}
		case "/p3/keep":
			content = []map[string]interface{}{
				{"name": "design.psd", "is_dir": false, "size": 22},
				{"name": "design.psd@SynoEAStream", "is_dir": false, "size": 33},
				{"name": "@eaDir", "is_dir": true, "size": 0},
			}
		default:
			t.Fatalf("unexpected list path %v", req["path"])
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data": map[string]interface{}{
				"total":   len(content),
				"content": content,
			},
		})
	}))
	defer server.Close()

	repo := &externalAssetRepoStub{}
	svc := NewService(repo, Config{
		Enabled:             true,
		AListBaseURL:        server.URL,
		AListToken:          "token",
		Mounts:              ParseMounts("/p3:nas_local"),
		FullSyncEnabled:     true,
		FullSyncPageSize:    100,
		FullSyncMaxDepth:    8,
		FullSyncMaxFiles:    100,
		FullSyncMaxDirs:     100,
		LinkRefreshInterval: time.Hour,
	}, nil)

	result, err := svc.SyncFullIndex(context.Background())
	if err != nil {
		t.Fatalf("SyncFullIndex() error = %v", err)
	}
	if result.ScannedCount != 2 || result.UpsertedCount != 2 {
		t.Fatalf("SyncFullIndex() counts = scanned %d upserted %d, want 2/2", result.ScannedCount, result.UpsertedCount)
	}
	names := make([]string, 0, len(repo.upserts))
	for _, item := range repo.upserts {
		names = append(names, item.FileName)
		if strings.Contains(item.OriginPath, "#recycle") || strings.Contains(item.OriginPath, "@eaDir") || strings.Contains(item.FileName, "@Syno") {
			t.Fatalf("system file was indexed: %+v", item)
		}
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "design.psd,root.jpg" {
		t.Fatalf("indexed names=%v, want design.psd and root.jpg", names)
	}
	if repo.missingMount != "/p3" {
		t.Fatalf("MarkMountMissingBefore mount=%q, want /p3", repo.missingMount)
	}
	if len(repo.finishedRuns) != 1 || repo.finishedRuns[0].status != domain.ExternalAssetSyncRunStatusCompleted {
		t.Fatalf("finished runs=%+v, want one completed run", repo.finishedRuns)
	}
}

type externalAssetRepoStub struct {
	upserts      []domain.ExternalAssetUpsert
	nextRunID    int64
	finishedRuns []struct {
		id       int64
		status   string
		scanned  int
		upserted int
		message  string
	}
	missingMount string
}

func (r *externalAssetRepoStub) Search(context.Context, domain.ExternalAssetSearchQuery) ([]*domain.ExternalAssetRecord, int64, error) {
	return nil, 0, nil
}

func (r *externalAssetRepoStub) Upsert(_ context.Context, item domain.ExternalAssetUpsert) (*domain.ExternalAssetRecord, error) {
	r.upserts = append(r.upserts, item)
	return &domain.ExternalAssetRecord{ID: int64(len(r.upserts)), FileName: item.FileName}, nil
}

func (r *externalAssetRepoStub) GetByID(context.Context, int64) (*domain.ExternalAssetRecord, error) {
	return nil, nil
}

func (r *externalAssetRepoStub) CreateSyncRun(_ context.Context, run *domain.ExternalAssetSyncRun) (int64, error) {
	r.nextRunID++
	return r.nextRunID, nil
}

func (r *externalAssetRepoStub) FinishSyncRun(_ context.Context, id int64, status string, scannedCount, upsertedCount int, errorMessage string) error {
	r.finishedRuns = append(r.finishedRuns, struct {
		id       int64
		status   string
		scanned  int
		upserted int
		message  string
	}{id: id, status: status, scanned: scannedCount, upserted: upsertedCount, message: errorMessage})
	return nil
}

func (r *externalAssetRepoStub) MarkMountMissingBefore(_ context.Context, mountPath string, _ time.Time) error {
	r.missingMount = mountPath
	return nil
}

func (r *externalAssetRepoStub) UpdateDirectURL(context.Context, int64, string, *time.Time, string) error {
	return nil
}

func (r *externalAssetRepoStub) MarkOSSPreparePending(context.Context, int64) error {
	return nil
}

func (r *externalAssetRepoStub) MarkPreviewPreparePending(context.Context, int64) error {
	return nil
}

func (r *externalAssetRepoStub) ListDirectURLRefreshCandidates(context.Context, int, time.Time) ([]*domain.ExternalAssetRecord, error) {
	return nil, nil
}

func (r *externalAssetRepoStub) ListPendingOSS(context.Context, int) ([]*domain.ExternalAssetRecord, error) {
	return nil, nil
}

func (r *externalAssetRepoStub) ListPendingPreview(context.Context, int) ([]*domain.ExternalAssetRecord, error) {
	return nil, nil
}

func (r *externalAssetRepoStub) MarkOSSReady(context.Context, int64, string) error {
	return nil
}

func (r *externalAssetRepoStub) MarkPreviewReady(context.Context, int64, string) error {
	return nil
}

func (r *externalAssetRepoStub) MarkPrepareFailed(context.Context, int64, string, string) error {
	return nil
}
