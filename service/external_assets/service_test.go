package externalassets

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
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

func TestBFFSearchPrefersParentAndNameWhenFullPathLosesLeadingSpace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" {
			t.Fatalf("path=%s, want /api/search", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"parent":    "/quark/A00001模拟/A-小夜灯",
					"name":      " id   tb701979542216-大六班.png",
					"is_dir":    false,
					"size":      126871,
					"full_path": "/quark/A00001模拟/A-小夜灯/id   tb701979542216-大六班.png",
				},
			},
		})
	}))
	defer server.Close()

	client := NewBFFClient(server.URL, "", time.Second)
	got, err := client.Search(context.Background(), "/quark", "png", 1, 20)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(got.Content) != 1 {
		t.Fatalf("Search() len=%d, want 1", len(got.Content))
	}
	item := got.Content[0]
	if item.Name != " id   tb701979542216-大六班.png" {
		t.Fatalf("name=%q, want leading space preserved", item.Name)
	}
	if origin := joinAListPath(item.Parent, item.Name); origin != "/quark/A00001模拟/A-小夜灯/ id   tb701979542216-大六班.png" {
		t.Fatalf("origin=%q, want leading space path", origin)
	}
}

func TestBFFFetchAvailableChecksPathWithoutFollowingBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/fetch" {
			t.Fatalf("path=%s, want /api/fetch", r.URL.Path)
		}
		if got := r.Header.Get("Range"); got != "bytes=0-0" {
			t.Fatalf("Range=%q, want bytes=0-0", got)
		}
		switch r.URL.Query().Get("path") {
		case "/p3/exists.jpg":
			w.WriteHeader(http.StatusOK)
		case "/p3/missing.jpg":
			http.Error(w, "file not found", http.StatusNotFound)
		default:
			t.Fatalf("unexpected path query %q", r.URL.Query().Get("path"))
		}
	}))
	defer server.Close()

	client := NewBFFClient(server.URL, "", time.Second)
	ok, err := client.FetchAvailable(context.Background(), "/p3/exists.jpg")
	if err != nil || !ok {
		t.Fatalf("FetchAvailable exists = %v, %v; want true nil", ok, err)
	}
	ok, err = client.FetchAvailable(context.Background(), "/p3/missing.jpg")
	if err != nil || ok {
		t.Fatalf("FetchAvailable missing = %v, %v; want false nil", ok, err)
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
				{"name": "Thumbs.db", "is_dir": false, "size": 12},
				{"name": "desktop.ini", "is_dir": false, "size": 13},
			}
		case "/p3/keep":
			content = []map[string]interface{}{
				{"name": "design.psd", "is_dir": false, "size": 22},
				{"name": "design.psd@SynoEAStream", "is_dir": false, "size": 33},
				{"name": ".DS_Store", "is_dir": false, "size": 44},
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
		if strings.Contains(item.OriginPath, "#recycle") || strings.Contains(item.OriginPath, "@eaDir") || strings.Contains(item.FileName, "@Syno") || isExternalSystemNoiseFile(item.FileName) {
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

func TestSyncFullIndexRetriesTransientListFailure(t *testing.T) {
	flakyAttempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		listPath, _ := req["path"].(string)
		if listPath == "/p3/flaky" {
			flakyAttempts++
			if flakyAttempts == 1 {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code":    500,
					"message": "context deadline exceeded while awaiting headers",
				})
				return
			}
		}
		content := []map[string]interface{}{}
		switch listPath {
		case "/p3":
			content = []map[string]interface{}{
				{"name": "flaky", "is_dir": true, "size": 0},
			}
		case "/p3/flaky":
			content = []map[string]interface{}{
				{"name": "ok.jpg", "is_dir": false, "size": 22},
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
	if flakyAttempts != 2 {
		t.Fatalf("flaky attempts = %d, want 2", flakyAttempts)
	}
	if result.ScannedCount != 1 || result.UpsertedCount != 1 {
		t.Fatalf("SyncFullIndex() counts = scanned %d upserted %d, want 1/1", result.ScannedCount, result.UpsertedCount)
	}
	if len(result.Mounts) != 1 || result.Mounts[0].Status != domain.ExternalAssetSyncRunStatusCompleted {
		t.Fatalf("mounts = %+v, want completed mount", result.Mounts)
	}
	if repo.missingMount != "/p3" {
		t.Fatalf("MarkMountMissingBefore mount=%q, want /p3 after retry success", repo.missingMount)
	}
}

func TestSyncFullIndexSkipsBrokenSubdirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req["path"] == "/quark/bad" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    500,
				"message": "failed get objs: failed get dir: object not found",
			})
			return
		}
		content := []map[string]interface{}{}
		switch req["path"] {
		case "/quark":
			content = []map[string]interface{}{
				{"name": "bad", "is_dir": true, "size": 0},
				{"name": "good", "is_dir": true, "size": 0},
				{"name": "root.png", "is_dir": false, "size": 11},
			}
		case "/quark/good":
			content = []map[string]interface{}{
				{"name": "design.png", "is_dir": false, "size": 22},
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
		Mounts:              ParseMounts("/quark:netdisk"),
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
	if len(result.Mounts) != 1 || result.Mounts[0].Status != domain.ExternalAssetSyncRunStatusPartial {
		t.Fatalf("mounts = %+v, want one partial mount", result.Mounts)
	}
	if !strings.Contains(result.Mounts[0].ErrorMessage, "/quark/bad") {
		t.Fatalf("mount error = %q, want skipped bad dir", result.Mounts[0].ErrorMessage)
	}
	if repo.missingMount != "" {
		t.Fatalf("MarkMountMissingBefore mount=%q, want no missing mark for partial scan", repo.missingMount)
	}
	if len(repo.finishedRuns) != 1 || repo.finishedRuns[0].status != domain.ExternalAssetSyncRunStatusPartial {
		t.Fatalf("finished runs=%+v, want one partial run", repo.finishedRuns)
	}
}

func TestSyncFullIndexHonorsMountFilter(t *testing.T) {
	visited := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		listPath, _ := req["path"].(string)
		visited = append(visited, listPath)
		if listPath != "/p3" {
			t.Fatalf("unexpected list path %q, want only /p3", listPath)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data": map[string]interface{}{
				"total": 1,
				"content": []map[string]interface{}{
					{"name": "root.tif", "is_dir": false, "size": 11},
				},
			},
		})
	}))
	defer server.Close()

	repo := &externalAssetRepoStub{}
	svc := NewService(repo, Config{
		Enabled:             true,
		AListBaseURL:        server.URL,
		AListToken:          "token",
		Mounts:              ParseMounts("/quark:netdisk,/p3:nas_local"),
		FullSyncEnabled:     true,
		FullSyncMounts:      ParseMountPaths("/p3"),
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
	if result.ScannedCount != 1 || result.UpsertedCount != 1 {
		t.Fatalf("SyncFullIndex() counts = scanned %d upserted %d, want 1/1", result.ScannedCount, result.UpsertedCount)
	}
	if len(result.Mounts) != 1 || result.Mounts[0].MountPath != "/p3" {
		t.Fatalf("mounts = %+v, want only /p3", result.Mounts)
	}
	if strings.Join(visited, ",") != "/p3" {
		t.Fatalf("visited paths = %v, want only /p3", visited)
	}
	if len(repo.upserts) != 1 || repo.upserts[0].MountPath != "/p3" {
		t.Fatalf("upserts = %+v, want one /p3 row", repo.upserts)
	}
}

func TestSyncFullIndexHonorsRootFilter(t *testing.T) {
	visited := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		listPath, _ := req["path"].(string)
		visited = append(visited, listPath)
		var content []map[string]interface{}
		switch listPath {
		case "/quark/我的备份/来自：ASUS Administrator 电脑备份/海报":
			content = []map[string]interface{}{
				{"name": "2026", "is_dir": true, "size": 0},
				{"name": "poster.jpg", "is_dir": false, "size": 11},
			}
		case "/quark/我的备份/来自：ASUS Administrator 电脑备份/海报/2026":
			content = []map[string]interface{}{
				{"name": "spring.png", "is_dir": false, "size": 22},
			}
		default:
			t.Fatalf("unexpected list path %q", listPath)
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
	root := "/quark/我的备份/来自：ASUS Administrator 电脑备份/海报"
	svc := NewService(repo, Config{
		Enabled:             true,
		AListBaseURL:        server.URL,
		AListToken:          "token",
		Mounts:              ParseMounts("/quark:netdisk,/p3:nas_local"),
		FullSyncEnabled:     true,
		FullSyncMounts:      ParseMountPaths("/quark"),
		FullSyncRoots:       ParseMountPaths(root),
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
	if strings.Join(visited, ",") != root+","+root+"/2026" {
		t.Fatalf("visited paths = %v, want only configured root and child", visited)
	}
	if repo.missingMount != "" {
		t.Fatalf("MarkMountMissingBefore mount=%q, want root-scoped missing", repo.missingMount)
	}
	if len(repo.missingPrefixes) != 1 || repo.missingPrefixes[0].MountPath != "/quark" || repo.missingPrefixes[0].OriginPath != root {
		t.Fatalf("missing prefixes = %+v, want /quark root prefix", repo.missingPrefixes)
	}
	for _, upsert := range repo.upserts {
		if upsert.MountPath != "/quark" || !strings.HasPrefix(upsert.OriginPath, root+"/") {
			t.Fatalf("upsert = %+v, want /quark record under configured root", upsert)
		}
	}
}

func TestNetdiskBrowserURLsDoNotExposeBFFProxyWhenDirectURLMissing(t *testing.T) {
	svc := NewService(&externalAssetRepoStub{}, Config{
		Enabled:           true,
		BFFBaseURL:        "http://internal-bff",
		BFFBrowserBaseURL: "http://browser-bff",
		Mounts:            ParseMounts("/quark:netdisk"),
	}, nil)
	row := &domain.ExternalAssetRecord{
		Kind:       domain.ExternalAssetKindNetdisk,
		MountPath:  "/quark",
		OriginPath: "/quark/a/b.jpg",
		FileName:   "b.jpg",
		FileExt:    ".jpg",
		MimeType:   "image/jpeg",
	}

	if previewURL := svc.BrowserPreviewURL(row); previewURL != "" {
		t.Fatalf("preview URL = %q, want empty URL until OSS/raw preview is ready", previewURL)
	}
	if downloadURL := svc.BrowserDownloadURL(row); downloadURL != "" {
		t.Fatalf("download URL = %q, want empty URL until OSS/raw download is ready", downloadURL)
	}
}

func TestNetdiskBrowserURLsPreferPublicRawURL(t *testing.T) {
	svc := NewService(&externalAssetRepoStub{}, Config{
		Enabled:           true,
		BFFBaseURL:        "http://internal-bff",
		BFFBrowserBaseURL: "http://browser-bff",
		Mounts:            ParseMounts("/quark:netdisk"),
	}, nil)
	row := &domain.ExternalAssetRecord{
		Kind:       domain.ExternalAssetKindNetdisk,
		MountPath:  "/quark",
		OriginPath: "/quark/a/b.jpg",
		FileName:   "b.jpg",
		FileExt:    ".jpg",
		MimeType:   "image/jpeg",
		RawURL:     "https://cdn.example.com/b.jpg",
	}

	if got := svc.BrowserPreviewURL(row); got != row.RawURL {
		t.Fatalf("preview URL = %q, want raw URL", got)
	}
	if got := svc.BrowserDownloadURL(row); got != row.RawURL {
		t.Fatalf("download URL = %q, want raw URL", got)
	}
}

func TestNetdiskDownloadQueuesOSSWhenOnlyInternalURLExists(t *testing.T) {
	checkedAt := time.Now().UTC()
	repo := &externalAssetRepoStub{getRow: &domain.ExternalAssetRecord{
		ID:                42,
		Kind:              domain.ExternalAssetKindNetdisk,
		MountPath:         "/quark",
		OriginPath:        "/quark/poster.psd",
		FileName:          "poster.psd",
		MimeType:          "image/vnd.adobe.photoshop",
		Status:            domain.ExternalAssetStatusIndexed,
		RawURL:            "http://172.21.0.1:5244/p/quark/poster.psd",
		LastLinkCheckedAt: &checkedAt,
	}}
	svc := NewService(repo, Config{
		Enabled: true,
		Mounts:  ParseMounts("/quark:netdisk"),
	}, nil)

	info, appErr := svc.DownloadInfo(context.Background(), 42)
	if appErr != nil {
		t.Fatalf("DownloadInfo() error = %+v", appErr)
	}
	if info == nil || info.DownloadURL != nil || !strings.Contains(info.AccessHint, "prepare_required") {
		t.Fatalf("DownloadInfo() = %+v, want queued OSS preparation", info)
	}
	if len(repo.ossPendingIDs) != 1 || repo.ossPendingIDs[0] != 42 {
		t.Fatalf("OSS pending ids = %+v, want [42]", repo.ossPendingIDs)
	}
}

func TestNetdiskDownloadUsesReadyOSSOriginal(t *testing.T) {
	ossDirect := baseservice.NewOSSDirectService(baseservice.OSSDirectConfig{
		Enabled:         true,
		Endpoint:        "oss-cn-hangzhou.aliyuncs.com",
		PublicEndpoint:  "oss-cn-hangzhou.aliyuncs.com",
		Bucket:          "test-bucket",
		AccessKeyID:     "test-key",
		AccessKeySecret: "test-secret",
		PresignExpiry:   15 * time.Minute,
	})
	repo := &externalAssetRepoStub{getRow: &domain.ExternalAssetRecord{
		ID:             42,
		Kind:           domain.ExternalAssetKindNetdisk,
		MountPath:      "/quark",
		OriginPath:     "/quark/poster.psd",
		FileName:       "poster.psd",
		MimeType:       "image/vnd.adobe.photoshop",
		Status:         domain.ExternalAssetStatusIndexed,
		OSSOriginalKey: "external-assets/alist/original/quark/hash/poster.psd",
		OSSSyncStatus:  domain.ExternalAssetOSSStatusReady,
	}}
	svc := NewService(repo, Config{Enabled: true, Mounts: ParseMounts("/quark:netdisk")}, ossDirect)

	info, appErr := svc.DownloadInfo(context.Background(), 42)
	if appErr != nil {
		t.Fatalf("DownloadInfo() error = %+v", appErr)
	}
	if info == nil || info.DownloadURL == nil || !strings.Contains(*info.DownloadURL, repo.getRow.OSSOriginalKey) {
		t.Fatalf("DownloadInfo() = %+v, want presigned OSS original", info)
	}
	if info.AccessHint != "external_original_oss" {
		t.Fatalf("AccessHint = %q, want external_original_oss", info.AccessHint)
	}
}

func TestPrepareMountPathsStayInsideConfiguredMounts(t *testing.T) {
	svc := NewService(&externalAssetRepoStub{}, Config{
		Enabled:       true,
		Mounts:        ParseMounts("/quark:netdisk,/p3:nas_local"),
		PrepareMounts: ParseMountPaths("/p3,/missing"),
	}, nil)
	if got := strings.Join(svc.PrepareMountPaths(), ","); got != "/p3" {
		t.Fatalf("PrepareMountPaths() = %q, want /p3", got)
	}
}

func TestNetdiskSourceReadsAreSerializedAcrossPrepareQueues(t *testing.T) {
	var fetchCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("proxy") == "0" {
			w.Header().Set("Location", "http://172.21.0.1:5244/p/quark/poster.psd")
			w.WriteHeader(http.StatusFound)
			return
		}
		fetchCalls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("source"))
	}))
	defer server.Close()

	svc := NewService(&externalAssetRepoStub{}, Config{
		Enabled:    true,
		BFFBaseURL: server.URL,
		Mounts:     ParseMounts("/quark:netdisk"),
	}, nil)
	row := &domain.ExternalAssetRecord{
		Kind:       domain.ExternalAssetKindNetdisk,
		MountPath:  "/quark",
		OriginPath: "/quark/poster.psd",
		FileName:   "poster.psd",
	}

	first, err := svc.openSourceForUpload(context.Background(), row)
	if err != nil {
		t.Fatalf("first openSourceForUpload() error = %v", err)
	}
	secondReady := make(chan io.ReadCloser, 1)
	secondErr := make(chan error, 1)
	go func() {
		second, openErr := svc.openSourceForUpload(context.Background(), row)
		if openErr != nil {
			secondErr <- openErr
			return
		}
		secondReady <- second
	}()

	time.Sleep(50 * time.Millisecond)
	if got := fetchCalls.Load(); got != 1 {
		t.Fatalf("fetch calls while first source is open = %d, want 1", got)
	}
	_ = first.Close()
	select {
	case err := <-secondErr:
		t.Fatalf("second openSourceForUpload() error = %v", err)
	case second := <-secondReady:
		_ = second.Close()
	case <-time.After(time.Second):
		t.Fatal("second source did not proceed after the first source closed")
	}
	if got := fetchCalls.Load(); got != 2 {
		t.Fatalf("fetch calls after release = %d, want 2", got)
	}
}

func TestNASLocalBrowserPreviewUsesReadyOriginalOSS(t *testing.T) {
	ossDirect := baseservice.NewOSSDirectService(baseservice.OSSDirectConfig{
		Enabled:         true,
		Endpoint:        "oss-cn-hangzhou.aliyuncs.com",
		PublicEndpoint:  "oss-cn-hangzhou.aliyuncs.com",
		Bucket:          "test-bucket",
		AccessKeyID:     "test-key",
		AccessKeySecret: "test-secret",
		PresignExpiry:   15 * time.Minute,
	})
	svc := NewService(&externalAssetRepoStub{}, Config{
		Enabled: true,
		Mounts:  ParseMounts("/p3:nas_local"),
	}, ossDirect)
	row := &domain.ExternalAssetRecord{
		Kind:           domain.ExternalAssetKindNASLocal,
		MountPath:      "/p3",
		OriginPath:     "/p3/a/b.jpg",
		FileName:       "b.jpg",
		FileExt:        ".jpg",
		MimeType:       "image/jpeg",
		OSSOriginalKey: "external-assets/alist/original/p3/abc/b.jpg",
		OSSSyncStatus:  domain.ExternalAssetOSSStatusReady,
	}

	previewURL := svc.BrowserPreviewURL(row)
	if previewURL == "" {
		t.Fatal("preview URL is empty, want ready original OSS preview URL")
	}
	if !strings.Contains(previewURL, row.OSSOriginalKey) || !strings.Contains(previewURL, "inline") {
		t.Fatalf("preview URL = %q, want original OSS inline URL", previewURL)
	}

	downloadURL := svc.BrowserDownloadURL(row)
	if downloadURL == "" {
		t.Fatal("download URL is empty, want ready original OSS download URL")
	}
	if !strings.Contains(downloadURL, row.OSSOriginalKey) || !strings.Contains(downloadURL, "attachment") {
		t.Fatalf("download URL = %q, want original OSS attachment URL", downloadURL)
	}
}

func TestNASLocalBrowserURLsDoNotExposeBFFProxyWhenOSSNotReady(t *testing.T) {
	svc := NewService(&externalAssetRepoStub{}, Config{
		Enabled:           true,
		BFFBaseURL:        "http://internal-bff",
		BFFBrowserBaseURL: "http://browser-bff",
		Mounts:            ParseMounts("/p3:nas_local"),
	}, nil)
	row := &domain.ExternalAssetRecord{
		Kind:       domain.ExternalAssetKindNASLocal,
		MountPath:  "/p3",
		OriginPath: "/p3/a/b.jpg",
		FileName:   "b.jpg",
		FileExt:    ".jpg",
		MimeType:   "image/jpeg",
	}

	if previewURL := svc.BrowserPreviewURL(row); previewURL != "" {
		t.Fatalf("preview URL = %q, want empty URL until OSS preview/original is ready", previewURL)
	}
	if downloadURL := svc.BrowserDownloadURL(row); downloadURL != "" {
		t.Fatalf("download URL = %q, want empty URL until OSS original is ready", downloadURL)
	}
}

func TestPreviewInfoQueuesDerivedPreviewInsteadOfReturningBFFURL(t *testing.T) {
	repo := &externalAssetRepoStub{getRow: &domain.ExternalAssetRecord{
		ID:            42,
		Kind:          domain.ExternalAssetKindNASLocal,
		MountPath:     "/p3",
		OriginPath:    "/p3/a/poster.tif",
		FileName:      "poster.tif",
		FileExt:       ".tif",
		MimeType:      "image/tiff",
		Status:        domain.ExternalAssetStatusIndexed,
		OSSSyncStatus: domain.ExternalAssetOSSStatusNone,
		PreviewStatus: domain.ExternalAssetPreviewStatusNone,
	}}
	svc := NewService(repo, Config{
		Enabled:           true,
		BFFBaseURL:        "http://internal-bff",
		BFFBrowserBaseURL: "http://browser-bff",
		Mounts:            ParseMounts("/p3:nas_local"),
	}, nil)

	info, appErr := svc.PreviewInfo(context.Background(), 42)
	if appErr != nil {
		t.Fatalf("PreviewInfo() error = %+v", appErr)
	}
	if info == nil || info.DownloadURL != nil || info.PreviewAvailable {
		t.Fatalf("PreviewInfo() = %+v, want prepare metadata without URL", info)
	}
	if !strings.Contains(info.AccessHint, "prepare_required") {
		t.Fatalf("AccessHint = %q, want prepare_required", info.AccessHint)
	}
	if len(repo.previewPendingIDs) != 1 || repo.previewPendingIDs[0] != 42 {
		t.Fatalf("preview pending ids = %+v, want [42]", repo.previewPendingIDs)
	}
}

func TestSearchReturnsCacheAndSchedulesKeywordRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search":
			if got := r.URL.Query().Get("q"); got != "png" {
				t.Fatalf("q=%q, want png", got)
			}
			if got := r.URL.Query().Get("mounts"); got != "/p3" {
				t.Fatalf("mounts=%q, want /p3", got)
			}
			if got := r.URL.Query().Get("match"); got != "contains" {
				t.Fatalf("match=%q, want contains", got)
			}
			if got := r.URL.Query().Get("only_files"); got != "1" {
				t.Fatalf("only_files=%q, want 1", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"parent":    "/p3/#recycle",
						"name":      "old.png",
						"is_dir":    false,
						"size":      11,
						"full_path": "/p3/#recycle/old.png",
					},
					{
						"parent":    "/p3/designs",
						"name":      "fresh.png",
						"is_dir":    false,
						"size":      22,
						"full_path": "/p3/designs/fresh.png",
					},
				},
			})
		case "/api/fetch":
			if r.URL.Query().Get("path") != "/p3/designs/fresh.png" {
				t.Fatalf("unexpected fetch path %q", r.URL.Query().Get("path"))
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("path=%s, want /api/search or /api/fetch", r.URL.Path)
		}
	}))
	defer server.Close()

	repo := &externalAssetRepoStub{}
	svc := NewService(repo, Config{
		Enabled:    true,
		BFFBaseURL: server.URL,
		Mounts:     ParseMounts("/p3:nas_local"),
	}, nil)
	var jobs []func()
	svc.keywordRefreshAsyncFn = func(fn func()) {
		jobs = append(jobs, fn)
	}

	rows, total, err := svc.Search(context.Background(), domain.ExternalAssetSearchQuery{
		Keyword: "png",
		Page:    1,
		Size:    20,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if total != 0 || len(rows) != 0 {
		t.Fatalf("Search() rows=%+v total=%d, want immediate cached empty result", rows, total)
	}
	if len(jobs) != 1 {
		t.Fatalf("scheduled jobs=%d, want 1", len(jobs))
	}
	jobs[0]()
	if len(repo.upserts) != 1 || repo.upserts[0].OriginPath != "/p3/designs/fresh.png" {
		t.Fatalf("upserts=%+v, want only non-system BFF result", repo.upserts)
	}
}

func TestKeywordRefreshSkipsAndMarksMissingNASLocalStaleSearchItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"parent":    "/p3/KT/designs",
						"name":      "HSC12654.jpg",
						"is_dir":    false,
						"size":      22,
						"full_path": "/p3/KT/designs/HSC12654.jpg",
					},
					{
						"parent":    "/p3/designs",
						"name":      "HSC12654.jpg",
						"is_dir":    false,
						"size":      22,
						"full_path": "/p3/designs/HSC12654.jpg",
					},
				},
			})
		case "/api/fetch":
			switch r.URL.Query().Get("path") {
			case "/p3/KT/designs/HSC12654.jpg":
				w.WriteHeader(http.StatusOK)
			case "/p3/designs/HSC12654.jpg":
				http.Error(w, "file not found", http.StatusNotFound)
			default:
				t.Fatalf("unexpected fetch path %q", r.URL.Query().Get("path"))
			}
		default:
			t.Fatalf("path=%s, want /api/search or /api/fetch", r.URL.Path)
		}
	}))
	defer server.Close()

	repo := &externalAssetRepoStub{}
	svc := NewService(repo, Config{
		Enabled:    true,
		BFFBaseURL: server.URL,
		Mounts:     ParseMounts("/p3:nas_local"),
	}, nil)

	if err := svc.SyncKeyword(context.Background(), "HSC12654", 20); err != nil {
		t.Fatalf("SyncKeyword() error = %v", err)
	}
	if len(repo.upserts) != 1 || repo.upserts[0].OriginPath != "/p3/KT/designs/HSC12654.jpg" {
		t.Fatalf("upserts=%+v, want only verified KT path", repo.upserts)
	}
	if len(repo.missingOrigins) != 1 || repo.missingOrigins[0] != "/p3/designs/HSC12654.jpg" {
		t.Fatalf("missing origins=%+v, want stale path marked missing", repo.missingOrigins)
	}
	if len(repo.finishedRuns) != 1 || repo.finishedRuns[0].scanned != 2 || repo.finishedRuns[0].upserted != 1 || repo.finishedRuns[0].status != domain.ExternalAssetSyncRunStatusCompleted {
		t.Fatalf("finishedRuns=%+v, want completed scanned=2 upserted=1", repo.finishedRuns)
	}
}

func TestKeywordRefreshFallsBackToAListWhenBFFOnlyReturnsSystemFiles(t *testing.T) {
	bff := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"parent":    "/p3/#recycle",
						"name":      "old.png",
						"is_dir":    false,
						"size":      11,
						"full_path": "/p3/#recycle/old.png",
					},
				},
			})
		case "/api/fetch":
			if r.URL.Query().Get("path") != "/p3/designs/fresh.png" {
				t.Fatalf("unexpected fetch path %q", r.URL.Query().Get("path"))
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("path=%s, want /api/search or /api/fetch", r.URL.Path)
		}
	}))
	defer bff.Close()
	alist := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/fs/search" {
			t.Fatalf("path=%s, want /api/fs/search", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data": map[string]interface{}{
				"content": []map[string]interface{}{
					{"parent": "/p3/designs", "name": "fresh.png", "is_dir": false, "size": 22},
				},
				"total": 1,
			},
		})
	}))
	defer alist.Close()

	repo := &externalAssetRepoStub{}
	svc := NewService(repo, Config{
		Enabled:      true,
		BFFBaseURL:   bff.URL,
		AListBaseURL: alist.URL,
		AListToken:   "token",
		Mounts:       ParseMounts("/p3:nas_local"),
	}, nil)

	if err := svc.SyncKeyword(context.Background(), "png", 20); err != nil {
		t.Fatalf("SyncKeyword() error = %v", err)
	}
	if len(repo.upserts) != 1 || repo.upserts[0].OriginPath != "/p3/designs/fresh.png" {
		t.Fatalf("SyncKeyword() upserts=%+v, want AList fallback row", repo.upserts)
	}
	if len(repo.finishedRuns) != 1 || repo.finishedRuns[0].upserted != 1 {
		t.Fatalf("finishedRuns=%+v, want one completed fallback run", repo.finishedRuns)
	}
}

func TestSearchFallsBackToCachedRowsWhenLiveRefreshFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bff unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	repo := &externalAssetRepoStub{
		searchRows: []*domain.ExternalAssetRecord{
			{ID: 7, ResourceID: "ext-7", FileName: "cached.jpg"},
		},
	}
	svc := NewService(repo, Config{
		Enabled:    true,
		BFFBaseURL: server.URL,
		Mounts:     ParseMounts("/p3:nas_local"),
	}, nil)
	scheduled := 0
	svc.keywordRefreshAsyncFn = func(fn func()) {
		scheduled++
	}

	rows, total, err := svc.Search(context.Background(), domain.ExternalAssetSearchQuery{
		Keyword: "cached",
		Page:    1,
		Size:    20,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].FileName != "cached.jpg" {
		t.Fatalf("Search() rows=%+v total=%d, want cached row", rows, total)
	}
	if len(repo.upserts) != 0 || len(repo.finishedRuns) != 0 {
		t.Fatalf("unexpected inline writes from failed refresh: upserts=%d finishedRuns=%d", len(repo.upserts), len(repo.finishedRuns))
	}
	if scheduled != 1 {
		t.Fatalf("scheduled refreshes=%d, want 1", scheduled)
	}
}

func TestSearchSchedulesBackgroundKeywordRefreshWithCooldown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": []map[string]interface{}{}})
	}))
	defer server.Close()

	repo := &externalAssetRepoStub{}
	svc := NewService(repo, Config{
		Enabled:    true,
		BFFBaseURL: server.URL,
		Mounts:     ParseMounts("/p3:nas_local"),
	}, nil)
	scheduled := 0
	svc.keywordRefreshAsyncFn = func(fn func()) {
		scheduled++
	}

	for i := 0; i < 2; i++ {
		if _, _, err := svc.Search(context.Background(), domain.ExternalAssetSearchQuery{
			Keyword: "same-keyword",
			Page:    1,
			Size:    20,
		}); err != nil {
			t.Fatalf("Search() error = %v", err)
		}
	}
	if scheduled != 1 {
		t.Fatalf("scheduled refreshes=%d, want 1", scheduled)
	}
}

func TestSearchDoesNotRefreshVeryShortASCIIKeyword(t *testing.T) {
	repo := &externalAssetRepoStub{}
	svc := NewService(repo, Config{
		Enabled: true,
		Mounts:  ParseMounts("/p3:nas_local"),
	}, nil)
	scheduled := 0
	svc.keywordRefreshAsyncFn = func(fn func()) {
		scheduled++
	}

	if _, _, err := svc.Search(context.Background(), domain.ExternalAssetSearchQuery{
		Keyword: "N",
		Page:    1,
		Size:    20,
	}); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if scheduled != 0 {
		t.Fatalf("scheduled refreshes=%d, want 0", scheduled)
	}
}

func TestFullSyncReadyWhenBFFAndAListAreConfigured(t *testing.T) {
	svc := NewService(&externalAssetRepoStub{}, Config{
		Enabled:         true,
		BFFBaseURL:      "http://bff",
		AListBaseURL:    "http://alist",
		AListToken:      "token",
		FullSyncEnabled: true,
		Mounts:          ParseMounts("/p3:nas_local"),
	}, nil)
	if !svc.FullSyncReady() {
		t.Fatal("FullSyncReady() = false, want true when BFF search and AList full sync are both configured")
	}
	if svc.LegacyIndexRefreshReady() {
		t.Fatal("LegacyIndexRefreshReady() = true, want false when BFF remains the search source")
	}
}

func TestFullSyncIntervalFallsBackToSyncInterval(t *testing.T) {
	svc := NewService(&externalAssetRepoStub{}, Config{
		Enabled:      true,
		SyncInterval: 2 * time.Hour,
		Mounts:       ParseMounts("/p3:nas_local"),
	}, nil)
	if got := svc.FullSyncInterval(); got != 2*time.Hour {
		t.Fatalf("FullSyncInterval() = %s, want SyncInterval fallback", got)
	}

	svc = NewService(&externalAssetRepoStub{}, Config{
		Enabled:          true,
		SyncInterval:     time.Hour,
		FullSyncInterval: 6 * time.Hour,
		Mounts:           ParseMounts("/p3:nas_local"),
	}, nil)
	if got := svc.FullSyncInterval(); got != 6*time.Hour {
		t.Fatalf("FullSyncInterval() = %s, want configured interval", got)
	}
}

func TestEnsureOSSRequiredPrefixesPendingUsesConfiguredMount(t *testing.T) {
	repo := &externalAssetRepoStub{}
	svc := NewService(repo, Config{
		Enabled:             true,
		Mounts:              ParseMounts("/p3:nas_local,/p2:netdisk"),
		OSSRequiredPrefixes: ParseOSSPrefixes("/p3/仓库素材区/徐凯,/p3/仓库素材区/徐凯/"),
	}, nil)

	queued, err := svc.EnsureOSSRequiredPrefixesPending(context.Background())
	if err != nil {
		t.Fatalf("EnsureOSSRequiredPrefixesPending() error = %v", err)
	}
	if queued != 1 {
		t.Fatalf("queued=%d, want 1 deduped prefix", queued)
	}
	if len(repo.ossPrefixMarks) != 1 {
		t.Fatalf("ossPrefixMarks=%+v, want one mark", repo.ossPrefixMarks)
	}
	got := repo.ossPrefixMarks[0]
	if got.MountPath != "/p3" || got.OriginPath != "/p3/仓库素材区/徐凯" {
		t.Fatalf("prefix=%+v, want /p3 徐凯", got)
	}
}

func TestProcessPendingOSSPrioritizesRequiredPrefixes(t *testing.T) {
	repo := &externalAssetRepoStub{}
	svc := NewService(repo, Config{
		Enabled:             true,
		Mounts:              ParseMounts("/p3:nas_local"),
		OSSRequiredPrefixes: ParseOSSPrefixes("/p3/仓库素材区/徐凯"),
	}, nil)

	if _, err := svc.ProcessPendingOSS(context.Background(), 20); err != nil {
		t.Fatalf("ProcessPendingOSS() error = %v", err)
	}
	if len(repo.ossPrefixMarks) != 1 || len(repo.ossPriorityReads) != 1 {
		t.Fatalf("marks=%+v priorityReads=%+v, want required prefix used", repo.ossPrefixMarks, repo.ossPriorityReads)
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
	missingMount      string
	missingPrefixes   []repo.ExternalAssetOriginPrefix
	missingOrigins    []string
	searchRows        []*domain.ExternalAssetRecord
	searchTotal       int64
	searchQueries     []domain.ExternalAssetSearchQuery
	getRow            *domain.ExternalAssetRecord
	previewPendingIDs []int64
	ossPendingIDs     []int64
	ossPrefixMarks    []repo.ExternalAssetOriginPrefix
	ossPriorityReads  []repo.ExternalAssetOriginPrefix
}

func (r *externalAssetRepoStub) Search(_ context.Context, query domain.ExternalAssetSearchQuery) ([]*domain.ExternalAssetRecord, int64, error) {
	r.searchQueries = append(r.searchQueries, query)
	if len(r.searchRows) == 0 && r.searchTotal == 0 && len(r.upserts) > 0 {
		rows := make([]*domain.ExternalAssetRecord, 0, len(r.upserts))
		for i, item := range r.upserts {
			id := int64(i + 1)
			rows = append(rows, &domain.ExternalAssetRecord{
				ID:            id,
				ResourceID:    domain.ExternalAssetResourceID(id),
				Provider:      item.Provider,
				Kind:          item.Kind,
				Driver:        item.Driver,
				MountPath:     item.MountPath,
				OriginPath:    item.OriginPath,
				ParentPath:    item.ParentPath,
				FileName:      item.FileName,
				FileExt:       item.FileExt,
				MimeType:      item.MimeType,
				FileSize:      item.FileSize,
				IsDir:         item.IsDir,
				Status:        domain.ExternalAssetStatusIndexed,
				OSSSyncStatus: domain.ExternalAssetOSSStatusNone,
				PreviewStatus: domain.ExternalAssetPreviewStatusNone,
			})
		}
		return rows, int64(len(rows)), nil
	}
	total := r.searchTotal
	if total == 0 && len(r.searchRows) > 0 {
		total = int64(len(r.searchRows))
	}
	return r.searchRows, total, nil
}

func (r *externalAssetRepoStub) Upsert(_ context.Context, item domain.ExternalAssetUpsert) (*domain.ExternalAssetRecord, error) {
	r.upserts = append(r.upserts, item)
	id := int64(len(r.upserts))
	return &domain.ExternalAssetRecord{
		ID:         id,
		ResourceID: domain.ExternalAssetResourceID(id),
		Kind:       item.Kind,
		MountPath:  item.MountPath,
		OriginPath: item.OriginPath,
		FileName:   item.FileName,
	}, nil
}

func (r *externalAssetRepoStub) GetByID(_ context.Context, id int64) (*domain.ExternalAssetRecord, error) {
	if r.getRow != nil && r.getRow.ID == id {
		row := *r.getRow
		return &row, nil
	}
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

func (r *externalAssetRepoStub) MarkOriginPrefixesMissingBefore(_ context.Context, prefixes []repo.ExternalAssetOriginPrefix, _ time.Time) error {
	r.missingPrefixes = append(r.missingPrefixes, prefixes...)
	return nil
}

func (r *externalAssetRepoStub) MarkOriginPathMissing(_ context.Context, _, _, originPath string) error {
	r.missingOrigins = append(r.missingOrigins, originPath)
	return nil
}

func (r *externalAssetRepoStub) UpdateDirectURL(context.Context, int64, string, *time.Time, string) error {
	return nil
}

func (r *externalAssetRepoStub) MarkOSSPreparePending(_ context.Context, id int64) error {
	r.ossPendingIDs = append(r.ossPendingIDs, id)
	return nil
}

func (r *externalAssetRepoStub) MarkOSSPendingByOriginPrefixes(_ context.Context, prefixes []repo.ExternalAssetOriginPrefix) (int64, error) {
	r.ossPrefixMarks = append(r.ossPrefixMarks, prefixes...)
	return int64(len(prefixes)), nil
}

func (r *externalAssetRepoStub) MarkPreviewPreparePending(_ context.Context, id int64) error {
	r.previewPendingIDs = append(r.previewPendingIDs, id)
	return nil
}

func (r *externalAssetRepoStub) ListDirectURLRefreshCandidates(context.Context, []string, int, time.Time) ([]*domain.ExternalAssetRecord, error) {
	return nil, nil
}

func (r *externalAssetRepoStub) ListPendingOSS(context.Context, []string, int) ([]*domain.ExternalAssetRecord, error) {
	return nil, nil
}

func (r *externalAssetRepoStub) ListPendingOSSPrioritized(_ context.Context, prefixes []repo.ExternalAssetOriginPrefix, _ []string, _ int) ([]*domain.ExternalAssetRecord, error) {
	r.ossPriorityReads = append(r.ossPriorityReads, prefixes...)
	return nil, nil
}

func (r *externalAssetRepoStub) ListPendingPreview(context.Context, []string, int) ([]*domain.ExternalAssetRecord, error) {
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
