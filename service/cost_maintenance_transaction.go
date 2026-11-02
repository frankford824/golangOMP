package service

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"
	"workflow/domain"
	"workflow/repo"
)

// Only replay transactional DB work after InnoDB has rolled back the entire
// victim transaction (1213). ERP baseline reads and external writes stay outside
// this callback. Never retry ambiguous commit/network errors or lock timeouts.
func (s *taskService) runCostMaintenanceTransaction(ctx context.Context, id int64, fn func(repo.Tx) error) error {
	const attempts = 4
	run := s.txRunner.RunInTx
	if currentReads, ok := s.txRunner.(interface {
		RunInReadCommittedTx(context.Context, func(repo.Tx) error) error
	}); ok {
		run = currentReads.RunInReadCommittedTx
	}
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := run(ctx, fn)
		var deadlock *mysql.MySQLError
		if !errors.As(err, &deadlock) || deadlock.Number != 1213 {
			return err
		}
		if attempt == attempts {
			return domain.NewAppError(domain.ErrCodeConflict, "后台数据正在更新，请重新读取后再确认；本次未保存", nil)
		}
		log.Printf("cost_maintenance_deadlock_retry trace_id=%s sku_item_id=%d attempt=%d", domain.TraceIDFromContext(ctx), id, attempt)
		timer := time.NewTimer(time.Duration(25*(1<<(attempt-1))) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
