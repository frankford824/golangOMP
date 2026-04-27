package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
	assetlifecycle "workflow/service/asset_lifecycle"
	taskdraft "workflow/service/task_draft"
	tasklifecycle "workflow/service/task_lifecycle"
)

const defaultReason = "v1.r6.a.1.manual.cleanup"
const defaultAutoArchiveReason = "v1.r6.a.3.manual.auto-archive"

type envFunc func(string) string

type commandOptions struct {
	subcommand string
	dryRun     bool
	limit      int
	cutoffDays int
	dsn        string
	reason     string
	jsonOut    bool
}

type noopDeleter struct{}

func (noopDeleter) Enabled() bool { return false }
func (noopDeleter) DeleteObject(context.Context, string) error {
	return nil
}

func main() {
	code := run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(args []string, getenv envFunc, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	subcommand := args[0]
	if subcommand != "oss-365" && subcommand != "drafts-7d" && subcommand != "auto-archive" {
		printUsage(stderr)
		return 2
	}
	opts, ok := parseSubcommand(subcommand, args[1:], getenv, stderr)
	if !ok {
		return 2
	}
	start := time.Now()
	if opts.subcommand == "auto-archive" {
		fmt.Fprintf(stderr, "[RUN-CLEANUP] subcommand=%s dry_run=%t limit=%d cutoff_days=%d ts=%s\n", opts.subcommand, opts.dryRun, opts.limit, opts.cutoffDays, start.UTC().Format(time.RFC3339))
	} else {
		fmt.Fprintf(stderr, "[RUN-CLEANUP] subcommand=%s dry_run=%t limit=%d ts=%s\n", opts.subcommand, opts.dryRun, opts.limit, start.UTC().Format(time.RFC3339))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	switch opts.subcommand {
	case "oss-365":
		return runOSS365(ctx, opts, getenv, stdout, start)
	case "drafts-7d":
		return runDrafts7D(ctx, opts, stdout, start)
	case "auto-archive":
		return runAutoArchive(ctx, opts, stdout, start)
	default:
		printUsage(stderr)
		return 2
	}
}

func parseSubcommand(subcommand string, args []string, getenv envFunc, stderr io.Writer) (commandOptions, bool) {
	opts := commandOptions{subcommand: subcommand, limit: 100, cutoffDays: 90, dsn: getenv("MYSQL_DSN"), reason: defaultReason, jsonOut: true}
	if subcommand == "auto-archive" {
		opts.limit = 1000
		opts.reason = defaultAutoArchiveReason
	}
	fs := flag.NewFlagSet(subcommand, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&opts.dryRun, "dry-run", false, "preview cleanup without deleting data")
	if subcommand == "auto-archive" {
		fs.IntVar(&opts.limit, "limit", 1000, "auto-archive candidate limit")
	} else {
		fs.IntVar(&opts.limit, "limit", 100, "cleanup candidate limit")
	}
	fs.IntVar(&opts.cutoffDays, "cutoff-days", 90, "auto-archive cutoff age in days")
	fs.StringVar(&opts.dsn, "dsn", opts.dsn, "MySQL DSN")
	fs.StringVar(&opts.reason, "reason", opts.reason, "cleanup reason")
	fs.BoolVar(&opts.jsonOut, "json", true, "write structured JSON")
	if err := fs.Parse(args); err != nil {
		printUsage(stderr)
		return opts, false
	}
	if fs.NArg() != 0 {
		printUsage(stderr)
		return opts, false
	}
	return opts, true
}

func runOSS365(ctx context.Context, opts commandOptions, getenv envFunc, stdout io.Writer, start time.Time) int {
	db, err := openDB(ctx, opts.dsn)
	if err != nil {
		writeError(stdout, opts.subcommand, err)
		return 1
	}
	defer db.Close()

	mdb := mysqlrepo.New(db)
	lifecycleRepo := mysqlrepo.NewTaskAssetLifecycleRepo(mdb)
	deleter, err := buildObjectDeleter(opts.dryRun, getenv)
	if err != nil {
		writeError(stdout, opts.subcommand, err)
		return 1
	}
	job := assetlifecycle.NewCleanupJob(lifecycleRepo, mdb, deleter, log.New(os.Stderr, "", log.LstdFlags))
	result, appErr := job.Run(ctx, assetlifecycle.CleanupOptions{DryRun: opts.dryRun, Limit: opts.limit})
	if appErr != nil {
		writeError(stdout, opts.subcommand, fmt.Errorf("%s: %s", appErr.Code, appErr.Message))
		return 1
	}
	return writeJSON(stdout, map[string]interface{}{
		"subcommand": opts.subcommand,
		"dry_run":    opts.dryRun,
		"scanned":    result.Scanned,
		"cleaned":    result.Cleaned,
		"elapsed_ms": elapsedMS(start),
	})
}

func runDrafts7D(ctx context.Context, opts commandOptions, stdout io.Writer, start time.Time) int {
	db, err := openDB(ctx, opts.dsn)
	if err != nil {
		writeError(stdout, opts.subcommand, err)
		return 1
	}
	defer db.Close()

	mdb := mysqlrepo.New(db)
	draftRepo := mysqlrepo.NewTaskDraftRepo(mdb)
	logRepo := mysqlrepo.NewPermissionLogRepo(mdb)
	srv := taskdraft.NewService(draftRepo, logRepo, mdb)
	deleted := 0
	if opts.dryRun {
		deleted, err = countExpiredDrafts(ctx, db)
	} else {
		deleted, err = srv.CleanupExpired(ctx)
	}
	if err != nil {
		writeError(stdout, opts.subcommand, err)
		return 1
	}
	return writeJSON(stdout, map[string]interface{}{
		"subcommand": opts.subcommand,
		"dry_run":    opts.dryRun,
		"deleted":    deleted,
		"elapsed_ms": elapsedMS(start),
	})
}

func runAutoArchive(ctx context.Context, opts commandOptions, stdout io.Writer, start time.Time) int {
	db, err := openDB(ctx, opts.dsn)
	if err != nil {
		writeError(stdout, opts.subcommand, err)
		return 1
	}
	defer db.Close()

	mdb := mysqlrepo.New(db)
	archiveRepo := mysqlrepo.NewTaskAutoArchiveRepo(mdb)
	job := tasklifecycle.NewAutoArchiveJob(archiveRepo, mdb, log.New(os.Stderr, "", log.LstdFlags))
	result, appErr := job.Run(ctx, tasklifecycle.AutoArchiveOptions{DryRun: opts.dryRun, Limit: opts.limit, CutoffDays: opts.cutoffDays})
	if appErr != nil {
		writeError(stdout, opts.subcommand, fmt.Errorf("%s: %s", appErr.Code, appErr.Message))
		return 1
	}
	return writeJSON(stdout, map[string]interface{}{
		"subcommand": opts.subcommand,
		"dry_run":    opts.dryRun,
		"scanned":    result.Scanned,
		"archived":   result.Archived,
		"cutoff":     result.Cutoff.Format(time.RFC3339),
		"elapsed_ms": elapsedMS(start),
	})
}

func openDB(ctx context.Context, dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("mysql dsn is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}

func buildObjectDeleter(dryRun bool, getenv envFunc) (assetlifecycle.ObjectDeleter, error) {
	if dryRun || getenv("OSS_DELETER_DISABLED") == "1" {
		return noopDeleter{}, nil
	}
	deleter := service.NewOSSDirectService(service.OSSDirectConfig{
		Enabled:         true,
		Endpoint:        getenv("OSS_ENDPOINT"),
		Bucket:          getenv("OSS_BUCKET"),
		AccessKeyID:     getenv("OSS_ACCESS_KEY_ID"),
		AccessKeySecret: getenv("OSS_ACCESS_KEY_SECRET"),
		PublicEndpoint:  getenv("OSS_PUBLIC_ENDPOINT"),
	})
	if !deleter.Enabled() {
		return nil, fmt.Errorf("oss deleter is not configured; set OSS_ENDPOINT/OSS_BUCKET/OSS_ACCESS_KEY_ID/OSS_ACCESS_KEY_SECRET or OSS_DELETER_DISABLED=1")
	}
	return deleter, nil
}

func countExpiredDrafts(ctx context.Context, db *sql.DB) (int, error) {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_drafts WHERE expires_at < ? AND expires_at IS NOT NULL`, time.Now().UTC()).Scan(&n); err != nil {
		return 0, fmt.Errorf("count expired task_drafts: %w", err)
	}
	return n, nil
}

func writeError(stdout io.Writer, subcommand string, err error) {
	_ = writeJSON(stdout, map[string]interface{}{
		"subcommand": subcommand,
		"error":      err.Error(),
	})
}

func writeJSON(stdout io.Writer, payload map[string]interface{}) int {
	raw, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(stdout, `{"error":%q}`+"\n", err.Error())
		return 1
	}
	fmt.Fprintln(stdout, string(raw))
	return 0
}

func elapsedMS(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: run-cleanup <oss-365|drafts-7d|auto-archive> [--dry-run] [--limit n] [--cutoff-days n] [--dsn dsn] [--reason reason] [--json]")
}
