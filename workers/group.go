package workers

import (
	"context"
	"database/sql"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
	"workflow/service"
)

// Group manages all background workers (spec §4.1).
type Group struct {
	db                             *sql.DB
	rdb                            *redis.Client
	logger                         *zap.Logger
	erpSyncSvc                     service.ERPSyncService
	productMgmt                    service.ProductManagementService
	skuComboSync                   service.SKUComboSyncService
	notificationProcessor          NotificationProcessor
	assetWorkbenchPreview          AssetWorkbenchPreviewProcessor
	assetWorkbenchMaintenance      AssetWorkbenchMaintenanceProcessor
	assetWorkbenchBatchJob         AssetWorkbenchBatchJobProcessor
	asyncProjectionOutbox          repo.AsyncProjectionOutboxRepo
	asyncProjectionTx              repo.TxRunner
	taskERPOutboxProcessor         service.TaskERPOutboxProcessor
	erpEnabled                     bool
	erpInterval                    time.Duration
	webPushEnabled                 bool
	webPushInterval                time.Duration
	webPushLimit                   int
	skuFailureReconcileInterval    time.Duration
	skuFailureReconcileLimit       int
	assetWorkbenchPreviewEnabled   bool
	assetWorkbenchPreviewInterval  time.Duration
	assetWorkbenchPreviewLimit     int
	assetWorkbenchExpiryEnabled    bool
	assetWorkbenchExpiryInterval   time.Duration
	assetWorkbenchExpiryLimit      int
	assetWorkbenchBatchJobEnabled  bool
	assetWorkbenchBatchJobInterval time.Duration
	assetWorkbenchBatchJobLimit    int
}

type GroupDeps struct {
	DB                              *sql.DB
	Redis                           *redis.Client
	Logger                          *zap.Logger
	ERPSync                         service.ERPSyncService
	ProductManagement               service.ProductManagementService
	SKUComboSync                    service.SKUComboSyncService
	Notification                    NotificationProcessor
	AssetWorkbenchPreview           AssetWorkbenchPreviewProcessor
	AssetWorkbenchMaintenance       AssetWorkbenchMaintenanceProcessor
	AssetWorkbenchBatchJob          AssetWorkbenchBatchJobProcessor
	AsyncProjectionOutbox           repo.AsyncProjectionOutboxRepo
	AsyncProjectionTx               repo.TxRunner
	TaskERPOutboxProcessor          service.TaskERPOutboxProcessor
	ERPEnabled                      bool
	ERPInterval                     time.Duration
	WebPushEnabled                  bool
	WebPushInterval                 time.Duration
	WebPushLimit                    int
	SKUSyncFailureReconcileInterval time.Duration
	SKUSyncFailureReconcileLimit    int
	AssetWorkbenchPreviewEnabled    bool
	AssetWorkbenchPreviewInterval   time.Duration
	AssetWorkbenchPreviewLimit      int
	AssetWorkbenchExpiryEnabled     bool
	AssetWorkbenchExpiryInterval    time.Duration
	AssetWorkbenchExpiryLimit       int
	AssetWorkbenchBatchJobEnabled   bool
	AssetWorkbenchBatchJobInterval  time.Duration
	AssetWorkbenchBatchJobLimit     int
}

type AssetWorkbenchPreviewProcessor interface {
	ProcessPendingPreviews(ctx context.Context, limit int) (int, *domain.AppError)
}

type AssetWorkbenchMaintenanceProcessor interface {
	ExpireUploadSessions(ctx context.Context, limit int) (int, *domain.AppError)
}

type AssetWorkbenchBatchJobProcessor interface {
	ProcessPendingBatchJobs(ctx context.Context, workerID string, limit int) (int, *domain.AppError)
}

func NewGroup(deps GroupDeps) *Group {
	return &Group{
		db:                             deps.DB,
		rdb:                            deps.Redis,
		logger:                         deps.Logger,
		erpSyncSvc:                     deps.ERPSync,
		productMgmt:                    deps.ProductManagement,
		skuComboSync:                   deps.SKUComboSync,
		notificationProcessor:          deps.Notification,
		assetWorkbenchPreview:          deps.AssetWorkbenchPreview,
		assetWorkbenchMaintenance:      deps.AssetWorkbenchMaintenance,
		assetWorkbenchBatchJob:         deps.AssetWorkbenchBatchJob,
		asyncProjectionOutbox:          deps.AsyncProjectionOutbox,
		asyncProjectionTx:              deps.AsyncProjectionTx,
		taskERPOutboxProcessor:         deps.TaskERPOutboxProcessor,
		erpEnabled:                     deps.ERPEnabled,
		erpInterval:                    deps.ERPInterval,
		webPushEnabled:                 deps.WebPushEnabled,
		webPushInterval:                deps.WebPushInterval,
		webPushLimit:                   deps.WebPushLimit,
		skuFailureReconcileInterval:    deps.SKUSyncFailureReconcileInterval,
		skuFailureReconcileLimit:       deps.SKUSyncFailureReconcileLimit,
		assetWorkbenchPreviewEnabled:   deps.AssetWorkbenchPreviewEnabled,
		assetWorkbenchPreviewInterval:  deps.AssetWorkbenchPreviewInterval,
		assetWorkbenchPreviewLimit:     deps.AssetWorkbenchPreviewLimit,
		assetWorkbenchExpiryEnabled:    deps.AssetWorkbenchExpiryEnabled,
		assetWorkbenchExpiryInterval:   deps.AssetWorkbenchExpiryInterval,
		assetWorkbenchExpiryLimit:      deps.AssetWorkbenchExpiryLimit,
		assetWorkbenchBatchJobEnabled:  deps.AssetWorkbenchBatchJobEnabled,
		assetWorkbenchBatchJobInterval: deps.AssetWorkbenchBatchJobInterval,
		assetWorkbenchBatchJobLimit:    deps.AssetWorkbenchBatchJobLimit,
	}
}

// Start launches all workers as goroutines. All stop when ctx is cancelled.
func (g *Group) Start(ctx context.Context) {
	go NewLeaseReaper(g.db, g.logger).Run(ctx)
	go NewRetryScheduler(g.db, g.logger).Run(ctx)
	go NewVerifyWorker(g.db, g.rdb, g.logger).Run(ctx)
	go NewEventDispatcher(g.db, g.rdb, g.logger).Run(ctx)
	if g.asyncProjectionOutbox != nil && g.asyncProjectionTx != nil {
		go NewSearchReindexOutboxWorker(g.asyncProjectionOutbox, g.asyncProjectionTx, AsyncOutboxWorkerConfig{}, g.logger.Named("search_reindex_outbox")).Run(ctx)
		if g.taskERPOutboxProcessor != nil {
			go NewTaskERPOutboxWorker(g.asyncProjectionOutbox, g.asyncProjectionTx, g.taskERPOutboxProcessor, AsyncOutboxWorkerConfig{}, g.logger.Named("task_erp_outbox")).Run(ctx)
		}
	}
	if g.erpEnabled && g.erpSyncSvc != nil {
		go NewERPSyncWorker(g.erpSyncSvc, g.logger, g.erpInterval).Run(ctx)
	}
	if g.shouldStartProductManagementSyncWorker() {
		go NewProductManagementSyncWorker(g.productMgmt, g.logger, 15*time.Second, 8).Run(ctx)
	}
	if g.shouldStartSKUComboSyncWorker() {
		go NewSKUComboSyncWorker(g.skuComboSync, g.logger, time.Minute).Run(ctx)
	}
	if g.shouldStartNotificationWebPushWorker() {
		go NewNotificationWebPushWorker(g.notificationProcessor, g.logger, g.webPushInterval, g.webPushLimit).Run(ctx)
	}
	if g.shouldStartSKUSyncFailureNotificationWorker() {
		go NewSKUSyncFailureNotificationWorker(g.notificationProcessor, g.logger, g.skuFailureReconcileInterval, g.skuFailureReconcileLimit).Run(ctx)
	}
	if g.shouldStartAssetWorkbenchPreviewWorker() {
		go NewAssetWorkbenchPreviewWorker(g.assetWorkbenchPreview, g.logger, g.assetWorkbenchPreviewInterval, g.assetWorkbenchPreviewLimit).Run(ctx)
	}
	if g.shouldStartAssetWorkbenchUploadExpiryWorker() {
		go NewAssetWorkbenchUploadExpiryWorker(g.assetWorkbenchMaintenance, g.logger, g.assetWorkbenchExpiryInterval, g.assetWorkbenchExpiryLimit).Run(ctx)
	}
	if g.shouldStartAssetWorkbenchBatchJobWorker() {
		go NewAssetWorkbenchBatchJobWorker(g.assetWorkbenchBatchJob, g.logger, g.assetWorkbenchBatchJobInterval, g.assetWorkbenchBatchJobLimit).Run(ctx)
	}
}

func (g *Group) shouldStartProductManagementSyncWorker() bool {
	return g.erpEnabled && g.productMgmt != nil
}

func (g *Group) shouldStartSKUComboSyncWorker() bool {
	return g.erpEnabled && g.skuComboSync != nil
}

func (g *Group) shouldStartNotificationWebPushWorker() bool {
	return g.webPushEnabled && g.notificationProcessor != nil
}

func (g *Group) shouldStartSKUSyncFailureNotificationWorker() bool {
	return g.notificationProcessor != nil
}

func (g *Group) shouldStartAssetWorkbenchPreviewWorker() bool {
	return g.assetWorkbenchPreviewEnabled && g.assetWorkbenchPreview != nil
}

func (g *Group) shouldStartAssetWorkbenchUploadExpiryWorker() bool {
	return g.assetWorkbenchExpiryEnabled && g.assetWorkbenchMaintenance != nil
}

func (g *Group) shouldStartAssetWorkbenchBatchJobWorker() bool {
	return g.assetWorkbenchBatchJobEnabled && g.assetWorkbenchBatchJob != nil
}
