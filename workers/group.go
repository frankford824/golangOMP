package workers

import (
	"context"
	"database/sql"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"workflow/service"
)

// Group manages all background workers (spec §4.1).
type Group struct {
	db          *sql.DB
	rdb         *redis.Client
	logger      *zap.Logger
	erpSyncSvc  service.ERPSyncService
	erpEnabled  bool
	erpInterval time.Duration
}

func NewGroup(
	db *sql.DB,
	rdb *redis.Client,
	logger *zap.Logger,
	erpSyncSvc service.ERPSyncService,
	erpEnabled bool,
	erpInterval time.Duration,
) *Group {
	return &Group{
		db:          db,
		rdb:         rdb,
		logger:      logger,
		erpSyncSvc:  erpSyncSvc,
		erpEnabled:  erpEnabled,
		erpInterval: erpInterval,
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
}
