package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"workflow/config"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
	externalassets "workflow/service/external_assets"
)

type summary struct {
	OSSProcessed        int                            `json:"oss_processed"`
	OSSRequiredQueued   int64                          `json:"oss_required_queued,omitempty"`
	PreviewProcessed    int                            `json:"preview_processed"`
	DirectURLReady      int                            `json:"direct_url_ready"`
	DirectURLFailed     int                            `json:"direct_url_failed"`
	DirectURLCandidates int                            `json:"direct_url_candidates,omitempty"`
	FullSync            *externalassets.FullSyncResult `json:"full_sync,omitempty"`
	DryRun              bool                           `json:"dry_run"`
	Message             string                         `json:"message,omitempty"`
}

func main() {
	var limit int
	var timeout time.Duration
	var dryRun bool
	var fullSync bool
	flag.IntVar(&limit, "limit", 20, "maximum jobs per queue to process")
	flag.DurationVar(&timeout, "timeout", 30*time.Minute, "whole run timeout")
	flag.BoolVar(&dryRun, "dry-run", false, "print pending counts without uploading")
	flag.BoolVar(&fullSync, "full-sync", false, "scan configured AList mounts into the external asset index")
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
	dbConns := cfg.ExternalAssets.PrepareConcurrency + 2
	if dbConns < 3 {
		dbConns = 3
	}
	if dbConns > 20 {
		dbConns = 20
	}
	db.SetMaxOpenConns(dbConns)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(cfg.MySQL.ConnMaxLifetime)
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping mysql: %v", err)
	}

	mdb := mysqlrepo.New(db)
	repo := mysqlrepo.NewExternalAssetRepo(mdb)
	ossDirect := service.NewOSSDirectService(service.OSSDirectConfig{
		Enabled:         cfg.OSSDirect.Enabled,
		Endpoint:        cfg.OSSDirect.Endpoint,
		Bucket:          cfg.OSSDirect.Bucket,
		AccessKeyID:     cfg.OSSDirect.AccessKeyID,
		AccessKeySecret: cfg.OSSDirect.AccessKeySecret,
		PresignExpiry:   cfg.OSSDirect.PresignExpiry,
		HTTPTimeout:     cfg.OSSDirect.HTTPTimeout,
		PublicEndpoint:  cfg.OSSDirect.PublicEndpoint,
		PartSize:        cfg.OSSDirect.PartSize,
	})
	svc := externalassets.NewService(repo, externalassets.ConfigFromApp(cfg.ExternalAssets), ossDirect)
	if !svc.Enabled() {
		writeSummary(summary{DryRun: dryRun, Message: "external assets disabled"})
		return
	}
	if dryRun {
		ossRows, _ := repo.ListPendingOSS(ctx, limit)
		previewRows, _ := repo.ListPendingPreview(ctx, limit)
		directRows, _ := repo.ListDirectURLRefreshCandidates(ctx, limit, time.Now().UTC().Add(-cfg.ExternalAssets.LinkRefreshInterval))
		writeSummary(summary{
			OSSProcessed:        len(ossRows),
			PreviewProcessed:    len(previewRows),
			DirectURLCandidates: len(directRows),
			DryRun:              true,
			Message:             "pending counts only",
		})
		return
	}
	if fullSync {
		full, err := svc.SyncFullIndex(ctx)
		out := summary{FullSync: full}
		if queued, queueErr := svc.EnsureOSSRequiredPrefixesPending(ctx); queueErr != nil {
			if err == nil {
				err = queueErr
			} else {
				out.Message = err.Error()
			}
		} else {
			out.OSSRequiredQueued = queued
		}
		if err != nil {
			out.Message = err.Error()
		}
		writeSummary(out)
		if err != nil {
			os.Exit(1)
		}
		return
	}

	directReady, directFailed, err := svc.RefreshDirectURLs(ctx, limit)
	if err != nil {
		log.Fatalf("refresh direct urls: %v", err)
	}
	ossProcessed, err := svc.ProcessPendingOSS(ctx, limit)
	if err != nil {
		log.Fatalf("process pending oss: %v", err)
	}
	previewProcessed, err := svc.ProcessPendingPreview(ctx, limit)
	if err != nil {
		log.Fatalf("process pending preview: %v", err)
	}
	writeSummary(summary{
		OSSProcessed:     ossProcessed,
		PreviewProcessed: previewProcessed,
		DirectURLReady:   directReady,
		DirectURLFailed:  directFailed,
	})
}

func writeSummary(s summary) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		fmt.Fprintf(os.Stderr, "encode summary: %v\n", err)
		os.Exit(1)
	}
}
