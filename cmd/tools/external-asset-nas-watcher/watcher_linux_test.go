//go:build linux

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"workflow/domain"
)

func TestFilesystemEventIDStableAndContentSensitive(t *testing.T) {
	snapshot := fileSnapshot{Size: 42, ModifiedUnixNano: 123}
	first := filesystemEventID("upsert", "/p3/仓库素材区/徐凯/A.jpg", snapshot)
	second := filesystemEventID("upsert", "/p3/仓库素材区/徐凯/A.jpg", snapshot)
	changed := filesystemEventID("upsert", "/p3/仓库素材区/徐凯/A.jpg", fileSnapshot{Size: 43, ModifiedUnixNano: 123})
	if first != second || first == changed {
		t.Fatalf("event ids first=%q second=%q changed=%q", first, second, changed)
	}
}

func TestDiffSnapshots(t *testing.T) {
	previous := map[string]fileSnapshot{"same.jpg": {Size: 1}, "changed.jpg": {Size: 1}, "deleted.jpg": {Size: 1}}
	current := map[string]fileSnapshot{"same.jpg": {Size: 1}, "changed.jpg": {Size: 2}, "new.jpg": {Size: 1}}
	upserts, deletes := diffSnapshots(current, previous)
	if len(upserts) != 2 || len(deletes) != 1 {
		t.Fatalf("upserts=%v deletes=%v", upserts, deletes)
	}
}

func TestShouldIgnoreSynologyMetadata(t *testing.T) {
	for _, rel := range []string{"A/@eaDir/thumb.jpg", "#recycle/a.jpg", "A/@SynoEAStream", ".DS_Store"} {
		if !shouldIgnoreRelative(rel) {
			t.Fatalf("should ignore %q", rel)
		}
	}
	if shouldIgnoreRelative("SKU001/main.jpg") {
		t.Fatal("business file unexpectedly ignored")
	}
}

func TestPostEventsUsesDedicatedToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/integration/external-assets/events" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if got := r.Header.Get("X-External-Asset-Event-Token"); got != "secret" {
			t.Fatalf("token=%q", got)
		}
		var batch domain.ExternalAssetFilesystemEventBatch
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil || batch.AgentID != "nas-1" || len(batch.Events) != 1 {
			t.Fatalf("batch=%+v err=%v", batch, err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	w := &nasWatcher{cfg: watcherConfig{BackendURL: server.URL, EventToken: "secret", AgentID: "nas-1"}, client: &http.Client{Timeout: time.Second}}
	event := domain.ExternalAssetFilesystemEvent{EventID: "evt", Type: domain.ExternalAssetFilesystemEventDelete, MountPath: "/p3", OriginPath: "/p3/仓库素材区/徐凯/a.jpg", ObservedAt: time.Now().UTC()}
	if err := w.postEvents(context.Background(), []domain.ExternalAssetFilesystemEvent{event}); err != nil {
		t.Fatal(err)
	}
}
