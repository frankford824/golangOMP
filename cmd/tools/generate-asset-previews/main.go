package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"workflow/config"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
)

type sourceAssetJob struct {
	TaskID        int64  `json:"task_id"`
	SourceAssetID int64  `json:"source_asset_id"`
	ActorID       int64  `json:"actor_id"`
	Filename      string `json:"filename"`
	MimeType      string `json:"mime_type"`
	StorageKey    string `json:"storage_key"`
}

type runSummary struct {
	Scanned   int      `json:"scanned"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Skipped   int      `json:"skipped"`
	Failures  []string `json:"failures,omitempty"`
	DryRun    bool     `json:"dry_run"`
}

func main() {
	var limit int
	var timeout time.Duration
	var dryRun bool
	var onlyAssetID int64
	flag.IntVar(&limit, "limit", 0, "maximum source assets to process; 0 means no explicit limit")
	flag.DurationVar(&timeout, "timeout", 30*time.Minute, "whole run timeout")
	flag.BoolVar(&dryRun, "dry-run", false, "list jobs without generating previews")
	flag.Int64Var(&onlyAssetID, "asset-id", 0, "process one source asset id only")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	db, err := sql.Open("mysql", cfg.MySQL.DSN)
	if err != nil {
		log.Fatalf("open mysql: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(cfg.MySQL.ConnMaxLifetime)
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping mysql: %v", err)
	}

	jobs, err := listAsyncPreviewJobs(ctx, db, limit, onlyAssetID)
	if err != nil {
		log.Fatalf("list jobs: %v", err)
	}
	summary := runSummary{Scanned: len(jobs), DryRun: dryRun}
	if dryRun {
		_ = json.NewEncoder(os.Stdout).Encode(struct {
			Summary runSummary       `json:"summary"`
			Jobs    []sourceAssetJob `json:"jobs"`
		}{Summary: summary, Jobs: jobs})
		return
	}

	mdb := mysqlrepo.New(db)
	ossDirect := service.NewOSSDirectService(service.OSSDirectConfig{
		Enabled:         cfg.OSSDirect.Enabled,
		Endpoint:        cfg.OSSDirect.Endpoint,
		Bucket:          cfg.OSSDirect.Bucket,
		AccessKeyID:     cfg.OSSDirect.AccessKeyID,
		AccessKeySecret: cfg.OSSDirect.AccessKeySecret,
		PresignExpiry:   cfg.OSSDirect.PresignExpiry,
		PublicEndpoint:  cfg.OSSDirect.PublicEndpoint,
		PartSize:        cfg.OSSDirect.PartSize,
	})
	uploadClient := service.NewUploadServiceClient(service.UploadServiceClientConfig{
		Enabled:                 cfg.UploadService.Enabled,
		BaseURL:                 cfg.UploadService.BaseURL,
		BrowserMultipartBaseURL: cfg.UploadService.BrowserMultipartBaseURL,
		BrowserDownloadBaseURL:  cfg.UploadService.BrowserDownloadBaseURL,
		Timeout:                 cfg.UploadService.Timeout,
		InternalToken:           cfg.UploadService.InternalToken,
		StorageProvider:         cfg.UploadService.StorageProvider,
	})
	taskAssetCenterSvc := service.NewTaskAssetCenterService(
		mysqlrepo.NewTaskRepo(mdb),
		mysqlrepo.NewDesignAssetRepo(mdb),
		mysqlrepo.NewTaskAssetRepo(mdb),
		mysqlrepo.NewUploadRequestRepo(mdb),
		mysqlrepo.NewAssetStorageRefRepo(mdb),
		mysqlrepo.NewTaskEventRepo(mdb),
		mdb,
		uploadClient,
		service.WithOSSDirectService(ossDirect),
	)

	for _, job := range jobs {
		if job.ActorID <= 0 {
			summary.Skipped++
			summary.Failures = append(summary.Failures, fmt.Sprintf("asset_id=%d skipped: actor_id is empty", job.SourceAssetID))
			continue
		}
		if appErr := taskAssetCenterSvc.EnsureDerivedPreviewAssets(ctx, job.TaskID, job.SourceAssetID, job.ActorID); appErr != nil {
			summary.Failed++
			summary.Failures = append(summary.Failures, fmt.Sprintf("asset_id=%d task_id=%d: %s", job.SourceAssetID, job.TaskID, appErr.Message))
			continue
		}
		summary.Succeeded++
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		log.Fatalf("encode summary: %v", err)
	}
	if summary.Failed > 0 {
		os.Exit(1)
	}
}

func listAsyncPreviewJobs(ctx context.Context, db *sql.DB, limit int, onlyAssetID int64) ([]sourceAssetJob, error) {
	where := []string{
		"da.asset_type NOT IN ('preview', 'design_thumb')",
		"COALESCE(ta.storage_key, '') <> ''",
		"COALESCE(ta.upload_status, '') = 'uploaded'",
		"COALESCE(ta.file_size, 0) >= 1024",
		"ta.cleaned_at IS NULL",
		"ta.deleted_at IS NULL",
		`LOWER(COALESCE(ta.original_filename, ta.file_name, '')) NOT REGEXP '[.](jpe?g|png|bmp|gif|webp|tiff?|heic|heif|avif)$'`,
		`(
			LOWER(COALESCE(ta.original_filename, ta.file_name, '')) REGEXP '[.](psd|psb|pdf|ai|eps|ps)$'
			OR LOWER(COALESCE(ta.mime_type, '')) IN ('image/vnd.adobe.photoshop', 'application/pdf', 'application/postscript', 'application/illustrator', 'application/vnd.adobe.illustrator')
		)`,
		`(
			NOT EXISTS (
				SELECT 1
				FROM design_assets dp
				JOIN task_assets tp ON tp.id = dp.current_version_id
				WHERE dp.source_asset_id = da.id
				  AND dp.asset_type = 'preview'
				  AND tp.source_asset_version_id = ta.id
				  AND COALESCE(tp.storage_key, '') <> ''
				  AND COALESCE(tp.mime_type, '') = 'image/webp'
				  AND COALESCE(tp.remark, '') = 'async-derived-preview:webp'
				  AND tp.cleaned_at IS NULL
				  AND tp.deleted_at IS NULL
			)
			OR NOT EXISTS (
				SELECT 1
				FROM design_assets dt
				JOIN task_assets tt ON tt.id = dt.current_version_id
				WHERE dt.source_asset_id = da.id
				  AND dt.asset_type = 'design_thumb'
				  AND tt.source_asset_version_id = ta.id
				  AND COALESCE(tt.storage_key, '') <> ''
				  AND COALESCE(tt.mime_type, '') = 'image/webp'
				  AND COALESCE(tt.remark, '') = 'async-derived-preview:webp'
				  AND tt.cleaned_at IS NULL
				  AND tt.deleted_at IS NULL
			)
		)`,
	}
	args := []interface{}{}
	if onlyAssetID > 0 {
		where = append(where, "da.id = ?")
		args = append(args, onlyAssetID)
	}
	query := `
SELECT da.task_id, da.id, COALESCE(NULLIF(ta.uploaded_by, 0), da.created_by), COALESCE(ta.original_filename, ta.file_name, ''), COALESCE(ta.mime_type, ''), COALESCE(ta.storage_key, '')
FROM design_assets da
JOIN task_assets ta ON ta.id = da.current_version_id
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY da.updated_at ASC, da.id ASC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []sourceAssetJob
	for rows.Next() {
		var job sourceAssetJob
		if err := rows.Scan(&job.TaskID, &job.SourceAssetID, &job.ActorID, &job.Filename, &job.MimeType, &job.StorageKey); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}
