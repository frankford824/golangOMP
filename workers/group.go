package workers

import (
	"context"
	"database/sql"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"workflow/domain"
	"workflow/service"
)

// Group manages all background workers (spec §4.1).
type Group struct {
	db                            *sql.DB
	rdb                           *redis.Client
	logger                        *zap.Logger
	erpSyncSvc                    service.ERPSyncService
	productMgmt                   service.ProductManagementService
	skuComboSync                  service.SKUComboSyncService
	assetWorkbenchPreview         AssetWorkbenchPreviewProcessor
	assetWorkbenchMaintenance     AssetWorkbenchMaintenanceProcessor
	erpEnabled                    bool
	erpInterval                   time.Duration
	assetWorkbenchPreviewEnabled  bool
	assetWorkbenchPreviewInterval time.Duration
	assetWorkbenchPreviewLimit    int
	assetWorkbenchExpiryEnabled   bool
	assetWorkbenchExpiryInterval  time.Duration
	assetWorkbenchExpiryLimit     int
}

type GroupDeps struct {
	DB                            *sql.DB
	Redis                         *redis.Client
	Logger                        *zap.Logger
	ERPSync                       service.ERPSyncService
	ProductManagement             service.ProductManagementService
	SKUComboSync                  service.SKUComboSyncService
	AssetWorkbenchPreview         AssetWorkbenchPreviewProcessor
	AssetWorkbenchMaintenance     AssetWorkbenchMaintenanceProcessor
	ERPEnabled                    bool
	ERPInterval                   time.Duration
	AssetWorkbenchPreviewEnabled  bool
	AssetWorkbenchPreviewInterval time.Duration
	AssetWorkbenchPreviewLimit    int
	AssetWorkbenchExpiryEnabled   bool
	AssetWorkbenchExpiryInterval  time.Duration
	AssetWorkbenchExpiryLimit     int
}

type AssetWorkbenchPreviewProcessor interface {
	ProcessPendingPreviews(ctx context.Context, limit int) (int, *domain.AppError)
}

type AssetWorkbenchMaintenanceProcessor interface {
	ExpireUploadSessions(ctx context.Context, limit int) (int, *domain.AppError)
}

func NewGroup(deps GroupDeps) *Group {
	return &Group{
		db:                            deps.DB,
		rdb:                           deps.Redis,
		logger:                        deps.Logger,
		erpSyncSvc:                    deps.ERPSync,
		productMgmt:                   deps.ProductManagement,
		skuComboSync:                  deps.SKUComboSync,
		assetWorkbenchPreview:         deps.AssetWorkbenchPreview,
		assetWorkbenchMaintenance:     deps.AssetWorkbenchMaintenance,
		erpEnabled:                    deps.ERPEnabled,
		erpInterval:                   deps.ERPInterval,
		assetWorkbenchPreviewEnabled:  deps.AssetWorkbenchPreviewEnabled,
		assetWorkbenchPreviewInterval: deps.AssetWorkbenchPreviewInterval,
		assetWorkbenchPreviewLimit:    deps.AssetWorkbenchPreviewLimit,
		assetWorkbenchExpiryEnabled:   deps.AssetWorkbenchExpiryEnabled,
		assetWorkbenchExpiryInterval:  deps.AssetWorkbenchExpiryInterval,
		assetWorkbenchExpiryLimit:     deps.AssetWorkbenchExpiryLimit,
	}
}

// Start launches all workers as goroutines. All stop when ctx is cancelled.
func (g *Group) Start(ctx context.Context) {
	go NewLeaseReaper(g.db, g.logger).Run(ctx)
	go NewRetryScheduler(g.db, g.logger).Run(ctx)
	go NewVerifyWorker(g.db, g.rdb, g.logger).Run(ctx)
	go NewEventDispatcher(g.db, g.rdb, g.logger).Run(ctx)
	if g.erpEnabled && g.erpSyncSvc != nil {
		go NewERPSyncWorker(g.erpSyncSvc, g.logger, g.erpInterval).Run(ctx)
	}
	if g.shouldStartProductManagementSyncWorker() {
		go NewProductManagementSyncWorker(g.productMgmt, g.logger, 15*time.Second, 8).Run(ctx)
	}
	if g.shouldStartSKUComboSyncWorker() {
		go NewSKUComboSyncWorker(g.skuComboSync, g.logger, time.Minute).Run(ctx)
	}
	if g.shouldStartAssetWorkbenchPreviewWorker() {
		go NewAssetWorkbenchPreviewWorker(g.assetWorkbenchPreview, g.logger, g.assetWorkbenchPreviewInterval, g.assetWorkbenchPreviewLimit).Run(ctx)
	}
	if g.shouldStartAssetWorkbenchUploadExpiryWorker() {
		go NewAssetWorkbenchUploadExpiryWorker(g.assetWorkbenchMaintenance, g.logger, g.assetWorkbenchExpiryInterval, g.assetWorkbenchExpiryLimit).Run(ctx)
	}
}

func (g *Group) shouldStartProductManagementSyncWorker() bool {
	return g.erpEnabled && g.productMgmt != nil
}

func (g *Group) shouldStartSKUComboSyncWorker() bool {
	return g.erpEnabled && g.skuComboSync != nil
}

func (g *Group) shouldStartAssetWorkbenchPreviewWorker() bool {
	return g.assetWorkbenchPreviewEnabled && g.assetWorkbenchPreview != nil
}

func (g *Group) shouldStartAssetWorkbenchUploadExpiryWorker() bool {
	return g.assetWorkbenchExpiryEnabled && g.assetWorkbenchMaintenance != nil
}
