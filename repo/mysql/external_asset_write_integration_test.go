package mysqlrepo

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func TestExternalAssetRepeatedScanWritesIntegration(t *testing.T) {
	dsn := os.Getenv("EXTERNAL_WRITE_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated MySQL DSN not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var name string
	if err = db.QueryRow("SELECT DATABASE()").Scan(&name); err != nil || !strings.HasPrefix(name, "codex_external_write_test_") {
		t.Fatal("refusing non-test database", err)
	}
	exec := func(q string) {
		t.Helper()
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	exec("SET SESSION information_schema_stats_expiry=0")
	for _, table := range []string{"external_asset_records", "external_asset_directory_index", "external_asset_source_fingerprints"} {
		exec("DELETE FROM " + table)
	}
	r := &externalAssetRepo{db: New(db)}
	scan := time.Date(2026, 11, 13, 1, 2, 3, 123000000, time.UTC)
	modified := scan.Add(-time.Hour)
	item := domain.ExternalAssetUpsert{Provider: "alist", Kind: domain.ExternalAssetKindNASLocal, Driver: "local", MountPath: "/p3", OriginPath: "/p3/test/image.jpg", ParentPath: "/p3/test", FileName: "image.jpg", FileExt: ".jpg", MimeType: "image/jpeg", FileSize: 10, ScannedAt: scan, SourceModifiedAt: &modified}
	hash := domain.ExternalAssetOriginHash(item.Provider, item.MountPath, item.OriginPath)
	upsert := func() {
		t.Helper()
		if err := r.upsertExternalAsset(context.Background(), item, hash); err != nil {
			t.Fatal(err)
		}
	}
	counter := func(table string) uint64 {
		t.Helper()
		var n uint64
		if err := db.QueryRow("SELECT AUTO_INCREMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?", table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	upsert()
	exec("UPDATE external_asset_records SET oss_original_key='kept.jpg',oss_sync_status='ready'")
	fileBefore, dirBefore := counter("external_asset_records"), counter("external_asset_directory_index")
	for i := 0; i < 30; i++ {
		upsert()
	}
	if counter("external_asset_records") != fileBefore || counter("external_asset_directory_index") != dirBefore {
		t.Fatal("unchanged scan still attempts inserts")
	}
	var key string
	if err := db.QueryRow("SELECT oss_original_key FROM external_asset_records WHERE origin_path_hash=?", hash).Scan(&key); err != nil || key != "kept.jpg" {
		t.Fatal("unchanged scan lost media", key, err)
	}
	item.ScannedAt = scan.Add(time.Hour)
	upsert()
	var seen time.Time
	if err := db.QueryRow("SELECT last_seen_at FROM external_asset_records WHERE origin_path_hash=?", hash).Scan(&seen); err != nil {
		t.Fatal(err)
	}
	item.ScannedAt = scan.Add(-time.Hour)
	upsert()
	var after time.Time
	if err := db.QueryRow("SELECT last_seen_at FROM external_asset_records WHERE origin_path_hash=?", hash).Scan(&after); err != nil || !after.Equal(seen) {
		t.Fatal("older scan regressed file watermark", after, seen, err)
	}
	var dirSeen time.Time
	if err := db.QueryRow("SELECT last_seen_at FROM external_asset_directory_index WHERE path='/p3/test'").Scan(&dirSeen); err != nil || !dirSeen.Equal(seen) {
		t.Fatal("directory watermark regressed", dirSeen, seen, err)
	}
	item.FileName = "IMAGE.jpg"
	item.ScannedAt = scan.Add(2 * time.Hour)
	upsert()
	var filename string
	if err := db.QueryRow("SELECT file_name FROM external_asset_records WHERE origin_path_hash=?", hash).Scan(&filename); err != nil || filename != "IMAGE.jpg" {
		t.Fatal("case-only edit lost", filename, err)
	}
	item.FileSize = 12
	upsert()
	var status string
	if err := db.QueryRow("SELECT oss_original_key,oss_sync_status FROM external_asset_records WHERE origin_path_hash=?", hash).Scan(&key, &status); err != nil || key != "" || status != "pending" {
		t.Fatal("source replacement did not invalidate old media", key, status, err)
	}
	// Count changes must bypass the presence-only deduplication.
	if err := upsertExternalAssetDirectory(context.Background(), db, "alist", domain.ExternalAssetKindNASLocal, "local", "/p3", "/p3/test", item.ScannedAt, 1, 1); err != nil {
		t.Fatal(err)
	}
	var descendants, direct int
	if err := db.QueryRow("SELECT descendant_file_count,direct_file_count FROM external_asset_directory_index WHERE path='/p3/test'").Scan(&descendants, &direct); err != nil || descendants != 2 || direct != 2 {
		t.Fatal("count delta lost", descendants, direct, err)
	}
	exec("UPDATE external_asset_directory_index SET status='missing' WHERE path='/p3/test'")
	if err := upsertExternalAssetDirectory(context.Background(), db, "alist", domain.ExternalAssetKindNASLocal, "LOCAL", "/p3", "/p3/test", item.ScannedAt, 0, 0); err != nil {
		t.Fatal(err)
	}
	var driver string
	if err := db.QueryRow("SELECT status,driver FROM external_asset_directory_index WHERE path='/p3/test'").Scan(&status, &driver); err != nil || status != "indexed" || driver != "LOCAL" {
		t.Fatal("directory restore or case edit lost", status, driver, err)
	}
}
