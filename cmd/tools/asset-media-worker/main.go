package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/go-sql-driver/mysql"
	"time"
	"workflow/config"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
	assetworkbench "workflow/service/asset_workbench"
	externalassets "workflow/service/external_assets"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	db, err := sql.Open("mysql", cfg.MySQL.DSN)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	if err = db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}
	mdb := mysqlrepo.New(db)
	jobs := mysqlrepo.NewAssetMediaJobRepo(mdb)
	oss := service.NewOSSDirectService(service.OSSDirectConfig{Enabled: cfg.OSSDirect.Enabled, Endpoint: cfg.OSSDirect.Endpoint,
		PublicEndpoint: cfg.OSSDirect.PublicEndpoint, Bucket: cfg.OSSDirect.Bucket, AccessKeyID: cfg.OSSDirect.AccessKeyID,
		AccessKeySecret: cfg.OSSDirect.AccessKeySecret, PresignExpiry: cfg.OSSDirect.PresignExpiry, PartSize: cfg.OSSDirect.PartSize})
	svc := service.NewTaskAssetCenterService(mysqlrepo.NewTaskRepo(mdb), mysqlrepo.NewDesignAssetRepo(mdb), mysqlrepo.NewTaskAssetRepo(mdb),
		mysqlrepo.NewUploadRequestRepo(mdb), mysqlrepo.NewAssetStorageRefRepo(mdb), mysqlrepo.NewTaskEventRepo(mdb), mdb, nil,
		service.WithOSSDirectService(oss), service.WithTaskAssetMediaJobs(jobs, true))
	host, _ := os.Hostname()
	pool := os.Getenv("ASSET_MEDIA_WORKER_POOL")
	if pool == "" {
		pool = "ecs"
	}
	if !cfg.AssetMedia.JobsEnabled || (pool == "ecs" && !cfg.AssetMedia.ECSWorkerEnabled) || (pool == "scan" && !cfg.AssetMedia.NASScanEnabled) || (pool != "ecs" && pool != "scan") {
		log.Fatal("requested media executor is disabled or invalid")
	}
	workbench := assetworkbench.NewService(assetworkbench.Config{Timezone: cfg.AssetWorkbench.Timezone, OSSPrefix: cfg.AssetWorkbench.OSSPrefix, PreviewWorkerLeaseTTL: 2 * time.Minute, PreviewWorkerMaxAttempts: 4}, assetworkbench.WithRepository(mysqlrepo.NewAssetWorkbenchRepo(mdb), mdb), assetworkbench.WithOSSDirect(oss), assetworkbench.WithPreviewRenderer(service.NewExternalAssetPreviewRenderer()))
	var processor service.SystemMediaProcessor = service.MediaECSProcessor{Previews: svc.(service.SystemMediaProcessor), OSS: oss, ExistingPreviews: func(ctx context.Context) (int, error) {
		n, e := workbench.ProcessPendingPreviews(ctx, 1)
		if e != nil {
			return n, e
		}
		return n, nil
	}}
	if pool == "scan" {
		external := externalassets.NewService(mysqlrepo.NewExternalAssetRepoWithAIRetrieval(mdb, cfg.VectorSearch.EmbeddingVersion), externalassets.ConfigFromApp(cfg.ExternalAssets), oss)
		external.ConfigureMedia(jobs, cfg.AssetMedia)
		processor = external
	}
	if err = service.RunSystemMediaWorker(ctx, jobs, processor, pool+"-"+host, pool); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
