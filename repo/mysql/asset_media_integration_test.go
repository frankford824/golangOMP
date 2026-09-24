package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"workflow/domain"
)

// Run only in the disposable network-isolated container created by the schema
// test script. Never infer a database from the application environment.
func TestAssetMediaMySQLIntegration(t *testing.T) {
	dsn := os.Getenv("ASSET_MEDIA_TEST_DSN")
	if dsn == "" {
		t.Skip("requires disposable asset_media_test database")
	}
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil || cfg.DBName != "asset_media_test" || cfg.Net != "unix" {
		t.Fatal("refusing non-fixture database")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r := NewAssetMediaJobRepo(&DB{db: db})
	ctx = domain.WithAssetMediaOptions(ctx, domain.AssetMediaOptions{AccessReference: &domain.AssetMediaAccessReference{ResourceKind: "external_asset", ResourceID: "ext-1", Purpose: "download", Rendition: "original"}})
	makeJob := func() *domain.AssetMediaJob {
		return &domain.AssetMediaJob{Kind: "external_original", Pool: "nas", ResourceID: "ext-1", SourceVersion: "version-1", Recipe: domain.AssetMediaRecipe, Payload: json.RawMessage(`{"external_id":1}`), Priority: 10}
	}
	first, requestA, err := r.Enqueue(ctx, makeJob(), 101)
	if err != nil {
		t.Fatal(err)
	}
	second, requestB, err := r.Enqueue(ctx, makeJob(), 102)
	if err != nil || first.ID != second.ID || requestA == requestB {
		t.Fatalf("dedupe/subscription failed: %v", err)
	}
	access, err := r.GetRequestAccess(ctx, first.ID, 101)
	if err != nil || access == nil || access.ExpectedSourceVersion != "version-1" {
		t.Fatalf("access context not bound: %+v %v", access, err)
	}
	var mu sync.Mutex
	var claims []*domain.AssetMediaJob
	var wg sync.WaitGroup
	for _, worker := range []string{"one", "two"} {
		wg.Add(1)
		go func(worker string) {
			defer wg.Done()
			jobs, e := r.Claim(ctx, "nas", worker, 1)
			if e != nil {
				t.Errorf("claim failed: %v", e)
				return
			}
			mu.Lock()
			claims = append(claims, jobs...)
			mu.Unlock()
		}(worker)
	}
	wg.Wait()
	if len(claims) != 1 {
		t.Fatalf("claims=%d, expected one writer", len(claims))
	}
	lease := claims[0]
	if err = r.Heartbeat(ctx, lease.ID, lease.LeaseOwner, lease.LeaseEpoch, "hash", 1000, 1000); err != nil {
		t.Fatal(err)
	}
	if err = r.Heartbeat(ctx, lease.ID, lease.LeaseOwner, lease.LeaseEpoch, "upload", 100, 200); err != nil {
		t.Fatal(err)
	}
	progress, err := r.Get(ctx, lease.ID)
	if err != nil || progress.ProcessedBytes != 100 || progress.TotalBytes != 200 {
		t.Fatalf("phase progress not reset: %+v %v", progress, err)
	}
	if err = r.CancelRequest(ctx, requestA, 101); err != nil {
		t.Fatal(err)
	}
	owned, err := r.HasRequest(ctx, lease.ID, 102)
	if err != nil || !owned {
		t.Fatal("one user's cancellation removed shared request")
	}
	if _, err = db.ExecContext(ctx, "UPDATE asset_media_jobs SET lease_expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE job_id=?", lease.ID); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := r.Claim(ctx, "nas", "replacement", 1)
	if err != nil || len(reclaimed) != 1 || reclaimed[0].LeaseEpoch <= lease.LeaseEpoch {
		t.Fatalf("lease recovery failed: %v", err)
	}
	if err = r.Finish(ctx, lease.ID, lease.LeaseOwner, lease.LeaseEpoch, "succeeded", json.RawMessage(`{}`), "", false); err == nil {
		t.Fatal("accepted late result from expired worker")
	}
	newLease := reclaimed[0]
	if err = r.SaveCheckpoint(ctx, newLease.ID, newLease.LeaseOwner, newLease.LeaseEpoch, json.RawMessage(`{"uploads":{}}`)); err != nil {
		t.Fatal(err)
	}
	if err = r.Finish(ctx, newLease.ID, newLease.LeaseOwner, newLease.LeaseEpoch, "succeeded", json.RawMessage(`{}`), "", false); err != nil {
		t.Fatal(err)
	}
	t.Run("both scan shards required before absence reconciliation", func(t *testing.T) {
		ext := &externalAssetRepo{db: &DB{db: db}}
		a := domain.ExternalMediaScan{ScanID: uuid.NewString(), AgentID: "shard-0", AgentEpoch: "epoch-0", RootIdentity: "root", OriginRoot: "/p3", ShardIndex: 0, ShardCount: 2, Parts: 1, Files: 1}
		b := a
		b.ScanID = uuid.NewString()
		b.AgentID = "shard-1"
		b.AgentEpoch = "epoch-1"
		b.ShardIndex = 1
		if err := ext.StartMediaScan(ctx, a); err != nil {
			t.Fatal(err)
		}
		if err := ext.StartMediaScan(ctx, b); err != nil {
			t.Fatal(err)
		}
		storedA, _, err := ext.GetMediaScan(ctx, a.ScanID)
		if err != nil {
			t.Fatal(err)
		}
		storedB, _, err := ext.GetMediaScan(ctx, b.ScanID)
		if err != nil {
			t.Fatal(err)
		}
		if storedA.RootGeneration == "" || storedA.RootGeneration != storedB.RootGeneration {
			t.Fatal("shards not paired into one server-assigned round")
		}
		if err := ext.FinishMediaScan(ctx, a); err != nil {
			t.Fatal(err)
		}
		var status string
		if err := db.QueryRowContext(ctx, "SELECT status FROM external_asset_sync_runs WHERE scan_id=?", a.ScanID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "validated" {
			t.Fatalf("missing shard incorrectly completed absence reconciliation: %s", status)
		}
		if err := ext.FinishMediaScan(ctx, b); err != nil {
			t.Fatal(err)
		}
		var completed int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM external_asset_sync_runs WHERE root_generation=? AND run_type='nas_snapshot' AND status='succeeded'", storedA.RootGeneration).Scan(&completed); err != nil {
			t.Fatal(err)
		}
		if completed != 2 {
			t.Fatalf("completed shards=%d", completed)
		}
		a.ScanID = uuid.NewString()
		if err := ext.StartMediaScan(ctx, a); err != nil {
			t.Fatal(err)
		}
		next, _, err := ext.GetMediaScan(ctx, a.ScanID)
		if err != nil {
			t.Fatal(err)
		}
		if next.RootGeneration == storedA.RootGeneration {
			t.Fatal("next scan reused completed generation")
		}
	})
}
