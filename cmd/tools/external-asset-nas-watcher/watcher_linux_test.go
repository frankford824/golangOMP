//go:build linux

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestShardOwnershipIsStableAndExclusive(t *testing.T) {
	shard0 := &nasWatcher{cfg: watcherConfig{ShardCount: 2, ShardIndex: 0}}
	shard1 := &nasWatcher{cfg: watcherConfig{ShardCount: 2, ShardIndex: 1}}
	for _, rel := range []string{"海报/a.jpg", "KT/a.jpg", "写真布/子目录/a.jpg", "root-file.jpg"} {
		first := shard0.ownsRelative(rel)
		second := shard1.ownsRelative(rel)
		if first == second {
			t.Fatalf("ownership for %q = shard0:%v shard1:%v; want exactly one", rel, first, second)
		}
		if shard0.ownsRelative(rel) != first || shard1.ownsRelative(rel) != second {
			t.Fatalf("ownership for %q is not stable", rel)
		}
	}
	if !shard0.ownsRelative(".") || !shard1.ownsRelative(".") {
		t.Fatal("every shard must own the shared root watch")
	}
}

func TestShardSnapshotsAreDisjointAndComplete(t *testing.T) {
	root := t.TempDir()
	files := []string{"海报/a.jpg", "KT/b.jpg", "写真布/子目录/c.jpg", "root.txt"}
	for _, rel := range files {
		filename := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(filename), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(rel), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	combined := map[string]struct{}{}
	for index := 0; index < 2; index++ {
		watcher := &nasWatcher{cfg: watcherConfig{Root: root, ShardCount: 2, ShardIndex: index}}
		snapshots, err := watcher.scanSnapshots(root)
		if err != nil {
			t.Fatal(err)
		}
		for rel := range snapshots {
			if _, exists := combined[rel]; exists {
				t.Fatalf("snapshot %q belongs to multiple shards", rel)
			}
			combined[rel] = struct{}{}
		}
	}
	if len(combined) != len(files) {
		t.Fatalf("combined snapshots = %v, want %d files", combined, len(files))
	}
}
