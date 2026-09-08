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

// runAssetTransaction is intentionally local to replay-safe asset writes. fn
// must perform only transactional DB work and reset any attempt-local outputs.
// Do not retry ambiguous commit/network failures, permission errors or CAS
// conflicts. InnoDB error 1213 guarantees the victim transaction was rolled back.
func (s *taskAssetCenterService) runAssetTransaction(ctx context.Context, taskID int64, operation string, fn func(repo.Tx) error) error {
	const maxAttempts = 4
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := s.txRunner.RunInTx(ctx, fn)
		var mysqlErr *mysql.MySQLError
		if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1213 || attempt == maxAttempts {
			return err
		}
		log.Printf("asset_transaction_deadlock_retry trace_id=%s task_id=%d operation=%s attempt=%d", domain.TraceIDFromContext(ctx), taskID, operation, attempt)
		timer := time.NewTimer(time.Duration(25*(1<<(attempt-1))) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
