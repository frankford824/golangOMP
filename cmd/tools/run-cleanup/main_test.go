package main

import (
	"bytes"
	"testing"
)

func TestUsage_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, testEnv(nil), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("usage: run-cleanup")) {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestUsage_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"unknown"}, testEnv(nil), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("usage: run-cleanup")) {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestFlagParsing_OSS365(t *testing.T) {
	opts, ok := parseSubcommand("oss-365", []string{"--limit", "50", "--dry-run", "--dsn", "user:pass@tcp(localhost:3306)/jst_erp_r3_test"}, testEnv(nil), &bytes.Buffer{})
	if !ok {
		t.Fatal("parseSubcommand ok = false")
	}
	if opts.subcommand != "oss-365" || !opts.dryRun || opts.limit != 50 || opts.dsn == "" {
		t.Fatalf("opts = %+v, want oss-365 dry-run limit=50 with dsn", opts)
	}
	if opts.reason != defaultReason || !opts.jsonOut {
		t.Fatalf("opts reason/json = %q/%t, want default/true", opts.reason, opts.jsonOut)
	}
}

func TestFlagParsing_Drafts7d(t *testing.T) {
	opts, ok := parseSubcommand("drafts-7d", []string{"--dry-run", "--dsn", "from-flag"}, testEnv(map[string]string{"MYSQL_DSN": "from-env"}), &bytes.Buffer{})
	if !ok {
		t.Fatal("parseSubcommand ok = false")
	}
	if opts.subcommand != "drafts-7d" || !opts.dryRun || opts.limit != 100 || opts.dsn != "from-flag" {
		t.Fatalf("opts = %+v, want drafts-7d dry-run default limit dsn override", opts)
	}
}

func TestFlagParsing_AutoArchive(t *testing.T) {
	opts, ok := parseSubcommand("auto-archive", []string{"--dry-run", "--limit", "200", "--cutoff-days", "30", "--dsn", "from-flag"}, testEnv(map[string]string{"MYSQL_DSN": "from-env"}), &bytes.Buffer{})
	if !ok {
		t.Fatal("parseSubcommand ok = false")
	}
	if opts.subcommand != "auto-archive" || !opts.dryRun || opts.limit != 200 || opts.cutoffDays != 30 || opts.dsn != "from-flag" {
		t.Fatalf("opts = %+v, want auto-archive dry-run limit=200 cutoff-days=30 dsn override", opts)
	}
	if opts.reason != defaultAutoArchiveReason || !opts.jsonOut {
		t.Fatalf("opts reason/json = %q/%t, want auto-archive default/true", opts.reason, opts.jsonOut)
	}
}

func TestUsage_AutoArchive_Help(t *testing.T) {
	var stderr bytes.Buffer
	if _, ok := parseSubcommand("auto-archive", []string{"--help"}, testEnv(nil), &stderr); ok {
		t.Fatal("parseSubcommand help ok = true, want false")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("cutoff-days")) {
		t.Fatalf("stderr = %q, want cutoff-days help", stderr.String())
	}
}

func testEnv(values map[string]string) envFunc {
	return func(key string) string {
		if values == nil {
			return ""
		}
		return values[key]
	}
}
